package daemon

import (
	"strings"
	"testing"
	"time"

	"golem/internal/memory"
)

func qFeature(t *testing.T, d *Daemon, gloss string) *memory.Fragment {
	t.Helper()
	f := &memory.Fragment{Kind: memory.KindConcept, Channel: "concept", Content: gloss,
		Template: "feat:" + gloss, Category: "feature", Limbic: d.chem.Snapshot(), Time: time.Now()}
	if _, err := d.store.Encode(f); err != nil {
		t.Fatal(err)
	}
	return f
}

func qLink(t *testing.T, d *Daemon, n *memory.Fragment, feats ...*memory.Fragment) {
	t.Helper()
	for _, f := range feats {
		if err := d.store.SetLinkRel(n.ID, f.ID, memory.RelFeature, 2.0); err != nil {
			t.Fatal(err)
		}
	}
}

// A role question whose verb names no relation retrieves through the relation its
// known argument's company most resembles, hedged. This is where resemblance lives
// now: on the question path, never on the teaching path.
func TestResembleQuestionRetrieval(t *testing.T) {
	d, _ := testDaemon(t)
	n := map[string]*memory.Fragment{}
	for _, g := range []string{"dog", "cat", "wolf", "rabbit", "cow", "grass"} {
		n[g] = groundNoun(t, d, g)
	}
	legs, fur, green := qFeature(t, d, "legs"), qFeature(t, d, "fur"), qFeature(t, d, "green")
	for _, a := range []string{"dog", "cat", "wolf", "rabbit"} {
		qLink(t, d, n[a], legs, fur)
	}
	qLink(t, d, n["cow"], legs)
	qLink(t, d, n["grass"], green)

	say(t, d, "the dog chased the cat")
	say(t, d, "the wolf chased the rabbit")
	say(t, d, "the cow ate the grass")
	d.growEmbeddings()

	// "hunt" names no relation; the wolf's company resembles chase, so the question
	// retrieves the chase fact, hedged.
	got := strings.ToLower(say(t, d, "what did the wolf hunt").Say)
	if !strings.Contains(got, "chase") || !strings.Contains(got, "probably") {
		t.Errorf("an unknown query verb should retrieve via the resembling relation, hedged: %q", got)
	}

	// The rail: a known verb answers firmly, never through resemblance.
	if got := strings.ToLower(say(t, d, "what did the wolf chase").Say); strings.Contains(got, "probably") {
		t.Errorf("a known verb should answer firmly, not hedged: %q", got)
	}
}
