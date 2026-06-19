package daemon

import (
	"bytes"
	"strconv"
	"strings"
	"testing"
	"time"

	"golem/internal/code"
	"golem/internal/memory"
	"golem/wire"
)

// A code computation represented as a graph subgraph is traversed to emit Go that
// runs to the same value the typed render produces: the first proof of generation
// by graph traversal for code, on the smallest shapes (a flat binary expression
// and a nested, precedence-sensitive one). The traversal is grounded the way every
// construction is, by running the source it emits, and checked against the existing
// typed render so the two agree.
func TestCodeGraphTraversalEmitsRunnableGo(t *testing.T) {
	d, s := testDaemon(t)
	d.cfg.RuntimeEnabled = true

	mk := func(content string) uint64 {
		f := &memory.Fragment{Kind: memory.KindConcept, Channel: "test", Content: content, Time: time.Now()}
		if _, err := s.Encode(f); err != nil {
			t.Fatal(err)
		}
		return f.ID
	}
	op := func(name string, l, r uint64) uint64 {
		root := mk("")
		if err := s.LinkRel(root, mk(name), memory.RelGrounds, 1); err != nil {
			t.Fatal(err)
		}
		if err := s.LinkRel(root, l, operand0Rel, 1); err != nil {
			t.Fatal(err)
		}
		if err := s.LinkRel(root, r, operand1Rel, 1); err != nil {
			t.Fatal(err)
		}
		return root
	}

	cases := []struct {
		root    uint64
		wantSrc string
		comp    code.Computation
	}{
		{
			op("add", mk("6"), mk("7")),
			"6 + 7",
			code.Computation{Op: "add", Args: []code.Computation{{Lit: "6"}, {Lit: "7"}}},
		},
		{
			// multiply(add(2,3), 4): the operand operation must be parenthesized.
			op("multiply", op("add", mk("2"), mk("3")), mk("4")),
			"(2 + 3) * 4",
			code.Computation{Op: "multiply", Args: []code.Computation{
				{Op: "add", Args: []code.Computation{{Lit: "2"}, {Lit: "3"}}},
				{Lit: "4"},
			}},
		},
	}
	for _, tc := range cases {
		src, ok := d.genCodeFromGraph(tc.root)
		if !ok || src != tc.wantSrc {
			t.Fatalf("traversal should emit %q, got %q ok %v", tc.wantSrc, src, ok)
		}
		typed, tok := d.code.ToCode(tc.comp)
		if !tok {
			t.Fatalf("the typed render should produce source for %+v", tc.comp)
		}
		if typed != tc.wantSrc {
			t.Fatalf("the traversed source should match the typed render: %q vs %q", src, typed)
		}
		// Grounded by running: the traversed source and the typed render run alike.
		gotVal, gok := d.evalCode(src)
		wantVal, wok := d.evalCode(typed)
		if !gok || !wok || gotVal != wantVal {
			t.Fatalf("the traversed source %q (=%q) should run to the typed render %q (=%q)", src, gotVal, typed, wantVal)
		}
	}
}

