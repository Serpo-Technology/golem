package daemon

import (
	"strings"
	"testing"
	"time"

	"golem/internal/attention"
	"golem/internal/memory"
)

// The done-when: an unprompted internal act fires because a drive
// called for it, and the drive is readable on the audit trail.
func TestTidinessDriveOnTrail(t *testing.T) {
	d, s := testDaemon(t)
	c := &memory.Fragment{Kind: memory.KindConcept, Channel: "concept", Content: "quiet shape",
		Template: "quiet shape", Category: "noun", Limbic: d.chem.Snapshot(), Time: time.Now()}
	if _, err := s.Encode(c); err != nil {
		t.Fatal(err)
	}
	calm(d)
	settleChem(d)
	d.encodesSinceSleep = 80
	d.ws.excite(c.ID, 1.0)
	d.lastObs = time.Now().Add(-time.Minute)
	d.lastSurprise = time.Now().Add(-time.Minute)

	d.startThinkRound()
	d.settleThinking()

	outcome, ok := auditHas(t, s, "act:consolidate")
	if !ok {
		t.Fatal("clutter should have driven an unprompted consolidation")
	}
	if !strings.Contains(outcome, "tidiness: 80 fresh moments") {
		t.Fatalf("the drive must be readable on the trail: %q", outcome)
	}
}

// Alarm is the relevance drive: it pushes the recall act and signs the
// trail with its reason.
func TestRelevanceDriveOnTrail(t *testing.T) {
	d, s := testDaemon(t)
	feedShape(t, s, "syslog", "Disk full on /dev/disk1s9", 4, time.Now().Add(-2*time.Hour))
	if _, err := s.MergeDuplicates(); err != nil {
		t.Fatal(err)
	}
	calm(d)
	d.growConcepts()
	c, _ := s.ConceptByTemplate(attention.Normalize("Disk full on /dev/disk1s9"))
	if c == nil {
		t.Fatal("the shape should distil")
	}
	settleChem(d)
	d.chem.SpikeNoradrenaline(0.8)
	d.ws.excite(c.ID, 1.0)
	d.lastObs = time.Now().Add(-time.Minute)
	d.lastSurprise = time.Now().Add(-time.Minute)

	d.startThinkRound()
	d.settleThinking()

	outcome, ok := auditHas(t, s, "act:recall")
	if !ok {
		t.Fatal("alarm should have driven an unprompted recall")
	}
	if !strings.Contains(outcome, "relevance: alarm") {
		t.Fatalf("the drive must be readable on the trail: %q", outcome)
	}
}

// The curiosity drive speaks: a warm nameless concept under high
// dopamine makes the speak act ask what it is.
func TestCuriosityDriveAsks(t *testing.T) {
	d, s := testDaemon(t)
	c := &memory.Fragment{Kind: memory.KindConcept, Channel: "concept", Content: "strange hum",
		Template: "strange hum", Category: "noun", Limbic: d.chem.Snapshot(), Time: time.Now()}
	if _, err := s.Encode(c); err != nil {
		t.Fatal(err)
	}
	settleChem(d)
	d.chem.SpikeDopamine(0.7)
	d.ws.excite(c.ID, 1.0)
	d.lastObs = time.Now().Add(-time.Minute)
	d.lastSurprise = time.Now().Add(-time.Minute)

	d.startThinkRound()
	d.settleThinking()

	outcome, ok := auditHas(t, s, "act:speak")
	if !ok {
		t.Fatal("a nameless warm thing under high dopamine should be asked about")
	}
	if !strings.Contains(outcome, "curiosity:") || !strings.Contains(outcome, "What is strange hum?") {
		t.Fatalf("the curiosity drive should ask and sign the trail: %q", outcome)
	}
}

