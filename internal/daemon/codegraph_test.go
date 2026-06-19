package daemon

import (
	"strconv"
	"testing"
	"time"

	"golem/internal/code"
	"golem/internal/memory"
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
