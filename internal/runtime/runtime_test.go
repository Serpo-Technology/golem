package runtime

import (
	"context"
	"strings"
	"testing"
	"time"
)

func run(t *testing.T, source string, budget time.Duration) Result {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), budget)
	defer cancel()
	return Interp{}.Run(ctx, source)
}

// The runtime offloads a computation: GOLEM writes an expression and gets its
// value back to ground.
func TestRunComputes(t *testing.T) {
	res := run(t, "6 * 7", time.Second)
	if !res.OK || res.Value != "42" {
		t.Fatalf("the runtime should evaluate the program: %+v", res)
	}
}

// A full program runs and its printed output comes back, so a transformation
// can report what it found.
func TestRunCapturesOutput(t *testing.T) {
	res := run(t, "package main\nimport \"fmt\"\nfunc main() { fmt.Print(\"hello\") }", time.Second)
	if !res.OK || !strings.Contains(res.Output, "hello") {
		t.Fatalf("the runtime should run the program and capture its output: %+v", res)
	}
}

// A panicking program is contained to an error GOLEM learns from, not a crash.
func TestRunContainsPanic(t *testing.T) {
	res := run(t, `panic("boom")`, time.Second)
	if res.OK || res.Err == "" {
		t.Fatalf("a panic should be contained to an error, not crash: %+v", res)
	}
}

// A runaway is contained to a timeout the caller learns from, without the call
// blocking past its budget.
func TestRunTimesOut(t *testing.T) {
	res := run(t, "for {}", 50*time.Millisecond)
	if res.OK || res.Err != "timed out" {
		t.Fatalf("a runaway program should time out: %+v", res)
	}
}

// The runtime is a calculator, not a door to the machine: a program reaching
// for the operating system fails to compile against the curated library.
func TestRunDeniesTheMachine(t *testing.T) {
	res := run(t, `import "os"; os.Getpid()`, time.Second)
	if res.OK || res.Err == "" {
		t.Fatalf("a program reaching for os should be refused: %+v", res)
	}
}
