package daemon

import (
	"strings"
)

// pendingComputation is a computation GOLEM was asked for but does not know. Rather
// than admit the gap and stop, it holds the operation and the argument it was asked
// about and invites the speaker to teach it, by a definition or one example at a
// time. When enough examples accumulate, the synthesis floor assembles the rule by
// running, and the original question is answered on the same turn it is learned.
type pendingComputation struct {
	op       string      // the unknown operation, "triple"
	arg      string      // the argument the question asked about, "4", to answer once learned
	examples [][2]string // input/output pairs gathered so far, toward a synthesis
}

// maxComputeExamples caps how many examples a learning loop gathers before giving
// up, so a rule the synthesis cannot reach does not invite examples without end.
const maxComputeExamples = 6

// requestComputation turns an unknown computation into a request to be taught it,
// for a plain unary call ("the triple of 4") whose argument is a number the synthesis
// floor works over. It records what was asked so the answer can follow the learning,
// and returns the invitation. A richer surface (a binary call, a nested one) is left
// to the caller to admit as a gap, since the one-example loop does not yet assemble it.
func (d *Daemon) requestComputation(src, op string) (string, bool) {
	toks := strings.Fields(src)
	if len(toks) != 4 || toks[0] != "the" || toks[1] != op || toks[2] != "of" || !isNumeric(toks[3]) {
		return "", false
	}
	d.pendingCompute = &pendingComputation{op: op, arg: toks[3]}
	return "I do not know the " + op + " yet. Tell me what it is, or show me, like: the " + op + " of 2 is 6.", true
}

// advancePendingCompute takes a turn while a computation is awaited and reads it as an
// example of that operation. Each example accumulates; once two stand, the synthesis
// floor is run, and as soon as it assembles a rule the operation is learned and the
// awaited question answered. A turn that is no example of the awaited operation is not
// consumed here, so an ordinary statement or a definition falls through to its own path.
func (d *Daemon) advancePendingCompute(content string) (string, bool) {
	if d.pendingCompute == nil {
		return "", false
	}
	name, in, out, ok := d.code.ComprehendExamplePair(content)
	if !ok || name != d.pendingCompute.op {
		return "", false
	}
	d.pendingCompute.examples = append(d.pendingCompute.examples, [2]string{in, out})
	ex := d.pendingCompute.examples
	if len(ex) >= 2 {
		ins := make([]string, len(ex))
		outs := make([]string, len(ex))
		for i, p := range ex {
			ins[i], outs[i] = p[0], p[1]
		}
		if op, found := d.induceFunction(name, ins, outs); found {
			line, err := d.learnFunction(op)
			if err != nil {
				d.pendingCompute = nil
				return "I could not keep the " + name + ".", true
			}
			return line + d.closeComputeLoop(op.Name), true
		}
	}
	if len(ex) >= maxComputeExamples {
		d.pendingCompute = nil
		return "I still cannot see the rule for the " + name + ".", true
	}
	return "Show me one more example of the " + name + ".", true
}

// closeComputeLoop answers the question that began a learning loop, now that the
// operation it asked about is known: it runs the original call and speaks the result,
// so the turn that learns "triple" also reports the triple of 4. It clears the pending
// computation, whether or not the answer comes out, so the loop closes once.
func (d *Daemon) closeComputeLoop(op string) string {
	if d.pendingCompute == nil || d.pendingCompute.op != op {
		return ""
	}
	arg := d.pendingCompute.arg
	d.pendingCompute = nil
	if arg == "" {
		return ""
	}
	val, ok := d.computeSay("the " + op + " of " + arg)
	if !ok || val == "" {
		return ""
	}
	return " So the " + op + " of " + arg + " is " + val + "."
}

// computeSay runs a comprehended computation and returns its result, the same path the
// runcode command takes, so a call learned this turn is answered exactly as it would
// be if asked fresh. It returns false when the surface does not comprehend or does not
// type-check, so a still-unknown or ill-formed call closes the loop silently.
func (d *Daemon) computeSay(src string) (string, bool) {
	if d.code == nil {
		return "", false
	}
	comp, ok := d.code.Comprehend(src)
	if !ok || !d.code.TypeCheck(comp) {
		return "", false
	}
	out := src
	walked, wok := d.traverseCode(comp)
	generated, gok := d.code.ToCode(comp)
	switch {
	case wok && (!gok || walked == generated):
		out = walked
	case gok:
		out = generated
	}
	return runSummary(d.runProgram(out)), true
}

// isNumeric reports whether a token is a plain non-negative integer, the argument a
// unary call over numbers carries.
func isNumeric(s string) bool {
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
