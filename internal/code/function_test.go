package code

import "testing"

// doubleEngine carries the primitives and one function reloaded from its stored
// body, the state a brain boots into after a function has been taught.
func doubleEngine() *Engine {
	return NewEngine([]Op{
		{Name: "add", Symbol: "+", Word: "sum"},
		{Name: "subtract", Symbol: "-", Word: "difference"},
		{Name: "multiply", Symbol: "*", Word: "product"},
	}, []Def{{Name: "double", Params: []string{"number"}, Body: "number * 2"}})
}

// A function taught in language comprehends to the operation it defines: the
// name, the parameter, and the body as a computation over that parameter.
func TestComprehendDefinition(t *testing.T) {
	e := testEngine()
	op, ok := e.ComprehendDefinition("the double of a number is the product of the number and 2")
	if !ok {
		t.Fatal("the definition should comprehend")
	}
	if op.Name != "double" || op.Word != "double" || len(op.Params) != 1 || op.Params[0] != "number" {
		t.Fatalf("the operation should be double(number): %+v", op)
	}
	if op.Body == nil || op.Body.Op != "multiply" || op.Body.Args[0].Var != "number" || op.Body.Args[1].Lit != "2" {
		t.Fatalf("the body should be multiply(number, 2): %+v", op.Body)
	}
}

// A defined function is called where a number would sit, and the call inlines to
// the source its body computes, so it runs in the runtime with no declaration to
// carry. A different argument inlines the same way, which is how the function
// generalizes.
func TestDefinedCallInlinesAndGeneralizes(t *testing.T) {
	e := doubleEngine()
	call, ok := e.Comprehend("the double of 6")
	if !ok || call.Op != "double" || call.Args[0].Lit != "6" {
		t.Fatalf("the call should comprehend to double(6): %+v ok %v", call, ok)
	}
	if src, ok := e.ToCode(call); !ok || src != "6 * 2" {
		t.Fatalf("the call should inline to source: %q ok %v", src, ok)
	}
	novel, _ := e.Comprehend("the double of 100")
	if src, ok := e.ToCode(novel); !ok || src != "100 * 2" {
		t.Fatalf("a number never seen should inline the same way: %q ok %v", src, ok)
	}
}

// A defined function nests where a number would: it is an operand of an outer
// operation, parenthesized so the inlined body keeps the call's grouping.
func TestDefinedCallNests(t *testing.T) {
	e := doubleEngine()
	c, ok := e.Comprehend("the sum of the double of 6 and 1")
	if !ok {
		t.Fatalf("a call as an operand should comprehend: %+v", c)
	}
	if src, ok := e.ToCode(c); !ok || src != "(6 * 2) + 1" {
		t.Fatalf("the call operand should inline and parenthesize: %q ok %v", src, ok)
	}
}

// The same body writes the Go function it means, the meaning pole run to the
// source a person would read.
func TestToFuncWritesDeclaration(t *testing.T) {
	e := doubleEngine()
	src, ok := e.ToFunc("double")
	if !ok || src != "func double(number int) int { return number * 2 }" {
		t.Fatalf("double should write its declaration: %q ok %v", src, ok)
	}
	if _, ok := e.ToFunc("add"); ok {
		t.Fatal("a primitive has no body to write")
	}
}

// The loop closes through storage: a function comprehended in language reads
// back into the parameter and body it persists as, and reloading from those
// rebuilds an engine that comprehends and runs the call.
func TestDefSourceRoundTrip(t *testing.T) {
	e := testEngine()
	op, ok := e.ComprehendDefinition("the double of a number is the product of the number and 2")
	if !ok {
		t.Fatal("the definition should comprehend")
	}
	params, body, ok := e.DefSource(op)
	if !ok || len(params) != 1 || params[0] != "number" || body != "number * 2" {
		t.Fatalf("DefSource should read the stored surface: %v %q ok %v", params, body, ok)
	}
	reloaded := NewEngine([]Op{
		{Name: "multiply", Symbol: "*", Word: "product"},
	}, []Def{{Name: op.Name, Params: params, Body: body}})
	call, ok := reloaded.Comprehend("the double of 9")
	if !ok {
		t.Fatal("the reloaded function should comprehend a call")
	}
	if src, ok := reloaded.ToCode(call); !ok || src != "9 * 2" {
		t.Fatalf("the reloaded call should inline: %q ok %v", src, ok)
	}
}

