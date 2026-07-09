package daemon

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"golem/internal/memory"
	"golem/wire"
)

// Five occurrences of one shape collapse to a prototype and an exemplar:
// the newest survives carrying the count, the most surprising other
// occurrence stays linked to it, and edges fold onto the prototype.
// Edge weights sum when several absorbed members point at one target, so
// repeated groundings keep their aggregate strength, capped at 1.0.
// Below the threshold nothing merges; a pair of duplicates is two moments.
// A cue grounded by repetition still proposes its command after its
// teaching episodes merge into prototypes.
func TestMergePreservesGrounding(t *testing.T) {
	d, s := testDaemon(t)
	ground(t, d, "run the tests", "go test ./...", 5)

	before, err := d.decide("run the tests")
	if err != nil || before.Proposed == nil {
		t.Fatalf("grounding should propose before the merge: %+v %v", before.Proposed, err)
	}

	if _, err := s.MergeDuplicates(); err != nil {
		t.Fatal(err)
	}

	after, err := d.decide("run the tests")
	if err != nil {
		t.Fatal(err)
	}
	if after.Proposed == nil || after.Proposed.Command != "go test ./..." {
		t.Fatalf("grounding should survive the merge, got %+v", after.Proposed)
	}
	if after.Proposed.Saliency < actionFloor {
		t.Fatalf("merged grounding should stay above the action floor, got %.3f", after.Proposed.Saliency)
	}
}

// Replay weaves the day's strong cross-channel moments that fell inside
// the replay window; same-channel pairs and distant pairs stay unwoven.
func TestReplayWeavesCrossChannel(t *testing.T) {
	d, s := testDaemon(t)
	base := time.Now().Add(-2 * time.Hour)

	syslog := encodeAt(t, s, "syslog", "Disk full on /dev/disk1s9", 5.0, base)
	vitals := encodeAt(t, s, "vitals", "memory pressure went high", 4.0, base.Add(2*time.Minute))
	shell := encodeAt(t, s, "shell", "command failed with exit status 1", 4.0, base.Add(3*time.Minute))
	syslog2 := encodeAt(t, s, "syslog", "Connection refused peer", 4.0, base.Add(4*time.Minute))
	far := encodeAt(t, s, "vitals", "cpu load went high", 4.0, base.Add(90*time.Minute))

	moments, woven := d.replay(base.Add(-time.Hour))
	if moments != 5 || woven == 0 {
		t.Fatalf("replay should walk the day and weave: %d moments, %d woven", moments, woven)
	}

	if edgeWeight(t, s, vitals.ID, syslog.ID) == 0 || edgeWeight(t, s, syslog.ID, vitals.ID) == 0 {
		t.Fatal("cross-channel moments inside the window should bind both ways")
	}
	if edgeWeight(t, s, shell.ID, vitals.ID) == 0 {
		t.Fatal("every cross-channel pair inside the window should bind")
	}
	if edgeWeight(t, s, syslog2.ID, syslog.ID) != 0 {
		t.Fatal("same-channel pairs are not replay's business")
	}
	if edgeWeight(t, s, far.ID, vitals.ID) != 0 {
		t.Fatal("moments outside the replay window must not bind")
	}
}

// Replay never weakens an edge waking already built.
func TestReplayKeepsStrongerEdges(t *testing.T) {
	d, s := testDaemon(t)
	base := time.Now().Add(-time.Hour)
	a := encodeAt(t, s, "syslog", "Disk full on /dev/disk1s9", 5.0, base)
	b := encodeAt(t, s, "vitals", "memory pressure went high", 4.0, base.Add(time.Minute))
	s.Link(a.ID, b.ID, 0.9)

	d.replay(base.Add(-time.Minute))
	if w := edgeWeight(t, s, a.ID, b.ID); w != 0.9 {
		t.Fatalf("replay must keep the stronger live edge, got %.2f", w)
	}
	if w := edgeWeight(t, s, b.ID, a.ID); w != replayWeight {
		t.Fatalf("the missing reverse edge should be woven at replay weight, got %.2f", w)
	}
}

