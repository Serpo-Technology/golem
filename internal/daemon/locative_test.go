package daemon

import (
	"strings"
	"testing"
)

// A locative relation is learned from its construction: "the leaf is on the tree"
// lays the place and marks the relation locative, so "where is the leaf" reads the
// place back as a copula with the preposition, the polar question affirms it, and
// the inverse role question names the figure on the ground.
func TestSpatialRelationFromConstruction(t *testing.T) {
	d, _ := testDaemon(t)
	groundNoun(t, d, "leaf")
	groundNoun(t, d, "tree")
	say(t, d, "the leaf is on the tree")

	where := say(t, d, "where is the leaf").Say
	t.Logf("where is the leaf -> %q", where)
	polar := say(t, d, "is the leaf on the tree").Say
	t.Logf("is the leaf on the tree -> %q", polar)
	what := say(t, d, "what is on the tree").Say
	t.Logf("what is on the tree -> %q", what)
	tell := say(t, d, "tell me about the leaf").Say
	t.Logf("tell me about the leaf -> %q", tell)

	if where != "The leaf is on the tree." {
		t.Errorf("where should read the figure's place: %q", where)
	}
	if polar != "Yes, the leaf is on the tree." {
		t.Errorf("the placed proposition should affirm: %q", polar)
	}
	if !strings.Contains(what, "leaf is on the tree") {
		t.Errorf("what-is-on should name the figure on the ground: %q", what)
	}
	if !strings.Contains(tell, "It is on the tree") {
		t.Errorf("a description should include the place: %q", tell)
	}
}

// A spatial relation does not collide with the is-a reading: "is a leaf a tree"
// (no preposition) still settles against the category hierarchy, and a relation
// is marked locative only by the construction, not by being a plain transitive.
func TestSpatialDoesNotBreakIsA(t *testing.T) {
	d, _ := testDaemon(t)
	groundNoun(t, d, "leaf")
	groundNoun(t, d, "tree")
	say(t, d, "the leaf is on the tree")
	if got := say(t, d, "is a leaf a tree").Say; !strings.HasPrefix(got, "No") {
		t.Errorf("a category question should still answer is-a: %q", got)
	}
	// A non-locative relation taught alongside is not marked locative.
	say(t, d, "the leaf covered the tree")
	if got := say(t, d, "where is the leaf").Say; !strings.Contains(got, "on the tree") {
		t.Errorf("where should read only the locative place, not the covered fact: %q", got)
	}
}
