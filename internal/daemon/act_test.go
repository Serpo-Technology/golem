package daemon

import (
	"strings"
	"testing"
	"time"

	"golem/internal/attention"
	"golem/internal/memory"
)

// auditHas reports whether the recent trail holds a row with the given
// cue prefix, returning its outcome.
func auditHas(t *testing.T, s *memory.Store, cue string) (string, bool) {
	t.Helper()
	entries, err := s.AuditTail(20)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if e.Cue == cue {
			return e.Outcome, true
		}
	}
	return "", false
}

// The act space parses and minds its boundaries: act templates resolve,
// foreign shapes are refused, and the mental-teach reaches acts.
func TestActSpaceSpansFaculties(t *testing.T) {
	if name, ok := actNameOf("act(consolidate)"); !ok || name != actConsolidate {
		t.Fatalf("act template should parse, got %q %v", name, ok)
	}
	if _, ok := actNameOf("act(reboot)"); ok {
		t.Fatal("an unknown faculty must be refused")
	}
	if _, ok := actNameOf("walk(temporal,weight)"); ok {
		t.Fatal("a walk template is not an act")
	}
	if gloss, ok := opGloss("act(recall)"); !ok || gloss != "recall" {
		t.Fatalf("an act's gloss is its faculty, got %q %v", gloss, ok)
	}
	if gloss, ok := opGloss("walk(grounds,stress)"); !ok || gloss != "walk grounds by stress" {
		t.Fatalf("a walk's gloss survives the shared resolver, got %q %v", gloss, ok)
	}
}

// Alarm with a calm grounding calls for relevant memory: the recall act
// outbids the walks, runs the same retrieval the CLI verb runs, and the
// firing lands on the audit trail.
func TestUnpromptedRecallAct(t *testing.T) {
	d, s := testDaemon(t)
	feedShape(t, s, "syslog", "Disk full on /dev/disk1s9", 4, time.Now().Add(-2*time.Hour))
	if _, err := s.MergeDuplicates(); err != nil {
		t.Fatal(err)
	}
	calm(d)
	d.growConcepts()
	c, _ := s.ConceptByTemplate(attention.Normalize("Disk full on /dev/disk1s9"))
	if c == nil {
		t.Fatal("the shape should distil")
	}
	settleChem(d)
	d.chem.SpikeNoradrenaline(0.8)
	d.ws.excite(c.ID, 1.0)
	d.lastObs = time.Now().Add(-time.Minute)
	d.lastSurprise = time.Now().Add(-time.Minute)

	d.startThinkRound()
	d.settleThinking()

	outcome, ok := auditHas(t, s, "act:recall")
	if !ok {
		t.Fatal("the alarmed quiet mind should have recalled unprompted")
	}
	if !strings.Contains(outcome, "found") || strings.Contains(outcome, "found 0") {
		t.Fatalf("the recall should have found the lived moments: %q", outcome)
	}
}

// Clutter plus sleep pressure calls for tidying: the consolidate act
// runs the same consolidation the sleep verb runs, resets the clutter,
// and audits the firing.
func TestUnpromptedConsolidateAct(t *testing.T) {
	d, s := testDaemon(t)
	c := &memory.Fragment{Kind: memory.KindConcept, Channel: "concept", Content: "quiet shape",
		Template: "quiet shape", Category: "noun", Limbic: d.chem.Snapshot(), Time: time.Now()}
	if _, err := s.Encode(c); err != nil {
		t.Fatal(err)
	}
	calm(d)
	settleChem(d)
	d.encodesSinceSleep = 80
	d.ws.excite(c.ID, 1.0)
	d.lastObs = time.Now().Add(-time.Minute)
	d.lastSurprise = time.Now().Add(-time.Minute)

	d.startThinkRound()
	d.settleThinking()

	if _, ok := auditHas(t, s, "act:consolidate"); !ok {
		t.Fatal("a cluttered quiet mind should have consolidated unprompted")
	}
	if d.sleeps != 1 {
		t.Fatalf("the act should run the same night the sleep verb runs, sleeps=%d", d.sleeps)
	}
	if d.encodesSinceSleep != 0 {
		t.Fatal("consolidation should reset the clutter")
	}
}

