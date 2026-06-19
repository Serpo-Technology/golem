package code

import "testing"

// The expression enumerator the loop body draws on reaches the updates a fold
// needs: the accumulator plus the element, plus the element squared, plus one.
func TestGenExprsReachesFoldUpdates(t *testing.T) {
	e := listEngine()
	set := map[string]bool{}
	for _, c := range e.genExprs([]string{"a", "x"}, []string{"0", "1", "2"}, loopMaxOps) {
		if s, ok := e.ToCode(c); ok {
			set[s] = true
		}
	}
	for _, want := range []string{"a + x", "a + (x * x)", "a + 1"} {
		if !set[want] {
			t.Fatalf("the enumerator should reach %q", want)
		}
	}
}

// A fold update interpreted over a list carries it to the value, and the loop it
// means writes the Go a runtime would run. The full search, with grounding, is
// exercised end to end in the daemon against the real runtime.
func TestFoldInterpretsAndRenders(t *testing.T) {
	e := listEngine()
	sum := Computation{Op: "add", Args: []Computation{{Var: "a"}, {Var: "x"}}}
	if !foldInterprets(sum, "a", 0, [][]int{{1, 2, 3}, {2, 4}}, []int{6, 6}) {
		t.Fatal("a + x should fold to the total")
	}
	if foldInterprets(sum, "a", 1, [][]int{{1, 2, 3}}, []int{6}) {
		t.Fatal("the wrong init should not fold to the total")
	}
	body, ok := e.foldBody("numbers", "a", 0, sum)
	want := "a := 0; for _, x := range numbers { a = a + x }; return a"
	if !ok || body != want {
		t.Fatalf("the fold should render its loop:\n got %q\nwant %q", body, want)
	}
}
