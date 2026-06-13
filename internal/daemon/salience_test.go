package daemon

import (
	"testing"
	"time"

	"golem/internal/attention"
	"golem/internal/memory"
	"golem/internal/senses"
)

// The walk weights by information, not frequency: a junk hub reached by
// many weak edges is damped below a focused neighbor reached by a few
// strong ones, so the inner loop does not circle whatever everything
// happens to touch.
func TestWalkAvoidsHighDegreeHub(t *testing.T) {
	d, s := testDaemon(t)
	now := time.Now()
	mk := func(kind, channel, content string) *memory.Fragment {
		f := &memory.Fragment{Kind: kind, Channel: channel, Content: content, Template: content,
			Limbic: d.chem.Snapshot(), Time: now}
		if _, err := s.Encode(f); err != nil {
			t.Fatal(err)
		}
		return f
	}

	seed := mk(memory.KindConcept, "concept", "seed shape")
	seedEp := mk(memory.KindEpisodic, "screen", "seed shape")
	if err := s.LinkRel(seed.ID, seedEp.ID, memory.RelGrounds, groundsWeight); err != nil {
		t.Fatal(err)
	}

	// A focused neighbor: a couple of strong co-occurrences.
	nb := mk(memory.KindConcept, "concept", "focused neighbor")
	for range 2 {
		ep := mk(memory.KindEpisodic, "screen", "focused neighbor")
		s.LinkRel(nb.ID, ep.ID, memory.RelGrounds, groundsWeight)
		s.LinkRel(seedEp.ID, ep.ID, memory.RelTemporal, 0.9)
	}
	// A junk hub: many weak co-occurrences, more raw mass than the neighbor.
	hub := mk(memory.KindConcept, "concept", "junk hub")
	for range 30 {
		ep := mk(memory.KindEpisodic, "screen", "junk hub")
		s.LinkRel(hub.ID, ep.ID, memory.RelGrounds, groundsWeight)
		s.LinkRel(seedEp.ID, ep.ID, memory.RelTemporal, 0.15)
	}

	dests, err := d.walkableDests(seed.ID)
	if err != nil {
		t.Fatal(err)
	}
	var nbW, hubW float64
	for _, dst := range dests[memory.RelTemporal] {
		switch dst.id {
		case nb.ID:
			nbW = dst.weight
		case hub.ID:
			hubW = dst.weight
		}
	}
	if nbW == 0 || hubW == 0 {
		t.Fatalf("both neighbors should be reachable: nb %.2f hub %.2f", nbW, hubW)
	}
	if hubW >= nbW {
		t.Fatalf("the hub's breadth should damp it below the focused neighbor: hub %.2f nb %.2f", hubW, nbW)
	}
}

// Salience follows information: a node lived many times in one fixed
// context warms its concept faintly, while a thing lived across varied
// contexts warms fully, so noise never pulls the inner loop above meaning.
func TestSalienceByInformation(t *testing.T) {
	d, s := testDaemon(t)
	base := time.Now().Add(-3 * time.Hour)
	// Junk: the same shape many times, one fixed channel. The merge pass
	// keeps an exemplar out, so four showings distil a prototype.
	for i := range 4 {
		d.recent = nil
		d.process(senses.Observation{Channel: "syslog", Content: "heartbeat tick",
			At: base.Add(time.Duration(i) * time.Minute)}, true, true)
	}
	// Varied: a shape distilled on one channel but lived across three.
	for i := range 4 {
		d.recent = nil
		d.process(senses.Observation{Channel: "screen", Content: "deploy done",
			At: base.Add(time.Hour + time.Duration(i)*time.Minute)}, true, true)
	}
	d.recent = nil
	d.process(senses.Observation{Channel: "shell", Content: "deploy done", At: base.Add(2 * time.Hour)}, true, true)
	d.recent = nil
	d.process(senses.Observation{Channel: "files", Content: "deploy done", At: base.Add(2*time.Hour + time.Minute)}, true, true)
	d.recent = nil

	if _, err := s.MergeDuplicates(); err != nil {
		t.Fatal(err)
	}
	calm(d)
	d.growConcepts()
	d.growFeatures() // computes and caches each concept's information

	junk, _ := s.ConceptByTemplate(attention.Normalize("heartbeat tick"))
	varied, _ := s.ConceptByTemplate(attention.Normalize("deploy done"))
	if junk == nil || varied == nil {
		t.Fatal("both shapes should distil")
	}
	if junk.Surprise >= varied.Surprise {
		t.Fatalf("a fixed-context node should carry less information: junk %.2f varied %.2f", junk.Surprise, varied.Surprise)
	}

	// When each recurs, the varied concept warms more than the junk, so the
	// junk cannot pull the inner loop above real knowledge.
	d.ws.clear()
	d.process(senses.Observation{Channel: "syslog", Content: "heartbeat tick", At: time.Now()}, true, true)
	junkHeat := d.ws.heat(junk.ID)
	d.process(senses.Observation{Channel: "screen", Content: "deploy done", At: time.Now()}, true, true)
	variedHeat := d.ws.heat(varied.ID)
	if variedHeat <= junkHeat {
		t.Fatalf("the varied concept should warm above the junk: varied %.2f junk %.2f", variedHeat, junkHeat)
	}
}