// The mental-teach generalizes to faculties: a taught word warms its
// act's concept, and the warmth steers the next quiet round to that act
// even when the body signal alone would not have won.
func TestTaughtWordInvokesAct(t *testing.T) {
	d, s := testDaemon(t)
	if err := d.teachThought("tidy", "act", "consolidate", ""); err != nil {
		t.Fatal(err)
	}
	if err := d.teachThought("tidy", "act", "daydream", ""); err == nil {
		t.Fatal("an unknown act must be refused")
	}
	c := d.resolveConcept("tidy")
	if c == nil || c.Content != "consolidate" {
		t.Fatalf("the word should name the faculty, got %+v", c)
	}
	grounds, err := s.LinksFromRel([]uint64{c.ID}, memory.RelGrounds)
	if err != nil || len(grounds) != 1 {
		t.Fatalf("the concept should ground exactly the act node: %v %v", grounds, err)
	}
	nodes, err := s.FragmentsByIDs([]uint64{grounds[0].To})
	if err != nil || len(nodes) != 1 || nodes[0].Template != "act(consolidate)" {
		t.Fatalf("the grounding should reach the act's operation node: %+v", nodes)
	}

	// Saying the word warmed its concept; with only modest clutter the
	// warm name is what tips the act past the walks.
	calm(d)
	settleChem(d)
	d.encodesSinceSleep = 30
	d.lastObs = time.Now().Add(-time.Minute)
	d.lastSurprise = time.Now().Add(-time.Minute)

	d.startThinkRound()
	d.settleThinking()

	if _, ok := auditHas(t, s, "act:consolidate"); !ok {
		t.Fatal("the taught word should have steered the loop to its act")
	}
}

// The propose act stages behind the same gate the propose verb uses:
// a taught name warms it past deliberation, the proposal parks in the
// pending slot, and nothing runs.
func TestProposeActStagesBehindGate(t *testing.T) {
	d, s := testDaemon(t)
	feedShape(t, s, "shell", "run: echo hello", 4, time.Now().Add(-2*time.Hour))
	if _, err := s.MergeDuplicates(); err != nil {
		t.Fatal(err)
	}
	calm(d)
	d.growConcepts()
	c, _ := s.ConceptByTemplate(attention.Normalize("run: echo hello"))
	if c == nil || c.Category != "verb" {
		t.Fatalf("the command shape should be a verb concept: %+v", c)
	}
	if err := d.teachThought("deliver", "act", "propose", ""); err != nil {
		t.Fatal(err)
	}
	settleChem(d)
	d.ws.excite(c.ID, 1.0)
	d.lastObs = time.Now().Add(-time.Minute)
	d.lastSurprise = time.Now().Add(-time.Minute)

	d.startThinkRound()
	d.settleThinking()

	outcome, ok := auditHas(t, s, "act:propose")
	if !ok {
		t.Fatal("the named propose act should have staged")
	}
	if !strings.Contains(outcome, "echo hello") {
		t.Fatalf("the staged command should be the grounded one: %q", outcome)
	}
	if d.pending == nil || d.pending.Proposed == nil || d.pending.Proposed.Command != "echo hello" {
		t.Fatalf("the proposal should wait in the pending slot: %+v", d.pending)
	}
	if len(d.running) != 0 {
		t.Fatal("staging must never run anything")
	}
}

// Introspection reports the inner loop's own product: with a fresh
// thought and a faintly warm seed, the introspect act wins and speaks
// what was being thought.
func TestIntrospectAct(t *testing.T) {
	d, s := testDaemon(t)
	c := &memory.Fragment{Kind: memory.KindConcept, Channel: "concept", Content: "quiet shape",
		Template: "quiet shape", Category: "noun", Limbic: d.chem.Snapshot(), Time: time.Now()}
	if _, err := s.Encode(c); err != nil {
		t.Fatal(err)
	}
	settleChem(d)
	d.lastThought = "thought: after make test, make deploy"
	d.lastThoughtAt = time.Now()
	d.ws.excite(c.ID, 0.3)
	d.lastObs = time.Now().Add(-time.Minute)
	d.lastSurprise = time.Now().Add(-time.Minute)

	d.startThinkRound()
	d.settleThinking()

	outcome, ok := auditHas(t, s, "act:introspect")
	if !ok {
		t.Fatal("a mind with a fresh thought and nothing else warm should introspect")
	}
	if !strings.Contains(outcome, "after make test, make deploy") {
		t.Fatalf("introspection should report the last thought: %q", outcome)
	}
}

// A fired act cools: its refractory drops it from the next panels while
// the other faculties stay eligible.
func TestActRefractory(t *testing.T) {
	d, _ := testDaemon(t)
	now := time.Now()
	if got := len(d.actPanel(now)); got != len(actNames) {
		t.Fatalf("a rested mind offers every act, got %d", got)
	}
	d.lastAct = map[string]time.Time{"act(recall)": now}
	panel := d.actPanel(now)
	if len(panel) != len(actNames)-1 {
		t.Fatalf("a just-fired act should cool, got %d", len(panel))
	}
	for _, op := range panel {
		if op.name == actRecall {
			t.Fatal("the cooling act must not be offered")
		}
	}
	if got := len(d.actPanel(now.Add(time.Hour))); got != len(actNames) {
		t.Fatalf("a cooled act returns, got %d", got)
	}
}
