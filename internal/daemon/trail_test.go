package daemon

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"golem/internal/events"
	"golem/internal/wire"
)

func readEvents(t *testing.T, home string) []events.Event {
	t.Helper()
	f, err := os.Open(filepath.Join(home, "events.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	var out []events.Event
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		var ev events.Event
		if err := json.Unmarshal(sc.Bytes(), &ev); err != nil {
			t.Fatalf("malformed event line %q: %v", sc.Text(), err)
		}
		out = append(out, ev)
	}
	if err := sc.Err(); err != nil {
		t.Fatal(err)
	}
	return out
}

// The daemon writes the trail for real activity: encodes, links, and
// proposals all land, and the dump carries the link graph for export.
func TestDaemonWritesTrail(t *testing.T) {
	home, done := startTestDaemon(t)

	for i := range 40 {
		req := wire.Request{Cmd: "observe", Channel: "testlog",
			Content: fmt.Sprintf("Ping seq=%d latency=%dms", i, 10+i%7)}
		if _, err := wire.Call(home, req); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := wire.Call(home, wire.Request{Cmd: "observe", Channel: "testlog",
		Content: "Disk full on /dev/disk1s9"}); err != nil {
		t.Fatal(err)
	}
	if _, err := wire.Call(home, wire.Request{Cmd: "tell", Content: "that was the disk problem"}); err != nil {
		t.Fatal(err)
	}
	if _, err := wire.Call(home, wire.Request{Cmd: "teach", Content: "check disk", Command: "df -h"}); err != nil {
		t.Fatal(err)
	}
	if _, err := wire.Call(home, wire.Request{Cmd: "propose", Content: "check disk"}); err != nil {
		t.Fatal(err)
	}

	resp, err := wire.Call(home, wire.Request{Cmd: "dump"})
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.Links) == 0 {
		t.Fatal("dump should carry the link graph")
	}
	if _, err := wire.Call(home, wire.Request{Cmd: "stop"}); err != nil {
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
