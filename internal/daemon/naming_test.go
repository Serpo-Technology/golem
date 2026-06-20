package daemon

import (
	"testing"
	"time"

	"golem/internal/memory"
)

// A concept the graph cannot name in a language borrows the nearest named neighbor's word
// through the vector layer: the embedding proposes where the naming edges are silent. The
// far neighbor's word is not borrowed, so only a confidently-near word is put in its mouth,
// and a concept with no embedding keeps its gloss, so naming never regresses.
func TestConceptNameBorrowsNearWord(t *testing.T) {
	d, s := testDaemon(t)

	mk := func(content string) uint64 {
		f := &memory.Fragment{Kind: memory.KindConcept, Channel: "test", Content: content, Time: time.Now()}
		if _, err := s.Encode(f); err != nil {
			t.Fatal(err)
		}
		return f.ID
	}
	name := func(concept uint64, form, lang string) {
		w := &memory.Fragment{Kind: memory.KindWord, Lang: lang, Channel: "test", Content: form, Time: time.Now()}
		if _, err := s.Encode(w); err != nil {
			t.Fatal(err)
		}
		if err := s.LinkRel(w.ID, concept, memory.RelNames, 1); err != nil {
			t.Fatal(err)
		}
	}
	vec := func(id uint64, v ...float32) {
		if err := s.SetVector(id, v); err != nil {
			t.Fatal(err)
		}
	}

	// A near named neighbor and a far one; the unnamable concept's embedding sits on the
	// near one, so its word is the one to borrow.
	near := mk("")
	name(near, "dog", "en")
	vec(near, 1, 0, 0)
	far := mk("")
	name(far, "cat", "en")
	vec(far, 0, 1, 0)
	gap := mk("opaque") // a concept with no naming word in English
	vec(gap, 1, 0, 0)

	if got := d.conceptName(gap, "opaque", "en"); got != "dog" {
		t.Fatalf("the unnamable concept should borrow the nearest named word %q, got %q", "dog", got)
	}

	// A concept with no embedding has nothing to propose, so naming stays its gloss.
	bare := mk("plain")
	if got := d.conceptName(bare, "plain", "en"); got != "plain" {
		t.Fatalf("a concept with no embedding should keep its gloss, got %q", got)
	}
}
