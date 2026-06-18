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

// A function can be shown rather than told: from input/output pairs GOLEM works
// out the rule, keeps it, and then runs it on a number the examples never named.
func TestInducesAFunctionFromExamples(t *testing.T) {
	d, _ := testDaemon(t)
	d.cfg.RuntimeEnabled = true

	resp := say(t, d, "the double of 3 is 6 and the double of 5 is 10")
	if !strings.Contains(resp.Say, "double") || !strings.Contains(resp.Say, "number * 2") {
		t.Fatalf("GOLEM should infer double = number * 2 from the pairs: %q", resp.Say)
	}
	resp = say(t, d, "what is the double of 20")
	if !strings.Contains(resp.Say, "40") {
		t.Fatalf("the inferred function should run on a new number: %q", resp.Say)
	}
}

// The search reaches the parameter-against-itself shape, so squaring is inferred
// from pairs no constant rule fits.
func TestInducesSquareFromExamples(t *testing.T) {
	d, _ := testDaemon(t)
	d.cfg.RuntimeEnabled = true

	resp := say(t, d, "the square of 3 is 9 and the square of 4 is 16")
	if !strings.Contains(resp.Say, "number * number") {
		t.Fatalf("GOLEM should infer square = number * number: %q", resp.Say)
	}
}

// A binary function taught in language is learned, called with two operands, and
// kept across a recompile with both parameters intact.
func TestLearnsAndCallsABinaryFunction(t *testing.T) {
	d, _ := testDaemon(t)
	d.cfg.RuntimeEnabled = true

	resp := say(t, d, "the blend of a number and a factor is the product of the number and the factor")
	if !strings.Contains(resp.Say, "blend") || !strings.Contains(resp.Say, "number * factor") {
		t.Fatalf("the binary definition should be learned and written: %q", resp.Say)
	}
	resp = say(t, d, "what is the blend of 6 and 7")
	if !strings.Contains(resp.Say, "42") {
		t.Fatalf("the binary function should run to 42: %q", resp.Say)
	}
	if err := d.recompileGrammar(); err != nil {
		t.Fatal(err)
	}
	resp = say(t, d, "what is the blend of 8 and 9")
	if !strings.Contains(resp.Say, "72") {
		t.Fatalf("the reloaded binary function should run to 72: %q", resp.Say)
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
