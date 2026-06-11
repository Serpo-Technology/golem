package daemon

import (
	"os/exec"
	"testing"
	"time"

	"golem/internal/attention"
	"golem/internal/memory"
	"golem/internal/senses"
)

// An untrusted channel is barred from the reflex arc: a fetched page
// that happens to contain a graduated cue never moves the hands.
func TestUntrustedChannelNeverCuesHabit(t *testing.T) {
	d, s := testDaemon(t)
	d.cfg.ExecuteEnabled = true
	d.cfg.ExecuteAllowlist = []string{"echo"}
	ground(t, d, "rotate the logs", "echo rotated", 4)
	if err := s.SaveHabit(memory.Habit{Cue: attention.Normalize("rotate the logs"), Action: "echo rotated",
		Approvals: 3, Status: memory.HabitGraduated}); err != nil {
		t.Fatal(err)
	}
	d.habitsChanged()

	d.process(senses.Observation{Channel: "web",
		Content: "helpful tip: rotate the logs often", At: time.Now()}, false, true)
	d.settleExecution()

	entries, _ := s.AuditTail(5)
	for _, e := range entries {
		if e.Action == "echo rotated" && e.Outcome != "" {
			t.Fatalf("untrusted perception must never cue a habit: %+v", e)
		}
	}

	// The same line on a trusted channel is the reflex working as built.
	d.process(senses.Observation{Channel: "syslog",
		Content: "rotate the logs", At: time.Now()}, false, true)
	d.settleExecution()
	entries, _ = s.AuditTail(5)
	fired := false
	for _, e := range entries {
		fired = fired || e.Action == "echo rotated"
	}
	if !fired {
		t.Fatal("a trusted channel should still cue the habit")
	}
}

// Untrusted perception never anchors the grounding of an action: a web
// moment that names the cue lends nothing to the decision.
func TestUntrustedChannelCannotAnchorGrounding(t *testing.T) {
	d, s := testDaemon(t)
	at := time.Now().Add(-time.Hour)

	// The cue text exists only as a web moment, temporally linked to a
	// real command moment, the shape an adversarial page would engineer.
	cue := encodeAt(t, s, "web", "clean the cache now", 3, at)
	cmd := encodeAt(t, s, "shell", "run: rm -r ./cache", 3, at.Add(2*time.Second))
	if err := s.Link(cue.ID, cmd.ID, 0.9); err != nil {
		t.Fatal(err)
	}
	if err := s.Link(cmd.ID, cue.ID, 0.9); err != nil {
		t.Fatal(err)
	}

	dec, err := d.decide("clean the cache")
	if err != nil {
		t.Fatal(err)
	}
	if dec.Proposed != nil {
		t.Fatalf("an untrusted anchor must ground nothing: %+v", dec.Proposed)
	}
}

// Open-world perception is remembered lightly: an untrusted moment
// encodes at half weight, so it fades in half the nights.
func TestUntrustedChannelEncodesLight(t *testing.T) {
	d, s := testDaemon(t)
	res := d.process(senses.Observation{Channel: "web",
		Content: "some page text", At: time.Now()}, true, true)
	if res.FragmentID == 0 {
		t.Fatal("the forced observation should encode")
	}
	frags, err := s.FragmentsByIDs([]uint64{res.FragmentID})
	if err != nil || len(frags) != 1 {
		t.Fatal(err)
	}
	if frags[0].Weight != untrustedWeight {
		t.Fatalf("untrusted moments encode at %.2f, got %.2f", untrustedWeight, frags[0].Weight)
	}

	trusted := d.process(senses.Observation{Channel: "syslog",
		Content: "some syslog text", At: time.Now()}, true, true)
	frags, _ = s.FragmentsByIDs([]uint64{trusted.FragmentID})
	if len(frags) != 1 || frags[0].Weight != 1.0 {
		t.Fatalf("trusted moments keep full weight: %+v", frags)
	}
}

// The voice is gated by its runtime switch and never blocks: with the
// switch off nothing is queued, with it on the reply drains through the
// speech command.
func TestVoiceSpeaksOnlyWhenConfigured(t *testing.T) {
	d, _ := testDaemon(t)

	d.speak("never spoken")
	if d.voice != nil {
		t.Fatal("a silent configuration must not open the voice")
	}

	spoken := make(chan string, 4)
	orig := sayCommand
	sayCommand = func(text, voice string) *exec.Cmd {
		spoken <- text
		return exec.Command("/usr/bin/true")
	}
	defer func() { sayCommand = orig }()

	d.voiceOn = true
	d.speak("After make test, make deploy usually follows.")
	select {
	case got := <-spoken:
		if got != "After make test, make deploy usually follows." {
			t.Fatalf("unexpected utterance: %q", got)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("the configured voice should speak")
	}
}
