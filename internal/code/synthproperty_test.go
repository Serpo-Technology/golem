package code

import "testing"

func magnitudeProperty() Property {
	sq := func(v Computation) Computation {
		return Computation{Op: "multiply", Args: []Computation{v, v}}
	}
	return Property{
		Equational: []Relation{{Left: sq(Computation{Var: ResultVar}), Cmp: "==", Right: sq(Computation{Var: "number"})}},
		Ordering:   &Relation{Left: Computation{Var: ResultVar}, Cmp: "<", Right: Computation{Lit: "0"}},
	}
}

// A function stated by a property is discovered, not shown: the magnitude property (the
// output squares to the input squared and is not below zero) lands on abs, the number when
// it is not negative and its negation when it is. The grounding runs against a stub here,
// the interpreted prefilter the filter that picks the conditional; the real runtime grounds
// it in the daemon.
func TestSynthesizeFromPropertyDiscoversAbs(t *testing.T) {
	e := testEngine()
	pass := func(string) (string, bool) { return "true", true }
	body, ok := e.SynthesizeFromProperty("number", magnitudeProperty(), pass)
	if !ok {
		t.Fatal("the magnitude property should discover abs")
	}
	for _, tc := range []struct{ n, want int }{{-3, 3}, {5, 5}, {-7, 7}, {0, 0}, {9, 9}, {-1, 1}} {
		got, ok := e.evalProperty(body, map[string]int{"number": tc.n})
		if !ok || got != tc.want {
			src, _ := e.ToCode(body)
			t.Fatalf("abs(%d) should be %d, got %d (body %q)", tc.n, tc.want, got, src)
		}
	}
}

// The identity and the negation each satisfy the squared equation but stray below zero on
// one side, so neither meets the whole property; only the conditional that switches on the
// sign does. This is what makes the property pick abs rather than a single expression.
func TestSynthesizeFromPropertyRejectsImpostors(t *testing.T) {
	e := testEngine()
	prop := magnitudeProperty()
	identity := Computation{Var: "number"}
	negation := Computation{Op: "subtract", Args: []Computation{{Lit: "0"}, {Var: "number"}}}
	if e.propertyHolds(identity, "number", prop, propertyProbes()) {
		t.Fatal("the identity strays below zero on negatives and must not meet the magnitude property")
	}
	if e.propertyHolds(negation, "number", prop, propertyProbes()) {
		t.Fatal("the negation strays below zero on positives and must not meet the magnitude property")
	}
}
