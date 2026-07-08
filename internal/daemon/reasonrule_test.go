package daemon

import (
	"strings"
	"testing"
	"time"

	"golem/internal/memory"
)

// The transitive-closure rule stored in the graph reproduces the hardcoded chaining handler:
// over a told ordering (cat > mouse > snail on the size scale), the graph-walked evaluator
// agrees with comparesTransitively on the chained pair, the direct pair, the reverse (which
// must not hold), and an unrelated pair. The rule's role is read from the graph, so the walk
// is data; comparesTransitively stays the authoritative handler and is unchanged.
func TestTransitivityRuleReproducesHandler(t *testing.T) {
	d, _ := testDaemon(t)
	scale, _, _ := teachSizeScale(t, d)
	cat := groundNoun(t, d, "cat")
	mouse := groundNoun(t, d, "mouse")
	snail := groundNoun(t, d, "snail")
	lone := groundNoun(t, d, "rock")
	say(t, d, "the cat is bigger than the mouse")
	say(t, d, "the mouse is bigger than the snail")

	root := d.ensureTransitivityRule()
	if root == 0 {
		t.Fatal("the transitivity rule should materialize a durable subgraph")
	}
	// The rule is a code node holding a reach over the agent role.
	g := d.storeView()
	if g.content(firstDest(g, root, memory.RelGrounds)) != reachRule {
		t.Fatal("the rule root should ground in the reach primitive")
	}
	if role := g.content(firstDest(g, root, operand0Rel)); role != agentRole {
		t.Fatalf("the rule should follow the agent role, got %q", role)
	}

	pairs := []struct {
		from, to uint64
		label    string
	}{
		{cat.ID, snail.ID, "cat over snail (chained)"},
		{cat.ID, mouse.ID, "cat over mouse (direct)"},
		{snail.ID, cat.ID, "snail over cat (reverse)"},
		{cat.ID, lone.ID, "cat over rock (unrelated)"},
		{mouse.ID, snail.ID, "mouse over snail (direct)"},
	}
	for _, p := range pairs {
		want := d.comparesTransitively(scale, p.from, p.to)
		got, ok := d.evalReachRule(root, scale.Content, p.from, p.to)
		if !ok {
			t.Fatalf("%s: the rule evaluator should run", p.label)
		}
		if got != want {
			t.Fatalf("%s: graph rule %v disagrees with the handler %v", p.label, got, want)
		}
	}
	// Sanity: the chained pair genuinely holds through the graph-stored rule.
	if got, _ := d.evalReachRule(root, scale.Content, cat.ID, snail.ID); !got {
		t.Fatal("cat should reach snail transitively through the graph-stored rule")
	}
}

// The live comparison path runs the graph-stored rule: after a chained comparison query the
// transitivity rule has been ensured and cached, so comparesTransitively answered through the
// rule (evalReachRule), not only the Go fallback, and the answer is right.
func TestComparisonRunsThroughStoredRule(t *testing.T) {
	d, _ := testDaemon(t)
	teachSizeScale(t, d)
	groundNoun(t, d, "cat")
	groundNoun(t, d, "mouse")
	groundNoun(t, d, "snail")
	say(t, d, "the cat is bigger than the mouse")
	say(t, d, "the mouse is bigger than the snail")
	if got := say(t, d, "is the cat bigger than the snail").Say; got != "Yes, the cat is bigger than the snail." {
		t.Fatalf("the chained comparison should still hold through the rule: %q", got)
	}
	if d.transitivityRuleID == 0 {
		t.Fatal("the comparison should have ensured and cached the transitivity rule")
	}
	if got := d.reasonRuleByTemplate(transitiveRuleTemplate); got != d.transitivityRuleID {
		t.Fatalf("the cached rule id %d should match the stored rule %d", d.transitivityRuleID, got)
	}
}

