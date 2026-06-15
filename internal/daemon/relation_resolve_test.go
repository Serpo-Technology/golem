package daemon

import (
	"testing"
	"time"

	"golem/internal/memory"
)

// resolveRelation picks the relation a word names even when a spurious noun of
// the same form outweighs it in the naming vote, so a role question's verb is
// not shadowed by an abstraction the constellation pass distilled from the
// verb's arguments.
func TestResolveRelationPrefersTheRelation(t *testing.T) {
	d, s := testDaemon(t)
	now := time.Now()
	rel, _ := s.Encode(&memory.Fragment{Kind: memory.KindConcept, Content: "chase", Template: "rel:en:chase", Category: "relation", Time: now})
	noun, _ := s.Encode(&memory.Fragment{Kind: memory.KindConcept, Content: "chase", Template: "abs:en:chase", Category: "noun", Time: now})
	w, _, err := s.WordNode("chase", "en", d.chem.Snapshot(), now)
	if err != nil {
		t.Fatal(err)
	}
	// The noun is named more heavily, the way the vote would otherwise pick it.
	if err := s.SetLinkRel(w.ID, noun, memory.RelNames, 0.9); err != nil {
		t.Fatal(err)
	}
	if err := s.SetLinkRel(w.ID, rel, memory.RelNames, 0.5); err != nil {
		t.Fatal(err)
	}

	if got := d.resolveRelation("chase"); got == nil || got.ID != rel {
		t.Fatalf("resolveRelation should return the relation despite the heavier noun: %+v", got)
	}
	// Plain resolveConcept still follows the vote to the noun, which is why the
	// role question needs the relation-specific resolver.
	if c := d.resolveConcept("chase"); c == nil || c.ID != noun {
		t.Fatalf("resolveConcept should follow the naming vote to the noun: %+v", c)
	}
}
