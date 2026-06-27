package daemon

import (
	"bytes"
	"strings"
	"testing"

	"golem/internal/memory"
)

// The declarative grammar's phrase rules live as concepts in the graph, each carrying its form pole as
// a slot subgraph, and the graph representation round-trips the compiled rule (the derive stays silent).
// The rule concepts are system scaffolding, kept out of the grown layer.
func TestDeclRulesLiveInGraph(t *testing.T) {
	d, s := testDaemon(t) // boot ran ensureDeclGrammar
	var buf bytes.Buffer
	d.out = &buf
	if err := d.recompileGrammar(); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(buf.String(), "declgrammar mismatch") {
		t.Fatalf("the graph-derived phrase rules should match the seed:\n%s", buf.String())
	}

	c, err := s.ConceptByTemplate("syn:en:s")
	if err != nil || c == nil {
		t.Fatalf("the main clause rule should be a concept in the graph: %v", err)
	}
	if !d.isSystemConcept(c.ID) {
		t.Fatal("a phrase rule concept should be excluded as system scaffolding")
	}

	// It reads back as its daughter order, head, and the subject-verb agreement flow.
	got := d.declSynFromGraph(d.storeView(), c.ID)
	if strings.Join(got.order, " ") != "subj vp" {
		t.Fatalf("the main clause rule should order subj before vp, got %v", got.order)
	}
	if got.head != "vp" {
		t.Fatalf("the main clause rule's head should be vp, got %q", got.head)
	}
	if len(got.flows) != 1 || got.flows[0] != (declFlow{feat: "num", from: "subj", to: "vp"}) {
		t.Fatalf("the main clause rule should flow num from subj to vp, got %+v", got.flows)
	}
}

// A phrase rule's form pole is derived from the graph: where a stored parameter disagrees with the
// concept the graph holds, the graph drives and the tripwire fires. So editing a rule in the graph
// reaches the engine, the cutover that makes the graph the source.
func TestDeclRulesDeriveFromGraph(t *testing.T) {
	d, _ := testDaemon(t)
	gnodes, err := d.store.NodesByKind(memory.KindGrammar)
	if err != nil {
		t.Fatal(err)
	}
	gparams := paramsOf(gnodes)
	idx := -1
	for i, n := range gnodes {
		if n.Template == "syn:en:s" {
			gparams[i].Value = "vp | | vp" // a seed disagreeing with the graph's subject-first clause
			idx = i
		}
	}
	if idx < 0 {
		t.Fatal("the main clause rule node should exist")
	}
	var buf bytes.Buffer
	d.out = &buf
	d.deriveDeclGrammar(gnodes, gparams)
	if gparams[idx].Value != "subj vp | num:subj>vp | vp" {
		t.Fatalf("the graph should drive the rule's form pole, got %q", gparams[idx].Value)
	}
	if !strings.Contains(buf.String(), "declgrammar mismatch") {
		t.Fatalf("a seed disagreeing with the graph should log the tripwire:\n%s", buf.String())
	}
}
