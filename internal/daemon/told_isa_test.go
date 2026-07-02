package daemon

import (
	"strings"
	"testing"
	"time"

	"golem/internal/limbic"
	"golem/internal/memory"
)

// A told membership reads a hedged yes and does not become a grounded one: told "a zebra
// is an animal", GOLEM answers "probably" and remembers it, but the zebra is not pulled
// into the animal category the grounded machinery builds, so a told fact cannot corrupt
// the categories learned from experience.
func TestToldIsAReadsHedgedWithoutGrounding(t *testing.T) {
	d, s := testDaemon(t)
	base := time.Now().Add(-3 * time.Hour)
	feedShape(t, s, "vision", "a small black cat", 4, base)
	feedShape(t, s, "vision", "a gray wolf", 4, base.Add(time.Minute))
	feedShape(t, s, "vision", "a striped equine", 4, base.Add(2*time.Minute))
	if _, err := s.MergeDuplicates(); err != nil {
		t.Fatal(err)
	}
	calm(d)
	d.growConcepts()

	conceptFor := func(caption string) *memory.Fragment {
		cs, err := s.Concepts()
		if err != nil {
			t.Fatal(err)
		}
		for _, c := range cs {
			if c.Content == caption {
				return c
			}
		}
		t.Fatalf("percept %q should distil", caption)
		return nil
	}
	catC, wolfC, zebraC := conceptFor("a small black cat"), conceptFor("a gray wolf"), conceptFor("a striped equine")

	name := func(form string, cid uint64) {
		w, _, err := s.WordNode(form, "en", limbic.State{}, time.Now())
		if err != nil {
			t.Fatal(err)
		}
		if err := s.SetLinkRel(w.ID, cid, memory.RelNames, 0.8); err != nil {
			t.Fatal(err)
		}
	}
	name("cat", catC.ID)
	name("wolf", wolfC.ID)
	name("zebra", zebraC.ID)
	// A grounded animal category over the cat and the wolf, not the zebra.
	animal := &memory.Fragment{Kind: memory.KindConcept, Channel: "concept", Content: "animal",
		Template: "abs:en:animal", Category: "noun", Time: time.Now()}
	if _, err := s.Encode(animal); err != nil {
		t.Fatal(err)
	}
	if err := s.SetLinkRel(animal.ID, catC.ID, memory.RelGrounds, 0.5); err != nil {
		t.Fatal(err)
	}
	if err := s.SetLinkRel(animal.ID, wolfC.ID, memory.RelGrounds, 0.5); err != nil {
		t.Fatal(err)
	}
	name("animal", animal.ID)

	// Before being told, the zebra is not an animal: not grounded, closed-world no.
	before := say(t, d, "is a zebra an animal")
	if strings.Contains(strings.ToLower(before.Say), "yes") || strings.Contains(strings.ToLower(before.Say), "probably") {
		t.Fatalf("before telling, an ungrounded membership should read no; got %q", before.Say)
	}
	// Told it, GOLEM takes it defeasibly.
	say(t, d, "a zebra is an animal")
	after := say(t, d, "is a zebra an animal")
	if !strings.Contains(strings.ToLower(after.Say), "probably") {
		t.Fatalf("a told membership should read a hedged yes; got %q", after.Say)
	}
	// The grounded categories stay clean: the zebra is not a grounded member of animal.
	for _, c := range d.categoriesOf(zebraC.ID) {
		if c == animal.ID {
			t.Fatal("a told membership must not become a grounded category member")
		}
	}

	// A thing new to GOLEM can be told into a category too: "a lynx is an animal" for a
	// lynx it has never seen introduces the concept and records the told membership.
	say(t, d, "a lynx is an animal")
	lynx := say(t, d, "is a lynx an animal")
	if !strings.Contains(strings.ToLower(lynx.Say), "probably") {
		t.Fatalf("a new thing told into a category should read a hedged yes; got %q", lynx.Say)
	}
}

