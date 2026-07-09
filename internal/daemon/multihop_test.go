package daemon

import (
	"reflect"
	"strings"
	"testing"

	"golem/internal/memory"
)

// The nested-query rule reproduces the Go walk: who chased what the dog chased names the dog and
// the mouse (the dog chased the cat, and the cat was chased by both), and the focus it leaves for
// a following pronoun is the cat (what the dog chased).
func TestMultiHopRuleReproducesGoWalk(t *testing.T) {
	d, _ := testDaemon(t)
	dog := groundNoun(t, d, "dog")
	cat := groundNoun(t, d, "cat")
	mouse := groundNoun(t, d, "mouse")
	say(t, d, "the dog chased the cat")
	say(t, d, "the mouse chased the cat")

	root := d.multiHopRule()
	if root == 0 {
		t.Fatal("the multi-hop rule should be created")
	}

	r, ok := d.evalRule(root, ruleQuery{from: dog.ID, rel: "chase", rel2: "chase"})
	if !ok {
		t.Fatal("the rule should evaluate")
	}
	if !reflect.DeepEqual(sliceToSet(r.set), map[uint64]bool{dog.ID: true, mouse.ID: true}) {
		t.Fatalf("the agents that chased what the dog chased should be the dog and the mouse: %v", r.set)
	}

	factsRule, focusRule, okR := d.multiHopReach(dog.ID, "chase", "chase")
	factsGo, focusGo, okG := d.multiHopReachGo(dog.ID, "chase", "chase")
	if !okR || !okG || focusRule != focusGo || focusRule != cat.ID {
		t.Fatalf("focus should be the cat (what the dog chased): rule=%d go=%d cat=%d", focusRule, focusGo, cat.ID)
	}
	// Same agents AND same glosses: the rebuilt gloss must match the walk's co-argument gloss so
	// an unnamed agent's borrowed name renders identically on both paths.
	if !reflect.DeepEqual(factGlosses(factsRule), factGlosses(factsGo)) {
		t.Fatalf("the rule and the Go walk should reach the same agents and glosses: %v vs %v", factsRule, factsGo)
	}
}

// factGlosses maps each fact's co-argument id to its gloss, for comparing the rule path's rebuilt
// glosses against the Go walk's.
func factGlosses(facts []focusFact) map[uint64]string {
	m := map[uint64]string{}
	for _, f := range facts {
		m[f.coID] = f.coGloss
	}
	return m
}

// The two hops may run over DIFFERENT relations ($rel2 the outer): who feared what the dog chased
// is the wolf, though the dog never feared and the wolf never chased. The rule reads the outer
// relation from its own binding, so it composes across two relations, not just one chained.
func TestMultiHopComposesTwoRelations(t *testing.T) {
	d, _ := testDaemon(t)
	dog := groundNoun(t, d, "dog")
	cat := groundNoun(t, d, "cat")
	wolf := groundNoun(t, d, "wolf")
	say(t, d, "the dog chased the cat")
	say(t, d, "the wolf feared the cat")

	relOf := func(agent *memory.Fragment) string {
		facts := d.gatherFacts(agent, agentRole, "")
		if len(facts) == 0 {
			t.Fatalf("no stored fact for %q", agent.Content)
		}
		return facts[0].rel
	}
	innerRel, outerRel := relOf(dog), relOf(wolf)
	if innerRel == outerRel {
		t.Fatalf("the inner and outer relations should differ: %q", innerRel)
	}

	factsRule, _, ok := d.multiHopReach(dog.ID, innerRel, outerRel)
	if !ok {
		t.Fatal("the rule should evaluate")
	}
	if !reflect.DeepEqual(coIDSet(factsRule), map[uint64]bool{wolf.ID: true}) {
		t.Fatalf("only the wolf feared what the dog chased: %v", factsRule)
	}
	factsGo, _, _ := d.multiHopReachGo(dog.ID, innerRel, outerRel)
	if !reflect.DeepEqual(coIDSet(factsRule), coIDSet(factsGo)) {
		t.Fatalf("the rule and the Go walk should agree across two relations: %v vs %v", factsRule, factsGo)
	}
	_ = cat
}

// The live nested-query answer runs through the compositional rule, not the Go fallback: the
// question answers AND populates the rule cache, which only the rule path writes.
func TestMultiHopRoutesThroughRule(t *testing.T) {
	d, _ := testDaemon(t)
	groundNoun(t, d, "dog")
	groundNoun(t, d, "cat")
	groundNoun(t, d, "mouse")
	say(t, d, "the dog chased the cat")
	say(t, d, "the mouse chased the cat")

	got := say(t, d, "who chased what the dog chased").Say
	if !strings.Contains(got, "dog") || !strings.Contains(got, "mouse") {
		t.Fatalf("the nested query should name the dog and the mouse: %q", got)
	}
	if d.reasonRuleID[multiHopRuleTemplate] == 0 {
		t.Fatal("the nested query should run the compositional rule, populating its cache")
	}
}
