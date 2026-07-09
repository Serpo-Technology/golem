package daemon

import (
	"strings"
	"testing"
	"time"

	"golem/internal/memory"
)

// derivedPatients collects the patients of the derived facts worked out about an agent, for tests
// that check a conclusion was drawn without pinning a relation's exact learned lemma.
func derivedPatients(t *testing.T, d *Daemon, agent uint64) map[uint64]string {
	t.Helper()
	out := map[uint64]string{}
	for _, f := range d.verifiedDerivedAbout(agent) {
		out[f.Patient] = f.Kind
	}
	return out
}

// teachOutrankChain tells a told-transitive relation and a two-step ordering, so the closure
// (cat outranks mouse) is a sound conclusion never directly told.
func teachOutrankChain(t *testing.T, d *Daemon) (cat, dog, mouse *memory.Fragment) {
	t.Helper()
	cat = groundNoun(t, d, "cat")
	dog = groundNoun(t, d, "dog")
	mouse = groundNoun(t, d, "mouse")
	say(t, d, "the cat outranked the dog")
	say(t, d, "the dog outranked the mouse")
	say(t, d, "outrank is transitive")
	return cat, dog, mouse
}

// The nightly derivation works out and remembers the transitive closure the day newly enables: told
// cat > dog and dog > mouse on a transitive relation, GOLEM keeps "cat outranks mouse" as a firm
// derived fact, and does NOT keep the directly told orderings (those are observed, not derived).
func TestDeriveFirmStoresTransitiveClosure(t *testing.T) {
	d, s := testDaemon(t)
	cat, dog, mouse := teachOutrankChain(t, d)

	if n := d.deriveFirmAbout(cat, deriveCapPerNight); n != 1 {
		t.Fatalf("the cat's one new sound conclusion (it outranks the mouse) should be drawn, got %d", n)
	}
	if has, _ := s.HasDerived("outrank", cat.ID, mouse.ID); !has {
		t.Fatal("the transitive closure cat -> mouse should be remembered as a derived fact")
	}
	// The directly told orderings are observed, not derived: they must not enter the store.
	if has, _ := s.HasDerived("outrank", cat.ID, dog.ID); has {
		t.Fatal("a directly told ordering is observed, not a derived fact")
	}
	if has, _ := s.HasDerived("outrank", dog.ID, mouse.ID); has {
		t.Fatal("a directly told ordering is observed, not a derived fact")
	}
	// Idempotent: a second night does not re-draw what is already known.
	if n := d.deriveFirmAbout(cat, deriveCapPerNight); n != 0 {
		t.Fatalf("an already-derived fact should not be re-drawn, got %d", n)
	}
}

// A derived fact is surfaced firm and labeled as reasoned: it answers a direct question plainly
// (byte-identical to the live deduction), the census lists it as worked out, a description reasons it,
// and the epistemic probes agree -- "why" says it was reasoned, "how sure" says certain.
func TestDerivedFactSurfacesFirmAndReasoned(t *testing.T) {
	d, _ := testDaemon(t)
	cat, _, _ := teachOutrankChain(t, d)
	d.deriveFirmAbout(cat, deriveCapPerNight)

	// Direct question: firm, and unchanged by the store (assessProposition recomputes it).
	if got := say(t, d, "did the cat outrank the mouse").Say; got != "Yes, the cat outranked the mouse." {
		t.Fatalf("the sound deduction should answer firm: %q", got)
	}
	// Why: reasoned, not seen.
	if got := say(t, d, "why do you say that").Say; !strings.Contains(got, "reasoned") {
		t.Fatalf("why should own the fact as reasoned: %q", got)
	}
	// How sure: certain and reasoned, not "a guess" (the consistency the fix restores). The exact
	// string is pinned so the reasoning surface's own period is not doubled.
	if got := say(t, d, "how sure are you").Say; got != "I am certain. I reasoned so." {
		t.Fatalf("how sure about a sound deduction should read certain and reasoned: %q", got)
	}
	// Census: the worked-out fact is shown, firm and self-labeled as reasoned.
	sum := say(t, d, "what do you know").Say
	if !strings.Contains(sum, "worked out") || !strings.Contains(sum, "mouse") {
		t.Fatalf("the census should show the reasoned fact about the mouse: %q", sum)
	}
	// Describe: the reasoned tier voices it behind a reasoning lead.
	desc := say(t, d, "tell me about the cat").Say
	if !strings.Contains(desc, "reasoned") || !strings.Contains(desc, "mouse") {
		t.Fatalf("a description should reason the derived fact: %q", desc)
	}
}

