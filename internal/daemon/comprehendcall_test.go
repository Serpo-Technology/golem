package daemon

import (
	"reflect"
	"strings"
	"testing"

	"golem/wire"
)

// comprehendCall is the one point the daemon comprehends a computation call: the
// grammar reads the operation-application surface and the code engine reads the
// rest. For an operation call the grammar path and the engine yield the same
// computation, so routing the call through the grammar changes which parser reads
// it, not what it means. For the surfaces the grammar does not claim (an
// operator-word expression, raw Go, a list literal) the call falls back to the
// engine unchanged. The ParseApplication assertions pin which parser ran, so a
// silent fall-through could not pass this as a false green.
func TestComprehendCallMatchesEngine(t *testing.T) {
	d, _ := testDaemon(t)

	opCalls := []string{
		"the sum of 6 and 7",
		"the product of 3 and 4",
		"the sum of the product of 3 and 4 and 5",
		"the list of 1 and 2 and 3", // a list literal is a call the grammar reads too
	}
	for _, s := range opCalls {
		if _, ok := d.gram.ParseApplication("en", s, d.opHead); !ok {
			t.Fatalf("the grammar should parse the operation call %q", s)
		}
		got, gok := d.comprehendCall("en", s)
		want, wok := d.code.Comprehend(s)
		if gok != wok || !reflect.DeepEqual(got, want) {
			t.Fatalf("comprehendCall(%q) = %+v (ok=%v), engine = %+v (ok=%v)", s, got, gok, want, wok)
		}
	}

	fallbacks := []string{
		"6 plus 7", // an operator-word expression
		"6 + 7",    // raw Go
	}
	for _, s := range fallbacks {
		if _, ok := d.gram.ParseApplication("en", s, d.opHead); ok {
			t.Fatalf("the grammar should not claim the non-call surface %q", s)
		}
		got, gok := d.comprehendCall("en", s)
		want, wok := d.code.Comprehend(s)
		if gok != wok || !reflect.DeepEqual(got, want) {
			t.Fatalf("comprehendCall(%q) = %+v (ok=%v), engine = %+v (ok=%v)", s, got, gok, want, wok)
		}
	}
}

// The bare imperative "double 4" / "add 6 and 7" names the operation with its
// leading word and applies it to the operands that follow, the call without the
// genitive scaffolding. A primitive answers by its verb, a learned function by its
// name, and a phrase that merely opens with an operation word does not compute.
// A function stated as the inverse of a known operation is discovered by synthesis,
// not shown: taught increment, GOLEM learns the decrement from "the decrement is the
// inverse of the increment" and computes it; an inverse of an operation it does not
// know is not learned.
func TestComprehendInverseDiscoversAFunction(t *testing.T) {
	d, _ := testDaemon(t)
	d.cfg.RuntimeEnabled = true
	say(t, d, "the increment of a number is the sum of the number and 1")

	r := say(t, d, "the decrement is the inverse of the increment")
	if !strings.Contains(r.Say, "decrement") {
		t.Fatalf("the inverse goal should learn the decrement: %q", r.Say)
	}
	out, _, _ := d.handle(wire.Request{Cmd: "runcode", Content: "the decrement of 5"}, nil)
	if !strings.Contains(out.Say, "4") {
		t.Errorf("the discovered decrement of 5 should be 4: %q", out.Say)
	}

	r2 := say(t, d, "the unthing is the inverse of the thing")
	if strings.Contains(r2.Say, "I learned unthing") {
		t.Errorf("an inverse of an operation GOLEM does not know should not be learned: %q", r2.Say)
	}
}

