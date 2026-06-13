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

// Lateral inhibition lets a name reach a category that grounds in
// concepts, not episodes. Warming such a category spreads into its
// well-grounded members; without inhibition they take the attention slot
// and the name lands on a member. With it, the category holds the slot
// and the name binds to the category itself, the second level of
// abstraction attention-grounding alone could not build.
func TestLateralInhibitionGroundsNameOnCategory(t *testing.T) {
	d, s := testDaemon(t)
	disk, deploy := constellationDay(t, d, s)
	for i := range 3 {
		d.mintWords("trouble", "en", 0, []recentMoment{disk[i], deploy[i]}, false)
	}
	calm(d)
	d.growConcepts()
	d.growAbstractions()
	trouble, _ := s.ConceptByTemplate("abs:en:trouble")
	if trouble == nil {
		t.Fatal("the concept-grounded category should exist")
	}
	diskC, _ := s.ConceptByTemplate(attention.Normalize("Disk full on /dev/disk1s9"))
	deployC, _ := s.ConceptByTemplate(attention.Normalize("run: make deploy"))
	if diskC == nil || deployC == nil {
		t.Fatal("the member concepts should exist")
	}

	// The category and its members, warmed alike: inhibition must seat the
	// category above the members it subsumes.
	warmAll := func() {
		d.ws.clear()
		d.ws.excite(trouble.ID, exciteRecall)
		d.ws.excite(diskC.ID, exciteRecall)
		d.ws.excite(deployC.ID, exciteRecall)
	}
	warmAll()
	warm := d.attendedWarmest(1)
	if len(warm) != 1 || warm[0].id != trouble.ID {
		t.Fatalf("the attended category should outrank its members: %+v", warm)
	}

	// A name told across situations while the category is held in mind
	// grounds on the category, not on a member.
	for range 3 {
		warmAll()
		d.mintWords("mess", "en", 0, d.attentionAnchors(), false)
		calm(d)
	}
	d.growConcepts()
	d.growAbstractions()
	if c := d.resolveConcept("mess"); c == nil || c.ID != trouble.ID {
		t.Fatalf("the name should reach the attended category, not a member: %+v", c)
	}

	// With inhibition off, the same warming lets the members take the slot.
	d.cfg.LateralInhibition = 0
	warmAll()
	off := d.attendedWarmest(1)
	if len(off) == 1 && off[0].id == trouble.ID {
		t.Fatal("with inhibition off a member should be able to hold the slot")
	}
}
