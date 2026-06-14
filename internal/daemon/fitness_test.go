package daemon

import "testing"

// A change that lifts the grounded blend with no factor collapsing is kept, and
// the verdict names the outcome that earned it.
func TestFitnessKeepsImprovement(t *testing.T) {
	before := fitnessSnapshot{goalProgress: 0.4, prediction: 0.4, trust: 0.4}
	after := fitnessSnapshot{goalProgress: 0.6, prediction: 0.6, trust: 0.6}
	v := judgeFitness(before, after)
	if !v.improved {
		t.Fatalf("a blend that rose should be kept: %+v", v)
	}
	if v.reason == "" || v.delta <= 0 {
		t.Fatalf("a kept change should record why and by how much: %+v", v)
	}
}

// No single signal can be gamed into a degenerate optimum: a change that buys a
// higher blend by collapsing one grounded factor is reverted.
func TestFitnessGuardsAgainstGaming(t *testing.T) {
	before := fitnessSnapshot{goalProgress: 0.5, prediction: 0.5, trust: 0.5}
	// Trust spikes and lifts the blend, but goal progress collapses.
	after := fitnessSnapshot{goalProgress: 0.2, prediction: 0.5, trust: 1.0}
	if after.score() <= before.score() {
		t.Fatal("the test should pose a higher blend, or the guard is not exercised")
	}
	v := judgeFitness(before, after)
	if v.improved {
		t.Fatalf("a collapsed factor should fail the change despite the higher blend: %+v", v)
	}
}

// A change that moves nothing is not an improvement and is reverted.
func TestFitnessRevertsNoChange(t *testing.T) {
	s := fitnessSnapshot{goalProgress: 0.5, prediction: 0.5, trust: 0.5}
	if v := judgeFitness(s, s); v.improved {
		t.Fatalf("an unchanged blend should not be kept: %+v", v)
	}
}

// A change that does not improve is undone by its revert; one that does stands.
func TestFitnessKeepOrRevert(t *testing.T) {
	reverted := false
	if keepOrRevert(fitnessVerdict{improved: false}, func() { reverted = true }); !reverted {
		t.Fatal("a non-improving change should be reverted")
	}
	reverted = false
	if !keepOrRevert(fitnessVerdict{improved: true}, func() { reverted = true }) || reverted {
		t.Fatal("an improving change should stand, not revert")
	}
}

// Fitness reads off the live state: goal progress is the share of goals
// satisfied rather than abandoned, and the factors stay in range.
func TestFitnessNowReadsState(t *testing.T) {
	d, _ := testDaemon(t)
	d.goalsSatisfied = 3
	d.goalsAbandoned = 1
	f := d.fitnessNow()
	if f.goalProgress != 0.75 {
		t.Fatalf("goal progress should be the satisfied share: got %v", f.goalProgress)
	}
	for _, v := range []float64{f.prediction, f.trust} {
		if v < 0 || v > 1 {
			t.Fatalf("fitness factors should be in [0,1]: %+v", f)
		}
	}
}
