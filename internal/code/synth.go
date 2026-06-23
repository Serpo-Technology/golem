package code

import (
	"strconv"
	"strings"
)

// Synthesis assembles a program by composing and running it, rather than fitting a
// fixed template. The engine enumerates expressions over the parameter and a few
// constants from the smallest up, evaluates each on the example inputs, and keeps
// the smallest whose results are the examples' outputs. Two expressions that
// compute the same values on the examples are interchangeable, so only the first
// of each behaviour is kept (observational equivalence), which holds the search
// finite. What it lands on is decided by running, the same standard the told and
// the called surfaces meet, so the rule is the code's effect, not a shape guessed
// here. This is the seed of writing code by reasoning over the Go floor rather
// than by per-shape templates: the operators and operands are the building blocks,
// and the procedure emerges from composing and grounding them.

const (
	synthMaxOps  = 3    // the most operators an assembled expression may carry
	synthPoolCap = 6000 // a ceiling on distinct behaviours, so the search ends
)

// SynthesizeExpr searches for an expression over the parameter that carries every
// example input to its output, assembled from the arithmetic primitives and ground
// by running. The parameter and a few constants are the leaves; each round
// combines what it has with the operators, keeping one expression per distinct
// behaviour, until one matches every example or the bound is reached.
func (e *Engine) SynthesizeExpr(param string, ins, outs []string, run func(string) (string, bool)) (Computation, bool) {
	if len(ins) < 2 || len(ins) != len(outs) {
		return Computation{}, false
	}
	c, ok := e.fitExpr(param, ins, outs)
	if !ok || !e.bodyFits(c, param, ins, outs, run) {
		return Computation{}, false
	}
	return c, true
}

// fitExpr finds the smallest expression over the parameter whose interpreted
// behaviour on the example inputs is the outputs, the search SynthesizeExpr grounds
// and a conditional's branch fit draws on. It prunes by behaviour: an expression
// whose results on the inputs match a kept one is interchangeable, so only the first
// of each is grown from, and the match is the one whose behaviour is the wanted
// outputs. It does not run the candidate, since a branch is decided by
// interpretation and the assembled body is what gets grounded; the caller grounds.
func (e *Engine) fitExpr(param string, ins, outs []string) (Computation, bool) {
	if len(ins) == 0 || len(ins) != len(outs) {
		return Computation{}, false
	}
	envs := make([]map[string]int, len(ins))
	for i, in := range ins {
		n, err := strconv.Atoi(in)
		if err != nil {
			return Computation{}, false
		}
		envs[i] = map[string]int{param: n}
	}
	var want strings.Builder
	for _, o := range outs {
		if _, err := strconv.Atoi(o); err != nil {
			return Computation{}, false
		}
		want.WriteString(o)
		want.WriteByte(',')
	}
	wantKey := want.String()

	leaves := make([]Computation, 0)
	for _, lit := range synthConstants(ins, outs) {
		leaves = append(leaves, Computation{Lit: lit})
	}
	leaves = append(leaves, Computation{Var: param})

	seen := map[string]bool{}
	var found Computation
	got := false
	e.buildExprs(leaves, synthMaxOps, synthPoolCap, func(c Computation, ops int) (bool, bool) {
		if dividesByParam(c, param) {
			return false, false // a step function from truncation, not a rule, see below
		}
		key, ok := e.behaviourKey(c, envs)
		if !ok || seen[key] {
			return false, false
		}
		seen[key] = true
		if key == wantKey {
			found, got = c, true
			return true, true
		}
		return true, false
	})
	return found, got
}

// dividesByParam reports whether a candidate divides by a quantity that depends on
// the parameter. Integer division by the input truncates to a step function, which
// over-fits a finite example set by mimicking a piecewise rule (relu as
// number * (1 + 3 / number)), so the expression search leaves it out and a
// genuinely piecewise function is discovered as a conditional instead. Division by
// a constant (number / 2) is untouched. A rule that truly divides by the input is
// vanishingly rare and better told than guessed.
func dividesByParam(c Computation, param string) bool {
	if c.Op == "divide" && len(c.Args) == 2 && usesVar(c.Args[1], param) {
		return true
	}
	for _, a := range c.Args {
		if dividesByParam(a, param) {
			return true
		}
	}
	return false
}

