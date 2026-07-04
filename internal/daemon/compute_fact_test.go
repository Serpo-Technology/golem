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

// A computation asked while the runtime is shut is held as a gap, and the
// compute faculty closes it at idle once the runtime opens: memory first, the
// run otherwise, the answer volunteered aloud and the fact landed.
func TestComputeActClosesHeldGap(t *testing.T) {
	d, s := testDaemon(t)
	d.cfg.RuntimeEnabled = false

	say(t, d, "what is the sum of 6 and 7")
	if len(d.computeGaps) != 1 || d.computeGaps[0].phrase != "the sum of 6 and 7" {
		t.Fatalf("a comprehended call the runtime refused should be held: %+v", d.computeGaps)
	}
	// The same asking again does not queue twice.
	say(t, d, "what is the sum of 6 and 7")
	if len(d.computeGaps) != 1 {
		t.Fatalf("a held gap dedupes: %+v", d.computeGaps)
	}

	// The signal gates on the runtime: shut means the faculty stays quiet.
	sn := thoughtSnapshot{seedHeat: 1, computeGap: d.computeGaps[0].phrase}
	if c := actThought(sn, actOp{name: actCompute}); c.ok {
		t.Fatal("the compute act should stay quiet while the runtime is shut")
	}
	sn.computeReady = true
	if c := actThought(sn, actOp{name: actCompute}); !c.ok || c.arg != "the sum of 6 and 7" {
		t.Fatalf("an open runtime and a held gap should call the faculty: %+v", c)
	}

	// The runtime opens; the faculty closes the gap and lands the fact.
	d.cfg.RuntimeEnabled = true
	outcome, fruitful := d.actComputeNow()
	if !fruitful || !strings.Contains(outcome, "13") {
		t.Fatalf("the faculty should compute the held gap: %q", outcome)
	}
	if len(d.computeGaps) != 0 {
		t.Fatal("the closed gap should be released")
	}
	if f, err := s.FragmentByTemplate("computed:add:6:7"); err != nil || f == nil {
		t.Fatal("the faculty's finding should land as a fact")
	}
	if d.computeRuns != 1 {
		t.Fatalf("a fresh run should count against the wake budget: %d", d.computeRuns)
	}

	// A second identical gap prefers the landed memory: no new run is spun.
	d.recordComputeGap("the sum of 6 and 7", "")
	outcome, fruitful = d.actComputeNow()
	if !fruitful || !strings.Contains(outcome, "13") {
		t.Fatalf("the faculty should answer from memory: %q", outcome)
	}
	if d.computeRuns != 1 {
		t.Fatalf("memory should spare the budget: %d runs", d.computeRuns)
	}
}