// A told membership is gated by the source's trust: a source GOLEM has come to distrust
// (below the trust a freshly-met peer starts at) cannot inject a category membership, while
// the architect's same claim is taken defeasibly. This keeps a promiscuous, distrusted peer
// from populating the category memory with claims GOLEM has no reason to entertain.
func TestTrustGatesToldIsA(t *testing.T) {
	d, s := testDaemon(t)
	base := time.Now().Add(-3 * time.Hour)
	feedShape(t, s, "vision", "a small black cat", 4, base)
	feedShape(t, s, "vision", "a gray wolf", 4, base.Add(time.Minute))
	feedShape(t, s, "vision", "a striped equine", 4, base.Add(2*time.Minute))
	if _, err := s.MergeDuplicates(); err != nil {
		t.Fatal(err)
	}
	calm(d)
	d.growConcepts()

	conceptFor := func(caption string) *memory.Fragment {
		cs, err := s.Concepts()
		if err != nil {
			t.Fatal(err)
		}
		for _, c := range cs {
			if c.Content == caption {
				return c
			}
		}
		t.Fatalf("percept %q should distil", caption)
		return nil
	}
	catC, wolfC, zebraC := conceptFor("a small black cat"), conceptFor("a gray wolf"), conceptFor("a striped equine")
	name := func(form string, cid uint64) {
		w, _, err := s.WordNode(form, "en", limbic.State{}, time.Now())
		if err != nil {
			t.Fatal(err)
		}
		if err := s.SetLinkRel(w.ID, cid, memory.RelNames, 0.8); err != nil {
			t.Fatal(err)
		}
	}
	name("cat", catC.ID)
	name("wolf", wolfC.ID)
	name("zebra", zebraC.ID)
	animal := &memory.Fragment{Kind: memory.KindConcept, Channel: "concept", Content: "animal",
		Template: "abs:en:animal", Category: "noun", Time: time.Now()}
	if _, err := s.Encode(animal); err != nil {
		t.Fatal(err)
	}
	if err := s.SetLinkRel(animal.ID, catC.ID, memory.RelGrounds, 0.5); err != nil {
		t.Fatal(err)
	}
	if err := s.SetLinkRel(animal.ID, wolfC.ID, memory.RelGrounds, 0.5); err != nil {
		t.Fatal(err)
	}
	name("animal", animal.ID)

	// A source GOLEM has come to distrust cannot inject the membership.
	if err := d.setSourceTrust("llm", 0.1); err != nil {
		t.Fatal(err)
	}
	sayAs(t, d, "llm", "", "a zebra is an animal")
	if got := say(t, d, "is a zebra an animal").Say; strings.Contains(strings.ToLower(got), "probably") || strings.Contains(strings.ToLower(got), "yes") {
		t.Fatalf("a distrusted source must not inject a told membership; got %q", got)
	}
	// The architect's same claim is taken defeasibly.
	say(t, d, "a zebra is an animal")
	if got := say(t, d, "is a zebra an animal").Say; !strings.Contains(strings.ToLower(got), "probably") {
		t.Fatalf("the architect's told membership should read a hedged yes; got %q", got)
	}
}

// Turkish asks the is-a with the interrogative particle and no copula ("kedi bir hayvan
// mı"). The grounded membership answers firmly in Turkish (evet), and a non-member is
// denied (hayır/değil), the same reasoning the English is-a runs, reached through the
// Turkish nominal-predication question the verb-medial parser could not see.
func TestTurkishIsAAnswers(t *testing.T) {
	d, s := testDaemon(t)
	base := time.Now().Add(-3 * time.Hour)
	feedShape(t, s, "vision", "a small black cat", 4, base)
	feedShape(t, s, "vision", "a round red ball", 4, base.Add(time.Minute))
	if _, err := s.MergeDuplicates(); err != nil {
		t.Fatal(err)
	}
	calm(d)
	d.growConcepts()

	conceptFor := func(caption string) *memory.Fragment {
		cs, err := s.Concepts()
		if err != nil {
			t.Fatal(err)
		}
		for _, c := range cs {
			if c.Content == caption {
				return c
			}
		}
		t.Fatalf("percept %q should distil", caption)
		return nil
	}
	catC, ballC := conceptFor("a small black cat"), conceptFor("a round red ball")
	nameTR := func(form string, cid uint64) {
		w, _, err := s.WordNode(form, "tr", limbic.State{}, time.Now())
		if err != nil {
			t.Fatal(err)
		}
		if err := s.SetLinkRel(w.ID, cid, memory.RelNames, 0.8); err != nil {
			t.Fatal(err)
		}
	}
	nameTR("kedi", catC.ID)
	nameTR("top", ballC.ID)
	// A grounded Turkish animal category over the cat, not the ball.
	animal := &memory.Fragment{Kind: memory.KindConcept, Channel: "concept", Content: "hayvan",
		Template: "abs:tr:hayvan", Category: "noun", Time: time.Now()}
	if _, err := s.Encode(animal); err != nil {
		t.Fatal(err)
	}
	if err := s.SetLinkRel(animal.ID, catC.ID, memory.RelGrounds, 0.5); err != nil {
		t.Fatal(err)
	}
	nameTR("hayvan", animal.ID)

	yes := strings.ToLower(say(t, d, "kedi bir hayvan mı").Say)
	if !strings.Contains(yes, "evet") {
		t.Fatalf("a grounded Turkish membership should answer evet; got %q", yes)
	}
	no := strings.ToLower(say(t, d, "top bir hayvan mı").Say)
	if strings.Contains(no, "evet") || !(strings.Contains(no, "hayır") || strings.Contains(no, "değil") || strings.Contains(no, "degil")) {
		t.Fatalf("a non-member should be denied in Turkish; got %q", no)
	}
}
