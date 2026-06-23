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

// Shown examples that no fixed-step rule fits, GOLEM synthesizes a recursive
// procedure, never told it recurs: a base case and a self-call over the
// predecessor, grounded by running over every pair. It then runs on a new input.
func TestSynthesizesRecursionFromExamples(t *testing.T) {
	d, _ := testDaemon(t)
	d.cfg.RuntimeEnabled = true

	resp := say(t, d, "the fact of 1 is 1 and the fact of 2 is 2 and the fact of 3 is 6 and the fact of 4 is 24 and the fact of 5 is 120")
	if !strings.Contains(resp.Say, "fact(number - 1)") {
		t.Fatalf("GOLEM should synthesize a recursive factorial from examples: %q", resp.Say)
	}
	if r := say(t, d, "what is the fact of 6"); !strings.Contains(r.Say, "720") {
		t.Fatalf("the synthesized recursion should run on a new input: %q", r.Say)
	}
}

// A function can be shown rather than told: from input/output pairs GOLEM works
// out the rule, keeps it, and then runs it on a number the examples never named.
func TestInducesAFunctionFromExamples(t *testing.T) {
	d, _ := testDaemon(t)
	d.cfg.RuntimeEnabled = true

	resp := say(t, d, "the double of 3 is 6 and the double of 5 is 10")
	if !strings.Contains(resp.Say, "I learned double") {
		t.Fatalf("GOLEM should work out double from the pairs: %q", resp.Say)
	}
	resp = say(t, d, "what is the double of 20")
	if !strings.Contains(resp.Say, "40") {
		t.Fatalf("the synthesized function should run on a new number: %q", resp.Say)
	}
}

