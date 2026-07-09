package daemon

import (
	"testing"
	"time"

	"golem/internal/memory"
)

// The resemblance rule reproduces the Go weighted-max: the wolf resembles the dog (a shared
// predator feature), and the dog chased the cat, so the support for "the wolf chased the cat"
// rides on the dog at a positive, hedged confidence -- never a firm hit.
func TestResembleRuleReproducesGoWalk(t *testing.T) {
	d, s := testDaemon(t)
	dog := groundNoun(t, d, "dog")
	cat := groundNoun(t, d, "cat")
	wolf := groundNoun(t, d, "wolf")
	say(t, d, "a dog chased a cat")
	feat := &memory.Fragment{Kind: memory.KindFeature, Channel: "feature", Content: "predator",
		Template: "predator", Limbic: d.chem.Snapshot(), Time: time.Now()}
	if _, err := s.Encode(feat); err != nil {
		t.Fatal(err)
	}
	s.SetLinkRel(dog.ID, feat.ID, memory.RelFeature, 1.0)
	s.SetLinkRel(wolf.ID, feat.ID, memory.RelFeature, 1.0)

	root := d.resembleRule()
	if root == 0 {
		t.Fatal("the resemblance rule should be created")
	}

	r, ok := d.evalRule(root, ruleQuery{from: wolf.ID, patient: cat.ID, rel: "chase"})
	if !ok {
		t.Fatal("the resemblance rule should evaluate")
	}
	if r.lender != dog.ID {
		t.Fatalf("the guess should ride on the dog (the resembled neighbour): lender=%d dog=%d", r.lender, dog.ID)
	}
	if r.conf <= 0 {
		t.Fatalf("the resemblance support should be positive: %v", r.conf)
	}
	if r.hit {
		t.Fatal("a resemblance result must never be firm (hit): it is a hedged confidence")
	}

	// The rule reproduces the Go weighted-max on every threaded value.
	g, okG := d.resembleSupportGo(wolf, "chase", cat)
	if !okG || g.conf != r.conf || g.lender != r.lender || g.depth != r.depth || g.from != r.from || g.supposed != r.supposed {
		t.Fatalf("the rule and the Go walk should agree: rule=%+v go=%+v", r, g)
	}
}

// The guardrail: a $like (vector-grounded) reach can never produce a firm answer. A hand-authored
// firm rule (contains) over a $like reach yields no firm hit -- the sound reach refuses the edge,
// so a vector proposal cannot be laundered into a firm fact through a boolean result kind.
func TestResembleLikeReachCannotFirm(t *testing.T) {
	d, _ := testDaemon(t)
	dog := groundNoun(t, d, "dog")
	cat := groundNoun(t, d, "cat")
	say(t, d, "a dog chased a cat")

	root := d.ensureCompRule("test:guard:contains-like",
		compSpec{result: resContains, edge: bindLike, role: agentRole, trans: transNo})
	if root == 0 {
		t.Fatal("the guard rule should be created for the test")
	}
	r, ok := d.evalRule(root, ruleQuery{from: dog.ID, to: cat.ID, rel: "chase"})
	if ok && r.hit {
		t.Fatal("a $like reach must never yield a firm hit through a sound result kind")
	}
}

// The live resemblance guess runs through the compositional rule, not the Go fallback: a hedged
// answer populates the rule cache, which only the rule path writes.
func TestResembleRoutesThroughRule(t *testing.T) {
	d, s := testDaemon(t)
	dog := groundNoun(t, d, "dog")
	groundNoun(t, d, "cat")
	wolf := groundNoun(t, d, "wolf")
	say(t, d, "a dog chased a cat")
	feat := &memory.Fragment{Kind: memory.KindFeature, Channel: "feature", Content: "predator",
		Template: "predator", Limbic: d.chem.Snapshot(), Time: time.Now()}
	if _, err := s.Encode(feat); err != nil {
		t.Fatal(err)
	}
	s.SetLinkRel(dog.ID, feat.ID, memory.RelFeature, 1.0)
	s.SetLinkRel(wolf.ID, feat.ID, memory.RelFeature, 1.0)

	if got := say(t, d, "did the wolf chase the cat").Say; got != "Probably, the wolf chased the cat." {
		t.Fatalf("the resemblance guess should be hedged: %q", got)
	}
	if d.reasonRuleID[resembleRuleTemplate] == 0 {
		t.Fatal("the resemblance guess should run the compositional rule, populating its cache")
	}
}
