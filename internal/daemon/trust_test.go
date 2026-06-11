package daemon

import (
	"testing"
	"time"

	"golem/internal/memory"
)

// Corrections lower trust; clean approvals raise it back, capped.
func TestTrustMovesOnJudgement(t *testing.T) {
	d, _ := testDaemon(t)
	ground(t, d, "deploy it", "make deploy", 4)
	now := time.Now()

	base := d.chem.Snapshot().Oxytocin
	dec, _ := d.decide("deploy it")
	if _, err := d.deny(dec, now); err != nil {
		t.Fatal(err)
	}
	low := d.chem.Snapshot().Oxytocin
	if low >= base {
		t.Fatalf("a denial should lower trust: %.2f then %.2f", base, low)
	}

	dec, _ = d.decide("deploy it")
	if _, _, err := d.approve(dec, now); err != nil {
		t.Fatal(err)
	}
	if got := d.chem.Snapshot().Oxytocin; got <= low {
		t.Fatalf("a clean approval should raise trust: %.2f then %.2f", low, got)
	}

	for range 100 {
		d.chem.RaiseTrust(trustOnConfirm)
	}
	if got := d.chem.Snapshot().Oxytocin; got > 1.0 {
		t.Fatalf("trust must cap at 1.0, got %.2f", got)
	}
}

// A failed real run is a correction and costs trust like a denial.
func TestFailedRunLowersTrust(t *testing.T) {
	d, _ := testDaemon(t)
	d.cfg.ExecuteEnabled = true
	d.cfg.ExecuteAllowlist = []string{"false"}
	ground(t, d, "do the thing", "false", 4)

	base := d.chem.Snapshot().Oxytocin
	dec, _ := d.decide("do the thing")
	if _, _, err := d.approve(dec, time.Now()); err != nil {
		t.Fatal(err)
	}
	if got := d.chem.Snapshot().Oxytocin; got >= base {
		t.Fatalf("a failed run should lower trust: %.2f then %.2f", base, got)
	}
}

// The bar reads trust: configured N at baseline and above, rising in
// proportion as trust falls, finite at the floor.
func TestPromotionBarTracksTrust(t *testing.T) {
	d, _ := testDaemon(t)
	n := d.cfg.GraduationApprovals

	if bar := d.promotionBar(); bar != n {
		t.Fatalf("at baseline the bar is the configured N, got %d", bar)
	}
	d.chem.RaiseTrust(0.5) // full trust
	if bar := d.promotionBar(); bar != n {
		t.Fatalf("high trust must not lower the bar below N, got %d", bar)
	}
	for range 10 {
		d.chem.LowerTrust(trustOnCorrection)
	}
	// Trust is floored at 0.05, so the bar tops out at ten times N.
	if bar := d.promotionBar(); bar != n*10 {
		t.Fatalf("rock-bottom trust should put the bar at %d, got %d", n*10, bar)
	}
}

// Under low trust a pair does not graduate at the configured N; it takes
// the longer run of consistent approvals the raised bar demands.
func TestLowTrustDelaysGraduation(t *testing.T) {
	d, _ := testDaemon(t)
	ground(t, d, "build it", "make build", 4)
	now := time.Now()

	for range 10 {
		d.chem.LowerTrust(trustOnCorrection)
	}

	approvals := 0
	var h memory.Habit
	for ; approvals < 50; approvals++ {
		dec, err := d.decide("build it")
		if err != nil {
			t.Fatal(err)
		}
		h, _, err = d.approve(dec, now)
		if err != nil {
			t.Fatal(err)
		}
		if h.Status == memory.HabitGraduated {
			break
		}
	}
	if h.Status != memory.HabitGraduated {
		t.Fatal("consistent approvals should eventually rebuild trust and graduate")
	}
	if approvals+1 <= d.cfg.GraduationApprovals {
		t.Fatalf("low trust should delay graduation past %d approvals, took %d",
			d.cfg.GraduationApprovals, approvals+1)
	}
}
