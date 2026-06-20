package daemon

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"golem/internal/grammar"
	"golem/internal/memory"
)

// The general engine runs a declarative grammar read entirely from memory:
// the seeded rules make a clause agree and order itself, and editing the
// word-order node and recompiling changes the grammar with no change to
// the engine.
func TestGeneralEngineFromMemory(t *testing.T) {
	d, s := testDaemon(t)
	tree := grammar.Phrase("s", nil, map[string]*grammar.GenNode{
		"subj": grammar.Phrase("np", map[string]string{"num": "pl"}, map[string]*grammar.GenNode{
			"det": grammar.Leaf("the", nil),
			"n":   grammar.Leaf("dog", nil),
		}),
		"vp": grammar.Phrase("vp", nil, map[string]*grammar.GenNode{
			"v": grammar.Leaf("run", nil),
		}),
	})
	if got, _, ok := d.gram.Generate("en", tree); !ok || got != "the dogs run" {
		t.Fatalf("the seeded declarative grammar should generate the clause: %q %v", got, ok)
	}

	// Edit the word-order rule in the graph and recompile; nothing in the
	// engine changes, only the data.
	if _, err := s.DeleteByTemplate("syn:en:s"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Encode(&memory.Fragment{Kind: memory.KindGrammar, Channel: "grammar",
		Template: "syn:en:s", Content: "vp subj | num:subj>vp | vp", Lang: "en", Time: time.Now()}); err != nil {
		t.Fatal(err)
	}
	if err := d.recompileGrammar(); err != nil {
		t.Fatal(err)
	}
	if got, _, ok := d.gram.Generate("en", tree); !ok || got != "run the dogs" {
		t.Fatalf("editing the order rule should reorder the clause: %q %v", got, ok)
	}
}

// The declarative engine agrees with the live Speak clause on a real stored proposition, even
// for vocabulary the curriculum never seeded: the engine mints a lexical entry for the learned
// words from the coded morphology, so the renderFacts shadow fires and logs no mismatch, and
// SpeakDecl speaks the same answer. This is what broadens the shadow from seeds to live speech.
func TestRenderFactsDeclarativeShadow(t *testing.T) {
	d, _ := testDaemon(t)
	wolf := groundNoun(t, d, "wolf") // none of wolf, fox, watch is in the seeded lexicon
	groundNoun(t, d, "fox")
	say(t, d, "a wolf watched a fox")

	var buf bytes.Buffer
	d.out = &buf
	got, ok := d.renderFacts("en", d.conceptName(wolf.ID, wolf.Content, "en"), d.gatherFacts(wolf, "", ""), true, false)
	if !ok || got != "The wolf watched the fox." {
		t.Fatalf("renderFacts should answer \"The wolf watched the fox.\", got %q (ok %v)", got, ok)
	}
	if strings.Contains(buf.String(), "declgen mismatch") {
		t.Fatalf("the declarative engine should agree with Speak on this clause; log: %q", buf.String())
	}
	m := grammar.Meaning{Relation: grammar.Term{Phrase: "watch"}, Roles: map[string]grammar.Term{
		"agent":   {Phrase: "wolf", Definite: true},
		"patient": {Phrase: "fox", Definite: true},
	}, Past: true}
	if alt, dok := d.gram.SpeakDecl("en", m); !dok || alt != got {
		t.Fatalf("SpeakDecl should mint the learned vocabulary and match the answer: %q %v vs %q", alt, dok, got)
	}
}

// A taught inflection persists to the store and a fresh boot reloads it, so morphology
// learned from examples is a fluid weight rather than a code change: GOLEM is taught the
// Turkish -DI past from four examples, and a reboot inflects a held-out stem by harmony.
func TestTaughtInflectionSurvivesReboot(t *testing.T) {
	d, s := testDaemon(t)
	if err := d.teachInflection("tr", "past", [][2]string{
		{"kovala", "kovaladı"}, {"gel", "geldi"}, {"oku", "okudu"}, {"gör", "gördü"},
	}); err != nil {
		t.Fatal(err)
	}
	if got, ok := d.gram.Inflect("tr", "past", "ara"); !ok || got != "aradı" {
		t.Fatalf("the taught rule should inflect ara -> aradı, got %q (ok %v)", got, ok)
	}
	// A second daemon over the same store is a fresh boot; the rule reloads from memory and
	// still generalizes to a stem it never saw.
	reboot := testDaemonOn(t, s)
	if got, ok := reboot.gram.Inflect("tr", "past", "yürü"); !ok || got != "yürüdü" {
		t.Fatalf("a reboot should reload the rule and inflect yürü -> yürüdü, got %q (ok %v)", got, ok)
	}

	// Re-teaching the feature replaces the rule rather than duplicating its node.
	if err := d.teachInflection("tr", "past", [][2]string{{"al", "aldı"}, {"ver", "verdi"}}); err != nil {
		t.Fatal(err)
	}
	nodes, err := s.NodesByKind(memory.KindGrammar)
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, n := range nodes {
		if n.Template == "infl:tr:past" {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("re-teaching should keep one infl:tr:past node, got %d", count)
	}
}

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
