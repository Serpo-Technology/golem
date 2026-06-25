package daemon

import (
	"bytes"
	"strings"
	"testing"
)

// The argument-structure constructions live as concepts in the graph, each carrying its form pole
// as a slot subgraph, and the graph representation round-trips the compiled construction (the shadow
// stays silent). The construction concepts are system scaffolding, kept out of the grown layer.
func TestArgConstructionsLiveInGraph(t *testing.T) {
	d, s := testDaemon(t) // boot ran ensureArgConstructions + the shadow
	var buf bytes.Buffer
	d.out = &buf
	if err := d.recompileGrammar(); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(buf.String(), "argcxn mismatch") {
		t.Fatalf("the graph-derived constructions should match the seed:\n%s", buf.String())
	}

	c, err := s.ConceptByTemplate("acxn:en:transitive")
	if err != nil || c == nil {
		t.Fatalf("the transitive construction should be a concept in the graph: %v", err)
	}
	if !d.isSystemConcept(c.ID) {
		t.Fatal("a construction concept should be excluded as system scaffolding")
	}

	// It carries its three ordered argument slots and marks the predicate role.
	g := d.storeView()
	if slots := g.dests(c.ID, cxnSlotRel); len(slots) != 3 {
		t.Fatalf("the transitive construction should carry 3 slots, got %d", len(slots))
	}
	if preds := g.dests(c.ID, cxnPredRel); len(preds) != 1 || g.content(preds[0]) != "rel" {
		t.Fatalf("the predicate role should be rel: %v", preds)
	}
	// A slot resolves to its role through the shared vocabulary.
	roles := map[string]bool{}
	for _, sid := range g.dests(c.ID, cxnSlotRel) {
		if r := g.dests(sid, cxnRoleRel); len(r) == 1 {
			roles[g.content(r[0])] = true
		}
	}
	for _, want := range []string{"agent", "rel", "patient"} {
		if !roles[want] {
			t.Fatalf("the transitive construction should hold the %q role, got %v", want, roles)
		}
	}
}
