package main

import (
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// seedGraduatedHabit writes a graduated pair into a fresh store before the
// daemon opens it, so a single propose autoruns without minutes of real
// approvals in the test.
func seedGraduatedHabit(t *testing.T, home, cue, command string) {
	t.Helper()
	s, err := OpenStore(filepath.Join(home, "memory.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if err := s.SaveHabit(Habit{Cue: Normalize(cue), Action: command,
		Approvals: 3, Status: HabitGraduated}); err != nil {
		t.Fatal(err)
	}
}

func execTestConfig() Config {
	cfg := DefaultConfig()
	cfg.SleepIntervalSec = 3600
	cfg.Channels = ChannelsConfig{}
	cfg.ExecuteEnabled = true
	cfg.ExecuteAllowlist = []string{"sleep"}
	return cfg
}

// The brain loop keeps answering while a command runs: status returns
// immediately mid-run and names the command, a second action is refused
// rather than queued, and the triggering client still gets its outcome.
func TestLoopAnswersMidRun(t *testing.T) {
	home := t.TempDir()
	seedGraduatedHabit(t, home, "take a nap", "sleep 1")
	home, done := startTestDaemonWith(t, home, execTestConfig())

	for range 4 {
		if _, err := ipcCall(home, Request{Cmd: "teach", Content: "take a nap", Command: "sleep 1"}); err != nil {
			t.Fatal(err)
		}
	}

	proposed := make(chan Response, 1)
	go func() {
		resp, _ := ipcCall(home, Request{Cmd: "propose", Content: "take a nap"})
		proposed <- resp
	}()

	// Status must answer while the command runs. Before this change the
	// loop was blocked for the whole second and the call timed out as
	// "daemon busy".
	var running string
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		resp, err := ipcCall(home, Request{Cmd: "status"})
		if err != nil {
			t.Fatal(err)
		}
		if resp.Status.Running != "" {
			running = resp.Status.Running
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if running != "sleep 1" {
		resp := <-proposed
		t.Fatalf("status should answer mid-run and name the command, got %q; propose said: %+v / %q",
			running, resp.Decision, resp.Error)
	}

	// One command at a time: a second action is refused with a clear
	// error, never silently queued.
	if _, err := ipcCall(home, Request{Cmd: "propose", Content: "take a nap"}); err == nil ||
		!strings.Contains(err.Error(), "already running") {
		t.Fatalf("a second action should be refused while one runs, got %v", err)
	}

	resp := <-proposed
	if resp.Decision == nil || !resp.Decision.Autonomous {
		t.Fatalf("the triggering client should still get its outcome: %+v", resp.Decision)
	}
	if !strings.HasPrefix(resp.Decision.Outcome, "ok") {
		t.Fatalf("the run should have succeeded, got %q", resp.Decision.Outcome)
	}

	if _, err := ipcCall(home, Request{Cmd: "status"}); err != nil {
		t.Fatal(err)
	}
	if _, err := ipcCall(home, Request{Cmd: "stop"}); err != nil {
		t.Fatal(err)
	}
	<-done
}

// Stopping mid-run settles the command first: its outcome lands in the
// audit trail before the store closes, so a stop cannot lose a result.
func TestStopSettlesInFlightRun(t *testing.T) {
	home := t.TempDir()
	seedGraduatedHabit(t, home, "quick nap", "sleep 0.4")
	home, done := startTestDaemonWith(t, home, execTestConfig())

	for range 4 {
		if _, err := ipcCall(home, Request{Cmd: "teach", Content: "quick nap", Command: "sleep 0.4"}); err != nil {
			t.Fatal(err)
		}
	}
	go ipcCall(home, Request{Cmd: "propose", Content: "quick nap"})

	deadline := time.Now().Add(3 * time.Second)
	started := false
	for time.Now().Before(deadline) {
		resp, err := ipcCall(home, Request{Cmd: "status"})
		if err != nil {
			t.Fatal(err)
		}
		if resp.Status.Running != "" {
			started = true
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if !started {
		t.Fatal("the autonomous run never started")
	}

	if _, err := ipcCall(home, Request{Cmd: "stop"}); err != nil {
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

	s, err := OpenStore(filepath.Join(home, "memory.db"))
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
