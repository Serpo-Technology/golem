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

// Re-teaching a function GOLEM already learned does not redefine it as a call to
// itself: it sits in the synthesis search inventory now, so a non-recursive search
// would find the trivial self-fit, but that is refused and the function is left
// computing as it did.
func TestReteachDoesNotSelfDefine(t *testing.T) {
	d, _ := testDaemon(t)
	d.cfg.RuntimeEnabled = true
	say(t, d, "the triple of 2 is 6 and the triple of 5 is 15")
	first, _, _ := d.handle(wire.Request{Cmd: "runcode", Content: "the triple of 4"}, nil)
	if !strings.Contains(first.Say, "12") {
		t.Fatalf("triple of 4 should be 12: %q", first.Say)
	}
	say(t, d, "the triple of 2 is 6 and the triple of 5 is 15") // re-teach
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
