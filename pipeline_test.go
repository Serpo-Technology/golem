package main

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// A plain command is a one-stage pipeline; quoting groups words
// literally and never expands anything.
func TestParseSimpleCommands(t *testing.T) {
	p, err := parsePipeline("go test ./pkg")
	if err != nil {
		t.Fatal(err)
	}
	if len(p.stages) != 1 || !slices.Equal(p.stages[0], []string{"go", "test", "./pkg"}) {
		t.Fatalf("argv wrong: %+v", p.stages)
	}

	p, err = parsePipeline(`grep 'hello world' "two words" plain`)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"grep", "hello world", "two words", "plain"}
	if !slices.Equal(p.stages[0], want) {
		t.Fatalf("quoted argv wrong: %+v", p.stages[0])
	}

	// Globbing and substitution characters are literal inside quotes,
	// because no shell will ever see them.
	p, err = parsePipeline(`echo '*.go' '$HOME' 'a;b'`)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(p.stages[0], []string{"echo", "*.go", "$HOME", "a;b"}) {
		t.Fatalf("quoted specials should be literal: %+v", p.stages[0])
	}
}

// Pipes split stages and redirects attach to their ends: input on the
// first stage, output on the last.
func TestParsePipesAndRedirects(t *testing.T) {
	p, err := parsePipeline("make test | tee log")
	if err != nil {
		t.Fatal(err)
	}
	if len(p.stages) != 2 ||
		!slices.Equal(p.stages[0], []string{"make", "test"}) ||
		!slices.Equal(p.stages[1], []string{"tee", "log"}) {
		t.Fatalf("stages wrong: %+v", p.stages)
	}

	p, err = parsePipeline("sort < in.txt | uniq > out.txt")
	if err != nil {
		t.Fatal(err)
	}
	if p.inFile != "in.txt" || p.outFile != "out.txt" || p.appendOut {
		t.Fatalf("redirects wrong: %+v", p)
	}

	p, err = parsePipeline("echo hi >> log.txt")
	if err != nil {
		t.Fatal(err)
	}
	if p.outFile != "log.txt" || !p.appendOut {
		t.Fatalf("append redirect wrong: %+v", p)
	}
}

// A leading unquoted tilde expands to the operator's home; a quoted one
// stays literal.
func TestParseTilde(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip("no home dir in this environment")
	}
	p, err := parsePipeline("ls ~/tmp")
	if err != nil {
		t.Fatal(err)
	}
	if p.stages[0][1] != filepath.Join(home, "tmp") {
		t.Fatalf("tilde should expand to home, got %q", p.stages[0][1])
	}
	p, err = parsePipeline(`echo '~/tmp'`)
	if err != nil {
		t.Fatal(err)
	}
	if p.stages[0][1] != "~/tmp" {
		t.Fatalf("quoted tilde must stay literal, got %q", p.stages[0][1])
	}
}

// Everything the safe subset cannot model is refused with a reason,
// never approximated and never handed to a shell.
func TestParseRefusals(t *testing.T) {
	cases := []struct{ cmd, reason string }{
		{"echo $(whoami)", "substitution"},
		{"echo $HOME", "substitution"},
		{"echo `id`", "substitution"},
		{`echo "$HOME"`, "substitution"},
		{"(echo hi)", "subshell"},
		{"echo hi; rm x", "chaining"},
		{"echo hi && rm x", "chaining"},
		{"echo hi &", "chaining"},
		{"rm *.go", "globbing"},
		{"ls ?", "globbing"},
		{"ls [ab]", "globbing"},
		{`echo a\ b`, "escape"},
		{`echo "a\b"`, "escape"},
		{"echo hi 2> err.log", "descriptor"},
		{"echo hi >&2", "descriptor"},
		{"cat << EOF", "heredoc"},
		{"echo hi >| out", "clobber"},
		{"echo 'unterminated", "unterminated"},
		{"ls ~root", "home"},
		{"", "empty"},
		{"   ", "empty"},
		{"echo hi |", "empty"},
		{"| echo hi", "empty"},
		{"echo hi >", "redirect needs a file"},
		{"cat < a < b", "input redirect"},
		{"echo hi > a > b", "output redirect"},
		{"cat < a | grep x < b", "input redirect"},
		{"echo hi > a | cat", "output redirect"},
	}
	for _, c := range cases {
		if _, err := parsePipeline(c.cmd); err == nil {
			t.Errorf("%q should be refused", c.cmd)
		} else if !strings.Contains(err.Error(), c.reason) {
			t.Errorf("%q refused for the wrong reason: %v", c.cmd, err)
		}
	}
}

// Claims are the redirect files plus every non-flag argument, cleaned
// and deduplicated: the conservative resource set exclusion holds.
func TestPipelineClaims(t *testing.T) {
	p, err := parsePipeline("sort -u < in.txt | tee ./out.txt > final.log")
	if err != nil {
		t.Fatal(err)
	}
	got := p.claims()
	want := []string{"final.log", "in.txt", "out.txt"}
	if !slices.Equal(got, want) {
		t.Fatalf("claims wrong: got %v want %v", got, want)
	}

	p, err = parsePipeline("sleep 1")
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(p.claims(), []string{"1"}) {
		t.Fatalf("argument claims wrong: %v", p.claims())
	}
}
