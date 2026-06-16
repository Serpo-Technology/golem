package daemon

import (
	"testing"
	"time"

	"golem/internal/memory"
)

// Resemblance gains a geometric source: a concept with an embedding draws its
// nearest neighbours in the space into its sibling set, even with no shared
// feature tokens, so analogy can run on nearness. A concept with no embedding
// is untouched, so the symbolic walk stands unchanged until the layer fills.
func TestFeatureSiblingsBlendsVectors(t *testing.T) {
	d, s := testDaemon(t)
	mk := func(name string, vec []float32) *memory.Fragment {
		c := &memory.Fragment{Kind: memory.KindConcept, Channel: "concept", Content: name,
			Template: name, Limbic: d.chem.Snapshot(), Time: time.Now()}
		if _, err := s.Encode(c); err != nil {
			t.Fatal(err)
		}
		if vec != nil {
			if err := s.SetVector(c.ID, vec); err != nil {
				t.Fatal(err)
			}
		}
		return c
	}
	dog := mk("dog", []float32{1, 0, 0})
	wolf := mk("wolf", []float32{0.95, 0.05, 0}) // near dog
	rock := mk("rock", []float32{0, 0, 1})       // orthogonal, below the floor

	sibs, err := d.featureSiblings(dog.ID, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	foundWolf := false
	for _, sib := range sibs {
		if sib.id == wolf.ID {
			foundWolf = true
		}
		if sib.id == rock.ID {
			t.Fatalf("rock is too far to be a vector sibling: %+v", sibs)
		}
	}
	if !foundWolf {
		t.Fatalf("the vector layer should make wolf a sibling of dog: %+v", sibs)
	}

	// A concept with neither embedding nor feature tokens has no siblings, so
	// nothing the vector layer added perturbs the bare case.
	bare := mk("bare", nil)
	if sibs2, _ := d.featureSiblings(bare.ID, time.Now()); len(sibs2) != 0 {
		t.Fatalf("a concept with no embedding and no features has no siblings: %+v", sibs2)
	}
}
