package daemon

import (
	"testing"
	"time"

	"golem/internal/memory"
)

// anchor gives a concept a lived perceptual episode so it survives a night's
// decay the way a concept grounded in real experience would, rather than dying
// of disconnection as a bare test concept.
func anchor(t *testing.T, d *Daemon, c *memory.Fragment) {
	t.Helper()
	e := &memory.Fragment{Kind: memory.KindEpisodic, Channel: "vision",
		Content: c.Content, Template: c.Template, Limbic: d.chem.Snapshot(), Time: time.Now(), Weight: 1}
	if _, err := d.store.Encode(e); err != nil {
		t.Fatal(err)
	}
	if err := d.store.LinkRel(c.ID, e.ID, memory.RelGrounds, groundsWeight); err != nil {
		t.Fatal(err)
	}
}

// constructionNode returns the construction node with the given key, or nil.
func constructionNode(t *testing.T, s *memory.Store, template string) *memory.Fragment {
	t.Helper()
	nodes, err := s.NodesByKind(memory.KindConstruction)
	if err != nil {
		t.Fatal(err)
	}
	for _, n := range nodes {
		if n.Template == template {
			return n
		}
	}
	return nil
}

// A relation told in several propositions, its slots filled by varied
// concepts, induces an argument-structure construction grown from that usage:
// a construction node, on one footing with the seed and entrenched above it by
// the recurrence.
func TestInduceConstructionFromUsage(t *testing.T) {
	d, s := testDaemon(t)
	for _, n := range []string{"dog", "cat", "fox", "mouse", "ball"} {
		groundNoun(t, d, n)
	}
	say(t, d, "a dog chased a cat")
	say(t, d, "a fox chased a mouse")
	say(t, d, "a dog chased a ball")

	if born := d.growConstructions(); born == 0 {
		t.Fatal("a relation told with varied fillers should induce a construction")
	}
	node := constructionNode(t, s, "acxn:en:chase")
	if node == nil {
		t.Fatal("the induced construction should be a construction node")
	}
	if node.Content != "agent/np rel/v patient/np" {
		t.Fatalf("the induced construction should be transitive: %q", node.Content)
	}
	if node.Weight <= 1.0 {
		t.Fatalf("usage should entrench the induced construction above the seed: %v", node.Weight)
	}
}

// A slot generalizes only when varied fillers occupy it: a relation told with
// the same agent and patient every time has shown no open position, so no
// construction is induced yet.
func TestInductionWaitsForGeneralizedSlot(t *testing.T) {
	d, s := testDaemon(t)
	groundNoun(t, d, "dog")
	groundNoun(t, d, "cat")
	say(t, d, "a dog chased a cat")
	say(t, d, "a dog chased a cat")
	say(t, d, "a dog chased a cat")

	d.growConstructions()
	if constructionNode(t, s, "acxn:en:chase") != nil {
		t.Fatal("one filler per slot is not a generalized construction")
	}
}

// A comprehended proposition persists through a night, a narrated event being
// a real instance of its relation: consolidation keeps the relation grounded
// in the moment it was told in rather than purging it as it does a thing
// spuriously grounded in an utterance.
func TestPropositionSurvivesConsolidation(t *testing.T) {
	d, s := testDaemon(t)
	anchor(t, d, groundNoun(t, d, "dog"))
	anchor(t, d, groundNoun(t, d, "cat"))
	say(t, d, "a dog chased a cat")

	d.sleepCycle()

	chase, _ := s.ConceptByTemplate("rel:en:chase")
	if chase == nil {
		t.Fatal("the relation should survive consolidation as a real instance")
	}
	if g, _ := s.LinksFromRel([]uint64{chase.ID}, memory.RelGrounds); len(g) == 0 {
		t.Fatal("the proposition should still ground the relation after a night")
	}
}

// Over a real night, varied tellings of a relation induce its construction:
// consolidation preserves the propositions and the induction pass grows the
// construction from them, the whole nightly path rather than a direct call.
func TestInductionAcrossConsolidation(t *testing.T) {
	d, s := testDaemon(t)
	for _, n := range []string{"dog", "cat", "fox", "mouse", "ball"} {
		anchor(t, d, groundNoun(t, d, n))
	}
	say(t, d, "a dog chased a cat")
	say(t, d, "a fox chased a mouse")
	say(t, d, "a dog chased a ball")

	d.sleepCycle()

	if constructionNode(t, s, "acxn:en:chase") == nil {
		t.Fatal("a night over varied usage should induce the construction")
	}
}

// The induced construction can carry the load the seed used to: with the
// seeded transitive removed, a brand-new transitive clause still comprehends
// through the construction grown from usage, the measure of the seed being
// superseded rather than load-bearing forever.
func TestInducedConstructionSupersedesSeed(t *testing.T) {
	d, s := testDaemon(t)
	dog := groundNoun(t, d, "dog")
	ball := groundNoun(t, d, "ball")
	for _, n := range []string{"cat", "fox", "mouse", "wolf"} {
		groundNoun(t, d, n)
	}
	say(t, d, "a dog chased a cat")
	say(t, d, "a fox chased a mouse")
	say(t, d, "a wolf chased a ball")
	d.growConstructions()

	if _, err := s.DeleteByTemplate("acxn:en:transitive"); err != nil {
		t.Fatal(err)
	}
	if err := d.recompileGrammar(); err != nil {
		t.Fatal(err)
	}

	// A transitive clause with a verb never heard before still parses into a
	// proposition, through the induced construction.
	resp := say(t, d, "a dog fetched a ball")
	moment := resp.Observe.FragmentID
	if fetch, _ := s.ConceptByTemplate("rel:en:fetch"); fetch == nil {
		t.Fatal("the induced construction should still comprehend a new transitive after the seed is gone")
	}
	ag, _ := s.LinksFromRel([]uint64{moment}, memory.RelAgent)
	if !linksTo(ag, dog.ID) {
		t.Fatalf("the dog should bind as agent through the induced construction: %+v", ag)
	}
	pat, _ := s.LinksFromRel([]uint64{moment}, memory.RelPatient)
	if !linksTo(pat, ball.ID) {
		t.Fatalf("the ball should bind as patient through the induced construction: %+v", pat)
	}
}
