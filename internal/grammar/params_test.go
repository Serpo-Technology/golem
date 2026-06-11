package grammar

import (
	"testing"
)

// The compiled curriculum is the engine's seed: parsing through it
// matches the package-level behavior exactly.
func TestCurriculumCompilesToBuiltinBehavior(t *testing.T) {
	p := Compile(Curriculum(), nil)
	f := p.Parse("what usually happens after make test", nil)
	if f.Force != ForceQuestion || f.Wh != "what" || !f.Habit {
		t.Fatalf("the compiled curriculum should parse like the builtin: %+v", f)
	}
	if rel, anchor := f.FirstQual(); rel != "after" || anchor.Phrase != "make test" {
		t.Fatalf("the qualifier should survive compilation: %q %q", rel, anchor.Phrase)
	}
	if got := finiteVerb(p, "see", false, false, true); got != "saw" {
		t.Fatalf("principal parts should read from the compiled copy, got %q", got)
	}
	if got := ablativeTR(p, "make test"); got != "make test'ten" {
		t.Fatalf("the suffix recipe should realize through the interpreter, got %q", got)
	}
}

// The weighted competition: a closed-class form whose lexical naming
// mass outweighs its seeded entry stops parsing as grammar and lands in
// content position like any other word.
func TestLexicalMassOutweighsSeededForm(t *testing.T) {
	seeded := Compile(Curriculum(), nil)
	if got := seeded.Parse("run next", nil); got.Patient.Phrase != "" {
		t.Fatalf("the seeded grammar reads next as filler: %+v", got)
	}
	contested := Compile(Curriculum(), map[string]float64{"en:next": 1.5})
	got := contested.Parse("run next", nil)
	if got.Force != ForceCommand || got.Patient.Phrase != "next" {
		t.Fatalf("an outweighed form should land as content: %+v", got)
	}
}

// Word order is a parameter, read from memory like the rest: with the
// verb-final setting gone, the SOV parse never engages and the anchor
// is no longer read from before its relator.
func TestWordOrderParameterDrivesParsing(t *testing.T) {
	if f := Builtin().Parse("make test'ten sonra ne gelir", nil); len(f.Quals) != 1 ||
		f.Quals[0].Anchor.Phrase != "make test" {
		t.Fatalf("the verb-final setting should parse the anchor before its relator: %+v", f)
	}
	var entries []Param
	for _, e := range Curriculum() {
		if e.Key == "order:tr" {
			e.Value = "svo"
		}
		entries = append(entries, e)
	}
	flipped := Compile(entries, nil)
	if f := flipped.Parse("make test'ten sonra ne gelir", nil); len(f.Quals) == 1 &&
		f.Quals[0].Anchor.Phrase == "make test" {
		t.Fatalf("without the verb-final setting the SOV parse must not engage: %+v", f)
	}
}

// A nil Params falls back to the builtin seed, so an engine without a
// memory-backed grammar still speaks.
func TestNilParamsFallBackToBuiltin(t *testing.T) {
	var p *Params
	f := p.Parse("what is disk", nil)
	if f.Force != ForceQuestion || f.Wh != "what" {
		t.Fatalf("a nil grammar should read as the builtin: %+v", f)
	}
	if got := p.ContentTokens("the tests are running"); len(got) != 2 {
		t.Fatalf("content tokens should drop the closed class: %v", got)
	}
}
