package code

import (
	"strconv"
	"strings"
)

// Teaching by example is the fourth way a function reaches the engine, beside
// the told definition "the double of a number is the product of the number and
// 2", the called surface, and the written Go. Shown input and output pairs, the
// engine searches for the rule that maps each input to its output, the function
// the examples mean. It composes that rule from the seeded operations and grounds
// each candidate by running it, so the rule it lands on is decided by what the
// code does, the same standard the other surfaces meet.

// ComprehendExamples reads a function taught by example, "the double of 3 is 6
// and the double of 5 is 10", into the name and the input/output pairs it shows.
// Every clause must name the same function and give numbers, and there must be
// at least two, since one pair is too few to fix a rule.
func (e *Engine) ComprehendExamples(text string) (name string, ins, outs []string, ok bool) {
	clauses := strings.Split(text, " and ")
	if len(clauses) < 2 {
		return "", nil, nil, false
	}
	for _, cl := range clauses {
		toks := strings.Fields(cl)
		if len(toks) != 6 || toks[0] != "the" || toks[2] != "of" || toks[4] != "is" {
			return "", nil, nil, false
		}
		nm, in, out := toks[1], toks[3], toks[5]
		if !isNumber(in) || !isNumber(out) {
			return "", nil, nil, false
		}
		if name == "" {
			name = nm
		} else if nm != name {
			return "", nil, nil, false
		}
		ins = append(ins, in)
		outs = append(outs, out)
	}
	return name, ins, outs, true
}

// minMultiStepExamples is how many pairs a rule of more than one operation needs
// before it is searched. A two-step rule has more freedom than a one-step one, so
// two points could be a coincidence; a third constrains it enough to ground a
// composed rule honestly.
const minMultiStepExamples = 3

// minRecursiveExamples is how many pairs a recursive rule needs before it is
// searched. Recursion is the most expressive shape, so it asks for the most
// evidence, lest a short sequence be read as a recursion it does not mean.
const minRecursiveExamples = 4

// InduceRecursive searches for a simple recursive rule that carries each input to
// its output, the synthesis beyond a fixed number of steps: the function is a
// base value for the smallest inputs, otherwise the input combined with the
// function of its predecessor, "factorial(n) = 1 if n < 1 else n * factorial(n -
// 1)". The base, its threshold, and the combining operation are searched over a
// small space, and each candidate is grounded by running it over every pair, the
// recursion run as the self-referential function the call generates. It declines
// unless there are pairs enough to pin a recursion, and when no candidate fits.
func (e *Engine) InduceRecursive(name, param string, ins, outs []string, run func(string) (string, bool)) (Op, bool) {
	if len(ins) < minRecursiveExamples || len(ins) != len(outs) {
		return Op{}, false
	}
	if _, ok := e.byName["subtract"]; !ok {
		return Op{}, false // no way to step toward the base
	}
	for _, threshold := range []string{"1", "0", "2"} {
		for _, base := range []string{"1", "0"} {
			for _, comb := range []string{"multiply", "add"} {
				if _, ok := e.byName[comb]; !ok {
					continue
				}
				step := Computation{Op: name, Args: []Computation{
					{Op: "subtract", Args: []Computation{{Var: param}, {Lit: "1"}}}}}
				body := Computation{Cond: &Conditional{
					Cmp: "<", Left: Computation{Var: param}, Right: Computation{Lit: threshold},
					Then: Computation{Lit: base},
					Else: Computation{Op: comb, Args: []Computation{{Var: param}, step}}}}
				hyp := Op{Name: name, Word: name, Params: []string{param}, Body: &body}
				if e.recursiveFits(hyp, ins, outs, run) {
					return hyp, true
				}
			}
		}
	}
	return Op{}, false
}

// recursiveFits reports whether a recursive candidate maps every input to its
// output when run, grounding it the way the call would, as a self-referential
// function invoked on the input.
func (e *Engine) recursiveFits(hyp Op, ins, outs []string, run func(string) (string, bool)) bool {
	for i := range ins {
		src, ok := e.genRecursiveCall(hyp, []Computation{{Lit: ins[i]}}, "")
		if !ok {
			return false
		}
		if val, ok := run(src); !ok || val != outs[i] {
			return false
		}
	}
	return true
}

