package daemon

import (
	"strings"
	"testing"
	"time"

	"golem/internal/attention"
	"golem/internal/memory"
	"golem/internal/senses"
	"golem/internal/wire"
)

// thinkingDay lays down a lived sequence A -> B -> C, distils the three
// concepts, and returns them ready to think over.
func thinkingDay(t *testing.T, d *Daemon, s *memory.Store) (a, b, c *memory.Fragment) {
	t.Helper()
	base := time.Now().Add(-4 * time.Hour)
	for i := range 4 {
		at := base.Add(time.Duration(i) * 10 * time.Minute)
		d.recent = nil
		d.process(senses.Observation{Channel: "shell", Content: "run: make test", At: at}, true, true)
		d.process(senses.Observation{Channel: "shell", Content: "run: make deploy", At: at.Add(5 * time.Second)}, true, true)
		d.process(senses.Observation{Channel: "shell", Content: "run: make push", At: at.Add(10 * time.Second)}, true, true)
	}
	d.recent = nil
	if _, err := s.MergeDuplicates(); err != nil {
		t.Fatal(err)
	}
	calm(d)
	d.growConcepts()
	a, _ = s.ConceptByTemplate(attention.Normalize("run: make test"))
	b, _ = s.ConceptByTemplate(attention.Normalize("run: make deploy"))
	c, _ = s.ConceptByTemplate(attention.Normalize("run: make push"))
	if a == nil || b == nil || c == nil {
		t.Fatal("the day should distil three concepts")
	}
	return a, b, c
}

// With input quiet, a warm concept extends into a chain of associated
// concepts across successive steps: the warmth moves through the lived
// graph instead of dying where it started.
func TestMindWanderingChainsAcrossTicks(t *testing.T) {
	d, s := testDaemon(t)
	a, b, c := thinkingDay(t, d, s)

	d.ws.excite(a.ID, 1.0)
	d.thinkStep()
	if w := d.ws.warmest(1); len(w) == 0 || w[0].id != b.ID {
		t.Fatalf("the first step should hand warmth to the successor concept, warmest %+v", w)
	}
	if d.ws.heat(a.ID) >= d.ws.heat(b.ID) {
		t.Fatal("the seed should fatigue below what it warmed")
	}
	d.thinkStep()
	if d.ws.heat(c.ID) == 0 {
		t.Fatalf("the second step should extend the chain: heat(c)=%.2f", d.ws.heat(c.ID))
	}
}

// An unreinforced chain damps back to nothing: fatigue and decay always
// win over a finite graph, so rumination is bounded.
func TestChainDampsToNothing(t *testing.T) {
	d, s := testDaemon(t)
	a, _, _ := thinkingDay(t, d, s)

	d.ws.excite(a.ID, 1.0)
	for range 400 {
		d.thinkStep()
		d.ws.decay()
		if len(d.ws.warmest(1)) == 0 {
			return
		}
	}
	t.Fatalf("the chain should have damped, workspace still warm: %+v", d.ws.warmest(3))
}

// Thoughts pass the same surprise gate perception uses: a routine
// association habituates and stays transient, so memory does not flood.
func TestRoutineThoughtsStayTransient(t *testing.T) {
	d, s := testDaemon(t)
	a, _, _ := thinkingDay(t, d, s)

	for range 30 {
		d.ws.excite(a.ID, 1.0)
		d.thinkStep()
		d.ws.clear()
	}
	frags, _ := s.Dump()
	thoughts := 0
	for _, f := range frags {
		if f.Channel == thoughtChannel {
			thoughts++
		}
	}
	if thoughts > 3 {
		t.Fatalf("a repeated thought should habituate, %d encoded", thoughts)
	}
}

// A novel combination clears the gate, is encoded as an insight, and a
// later recall shows GOLEM remembers having thought it.
func TestInsightIsRememberedThought(t *testing.T) {
	d, s := testDaemon(t)
	a, _, _ := thinkingDay(t, d, s)

	// The thought channel learns its routine shapes first.
	for i := range 6 {
		d.process(senses.Observation{Channel: thoughtChannel,
			Content: "thought: filler shape " + string(rune('a'+i)), At: time.Now()}, true, true)
	}
	d.ws.excite(a.ID, 1.0)
	d.thinkStep()

	frags, _ := s.Dump()
	var insight *memory.Fragment
	for _, f := range frags {
		if f.Channel == thoughtChannel && strings.Contains(f.Content, "make") {
			insight = f
		}
	}
	if insight == nil {
		t.Fatal("a novel association should encode as an insight")
	}
	resp, _, _ := d.handle(wire.Request{Cmd: "recall", Query: "thought"}, nil)
	var remembered bool
	for _, f := range resp.Fragments {
		remembered = remembered || f.ID == insight.ID
	}
	if !remembered {
		t.Fatal("recall should surface the remembered thought")
	}
}

// The mind's own activity is not the world's: a thought never fires a
// graduated habit, however plainly it contains the cue.
func TestThoughtsNeverCueHabits(t *testing.T) {
	d, s := testDaemon(t)
	d.cfg.ExecuteEnabled = true
	d.cfg.ExecuteAllowlist = []string{"echo"}
	ground(t, d, "make test", "echo fired", 4)
	if err := s.SaveHabit(memory.Habit{Cue: attention.Normalize("make test"), Action: "echo fired",
		Approvals: 3, Status: memory.HabitGraduated}); err != nil {
		t.Fatal(err)
	}
	d.habitsChanged()

	d.process(senses.Observation{Channel: thoughtChannel,
		Content: "thought: after make test, make deploy", At: time.Now()}, false, true)
	d.settleExecution()

	entries, _ := s.AuditTail(5)
	for _, e := range entries {
		if strings.HasPrefix(e.Outcome, "autonomous") {
			t.Fatalf("a thought must never cue a habit: %+v", e)
		}
	}
}

// Thoughts leave the quiet timers alone: mind-wandering neither throttles
// itself nor holds off sleep's input-silence test.
func TestThoughtsDoNotResetQuietTimers(t *testing.T) {
	d, _ := testDaemon(t)
	obs := time.Now().Add(-time.Minute)
	sur := time.Now().Add(-2 * time.Minute)
	d.lastObs, d.lastSurprise = obs, sur

	d.process(senses.Observation{Channel: thoughtChannel,
		Content: "thought: anything at all", At: time.Now()}, false, true)
	if !d.lastObs.Equal(obs) || !d.lastSurprise.Equal(sur) {
		t.Fatalf("thoughts must not touch the quiet timers: obs %v sur %v", d.lastObs, d.lastSurprise)
	}
}

// The default-mode condition: quiet, calm, not yet asleep, something
// warm to think from.
func TestMindWanderingCondition(t *testing.T) {
	d, s := testDaemon(t)
	a, _, _ := thinkingDay(t, d, s)

	d.lastObs = time.Now().Add(-10 * time.Second)
	d.lastSurprise = time.Now().Add(-10 * time.Second)
	d.lastSleep = time.Now()
	if d.mindWandering() {
		t.Fatal("an empty workspace has nothing to wander over")
	}
	d.ws.excite(a.ID, 0.8)
	if !d.mindWandering() {
		t.Fatal("quiet, calm, and warm should wander")
	}
	d.lastObs = time.Now()
	if d.mindWandering() {
		t.Fatal("fresh input should hold thinking off")
	}
}
