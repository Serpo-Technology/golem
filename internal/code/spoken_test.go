package code

import "testing"

// A computation spoken in operator words reads to the same source the symbols
// would, the pseudocode register of the operations.
func TestComprehendSpokenOperators(t *testing.T) {
	e := testEngine()
	cases := map[string]string{
		"6 plus 7":   "6 + 7",
		"3 times 4":  "3 * 4",
		"20 minus 8": "20 - 8",
	}
	for in, want := range cases {
		c, ok := e.Comprehend(in)
		if !ok {
			t.Fatalf("%q should comprehend", in)
		}
		if src, ok := e.ToCode(c); !ok || src != want {
			t.Fatalf("Comprehend(%q) generates %q, want %q", in, src, want)
		}
	}
}

// Operator words go through the Go parser, so precedence holds: "2 plus 3 times
// 4" is 2 + (3 * 4), the multiply bound before the add.
func TestSpokenOperatorsRespectPrecedence(t *testing.T) {
	e := testEngine()
	c, ok := e.Comprehend("2 plus 3 times 4")
	if !ok || c.Op != "add" || c.Args[1].Op != "multiply" {
		t.Fatalf("spoken precedence should bind times before plus: %+v ok %v", c, ok)
	}
}

// With no operator word present, the spoken path declines and leaves a plain
// expression to the code path, so nothing about reading symbols changes.
func TestSpokenDeclinesPlainExpression(t *testing.T) {
	e := testEngine()
	if _, ok := e.comprehendSpoken("6 + 7"); ok {
		t.Fatal("a symbol expression carries no operator word for the spoken path")
	}
	if c, ok := e.Comprehend("6 + 7"); !ok || c.Op != "add" {
		t.Fatalf("the code path should still read the symbols: %+v ok %v", c, ok)
	}
}