// InduceBody searches the primitive operations for a unary body that carries each
// input to its output, the rule the examples mean. It tries the one-step forms
// first (an operation over the parameter and a constant, or the parameter against
// itself), and only if none fit, and there are examples enough to constrain it,
// the two-step compositions (a rule like cube, n*n*n). Each candidate is grounded
// by running it over every pair, the first that fits them all kept, so the rule
// is decided by what the code does and not by arithmetic done here. The simplest
// fitting rule wins, since the one-step forms are tried before the composed ones.
func (e *Engine) InduceBody(param string, ins, outs []string, run func(string) (string, bool)) (Computation, bool) {
	if len(ins) < 2 || len(ins) != len(outs) {
		return Computation{}, false
	}
	for _, body := range e.oneStepBodies(param, ins[0], outs[0]) {
		if e.bodyFits(body, param, ins, outs, run) {
			return body, true
		}
	}
	if len(ins) >= minMultiStepExamples {
		for _, body := range e.multiStepBodies(param, ins[0], outs[0]) {
			if e.bodyFits(body, param, ins, outs, run) {
				return body, true
			}
		}
	}
	return Computation{}, false
}

// oneStepBodies builds the single-operation hypotheses, the constant forms before
// the parameter-against-itself form so "double" reads as times two rather than
// the parameter added to itself.
func (e *Engine) oneStepBodies(param, in0, out0 string) []Computation {
	prims := e.primitives()
	var bodies []Computation
	for _, c := range deriveConstants(in0, out0) {
		for _, op := range prims {
			bodies = append(bodies, Computation{Op: op.Name, Args: []Computation{{Var: param}, {Lit: c}}})
		}
	}
	for _, op := range prims {
		bodies = append(bodies, Computation{Op: op.Name, Args: []Computation{{Var: param}, {Var: param}}})
	}
	return bodies
}

// multiStepBodies composes one further operation over each one-step body and a
// leaf (the parameter or a derived constant), in both operand orders, so a rule
// of two operations is reached: cube is the parameter squared, times the
// parameter again. The bodies are deduped by the source they generate. Deeper
// compositions (three operations and up) are left for a later widening, since the
// search grows quickly with depth.
func (e *Engine) multiStepBodies(param, in0, out0 string) []Computation {
	prims := e.primitives()
	leaves := []Computation{{Var: param}}
	for _, c := range deriveConstants(in0, out0) {
		leaves = append(leaves, Computation{Lit: c})
	}
	var bodies []Computation
	seen := map[string]bool{}
	add := func(b Computation) {
		key, ok := e.ToCode(b)
		if !ok || seen[key] {
			return
		}
		seen[key] = true
		bodies = append(bodies, b)
	}
	for _, s := range e.oneStepBodies(param, in0, out0) {
		for _, op := range prims {
			for _, leaf := range leaves {
				add(Computation{Op: op.Name, Args: []Computation{s, leaf}})
				add(Computation{Op: op.Name, Args: []Computation{leaf, s}})
			}
		}
	}
	return bodies
}

// bodyFits reports whether a candidate maps every input to its output when run,
// the grounding that decides which rule the examples teach.
func (e *Engine) bodyFits(body Computation, param string, ins, outs []string, run func(string) (string, bool)) bool {
	for i := range ins {
		applied := substitute(body, map[string]Computation{param: {Lit: ins[i]}})
		src, ok := e.ToCode(applied)
		if !ok {
			return false
		}
		val, ok := run(src)
		if !ok || val != outs[i] {
			return false
		}
	}
	return true
}

// primitives lists the seeded operations, sorted, the building blocks induction
// composes from. Learned functions are left out so the search stays over the
// arithmetic floor and cannot lean on a name taught moments earlier.
func (e *Engine) primitives() []Op {
	var prims []Op
	for _, o := range e.Ops() {
		if o.Body == nil {
			prims = append(prims, o)
		}
	}
	return prims
}

// Primitive reports whether a word names a seeded operation, so induction and
// definition both refuse to shadow one.
func (e *Engine) Primitive(word string) bool {
	if op, ok := e.byName[word]; ok && op.Body == nil {
		return true
	}
	if op, ok := e.byWord[word]; ok && op.Body == nil {
		return true
	}
	return false
}

// deriveConstants reads the constants worth trying from a single pair: the
// difference each way, the sum, and the clean quotients, the values a one-step
// rule over this pair could turn on. Division guards against a zero input.
func deriveConstants(in0, out0 string) []string {
	a, err1 := strconv.Atoi(in0)
	b, err2 := strconv.Atoi(out0)
	if err1 != nil || err2 != nil {
		return nil
	}
	cands := []int{b - a, a - b, a + b}
	if a != 0 && b%a == 0 {
		cands = append(cands, b/a)
	}
	if b != 0 && a%b == 0 {
		cands = append(cands, a/b)
	}
	seen := map[int]bool{}
	var out []string
	for _, c := range cands {
		if seen[c] {
			continue
		}
		seen[c] = true
		out = append(out, strconv.Itoa(c))
	}
	return out
}

// isNumber reports whether a token is a plain integer, the only operand the
// arithmetic floor grounds.
func isNumber(s string) bool {
	_, err := strconv.Atoi(s)
	return err == nil
}
