package daemon

import (
	"strings"
	"testing"
	"time"

	"golem/internal/memory"
)

// What a gradable means in number is taught, trust-gated: the architect's
// telling marks the relation with the grounded comparison; a low-trust peer's
// is refused, since a flipped pole would invert arithmetic truth.
func TestNumericPoleTaughtAndTrustGated(t *testing.T) {
	d, _ := testDaemon(t)
	d.cfg.RuntimeEnabled = true
	// The gradable exists as a relation once an ordering has been told with it.
	say(t, d, "2 is bigger than 1")

	// A peer cannot set the pole.
	r := sayAs(t, d, "llm", "", "bigger is above")
	if !strings.Contains(r.Say, "architect") {
		t.Fatalf("a low-trust speaker should be refused the pole: %q", r.Say)
	}
	if _, ok := d.numericPole("bigger"); ok {
		t.Fatal("a refused telling must not mark the pole")
	}

	// The architect sets it, and the mark reads back.
	say(t, d, "bigger is above")
	sym, ok := d.numericPole("bigger")
	if !ok || sym != ">" {
		t.Fatalf("the taught pole should read back as the comparison symbol: %q %v", sym, ok)
	}
}

// A numeric comparison settles by computing: firm both ways from one landed
// difference, the false direction denied on record, and no second program for
// the converse asking.
func TestNumericComparisonComputedAndShared(t *testing.T) {
	d, s := testDaemon(t)
	d.cfg.RuntimeEnabled = true
	say(t, d, "2 is bigger than 1")
	say(t, d, "bigger is above")

	yes := say(t, d, "is 17 bigger than 12").Say
	if !strings.Contains(strings.ToLower(yes), "yes") {
		t.Fatalf("a true numeric comparison should answer firm yes: %q", yes)
	}
	// One difference landed; the converse asking reads its sign, no new run.
	if _, err := s.FragmentByTemplate("computed:subtract:17:12"); err != nil {
		t.Fatal(err)
	}
	d.cfg.RuntimeEnabled = false // any further need to run would now fail
	no := say(t, d, "is 12 bigger than 17").Say
	if !strings.Contains(strings.ToLower(no), "no") {
		t.Fatalf("the converse should answer firm no from the landed difference: %q", no)
	}

	// The false direction stands denied between the number concepts.
	rel := d.resolveRelation("bigger")
	if rel == nil {
		t.Fatal("the gradable should resolve to its relation")
	}
	twelve, _ := d.store.ConceptByTemplate("num:12")
	seventeen, _ := d.store.ConceptByTemplate("num:17")
	if twelve == nil || seventeen == nil {
		t.Fatal("the numerals should stand as number concepts")
	}
	if !d.propositionDenied(twelve, rel.Content, seventeen.ID) {
		t.Fatal("the refuted direction should be denied on record")
	}
}

// A peer whose numeric claim the run refutes is contradicted on record, and
// the nightly trust pass counts it against the source.
func TestComputedRefutationErodesTrust(t *testing.T) {
	d, _ := testDaemon(t)
	d.cfg.RuntimeEnabled = true
	say(t, d, "2 is bigger than 1")
	say(t, d, "bigger is above")

	// The peer claims a falsehood; the claim lands on the number concepts.
	sayAs(t, d, "llm", "", "12 is bigger than 17")
	llm, err := d.ensureSource("llm")
	if err != nil {
		t.Fatal(err)
	}
	before := d.sourceTrust(llm.ID)

	// The check refutes it: asked, GOLEM computes and denies the direction.
	no := say(t, d, "is 12 bigger than 17").Say
	if !strings.Contains(strings.ToLower(no), "no") {
		t.Fatalf("the run should refute the claim: %q", no)
	}

	d.growSourceTrust()
	after := d.sourceTrust(llm.ID)
	if after >= before {
		t.Fatalf("a claim the run refuted should erode the source's trust: %.2f then %.2f", before, after)
	}
}

// An unmarked gradable falls through: numerals compared on a word never taught
// a pole take the told-ordering path exactly as before.
func TestUnmarkedGradableFallsThrough(t *testing.T) {
	d, s := testDaemon(t)
	d.cfg.RuntimeEnabled = true
	say(t, d, "2 is heavier than 1")
	say(t, d, "is 17 heavier than 12")
	if _, err := s.FragmentByTemplate("computed:subtract:17:12"); err == nil {
		if f, _ := s.FragmentByTemplate("computed:subtract:17:12"); f != nil {
			t.Fatal("an unmarked gradable must not reach the runtime")
		}
	}
	if _, ok := d.numericPole("heavier"); ok {
		t.Fatal("no pole was taught for heavier")
	}
}

