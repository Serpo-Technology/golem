package daemon

import (
	"strings"
	"testing"
	"time"

	"golem/internal/attention"
	"golem/internal/memory"
	"golem/internal/senses"
	"golem/internal/wire"
)

// diskDay distils one syslog shape into a concept with a grown
// signature: the recurring tokens, the channel, and how it felt.
func diskDay(t *testing.T, d *Daemon, s *memory.Store) *memory.Fragment {
	t.Helper()
	for i := range 4 {
		d.recent = nil
		d.chem.SpikeNoradrenaline(0.8) // the alarms were felt
		d.process(senses.Observation{Channel: "syslog",
			Content: "Disk full on /dev/disk1s9", At: time.Now().Add(-2 * time.Hour).Add(time.Duration(i) * time.Minute)},
			true, true)
	}
	d.recent = nil
	if _, err := s.MergeDuplicates(); err != nil {
		t.Fatal(err)
	}
	calm(d)
	d.growConcepts()
	d.growFeatures()
	settleChem(d)
	c, _ := s.ConceptByTemplate(attention.Normalize("Disk full on /dev/disk1s9"))
	if c == nil {
		t.Fatal("the shape should distil")
	}
	return c
}

// featureNames reads a concept's signature off the graph.
func featureNames(t *testing.T, d *Daemon, conceptID uint64) map[string]float64 {
	t.Helper()
	edges, err := d.store.LinksFromRel([]uint64{conceptID}, memory.RelFeature)
	if err != nil {
		t.Fatal(err)
	}
	ids := make([]uint64, 0, len(edges))
	for _, e := range edges {
		ids = append(ids, e.To)
	}
	feats, err := d.store.FragmentsByIDs(ids)
	if err != nil {
		t.Fatal(err)
	}
	byID := make(map[uint64]string, len(feats))
	for _, f := range feats {
		byID[f.ID] = f.Content
	}
	out := map[string]float64{}
	for _, e := range edges {
		out[byID[e.To]] = e.Weight
	}
	return out
}

// Consolidation hangs an interpretable signature under each concept:
// the recurring open-class words of its shape, its channel, and the
// felt chemistry, with placeholders never becoming attributes.
func TestFeatureSignatureGrows(t *testing.T) {
	d, s := testDaemon(t)
	c := diskDay(t, d, s)
	sig := featureNames(t, d, c.ID)
	for _, want := range []string{"tok:disk", "tok:full", "ch:syslog", "feel:stress"} {
		if sig[want] == 0 {
			t.Fatalf("the signature should carry %s: %+v", want, sig)
		}
	}
	for name := range sig {
		if name == "tok:path" || name == "tok:n" {
			t.Fatalf("a placeholder must not become an attribute: %+v", sig)
		}
	}
}

// A signature stays sparse: many distinct tokens cap at the top K.
func TestFeatureSignatureSparse(t *testing.T) {
	d, s := testDaemon(t)
	long := "alpha bravo charlie delta echofoo foxtrot golf hotel india juliet kilo lima mike november oscar papa"
	for i := range 4 {
		d.recent = nil
		d.process(senses.Observation{Channel: "syslog", Content: long,
			At: time.Now().Add(-time.Hour).Add(time.Duration(i) * time.Minute)}, true, true)
	}
	d.recent = nil
	if _, err := s.MergeDuplicates(); err != nil {
		t.Fatal(err)
	}
	calm(d)
	d.growConcepts()
	d.growFeatures()
	c, _ := s.ConceptByTemplate(attention.Normalize(long))
	if c == nil {
		t.Fatal("the shape should distil")
	}
	if sig := featureNames(t, d, c.ID); len(sig) > featureTopK {
		t.Fatalf("a signature must stay sparse, got %d attributes", len(sig))
	}
}

// The done-when: a phrasing never seen, sharing features with a known
// concept, recalls that concept's experience by overlap, and the shared
// features come back to explain the match.
func TestRecallByFeatureOverlap(t *testing.T) {
	d, s := testDaemon(t)
	diskDay(t, d, s)

	// No moment contains this phrase; the words have never been told.
	query := "full disk emergency"
	if direct, err := s.Recall(query, 3); err != nil || len(direct) != 0 {
		t.Fatalf("the literal path must find nothing for the novel phrasing: %v %v", direct, err)
	}
	seeds, mediators, err := d.recallSeeds(query)
	if err != nil {
		t.Fatal(err)
	}
	var reached bool
	for _, f := range seeds {
		if strings.Contains(f.Content, "Disk full") {
			reached = true
		}
	}
	if !reached {
		t.Fatalf("overlap should land the novel phrasing on the lived experience: %+v", seeds)
	}
	shared := map[string]bool{}
	for _, m := range mediators {
		if m.Kind == memory.KindFeature {
			shared[m.Content] = true
		}
	}
	if !shared["tok:full"] || !shared["tok:disk"] {
		t.Fatalf("the shared features should explain the match: %+v", shared)
	}
}

