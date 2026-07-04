package daemon

import (
	"strings"
	"testing"
)

// With nothing owed and curiosity up, the compute faculty experiments: it
// conjectures that two learned functions undo each other, composes the nested
// probes, runs them, and holds the discovered rule as a probable belief,
// spoken hedged, the honest reach of finite evidence.
func TestExperimentDiscoversInverse(t *testing.T) {
	d, _ := testDaemon(t)
	d.cfg.RuntimeEnabled = true
	say(t, d, "the double of a number is the product of the number and 2")
	say(t, d, "the halve of a number is the quotient of the number and 2")
	d.chem.SpikeDopamine(1)

	found := false
	for i := 0; i < 4 && !found; i++ {
		outcome, _ := d.actComputeNow()
		if strings.Contains(outcome, "inverse of the") && strings.Contains(strings.ToLower(outcome), "probab") {
			found = true
		}
	}
	if !found {
		t.Fatal("the experiments should discover an inverse pair among double and halve")
	}
	fc, _ := d.store.ConceptByTemplate("op:double")
	gc, _ := d.store.ConceptByTemplate("op:halve")
	if fc == nil || gc == nil {
		t.Fatal("the learned functions should have operation concepts")
	}
	if d.held.findID("inverse", fc.ID, gc.ID, heldInferred) == nil &&
		d.held.findID("inverse", gc.ID, fc.ID, heldInferred) == nil {
		t.Fatal("the discovered rule should be held as a belief")
	}
	if d.computeRuns == 0 {
		t.Fatal("the probes should count against the wake budget")
	}
}

// A conjecture the runs refute is put down for the wake and no belief is
// held: double and triple do not undo each other, and the mind moves on.
func TestExperimentRefutesNonInverse(t *testing.T) {
	d, _ := testDaemon(t)
	d.cfg.RuntimeEnabled = true
	say(t, d, "the double of a number is the product of the number and 2")
	say(t, d, "the triple of a number is the product of the number and 3")
	d.chem.SpikeDopamine(1)

	outcome, fruitful := d.actComputeNow()
	if !fruitful || !strings.Contains(outcome, "refuted") {
		t.Fatalf("a false conjecture should be refuted by the runs: %q", outcome)
	}
	fc, _ := d.store.ConceptByTemplate("op:double")
	gc, _ := d.store.ConceptByTemplate("op:triple")
	if fc != nil && gc != nil {
		if d.held.findID("inverse", fc.ID, gc.ID, heldInferred) != nil {
			t.Fatal("a refuted conjecture must hold no belief")
		}
	}
	// The pair is not worried again this wake.
	if _, fruitful := d.actComputeNow(); fruitful {
		if d.triedConjectures == nil || len(d.triedConjectures) < 2 {
			t.Fatal("the wake should move to a fresh conjecture, not repeat the refuted pair")
		}
	}
}
