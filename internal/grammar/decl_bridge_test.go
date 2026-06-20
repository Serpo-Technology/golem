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
		{"indefinite", Meaning{Relation: Term{Phrase: "chase"}, Roles: map[string]Term{
			"agent": {Phrase: "dog"}, "patient": {Phrase: "cat", Definite: true}}}},
	}
	for _, c := range cases {
		if got, ok := p.SpeakDecl("en", c.m); ok {
			t.Fatalf("%s: SpeakDecl should decline, got %q", c.name, got)
		}
	}
}

// SpeakDecl renders open vocabulary the curriculum never seeded by minting a lexical entry
// from the coded morphology, so the shadow fires on learned words and still matches Speak: the
// minted verb and noun forms come from the same rules Speak conjugates from.
func TestSpeakDeclMintsOpenVocabulary(t *testing.T) {
	p := Builtin()
	m := Meaning{Relation: Term{Phrase: "watch"}, Roles: map[string]Term{
		"agent":   {Phrase: "wolf", Definite: true},
		"patient": {Phrase: "fox", Definite: true},
	}, Past: true}
	want, wok := p.Speak("en", m)
	got, gok := p.SpeakDecl("en", m)
	if !gok {
		t.Fatal("SpeakDecl should mint entries for unseeded vocabulary and speak the clause")
	}
	if !wok || got != want || got != "The wolf watched the fox." {
		t.Fatalf("SpeakDecl %q should match Speak %q (want \"The wolf watched the fox.\")", got, want)
	}
}

// The engine stacks Turkish verb morphemes, each a learned rule: negation, tense, and person
// fold in order, so "Köpek kuşu kovalamadı." and "Ben kuşu kovalamadım." generate from the
// composed rules and match the coded Speak. This is the agglutinative case, learned morphemes
// composing rather than one rule per form.
func TestSpeakDeclTurkishNegationComposes(t *testing.T) {
	p := Compile(Curriculum(), nil).
		SetConstructions(Constructions()).
		SetDecl(Curriculum()).
		SetArgConstructions(Constructions())
	p.LearnInflection("tr", "neg", [][2]string{
		{"kovala", "kovalama"}, {"gel", "gelme"}, {"oku", "okuma"}, {"gör", "görme"},
	})
	p.LearnInflection("tr", "past", [][2]string{
		{"kovala", "kovaladı"}, {"gel", "geldi"}, {"oku", "okudu"}, {"gör", "gördü"},
	})
	p.LearnInflection("tr", "pers1sg", [][2]string{
		{"kovaladı", "kovaladım"}, {"geldi", "geldim"}, {"okudu", "okudum"}, {"gördü", "gördüm"},
	})
	cases := map[string]struct {
		agent string
		want  string
	}{
		"third person negative": {"köpek", "Köpek kuşu kovalamadı."},
		"first person negative": {"ben", "Ben kuşu kovalamadım."},
	}
	for name, c := range cases {
		m := Meaning{Relation: Term{Phrase: "kovala"}, Roles: map[string]Term{
			"agent":   {Phrase: c.agent},
			"patient": {Phrase: "kuş", Definite: true},
		}, Past: true, Negate: true}
		want, wok := p.Speak("tr", m)
		got, gok := p.SpeakDecl("tr", m)
		if !gok || !wok || got != want || got != c.want {
			t.Fatalf("%s: SpeakDecl %q should match Speak %q (want %q)", name, got, want, c.want)
		}
	}
}

// SpeakDecl speaks the coordinated and pronominal clauses that multi-clause answers produce,
// matching Speak: a coordinated object joins its parts with the coordinator, and a referent in
// play speaks as its pronoun. These are what renderFacts emits beyond the first plain clause.
func TestSpeakDeclCoordinationAndPronoun(t *testing.T) {
	p := Builtin()
	cases := []struct {
		name string
		m    Meaning
	}{
		{"coordinated object", Meaning{Relation: Term{Phrase: "chase"}, Roles: map[string]Term{
			"agent":   {Phrase: "dog", Definite: true},
			"patient": {Phrase: "cat", Definite: true, With: []string{"mouse"}},
		}, Past: true}},
		{"given agent", Meaning{Relation: Term{Phrase: "chase"}, Roles: map[string]Term{
			"agent":   {Given: true},
			"patient": {Phrase: "cat", Definite: true},
		}, Past: true}},
		{"given patient", Meaning{Relation: Term{Phrase: "chase"}, Roles: map[string]Term{
			"agent":   {Phrase: "dog", Definite: true},
			"patient": {Given: true},
		}, Past: true}},
	}
	for _, c := range cases {
		want, wok := p.Speak("en", c.m)
		got, gok := p.SpeakDecl("en", c.m)
		if !gok {
			t.Fatalf("%s: SpeakDecl should speak the clause", c.name)
		}
		if !wok || got != want {
			t.Fatalf("%s: SpeakDecl %q should match Speak %q", c.name, got, want)
		}
	}
}

