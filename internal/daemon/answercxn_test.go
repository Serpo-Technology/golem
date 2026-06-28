package daemon

import (
	"bytes"
	"strings"
	"testing"

	"golem/internal/grammar"
	"golem/internal/memory"
)

// The answer templates Compose fills live as concepts in the graph, each carrying its template, and the
// graph representation round-trips the compiled template (the derive stays silent). The template concepts
// are system scaffolding, kept out of the grown layer.
func TestAnswerConstructionsLiveInGraph(t *testing.T) {
	d, s := testDaemon(t) // boot ran ensureAnswerConstructions
	var buf bytes.Buffer
	d.out = &buf
	if err := d.recompileGrammar(); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(buf.String(), "answer cxn mismatch") {
		t.Fatalf("the graph-derived answer templates should match the seed:\n%s", buf.String())
	}

	c, err := s.ConceptByTemplate("anscxn:en:definition:plain")
	if err != nil || c == nil {
		t.Fatalf("the definition template should be a concept in the graph: %v", err)
	}
	if !d.isSystemConcept(c.ID) {
		t.Fatal("an answer template concept should be excluded as system scaffolding")
	}
	if c.Content != "{subject} is {kind}." {
		t.Fatalf("the concept should carry the template as its content, got %q", c.Content)
	}
}

// An answer template is derived from the graph: where a stored parameter disagrees with the concept the
// graph holds, the graph drives and the tripwire fires. So editing a template in the graph reaches Compose.
func TestAnswerConstructionsDeriveFromGraph(t *testing.T) {
	d, _ := testDaemon(t)
	cnodes, err := d.store.NodesByKind(memory.KindConstruction)
	if err != nil {
		t.Fatal(err)
	}
	cparams := paramsOf(cnodes)
	idx := -1
	for i, n := range cnodes {
		if n.Template == "cxn:en:definition:plain" {
			cparams[i].Value = "{subject} = {kind}" // a seed disagreeing with the graph's template
			idx = i
		}
	}
	if idx < 0 {
		t.Fatal("the definition template node should exist")
	}
	var buf bytes.Buffer
	d.out = &buf
	d.deriveAnswerConstructions(cnodes, cparams)
	if cparams[idx].Value != "{subject} is {kind}." {
		t.Fatalf("the graph should drive the template, got %q", cparams[idx].Value)
	}
	if !strings.Contains(buf.String(), "answer cxn mismatch") {
		t.Fatalf("a seed disagreeing with the graph should log the tripwire:\n%s", buf.String())
	}
}

// An answer template taught ONLY into the graph (no seed entry) reaches Compose and fills its slots, and
// it survives the night, so a realization template learned into the graph is a durable fluid weight.
func TestTaughtAnswerTemplateComposesAndSurvivesSleep(t *testing.T) {
	d, s := testDaemon(t)
	d.buildAnswerConstructionConcept("en:greet:plain", "Hello, {name}.", "", taughtConstructionWeight)
	if err := d.registerGraphAnswerConstructions(); err != nil {
		t.Fatal(err)
	}
	if err := d.recompileGrammar(); err != nil {
		t.Fatal(err)
	}

	compose := func() (string, bool) {
		line, _, ok := d.gram.Compose("en", grammar.Message{Intent: "greet",
			Slots: grammar.TextSlots(map[string]string{"name": "world"})}, d.chem.Snapshot())
		return line, ok
	}
	if line, ok := compose(); !ok || line != "Hello, world." {
		t.Fatalf("the taught answer template should compose: %q (ok %v)", line, ok)
	}

	// It survives the night and still composes: a template taught into the graph is durable.
	if _, err := s.Sleep(d.cfg.NightlyDecay, d.cfg.PruneFloor); err != nil {
		t.Fatal(err)
	}
	if err := d.recompileGrammar(); err != nil {
		t.Fatal(err)
	}
	if line, ok := compose(); !ok || line != "Hello, world." {
		t.Fatalf("the taught answer template should survive the night and still compose: %q (ok %v)", line, ok)
	}
}
