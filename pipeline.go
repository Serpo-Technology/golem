package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// A command never reaches a shell. It is parsed here into a pipeline of
// argv stages with explicit pipe and redirect structure, and anything
// the safe subset cannot model is refused with a reason. That rule,
// parse or refuse, is what lets the allowlist authorize each stage's
// program on its own: there is no hidden interpreter left for a prefix
// match to smuggle a compound program past.
//
// The subset on purpose: words with single and double quoting (always
// literal, never expanding), pipes, one input redirect on the first
// stage, one output redirect (truncate or append) on the last, and a
// leading ~/ expanded to the operator's home. Everything a shell would
// interpret beyond that (substitution, subshells, chaining, globbing,
// escapes, descriptor games) is refused, never approximated.

// pipeline is the executable form of a command: argv stages wired
// stdout to stdin left to right, with at most one file feeding the
// first stage and one file capturing the last.
type pipeline struct {
	stages    [][]string
	inFile    string
	outFile   string
	appendOut bool
}

// stageString is the form of one stage that policy matches against:
// its argv joined by single spaces, so quoting tricks cannot hide a
// denylisted shape from the substring check.
func stageString(argv []string) string {
	return strings.Join(argv, " ")
}

// pipeToken is one lexed element: a word (with a note of whether any
// part of it was quoted, which tilde expansion needs) or an operator.
type pipeToken struct {
	text   string
	op     bool // "|", "<", ">", ">>"
	quoted bool
}

// parsePipeline turns a command string into its pipeline, or refuses
// with the reason the audit trail records.
func parsePipeline(cmd string) (*pipeline, error) {
	tokens, err := lexPipeline(cmd)
	if err != nil {
		return nil, err
	}
	if len(tokens) == 0 {
		return nil, errors.New("empty command")
	}

	type stageAST struct {
		argv          []string
		in, out       string
		appendOut     bool
		hasIn, hasOut bool
	}
	stages := []*stageAST{{}}
	cur := stages[0]
	for i := 0; i < len(tokens); i++ {
		tok := tokens[i]
		if !tok.op {
			word, err := expandTilde(tok)
			if err != nil {
				return nil, err
			}
			cur.argv = append(cur.argv, word)
			continue
		}
		switch tok.text {
		case "|":
			cur = &stageAST{}
			stages = append(stages, cur)
		case "<", ">", ">>":
			if i+1 >= len(tokens) || tokens[i+1].op {
				return nil, errors.New("redirect needs a file")
			}
			i++
			target, err := expandTilde(tokens[i])
			if err != nil {
				return nil, err
			}
			if tok.text == "<" {
				if cur.hasIn {
					return nil, errors.New("only one input redirect is allowed, on the first stage")
				}
				cur.in, cur.hasIn = target, true
			} else {
				if cur.hasOut {
					return nil, errors.New("only one output redirect is allowed, on the last stage")
				}
				cur.out, cur.hasOut = target, true
				cur.appendOut = tok.text == ">>"
			}
		}
	}

	p := &pipeline{}
	for i, st := range stages {
		if len(st.argv) == 0 {
			return nil, errors.New("empty pipeline stage")
		}
		if st.hasIn {
			if i != 0 {
				return nil, errors.New("only one input redirect is allowed, on the first stage")
			}
			p.inFile = st.in
		}
		if st.hasOut {
			if i != len(stages)-1 {
				return nil, errors.New("only one output redirect is allowed, on the last stage")
			}
			p.outFile = st.out
			p.appendOut = st.appendOut
		}
		p.stages = append(p.stages, st.argv)
	}
	return p, nil
}

