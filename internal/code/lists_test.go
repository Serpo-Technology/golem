package code

import "testing"

// listEngine carries the arithmetic primitives and the list reductions. The
// reductions are no longer floor; a brain learns them, so a test engine seeds
// them as the constructions a learned brain reloads from.
func listEngine() *Engine {
	return NewEngine([]Op{
		{Name: "add", Symbol: "+", Word: "sum"},
		{Name: "subtract", Symbol: "-", Word: "difference"},
		{Name: "multiply", Symbol: "*", Word: "product"},
		{Name: "divide", Symbol: "/", Word: "quotient"},
	}, reductionDefs())
}

// reductionDefs are the four list reductions as the stored constructions a brain
// carries once taught, the bodies the fold synthesizer produces, so a test that
// uses a reduction seeds it the way a learned brain holds it.
func reductionDefs() []Def {
	raw := func(name, body string) Def {
		return Def{Name: name, Params: []string{"numbers"}, ParamTypes: []string{typeList}, Body: rawDefPrefix + typeInt + " " + body}
	}
	return []Def{
		raw("total", "a := 0; for _, x := range numbers { a = a + x }; return a"),
		raw("length", "return len(numbers)"),
		raw("first", "return numbers[0]"),
		raw("largest", "a := numbers[0]; for _, x := range numbers { if x > a { a = x } }; return a"),
	}
}

const avgDef = "the average of a list of numbers is the quotient of the total of the numbers and the length of the numbers"

// A function over a list declares its parameter with a named type, "a list of
// numbers", and refers to it by the noun in the body.
func TestComprehendListParameter(t *testing.T) {
	e := listEngine()
	op, ok := e.ComprehendDefinition(avgDef)
	if !ok {
		t.Fatal("a list-parameter definition should comprehend")
	}
	if len(op.Params) != 1 || op.Params[0] != "numbers" || op.paramType(0) != typeList {
		t.Fatalf("the parameter should be a list named numbers: %+v", op)
	}
}

// A list function writes the Go that takes a slice and reduces it.
func TestListFunctionToFunc(t *testing.T) {
	e := listEngine()
	op, ok := e.ComprehendDefinition(avgDef)
	if !ok {
		t.Fatal("average should comprehend")
	}
	e.index(op)
	want := "func average(numbers []int) int { return (func(numbers []int) int { a := 0; for _, x := range numbers { a = a + x }; return a })(numbers) / (func(numbers []int) int { return len(numbers) })(numbers) }"
	if src, ok := e.ToFunc("average"); !ok || src != want {
		t.Fatalf("average should write a slice-taking function:\n got %q\nwant %q", src, want)
	}
}

// A definition that uses a list where a number is expected is refused by the type
// check, so a function that would run to broken Go is not learned.
func TestListFunctionTypeErrorRejected(t *testing.T) {
	e := listEngine()
	if _, ok := e.ComprehendDefinition("the bad of a list of numbers is the sum of the numbers and 1"); ok {
		t.Fatal("adding a list to a number should be refused")
	}
}

// The list-function loop closes through storage: the body keeps the collection
// operations as calls and the parameter keeps its type, and reloading rebuilds an
// engine that comprehends the call.
func TestListFunctionRoundTrip(t *testing.T) {
	e := listEngine()
	op, ok := e.ComprehendDefinition(avgDef)
	if !ok {
		t.Fatal("average should comprehend")
	}
	_, body, ok := e.DefSource(op)
	if !ok || body != "total(numbers) / length(numbers)" {
		t.Fatalf("DefSource should keep the list reductions as calls: %q", body)
	}
	reloaded := NewEngine([]Op{{Name: "divide", Symbol: "/", Word: "quotient"}},
		[]Def{{Name: op.Name, Params: []string{"numbers"}, ParamTypes: []string{typeList}, Body: body}})
	if reloaded.byName["average"].paramType(0) != typeList {
		t.Fatal("the reloaded average should keep its list parameter")
	}
	if _, ok := reloaded.Comprehend("the average of the list of 2 and 4 and 6"); !ok {
		t.Fatal("the reloaded function should comprehend a call")
	}
}

// A list literal comprehends to a list value over its elements, however many.
func TestComprehendListLiteral(t *testing.T) {
	e := listEngine()
	c, ok := e.Comprehend("the list of 1 and 2 and 3")
	if !ok || c.Op != "list" || len(c.Args) != 3 || c.Args[0].Lit != "1" || c.Args[2].Lit != "3" {
		t.Fatalf("a list literal should read its elements: %+v ok %v", c, ok)
	}
}

// The seeded list reductions generate the Go that runs them, each the loop it
// means applied to the list: a fold for the total, len for the length, an index
// for the first.
func TestListOpCode(t *testing.T) {
	e := listEngine()
	cases := []struct{ surface, want string }{
		{"the total of the list of 1 and 2 and 3", "(func(numbers []int) int { a := 0; for _, x := range numbers { a = a + x }; return a })([]int{1, 2, 3})"},
		{"the length of the list of 1 and 2 and 3", "(func(numbers []int) int { return len(numbers) })([]int{1, 2, 3})"},
		{"the first of the list of 4 and 5 and 6", "(func(numbers []int) int { return numbers[0] })([]int{4, 5, 6})"},
	}
	for _, tc := range cases {
		c, ok := e.Comprehend(tc.surface)
		if !ok {
			t.Fatalf("%q should comprehend", tc.surface)
		}
		if src, ok := e.ToCode(c); !ok || src != tc.want {
			t.Fatalf("%q ->\n got %q\nwant %q", tc.surface, src, tc.want)
		}
	}
}

// A list reduction given a number instead of a list does not type-check, so it is
// refused rather than run to a broken program; given a list it does.
func TestListTypeCheck(t *testing.T) {
	e := listEngine()
	bad, ok := e.Comprehend("the total of 5")
	if !ok {
		t.Fatal("the surface comprehends; the type check is what refuses it")
	}
	if e.TypeCheck(bad) {
		t.Fatal("the total of a number should not type-check")
	}
	good, _ := e.Comprehend("the total of the list of 1 and 2 and 3")
	if !e.TypeCheck(good) {
		t.Fatal("the total of a list should type-check")
	}
}
