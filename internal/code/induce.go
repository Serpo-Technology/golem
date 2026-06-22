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

// ComprehendComputeExamplePair reports whether the text is a single well-formed
// example of a computation and returns the operation it names: "the triple of 2 is 6"
// (a number to a number), "the tally of the list of 1 and 2 and 3 is 6" (a list to a
// number), or "the doubled of the list of 1 and 2 is the list of 2 and 4" (a list to a
// list). The three shapes are the ones the synthesis floor assembles. It is how a
// dialogue recognizes a turn as an example of the operation it is waiting to learn, one
// at a time, before the gathered examples are fed to the search together.
func (e *Engine) ComprehendComputeExamplePair(text string) (name string, ok bool) {
	toks := strings.Fields(text)
	if len(toks) < 2 || toks[0] != "the" {
		return "", false
	}
	name = toks[1]
	rest, okc := strings.CutPrefix(text, "the "+name+" of ")
	if !okc {
		return "", false
	}
	isAt := strings.Index(rest, " is ")
	if isAt < 0 {
		return "", false
	}
	if !e.isOperand(strings.TrimSpace(rest[:isAt])) || !e.isOperand(strings.TrimSpace(rest[isAt+len(" is "):])) {
		return "", false
	}
	return name, true
}

// LearnableCallArg reports whether the text is a plain call "the <op> of <arg>" whose
// argument is a number or a list of numbers, the surface a dialogue can gather a rule
// for one example at a time, and returns the argument. A nested or many-operand call is
// not learnable by that loop and is left to admit the gap.
func (e *Engine) LearnableCallArg(text, op string) (arg string, ok bool) {
	rest, okc := strings.CutPrefix(text, "the "+op+" of ")
	if !okc {
		return "", false
	}
	rest = strings.TrimSpace(rest)
	if e.isOperand(rest) {
		return rest, true
	}
	return "", false
}

// isOperand reports whether the text is a single number or a list of numbers, the
// operands a learnable call and its examples carry.
func (e *Engine) isOperand(text string) bool {
	if isNumber(text) {
		return true
	}
	_, ok := e.listLiteralNums(text)
	return ok
}

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

// ComprehendListExamples reads a function shown by example over a LIST input, "the
// tally of the list of 1 and 2 and 3 is 6 and the tally of the list of 2 and 4 is
// 6", into the name, the example lists, and their outputs. The clauses are split on
// the repeated "and the <name> of", so a list's own "and"s do not split them; each
// gives a list literal and a number. At least two pairs are needed to fix a rule.
func (e *Engine) ComprehendListExamples(text string) (name string, lists [][]string, outs []string, ok bool) {
	toks := strings.Fields(text)
	if len(toks) < 2 || toks[0] != "the" {
		return "", nil, nil, false
	}
	name = toks[1]
	head := "the " + name + " of "
	for i, clause := range strings.Split(text, " and "+head) {
		if i > 0 {
			clause = head + clause
		}
		rest, ok := strings.CutPrefix(clause, head)
		if !ok {
			return "", nil, nil, false
		}
		isAt := strings.LastIndex(rest, " is ")
		if isAt < 0 {
			return "", nil, nil, false
		}
		out := strings.TrimSpace(rest[isAt+len(" is "):])
		nums, ok := e.listLiteralNums(strings.TrimSpace(rest[:isAt]))
		if !ok || !isNumber(out) {
			return "", nil, nil, false
		}
		lists = append(lists, nums)
		outs = append(outs, out)
	}
	if len(lists) < 2 {
		return "", nil, nil, false
	}
	return name, lists, outs, true
}