// lexPipeline splits a command into words and operators. Quoted spans
// are literal: with no shell downstream there is nothing for $ or a
// backtick to mean, so inside single quotes everything is text, and
// double quotes differ only in still refusing the substitution and
// escape characters a reader would expect to be live.
func lexPipeline(cmd string) ([]pipeToken, error) {
	var tokens []pipeToken
	var word strings.Builder
	wordStarted := false
	quoted := false

	flush := func() {
		if wordStarted {
			tokens = append(tokens, pipeToken{text: word.String(), quoted: quoted})
			word.Reset()
			wordStarted = false
			quoted = false
		}
	}
	pushOp := func(op string) {
		tokens = append(tokens, pipeToken{text: op, op: true})
	}

	runes := []rune(cmd)
	for i := 0; i < len(runes); i++ {
		r := runes[i]
		switch r {
		case '\'', '"':
			close := r
			wordStarted, quoted = true, true
			j := i + 1
			for ; j < len(runes) && runes[j] != close; j++ {
				if close == '"' {
					switch runes[j] {
					case '$', '`':
						return nil, errors.New("variable and command substitution are not modeled")
					case '\\':
						return nil, errors.New("escape sequences are not modeled")
					}
				}
				word.WriteRune(runes[j])
			}
			if j >= len(runes) {
				return nil, errors.New("unterminated quote")
			}
			i = j
		case ' ', '\t':
			flush()
		case '|':
			flush()
			pushOp("|")
		case '<', '>':
			// A bare digit word touching the operator is how "2>" and
			// "2>&1" begin; descriptor games stay out of the subset.
			if wordStarted && !quoted && isAllDigits(word.String()) {
				return nil, errors.New("file-descriptor redirects are not modeled")
			}
			flush()
			op := string(r)
			if r == '>' && i+1 < len(runes) && runes[i+1] == '>' {
				op, i = ">>", i+1
			} else if r == '<' && i+1 < len(runes) && runes[i+1] == '<' {
				return nil, errors.New("heredocs are not modeled")
			}
			if r == '>' && i+1 < len(runes) && runes[i+1] == '|' {
				return nil, errors.New("clobber control is not modeled")
			}
			if i+1 < len(runes) && runes[i+1] == '&' {
				return nil, errors.New("file-descriptor redirects are not modeled")
			}
			pushOp(op)
		case '$', '`':
			return nil, errors.New("variable and command substitution are not modeled")
		case '(', ')':
			return nil, errors.New("subshells are not modeled")
		case ';', '&', '\n':
			return nil, errors.New("chaining and backgrounding are not modeled")
		case '*', '?', '[':
			return nil, errors.New("globbing is not modeled")
		case '\\':
			return nil, errors.New("escape sequences are not modeled")
		default:
			wordStarted = true
			word.WriteRune(r)
		}
	}
	flush()
	return tokens, nil
}

func isAllDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// expandTilde resolves a leading unquoted ~ to the operator's home
// directory, the one expansion the subset models because grounded
// commands already rely on it. Another user's home is refused.
func expandTilde(tok pipeToken) (string, error) {
	w := tok.text
	if tok.quoted || !strings.HasPrefix(w, "~") {
		return w, nil
	}
	rest := w[1:]
	if rest != "" && !strings.HasPrefix(rest, "/") {
		return "", errors.New("home directories of other users are not modeled")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("home directory unknown: %v", err)
	}
	return home + rest, nil
}

// claims lists the resources a pipeline touches, for effector
// exclusion: the redirect files plus every argument that is not a
// flag. Deliberately conservative, the way one hand refuses to grab
// two things: a false conflict only serializes two actions, while a
// missed one would let them race on the same file.
func (p *pipeline) claims() []string {
	seen := map[string]bool{}
	add := func(s string) {
		if s == "" || strings.HasPrefix(s, "-") {
			return
		}
		seen[filepath.Clean(s)] = true
	}
	add(p.inFile)
	add(p.outFile)
	for _, argv := range p.stages {
		for _, arg := range argv[1:] {
			add(arg)
		}
	}
	out := make([]string, 0, len(seen))
	for s := range seen {
		out = append(out, s)
	}
	sort.Strings(out)
	return out
}
