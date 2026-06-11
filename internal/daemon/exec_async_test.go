package daemon

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	"golem/internal/attention"
	"golem/internal/config"
	"golem/internal/memory"
	"golem/internal/wire"
)

// seedGraduatedHabit writes a graduated pair into a fresh store before the
// daemon opens it, so a single propose autoruns without minutes of real
// approvals in the test.
func seedGraduatedHabit(t *testing.T, home, cue, command string) {
	t.Helper()
	s, err := memory.OpenStore(filepath.Join(home, "memory.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if err := s.SaveHabit(memory.Habit{Cue: attention.Normalize(cue), Action: command,
		Approvals: 3, Status: memory.HabitGraduated}); err != nil {
		t.Fatal(err)
	}
}

func execTestConfig() config.Config {
	cfg := config.Default()
	cfg.SleepIntervalSec = 3600
	cfg.Channels = config.ChannelsConfig{}
	cfg.ExecuteEnabled = true
	cfg.ExecuteAllowlist = []string{"sleep"}
	return cfg
}

// ground a cue over IPC the way the tests need it: enough repetitions to
// propose cleanly.
func teachOverIPC(t *testing.T, home, cue, command string) {
	t.Helper()
	for range 4 {
		if _, err := wire.Call(home, wire.Request{Cmd: "teach", Content: cue, Command: command}); err != nil {
			t.Fatal(err)
		}
	}
}

// pollStatus polls until the condition holds or the deadline passes, and
// returns the last status seen.
func pollStatus(t *testing.T, home string, want func(*wire.StatusInfo) bool) *wire.StatusInfo {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	var st *wire.StatusInfo
	for time.Now().Before(deadline) {
		resp, err := wire.Call(home, wire.Request{Cmd: "status"})
		if err != nil {
			t.Fatal(err)
		}
		st = resp.Status
		if want(st) {
			return st
		}
		time.Sleep(20 * time.Millisecond)
	}
	return st
}

// The brain loop keeps answering while a command runs: status returns
// immediately mid-run and names the command, and the triggering client
// still gets its outcome.
func TestLoopAnswersMidRun(t *testing.T) {
	home := t.TempDir()
	seedGraduatedHabit(t, home, "take a nap", "sleep 1")
	home, done := startTestDaemonWith(t, home, execTestConfig())
	teachOverIPC(t, home, "take a nap", "sleep 1")

	proposed := make(chan wire.Response, 1)
	go func() {
		resp, _ := wire.Call(home, wire.Request{Cmd: "propose", Content: "take a nap"})
		proposed <- resp
	}()

	st := pollStatus(t, home, func(st *wire.StatusInfo) bool { return len(st.Running) > 0 })
	if len(st.Running) == 0 || st.Running[0] != "sleep 1" {
		resp := <-proposed
		t.Fatalf("status should answer mid-run and name the command, got %v; propose said: %+v / %q",
			st.Running, resp.Decision, resp.Error)
	}

	resp := <-proposed
	if resp.Decision == nil || !resp.Decision.Autonomous {
		t.Fatalf("the triggering client should still get its outcome: %+v", resp.Decision)
	}
	if !strings.HasPrefix(resp.Decision.Outcome, "ok") {
		t.Fatalf("the run should have succeeded, got %q", resp.Decision.Outcome)
	}

	if _, err := wire.Call(home, wire.Request{Cmd: "stop"}); err != nil {
		t.Fatal(err)
	}
	<-done
}

// Two actions whose claims collide serialize: the second waits in the
// queue while the first holds the claim, then runs, and both clients get
// real outcomes. Nothing overlaps and nothing is refused.
func TestSameClaimActionsSerialize(t *testing.T) {
	home := t.TempDir()
	seedGraduatedHabit(t, home, "take a nap", "sleep 0.5")
	home, done := startTestDaemonWith(t, home, execTestConfig())
	teachOverIPC(t, home, "take a nap", "sleep 0.5")

	start := time.Now()
	first := make(chan wire.Response, 1)
	second := make(chan wire.Response, 1)
	go func() {
		resp, _ := wire.Call(home, wire.Request{Cmd: "propose", Content: "take a nap"})
		first <- resp
	}()
	pollStatus(t, home, func(st *wire.StatusInfo) bool { return len(st.Running) > 0 })
	go func() {
		resp, _ := wire.Call(home, wire.Request{Cmd: "propose", Content: "take a nap"})
		second <- resp
	}()

	// The second action must park rather than run alongside its twin.
	st := pollStatus(t, home, func(st *wire.StatusInfo) bool { return st.Queued > 0 })
	if st.Queued == 0 {
		t.Fatalf("a conflicting action should wait in the queue: %+v", st)
	}
	if len(st.Running) > 1 {
		t.Fatalf("conflicting actions must never overlap: %v", st.Running)
	}

	r1, r2 := <-first, <-second
	elapsed := time.Since(start)
	for _, r := range []wire.Response{r1, r2} {
		if r.Decision == nil || !r.Decision.Autonomous || !strings.HasPrefix(r.Decision.Outcome, "ok") {
			t.Fatalf("both serialized runs should complete: %+v %q", r.Decision, r.Error)
		}
	}
	if elapsed < 950*time.Millisecond {
		t.Fatalf("two 0.5s actions on one claim should take at least their sum, took %s", elapsed)
	}

	wire.Call(home, wire.Request{Cmd: "stop"})
	<-done
}

