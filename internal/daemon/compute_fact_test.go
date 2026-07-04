package daemon

import (
	"strings"
	"testing"

	"golem/internal/grammar"
	"golem/internal/memory"
)

// An operator symbol is a token, a citizen the graph can name; sentence
// punctuation stays consumed, so "?" still carries force without becoming a word.
func TestSymbolsTokenizeAndPunctuationDoesNot(t *testing.T) {
	toks, question := grammar.Tokenize("what is 3 + 4?")
	want := []string{"what", "is", "3", "+", "4"}
	if len(toks) != len(want) {
		t.Fatalf("tokens: got %v want %v", toks, want)
	}
	for i := range want {
		if toks[i] != want[i] {
			t.Fatalf("tokens: got %v want %v", toks, want)
		}
	}
	if !question {
		t.Fatal("the question mark should still carry force")
	}
	// The negative literal and the hyphenated word keep their existing shapes.
	if toks, _ := grammar.Tokenize("it is -8 outside"); toks[2] != "-8" {
		t.Fatalf("a negative literal should survive: %v", toks)
	}
	if toks, _ := grammar.Tokenize("a small, black cat."); strings.Join(toks, " ") != "a small black cat" {
		t.Fatalf("sentence punctuation stays consumed: %v", toks)
	}
}

// A computation asked in the operator register answers by running, lands as a
// walkable fact with provenance to the program artifact, and a second asking is
// answered from that memory with no program spun.
func TestComputedFactLandsAndAnswersFromMemory(t *testing.T) {
	d, s := testDaemon(t)
	d.cfg.RuntimeEnabled = true

	first := say(t, d, "what is 3 + 4")
	if !strings.Contains(first.Say, "7") {
		t.Fatalf("the symbol call should compute: got %q", first.Say)
	}

	// The fact landed: operands and value as number concepts, the artifact on the
	// provenance edge.
	fact, err := s.FragmentByTemplate("computed:add:3:4")
	if err != nil || fact == nil {
		t.Fatal("the computed fact should stand in the graph")
	}
	vals, _ := s.LinksFromRel([]uint64{fact.ID}, memory.RelValue)
	if len(vals) != 1 {
		t.Fatal("the fact should carry its value edge")
	}
	vf, _ := s.FragmentsByIDs([]uint64{vals[0].To})
	if len(vf) != 1 || vf[0].Content != "7" || vf[0].Category != "number" {
		t.Fatalf("the value should be the number concept 7: %+v", vf)
	}
	prov, _ := s.LinksFromRel([]uint64{fact.ID}, memory.RelComputedBy)
	if len(prov) != 1 {
		t.Fatal("the fact should carry provenance to the program artifact")
	}
	art, _ := s.FragmentsByIDs([]uint64{prov[0].To})
	if len(art) != 1 || !strings.Contains(art[0].Content, "3") {
		t.Fatalf("the provenance should reach the program source: %+v", art)
	}

	// Memory before the machine: with the runtime shut, the same asking still
	// answers, from the landed fact.
	d.cfg.RuntimeEnabled = false
	again := say(t, d, "what is the sum of 3 and 4")
	if !strings.Contains(again.Say, "7") {
		t.Fatalf("a grounded computation should answer from memory without a run: got %q", again.Say)
	}
}

// The number concepts a computed fact rests on are spared the nightly decay, so
// the fact's referents stand across nights the way classes do.
func TestNumberConceptsSurviveSleep(t *testing.T) {
	d, s := testDaemon(t)
	d.cfg.RuntimeEnabled = true
	say(t, d, "what is 6 + 7")
	if _, err := s.Sleep(d.cfg.NightlyDecay, d.cfg.PruneFloor); err != nil {
		t.Fatal(err)
	}
	c, err := s.ConceptByTemplate("num:13")
	if err != nil || c == nil {
		t.Fatal("a number concept should survive the night")
	}
	names, _ := s.LinksToRel([]uint64{c.ID}, memory.RelNames)
	if len(names) == 0 {
		t.Fatal("its numeral should still name it at full weight")
	}
}
