package code

import "testing"

func testEngine() *Engine {
	return NewEngine([]Op{
		{Name: "add", Symbol: "+", Word: "sum"},
		{Name: "subtract", Symbol: "-", Word: "difference"},
		{Name: "multiply", Symbol: "*", Word: "product"},
	}, nil)
}

// The literal teaching surface and the code surface both comprehend to the one
// computation, the form pole run to the meaning either way.
func TestComprehendBothSurfaces(t *testing.T) {
	e := testEngine()
	lit, ok := e.Comprehend("the sum of 6 and 7")
	if !ok || lit.Op != "add" || lit.Args[0].Lit != "6" || lit.Args[1].Lit != "7" {
		t.Fatalf("the literal should comprehend to add(6, 7): %+v ok %v", lit, ok)
	}
	src, ok := e.Comprehend("6 + 7")
	if !ok || src.Op != "add" || src.Args[0].Lit != "6" || src.Args[1].Lit != "7" {
		t.Fatalf("the code should comprehend to add(6, 7): %+v ok %v", src, ok)
	}
}

// Generating from a computation yields code and the teaching surface, the one
// engine run the other way.
func TestGenerateBothSurfaces(t *testing.T) {
	e := testEngine()
	c := Computation{Op: "multiply", Args: []Computation{{Lit: "6"}, {Lit: "7"}}}
	if got, ok := e.ToCode(c); !ok || got != "6 * 7" {
		t.Fatalf("the computation should generate code: %q ok %v", got, ok)
	}
	if got, ok := e.ToLiteral(c); !ok || got != "the product of 6 and 7" {
		t.Fatalf("the computation should generate the teaching surface: %q ok %v", got, ok)
	}
}

// The round trip closes: a computation taught in one surface generates the
// other and comprehends back to the same computation.
func TestRoundTrip(t *testing.T) {
	e := testEngine()
	want, _ := e.Comprehend("the difference of 9 and 4")
	src, ok := e.ToCode(want)
	if !ok || src != "9 - 4" {
		t.Fatalf("should generate code from the comprehended computation: %q", src)
	}
	back, ok := e.Comprehend(src)
	if !ok || back.Op != want.Op || back.Args[0].Lit != want.Args[0].Lit {
		t.Fatalf("the code should comprehend back to the same computation: %+v vs %+v", back, want)
	}
}

// A surface with no construction for it does not comprehend, so the engine
// fails cleanly rather than guessing.
func TestComprehendUnknownFails(t *testing.T) {
	e := testEngine()
	if _, ok := e.Comprehend("the gizmo of 6 and 7"); ok {
		t.Fatal("an unknown operation word should not comprehend")
	}
}

// The code surface is real Go, read by the language's own parser, so a nested,
// parenthesized expression comprehends to the computation its grouping means.
func TestComprehendRealGoExpression(t *testing.T) {
	e := testEngine()
	c, ok := e.Comprehend("6 * (7 + 1)")
	if !ok || c.Op != "multiply" {
		t.Fatalf("the outer operation should be multiply: %+v ok %v", c, ok)
	}
	if c.Args[0].Lit != "6" || c.Args[1].Op != "add" {
		t.Fatalf("the second operand should be the sum (7 + 1): %+v", c)
	}
	if c.Args[1].Args[0].Lit != "7" || c.Args[1].Args[1].Lit != "1" {
		t.Fatalf("the inner sum should be over 7 and 1: %+v", c.Args[1])
	}
}

// Go's own precedence shapes the tree: in "6 + 7 * 1" the product binds first,
// so the outer operation is the sum over 6 and the product.
func TestComprehendRespectsPrecedence(t *testing.T) {
	e := testEngine()
	c, ok := e.Comprehend("6 + 7 * 1")
	if !ok || c.Op != "add" || c.Args[1].Op != "multiply" {
		t.Fatalf("precedence should make this add(6, multiply(7, 1)): %+v ok %v", c, ok)
	}
}

// A signed literal reads as one value, so a negative operand comprehends.
func TestComprehendSignedLiteral(t *testing.T) {
	e := testEngine()
	c, ok := e.Comprehend("6 * -3")
	if !ok || c.Op != "multiply" || c.Args[1].Lit != "-3" {
		t.Fatalf("a signed literal should read as one value: %+v ok %v", c, ok)
	}
}

// A nested computation generates source that carries its grouping in
// parentheses and comprehends back to the same tree, the round trip closing for
// a compound expression as it does for a flat one.
func TestNestedCodeRoundTrip(t *testing.T) {
	e := testEngine()
	c := Computation{Op: "multiply", Args: []Computation{
		{Lit: "6"},
		{Op: "add", Args: []Computation{{Lit: "7"}, {Lit: "1"}}},
	}}
	src, ok := e.ToCode(c)
	if !ok || src != "6 * (7 + 1)" {
		t.Fatalf("the nested computation should parenthesize the compound operand: %q ok %v", src, ok)
	}
	back, ok := e.Comprehend(src)
	if !ok || back.Op != "multiply" || back.Args[1].Op != "add" {
		t.Fatalf("the source should comprehend back to the same tree: %+v", back)
	}
}

// The teaching surface composes: an operand can itself be an operation, so a
// nested literal surface comprehends to the same tree a nested code surface
// does, the meaning pole reading however deep the words go.
func TestComprehendNestedLiteral(t *testing.T) {
	e := testEngine()
	c, ok := e.Comprehend("the sum of the product of 2 and 3 and 4")
	if !ok || c.Op != "add" || c.Args[1].Lit != "4" {
		t.Fatalf("the nested literal should comprehend to add(_, 4): %+v ok %v", c, ok)
	}
	if c.Args[0].Op != "multiply" || c.Args[0].Args[0].Lit != "2" || c.Args[0].Args[1].Lit != "3" {
		t.Fatalf("the first operand should be multiply(2, 3): %+v", c.Args[0])
	}
	// The grouping the words carry is the grouping the source runs: (2 * 3) + 4.
	if src, ok := e.ToCode(c); !ok || src != "(2 * 3) + 4" {
		t.Fatalf("the nested literal should generate grouped source: %q ok %v", src, ok)
	}
}

// A nested literal round-trips through ToLiteral: generating the surface from a
// compound computation and comprehending it back closes on the same tree.
func TestNestedLiteralRoundTrip(t *testing.T) {
	e := testEngine()
	c := Computation{Op: "add", Args: []Computation{
		{Op: "multiply", Args: []Computation{{Lit: "2"}, {Lit: "3"}}},
		{Lit: "4"},
	}}
	surface, ok := e.ToLiteral(c)
	if !ok || surface != "the sum of the product of 2 and 3 and 4" {
		t.Fatalf("the compound computation should generate the nested surface: %q ok %v", surface, ok)
	}
	back, ok := e.Comprehend(surface)
	if !ok || back.Op != "add" || back.Args[0].Op != "multiply" {
		t.Fatalf("the surface should comprehend back to the same tree: %+v", back)
	}
}

// A binary operation handed a third operand is ill-formed: the parse leaves a
// tail it cannot place, so the surface fails rather than silently dropping it.
func TestComprehendRejectsTrailingOperand(t *testing.T) {
	e := testEngine()
	if c, ok := e.Comprehend("the sum of 2 and 3 and 4"); ok {
		t.Fatalf("a third operand to a binary operation should not comprehend: %+v", c)
	}
}
