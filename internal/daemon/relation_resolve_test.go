package daemon

import (
	"testing"
	"time"

	"golem/internal/memory"
)

// resolveRelation picks the relation a word names even when a spurious noun of
// the same form outweighs it in the naming vote, so a role question's verb is
// not shadowed by an abstraction the constellation pass distilled from the
// verb's arguments.
func TestResolveRelationPrefersTheRelation(t *testing.T) {
	d, s := testDaemon(t)
	now := time.Now()
	rel, _ := s.Encode(&memory.Fragment{Kind: memory.KindConcept, Content: "chase", Template: "rel:en:chase", Category: "relation", Time: now})
	noun, _ := s.Encode(&memory.Fragment{Kind: memory.KindConcept, Content: "chase", Template: "abs:en:chase", Category: "noun", Time: now})
	w, _, err := s.WordNode("chase", "en", d.chem.Snapshot(), now)
	if err != nil {
		t.Fatal(err)
	}
	// The noun is named more heavily, the way the vote would otherwise pick it.
	if err := s.SetLinkRel(w.ID, noun, memory.RelNames, 0.9); err != nil {
		t.Fatal(err)
	}
	if err := s.SetLinkRel(w.ID, rel, memory.RelNames, 0.5); err != nil {
		t.Fatal(err)
	}

	if got := d.resolveRelation("chase"); got == nil || got.ID != rel {
		t.Fatalf("resolveRelation should return the relation despite the heavier noun: %+v", got)
	}
	// Plain resolveConcept still follows the vote to the noun, which is why the
	// role question needs the relation-specific resolver.
	if c := d.resolveConcept("chase"); c == nil || c.ID != noun {
		t.Fatalf("resolveConcept should follow the naming vote to the noun: %+v", c)
	}
}

// A word whose strongest name is a word-echo shadow (an abstraction grounded
// only in utterance) resolves by attention to the perceptually grounded concept
// warm in mind: how a noun taught in a fresh tongue settles on the grounded
// concept across the tongue rather than its own shadow.
func TestResolveConceptPrefersWarmSenseOverShadow(t *testing.T) {
	d, s := testDaemon(t)
	now := time.Now()
	// The real concept: an abstraction grounded in a percept.
	vis := encodeAt(t, s, "vision", "a small brown dog", 3.0, now)
	dog, err := s.Encode(&memory.Fragment{Kind: memory.KindConcept, Content: "dog",
		Template: "abs:en:dog", Category: "noun", Time: now})
	if err != nil {
		t.Fatal(err)
	}
	if err := s.SetLinkRel(dog, vis.ID, memory.RelGrounds, 0.8); err != nil {
		t.Fatal(err)
	}
	// The word-echo: an abstraction grounded only in the saying of it.
	tell := encodeAt(t, s, "tell", "köpek nedir", 3.0, now)
	shadow, err := s.Encode(&memory.Fragment{Kind: memory.KindConcept, Content: "köpek",
		Template: "abs:tr:köpek", Category: "noun", Time: now})
	if err != nil {
		t.Fatal(err)
	}
	if err := s.SetLinkRel(shadow, tell.ID, memory.RelGrounds, 0.8); err != nil {
		t.Fatal(err)
	}
	w, _, err := s.WordNode("köpek", "tr", d.chem.Snapshot(), now)
	if err != nil {
		t.Fatal(err)
	}
	// The shadow is named more heavily, the way the cold vote would pick it.
	if err := s.SetLinkRel(w.ID, shadow, memory.RelNames, 0.9); err != nil {
		t.Fatal(err)
	}
	if err := s.SetLinkRel(w.ID, dog, memory.RelNames, 0.4); err != nil {
		t.Fatal(err)
	}

	if c := d.resolveConcept("köpek"); c == nil || c.ID != shadow {
		t.Fatalf("cold, the strongest name should win: %+v", c)
	}
	d.ws.excite(dog, exciteSpeech)
	if c := d.resolveConcept("köpek"); c == nil || c.ID != dog {
		t.Fatalf("warm, attention should resolve köpek to the grounded dog: %+v", c)
	}
}

// A word whose strongest name is itself a perceptually grounded concept is left
// alone: attention does not second-guess it toward whatever else is warm, even
// a co-argument the word was lightly polluted toward. The dog stays the dog when
// a rabbit it once co-occurred with is warm.
func TestResolveConceptKeepsGroundedSenseWhenOtherWarm(t *testing.T) {
	d, s := testDaemon(t)
	now := time.Now()
	dogVis := encodeAt(t, s, "vision", "a small brown dog", 3.0, now)
	dog, err := s.Encode(&memory.Fragment{Kind: memory.KindConcept, Content: "dog",
		Template: "abs:en:dog", Category: "noun", Time: now})
	if err != nil {
		t.Fatal(err)
	}
	if err := s.SetLinkRel(dog, dogVis.ID, memory.RelGrounds, 0.8); err != nil {
		t.Fatal(err)
	}
	rabbit := groundNoun(t, d, "rabbit")
	w, _, err := s.WordNode("dog", "en", d.chem.Snapshot(), now)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.SetLinkRel(w.ID, dog, memory.RelNames, 0.9); err != nil {
		t.Fatal(err)
	}
	// A light pollution edge toward the rabbit it co-occurred with.
	if err := s.SetLinkRel(w.ID, rabbit.ID, memory.RelNames, 0.3); err != nil {
		t.Fatal(err)
	}

	d.ws.excite(rabbit.ID, exciteSpeech)
	if c := d.resolveConcept("dog"); c == nil || c.ID != dog {
		t.Fatalf("a grounded sense should stand against a warm co-argument: %+v", c)
	}
}