// One shared word is coincidence, not resemblance: below the overlap
// minimum the feature path stays silent.
func TestOverlapNeedsMoreThanOneFeature(t *testing.T) {
	d, s := testDaemon(t)
	diskDay(t, d, s)
	seeds, mediators := d.featureSeeds("disk jockey")
	if len(seeds) != 0 || len(mediators) != 0 {
		t.Fatalf("a single shared token must not match: %v %v", seeds, mediators)
	}
}

// The whole path works over the wire: a recall request with the novel
// phrasing returns the resembled moments plus the feature mediators.
func TestRecallVerbReadsFeatureLayer(t *testing.T) {
	d, s := testDaemon(t)
	diskDay(t, d, s)
	resp, _, _ := d.handle(wire.Request{Cmd: "recall", Query: "full disk emergency"}, nil)
	if !resp.OK {
		t.Fatalf("recall failed: %s", resp.Error)
	}
	var moment, feature bool
	for _, f := range resp.Fragments {
		if strings.Contains(f.Content, "Disk full") {
			moment = true
		}
		if f.Kind == memory.KindFeature {
			feature = true
		}
	}
	if !moment || !feature {
		t.Fatalf("the reply should carry the moment and the explaining features: %+v", resp.Fragments)
	}
}

// Resemblance is a kind of thought: two concepts sharing signature
// tokens, never adjacent in time, associate through the feature layer
// and the thought speaks as likeness.
func TestResemblanceThought(t *testing.T) {
	d, s := testDaemon(t)
	// Two shapes sharing their frame, lived hours apart so no temporal
	// or successor edge connects their episodes.
	base := time.Now().Add(-8 * time.Hour)
	for i := range 4 {
		d.recent = nil
		d.process(senses.Observation{Channel: "screen", Content: "app Slack frontmost",
			At: base.Add(time.Duration(i) * time.Minute)}, true, true)
	}
	d.recent = nil
	for i := range 4 {
		d.recent = nil
		d.process(senses.Observation{Channel: "screen", Content: "app Xcode frontmost",
			At: base.Add(4*time.Hour + time.Duration(i)*time.Minute)}, true, true)
	}
	d.recent = nil
	if _, err := s.MergeDuplicates(); err != nil {
		t.Fatal(err)
	}
	calm(d)
	d.growConcepts()
	d.growFeatures()
	a, _ := s.ConceptByTemplate(attention.Normalize("app Slack frontmost"))
	b, _ := s.ConceptByTemplate(attention.Normalize("app Xcode frontmost"))
	if a == nil || b == nil {
		t.Fatal("both shapes should distil")
	}
	// The thought channel knows enough shapes that a fresh one encodes.
	for i := range 6 {
		d.process(senses.Observation{Channel: thoughtChannel,
			Content: "thought: filler shape " + string(rune('a'+i)), At: time.Now()}, true, true)
	}
	settleChem(d)
	d.ws.excite(a.ID, 1.0)
	d.lastObs = time.Now().Add(-time.Minute)
	d.lastSurprise = time.Now().Add(-time.Minute)

	d.startThinkRound()
	d.settleThinking()

	thought := newestThought(t, s)
	if thought == nil || !strings.Contains(thought.Content, "is like") {
		t.Fatalf("disconnected kin should associate by resemblance: %+v", thought)
	}
	if d.ws.heat(b.ID) == 0 {
		t.Fatal("the resembled concept should warm, so the chain can continue from it")
	}
}

// Features die by disconnection: when the experience and its concept
// go, the attributes follow, and nothing dangles.
func TestFeaturesDieWithTheirConcept(t *testing.T) {
	d, s := testDaemon(t)
	c := diskDay(t, d, s)
	if sig := featureNames(t, d, c.ID); len(sig) == 0 {
		t.Fatal("the concept should carry a signature")
	}
	// A night harsh enough to prune all episodic memory takes the
	// concept (disconnection), and the next takes its features.
	if _, err := s.Sleep(0.99, 0.99); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Sleep(0.99, 0.99); err != nil {
		t.Fatal(err)
	}
	feats, err := s.NodesByKind(memory.KindFeature)
	if err != nil {
		t.Fatal(err)
	}
	if len(feats) != 0 {
		t.Fatalf("orphaned features must die by disconnection: %+v", feats)
	}
}