// Two actions with disjoint claims run at once: both are visible in the
// run table together and the pair finishes in far less than serial time.
func TestUnclaimedActsOverlap(t *testing.T) {
	home := t.TempDir()
	seedGraduatedHabit(t, home, "short nap", "sleep 0.6")
	seedGraduatedHabit(t, home, "long nap", "sleep 0.7")
	home, done := startTestDaemonWith(t, home, execTestConfig())
	teachOverIPC(t, home, "short nap", "sleep 0.6")
	teachOverIPC(t, home, "long nap", "sleep 0.7")

	start := time.Now()
	first := make(chan wire.Response, 1)
	second := make(chan wire.Response, 1)
	go func() {
		resp, _ := wire.Call(home, wire.Request{Cmd: "propose", Content: "short nap"})
		first <- resp
	}()
	go func() {
		resp, _ := wire.Call(home, wire.Request{Cmd: "propose", Content: "long nap"})
		second <- resp
	}()

	st := pollStatus(t, home, func(st *wire.StatusInfo) bool { return len(st.Running) == 2 })
	if len(st.Running) != 2 {
		t.Fatalf("independent actions should run at once, saw %v", st.Running)
	}

	r1, r2 := <-first, <-second
	elapsed := time.Since(start)
	for _, r := range []wire.Response{r1, r2} {
		if r.Decision == nil || !r.Decision.Autonomous || !strings.HasPrefix(r.Decision.Outcome, "ok") {
			t.Fatalf("both parallel runs should complete: %+v %q", r.Decision, r.Error)
		}
	}
	if elapsed >= 1200*time.Millisecond {
		t.Fatalf("independent actions should overlap, not sum: took %s", elapsed)
	}

	wire.Call(home, wire.Request{Cmd: "stop"})
	<-done
}

// Stopping mid-run settles the command first: its outcome lands in the
// audit trail before the store closes, so a stop cannot lose a result.
func TestStopSettlesInFlightRun(t *testing.T) {
	home := t.TempDir()
	seedGraduatedHabit(t, home, "quick nap", "sleep 0.4")
	home, done := startTestDaemonWith(t, home, execTestConfig())
	teachOverIPC(t, home, "quick nap", "sleep 0.4")

	go wire.Call(home, wire.Request{Cmd: "propose", Content: "quick nap"})
	st := pollStatus(t, home, func(st *wire.StatusInfo) bool { return len(st.Running) > 0 })
	if len(st.Running) == 0 {
		t.Fatal("the autonomous run never started")
	}

	if _, err := wire.Call(home, wire.Request{Cmd: "stop"}); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("daemon exited with error: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("daemon did not exit after stop")
	}

	s, err := memory.OpenStore(filepath.Join(home, "memory.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	entries, err := s.AuditTail(10)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if strings.HasPrefix(e.Outcome, "autonomous") && strings.Contains(e.Outcome, "ok") {
			return
		}
	}
	t.Fatalf("the in-flight outcome should be audited before shutdown, got %+v", entries)
}

// A queued action never starts once the daemon is stopping: it resolves
// with a drop the audit trail records, while the running action still
// settles normally.
func TestQueueDropsAtStop(t *testing.T) {
	home := t.TempDir()
	seedGraduatedHabit(t, home, "take a nap", "sleep 0.8")
	home, done := startTestDaemonWith(t, home, execTestConfig())
	teachOverIPC(t, home, "take a nap", "sleep 0.8")

	go wire.Call(home, wire.Request{Cmd: "propose", Content: "take a nap"})
	pollStatus(t, home, func(st *wire.StatusInfo) bool { return len(st.Running) > 0 })

	dropped := make(chan error, 1)
	go func() {
		_, err := wire.Call(home, wire.Request{Cmd: "propose", Content: "take a nap"})
		dropped <- err
	}()
	pollStatus(t, home, func(st *wire.StatusInfo) bool { return st.Queued > 0 })

	if _, err := wire.Call(home, wire.Request{Cmd: "stop"}); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-dropped:
		if err == nil || !strings.Contains(err.Error(), "dropped") {
			t.Fatalf("the queued client should learn its action was dropped, got %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the queued client never got an answer")
	}
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("daemon exited with error: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("daemon did not exit after stop")
	}

	s, err := memory.OpenStore(filepath.Join(home, "memory.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	entries, _ := s.AuditTail(10)
	var sawDrop, sawSettled bool
	for _, e := range entries {
		if strings.Contains(e.Outcome, "queued action dropped") {
			sawDrop = true
		}
		if strings.HasPrefix(e.Outcome, "autonomous") && strings.Contains(e.Outcome, "ok") {
			sawSettled = true
		}
	}
	if !sawDrop || !sawSettled {
		t.Fatalf("the trail should hold both the settled run and the drop, got %+v", entries)
	}
}
