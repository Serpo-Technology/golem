package daemon

import (
	"strings"
	"testing"
	"time"

	"golem/internal/memory"
)

// resembleFeature mints a bare feature concept so the nouns carry traits and the
// embedding has a non-empty matrix to factor.
func resembleFeature(t *testing.T, d *Daemon, gloss string) *memory.Fragment {
	t.Helper()
	f := &memory.Fragment{Kind: memory.KindConcept, Channel: "concept", Content: gloss,
		Template: "feat:" + gloss, Category: "feature", Limbic: d.chem.Snapshot(), Time: time.Now()}
	if _, err := d.store.Encode(f); err != nil {
		t.Fatal(err)
	}
	return f
}

func resembleLink(t *testing.T, d *Daemon, n *memory.Fragment, feats ...*memory.Fragment) {
	t.Helper()
	for _, f := range feats {
		if err := d.store.SetLinkRel(n.ID, f.ID, memory.RelFeature, 2.0); err != nil {
			t.Fatal(err)
		}
	}
}

// An unknown verb whose arguments keep a relation's company is read as that
// relation, hedged, so it can be corrected, while a known verb still resolves
// exactly and teaches in silence.
func TestResembleRelationFallbackAccepts(t *testing.T) {
	d, _ := testDaemon(t)
	n := map[string]*memory.Fragment{}
	for _, g := range []string{"dog", "cat", "wolf", "rabbit", "cow", "grass", "hay"} {
		n[g] = groundNoun(t, d, g)
	}
	legs, fur, green := resembleFeature(t, d, "legs"), resembleFeature(t, d, "fur"), resembleFeature(t, d, "green")
	for _, a := range []string{"dog", "cat", "wolf", "rabbit"} {
		resembleLink(t, d, n[a], legs, fur)
	}
	resembleLink(t, d, n["cow"], legs)
	resembleLink(t, d, n["grass"], green)
	resembleLink(t, d, n["hay"], green)

	// chase keeps predator/prey company; eat keeps animal/plant company.
	say(t, d, "the dog chased the cat")
	say(t, d, "the wolf chased the rabbit")
	say(t, d, "the cow ate the grass")
	say(t, d, "the cow ate the hay")
	d.growEmbeddings()

	got := strings.ToLower(say(t, d, "the wolf hunted the cat").Say)
	if !strings.Contains(got, "chase") || !strings.Contains(got, "probably") {
		t.Errorf("an unknown verb with chase-like arguments should read as chase, hedged: %q", got)
	}

	// The rail: a known verb resolves exactly, no fallback, teaches in silence.
	if got := say(t, d, "the dog chased the rabbit").Say; got != "" {
		t.Errorf("a known verb should teach silently, never hedge: %q", got)
	}

	// The held reading answers the polar question hedged, never as a firm fact.
	if got := say(t, d, "did the wolf chase the cat").Say; !strings.Contains(strings.ToLower(got), "probably") {
		t.Errorf("the resembled reading should answer hedged: %q", got)
	}
}

// When a clause's arguments resemble two relations equally, the thin margin earns
// a question rather than a guess.
func TestResembleRelationFallbackClarifies(t *testing.T) {
	d, _ := testDaemon(t)
	dog := groundNoun(t, d, "dog")
	cat := groundNoun(t, d, "cat")
	legs, fur := resembleFeature(t, d, "legs"), resembleFeature(t, d, "fur")
	resembleLink(t, d, dog, legs, fur)
	resembleLink(t, d, cat, legs, fur)

	// chase and see keep the very same company, so neither wins by the margin.
	say(t, d, "the dog chased the cat")
	say(t, d, "the dog saw the cat")
	d.growEmbeddings()

	if got := say(t, d, "the dog watched the cat").Say; !strings.HasPrefix(got, "Do you mean") {
		t.Errorf("a verb matching two relations equally should ask which is meant: %q", got)
	}
}