// ComprehendListReturnExamples reads a function shown by example whose result is a
// LIST, "the big of the list of 1 and 5 and 2 is the list of 5 and the big of the
// list of 4 and 1 is the list of 4", into the name, the example input lists, and
// the output lists. Clauses split on the repeated "and the <name> of"; the input
// and output list literals split on the clause's " is ". At least two pairs.
func (e *Engine) ComprehendListReturnExamples(text string) (name string, ins, outs [][]string, ok bool) {
	toks := strings.Fields(text)
	if len(toks) < 2 || toks[0] != "the" {
		return "", nil, nil, false
	}
	name = toks[1]
	head := "the " + name + " of "
	for i, clause := range strings.Split(text, " and "+head) {
		if i > 0 {
			clause = head + clause
		}
		rest, ok := strings.CutPrefix(clause, head)
		if !ok {
			return "", nil, nil, false
		}
		isAt := strings.Index(rest, " is ")
		if isAt < 0 {
			return "", nil, nil, false
		}
		inNums, iok := e.listLiteralNums(strings.TrimSpace(rest[:isAt]))
		outNums, ook := e.listLiteralNums(strings.TrimSpace(rest[isAt+len(" is "):]))
		if !iok || !ook {
			return "", nil, nil, false
		}
		ins = append(ins, inNums)
		outs = append(outs, outNums)
	}
	if len(ins) < 2 {
		return "", nil, nil, false
	}
	return name, ins, outs, true
}

// listLiteralNums comprehends "the list of A and B and ..." into its number
// elements, declining anything that is not a list of numbers.
func (e *Engine) listLiteralNums(text string) ([]string, bool) {
	comp, ok := e.comprehendLiteral(strings.Fields(text))
	if !ok || comp.Op != listWord {
		return nil, false
	}
	nums := make([]string, 0, len(comp.Args))
	for _, el := range comp.Args {
		if el.Op != "" || el.Var != "" || !isNumber(el.Lit) {
			return nil, false
		}
		nums = append(nums, el.Lit)
	}
	return nums, true
}

// minRecursiveExamples is how many pairs a recursive rule needs before it is
// searched. Recursion is the most expressive shape, so it asks for the most
// evidence, lest a short sequence be read as a recursion it does not mean.
const minRecursiveExamples = 4

// InduceRecursive searches for a simple recursive rule that carries each input to
// its output: the function is a base value for the smallest inputs, otherwise a
// searched combination of the input and the function of its predecessor. The base
// and its threshold are searched over a small space, and the COMBINE over the
// input, the predecessor's result, and small constants -- the same expression
// enumeration the rest of the synthesis uses, not a fixed pair of operations -- so
// factorial (number times the predecessor's result), a running sum (number plus
// it), and a doubling recurrence (it plus it) are all reachable. Each candidate is
// interpreted to prune and the one kept is grounded by running the recursion as the
// self-referential function the call generates. It declines unless there are pairs
// enough to pin a recursion, and when none fits.
func (e *Engine) InduceRecursive(name, param string, ins, outs []string, run func(string) (string, bool)) (Op, bool) {
	if len(ins) < minRecursiveExamples || len(ins) != len(outs) {
		return Op{}, false
	}
	if _, ok := e.byName["subtract"]; !ok {
		return Op{}, false // no way to step toward the base
	}
	inN, outN, ok := numberPairs(ins, outs)
	if !ok {
		return Op{}, false
	}
	selfCall := Computation{Op: name, Args: []Computation{
		{Op: "subtract", Args: []Computation{{Var: param}, {Lit: "1"}}}}}
	combines := e.genExprs([]string{param, recVar}, []string{"0", "1", "2"}, recMaxOps)
	for _, threshold := range []int{1, 0, 2} {
		for _, base := range []int{1, 0} {
			for _, combine := range combines {
				if !usesVar(combine, recVar) {
					continue // a combine that ignores the predecessor's result is no recursion
				}
				if !e.recFits(combine, threshold, base, param, inN, outN) {
					continue
				}
				body := Computation{Cond: &Conditional{
					Cmp: "<", Left: Computation{Var: param}, Right: Computation{Lit: strconv.Itoa(threshold)},
					Then: Computation{Lit: strconv.Itoa(base)},
					Else: substitute(combine, map[string]Computation{recVar: selfCall})}}
				hyp := Op{Name: name, Word: name, Params: []string{param}, Body: &body}
				if e.recursiveFits(hyp, ins, outs, run) {
					return hyp, true
				}
			}
		}
	}
	return Op{}, false
}

