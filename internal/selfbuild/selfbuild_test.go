package selfbuild

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// tempModule writes a tiny self-contained Go module: a greeting the program
// prints and a test that only asks the greeting be non-empty, so a change to
// the greeting keeps the tests passing while a change that empties it fails
// them.
func tempModule(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	write := func(name, body string) {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("go.mod", "module probe\n\ngo 1.21\n")
	write("main.go", "package main\n\nimport \"fmt\"\n\nfunc greet() string { return \"hello\" }\n\nfunc main() { fmt.Println(greet()) }\n")
	write("main_test.go", "package main\n\nimport \"testing\"\n\nfunc TestGreet(t *testing.T) {\n\tif greet() == \"\" {\n\t\tt.Fatal(\"greeting is empty\")\n\t}\n}\n")
	return dir
}

func ctx(t *testing.T) context.Context {
	c, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	t.Cleanup(cancel)
	return c
}

// A change that compiles and keeps the tests passing is accepted, a candidate
// binary staged off to the side, and the original tree left untouched.
func TestSelfBuildAcceptsGoodChange(t *testing.T) {
	src := tempModule(t)
	stage := t.TempDir()
	ch := Change{File: "main.go", Why: "warmer greeting",
		Apply: func(s string) (string, error) { return strings.Replace(s, `"hello"`, `"howdy"`, 1), nil }}

	res := Build(ctx(t), src, stage, ".", "candidate", ch)
	if !res.Accepted {
		t.Fatalf("a clean change should build and test: %+v", res)
	}
	if _, err := os.Stat(res.Candidate); err != nil {
		t.Fatalf("the candidate binary should be staged: %v", err)
	}
	orig, _ := os.ReadFile(filepath.Join(src, "main.go"))
	if !strings.Contains(string(orig), `"hello"`) {
		t.Fatal("the original source must be untouched by the pipeline")
	}
}

// A change that breaks the compile is rejected at the build stage, no candidate
// produced.
func TestSelfBuildRejectsBrokenCompile(t *testing.T) {
	src := tempModule(t)
	stage := t.TempDir()
	ch := Change{File: "main.go", Why: "broken",
		Apply: func(s string) (string, error) { return s + "\nthis is not valid go\n", nil }}

	res := Build(ctx(t), src, stage, ".", "candidate", ch)
	if res.Accepted || res.Stage != "build" {
		t.Fatalf("a broken compile should be rejected at build: %+v", res)
	}
	if _, err := os.Stat(filepath.Join(stage, "candidate")); err == nil {
		t.Fatal("no candidate should be staged for a failed build")
	}
}

// A change that compiles but fails the tests is rejected at the test stage.
func TestSelfBuildRejectsFailingTest(t *testing.T) {
	src := tempModule(t)
	stage := t.TempDir()
	ch := Change{File: "main.go", Why: "empties the greeting",
		Apply: func(s string) (string, error) { return strings.Replace(s, `"hello"`, `""`, 1), nil }}

	res := Build(ctx(t), src, stage, ".", "candidate", ch)
	if res.Accepted || res.Stage != "test" {
		t.Fatalf("a failing test should be rejected at test: %+v", res)
	}
}
