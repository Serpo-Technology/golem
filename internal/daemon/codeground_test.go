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