// The traverser widens to variable references and conditionals: a conditional
// function body over variables emits the branch function literal the typed render
// uses (grounded by applying it to arguments), and a conditional over literals runs
// standalone. Variables ride the leaf case, since a leaf emits its token either way.
func TestCodeGraphTraversalConditional(t *testing.T) {
	d, s := testDaemon(t)
	d.cfg.RuntimeEnabled = true

	mk := func(content string) uint64 {
		f := &memory.Fragment{Kind: memory.KindConcept, Channel: "test", Content: content, Time: time.Now()}
		if _, err := s.Encode(f); err != nil {
			t.Fatal(err)
		}
		return f.ID
	}
	cond := func(cmp string, l, r, then, els uint64) uint64 {
		root := mk(cmp)
		link := func(rel string, to uint64) {
			if err := s.LinkRel(root, to, rel, 1); err != nil {
				t.Fatal(err)
			}
		}
		link(condLeftRel, l)
		link(condRightRel, r)
		link(condThenRel, then)
		link(condElseRel, els)
		return root
	}

	// larger(a, b) = a if a > b else b: a conditional over variable references.
	larger := cond(">", mk("a"), mk("b"), mk("a"), mk("b"))
	src, ok := d.genCodeFromGraph(larger)
	want := "(func() int { if a > b { return a }; return b }())"
	if !ok || src != want {
		t.Fatalf("the conditional body should emit %q, got %q ok %v", want, src, ok)
	}
	comp := code.Computation{Cond: &code.Conditional{Cmp: ">",
		Left: code.Computation{Var: "a"}, Right: code.Computation{Var: "b"},
		Then: code.Computation{Var: "a"}, Else: code.Computation{Var: "b"}}}
	if typed, tok := d.code.ToCode(comp); !tok || typed != want {
		t.Fatalf("the typed render should match the traversal: %q vs %q", typed, src)
	}
	// Grounded by running: wrap the body in its parameters and apply (the vars are
	// undefined standalone, as they are for the typed render too).
	wrapped := "(func(a int, b int) int { return " + src + " })(6, 4)"
	if v, vok := d.evalCode(wrapped); !vok || v != "6" {
		t.Fatalf("larger(6, 4) should run to 6, got %q from %q", v, wrapped)
	}

	// A conditional over literals runs standalone: 100 if 2 < 9 else 200.
	lit := cond("<", mk("2"), mk("9"), mk("100"), mk("200"))
	lsrc, lok := d.genCodeFromGraph(lit)
	if !lok {
		t.Fatal("the literal conditional should emit")
	}
	if v, vok := d.evalCode(lsrc); !vok || v != "100" {
		t.Fatalf("100 if 2 < 9 else 200 should run to 100, got %q from %q", v, lsrc)
	}
}

