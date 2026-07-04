package main

import (
	"testing"

	"golem/wire"
)

func TestRecentKeepsTheNewestHead(t *testing.T) {
	// The daemon returns the trail newest first, so recent keeps the head.
	trail := []*wire.AuditInfo{
		{Cue: "d"}, {Cue: "c"}, {Cue: "b"}, {Cue: "a"},
	}
	got := recent(trail, 2)
	if len(got) != 2 || got[0].Cue != "d" || got[1].Cue != "c" {
		t.Fatalf("the two newest should remain: %+v", got)
	}
	if all := recent(trail, 10); len(all) != 4 {
		t.Fatalf("a window larger than the trail returns all: %d", len(all))
	}
	if none := recent(trail, 0); len(none) != 4 {
		t.Fatalf("a non-positive window returns all: %d", len(none))
	}
}
