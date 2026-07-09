package daemon

import (
	"strings"
	"testing"

	"golem/internal/memory"
)

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
