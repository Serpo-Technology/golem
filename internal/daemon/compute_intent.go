package daemon

import (
	"strings"

	"golem/internal/code"
)

// pendingComputation is a computation GOLEM was asked for but does not know. Rather
// than admit the gap and stop, it holds the operation and the call it was asked, and
// invites the speaker to teach it, by a definition or one example at a time. When
// enough examples accumulate, the synthesis floor assembles the rule by running, and
// the call that began the exchange is answered on the turn it is learned.
type pendingComputation struct {
	op       string   // the unknown operation, "spread"
	surface  string   // the call asked, "the spread of the list of 2 and 5 and 9", re-run to answer once learned
	examples []string // the example surfaces gathered so far, fed to the search together
}

// maxComputeExamples caps how many examples a learning loop gathers before giving up,
// so a rule the synthesis cannot reach does not invite examples without end.
const maxComputeExamples = 6

// requestComputation turns an unknown computation into a request to be taught it, for a
// plain call "the <op> of <arg>" whose argument is a number or a list of numbers, the
// surface the example loop gathers a rule for. It records the call so the answer can
// follow the learning, and returns the invitation with its hint shaped to the argument.
// A richer surface (a nested call, several operands) is left for the caller to admit as
// a gap, since the one-at-a-time loop does not yet assemble it.
func (d *Daemon) requestComputation(src, op string) (string, bool) {
	arg, ok := d.code.LearnableCallArg(src, op)
	if !ok {
		return "", false
	}
	d.pendingCompute = &pendingComputation{op: op, surface: src}
	hint := "the " + op + " of 2 is 6"
	if strings.HasPrefix(arg, "the list of") {
		hint = "the " + op + " of the list of 2 and 5 and 9 is 7"
	}
	return "I do not know the " + op + " yet. Tell me what it is, or show me, like: " + hint + ".", true
}

// comprehendInverse learns a function stated as the inverse of a known operation,
// "the halving is the inverse of the double": the grammar reads the goal, the
// synthesis floor discovers the body whose output the operation returns to the
// input, grounded by running, and the discovered function is kept. It returns false
// when the turn is no inverse goal, so the other paths read it; when the goal is
// understood but no inverse is found, it admits the gap rather than learning a wrong
// rule.
func (d *Daemon) comprehendInverse(content string) (string, bool) {
	if !d.cfg.RuntimeEnabled || d.code == nil {
		return "", false
	}
	name, op, ok := d.gram.ParseInverse("en", content, d.opHead)
	if !ok {
		return "", false
	}
	// The inverse being defined is left out of its own search, so it is not found as
	// the trivial call to itself.
	d.code.SetSynthSelf(name)
	defer d.code.SetSynthSelf("")
	body, ok := d.code.SynthesizeInverse(inducedParam, op, d.evalCode)
	if !ok {
		return "I could not work out the " + name + ".", true
	}
	line, err := d.learnFunction(code.Op{Name: name, Word: name, Params: []string{inducedParam}, Body: &body})
	if err != nil {
		return "I could not keep the " + name + ".", true
	}
	return line, true
}

// comprehendExtremal learns a list function stated by an extremal property, "the
// peak is the greatest": the grammar reads the goal, the synthesis floor discovers
// the running-fold whose output is the list's maximum (or minimum) verified by
// running, and the discovered function is kept. It returns false when the turn is
// no extremal goal, so the other paths read it; when the goal is understood but no
// fold is found, it admits the gap rather than learning a wrong rule.
func (d *Daemon) comprehendExtremal(content string) (string, bool) {
	if !d.cfg.RuntimeEnabled || d.code == nil {
		return "", false
	}
	name, sym, ok := d.gram.ParseExtremal("en", content, d.opHead, d.extremalRelator)
	if !ok {
		return "", false
	}
	// The function being defined is left out of its own search, so it is not found as
	// the trivial call to itself.
	d.code.SetSynthSelf(name)
	defer d.code.SetSynthSelf("")
	body, ok := d.code.SynthesizeExtremal(listParam, sym, d.evalCode)
	if !ok {
		return "I could not work out the " + name + ".", true
	}
	op := code.Op{Name: name, Word: name, Params: []string{listParam}, ParamTypes: []string{"list"}, Ret: "int", Raw: body}
	line, err := d.learnFunction(op)
	if err != nil {
		return "I could not keep the " + name + ".", true
	}
	return line, true
}

// advancePendingCompute takes a turn while a computation is awaited and reads it as an
// example of that operation. Each example accumulates; once two stand, they are fed to
// the synthesis floor together, and as soon as it assembles a rule the operation is
// learned and the awaited call answered. A turn that is no example of the awaited
// operation is not consumed here, so an ordinary statement or a definition falls
// through to its own path.
func (d *Daemon) advancePendingCompute(content string) (string, bool) {
	if d.pendingCompute == nil {
		return "", false
	}
	name, ok := d.code.ComprehendComputeExamplePair(content)
	if !ok || name != d.pendingCompute.op {
		return "", false
	}
	d.pendingCompute.examples = append(d.pendingCompute.examples, content)
	ex := d.pendingCompute.examples
	if len(ex) >= 2 {
		if op, found := d.synthesizeFromExamples(name, strings.Join(ex, " and ")); found {
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

// synthesizeFromExamples assembles the operation the gathered examples show, dispatched
// by their shape: a number to a number is searched as an expression, a list to a number
// as a fold, a list to a list as a transform, each grounded by running. The examples are
// joined as the by-example tellings are written, so the same parsers and searches those
// surfaces use serve the gathered ones here.
func (d *Daemon) synthesizeFromExamples(name, joined string) (code.Op, bool) {
	if nm, ins, outs, ok := d.code.ComprehendExamples(joined); ok && nm == name {
		return d.induceFunction(nm, ins, outs)
	}
	if nm, lists, outs, ok := d.code.ComprehendListExamples(joined); ok && nm == name {
		return d.synthesizeFold(nm, lists, outs)
	}
	if nm, ins, outs, ok := d.code.ComprehendListReturnExamples(joined); ok && nm == name {
		return d.synthesizeListReturn(nm, ins, outs)
	}
	return code.Op{}, false
}

// closeComputeLoop answers the call that began a learning loop, now that the operation
// it asked about is known: it runs the original call and speaks the result, so the turn
// that learns "spread" also reports the spread of the list it was asked. It clears the
// pending computation, whether or not the answer comes out, so the loop closes once.
func (d *Daemon) closeComputeLoop(op string) string {
	if d.pendingCompute == nil || d.pendingCompute.op != op {
		return ""
	}
	surface := d.pendingCompute.surface
	d.pendingCompute = nil
	if surface == "" {
		return ""
	}
	val, ok := d.computeSay(surface)
	if !ok || val == "" {
		return ""
	}
	return " So " + surface + " is " + val + "."
}

// computeSay runs a comprehended computation and returns its result, the same path the
// runcode command takes, so a call learned this turn is answered exactly as it would be
// if asked fresh. It returns false when the surface does not comprehend or does not
// type-check, so a still-unknown or ill-formed call closes the loop silently.
func (d *Daemon) computeSay(src string) (string, bool) {
	if d.code == nil {
		return "", false
	}
	comp, ok := d.comprehendCall("en", src)
	if !ok || !d.code.TypeCheck(comp) {
		return "", false
	}
	out := src
	if generated, ok := d.generateCode(comp); ok {
		out = generated
	}
	return runSummary(d.runProgram(out)), true
}
