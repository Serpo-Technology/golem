package memory

import (
	"testing"
	"time"
)

// mintConcept distils a shape into a concept node by hand, the way the
// daemon's genesis pass does.
func mintConcept(t *testing.T, s *Store, gloss, template, category string) *Fragment {
	t.Helper()
	c := &Fragment{Kind: KindConcept, Channel: "concept", Content: gloss,
		Template: template, Category: category, Time: time.Now()}
	if _, err := s.Encode(c); err != nil {
		t.Fatal(err)
	}
	return c
}

// An utterance's shape never becomes a concept, however often it
// recurs: speech is language about the world, not the world.
func TestUtterancesAreNotConceptCandidates(t *testing.T) {
	s := openTestStore(t)
	now := time.Now()
	for i := range 4 {
		encodeAt(t, s, "tell", "what usually happens after make test", 2.0,
			now.Add(time.Duration(i)*time.Minute))
	}
	for i := range 4 {
		encodeAt(t, s, "syslog", "Disk full on /dev/disk1s1", 3.0,
			now.Add(time.Duration(i)*time.Minute))
	}
	if _, err := s.MergeDuplicates(); err != nil {
		t.Fatal(err)
	}

	cands, err := s.ConceptCandidates(MergeThreshold, 16)
	if err != nil {
		t.Fatal(err)
	}
	if len(cands) != 1 || cands[0].Channel != "syslog" {
		t.Fatalf("only the lived shape should be a candidate: %+v", cands)
	}
}

// Reground never attaches a concept to an utterance, even when the
// templates happen to match.
func TestRegroundSkipsUtterances(t *testing.T) {
	s := openTestStore(t)
	now := time.Now()
	lived := encodeAt(t, s, "syslog", "Disk full on /dev/disk1s1", 3.0, now)
	spoken := encodeAt(t, s, "tell", "Disk full on /dev/disk1s1", 2.0, now)
	c := mintConcept(t, s, "disk full", lived.Template, "noun")

	if _, err := s.Reground(0.8); err != nil {
		t.Fatal(err)
	}
	if edgeWeightRel(t, s, c.ID, lived.ID, RelGrounds) == 0 {
		t.Fatal("the lived moment should ground the concept")
	}
	if edgeWeightRel(t, s, c.ID, spoken.ID, RelGrounds) != 0 {
		t.Fatal("an utterance must not ground a concept")
	}
}

// A concept whose only groundings are utterances loses them at sleep and
// dies of disconnection the same night: the live-memory path out of the
// old greedy genesis.
func TestSleepRetiresUtteranceConcepts(t *testing.T) {
	s := openTestStore(t)
	now := time.Now()
	spoken := encodeAt(t, s, "tell", "what usually happens after make test", 2.0, now)
	old := mintConcept(t, s, "what usually happens after make test", spoken.Template, "noun")
	if err := s.LinkRel(old.ID, spoken.ID, RelGrounds, 0.8); err != nil {
		t.Fatal(err)
	}
	lived := encodeAt(t, s, "syslog", "Disk full on /dev/disk1s1", 3.0, now)
	keep := mintConcept(t, s, "disk full", lived.Template, "noun")
	if err := s.LinkRel(keep.ID, lived.ID, RelGrounds, 0.8); err != nil {
		t.Fatal(err)
	}

	if _, err := s.Sleep(0.10, 0.10); err != nil {
		t.Fatal(err)
	}
	frags, _ := s.Dump()
	for _, f := range frags {
		if f.ID == old.ID {
			t.Fatal("a concept grounded only by utterances should die at sleep")
		}
	}
	var kept bool
	for _, f := range frags {
		kept = kept || f.ID == keep.ID
	}
	if !kept {
		t.Fatal("a concept grounded by lived experience must survive")
	}
}
