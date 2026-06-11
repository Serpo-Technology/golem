package main

import (
	"strings"
	"testing"
	"time"
)

func ground(t *testing.T, d *Daemon, cue, command string, times int) {
	t.Helper()
	base := time.Now().Add(-200 * time.Hour)
	for i := 0; i < times; i++ {
		d.teach(cue, command, base.Add(time.Duration(i)*time.Hour))
	}
}

func TestCommandOf(t *testing.T) {
	cmd, ok := commandOf(&Fragment{Channel: "shell", Content: "run: go test ./..."})
	if !ok || cmd != "go test ./..." {
		t.Fatalf("commandOf shell run: got %q %v", cmd, ok)
	}
	if _, ok := commandOf(&Fragment{Channel: "shell", Content: "command failed"}); ok {
		t.Fatal("a non-run shell line is not a command")
	}
	if _, ok := commandOf(&Fragment{Channel: "tell", Content: "run: x"}); ok {
		t.Fatal("only the shell channel carries commands")
	}
}

// Every denylist entry must actually be caught, so dropping any one of
// them turns this test red. Matching is case-insensitive.
func TestDenylist(t *testing.T) {
	for _, entry := range actionDenylist {
		cmd := "please " + entry + " now"
		if !isDenylisted(cmd) {
			t.Errorf("denylist entry %q is not enforced", entry)
		}
		if !isDenylisted(strings.ToUpper(cmd)) {
			t.Errorf("denylist entry %q is not matched case-insensitively", entry)
		}
	}
	for _, ok := range []string{"go test ./...", "git status", "make build", "git push origin main"} {
		if isDenylisted(ok) {
			t.Errorf("%q should not be denylisted", ok)
		}
	}
}

// A grounded cue proposes its command; an ungrounded one proposes nothing.
func TestGroundingProposes(t *testing.T) {
	d, _ := testDaemon(t)
	ground(t, d, "run the tests", "go test ./...", 4)

	dec, err := d.decide("run the tests")
	if err != nil {
		t.Fatal(err)
	}
	if dec.Proposed == nil || dec.Proposed.Command != "go test ./..." {
		t.Fatalf("expected the grounded command, got %+v", dec.Proposed)
	}
	if dec.Conflict {
		t.Fatal("a single grounded command is not a conflict")
	}

	empty, err := d.decide("something never taught")
	if err != nil {
		t.Fatal(err)
	}
	if empty.Proposed != nil {
		t.Fatalf("ungrounded cue should propose nothing, got %+v", empty.Proposed)
	}
}

// Consistent approvals graduate a pair; the status persists.
func TestGraduationAfterApprovals(t *testing.T) {
	d, s := testDaemon(t)
	ground(t, d, "build it", "make build", 4)
	now := time.Now()

	for i := 0; i < d.cfg.GraduationApprovals-1; i++ {
		dec, _ := d.decide("build it")
		h, _, err := d.approve(dec, now)
		if err != nil {
			t.Fatal(err)
		}
		if h.Status != HabitProposing {
			t.Fatalf("approval %d should still be proposing, got %s", i+1, h.Status)
		}
	}
	dec, _ := d.decide("build it")
	h, _, err := d.approve(dec, now)
	if err != nil {
		t.Fatal(err)
	}
	if h.Status != HabitGraduated {
		t.Fatalf("the final approval should graduate, got %s", h.Status)
	}
	stored, _ := s.LoadHabit(Normalize("build it"), "make build")
	if stored.Status != HabitGraduated || stored.Approvals != d.cfg.GraduationApprovals {
		t.Fatalf("graduation should persist, got %+v", stored)
	}
}

// A denylisted command can be proposed and approved but never graduates.
func TestDenylistNeverGraduates(t *testing.T) {
	d, _ := testDaemon(t)
	ground(t, d, "wipe temp", "rm -rf ./tmp", 4)
	now := time.Now()

	var h Habit
	for i := 0; i < d.cfg.GraduationApprovals+3; i++ {
		dec, _ := d.decide("wipe temp")
		var err error
		h, _, err = d.approve(dec, now)
		if err != nil {
			t.Fatal(err)
		}
	}
	if h.Status == HabitGraduated {
		t.Fatal("a denylisted command must never graduate")
	}
	if h.Approvals < d.cfg.GraduationApprovals {
		t.Fatalf("approvals should still accumulate, got %d", h.Approvals)
	}
}

