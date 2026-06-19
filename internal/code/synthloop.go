package code

import (
	"strconv"
	"strings"
)

// Loop synthesis assembles an imperative program over a list, the structure
// emerging from the search rather than a fixed fold being filled in. A candidate
// is a range over the list whose body the search composes: an accumulator updated
// each step, or a result list a guarded element is appended to. The loop, the
// accumulators it starts, and what it returns follow from the body the search
// lands on, so "total" and "the squares over three" come out as the different
// programs they are. Each candidate is interpreted on the examples to prune, and
// the one kept is grounded by running it as Go, the same standard every
// construction meets.

const (
	loopMaxOps  = 2   // the largest expression a loop body statement may carry
	loopExprCap = 800 // a ceiling on the expressions enumerated per body slot
)

// genExprs enumerates the expressions over the given variables and constants up to
// the operator bound, the building blocks a loop body's statements draw on. They
// are deduped by the code they generate, so the same expression is not tried
// twice, and capped so the enumeration ends.
func (e *Engine) genExprs(vars, consts []string, maxOps int) []Computation {
	type sized struct {
		c   Computation
		ops int
	}
	seen := map[string]bool{}
	var pool []sized
	add := func(c Computation, ops int) {
		key, ok := e.ToCode(c)
		if !ok || seen[key] {
			return
		}
		seen[key] = true
		pool = append(pool, sized{c, ops})
	}
	for _, v := range vars {
		add(Computation{Var: v}, 0)
	}
	for _, c := range consts {
		add(Computation{Lit: c}, 0)
	}
	prims := e.primitives()
	for round := 1; round <= maxOps && len(pool) < loopExprCap; round++ {
		snapshot := append([]sized(nil), pool...)
		for _, a := range snapshot {
			for _, b := range snapshot {
				if a.ops+b.ops+1 > maxOps {
					continue
				}
				for _, op := range prims {
					add(Computation{Op: op.Name, Args: []Computation{a.c, b.c}}, a.ops+b.ops+1)
					if len(pool) >= loopExprCap {
						break
					}
				}
			}
		}
	}
	out := make([]Computation, len(pool))
	for i, p := range pool {
		out[i] = p.c
	}
	return out
}

// SynthesizeFold searches for a program that folds a list to a number, the
// structure assembled rather than given: an accumulator started at zero or one,
// updated each step by an expression over the accumulator and the element, and
// returned. It interprets each candidate on the examples and grounds the one it
// keeps by running it. lists are the example inputs, outs their wanted results. It
// returns the loop body as Go and false when nothing within the bound fits.
func (e *Engine) SynthesizeFold(param string, lists [][]int, outs []int, run func(string) (string, bool)) (string, bool) {
	if len(lists) < 2 || len(lists) != len(outs) {
		return "", false
	}
	const acc = "a"
	for _, expr := range e.genExprs([]string{acc, "x"}, []string{"0", "1", "2"}, loopMaxOps) {
		if !usesVar(expr, acc) {
			continue // an update that ignores the accumulator is no fold
		}
		for _, init := range []int{0, 1} {
			if !foldInterprets(expr, acc, init, lists, outs) {
				continue
			}
			body, ok := e.foldBody(param, acc, init, expr)
			if ok && e.foldGrounds(param, body, lists, outs, run) {
				return body, true
			}
		}
	}
	return "", false
}

// foldInterprets reports whether the candidate fold maps every example list to its
// output, the quick check the search prunes by before grounding.
func foldInterprets(update Computation, acc string, init int, lists [][]int, outs []int) bool {
	for i, list := range lists {
		a := init
		ok := true
		for _, x := range list {
			a, ok = evalExpr(update, map[string]int{acc: a, "x": x})
			if !ok {
				break
			}
		}
		if !ok || a != outs[i] {
			return false
		}
	}
	return true
}

// foldBody writes the loop the fold means as Go: start the accumulator, range the
// list updating it, return it. The element is the loop variable and the
// accumulator the carried state, the two names the update expression is over.
func (e *Engine) foldBody(param, acc string, init int, update Computation) (string, bool) {
	step, ok := e.ToCode(update)
	if !ok {
		return "", false
	}
	return acc + " := " + strconv.Itoa(init) + "; for _, x := range " + param +
		" { " + acc + " = " + step + " }; return " + acc, true
}

// foldGrounds runs the synthesized loop over each example list and checks it
// yields the output, the grounding that decides the program is the rule. The body
// is wrapped in a function literal applied to the list, so it runs as the Go it is.
func (e *Engine) foldGrounds(param, body string, lists [][]int, outs []int, run func(string) (string, bool)) bool {
	for i, list := range lists {
		src := "(func(" + param + " []int) int { " + body + " })(" + intList(list) + ")"
		val, ok := run(src)
		if !ok || val != strconv.Itoa(outs[i]) {
			return false
		}
	}
	return true
}

// intList writes a Go slice literal for the integers, the form an example list
// runs as.
func intList(xs []int) string {
	parts := make([]string, len(xs))
	for i, x := range xs {
		parts[i] = strconv.Itoa(x)
	}
	return "[]int{" + strings.Join(parts, ", ") + "}"
}
