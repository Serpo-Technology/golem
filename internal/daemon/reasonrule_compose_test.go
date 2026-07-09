package daemon

import (
	"testing"
	"time"

	"golem/internal/memory"
)

func sameSet(a, b map[uint64]bool) bool {
	if len(a) != len(b) {
		return false
	}
	for k := range a {
		if !b[k] {
			return false
		}
	}
	return true
}

// The one compositional evaluator reproduces the transitive-comparison walk from a rule that
// is pure data (a contains / set over a reach spec), across the chained, direct, and reverse
// pairs and the whole reached set, and then runs an inference NO handler provides: a
// single-hop reach (trans=no) yields the concepts directly exceeded, not the closure. The
// only difference between the transitive rule and the novel one is a stored leaf, which is
// the taught-rule plasticity the compositional form buys.
func TestCompositionalReachReproducesTransitivity(t *testing.T) {
	d, _ := testDaemon(t)
	scale, _, _ := teachSizeScale(t, d)
	cat := groundNoun(t, d, "cat")
	mouse := groundNoun(t, d, "mouse")
	snail := groundNoun(t, d, "snail")
	say(t, d, "the cat is bigger than the mouse")
	say(t, d, "the mouse is bigger than the snail")

	contains := d.ensureCompRule("rule:comp:trans:contains",
		compSpec{result: resContains, edge: bindRel, role: agentRole, trans: transYes})
	set := d.ensureCompRule("rule:comp:trans:set",
		compSpec{result: resSet, edge: bindRel, role: agentRole, trans: transYes})
	if contains == 0 || set == 0 {
		t.Fatal("the compositional transitivity rules should build")
	}

	for _, p := range []struct {
		from, to uint64
		label    string
	}{
		{cat.ID, snail.ID, "cat>snail (chained)"},
		{cat.ID, mouse.ID, "cat>mouse (direct)"},
		{snail.ID, cat.ID, "snail>cat (reverse)"},
	} {
		r, ok := d.evalRule(contains, ruleQuery{from: p.from, to: p.to, rel: scale.Content})
		if !ok || r.kind != resContains {
			t.Fatalf("%s: the contains rule should run", p.label)
		}
		if want := d.comparesTransitivelyGo(scale, p.from, p.to); r.hit != want {
			t.Fatalf("%s: compositional contains %v vs handler %v", p.label, r.hit, want)
		}
	}

	r, ok := d.evalRule(set, ruleQuery{from: cat.ID, rel: scale.Content})
	if !ok {
		t.Fatal("the set rule should run")
	}
	if want := d.comparativeReachGo(scale, cat.ID); !sameSet(sliceToSet(r.set), want) {
		t.Fatalf("compositional set %v disagrees with comparativeReach", r.set)
	}

	// The novel single-hop reach: an inference no handler exposes, differing from the transitive
	// rule only by the stored trans leaf. The cat directly exceeds the mouse, not the snail.
	novel := d.ensureCompRule("rule:comp:trans:direct",
		compSpec{result: resSet, edge: bindRel, role: agentRole, trans: transNo})
	nr, ok := d.evalRule(novel, ruleQuery{from: cat.ID, rel: scale.Content})
	if !ok {
		t.Fatal("the novel single-hop rule should run")
	}
	if len(nr.set) != 1 || nr.set[0] != mouse.ID {
		t.Fatalf("the direct-only reach should yield just the mouse, got %v", nr.set)
	}
}

// The switched-over handlers run the compositional rule, not the Go fallback: after each kind
// of reasoning query the rule for its template is ensured and cached, so a silent regression
// that dropped a handler back to its Go walk would be caught here even though it would return
// the same answer. This exercises the production spec constructors through the live routers.
func TestReasoningRoutesThroughCompositionalRules(t *testing.T) {
	d, s := testDaemon(t)

	// A comparison chains through the transitive contains rule.
	teachSizeScale(t, d)
	groundNoun(t, d, "cat")
	groundNoun(t, d, "mouse")
	say(t, d, "the cat is bigger than the mouse")
	say(t, d, "is the cat bigger than the mouse")
	if d.reasonRuleID[transContainsRuleTemplate] == 0 {
		t.Fatal("a comparison should route through the transitive contains rule")
	}

	// An inheritance guess runs the find rule.
	animal := &memory.Fragment{Kind: memory.KindConcept, Channel: "concept", Content: "animal",
		Template: "abs:en:animal", Category: "noun", Limbic: d.chem.Snapshot(), Time: time.Now()}
	if _, err := s.Encode(animal); err != nil {
		t.Fatal(err)
	}
	animalW, _, err := s.WordNode("animal", "en", d.chem.Snapshot(), time.Now())
	if err != nil {
		t.Fatal(err)
	}
	s.SetLinkRel(animalW.ID, animal.ID, memory.RelNames, 0.8)
	catID := d.resolveConcept("cat").ID
	s.SetLinkRel(animal.ID, catID, memory.RelGrounds, 0.5)
	say(t, d, "an animal chased the mouse")
	say(t, d, "did the cat chase the mouse")
	if d.reasonRuleID[inheritRuleTemplate] == 0 {
		t.Fatal("an inheritance guess should route through the find rule")
	}

	// A containment chain steps through the part-whole rule.
	groundNoun(t, d, "leaf")
	groundNoun(t, d, "tree")
	groundNoun(t, d, "forest")
	say(t, d, "the leaf is part of the tree")
	say(t, d, "the tree is in the forest")
	say(t, d, "is the leaf in the forest")
	if d.reasonRuleID[partWholeRuleTemplate] == 0 {
		t.Fatal("a containment chain should route through the part-whole rule")
	}
}

