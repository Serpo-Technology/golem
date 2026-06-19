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
	sum := foldUpdate{expr: Computation{Op: "add", Args: []Computation{{Var: "a"}, {Var: "x"}}}}
	if !foldInterprets(sum, "a", foldInit{lit: "0"}, [][]int{{1, 2, 3}, {2, 4}}, []int{6, 6}) {
		t.Fatal("a + x should fold to the total")
	}
	if foldInterprets(sum, "a", foldInit{lit: "1"}, [][]int{{1, 2, 3}}, []int{6}) {
		t.Fatal("the wrong init should not fold to the total")
	}
	body, ok := e.foldBody("numbers", "a", foldInit{lit: "0"}, sum)
	want := "a := 0; for _, x := range numbers { a = a + x }; return a"
	if !ok || body != want {
		t.Fatalf("the fold should render its loop:\n got %q\nwant %q", body, want)
	}
}

// The fold search reaches the running maximum and the first element: a guarded
// update from the first element keeps the max, and an unchanging accumulator from
// the first element reads as the first element itself.
func TestFoldFindsFirstAndMax(t *testing.T) {
	e := listEngine()
	max := foldUpdate{expr: Computation{Var: "x"}, guard: ">"}
	if !foldInterprets(max, "a", foldInit{first: true}, [][]int{{1, 5, 2}, {4, 1}}, []int{5, 4}) {
		t.Fatal("a guarded update from the first element should fold to the max")
	}
	body, ok := e.foldBody("numbers", "a", foldInit{first: true}, max)
	wantMax := "a := numbers[0]; for _, x := range numbers { if x > a { a = x } }; return a"
	if !ok || body != wantMax {
		t.Fatalf("the max fold should render its guarded loop:\n got %q\nwant %q", body, wantMax)
	}
	first := foldUpdate{expr: Computation{Var: "a"}}
	if !foldInterprets(first, "a", foldInit{first: true}, [][]int{{1, 2, 3}, {4, 5}}, []int{1, 4}) {
		t.Fatal("an unchanging accumulator from the first element should fold to the first")
	}
	body, ok = e.foldBody("numbers", "a", foldInit{first: true}, first)
	if !ok || body != "return numbers[0]" {
		t.Fatalf("an unchanging fold from the first element should read as the first: %q", body)
	}
}

// A list-returning candidate interpreted over the examples carries them, and the
// loop it means writes the Go a runtime would run: a map builds the transform of
// each element, a filter keeps the elements a comparison holds for.
func TestListReturnInterpretsAndRenders(t *testing.T) {
	e := listEngine()
	dbl := Computation{Op: "add", Args: []Computation{{Var: "x"}, {Var: "x"}}}
	if !mapInterprets(dbl, [][]int{{1, 2, 3}}, [][]int{{2, 4, 6}}) {
		t.Fatal("x + x should map to the doubles")
	}
	body, ok := e.mapBody("numbers", dbl)
	wantMap := "b := []int{}; for _, x := range numbers { b = append(b, x + x) }; return b"
	if !ok || body != wantMap {
		t.Fatalf("the map should render its loop:\n got %q\nwant %q", body, wantMap)
	}
	if !filterInterprets(">", 2, [][]int{{1, 5, 2, 9}}, [][]int{{5, 9}}) {
		t.Fatal("x > 2 should keep 5 and 9")
	}
	wantFilter := "b := []int{}; for _, x := range numbers { if x > 2 { b = append(b, x) } }; return b"
	if fb := e.filterBody("numbers", ">", 2); fb != wantFilter {
		t.Fatalf("the filter should render its loop:\n got %q\nwant %q", fb, wantFilter)
	}
}
