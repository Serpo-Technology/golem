package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func readEvents(t *testing.T, home string) []Event {
	t.Helper()
	f, err := os.Open(filepath.Join(home, "events.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	var out []Event
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		var ev Event
		if err := json.Unmarshal(sc.Bytes(), &ev); err != nil {
			t.Fatalf("malformed event line %q: %v", sc.Text(), err)
		}
		out = append(out, ev)
	}
	return out
}

// Every line is one JSON event with a stamp and a kind.
func TestEventLogAppends(t *testing.T) {
	home := t.TempDir()
	l, err := OpenEventLog(home)
	if err != nil {
		t.Fatal(err)
	}
	if err := l.Append(Event{At: 1, Ev: "encode", Channel: "syslog", ID: 7, Bits: 4.5}); err != nil {
		t.Fatal(err)
	}
	if err := l.Append(Event{At: 2, Ev: "link", From: 7, To: 3, Weight: 0.4}); err != nil {
		t.Fatal(err)
	}
	l.Close()

	evs := readEvents(t, home)
	if len(evs) != 2 {
		t.Fatalf("expected two events, got %d", len(evs))
	}
	if evs[0].Ev != "encode" || evs[0].ID != 7 || evs[1].Ev != "link" || evs[1].To != 3 {
		t.Fatalf("events should round-trip: %+v", evs)
	}
}

// Past the size cap the log rotates, keeping one older generation.
func TestEventLogRotates(t *testing.T) {
	home := t.TempDir()
	l, err := OpenEventLog(home)
	if err != nil {
		t.Fatal(err)
	}
	l.maxBytes = 512
	l.checkEvery = 8
	for i := 0; i < 200; i++ {
		if err := l.Append(Event{At: int64(i), Ev: "encode", Channel: "testlog",
			Tpl: "some shape that takes a little room"}); err != nil {
			t.Fatal(err)
		}
	}
	l.Close()

	if _, err := os.Stat(filepath.Join(home, "events.jsonl.1")); err != nil {
		t.Fatal("an older generation should exist after rotation")
	}
	st, err := os.Stat(filepath.Join(home, "events.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	if st.Size() > 2048 {
		t.Fatalf("the live log should have been rotated down, %d bytes", st.Size())
	}
}

// The daemon writes the trail for real activity: encodes, links, and
// proposals all land, and the dump carries the link graph for export.
func TestDaemonWritesTrail(t *testing.T) {
	home, done := startTestDaemon(t)

	for i := 0; i < 40; i++ {
		req := Request{Cmd: "observe", Channel: "testlog",
			Content: fmt.Sprintf("Ping seq=%d latency=%dms", i, 10+i%7)}
		if _, err := ipcCall(home, req); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := ipcCall(home, Request{Cmd: "observe", Channel: "testlog",
		Content: "Disk full on /dev/disk1s9"}); err != nil {
		t.Fatal(err)
	}
	if _, err := ipcCall(home, Request{Cmd: "tell", Content: "that was the disk problem"}); err != nil {
		t.Fatal(err)
	}
	if _, err := ipcCall(home, Request{Cmd: "teach", Content: "check disk", Command: "df -h"}); err != nil {
		t.Fatal(err)
	}
	if _, err := ipcCall(home, Request{Cmd: "propose", Content: "check disk"}); err != nil {
		t.Fatal(err)
	}

	resp, err := ipcCall(home, Request{Cmd: "dump"})
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.Links) == 0 {
		t.Fatal("dump should carry the link graph")
	}
	dot := renderDOT(resp.Fragments, resp.Links)
	if !strings.HasPrefix(dot, "digraph golem {") || !strings.Contains(dot, "->") {
		t.Fatalf("the DOT export should hold nodes and edges:\n%s", dot)
	}

	if _, err := ipcCall(home, Request{Cmd: "stop"}); err != nil {
		t.Fatal(err)
	}
	<-done

	kinds := map[string]int{}
	for _, ev := range readEvents(t, home) {
		kinds[ev.Ev]++
		if ev.At == 0 {
			t.Fatalf("every event carries its stamp: %+v", ev)
		}
	}
	for _, want := range []string{"encode", "link", "proposal", "sleep"} {
		if kinds[want] == 0 {
			t.Fatalf("the trail should hold %q events, got %v", want, kinds)
		}
	}
}

// Content with quotes and backslashes cannot break out of a DOT label.
func TestDotEscapes(t *testing.T) {
	frags := []*FragmentInfo{{ID: 1, Channel: "shell",
		Content: `run: echo "hi\there"`, Weight: 1.0}}
	dot := renderDOT(frags, nil)
	if strings.Contains(dot, `"hi\there"`) {
		t.Fatalf("label should be escaped:\n%s", dot)
	}
	if !strings.Contains(dot, `\"hi\\there\"`) {
		t.Fatalf("escaped quotes and backslashes should survive:\n%s", dot)
	}
}
