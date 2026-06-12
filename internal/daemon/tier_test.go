package daemon

import (
	"testing"
	"time"

	"golem/internal/attention"
	"golem/internal/limbic"
	"golem/internal/memory"
	"golem/internal/senses"
	"golem/internal/wire"
)

// A recall that has to reach the archive promotes its findings into the
// working set.
func TestRecallPromotesToHot(t *testing.T) {
	d, s := testDaemon(t)
	f := &memory.Fragment{Channel: "syslog", Content: "Segfault in worker 3",
		Template: attention.Normalize("Segfault in worker 3"), Surprise: 5,
		Limbic: limbic.State{Noradrenaline: 0.8}, Time: time.Now()}
	if _, err := s.Encode(f); err != nil { // straight into the archive, never hot
		t.Fatal(err)
	}
	if _, ok := d.hot.Get(f.ID); ok {
		t.Fatal("the fragment should start cold")
	}

	resp, _, _ := d.handle(wire.Request{Cmd: "recall", Query: "segfault"}, nil)
	if !resp.OK || len(resp.Fragments) == 0 {
		t.Fatalf("recall should answer from the archive: %+v", resp)
	}
	if _, ok := d.hot.Get(f.ID); !ok {
		t.Fatal("an archive hit should promote into the working set")
	}
}

// The working set answers first: a hot match outranks a colder archive
// match in the reply order, and both arrive.
func TestRecallPrefersHot(t *testing.T) {
	d, s := testDaemon(t)
	cold := &memory.Fragment{Channel: "syslog", Content: "panic stack overflow deep",
		Template: attention.Normalize("panic stack overflow deep"), Surprise: 6,
		Limbic: limbic.State{Noradrenaline: 0.95}, Time: time.Now()}
	if _, err := s.Encode(cold); err != nil {
		t.Fatal(err)
	}
	hot := &memory.Fragment{Channel: "vitals", Content: "panic averted barely",
		Template: attention.Normalize("panic averted barely"), Surprise: 3,
		Limbic: limbic.State{Noradrenaline: 0.2}, Time: time.Now()}
	if _, err := s.Encode(hot); err != nil {
		t.Fatal(err)
	}
	d.hot.Put(hot)

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
	f := &memory.Fragment{Channel: "syslog", Content: "thermal throttle engaged",
		Template: attention.Normalize("thermal throttle engaged"), Surprise: 4,
		Limbic: limbic.State{Noradrenaline: 0.5}, Time: time.Now()}
	if _, err := s.Encode(f); err != nil {
		t.Fatal(err)
	}
	d.hot.Put(f)
	// Nights of decay fade the archive copy while the hot copy stays.
	for range 60 {
		if _, err := s.Sleep(0.10, 0.01); err != nil {
			t.Fatal(err)
		}
	}
	if before, err := s.FragmentsByIDs([]uint64{f.ID}); err != nil || len(before) != 1 || before[0].Weight >= 0.5 {
		t.Fatalf("the archive copy should have faded first: %+v %v", before, err)
	}

	if _, err := d.recallTiered("thermal", 3); err != nil {
		t.Fatal(err)
	}
	after, err := s.FragmentsByIDs([]uint64{f.ID})
	if err != nil || len(after) != 1 || after[0].Weight != 1.0 {
		t.Fatalf("a hot recall must refresh the archive weight: %+v %v", after, err)
	}
}

// Sleep clears the working set: consolidation rewrites the archive under
// any cached copy.
func TestSleepClearsHot(t *testing.T) {
	d, _ := testDaemon(t)
	d.process(senses.Observation{Channel: "testlog", Content: "a moment to hold", At: time.Now()}, true, true)
	if d.hot.Len() == 0 {
		t.Fatal("an encode should enter the working set")
	}
	d.sleepCycle()
	if d.hot.Len() != 0 {
		t.Fatalf("sleep should clear the working set, %d left", d.hot.Len())
	}
}

// A thought re-enters memory carrying a concept's gloss verbatim, so it
// substring-matches a concept query. Tiered recall keeps it reachable but
// ranks grounded recollections above the mind's self-talk, across both
// tiers, so an inner-loop thought cannot crowd out the experience asked for.
func TestRecallTieredDemotesThoughts(t *testing.T) {
	d, s := testDaemon(t)
	s.Encode(&memory.Fragment{Channel: "thought", Content: "thought: that time: a small brown dog",
		Surprise: 5, Limbic: limbic.State{Noradrenaline: 1.0}, Time: time.Now()})
	s.Encode(&memory.Fragment{Channel: "vision", Content: "a small brown dog",
		Surprise: 4, Limbic: limbic.State{Noradrenaline: 0.4}, Time: time.Now()})

	frags, err := d.recallTiered("dog", 5)
	if err != nil {
		t.Fatal(err)
	}
	if len(frags) != 2 {
		t.Fatalf("both the sighting and the thought should be recallable: %+v", frags)
	}
	if frags[0].Channel != "vision" {
		t.Fatalf("a grounded recollection should outrank self-talk, got %q first", frags[0].Channel)
	}
}
