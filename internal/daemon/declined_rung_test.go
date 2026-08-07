package daemon

import (
	"testing"
	"time"

	"golem/internal/memory"
)

// declinedRungSetup puts a class inside a category that the coverage test will turn down.
// The narrower class grounds only a percept the category does not hold, so nothing it
// covers falls under the category it sits in, which is the shape of every junk rung the
// pass stops refreshing: a part over a flower, a reptile over an animal.
func declinedRungSetup(t *testing.T) (d *Daemon, s *memory.Store, circle uint64) {
	t.Helper()
	d, s, _, _, _, potC := smearedCategorySetup(t)
	d.cfg.OstensiveCoreEnabled = true
	d.growAbstractions() // the class has to exist before a rung can sit inside it
	c := &memory.Fragment{Kind: memory.KindConcept, Channel: "concept", Content: "circle",
		Template: "abs:en:circle", Category: "noun", Time: time.Now()}
	if _, err := s.Encode(c); err != nil {
		t.Fatal(err)
	}
	if err := s.SetLinkRel(c.ID, potC, memory.RelGrounds, linkWeight); err != nil {
		t.Fatal(err)
	}
	animal, _ := s.ConceptByTemplate("abs:en:animal")
	if animal == nil {
		t.Fatal("the category should already have grown")
	}
	if err := s.SetLinkRel(animal.ID, c.ID, memory.RelGrounds, 0.4); err != nil {
		t.Fatal(err)
	}
	d.cfg.OstensiveCoreEnabled = true
	return d, s, c.ID
}

func rungWeight(t *testing.T, s *memory.Store, member uint64) float64 {
	t.Helper()
	animal, _ := s.ConceptByTemplate("abs:en:animal")
	if animal == nil {
		t.Fatal("the category should still be there")
	}
	return edgeWeightRel(t, s, animal.ID, member, memory.RelGrounds)
}

// The defect, guarded so the fix cannot quietly become a no-op: the pass declines the rung
// and then leaves it exactly where it was, to fade a fiftieth a night.
func TestDeclinedRungIsLeftStandingWithoutRetirement(t *testing.T) {
	d, s, circle := declinedRungSetup(t)
	d.cfg.RetireDeclinedRungsEnabled = false // the reading this replaced
	d.growAbstractions()
	if w := rungWeight(t, s, circle); w < 0.4 {
		t.Fatalf("without retirement a declined rung should be left untouched, got %.3f", w)
	}
}

// A rung the coverage test turns down is written down the same night, so the falsehood it
// carries stops being answered firmly within a night or two instead of seventy.
func TestDeclinedRungIsWrittenDown(t *testing.T) {
	d, s, circle := declinedRungSetup(t)
	d.cfg.RetireDeclinedRungsEnabled = true
	d.growAbstractions()
	w := rungWeight(t, s, circle)
	if w >= 0.4 {
		t.Fatalf("a declined rung must be written down, got %.3f", w)
	}
	if w <= 0 {
		t.Fatal("one night of refusal must take a rung down, not out: a thin core is not a refutation")
	}
	// Three nights of refusal carry it under the bar a firm answer climbs.
	for range 2 {
		d.growAbstractions()
	}
	if w := rungWeight(t, s, circle); w >= absPwcFloor {
		t.Fatalf("three nights of refusal should carry the rung under the membership bar, got %.3f", w)
	}
}

// Coverage alone does not bring a retired rung back. A class reaches a category only by
// being proposed as a member of its word's company in the first place, and the coverage
// test runs on what the company proposes; a rung that has fallen out is not reconsidered
// on the strength of what it grounds. Retirement is therefore recoverable by the ordinary
// road and by no other, which is the property this pins down so a later reading of the
// retirement does not assume more than it does.
func TestRetiredRungIsNotRewrittenByCoverageAlone(t *testing.T) {
	d, s, circle := declinedRungSetup(t)
	d.cfg.RetireDeclinedRungsEnabled = true
	for range 4 {
		d.growAbstractions()
	}
	animal, _ := s.ConceptByTemplate("abs:en:animal")
	members, err := s.LinksFromRel([]uint64{animal.ID}, memory.RelGrounds)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range members {
		if e.To != circle {
			if err := s.SetLinkRel(circle, e.To, memory.RelGrounds, linkWeight); err != nil {
				t.Fatal(err)
			}
		}
	}
	d.growAbstractions()
	if w := rungWeight(t, s, circle); w >= absPwcFloor {
		t.Fatalf("coverage alone should not restore a retired rung, got %.3f", w)
	}
}

