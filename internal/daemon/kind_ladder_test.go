package daemon

import (
	"strings"
	"testing"
	"time"

	"golem/internal/limbic"
	"golem/internal/memory"
)

// mkAbstraction encodes a category concept with an abs: template so abstractionLemma reads it.
func mkAbstraction(t *testing.T, s *memory.Store, content, template string) *memory.Fragment {
	t.Helper()
	a := &memory.Fragment{Kind: memory.KindConcept, Channel: "concept", Content: content,
		Template: template, Category: "noun", Time: time.Now()}
	if _, err := s.Encode(a); err != nil {
		t.Fatal(err)
	}
	return a
}

// mkMember encodes a percept-level concept, a thing a category grounds in (never an abstraction).
func mkMember(t *testing.T, s *memory.Store, content string) *memory.Fragment {
	t.Helper()
	m := &memory.Fragment{Kind: memory.KindConcept, Channel: "vision", Content: content,
		Template: content, Category: "noun", Time: time.Now()}
	if _, err := s.Encode(m); err != nil {
		t.Fatal(err)
	}
	return m
}

func groundKL(t *testing.T, s *memory.Store, absID, memberID uint64) {
	t.Helper()
	if err := s.SetLinkRel(absID, memberID, memory.RelGrounds, 0.5); err != nil {
		t.Fatal(err)
	}
}

func groundManyKL(t *testing.T, s *memory.Store, absID uint64, members []uint64) {
	t.Helper()
	for _, m := range members {
		groundKL(t, s, absID, m)
	}
}

