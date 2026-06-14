package code

import (
	"strconv"
	"strings"
	"testing"
)

// arith evaluates a single "<a> <op> <b>" expression, standing in for the
// runtime so the induction search can be tested without one.
func arith(src string) (string, bool) {
	f := strings.Fields(src)
	if len(f) != 3 {
		return "", false
	}
	a, e1 := strconv.Atoi(f[0])
	b, e2 := strconv.Atoi(f[2])
	if e1 != nil || e2 != nil {
		return "", false
	}
	switch f[1] {
	case "+":
		return strconv.Itoa(a + b), true
	case "-":
		return strconv.Itoa(a - b), true
	case "*":
		return strconv.Itoa(a * b), true
	case "/":
		if b == 0 {
			return "", false
		}
		return strconv.Itoa(a / b), true
	}
	return "", false
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

// An offset rule is found too, and pairs no one-step rule fits decline rather
// than forcing a wrong guess.
func TestInduceBodyOffsetAndMiss(t *testing.T) {
	e := testEngine()
	body, ok := e.InduceBody("number", []string{"3", "10"}, []string{"4", "11"}, arith)
	if !ok {
		t.Fatal("successor should be inferred")
	}
	if src, _ := e.ToCode(body); src != "number + 1" {
		t.Fatalf("successor should read as plus one: %q", src)
	}
	// n*n + 1 is no one-step rule over the primitives, so the search declines
	// rather than forcing a wrong guess.
	if _, ok := e.InduceBody("number", []string{"1", "2", "3"}, []string{"2", "5", "10"}, arith); ok {
		t.Fatal("a rule beyond one step should not be forced to fit")
	}
}
