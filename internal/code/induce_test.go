package code

import (
	"go/ast"
	"go/parser"
	"go/token"
	"strconv"
	"testing"
)

// arith evaluates an integer arithmetic expression, nesting and all, standing in
// for the runtime so the induction search can be tested without one. It reads the
// expression with Go's own parser and folds it, so a composed candidate like
// "(number * number) * number" grounds the way it would when run.
func arith(src string) (string, bool) {
	expr, err := parser.ParseExpr(src)
	if err != nil {
		return "", false
	}
	v, ok := evalInt(expr)
	if !ok {
		return "", false
	}
	return strconv.Itoa(v), true
}

func evalInt(e ast.Expr) (int, bool) {
	switch x := e.(type) {
	case *ast.BasicLit:
		v, err := strconv.Atoi(x.Value)
		return v, err == nil
	case *ast.ParenExpr:
		return evalInt(x.X)
	case *ast.UnaryExpr:
		v, ok := evalInt(x.X)
		switch {
		case !ok:
			return 0, false
		case x.Op == token.SUB:
			return -v, true
		case x.Op == token.ADD:
			return v, true
		}
		return 0, false
	case *ast.BinaryExpr:
		a, aok := evalInt(x.X)
		b, bok := evalInt(x.Y)
		if !aok || !bok {
			return 0, false
		}
		switch x.Op {
		case token.ADD:
			return a + b, true
		case token.SUB:
			return a - b, true
		case token.MUL:
			return a * b, true
		case token.QUO:
			if b == 0 {
				return 0, false
			}
			return a / b, true
		}
	}
	return 0, false
}

// A function shown by example reads to its name and pairs; a told definition and
// a single example both decline.
func TestComprehendExamples(t *testing.T) {
	e := testEngine()
	name, ins, outs, ok := e.ComprehendExamples("the double of 3 is 6 and the double of 5 is 10")
	if !ok || name != "double" || len(ins) != 2 || ins[1] != "5" || outs[0] != "6" {
		t.Fatalf("examples should read to double over (3,6),(5,10): %q %v %v ok %v", name, ins, outs, ok)
	}
	if _, _, _, ok := e.ComprehendExamples("the double of a number is the product of the number and 2"); ok {
		t.Fatal("a told definition should not parse as examples")
	}
	if _, _, _, ok := e.ComprehendExamples("the double of 3 is 6"); ok {
		t.Fatal("one pair is too few to fix a rule")
	}
}

// The search assembles an expression that carries the examples, decided by
// running: double, square, a successor, and the two-step n*n + 1 and cube all
// emerge by composing the operators and grounding. The found rule is checked on a
// held-out input, not by its exact shape, since expressions that agree on the
// examples are interchangeable.
func TestSynthesizesExpressions(t *testing.T) {
	e := testEngine()
	cases := []struct {
		ins, outs   []string
		probe, want string
	}{
		{[]string{"3", "5"}, []string{"6", "10"}, "7", "14"},             // double
		{[]string{"3", "4"}, []string{"9", "16"}, "5", "25"},             // square
		{[]string{"3", "10"}, []string{"4", "11"}, "8", "9"},             // successor
		{[]string{"1", "2", "3"}, []string{"2", "5", "10"}, "4", "17"},   // n*n + 1
		{[]string{"2", "3", "5"}, []string{"8", "27", "125"}, "4", "64"}, // cube
	}
	for _, tc := range cases {
		body, ok := e.SynthesizeExpr("number", tc.ins, tc.outs, arith)
		if !ok {
			t.Fatalf("a rule for %v -> %v should be synthesized", tc.ins, tc.outs)
		}
		if got := evalBody(t, e, body, tc.probe); got != tc.want {
			t.Fatalf("the rule for %v should give %s at %s, got %s", tc.ins, tc.want, tc.probe, got)
		}
	}
}

// A rule beyond the search's reach declines rather than forcing a wrong guess:
// 2^n is no bounded arithmetic expression over the parameter, and the examples
// constrain it enough that nothing within the bound fits.
func TestSynthesizeDeclinesBeyondReach(t *testing.T) {
	e := testEngine()
	if _, ok := e.SynthesizeExpr("number", []string{"1", "2", "3", "4"}, []string{"2", "4", "8", "16"}, arith); ok {
		t.Fatal("an exponential is not a bounded arithmetic expression and should decline")
	}
}

// evalBody runs a synthesized body on a probe input and returns its value, so a
// rule is checked by what it computes rather than how it is written.
func evalBody(t *testing.T, e *Engine, body Computation, in string) string {
	t.Helper()
	src, ok := e.ToCode(substitute(body, map[string]Computation{"number": {Lit: in}}))
	if !ok {
		t.Fatalf("the body should render: %+v", body)
	}
	v, ok := arith(src)
	if !ok {
		t.Fatalf("the body should evaluate: %q", src)
	}
	return v
}

// Recursion is searched only with examples enough to pin it: two pairs are too
// few, so the runner is never even reached.
func TestInduceRecursiveNeedsExamples(t *testing.T) {
	e := listEngine() // carries subtract, so the step toward the base exists
	called := func(string) (string, bool) {
		t.Fatal("the runner should not run with too few examples")
		return "", false
	}
	if _, ok := e.InduceRecursive("f", "number", []string{"1", "2"}, []string{"1", "2"}, called); ok {
		t.Fatal("recursion should not be induced from two pairs")
	}
}
