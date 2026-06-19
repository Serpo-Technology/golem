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

	// The search prunes by behaviour: an expression whose results on the example
	// inputs match a kept one is interchangeable, so only the first of each is grown
	// from, and the match is the one whose behaviour is the wanted outputs and which
	// grounds by running.
	seen := map[string]bool{}
	var found Computation
	got := false
	e.buildExprs(leaves, synthMaxOps, synthPoolCap, func(c Computation, ops int) (bool, bool) {
		key, ok := behaviourKey(c, envs)
		if !ok || seen[key] {
			return false, false
		}
		seen[key] = true
		if key == wantKey && e.bodyFits(c, param, ins, outs, run) {
			found, got = c, true
			return true, true
		}
		return true, false
	})
	return found, got
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
	prims := e.primitives()
	for grew := true; grew && len(pool) < cap; {
		grew = false
		snapshot := append([]cand(nil), pool...)
		for _, a := range snapshot {
			for _, b := range snapshot {
				if a.ops+b.ops+1 > maxOps {
					continue
				}
				for _, op := range prims {
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
func behaviourKey(c Computation, envs []map[string]int) (string, bool) {
	var b strings.Builder
	for _, env := range envs {
		v, ok := evalExpr(c, env)
		if !ok {
			return "", false
		}
		b.WriteString(strconv.Itoa(v))
		b.WriteByte(',')
	}
	return b.String(), true
}

// evalExpr evaluates an assembled arithmetic expression over the integer
// environment, the quick filter the search prunes by before the expression it
// keeps is grounded by running. It fails on a division by zero, the one case
// running would refuse too.
func evalExpr(c Computation, env map[string]int) (int, bool) {
	switch {
	case c.Var != "":
		v, ok := env[c.Var]
		return v, ok
	case c.Op == "":
		n, err := strconv.Atoi(c.Lit)
		return n, err == nil
	}
	if len(c.Args) != 2 {
		return 0, false
	}
	a, aok := evalExpr(c.Args[0], env)
	b, bok := evalExpr(c.Args[1], env)
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
	case "divide":
		if b == 0 {
			return 0, false
		}
		return a / b, true
	}
	return 0, false
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
