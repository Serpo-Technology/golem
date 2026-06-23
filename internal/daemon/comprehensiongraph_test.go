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
// base shapes (arithmetic, a nested operation, a list) and a defined-function call,
// including a recursive one whose body the function materializer supplies. This is the
// comprehension mirror of the generation shadow.
func TestComprehensionGraphShadowsLivePath(t *testing.T) {
	d, _ := testDaemon(t)
	d.cfg.RuntimeEnabled = true
	say(t, d, "the triple of a number is the product of the number and 3")
	say(t, d, "the factorial of a number is 1 if the number is below 1 else the product of the number and the factorial of the difference of the number and 1")
	if err := d.recompileGrammar(); err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer
	d.out = &buf
	for _, tc := range []struct{ in, want string }{
		{"the sum of 6 and 7", "13"},
		{"the product of the sum of 1 and 2 and 3", "9"},
		{"the list of 1 and 2 and 3", "[1 2 3]"},
		{"the triple of 5", "15"},
		{"the factorial of 5", "120"},
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
// typed render produces, and it runs to the same value. A single-element list, a negative
// literal, a defined-function call, a call with a compound argument, and a recursive call
// are covered, the shapes a build is most likely to special-case wrong.
func TestApplicationToGraphMatchesIR(t *testing.T) {
	d, _ := testDaemon(t)
	d.cfg.RuntimeEnabled = true
	say(t, d, "the triple of a number is the product of the number and 3")
	say(t, d, "the factorial of a number is 1 if the number is below 1 else the product of the number and the factorial of the difference of the number and 1")
	if err := d.recompileGrammar(); err != nil {
		t.Fatal(err)
	}

	for _, surface := range []string{
		"the sum of 6 and 7",
		"the product of the sum of 1 and 2 and 3", // nested, so the inner operation parenthesizes
		"the list of 1 and 2 and 3",
		"the list of 7",                    // a single element, attached at rank 0 like any other
		"the sum of -8 and 3",              // the leading minus is kept by the tokenizer
		"the triple of 5",                  // a defined-function call, the body from the stored op
		"the triple of the sum of 1 and 2", // a call with a compound argument
		"the factorial of 5",               // recursion, the body riding the function materializer
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

// The build declines what belongs to a definition body or cannot ground, so the shadow
// skips those shapes: a conditional and a bare parameter reference (which live in a body,
// not a computed call), and an unknown operation (which names no function to build).
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
	if _, _, ok := d.applicationToGraph(grammar.Application{Var: "number"}); ok {
		t.Fatal("a parameter reference should decline")
	}
	if _, _, ok := d.applicationToGraph(grammar.Application{Head: "nonesuch", Args: []grammar.Application{{Lit: "5"}}}); ok {
		t.Fatal("an unknown operation should decline")
	}
}

// The type check reads the comprehension graph and agrees with the engine reading the
// Computation, across well-typed shapes (arithmetic, a list, a call, a fold over a list, a
// nested call argument) and the ill-typed ones the engine refuses (a list reduction on a
// number, an integer operation on a list). So reading the type off the substrate admits the
// same gaps as reading it off the Computation.
func TestTypeCheckGraphMatchesEngine(t *testing.T) {
	d, _ := testDaemon(t)
	d.cfg.RuntimeEnabled = true
	say(t, d, "the triple of a number is the product of the number and 3")
	teachReductions(t, d, "total")
	if err := d.recompileGrammar(); err != nil {
		t.Fatal(err)
	}

	for _, surface := range []string{
		"the sum of 6 and 7",
		"the list of 1 and 2 and 3",
		"the triple of 5",
		"the total of the list of 1 and 2 and 3",
		"the sum of the triple of 4 and 2",
		"the total of 5",                  // ill-typed: a list reduction on a number
		"the triple of the list of 1 and 2", // ill-typed: an integer function on a list
	} {
		app, ok := d.gram.ParseApplication("en", surface, d.opHead)
		if !ok {
			t.Fatalf("the grammar should parse %q", surface)
		}
		g, root, gok := d.applicationToGraph(app)
		if !gok {
			t.Fatalf("the application should build a graph: %q", surface)
		}
		comp := applicationToComputation(app)
		graphOK := d.typeCheckGraph(g, root)
		if graphOK != d.code.TypeCheck(comp) {
			t.Errorf("%q: graph type check %v disagrees with the engine %v", surface, graphOK, d.code.TypeCheck(comp))
		}
	}
}

// The live path type-checks off the graph: a well-typed reduction runs, an ill-typed one (a
// list reduction on a number) admits a gap, and the graph check agrees with the engine so
// nothing is logged.
func TestTypeCheckGraphLivePath(t *testing.T) {
	d, _ := testDaemon(t)
	d.cfg.RuntimeEnabled = true
	teachReductions(t, d, "total")
	if err := d.recompileGrammar(); err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer
	d.out = &buf
	good, _, _ := d.handle(wire.Request{Cmd: "runcode", Content: "the total of the list of 4 and 5 and 6"}, nil)
	if !strings.Contains(good.Say, "15") {
		t.Fatalf("a well-typed reduction should run to 15: %q", good.Say)
	}
	bad, _, _ := d.handle(wire.Request{Cmd: "runcode", Content: "the total of 5"}, nil)
	if !strings.Contains(bad.Say, "could not") {
		t.Fatalf("an ill-typed reduction should admit a gap: %q", bad.Say)
	}
	if strings.Contains(buf.String(), "typecheck graph mismatch") {
		t.Fatalf("the graph type check should agree with the engine, but logged: %q", buf.String())
	}
}
