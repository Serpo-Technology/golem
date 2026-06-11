package main

import (
	"fmt"
	"testing"
	"time"
)

func hotFragment(id uint64, content string, na float64) *Fragment {
	return &Fragment{ID: id, Channel: "testlog", Content: content,
		Template: Normalize(content), Limbic: LimbicState{Noradrenaline: na}, Weight: 1.0}
}

// The working set answers substring queries ranked most stressful first,
// the same order the archive uses.
func TestHotTierSearchRanks(t *testing.T) {
	h := newHotTier()
	h.put(hotFragment(1, "disk almost full", 0.3))
	h.put(hotFragment(2, "Disk full on /dev/disk1s2", 0.9))
	h.put(hotFragment(3, "ping fine", 0.1))

	out := h.search("disk", 5)
	if len(out) != 2 {
		t.Fatalf("expected the two disk moments, got %d", len(out))
	}
	if out[0].ID != 2 || out[1].ID != 1 {
		t.Fatalf("search should rank by noradrenaline: %+v", out)
	}
	if got := h.search("disk", 1); len(got) != 1 || got[0].ID != 2 {
		t.Fatalf("the limit keeps the most stressful, got %+v", got)
	}
	if got := h.search("", 5); got != nil {
		t.Fatalf("an empty query matches nothing, got %+v", got)
	}
}

// Past the budget the least recently touched entry is evicted; touching
// an entry keeps it alive.
func TestHotTierEviction(t *testing.T) {
	h := newHotTier()
	for i := 1; i <= hotBudget; i++ {
		h.put(hotFragment(uint64(i), fmt.Sprintf("moment %d", i), 0.1))
	}
	h.get(1) // the oldest entry is touched and must survive

	h.put(hotFragment(hotBudget+1, "one over budget", 0.1))
	if h.len() != hotBudget {
		t.Fatalf("the tier should hold its budget, got %d", h.len())
	}
	if _, ok := h.get(1); !ok {
		t.Fatal("a touched entry must survive eviction")
	}
	if _, ok := h.get(2); ok {
		t.Fatal("the least recently touched entry should have been evicted")
	}
}

// A recall that has to reach the archive promotes its findings into the
// working set.
func TestRecallPromotesToHot(t *testing.T) {
	d, s := testDaemon(t)
	f := &Fragment{Channel: "syslog", Content: "Segfault in worker 3",
		Template: Normalize("Segfault in worker 3"), Surprise: 5,
		Limbic: LimbicState{Noradrenaline: 0.8}, Time: time.Now()}
	if _, err := s.Encode(f); err != nil { // straight into the archive, never hot
		t.Fatal(err)
	}
	if _, ok := d.hot.get(f.ID); ok {
		t.Fatal("the fragment should start cold")
	}

	resp, _, _ := d.handle(Request{Cmd: "recall", Query: "segfault"}, nil)
	if !resp.OK || len(resp.Fragments) == 0 {
		t.Fatalf("recall should answer from the archive: %+v", resp)
	}
	if _, ok := d.hot.get(f.ID); !ok {
		t.Fatal("an archive hit should promote into the working set")
	}
}

// The working set answers first: a hot match outranks a colder archive
// match in the reply order, and both arrive.
func TestRecallPrefersHot(t *testing.T) {
	d, s := testDaemon(t)
	cold := &Fragment{Channel: "syslog", Content: "panic stack overflow deep",
		Template: Normalize("panic stack overflow deep"), Surprise: 6,
		Limbic: LimbicState{Noradrenaline: 0.95}, Time: time.Now()}
	if _, err := s.Encode(cold); err != nil {
		t.Fatal(err)
	}
	hot := &Fragment{Channel: "vitals", Content: "panic averted barely",
		Template: Normalize("panic averted barely"), Surprise: 3,
		Limbic: LimbicState{Noradrenaline: 0.2}, Time: time.Now()}
	if _, err := s.Encode(hot); err != nil {
		t.Fatal(err)
	}
	d.hot.put(hot)

	seeds, err := d.recallTiered("panic", 3)
	if err != nil {
		t.Fatal(err)
	}
	if len(seeds) != 2 {
		t.Fatalf("both moments should arrive, got %d", len(seeds))
	}
	if seeds[0].ID != hot.ID {
		t.Fatalf("working memory answers first, got #%d", seeds[0].ID)
	}
	if seeds[1].ID != cold.ID {
		t.Fatalf("the archive tops up the reply, got #%d", seeds[1].ID)
	}
}

// A hot hit still writes its retrieval practice through to the archive.
func TestHotRecallPracticesThrough(t *testing.T) {
	d, s := testDaemon(t)
	f := &Fragment{Channel: "syslog", Content: "thermal throttle engaged",
		Template: Normalize("thermal throttle engaged"), Surprise: 4,
		Limbic: LimbicState{Noradrenaline: 0.5}, Time: time.Now()}
	if _, err := s.Encode(f); err != nil {
		t.Fatal(err)
	}
	d.hot.put(f)
	if _, err := s.db.Exec(`UPDATE fragments SET weight = 0.3 WHERE id = ?`, f.ID); err != nil {
		t.Fatal(err)
	}

	if _, err := d.recallTiered("thermal", 3); err != nil {
		t.Fatal(err)
	}
	var w float64
	if err := s.db.QueryRow(`SELECT weight FROM fragments WHERE id = ?`, f.ID).Scan(&w); err != nil {
		t.Fatal(err)
	}
	if w != 1.0 {
		t.Fatalf("a hot recall must refresh the archive weight, got %.2f", w)
	}
}

// Sleep clears the working set: consolidation rewrites the archive under
// any cached copy.
func TestSleepClearsHot(t *testing.T) {
	d, _ := testDaemon(t)
	d.process(Observation{Channel: "testlog", Content: "a moment to hold", At: time.Now()}, true, true)
	if d.hot.len() == 0 {
		t.Fatal("an encode should enter the working set")
	}
	d.sleepCycle()
	if d.hot.len() != 0 {
		t.Fatalf("sleep should clear the working set, %d left", d.hot.len())
	}
}
