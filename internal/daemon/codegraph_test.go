package daemon

import (
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
