package code

import (
	"strings"
	"testing"
)

// composingEngine carries the arithmetic primitives and two learned functions, so
// the synthesis search can compose acquired vocabulary, not arithmetic alone.
func composingEngine() *Engine {
	return NewEngine([]Op{
		{Name: "add", Symbol: "+", Word: "sum"},
		{Name: "subtract", Symbol: "-", Word: "difference"},
		{Name: "multiply", Symbol: "*", Word: "product"},
		{Name: "divide", Symbol: "/", Word: "quotient"},
	}, []Def{
		{Name: "square", Params: []string{"number"}, Body: "number * number"},
		{Name: "increment", Params: []string{"number"}, Body: "number + 1"},
		{Name: "double", Params: []string{"number"}, Body: "number * 2"},
	})
}

// The inverse of a known operation is discovered from the property that the
// operation returns its output to the input, not from examples: the decrement is
// the inverse of increment, and halving the inverse of double, the latter's test
// inputs sampled as doubles so the integer inverse stays exact.
func TestSynthesizeInverse(t *testing.T) {
	e := composingEngine()
	dec, ok := e.SynthesizeInverse("number", "increment", arith)
	if !ok {
		t.Fatal("the inverse of increment should be discovered")
	}
	if got := evalBody(t, e, dec, "5"); got != "4" {
		t.Fatalf("the inverse of increment at 5 should be 4, got %s", got)
	}
	half, ok := e.SynthesizeInverse("number", "double", arith)
	if !ok {
		t.Fatal("the inverse of double should be discovered")
	}
	if got := evalBody(t, e, half, "8"); got != "4" {
		t.Fatalf("the inverse of double at 8 should be 4, got %s", got)
	}
}

// An operation with no bounded integer inverse declines rather than forcing a wrong
// rule: the square root is no arithmetic expression over the parameter.
func TestSynthesizeInverseDeclinesNonInvertible(t *testing.T) {
	e := composingEngine()
	if _, ok := e.SynthesizeInverse("number", "square", arith); ok {
		t.Fatal("square has no bounded integer inverse and should decline")
	}
}

// The function being synthesized is left out of its own search, so a re-synthesis of
// an already-learned operation re-finds its rule rather than the trivial call to
// itself it would otherwise compose from the inventory.
func TestSearchOpsExcludesSynthSelf(t *testing.T) {
	e := composingEngine()
	hasSquare := func() bool {
		for _, o := range e.searchOps() {
			if o.Name == "square" {
				return true
			}
		}
		return false
	}
	if !hasSquare() {
		t.Fatal("square should be a search op when nothing is excluded")
	}
	e.SetSynthSelf("square")
	if hasSquare() {
		t.Fatal("the function being synthesized should be left out of its own search")
	}
	e.SetSynthSelf("")
	if !hasSquare() {
		t.Fatal("clearing the exclusion restores the search op")
	}

	// With octic in the inventory but excluded, re-synthesizing it finds the
	// composition again, never octic(number).
	body, ok := e.SynthesizeExpr("number", []string{"1", "2", "3"}, []string{"1", "256", "6561"}, arith)
	if !ok {
		t.Fatal("octic should synthesize")
	}
	e.index(Op{Name: "octic", Word: "octic", Params: []string{"number"}, Body: &body})
	e.SetSynthSelf("octic")
	again, ok := e.SynthesizeExpr("number", []string{"1", "2", "3"}, []string{"1", "256", "6561"}, arith)
	e.SetSynthSelf("")
	if !ok {
		t.Fatal("octic should re-synthesize with itself excluded")
	}
	if references(again, "octic") {
		t.Fatalf("a re-synthesis must not define a function as itself: %+v", again)
	}
}

