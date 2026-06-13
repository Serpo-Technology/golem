package grammar

import "testing"

// The embedded bootstrap loads into parameters and splits cleanly: the
// curriculum carries every grammar node, the constructions carry the sentence
// patterns, nothing lands in both, and nothing is dropped.
func TestSeedBootstrapSplits(t *testing.T) {
	cur, cxn := Curriculum(), Constructions()
	if len(cur) < 100 {
		t.Fatalf("the curriculum should carry the grammar seed, got %d", len(cur))
	}
	if len(cxn) == 0 {
		t.Fatal("the constructions should carry the sentence patterns")
	}
	for _, p := range cur {
		if isConstruction(p.Key) {
			t.Fatalf("a construction leaked into the curriculum: %s", p.Key)
		}
	}
	for _, p := range cxn {
		if !isConstruction(p.Key) {
			t.Fatalf("a non-construction leaked into the constructions: %s", p.Key)
		}
	}
	if got := len(cur) + len(cxn); got != len(parseSeed()) {
		t.Fatalf("the split is not exhaustive: %d split vs %d parsed", got, len(parseSeed()))
	}
}

// A form taught only by the seed still classifies through the compiled engine,
// so the data round-trips into the same behavior the tables used to give.
func TestSeedRoundTripsThroughEngine(t *testing.T) {
	p := Builtin()
	if p.category("en", "what") != catWh {
		t.Fatal("the seed should classify 'what' as an interrogative")
	}
	if _, ok := p.sortal("en", "command"); !ok {
		t.Fatal("the seed should carry the 'command' sortal")
	}
	if _, ok := p.hap("en", "follows"); !ok {
		t.Fatal("the seed should carry 'follows' as an event verb")
	}
}