// StrongSince honors the floor, the since bound, and the cap.
// The full night composes: replay weaves the incident, the merge folds
// the day's repeats, and the morning recall walks from one sense to what
// the other senses saw.
func TestSleepCycleConsolidates(t *testing.T) {
	d, s := testDaemon(t)
	base := time.Now().Add(-3 * time.Hour)

	for i := range 5 {
		encodeAt(t, s, "syslog", fmt.Sprintf("Disk full on /dev/disk1s%d", i+1), 3.0,
			base.Add(time.Duration(i)*12*time.Minute))
	}
	vitals := encodeAt(t, s, "vitals", "memory pressure went high", 4.0, base.Add(49*time.Minute))

	d.sleepCycle()

	resp, _, _ := d.handle(wire.Request{Cmd: "recall", Query: "memory pressure"}, nil)
	if len(resp.Fragments) < 2 {
		t.Fatalf("the morning recall should carry replayed context, got %+v", resp.Fragments)
	}
	var sawDisk bool
	for _, f := range resp.Fragments {
		if f.ID != vitals.ID && strings.Contains(f.Content, "Disk full") {
			sawDisk = true
		}
	}
	if !sawDisk {
		t.Fatal("recalling one sense should surface what the other sense saw that day")
	}
}

// A database written before the count column existed gains it on open.
// Edges fold onto the prototype per relation: a grounding stays a
// grounding and an order stays an order, with weights summed within each
// relation rather than across them.

// Consolidation reasons over the day's concepts unprompted: taught that the dog chased the cat
// and (separately) that the wolf feared the deer, with the wolf and the dog sharing a predator
// feature, a night draws that the wolf probably chased the cat too -- a guess nobody asked for,
// held in mind and never written to the graph, so the idle reasoning stays honest.
func TestConsolidationDrawsUnpromptedGuess(t *testing.T) {
	d, s := testDaemon(t)
	dog := groundNoun(t, d, "dog")
	cat := groundNoun(t, d, "cat")
	wolf := groundNoun(t, d, "wolf")
	groundNoun(t, d, "deer")
	d.lastSleep = time.Time{} // the whole session counts as the day to reason over

	say(t, d, "the dog chased the cat")
	say(t, d, "the wolf feared the deer")
	feat := &memory.Fragment{Kind: memory.KindFeature, Channel: "feature", Content: "predator",
		Template: "predator", Limbic: d.chem.Snapshot(), Time: time.Now()}
	if _, err := s.Encode(feat); err != nil {
		t.Fatal(err)
	}
	s.SetLinkRel(dog.ID, feat.ID, memory.RelFeature, 1.0)
	s.SetLinkRel(wolf.ID, feat.ID, memory.RelFeature, 1.0)

	// Nobody asked whether the wolf chased the cat, and it is no observed fact.
	if _, held := d.held.holds("chase", wolf.ID, cat.ID, heldInferred); held {
		t.Fatal("the guess should not be held before consolidation reasons")
	}

	topics, drawn, _ := d.consolidateReasoning()
	if topics == 0 {
		t.Fatal("consolidation should reason over the day's concepts")
	}
	if drawn == 0 {
		t.Fatal("consolidation should draw at least one fresh conclusion")
	}

	// The unprompted conclusion is now held as a guess.
	if _, held := d.held.holds("chase", wolf.ID, cat.ID, heldInferred); !held {
		t.Fatalf("consolidation should draw that the wolf probably chased the cat (drew %d)", drawn)
	}
	// Honesty: it is held, never asserted as an observed fact in the graph.
	if d.propositionHolds(wolf, "chase", cat.ID) {
		t.Fatal("the idle guess must stay a held guess, never an observed fact")
	}
}

// Consolidation reasons about learned concepts, never the discourse participants: told a
// first-person fact whose agent is the speaker, the night must not draw guesses attributed to
// the speaker or to GOLEM itself, which would fabricate their doings from a look-alike.
func TestConsolidationSkipsParticipants(t *testing.T) {
	d, _ := testDaemon(t)
	groundNoun(t, d, "cat")
	d.lastSleep = time.Time{}
	say(t, d, "I chased the cat") // the agent resolves to the speaker, a participant concept

	d.consolidateReasoning()

	for i := range d.held.props {
		if agent := d.held.props[i].agent; d.isSystemConcept(agent) {
			t.Fatalf("consolidation reasoned about a participant/system concept (agent %d)", agent)
		}
	}
}