// Two equally grounded commands trigger the ACC veto; clarifying tips it.
func TestConflictVetoAndResolution(t *testing.T) {
	d, _ := testDaemon(t)
	base := time.Now().Add(-200 * time.Hour)
	for i := 0; i < 2; i++ {
		d.teach("clean up", "git clean -fd", base.Add(time.Duration(i)*time.Hour))
		d.teach("clean up", "docker system prune", base.Add(time.Duration(10+i)*time.Hour))
	}

	dec, err := d.decide("clean up")
	if err != nil {
		t.Fatal(err)
	}
	if !dec.Conflict || dec.Question == "" {
		t.Fatalf("equally grounded commands should conflict, got %+v", dec)
	}
	// The question must name the actual tied commands, or it cannot guide
	// the operator's clarification.
	if !strings.Contains(dec.Question, "git clean -fd") || !strings.Contains(dec.Question, "docker system prune") {
		t.Fatalf("the question should name both commands, got %q", dec.Question)
	}

	// A vetoed proposal cannot be approved or denied; the veto is binding,
	// not advisory.
	if _, _, err := d.approve(dec, time.Now()); err == nil {
		t.Fatal("approving a conflicted proposal must error")
	}
	if _, err := d.deny(dec, time.Now()); err == nil {
		t.Fatal("denying a conflicted proposal must error")
	}

	// Clarify toward git; the conflict should resolve to it.
	d.teach("clean up", "git clean -fd", base.Add(50*time.Hour))
	d.teach("clean up", "git clean -fd", base.Add(51*time.Hour))
	dec, _ = d.decide("clean up")
	if dec.Conflict {
		t.Fatalf("clarification should break the tie, still conflicting: %+v", dec)
	}
	if dec.Proposed.Command != "git clean -fd" {
		t.Fatalf("the clarified command should win, got %+v", dec.Proposed)
	}
}

// Denial demotes the pair, weakens the grounding, and records the denial.
func TestDenyDemotes(t *testing.T) {
	d, s := testDaemon(t)
	ground(t, d, "deploy it", "make deploy", 4)
	now := time.Now()

	dec, _ := d.decide("deploy it")
	before := dec.Proposed.Saliency

	h, err := d.deny(dec, now)
	if err != nil {
		t.Fatal(err)
	}
	if h.Status != HabitDemoted || h.Denials != 1 {
		t.Fatalf("denial should demote, got %+v", h)
	}

	after, _ := d.decide("deploy it")
	if after.Proposed.Saliency >= before {
		t.Fatalf("denial should weaken the grounding: %.3f then %.3f", before, after.Proposed.Saliency)
	}

	entries, _ := s.AuditTail(5)
	if len(entries) == 0 || entries[0].Outcome != "denied" {
		t.Fatalf("denial should be audited, got %+v", entries)
	}
}

// decide weighs memory without changing it: no habit, audit, or fragment
// is written by a proposal.
func TestDecideIsReadOnly(t *testing.T) {
	d, s := testDaemon(t)
	ground(t, d, "run the tests", "go test ./...", 4)

	habitsBefore, _ := s.AllHabits()
	auditBefore, _ := s.AuditTail(1000)
	fragsBefore, _ := s.Count()

	for i := 0; i < 3; i++ {
		if _, err := d.decide("run the tests"); err != nil {
			t.Fatal(err)
		}
	}

	habitsAfter, _ := s.AllHabits()
	auditAfter, _ := s.AuditTail(1000)
	fragsAfter, _ := s.Count()
	if len(habitsAfter) != len(habitsBefore) {
		t.Fatalf("decide wrote a habit: %d -> %d", len(habitsBefore), len(habitsAfter))
	}
	if len(auditAfter) != len(auditBefore) {
		t.Fatalf("decide wrote an audit row: %d -> %d", len(auditBefore), len(auditAfter))
	}
	if fragsAfter != fragsBefore {
		t.Fatalf("decide encoded a fragment: %d -> %d", fragsBefore, fragsAfter)
	}
}

// Approval strengthens the grounding it acted on, so the same cue proposes
// the same command more strongly next time.
func TestApprovalStrengthens(t *testing.T) {
	d, _ := testDaemon(t)
	ground(t, d, "format the code", "gofmt -w .", 4)

	before, _ := d.decide("format the code")
	if _, _, err := d.approve(before, time.Now()); err != nil {
		t.Fatal(err)
	}
	after, _ := d.decide("format the code")
	if after.Proposed.Saliency <= before.Proposed.Saliency {
		t.Fatalf("approval should strengthen the grounding: %.3f then %.3f",
			before.Proposed.Saliency, after.Proposed.Saliency)
	}
}

// Saliency aggregates every grounded occurrence of a command, so a cue
// taught three times carries three contributing action fragments.
func TestSaliencyAggregatesOccurrences(t *testing.T) {
	d, _ := testDaemon(t)
	ground(t, d, "say hello", "echo hi", 3)

	dec, _ := d.decide("say hello")
	if dec.Proposed == nil || len(dec.Proposed.Members) != 3 {
		t.Fatalf("three groundings should contribute three members, got %+v", dec.Proposed)
	}
}

// Approval is recorded in the durable audit trail with its outcome.
func TestApprovalAudited(t *testing.T) {
	d, s := testDaemon(t)
	ground(t, d, "list files", "ls -la", 3)
	dec, _ := d.decide("list files")
	if _, _, err := d.approve(dec, time.Now()); err != nil {
		t.Fatal(err)
	}
	entries, _ := s.AuditTail(5)
	if len(entries) == 0 {
		t.Fatal("approval should be audited")
	}
	if entries[0].Action != "ls -la" || entries[0].Outcome == "" {
		t.Fatalf("audit entry malformed: %+v", entries[0])
	}
}
