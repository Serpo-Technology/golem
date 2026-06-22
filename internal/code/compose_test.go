package code

import "testing"

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
	})
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
