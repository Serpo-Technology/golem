package daemon

import (
	"strings"
	"testing"
	"time"

	"golem/internal/memory"
)

// A transitive clause whose object stands bare (a mass or plural noun: "a zebra
// ate grass") carries one determiner, which the span count alone reads as an
// intransitive with the object mistaken for the verb. The graph settles it: the
// token that resolves to the learned relation is the predicate, and the trailing
// content binds as the patient, so the telling writes the whole proposition.
func TestBareObjectTransitiveComprehends(t *testing.T) {
	d, s := testDaemon(t)
	base := time.Now().Add(-3 * time.Hour)
	feedShape(t, s, "vision", "a large brown horse", 5, base)
	feedShape(t, s, "vision", "a green grass tuft", 5, base)
	feedShape(t, s, "vision", "a striped gray zebra", 5, base)
	if _, err := s.MergeDuplicates(); err != nil {
		t.Fatal(err)
	}
	calm(d)
	d.growConcepts()
	d.growFeatures()
	// Teach eat the corpus way, article'd, so the relation and its construction exist.
	say(t, d, "a horse ate a grass")
	say(t, d, "a horse ate a grass")

	resp := say(t, d, "a zebra ate grass")
	if resp.Observe == nil || resp.Observe.FragmentID == 0 {
		t.Fatal("the telling was not remembered")
	}
	pt, err := s.LinksFromRel([]uint64{resp.Observe.FragmentID}, memory.RelPatient)
	if err != nil {
		t.Fatal(err)
	}
	if len(pt) == 0 {
		t.Fatal("a bare object must still bind as the patient")
	}
	frags, err := s.FragmentsByIDs([]uint64{pt[0].To})
	if err != nil || len(frags) != 1 {
		t.Fatalf("patient concept unreadable: %v", err)
	}
	if frags[0].Content != "a green grass tuft" {
		t.Fatalf("the bare object should bind the grass percept, got %q", frags[0].Content)
	}

	// The proposition reads back through the polar question.
	r := say(t, d, "did a zebra eat a grass")
	low := strings.ToLower(r.Say)
	if !strings.Contains(low, "yes") && !strings.Contains(low, "probab") {
		t.Fatalf("the bare-object telling should be readable, got %q", r.Say)
	}
}