// The traverser widens to ordered children: a list literal's elements are bound to
// their positions through slot nodes, so the surface follows the ordinal rather than
// storage order and a value can repeat at two positions (the case a plain edge could
// not represent, since the links primary key is (from, to, rel)). Each shape is
// checked against the typed render and grounded by running, the same bar the other
// traversal tests hold.
func TestCodeGraphTraversalList(t *testing.T) {
	d, s := testDaemon(t)
	d.cfg.RuntimeEnabled = true

	mk := func(content string) uint64 {
		f := &memory.Fragment{Kind: memory.KindConcept, Channel: "test", Content: content, Time: time.Now()}
		if _, err := s.Encode(f); err != nil {
			t.Fatal(err)
		}
		return f.ID
	}
	// listOf builds a list-literal subgraph: a root grounded in the list concept, with
	// one slot per element binding a value node to its position. Slots are added in the
	// given order, so passing positions out of order proves that the ordinal decides
	// the surface and storage order does not.
	listOf := func(positions []int, values []uint64) uint64 {
		root := mk("")
		if err := s.LinkRel(root, mk("list"), memory.RelGrounds, 1); err != nil {
			t.Fatal(err)
		}
		for i, pos := range positions {
			slot := mk("")
			if err := s.LinkRel(root, slot, elemRel, 1); err != nil {
				t.Fatal(err)
			}
			if err := s.LinkRel(slot, mk(strconv.Itoa(pos)), rankRel, 1); err != nil {
				t.Fatal(err)
			}
			if err := s.LinkRel(slot, values[i], holdsRel, 1); err != nil {
				t.Fatal(err)
			}
		}
		return root
	}
	op := func(name string, l, r uint64) uint64 {
		root := mk("")
		if err := s.LinkRel(root, mk(name), memory.RelGrounds, 1); err != nil {
			t.Fatal(err)
		}
		if err := s.LinkRel(root, l, operand0Rel, 1); err != nil {
			t.Fatal(err)
		}
		if err := s.LinkRel(root, r, operand1Rel, 1); err != nil {
			t.Fatal(err)
		}
		return root
	}

	// A flat list emits the composite literal, matches the typed render, and both run
	// to the same value.
	flat := listOf([]int{0, 1, 2}, []uint64{mk("1"), mk("2"), mk("3")})
	src, ok := d.genCodeFromGraph(flat)
	want := "[]int{1, 2, 3}"
	if !ok || src != want {
		t.Fatalf("the list should emit %q, got %q ok %v", want, src, ok)
	}
	comp := code.Computation{Op: "list", Args: []code.Computation{{Lit: "1"}, {Lit: "2"}, {Lit: "3"}}}
	typed, tok := d.code.ToCode(comp)
	if !tok || typed != want {
		t.Fatalf("the typed render should match the traversal: %q vs %q", typed, src)
	}
	gotVal, gok := d.evalCode(src)
	wantVal, wok := d.evalCode(typed)
	if !gok || !wok || gotVal != wantVal {
		t.Fatalf("the traversed source %q (=%q) should run to the typed render %q (=%q)", src, gotVal, typed, wantVal)
	}

	// Order is meaning: slots added out of position order still emit position-sorted.
	ooo := listOf([]int{2, 0, 1}, []uint64{mk("30"), mk("10"), mk("20")})
	osrc, ook := d.genCodeFromGraph(ooo)
	if !ook || osrc != "[]int{10, 20, 30}" {
		t.Fatalf("the ordinal should decide the surface, got %q ok %v", osrc, ook)
	}

	// A repeat binds one value node at two distinct slots, which a plain elem edge
	// could not (the second would collide on the primary key and place once).
	five := mk("5")
	rep := listOf([]int{0, 1}, []uint64{five, five})
	rsrc, rok := d.genCodeFromGraph(rep)
	if !rok || rsrc != "[]int{5, 5}" {
		t.Fatalf("a repeated value should place twice, got %q ok %v", rsrc, rok)
	}
	if v, vok := d.evalCode(rsrc); !vok || v != "[5 5]" {
		t.Fatalf("[]int{5, 5} should run to [5 5], got %q", v)
	}

	// A held value recurses through the traverser: a list holding an operation node.
	nested := listOf([]int{0}, []uint64{op("add", mk("6"), mk("7"))})
	nsrc, nok := d.genCodeFromGraph(nested)
	if !nok || nsrc != "[]int{6 + 7}" {
		t.Fatalf("a nested element should recurse, got %q ok %v", nsrc, nok)
	}
	if v, vok := d.evalCode(nsrc); !vok || v != "[13]" {
		t.Fatalf("[]int{6 + 7} should run to [13], got %q", v)
	}
}

