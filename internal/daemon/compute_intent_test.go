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

// The same loop learns a computation over a LIST. Asked for an unknown list operation,
// GOLEM invites the example, gathers the list/result pairs, and assembles the fold by
// running once two stand, answering the list call it was asked.
func TestComputeIntentLearnsListFold(t *testing.T) {
	d, _ := testDaemon(t)
	d.cfg.RuntimeEnabled = true

	r, _, _ := d.handle(wire.Request{Cmd: "runcode", Content: "the tally of the list of 1 and 2 and 3"}, nil)
	if !strings.Contains(r.Say, "tally") || d.pendingCompute == nil {
		t.Fatalf("an unknown list computation should be elicited and held: %q pending=%v", r.Say, d.pendingCompute)
	}
	say(t, d, "the tally of the list of 1 and 2 and 3 is 6")
	two := say(t, d, "the tally of the list of 2 and 4 is 6")
	if !strings.Contains(two.Say, "tally") {
		t.Errorf("two list examples should let GOLEM learn the tally: %q", two.Say)
	}
	if !strings.Contains(two.Say, "the tally of the list of 1 and 2 and 3 is 6") {
		t.Errorf("the original list call should be answered on the learning turn: %q", two.Say)
	}
	if d.pendingCompute != nil {
		t.Error("the loop should close once the list operation is learned")
	}
}

// And a computation whose result is a LIST, list in and list out, learned and answered
// the same way through the loop.
func TestComputeIntentLearnsListReturn(t *testing.T) {
	d, _ := testDaemon(t)
	d.cfg.RuntimeEnabled = true

	if _, _, _ = d.handle(wire.Request{Cmd: "runcode", Content: "the doubled of the list of 1 and 2"}, nil); d.pendingCompute == nil {
		t.Fatal("an unknown list-returning computation should be elicited and held")
	}
	say(t, d, "the doubled of the list of 1 and 2 is the list of 2 and 4")
	two := say(t, d, "the doubled of the list of 3 and 5 is the list of 6 and 10")
	if !strings.Contains(two.Say, "I learned doubled") {
		t.Errorf("two list-to-list examples should let GOLEM learn the transform: %q", two.Say)
	}
	if d.pendingCompute != nil {
		t.Error("the loop should close once the transform is learned")
	}
}