// SynthesizeConditional discovers a two-way conditional from examples that no single
// expression fits: a predicate over the parameter splits the examples, each side fit
// by its own expression, "the abs of -3 is 3 and the abs of 5 is 5 ..." landing on
// number if number > 0 else 0 - number. The predicate is the parameter compared to a
// constant, the comparison and the constant drawn from the floor the rest of the
// search uses, the branches found by interpretation, and the whole grounded by
// running. It needs at least two examples on each side of a split, so at least four
// in all, and declines when no split with two distinct fitting branches grounds. A
// split whose branches are the same expression is no conditional and is left to the
// expression search.
func (e *Engine) SynthesizeConditional(param string, ins, outs []string, run func(string) (string, bool)) (Computation, bool) {
	if len(ins) < 4 || len(ins) != len(outs) {
		return Computation{}, false
	}
	inN, _, ok := numberPairs(ins, outs)
	if !ok {
		return Computation{}, false
	}
	for _, cmp := range []string{">", "<", "=="} {
		for _, c := range condConstants(ins, outs) {
			cv, err := strconv.Atoi(c)
			if err != nil {
				continue
			}
			var tIns, tOuts, fIns, fOuts []string
			for i := range ins {
				if compareInt(cmp, inN[i], cv) {
					tIns, tOuts = append(tIns, ins[i]), append(tOuts, outs[i])
				} else {
					fIns, fOuts = append(fIns, ins[i]), append(fOuts, outs[i])
				}
			}
			if len(tIns) < 2 || len(fIns) < 2 {
				continue // an empty or single-point side underdetermines a branch
			}
			thenE, tok := e.fitExpr(param, tIns, tOuts)
			elseE, fok := e.fitExpr(param, fIns, fOuts)
			if !tok || !fok || sameComputation(thenE, elseE) {
				continue
			}
			cond := Computation{Cond: &Conditional{
				Cmp: cmp, Left: Computation{Var: param}, Right: Computation{Lit: c},
				Then: thenE, Else: elseE}}
			if e.bodyFits(cond, param, ins, outs, run) {
				return cond, true
			}
		}
	}
	return Computation{}, false
}

// condConstants are the thresholds a conditional's predicate is tried against: the
// small constants the floor seeds, zero among them since abs and relu and their kind
// turn on the sign, plus the constants a pair suggests.
func condConstants(ins, outs []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, c := range append([]string{"0", "1", "-1", "2", "-2"}, deriveConstants(ins[0], outs[0])...) {
		if !seen[c] {
			seen[c] = true
			out = append(out, c)
		}
	}
	return out
}

