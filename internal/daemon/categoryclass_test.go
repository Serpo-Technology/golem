package daemon

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"golem/internal/memory"
)

// A bare grammatical class is a graph concept the closed-class words of that class are
// members of: each word links to a class concept, and the grammar derives its category from
// the link rather than reading it from the seed. The classes are seeded from the words' own
// seed categories (the floor), so the derive agrees with the seed and a tripwire stays
// silent; closed-class detection is unchanged across the classes; a determiner sentence
// still parses; the class concepts sit outside the grown layer; and a fresh word linked to a
// class parses as a member, so membership is a teachable graph fact.
func TestBareClassesDerivedFromGraph(t *testing.T) {
	d, s := testDaemon(t)

	// Every class is a system concept, kept out of the grown layer; all seventeen the seed
	// carries are present (the twelve bare classes and the five refined-class bases).
	if len(d.categoryClasses) != 17 {
		t.Fatalf("each broad grammatical class should be a graph concept, got %d", len(d.categoryClasses))
	}
	for id := range d.categoryClasses {
		if !d.isSystemConcept(id) {
			t.Errorf("the %q class concept should be a system concept", d.categoryClasses[id])
		}
	}

	// Closed-class detection is unchanged: a representative word of each class still reads
	// as closed class, its category derived from the link, including the refined-class bases.
	for _, tc := range []struct{ lang, form string }{
		{"en", "the"}, {"en", "is"}, {"en", "not"}, {"en", "of"}, {"en", "always"},
		{"en", "do"}, {"en", "than"}, {"en", "again"}, {"en", "other"}, {"en", "many"},
		{"tr", "bir"}, {"tr", "var"},
		{"en", "say"}, {"en", "after"}, {"en", "what"}, {"en", "he"}, {"en", "transitive"},
	} {
		if !d.gram.IsFunctionWord(tc.lang, tc.form) {
			t.Errorf("%q (%s) should read as closed class from the graph", tc.form, tc.lang)
		}
	}

	// A refined class composes: its base comes from the class link and its colon refinement
	// appends on top, so a speech verb still derives its discourse move.
	if mv := d.gram.ParseHint("explain", "en", d.resolveConcept).Speech; mv != "clarify" {
		t.Errorf("the refined speech base should still take its refinement, got move %q", mv)
	}

	// A determiner sentence parses with the articles dropped from the content.
	f := d.gram.ParseHint("the dog followed the cat", "en", d.resolveConcept)
	if f.Agent.Phrase != "dog" || f.Patient.Phrase != "cat" {
		t.Fatalf("determiners should drop from the content: agent %q patient %q", f.Agent.Phrase, f.Patient.Phrase)
	}

	// The derive agrees with the seed for every linked word, so the recompile is silent;
	// and a fresh word with NO seeded category, linked to the det class, parses as a
	// determiner, the membership read from the graph rather than the seed.
	var detID uint64
	for id, name := range d.categoryClasses {
		if name == "det" {
			detID = id
		}
	}
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
