package daemon

import (
	"testing"
	"time"

	"golem/internal/attention"
	"golem/internal/memory"
)

// smearedCategorySetup builds the failure the ostensive core answers: three percepts, a
// category word taught over two of them, and a third that merely kept the word's company
// because it was still in the room when the word was said. The word's naming company
// therefore spans all three while its ostension covers only the two it was shown over.
// Setup stops before the abstraction pass so a caller can run it with the gate either way.
func smearedCategorySetup(t *testing.T) (d *Daemon, s *memory.Store, wid, catC, dogC, potC uint64) {
	t.Helper()
	d, s = testDaemon(t)
	base := time.Now().Add(-3 * time.Hour)
	cat := feedShape(t, s, "vision", "a small black cat", 4, base)
	dog := feedShape(t, s, "vision", "a small brown dog", 4, base.Add(time.Minute))
	pot := feedShape(t, s, "vision", "a round clay pot", 4, base.Add(2*time.Minute))
	if _, err := s.MergeDuplicates(); err != nil {
		t.Fatal(err)
	}
	calm(d)
	d.growConcepts()
	concept := func(caption string) uint64 {
		c, _ := s.ConceptByTemplate(attention.Normalize(caption))
		if c == nil {
			t.Fatalf("%q should distil into a concept", caption)
		}
		return c.ID
	}
	catC, dogC, potC = concept("a small black cat"), concept("a small brown dog"), concept("a round clay pot")
	// The category was said over each member, and the pot was in the room each time.
	for i := range 3 {
		d.mintWords("animal", "en", 0, []recentMoment{cat[i], dog[i], pot[i]}, false)
	}
	calm(d)
	d.growConcepts()
	wid = wordID(t, s, "animal")
	// Ostension records only what the word was pointed at: the two members, never the
	// scene-mate that was merely present.
	for _, c := range []uint64{catC, dogC} {
		if err := s.SetLinkRel(wid, groundedEpisode(t, s, c), memory.RelOstends, linkWeight); err != nil {
			t.Fatal(err)
		}
	}
	return
}

// grounds reports whether an abstraction grounds a concept.
func grounds(t *testing.T, s *memory.Store, absTpl string, member uint64) bool {
	t.Helper()
	abs, _ := s.ConceptByTemplate(absTpl)
	if abs == nil {
		return false
	}
	return edgeWeightRel(t, s, abs.ID, member, memory.RelGrounds) > 0
}

// The gate is load-bearing: with it off, the scene-mate the word was never pointed at
// joins the category on company alone. This is the defect, guarded so the fix cannot
// quietly become a no-op.
func TestSmearedCompanyAdmitsSceneMateWithoutOstensiveCore(t *testing.T) {
	d, s, _, _, _, potC := smearedCategorySetup(t)
	d.growAbstractions()
	if !grounds(t, s, "abs:en:animal", potC) {
		t.Fatal("without the ostensive core the scene-mate should join the category on company alone")
	}
}

// A percept the category word was never pointed at is a scene-mate, not a member: with
// the gate on, the category grounds the two it was shown over and drops the third.
func TestOstensiveCoreKeepsOnlyWhatTheWordWasPointedAt(t *testing.T) {
	d, s, _, catC, dogC, potC := smearedCategorySetup(t)
	d.cfg.OstensiveCoreEnabled = true
	d.growAbstractions()
	if grounds(t, s, "abs:en:animal", potC) {
		t.Fatal("a percept the word was never pointed at must not be a member")
	}
	for _, m := range []struct {
		id   uint64
		what string
	}{{catC, "cat"}, {dogC, "dog"}} {
		if !grounds(t, s, "abs:en:animal", m.id) {
			t.Fatalf("the %s was pointed at and must stay a member", m.what)
		}
	}
}

// A word that was never shown a thing keeps the company reading unchanged. A category
// carried entirely by talk earns no ostension, so gating on it would dissolve categories
// that are perfectly well taught; the gate stands aside for them.
func TestOstensiveCoreLeavesAnUnpointedWordToCompany(t *testing.T) {
	off, sOff, _, _, _, potC := smearedCategorySetup(t)
	// Drop the ostension so the word points at nothing at all.
	if err := sOff.DropRelFrom([]uint64{wordID(t, sOff, "animal")}, memory.RelOstends); err != nil {
		t.Fatal(err)
	}
	off.cfg.OstensiveCoreEnabled = true
	off.growAbstractions()
	if !grounds(t, sOff, "abs:en:animal", potC) {
		t.Fatal("a word that points at nothing must keep the company reading, gate or no gate")
	}
}

// An abstraction cannot be pointed at: there is no percept to show for a class, so it
// earns no ostension however faithfully it was taught. It keeps the company bar, so the
// backbone a category builds over its narrower categories survives the gate. Without this
// exemption the gate would cut animal's edge to mammal and take "is a mammal an animal"
// down with it.
func TestOstensiveCoreKeepsAbstractionMembers(t *testing.T) {
	d, s, wid, catC, dogC, _ := smearedCategorySetup(t)
	// A narrower category standing among the members, grounded in one of them and named
	// by a word of its own, the shape growAbstractions leaves behind for a kind.
	mammal := &memory.Fragment{
		Kind: memory.KindConcept, Channel: "concept", Content: "mammal",
		Template: "abs:en:mammal", Category: "noun", Time: time.Now(),
	}
	if _, err := s.Encode(mammal); err != nil {
		t.Fatal(err)
	}
	if err := s.SetLinkRel(mammal.ID, catC, memory.RelGrounds, 0.5); err != nil {
		t.Fatal(err)
	}
	// The category word kept the kind's company directly, the way a word said with a
	// class in mind reaches it, but pointed at no percept of it: a class has none.
	if err := s.SetLinkRel(wid, mammal.ID, memory.RelTemporal, linkWeight); err != nil {
		t.Fatal(err)
	}
	d.cfg.OstensiveCoreEnabled = true
	d.growAbstractions()
	if !grounds(t, s, "abs:en:animal", mammal.ID) {
		t.Fatal("an abstraction member earns no ostension and must keep the company bar")
	}
	if !grounds(t, s, "abs:en:animal", dogC) {
		t.Fatal("the exemption must not disturb the percept members beside it")
	}
}