// SynthesizeInverse searches for the inverse of a known unary operation: the
// function whose output, fed back through the operation, returns the input. The
// goal states a property, not examples, so the procedure is discovered rather than
// shown. The inverse's domain is the operation's range, so the test inputs are the
// operation applied to a sample (which keeps an integer inverse exact, the inverse
// of double tested on even numbers), and the pre-image each came from seeds the
// search constants. Each candidate body is assembled over the parameter (composing
// the grounded operation inventory), the property op(candidate(in)) == in checked
// by interpreting to prune and by running to ground. It returns the body and false
// when the operation is no one-to-one unary function or its inverse is beyond the
// search's bound.
func (e *Engine) SynthesizeInverse(param, opName string, run func(string) (string, bool)) (Computation, bool) {
	op, ok := e.byName[opName]
	if !ok || op.Body == nil || op.arity() != 1 {
		return Computation{}, false
	}
	var ins, pre []int
	seenIn := map[int]bool{}
	for _, k := range []int{1, 2, 3, 4, 5} {
		v, ok := e.runOp(op, k, run)
		if !ok || seenIn[v] {
			continue // a repeated output means the operation is not one-to-one here
		}
		seenIn[v] = true
		ins = append(ins, v)
		pre = append(pre, k)
	}
	if len(ins) < 2 {
		return Computation{}, false
	}
	inStr := make([]string, len(ins))
	preStr := make([]string, len(pre))
	envs := make([]map[string]int, len(ins))
	for i := range ins {
		inStr[i], preStr[i] = strconv.Itoa(ins[i]), strconv.Itoa(pre[i])
		envs[i] = map[string]int{param: ins[i]}
	}
	leaves := make([]Computation, 0)
	for _, lit := range synthConstants(inStr, preStr) {
		leaves = append(leaves, Computation{Lit: lit})
	}
	leaves = append(leaves, Computation{Var: param})

	seen := map[string]bool{}
	var found Computation
	got := false
	e.buildExprs(leaves, synthMaxOps, synthPoolCap, func(c Computation, ops int) (bool, bool) {
		key, ok := e.behaviourKey(c, envs)
		if !ok || seen[key] {
			return false, false
		}
		seen[key] = true
		if e.inverseHolds(c, op, param, ins) && e.inverseGrounds(c, op, param, ins, run) {
			found, got = c, true
			return true, true
		}
		return true, false
	})
	return found, got
}

// runOp evaluates a unary operation at an integer by running it, the forward
// direction whose result the synthesized inverse must return to.
func (e *Engine) runOp(op Op, k int, run func(string) (string, bool)) (int, bool) {
	src, ok := e.ToCode(Computation{Op: op.Name, Args: []Computation{{Lit: strconv.Itoa(k)}}})
	if !ok {
		return 0, false
	}
	val, ok := run(src)
	if !ok {
		return 0, false
	}
	n, err := strconv.Atoi(val)
	return n, err == nil
}

// inverseHolds reports whether the candidate is the operation's inverse on the test
// inputs when interpreted: the operation applied to the candidate's output returns
// the input. It is the fast prefilter before grounding.
func (e *Engine) inverseHolds(c Computation, op Op, param string, ins []int) bool {
	for _, in := range ins {
		applied := substitute(c, map[string]Computation{param: {Lit: strconv.Itoa(in)}})
		v, ok := e.evalExpr(Computation{Op: op.Name, Args: []Computation{applied}}, nil)
		if !ok || v != in {
			return false
		}
	}
	return true
}

// inverseGrounds reports whether the candidate is the operation's inverse on the
// test inputs when run, the grounding that decides the discovered procedure: the
// operation applied to the candidate's output returns the input.
func (e *Engine) inverseGrounds(c Computation, op Op, param string, ins []int, run func(string) (string, bool)) bool {
	for _, in := range ins {
		applied := substitute(c, map[string]Computation{param: {Lit: strconv.Itoa(in)}})
		src, ok := e.ToCode(Computation{Op: op.Name, Args: []Computation{applied}})
		if !ok {
			return false
		}
		val, ok := run(src)
		if !ok || val != strconv.Itoa(in) {
			return false
		}
	}
	return true
}

