package grammar

import (
	"strings"
	"testing"
)

// A clause through the general engine: the subject's number flows to the
// verb (agreement read from a memory rule), and the daughters lay out in
// the rule's order (word order read from a memory rule). The noun phrase
// nests inside the clause, so two structures combine to depth.
func clause(num, noun, verb string) *GenNode {
	return Phrase("s", nil, map[string]*GenNode{
		"subj": Phrase("np", map[string]string{"num": num}, map[string]*GenNode{
			"det": Leaf("the", nil),
			"n":   Leaf(noun, nil),
		}),
		"vp": Phrase("vp", nil, map[string]*GenNode{
			"v": Leaf(verb, nil),
		}),
	})
}

func TestGeneralEngineAgreement(t *testing.T) {
	p := Builtin()
	for _, tc := range []struct{ num, want string }{
		{"sg", "the dog runs"},
		{"pl", "the dogs run"},
	} {
		got, trace, ok := p.Generate("en", clause(tc.num, "dog", "run"))
		if !ok || got != tc.want {
			t.Fatalf("agreement should flow from memory: got %q ok %v", got, ok)
		}
		// The composition reads back as the structures that built it.
		joined := strings.Join(trace, " ")
		for _, want := range []string{"lex:dog", "lex:run", "syn:np", "syn:vp", "syn:s"} {
			if !strings.Contains(joined, want) {
				t.Fatalf("the trace should name %q: %v", want, trace)
			}
		}
	}
}

// Editing the word-order rule changes the grammar with no change to the
// engine: the same meaning lays out verb-first once the rule says so, and
// agreement still holds.
func TestGeneralEngineWordOrderIsData(t *testing.T) {
	seed := Curriculum()
	for i := range seed {
		if seed[i].Key == "syn:en:s" {
			seed[i].Value = "vp subj | num:subj>vp | vp"
		}
	}
	p := Compile(nil, nil).SetDecl(seed)
	got, _, ok := p.Generate("en", clause("pl", "dog", "run"))
	if !ok || got != "run the dogs" {
		t.Fatalf("editing the order rule should reorder the clause: got %q ok %v", got, ok)
	}
}

// A meaning the grammar cannot build fails cleanly, so the caller can fall
// back to the procedural realizer rather than speak a broken clause.
func TestGeneralEngineFailsClean(t *testing.T) {
	p := Builtin()
	if _, _, ok := p.Generate("en", Phrase("s", nil, map[string]*GenNode{
		"subj": Leaf("unknownword", nil),
		"vp":   Phrase("vp", nil, map[string]*GenNode{"v": Leaf("run", nil)}),
	})); ok {
		t.Fatal("an unknown lemma must fail the whole generation, not guess")
	}
	if _, _, ok := p.Generate("xx", clause("sg", "dog", "run")); ok {
		t.Fatal("a language with no declarative grammar must fail to the fallback")
	}
}
