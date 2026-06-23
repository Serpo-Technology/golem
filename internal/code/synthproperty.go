package code

import (
	"strconv"
	"strings"
)

// Property discovery from a relation, not examples. A function is stated by a property its
// output must satisfy, and the procedure is discovered by assembling candidates and keeping
// the one whose property holds when run, the same standard the inverse and the extremal
// meet. This is the general two-sided form: a property is a conjunction of relations between
// two computations over the input and the output, so the inverse (a known operation of the
// output returns the input) and the magnitude (the output squares to the input squared and
// is not negative) are the same shape with different relations. The relations are grounded
// computations over grounded operations and comparisons, never a phrase guessed here.

// ResultVar names the function's output inside a property's relations, the value a candidate
// body produces for the input. A relation reads it like a parameter; grounding renders it as
// the variable holding the run output.
const ResultVar = "result"

// A Relation is one constraint of a property: left <cmp> right, where left and right are
// computations over the input parameter, the output (ResultVar), and grounded operations,
// and cmp is a grounded comparison symbol ("==", "<", ">").
type Relation struct {
	Left  Computation
	Cmp   string
	Right Computation
}

// A Property is the conjunction a discovered function must satisfy. Equational holds the
// relations a single candidate expression must meet. Ordering, when set, marks the region
// the output must AVOID (so non-negativity is "the output is not below zero"); it both rules
// out a single expression that strays into it and supplies the split when two expressions
// each satisfy the equational part on a different side.
type Property struct {
	Equational []Relation
	Ordering   *Relation
}

// propertyProbes are the inputs a scalar property is checked on, no outputs given since the
// property decides. They span the sign so an impostor is caught: a negative rejects the
// identity, a positive rejects the negation, and zero is the edge between them.
func propertyProbes() []int {
	return []int{-7, -3, -1, 0, 1, 5, 9}
}

// SynthesizeFromProperty discovers a unary function whose output satisfies the property,
// verified by running. It assembles expression candidates over the parameter, keeps those
// satisfying the equational relations on the probes, and returns the one that meets the
// whole property; when none does but two expressions each satisfy the equational part, it
// builds a two-way conditional that uses the ordering to choose between them and grounds the
// whole property by running. abs is the conditional case (number and its negation both
// square to the input squared; the ordering, not below zero, splits them on sign).
func (e *Engine) SynthesizeFromProperty(param string, prop Property, run func(string) (string, bool)) (Computation, bool) {
	if len(prop.Equational) == 0 {
		return Computation{}, false
	}
	probes := propertyProbes()
	envs := make([]map[string]int, len(probes))
	for i, n := range probes {
		envs[i] = map[string]int{param: n}
	}
	leaves := []Computation{{Lit: "0"}, {Lit: "1"}, {Lit: "2"}, {Var: param}}

	const qualifierCap = 16
	var qualifiers []Computation
	seen := map[string]bool{}
	e.buildExprs(leaves, synthMaxOps, synthPoolCap, func(c Computation, ops int) (bool, bool) {
		key, ok := e.behaviourKey(c, envs)
		if !ok || seen[key] {
			return false, false
		}
		seen[key] = true
		if len(qualifiers) < qualifierCap && e.equationalHolds(c, param, prop.Equational, probes) {
			qualifiers = append(qualifiers, c)
		}
		return true, false
	})

	// A single expression that meets the whole property is the answer (the inverse and the
	// halving land here): no split is needed.
	for _, q := range qualifiers {
		if e.propertyHolds(q, param, prop, probes) && e.propertyGrounds(q, param, prop, probes, run) {
			return q, true
		}
	}
	// Otherwise the ordering splits two equational-satisfying expressions into a conditional,
	// the region one strays into served by the other; the prefilter interprets the choice and
	// the whole property grounds it.
	if prop.Ordering != nil {
		for _, a := range qualifiers {
			for _, b := range qualifiers {
				if sameComputation(a, b) {
					continue
				}
				cond := conditionalFromOrdering(a, b, *prop.Ordering)
				if e.propertyHolds(cond, param, prop, probes) && e.propertyGrounds(cond, param, prop, probes, run) {
					return cond, true
				}
			}
		}
	}
	return Computation{}, false
}

// evalProperty interprets a candidate over the environment, handling a top-level conditional
// the expression evaluator does not: the predicate decides which branch evaluates. A
// candidate from the property search is either an expression or a two-way conditional whose
// branches are expressions, so delegating the rest to evalExpr is enough.
func (e *Engine) evalProperty(c Computation, env map[string]int) (int, bool) {
	if c.Cond != nil {
		l, lok := e.evalProperty(c.Cond.Left, env)
		r, rok := e.evalProperty(c.Cond.Right, env)
		if !lok || !rok {
			return 0, false
		}
		if compareInt(c.Cond.Cmp, l, r) {
			return e.evalProperty(c.Cond.Then, env)
		}
		return e.evalProperty(c.Cond.Else, env)
	}
	return e.evalExpr(c, env)
}

