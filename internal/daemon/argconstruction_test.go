package daemon

import (
	"bytes"
	"strings"
	"testing"

	"golem/internal/memory"
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

// The construction's form pole is derived from the graph: where a stored parameter disagrees with the
// concept the graph holds, the graph drives and the tripwire fires. So editing the construction in the
// graph reaches the engine, the cutover that makes the graph the source.
func TestArgConstructionsDeriveFromGraph(t *testing.T) {
	d, _ := testDaemon(t)
	cnodes, err := d.store.NodesByKind(memory.KindConstruction)
	if err != nil {
		t.Fatal(err)
	}
	cparams := paramsOf(cnodes)
	idx := -1
	for i, n := range cnodes {
		if n.Template == "acxn:en:transitive" {
			cparams[i].Value = "agent/np" // a seed disagreeing with the graph's full transitive
			idx = i
		}
	}
	if idx < 0 {
		t.Fatal("the transitive construction node should exist")
	}
	var buf bytes.Buffer
	d.out = &buf
	d.deriveArgConstructions(cnodes, cparams)
	if cparams[idx].Value != "agent/np/animate rel/v patient/np" {
		t.Fatalf("the graph should drive the construction's form pole, got %q", cparams[idx].Value)
	}
	if cparams[idx].Category != "rel" {
		t.Fatalf("the graph should drive the predicate role, got %q", cparams[idx].Category)
	}
	if !strings.Contains(buf.String(), "argcxn mismatch") {
		t.Fatalf("a seed disagreeing with the graph should log the tripwire:\n%s", buf.String())
	}
}
