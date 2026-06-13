package daemon

import (
	"testing"
	"time"

	"golem/internal/attention"
	"golem/internal/senses"
)

// The done-when for attention-grounding: a name told while a category is
// only held in mind, with nothing perceptual present, grounds in that
// category rather than in nothing. Naming follows what is attended, not
// only what is sensed.
func TestAttentionGroundsToldWord(t *testing.T) {
	d, s := testDaemon(t)
	base := time.Now().Add(-2 * time.Hour)
	for i := range 4 {
		d.recent = nil
		d.process(senses.Observation{Channel: "vision", Content: "a four legged animal",
			At: base.Add(time.Duration(i) * time.Minute)}, true, true)
	}
	d.recent = nil
	if _, err := s.MergeDuplicates(); err != nil {
		t.Fatal(err)
	}
	calm(d)
	d.growConcepts()
	animal, _ := s.ConceptByTemplate(attention.Normalize("a four legged animal"))
	if animal == nil {
		t.Fatal("the category should distil into a concept")
	}

	// Hold the category in mind, nothing perceptual present, and tell a new
	// name for it.
	d.ws.clear()
	d.recent = nil
	d.ws.excite(animal.ID, exciteRecall)
	say(t, d, "beast")

	// The naming pass bonds the told word to what was attended.
	calm(d)
	d.growConcepts()
	if c := d.resolveConcept("beast"); c == nil || c.ID != animal.ID {
		t.Fatalf("the told name should ground in the attended category: %+v", c)
	}
}

// A word warmed by accident in one scene does not stick: with the binding
// seen only once, cross-situational reliability lets it fade rather than
// becoming a new junk name.
func TestAttentionGroundingNeedsRecurrence(t *testing.T) {
	d, s := testDaemon(t)
	base := time.Now().Add(-2 * time.Hour)
	for i := range 4 {
		d.recent = nil
		d.process(senses.Observation{Channel: "vision", Content: "a four legged animal",
			At: base.Add(time.Duration(i) * time.Minute)}, true, true)
	}
	d.recent = nil
	if _, err := s.MergeDuplicates(); err != nil {
		t.Fatal(err)
	}
	calm(d)
	d.growConcepts()
	animal, _ := s.ConceptByTemplate(attention.Normalize("a four legged animal"))
	if animal == nil {
		t.Fatal("the category should distil")
	}

	// One stray word said while the category happened to be warm.
	d.ws.clear()
	d.recent = nil
	d.ws.excite(animal.ID, exciteRecall)
	say(t, d, "beast")
	calm(d)
	d.growConcepts()

	// Now the word recurs in scenes that have nothing to do with the
	// category; the accidental bond weakens instead of hardening.
	for range 3 {
		d.ws.clear()
		d.recent = nil
		say(t, d, "beast elsewhere entirely")
		calm(d)
		d.growConcepts()
	}
	if into, err := s.LinksToRel([]uint64{animal.ID}, "names"); err == nil {
		for _, e := range into {
			if e.Weight > 0.8 {
				t.Fatalf("an unrepeated accidental binding must not harden into a firm name: %.2f", e.Weight)
			}
		}
	}
}
