package daemon

import (
	"testing"

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
