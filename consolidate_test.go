package main

import (
	"database/sql"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func encodeAt(t *testing.T, s *Store, channel, content string, surprise float64, at time.Time) *Fragment {
	t.Helper()
	f := &Fragment{Channel: channel, Content: content, Template: Normalize(content),
		Surprise: surprise, Time: at}
	if _, err := s.Encode(f); err != nil {
		t.Fatal(err)
	}
	return f
}

// Five occurrences of one shape collapse to a prototype and an exemplar:
// the newest survives carrying the count, the most surprising other
// occurrence stays linked to it, and edges fold onto the prototype.
func TestMergeDuplicates(t *testing.T) {
	d, s := testDaemon(t)
	now := time.Now()

	var group []*Fragment
	for i := range 5 {
		f := encodeAt(t, s, "syslog", fmt.Sprintf("Disk full on /dev/disk1s%d", i+1),
			3.0, now.Add(time.Duration(i)*time.Minute))
		group = append(group, f)
	}
	// The third occurrence was the sharpest; it should be the exemplar.
	if _, err := s.db.Exec(`UPDATE fragments SET surprise = 9 WHERE id = ?`, group[2].ID); err != nil {
		t.Fatal(err)
	}
	other := encodeAt(t, s, "vitals", "memory pressure went high", 4.0, now)
	s.Link(group[0].ID, other.ID, 0.4) // an absorbed member's edge
	s.Link(other.ID, group[0].ID, 0.4)

	merged, err := s.MergeDuplicates()
	if err != nil {
		t.Fatal(err)
	}
	if merged != 3 {
		t.Fatalf("five occurrences should absorb three (prototype and exemplar stay), got %d", merged)
	}

	frags, _ := s.Dump()
	var proto, exemplar *Fragment
	for _, f := range frags {
		if f.Channel != "syslog" {
			continue
		}
		switch f.ID {
		case group[4].ID:
			proto = f
		case group[2].ID:
			exemplar = f
		default:
			t.Fatalf("unexpected survivor #%d %q", f.ID, f.Content)
		}
	}
	if proto == nil || exemplar == nil {
		t.Fatalf("expected the newest as prototype and the sharpest as exemplar, got %+v", frags)
	}
	if proto.Count != 4 {
		t.Fatalf("the prototype should carry the absorbed count, got %d", proto.Count)
	}
	if exemplar.Count != 1 {
		t.Fatalf("the exemplar keeps its own count, got %d", exemplar.Count)
	}

	// The absorbed member's edges moved to the prototype, both ways.
	if w := edgeWeight(t, s, proto.ID, other.ID); w == 0 {
		t.Fatal("absorbed out-edges should fold onto the prototype")
	}
	if w := edgeWeight(t, s, other.ID, proto.ID); w == 0 {
		t.Fatal("absorbed in-edges should fold onto the prototype")
	}
	// The exemplar pointer holds in both directions.
	if edgeWeight(t, s, proto.ID, exemplar.ID) == 0 || edgeWeight(t, s, exemplar.ID, proto.ID) == 0 {
		t.Fatal("the prototype and its exemplar should stay linked")
	}

	// Recall still answers the same query after the merge.
	resp, _, _ := d.handle(Request{Cmd: "recall", Query: "disk full"}, nil)
	if len(resp.Fragments) == 0 {
		t.Fatal("recall should still answer after merging")
	}
}

// Edge weights sum when several absorbed members point at one target, so
// repeated groundings keep their aggregate strength, capped at 1.0.
func TestMergeSumsEdgeWeights(t *testing.T) {
	_, s := testDaemon(t)
	now := time.Now()

	target := encodeAt(t, s, "tell", "the anchor", 4.0, now)
	var ids []uint64
	for i := range 4 {
		f := encodeAt(t, s, "shell", "run: make build", 3.0, now.Add(time.Duration(i)*time.Minute))
		s.Link(target.ID, f.ID, 0.3)
		ids = append(ids, f.ID)
	}
	if _, err := s.MergeDuplicates(); err != nil {
		t.Fatal(err)
	}
	proto := ids[3] // newest
	w := edgeWeight(t, s, target.ID, proto)
	// Three absorbed 0.3 edges fold into the prototype's own: capped sum.
	if w < 0.89 {
		t.Fatalf("folded edge should carry the summed grounding, got %.2f", w)
	}
}

// Below the threshold nothing merges; a pair of duplicates is two moments.
func TestMergeBelowThresholdUntouched(t *testing.T) {
	_, s := testDaemon(t)
	now := time.Now()
	encodeAt(t, s, "syslog", "Disk full on /dev/disk1s1", 3.0, now)
	encodeAt(t, s, "syslog", "Disk full on /dev/disk1s2", 3.0, now.Add(time.Minute))

	merged, err := s.MergeDuplicates()
	if err != nil {
		t.Fatal(err)
	}
	if merged != 0 {
		t.Fatalf("two occurrences are below the threshold, merged %d", merged)
	}
	if n, _ := s.Count(); n != 2 {
		t.Fatalf("both moments should survive, got %d", n)
	}
}

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
func TestStrongSince(t *testing.T) {
	_, s := testDaemon(t)
	now := time.Now()

	old := encodeAt(t, s, "syslog", "ancient history", 4.0, now.Add(-48*time.Hour))
	faded := encodeAt(t, s, "syslog", "faded moment", 4.0, now.Add(-time.Hour))
	if _, err := s.db.Exec(`UPDATE fragments SET weight = 0.5 WHERE id = ?`, faded.ID); err != nil {
		t.Fatal(err)
	}
	for i := range 40 {
		encodeAt(t, s, "syslog", fmt.Sprintf("fresh moment %c", rune('a'+i)), 4.0,
			now.Add(time.Duration(-40+i)*time.Minute))
	}

	frags, err := s.StrongSince(now.Add(-24*time.Hour), replayFloor, replayCap)
	if err != nil {
		t.Fatal(err)
	}
	if len(frags) != replayCap {
		t.Fatalf("the cap should bound replay, got %d", len(frags))
	}
	for _, f := range frags {
		if f.ID == old.ID {
			t.Fatal("fragments before the since bound must not replay")
		}
		if f.ID == faded.ID {
			t.Fatal("fragments below the weight floor must not replay")
		}
	}
}

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

	resp, _, _ := d.handle(Request{Cmd: "recall", Query: "memory pressure"}, nil)
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
func TestCountColumnMigration(t *testing.T) {
	path := filepath.Join(t.TempDir(), "old.db")
	db, err := sql.Open("sqlite", "file:"+path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`CREATE TABLE fragments (
		id            INTEGER PRIMARY KEY AUTOINCREMENT,
		channel       TEXT    NOT NULL,
		content       TEXT    NOT NULL,
		template      TEXT    NOT NULL,
		surprise      REAL    NOT NULL,
		dopamine      REAL    NOT NULL,
		noradrenaline REAL    NOT NULL,
		serotonin     REAL    NOT NULL,
		at            INTEGER NOT NULL,
		weight        REAL    NOT NULL
	)`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO fragments
		(channel, content, template, surprise, dopamine, noradrenaline, serotonin, at, weight)
		VALUES ('syslog', 'old line', 'old line', 3.0, 0.2, 0.1, 0.5, ?, 1.0)`,
		time.Now().UnixNano()); err != nil {
		t.Fatal(err)
	}
	db.Close()

	s, err := OpenStore(path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	frags, err := s.Dump()
	if err != nil {
		t.Fatal(err)
	}
	if len(frags) != 1 || frags[0].Count != 1 {
		t.Fatalf("a pre-migration fragment should read back with count 1, got %+v", frags)
	}
}

// Edges fold onto the prototype per relation: a grounding stays a
// grounding and an order stays an order, with weights summed within each
// relation rather than across them.
func TestMergeFoldsEdgesPerRelation(t *testing.T) {
	_, s := testDaemon(t)
	now := time.Now()

	target := encodeAt(t, s, "tell", "the anchor", 4.0, now)
	var ids []uint64
	for i := range 4 {
		f := encodeAt(t, s, "shell", "run: make build", 3.0, now.Add(time.Duration(i)*time.Minute))
		if err := s.Link(target.ID, f.ID, 0.3); err != nil {
			t.Fatal(err)
		}
		if err := s.LinkRel(target.ID, f.ID, RelSuccessor, 0.2); err != nil {
			t.Fatal(err)
		}
		ids = append(ids, f.ID)
	}
	if _, err := s.MergeDuplicates(); err != nil {
		t.Fatal(err)
	}
	proto := ids[3] // newest survives

	temporal, _ := s.LinksFromRel([]uint64{target.ID}, RelTemporal)
	succ, _ := s.LinksFromRel([]uint64{target.ID}, RelSuccessor)
	var tw, sw float64
	for _, e := range temporal {
		if e.To == proto {
			tw = e.Weight
		}
	}
	for _, e := range succ {
		if e.To == proto {
			sw = e.Weight
		}
	}
	if tw < 0.89 {
		t.Fatalf("temporal weights should sum within their relation, got %.2f", tw)
	}
	if sw < 0.59 || sw > 0.61 {
		t.Fatalf("successor weights should sum within their relation, got %.2f", sw)
	}
}
