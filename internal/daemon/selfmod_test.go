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