// buildExprs grows expression trees over the leaves up to maxOps, combining them
// with the arithmetic primitives. visit is called on each leaf and each freshly
// built tree with its operator count; it returns keep (the tree is new by the
// caller's deduplication and should be grown from) and stop (a match was found,
// halt). The pruning lives in visit, because a target search prunes by behaviour
// while a pool of body candidates prunes by code, but the growth is the one shared
// enumeration here. It stops at the cap so the search ends.
func (e *Engine) buildExprs(leaves []Computation, maxOps, cap int, visit func(Computation, int) (keep, stop bool)) {
	type cand struct {
		c   Computation
		ops int
	}
	var pool []cand
	consider := func(c Computation, ops int) bool {
		keep, stop := visit(c, ops)
		if keep {
			pool = append(pool, cand{c, ops})
		}
		return stop
	}
	for _, l := range leaves {
		if consider(l, 0) {
			return
		}
	}
	ops := e.searchOps()
	for grew := true; grew && len(pool) < cap; {
		grew = false
		snapshot := append([]cand(nil), pool...)
		for _, a := range snapshot {
			// A unary operation wraps one candidate; a binary one combines a pair. With
			// no vocabulary learned searchOps is just the arithmetic primitives in the
			// order they were tried before, so a purely arithmetic search is unchanged.
			for _, op := range ops {
				if op.arity() != 1 || a.ops+1 > maxOps {
					continue
				}
				before := len(pool)
				if consider(Computation{Op: op.Name, Args: []Computation{a.c}}, a.ops+1) {
					return
				}
				if len(pool) > before {
					grew = true
				}
				if len(pool) >= cap {
					return
				}
			}
			for _, b := range snapshot {
				if a.ops+b.ops+1 > maxOps {
					continue
				}
				for _, op := range ops {
					if op.arity() != 2 {
						continue
					}
					before := len(pool)
					if consider(Computation{Op: op.Name, Args: []Computation{a.c, b.c}}, a.ops+b.ops+1) {
						return
					}
					if len(pool) > before {
						grew = true
					}
					if len(pool) >= cap {
						return
					}
				}
			}
		}
	}
}

// behaviourKey is an expression's results on the example inputs, the signature
// observational equivalence groups by. It fails when the expression cannot be
// evaluated on some input (a division by zero), so such a candidate is dropped.
func (e *Engine) behaviourKey(c Computation, envs []map[string]int) (string, bool) {
	var b strings.Builder
	for _, env := range envs {
		v, ok := e.evalExpr(c, env)
		if !ok {
			return "", false
		}
		b.WriteString(strconv.Itoa(v))
		b.WriteByte(',')
	}
	return b.String(), true
}

// evalExpr evaluates an assembled expression over the integer environment, the
// quick filter the search prunes by before the expression it keeps is grounded by
// running. The arithmetic primitives compute directly; a learned operation
// evaluates by inlining its body over the arguments, so a candidate composed from
// acquired vocabulary is interpreted rather than silently dropped. It fails on a
// division by zero, the one case running would refuse too.
func (e *Engine) evalExpr(c Computation, env map[string]int) (int, bool) {
	if c.Var != "" {
		v, ok := env[c.Var]
		return v, ok
	}
	if c.Op == "" {
		n, err := strconv.Atoi(c.Lit)
		return n, err == nil
	}
	switch c.Op {
	case "add", "subtract", "multiply", "divide":
		if len(c.Args) != 2 {
			return 0, false
		}
		a, aok := e.evalExpr(c.Args[0], env)
		b, bok := e.evalExpr(c.Args[1], env)
		if !aok || !bok {
			return 0, false
		}
		switch c.Op {
		case "add":
			return a + b, true
		case "subtract":
			return a - b, true
		case "multiply":
			return a * b, true
		default: // divide
			if b == 0 {
				return 0, false
			}
			return a / b, true
		}
	}
	// A learned defined operation: inline its body over the arguments and evaluate.
	// searchOps admits only non-recursive integer functions, so the inline ends.
	op, ok := e.byName[c.Op]
	if !ok || op.Body == nil {
		return 0, false
	}
	inlined, ok := e.inline(op, c.Args)
	if !ok {
		return 0, false
	}
	return e.evalExpr(inlined, env)
}

// synthConstants are the leaf literals the search starts from: zero, one, two, and
// the constants a single pair suggests (its differences and clean quotients), so a
// rule turning on a constant is reachable without enumerating every integer.
func synthConstants(ins, outs []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, c := range append([]string{"0", "1", "2"}, deriveConstants(ins[0], outs[0])...) {
		if !seen[c] {
			seen[c] = true
			out = append(out, c)
		}
	}
	return out
}
