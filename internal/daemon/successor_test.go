package daemon

import (
	"testing"
	"time"

	"golem/internal/memory"
	"golem/internal/senses"
)

// Binding is asymmetric by spike timing: the earlier moment gains a
// directed edge toward what followed it, and nothing is written back.
func TestBindWritesSuccessorForwardOnly(t *testing.T) {
	d, s := testDaemon(t)
	base := time.Now()

	d.process(senses.Observation{Channel: "shell", Content: "run: make test", At: base}, true, true)
	d.process(senses.Observation{Channel: "shell", Content: "run: make deploy", At: base.Add(5 * time.Second)}, true, true)

	frags, _ := s.Dump()
	var moments []*memory.Fragment
	for _, f := range frags {
		if f.Kind == memory.KindEpisodic {
			moments = append(moments, f)
		}
	}
	if len(moments) != 2 {
		t.Fatalf("expected two moments, got %d", len(moments))
	}
	test, deploy := moments[0], moments[1]

	if edgeWeightRel(t, s, test.ID, deploy.ID, memory.RelSuccessor) == 0 {
		t.Fatal("the earlier moment should point at what followed it")
	}
	if edgeWeightRel(t, s, deploy.ID, test.ID, memory.RelSuccessor) != 0 {
		t.Fatal("no successor edge may run backward in time")
	}
	// Co-occurrence still binds both ways alongside the order.
	if edgeWeight(t, s, test.ID, deploy.ID) == 0 || edgeWeight(t, s, deploy.ID, test.ID) == 0 {
		t.Fatal("temporal adjacency should still bind bidirectionally")
	}
}

// A moment following itself is repetition and earns no order edge.
func TestNoSuccessorBetweenIdenticalShapes(t *testing.T) {
	d, s := testDaemon(t)
	base := time.Now()

	d.process(senses.Observation{Channel: "syslog", Content: "Disk full on /dev/disk1s1", At: base}, true, true)
	d.process(senses.Observation{Channel: "syslog", Content: "Disk full on /dev/disk1s2", At: base.Add(2 * time.Second)}, true, true)

	frags, _ := s.Dump()
	if edgeWeightRel(t, s, frags[0].ID, frags[1].ID, memory.RelSuccessor) != 0 {
		t.Fatal("identical templates must not bind as order")
	}
}

// The same ordered pair recurring strengthens the forward edge, the
// directional twin of the co-occurrence repeat bonus.
func TestSuccessorRepetitionStrengthens(t *testing.T) {
	d, s := testDaemon(t)
	base := time.Now().Add(-time.Hour)

	weightAfter := func(round int) float64 {
		at := base.Add(time.Duration(round) * 10 * time.Minute)
		d.recent = nil // each round is its own episode
		d.process(senses.Observation{Channel: "shell", Content: "run: make test", At: at}, true, true)
		d.process(senses.Observation{Channel: "shell", Content: "run: make deploy", At: at.Add(5 * time.Second)}, true, true)
		frags, _ := s.Dump()
		earlier := frags[len(frags)-2]
		later := frags[len(frags)-1]
		return edgeWeightRel(t, s, earlier.ID, later.ID, memory.RelSuccessor)
	}

	first := weightAfter(0)
	second := weightAfter(1)
	third := weightAfter(2)
	if first <= 0 {
		t.Fatal("the first ordered pair should already bind")
	}
	if !(second > first && third > second) {
		t.Fatalf("repetition should strengthen order: %.2f, %.2f, %.2f", first, second, third)
	}
}

// Replay lays order along the day: every ordered pair with distinct
// shapes inside the window gains a forward successor edge, including
// same-channel pairs that co-occurrence weaving skips.
func TestReplayWeavesSuccessorOrder(t *testing.T) {
	d, s := testDaemon(t)
	base := time.Now().Add(-2 * time.Hour)

	test := encodeAt(t, s, "shell", "run: make test", 4.0, base)
	deploy := encodeAt(t, s, "shell", "run: make deploy", 4.0, base.Add(2*time.Minute))
	vitals := encodeAt(t, s, "vitals", "memory pressure went high", 4.0, base.Add(3*time.Minute))
	far := encodeAt(t, s, "shell", "run: make clean", 4.0, base.Add(90*time.Minute))

	d.replay(base.Add(-time.Hour))

	if edgeWeightRel(t, s, test.ID, deploy.ID, memory.RelSuccessor) == 0 {
		t.Fatal("replay should weave order along same-channel pairs")
	}
	if edgeWeightRel(t, s, deploy.ID, test.ID, memory.RelSuccessor) != 0 {
		t.Fatal("replayed order must stay directed")
	}
	if edgeWeightRel(t, s, deploy.ID, vitals.ID, memory.RelSuccessor) == 0 {
		t.Fatal("replay should weave order across channels too")
	}
	if edgeWeightRel(t, s, vitals.ID, far.ID, memory.RelSuccessor) != 0 {
		t.Fatal("order must not span beyond the replay window")
	}
}