// recVar stands for the recursive result, the function of the predecessor, inside a
// combine candidate before it is substituted by the self-call.
const recVar = "rec"

// recMaxOps bounds the combine searched for a recursive rule, and recDepthCap
// guards the interpreter against an unbounded descent.
const recMaxOps = 2
const recDepthCap = 10000

// recFits reports whether the recursive rule maps every input to its output when
// interpreted, the quick check before the candidate is grounded by running.
func (e *Engine) recFits(combine Computation, threshold, base int, param string, ins, outs []int) bool {
	for i, n := range ins {
		v, ok := e.recEval(combine, threshold, base, param, n, 0)
		if !ok || v != outs[i] {
			return false
		}
	}
	return true
}

// recEval evaluates a recursive candidate at n: the base below the threshold, else
// the combine over the input and the predecessor's result, descending toward the
// base and guarded against running away.
func (e *Engine) recEval(combine Computation, threshold, base int, param string, n, depth int) (int, bool) {
	if depth > recDepthCap {
		return 0, false
	}
	if n < threshold {
		return base, true
	}
	prev, ok := e.recEval(combine, threshold, base, param, n-1, depth+1)
	if !ok {
		return 0, false
	}
	return e.evalExpr(combine, map[string]int{param: n, recVar: prev})
}

// numberPairs parses example input/output strings into integers.
func numberPairs(ins, outs []string) (in, out []int, ok bool) {
	in = make([]int, len(ins))
	out = make([]int, len(outs))
	for i := range ins {
		a, e1 := strconv.Atoi(ins[i])
		b, e2 := strconv.Atoi(outs[i])
		if e1 != nil || e2 != nil {
			return nil, nil, false
		}
		in[i], out[i] = a, b
	}
	return in, out, true
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

// searchOps are the operations the synthesis search composes: the arithmetic
// primitives, and the defined integer functions acquired since, so a procedure is
// assembled from learned vocabulary and not arithmetic alone. A composed function
// must be one the search can both interpret (by inlining) and ground: every
// parameter and the result an integer, arity one or two, and not recursive, so the
// inline terminates. Raw loop operations (list-typed) and recursive ones are left
// out. The count of learned operations is bounded so a brain of many functions does
// not swamp the search before a match is found.
func (e *Engine) searchOps() []Op {
	var ops []Op
	learned := 0
	for _, o := range e.Ops() {
		if o.Name == e.synthSelf {
			continue // the function being defined is not a building block of itself
		}
		switch {
		case o.Symbol != "":
			ops = append(ops, o) // an arithmetic primitive
		case learned < searchOpCap && o.Body != nil && !e.recursive(o) && e.intOp(o):
			if a := o.arity(); a == 1 || a == 2 {
				ops = append(ops, o)
				learned++
			}
		}
	}
	return ops
}

// searchOpCap bounds how many learned operations enter the synthesis search, a
// ceiling on the building blocks so a large inventory does not swamp it.
const searchOpCap = 32

// intOp reports whether a defined operation is a plain integer function, every
// parameter and the result an integer, the shape the expression search composes
// and interprets.
func (e *Engine) intOp(o Op) bool {
	for i := range o.Params {
		if o.paramType(i) != typeInt {
			return false
		}
	}
	ret, ok := e.retType(o)
	return ok && ret == typeInt
}

// Primitive reports whether a word names a seeded operation, so induction and
// definition both refuse to shadow one.
func (e *Engine) Primitive(word string) bool {
	if op, ok := e.byName[word]; ok && op.Body == nil {
		return true
	}
	if ref, ok := e.opRef(word); ok {
		if op, ok := e.byName[ref.Name]; ok && op.Body == nil {
			return true
		}
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
