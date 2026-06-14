package main

import (
	"testing"

	"golem/wire"
)

func TestLastNKeepsTheRecentTail(t *testing.T) {
	trail := []*wire.AuditInfo{
		{Cue: "a"}, {Cue: "b"}, {Cue: "c"}, {Cue: "d"},
	}
	got := lastN(trail, 2)
	if len(got) != 2 || got[0].Cue != "c" || got[1].Cue != "d" {
		t.Fatalf("the two most recent should remain: %+v", got)
	}
	if all := lastN(trail, 10); len(all) != 4 {
		t.Fatalf("a window larger than the trail returns all: %d", len(all))
	}
	if none := lastN(trail, 0); len(none) != 4 {
		t.Fatalf("a non-positive window returns all: %d", len(none))
	}
}
