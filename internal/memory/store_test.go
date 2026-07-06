package memory

import (
	"database/sql"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"testing"
	"time"

	"golem/internal/limbic"
)

func tempStore(t *testing.T) (*Store, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "memory.db")
	s, err := OpenStore(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	return s, path
}

// Memory must survive a process boundary: encode, reopen, recall. There is
// no flush step in between, because encode itself is the commit.
func TestMemorySurvivesReopen(t *testing.T) {
	s, path := tempStore(t)
	_, err := s.Encode(&Fragment{
		Channel: "syslog", Content: "Disk full on /dev/disk1s5",
		Template: "Disk full on <path>", Surprise: 4.0,
		Limbic: limbic.State{Noradrenaline: 0.6}, Time: time.Now(),
	})
	if err != nil {
		t.Fatal(err)
	}
	id2, err := s.Encode(&Fragment{
		Channel: "syslog", Content: "Segfault at 0x7f in worker 3",
		Template: "Segfault at <hex> in worker <n>", Surprise: 5.0,
		Limbic: limbic.State{Noradrenaline: 0.95}, Time: time.Now(),
	})
	if err != nil {
		t.Fatal(err)
	}
	s.Close()

	re, err := OpenStore(path)
	if err != nil {
		t.Fatal(err)
	}
	defer re.Close()
	f, err := re.RecallByStress()
	if err != nil {
		t.Fatal(err)
	}
	if f == nil || f.ID != id2 {
		t.Fatalf("expected the segfault moment (#%d) back, got %+v", id2, f)
	}
	if f.Limbic.Noradrenaline != 0.95 {
		t.Fatalf("limbic snapshot did not survive: %+v", f.Limbic)
	}
}

