package daemon

import (
	"testing"
)

// The runtime is closed by default: running code GOLEM wrote waits for the
// operator to open the gate inside the sandbox.
func TestRunProgramGatedShut(t *testing.T) {
	d, _ := testDaemon(t)
	res := d.runProgram("6 * 7")
	if res.OK || res.Err == "" {
		t.Fatalf("the runtime should be closed by default: %+v", res)
	}
}

// With the gate open, GOLEM runs a program, grounds its result as a percept,
// and leaves the program as a readable artifact in the graph.
func TestRunProgramComputesGroundsAndRecords(t *testing.T) {
	d, s := testDaemon(t)
	d.cfg.RuntimeEnabled = true

	res := d.runProgram("6 * 7")
	if !res.OK || res.Value != "42" {
		t.Fatalf("the runtime should compute the program: %+v", res)
	}
	// The program itself is an artifact in the graph.
	if src, _ := s.EpisodesByExactShape("6 * 7", 4); len(src) == 0 {
		t.Fatal("the program should be written to the graph as an artifact")
	}
	// The result is grounded as a percept the brain can recall.
	if got, _ := s.EpisodesByExactShape("42", 4); len(got) == 0 {
		t.Fatal("the result should be grounded as a percept")
	}
}

// A failing program is contained to an error GOLEM learns from, gate open, the
// loop and the daemon unharmed.
func TestRunProgramContainsFailure(t *testing.T) {
	d, _ := testDaemon(t)
	d.cfg.RuntimeEnabled = true
	res := d.runProgram(`panic("boom")`)
	if res.OK || res.Err == "" {
		t.Fatalf("a failing program should be contained to an error: %+v", res)
	}
}
