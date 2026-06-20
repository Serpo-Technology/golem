package grammar

import "testing"

// SpeakDecl reproduces the live Speak clause for the cases the seeded declarative grammar
// covers, so the declarative engine can shadow the coded path with no divergence. The verb
// forms come from the lexicon (chase->chased, run->ran), the word order and definiteness from
// the grammar data, so the surface is character-identical to Speak's.
func TestSpeakDeclMatchesSpeak(t *testing.T) {
	p := Builtin()
	transitive := func(past bool) Meaning {
		return Meaning{Relation: Term{Phrase: "chase"}, Roles: map[string]Term{
			"agent":   {Phrase: "dog", Definite: true},
			"patient": {Phrase: "cat", Definite: true},
		}, Past: past}
	}
	intransitive := func(past bool) Meaning {
		return Meaning{Relation: Term{Phrase: "run"}, Roles: map[string]Term{
			"agent": {Phrase: "dog", Definite: true},
		}, Past: past}
	}
	cases := []struct {
		name string
		m    Meaning
	}{
		{"transitive past", transitive(true)},
		{"transitive present", transitive(false)},
		{"intransitive past", intransitive(true)},
		{"intransitive present", intransitive(false)},
	}
	for _, c := range cases {
		want, wok := p.Speak("en", c.m)
		got, gok := p.SpeakDecl("en", c.m)
		if !gok {
			t.Fatalf("%s: SpeakDecl declined a clause it should speak", c.name)
		}
		if !wok || got != want {
			t.Fatalf("%s: SpeakDecl %q should match Speak %q (ok %v)", c.name, got, want, wok)
		}
	}
}

// SpeakDecl declines the cases the declarative grammar does not yet cover, so the shadow stays
// silent (not a false mismatch) and the caller keeps the coded clause.
func TestSpeakDeclDeclinesOutOfScope(t *testing.T) {
	p := Builtin()
	transitive := map[string]Term{
		"agent":   {Phrase: "dog", Definite: true},
		"patient": {Phrase: "cat", Definite: true},
	}
	cases := []struct {
		name string
		m    Meaning
	}{
		{"negation", Meaning{Relation: Term{Phrase: "chase"}, Roles: transitive, Negate: true}},
		{"unseeded verb", Meaning{Relation: Term{Phrase: "zorp"}, Roles: transitive}},
		{"given referent", Meaning{Relation: Term{Phrase: "chase"}, Roles: map[string]Term{
			"agent": {Given: true}, "patient": {Phrase: "cat", Definite: true}}}},
		{"coordination", Meaning{Relation: Term{Phrase: "chase"}, Roles: map[string]Term{
			"agent":   {Phrase: "dog", Definite: true},
			"patient": {Phrase: "cat", Definite: true, With: []string{"mouse"}}}}},
		{"indefinite", Meaning{Relation: Term{Phrase: "chase"}, Roles: map[string]Term{
			"agent": {Phrase: "dog"}, "patient": {Phrase: "cat", Definite: true}}}},
	}
	for _, c := range cases {
		if got, ok := p.SpeakDecl("en", c.m); ok {
			t.Fatalf("%s: SpeakDecl should decline, got %q", c.name, got)
		}
	}
}

// SpeakDecl inflects the subject noun for number from the lexicon, which the coded Speak path
// leaves to its caller. The live shadow never sets the plural flag, so this divergence does
// not fire there; it is the declarative engine doing from data what Speak does not.
func TestSpeakDeclPluralizesNoun(t *testing.T) {
	p := Builtin()
	m := Meaning{Relation: Term{Phrase: "chase"}, Roles: map[string]Term{
		"agent":   {Phrase: "dog", Definite: true, Plural: true},
		"patient": {Phrase: "cat", Definite: true},
	}}
	if got, ok := p.SpeakDecl("en", m); !ok || got != "The dogs chase the cat." {
		t.Fatalf("SpeakDecl should pluralize the subject noun: %q %v", got, ok)
	}
}

// A tense feature selects a tense-keyed lexical form ahead of the number-keyed one, and its
// absence leaves the number lookup unchanged (the present clause and the bare noun phrase).
func TestDeclTenseFormSelection(t *testing.T) {
	entries := []Param{
		{Key: "syn:en:s", Value: "subj vp | num:subj>vp | vp", Lang: "en"},
		{Key: "syn:en:vp", Value: "v | | v", Lang: "en"},
		{Key: "syn:en:np", Value: "det n | | n", Lang: "en"},
		{Key: "lex:en:the", Value: "cat=det;any=the", Lang: "en"},
		{Key: "lex:en:fox", Value: "cat=n;sg=fox;pl=foxes", Lang: "en"},
		{Key: "lex:en:jump", Value: "cat=v;sg=jumps;pl=jump;past=jumped", Lang: "en"},
	}
	p := Compile(entries, nil).SetDecl(entries)
	np := Phrase("np", fs{"num": "sg"}, map[string]*GenNode{"det": Leaf("the", nil), "n": Leaf("fox", fs{"num": "sg"})})
	vp := func() *GenNode { return Phrase("vp", nil, map[string]*GenNode{"v": Leaf("jump", nil)}) }
	past := Phrase("s", fs{"tense": "past"}, map[string]*GenNode{"subj": np, "vp": vp()})
	if got, _, ok := p.Generate("en", past); !ok || got != "the fox jumped" {
		t.Fatalf("a past clause should select the past form: %q %v", got, ok)
	}
	pres := Phrase("s", nil, map[string]*GenNode{"subj": np, "vp": vp()})
	if got, _, ok := p.Generate("en", pres); !ok || got != "the fox jumps" {
		t.Fatalf("a clause with no tense should select the number form: %q %v", got, ok)
	}
}
