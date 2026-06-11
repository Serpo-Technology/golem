package events

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
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
	if err := sc.Err(); err != nil {
		t.Fatal(err)
	}
	return out
}

// Every line is one JSON event with a stamp and a kind.
func TestEventLogAppends(t *testing.T) {
	home := t.TempDir()
	l, err := Open(home)
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
	l, err := Open(home)
	if err != nil {
		t.Fatal(err)
	}
	l.maxBytes = 512
	l.checkEvery = 8
	for i := range 200 {
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