// A function defined over another inlines through it, so a composition runs to
// the same source as if it had been written flat.
func TestDefinedFunctionComposes(t *testing.T) {
	e := doubleEngine()
	op, ok := e.ComprehendDefinition("the quadruple of a number is the double of the double of the number")
	if !ok {
		t.Fatal("quadruple should comprehend in terms of double")
	}
	e.byWord[op.Word] = op
	e.byName[op.Name] = op
	call, _ := e.Comprehend("the quadruple of 3")
	if src, ok := e.ToCode(call); !ok || src != "(3 * 2) * 2" {
		t.Fatalf("quadruple should inline through double: %q ok %v", src, ok)
	}
}

// A definition that never uses its parameter is a constant, not a function, and
// one that would shadow a seeded primitive is refused, so the inventory stays
// sound.
func TestDefinitionRejectsConstantAndShadow(t *testing.T) {
	e := testEngine()
	if _, ok := e.ComprehendDefinition("the double of a number is the sum of 6 and 7"); ok {
		t.Fatal("a body that ignores the parameter is not a function of it")
	}
	if _, ok := e.ComprehendDefinition("the sum of a number is the product of the number and 2"); ok {
		t.Fatal("a function may not shadow a seeded primitive's word")
	}
}

// A binary function taught in language comprehends to an operation over two
// parameters, the n-ary generalization of the unary definition.
func TestComprehendBinaryDefinition(t *testing.T) {
	e := testEngine()
	op, ok := e.ComprehendDefinition("the blend of a number and a factor is the product of the number and the factor")
	if !ok {
		t.Fatal("a binary definition should comprehend")
	}
	if op.Name != "blend" || len(op.Params) != 2 || op.Params[0] != "number" || op.Params[1] != "factor" {
		t.Fatalf("the operation should be blend(number, factor): %+v", op)
	}
	if op.Body == nil || op.Body.Op != "multiply" || op.Body.Args[0].Var != "number" || op.Body.Args[1].Var != "factor" {
		t.Fatalf("the body should be multiply(number, factor): %+v", op.Body)
	}
}

// A binary function is called with two operands, inlines both into the source it
// computes, and writes a two-parameter declaration.
func TestBinaryCallAndDeclaration(t *testing.T) {
	e := NewEngine([]Op{{Name: "multiply", Symbol: "*", Word: "product"}},
		[]Def{{Name: "blend", Params: []string{"number", "factor"}, Body: "number * factor"}})
	call, ok := e.Comprehend("the blend of 6 and 7")
	if !ok || call.Op != "blend" || len(call.Args) != 2 || call.Args[0].Lit != "6" || call.Args[1].Lit != "7" {
		t.Fatalf("the call should comprehend to blend(6, 7): %+v ok %v", call, ok)
	}
	if src, ok := e.ToCode(call); !ok || src != "6 * 7" {
		t.Fatalf("the binary call should inline both arguments: %q ok %v", src, ok)
	}
	if src, ok := e.ToFunc("blend"); !ok || src != "func blend(number int, factor int) int { return number * factor }" {
		t.Fatalf("blend should write a two-parameter declaration: %q ok %v", src, ok)
	}
}

// The binary loop closes through storage: a two-parameter definition reads back
// into its parameters and body, and reloading rebuilds an engine that runs the
// call.
func TestBinaryDefSourceRoundTrip(t *testing.T) {
	e := testEngine()
	op, ok := e.ComprehendDefinition("the blend of a number and a factor is the product of the number and the factor")
	if !ok {
		t.Fatal("the binary definition should comprehend")
	}
	params, body, ok := e.DefSource(op)
	if !ok || len(params) != 2 || params[0] != "number" || params[1] != "factor" || body != "number * factor" {
		t.Fatalf("DefSource should read both parameters: %v %q ok %v", params, body, ok)
	}
	reloaded := NewEngine([]Op{{Name: "multiply", Symbol: "*", Word: "product"}},
		[]Def{{Name: op.Name, Params: params, Body: body}})
	call, _ := reloaded.Comprehend("the blend of 9 and 4")
	if src, ok := reloaded.ToCode(call); !ok || src != "9 * 4" {
		t.Fatalf("the reloaded binary call should inline: %q ok %v", src, ok)
	}
}

// Each declared parameter must be used: a binary definition whose body ignores
// one is the n-ary form of refusing a constant.
func TestBinaryDefinitionRejectsDeadParameter(t *testing.T) {
	e := testEngine()
	if _, ok := e.ComprehendDefinition("the pick of a number and a factor is the product of the number and 2"); ok {
		t.Fatal("a definition that ignores a parameter should be rejected")
	}
}
