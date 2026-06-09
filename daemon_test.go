package main

import (
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func startTestDaemon(t *testing.T) (home string, done chan error) {
	t.Helper()
	home = t.TempDir()
	cfg := DefaultConfig()
	cfg.SleepIntervalSec = 3600 // keep autonomous sleep out of the test's way

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