// A derived fact dies with its premise: retract an ordering the chain rested on and the conclusion
// stops being firm (re-verified live), the census drops it, and the nightly re-verify prunes the row.
// Honesty never depended on a cascade -- the direct answer was always recomputed.
func TestDerivedFactCascadesOnRetraction(t *testing.T) {
	d, s := testDaemon(t)
	cat, _, mouse := teachOutrankChain(t, d)
	d.deriveFirmAbout(cat, deriveCapPerNight)
	if has, _ := s.HasDerived("outrank", cat.ID, mouse.ID); !has {
		t.Fatal("setup: the closure should be derived")
	}

	// Retract a premise the chain rests on.
	say(t, d, "the dog did not outrank the mouse")

	// The direct question is no longer firm: the deduction is recomputed and no longer holds.
	if got := say(t, d, "did the cat outrank the mouse").Say; strings.HasPrefix(got, "Yes") {
		t.Fatalf("with a premise retracted the chain must not answer firm: %q", got)
	}
	// The census no longer shows it (re-verify on read drops the stale conclusion).
	if sum := say(t, d, "what do you know").Say; strings.Contains(sum, "outranked the mouse") {
		t.Fatalf("the census must not show a conclusion whose premise was retracted: %q", sum)
	}
	if len(d.verifiedDerived()) != 0 {
		t.Fatal("the re-verified ledger should be empty once the premise is gone")
	}
	// The nightly re-verify prunes the stale row from the store.
	if dropped := d.reverifyDerived(); dropped != 1 {
		t.Fatalf("the nightly re-verify should prune the one stale row, dropped %d", dropped)
	}
	if has, _ := s.HasDerived("outrank", cat.ID, mouse.ID); has {
		t.Fatal("the stale derived row should be pruned")
	}
}

// A categorical composition is worked out and voiced: the dog chased a cat and a cat is an animal, so
// the dog chased an animal, a firm generalization spoken as reasoned.
func TestDeriveFirmCategorical(t *testing.T) {
	d, s := testDaemon(t)
	dog := groundNoun(t, d, "dog")
	cat := groundNoun(t, d, "cat")
	animal := &memory.Fragment{Kind: memory.KindConcept, Channel: "concept", Content: "animal",
		Template: "abs:en:animal", Category: "noun", Limbic: d.chem.Snapshot(), Time: time.Now()}
	if _, err := s.Encode(animal); err != nil {
		t.Fatal(err)
	}
	animalW, _, err := s.WordNode("animal", "en", d.chem.Snapshot(), time.Now())
	if err != nil {
		t.Fatal(err)
	}
	s.SetLinkRel(animalW.ID, animal.ID, memory.RelNames, 0.8)
	s.SetLinkRel(animal.ID, cat.ID, memory.RelGrounds, 0.5) // the animal category grounds in the cat
	say(t, d, "the dog chased the cat")

	d.deriveFirmAbout(dog, deriveCapPerNight)
	if kind, ok := derivedPatients(t, d, dog.ID)[animal.ID]; !ok || kind != derivedCategorical {
		t.Fatalf("the dog chasing an animal should be a categorical derived fact: kind=%q ok=%v", kind, ok)
	}
	if sum := say(t, d, "what do you know").Say; !strings.Contains(sum, "worked out") || !strings.Contains(sum, "animal") {
		t.Fatalf("the census should show the categorical fact: %q", sum)
	}
}

// A symmetric relation's reverse is worked out: told the cat matched the dog and match is symmetric,
// GOLEM remembers the dog matched the cat.
func TestDeriveFirmSymmetric(t *testing.T) {
	d, _ := testDaemon(t)
	cat := groundNoun(t, d, "cat")
	dog := groundNoun(t, d, "dog")
	say(t, d, "the cat matched the dog")
	say(t, d, "match is symmetric")

	d.deriveFirmAbout(dog, deriveCapPerNight)
	if kind, ok := derivedPatients(t, d, dog.ID)[cat.ID]; !ok || kind != derivedSymmetric {
		t.Fatalf("the reverse of the symmetric fact should be worked out: kind=%q ok=%v", kind, ok)
	}
	// It answers firm and reasoned through the same live path.
	if got := say(t, d, "did the dog match the cat").Say; !strings.HasPrefix(got, "Yes") {
		t.Fatalf("the symmetric reverse should answer firm: %q", got)
	}
}

