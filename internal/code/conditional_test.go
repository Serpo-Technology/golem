package code

import "testing"

// largerEngine carries the primitives and the conditional "larger" reloaded from
// its stored body, the state a brain boots into after the function is taught.
func largerEngine() *Engine {
	return NewEngine([]Op{
		{Name: "add", Symbol: "+", Word: "sum"},
		{Name: "multiply", Symbol: "*", Word: "product"},
	}, []Def{{Name: "larger", Params: []string{"number", "count"},
		Body: "if > ; number ; count ; number ; count"}})
}

// A conditional definition comprehends to a choice on a comparison: the larger of
// two numbers is the first when it is above the second, else the second.
func TestComprehendConditionalDefinition(t *testing.T) {
	e := testEngine()
	op, ok := e.ComprehendDefinition("the larger of a number and a count is the number if the number is above the count else the count")
	if !ok {
		t.Fatal("a conditional definition should comprehend")
	}
	if op.Name != "larger" || len(op.Params) != 2 {
		t.Fatalf("larger should take two parameters: %+v", op)
	}
	c := op.Body.Cond
	if c == nil || c.Cmp != ">" || c.Left.Var != "number" || c.Right.Var != "count" || c.Then.Var != "number" || c.Else.Var != "count" {
		t.Fatalf("the body should choose number over count on >: %+v", op.Body)
	}
}

// A bare "is" in the test reads as equality, the comparison a base case turns on
// ("1 if the number is 0 else ..."); "above" and "below" are the ordered ones.
func TestComprehendConditionalEquality(t *testing.T) {
	e := testEngine()
	op, ok := e.ComprehendDefinition("the flag of a number is 1 if the number is 0 else the number")
	if !ok {
		t.Fatal("an equality conditional should comprehend")
	}
	if c := op.Body.Cond; c == nil || c.Cmp != "==" || c.Left.Var != "number" || c.Right.Lit != "0" {
		t.Fatalf("a bare is should read as equality on 0: %+v", op.Body)
	}
}

// The readable function form writes the branch a person reads, the if and the two
// returns, not the function-literal form the engine composes with.
func TestConditionalToFunc(t *testing.T) {
	e := largerEngine()
	src, ok := e.ToFunc("larger")
	if !ok || src != "func larger(number int, count int) int { if number > count { return number }; return count }" {
		t.Fatalf("larger should write the branch form: %q ok %v", src, ok)
	}
}

// The conditional loop closes through storage: a definition reads back into the
// structured body it persists as, and reloading rebuilds an engine whose call
// inlines to the function-literal form that runs as an expression.
func TestConditionalRoundTrip(t *testing.T) {
	e := testEngine()
	op, ok := e.ComprehendDefinition("the larger of a number and a count is the number if the number is above the count else the count")
	if !ok {
		t.Fatal("the conditional definition should comprehend")
	}
	params, body, ok := e.DefSource(op)
	if !ok || body != "if > ; number ; count ; number ; count" {
		t.Fatalf("DefSource should store the structured conditional: %q ok %v", body, ok)
	}
	reloaded := NewEngine([]Op{{Name: "multiply", Symbol: "*", Word: "product"}},
		[]Def{{Name: op.Name, Params: params, Body: body}})
	call, ok := reloaded.Comprehend("the larger of 6 and 4")
	if !ok {
		t.Fatal("the reloaded conditional should comprehend a call")
	}
	if src, ok := reloaded.ToCode(call); !ok || src != "(func() int { if 6 > 4 { return 6 }; return 4 }())" {
		t.Fatalf("the call should inline to the function-literal form: %q ok %v", src, ok)
	}
}

// A comma before "else" is tolerated, the way the construction reads aloud.
func TestConditionalToleratesComma(t *testing.T) {
	e := testEngine()
	if _, ok := e.ComprehendDefinition("the larger of a number and a count is the number if the number is above the count, else the count"); !ok {
		t.Fatal("a comma before else should still comprehend")
	}
}

// A definition that ignores a parameter is still refused when the body is a
// conditional, the comparison and branches all searched for the reference.
func TestConditionalRejectsDeadParameter(t *testing.T) {
	e := testEngine()
	if _, ok := e.ComprehendDefinition("the pick of a number and a count is 1 if the number is above 2 else 3"); ok {
		t.Fatal("a conditional that never uses the count is not a function of it")
	}
}
