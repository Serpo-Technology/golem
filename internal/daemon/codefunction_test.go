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

// A rule of two operations is induced when the examples constrain it: cube is no
// one-step rule, so the search composes the square with one more multiply and
// grounds it by running over every pair.
func TestInducesCubeFromExamples(t *testing.T) {
	d, _ := testDaemon(t)
	d.cfg.RuntimeEnabled = true

	resp := say(t, d, "the cube of 2 is 8 and the cube of 3 is 27 and the cube of 5 is 125")
	if !strings.Contains(resp.Say, "cube") || !strings.Contains(resp.Say, "(number * number) * number") {
		t.Fatalf("GOLEM should infer cube = (number * number) * number: %q", resp.Say)
	}
	resp = say(t, d, "what is the cube of 4")
	if !strings.Contains(resp.Say, "64") {
		t.Fatalf("the inferred cube should run on a new number: %q", resp.Say)
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

// A conditional function is taught, runs the branch its comparison selects, and
// survives a recompile, the first control flow the faculty grounds by running.
func TestLearnsAndCallsAConditional(t *testing.T) {
	d, _ := testDaemon(t)
	d.cfg.RuntimeEnabled = true

	resp := say(t, d, "the larger of a number and a count is the number if the number is above the count else the count")
	if !strings.Contains(resp.Say, "larger") || !strings.Contains(resp.Say, "if number > count") {
		t.Fatalf("the conditional should be learned and written as a branch: %q", resp.Say)
	}
	if r := say(t, d, "what is the larger of 6 and 4"); !strings.Contains(r.Say, "6") {
		t.Fatalf("larger(6, 4) should choose 6: %q", r.Say)
	}
	if r := say(t, d, "what is the larger of 2 and 9"); !strings.Contains(r.Say, "9") {
		t.Fatalf("larger(2, 9) should choose 9: %q", r.Say)
	}
	if err := d.recompileGrammar(); err != nil {
		t.Fatal(err)
	}
	if r := say(t, d, "what is the larger of 8 and 3"); !strings.Contains(r.Say, "8") {
		t.Fatalf("the reloaded conditional should still choose the larger: %q", r.Say)
	}
}

// A recursive function is taught, calls itself to a base case, runs, and survives
// a recompile. The body refers to factorial before it is in the inventory, and a
// call runs as a self-referential local function in the interpreter.
func TestLearnsAndRunsRecursion(t *testing.T) {
	d, _ := testDaemon(t)
	d.cfg.RuntimeEnabled = true

	resp := say(t, d, "the factorial of a number is 1 if the number is below 1 else the product of the number and the factorial of the difference of the number and 1")
	if !strings.Contains(resp.Say, "factorial") || !strings.Contains(resp.Say, "factorial(number - 1)") {
		t.Fatalf("the recursive definition should be learned and write a self-call: %q", resp.Say)
	}
	if r := say(t, d, "what is the factorial of 5"); !strings.Contains(r.Say, "120") {
		t.Fatalf("factorial(5) should be 120: %q", r.Say)
	}
	if r := say(t, d, "what is the factorial of 6"); !strings.Contains(r.Say, "720") {
		t.Fatalf("factorial(6) should be 720: %q", r.Say)
	}
	if err := d.recompileGrammar(); err != nil {
		t.Fatal(err)
	}
	if r := say(t, d, "what is the factorial of 4"); !strings.Contains(r.Say, "24") {
		t.Fatalf("the reloaded recursion should still run: %q", r.Say)
	}
}

// A function over a list is taught with a named-type parameter, runs over a list
// literal, and survives a recompile with its parameter type intact.
func TestLearnsAndCallsAListFunction(t *testing.T) {
	d, _ := testDaemon(t)
	d.cfg.RuntimeEnabled = true

	resp := say(t, d, "the average of a list of numbers is the quotient of the total of the numbers and the length of the numbers")
	if !strings.Contains(resp.Say, "average") || !strings.Contains(resp.Say, "numbers []int") {
		t.Fatalf("the list function should be learned and write a slice parameter: %q", resp.Say)
	}
	if r := say(t, d, "what is the average of the list of 2 and 4 and 6"); !strings.Contains(r.Say, "4") {
		t.Fatalf("average of 2, 4, 6 should be 4: %q", r.Say)
	}
	if err := d.recompileGrammar(); err != nil {
		t.Fatal(err)
	}
	if r := say(t, d, "what is the average of the list of 10 and 20 and 30 and 40"); !strings.Contains(r.Say, "25") {
		t.Fatalf("the reloaded list function should still run: %q", r.Say)
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
