package daemon

import (
	"reflect"
	"testing"
)

// coIDSet collects the co-argument ids of a walk's focus facts, deduped, for comparing the
// compositional analogy rule's reached set against the Go walk's fourth terms.
func coIDSet(facts []focusFact) map[uint64]bool {
	m := map[uint64]bool{}
	for _, f := range facts {
		m[f.coID] = true
	}
	return m
}

// The compositional analogy rule reproduces the Go walk over the base fixture: the relation
// that holds cat->mouse is chase, so the dog (which chased the bird) is to the bird. A pair
// with no relation between its first two terms is admitted (hit false), not forced.
func TestAnalogyRuleReproducesGoWalk(t *testing.T) {
	d, _ := testDaemon(t)
	cat := groundNoun(t, d, "cat")
	mouse := groundNoun(t, d, "mouse")
	dog := groundNoun(t, d, "dog")
	bird := groundNoun(t, d, "bird")
	say(t, d, "the cat chased the mouse")
	say(t, d, "the dog chased the bird")

	root := d.analogyRule()
	if root == 0 {
		t.Fatal("the analogy rule should be created")
	}

	r, ok := d.evalRule(root, ruleQuery{from: cat.ID, to: mouse.ID, third: dog.ID})
	if !ok || !r.hit || r.foundRel != "chase" {
		t.Fatalf("the rule should find chase and the fourth term: ok=%v hit=%v rel=%q", ok, r.hit, r.foundRel)
	}
	relGo, ansGo, okGo := d.analogizeGo(cat, mouse, dog)
	if !okGo || relGo != r.foundRel {
		t.Fatalf("the Go walk should agree on the relation: %q vs %q", relGo, r.foundRel)
	}
	if !reflect.DeepEqual(sliceToSet(r.set), coIDSet(ansGo)) {
		t.Fatalf("the rule's reached set should equal the Go walk's fourth terms: %v vs %v", r.set, ansGo)
	}
	if !sliceToSet(r.set)[bird.ID] {
		t.Fatalf("the fourth term should be the bird: %v", r.set)
	}

	// No relation holds bird->dog, so the proportion has no fourth term: the rule admits it.
	r2, ok := d.evalRule(root, ruleQuery{from: bird.ID, to: dog.ID, third: cat.ID})
	if !ok || r2.hit {
		t.Fatalf("bird->dog has no relation, the rule should admit the gap: ok=%v hit=%v", ok, r2.hit)
	}
	if _, _, okGo := d.analogizeGo(bird, dog, cat); okGo {
		t.Fatal("the Go walk should also admit the gap")
	}
}

// The multi-candidate distinction: a->b may hold via several relations, and the rule returns
// the fourth term of the FIRST relation the third concept also stands in, not the first a->b
// relation and not a union. The cat both chased and feared the mouse, but the dog only feared
// the bird (it chased nothing), so the productive relation is fear, yielding just the bird.
func TestAnalogyRuleTriesEveryConnectingRelation(t *testing.T) {
	d, _ := testDaemon(t)
	cat := groundNoun(t, d, "cat")
	mouse := groundNoun(t, d, "mouse")
	dog := groundNoun(t, d, "dog")
	bird := groundNoun(t, d, "bird")
	say(t, d, "the cat chased the mouse")
	say(t, d, "the cat feared the mouse") // cat->mouse via a SECOND relation
	say(t, d, "the dog feared the bird")  // the dog stands only in the second relation

	// The Go walk picks the productive (second) relation and fourth term; the rule must match
	// it, and must NOT pick the first candidate chase (which the dog does not stand in).
	relGo, ansGo, okGo := d.analogizeGo(cat, mouse, dog)
	if !okGo || relGo == "chase" || !coIDSet(ansGo)[bird.ID] {
		t.Fatalf("the Go walk should skip chase and pick the fear -> bird relation: rel=%q ans=%+v", relGo, ansGo)
	}

	root := d.analogyRule()
	r, ok := d.evalRule(root, ruleQuery{from: cat.ID, to: mouse.ID, third: dog.ID})
	if !ok || !r.hit {
		t.Fatalf("the rule should find a productive relation: ok=%v hit=%v", ok, r.hit)
	}
	if r.foundRel == "chase" {
		t.Fatalf("the rule must skip the first candidate chase (the dog chased nothing): %q", r.foundRel)
	}
	if r.foundRel != relGo {
		t.Fatalf("the rule and the Go walk should pick the same relation: %q vs %q", r.foundRel, relGo)
	}
	if !reflect.DeepEqual(sliceToSet(r.set), map[uint64]bool{bird.ID: true}) {
		t.Fatalf("only the bird (the dog feared the bird), not a union: %v", r.set)
	}
	if !reflect.DeepEqual(sliceToSet(r.set), coIDSet(ansGo)) {
		t.Fatalf("the rule and the Go walk should reach the same set: %v vs %v", r.set, ansGo)
	}
}

// A reflexive fourth term is kept: when the connecting relation lands the third concept on
// itself (the dog chased the dog), that self co-argument is a valid answer, matching the Go
// walk. The apply reach keeps its start, so the rule does not drop it to a don't-know gap.
func TestAnalogyKeepsReflexiveFourthTerm(t *testing.T) {
	d, _ := testDaemon(t)
	cat := groundNoun(t, d, "cat")
	mouse := groundNoun(t, d, "mouse")
	dog := groundNoun(t, d, "dog")
	say(t, d, "the cat chased the mouse")
	say(t, d, "the dog chased the dog") // the dog stands in chase only to itself

	root := d.analogyRule()
	r, ok := d.evalRule(root, ruleQuery{from: cat.ID, to: mouse.ID, third: dog.ID})
	if !ok || !r.hit {
		t.Fatalf("the reflexive fourth term (the dog) should carry through, not a gap: ok=%v hit=%v", ok, r.hit)
	}
	if !reflect.DeepEqual(sliceToSet(r.set), map[uint64]bool{dog.ID: true}) {
		t.Fatalf("the fourth term should be the dog itself: %v", r.set)
	}

	// The rule agrees with the Go walk it replaced (which counts the self co-argument).
	relGo, ansGo, okGo := d.analogizeGo(cat, mouse, dog)
	if !okGo || relGo != r.foundRel || !reflect.DeepEqual(sliceToSet(r.set), coIDSet(ansGo)) {
		t.Fatalf("the rule and the Go walk should agree on the reflexive case: rel %q/%q set %v/%v", r.foundRel, relGo, r.set, ansGo)
	}
}

// The live analogy answer runs through the compositional rule, not the Go fallback: asking a
// proportion answers it AND populates the rule cache, which only the rule path writes.
func TestAnalogyRoutesThroughCompositionalRule(t *testing.T) {
	d, _ := testDaemon(t)
	groundNoun(t, d, "cat")
	groundNoun(t, d, "mouse")
	groundNoun(t, d, "dog")
	groundNoun(t, d, "bird")
	say(t, d, "the cat chased the mouse")
	say(t, d, "the dog chased the bird")

	if got := say(t, d, "the cat is to the mouse as the dog is to what").Say; got != "The bird." {
		t.Fatalf("the proportion should yield the bird through the rule: %q", got)
	}
	if d.reasonRuleID[analogyRuleTemplate] == 0 {
		t.Fatal("the analogy answer should run the compositional rule, populating its cache")
	}
}