// The rungs the pass still writes are the taxonomy backbone, and they are written every
// night at the floor when their share is thin. Retirement must not touch them.
func TestRetirementSparesTheRungsThePassStillWrites(t *testing.T) {
	d, s, _, catC, dogC, _ := smearedCategorySetup(t)
	d.cfg.OstensiveCoreEnabled = true
	d.cfg.RetireDeclinedRungsEnabled = true
	d.growAbstractions()
	for _, m := range []struct {
		id   uint64
		what string
	}{{catC, "cat"}, {dogC, "dog"}} {
		if !grounds(t, s, "abs:en:animal", m.id) {
			t.Fatalf("the %s is still covered and must keep its place", m.what)
		}
	}
}

// A percept member answers to the pointing, not to the coverage test, so retirement leaves
// it alone: taking it down here would punish it twice for the same absence.
func TestRetirementLeavesPerceptMembersAlone(t *testing.T) {
	d, s, _, _, _, potC := smearedCategorySetup(t)
	d.cfg.OstensiveCoreEnabled = true
	d.growAbstractions()
	animal, _ := s.ConceptByTemplate("abs:en:animal")
	if animal == nil {
		t.Fatal("the category should already have grown")
	}
	if err := s.SetLinkRel(animal.ID, potC, memory.RelGrounds, 0.4); err != nil {
		t.Fatal(err)
	}
	d.cfg.RetireDeclinedRungsEnabled = true
	d.growAbstractions()
	if w := edgeWeightRel(t, s, animal.ID, potC, memory.RelGrounds); w < 0.4 {
		t.Fatalf("a percept member is judged by the pointing, not by coverage, got %.3f", w)
	}
}

// The corroboration rule reads a percept: a thing that kept a word's company in one tongue
// alone, with no twin in the other to vouch for it, is a scene-mate and not a member. It
// exempts the co-members that have no percept company to corroborate by asking whether they
// are perceptless, and a grown class is not perceptless, since it grounds the percepts it
// was distilled from. So a class fell to a rule written for scene-mates, and in a brain
// taught two tongues it fell reliably: an English class is corroborated only by an
// other-tongue word's company, which an English-minted abstraction rarely enters. The word
// animal kept a bird among its firmest company and the rule struck it out, so the coverage
// test that decides these rungs never saw it at all.
func TestAClassIsNotJudgedByTheSceneMateRule(t *testing.T) {
	d, s, wid, catC, dogC, potC := smearedCategorySetup(t)
	d.cfg.OstensiveCoreEnabled = true
	d.growAbstractions()
	animal, _ := s.ConceptByTemplate("abs:en:animal")
	if animal == nil {
		t.Fatal("the category should already have grown")
	}
	// A class over what the category holds, in the category word's company, corroborated by
	// nothing in the other tongue.
	bird := &memory.Fragment{Kind: memory.KindConcept, Channel: "concept", Content: "bird",
		Template: "abs:en:bird", Category: "noun", Time: time.Now()}
	if _, err := s.Encode(bird); err != nil {
		t.Fatal(err)
	}
	// Over the members the category was pointed at, so the coverage test below has no
	// quarrel with it and the scene-mate rule is the only thing that can strike it out.
	for _, m := range []uint64{catC, dogC} {
		if err := s.SetLinkRel(bird.ID, m, memory.RelGrounds, linkWeight); err != nil {
			t.Fatal(err)
		}
		if err := s.SetLinkRel(animal.ID, m, memory.RelGrounds, linkWeight); err != nil {
			t.Fatal(err)
		}
	}
	_ = potC
	if err := s.SetLinkRel(wid, bird.ID, memory.RelTemporal, 0.95); err != nil {
		t.Fatal(err)
	}
	// An other-tongue word whose company is a subset of this one's, which is what makes the
	// English word a twin and arms the rule.
	tr := &memory.Fragment{Kind: memory.KindWord, Channel: "word", Content: "hayvan",
		Template: "hayvan", Lang: "tr", Time: time.Now()}
	if _, err := s.Encode(tr); err != nil {
		t.Fatal(err)
	}
	for _, m := range []uint64{catC, dogC} {
		if err := s.SetLinkRel(tr.ID, m, memory.RelTemporal, 0.9); err != nil {
			t.Fatal(err)
		}
	}
	d.cfg.RetireDeclinedRungsEnabled = true
	d.growAbstractions()
	if w := edgeWeightRel(t, s, animal.ID, bird.ID, memory.RelGrounds); w < absPwcFloor {
		t.Fatalf("a class covering the category must be judged by coverage, not struck out as a scene-mate, got %.3f", w)
	}
}

