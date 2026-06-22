package code

import "testing"

// bumpDef maps an inline per-element expression, so it needs no helper function
// beyond the arithmetic listEngine carries.
const bumpDef = "the bumped of a list of numbers is the sum of each number and 10"

// A body that names "each <element>" comprehends to a map over the list
// parameter, the per-element body the computation around the element.
func TestComprehendMap(t *testing.T) {
	e := listEngine()
	op, ok := e.ComprehendDefinition(bumpDef)
	if !ok {
		t.Fatal("a map definition should comprehend")
	}
	m := op.Body.Map
	if m == nil || m.Var != "number" || m.List.Var != "numbers" || m.Body.Op != "add" {
		t.Fatalf("the body should map number + 10 over numbers: %+v", op.Body)
	}
}

// A map writes a slice-returning function that builds its result element by
// element.
func TestMapToFunc(t *testing.T) {
	e := listEngine()
	op, ok := e.ComprehendDefinition(bumpDef)
	if !ok {
		t.Fatal("bumped should comprehend")
	}
	e.index(op)
	want := "func bumped(numbers []int) []int { return (func() []int { out := []int{}; for _, number := range numbers { out = append(out, number + 10) }; return out }()) }"
	if src, ok := e.ToFunc("bumped"); !ok || src != want {
		t.Fatalf("bumped should write a slice-returning map:\n got %q\nwant %q", src, want)
	}
}

// A map returns a list, so it composes under a reduction: the total of a mapped
// list type-checks where a number-returning function would not.
func TestMapComposesUnderReduction(t *testing.T) {
	e := listEngine()
	op, ok := e.ComprehendDefinition(bumpDef)
	if !ok {
		t.Fatal("bumped should comprehend")
	}
	e.index(op)
	c, ok := e.Comprehend("the total of the bumped of the list of 1 and 2 and 3")
	if !ok || !e.TypeCheck(c) {
		t.Fatalf("a reduction over a map should type-check: ok %v", ok)
	}
}

// The map loop closes through storage: the body stores structurally and reloads
// as a map.
func TestMapRoundTrip(t *testing.T) {
	e := listEngine()
	op, ok := e.ComprehendDefinition(bumpDef)
	if !ok {
		t.Fatal("bumped should comprehend")
	}
	_, body, ok := e.DefSource(op)
	if !ok || body != "map number ; numbers ; number + 10" {
		t.Fatalf("DefSource should store the map structurally: %q", body)
	}
	reloaded := NewEngine([]Op{{Name: "add", Symbol: "+", Word: "sum"}},
		[]Def{{Name: op.Name, Params: []string{"numbers"}, ParamTypes: []string{typeList}, Body: body}})
	if reloaded.byName["bumped"].Body.Map == nil {
		t.Fatal("the reloaded bumped should still be a map")
	}
}

// "each" with no list parameter to range over is refused, so a transform with
// nowhere to map is not half-learned.
func TestMapRejectsNoList(t *testing.T) {
	e := listEngine()
	if _, ok := e.ComprehendDefinition("the bad of a number is the sum of each item and 1"); ok {
		t.Fatal("a map with no list parameter should be refused")
	}
}
