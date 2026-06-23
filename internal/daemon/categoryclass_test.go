package daemon

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"golem/internal/memory"
)

// A bare grammatical class is a graph concept a closed-class word is a member of: the
// determiner forms link to a det class concept, and the grammar derives their "det"
// category from the link rather than a seeded string. Every seeded determiner still parses
// as one (dropped from the content of a noun phrase), the class concept is kept out of the
// grown layer, and a fresh word linked to the class parses as a determiner, so class
// membership is a teachable graph fact.
func TestDeterminerClassDerivedFromGraph(t *testing.T) {
	d, s := testDaemon(t)

	// A determiner sentence parses with the articles dropped from the content.
	f := d.gram.ParseHint("the dog followed the cat", "en", d.resolveConcept)
	if f.Agent.Phrase != "dog" || f.Patient.Phrase != "cat" {
		t.Fatalf("determiners should drop from the content: agent %q patient %q", f.Agent.Phrase, f.Patient.Phrase)
	}
	// Every determiner form is closed class, its category derived to "det" from the graph.
	for _, w := range []string{"the", "a", "an", "my", "your"} {
		if !d.gram.IsFunctionWord("en", w) {
			t.Errorf("%q should be a closed-class determiner derived from the graph", w)
		}
	}
	if !d.gram.IsFunctionWord("tr", "bir") {
		t.Error("the Turkish determiner bir should stay closed class from the graph")
	}

	// The det class concept is innate scaffolding, kept out of the grown concept layer.
	var detID uint64
	for id, name := range d.categoryClasses {
		if name == "det" {
			detID = id
		}
	}
	if detID == 0 {
		t.Fatal("the det class concept should exist after boot")
	}
	if !d.isSystemConcept(detID) {
		t.Error("the det class concept should be a system concept")
	}

	// Learnability: a fresh closed-class word with NO seeded category, linked to the det
	// class, parses as a determiner once the grammar recompiles. The recompile is silent,
	// the derived class agreeing with the seeded one for the real determiners.
	fw := &memory.Fragment{Kind: memory.KindGrammar, Channel: "grammar", Content: "zix",
		Template: "fw:en:zix", Lang: "en", Time: time.Now()}
	if _, err := s.Encode(fw); err != nil {
		t.Fatal(err)
	}
	if err := s.LinkRel(fw.ID, detID, memory.RelMarks, 1); err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	d.out = &buf
	if err := d.recompileGrammar(); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(buf.String(), "category class mismatch") {
		t.Fatalf("the bare-class derive should agree with the seed, but logged: %q", buf.String())
	}
	if !d.gram.IsFunctionWord("en", "zix") {
		t.Fatal("a word linked to the det class should derive a closed-class category")
	}
	lf := d.gram.ParseHint("zix dog followed the cat", "en", d.resolveConcept)
	if lf.Agent.Phrase != "dog" {
		t.Fatalf("a learned determiner should drop from the content like the: agent %q", lf.Agent.Phrase)
	}
}
