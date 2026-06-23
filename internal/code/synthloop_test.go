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
	if !e.foldInterprets(sum, "a", foldInit{lit: "0"}, [][]int{{1, 2, 3}, {2, 4}}, []int{6, 6}) {
		t.Fatal("a + x should fold to the total")
	}
	if e.foldInterprets(sum, "a", foldInit{lit: "1"}, [][]int{{1, 2, 3}}, []int{6}) {
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
	if !e.foldInterprets(max, "a", foldInit{first: true}, [][]int{{1, 5, 2}, {4, 1}}, []int{5, 4}) {
		t.Fatal("a guarded update from the first element should fold to the max")
	}
	body, ok := e.foldBody("numbers", "a", foldInit{first: true}, max)
	wantMax := "a := numbers[0]; for _, x := range numbers { if x > a { a = x } }; return a"
	if !ok || body != wantMax {
		t.Fatalf("the max fold should render its guarded loop:\n got %q\nwant %q", body, wantMax)
	}
	first := foldUpdate{expr: Computation{Var: "a"}}
	if !e.foldInterprets(first, "a", foldInit{first: true}, [][]int{{1, 2, 3}, {4, 5}}, []int{1, 4}) {
		t.Fatal("an unchanging accumulator from the first element should fold to the first")
	}
	body, ok = e.foldBody("numbers", "a", foldInit{first: true}, first)
	if !ok || body != "return numbers[0]" {
		t.Fatalf("an unchanging fold from the first element should read as the first: %q", body)
	}
}

// isExtremum decides the property the extremal search verifies: the value is an
// element of the list and nothing stands cmp to it.
func TestIsExtremum(t *testing.T) {
	if !isExtremum(">", 9, []int{3, 9, 5}) {
		t.Fatal("9 is the greatest of 3 9 5")
	}
	if isExtremum(">", 3, []int{3, 9, 5}) {
		t.Fatal("3 is not the greatest, 9 is above it")
	}
	if isExtremum(">", 0, []int{-5, -2, -9}) {
		t.Fatal("0 is not even in an all-negative list")
	}
	if !isExtremum("<", -9, []int{-5, -2, -9}) {
		t.Fatal("-9 is the smallest")
	}
	if !isExtremum(">", 6, []int{6, 6, 1}) {
		t.Fatal("a duplicate greatest still holds the property")
	}
}

// The extremal search discovers the running maximum and minimum from the property,
// the same guarded folds the by-example search finds, returned in the one form they
// store and write. The grounding runs against a stub here, since the interpreted
// prefilter is the filter that rejects impostors; the real runtime grounds it in
// the daemon.
func TestSynthesizeExtremalFindsMaxAndMin(t *testing.T) {
	e := listEngine()
	pass := func(string) (string, bool) { return "true", true }
	max, ok := e.SynthesizeExtremal("numbers", ">", pass)
	wantMax := "a := numbers[0]; for _, x := range numbers { if x > a { a = x } }; return a"
	if !ok || max != wantMax {
		t.Fatalf("the greatest should be the running max:\n got %q\nwant %q", max, wantMax)
	}
	min, ok := e.SynthesizeExtremal("numbers", "<", pass)
	wantMin := "a := numbers[0]; for _, x := range numbers { if x < a { a = x } }; return a"
	if !ok || min != wantMin {
		t.Fatalf("the smallest should be the running min:\n got %q\nwant %q", min, wantMin)
	}
}

// The probe lists reject the impostors the enumeration reaches before the true
// extremal: a running maximum started at zero gives zero on the all-negative list,
// which is not in it, and returning the first element is not the maximum when it is
// interior. So neither passes the prefilter, and the all-negative probe is what
// rejects the zero start.
func TestSynthesizeExtremalRejectsImpostors(t *testing.T) {
	e := listEngine()
	zeroStart := foldUpdate{expr: Computation{Var: "x"}, guard: ">"}
	if e.extremalInterprets(zeroStart, "a", foldInit{lit: "0"}, ">", [][]int{{-5, -2, -9}}) {
		t.Fatal("a maximum started at zero must fail on an all-negative list")
	}
	first := foldUpdate{expr: Computation{Var: "a"}}
	if e.extremalInterprets(first, "a", foldInit{first: true}, ">", extremalProbeLists()) {
		t.Fatal("returning the first element is not the maximum across the probes")
	}
}

// An order the comparison does not name is declined rather than searched.
func TestSynthesizeExtremalRejectsBadComparison(t *testing.T) {
	e := listEngine()
	fail := func(string) (string, bool) { t.Fatal("a bad comparison should not run a candidate"); return "", false }
	if _, ok := e.SynthesizeExtremal("numbers", ">=", fail); ok {
		t.Fatal("an unsupported comparison should decline")
	}
}

// A list-returning candidate interpreted over the examples carries them, and the
// loop it means writes the Go a runtime would run: a map builds the transform of
// each element, a filter keeps the elements a comparison holds for.
func TestListReturnInterpretsAndRenders(t *testing.T) {
	e := listEngine()
	dbl := Computation{Op: "add", Args: []Computation{{Var: "x"}, {Var: "x"}}}
	if !e.mapInterprets(dbl, [][]int{{1, 2, 3}}, [][]int{{2, 4, 6}}) {
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
