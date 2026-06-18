package daemon

import (
	"strings"
	"testing"

	"golem/wire"
)

// The code constructions are seeded into memory like the rest of the grammar,
// so the engine comprehends a computation from its surface.
func TestCodeEngineSeeded(t *testing.T) {
	d, _ := testDaemon(t)
	if d.code == nil {
		t.Fatal("the code engine should be built from the seeded constructions")
	}
	comp, ok := d.code.Comprehend("the product of 6 and 7")
	if !ok || comp.Op != "multiply" {
		t.Fatalf("a seeded code construction should comprehend: %+v ok %v", comp, ok)
	}
}

// The whole loop: a computation stated in language is comprehended to its
// meaning, the source generated from it, run in the runtime, and the result
// grounded, so the construction's meaning is tied to what the code does.
func TestComputeGroundsAComprehendedComputation(t *testing.T) {
	d, _ := testDaemon(t)
	d.cfg.RuntimeEnabled = true

	resp, _, _ := d.handle(wire.Request{Cmd: "runcode", Content: "the sum of 6 and 7"}, nil)
	if !resp.OK {
		t.Fatalf("runcode failed: %s", resp.Error)
	}
	if !strings.Contains(resp.Say, "13") {
		t.Fatalf("the computation should comprehend, generate code, and run to 13: %q", resp.Say)
	}
}

// A call of an operation GOLEM has not learned admits the gap by name instead
// of handing the words to the compiler as a broken program. The runtime is open
// so a fall-through would surface a Go error, which the admission must replace.
func TestComputeAdmitsUnknownFunction(t *testing.T) {
	d, _ := testDaemon(t)
	d.cfg.RuntimeEnabled = true

	resp, _, _ := d.handle(wire.Request{Cmd: "runcode", Content: "the cube of 3"}, nil)
	if !resp.OK {
		t.Fatalf("runcode failed: %s", resp.Error)
	}
	if !strings.Contains(resp.Say, "do not know") || !strings.Contains(resp.Say, "cube") {
		t.Fatalf("an unlearned function should be admitted by name, not run: %q", resp.Say)
	}
}

// An ill-formed computation (a binary operation given a third operand) is a gap
// to admit too, so a recognizable computation request never leaks a Go error.
func TestComputeAdmitsIllFormedComputation(t *testing.T) {
	d, _ := testDaemon(t)
	d.cfg.RuntimeEnabled = true

	resp, _, _ := d.handle(wire.Request{Cmd: "runcode", Content: "the sum of 2 and 3 and 4"}, nil)
	if !resp.OK {
		t.Fatalf("runcode failed: %s", resp.Error)
	}
	if strings.Contains(resp.Say, "error:") {
		t.Fatalf("an ill-formed computation should be admitted, not leaked as a Go error: %q", resp.Say)
	}
}

// The built-in collection operations run over a list literal and ground to the
// value the code computes: a total, a length, a first, a largest.
func TestComputesOverLists(t *testing.T) {
	d, _ := testDaemon(t)
	d.cfg.RuntimeEnabled = true

	cases := []struct{ surface, want string }{
		{"the total of the list of 1 and 2 and 3", "6"},
		{"the length of the list of 1 and 2 and 3", "3"},
		{"the first of the list of 4 and 5 and 6", "4"},
		{"the largest of the list of 3 and 9 and 2", "9"},
	}
	for _, tc := range cases {
		resp, _, _ := d.handle(wire.Request{Cmd: "runcode", Content: tc.surface}, nil)
		if !resp.OK || !strings.Contains(resp.Say, tc.want) {
			t.Fatalf("%q should run to %s: %q", tc.surface, tc.want, resp.Say)
		}
	}
	// A collection operation on a number is admitted as a gap, not run to a Go error.
	resp, _, _ := d.handle(wire.Request{Cmd: "runcode", Content: "the total of 5"}, nil)
	if strings.Contains(resp.Say, "error:") {
		t.Fatalf("a list operation on a number should be admitted, not leaked: %q", resp.Say)
	}
}

// A nested expression in real Go comprehends to its computation, generates back
// to grouped source, and grounds by running to the value its grouping means.
func TestComputeGroundsNestedExpression(t *testing.T) {
	d, _ := testDaemon(t)
	d.cfg.RuntimeEnabled = true

	resp, _, _ := d.handle(wire.Request{Cmd: "runcode", Content: "6 * (7 + 1)"}, nil)
	if !resp.OK {
		t.Fatalf("runcode failed: %s", resp.Error)
	}
	if !strings.Contains(resp.Say, "48") {
		t.Fatalf("the nested expression should comprehend, regenerate, and run to 48: %q", resp.Say)
	}
}
