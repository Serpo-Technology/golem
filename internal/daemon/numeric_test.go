package daemon

import (
	"strings"
	"testing"
)

// What a gradable means in number is taught, trust-gated: the architect's
// telling marks the relation with the grounded comparison; a low-trust peer's
// is refused, since a flipped pole would invert arithmetic truth.
func TestNumericPoleTaughtAndTrustGated(t *testing.T) {
	d, _ := testDaemon(t)
	d.cfg.RuntimeEnabled = true
	// The gradable exists as a relation once an ordering has been told with it.
	say(t, d, "2 is bigger than 1")

	// A peer cannot set the pole.
	r := sayAs(t, d, "llm", "", "bigger is above")
	if !strings.Contains(r.Say, "architect") {
		t.Fatalf("a low-trust speaker should be refused the pole: %q", r.Say)
	}
	if _, ok := d.numericPole("bigger"); ok {
		t.Fatal("a refused telling must not mark the pole")
	}

	// The architect sets it, and the mark reads back.
	say(t, d, "bigger is above")
	sym, ok := d.numericPole("bigger")
	if !ok || sym != ">" {
		t.Fatalf("the taught pole should read back as the comparison symbol: %q %v", sym, ok)
	}
}

// A numeric comparison settles by computing: firm both ways from one landed
// difference, the false direction denied on record, and no second program for
// the converse asking.
func TestNumericComparisonComputedAndShared(t *testing.T) {
	d, s := testDaemon(t)
	d.cfg.RuntimeEnabled = true
	say(t, d, "2 is bigger than 1")
	say(t, d, "bigger is above")

	yes := say(t, d, "is 17 bigger than 12").Say
	if !strings.Contains(strings.ToLower(yes), "yes") {
		t.Fatalf("a true numeric comparison should answer firm yes: %q", yes)
	}
	// One difference landed; the converse asking reads its sign, no new run.
	if _, err := s.FragmentByTemplate("computed:subtract:17:12"); err != nil {
		t.Fatal(err)
	}
	d.cfg.RuntimeEnabled = false // any further need to run would now fail
	no := say(t, d, "is 12 bigger than 17").Say
	if !strings.Contains(strings.ToLower(no), "no") {
		t.Fatalf("the converse should answer firm no from the landed difference: %q", no)
	}

	// The false direction stands denied between the number concepts.
	rel := d.resolveRelation("bigger")
	if rel == nil {
		t.Fatal("the gradable should resolve to its relation")
	}
	twelve, _ := d.store.ConceptByTemplate("num:12")
	seventeen, _ := d.store.ConceptByTemplate("num:17")
	if twelve == nil || seventeen == nil {
		t.Fatal("the numerals should stand as number concepts")
	}
	if !d.propositionDenied(twelve, rel.Content, seventeen.ID) {
		t.Fatal("the refuted direction should be denied on record")
	}
}

// A peer whose numeric claim the run refutes is contradicted on record, and
// the nightly trust pass counts it against the source.
func TestComputedRefutationErodesTrust(t *testing.T) {
	d, _ := testDaemon(t)
	d.cfg.RuntimeEnabled = true
	say(t, d, "2 is bigger than 1")
	say(t, d, "bigger is above")

	// The peer claims a falsehood; the claim lands on the number concepts.
	sayAs(t, d, "llm", "", "12 is bigger than 17")
	llm, err := d.ensureSource("llm")
	if err != nil {
		t.Fatal(err)
	}
	before := d.sourceTrust(llm.ID)

	// The check refutes it: asked, GOLEM computes and denies the direction.
	no := say(t, d, "is 12 bigger than 17").Say
	if !strings.Contains(strings.ToLower(no), "no") {
		t.Fatalf("the run should refute the claim: %q", no)
	}

	d.growSourceTrust()
	after := d.sourceTrust(llm.ID)
	if after >= before {
		t.Fatalf("a claim the run refuted should erode the source's trust: %.2f then %.2f", before, after)
	}
}

// An unmarked gradable falls through: numerals compared on a word never taught
// a pole take the told-ordering path exactly as before.
func TestUnmarkedGradableFallsThrough(t *testing.T) {
	d, s := testDaemon(t)
	d.cfg.RuntimeEnabled = true
	say(t, d, "2 is heavier than 1")
	say(t, d, "is 17 heavier than 12")
	if _, err := s.FragmentByTemplate("computed:subtract:17:12"); err == nil {
		if f, _ := s.FragmentByTemplate("computed:subtract:17:12"); f != nil {
			t.Fatal("an unmarked gradable must not reach the runtime")
		}
	}
	if _, ok := d.numericPole("heavier"); ok {
		t.Fatal("no pole was taught for heavier")
	}
}
