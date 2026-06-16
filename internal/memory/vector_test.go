package memory

import (
	"testing"
	"time"
)

func TestCosine(t *testing.T) {
	if got := Cosine([]float32{1, 0, 0}, []float32{1, 0, 0}); got < 0.999 {
		t.Errorf("identical vectors should be ~1, got %v", got)
	}
	if got := Cosine([]float32{1, 0}, []float32{0, 1}); got != 0 {
		t.Errorf("orthogonal vectors should be 0, got %v", got)
	}
	if got := Cosine([]float32{1, 0}, []float32{1, 0, 0}); got != 0 {
		t.Errorf("mismatched widths should be 0, got %v", got)
	}
	if got := Cosine(nil, nil); got != 0 {
		t.Errorf("empty vectors should be 0, got %v", got)
	}
}

// A fragment's embedding round-trips through the store at full width, and a
// fragment with none reports so.
func TestVectorRoundTrip(t *testing.T) {
	s, _ := tempStore(t)
	f := encodeAt(t, s, "vision", "a brown dog", 4, time.Now())
	v := []float32{0.1, -0.2, 0.3, 0.4}
	if err := s.SetVector(f.ID, v); err != nil {
		t.Fatal(err)
	}
	got, ok := s.Vector(f.ID)
	if !ok || len(got) != 4 || got[2] != 0.3 || got[1] != -0.2 {
		t.Fatalf("the embedding should read back intact: %v %v", got, ok)
	}
	if _, ok := s.Vector(999999); ok {
		t.Fatal("a fragment with no embedding should report none")
	}
}

// NearestVectors ranks the concepts nearest a target by cosine, the seed
// excluded, and a kind filter keeps it among concepts.
func TestNearestVectors(t *testing.T) {
	s, _ := tempStore(t)
	mkConcept := func(name string, vec []float32) uint64 {
		c := &Fragment{Kind: KindConcept, Channel: "concept", Content: name, Template: name, Time: time.Now()}
		if _, err := s.Encode(c); err != nil {
			t.Fatal(err)
		}
		if err := s.SetVector(c.ID, vec); err != nil {
			t.Fatal(err)
		}
		return c.ID
	}
	dog := mkConcept("dog", []float32{1, 0, 0})
	wolf := mkConcept("wolf", []float32{0.95, 0.05, 0}) // near dog
	mkConcept("rock", []float32{0, 0, 1})               // far from dog
	// An episode with an embedding must not show among concept neighbours.
	ep := encodeAt(t, s, "vision", "a passing cloud", 4, time.Now())
	if err := s.SetVector(ep.ID, []float32{1, 0, 0}); err != nil {
		t.Fatal(err)
	}

	got, err := s.NearestVectors([]float32{1, 0, 0}, KindConcept, 10, dog)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("two other concepts have embeddings (the episode is filtered out): %+v", got)
	}
	if got[0].ID != wolf {
		t.Fatalf("wolf should be the nearest neighbour of dog: %+v", got)
	}
	if got[0].Score <= got[1].Score {
		t.Fatalf("the nearer neighbour should score higher: %+v", got)
	}
	for _, m := range got {
		if m.ID == ep.ID {
			t.Fatal("the episode embedding should be excluded by the kind filter")
		}
	}
}
