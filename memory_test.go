package main

import "testing"

func held(h *Hippocampus, id uint64) bool {
	for _, f := range h.Dump() {
		if f.ID == id {
			return true
		}
	}
	return false
}

// Every fragment must eventually fade below the floor and be pruned, no
// matter how surprising its birth. Immortality belongs to recalled
// memories only.
func TestPruningFires(t *testing.T) {
	h := NewHippocampus()
	h.Encode(&Fragment{Content: "x", Surprise: 6})
	for night := 0; night < 5000 && h.Count() > 0; night++ {
		h.Sleep(0.10, 0.10)
	}
	if h.Count() != 0 {
		t.Fatal("high-surprise fragment was never pruned")
	}
}

// Sharper memories must outlive duller ones under the same nightly decay.
func TestSurpriseOrdersLifetime(t *testing.T) {
	h := NewHippocampus()
	dull := h.Encode(&Fragment{Content: "dull", Surprise: 2})
	sharp := h.Encode(&Fragment{Content: "sharp", Surprise: 6})

	var dullGone, sharpGone int
	for night := 1; night <= 5000 && h.Count() > 0; night++ {
		h.Sleep(0.10, 0.10)
		if dullGone == 0 && !held(h, dull) {
			dullGone = night
		}
		if sharpGone == 0 && !held(h, sharp) {
			sharpGone = night
		}
	}
	if dullGone == 0 || sharpGone == 0 {
		t.Fatalf("both fragments should eventually prune: dull=%d sharp=%d", dullGone, sharpGone)
	}
	if dullGone >= sharpGone {
		t.Fatalf("dull memory should die first: dull=%d sharp=%d", dullGone, sharpGone)
	}
}

// Recalling a memory restores its weight, resetting the forgetting clock.
func TestRecallRefreshesWeight(t *testing.T) {
	h := NewHippocampus()
	h.Encode(&Fragment{Content: "x", Surprise: 4, Limbic: LimbicState{Noradrenaline: 0.9}})
	for range 40 {
		h.Sleep(0.10, 0.10)
	}
	f := h.RecallByStress()
	if f == nil {
		t.Fatal("fragment should still be held after 40 nights")
	}
	if f.Weight != 1.0 {
		t.Fatalf("recall should refresh weight to 1.0, got %.2f", f.Weight)
	}
}
