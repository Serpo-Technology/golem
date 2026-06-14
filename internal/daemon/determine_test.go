package daemon

import (
	"strings"
	"testing"
	"time"

	"golem/internal/senses"
)

// With nothing pressing, the determination is idle: the loop rests and wanders
// rather than stalling, the floor that means there is always an answer.
func TestDetermineIdleIsTheFloor(t *testing.T) {
	d, _ := testDaemon(t)
	if det := d.determine(time.Now()); det.mode != "idle" {
		t.Fatalf("with nothing pressing the determination should be idle, got %q", det.mode)
	}
}

// Under sleep pressure consolidation wins the arbitration, outranking idle.
func TestDetermineConsolidatesUnderPressure(t *testing.T) {
	d, _ := testDaemon(t)
	calm(d) // build serotonin past the sleep gate
	if !d.readyToSleep() {
		t.Fatal("calm should build enough sleep pressure to be ready")
	}
	if det := d.determine(time.Now()); det.mode != "consolidate" {
		t.Fatalf("under sleep pressure the determination should be consolidate, got %q", det.mode)
	}
}

// A held goal wins over idle and survives a passing distraction: the loop keeps
// pursuing rather than re-deciding from scratch when the world flickers.
func TestDeterminePursuesAndSurvivesDistraction(t *testing.T) {
	d, _ := testDaemon(t)
	d.adoptGoal("learn", "flurble")
	if det := d.determine(time.Now()); det.mode != "pursue" {
		t.Fatalf("a held goal should be the determination, got %q", det.mode)
	}
	d.lastMode = "pursue" // the loop is already pursuing

	// A surprising observation arrives and passes.
	d.process(senses.Observation{Channel: "syslog", Content: "Disk full on /dev/disk2", At: time.Now()}, true, true)

	if d.goal == nil {
		t.Fatal("a passing distraction should not abandon the goal")
	}
	if det := d.determine(time.Now()); det.mode != "pursue" {
		t.Fatalf("the goal should survive the distraction and stay the determination, got %q", det.mode)
	}
}

// A user request is served while a goal is held, and answering it does not
// abandon the pursuit: the request preempts, the goal resumes.
func TestRequestServedWhileGoalHeld(t *testing.T) {
	d, _ := testDaemon(t)
	groundNoun(t, d, "dog")
	d.adoptGoal("learn", "flurble")

	if got := say(t, d, "what is a dog").Say; !strings.HasPrefix(got, "Dog is a thing") {
		t.Fatalf("the request should be answered: %q", got)
	}
	if d.goal == nil {
		t.Fatal("answering a request should not abandon the held goal")
	}
}

// Pursuing a goal actively mulls its topic: a paced recall that keeps the
// topic's experience warm, the pursuit between wanders.
func TestPursueGoalMullsTopic(t *testing.T) {
	d, _ := testDaemon(t)
	d.process(senses.Observation{Channel: "syslog", Content: "Disk full on /dev/disk2", At: time.Now().Add(-time.Hour)}, true, true)
	d.recent = nil
	d.adoptGoal("learn", "disk")

	d.pursueGoal(time.Now())
	if d.lastPursuit.IsZero() {
		t.Fatal("pursuing the goal should mark the pursuit time, pacing the next mull")
	}
}
