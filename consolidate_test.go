package main

import (
	"database/sql"
	"fmt"
	"path/filepath"
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
	for i := 0; i < 5; i++ {
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
	for i := 0; i < 4; i++ {
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
