package memory

import (
	"math"
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

// Sub-conscious mass is decayable: it bleeds each night so a percept that stops recurring fades below
// the genesis threshold, while conscious mass, which carries no submass, is conserved across a night.
// This is the time axis of faintness. It keeps every existing brain and all conscious genesis unchanged
// (submass is zero there, so the decay is a no-op) while denying a spaced ambient drip an endless climb.
func TestSubmassBleedsWhileConsciousMassIsConserved(t *testing.T) {
	s, err := OpenStore(filepath.Join(t.TempDir(), "m.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	// A conscious prototype (mass == count, no submass) and a sub-conscious one (all mass is submass).
	consc := &Fragment{Channel: "vision", Content: "c", Template: "c", Kind: KindEpisodic,
		Weight: 1.0, Count: 5, Mass: 5, Time: time.Unix(0, 1)}
	cid, err := s.Encode(consc)
	if err != nil {
		t.Fatal(err)
	}
	subl := &Fragment{Channel: "vision", Content: "s", Template: "s", Kind: KindEpisodic,
		Weight: 0.35, Mass: 3.0, Submass: 3.0, Time: time.Unix(0, 2)}
	sid, err := s.Encode(subl)
	if err != nil {
		t.Fatal(err)
	}

	if _, err := s.Sleep(0.1, 0.01); err != nil {
		t.Fatal(err)
	}

	frags, err := s.FragmentsByIDs([]uint64{cid, sid})
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range frags {
		switch f.ID {
		case cid:
			if f.Mass != 5 || f.Submass != 0 {
				t.Fatalf("conscious mass must be conserved with no submass: mass=%v submass=%v", f.Mass, f.Submass)
			}
		case sid:
			if f.Submass >= 3.0 || f.Mass >= 3.0 {
				t.Fatalf("sub-conscious mass must bleed below its start: mass=%v submass=%v", f.Mass, f.Submass)
			}
			if math.Abs(f.Mass-f.Submass) > 1e-9 {
				t.Fatalf("a pure sub-conscious trace's mass is all submass: mass=%v submass=%v", f.Mass, f.Submass)
			}
		}
	}
}

// A percept dripping one sub-conscious sighting per night never mints, because the bleed outpaces the
// slow accrual; the same number of sightings CLUSTERED into one wake window (before any bleed) does mint.
// This is what actually closes the ambient hole: recurrence must be dense enough to beat the decay, so
// sustained cross-situational learning forms a concept while a sparse background drip fades away.
func TestSpacedSubliminalDripDoesNotMintButClusterDoes(t *testing.T) {
	drip := func(spaced bool) bool {
		s, err := OpenStore(filepath.Join(t.TempDir(), "m.db"))
		if err != nil {
			t.Fatal(err)
		}
		defer s.Close()
		const subMass = 0.3 // one sub-conscious sighting's mass, all of it decayable
		for i := 0; i < 20; i++ {
			f := &Fragment{Channel: "vision", Content: "kettle", Template: "kettle", Kind: KindEpisodic,
				Weight: 0.35, Mass: subMass, Submass: subMass, Time: time.Unix(0, int64(i+1))}
			if _, err := s.Encode(f); err != nil {
				t.Fatal(err)
			}
			if spaced { // a night between each sighting bleeds the accrued submass
				if _, err := s.Sleep(0.1, 0.01); err != nil {
					t.Fatal(err)
				}
			}
		}
		if _, err := s.MergeDuplicates(); err != nil {
			t.Fatal(err)
		}
		cands, err := s.ConceptCandidates(float64(MergeThreshold), 100)
		if err != nil {
			t.Fatal(err)
		}
		for _, c := range cands {
			if c.Template == "kettle" {
				return true
			}
		}
		return false
	}
	if drip(true) {
		t.Fatal("a spaced drip must bleed away and never mint")
	}
	if !drip(false) {
		t.Fatal("a dense cluster of the same sightings should mint before the bleed")
	}
}
