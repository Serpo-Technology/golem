package daemon

import "testing"

// A computation taught three ways lands on one grounded meaning: the words, the
// code, and a set of examples all converge on the same operation, the examples
// resolved by running candidates until one matches.
func TestTeachByExampleConverges(t *testing.T) {
	d, _ := testDaemon(t)
	d.cfg.RuntimeEnabled = true

	words, ok := d.code.Comprehend("the sum of 6 and 7")
	if !ok {
		t.Fatal("the words should comprehend")
	}
	source, ok := d.code.Comprehend("6 + 7")
	if !ok {
		t.Fatal("the code should comprehend")
	}
	example, ok := d.induceByExample([]example{{"6", "7", "13"}, {"2", "3", "5"}})
	if !ok {
		t.Fatal("the examples should induce an operation")
	}
	if words.Op != "add" || source.Op != "add" || example.Op != "add" {
		t.Fatalf("words, code, and example should converge on add: %q %q %q",
			words.Op, source.Op, example.Op)
	}
}

// Induction by example distinguishes operations by their grounded effect: the
// same inputs with a product as the result induce multiply, the running of each
// candidate the thing that decides.
func TestInduceByExampleDistinguishesOps(t *testing.T) {
	d, _ := testDaemon(t)
	d.cfg.RuntimeEnabled = true
	c, ok := d.induceByExample([]example{{"6", "7", "42"}, {"2", "3", "6"}})
	if !ok || c.Op != "multiply" {
		t.Fatalf("examples making a product should induce multiply: %+v ok %v", c, ok)
	}
}

// Examples no operation fits induce nothing, so induction fails cleanly rather
// than forcing a wrong meaning.
func TestInduceByExampleNoFit(t *testing.T) {
	d, _ := testDaemon(t)
	d.cfg.RuntimeEnabled = true
	if _, ok := d.induceByExample([]example{{"6", "7", "100"}}); ok {
		t.Fatal("examples that no seeded operation fits should not induce")
	}
}

// Induction runs candidates, so it needs the runtime open like any execution.
func TestInduceByExampleNeedsRuntime(t *testing.T) {
	d, _ := testDaemon(t)
	if _, ok := d.induceByExample([]example{{"6", "7", "13"}}); ok {
		t.Fatal("induction should be closed while the runtime is")
	}
}
