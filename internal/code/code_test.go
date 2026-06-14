package code

import "testing"

func testEngine() *Engine {
	return NewEngine([]Op{
		{"add", "+", "sum"},
		{"subtract", "-", "difference"},
		{"multiply", "*", "product"},
	})
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
