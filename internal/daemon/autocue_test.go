package daemon

import (
	"strings"
	"testing"
	"time"

	"golem/internal/attention"
	"golem/internal/memory"
	"golem/internal/senses"
)

func TestContainsPhrase(t *testing.T) {
	cases := []struct {
		s, phrase string
		want      bool
	}{
		{"Disk full on <path>", "disk full", true},
		{"run: go test ./...", "go test", true},
		{"GO TEST now", "go test", true},
		{"also", "ls", false},
		{"go testing", "go test", false},
		{"x_go test", "go test", false},
		{"a go test b", "go test", true},
		{"go test", "go test", true},
		{"", "x", false},
		{"x", "", false},
	}
	for _, c := range cases {
		if got := containsPhrase(c.s, c.phrase); got != c.want {
			t.Errorf("containsPhrase(%q, %q) = %v, want %v", c.s, c.phrase, got, c.want)
		}
	}
}

// graduate grounds a pair and approves it to graduation while execution is
// off, so the approvals are dry runs that touch nothing.
func graduate(t *testing.T, d *Daemon, cue, command string) {
	t.Helper()
	ground(t, d, cue, command, 4)
	for i := 0; i < d.cfg.GraduationApprovals; i++ {
		dec, err := d.decide(cue)
		if err != nil {
			t.Fatal(err)
		}
		if _, _, err := d.approve(dec, time.Now().Add(-time.Hour)); err != nil {
			t.Fatal(err)
		}
	}
	h, _ := d.store.LoadHabit(attention.Normalize(cue), command)
	if h.Status != memory.HabitGraduated {
		t.Fatalf("pair should be graduated before the reflex test, got %s", h.Status)
	}
}

// autonomousRuns counts audit entries from observation-triggered runs.
func autonomousRuns(t *testing.T, d *Daemon) int {
	t.Helper()
	entries, err := d.store.AuditTail(100)
	if err != nil {
		t.Fatal(err)
	}
	n := 0
	for _, e := range entries {
		if strings.HasPrefix(e.Outcome, "autonomous (observed)") {
			n++
		}
	}
	return n
}

// An observation containing a graduated cue fires the habit with no human
// in the loop, leaves nothing pending, and keeps the pair graduated.
func TestObservationFiresGraduatedHabit(t *testing.T) {
	d, _ := testDaemon(t)
	graduate(t, d, "disk pressure", "echo reflex-ran")
	d.cfg.ExecuteEnabled = true
	d.cfg.ExecuteAllowlist = []string{"echo"}

	d.process(senses.Observation{Channel: "syslog",
		Content: "alert: disk pressure rising on volume 2", At: time.Now()}, false, true)
	d.settleExecution()

	entries, _ := d.store.AuditTail(5)
	var hit *memory.AuditEntry
	for i := range entries {
		if strings.HasPrefix(entries[i].Outcome, "autonomous (observed)") {
			hit = &entries[i]
		}
	}
	if hit == nil {
		t.Fatalf("the observation should have run the habit, audit: %+v", entries)
	}
	if !strings.Contains(hit.Outcome, "reflex-ran") {
		t.Fatalf("the audited outcome should carry the real run, got %q", hit.Outcome)
	}
	if d.pending != nil {
		t.Fatal("a reflex run must leave nothing pending")
	}
	h, _ := d.store.LoadHabit(attention.Normalize("disk pressure"), "echo reflex-ran")
	if h.Status != memory.HabitGraduated {
		t.Fatalf("a successful reflex keeps the pair graduated, got %s", h.Status)
	}
}

// A pair that has not graduated never fires from an observation.
func TestReflexNeedsGraduation(t *testing.T) {
	d, _ := testDaemon(t)
	ground(t, d, "disk pressure", "echo nope", 4)
	d.cfg.ExecuteEnabled = true
	d.cfg.ExecuteAllowlist = []string{"echo"}

	d.process(senses.Observation{Channel: "syslog",
		Content: "alert: disk pressure rising", At: time.Now()}, false, true)
	d.settleExecution()

	if n := autonomousRuns(t, d); n != 0 {
		t.Fatalf("an ungraduated pair must not fire, got %d runs", n)
	}
	if d.pending != nil {
		t.Fatal("automatic cueing must not plant proposals for the operator")
	}
}

// The three execution gates hold for reflex runs: graduated but not
// allowed means nothing runs and nothing is audited as autonomous.
func TestReflexRespectsExecutionGates(t *testing.T) {
	d, _ := testDaemon(t)
	graduate(t, d, "disk pressure", "echo gated")

	// Master switch off.
	d.process(senses.Observation{Channel: "syslog",
		Content: "warning: disk pressure climbing", At: time.Now()}, false, true)
	d.settleExecution()
	if n := autonomousRuns(t, d); n != 0 {
		t.Fatalf("execution off must hold the reflex, got %d runs", n)
	}

	// Switch on but not allowlisted.
	d.cfg.ExecuteEnabled = true
	d.cfg.ExecuteAllowlist = []string{"ls"}
	d.process(senses.Observation{Channel: "syslog",
		Content: "warning: disk pressure climbing again", At: time.Now().Add(2 * time.Minute)}, false, true)
	d.settleExecution()
	if n := autonomousRuns(t, d); n != 0 {
		t.Fatalf("the allowlist must hold the reflex, got %d runs", n)
	}
	h, _ := d.store.LoadHabit(attention.Normalize("disk pressure"), "echo gated")
	if h.Status != memory.HabitGraduated {
		t.Fatalf("a held-back reflex must not change the habit, got %s", h.Status)
	}
}

