package daemon

import (
	"testing"
	"time"

	"golem/internal/memory"
)

// groundNoun seeds a concept and a word that names it, so the planner's
// resolver reaches the concept from its surface form the way it would after
// the concept distilled from lived experience.
func groundNoun(t *testing.T, d *Daemon, gloss string) *memory.Fragment {
	t.Helper()
	now := time.Now()
	c := &memory.Fragment{Kind: memory.KindConcept, Channel: "concept", Content: gloss,
		Template: gloss, Category: "noun", Limbic: d.chem.Snapshot(), Time: now}
	if _, err := d.store.Encode(c); err != nil {
		t.Fatal(err)
	}
	w, _, err := d.store.WordNode(gloss, "en", d.chem.Snapshot(), now)
	if err != nil {
		t.Fatal(err)
	}
	if err := d.store.SetLinkRel(w.ID, c.ID, memory.RelNames, 0.9); err != nil {
		t.Fatal(err)
	}
	return c
}

// The done-when of the comprehension producer: "a dog chased a cat," told to
// a brain that has grounded dog and cat, parses into chaser and chased and is
// written as a proposition. The relation is minted and grounds in the moment
// of its telling, and the moment points at the dog as agent and the cat as
// patient.
func TestComprehendWritesProposition(t *testing.T) {
	d, s := testDaemon(t)
	dog := groundNoun(t, d, "dog")
	cat := groundNoun(t, d, "cat")

	resp := say(t, d, "a dog chased a cat")
	moment := resp.Observe.FragmentID
	if moment == 0 {
		t.Fatal("the telling should leave a moment")
	}

	chase, err := s.ConceptByTemplate("rel:en:chase")
	if err != nil {
		t.Fatal(err)
	}
	if chase == nil {
		t.Fatal("the verb should mint a relation concept")
	}
	if chase.Category != "relation" {
		t.Fatalf("chase should be a relation, got category %q", chase.Category)
	}

	grounds, _ := s.LinksFromRel([]uint64{chase.ID}, memory.RelGrounds)
	if !linksTo(grounds, moment) {
		t.Fatalf("chase should ground in the telling moment: %+v", grounds)
	}
	ag, _ := s.LinksFromRel([]uint64{moment}, memory.RelAgent)
	if !linksTo(ag, dog.ID) {
		t.Fatalf("the dog should be the agent: %+v", ag)
	}
	pat, _ := s.LinksFromRel([]uint64{moment}, memory.RelPatient)
	if !linksTo(pat, cat.ID) {
		t.Fatalf("the cat should be the patient: %+v", pat)
	}
}

// Once the proposition exists, the walk that crosses a relation reaches the
// co-argument: from the dog to the cat it chased, the dormant crossing lit.
func TestComprehendLightsWalkCrossing(t *testing.T) {
	d, _ := testDaemon(t)
	dog := groundNoun(t, d, "dog")
	cat := groundNoun(t, d, "cat")
	say(t, d, "a dog chased a cat")

	dests, err := d.walkableDests(dog.ID)
	if err != nil {
		t.Fatal(err)
	}
	crossed := false
	for _, dst := range dests[memory.RelPatient] {
		if dst.id == cat.ID {
			crossed = true
			if dst.via != "chase" {
				t.Fatalf("the crossing should name the relation, got %q", dst.via)
			}
		}
	}
	if !crossed {
		t.Fatal("the walk from the dog should cross to the cat it chased")
	}
}

// A second relation taught the same way stands apart from the first, so the
// brain accrues distinct relations rather than collapsing them into one.
func TestComprehendDistinctRelations(t *testing.T) {
	d, s := testDaemon(t)
	groundNoun(t, d, "dog")
	groundNoun(t, d, "cat")
	groundNoun(t, d, "ball")
	say(t, d, "a dog chased a cat")
	say(t, d, "a dog fetched a ball")

	chase, _ := s.ConceptByTemplate("rel:en:chase")
	fetch, _ := s.ConceptByTemplate("rel:en:fetch")
	if chase == nil || fetch == nil {
		t.Fatal("both verbs should mint relations")
	}
	if chase.ID == fetch.ID {
		t.Fatal("the two relations should be distinct concepts")
	}
}

// A novel verb degrades gracefully rather than failing to parse: an unknown
// verb in transitive position becomes a candidate relation with its slots
// bound, and the brain does not blurt a confused question about it.
func TestComprehendUnknownVerbDegradesGracefully(t *testing.T) {
	d, s := testDaemon(t)
	dog := groundNoun(t, d, "dog")
	cat := groundNoun(t, d, "cat")

	resp := say(t, d, "a dog blorped a cat")
	if resp.Say != "" {
		t.Fatalf("an understood novel statement should not blurt: %q", resp.Say)
	}
	blorp, _ := s.ConceptByTemplate("rel:en:blorp")
	if blorp == nil || blorp.Category != "relation" {
		t.Fatal("an unknown verb should become a candidate relation")
	}
	moment := resp.Observe.FragmentID
	ag, _ := s.LinksFromRel([]uint64{moment}, memory.RelAgent)
	pat, _ := s.LinksFromRel([]uint64{moment}, memory.RelPatient)
	if !linksTo(ag, dog.ID) || !linksTo(pat, cat.ID) {
		t.Fatalf("the unknown verb should still leave its slots bound: agent %+v patient %+v", ag, pat)
	}
}

// A partial utterance yields a partial meaning and a learning signal: the
// grounded part is written, and the role whose filler grounds in nothing is
// held as a goal rather than dropped.
func TestComprehendPartialHoldsTheGap(t *testing.T) {
	d, s := testDaemon(t)
	dog := groundNoun(t, d, "dog")
	d.chem.SpikeDopamine(0.6) // curious enough to hold the gap

	resp := say(t, d, "a dog chased a wug")
	moment := resp.Observe.FragmentID

	chase, _ := s.ConceptByTemplate("rel:en:chase")
	if chase == nil {
		t.Fatal("the grounded part of the proposition should still be written")
	}
	ag, _ := s.LinksFromRel([]uint64{moment}, memory.RelAgent)
	if !linksTo(ag, dog.ID) {
		t.Fatal("the dog should be the agent of the partial proposition")
	}
	if d.goal == nil || d.goal.topic != "wug" {
		t.Fatalf("the unknown patient should be held as a learning goal: %+v", d.goal)
	}
}

// linksTo reports whether any edge points at the given target.
func linksTo(edges []memory.LinkEdge, to uint64) bool {
	for _, e := range edges {
		if e.To == to {
			return true
		}
	}
	return false
}
