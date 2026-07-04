package daemon

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"golem/internal/selfbuild"
	"golem/wire"
)

// selfModModule writes a tiny module to stand in for GOLEM's own source, so a
// self-change can be exercised without building the whole tree.
func selfModModule(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	write := func(name, body string) {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("go.mod", "module probe\n\ngo 1.21\n")
	write("main.go", "package main\n\nimport \"fmt\"\n\nfunc greet() string { return \"hello\" }\n\nfunc main() { fmt.Println(greet()) }\n")
	write("main_test.go", "package main\n\nimport \"testing\"\n\nfunc TestGreet(t *testing.T) {\n\tif greet() == \"\" {\n\t\tt.Fatal(\"greeting is empty\")\n\t}\n}\n")
	return dir
}

// tunableModule writes a tiny module with a tuning constant a check reads, so a
// retune read out of its own source can be exercised: lifting the threshold
// keeps the test passing, raising it past the checked value fails the test.
func tunableModule(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	write := func(name, body string) {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("go.mod", "module probe\n\ngo 1.21\n")
	write("main.go", "package main\n\nimport \"fmt\"\n\n// threshold is the cutoff over compares against.\nconst threshold = 3\n\nfunc over(n int) bool { return n > threshold }\n\nfunc main() { fmt.Println(over(5)) }\n")
	write("main_test.go", "package main\n\nimport \"testing\"\n\nfunc TestOver(t *testing.T) {\n\tif !over(5) {\n\t\tt.Fatal(\"5 should clear the threshold\")\n\t}\n}\n")
	return dir
}

func improvedFitness() fitnessVerdict {
	return fitnessVerdict{improved: true, reason: "improved goal progress"}
}
func flatFitness() fitnessVerdict {
	return fitnessVerdict{improved: false, reason: "no grounded improvement"}
}

func rename(from, to string) selfbuild.Change {
	return selfbuild.Change{File: "main.go", Why: "change the greeting",
		Apply: func(s string) (string, error) { return strings.Replace(s, from, to, 1), nil }}
}

// Self-modification is closed by default: it takes both the runtime open and a
// source path set, two opt-ins, before GOLEM may rebuild itself.
func TestSelfModGatedShut(t *testing.T) {
	d, _ := testDaemon(t)
	res := d.proposeSelfChange(rename(`"hello"`, `"howdy"`), improvedFitness)
	if res.Accepted || res.Stage != "gate" {
		t.Fatalf("self-modification should be closed by default: %+v", res)
	}
}

// With both gates open, a change that compiles, tests clean, and improves a
// grounded outcome is accepted and its candidate staged, the live binary never
// touched.
func TestSelfModAcceptsImprovingChange(t *testing.T) {
	d, _ := testDaemon(t)
	d.home = t.TempDir()
	d.cfg.RuntimeEnabled = true
	d.cfg.SourcePath = selfModModule(t)
	d.cfg.MainPackage = "."

	res := d.proposeSelfChange(rename(`"hello"`, `"howdy"`), improvedFitness)
	if !res.Accepted {
		t.Fatalf("an improving, clean change should be accepted: %+v", res)
	}
	if _, err := os.Stat(res.Candidate); err != nil {
		t.Fatalf("the candidate should be staged for deploy: %v", err)
	}
	if !strings.HasPrefix(res.Candidate, d.home) {
		t.Fatalf("the candidate must stage under home, never the live binary: %q", res.Candidate)
	}
}

// A change that compiles and tests clean but does not improve a grounded
// outcome is reverted: the candidate is discarded and the prior stands.
func TestSelfModRevertsRegressingChange(t *testing.T) {
	d, _ := testDaemon(t)
	d.home = t.TempDir()
	d.cfg.RuntimeEnabled = true
	d.cfg.SourcePath = selfModModule(t)
	d.cfg.MainPackage = "."

	res := d.proposeSelfChange(rename(`"hello"`, `"howdy"`), flatFitness)
	if res.Accepted || res.Stage != "fitness" {
		t.Fatalf("a change that does not improve should be reverted at fitness: %+v", res)
	}
	if _, err := os.Stat(filepath.Join(d.home, "stage", "golem-candidate")); err == nil {
		t.Fatal("the candidate should be discarded when fitness rejects it")
	}
}

// A change that fails to compile is rejected at the build stage, before fitness
// is even consulted, the prior binary the floor.
func TestSelfModRejectsBrokenChange(t *testing.T) {
	d, _ := testDaemon(t)
	d.home = t.TempDir()
	d.cfg.RuntimeEnabled = true
	d.cfg.SourcePath = selfModModule(t)
	d.cfg.MainPackage = "."

	broken := selfbuild.Change{File: "main.go", Why: "break it",
		Apply: func(s string) (string, error) { return s + "\nnot valid go\n", nil }}
	res := d.proposeSelfChange(broken, improvedFitness)
	if res.Accepted || res.Stage != "build" {
		t.Fatalf("a broken change should be rejected at build: %+v", res)
	}
}

// A change read out of GOLEM's own source runs the same pipeline: it reads the
// tuning constant, proposes a new value, and the AST-located rewrite builds,
// tests, and improves, so the candidate is staged and the original untouched.
func TestRetuneFromOwnSourceAccepted(t *testing.T) {
	d, _ := testDaemon(t)
	d.home = t.TempDir()
	d.cfg.RuntimeEnabled = true
	d.cfg.SourcePath = tunableModule(t)
	d.cfg.MainPackage = "."

	res := d.proposeRetune("main.go", "threshold", "4", "tighten the cutoff", improvedFitness)
	if !res.Accepted {
		t.Fatalf("a clean, improving retune should be accepted: %+v", res)
	}
	if !strings.HasPrefix(res.Candidate, d.home) {
		t.Fatalf("the candidate must stage under home, never the live binary: %q", res.Candidate)
	}
	orig, _ := os.ReadFile(filepath.Join(d.cfg.SourcePath, "main.go"))
	if !strings.Contains(string(orig), "const threshold = 3") {
		t.Fatal("the original source must be untouched by the pipeline")
	}
}

// A retune that breaks the tests is caught: raising the threshold past the
// checked value fails the test, so the change is rejected and the prior stands.
func TestRetuneRejectedWhenItBreaksTests(t *testing.T) {
	d, _ := testDaemon(t)
	d.home = t.TempDir()
	d.cfg.RuntimeEnabled = true
	d.cfg.SourcePath = tunableModule(t)
	d.cfg.MainPackage = "."

	res := d.proposeRetune("main.go", "threshold", "5", "over-tighten", improvedFitness)
	if res.Accepted || res.Stage != "test" {
		t.Fatalf("a retune that fails the tests should be rejected at test: %+v", res)
	}
}

// A retune of a constant that is not there is declined at the reading, before
// any build: the change has to come from a declaration that was actually found.
func TestRetuneUnknownConstDeclined(t *testing.T) {
	d, _ := testDaemon(t)
	d.home = t.TempDir()
	d.cfg.RuntimeEnabled = true
	d.cfg.SourcePath = tunableModule(t)
	d.cfg.MainPackage = "."

	res := d.proposeRetune("main.go", "missing", "1", "retune a ghost", improvedFitness)
	if res.Accepted || res.Stage != "read" {
		t.Fatalf("a retune of an absent constant should be declined at read: %+v", res)
	}
}

// A retune is gated shut by default, like any self-change.
func TestRetuneGatedShut(t *testing.T) {
	d, _ := testDaemon(t)
	res := d.proposeRetune("main.go", "threshold", "4", "tighten", improvedFitness)
	if res.Accepted || res.Stage != "gate" {
		t.Fatalf("a retune should be closed by default: %+v", res)
	}
}

// The file named in a retune must live inside the source tree: a path that
// climbs out is refused at the reading, before anything is opened.
func TestRetuneRefusesPathsOutsideTheTree(t *testing.T) {
	d, _ := testDaemon(t)
	d.home = t.TempDir()
	d.cfg.RuntimeEnabled = true
	d.cfg.SourcePath = tunableModule(t)
	d.cfg.MainPackage = "."
	// A real file outside the tree, reachable only by climbing.
	outside := filepath.Join(filepath.Dir(d.cfg.SourcePath), "outside.go")
	if err := os.WriteFile(outside, []byte("package x\n\nconst threshold = 3\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, file := range []string{"../outside.go", "..", "a/../../outside.go"} {
		res := d.proposeRetune(file, "threshold", "4", "climb out", improvedFitness)
		if res.Accepted || res.Stage != "read" ||
			!strings.Contains(res.Err, "inside the source tree") {
			t.Fatalf("a climbing path %q should be refused at read: %+v", file, res)
		}
	}
}

// The off-loop entry hands the build to a goroutine and returns at once: the
// verdict must not land until the loop settles it through execDone, the build
// holds a run slot while it works, a second self-change is refused while one
// builds, and the settled verdict matches the in-line pipeline's.
// drainActions plays the Run loop's part here.
func TestSelfChangeBuildsOffTheLoop(t *testing.T) {
	d, _ := testDaemon(t)
	d.home = t.TempDir()
	d.cfg.RuntimeEnabled = true
	d.cfg.SourcePath = selfModModule(t)
	d.cfg.MainPackage = "."

	var got *selfbuild.Result
	refusal := d.beginSelfChange(rename(`"hello"`, `"howdy"`), operatorCandidate, improvedFitness,
		func(res selfbuild.Result) { got = &res })
	if refusal != "" {
		t.Fatalf("an open gate should launch the build: %s", refusal)
	}
	if got != nil {
		t.Fatal("the verdict must not land before the loop settles it")
	}
	if len(d.running) != 1 {
		t.Fatalf("the build should hold a run slot while it works: %d", len(d.running))
	}
	if r := d.beginSelfChange(rename(`"hello"`, `"hiya"`), operatorCandidate, improvedFitness,
		func(selfbuild.Result) {}); !strings.Contains(r, "already building") {
		t.Fatalf("a second self-change should be refused while one builds: %q", r)
	}
	d.drainActions()
	if got == nil || !got.Accepted {
		t.Fatalf("the settled verdict should accept the clean change: %+v", got)
	}
	if !strings.HasPrefix(got.Candidate, d.home) {
		t.Fatalf("the candidate must stage under home: %q", got.Candidate)
	}
	if len(d.running) != 0 {
		t.Fatal("the run slot should be released when the build settles")
	}
}

// The conversational retune replies before the toolchain runs and speaks the
// verdict when the build lands: the reply promises the outcome, and after the
// loop settles the build the spoken line says where the candidate waits.
func TestRetuneEntryRepliesBeforeTheBuild(t *testing.T) {
	d, _ := testDaemon(t)
	d.home = t.TempDir()
	d.cfg.RuntimeEnabled = true
	d.cfg.SourcePath = tunableModule(t)
	d.cfg.MainPackage = "."
	d.reqPrincipal = principal{architectSigned: true}
	var out bytes.Buffer
	d.out = &out

	resp := d.handleRetune(wire.Request{Query: "main.go", Arg: "threshold", Content: "4"})
	if !strings.Contains(resp.Say, "building") {
		t.Fatalf("the reply should promise the build, not wait for it: %q", resp.Say)
	}
	if strings.Contains(out.String(), "deploy") {
		t.Fatalf("the verdict must not be spoken before the build lands: %q", out.String())
	}
	d.drainActions()
	said := out.String()
	if !strings.Contains(said, "built and tested clean") ||
		!strings.Contains(said, "for the architect to deploy") {
		t.Fatalf("the settled verdict should be spoken with where the candidate waits: %q", said)
	}
}
