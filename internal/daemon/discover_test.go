package daemon

import (
	"strings"
	"testing"
	"time"

	"golem/internal/memory"
)

// GOLEM notices an un-named kind in its own space: concepts that sit together in
// the embedding but were never given a common word are offered, hedged, as a kind
// for the speaker to name. A concept far off in the space is left out, and the
// kind is proposed once. This is the vector proposing a category the graph never
// grew; the graph disposes, grounding nothing until the kind is named.
func TestProposesUnnamedKindFromCluster(t *testing.T) {
	d, s := testDaemon(t)
	mk := func(name string, vec []float32) *memory.Fragment {
		c := &memory.Fragment{Kind: memory.KindConcept, Channel: "concept", Content: name,
			Template: name, Limbic: d.chem.Snapshot(), Time: time.Now()}
		if _, err := s.Encode(c); err != nil {
			t.Fatal(err)
		}
		if err := s.SetVector(c.ID, vec); err != nil {
			t.Fatal(err)
		}
		return c
	}
	// cat, dog, mouse cluster; rock sits orthogonal, outside the kind. None shares
	// a category, so the cluster is an un-named kind.
	cat := mk("cat", []float32{1, 0, 0})
	mk("dog", []float32{0.97, 0.06, 0})
	mk("mouse", []float32{0.95, 0.09, 0})
	mk("rock", []float32{0, 0, 1})

	d.topic = cat.ID
	line, ok := d.volunteerRemark("en")
	if !ok {
		t.Fatal("GOLEM should propose the un-named kind it noticed around the cat")
	}
	for _, w := range []string{"cat", "dog", "mouse", "kind"} {
		if !strings.Contains(line, w) {
			t.Fatalf("the proposal should name the cluster and offer it as a kind: %q", line)
		}
	}
	if strings.Contains(line, "rock") {
		t.Fatalf("rock sits outside the cluster and should not be in the kind: %q", line)
	}

	// The kind is proposed once: a second prompt about the same cluster adds nothing.
	if again, ok := d.volunteerRemark("en"); ok {
		t.Fatalf("an un-named kind is offered once, not on every turn: %q", again)
	}
}

// An already-named kind is not rediscovered: when the near concepts already share
// a category, there is no un-named kind to propose, so GOLEM stays silent rather
// than re-offering what the graph already holds.
func TestNamedKindIsNotReproposed(t *testing.T) {
	d, s := testDaemon(t)
	mk := func(name string, vec []float32) *memory.Fragment {
		c := &memory.Fragment{Kind: memory.KindConcept, Channel: "concept", Content: name,
			Template: name, Limbic: d.chem.Snapshot(), Time: time.Now()}
		if _, err := s.Encode(c); err != nil {
			t.Fatal(err)
		}
		if err := s.SetVector(c.ID, vec); err != nil {
			t.Fatal(err)
		}
		return c
	}
	cat := mk("cat", []float32{1, 0, 0})
	dog := mk("dog", []float32{0.97, 0.06, 0})
	mouse := mk("mouse", []float32{0.95, 0.09, 0})
	// An abstraction grounds in all three: the kind already has a word.
	animal := &memory.Fragment{Kind: memory.KindConcept, Channel: "concept", Content: "animal",
		Template: "abs:en:animal", Limbic: d.chem.Snapshot(), Time: time.Now()}
	if _, err := s.Encode(animal); err != nil {
		t.Fatal(err)
	}
	for _, m := range []*memory.Fragment{cat, dog, mouse} {
		s.SetLinkRel(animal.ID, m.ID, memory.RelGrounds, 1)
	}

	d.topic = cat.ID
	if line, ok := d.volunteerRemark("en"); ok {
		t.Fatalf("a kind that already has a word should not be re-proposed: %q", line)
	}
}
