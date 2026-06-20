package daemon

import "testing"

// A code surface comprehends to a meaning graph and generates back through the one
// traverser: parsing the surface to a graph and rendering it with genCode yields the same
// source the engine's comprehend + render produces, and runs to the same value. Both
// directions go through the graph, the Computation type bypassed on each.
func TestComprehendCodeGraphRoundTrips(t *testing.T) {
	d, _ := testDaemon(t)
	d.cfg.RuntimeEnabled = true

	for _, surface := range []string{
		"the sum of 6 and 7",
		"the product of the sum of 1 and 2 and 3", // nested, so the inner operation parenthesizes
		"the list of 1 and 2 and 3",
	} {
		g, root, ok := d.parseCodeToGraph(surface)
		if !ok {
			t.Fatalf("the surface should comprehend to a graph: %q", surface)
		}
		got, _, gok := d.genCode(g, root, nil, 0)
		comp, cok := d.code.Comprehend(surface)
		if !cok {
			t.Fatalf("the engine should comprehend %q", surface)
		}
		want, wok := d.code.ToCode(comp)
		if !gok || !wok || got != want {
			t.Fatalf("%q: the comprehended graph should render to %q, got %q", surface, want, got)
		}
		gv, gvok := d.evalCode(got)
		wv, wvok := d.evalCode(want)
		if !gvok || !wvok || gv != wv {
			t.Fatalf("%q: rendered %q (=%q) should run to the engine's %q (=%q)", surface, got, gv, want, wv)
		}
	}
}
