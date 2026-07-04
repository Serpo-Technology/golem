package daemon

import (
	"bytes"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"golem/internal/code"
)

// wantLower and wantRaise mirror the review's own arithmetic, so the suite
// asserts behavior rather than a literal post-retune value and stays green
// whatever experimentConf is compiled to (including a value the mind itself
// retuned it to). Each returns the proposed target and whether a change is
// possible at all; at a bound the review forms nothing.
func wantLower() (float64, bool) {
	to := math.Max(selftuneFloor, round2(experimentConf-selftuneStepDown))
	return to, to < experimentConf
}
func wantRaise() (float64, bool) {
	to := math.Min(selftuneCeil, round2(experimentConf+selftuneStepUp))
	return to, to > experimentConf
}

// selftuneModule writes a tiny module shaped like the real tree at the one
// path the review may retune, so the act's build can be exercised without
// compiling GOLEM itself.
func selftuneModule(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	write := func(name, body string) {
		path := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("go.mod", "module probe\n\ngo 1.21\n")
	write("internal/daemon/experiment.go",
		"package daemon\n\nconst experimentConf = 0.6\n\nfunc Conf() float64 { return experimentConf }\n")
	write("main.go", "package main\n\nimport (\n\t\"fmt\"\n\n\t\"probe/internal/daemon\"\n)\n\nfunc main() { fmt.Println(daemon.Conf()) }\n")
	write("main_test.go", "package main\n\nimport (\n\t\"testing\"\n\n\t\"probe/internal/daemon\"\n)\n\nfunc TestConf(t *testing.T) {\n\tif daemon.Conf() <= 0 {\n\t\tt.Fatal(\"the confidence should be positive\")\n\t}\n}\n")
	return dir
}

// openSandbox opens both self-modification gates over a throwaway source
// path, so a review may form a wish (formation is gated on the sandbox).
func openSandbox(t *testing.T, d *Daemon) {
	t.Helper()
	d.cfg.RuntimeEnabled = true
	d.cfg.SourcePath = t.TempDir()
}

// seedExperiment writes one experiment outcome onto the trail the way the
// organ does.
func seedExperiment(t *testing.T, d *Daemon, at int64, pair, outcome string) {
	t.Helper()
	if err := d.store.Audit(at, "experiment", pair, outcome); err != nil {
		t.Fatal(err)
	}
}

// A rule held one wake and refuted on a later one is a betrayal, and a
// betrayed record argues the confidence is set too high: the review forms a
// proposal to lower it, on the trail and on the voice, held pending for the
// retune act.
func TestBetrayalProposesLoweringTheConfidence(t *testing.T) {
	d, s := testDaemon(t)
	var out bytes.Buffer
	d.out = &out
	openSandbox(t, d)
	seedExperiment(t, d, 1000, "inverse(double, half)", "held over 3 runs")
	seedExperiment(t, d, 2000, "inverse(double, half)", "refuted after 1 run(s)")

	want, canLower := wantLower()
	if !canLower {
		t.Skip("experimentConf is already at the floor; nothing to lower")
	}
	d.reviewExperimentTuning()
	p := d.pendingRetune
	if p == nil || p.name != "experimentConf" || p.to != want {
		t.Fatalf("a betrayal should propose lowering the confidence a step to %v: %+v", want, p)
	}
	rows, err := s.AuditByCue("selftune")
	wantRow := "proposed " + sayNumber(experimentConf) + " -> " + sayNumber(want)
	if err != nil || len(rows) != 1 || !strings.Contains(rows[0].Outcome, wantRow) {
		t.Fatalf("the proposal should be on the trail: %v %+v", err, rows)
	}
	if said := out.String(); !strings.Contains(said, "I think experimentConf should be "+sayNumber(want)) ||
		!strings.Contains(said, "the deploy stays the architect's") {
		t.Fatalf("the wish should be spoken with its evidence and its bounds: %q", said)
	}
}

// Rules that keep faith wake after wake argue the confidence undersells
// them: enough reconfirmations with no betrayal propose a nudge up, still
// bounded well under grounded certainty.
func TestSteadyReconfirmationProposesARaise(t *testing.T) {
	d, _ := testDaemon(t)
	openSandbox(t, d)
	for i, pair := range []string{"inverse(double, half)", "inverse(up, down)"} {
		base := int64(1000 * (i + 1))
		seedExperiment(t, d, base, pair, "held over 3 runs")
		seedExperiment(t, d, base+10000, pair, "held over 3 runs")
		seedExperiment(t, d, base+20000, pair, "held over 3 runs")
	}
	want, canRaise := wantRaise()
	if !canRaise {
		t.Skip("experimentConf is already at the ceiling; nothing to raise")
	}
	d.reviewExperimentTuning()
	p := d.pendingRetune
	if p == nil || p.to != want {
		t.Fatalf("a faithful record should propose raising the confidence a notch to %v: %+v", want, p)
	}
}

// A test the budget cut short says nothing about the rule: stopped rows are
// not betrayals, so a record of stops after a held rule proposes nothing.
func TestStoppedRunsAreNotBetrayals(t *testing.T) {
	d, _ := testDaemon(t)
	openSandbox(t, d)
	seedExperiment(t, d, 1000, "inverse(double, half)", "held over 3 runs")
	seedExperiment(t, d, 2000, "inverse(double, half)", "stopped after 1 run(s)")
	seedExperiment(t, d, 3000, "inverse(double, half)", "stopped after 0 run(s)")

	d.reviewExperimentTuning()
	if d.pendingRetune != nil {
		t.Fatalf("stopped tests are no evidence: %+v", d.pendingRetune)
	}
}

// Refutations of rules that were never held say nothing about the held
// confidence, and a thin record clears no threshold either way.
func TestQuietTrailProposesNothing(t *testing.T) {
	d, _ := testDaemon(t)
	openSandbox(t, d)
	seedExperiment(t, d, 1000, "inverse(double, up)", "refuted after 1 run(s)")
	seedExperiment(t, d, 2000, "inverse(up, double)", "refuted after 1 run(s)")
	seedExperiment(t, d, 3000, "inverse(double, half)", "held over 3 runs")

	d.reviewExperimentTuning()
	if d.pendingRetune != nil {
		t.Fatalf("an unbetrayed, thin record should propose nothing: %+v", d.pendingRetune)
	}
}

// A shut sandbox forms no wish: the mind never promises a build it cannot
// make, even when the evidence would otherwise argue one.
func TestShutSandboxFormsNoWish(t *testing.T) {
	d, s := testDaemon(t)
	seedExperiment(t, d, 1000, "inverse(double, half)", "held over 3 runs")
	seedExperiment(t, d, 2000, "inverse(double, half)", "refuted after 1 run(s)")

	d.reviewExperimentTuning()
	if d.pendingRetune != nil {
		t.Fatalf("a shut sandbox must form no wish: %+v", d.pendingRetune)
	}
	if rows, _ := s.AuditByCue("selftune"); len(rows) != 0 {
		t.Fatalf("nothing should reach the trail with the sandbox shut: %+v", rows)
	}
}

// A built candidate splits the evidence window: the betrayals it was built on
// are spent, so the same record argues no second proposal.
func TestBuiltCandidateSplitsTheWindow(t *testing.T) {
	d, _ := testDaemon(t)
	openSandbox(t, d)
	seedExperiment(t, d, 1000, "inverse(double, half)", "held over 3 runs")
	seedExperiment(t, d, 2000, "inverse(double, half)", "refuted after 1 run(s)")
	d.auditSelftune("experimentConf", "proposed 0.6 -> 0.5: a betrayal")
	if err := d.store.Audit(2500, "selftune", "experimentConf",
		"built 0.5: candidate at /stage/golem-candidate-selftune"); err != nil {
		t.Fatal(err)
	}

	d.reviewExperimentTuning()
	if d.pendingRetune != nil {
		t.Fatalf("a built candidate should spend its evidence: %+v", d.pendingRetune)
	}
}

// A bare proposal that never built does not spend its evidence: a wish lost
// to a restart or a busy pipeline lets the same betrayals argue again, so the
// review self-heals rather than dropping the change silently.
func TestUnbuiltProposalDoesNotSpendEvidence(t *testing.T) {
	d, _ := testDaemon(t)
	openSandbox(t, d)
	seedExperiment(t, d, 1000, "inverse(double, half)", "held over 3 runs")
	seedExperiment(t, d, 2000, "inverse(double, half)", "refuted after 1 run(s)")
	// A proposal was formed on an earlier night but never built (no "built"
	// row follows it), then the wish was lost. Its target is deliberately not
	// the running value, so it reads as unbuilt rather than deployed.
	notRunning := sayNumber(round2(experimentConf + 0.13))
	if err := d.store.Audit(2500, "selftune", "experimentConf",
		"proposed 0.4 -> "+notRunning+": a betrayal"); err != nil {
		t.Fatal(err)
	}

	want, canLower := wantLower()
	if !canLower {
		t.Skip("experimentConf is already at the floor; nothing to lower")
	}
	d.reviewExperimentTuning()
	if d.pendingRetune == nil || d.pendingRetune.to != want {
		t.Fatalf("an unbuilt proposal should let the evidence argue again to %v: %+v", want, d.pendingRetune)
	}
}

// When the running value matches a past ask, the architect deployed it, and
// the review marks the deploy once without fabricating a count over evidence
// from the binary the retune replaced.
func TestDeployedRetuneMarkedOnce(t *testing.T) {
	d, s := testDaemon(t)
	var out bytes.Buffer
	d.out = &out
	openSandbox(t, d)
	// The past ask's target is the value now running, which is how the review
	// recognizes the architect deployed it, at whatever experimentConf is.
	if err := s.Audit(500, "selftune", "experimentConf",
		"proposed 0.4 -> "+sayNumber(experimentConf)+": a past ask"); err != nil {
		t.Fatal(err)
	}
	seedExperiment(t, d, 1000, "inverse(double, half)", "held over 3 runs")
	seedExperiment(t, d, 2000, "inverse(double, half)", "held over 3 runs")

	d.reviewExperimentTuning()
	if !strings.Contains(out.String(), "deployed") {
		t.Fatalf("a deployed ask should be spoken as such: %q", out.String())
	}
	if d.pendingRetune != nil {
		t.Fatalf("the deploy night forms no new proposal: %+v", d.pendingRetune)
	}
	d.reviewExperimentTuning()
	rows, _ := s.AuditByCue("selftune")
	verdicts := 0
	for _, r := range rows {
		if strings.HasPrefix(r.Outcome, "deployed") {
			verdicts++
		}
	}
	if verdicts != 1 {
		t.Fatalf("the deploy should be marked once, not each night: %+v", rows)
	}
}

// The leash holds regardless of the signal: a proposal naming a constant on
// the denylist is refused on record, never held pending.
func TestLeashedConstantIsRefused(t *testing.T) {
	d, s := testDaemon(t)
	d.holdRetuneProposal("selfBuildBudget", 5, 50, "a runaway signal")
	if d.pendingRetune != nil {
		t.Fatal("a leashed constant must never be held for retuning")
	}
	rows, _ := s.AuditByCue("selftune")
	if len(rows) != 1 || !strings.Contains(rows[0].Outcome, "leashed") {
		t.Fatalf("the refusal should be on the trail: %+v", rows)
	}
}

// The retune act takes the pending wish through the same off-loop pipeline
// an operator's retune takes, under its own candidate name so it never
// overwrites an operator's staged binary: the outcome is spoken and marked
// built when the build lands, the candidate staged, and the wish cleared on
// hand-off.
func TestRetuneActBuildsThePendingWish(t *testing.T) {
	d, s := testDaemon(t)
	d.home = t.TempDir()
	d.cfg.RuntimeEnabled = true
	d.cfg.SourcePath = selftuneModule(t)
	d.cfg.MainPackage = "."
	var out bytes.Buffer
	d.out = &out
	d.pendingRetune = &retuneProposal{name: "experimentConf", from: 0.6, to: 0.5,
		reason: "1 held rule(s) refuted on a later wake against 0 reconfirmation(s)",
		at:     time.Now()}

	outcome, fruitful := d.actRetuneNow()
	if !fruitful || !strings.Contains(outcome, "building my retune") {
		t.Fatalf("the act should hand the build off and say so: %q %v", outcome, fruitful)
	}
	if d.pendingRetune != nil {
		t.Fatal("the wish clears when the build is handed off")
	}
	d.drainActions()
	said := out.String()
	if !strings.Contains(said, "I built and tested my retune of experimentConf to 0.5") ||
		!strings.Contains(said, "for the architect to deploy") {
		t.Fatalf("the landed verdict should be spoken with where the candidate waits: %q", said)
	}
	cand := filepath.Join(d.home, "stage", selftuneCandidate)
	if _, err := os.Stat(cand); err != nil {
		t.Fatalf("the candidate should be staged under its own name: %v", err)
	}
	rows, _ := s.AuditByCue("selftune")
	if len(rows) == 0 || !strings.HasPrefix(rows[len(rows)-1].Outcome, "built") {
		t.Fatalf("a built candidate should be marked on the trail: %+v", rows)
	}
}

// The selftune drive presses only while a wish is pending and the sandbox is
// open, and the busy check sees a running self-build so a second one waits.
func TestRetuneDriveAndReadinessGates(t *testing.T) {
	d, _ := testDaemon(t)
	now := time.Now()
	hasDrive := func() bool {
		for _, dr := range d.computeDrives(now, false) {
			if dr.name == "selftune" {
				return true
			}
		}
		return false
	}
	if hasDrive() {
		t.Fatal("no wish, no drive")
	}
	d.pendingRetune = &retuneProposal{name: "experimentConf", from: 0.6, to: 0.5,
		reason: "the record argued it", at: now.Add(-time.Minute)}
	if hasDrive() {
		t.Fatal("a shut sandbox keeps the drive quiet")
	}
	d.cfg.RuntimeEnabled = true
	d.cfg.SourcePath = t.TempDir()
	if !hasDrive() {
		t.Fatal("an open sandbox with a pending wish should press")
	}
	d.running[999] = &actionRun{cmd: "selfbuild internal/daemon/experiment.go"}
	if !d.selfBuildRunning() {
		t.Fatal("the busy check should see the running build")
	}
}

// A budget already spent stops an experiment before its first run, and the
// stop reports itself as a stop: no completed run, no refutation.
func TestExperimentStoppedByBudgetIsNoRefutation(t *testing.T) {
	d, _ := testDaemon(t)
	d.computeRuns = computeRunsPerWake
	holds, tried, stopped := d.runExperiment(code.Op{Name: "f"}, code.Op{Name: "g"})
	if holds || tried != 0 || !stopped {
		t.Fatalf("a starved test must report stopped, not refuted: %v %d %v", holds, tried, stopped)
	}
}
