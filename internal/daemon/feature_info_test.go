package daemon

import (
	"testing"
	"time"

	"golem/internal/attention"
	"golem/internal/senses"
)

// distil feeds a shape four times, enough for the merge pass to form a
// prototype the genesis pass distils into a concept.
func distil(t *testing.T, d *Daemon, channel, content string, base time.Time) {
	t.Helper()
	for i := range 4 {
		d.recent = nil
		d.process(senses.Observation{Channel: channel, Content: content,
			At: base.Add(time.Duration(i) * time.Minute)}, true, true)
	}
	d.recent = nil
}

// The done-when for feature-layer information-weighting: a feature carried
// by many concepts (frequent, low-information) is discounted below one
// carried by few (rare, defining), both in the signature a concept keeps
// and in the resemblance a probe ranks by.
func TestFeatureInformationWeighting(t *testing.T) {
	d, s := testDaemon(t)
	base := time.Now().Add(-3 * time.Hour)
	// A family of concepts all share the token "green"; only one also has
	// the rare token "thorn".
	distil(t, d, "vision", "green alpha shape", base)
	distil(t, d, "vision", "green bravo shape", base.Add(10*time.Minute))
	distil(t, d, "vision", "green charlie shape", base.Add(20*time.Minute))
	distil(t, d, "vision", "green thorn shape", base.Add(30*time.Minute))

	if _, err := s.MergeDuplicates(); err != nil {
		t.Fatal(err)
	}
	calm(d)
	d.growConcepts()
	d.growFeatures()

	thorny, _ := s.ConceptByTemplate(attention.Normalize("green thorn shape"))
	if thorny == nil {
		t.Fatal("the shapes should distil")
	}

	// Composition by information: in the thorny concept's signature the
	// rare "thorn" outweighs the common "green", though both fill all of
	// its episodes.
	sig := featureNames(t, d, thorny.ID)
	if sig["tok:thorn"] == 0 || sig["tok:green"] == 0 {
		t.Fatalf("both tokens should be in the signature: %+v", sig)
	}
	if sig["tok:thorn"] <= sig["tok:green"] {
		t.Fatalf("the rare defining token should outweigh the common one: thorn %.2f green %.2f",
			sig["tok:thorn"], sig["tok:green"])
	}

	// Resemblance ranking by information: a probe sharing the rare token
	// lands on the thorny concept's experience, the rare feature carrying
	// the match.
	seeds, mediators := d.featureSeeds("green thorn shape")
	var reached bool
	for _, f := range seeds {
		if f.Template == attention.Normalize("green thorn shape") {
			reached = true
		}
	}
	if !reached {
		t.Fatalf("the rare feature should carry the probe to its concept: %+v", seeds)
	}
	var sawThorn bool
	for _, m := range mediators {
		if m.Content == "tok:thorn" {
			sawThorn = true
		}
	}
	if !sawThorn {
		t.Fatalf("the explaining mediators should include the rare feature: %+v", mediators)
	}
}