// A function that composes learned vocabulary is written and stored in those terms,
// not flattened to arithmetic, and reloads as the composition while still running
// to the right value (the runnable form inlines it).
func TestComposedFunctionPreservesVocabulary(t *testing.T) {
	e := composingEngine()
	body, ok := e.SynthesizeExpr("number", []string{"1", "2", "3"}, []string{"1", "256", "6561"}, arith)
	if !ok {
		t.Fatal("the eighth power should synthesize by composing square")
	}
	op := Op{Name: "octic", Word: "octic", Params: []string{"number"}, Body: &body}
	e.index(op)
	src, ok := e.ToFunc("octic")
	if !ok || !strings.Contains(src, "square(") {
		t.Fatalf("octic should be written in terms of square: %q", src)
	}
	_, stored, ok := e.DefSource(op)
	if !ok || !strings.Contains(stored, "square(") {
		t.Fatalf("octic should store as a composition of square: %q", stored)
	}
	// The stored composition reloads and still runs to the right value.
	reloaded := NewEngine([]Op{{Name: "multiply", Symbol: "*", Word: "product"}},
		[]Def{
			{Name: "square", Params: []string{"number"}, Body: "number * number"},
			{Name: "octic", Params: []string{"number"}, Body: stored},
		})
	call, _ := reloaded.Comprehend("the octic of 2")
	got, ok := reloaded.ToCode(call)
	if !ok {
		t.Fatal("the reloaded octic should render to runnable code")
	}
	if v, vok := arith(got); !vok || v != "256" {
		t.Fatalf("the reloaded octic of 2 should be 256: %q -> %q", got, v)
	}
}

// The search composes a learned operation to reach a rule beyond the primitive
// floor's depth: with square learned, the eighth power is square(square(square(n))),
// three composed steps, where the eighth power as an arithmetic expression needs
// more operators than the search's bound. The rule is checked by what it computes,
// since equivalent expressions are interchangeable.
func TestSynthesizeComposesLearnedOp(t *testing.T) {
	e := composingEngine()
	body, ok := e.SynthesizeExpr("number", []string{"1", "2", "3"}, []string{"1", "256", "6561"}, arith)
	if !ok {
		t.Fatal("the eighth power should be reachable by composing the learned square")
	}
	for in, want := range map[string]string{"1": "1", "2": "256", "4": "65536"} {
		if got := evalBody(t, e, body, in); got != want {
			t.Fatalf("the composed rule should give %s at %s, got %s", want, in, got)
		}
	}
}

// The same rule is beyond the primitive search's bound, so without the learned
// operation it declines: it is the composition that extends the reach.
func TestSynthesizeWithoutLearnedOpDeclines(t *testing.T) {
	e := testEngine() // arithmetic primitives only
	if _, ok := e.SynthesizeExpr("number", []string{"1", "2", "3"}, []string{"1", "256", "6561"}, arith); ok {
		t.Fatal("the eighth power is beyond the primitive search's bound and should decline")
	}
}

// An arity-1 learned operation enters the search and composes with the arithmetic:
// with increment learned, n+2 reads as increment(increment(n)).
func TestSynthesizeComposesUnaryLearnedOp(t *testing.T) {
	e := composingEngine()
	body, ok := e.SynthesizeExpr("number", []string{"5", "10"}, []string{"7", "12"}, arith)
	if !ok {
		t.Fatal("a successor-by-two rule should synthesize")
	}
	if got := evalBody(t, e, body, "3"); got != "5" {
		t.Fatalf("the rule should give 5 at 3, got %s", got)
	}
}

// searchOps admits the composable integer functions and leaves out a recursive one,
// so inlining a learned op in the prefilter terminates, and a primitive is always
// present.
func TestSearchOpsExcludesRecursive(t *testing.T) {
	e := NewEngine([]Op{
		{Name: "multiply", Symbol: "*", Word: "product"},
		{Name: "subtract", Symbol: "-", Word: "difference"},
	}, []Def{
		{Name: "square", Params: []string{"number"}, Body: "number * number"},
		{Name: "factorial", Params: []string{"number"}, Body: "if < ; number ; 1 ; 1 ; number * factorial(number - 1)"},
	})
	has := map[string]bool{}
	for _, o := range e.searchOps() {
		has[o.Name] = true
	}
	if !has["multiply"] {
		t.Error("a primitive should be among the search ops")
	}
	if !has["square"] {
		t.Error("a composable integer function should be among the search ops")
	}
	if has["factorial"] {
		t.Error("a recursive function should be left out of the search ops")
	}
}
