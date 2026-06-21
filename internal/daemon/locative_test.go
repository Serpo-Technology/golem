package daemon

import (
	"strings"
	"testing"
	"time"

	"golem/internal/memory"
)

// groundBilingual grounds one concept named in both tongues, so a Turkish clause
// and its English form resolve to the same referents.
func groundBilingual(t *testing.T, d *Daemon, gloss, en, tr string) *memory.Fragment {
	t.Helper()
	now := time.Now()
	c := &memory.Fragment{Kind: memory.KindConcept, Channel: "concept", Content: gloss,
		Template: gloss, Category: "noun", Limbic: d.chem.Snapshot(), Time: now}
	if _, err := d.store.Encode(c); err != nil {
		t.Fatal(err)
	}
	for _, p := range []struct{ w, lang string }{{en, "en"}, {tr, "tr"}} {
		w, _, err := d.store.WordNode(p.w, p.lang, d.chem.Snapshot(), now)
		if err != nil {
			t.Fatal(err)
		}
		if err := d.store.SetLinkRel(w.ID, c.ID, memory.RelNames, 0.9); err != nil {
			t.Fatal(err)
		}
	}
	return c
}

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

// A Turkish spatial postposition names a precise place over the genitive ground
// ("yaprak ağacın üstünde") and joins the relation its English preposition does, so
// "where" answers with the postposition in Turkish and the preposition in English.
func TestTurkishSpatialPostposition(t *testing.T) {
	d, _ := testDaemon(t)
	groundBilingual(t, d, "leaf", "leaf", "yaprak")
	groundBilingual(t, d, "tree", "tree", "ağaç")
	say(t, d, "the leaf is on the tree")
	sayLang(t, d, "tr", "yaprak ağacın üstünde")

	tr := sayLang(t, d, "tr", "yaprak nerede").Say
	t.Logf("yaprak nerede -> %q", tr)
	en := say(t, d, "where is the leaf").Say
	t.Logf("where is the leaf -> %q", en)

	if tr != "Yaprak ağacın üstünde." {
		t.Errorf("Turkish where should read the place as a postposition: %q", tr)
	}
	if !strings.Contains(en, "on the tree") {
		t.Errorf("the unified relation should answer in English: %q", en)
	}
}

// The three postpositions map to on/in/under, bidirectionally: a Turkish place taught
// with a postposition answers the English preposition's query, the in/under cases
// staying distinct rather than collapsing to the general locative.
func TestTurkishPostpositionsDistinct(t *testing.T) {
	d, _ := testDaemon(t)
	groundBilingual(t, d, "apple", "apple", "elma")
	groundBilingual(t, d, "cat", "cat", "kedi")
	groundBilingual(t, d, "tree", "tree", "ağaç")
	sayLang(t, d, "tr", "elma ağacın içinde")
	sayLang(t, d, "tr", "kedi ağacın altında")

	if got := sayLang(t, d, "tr", "elma nerede").Say; got != "Elma ağacın içinde." {
		t.Errorf("in should read as içinde: %q", got)
	}
	if got := sayLang(t, d, "tr", "kedi nerede").Say; got != "Kedi ağacın altında." {
		t.Errorf("under should read as altında: %q", got)
	}
	// The Turkish-taught places answer the English preposition's polar question, and
	// in and under stay distinct (the apple is in, not under).
	if got := say(t, d, "is the apple in the tree").Say; !strings.HasPrefix(got, "Yes") {
		t.Errorf("a Turkish-taught 'in' should affirm the English query: %q", got)
	}
	if got := say(t, d, "is the apple under the tree").Say; !strings.HasPrefix(got, "No") {
		t.Errorf("in and under must stay distinct: %q", got)
	}
}

// Turkish locative comprehension works without an English priming: a bare place
// clause mints a general Turkish locative and "nerede" reads it back.
func TestTurkishSpatialStandalone(t *testing.T) {
	d, _ := testDaemon(t)
	groundBilingual(t, d, "cat", "cat", "kedi")
	groundBilingual(t, d, "box", "box", "kutu")
	sayLang(t, d, "tr", "kedi kutuda")
	if got := sayLang(t, d, "tr", "kedi nerede").Say; got != "Kedi kutuda." {
		t.Errorf("a standalone Turkish locative should read back: %q", got)
	}
}