// The concept-side share asks what fraction of a class's naming company one word accounts
// for, so every word ever said near that class sits in the denominator. A word that heads a
// class of its own is standing for that one, and its company with a neighbour dilutes the
// neighbour's real namers: a mammal was kept company by fish and by moon, which took the
// word animal's share of the mammals just under the floor and cost the taxonomy its rung.
func TestANeighbourClassWordDoesNotDiluteTheShare(t *testing.T) {
	d, s, wid, catC, dogC, _ := smearedCategorySetup(t)
	d.cfg.OstensiveCoreEnabled = true
	d.growAbstractions()
	animal, _ := s.ConceptByTemplate("abs:en:animal")
	if animal == nil {
		t.Fatal("the category should already have grown")
	}
	mammal := &memory.Fragment{Kind: memory.KindConcept, Channel: "concept", Content: "mammal",
		Template: "abs:en:mammal", Category: "noun", Time: time.Now()}
	if _, err := s.Encode(mammal); err != nil {
		t.Fatal(err)
	}
	for _, m := range []uint64{catC, dogC} {
		if err := s.SetLinkRel(mammal.ID, m, memory.RelGrounds, linkWeight); err != nil {
			t.Fatal(err)
		}
		if err := s.SetLinkRel(animal.ID, m, memory.RelGrounds, linkWeight); err != nil {
			t.Fatal(err)
		}
	}
	if err := s.SetLinkRel(wid, mammal.ID, memory.RelTemporal, 0.2); err != nil {
		t.Fatal(err)
	}
	// A neighbour with a class of its own, said in the same room and standing for none of
	// it, heavy enough to swamp the denominator on its own.
	moon := &memory.Fragment{Kind: memory.KindWord, Channel: "word", Content: "moon",
		Template: "moon", Lang: "en", Time: time.Now()}
	if _, err := s.Encode(moon); err != nil {
		t.Fatal(err)
	}
	moonClass := &memory.Fragment{Kind: memory.KindConcept, Channel: "concept", Content: "moon",
		Template: "abs:en:moon", Category: "noun", Time: time.Now()}
	if _, err := s.Encode(moonClass); err != nil {
		t.Fatal(err)
	}
	if err := s.SetLinkRel(moon.ID, moonClass.ID, memory.RelTemporal, 0.5); err != nil {
		t.Fatal(err)
	}
	if err := s.SetLinkRel(moon.ID, mammal.ID, memory.RelTemporal, 5.0); err != nil {
		t.Fatal(err)
	}
	d.cfg.RetireDeclinedRungsEnabled = true
	d.growAbstractions()
	if w := edgeWeightRel(t, s, animal.ID, mammal.ID, memory.RelGrounds); w < absPwcFloor {
		t.Fatalf("a neighbour's company must not dilute the class's own namers, got %.3f", w)
	}
}
