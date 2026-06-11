package memory

import (
	"fmt"
	"testing"

	"golem/internal/limbic"
)

func hotFragment(id uint64, content string, na float64) *Fragment {
	return &Fragment{ID: id, Channel: "testlog", Content: content,
		Template: content, Limbic: limbic.State{Noradrenaline: na}, Weight: 1.0}
}

// The working set answers substring queries ranked most stressful first,
// the same order the archive uses.
func TestHotTierSearchRanks(t *testing.T) {
	h := NewHotTier()
	h.Put(hotFragment(1, "disk almost full", 0.3))
	h.Put(hotFragment(2, "Disk full on /dev/disk1s2", 0.9))
	h.Put(hotFragment(3, "ping fine", 0.1))

	out := h.Search("disk", 5)
	if len(out) != 2 {
		t.Fatalf("expected the two disk moments, got %d", len(out))
	}
	if out[0].ID != 2 || out[1].ID != 1 {
		t.Fatalf("search should rank by noradrenaline: %+v", out)
	}
	if got := h.Search("disk", 1); len(got) != 1 || got[0].ID != 2 {
		t.Fatalf("the limit keeps the most stressful, got %+v", got)
	}
	if got := h.Search("", 5); got != nil {
		t.Fatalf("an empty query matches nothing, got %+v", got)
	}
}

// Past the budget the least recently touched entry is evicted; touching
// an entry keeps it alive.
func TestHotTierEviction(t *testing.T) {
	h := NewHotTier()
	for i := 1; i <= hotBudget; i++ {
		h.Put(hotFragment(uint64(i), fmt.Sprintf("moment %d", i), 0.1))
	}
	h.Get(1) // the oldest entry is touched and must survive

	h.Put(hotFragment(hotBudget+1, "one over budget", 0.1))
	if h.Len() != hotBudget {
		t.Fatalf("the tier should hold its budget, got %d", h.Len())
	}
	if _, ok := h.Get(1); !ok {
		t.Fatal("a touched entry must survive eviction")
	}
	if _, ok := h.Get(2); ok {
		t.Fatal("the least recently touched entry should have been evicted")
	}
}
