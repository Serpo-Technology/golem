package daemon

import (
	"bytes"
	"strings"
	"testing"

	"golem/internal/grammar"
	"golem/wire"
)

// Comprehension lands the meaning in the substrate graph directly, through the grounded
// grammar. On the live runcode path the comprehension graph built from the parsed
// application renders the same Go the IR route renders, so nothing is logged across the
// base shapes (arithmetic, a nested operation, a list). This is the comprehension mirror of
// the generation shadow.
func TestComprehensionGraphShadowsLivePath(t *testing.T) {
	d, _ := testDaemon(t)
	d.cfg.RuntimeEnabled = true

	var buf bytes.Buffer
	d.out = &buf
	for _, tc := range []struct{ in, want string }{
		{"the sum of 6 and 7", "13"},
		{"the product of the sum of 1 and 2 and 3", "9"},
		{"the list of 1 and 2 and 3", "[1 2 3]"},
	} {
		resp, _, _ := d.handle(wire.Request{Cmd: "runcode", Content: tc.in}, nil)
		if !resp.OK || !strings.Contains(resp.Say, tc.want) {
			t.Fatalf("runcode %q should say %q, got %q (ok %v)", tc.in, tc.want, resp.Say, resp.OK)
		}
	}
	if strings.Contains(buf.String(), "comprehension graph mismatch") {
		t.Fatalf("the comprehension graph should agree with the IR route on the live path, but logged: %q", buf.String())
	}
}

// The graph built from a grounded application matches the IR route exactly: walking it
// renders what materializing the comprehended computation renders and what the engine's
// typed render produces, and it runs to the same value. A single-element list and a
// negative literal are covered, the shapes a build is most likely to special-case wrong.
func TestApplicationToGraphMatchesIR(t *testing.T) {
	d, _ := testDaemon(t)
	d.cfg.RuntimeEnabled = true

	for _, surface := range []string{
		"the sum of 6 and 7",
		"the product of the sum of 1 and 2 and 3", // nested, so the inner operation parenthesizes
		"the list of 1 and 2 and 3",
		"the list of 7",       // a single element, attached at rank 0 like any other
		"the sum of -8 and 3", // the leading minus is kept by the tokenizer
	} {
		app, ok := d.gram.ParseApplication("en", surface, d.opHead)
		if !ok {
			t.Fatalf("the grammar should parse %q as an application", surface)
		}
		g, root, gok := d.applicationToGraph(app)
		if !gok {
			t.Fatalf("the application should build a graph: %q", surface)
		}
		walked, wok := d.traverse(goSurface{}, g, root, nil)
		comp := applicationToComputation(app)
		ir, iok := d.traverseCode(comp)
		typed, tok := d.code.ToCode(comp)
		if !wok || !iok || !tok || walked.src != ir || walked.src != typed {
			t.Fatalf("%q: the comprehension graph %q should match the IR route %q and the render %q", surface, walked.src, ir, typed)
		}
		gv, gvok := d.evalCode(walked.src)
		wv, wvok := d.evalCode(ir)
		if !gvok || !wvok || gv != wv {
			t.Fatalf("%q: the comprehension graph should run to the IR value, got %q vs %q", surface, gv, wv)
		}
	}
}

// The base-shape build declines what belongs to a definition body or a stored function, so
// the shadow skips those shapes the way the materializer skipped its own unbuilt ones at
// the start: a conditional, and a defined-function call (its body lives in the stored op,
// not the call surface).
func TestApplicationToGraphDeclinesUnbuilt(t *testing.T) {
	d, _ := testDaemon(t)
	d.cfg.RuntimeEnabled = true

	cond := grammar.Application{Cond: &grammar.Conditional{
		Cmp:   ">",
		Left:  grammar.Application{Var: "number"},
		Right: grammar.Application{Lit: "0"},
		Then:  grammar.Application{Lit: "1"},
		Else:  grammar.Application{Lit: "0"},
	}}
	if _, _, ok := d.applicationToGraph(cond); ok {
		t.Fatal("a conditional application should decline to the later slice")
	}

	say(t, d, "the triple of a number is the product of the number and 3")
	if err := d.recompileGrammar(); err != nil {
		t.Fatal(err)
	}
	app, ok := d.gram.ParseApplication("en", "the triple of 5", d.opHead)
	if !ok {
		t.Fatal("the grammar should parse the defined-function call")
	}
	if _, _, gok := d.applicationToGraph(app); gok {
		t.Fatal("a defined-function call should decline to the later slice")
	}
}
