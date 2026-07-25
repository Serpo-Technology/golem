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

// The naming recount reads the pointing too. A word shown one thing while a scene-mate
// shared every window keeps its bond to what it was shown and loses the one it merely
// stood beside, which is what decides the concept a bare noun resolves to.
func TestOstensiveNamingDropsTheSceneMateBond(t *testing.T) {
	d, s := testDaemon(t)
	base := time.Now().Add(-3 * time.Hour)
	apple := feedShape(t, s, "vision", "a round red apple", 6, base)
	leaf := feedShape(t, s, "vision", "a green oak leaf", 6, base.Add(time.Minute))
	if _, err := s.MergeDuplicates(); err != nil {
		t.Fatal(err)
	}
	calm(d)
	d.growConcepts()
	appleC, _ := s.ConceptByTemplate(attention.Normalize("a round red apple"))
	leafC, _ := s.ConceptByTemplate(attention.Normalize("a green oak leaf"))
	if appleC == nil || leafC == nil {
		t.Fatalf("both shapes should distil: apple=%v leaf=%v", appleC, leafC)
	}
	// "apple" is said with both in the window every time, so company alone cannot tell
	// which it names; it was shown the apple.
	for i := range 5 {
		d.mintWords("apple", "en", 0, []recentMoment{apple[i], leaf[i]}, false)
	}
	calm(d)
	if err := s.SetLinkRel(wordID(t, s, "apple"), groundedEpisode(t, s, appleC.ID), memory.RelOstends, linkWeight); err != nil {
		t.Fatal(err)
	}
	d.cfg.OstensiveNamingEnabled = true
	d.growConcepts()
	wid := wordID(t, s, "apple")
	if w := edgeWeightRel(t, s, wid, leafC.ID, memory.RelNames); w >= namesWriteFloor {
		t.Fatalf("the scene-mate it was never shown must not stay named, got %.3f", w)
	}
	if w := edgeWeightRel(t, s, wid, appleC.ID, memory.RelNames); w < namesWriteFloor {
		t.Fatalf("the percept it was shown must keep its name, got %.3f", w)
	}
}

// A pointing is one perishable edge to one episode, so a word taught two exemplars of a
// kind months apart may hold the pointing for the later one alone. That gap is not
// evidence it never named the earlier one, so the gate asks whether the word points at
// the KIND, not at the individual: a second glove must not evict the first.
func TestOstensiveNamingKeepsAnOlderExemplarOfTheSameKind(t *testing.T) {
	d, s := testDaemon(t)
	base := time.Now().Add(-4 * time.Hour)
	leather := feedShape(t, s, "vision", "a warm leather glove", 6, base)
	batting := feedShape(t, s, "vision", "a padded batting glove", 6, base.Add(time.Minute))
	pot := feedShape(t, s, "vision", "a round clay pot", 6, base.Add(2*time.Minute))
	if _, err := s.MergeDuplicates(); err != nil {
		t.Fatal(err)
	}
	calm(d)
	d.growConcepts()
	leatherC, _ := s.ConceptByTemplate(attention.Normalize("a warm leather glove"))
	potC, _ := s.ConceptByTemplate(attention.Normalize("a round clay pot"))
	battingC, _ := s.ConceptByTemplate(attention.Normalize("a padded batting glove"))
	if leatherC == nil || potC == nil || battingC == nil {
		t.Fatal("all three shapes should distil")
	}
	// The word kept company with both gloves and with the pot that shared their windows.
	for i := range 5 {
		d.mintWords("eldiven", "tr", 0, []recentMoment{leather[i], batting[i], pot[i]}, false)
	}
	calm(d)
	d.growConcepts()
	// Only the later glove's pointing survives, the way an older episode's does not.
	if err := s.SetLinkRel(wordID(t, s, "eldiven"), groundedEpisode(t, s, battingC.ID), memory.RelOstends, linkWeight); err != nil {
		t.Fatal(err)
	}
	wid := wordID(t, s, "eldiven")
	gloveBefore := edgeWeightRel(t, s, wid, leatherC.ID, memory.RelNames)
	potBefore := edgeWeightRel(t, s, wid, potC.ID, memory.RelNames)
	if gloveBefore < namesWriteFloor || potBefore < namesWriteFloor {
		t.Fatalf("setup: both should start out named, glove %.3f pot %.3f", gloveBefore, potBefore)
	}
	d.cfg.OstensiveNamingEnabled = true
	d.growConcepts()
	// The older glove keeps the ordinary recount, which reads its share against the
	// company the word was shown and so lifts it, rather than being written down.
	if w := edgeWeightRel(t, s, wid, leatherC.ID, memory.RelNames); w <= gloveBefore {
		t.Fatalf("the older glove of the same kind must keep its name, got %.3f (was %.3f)", w, gloveBefore)
	}
	// The pot is no kind of glove, so its bond is written down and fades out over the
	// next nights rather than surviving as a name.
	if w := edgeWeightRel(t, s, wid, potC.ID, memory.RelNames); w >= potBefore {
		t.Fatalf("the pot shared a window and is no glove: its bond must fall, got %.3f (was %.3f)", w, potBefore)
	}
}
