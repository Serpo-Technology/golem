package daemon

import (
	"reflect"
	"testing"
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
		"6 plus 7",                    // an operator-word expression
		"6 + 7",                       // raw Go
		"the list of 1 and 2 and 3",   // a list literal, deferred to the engine
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
