package memory

import (
	"testing"
	"time"
)

func repointFixture(t *testing.T) (*Store, uint64, uint64, uint64, uint64) {
	t.Helper()
	s, _ := tempStore(t)
	now := time.Now()
	stub, _ := s.Encode(&Fragment{Kind: KindConcept, Content: "person", Template: "noun:en:person", Time: now})
	class, _ := s.Encode(&Fragment{Kind: KindConcept, Content: "person", Template: "abs:en:person", Time: now})
	animal, _ := s.Encode(&Fragment{Kind: KindConcept, Content: "animal", Template: "abs:en:animal", Time: now})
	moment, _ := s.Encode(&Fragment{Kind: KindEpisodic, Channel: "tell", Content: "a person walked", Template: "a person walked", Time: now})
	return s, stub, class, animal, moment
}

// What a word was told before anything had been shown to it sits on the placeholder the
// telling minted, while the word comes to resolve to the class that grew later. The told
// edge has to follow the word, on whichever side of the edge the placeholder stood.
func TestRepointLinksMovesBothSides(t *testing.T) {
	s, stub, class, animal, moment := repointFixture(t)
	if err := s.SetLinkRel(stub, animal, RelToldIsA, 0.5); err != nil {
		t.Fatal(err)
	}
	if err := s.SetLinkRel(moment, stub, RelAgent, 0.7); err != nil {
		t.Fatal(err)
	}
	if err := s.RepointLinks(stub, class, RelToldIsA); err != nil {
		t.Fatal(err)
	}
	if err := s.RepointLinks(stub, class, RelAgent); err != nil {
		t.Fatal(err)
	}

	told, _ := s.LinksFromRel([]uint64{class}, RelToldIsA)
	if len(told) != 1 || told[0].To != animal || told[0].Weight != 0.5 {
		t.Fatalf("the telling should have followed the word to the class: %+v", told)
	}
	if left, _ := s.LinksFromRel([]uint64{stub}, RelToldIsA); len(left) != 0 {
		t.Fatalf("the placeholder should keep nothing it was told: %+v", left)
	}
	ag, _ := s.LinksFromRel([]uint64{moment}, RelAgent)
	if len(ag) != 1 || ag[0].To != class {
		t.Fatalf("the moment's agent should be the class: %+v", ag)
	}
}

// The class earned whatever it holds of its own, so an arriving duplicate yields to it
// rather than overwriting a weight the class was given.
func TestRepointLinksKeepsTheDestinationsOwnEdge(t *testing.T) {
	s, stub, class, animal, _ := repointFixture(t)
	if err := s.SetLinkRel(stub, animal, RelToldIsA, 0.2); err != nil {
		t.Fatal(err)
	}
	if err := s.SetLinkRel(class, animal, RelToldIsA, 0.9); err != nil {
		t.Fatal(err)
	}
	if err := s.RepointLinks(stub, class, RelToldIsA); err != nil {
		t.Fatal(err)
	}
	told, _ := s.LinksFromRel([]uint64{class}, RelToldIsA)
	if len(told) != 1 || told[0].Weight != 0.9 {
		t.Fatalf("the class should keep the edge it earned: %+v", told)
	}
	if left, _ := s.LinksFromRel([]uint64{stub}, RelToldIsA); len(left) != 0 {
		t.Fatalf("the placeholder should be left holding nothing: %+v", left)
	}
}

// An edge between the two nodes would become a loop on the destination, which asserts
// nothing, so it is dropped rather than written.
func TestRepointLinksDropsWhatWouldLoop(t *testing.T) {
	s, stub, class, _, _ := repointFixture(t)
	if err := s.SetLinkRel(stub, class, RelToldIsA, 0.5); err != nil {
		t.Fatal(err)
	}
	if err := s.SetLinkRel(class, stub, RelKindOf, 0.5); err != nil {
		t.Fatal(err)
	}
	if err := s.RepointLinks(stub, class, RelToldIsA); err != nil {
		t.Fatal(err)
	}
	if err := s.RepointLinks(stub, class, RelKindOf); err != nil {
		t.Fatal(err)
	}
	for _, rel := range []string{RelToldIsA, RelKindOf} {
		if e, _ := s.LinksFromRel([]uint64{class}, rel); len(e) != 0 {
			t.Fatalf("%s should carry no loop on the class: %+v", rel, e)
		}
	}
}

// Repointing a node onto itself, or onto nothing, leaves the graph alone.
func TestRepointLinksIsInertOnItself(t *testing.T) {
	s, stub, _, animal, _ := repointFixture(t)
	if err := s.SetLinkRel(stub, animal, RelToldIsA, 0.5); err != nil {
		t.Fatal(err)
	}
	for _, dst := range []uint64{stub, 0} {
		if err := s.RepointLinks(stub, dst, RelToldIsA); err != nil {
			t.Fatal(err)
		}
	}
	if e, _ := s.LinksFromRel([]uint64{stub}, RelToldIsA); len(e) != 1 {
		t.Fatalf("the edge should have been left alone: %+v", e)
	}
}