// The traverser widens to n-ary calls: a function is itself a graph (ordered parameter
// slots and a body subgraph), and a call inlines by walking that body with the
// parameters bound to the call's arguments, the graph counterpart of the typed render's
// inline + substitute. Each call is matched to the typed render (the same function
// taught to the engine) and grounded by running. Scope is simple arguments; a compound
// argument's operand-position parenthesizing and recursion are not exercised here.
func TestCodeGraphTraversalCall(t *testing.T) {
	d, s := testDaemon(t)
	d.cfg.RuntimeEnabled = true

	// Teach the functions the realistic way, so the engine can render the typed call to
	// compare against; recompile so d.code carries them.
	say(t, d, "the triple of a number is the product of the number and 3")
	say(t, d, "the gap of a high and a low is the difference of the high and the low")
	say(t, d, "the square of a number is the product of the number and the number")
	if err := d.recompileGrammar(); err != nil {
		t.Fatal(err)
	}

	mk := func(content string) uint64 {
		f := &memory.Fragment{Kind: memory.KindConcept, Channel: "test", Content: content, Time: time.Now()}
		if _, err := s.Encode(f); err != nil {
			t.Fatal(err)
		}
		return f.ID
	}
	// slotInto attaches one ordered child to a parent under rel: a slot at the given
	// rank holding the value node.
	slotInto := func(parent uint64, rel string, rank int, value uint64) {
		slot := mk("")
		if err := s.LinkRel(parent, slot, rel, 1); err != nil {
			t.Fatal(err)
		}
		if err := s.LinkRel(slot, mk(strconv.Itoa(rank)), rankRel, 1); err != nil {
			t.Fatal(err)
		}
		if err := s.LinkRel(slot, value, holdsRel, 1); err != nil {
			t.Fatal(err)
		}
	}
	op := func(name string, l, r uint64) uint64 {
		root := mk("")
		if err := s.LinkRel(root, mk(name), memory.RelGrounds, 1); err != nil {
			t.Fatal(err)
		}
		if err := s.LinkRel(root, l, operand0Rel, 1); err != nil {
			t.Fatal(err)
		}
		if err := s.LinkRel(root, r, operand1Rel, 1); err != nil {
			t.Fatal(err)
		}
		return root
	}
	// fn builds a function as a graph: a named concept with ordered parameter slots and
	// a body subgraph. call builds an application of it: a node grounding in the function
	// with its arguments as ordered children.
	fn := func(name string, params []string, body uint64) uint64 {
		f := mk(name)
		for i, p := range params {
			slotInto(f, paramRel, i, mk(p))
		}
		if err := s.LinkRel(f, body, bodyRel, 1); err != nil {
			t.Fatal(err)
		}
		return f
	}
	call := func(f uint64, args ...uint64) uint64 {
		c := mk("")
		if err := s.LinkRel(c, f, memory.RelGrounds, 1); err != nil {
			t.Fatal(err)
		}
		for i, a := range args {
			slotInto(c, elemRel, i, a)
		}
		return c
	}
	// matchesAndRuns checks the traversed source equals the typed render of the call and
	// that both run to the same value.
	matchesAndRuns := func(label, src string, ok bool, want string, comp code.Computation) {
		if !ok || src != want {
			t.Fatalf("%s should emit %q, got %q ok %v", label, want, src, ok)
		}
		typed, tok := d.code.ToCode(comp)
		if !tok || typed != want {
			t.Fatalf("%s typed render should match the traversal: %q vs %q", label, typed, src)
		}
		gv, gok := d.evalCode(src)
		wv, wok := d.evalCode(typed)
		if !gok || !wok || gv != wv {
			t.Fatalf("%s traversed %q (=%q) should run to the typed render %q (=%q)", label, src, gv, typed, wv)
		}
	}

	// 1-ary: triple(4) inlines product(number, 3) with number bound to 4.
	tripleSrc, tok := d.genCodeFromGraph(call(fn("triple", []string{"number"}, op("multiply", mk("number"), mk("3"))), mk("4")))
	matchesAndRuns("triple(4)", tripleSrc, tok, "4 * 3",
		code.Computation{Op: "triple", Args: []code.Computation{{Lit: "4"}}})

	// 2-ary: gap(9, 4) binds two distinct parameters by position.
	gapSrc, gok := d.genCodeFromGraph(call(fn("gap", []string{"high", "low"}, op("subtract", mk("high"), mk("low"))), mk("9"), mk("4")))
	matchesAndRuns("gap(9, 4)", gapSrc, gok, "9 - 4",
		code.Computation{Op: "gap", Args: []code.Computation{{Lit: "9"}, {Lit: "4"}}})

	// A parameter used twice: square(5) fills every occurrence from one argument.
	squareSrc, sok := d.genCodeFromGraph(call(fn("square", []string{"number"}, op("multiply", mk("number"), mk("number"))), mk("5")))
	matchesAndRuns("square(5)", squareSrc, sok, "5 * 5",
		code.Computation{Op: "square", Args: []code.Computation{{Lit: "5"}}})

	// A conditional body: larger(6, 4) inlines the branch literal with the arguments
	// bound, proving the binding threads through a conditional, not only arithmetic.
	// Built directly as a graph and grounded by running.
	cond := func(cmp string, l, r, then, els uint64) uint64 {
		root := mk(cmp)
		link := func(rel string, to uint64) {
			if err := s.LinkRel(root, to, rel, 1); err != nil {
				t.Fatal(err)
			}
		}
		link(condLeftRel, l)
		link(condRightRel, r)
		link(condThenRel, then)
		link(condElseRel, els)
		return root
	}
	largerBody := cond(">", mk("a"), mk("b"), mk("a"), mk("b"))
	largerSrc, lok := d.genCodeFromGraph(call(fn("larger", []string{"a", "b"}, largerBody), mk("6"), mk("4")))
	wantLarger := "(func() int { if 6 > 4 { return 6 }; return 4 }())"
	if !lok || largerSrc != wantLarger {
		t.Fatalf("larger(6, 4) should emit %q, got %q ok %v", wantLarger, largerSrc, lok)
	}
	if v, vok := d.evalCode(largerSrc); !vok || v != "6" {
		t.Fatalf("larger(6, 4) should run to 6, got %q from %q", v, largerSrc)
	}
}