// The true-forget path destroys a node and its edges outright, the concept
// and its episodes alike, and records the deletion on the trail.
func TestForgetDeletesNode(t *testing.T) {
	d, s := testDaemon(t)
	base := time.Now().Add(-time.Hour)
	for i := range 4 {
		d.recent = nil
		d.process(senses.Observation{Channel: "syslog", Content: "junk to forget",
			At: base.Add(time.Duration(i) * time.Minute)}, true, true)
	}
	d.recent = nil
	if _, err := s.MergeDuplicates(); err != nil {
		t.Fatal(err)
	}
	calm(d)
	d.growConcepts()
	if c, _ := s.ConceptByTemplate(attention.Normalize("junk to forget")); c == nil {
		t.Fatal("the shape should distil before it is forgotten")
	}

	n, what, err := d.forget("junk to forget")
	if err != nil || n == 0 {
		t.Fatalf("forget should delete the shape: %d %q %v", n, what, err)
	}
	if c, _ := s.ConceptByTemplate(attention.Normalize("junk to forget")); c != nil {
		t.Fatal("the concept should be gone")
	}
	if hits, _ := s.Recall("junk to forget", 5); len(hits) != 0 {
		t.Fatalf("the episodes should be gone: %+v", hits)
	}
	if _, ok := auditHas(t, s, "forget"); !ok {
		t.Fatal("the deletion belongs on the trail")
	}
}

// forget prefers the concept a query names over an incidental episodic
// mention of it, and sweeps the now-nameless word the cut orphans, so an
// abstraction whose template is shared by neither its members' moments nor a
// question carrying its name still goes in one pass. The member it grounded
// in survives untouched.
func TestForgetPrefersConceptAndSweepsName(t *testing.T) {
	d, s := testDaemon(t)
	now := time.Now()
	// An abstraction, the word that names it, a member it grounds in, and a
	// question that merely mentions the word. The abstraction shares its
	// template with none of them, the shape that used to take three passes.
	item, _ := s.Encode(&memory.Fragment{Kind: memory.KindConcept, Content: "item", Template: "abs:en:item", Time: now})
	word, _ := s.Encode(&memory.Fragment{Kind: memory.KindWord, Content: "item", Template: "item", Time: now})
	member, _ := s.Encode(&memory.Fragment{Kind: memory.KindConcept, Content: "a green ribbon", Template: "a green ribbon", Time: now})
	tell, _ := s.Encode(&memory.Fragment{Kind: memory.KindEpisodic, Channel: "tell", Content: "what is item", Template: "what is item", Time: now})
	s.LinkRel(word, item, memory.RelNames, 1.0)
	s.LinkRel(item, member, memory.RelGrounds, 1.0)

	// Under the old order recall returned the mention first, so forget took it
	// and left the concept behind.
	if hits, _ := s.Recall("item", 1); len(hits) == 0 || hits[0].ID != tell {
		t.Fatalf("setup: the mention should shadow the concept in recall, got %+v", hits)
	}

	n, what, err := d.forget("item")
	if err != nil || n == 0 {
		t.Fatalf("forget should delete the concept: %d %q %v", n, what, err)
	}
	if got, _ := s.FragmentsByIDs([]uint64{item}); len(got) != 0 {
		t.Fatal("the abstraction should be gone in one pass")
	}
	if got, _ := s.FragmentsByIDs([]uint64{word}); len(got) != 0 {
		t.Fatal("the orphaned name-word should be swept with it")
	}
	if got, _ := s.FragmentsByIDs([]uint64{member}); len(got) != 1 {
		t.Fatal("a member concept must survive forgetting the abstraction over it")
	}
}
