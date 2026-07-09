package daemon

import (
	"testing"
	"time"

	"golem/internal/memory"
)

// A proportion a:b::c:? on a regularity no relation names is completed by the offset the first
// pair shares in the kind space: king is to queen as man is to woman, where queen - king is the
// same move as woman - man. The fourth term is the concept nearest man + (queen - king), and it
// is spoken as a thin "perhaps" guess, since the geometry proposes it with no fact to firm it.
func TestVectorAnalogyCompletesParallelogram(t *testing.T) {
	d, s := testDaemon(t)
	king := groundNoun(t, d, "king")
	queen := groundNoun(t, d, "queen")
	man := groundNoun(t, d, "man")
	woman := groundNoun(t, d, "woman")
	child := groundNoun(t, d, "child")
	set := func(f *memory.Fragment, v []float32) {
		if err := s.SetVector(f.ID, v); err != nil {
			t.Fatal(err)
		}
	}
	// Dim 0 the gender move, dim 1 the royalty axis: queen - king == woman - man == [-2,0,0,0].
	set(king, []float32{1, 1, 0, 0})
	set(queen, []float32{-1, 1, 0, 0})
	set(man, []float32{1, -1, 0, 0})
	set(woman, []float32{-1, -1, 0, 0})
	set(child, []float32{0, 0, 1, 0}) // a distractor off the plane

	got, ok := d.vectorAnalogy(king, queen, man)
	if !ok || got != woman.ID {
		t.Fatalf("king:queen::man:? should complete to woman by the offset, got id=%d ok=%v", got, ok)
	}
	if line, ok := d.sayVectorAnalogy("en", got); !ok || line != "Perhaps the woman." {
		t.Fatalf("the vector guess should read as a thin perhaps: %q ok=%v", line, ok)
	}

	// End to end through the proportion parse: the same guess, spoken.
	if line := say(t, d, "the king is to the queen as the man is to what").Say; line != "Perhaps the woman." {
		t.Fatalf("the spoken proportion should offer the woman as a perhaps: %q", line)
	}
}

// The vector offset admits the gap honestly: a featureless concept has no kind vector to ride,
// and a target near nothing names no fourth term, so neither forces a guess.
func TestVectorAnalogyAdmitsGap(t *testing.T) {
	d, s := testDaemon(t)
	king := groundNoun(t, d, "king")
	queen := groundNoun(t, d, "queen")
	man := groundNoun(t, d, "man")
	rock := groundNoun(t, d, "rock")
	set := func(f *memory.Fragment, v []float32) {
		if err := s.SetVector(f.ID, v); err != nil {
			t.Fatal(err)
		}
	}
	// king/queen sit together on one axis; man's offset target lands off toward an empty region
	// no concept occupies, so the nearest clears no floor.
	set(king, []float32{1, 0, 0, 0})
	set(queen, []float32{1, 0.1, 0, 0})
	set(man, []float32{0, 0, 1, 0})
	set(rock, []float32{0, 0, 0, 1})
	if _, ok := d.vectorAnalogy(king, queen, man); ok {
		t.Fatal("a target near no concept should admit the gap, not force a fourth term")
	}

	// A featureless concept (no kind vector) cannot ride the geometry at all.
	idea := groundNoun(t, d, "idea") // never given a vector
	if _, ok := d.vectorAnalogy(king, queen, idea); ok {
		t.Fatal("a concept with no kind vector should admit the gap")
	}
}

// A nameless fourth term is withheld: a concept the offset lands on that carries a kind vector but
// no name in the tongue is no answer to speak, so the guess admits the gap rather than "Perhaps the ."
func TestVectorAnalogyGapsOnNamelessTerm(t *testing.T) {
	d, s := testDaemon(t)
	ghost := &memory.Fragment{Kind: memory.KindConcept, Channel: "concept", Content: "ghost", Time: time.Now()}
	if _, err := s.Encode(ghost); err != nil { // a concept with a vector but no naming word
		t.Fatal(err)
	}
	if err := s.SetVector(ghost.ID, []float32{1, 0, 0, 0}); err != nil {
		t.Fatal(err)
	}
	if _, ok := d.sayVectorAnalogy("en", ghost.ID); ok {
		t.Fatal("a nameless fourth term should admit the gap, not speak an empty perhaps")
	}
}
