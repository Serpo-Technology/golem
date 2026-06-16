package daemon

import "testing"

// The held store keeps a proposition that is not a graph fact, reads it back at
// the confidence it was held at, refreshes rather than duplicates a re-derived
// belief, and forgets a thought once it has cooled past the floor.
func TestHeldStoreHoldsAndDecays(t *testing.T) {
	h := newHeldStore()
	h.add(heldProp{rel: "chase", agent: 1, patient: 2, status: heldInferred, conf: 0.6})

	if conf, ok := h.holds("chase", 1, 2, heldInferred); !ok || conf != 0.6 {
		t.Fatalf("a held proposition should read back at its confidence: got (%v, %v)", conf, ok)
	}
	// A different mode of the same proposition is a different hold.
	if _, ok := h.holds("chase", 1, 2, heldHypothetical); ok {
		t.Fatal("a hypothetical hold should not satisfy an inferred read")
	}
	// Re-deriving the belief at higher confidence refreshes the one entry.
	h.add(heldProp{rel: "chase", agent: 1, patient: 2, status: heldInferred, conf: 0.8})
	if h.len() != 1 {
		t.Fatalf("a re-derived belief should refresh, not duplicate: held %d", h.len())
	}
	if conf, _ := h.holds("chase", 1, 2, heldInferred); conf != 0.8 {
		t.Fatalf("a refresh should keep the higher confidence: %v", conf)
	}

	// Decay cools activation but never touches confidence; enough ticks forget it.
	for i := 0; i < 500; i++ {
		h.decay()
	}
	if _, ok := h.holds("chase", 1, 2, heldInferred); ok {
		t.Fatal("a held proposition should leave mind once it has cooled past the floor")
	}
}

// byAgent reads what is held about a concept as agent in a given mode, freshest
// first, and clearStatus ends a supposition without disturbing held beliefs.
func TestHeldStoreByAgentAndClearStatus(t *testing.T) {
	h := newHeldStore()
	h.add(heldProp{rel: "chase", agent: 1, patient: 2, status: heldInferred, conf: 0.6})
	h.add(heldProp{rel: "chase", agent: 1, patient: 3, status: heldHypothetical, conf: 0.9})
	h.add(heldProp{rel: "chase", agent: 1, patient: 4, status: heldHypothetical, conf: 0.9})

	if got := h.byAgent(1, heldHypothetical); len(got) != 2 {
		t.Fatalf("byAgent should return both suppositions about the agent: %d", len(got))
	}
	if got := h.byAgent(1, heldInferred); len(got) != 1 || got[0].patient != 2 {
		t.Fatalf("byAgent should filter to the mode asked: %+v", got)
	}

	// Ending the supposition drops its hypotheticals; the inferred belief stands.
	h.clearStatus(heldHypothetical)
	if got := h.byAgent(1, heldHypothetical); len(got) != 0 {
		t.Fatalf("clearStatus should drop every hypothetical: %d left", len(got))
	}
	if _, ok := h.holds("chase", 1, 2, heldInferred); !ok {
		t.Fatal("clearStatus on hypotheticals should leave inferred beliefs untouched")
	}
}

// Past the budget the coldest held proposition is evicted, so working memory
// holds a bounded set rather than growing without end.
func TestHeldStoreEvictsPastBudget(t *testing.T) {
	h := newHeldStore()
	for i := 0; i < heldBudget+5; i++ {
		h.add(heldProp{rel: "chase", agent: uint64(i + 1), patient: 999, status: heldInferred, conf: 0.5})
	}
	if h.len() != heldBudget {
		t.Fatalf("held store should cap at the budget: %d", h.len())
	}
}