func nameKL(t *testing.T, s *memory.Store, form string, cid uint64) {
	t.Helper()
	w, _, err := s.WordNode(form, "en", limbic.State{}, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if err := s.SetLinkRel(w.ID, cid, memory.RelNames, 0.8); err != nil {
		t.Fatal(err)
	}
}

func supersKL(s *memory.Store, id uint64) map[uint64]bool {
	out := map[uint64]bool{}
	es, _ := s.LinksFromRel([]uint64{id}, memory.RelKindOf)
	for _, e := range es {
		out[e.To] = true
	}
	return out
}

// A member reaches a superordinate up the kind ladder, hedged, even after the superordinate's own
// grounding of that member has faded: the dog is a mammal (grounded), mammal is-a animal (a kind
// edge), so a dog is probably an animal, though animal no longer grounds the dog.
func TestKindLadderReachesSuperordinateHedged(t *testing.T) {
	d, s := testDaemon(t)
	d.cfg.KindLadderEnabled = true
	dog := mkMember(t, s, "a small brown dog")
	cat := mkMember(t, s, "a small black cat")
	mammal := mkAbstraction(t, s, "mammal", "abs:en:mammal")
	animal := mkAbstraction(t, s, "animal", "abs:en:animal")
	groundKL(t, s, mammal.ID, dog.ID)
	groundKL(t, s, mammal.ID, cat.ID)
	groundKL(t, s, animal.ID, cat.ID) // the collapsed superordinate no longer grounds the dog
	// the kind ladder edge (laid earlier while animal was richly grounded, spared decay since)
	if err := s.SetLinkRel(mammal.ID, animal.ID, memory.RelKindOf, kindLadderWeight); err != nil {
		t.Fatal(err)
	}
	nameKL(t, s, "dog", dog.ID)
	nameKL(t, s, "mammal", mammal.ID)
	nameKL(t, s, "animal", animal.ID)

	// The dog is NOT a grounded member of the collapsed animal: the ladder never enters the
	// grounded hierarchy, so a firm is-a is not asserted.
	if d.isA(dog.ID, animal.ID) {
		t.Fatal("the kind ladder must not make the dog a GROUNDED member of animal")
	}
	// But the question reaches animal up the ladder and answers a hedged yes.
	r := say(t, d, "is a dog an animal")
	if !strings.Contains(strings.ToLower(r.Say), "probably") {
		t.Fatalf("dog should reach animal up the kind ladder, hedged; got %q", r.Say)
	}
	if strings.Contains(strings.ToLower(r.Say), "no,") {
		t.Fatalf("dog should not be denied; got %q", r.Say)
	}
}

// A kind itself is-a the broader category it falls under, hedged: is a mammal an animal.
func TestKindLadderKindIsASuperordinate(t *testing.T) {
	d, s := testDaemon(t)
	d.cfg.KindLadderEnabled = true
	cat := mkMember(t, s, "a small black cat")
	mammal := mkAbstraction(t, s, "mammal", "abs:en:mammal")
	animal := mkAbstraction(t, s, "animal", "abs:en:animal")
	groundKL(t, s, mammal.ID, cat.ID)
	groundKL(t, s, animal.ID, cat.ID)
	if err := s.SetLinkRel(mammal.ID, animal.ID, memory.RelKindOf, kindLadderWeight); err != nil {
		t.Fatal(err)
	}
	nameKL(t, s, "mammal", mammal.ID)
	nameKL(t, s, "animal", animal.ID)
	r := say(t, d, "is a mammal an animal")
	if !strings.Contains(strings.ToLower(r.Say), "probably") {
		t.Fatalf("a mammal should be an animal up the ladder, hedged; got %q", r.Say)
	}
}

// A grounded membership stays FIRM: the ladder softens only the memberships it alone reaches,
// never one the grounding already holds. Is a dog a mammal answers a plain yes, not probably.
func TestKindLadderKeepsGroundedFirm(t *testing.T) {
	d, s := testDaemon(t)
	d.cfg.KindLadderEnabled = true
	dog := mkMember(t, s, "a small brown dog")
	cat := mkMember(t, s, "a small black cat")
	mammal := mkAbstraction(t, s, "mammal", "abs:en:mammal")
	groundKL(t, s, mammal.ID, dog.ID)
	groundKL(t, s, mammal.ID, cat.ID)
	nameKL(t, s, "dog", dog.ID)
	nameKL(t, s, "mammal", mammal.ID)
	r := say(t, d, "is a dog a mammal")
	low := strings.ToLower(r.Say)
	if !strings.Contains(low, "yes") || strings.Contains(low, "probably") {
		t.Fatalf("a grounded membership must stay firm yes; got %q", r.Say)
	}
}

// growKindLadder derives mammal->animal when most of a kind's members fall under a strictly
// broader superordinate, tolerating a stray grounding, while a cross-cutting category (pet) that
// shares only a few members with a kind mints no edge either direction, so no false is-a derives.
func TestGrowKindLadderOverlapOnly(t *testing.T) {
	d, s := testDaemon(t)
	d.cfg.KindLadderEnabled = true
	dog := mkMember(t, s, "a small brown dog")
	cat := mkMember(t, s, "a small black cat")
	eleph := mkMember(t, s, "a large gray elephant")
	wolf := mkMember(t, s, "a gray wolf")
	bear := mkMember(t, s, "a brown bear")
	fish := mkMember(t, s, "a small goldfish")
	crow := mkMember(t, s, "a black crow")
	mammal := mkAbstraction(t, s, "mammal", "abs:en:mammal")
	animal := mkAbstraction(t, s, "animal", "abs:en:animal")
	pet := mkAbstraction(t, s, "pet", "abs:en:pet")
	// mammal {dog,cat,eleph,wolf,bear}; animal grounds all five plus fish and crow: strictly
	// broader with full overlap.
	groundManyKL(t, s, mammal.ID, []uint64{dog.ID, cat.ID, eleph.ID, wolf.ID, bear.ID})
	groundManyKL(t, s, animal.ID, []uint64{dog.ID, cat.ID, eleph.ID, wolf.ID, bear.ID, fish.ID, crow.ID})
	// pet {dog,cat,fish,crow}: shares only dog,cat with mammal (2/5 = 0.4 < 0.8) -> no edge.
	groundManyKL(t, s, pet.ID, []uint64{dog.ID, cat.ID, fish.ID, crow.ID})
	d.growKindLadder()
	if !supersKL(s, mammal.ID)[animal.ID] {
		t.Fatal("a kind mostly inside a strictly-broader category should derive mammal->animal")
	}
	if supersKL(s, mammal.ID)[pet.ID] || supersKL(s, pet.ID)[mammal.ID] {
		t.Fatal("cross-cutting pet vs mammal must derive no edge either direction")
	}
	if supersKL(s, animal.ID)[mammal.ID] {
		t.Fatal("the broader animal must never be made a kind of the narrower mammal")
	}
}

// A stray grounding does not hide a real kind: mammal with one member animal has not re-grounded
// still clears the 0.8 overlap bar, so mammal->animal is derived despite imperfect containment.
func TestGrowKindLadderToleratesStray(t *testing.T) {
	d, s := testDaemon(t)
	d.cfg.KindLadderEnabled = true
	dog := mkMember(t, s, "a small brown dog")
	cat := mkMember(t, s, "a small black cat")
	eleph := mkMember(t, s, "a large gray elephant")
	wolf := mkMember(t, s, "a gray wolf")
	bear := mkMember(t, s, "a brown bear")
	fox := mkMember(t, s, "a red fox")
	crow := mkMember(t, s, "a black crow")
	mammal := mkAbstraction(t, s, "mammal", "abs:en:mammal")
	animal := mkAbstraction(t, s, "animal", "abs:en:animal")
	// mammal has 5 members; animal grounds 4 of them (missing fox, a stray) plus crow: 4/5 = 0.8.
	groundManyKL(t, s, mammal.ID, []uint64{dog.ID, cat.ID, eleph.ID, wolf.ID, fox.ID})
	groundManyKL(t, s, animal.ID, []uint64{dog.ID, cat.ID, eleph.ID, wolf.ID, bear.ID, crow.ID})
	d.growKindLadder()
	if !supersKL(s, mammal.ID)[animal.ID] {
		t.Fatal("a stray grounding (fox not re-grounded as animal) must not hide mammal->animal at 0.8 overlap")
	}
}

// A thin abstraction (below the min core) is never read as a kind on the ladder: neither a
// degenerate one-member abstraction nor a two/three-member noisy one is wired to a broader set
// that happens to contain its handful of members.
func TestGrowKindLadderSkipsThinCores(t *testing.T) {
	d, s := testDaemon(t)
	d.cfg.KindLadderEnabled = true
	dog := mkMember(t, s, "a small brown dog")
	cat := mkMember(t, s, "a small black cat")
	eleph := mkMember(t, s, "a large gray elephant")
	wolf := mkMember(t, s, "a gray wolf")
	bear := mkMember(t, s, "a brown bear")
	thin := mkAbstraction(t, s, "thin", "abs:en:thin") // two members: below kindLadderMinCore
	animal := mkAbstraction(t, s, "animal", "abs:en:animal")
	groundManyKL(t, s, thin.ID, []uint64{dog.ID, cat.ID})
	groundManyKL(t, s, animal.ID, []uint64{dog.ID, cat.ID, eleph.ID, wolf.ID, bear.ID})
	d.growKindLadder()
	if len(supersKL(s, thin.ID)) != 0 {
		t.Fatalf("a below-min-core abstraction must lay no kind edge; got %v", supersKL(s, thin.ID))
	}
}

// The first (healthy) orientation is kept: once mammal->animal is derived from a rich animal
// core, a later pass where the superordinate has thinned below the kind must NOT lay the reversed
// animal->mammal edge, nor form a two-cycle.
func TestGrowKindLadderKeepsFirstOrientation(t *testing.T) {
	d, s := testDaemon(t)
	d.cfg.KindLadderEnabled = true
	dog := mkMember(t, s, "a small brown dog")
	cat := mkMember(t, s, "a small black cat")
	eleph := mkMember(t, s, "a large gray elephant")
	wolf := mkMember(t, s, "a gray wolf")
	bat := mkMember(t, s, "a small brown bat")
	crow := mkMember(t, s, "a black crow")
	mammal := mkAbstraction(t, s, "mammal", "abs:en:mammal")
	animal := mkAbstraction(t, s, "animal", "abs:en:animal")
	groundManyKL(t, s, mammal.ID, []uint64{dog.ID, cat.ID, eleph.ID, wolf.ID})
	groundManyKL(t, s, animal.ID, []uint64{dog.ID, cat.ID, eleph.ID, wolf.ID, bat.ID, crow.ID})
	d.growKindLadder()
	if !supersKL(s, mammal.ID)[animal.ID] {
		t.Fatal("the healthy pass should lay mammal->animal")
	}
	// The superordinate thins to a subset of the kind: animal grounds {dog,cat,eleph,wolf} (still
	// above the min core), mammal grows to include bat, so animal is now strictly inside mammal.
	if err := s.DropRelFrom([]uint64{animal.ID}, memory.RelGrounds); err != nil {
		t.Fatal(err)
	}
	groundManyKL(t, s, animal.ID, []uint64{dog.ID, cat.ID, eleph.ID, wolf.ID})
	groundKL(t, s, mammal.ID, bat.ID)
	d.growKindLadder()
	if supersKL(s, animal.ID)[mammal.ID] {
		t.Fatal("a thinned superordinate must not reverse into animal->mammal")
	}
	if !supersKL(s, mammal.ID)[animal.ID] {
		t.Fatal("the original mammal->animal must persist")
	}
}

// With the flag off the ladder is inert: growKindLadder writes nothing, and a membership only the
// ladder could reach reads a plain NO, byte-identical to before the feature -- even with a kind
// edge present in the store, the off flag ignores it.
func TestKindLadderFlagOffInert(t *testing.T) {
	d, s := testDaemon(t)
	d.cfg.KindLadderEnabled = false
	dog := mkMember(t, s, "a small brown dog")
	cat := mkMember(t, s, "a small black cat")
	eleph := mkMember(t, s, "a large gray elephant")
	wolf := mkMember(t, s, "a gray wolf")
	bear := mkMember(t, s, "a brown bear")
	mammal := mkAbstraction(t, s, "mammal", "abs:en:mammal")
	animal := mkAbstraction(t, s, "animal", "abs:en:animal")
	// dog is grounded under mammal only; animal never grounds it, so dog->animal is reachable
	// ONLY up the (present but flag-ignored) kind edge.
	groundManyKL(t, s, mammal.ID, []uint64{dog.ID, cat.ID, eleph.ID, wolf.ID})
	groundManyKL(t, s, animal.ID, []uint64{cat.ID, eleph.ID, wolf.ID, bear.ID})
	nameKL(t, s, "dog", dog.ID)
	nameKL(t, s, "mammal", mammal.ID)
	nameKL(t, s, "animal", animal.ID)
	if laid := d.growKindLadder(); laid != 0 {
		t.Fatalf("growKindLadder must write nothing with the flag off; laid %d", laid)
	}
	if err := s.SetLinkRel(mammal.ID, animal.ID, memory.RelKindOf, kindLadderWeight); err != nil {
		t.Fatal(err)
	}
	r := say(t, d, "is a dog an animal")
	if strings.Contains(strings.ToLower(r.Say), "probably") || strings.Contains(strings.ToLower(r.Say), "yes") {
		t.Fatalf("with the ladder off, an ungrounded membership must read no; got %q", r.Say)
	}
}

// A grounding the abstraction pass has stopped refreshing fades for some eighty nights
// before it reaches the link floor, and the climb reads weight nowhere, so a rung the
// writer already judged unfit would go on handing a firm membership to everything beneath
// it for all of that time. A member reached only through such a rung is not firmly a
// member. Inheritance and the nightly derivations go on seeing the graph entire.
func TestFadingRungCarriesNoFirmMembership(t *testing.T) {
	d, s := testDaemon(t)
	mk := func(word, tpl string) *memory.Fragment {
		f := &memory.Fragment{Kind: memory.KindConcept, Channel: "concept", Content: word,
			Template: tpl, Category: "noun", Time: time.Now()}
		if _, err := s.Encode(f); err != nil {
			t.Fatal(err)
		}
		return f
	}
	dog := mk("a small brown dog", "a small brown dog")
	mammal := mk("mammal", "abs:en:mammal")
	circle := mk("circle", "abs:en:circle")
	// A rung the pass still writes, and one it has stopped refreshing.
	if err := s.SetLinkRel(mammal.ID, dog.ID, memory.RelGrounds, absPwcFloor); err != nil {
		t.Fatal(err)
	}
	if err := s.SetLinkRel(circle.ID, mammal.ID, memory.RelGrounds, absPwcFloor*0.9); err != nil {
		t.Fatal(err)
	}
	d.cfg.OstensiveCoreEnabled = true
	if !d.standingIsA(dog.ID, mammal.ID) {
		t.Fatal("a rung at the write floor still carries a firm membership")
	}
	if d.standingIsA(dog.ID, circle.ID) {
		t.Fatal("a membership reached only through a fading rung must not be firm")
	}
	// The whole graph is still there for everything that is not a firm is-a answer.
	if !d.isA(dog.ID, circle.ID) {
		t.Fatal("categoriesOf must go on seeing the fading rung")
	}
	// With the gate off the reading is the old one, whatever the weights.
	d.cfg.OstensiveCoreEnabled = false
	if !d.standingIsA(dog.ID, circle.ID) {
		t.Fatal("with the gate off the climb must be unchanged")
	}
}
