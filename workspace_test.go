package main

import (
	"testing"
	"time"
)

// Activation fades across ticks and vanishes below the floor, so working
// memory empties itself when nothing keeps it warm.
func TestWorkspaceDecays(t *testing.T) {
	w := newWorkspace()
	w.excite(7, 1.0)
	for i := 0; i < 200; i++ {
		w.decay()
	}
	if h := w.heat(7); h >= 1.0 || h <= workspaceFloor {
		t.Fatalf("activation should have cooled without dying yet: %.3f", h)
	}
	for i := 0; i < 2000; i++ {
		w.decay()
	}
	if h := w.heat(7); h != 0 {
		t.Fatalf("unrefreshed activation should fade out entirely, still %.3f", h)
	}
}

// Re-excitation keeps the strongest level rather than stacking, and the
// warmest query ranks by heat.
func TestWorkspaceWarmest(t *testing.T) {
	w := newWorkspace()
	w.excite(1, 0.4)
	w.excite(2, 0.9)
	w.excite(3, 0.6)
	w.excite(2, 0.3) // weaker re-excitation must not cool it

	top := w.warmest(2)
	if len(top) != 2 || top[0].id != 2 || top[1].id != 3 {
		t.Fatalf("warmest should rank by heat: %+v", top)
	}
	if top[0].heat != 0.9 {
		t.Fatalf("re-excitation keeps the stronger level, got %.2f", top[0].heat)
	}
}

// The budget bounds working memory: past it, the coldest entry is
// evicted.
func TestWorkspaceBudget(t *testing.T) {
	w := newWorkspace()
	w.excite(999, 0.06) // the coldest
	for i := 1; i <= workspaceBudget; i++ {
		w.excite(uint64(i), 0.5)
	}
	if len(w.act) != workspaceBudget {
		t.Fatalf("the budget should hold, got %d", len(w.act))
	}
	if w.heat(999) != 0 {
		t.Fatal("the coldest entry should have been evicted")
	}
}

// Encoding a moment whose shape has a concept warms that concept, and a
// night of sleep clears the buffer.
func TestEncodeWarmsConceptAndSleepClears(t *testing.T) {
	d, s := testDaemon(t)
	feedShape(t, s, "syslog", "Disk full on /dev/disk1s9", 4, time.Now().Add(-time.Hour))
	if _, err := s.MergeDuplicates(); err != nil {
		t.Fatal(err)
	}
	d.growConcepts()
	concept, _ := s.ConceptByTemplate(Normalize("Disk full on /dev/disk1s9"))
	if concept == nil {
		t.Fatal("the shape should have a concept")
	}

	d.process(Observation{Channel: "syslog", Content: "Disk full on /dev/disk1s5", At: time.Now()}, true, true)
	if d.ws.heat(concept.ID) == 0 {
		t.Fatal("a moment of a known shape should warm its concept")
	}
	if warm := d.warmInfos(3); len(warm) == 0 || warm[0].Gloss != concept.Content {
		t.Fatalf("status should show the warm concept, got %+v", warm)
	}

	d.sleepCycle()
	if d.ws.heat(concept.ID) != 0 {
		t.Fatal("working memory does not survive the night")
	}
}

// Speaking words that name a concept warms it to full presence.
func TestSpeechWarmsNamedConcepts(t *testing.T) {
	d, s := testDaemon(t)
	anchors := feedShape(t, s, "shell", "run: go test ./...", 4, time.Now().Add(-time.Hour))
	d.mintWords("tests", "en", 0, anchors)
	if _, err := s.MergeDuplicates(); err != nil {
		t.Fatal(err)
	}
	d.growConcepts()
	concept, _ := s.ConceptByTemplate(Normalize("run: go test ./..."))

	resp, _, _ := d.handle(Request{Cmd: "tell", Content: "the tests look fine"}, nil)
	if !resp.OK {
		t.Fatalf("tell failed: %+v", resp)
	}
	if h := d.ws.heat(concept.ID); h != exciteSpeech {
		t.Fatalf("speaking a concept's name should warm it fully, got %.2f", h)
	}
}
