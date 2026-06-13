package daemon

import (
	"testing"
	"time"

	"golem/internal/memory"
)

// The walk crosses a relation: from a concept that filled the agent role it
// reaches what filled the patient of the shared moment, and from the patient
// back to the agent, the proposition substrate stepped through. The crossing
// names the relation it went by.
func TestWalkCrossesRelation(t *testing.T) {
	d, s := testDaemon(t)
	now := time.Now()
	chase, _ := s.Encode(&memory.Fragment{Kind: memory.KindConcept, Content: "chase", Template: "rel:en:chase", Category: "relation", Weight: 1, Time: now})
	dog, _ := s.Encode(&memory.Fragment{Kind: memory.KindConcept, Content: "a dog", Template: "a dog", Weight: 1, Time: now})
	cat, _ := s.Encode(&memory.Fragment{Kind: memory.KindConcept, Content: "a cat", Template: "a cat", Weight: 1, Time: now})
	moment, _ := s.Encode(&memory.Fragment{Kind: memory.KindEpisodic, Channel: "vision", Content: "a dog chased a cat", Template: "a dog chased a cat", Weight: 1, Time: now})
	if err := s.Propose(chase, moment, dog, cat, 0.9); err != nil {
		t.Fatal(err)
	}

	crosses := func(seed, want uint64, rel string) bool {
		dests, err := d.walkableDests(seed)
		if err != nil {
			t.Fatal(err)
		}
		for _, dst := range dests[rel] {
			if dst.id == want {
				if dst.via != "chase" {
					t.Fatalf("the crossing should name the relation, got %q", dst.via)
				}
				return true
			}
		}
		return false
	}

	if !crosses(dog, cat, memory.RelPatient) {
		t.Fatal("a walk from the dog should cross to the cat it chased")
	}
	if !crosses(cat, dog, memory.RelAgent) {
		t.Fatal("a walk from the cat should cross to the dog that chased it")
	}
}