// The loop closes on the daemon's own comprehension: a computation comprehended from a
// real utterance is materialized as a subgraph (not hand-built), and walking that
// subgraph emits the same source the engine's typed render does and runs to the same
// value. This is the bridge from test-built graphs to graphs the daemon produces.
func TestCodeGraphClosesTheLoop(t *testing.T) {
	d, _ := testDaemon(t)
	d.cfg.RuntimeEnabled = true

	for _, utterance := range []string{
		"the sum of 6 and 7",
		"6 * (7 + 1)",
		"the difference of 10 and 4",
		"the list of 1 and 2 and 3",
	} {
		comp, ok := d.code.Comprehend(utterance)
		if !ok {
			t.Fatalf("the engine should comprehend %q", utterance)
		}
		walked, wok := d.shadowCheck(comp)
		typed, tok := d.code.ToCode(comp)
		if !wok || !tok || walked != typed {
			t.Fatalf("%q: the materialized graph should walk to the render: %q vs %q (walked ok %v)", utterance, walked, typed, wok)
		}
		gv, gok := d.evalCode(walked)
		wv, wvok := d.evalCode(typed)
		if !gok || !wvok || gv != wv {
			t.Fatalf("%q: the walked source %q (=%q) should run to the render %q (=%q)", utterance, walked, gv, typed, wv)
		}
	}
}

// The shadow widens to calls: a computation that calls a defined function is
// materialized by building the function itself as a graph (its ordered parameters and
// its body) and inlining it, so the walked source matches the engine's inlined render
// and runs to the same value. The body may be arithmetic over the parameters or a
// conditional, the larger case proving a function whose body is a choice materializes.
func TestCodeGraphShadowsCalls(t *testing.T) {
	d, _ := testDaemon(t)
	d.cfg.RuntimeEnabled = true

	say(t, d, "the triple of a number is the product of the number and 3")
	say(t, d, "the gap of a high and a low is the difference of the high and the low")
	say(t, d, "the larger of a number and a count is the number if the number is above the count else the count")
	if err := d.recompileGrammar(); err != nil {
		t.Fatal(err)
	}

	for _, utterance := range []string{
		"the triple of 4",
		"the gap of 9 and 4",
		"the larger of 6 and 4",
	} {
		comp, ok := d.code.Comprehend(utterance)
		if !ok {
			t.Fatalf("the engine should comprehend %q", utterance)
		}
		walked, wok := d.shadowCheck(comp)
		typed, tok := d.code.ToCode(comp)
		if !wok || !tok || walked != typed {
			t.Fatalf("%q: the materialized call should walk to the render: %q vs %q (walked ok %v)", utterance, walked, typed, wok)
		}
		gv, gok := d.evalCode(walked)
		wv, wvok := d.evalCode(typed)
		if !gok || !wvok || gv != wv {
			t.Fatalf("%q: the walked call %q (=%q) should run to the render %q (=%q)", utterance, walked, gv, typed, wv)
		}
	}
}

