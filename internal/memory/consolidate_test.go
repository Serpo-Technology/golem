package memory

import (
	"database/sql"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	_ "modernc.org/sqlite"
)

func TestMergeDuplicates(t *testing.T) {
	s := openTestStore(t)
	now := time.Now()

	var group []*Fragment
	for i := range 5 {
		surprise := 3.0
		if i == 2 {
			// The third occurrence is the sharpest; it should be the exemplar.
			surprise = 9
		}
		f := encodeAt(t, s, "syslog", fmt.Sprintf("Disk full on /dev/disk1s%d", i+1),
			surprise, now.Add(time.Duration(i)*time.Minute))
		group = append(group, f)
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
	if hits, err := s.Recall("disk full", 3); err != nil || len(hits) == 0 {
		t.Fatalf("recall should still answer after merging: %v %v", hits, err)
	}
}

func TestMergeSumsEdgeWeights(t *testing.T) {
	s := openTestStore(t)
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

func TestMergeBelowThresholdUntouched(t *testing.T) {
	s := openTestStore(t)
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

func TestStrongSince(t *testing.T) {
	s := openTestStore(t)
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

	// Mirror the replay tuning: only strong moments, capped at one
	// night's worth.
	const floor, cap = 0.8, 32
	frags, err := s.StrongSince(now.Add(-24*time.Hour), floor, cap)
	if err != nil {
		t.Fatal(err)
	}
	if len(frags) != cap {
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

func TestMergeFoldsEdgesPerRelation(t *testing.T) {
	s := openTestStore(t)
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
