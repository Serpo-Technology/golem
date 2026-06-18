package code

import "testing"

// listEngine carries a couple of primitives; the built-in collection operations
// are the floor every engine boots with, so they need no seeding here.
func listEngine() *Engine {
	return NewEngine([]Op{
		{Name: "add", Symbol: "+", Word: "sum"},
		{Name: "multiply", Symbol: "*", Word: "product"},
	}, nil)
}

// A list literal comprehends to a list value over its elements, however many.
func TestComprehendListLiteral(t *testing.T) {
	e := listEngine()
	c, ok := e.Comprehend("the list of 1 and 2 and 3")
	if !ok || c.Op != "list" || len(c.Args) != 3 || c.Args[0].Lit != "1" || c.Args[2].Lit != "3" {
		t.Fatalf("a list literal should read its elements: %+v ok %v", c, ok)
	}
}

// The built-in collection operations generate the Go that runs them: a fold for
// the total, len for the length, an index for the first.
func TestListOpCode(t *testing.T) {
	e := listEngine()
	cases := []struct{ surface, want string }{
		{"the total of the list of 1 and 2 and 3", "(func() int { s := 0; for _, v := range []int{1, 2, 3} { s += v }; return s }())"},
		{"the length of the list of 1 and 2 and 3", "len([]int{1, 2, 3})"},
		{"the first of the list of 4 and 5 and 6", "(func() int { xs := []int{4, 5, 6}; return xs[0] }())"},
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

// A collection operation given a number instead of a list does not type-check, so
// it is refused rather than run to a broken program; given a list it does.
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
