package memory

import (
	"testing"
	"time"
)

// A relation reified as a concept grounds in the moment that instantiates it,
// and the moment names what filled each role, so "a dog chased a cat" leaves
// chase grounded in the moment with the dog as agent and the cat as patient.
func TestProposeRecordsRolesAndGrounding(t *testing.T) {
	s, _ := tempStore(t)
	now := time.Now()
	chase, _ := s.Encode(&Fragment{Kind: KindConcept, Content: "chase", Template: "rel:en:chase", Category: "relation", Time: now})
	dog, _ := s.Encode(&Fragment{Kind: KindConcept, Content: "a dog", Template: "a dog", Time: now})
	cat, _ := s.Encode(&Fragment{Kind: KindConcept, Content: "a cat", Template: "a cat", Time: now})
	moment, _ := s.Encode(&Fragment{Kind: KindEpisodic, Channel: "tell", Content: "a dog chased a cat", Template: "a dog chased a cat", Time: now})

	if err := s.Propose(chase, moment, dog, cat, 0.8); err != nil {
		t.Fatal(err)
	}

	grounds, _ := s.LinksFromRel([]uint64{chase}, RelGrounds)
	if len(grounds) != 1 || grounds[0].To != moment {
		t.Fatalf("chase should ground in the moment: %+v", grounds)
	}
	ag, _ := s.LinksFromRel([]uint64{moment}, RelAgent)
	if len(ag) != 1 || ag[0].To != dog {
		t.Fatalf("the dog should be the agent: %+v", ag)
	}
	pat, _ := s.LinksFromRel([]uint64{moment}, RelPatient)
	if len(pat) != 1 || pat[0].To != cat {
		t.Fatalf("the cat should be the patient: %+v", pat)
	}
}

// An intransitive instance carries an agent and no patient, so the same
// structure holds a one-place relation as well as a two-place one.
func TestProposeIntransitiveLeavesPatientUnbound(t *testing.T) {
	s, _ := tempStore(t)
	now := time.Now()
	sleep, _ := s.Encode(&Fragment{Kind: KindConcept, Content: "sleep", Template: "rel:en:sleep", Category: "relation", Time: now})
	dog, _ := s.Encode(&Fragment{Kind: KindConcept, Content: "a dog", Template: "a dog", Time: now})
	moment, _ := s.Encode(&Fragment{Kind: KindEpisodic, Channel: "tell", Content: "a dog slept", Template: "a dog slept", Time: now})

	if err := s.Propose(sleep, moment, dog, 0, 0.8); err != nil {
		t.Fatal(err)
	}
	if pat, _ := s.LinksFromRel([]uint64{moment}, RelPatient); len(pat) != 0 {
		t.Fatalf("an intransitive moment should have no patient: %+v", pat)
	}
	if ag, _ := s.LinksFromRel([]uint64{moment}, RelAgent); len(ag) != 1 {
		t.Fatal("the agent should still bind")
	}
}
