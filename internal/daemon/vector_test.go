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

// The pure-Go embedder learns concept coordinates from feature company: two
// concepts that share features land near in the learned space, one with a
// distinct signature lands apart. The embedder runs over GOLEM's own grounded
// signatures, no outside model.
func TestGrowEmbeddingsFromFeatures(t *testing.T) {
	d, s := testDaemon(t)
	concept := func(name string) *memory.Fragment {
		c := &memory.Fragment{Kind: memory.KindConcept, Channel: "concept", Content: name,
			Template: name, Limbic: d.chem.Snapshot(), Time: time.Now()}
		if _, err := s.Encode(c); err != nil {
			t.Fatal(err)
		}
		return c
	}
	feature := func(name string) *memory.Fragment {
		f := &memory.Fragment{Kind: memory.KindFeature, Channel: "feature", Content: name,
			Template: name, Limbic: d.chem.Snapshot(), Time: time.Now()}
		if _, err := s.Encode(f); err != nil {
			t.Fatal(err)
		}
		return f
	}
	// dog and wolf share two features; cat shares one with each; rock shares none.
	furry, fourLegs, wild, hard := feature("furry"), feature("four-legs"), feature("wild"), feature("hard")
	link := func(c, f *memory.Fragment, w float64) { s.SetLinkRel(c.ID, f.ID, memory.RelFeature, w) }
	dog, wolf, cat, rock := concept("dog"), concept("wolf"), concept("cat"), concept("rock")
	link(dog, furry, 1)
	link(dog, fourLegs, 1)
	link(wolf, furry, 1)
	link(wolf, fourLegs, 1)
	link(wolf, wild, 1)
	link(cat, furry, 1)
	link(cat, fourLegs, 1)
	link(rock, hard, 1)

	d.growEmbeddings()

	vd, ok1 := s.Vector(dog.ID)
	vw, ok2 := s.Vector(wolf.ID)
	vr, ok3 := s.Vector(rock.ID)
	if !ok1 || !ok2 || !ok3 {
		t.Fatalf("every concept with features should gain an embedding: %v %v %v", ok1, ok2, ok3)
	}
	dogWolf := memory.Cosine(vd, vw)
	dogRock := memory.Cosine(vd, vr)
	if dogWolf <= dogRock {
		t.Fatalf("dog should sit nearer wolf than rock in the learned space: dog~wolf=%.3f dog~rock=%.3f",
			dogWolf, dogRock)
	}
}

// Category membership bridges the gap feature overlap leaves: two concepts with
// no shared feature still land near in the learned space when they belong to the
// same category, so a sparsely-featured concept resembles its kind above the
// blend floor. This is the source-matrix enrichment that lets a golden lion find
// the dog through the animal they share, where feature tokens alone left it adrift.
func TestGrowEmbeddingsBridgesByCategory(t *testing.T) {
	d, s := testDaemon(t)
	concept := func(name string) *memory.Fragment {
		c := &memory.Fragment{Kind: memory.KindConcept, Channel: "concept", Content: name,
			Template: name, Limbic: d.chem.Snapshot(), Time: time.Now()}
		if _, err := s.Encode(c); err != nil {
			t.Fatal(err)
		}
		return c
	}
	feature := func(name string) *memory.Fragment {
		f := &memory.Fragment{Kind: memory.KindFeature, Channel: "feature", Content: name,
			Template: name, Limbic: d.chem.Snapshot(), Time: time.Now()}
		if _, err := s.Encode(f); err != nil {
			t.Fatal(err)
		}
		return f
	}
	link := func(from, to *memory.Fragment, rel string, w float64) { s.SetLinkRel(from.ID, to.ID, rel, w) }
	// lion and dog share NO feature; rock shares none either and is no animal.
	golden, mane := feature("golden"), feature("mane")
	small, brown := feature("small"), feature("brown")
	hard := feature("hard")
	animal := concept("animal")
	lion, dog, rock := concept("lion"), concept("dog"), concept("rock")
	link(lion, golden, memory.RelFeature, 1)
	link(lion, mane, memory.RelFeature, 1)
	link(dog, small, memory.RelFeature, 1)
	link(dog, brown, memory.RelFeature, 1)
	link(rock, hard, memory.RelFeature, 1)
	// The animal abstraction grounds in lion and dog, the is-a membership, not rock.
	link(animal, lion, memory.RelGrounds, 1)
	link(animal, dog, memory.RelGrounds, 1)

	d.growEmbeddings()

	vl, ok1 := s.Vector(lion.ID)
	vd, ok2 := s.Vector(dog.ID)
	vr, ok3 := s.Vector(rock.ID)
	if !ok1 || !ok2 || !ok3 {
		t.Fatalf("every concept should gain an embedding: %v %v %v", ok1, ok2, ok3)
	}
	lionDog := memory.Cosine(vl, vd)
	lionRock := memory.Cosine(vl, vr)
	if lionDog < vectorSimFloor {
		t.Fatalf("lion should resemble dog through the shared animal category, above the blend floor: lion~dog=%.3f floor=%.2f", lionDog, vectorSimFloor)
	}
	if lionDog <= lionRock {
		t.Fatalf("the shared category should put lion nearer dog than the category-less rock: lion~dog=%.3f lion~rock=%.3f", lionDog, lionRock)
	}
}
