package main

import (
	"fmt"
	"io"
	"math"
	"net"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"
)

func startTestDaemon(t *testing.T) (home string, done chan error) {
	t.Helper()
	home = t.TempDir()
	cfg := DefaultConfig()
	cfg.SleepIntervalSec = 3600     // keep autonomous sleep out of the test's way
	cfg.Channels = ChannelsConfig{} // no built-in sensors; tests feed channels themselves

	d, err := NewDaemon(home, cfg, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	done = make(chan error, 1)
	go func() { done <- d.Run() }()

	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if conn, err := net.Dial("unix", socketPath(home)); err == nil {
			conn.Close()
			return home, done
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("daemon socket never came up")
	return "", nil
}

func TestDaemonEndToEnd(t *testing.T) {
	home, done := startTestDaemon(t)

	// Teach the daemon what routine looks like.
	for i := 0; i < 40; i++ {
		req := Request{Cmd: "observe", Channel: "testlog",
			Content: fmt.Sprintf("Ping seq=%d latency=%dms", i, 10+i%7)}
		if _, err := ipcCall(home, req); err != nil {
			t.Fatal(err)
		}
	}

	// An anomaly must reach consciousness and be encoded.
	resp, err := ipcCall(home, Request{Cmd: "observe", Channel: "testlog",
		Content: "Disk full on /dev/disk1s9"})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Observe == nil || !resp.Observe.Conscious {
		t.Fatalf("anomaly should be conscious: %+v", resp.Observe)
	}

	// Status reflects the channel's history.
	resp, err = ipcCall(home, Request{Cmd: "status"})
	if err != nil {
		t.Fatal(err)
	}
	st := resp.Status
	if st.Fragments < 1 {
		t.Fatalf("expected at least one fragment held, got %d", st.Fragments)
	}
	if ch := st.Channels["testlog"]; ch == nil || ch.Observed != 41 || ch.Conscious < 1 {
		t.Fatalf("channel stats wrong: %+v", st.Channels)
	}

	// Recall by query finds the moment.
	resp, err = ipcCall(home, Request{Cmd: "recall", Query: "disk"})
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.Fragments) != 1 || resp.Fragments[0].Channel != "testlog" {
		t.Fatalf("recall by query failed: %+v", resp.Fragments)
	}

	// Stop shuts the daemon down and removes the socket.
	if _, err := ipcCall(home, Request{Cmd: "stop"}); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("daemon exited with error: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("daemon did not exit after stop")
	}
	if _, err := os.Stat(socketPath(home)); !os.IsNotExist(err) {
		t.Fatal("socket file should be removed on shutdown")
	}
}

func testDaemon(t *testing.T) (*Daemon, *Store) {
	t.Helper()
	s, err := OpenStore(filepath.Join(t.TempDir(), "m.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	return &Daemon{
		store:       s,
		chem:        NewNeurochemistry(),
		out:         io.Discard,
		cfg:         DefaultConfig(),
		predictors:  make(map[string]*Predictor),
		calibrators: make(map[string]*Calibrator),
		stats:       make(map[string]*ChannelStat),
		drops:       make(map[string]*atomic.Int64),
	}, s
}

func edgeWeight(t *testing.T, s *Store, from, to uint64) float64 {
	t.Helper()
	edges, err := s.LinksFrom([]uint64{from})
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range edges {
		if e.To == to {
			return e.Weight
		}
	}
	return 0
}

// Activation must fall with distance and weak edges must not carry it.
// Edges that carried activation gain practice weight; skipped ones keep
// theirs.
func TestSpreadActivation(t *testing.T) {
	d, s := testDaemon(t)

	mk := func(content string) *Fragment {
		f := &Fragment{Content: content, Surprise: 4, Time: time.Now()}
		if _, err := s.Encode(f); err != nil {
			t.Fatal(err)
		}
		return f
	}
	a, b, c, far := mk("a"), mk("b"), mk("c"), mk("far")
	s.Link(a.ID, b.ID, 0.9)
	s.Link(b.ID, c.ID, 0.9)
	s.Link(a.ID, far.ID, 0.06) // too weak to carry activation

	cloud, err := d.spread([]*Fragment{a})
	if err != nil {
		t.Fatal(err)
	}
	if len(cloud) != 2 {
		t.Fatalf("expected b and c in the cloud, got %+v", cloud)
	}
	if cloud[0].ID != b.ID || cloud[1].ID != c.ID {
		t.Fatalf("activation should rank b before c: %+v", cloud)
	}
	if cloud[0].Activation <= cloud[1].Activation {
		t.Fatalf("activation should fall with distance: %.2f vs %.2f",
			cloud[0].Activation, cloud[1].Activation)
	}

	if w := edgeWeight(t, s, a.ID, b.ID); math.Abs(w-0.95) > 1e-9 {
		t.Fatalf("traversed edge a->b should be practiced to 0.95, got %.4f", w)
	}
	if w := edgeWeight(t, s, b.ID, c.ID); math.Abs(w-0.95) > 1e-9 {
		t.Fatalf("traversed edge b->c should be practiced to 0.95, got %.4f", w)
	}
	if w := edgeWeight(t, s, a.ID, far.ID); math.Abs(w-0.06) > 1e-9 {
		t.Fatalf("an edge below the activation floor must not be practiced, got %.4f", w)
	}
}

// The budget keeps the strongest moments; sorting happens before the cut.
func TestSpreadBudgetKeepsStrongest(t *testing.T) {
	d, s := testDaemon(t)

	seed := &Fragment{Content: "seed", Surprise: 4, Time: time.Now()}
	if _, err := s.Encode(seed); err != nil {
		t.Fatal(err)
	}
	var weakest1, weakest2 uint64
	for i := 0; i < 12; i++ {
		f := &Fragment{Content: fmt.Sprintf("n%d", i), Surprise: 4, Time: time.Now()}
		if _, err := s.Encode(f); err != nil {
			t.Fatal(err)
		}
		s.Link(seed.ID, f.ID, 0.92-0.04*float64(i))
		if i == 10 {
			weakest2 = f.ID
		}
		if i == 11 {
			weakest1 = f.ID
		}
	}

	cloud, err := d.spread([]*Fragment{seed})
	if err != nil {
		t.Fatal(err)
	}
	if len(cloud) != spreadBudget {
		t.Fatalf("cloud should respect the budget, got %d", len(cloud))
	}
	for _, f := range cloud {
		if f.ID == weakest1 || f.ID == weakest2 {
			t.Fatalf("the weakest neighbors should be the ones cut: %+v", f)
		}
	}
	for i := 1; i < len(cloud); i++ {
		if cloud[i].Activation > cloud[i-1].Activation {
			t.Fatalf("cloud should be ranked by activation: %+v", cloud)
		}
	}
}

// Confirmed associations harden the edge and pay a reward; identical
// templates bind without confirming; edge weight caps at 0.95.
func TestLinkConfirmationRewardAndCap(t *testing.T) {
	d, s := testDaemon(t)
	now := time.Now()

	d.recent = []recentMoment{{id: 1, template: "beta", at: now}}
	d.linkMoment(&Fragment{ID: 2, Template: "alpha", Time: now})
	if da := d.chem.Snapshot().Dopamine; da != 0.2 {
		t.Fatalf("a first co-occurrence must not reward: DA %.2f", da)
	}
	first := edgeWeight(t, s, 2, 1)
	if first == 0 {
		t.Fatal("the first co-occurrence should still bind")
	}

	d.recent = []recentMoment{{id: 1, template: "beta", at: now}}
	d.linkMoment(&Fragment{ID: 3, Template: "alpha", Time: now})
	if da := d.chem.Snapshot().Dopamine; da <= 0.2 {
		t.Fatalf("a confirmed pair should reward: DA %.2f", da)
	}
	second := edgeWeight(t, s, 3, 1)
	if second <= first {
		t.Fatalf("confirmation should harden the edge: %.4f then %.4f", first, second)
	}

	daBefore := d.chem.Snapshot().Dopamine
	d.recent = []recentMoment{{id: 7, template: "alpha", at: now}}
	d.linkMoment(&Fragment{ID: 8, Template: "alpha", Time: now})
	if da := d.chem.Snapshot().Dopamine; da != daBefore {
		t.Fatalf("a repeating template must not confirm itself: DA %.4f -> %.4f", daBefore, da)
	}
	if edgeWeight(t, s, 8, 7) == 0 {
		t.Fatal("identical templates should still bind episodically")
	}

	var w float64
	for i := uint64(0); i < 20; i++ {
		d.recent = []recentMoment{{id: 1, template: "beta", at: now}}
		d.linkMoment(&Fragment{ID: 100 + i, Template: "alpha", Time: now})
		w = edgeWeight(t, s, 100+i, 1)
	}
	if math.Abs(w-0.95) > 1e-9 {
		t.Fatalf("edge weight should cap at 0.95, got %.4f", w)
	}
}

// However many pairs confirm within one moment, the reward fires once.
func TestConfirmRewardOncePerMoment(t *testing.T) {
	d, _ := testDaemon(t)
	now := time.Now()

	d.recent = []recentMoment{
		{id: 1, template: "beta", at: now},
		{id: 2, template: "gamma", at: now},
	}
	d.linkMoment(&Fragment{ID: 3, Template: "alpha", Time: now})
	if da := d.chem.Snapshot().Dopamine; da != 0.2 {
		t.Fatalf("nothing confirmed yet: DA %.4f", da)
	}

	d.recent = []recentMoment{
		{id: 1, template: "beta", at: now},
		{id: 2, template: "gamma", at: now},
	}
	d.linkMoment(&Fragment{ID: 4, Template: "alpha", Time: now})
	want := 0.2 + confirmReward
	if da := d.chem.Snapshot().Dopamine; math.Abs(da-want) > 1e-9 {
		t.Fatalf("two confirmations in one moment should pay one reward: DA %.4f, want %.4f", da, want)
	}
}

// The binding window has a cutoff and a fanout cap, and the cap keeps the
// most recent moments.
func TestLinkMomentWindowAndFanout(t *testing.T) {
	d, s := testDaemon(t)
	now := time.Now()

	d.recent = []recentMoment{
		{id: 1, template: "t1", at: now.Add(-40 * time.Second)},
		{id: 2, template: "t2", at: now.Add(-5 * time.Second)},
		{id: 3, template: "t3", at: now.Add(-1 * time.Second)},
	}
	linked := d.linkMoment(&Fragment{ID: 10, Template: "new", Time: now})
	if len(linked) != 2 {
		t.Fatalf("only in-window moments should bind, got %v", linked)
	}
	if edgeWeight(t, s, 10, 1) != 0 {
		t.Fatal("a stale moment must not bind")
	}

	d.recent = nil
	for i := uint64(1); i <= 12; i++ {
		d.recent = append(d.recent, recentMoment{id: 20 + i, template: fmt.Sprintf("t%d", i), at: now})
	}
	linked = d.linkMoment(&Fragment{ID: 50, Template: "fresh", Time: now})
	if len(linked) != maxLinkFanout {
		t.Fatalf("fanout should cap at %d, got %d", maxLinkFanout, len(linked))
	}
	for _, id := range linked {
		if id <= 24 {
			t.Fatalf("the cap should keep the most recent moments, bound #%d", id)
		}
	}
}

// Retroactive annotation binds words to queried moments long after the
// binding window has closed, refuses words that match nothing, and stays
// out of the present's window itself.
func TestTellAboutBindsRetroactively(t *testing.T) {
	d, s := testDaemon(t)

	res := d.process(Observation{Channel: "testlog",
		Content: "Disk full on /dev/disk1s9", At: time.Now()}, true, true)
	anomalyID := res.FragmentID
	d.recent = nil // the moment has left the binding window

	resp, _ := d.handle(Request{Cmd: "tell", Query: "disk1s9",
		Content: "that was the broken external drive"})
	if !resp.OK {
		t.Fatalf("annotation should succeed: %+v", resp)
	}
	tellID := resp.Observe.FragmentID
	if edgeWeight(t, s, tellID, anomalyID) == 0 || edgeWeight(t, s, anomalyID, tellID) == 0 {
		t.Fatal("annotation should bind both directions to the queried moment")
	}
	var found bool
	for _, f := range resp.Fragments {
		if f.ID == anomalyID {
			found = true
		}
	}
	if !found {
		t.Fatalf("the reply should bring the annotated moment to mind: %+v", resp.Fragments)
	}
	if len(d.recent) != 0 {
		t.Fatal("a retroactive annotation must not enter the present's binding window")
	}

	before, _ := s.Count()
	resp, _ = d.handle(Request{Cmd: "tell", Query: "matches nothing at all",
		Content: "orphan words"})
	if resp.OK || resp.Error == "" {
		t.Fatalf("annotating nothing should refuse: %+v", resp)
	}
	if after, _ := s.Count(); after != before {
		t.Fatal("refused words must not be stored")
	}
}

// The grounding loop end to end: an anomaly happens, the user names it,
// and recalling the name surfaces the moment it pointed at.
func TestTellBindsAndRecallSpreads(t *testing.T) {
	home, done := startTestDaemon(t)

	for i := 0; i < 40; i++ {
		req := Request{Cmd: "observe", Channel: "testlog",
			Content: fmt.Sprintf("Ping seq=%d latency=%dms", i, 10+i%7)}
		if _, err := ipcCall(home, req); err != nil {
			t.Fatal(err)
		}
	}
	resp, err := ipcCall(home, Request{Cmd: "observe", Channel: "testlog",
		Content: "Disk full on /dev/disk1s9"})
	if err != nil || !resp.Observe.Conscious {
		t.Fatalf("anomaly should encode: %+v %v", resp.Observe, err)
	}
	anomalyID := resp.Observe.FragmentID

	resp, err = ipcCall(home, Request{Cmd: "tell", Content: "that was the famous disk problem"})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Observe == nil || !resp.Observe.Conscious || resp.Observe.FragmentID == 0 {
		t.Fatalf("tell should always encode: %+v", resp.Observe)
	}
	foundInReply := false
	for _, f := range resp.Fragments {
		if f.ID == anomalyID {
			foundInReply = true
		}
	}
	if !foundInReply {
		t.Fatalf("the words should bring the anomaly to mind: %+v", resp.Fragments)
	}

	resp, err = ipcCall(home, Request{Cmd: "recall", Query: "famous"})
	if err != nil {
		t.Fatal(err)
	}
	var seedOK, spreadOK bool
	var tellID uint64
	for _, f := range resp.Fragments {
		if f.Channel == "tell" && f.Activation == 1.0 {
			seedOK = true
			tellID = f.ID
		}
		if f.ID == anomalyID && f.Activation > 0 && f.Activation < 1 {
			spreadOK = true
		}
	}
	if !seedOK || !spreadOK {
		t.Fatalf("recall should return the words and the moment they point at: %+v", resp.Fragments)
	}

	// The reverse edge: recalling the moment surfaces the words about it.
	resp, err = ipcCall(home, Request{Cmd: "recall", Query: "disk1s9"})
	if err != nil {
		t.Fatal(err)
	}
	var reverseOK bool
	for _, f := range resp.Fragments {
		if f.ID == tellID && f.Activation > 0 && f.Activation < 1 {
			reverseOK = true
		}
	}
	if !reverseOK {
		t.Fatalf("recalling the anomaly should surface the words said about it: %+v", resp.Fragments)
	}

	// Dump reports each fragment's link degree.
	resp, err = ipcCall(home, Request{Cmd: "dump"})
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range resp.Fragments {
		if (f.ID == anomalyID || f.ID == tellID) && f.LinkCount < 1 {
			t.Fatalf("linked fragments should report their degree: %+v", f)
		}
	}

	if _, err := ipcCall(home, Request{Cmd: "stop"}); err != nil {
		t.Fatal(err)
	}
	<-done
}

// Memory and the channel model must survive a daemon restart: what was
// normal stays normal, and what was encoded stays recallable.
func TestDaemonMemoryAcrossRestart(t *testing.T) {
	home, done := startTestDaemon(t)

	for i := 0; i < 40; i++ {
		req := Request{Cmd: "observe", Channel: "testlog",
			Content: fmt.Sprintf("Ping seq=%d latency=%dms", i, 10+i%7)}
		if _, err := ipcCall(home, req); err != nil {
			t.Fatal(err)
		}
	}
	resp, err := ipcCall(home, Request{Cmd: "observe", Channel: "testlog",
		Content: "Segfault at 0x7ffee9c01234 in worker 3"})
	if err != nil || !resp.Observe.Conscious {
		t.Fatalf("anomaly should encode before restart: %+v, %v", resp.Observe, err)
	}
	if _, err := ipcCall(home, Request{Cmd: "stop"}); err != nil {
		t.Fatal(err)
	}
	<-done

	// Second life. The model was flushed at shutdown, so a routine ping
	// must still be unsurprising even though this process never saw one.
	cfg := DefaultConfig()
	cfg.SleepIntervalSec = 3600
	cfg.Channels = ChannelsConfig{}
	d, err := NewDaemon(home, cfg, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	done2 := make(chan error, 1)
	go func() { done2 <- d.Run() }()
	defer func() { ipcCall(home, Request{Cmd: "stop"}); <-done2 }()

	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if conn, err := net.Dial("unix", socketPath(home)); err == nil {
			conn.Close()
			break
		}
		time.Sleep(20 * time.Millisecond)
	}

	resp, err = ipcCall(home, Request{Cmd: "observe", Channel: "testlog",
		Content: "Ping seq=99 latency=11ms"})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Observe.Conscious {
		t.Fatalf("routine ping should stay predicted away after restart, got %.2f bits", resp.Observe.Bits)
	}

	resp, err = ipcCall(home, Request{Cmd: "recall", Query: "segfault"})
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.Fragments) != 1 {
		t.Fatalf("the segfault memory should survive the restart: %+v", resp.Fragments)
	}

	// And the database file is plainly on disk.
	if _, err := os.Stat(filepath.Join(home, "memory.db")); err != nil {
		t.Fatal("memory.db should exist on disk")
	}
}
