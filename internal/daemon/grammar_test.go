package daemon

import (
	"testing"
	"time"

	"golem/internal/memory"
)

// The grammar seeds into starter memory on first run, through the same
// encode path everything takes: a function word's category and a verb's
// principal parts are ordinary nodes, readable from a dump.
func TestGrammarSeedsIntoMemory(t *testing.T) {
	_, s := testDaemon(t)

	nodes, err := s.NodesByKind(memory.KindGrammar)
	if err != nil || len(nodes) == 0 {
		t.Fatalf("the curriculum should seed grammar nodes: %d, %v", len(nodes), err)
	}
	byKey := map[string]*memory.Fragment{}
	for _, n := range nodes {
		byKey[n.Template] = n
	}
	what := byKey["fw:en:what"]
	if what == nil || what.Category != "wh" || what.Weight != 1.0 {
		t.Fatalf("the function word should carry its category at full weight: %+v", what)
	}
	follow := byKey["verb:en:follow"]
	if follow == nil || follow.Content != "follow follows followed followed" {
		t.Fatalf("the verb should carry its principal parts: %+v", follow)
	}
	if byKey["order:tr"] == nil || byKey["order:tr"].Content != "sov" {
		t.Fatal("the word-order setting should be a node")
	}
	if byKey["suffix:tr:ablative"] == nil || byKey["suffix:tr:ablative"].Content != "DAn" {
		t.Fatal("the suffix recipe should be a node")
	}

	// Seeding is idempotent: a second boot reads, never re-trains.
	count := len(nodes)
	if err := (&Daemon{store: s}).loadGrammar(); err != nil {
		t.Fatal(err)
	}
	again, _ := s.NodesByKind(memory.KindGrammar)
	if len(again) != count {
		t.Fatalf("a second load must not re-seed: %d then %d", count, len(again))
	}
}

// Grammar is entrenched: a sleep harsh enough to prune every ordinary
// memory leaves the grammar standing, and the engine still parses from
// it.
func TestGrammarSurvivesHarshSleep(t *testing.T) {
	d, s := testDaemon(t)
	encodeAt(t, s, "syslog", "an ordinary moment", 0, time.Now())

	before, _ := s.NodesByKind(memory.KindGrammar)
	pruned, err := s.Sleep(0.99, 0.99)
	if err != nil {
		t.Fatal(err)
	}
	if pruned == 0 {
		t.Fatal("the harsh night should prune the ordinary moment")
	}
	after, _ := s.NodesByKind(memory.KindGrammar)
	if len(after) != len(before) {
		t.Fatalf("grammar must survive any sleep: %d then %d", len(before), len(after))
	}
	if err := d.loadGrammar(); err != nil {
		t.Fatal(err)
	}
	if f := d.gram.Parse("what usually happens after make test", nil); f.Wh != "what" {
		t.Fatalf("the morning grammar should still parse: %+v", f)
	}
}

// Sustained contrary usage re-weights a seeded parameter: a form taught
// as a deliberate label accrues naming mass, the nightly competition
// erodes the seed, and once outweighed the form parses as content. When
// the usage fades, the seed heals.
func TestContraryUsageReweightsSeededParameter(t *testing.T) {
	d, s := testDaemon(t)

	// "next" is deliberately taught as the name of a lived shape.
	anchors := feedShape(t, s, "shell", "run: make deploy", 4, time.Now().Add(-2*time.Hour))
	if _, err := s.MergeDuplicates(); err != nil {
		t.Fatal(err)
	}
	calm(d)
	d.growConcepts()
	d.mintWords("next", "en", 0, anchors, true)
	calm(d)
	d.growConcepts() // the naming pass bonds the word cross-situationally

	mass, err := s.NamesMassByForm()
	if err != nil || mass["en:next"] < grammarContestMass {
		t.Fatalf("the deliberate label should carry naming mass: %v %v", mass, err)
	}

	var nights int
	for nights = 1; nights <= 20; nights++ {
		d.grammarCompetition()
		if f := d.gram.Parse("run next", nil); f.Patient.Phrase == "next" {
			break
		}
	}
	if f := d.gram.Parse("run next", nil); f.Patient.Phrase != "next" {
		t.Fatalf("sustained usage should out-weigh the seed: %+v", f)
	}
	if nights < 2 {
		t.Fatal("one night must not flip an entrenched parameter")
	}

	// The usage fades: the word dies of disconnection and the seed heals
	// back toward full entrenchment.
	if _, err := s.Sleep(0.99, 0.99); err != nil {
		t.Fatal(err)
	}
	nodes, _ := s.NodesByKind(memory.KindGrammar)
	var worn float64
	for _, n := range nodes {
		if n.Template == "fw:en:next" {
			worn = n.Weight
		}
	}
	d.grammarCompetition()
	nodes, _ = s.NodesByKind(memory.KindGrammar)
	for _, n := range nodes {
		if n.Template == "fw:en:next" && n.Weight <= worn {
			t.Fatalf("a seed without live contest should heal: %.2f then %.2f", worn, n.Weight)
		}
	}
}