// conditionalFromOrdering builds "b if a strays into the forbidden region else a": the
// predicate is the ordering with the output replaced by the first expression, so where a's
// output would violate the ordering, the second expression is used instead. For abs with
// a = number, b = 0 - number, ordering result < 0, this is 0 - number if number < 0 else
// number. The predicate's comparison is the ordering's, a grounded symbol, so no comparison
// is invented.
func conditionalFromOrdering(a, b Computation, ordering Relation) Computation {
	pred := substitute(ordering.Left, map[string]Computation{ResultVar: a})
	return Computation{Cond: &Conditional{
		Cmp: ordering.Cmp, Left: pred, Right: ordering.Right, Then: b, Else: a,
	}}
}

// equationalHolds reports whether the candidate expression meets every equational relation
// on the probes when interpreted, the prefilter that collects the candidates a split draws
// on.
func (e *Engine) equationalHolds(c Computation, param string, relations []Relation, probes []int) bool {
	for _, n := range probes {
		out, ok := e.evalProperty(c, map[string]int{param: n})
		if !ok {
			return false
		}
		env := map[string]int{param: n, ResultVar: out}
		for _, r := range relations {
			if !e.relationHolds(r, env) {
				return false
			}
		}
	}
	return true
}

// propertyHolds reports whether the candidate expression meets the whole property on the
// probes when interpreted: every equational relation holds and the output never strays into
// the ordering's forbidden region. It is the prefilter before grounding a single expression.
func (e *Engine) propertyHolds(c Computation, param string, prop Property, probes []int) bool {
	for _, n := range probes {
		out, ok := e.evalProperty(c, map[string]int{param: n})
		if !ok {
			return false
		}
		env := map[string]int{param: n, ResultVar: out}
		for _, r := range prop.Equational {
			if !e.relationHolds(r, env) {
				return false
			}
		}
		if prop.Ordering != nil && e.relationHolds(*prop.Ordering, env) {
			return false
		}
	}
	return true
}

// relationHolds evaluates a relation over the environment: both sides interpret and the
// comparison holds.
func (e *Engine) relationHolds(r Relation, env map[string]int) bool {
	l, lok := e.evalExpr(r.Left, env)
	rr, rok := e.evalExpr(r.Right, env)
	return lok && rok && compareInt(r.Cmp, l, rr)
}

// propertyGrounds runs the candidate body over each probe inside a program that also checks
// the property, keeping it only when every run holds, the grounding that decides the
// discovered procedure. The property is verified in the Go that will be stored, not against
// a precomputed answer.
func (e *Engine) propertyGrounds(body Computation, param string, prop Property, probes []int, run func(string) (string, bool)) bool {
	for _, n := range probes {
		src, ok := e.propertyCheckSrc(param, body, prop, n)
		if !ok {
			return false
		}
		val, ok := run(src)
		if !ok || val != "true" {
			return false
		}
	}
	return true
}

// propertyCheckSrc writes a program that runs the body at the probe to its output, then
// reports whether the property holds of that output: every equational relation, and the
// output not in the ordering's forbidden region. The relations render the output as the run
// variable and the input as the probe, the comparisons their grounded symbols.
func (e *Engine) propertyCheckSrc(param string, body Computation, prop Property, probe int) (string, bool) {
	bodySrc, ok := e.ToCode(body)
	if !ok {
		return "", false
	}
	var parts []string
	for _, r := range prop.Equational {
		g, ok := e.relationGo(r)
		if !ok {
			return "", false
		}
		parts = append(parts, g)
	}
	if prop.Ordering != nil {
		g, ok := e.relationGo(*prop.Ordering)
		if !ok {
			return "", false
		}
		parts = append(parts, "!("+g+")")
	}
	return "(func(" + param + " int) bool { out := " + bodySrc + "; return " + strings.Join(parts, " && ") + " })(" + strconv.Itoa(probe) + ")", true
}

// relationGo renders a relation to a Go boolean, the output as the run variable "out" and
// the input parameter as itself, the comparison its grounded symbol.
func (e *Engine) relationGo(r Relation) (string, bool) {
	left, lok := e.ToCode(substitute(r.Left, map[string]Computation{ResultVar: {Var: "out"}}))
	right, rok := e.ToCode(substitute(r.Right, map[string]Computation{ResultVar: {Var: "out"}}))
	if !lok || !rok {
		return "", false
	}
	return left + " " + r.Cmp + " " + right, true
}
