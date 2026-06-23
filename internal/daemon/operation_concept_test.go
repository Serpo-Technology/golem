package daemon

import (
	"strings"
	"testing"
)

// An operation is a grounded concept: a seeded primitive is an op:<name> concept in
// the operation category, a system concept kept out of the grown listing, and which
// operation a word names is resolved through the graph, the English word map gone.
func TestOperationsAreGroundedConcepts(t *testing.T) {
	d, _ := testDaemon(t)
	c, err := d.store.ConceptByTemplate("op:add")
	if err != nil || c == nil {
		t.Fatalf("the add primitive should be a concept op:add: %v", err)
	}
	if c.Category != "operation" {
		t.Errorf("an operation concept belongs to the operation category, got %q", c.Category)
	}
	if !d.isSystemConcept(c.ID) {
		t.Error("an operation concept should be a system concept, out of the grown listing")
	}
	for _, w := range []string{"sum", "plus"} {
		if ref, ok := d.resolveOperation(w); !ok || ref.Name != "add" {
			t.Errorf("resolveOperation(%q) should name add, got %+v ok=%v", w, ref, ok)
		}
	}
	if _, ok := d.resolveOperation("dog"); ok {
		t.Error("a non-operation word should not resolve to an operation")
	}
}

// An extremal is a grounded concept like a comparison, kept in its own namespace:
// op:max/op:min are system concepts, "greatest" resolves to the maximum turning on
// the same symbol the gt comparison carries, and it reads as neither a call head nor
// a conditional relator.
func TestExtremalsAreGroundedConcepts(t *testing.T) {
	d, _ := testDaemon(t)
	for _, tpl := range []string{"op:max", "op:min"} {
		c, err := d.store.ConceptByTemplate(tpl)
		if err != nil || c == nil {
			t.Fatalf("%s should be a concept: %v", tpl, err)
		}
		if !d.isSystemConcept(c.ID) {
			t.Errorf("%s should be a system concept, out of the grown listing", tpl)
		}
	}
	gt, _ := d.cmpSymbolByName("gt")
	if sym, ok := d.extremalRelator("greatest"); !ok || sym != gt {
		t.Errorf("greatest should turn on the gt symbol %q, got %q ok=%v", gt, sym, ok)
	}
	lt, _ := d.cmpSymbolByName("lt")
	if sym, ok := d.extremalRelator("smallest"); !ok || sym != lt {
		t.Errorf("smallest should turn on the lt symbol %q, got %q ok=%v", lt, sym, ok)
	}
	if _, ok := d.extremalRelator("dog"); ok {
		t.Error("a non-extremal word should not resolve to an extremal")
	}
	// No cross: an extremal word reads as neither a call head nor a conditional relator.
	if _, _, ok := d.opHead("greatest"); ok {
		t.Error("greatest must not read as a call head")
	}
	if _, ok := d.cmpRelator("greatest"); ok {
		t.Error("greatest must not read as a conditional relator")
	}
}

// A learned function becomes an operation concept the moment it is taught, so it
// resolves and survives a reload, closing the gap where the operation lived only in
// the engine's word map with no concept.
func TestLearnedFunctionMintsOperationConcept(t *testing.T) {
	d, _ := testDaemon(t)
	d.cfg.RuntimeEnabled = true
	say(t, d, "the triple of a number is the product of the number and 3")
	if c, err := d.store.ConceptByTemplate("op:triple"); err != nil || c == nil {
		t.Fatalf("a learned function should mint its operation concept: %v", err)
	}
	if ref, ok := d.resolveOperation("triple"); !ok || ref.Name != "triple" {
		t.Fatalf("triple should resolve to its operation: %+v ok=%v", ref, ok)
	}
	if err := d.recompileGrammar(); err != nil {
		t.Fatal(err)
	}
	d.ensureOperations()
	if _, ok := d.resolveOperation("triple"); !ok {
		t.Error("triple should still resolve after a reload")
	}
}

// The cross-tongue payoff the dissolution buys: a Turkish operation word resolves to
// the same operation the English one does, with only the seed word added and no new
// code, and a computation read through it runs to its value.
func TestOperationCrossTongue(t *testing.T) {
	d, _ := testDaemon(t)
	d.cfg.RuntimeEnabled = true
	en, _ := d.resolveOperation("times")
	tr, ok := d.resolveOperation("çarp")
	if !ok || tr.Name != "multiply" || tr.Name != en.Name {
		t.Fatalf("Turkish çarp should name the same operation as times: en=%+v tr=%+v ok=%v", en, tr, ok)
	}
	if r := say(t, d, "what is the çarp of 6 and 7"); !strings.Contains(r.Say, "42") {
		t.Errorf("a computation read through the Turkish operation word should run: %q", r.Say)
	}
}