// An asked question becomes a held goal: adopted on the trail, visible
// in status, aimed at by the recall act, and satisfied with a reward
// when the word finally names something.
func TestGoalLifecycle(t *testing.T) {
	d, s := testDaemon(t)
	settleChem(d)
	d.chem.SpikeDopamine(0.6)

	resp := say(t, d, "when did the flurble happen")
	if !strings.Contains(resp.Say, "What is flurble?") {
		t.Fatalf("the gap should be asked about: %q", resp.Say)
	}
	if d.goal == nil || d.goal.topic != "flurble" {
		t.Fatalf("the asked question should be held as a goal: %+v", d.goal)
	}
	if _, ok := auditHas(t, s, "goal"); !ok {
		t.Fatal("adoption belongs on the trail")
	}
	if desc := d.goalDescription(); !strings.Contains(desc, "flurble") {
		t.Fatalf("the goal should be visible in status: %q", desc)
	}

	// The held goal aims the recall act at its topic across the next
	// quiet rounds: other faculties may speak first, but each cools on
	// its refractory and the hunt comes through.
	c := &memory.Fragment{Kind: memory.KindConcept, Channel: "concept", Content: "quiet shape",
		Template: "quiet shape", Category: "noun", Limbic: d.chem.Snapshot(), Time: time.Now()}
	if _, err := s.Encode(c); err != nil {
		t.Fatal(err)
	}
	settleChem(d)
	var outcome string
	var ok bool
	for range 4 {
		d.ws.excite(c.ID, 1.0)
		d.lastObs = time.Now().Add(-time.Minute)
		d.lastSurprise = time.Now().Add(-time.Minute)
		d.startThinkRound()
		d.settleThinking()
		if outcome, ok = auditHas(t, s, "act:recall"); ok {
			break
		}
	}
	if !ok || !strings.Contains(outcome, "goal: wants to learn flurble") {
		t.Fatalf("the goal should drive recall toward its topic: %q %v", outcome, ok)
	}
	entries, err := s.AuditTail(5)
	if err != nil {
		t.Fatal(err)
	}
	var aimed bool
	for _, e := range entries {
		if e.Cue == "act:recall" && e.Action == "flurble" {
			aimed = true
		}
	}
	if !aimed {
		t.Fatal("the recall act should hunt the goal's topic")
	}

	// Naming the warm thing flurble answers the held question.
	before := d.chem.Snapshot().Dopamine
	resp = say(t, d, "remember this as a flurble")
	if !strings.Contains(resp.Say, "names") {
		t.Fatalf("the naming should be confirmed: %q", resp.Say)
	}
	if d.goal != nil {
		t.Fatalf("the answered goal should clear: %+v", d.goal)
	}
	outcome, ok = auditHas(t, s, "goal")
	if !ok || !strings.HasPrefix(outcome, "satisfied:") {
		t.Fatalf("satisfaction belongs on the trail: %q %v", outcome, ok)
	}
	if d.chem.Snapshot().Dopamine <= before {
		t.Fatal("a satisfied goal should reward")
	}
}

// A goal that nothing answers goes stale instead of becoming an
// obsession, and the abandonment is on the trail.
func TestGoalExpires(t *testing.T) {
	d, s := testDaemon(t)
	d.adoptGoal("learn", "flurble")
	d.goal.since = time.Now().Add(-goalTTL - time.Minute)
	d.checkGoal()
	if d.goal != nil {
		t.Fatalf("a stale goal should clear: %+v", d.goal)
	}
	outcome, ok := auditHas(t, s, "goal")
	if !ok || !strings.HasPrefix(outcome, "abandoned:") {
		t.Fatalf("abandonment belongs on the trail: %q %v", outcome, ok)
	}
}

// One goal at a time: the first open question keeps the slot.
func TestGoalSlotIsSingular(t *testing.T) {
	d, _ := testDaemon(t)
	d.adoptGoal("learn", "flurble")
	d.adoptGoal("learn", "bazonk")
	if d.goal == nil || d.goal.topic != "flurble" {
		t.Fatalf("the first goal should hold the slot: %+v", d.goal)
	}
}
