package daemon

import (
	"strings"
	"testing"

	"golem/wire"
)

// Asked for a computation it does not know, GOLEM invites the speaker to teach it and
// holds the question. Shown the rule one example at a time, it gathers the examples,
// assembles the rule by running once two stand, learns it, and answers the question
// that began the exchange on the very turn it learns.
func TestComputeIntentLearnsByExamples(t *testing.T) {
	d, _ := testDaemon(t)
	d.cfg.RuntimeEnabled = true

	r, _, _ := d.handle(wire.Request{Cmd: "runcode", Content: "the triple of 4"}, nil)
	if !strings.Contains(r.Say, "triple") || d.pendingCompute == nil {
		t.Fatalf("an unknown computation should be elicited and held: %q pending=%v", r.Say, d.pendingCompute)
	}
	one := say(t, d, "the triple of 2 is 6")
	if !strings.Contains(strings.ToLower(one.Say), "one more") {
		t.Errorf("one example is too few; GOLEM should ask for another: %q", one.Say)
	}
	two := say(t, d, "the triple of 5 is 15")
	if !strings.Contains(two.Say, "triple") || !strings.Contains(two.Say, "the triple of 4 is 12") {
		t.Errorf("two examples should learn the triple and answer the held question: %q", two.Say)
	}
	if d.pendingCompute != nil {
		t.Error("the loop should close once the operation is learned")
	}
	r3, _, _ := d.handle(wire.Request{Cmd: "runcode", Content: "the triple of 7"}, nil)
	if !strings.Contains(r3.Say, "21") {
		t.Errorf("a learned computation answers directly afterward: %q", r3.Say)
	}
}

// The same loop accepts a definition instead of examples: told what the operation is,
// GOLEM learns it and answers the held question the same way.
func TestComputeIntentLearnsByDefinition(t *testing.T) {
	d, _ := testDaemon(t)
	d.cfg.RuntimeEnabled = true

	d.handle(wire.Request{Cmd: "runcode", Content: "the triple of 4"}, nil)
	r := say(t, d, "the triple of a number is the product of the number and 3")
	if !strings.Contains(r.Say, "the triple of 4 is 12") {
		t.Errorf("a definition should learn the operation and answer the held question: %q", r.Say)
	}
	if d.pendingCompute != nil {
		t.Error("the loop should close on a definition too")
	}
}

// A richer unknown surface the one-example loop does not assemble still admits the gap
// rather than inviting examples it cannot use.
func TestComputeIntentLeavesRicherGap(t *testing.T) {
	d, _ := testDaemon(t)
	d.cfg.RuntimeEnabled = true
	r, _, _ := d.handle(wire.Request{Cmd: "runcode", Content: "the blend of 6 and 7"}, nil)
	if !strings.Contains(strings.ToLower(r.Say), "do not know") || d.pendingCompute != nil {
		t.Errorf("a binary unknown call should admit the gap, not start a unary loop: %q pending=%v", r.Say, d.pendingCompute)
	}
}