// The compositional evaluator reproduces category inheritance from a find rule that is pure
// data (a find over a transitive is-a reach, testing the queried relation at the patient): a
// cat that is an animal inherits the animal's chasing of the mouse, naming the same category
// the handler does, and a non-member inherits nothing.
func TestCompositionalRuleReproducesInheritance(t *testing.T) {
	d, s := testDaemon(t)
	cat := groundNoun(t, d, "cat")
	mouse := groundNoun(t, d, "mouse")
	rabbit := groundNoun(t, d, "rabbit")
	animal := &memory.Fragment{Kind: memory.KindConcept, Channel: "concept", Content: "animal",
		Template: "abs:en:animal", Category: "noun", Limbic: d.chem.Snapshot(), Time: time.Now()}
	if _, err := s.Encode(animal); err != nil {
		t.Fatal(err)
	}
	animalW, _, err := s.WordNode("animal", "en", d.chem.Snapshot(), time.Now())
	if err != nil {
		t.Fatal(err)
	}
	s.SetLinkRel(animalW.ID, animal.ID, memory.RelNames, 0.8)
	s.SetLinkRel(animal.ID, cat.ID, memory.RelGrounds, 0.5)
	say(t, d, "an animal chased the mouse")

	chase, err := s.ConceptByTemplate("rel:en:chase")
	if err != nil || chase == nil {
		t.Fatal("the chase relation should exist")
	}

	find := d.ensureCompRule("rule:comp:inherit", compSpec{result: resFind, edge: isAEdge,
		role: agentRole, trans: transYes, testRel: bindRel, testArg: bindPatient})
	r, ok := d.evalRule(find, ruleQuery{from: cat.ID, rel: chase.Content, patient: mouse.ID})
	if !ok || r.kind != resFind {
		t.Fatal("the find rule should run")
	}
	wantCat, wantFound := d.categoryInheritsGo(cat, chase.Content, mouse.ID)
	if r.hit != wantFound || r.id != wantCat {
		t.Fatalf("compositional find (%d,%v) vs handler (%d,%v)", r.id, r.hit, wantCat, wantFound)
	}
	if !r.hit || r.id != animal.ID {
		t.Fatalf("the cat should inherit from the animal (%d), got %d found %v", animal.ID, r.id, r.hit)
	}

	rr, _ := d.evalRule(find, ruleQuery{from: rabbit.ID, rel: chase.Content, patient: mouse.ID})
	if rr.hit {
		t.Fatal("the rabbit should inherit nothing")
	}
}

// The compositional evaluator reproduces the part-whole reach from a set rule that is pure
// data (a set over a single-hop, marker-filtered, any-relation reach): the wholes a leaf is a
// part of come back the same as the handler's.
func TestCompositionalRuleReproducesPartWhole(t *testing.T) {
	d, _ := testDaemon(t)
	leaf := groundNoun(t, d, "leaf")
	tree := groundNoun(t, d, "tree")
	say(t, d, "the leaf is part of the tree")

	set := d.ensureCompRule("rule:comp:partwhole", compSpec{result: resSet, edge: anyEdge,
		role: agentRole, trans: transNo, filter: partWholeMarker})
	r, ok := d.evalRule(set, ruleQuery{from: leaf.ID})
	if !ok {
		t.Fatal("the part-whole rule should run")
	}
	if want := sliceToSet(d.partWholesGo(leaf.ID)); !sameSet(sliceToSet(r.set), want) {
		t.Fatalf("compositional wholes %v disagree with partWholes", r.set)
	}
	if len(r.set) != 1 || r.set[0] != tree.ID {
		t.Fatalf("the leaf should be part of the tree (%d), got %v", tree.ID, r.set)
	}
}
