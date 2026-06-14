package daemon

import (
	"strings"
	"testing"
)

// A function taught in language is learned and then called: the definition mints
// the operation and writes its Go, and a question runs it, generalizing to a
// number the definition never mentioned.
func TestLearnsAndCallsAFunction(t *testing.T) {
	d, _ := testDaemon(t)
	d.cfg.RuntimeEnabled = true

	resp := say(t, d, "the double of a number is the product of the number and 2")
	if !strings.Contains(resp.Say, "double") || !strings.Contains(resp.Say, "number * 2") {
		t.Fatalf("the definition should be learned and the function written: %q", resp.Say)
	}
	resp = say(t, d, "what is the double of 6")
	if !strings.Contains(resp.Say, "12") {
		t.Fatalf("the function should run to 12: %q", resp.Say)
	}
	resp = say(t, d, "what is the double of 100")
	if !strings.Contains(resp.Say, "200") {
		t.Fatalf("the function should generalize to 200: %q", resp.Say)
	}
}

// A learned function survives a recompile, the boot-and-nightly path that
// rebuilds the engine from memory, so it is kept the way a weight is and not held
// only in the running engine.
func TestLearnedFunctionPersistsAcrossRecompile(t *testing.T) {
	d, _ := testDaemon(t)
	d.cfg.RuntimeEnabled = true

	say(t, d, "the triple of a number is the product of the number and 3")
	if err := d.recompileGrammar(); err != nil {
		t.Fatal(err)
	}
	resp := say(t, d, "what is the triple of 4")
	if !strings.Contains(resp.Say, "12") {
		t.Fatalf("the function should still run after a recompile: %q", resp.Say)
	}
}