// The cooldown collapses a storm of matching observations into one run.
func TestReflexCooldown(t *testing.T) {
	d, _ := testDaemon(t)
	graduate(t, d, "disk pressure", "echo cooled")
	d.cfg.ExecuteEnabled = true
	d.cfg.ExecuteAllowlist = []string{"echo"}
	now := time.Now()

	for i := range 5 {
		d.process(senses.Observation{Channel: "syslog",
			Content: "disk pressure spike", At: now.Add(time.Duration(i) * time.Second)}, false, true)
		d.settleExecution()
	}
	if n := autonomousRuns(t, d); n != 1 {
		t.Fatalf("five matches inside the cooldown should run once, got %d", n)
	}

	d.process(senses.Observation{Channel: "syslog",
		Content: "disk pressure spike", At: now.Add(10 * time.Minute)}, false, true)
	d.settleExecution()
	if n := autonomousRuns(t, d); n != 2 {
		t.Fatalf("a match past the cooldown should run again, got %d", n)
	}
}

// Action echoes never cue: the feedback channel and shell lines that are
// themselves commands must not re-trigger the habit they came from.
func TestReflexIgnoresActionEchoes(t *testing.T) {
	d, _ := testDaemon(t)
	graduate(t, d, "deep clean", "echo deep clean done")
	d.cfg.ExecuteEnabled = true
	d.cfg.ExecuteAllowlist = []string{"echo"}
	now := time.Now()

	d.process(senses.Observation{Channel: "action",
		Content: "ran echo deep clean done -> ok", At: now}, false, true)
	d.settleExecution()
	d.process(senses.Observation{Channel: "shell",
		Content: "run: deep clean now", At: now.Add(2 * time.Minute)}, false, true)
	d.settleExecution()
	if n := autonomousRuns(t, d); n != 0 {
		t.Fatalf("echoes must not cue, got %d runs", n)
	}

	d.process(senses.Observation{Channel: "syslog",
		Content: "time for the deep clean cycle", At: now.Add(4 * time.Minute)}, false, true)
	d.settleExecution()
	if n := autonomousRuns(t, d); n != 1 {
		t.Fatalf("a sensory observation should still cue, got %d runs", n)
	}
}

// The ACC veto holds for reflexes: a cue that has become ambiguous stays
// still instead of guessing.
func TestReflexHonorsConflictVeto(t *testing.T) {
	d, s := testDaemon(t)
	d.cfg.ExecuteEnabled = true
	d.cfg.ExecuteAllowlist = []string{"echo"}
	now := time.Now()

	// Two commands grounded to one cue with identical strength, and one
	// pair graduated by record: the textbook ambiguous reflex.
	cue := &memory.Fragment{Channel: "tell", Content: "tidy up", Template: attention.Normalize("tidy up"), Surprise: 4, Time: now}
	a := &memory.Fragment{Channel: "shell", Content: "run: echo tidy-a", Template: attention.Normalize("run: echo tidy-a"), Surprise: 4, Time: now}
	b := &memory.Fragment{Channel: "shell", Content: "run: echo tidy-b", Template: attention.Normalize("run: echo tidy-b"), Surprise: 4, Time: now}
	for _, f := range []*memory.Fragment{cue, a, b} {
		if _, err := s.Encode(f); err != nil {
			t.Fatal(err)
		}
	}
	s.Link(cue.ID, a.ID, 0.6)
	s.Link(cue.ID, b.ID, 0.6)
	if err := s.SaveHabit(memory.Habit{Cue: attention.Normalize("tidy up"), Action: "echo tidy-a",
		Approvals: 3, Status: memory.HabitGraduated}); err != nil {
		t.Fatal(err)
	}

	dec, err := d.decide("tidy up")
	if err != nil {
		t.Fatal(err)
	}
	if !dec.Conflict {
		t.Fatalf("equal groundings should conflict, got %+v", dec)
	}

	d.process(senses.Observation{Channel: "syslog",
		Content: "scheduled tidy up window open", At: now}, false, true)
	d.settleExecution()
	if n := autonomousRuns(t, d); n != 0 {
		t.Fatalf("an ambiguous cue must not fire, got %d runs", n)
	}
}

// Demotion takes effect immediately: the graduated-cue cache is
// invalidated when a habit changes, so a denied pair stops firing.
func TestDemotionStopsReflex(t *testing.T) {
	d, _ := testDaemon(t)
	graduate(t, d, "disk pressure", "echo demoted-soon")
	d.cfg.ExecuteEnabled = true
	d.cfg.ExecuteAllowlist = []string{"echo"}
	now := time.Now()

	d.process(senses.Observation{Channel: "syslog",
		Content: "disk pressure high", At: now}, false, true)
	d.settleExecution()
	if n := autonomousRuns(t, d); n != 1 {
		t.Fatalf("the graduated pair should fire first, got %d", n)
	}

	dec, err := d.decide("disk pressure")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := d.deny(dec, now); err != nil {
		t.Fatal(err)
	}

	d.process(senses.Observation{Channel: "syslog",
		Content: "disk pressure high", At: now.Add(10 * time.Minute)}, false, true)
	d.settleExecution()
	if n := autonomousRuns(t, d); n != 1 {
		t.Fatalf("a demoted pair must stop firing, got %d runs", n)
	}
}