// A list function stated by an extremal property is discovered by synthesis, not
// shown: GOLEM learns the peak from "the peak is the greatest" and computes the
// list's maximum whether it is interior or at the end (so the fold is a running
// maximum, not "return the first"); the smallest yields the minimum. The
// all-negative case (the fold starting from the first element, not zero) is
// verified at the synthesis level in the code package.
func TestComprehendExtremalDiscoversAFunction(t *testing.T) {
	d, _ := testDaemon(t)
	d.cfg.RuntimeEnabled = true

	r := say(t, d, "the peak is the greatest")
	if !strings.Contains(r.Say, "peak") {
		t.Fatalf("the extremal goal should learn the peak: %q", r.Say)
	}
	out, _, _ := d.handle(wire.Request{Cmd: "runcode", Content: "the peak of the list of 3 and 9 and 2"}, nil)
	if !strings.Contains(out.Say, "9") {
		t.Errorf("the discovered peak should be the interior max 9: %q", out.Say)
	}
	last, _, _ := d.handle(wire.Request{Cmd: "runcode", Content: "the peak of the list of 2 and 5 and 8"}, nil)
	if !strings.Contains(last.Say, "8") {
		t.Errorf("the discovered peak should be the trailing max 8: %q", last.Say)
	}

	r2 := say(t, d, "the trough is the smallest")
	if !strings.Contains(r2.Say, "trough") {
		t.Fatalf("the smallest should learn the trough: %q", r2.Say)
	}
	mn, _, _ := d.handle(wire.Request{Cmd: "runcode", Content: "the trough of the list of 4 and 1 and 7"}, nil)
	if !strings.Contains(mn.Say, "1") {
		t.Errorf("the discovered trough should be the min 1: %q", mn.Say)
	}
}

// The discovered extremal and a largest reduction taught by example coexist: the
// extremal marker "greatest" and a taught "largest" live in separate concept
// namespaces, so each computes without interfering with the other.
func TestExtremalAndTaughtLargestCoexist(t *testing.T) {
	d, _ := testDaemon(t)
	d.cfg.RuntimeEnabled = true
	teachReductions(t, d, "largest")
	say(t, d, "the peak is the greatest")

	lg, _, _ := d.handle(wire.Request{Cmd: "runcode", Content: "the largest of the list of 4 and 9 and 1"}, nil)
	if !strings.Contains(lg.Say, "9") {
		t.Errorf("the taught largest should still compute the max: %q", lg.Say)
	}
	pk, _, _ := d.handle(wire.Request{Cmd: "runcode", Content: "the peak of the list of 4 and 9 and 1"}, nil)
	if !strings.Contains(pk.Say, "9") {
		t.Errorf("the discovered peak should compute the max: %q", pk.Say)
	}
}

// Re-teaching a function GOLEM already learned re-finds its rule rather than the
// trivial call to itself: it sits in the synthesis search inventory now, but it is
// left out of its own search, so the re-synthesis lands on the rule again.
func TestReteachRefindsRule(t *testing.T) {
	d, _ := testDaemon(t)
	d.cfg.RuntimeEnabled = true
	say(t, d, "the triple of 2 is 6 and the triple of 5 is 15")
	first, _, _ := d.handle(wire.Request{Cmd: "runcode", Content: "the triple of 4"}, nil)
	if !strings.Contains(first.Say, "12") {
		t.Fatalf("triple of 4 should be 12: %q", first.Say)
	}
	reteach := say(t, d, "the triple of 2 is 6 and the triple of 5 is 15")
	if !strings.Contains(reteach.Say, "triple") {
		t.Fatalf("re-teaching should re-learn the triple, not decline: %q", reteach.Say)
	}
	again, _, _ := d.handle(wire.Request{Cmd: "runcode", Content: "the triple of 4"}, nil)
	if !strings.Contains(again.Say, "12") {
		t.Fatalf("triple should still be 12 after a re-teach, not self-defined: %q", again.Say)
	}
}

func TestBareImperativeComputes(t *testing.T) {
	d, _ := testDaemon(t)
	d.cfg.RuntimeEnabled = true
	say(t, d, "the double of a number is the product of the number and 2")

	for _, c := range []struct{ in, want string }{
		{"sum 6 and 7", "13"},      // a primitive by its teaching word
		{"add 6 and 7", "13"},      // the same primitive by its verb
		{"multiply 3 and 4", "12"}, // a verb naming arity two
		{"double 9", "18"},         // a learned function by its name
	} {
		r, _, _ := d.handle(wire.Request{Cmd: "runcode", Content: c.in}, nil)
		if !strings.Contains(r.Say, c.want) {
			t.Errorf("compute %q should run to %s: %q", c.in, c.want, r.Say)
		}
	}
}
