package daemon

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"golem/internal/selfbuild"
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
