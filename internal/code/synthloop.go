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
	leaves := make([]Computation, 0, len(vars)+len(consts))
	for _, v := range vars {
		leaves = append(leaves, Computation{Var: v})
	}
	for _, c := range consts {
		leaves = append(leaves, Computation{Lit: c})
	}
	// Here the pruning is by code: an expression already generated is not generated
	// again. Every distinct one is kept, since a loop body uses them in a context
	// (the accumulator changing each step) where behaviour cannot be judged ahead.
	seen := map[string]bool{}
	var out []Computation
	e.buildExprs(leaves, maxOps, loopExprCap, func(c Computation, ops int) (bool, bool) {
		key, ok := e.ToCode(c)
		if !ok || seen[key] {
			return false, false
		}
		seen[key] = true
		out = append(out, c)
		return true, false
	})
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

// SynthesizeListReturn searches for a program that turns a list into a list, the
// structure assembled rather than given: a range over the list building a result,
// either by transforming each element (a map) or by keeping the elements a
// comparison holds for (a filter). It interprets each candidate on the examples
// and grounds the one it keeps by running it. It returns the loop body as Go and
// false when nothing within the bound fits.
func (e *Engine) SynthesizeListReturn(param string, ins, outs [][]int, run func(string) (string, bool)) (string, bool) {
	if len(ins) < 2 || len(ins) != len(outs) {
		return "", false
	}
	for _, t := range e.genExprs([]string{"x"}, []string{"0", "1", "2"}, loopMaxOps) {
		if !usesVar(t, "x") {
			continue // a transform that ignores the element is no map
		}
		if mapInterprets(t, ins, outs) {
			if body, ok := e.mapBody(param, t); ok && e.listGrounds(param, body, ins, outs, run) {
				return body, true
			}
		}
	}
	for _, c := range filterConsts(ins) {
		for _, cmp := range []string{">", "<", "=="} {
			if filterInterprets(cmp, c, ins, outs) {
				body := e.filterBody(param, cmp, c)
				if e.listGrounds(param, body, ins, outs, run) {
					return body, true
				}
			}
		}
	}
	return "", false
}

// mapInterprets reports whether transforming each element by the candidate yields
// the output list, for every example.
func mapInterprets(t Computation, ins, outs [][]int) bool {
	for i, list := range ins {
		if len(list) != len(outs[i]) {
			return false
		}
		for j, x := range list {
			v, ok := evalExpr(t, map[string]int{"x": x})
			if !ok || v != outs[i][j] {
				return false
			}
		}
	}
	return true
}

// filterInterprets reports whether keeping the elements the comparison holds for
// yields the output list, for every example.
func filterInterprets(cmp string, c int, ins, outs [][]int) bool {
	for i, list := range ins {
		var kept []int
		for _, x := range list {
			if compareInt(cmp, x, c) {
				kept = append(kept, x)
			}
		}
		if !sameInts(kept, outs[i]) {
			return false
		}
	}
	return true
}

// compareInt evaluates a comparison the filter turns on.
func compareInt(cmp string, a, b int) bool {
	switch cmp {
	case ">":
		return a > b
	case "<":
		return a < b
	case "==":
		return a == b
	}
	return false
}

// sameInts reports whether two integer lists are equal.
func sameInts(a, b []int) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// mapBody writes the loop a map means: build a result, range the list appending
// the transform of each element, return it.
func (e *Engine) mapBody(param string, t Computation) (string, bool) {
	step, ok := e.ToCode(t)
	if !ok {
		return "", false
	}
	return mapLoop("b", "x", param, step), true
}

// mapLoop writes a range over a list that builds a new list by appending the body
// computed for each element, the one form a map takes whether it was synthesized
// from examples (here) or comprehended from a told "each" definition (the Map
// render). The variable names are the caller's so the two sites read as they did
// while sharing the single shape.
func mapLoop(resultVar, elemVar, listExpr, bodyExpr string) string {
	return resultVar + " := []int{}; for _, " + elemVar + " := range " + listExpr +
		" { " + resultVar + " = append(" + resultVar + ", " + bodyExpr + ") }; return " + resultVar
}

// filterBody writes the loop a filter means: build a result, range the list
// appending each element the comparison keeps, return it.
func (e *Engine) filterBody(param, cmp string, c int) string {
	return "b := []int{}; for _, x := range " + param + " { if x " + cmp + " " + strconv.Itoa(c) +
		" { b = append(b, x) } }; return b"
}

// filterConsts are the thresholds a filter's comparison is tried against: the
// distinct values across the example lists, plus zero, one, and two, so a split
// between kept and dropped elements is reachable.
func filterConsts(ins [][]int) []int {
	seen := map[int]bool{0: true, 1: true, 2: true}
	out := []int{0, 1, 2}
	for _, list := range ins {
		for _, x := range list {
			if !seen[x] {
				seen[x] = true
				out = append(out, x)
			}
		}
	}
	return out
}

// listGrounds runs the synthesized list loop over each example and checks it yields
// the output list, the grounding that decides the program. The body is wrapped in a
// function literal returning a slice, so it runs as the Go it is.
func (e *Engine) listGrounds(param, body string, ins, outs [][]int, run func(string) (string, bool)) bool {
	for i, list := range ins {
		src := "(func(" + param + " []int) []int { " + body + " })(" + intList(list) + ")"
		val, ok := run(src)
		if !ok || val != intResult(outs[i]) {
			return false
		}
	}
	return true
}

// intResult writes an integer list the way a run reports it, "[5 9]", so a
// grounded list output is compared in the form it comes back.
func intResult(xs []int) string {
	parts := make([]string, len(xs))
	for i, x := range xs {
		parts[i] = strconv.Itoa(x)
	}
	return "[" + strings.Join(parts, " ") + "]"
}
