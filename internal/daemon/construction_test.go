package daemon

import (
	"strings"
	"testing"
	"time"

	"golem/internal/grammar"
	"golem/internal/memory"
)

// Step 21: the introspection report is built from a construction read out
// of memory. A second way to say it can be taught and is then chosen, and
// forgetting it reverts the line, the engine still speaking, so a removed
// construction changes how GOLEM says a thing without breaking anything.
func TestIntrospectConstructionTaughtAndForgotten(t *testing.T) {
	d, s := testDaemon(t)
	d.lastThought = "thought: after make test, make deploy"
	d.lastThoughtAt = time.Now()

	if got := d.introspectLine(); got != "I was thinking: after make test, make deploy" {
		t.Fatalf("the seeded construction should report the thought: %q", got)
	}

	// The constructions are nodes in the same store as everything else.
	cnodes, err := s.NodesByKind(memory.KindConstruction)
	if err != nil {
		t.Fatal(err)
	}
	if len(cnodes) == 0 {
		t.Fatal("constructions should be inspectable nodes")
	}

	// A taught construction outweighs the seed and becomes the way it speaks.
	if err := d.teachConstruction("introspect", "my mind was on {thought}", "en"); err != nil {
		t.Fatal(err)
	}
	if got := d.introspectLine(); got != "My mind was on after make test, make deploy" {
		t.Fatalf("a taught construction should be chosen: %q", got)
	}

	// Forgetting it reverts to the seed; the engine never broke.
	if _, err := s.DeleteByTemplate("cxn:en:introspect:taught0"); err != nil {
		t.Fatal(err)
	}
	if err := d.recompileGrammar(); err != nil {
		t.Fatal(err)
	}
	if got := d.introspectLine(); got != "I was thinking: after make test, make deploy" {
		t.Fatalf("forgetting the taught construction should revert the line: %q", got)
	}
}

// The succession answer is built from a construction, and removing every
// way to say it falls back to the innate realizer rather than breaking, so
// the engine is the floor under the learned patterns.
func TestSuccessionConstructionWithEngineFallback(t *testing.T) {
	d, s := testDaemon(t)
	feedSequence(t, d, "run: make test", "run: make deploy", 3, time.Now().Add(-3*time.Hour))

	if got := say(t, d, "what usually happens after make test").Say; got != "After make test, make deploy usually follows." {
		t.Fatalf("the answer should come from the construction: %q", got)
	}

	cnodes, err := s.NodesByKind(memory.KindConstruction)
	if err != nil {
		t.Fatal(err)
	}
	forgot := false
	for _, n := range cnodes {
		if strings.HasPrefix(n.Template, "cxn:en:succession_after:") {
			if _, err := s.DeleteByTemplate(n.Template); err != nil {
				t.Fatal(err)
			}
			forgot = true
		}
	}
	if !forgot {
		t.Fatal("the succession constructions should have been seeded")
	}
	if err := d.recompileGrammar(); err != nil {
		t.Fatal(err)
	}
	if got := say(t, d, "what usually happens after make test").Say; got != "After make test, make deploy usually follows." {
		t.Fatalf("the engine should still realize the answer with no construction: %q", got)
	}
}

// The definition answer is built from a construction too, and a taught
// alternative is chosen the next time the same thing is defined.
func TestDefinitionConstructionTaught(t *testing.T) {
	d, s := testDaemon(t)
	c := &memory.Fragment{Kind: memory.KindConcept, Channel: "concept", Content: "brass bell",
		Template: "brass bell", Category: "noun", Limbic: d.chem.Snapshot(), Time: time.Now()}
	if _, err := s.Encode(c); err != nil {
		t.Fatal(err)
	}

	if got := say(t, d, "what is brass bell").Say; !strings.Contains(got, "Brass bell is a thing.") {
		t.Fatalf("the definition should be built from a construction: %q", got)
	}

	if err := d.teachConstruction("definition", "{subject} seems to be {kind}.", "en"); err != nil {
		t.Fatal(err)
	}
	if got := say(t, d, "what is brass bell").Say; !strings.Contains(got, "Brass bell seems to be a thing") {
		t.Fatalf("a taught definition construction should be chosen: %q", got)
	}
}

// Step 22: the full definition answer is composed from the definition
// clause and the count clause, three constructions assembled into a
// sentence no single template holds, and the composition reads back as the
// constructions that made it.
func TestDefinitionComposedFromConstructions(t *testing.T) {
	d, s := testDaemon(t)
	now := time.Now()
	c := &memory.Fragment{Kind: memory.KindConcept, Channel: "concept", Content: "brass bell",
		Template: "brass bell", Category: "noun", Limbic: d.chem.Snapshot(), Time: now}
	if _, err := s.Encode(c); err != nil {
		t.Fatal(err)
	}
	e := &memory.Fragment{Kind: memory.KindEpisodic, Channel: "vision", Content: "brass bell",
		Template: "brass bell", Limbic: d.chem.Snapshot(), Time: now}
	if _, err := s.Encode(e); err != nil {
		t.Fatal(err)
	}
	if err := s.LinkRel(c.ID, e.ID, memory.RelGrounds, groundsWeight); err != nil {
		t.Fatal(err)
	}

	if got := say(t, d, "what is brass bell").Say; got != "Brass bell is a thing. I have seen it once." {
		t.Fatalf("the answer should compose the definition and the count: %q", got)
	}

	// The same composition reads back as the constructions that made it.
	line, trace, ok := d.gram.Compose("en", grammar.Message{Intent: "described", Slots: map[string]grammar.Filler{
		"defn": {Sub: &grammar.Message{Intent: "definition",
			Slots: grammar.TextSlots(map[string]string{"subject": "brass bell", "kind": "a thing"})}},
		"count": {Sub: &grammar.Message{Intent: "seen_count",
			Slots: grammar.TextSlots(map[string]string{"object": "it", "count": "once"})}},
	}}, d.chem.Snapshot())
	if !ok || line != "Brass bell is a thing. I have seen it once." {
		t.Fatalf("compose should build the same sentence: %q %v", line, ok)
	}
	made := map[string]bool{}
	for _, k := range trace {
		made[k] = true
	}
	for _, want := range []string{"definition:plain", "seen_count:plain", "described:full"} {
		if !made[want] {
			t.Fatalf("the composition should name %q among its constructions: %v", want, trace)
		}
	}
}
