package daemon

import (
	"testing"

	"golem/wire"
)

// The scale and cmpform teaching commands build a working scale block over the daemon:
// resolvable pole adjectives that grade one dimension with the right greater and lesser
// roles, the antonymy between them, and a stored comparative surface. This is the teaching
// path a re-grow uses, the daemon-facing counterpart to the in-process teachScale helper.
func TestScaleCommandBuildsBlock(t *testing.T) {
	d, _ := testDaemon(t)
	cmd := func(req wire.Request) {
		t.Helper()
		if resp, _, _ := d.handle(req, nil); !resp.OK {
			t.Fatalf("%s failed: %s", req.Cmd, resp.Error)
		}
	}
	cmd(wire.Request{Cmd: "scale", Content: "size", Query: "big", Command: "small"})
	cmd(wire.Request{Cmd: "cmpform", Content: "big", Query: "bigger"})

	big, small := d.resolveConcept("big"), d.resolveConcept("small")
	if big == nil || small == nil {
		t.Fatal("scale should mint resolvable pole adjectives")
	}
	if sign, ok := d.poleSign(big); !ok || sign != "gt" {
		t.Fatalf("big should carry the greater pole role, got %q ok=%v", sign, ok)
	}
	if sign, ok := d.poleSign(small); !ok || sign != "lt" {
		t.Fatalf("small should carry the lesser pole role, got %q ok=%v", sign, ok)
	}
	if _, ok := d.scaleOf(big); !ok {
		t.Fatal("big should grade along the size scale")
	}
	if !d.opposesDir("big", "small") {
		t.Fatal("big and small should be opposite poles of one scale")
	}
	if got := d.realizeComparative(big, "en", ""); got != "bigger" {
		t.Fatalf("cmpform should store the comparative surface, got %q", got)
	}
	if d.isSystemConcept(big.ID) {
		t.Fatal("a pole adjective should be grown content, not a hidden system concept")
	}

	// cmpform refuses a word that is not a scale pole, so a stray form mints nothing.
	if resp, _, _ := d.handle(wire.Request{Cmd: "cmpform", Content: "purple", Query: "purpler"}, nil); resp.OK {
		t.Fatal("cmpform should refuse a non-pole adjective")
	}
}
