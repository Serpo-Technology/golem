package memory

import (
	"path/filepath"
	"testing"
	"time"
)

// Concept genesis is gated on SALIENCE MASS, not raw occurrence count. Two templates seen the same
// number of times form the same-size prototype, but only the one seen with full salience earns a
// concept; the routine (low-mass) one does not. Enough routine showings still reach genesis by
// accumulated mass, so the gate is a threshold on mass, not a ban on low salience. This is the
// invariant that lets an always-on GOLEM take ambient input without minting junk concepts.
func TestGenesisGatesOnSalienceMass(t *testing.T) {
	s, err := OpenStore(filepath.Join(t.TempDir(), "m.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	enc := func(tpl string, mass float64, n int) {
		for i := 0; i < n; i++ {
			f := &Fragment{Channel: "vision", Content: tpl, Template: tpl,
				Kind: KindEpisodic, Weight: 1.0, Mass: mass, Time: time.Unix(0, int64(i+1))}
			if _, err := s.Encode(f); err != nil {
				t.Fatal(err)
			}
		}
	}
	// Same count for faint and taught (5 each -> the merge keeps one exemplar out, so a prototype of
	// count 4), so only the MASS differs between them.
	enc("faint", 0.1, 5)    // routine: prototype mass ~0.4, far below the threshold
	enc("taught", 1.0, 5)   // attended: prototype mass ~4.0, above the threshold
	enc("drilled", 0.1, 40) // routine but persistent: prototype mass ~3.9, reaches genesis by volume

	if _, err := s.MergeDuplicates(); err != nil {
		t.Fatal(err)
	}
	cands, err := s.ConceptCandidates(float64(MergeThreshold), 100)
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]bool{}
	for _, c := range cands {
		got[c.Template] = true
	}

	if got["faint"] {
		t.Fatal("a few routine (low-mass) showings must not earn a concept")
	}
	if !got["taught"] {
		t.Fatal("a few attended (full-mass) showings should earn a concept")
	}
	if !got["drilled"] {
		t.Fatal("enough routine showings should still reach genesis by accumulated mass")
	}
}

// The default is byte-identical to the old count gate: a fragment carrying no explicit mass weighs one
// unit per count, so mass equals count and a merged prototype of count >= the threshold is still a
// candidate. This pins that nothing changes for the current teaching path (every occurrence is full
// salience) before the graded gate ever lowers a mass.
func TestGenesisDefaultMassEqualsCount(t *testing.T) {
	s, err := OpenStore(filepath.Join(t.TempDir(), "m.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	for i := 0; i < 5; i++ { // no Mass set: defaults to the count (1 each)
		f := &Fragment{Channel: "vision", Content: "kettle", Template: "kettle",
			Kind: KindEpisodic, Weight: 1.0, Time: time.Unix(0, int64(i+1))}
		if _, err := s.Encode(f); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := s.MergeDuplicates(); err != nil {
		t.Fatal(err)
	}
	cands, err := s.ConceptCandidates(float64(MergeThreshold), 100)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, c := range cands {
		if c.Template == "kettle" {
			found = true
			if c.Mass < float64(c.Count) { // mass tracks count for a full-salience prototype
				t.Fatalf("default mass should equal count: mass=%v count=%d", c.Mass, c.Count)
			}
		}
	}
	if !found {
		t.Fatal("a full-salience template seen enough should earn a concept exactly as before")
	}
}