// A containment chain is worked out but kept out of the verbal census: the leaf is on the tree and the
// tree is in the forest, so the leaf is in the forest, remembered and queryable but not spoken as a verb.
func TestDeriveFirmContainment(t *testing.T) {
	d, s := testDaemon(t)
	leaf := groundNoun(t, d, "leaf")
	groundNoun(t, d, "tree")
	forest := groundNoun(t, d, "forest")
	say(t, d, "the leaf is on the tree")
	say(t, d, "the tree is in the forest")

	d.deriveFirmAbout(leaf, deriveCapPerNight)
	if has, _ := s.HasDerived("in", leaf.ID, forest.ID); !has {
		t.Fatal("the leaf being in the forest should be worked out")
	}
	if kind := derivedPatients(t, d, leaf.ID)[forest.ID]; kind != derivedContainment {
		t.Fatalf("the containment should be recorded as such: %q", kind)
	}
	// The containment answers firm directly, but does not verbalize in the census (it reads as a place).
	if got := say(t, d, "is the leaf in the forest").Say; !strings.HasPrefix(got, "Yes") {
		t.Fatalf("the composed containment should answer firm: %q", got)
	}
	if sum := say(t, d, "what do you know").Say; strings.Contains(sum, "worked out") && strings.Contains(sum, "forest") {
		t.Fatalf("a containment should stay out of the verbal census: %q", sum)
	}
}

// The store is an index, never the authority: assessProposition never reads it, so a row that does
// not actually derive can neither be spoken firm nor firm another conclusion. A bogus row is filtered
// out on read and pruned nightly.
func TestDerivedStoreIsNotAuthoritative(t *testing.T) {
	d, s := testDaemon(t)
	cat, _, mouse := teachOutrankChain(t, d)
	rabbit := groundNoun(t, d, "rabbit") // the cat stands in no relation to the rabbit

	// Plant a row the graph does not support.
	if err := s.SaveDerived(memory.DerivedFact{Rel: "outrank", Agent: cat.ID, Patient: rabbit.ID,
		AgentGloss: "cat", PatGloss: "rabbit", Kind: derivedTransitive}); err != nil {
		t.Fatal(err)
	}
	// A direct question is answered by the live deduction, which does not consult the store.
	if got := say(t, d, "did the cat outrank the rabbit").Say; strings.HasPrefix(got, "Yes") {
		t.Fatalf("a bogus derived row must not make a question answer firm: %q", got)
	}
	// The re-verify filters it out, and the nightly prune removes it.
	for _, f := range d.verifiedDerived() {
		if f.Patient == rabbit.ID {
			t.Fatal("a row that does not derive must not survive re-verify")
		}
	}
	d.reverifyDerived()
	if has, _ := s.HasDerived("outrank", cat.ID, rabbit.ID); has {
		t.Fatal("the bogus row should be pruned")
	}
	// The genuine closure still verifies.
	if !d.derivesStill(memory.DerivedFact{Rel: "outrank", Agent: cat.ID, Patient: mouse.ID}) {
		t.Fatal("the genuine closure should still verify")
	}
}

// The derived store never changes a direct answer: a transitive question answers identically whether
// or not the nightly derivation has run, because the store is not read on the answering path.
func TestDirectAnswerUnchangedByDerivedStore(t *testing.T) {
	// Two fresh brains asked once each: one whose nightly derivation has run, one whose has not.
	d1, _ := testDaemon(t)
	teachOutrankChain(t, d1)
	without := say(t, d1, "did the cat outrank the mouse").Say

	d2, _ := testDaemon(t)
	cat, _, _ := teachOutrankChain(t, d2)
	d2.deriveFirmAbout(cat, deriveCapPerNight)
	with := say(t, d2, "did the cat outrank the mouse").Say

	if without != with || with != "Yes, the cat outranked the mouse." {
		t.Fatalf("the derived store must not change the direct answer: without=%q with=%q", without, with)
	}
}
