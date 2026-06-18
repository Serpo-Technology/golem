package code

import "testing"

// factEngine carries the primitives and the recursive "factorial" reloaded from
// its stored body, the state a brain boots into after the function is taught.
func factEngine() *Engine {
	return NewEngine([]Op{
		{Name: "subtract", Symbol: "-", Word: "difference"},
		{Name: "multiply", Symbol: "*", Word: "product"},
	}, []Def{{Name: "factorial", Params: []string{"number"},
		Body: "if < ; number ; 1 ; 1 ; number * factorial(number - 1)"}})
}

const factDef = "the factorial of a number is 1 if the number is below 1 else the product of the number and the factorial of the difference of the number and 1"

// A recursive definition comprehends with a call to itself in its body: the
// function is in scope as the body is read, so the self-call resolves before the
// function is in the inventory.
func TestComprehendRecursiveDefinition(t *testing.T) {
	e := testEngine()
	op, ok := e.ComprehendDefinition(factDef)
	if !ok {
		t.Fatal("a recursive definition should comprehend")
	}
	if !e.recursive(op) {
		t.Fatalf("factorial should be recognized as recursive: %+v", op.Body)
	}
}

// A recursive function writes the Go that calls itself by name.
func TestRecursiveToFunc(t *testing.T) {
	e := factEngine()
	src, ok := e.ToFunc("factorial")
	want := "func factorial(number int) int { if number < 1 { return 1 }; return number * factorial(number - 1) }"
	if !ok || src != want {
		t.Fatalf("factorial should write a self-calling declaration:\n got %q\nwant %q", src, want)
	}
}

// A call to a recursive function renders as a self-referential local function,
// invoked, a self-contained expression that composes and runs.
func TestRecursiveCallRenders(t *testing.T) {
	e := factEngine()
	call, ok := e.Comprehend("the factorial of 5")
	if !ok {
		t.Fatal("a recursive call should comprehend")
	}
	want := "(func() int { var factorial func(int) int; factorial = func(number int) int { if number < 1 { return 1 }; return number * factorial(number - 1) }; return factorial(5) }())"
	if src, ok := e.ToCode(call); !ok || src != want {
		t.Fatalf("the recursive call should render as a local function:\n got %q\nwant %q", src, want)
	}
}

// The recursive loop closes through storage: the body stores with the self-call
// kept as a call, and reloading rebuilds an engine that reads it back as a call
// and renders it again.
func TestRecursiveRoundTrip(t *testing.T) {
	e := testEngine()
	op, ok := e.ComprehendDefinition(factDef)
	if !ok {
		t.Fatal("the recursive definition should comprehend")
	}
	params, body, ok := e.DefSource(op)
	if !ok || body != "if < ; number ; 1 ; 1 ; number * factorial(number - 1)" {
		t.Fatalf("DefSource should keep the self-call as a call: %q ok %v", body, ok)
	}
	reloaded := NewEngine([]Op{
		{Name: "subtract", Symbol: "-", Word: "difference"},
		{Name: "multiply", Symbol: "*", Word: "product"},
	}, []Def{{Name: op.Name, Params: params, Body: body}})
	if !reloaded.recursive(reloaded.byName["factorial"]) {
		t.Fatal("the reloaded factorial should still be recursive")
	}
	if _, ok := reloaded.Comprehend("the factorial of 6"); !ok {
		t.Fatal("the reloaded function should comprehend a call")
	}
}