// Recall refreshes weight, and the refresh itself must persist.
func TestRecallRefreshPersists(t *testing.T) {
	s, path := tempStore(t)
	if _, err := s.Encode(&Fragment{Content: "x", Surprise: 4, Time: time.Now()}); err != nil {
		t.Fatal(err)
	}
	for range 40 {
		if _, err := s.Sleep(0.10, 0.10); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := s.RecallByStress(); err != nil {
		t.Fatal(err)
	}
	s.Close()

	re, err := OpenStore(path)
	if err != nil {
		t.Fatal(err)
	}
	defer re.Close()
	frags, err := re.Dump()
	if err != nil {
		t.Fatal(err)
	}
	if len(frags) != 1 || frags[0].Weight != 1.0 {
		t.Fatalf("refreshed weight should persist at 1.0, got %+v", frags)
	}
}

// The SQL consolidation must prune, and sharper memories must outlive dull
// ones, exactly like the in-RAM hippocampus.
func TestStoreSleepPrunes(t *testing.T) {
	s, _ := tempStore(t)
	s.Encode(&Fragment{Content: "dull", Surprise: 2, Time: time.Now()})
	s.Encode(&Fragment{Content: "sharp", Surprise: 6, Time: time.Now()})

	var dullGone, sharpGone int
	for night := 1; night <= 5000; night++ {
		if _, err := s.Sleep(0.10, 0.10); err != nil {
			t.Fatal(err)
		}
		frags, err := s.Dump()
		if err != nil {
			t.Fatal(err)
		}
		alive := map[string]bool{}
		for _, f := range frags {
			alive[f.Content] = true
		}
		if dullGone == 0 && !alive["dull"] {
			dullGone = night
		}
		if sharpGone == 0 && !alive["sharp"] {
			sharpGone = night
		}
		if len(frags) == 0 {
			break
		}
	}
	if dullGone == 0 || sharpGone == 0 || dullGone >= sharpGone {
		t.Fatalf("dull should die first and both eventually: dull=%d sharp=%d", dullGone, sharpGone)
	}
}

// The decay law in SQL and the one in memory.go are the same law. If they
// drift apart, the demo brain and the daemon brain forget differently.
func TestDecayLawMatchesHippocampus(t *testing.T) {
	for _, surprise := range []float64{0.5, 2, 4.5, 7} {
		ram := NewHippocampus()
		ram.Encode(&Fragment{Content: "x", Surprise: surprise})
		ram.Sleep(0.10, 0.0) // floor 0 so nothing is pruned

		s, _ := tempStore(t)
		s.Encode(&Fragment{Content: "x", Surprise: surprise, Time: time.Now()})
		if _, err := s.Sleep(0.10, 0.0); err != nil {
			t.Fatal(err)
		}

		ramW := ram.Dump()[0].Weight
		frags, err := s.Dump()
		if err != nil {
			t.Fatal(err)
		}
		sqlW := frags[0].Weight
		if math.Abs(ramW-sqlW) > 1e-9 {
			t.Fatalf("decay laws diverge at surprise=%.1f: ram=%.12f sql=%.12f", surprise, ramW, sqlW)
		}
	}
}

func TestRecallByQuery(t *testing.T) {
	s, _ := tempStore(t)
	s.Encode(&Fragment{Channel: "syslog", Content: "Disk full on /dev/disk1s5",
		Template: "Disk full on <path>", Surprise: 4, Limbic: limbic.State{Noradrenaline: 0.5}, Time: time.Now()})
	s.Encode(&Fragment{Channel: "syslog", Content: "Connection refused peer=10.0.0.7",
		Template: "Connection refused peer=<ip>", Surprise: 3, Limbic: limbic.State{Noradrenaline: 0.9}, Time: time.Now()})

	frags, err := s.Recall("disk", 5)
	if err != nil {
		t.Fatal(err)
	}
	if len(frags) != 1 || frags[0].Content != "Disk full on /dev/disk1s5" {
		t.Fatalf("query recall failed: %+v", frags)
	}
}

// Links bind, never weaken by rebinding, strengthen with practice, fade
// each night, and die with their endpoints.
func TestLinksLifecycle(t *testing.T) {
	s, _ := tempStore(t)
	a, _ := s.Encode(&Fragment{Content: "a", Surprise: 6, Time: time.Now()})
	b, _ := s.Encode(&Fragment{Content: "b", Surprise: 6, Time: time.Now()})

	if err := s.Link(a, b, 0.5); err != nil {
		t.Fatal(err)
	}
	s.Link(b, a, 0.5)
	s.Link(a, b, 0.3) // a weaker rebind must not downgrade the edge

	edges, err := s.LinksFrom([]uint64{a})
	if err != nil || len(edges) != 1 {
		t.Fatalf("expected one edge from a: %v %v", edges, err)
	}
	if edges[0].Weight != 0.5 {
		t.Fatalf("rebind downgraded the edge: %.2f", edges[0].Weight)
	}

	if err := s.StrengthenLinks(edges, 0.6); err != nil {
		t.Fatal(err)
	}
	edges, _ = s.LinksFrom([]uint64{a})
	if edges[0].Weight != 1.0 {
		t.Fatalf("practice should cap at 1.0, got %.2f", edges[0].Weight)
	}
	back, _ := s.LinksFrom([]uint64{b})
	if len(back) != 1 || back[0].Weight != 1.0 {
		t.Fatalf("practice should strengthen both directions, got %+v", back)
	}

	s.Sleep(0.0, 0.0) // one night: links fade by the nightly factor
	edges, _ = s.LinksFrom([]uint64{a})
	if edges[0].Weight >= 1.0 {
		t.Fatalf("links should fade nightly, got %.2f", edges[0].Weight)
	}

	s.Sleep(0.10, 2.0) // a floor above every weight prunes both fragments
	edges, _ = s.LinksFrom([]uint64{a})
	if len(edges) != 0 {
		t.Fatalf("links should die with their endpoints: %v", edges)
	}
}

// An edge that fades below the link floor dies even while both of its
// endpoints live on.
func TestLinkFloorPrunesFadedEdges(t *testing.T) {
	s, _ := tempStore(t)
	a, _ := s.Encode(&Fragment{Content: "a", Surprise: 6, Time: time.Now()})
	b, _ := s.Encode(&Fragment{Content: "b", Surprise: 6, Time: time.Now()})
	s.Link(a, b, 0.051)

	if _, err := s.Sleep(0.0, 0.0); err != nil {
		t.Fatal(err)
	}
	edges, _ := s.LinksFrom([]uint64{a})
	if len(edges) != 0 {
		t.Fatalf("a faded edge should be pruned by the link floor: %+v", edges)
	}
	if n, _ := s.Count(); n != 2 {
		t.Fatalf("the fragments themselves should survive, got %d", n)
	}
}

// Checkpointing must actually fold the WAL back into the database file:
// after a TRUNCATE checkpoint the -wal file is zero bytes. A passive
// checkpoint or an ignored misspelled pragma would leave bytes behind.
func TestCheckpointTruncatesWAL(t *testing.T) {
	s, path := tempStore(t)
	for i := range 50 {
		if _, err := s.Encode(&Fragment{Content: fmt.Sprintf("filler %d", i),
			Surprise: 4, Time: time.Now()}); err != nil {
			t.Fatal(err)
		}
	}
	wal := path + "-wal"
	if fi, err := os.Stat(wal); err != nil || fi.Size() == 0 {
		t.Fatalf("the WAL should have grown before the checkpoint: %v", err)
	}
	if err := s.Checkpoint(); err != nil {
		t.Fatal(err)
	}
	fi, err := os.Stat(wal)
	if err != nil {
		t.Fatal(err)
	}
	if fi.Size() != 0 {
		t.Fatalf("a truncating checkpoint should empty the WAL, %d bytes left", fi.Size())
	}
}

// Calibration state roundtrips and upserts per channel.
func TestCalibRoundtrip(t *testing.T) {
	s, _ := tempStore(t)
	if err := s.SaveCalib("syslog", 412, 5.2, 1.7); err != nil {
		t.Fatal(err)
	}
	if err := s.SaveCalib("syslog", 500, 5.5, 1.9); err != nil {
		t.Fatal(err)
	}
	m, err := s.LoadCalibs()
	if err != nil {
		t.Fatal(err)
	}
	cs, ok := m["syslog"]
	if !ok || len(m) != 1 || cs.N != 500 || cs.Mean != 5.5 || cs.Vari != 1.9 {
		t.Fatalf("calibration did not roundtrip: %+v", m)
	}
}

// Association counts are order-insensitive and cumulative.
func TestBumpAssoc(t *testing.T) {
	s, _ := tempStore(t)
	c1, err := s.BumpAssoc("run: make test", "Disk full on <path>")
	if err != nil || c1 != 1 {
		t.Fatalf("first bump: %d %v", c1, err)
	}
	c2, err := s.BumpAssoc("Disk full on <path>", "run: make test")
	if err != nil || c2 != 2 {
		t.Fatalf("reversed bump should hit the same pair: %d %v", c2, err)
	}
}

func TestModelRoundtrip(t *testing.T) {
	s, _ := tempStore(t)
	in := map[string]float64{"Ping seq=<n>": 41.5, "Disk full on <path>": 3.25}
	if err := s.SaveModel("syslog", in); err != nil {
		t.Fatal(err)
	}
	out, err := s.LoadModel("syslog")
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != len(in) || out["Ping seq=<n>"] != 41.5 || out["Disk full on <path>"] != 3.25 {
		t.Fatalf("model did not roundtrip: %+v", out)
	}
	chans, err := s.ModelChannels()
	if err != nil || len(chans) != 1 || chans[0] != "syslog" {
		t.Fatalf("channel listing wrong: %v %v", chans, err)
	}
}

func TestBumpSuccIsOrderSensitive(t *testing.T) {
	s, _ := tempStore(t)
	c1, err := s.BumpSucc("run: make test", "run: make deploy")
	if err != nil || c1 != 1 {
		t.Fatalf("first bump: %d %v", c1, err)
	}
	c2, err := s.BumpSucc("run: make test", "run: make deploy")
	if err != nil || c2 != 2 {
		t.Fatalf("same order should accumulate: %d %v", c2, err)
	}
	c3, err := s.BumpSucc("run: make deploy", "run: make test")
	if err != nil || c3 != 1 {
		t.Fatalf("reversed order is a different pair: %d %v", c3, err)
	}
}

// Two relations may hold between the same pair of nodes without
// clobbering each other, and a rel-filtered read sees only its own.
func TestLinkRelationsCoexist(t *testing.T) {
	s, _ := tempStore(t)
	a := encodeAt(t, s, "tell", "the cue", 1.0, time.Now())
	b := encodeAt(t, s, "shell", "run: make test", 1.0, time.Now())

	if err := s.Link(a.ID, b.ID, 0.4); err != nil {
		t.Fatal(err)
	}
	if err := s.LinkRel(a.ID, b.ID, RelSuccessor, 0.7); err != nil {
		t.Fatal(err)
	}

	all, err := s.LinksFrom([]uint64{a.ID})
	if err != nil || len(all) != 2 {
		t.Fatalf("both relations should leave the node: %+v %v", all, err)
	}
	succ, err := s.LinksFromRel([]uint64{a.ID}, RelSuccessor)
	if err != nil || len(succ) != 1 || succ[0].Weight != 0.7 {
		t.Fatalf("rel filter should see one edge: %+v %v", succ, err)
	}

	// Strengthening one relation must not bleed into the other.
	if err := s.StrengthenLinks(succ, 0.1); err != nil {
		t.Fatal(err)
	}
	temporal, _ := s.LinksFromRel([]uint64{a.ID}, RelTemporal)
	if len(temporal) != 1 || temporal[0].Weight != 0.4 {
		t.Fatalf("temporal edge should be untouched: %+v", temporal)
	}
	succ, _ = s.LinksFromRel([]uint64{a.ID}, RelSuccessor)
	if math.Abs(succ[0].Weight-0.8) > 1e-9 {
		t.Fatalf("successor edge should have gained: %+v", succ)
	}
}

// A database from before edge relations and node kinds reads back whole:
// links carry the temporal relation, fragments read as episodic moments.
func TestRelAndKindMigration(t *testing.T) {
	path := filepath.Join(t.TempDir(), "old.db")
	db, err := sql.Open("sqlite", "file:"+path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`
		CREATE TABLE fragments (
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
		);
		CREATE TABLE links (
			from_id INTEGER NOT NULL,
			to_id   INTEGER NOT NULL,
			weight  REAL    NOT NULL,
			PRIMARY KEY (from_id, to_id)
		);
		INSERT INTO fragments (channel, content, template, surprise, dopamine, noradrenaline, serotonin, at, weight)
			VALUES ('syslog', 'old line', 'old line', 3.0, 0.2, 0.1, 0.5, 1000, 1.0),
			       ('vitals', 'old vitals', 'old vitals', 2.0, 0.2, 0.1, 0.5, 2000, 1.0);
		INSERT INTO links (from_id, to_id, weight) VALUES (1, 2, 0.6), (2, 1, 0.6);`); err != nil {
		t.Fatal(err)
	}
	db.Close()

	s, err := OpenStore(path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	frags, err := s.Dump()
	if err != nil || len(frags) != 2 {
		t.Fatalf("old fragments should read back: %v %v", frags, err)
	}
	for _, f := range frags {
		if f.Kind != KindEpisodic {
			t.Fatalf("a pre-migration fragment is a moment, got kind %q", f.Kind)
		}
	}
	edges, err := s.LinksFrom([]uint64{1})
	if err != nil || len(edges) != 1 {
		t.Fatalf("old links should survive the rebuild: %+v %v", edges, err)
	}
	if edges[0].Rel != RelTemporal || edges[0].Weight != 0.6 {
		t.Fatalf("a pre-migration edge is temporal co-occurrence: %+v", edges[0])
	}

	// The rebuilt table must accept the wider primary key.
	if err := s.LinkRel(1, 2, RelSuccessor, 0.3); err != nil {
		t.Fatal(err)
	}
	if both, _ := s.LinksFrom([]uint64{1}); len(both) != 2 {
		t.Fatalf("the rebuilt table should hold two relations per pair: %+v", both)
	}
}

// A naming bond is governed by the nightly recount alone: the generic link
// decay leaves it standing while ordinary associative edges thin, so a bond the
// recount is holding frozen (its word's company starved) is not surrendered to
// disuse behind the recount's back.
func TestNamesEdgeSparedNightlyDecay(t *testing.T) {
	s := openTestStore(t)
	now := time.Now()
	word := &Fragment{Kind: KindWord, Channel: "word", Content: "dog", Template: "dog", Lang: "en", Time: now}
	if _, err := s.Encode(word); err != nil {
		t.Fatal(err)
	}
	concept := &Fragment{Kind: KindConcept, Channel: "concept", Content: "a small brown dog",
		Template: "a small brown dog", Category: "noun", Time: now}
	if _, err := s.Encode(concept); err != nil {
		t.Fatal(err)
	}
	moment := encodeAt(t, s, "vision", "a small brown dog", 0, now)
	if err := s.LinkRel(concept.ID, moment.ID, RelGrounds, 0.8); err != nil {
		t.Fatal(err)
	}
	if err := s.LinkRel(word.ID, concept.ID, RelNames, 0.784); err != nil {
		t.Fatal(err)
	}
	if err := s.LinkRel(word.ID, moment.ID, RelTemporal, 0.5); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Sleep(0.1, 0.05); err != nil {
		t.Fatal(err)
	}
	if w := edgeWeightRel(t, s, word.ID, concept.ID, RelNames); w != 0.784 {
		t.Fatalf("a naming bond must not thin under the generic decay: %v", w)
	}
	if w := edgeWeightRel(t, s, word.ID, moment.ID, RelTemporal); w >= 0.5 {
		t.Fatalf("ordinary company must still decay: %v", w)
	}
}