// A rule of two operations is searched out and grounded: cube is no one-step
// rule, so the search composes multiplies until one carries every pair, then runs
// it on a number the examples never named.
func TestInducesCubeFromExamples(t *testing.T) {
	d, _ := testDaemon(t)
	d.cfg.RuntimeEnabled = true

	resp := say(t, d, "the cube of 2 is 8 and the cube of 3 is 27 and the cube of 5 is 125")
	if !strings.Contains(resp.Say, "I learned cube") {
		t.Fatalf("GOLEM should work out cube from the pairs: %q", resp.Say)
	}
	resp = say(t, d, "what is the cube of 4")
	if !strings.Contains(resp.Say, "64") {
		t.Fatalf("the synthesized cube should run on a new number: %q", resp.Say)
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

// A two-way conditional is discovered from examples no single expression fits: a
// predicate over the input splits the examples, each side a searched expression,
// grounded by running. abs branches on the sign into the number and its negation;
// relu's else branch is a constant. The examples span a wide range, so an
// arithmetic rule that fits a narrow set by integer-division truncation is ruled
// out and the conditional is what remains. The learned rule runs on an input the
// examples never named.
func TestInducesConditionalFromExamples(t *testing.T) {
	d, _ := testDaemon(t)
	d.cfg.RuntimeEnabled = true

	resp := say(t, d, "the abs of -3 is 3 and the abs of 1 is 1 and the abs of -5 is 5 and the abs of 8 is 8")
	if !strings.Contains(resp.Say, "I learned abs") || !strings.Contains(resp.Say, "if number > 0") {
		t.Fatalf("abs should be discovered as a conditional from examples: %q", resp.Say)
	}
	if r := say(t, d, "what is the abs of -7"); !strings.Contains(r.Say, "is 7") {
		t.Fatalf("the discovered abs of -7 should be 7: %q", r.Say)
	}
	resp = say(t, d, "the relu of -3 is 0 and the relu of 1 is 1 and the relu of -2 is 0 and the relu of 8 is 8")
	if !strings.Contains(resp.Say, "I learned relu") || !strings.Contains(resp.Say, "if number > 0") {
		t.Fatalf("relu should be discovered as a conditional from examples: %q", resp.Say)
	}
	if r := say(t, d, "what is the relu of -9"); !strings.Contains(r.Say, "is 0") {
		t.Fatalf("the discovered relu of -9 should be 0: %q", r.Say)
	}
}

// A function over a list is synthesized as a loop, the structure assembled from
// the examples rather than templated: the search lands on a range over the list
// with an accumulator, written as real Go and grounded by running. The loop body's
// update emerges (a sum, then a sum of squares), and the learned loop runs on a
// list the examples never named, surviving a recompile.
func TestSynthesizesFoldFromListExamples(t *testing.T) {
	d, _ := testDaemon(t)
	d.cfg.RuntimeEnabled = true

	resp := say(t, d, "the tally of the list of 1 and 2 and 3 is 6 and the tally of the list of 2 and 4 is 6")
	if !strings.Contains(resp.Say, "I learned tally") || !strings.Contains(resp.Say, "for _, x := range numbers") {
		t.Fatalf("a loop should be synthesized from the list examples: %q", resp.Say)
	}
	if r := say(t, d, "what is the tally of the list of 4 and 5 and 6"); !strings.Contains(r.Say, "15") {
		t.Fatalf("the synthesized loop should run on a new list: %q", r.Say)
	}
	say(t, d, "the energy of the list of 1 and 2 and 3 is 14 and the energy of the list of 2 and 3 is 13")
	if r := say(t, d, "what is the energy of the list of 2 and 3 and 4"); !strings.Contains(r.Say, "29") {
		t.Fatalf("a sum of squares should be synthesized and run: %q", r.Say)
	}
}

// A function whose result is a list is synthesized as a map or a filter loop, the
// structure emerging from list-in, list-out examples and grounded by running.
func TestSynthesizesListReturnFromExamples(t *testing.T) {
	d, _ := testDaemon(t)
	d.cfg.RuntimeEnabled = true

	resp := say(t, d, "the big of the list of 1 and 5 and 2 and 9 is the list of 5 and 9 and the big of the list of 4 and 1 and 7 is the list of 4 and 7")
	if !strings.Contains(resp.Say, "I learned big") || !strings.Contains(resp.Say, "if x") {
		t.Fatalf("a filter loop should be synthesized: %q", resp.Say)
	}
	if r := say(t, d, "what is the big of the list of 3 and 8 and 1"); !strings.Contains(r.Say, "[3 8]") {
		t.Fatalf("the synthesized filter should run on a new list: %q", r.Say)
	}
	resp = say(t, d, "the doubled of the list of 1 and 2 and 3 is the list of 2 and 4 and 6 and the doubled of the list of 5 and 0 is the list of 10 and 0")
	if !strings.Contains(resp.Say, "I learned doubled") {
		t.Fatalf("a map loop should be synthesized: %q", resp.Say)
	}
	if r := say(t, d, "what is the doubled of the list of 3 and 4"); !strings.Contains(r.Say, "[6 8]") {
		t.Fatalf("the synthesized map should run on a new list: %q", r.Say)
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

// A turn shaped like a definition whose body computes over its parameters, but
// which the definition path declines on a soundness check, is admitted as an honest
// gap rather than grounded as a fabricated world fact. "the sum of a number is the
// product of the number and 2" shadows the add primitive, so it cannot be learned;
// the daemon says so by name and does not ground it.
func TestFailedDefinitionAdmitsGap(t *testing.T) {
	d, _ := testDaemon(t)
	d.cfg.RuntimeEnabled = true

	resp := say(t, d, "the sum of a number is the product of the number and 2")
	if resp.Say != "I could not work out the sum." {
		t.Fatalf("a failed definition should admit a gap by name: %q", resp.Say)
	}
	if resp.Observe != nil {
		t.Fatalf("a failed definition must not ground as a world fact")
	}
}

// A generic fact that shares the genitive shape but whose predicate is not a
// computation over the parameter still grounds as a fact; it is not mistaken for a
// failed definition and admitted as a gap.
func TestGenericFactStillGrounds(t *testing.T) {
	d, _ := testDaemon(t)
	d.cfg.RuntimeEnabled = true

	resp := say(t, d, "the friend of a man is a dog")
	if strings.HasPrefix(resp.Say, "I could not work out the") {
		t.Fatalf("a generic fact must not be admitted as a failed definition: %q", resp.Say)
	}
	if resp.Observe == nil {
		t.Fatalf("a generic fact should ground as an observation")
	}
}

// A function may take a parameter whose word names a learned operation. Once
// "double" is a known operation, "the chooser of a double and a low ..." still
// learns: the body reads "the double" as the parameter, so the definition is not
// refused and silently grounded as a statement (the bug this guards). Without a
// rich enough operation inventory the collision never arises, so it is the loaded
// brain that needs the guarantee.
func TestDefinitionParameterShadowsLearnedOperation(t *testing.T) {
	d, _ := testDaemon(t)
	d.cfg.RuntimeEnabled = true

	if r := say(t, d, "the double of a number is the product of the number and 2"); !strings.Contains(r.Say, "double") {
		t.Fatalf("double should be learned first: %q", r.Say)
	}
	resp := say(t, d, "the chooser of a double and a low is the double if the double is above the low else the low")
	if !strings.Contains(resp.Say, "I learned chooser") {
		t.Fatalf("a parameter named like a learned operation should still learn, not ground: %q", resp.Say)
	}
	if r := say(t, d, "what is the chooser of 8 and 3"); !strings.Contains(r.Say, "8") {
		t.Fatalf("chooser(8, 3) should choose 8: %q", r.Say)
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
	teachReductions(t, d, "total", "length")

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

// A transform over a list maps a function across its elements, returns a list,
// composes under a reduction, and survives a recompile. doubled maps double; the
// total and largest of a doubled list reduce the mapped result.
func TestLearnsAndCallsAMap(t *testing.T) {
	d, _ := testDaemon(t)
	d.cfg.RuntimeEnabled = true
	teachReductions(t, d, "total", "largest")

	say(t, d, "the double of a number is the product of the number and 2")
	resp := say(t, d, "the doubled of a list of numbers is the double of each number")
	if !strings.Contains(resp.Say, "doubled") || !strings.Contains(resp.Say, "[]int") {
		t.Fatalf("the map should be learned returning a slice: %q", resp.Say)
	}
	if r := say(t, d, "what is the total of the doubled of the list of 1 and 2 and 3"); !strings.Contains(r.Say, "12") {
		t.Fatalf("total of the doubled list should be 12: %q", r.Say)
	}
	if err := d.recompileGrammar(); err != nil {
		t.Fatal(err)
	}
	if r := say(t, d, "what is the largest of the doubled of the list of 5 and 2 and 8"); !strings.Contains(r.Say, "16") {
		t.Fatalf("the reloaded map should still run under a reduction: %q", r.Say)
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
