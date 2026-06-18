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

// The search infers the rule the examples mean: a constant multiple for double,
// and the parameter against itself for square.
func TestInduceBodyInfersConstantAndSquare(t *testing.T) {
	e := testEngine()
	body, ok := e.InduceBody("number", []string{"3", "5"}, []string{"6", "10"}, arith)
	if !ok {
		t.Fatal("double should be inferred")
	}
	if src, _ := e.ToCode(body); src != "number * 2" {
		t.Fatalf("double should read as times two: %q", src)
	}
	body, ok = e.InduceBody("number", []string{"3", "4"}, []string{"9", "16"}, arith)
	if !ok {
		t.Fatal("square should be inferred")
	}
	if src, _ := e.ToCode(body); src != "number * number" {
		t.Fatalf("square should read as the parameter against itself: %q", src)
	}
}

// An offset rule is found from two pairs; a two-step rule is reached once there
// are pairs enough to constrain it; and a rule beyond two steps declines rather
// than forcing a wrong guess.
func TestInduceBodyOffsetMultiStepAndMiss(t *testing.T) {
	e := testEngine()
	body, ok := e.InduceBody("number", []string{"3", "10"}, []string{"4", "11"}, arith)
	if !ok {
		t.Fatal("successor should be inferred")
	}
	if src, _ := e.ToCode(body); src != "number + 1" {
		t.Fatalf("successor should read as plus one: %q", src)
	}
	// n*n + 1 is two operations: no one-step rule fits, but the composed search
	// finds it now that there are three pairs to ground it on.
	body, ok = e.InduceBody("number", []string{"1", "2", "3"}, []string{"2", "5", "10"}, arith)
	if !ok {
		t.Fatal("n*n + 1 should be inferred as a two-step rule")
	}
	if src, _ := e.ToCode(body); src != "(number * number) + 1" {
		t.Fatalf("n*n + 1 should read as the square plus one: %q", src)
	}
	// 2^n is no composition of two arithmetic operations, so even the wider search
	// declines rather than forcing a wrong guess.
	if _, ok := e.InduceBody("number", []string{"1", "2", "3"}, []string{"2", "4", "8"}, arith); ok {
		t.Fatal("a rule beyond two steps should not be forced to fit")
	}
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

// A two-step rule is found only with enough examples: cube needs three pairs,
// since two points alone could be a coincidence a composed rule overfits.
func TestInduceBodyMultiStepNeedsEnoughExamples(t *testing.T) {
	e := testEngine()
	// Cube over three pairs: the parameter squared, times the parameter again.
	body, ok := e.InduceBody("number", []string{"2", "3", "5"}, []string{"8", "27", "125"}, arith)
	if !ok {
		t.Fatal("cube should be inferred from three pairs")
	}
	if src, _ := e.ToCode(body); src != "(number * number) * number" {
		t.Fatalf("cube should read as the square times the parameter: %q", src)
	}
	// The same rule from only two pairs is withheld: a composition is not grounded
	// on two points.
	if _, ok := e.InduceBody("number", []string{"2", "3"}, []string{"8", "27"}, arith); ok {
		t.Fatal("a two-step rule should not be induced from two pairs")
	}
}
