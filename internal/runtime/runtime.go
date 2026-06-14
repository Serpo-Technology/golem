// Package runtime is GOLEM's hand on the machine's own gift: it writes a
// program and runs it, so a computation it would labor at symbolically,
// arithmetic, search, a transformation, is offloaded to executed code the way
// a person reaches for a tool. Executed code is more auditable than a weight,
// because it can be read; the program GOLEM ran is an artifact, as legible as
// what it thought.
//
// The runtime is one capability behind a small interface, so the engine that
// carries it out can change without the caller changing: an embedded
// interpreter runs the snippets a tool needs today, and a toolchain that
// compiles a binary can stand behind the same interface when GOLEM comes to
// rebuild itself. The bounding of a single program (a time budget, a recovered
// panic) is what the runtime provides; the sealed container around the whole
// process is the operator's, placed from outside.
package runtime

import (
	"bytes"
	"context"
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/traefik/yaegi/interp"
	"github.com/traefik/yaegi/stdlib"
)

// Result is what running a program leaves behind, a percept GOLEM can ground:
// the value the program evaluated to, anything it printed, how long it took,
// and an error when it failed or overran. It is written to the graph and the
// trail as the artifact of the run.
type Result struct {
	Value   string        // the program's final value, rendered
	Output  string        // what the program printed
	Err     string        // a compile, run, panic, or timeout error, empty on success
	Elapsed time.Duration // how long the run took
	OK      bool          // the program ran to completion without error
}

// Runner runs a program GOLEM wrote and returns the result. It is the seam the
// runtime turns on: the interpreter here, a compiling toolchain later, the same
// caller either way.
type Runner interface {
	Run(ctx context.Context, source string) Result
}

// Interp runs a program through an embedded Go interpreter over a curated,
// computation-only slice of the standard library, so a tool can compute but
// not reach the filesystem, the network, or the process. It is the default
// runtime: pure computation, the use the runtime was opened for.
type Interp struct{}

// Run evaluates the source in a fresh interpreter, bounded by the context's
// deadline and a recovered panic. The interpreter runs in its own goroutine so
// a program that overruns is contained to a timeout error the caller learns
// from without the loop being blocked; a true runaway leaks that goroutine
// until the process ends, which the operator's container is the backstop for.
func (Interp) Run(ctx context.Context, source string) Result {
	start := time.Now()
	done := make(chan Result, 1)
	go func() {
		var out bytes.Buffer
		i := interp.New(interp.Options{Stdout: &out, Stderr: &out})
		_ = i.Use(safeSymbols())
		defer func() {
			if p := recover(); p != nil {
				done <- Result{Output: out.String(), Err: fmt.Sprintf("panic: %v", p)}
			}
		}()
		v, err := i.Eval(source)
		res := Result{Output: out.String()}
		if err != nil {
			res.Err = err.Error()
		} else {
			res.Value = render(v)
			res.OK = true
		}
		done <- res
	}()
	select {
	case <-ctx.Done():
		return Result{Err: "timed out", Elapsed: time.Since(start)}
	case res := <-done:
		res.Elapsed = time.Since(start)
		return res
	}
}

// render is a program value's face for grounding: its printed form, or empty
// when the program evaluated to nothing or to something not worth grounding. A
// full program evaluates to its main function, which is not a result, so a
// function value renders to nothing and the program's printed output carries
// the meaning instead.
func render(v reflect.Value) string {
	if !v.IsValid() || !v.CanInterface() || v.Kind() == reflect.Func {
		return ""
	}
	return fmt.Sprintf("%v", v.Interface())
}

// safeImportPrefixes is the allowlist of standard-library packages the
// computation runtime exposes: numbers, text, collections, encodings. It is
// default-deny, so os, os/exec, net, syscall, and the rest of the machine stay
// out of reach of a program GOLEM wrote, the inner guard under the operator's
// container.
var safeImportPrefixes = []string{
	"fmt/", "errors/", "math/", "strings/", "strconv/", "sort/", "slices/",
	"maps/", "cmp/", "unicode/", "bytes/", "bufio/", "regexp/", "time/",
	"container/", "encoding/", "hash/", "crypto/", "index/", "text/", "math/rand/",
}

// safeSymbols filters the interpreter's standard library down to the
// computation allowlist, so the runtime is a calculator and a transformer, not
// a door to the machine.
func safeSymbols() interp.Exports {
	out := interp.Exports{}
	for path, syms := range stdlib.Symbols {
		for _, ok := range safeImportPrefixes {
			if strings.HasPrefix(path, ok) {
				out[path] = syms
				break
			}
		}
	}
	return out
}