// The inheritance rule stored in the graph reproduces the hardcoded composition, a second
// inference kind on the one rule interpreter: a cat that is an animal inherits the animal's
// chasing of the mouse (defeasibly), and the graph-walked evaluator agrees with
// categoryInheritsGo on the inheriting member and a non-member. The live inheritance query
// still fires the guess through the rule. categoryInheritsGo stays the authoritative handler.
func TestInheritanceRuleReproducesHandler(t *testing.T) {
	d, s := testDaemon(t)
	cat := groundNoun(t, d, "cat")
	mouse := groundNoun(t, d, "mouse")
	rabbit := groundNoun(t, d, "rabbit") // grounded, but belongs to no category with a fact
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
	s.SetLinkRel(animal.ID, cat.ID, memory.RelGrounds, 0.5) // the cat is an animal
	say(t, d, "an animal chased the mouse")                 // a category-level fact

	chase, err := s.ConceptByTemplate("rel:en:chase")
	if err != nil || chase == nil {
		t.Fatal("the chase relation should exist after the category fact")
	}

	root := d.ensureInheritanceRule()
	if root == 0 {
		t.Fatal("the inheritance rule should materialize a durable subgraph")
	}
	g := d.storeView()
	if g.content(firstDest(g, root, memory.RelGrounds)) != inheritRule {
		t.Fatal("the rule root should ground in the inherit primitive")
	}
	if edge := g.content(firstDest(g, root, operand0Rel)); edge != isAEdge {
		t.Fatalf("the rule should reach over is-a, got %q", edge)
	}

	// The cat inherits chasing the mouse from the animal; the rule agrees with the handler and
	// names the inheriting category.
	wantCat, wantFound := d.categoryInheritsGo(cat, chase.Content, mouse.ID)
	gotCat, gotFound, ok := d.evalInheritRule(root, chase.Content, cat.ID, mouse.ID)
	if !ok {
		t.Fatal("the inherit rule should run")
	}
	if gotFound != wantFound || gotCat != wantCat {
		t.Fatalf("rule (%d,%v) disagrees with handler (%d,%v)", gotCat, gotFound, wantCat, wantFound)
	}
	if !gotFound || gotCat != animal.ID {
		t.Fatalf("the cat should inherit from the animal (%d), got %d found %v", animal.ID, gotCat, gotFound)
	}

	// The rabbit belongs to no category with the fact, so it inherits nothing; the rule agrees.
	rc, rf, _ := d.evalInheritRule(root, chase.Content, rabbit.ID, mouse.ID)
	wc, wf := d.categoryInheritsGo(rabbit, chase.Content, mouse.ID)
	if rf != wf || rc != wc {
		t.Fatalf("rabbit: rule (%d,%v) vs handler (%d,%v)", rc, rf, wc, wf)
	}
	if rf {
		t.Fatal("the rabbit should inherit nothing")
	}

	// The live query fires the guess through the stored rule, which is then cached.
	if got := say(t, d, "did the cat chase the mouse").Say; got != "Probably, the cat chased the mouse." {
		t.Fatalf("the inheritance guess should fire through the rule: %q", got)
	}
	if d.inheritanceRuleID == 0 {
		t.Fatal("the query should have ensured and cached the inheritance rule")
	}
}

// The part-whole rule stored in the graph reproduces the hardcoded marker-filtered reach, a
// third rule shape on the one interpreter: the wholes a leaf is a part of come back the same
// from the graph-walked evaluator and partWholesGo, and the live containment query that steps
// through part-whole still composes. partWholesGo stays the authoritative handler.
func TestPartWholeRuleReproducesHandler(t *testing.T) {
	d, _ := testDaemon(t)
	leaf := groundNoun(t, d, "leaf")
	tree := groundNoun(t, d, "tree")
	forest := groundNoun(t, d, "forest")
	say(t, d, "the leaf is part of the tree")

	root := d.ensurePartWholeRule()
	if root == 0 {
		t.Fatal("the part-whole rule should materialize a durable subgraph")
	}
	g := d.storeView()
	if g.content(firstDest(g, root, memory.RelGrounds)) != partWholeRule {
		t.Fatal("the rule root should ground in the part-whole primitive")
	}

	// The leaf is part of the tree; the rule and the handler agree on the wholes.
	want := d.partWholesGo(leaf.ID)
	got, ok := d.evalPartWholeRule(root, leaf.ID)
	if !ok {
		t.Fatal("the part-whole rule should run")
	}
	if len(got) != len(want) || (len(want) == 1 && got[0] != want[0]) {
		t.Fatalf("rule wholes %v disagree with handler %v", got, want)
	}
	if len(got) != 1 || got[0] != tree.ID {
		t.Fatalf("the leaf should be part of the tree (%d), got %v", tree.ID, got)
	}

	// The forest is part of nothing here; the rule agrees (empty).
	if fw, _ := d.evalPartWholeRule(root, forest.ID); len(fw) != 0 {
		t.Fatalf("the forest should be part of nothing, got %v", fw)
	}

	// The live containment query steps through part-whole and composes, routed through the rule.
	say(t, d, "the tree is in the forest")
	if answer := say(t, d, "is the leaf in the forest").Say; !strings.HasPrefix(answer, "Yes") {
		t.Fatalf("a part of a thing in a place is in that place: %q", answer)
	}
	if d.partWholeRuleID == 0 {
		t.Fatal("the containment query should have ensured and cached the part-whole rule")
	}
}

// The rule is durable and generic: a second ensure returns the same root (no duplicate is
// minted), and it is still found and still reproduces the handler after a recompile, the
// reload path a real boot takes.
func TestTransitivityRuleDurableAndIdempotent(t *testing.T) {
	d, _ := testDaemon(t)
	scale, _, _ := teachSizeScale(t, d)
	cat := groundNoun(t, d, "cat")
	groundNoun(t, d, "mouse")
	snail := groundNoun(t, d, "snail")
	say(t, d, "the cat is bigger than the mouse")
	say(t, d, "the mouse is bigger than the snail")

	root := d.ensureTransitivityRule()
	if root == 0 {
		t.Fatal("the rule should materialize")
	}
	if again := d.ensureTransitivityRule(); again != root {
		t.Fatalf("ensure should be idempotent: first %d, second %d", root, again)
	}
	if err := d.recompileGrammar(); err != nil {
		t.Fatal(err)
	}
	if found := d.reasonRuleByTemplate(transitiveRuleTemplate); found != root {
		t.Fatalf("the rule should survive a recompile: was %d, found %d", root, found)
	}
	got, ok := d.evalReachRule(root, scale.Content, cat.ID, snail.ID)
	if !ok || !got {
		t.Fatalf("the reloaded rule should still chain cat over snail (ok %v, holds %v)", ok, got)
	}
}