// With operand parenthesizing in the walk, a call composes anywhere: a compound argument
// is parenthesized where the body uses its parameter as an operand, and a call in an
// operand position is parenthesized as the operation it inlines to. The walked source
// matches the engine's inlined render and runs alike.
func TestCodeGraphShadowsCompoundCalls(t *testing.T) {
	d, _ := testDaemon(t)
	d.cfg.RuntimeEnabled = true
	say(t, d, "the triple of a number is the product of the number and 3")
	if err := d.recompileGrammar(); err != nil {
		t.Fatal(err)
	}

	lit := func(s string) code.Computation { return code.Computation{Lit: s} }
	add := func(a, b code.Computation) code.Computation {
		return code.Computation{Op: "add", Args: []code.Computation{a, b}}
	}
	triple := func(a code.Computation) code.Computation {
		return code.Computation{Op: "triple", Args: []code.Computation{a}}
	}

	cases := []struct {
		comp   code.Computation
		want   string
		runsTo string
	}{
		{triple(add(lit("6"), lit("7"))), "(6 + 7) * 3", "39"}, // a compound argument
		{add(triple(lit("4")), lit("5")), "(4 * 3) + 5", "17"}, // a call in an operand position
	}
	for _, tc := range cases {
		walked, wok := d.shadowCheck(tc.comp)
		typed, tok := d.code.ToCode(tc.comp)
		if !wok || !tok || walked != tc.want || typed != tc.want {
			t.Fatalf("want %q: walked %q (ok %v), typed %q (ok %v)", tc.want, walked, wok, typed, tok)
		}
		if v, ok := d.evalCode(walked); !ok || v != tc.runsTo {
			t.Fatalf("%q should run to %s, got %q", walked, tc.runsTo, v)
		}
	}
}

// Recursion is the last shape: a function whose body calls itself cannot inline, so a
// call to it is written as the self-referential local-function form the engine uses. The
// materialized graph carries the self-call as a reference back to the function node, and
// the walk emits the same recursive expression, which runs to the same value.
func TestCodeGraphShadowsRecursion(t *testing.T) {
	d, _ := testDaemon(t)
	d.cfg.RuntimeEnabled = true
	say(t, d, "the factorial of a number is 1 if the number is below 1 else the product of the number and the factorial of the difference of the number and 1")
	if err := d.recompileGrammar(); err != nil {
		t.Fatal(err)
	}

	comp, ok := d.code.Comprehend("the factorial of 5")
	if !ok {
		t.Fatal("the engine should comprehend the factorial call")
	}
	walked, wok := d.shadowCheck(comp)
	typed, tok := d.code.ToCode(comp)
	if !wok || !tok || walked != typed {
		t.Fatalf("the recursive call should walk to the render:\n walked %q (ok %v)\n typed  %q (ok %v)", walked, wok, typed, tok)
	}
	gv, gok := d.evalCode(walked)
	wv, wvok := d.evalCode(typed)
	if !gok || !wvok || gv != wv || gv != "120" {
		t.Fatalf("the recursive call should run to 120: walked ran to %q, typed to %q", gv, wv)
	}
}

// On the live runcode path the shadow is wired in and silent: the reply is the engine's
// result as before, and because the graph the daemon materializes agrees with the
// render, nothing is logged. A mismatch would print, so an empty log is the agreement.
func TestRuncodeShadowAgreesOnLivePath(t *testing.T) {
	d, _ := testDaemon(t)
	d.cfg.RuntimeEnabled = true

	var buf bytes.Buffer
	d.out = &buf
	resp, _, _ := d.handle(wire.Request{Cmd: "runcode", Content: "the sum of 6 and 7"}, nil)
	if !resp.OK || !strings.Contains(resp.Say, "13") {
		t.Fatalf("runcode should still compute 13, got %q (ok %v)", resp.Say, resp.OK)
	}
	if strings.Contains(buf.String(), "shadow mismatch") {
		t.Fatalf("the live shadow should agree, but logged: %q", buf.String())
	}
}
