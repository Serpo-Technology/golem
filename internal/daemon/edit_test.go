package daemon

import (
	"testing"
	"time"

	"golem/internal/memory"
	"golem/wire"
)

func encodeFor(t *testing.T, d *Daemon, content string) uint64 {
	t.Helper()
	id, err := d.store.Encode(&memory.Fragment{
		Channel: "tell", Content: content, Template: content, Surprise: 4, Time: time.Now(),
	})
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func auditMentionsEditor(t *testing.T, d *Daemon) bool {
	t.Helper()
	tail, err := d.store.AuditTail(20)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range tail {
		if e.Cue == "editor" {
			return true
		}
	}
	return false
}

// A raw edit splices a fragment's content, marks it edited, and lands on the
// audit trail, so the editor's hand is never silent.
func TestEditSplicesMarksAndAudits(t *testing.T) {
	d, _ := testDaemon(t)
	id := encodeFor(t, d, "a small grey owl")

	resp, _, _ := d.handle(wire.Request{Cmd: "edit", ID: id, Content: "a large grey owl"}, nil)
	if !resp.OK {
		t.Fatalf("edit failed: %s", resp.Error)
	}
	fs, _ := d.store.FragmentsByIDs([]uint64{id})
	if len(fs) == 0 || fs[0].Content != "a large grey owl" || !fs[0].Edited {
		t.Fatalf("the fragment should be spliced and marked edited: %+v", fs)
	}
	if !auditMentionsEditor(t, d) {
		t.Fatal("a raw edit must be logged to the audit trail")
	}
}

// Deletion takes exactly the node named, and deleting a node that is already
// gone is refused rather than reported as success.
func TestDeleteByIDOverWire(t *testing.T) {
	d, _ := testDaemon(t)
	id := encodeFor(t, d, "a brass key")

	resp, _, _ := d.handle(wire.Request{Cmd: "delete", ID: id}, nil)
	if !resp.OK {
		t.Fatalf("delete failed: %s", resp.Error)
	}
	if fs, _ := d.store.FragmentsByIDs([]uint64{id}); len(fs) != 0 {
		t.Fatal("the fragment should be gone")
	}
	if again, _, _ := d.handle(wire.Request{Cmd: "delete", ID: id}, nil); again.OK {
		t.Fatal("deleting an absent fragment should not report success")
	}
}

// The provenance mark can be set without otherwise changing the fragment.
func TestMarkOverWire(t *testing.T) {
	d, _ := testDaemon(t)
	id := encodeFor(t, d, "a green ribbon")

	resp, _, _ := d.handle(wire.Request{Cmd: "mark", ID: id}, nil)
	if !resp.OK {
		t.Fatalf("mark failed: %s", resp.Error)
	}
	fs, _ := d.store.FragmentsByIDs([]uint64{id})
	if len(fs) == 0 || !fs[0].Edited {
		t.Fatal("mark should set the provenance bit")
	}
}

// A raw edit with no id is refused, so surgery never lands on nothing.
func TestEditNeedsAnID(t *testing.T) {
	d, _ := testDaemon(t)
	resp, _, _ := d.handle(wire.Request{Cmd: "edit", Content: "x"}, nil)
	if resp.OK || resp.Error == "" {
		t.Fatal("an edit with no id should be refused")
	}
}
