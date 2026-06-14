package daemon

import (
	"context"
	"strconv"

	"golem/internal/code"
	"golem/internal/runtime"
)

// Teaching by example is the third way a computation is taught, beside the words
// "the sum of 6 and 7" and the code "6 + 7": shown inputs and the result they
// should make, GOLEM finds the operation that means it by running each candidate
// and keeping the one whose grounded effect matches. The meaning it lands on is
// the same construction the other two surfaces reach, so all three converge on
// one grounded computation. Running the candidates needs the runtime open, the
// same as any execution.

// example is one input-output a computation is taught by: two operands and the
// result running the operation over them should yield.
type example struct {
	a, b, result string
}

// induceByExample finds the operation whose execution on every example yields
// the example's result, the construction the examples mean. It returns the
// computation for the first example, so the caller sees which operation was
// induced and that it converges with the other surfaces. It fails when the
// runtime is closed, an input is not a number, or no single operation fits all
// the examples.
func (d *Daemon) induceByExample(exs []example) (code.Computation, bool) {
	if !d.cfg.RuntimeEnabled || d.code == nil || len(exs) == 0 {
		return code.Computation{}, false
	}
	for _, ex := range exs {
		if !isNumber(ex.a) || !isNumber(ex.b) {
			return code.Computation{}, false
		}
	}
	for _, op := range d.code.Ops() {
		if d.opFitsExamples(op, exs) {
			return code.Computation{Op: op.Name,
				Args: []code.Computation{{Lit: exs[0].a}, {Lit: exs[0].b}}}, true
		}
	}
	return code.Computation{}, false
}

// opFitsExamples reports whether running the operation over each example's
// inputs yields its result, the grounding that decides which construction the
// examples teach.
func (d *Daemon) opFitsExamples(op code.Op, exs []example) bool {
	for _, ex := range exs {
		comp := code.Computation{Op: op.Name, Args: []code.Computation{{Lit: ex.a}, {Lit: ex.b}}}
		src, ok := d.code.ToCode(comp)
		if !ok {
			return false
		}
		val, ok := d.evalCode(src)
		if !ok || val != ex.result {
			return false
		}
	}
	return true
}

// evalCode runs a small expression for its value alone, the quiet evaluation
// induction leans on: it grounds a candidate by running it without writing the
// artifact or the percept a deliberate run leaves, so trying many candidates
// does not flood memory.
func (d *Daemon) evalCode(src string) (string, bool) {
	runner := d.runner
	if runner == nil {
		runner = runtime.Interp{}
	}
	ctx, cancel := context.WithTimeout(context.Background(), runtimeBudget)
	defer cancel()
	res := runner.Run(ctx, src)
	if !res.OK {
		return "", false
	}
	return runPercept(res), true
}

// isNumber reports whether a teaching input is a plain number, the only operand
// the arithmetic bootstrap grounds, so a non-numeric input is refused before it
// could reach the runner.
func isNumber(s string) bool {
	_, err := strconv.ParseFloat(s, 64)
	return err == nil
}
