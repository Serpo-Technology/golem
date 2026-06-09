package main

import (
	"math"
	"path/filepath"
	"testing"
	"time"
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
		Limbic: LimbicState{Noradrenaline: 0.6}, Time: time.Now(),
	})
	if err != nil {
		t.Fatal(err)
	}
	id2, err := s.Encode(&Fragment{
		Channel: "syslog", Content: "Segfault at 0x7f in worker 3",
		Template: "Segfault at <hex> in worker <n>", Surprise: 5.0,
		Limbic: LimbicState{Noradrenaline: 0.95}, Time: time.Now(),
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
	for i := 0; i < 40; i++ {
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
		Template: "Disk full on <path>", Surprise: 4, Limbic: LimbicState{Noradrenaline: 0.5}, Time: time.Now()})
	s.Encode(&Fragment{Channel: "syslog", Content: "Connection refused peer=10.0.0.7",
		Template: "Connection refused peer=<ip>", Surprise: 3, Limbic: LimbicState{Noradrenaline: 0.9}, Time: time.Now()})

	frags, err := s.Recall("disk", 5)
	if err != nil {
		t.Fatal(err)
	}
	if len(frags) != 1 || frags[0].Content != "Disk full on /dev/disk1s5" {
		t.Fatalf("query recall failed: %+v", frags)
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