// The learned Turkish harmony rule drives the verb of a clause the engine generates: with the
// -DI past taught, SpeakDecl speaks "Köpek kuşu kovaladı." through the declarative grammar,
// matching the coded Speak, and without the rule it declines (the verb has no other source), so
// the verb form can only have come from the learned rule. The accusative object stays coded
// phonology, minted to match Speak. This is the payoff: taught morphology generating an answer.
func TestSpeakDeclTurkishLearnedVerb(t *testing.T) {
	p := Compile(Curriculum(), nil).
		SetConstructions(Constructions()).
		SetDecl(Curriculum()).
		SetArgConstructions(Constructions())
	m := Meaning{Relation: Term{Phrase: "kovala"}, Roles: map[string]Term{
		"agent":   {Phrase: "köpek", Definite: true},
		"patient": {Phrase: "kuş", Definite: true},
	}, Past: true}
	if _, ok := p.SpeakDecl("tr", m); ok {
		t.Fatal("without the learned past rule the verb is not formable; SpeakDecl should decline")
	}
	if !p.LearnInflection("tr", "past", [][2]string{
		{"kovala", "kovaladı"}, {"gel", "geldi"}, {"oku", "okudu"},
		{"gör", "gördü"}, {"bak", "baktı"}, {"geç", "geçti"},
	}) {
		t.Fatal("the harmonic -DI past should be learned")
	}
	want, wok := p.Speak("tr", m)
	got, gok := p.SpeakDecl("tr", m)
	if !gok {
		t.Fatal("with the rule taught, SpeakDecl should speak the clause")
	}
	if !wok || got != want || got != "Köpek kuşu kovaladı." {
		t.Fatalf("SpeakDecl %q should match Speak %q (want \"Köpek kuşu kovaladı.\")", got, want)
	}
}

// Turkish person agreement comes from per-person past rules the engine selects by the
// subject's person: the first-person past is its own learned harmonic rule (the -m suffix
// riding the harmony, frame d...m), so "Ben kuşu kovaladım." generates with its verb from that
// rule and matches the coded Speak, while a third-person subject keeps the bare -DI past.
func TestSpeakDeclTurkishPersonAgreement(t *testing.T) {
	p := Compile(Curriculum(), nil).
		SetConstructions(Constructions()).
		SetDecl(Curriculum()).
		SetArgConstructions(Constructions())
	p.LearnInflection("tr", "past", [][2]string{
		{"kovala", "kovaladı"}, {"gel", "geldi"}, {"oku", "okudu"}, {"gör", "gördü"},
	})
	// The first person is its own morpheme stacked after the tense, learned from past-to-first
	// person pairs (the -m suffix), so the engine composes kovaladı + m.
	p.LearnInflection("tr", "pers1sg", [][2]string{
		{"kovaladı", "kovaladım"}, {"geldi", "geldim"}, {"okudu", "okudum"}, {"gördü", "gördüm"},
	})
	cases := map[string]struct {
		agent string
		want  string
	}{
		"first person": {"ben", "Ben kuşu kovaladım."},
		"third person": {"köpek", "Köpek kuşu kovaladı."},
	}
	for name, c := range cases {
		m := Meaning{Relation: Term{Phrase: "kovala"}, Roles: map[string]Term{
			"agent":   {Phrase: c.agent},
			"patient": {Phrase: "kuş", Definite: true},
		}, Past: true}
		want, wok := p.Speak("tr", m)
		got, gok := p.SpeakDecl("tr", m)
		if !gok || !wok || got != want || got != c.want {
			t.Fatalf("%s: SpeakDecl %q should match Speak %q (want %q)", name, got, want, c.want)
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