// A magnitude question over bare numerals answers with the metric amount the
// difference computes, from the landed fact, no fresh program for the asking.
func TestNumericMagnitudeComputed(t *testing.T) {
	d, _ := testDaemon(t)
	d.cfg.RuntimeEnabled = true
	say(t, d, "2 is bigger than 1")
	say(t, d, "bigger is above")
	say(t, d, "is 17 bigger than 12") // lands the difference

	d.cfg.RuntimeEnabled = false // the magnitude must read the landed fact
	got := say(t, d, "how much bigger is 17 than 12").Say
	if !strings.Contains(got, "5") || !strings.Contains(got, "bigger") {
		t.Fatalf("the magnitude should be the computed amount: %q", got)
	}
	no := say(t, d, "how much bigger is 12 than 17").Say
	if !strings.Contains(strings.ToLower(no), "no") {
		t.Fatalf("the wrong side takes the plain denial: %q", no)
	}
}

// A counting question over a category is an introspective tally: GOLEM looks
// at its own graph and counts the members, fresh at each asking.
func TestCountMembersIntrospects(t *testing.T) {
	d, s := testDaemon(t)
	base := time.Now().Add(-3 * time.Hour)
	feedShape(t, s, "vision", "a small black cat", 5, base)
	feedShape(t, s, "vision", "a large brown dog", 5, base)
	feedShape(t, s, "vision", "a small gray mouse", 5, base)
	if _, err := s.MergeDuplicates(); err != nil {
		t.Fatal(err)
	}
	calm(d)
	d.growConcepts()
	d.growFeatures()
	// A category grounded in three member concepts, the abstraction shape.
	abs := &memory.Fragment{Kind: memory.KindConcept, Channel: "concept", Content: "animal",
		Template: "abs:en:animal", Category: "noun", Time: time.Now()}
	if _, err := s.Encode(abs); err != nil {
		t.Fatal(err)
	}
	d.nameConceptWord(abs.ID, "animal", "en")
	n := 0
	for _, cap := range []string{"a small black cat", "a large brown dog", "a small gray mouse"} {
		if c, err := s.ConceptByTemplate(cap); err == nil && c != nil {
			s.SetLinkRel(abs.ID, c.ID, memory.RelGrounds, 0.4)
			n++
		}
	}
	if n != 3 {
		t.Fatalf("the three members should ground: %d", n)
	}

	got := say(t, d, "how many animals do you know").Say
	if !strings.Contains(got, "3") {
		t.Fatalf("the tally should count the grounded members: %q", got)
	}
}

// A counting question over a relation composes a graph query: the focus's
// co-arguments, filtered by category when one is asked, distinct concepts
// counted fresh from the graph.
func TestRelationCountComposesQuery(t *testing.T) {
	d, s := testDaemon(t)
	base := time.Now().Add(-3 * time.Hour)
	// A brain past the universal-feature threshold, the state a real one is in
	// before relations are ever taught: with the shared channel named
	// universal, a ball resembles a dog by nothing, so role fit reads real
	// kinship rather than the collapsed likeness of a tiny graph.
	for _, cap := range []string{
		"a large brown dog", "a small black cat", "a small gray mouse",
		"a small brown bird", "a tall green tree", "a bright yellow flower",
		"a heavy iron key", "a long blue river", "a warm woolen coat",
		"a sweet ripe apple", "a sharp steel knife", "a soft white cloud"} {
		feedShape(t, s, "vision", cap, 5, base)
	}
	if _, err := s.MergeDuplicates(); err != nil {
		t.Fatal(err)
	}
	calm(d)
	d.growConcepts()
	d.growFeatures()
	abs := &memory.Fragment{Kind: memory.KindConcept, Channel: "concept", Content: "animal",
		Template: "abs:en:animal", Category: "noun", Time: time.Now()}
	if _, err := s.Encode(abs); err != nil {
		t.Fatal(err)
	}
	d.nameConceptWord(abs.ID, "animal", "en")
	// The nouns are named before the relations mention them, the curriculum
	// invariant, so each patient resolves to its own percept.
	for word, cap := range map[string]string{
		"dog": "a large brown dog", "cat": "a small black cat",
		"mouse": "a small gray mouse", "bird": "a small brown bird"} {
		if c, err := s.ConceptByTemplate(cap); err == nil && c != nil {
			d.nameConceptWord(c.ID, word, "en")
		}
	}
	for _, cap := range []string{"a small black cat", "a small gray mouse"} {
		if c, err := s.ConceptByTemplate(cap); err == nil && c != nil {
			s.SetLinkRel(abs.ID, c.ID, memory.RelGrounds, 0.4)
		}
	}
	say(t, d, "a dog chased a cat")
	say(t, d, "a dog chased a mouse")
	say(t, d, "a dog chased a bird")

	got := say(t, d, "how many animals did the dog chase").Say
	if !strings.Contains(got, "2") {
		t.Fatalf("the filtered tally should count the animal co-arguments: %q", got)
	}
	all := say(t, d, "how many things did the dog chase").Say
	if !strings.Contains(all, "3") {
		t.Fatalf("the unfiltered tally should count every co-argument: %q", all)
	}
}
