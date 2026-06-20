package grammar

// SpeakDecl realizes a proposition through the declarative engine, the data-driven dual of
// Speak: it builds a GenNode tree from the meaning and runs Generate, so the clause's word
// order, agreement, and morphology come from the grammar parameters rather than coded
// realization. It declines (false) on anything the seeded declarative grammar does not yet
// cover, so a caller can shadow it against Speak and fall back rather than speak a broken
// clause. A success is routed through Sentence, so it is character-identical to Speak.
func (p *Params) SpeakDecl(lang string, m Meaning) (string, bool) {
	p = p.get()
	if lang == "" {
		lang = "en"
	}
	if m.Negate {
		return "", false // do-support is not in the declarative grammar yet
	}
	g := p.decl[lang]
	if g == nil {
		return "", false
	}
	verb := m.Relation.Speak()
	if verb == "" {
		return "", false
	}
	if p.Article(lang) == "" {
		// A tongue that marks definiteness by case rather than an article (Turkish).
		return p.speakDeclCased(g, lang, m, verb)
	}
	cxn, ok := p.selectArg(lang, m)
	if !ok {
		return "", false
	}
	roles := map[string]*GenNode{}
	for _, s := range cxn.Slots {
		if s.Role == cxn.Pred {
			continue
		}
		np, ok := nounPhrase(p, g, lang, m.Roles[s.Role])
		if !ok {
			return "", false
		}
		roles[s.Role] = np
	}
	subj, ok := roles["agent"]
	if !ok {
		return "", false
	}
	vpRule := "vp"
	vpKids := map[string]*GenNode{"v": Leaf(verb, fs{"cat": "v"})}
	if obj, ok := roles["patient"]; ok {
		vpRule = "vpt"
		vpKids["obj"] = obj
	}
	clause := fs{}
	if m.Past {
		clause["tense"] = "past" // selects the verb's stored past form in the leaf lookup
	}
	root := Phrase("s", clause, map[string]*GenNode{
		"subj": subj,
		"vp":   Phrase(vpRule, nil, vpKids),
	})
	surface, _, ok := p.Generate(lang, root)
	if !ok {
		return "", false
	}
	return Sentence(surface), true
}

// speakDeclCased speaks a clause for a tongue that marks definiteness by case rather than an
// article (Turkish): the subject is bare nominative, the definite object takes the accusative,
// the verb closes the clause (the syn rule's SOV order), and the verb's past comes from the
// learned harmony rule. It is transitive past only for now; every other shape declines, so the
// shadow stays silent and the caller keeps the coded clause.
func (p *Params) speakDeclCased(g *DeclGrammar, lang string, m Meaning, verb string) (string, bool) {
	if !m.Past || !p.HasInflection(lang, "past") {
		return "", false // the verb morphology must be a learned rule, else fall back
	}
	agent, patient := m.Roles["agent"], m.Roles["patient"]
	if agent.Empty() || agent.Given || len(agent.With) > 0 || agent.Speak() == "" {
		return "", false
	}
	if patient.Empty() || patient.Given || len(patient.With) > 0 || !patient.Definite || patient.Speak() == "" {
		return "", false
	}
	root := Phrase("s", fs{"tense": "past"}, map[string]*GenNode{
		"subj": Leaf(agent.Speak(), fs{"cat": "n"}),
		"obj":  Leaf(patient.Speak(), fs{"cat": "n", "case": "acc"}),
		"v":    Leaf(verb, fs{"cat": "v"}),
	})
	surface, _, ok := p.Generate(lang, root)
	if !ok {
		return "", false
	}
	return Sentence(surface), true
}

// nounPhrase builds a definite np GenNode for a role's filler, declining on the cases the
// seeded declarative grammar does not yet render: a referent in play (a pronoun), a
// coordinated co-argument, an indefinite or bare phrase, or a word with no lexical entry. The
// noun carries its number, and the phrase takes the tongue's definite determiner.
func nounPhrase(p *Params, g *DeclGrammar, lang string, t Term) (*GenNode, bool) {
	if t.Empty() || t.Given || len(t.With) > 0 || !t.Definite {
		return nil, false
	}
	word := t.Speak()
	if word == "" {
		return nil, false
	}
	art := p.Article(lang)
	if art == "" {
		return nil, false // a tongue marking definiteness by case is a later stage
	}
	if _, ok := g.lex[art]; !ok {
		return nil, false
	}
	num := "sg"
	if t.Plural {
		num = "pl"
	}
	// The noun carries its category so the engine can mint a lexical entry for it when the
	// curriculum never seeded the word, the open-vocabulary path.
	return Phrase("np", fs{"num": num}, map[string]*GenNode{
		"det": Leaf(art, nil),
		"n":   Leaf(word, fs{"num": num, "cat": "n"}),
	}), true
}

// mintLex synthesizes a lexical entry for a word the curriculum never seeded, so the
// declarative engine renders open vocabulary. The forms come from the same coded morphology
// Speak conjugates from (the verb's principal parts, the noun's plural), so a minted clause
// matches the coded clause. English only: these are English rules, and a learned language
// inflects by its own rules through the inflect fallback instead.
func (p *Params) mintLex(lang, lemma, cat string) (lexEntry, bool) {
	if lemma == "" {
		return lexEntry{}, false
	}
	switch lang {
	case "en":
		switch cat {
		case "n":
			return lexEntry{cat: "n", forms: map[string]string{"sg": lemma, "pl": pluralEN(lemma)}}, true
		case "v":
			v := p.verb(lemma)
			return lexEntry{cat: "v", forms: map[string]string{"sg": v.third, "pl": v.base, "past": v.past}}, true
		}
	case "tr":
		switch cat {
		case "n":
			// The bare lemma is the nominative; the accusative stays coded phonology (the
			// y-buffer and final-stop softening the learned harmonic rule does not yet model).
			// Built from the suffix recipe directly, the way accusativeCommonTR does.
			stem := lemma
			if !endsVowelTR(lemma) {
				stem = softenFinalTR(lemma)
			}
			acc := stem + applySuffixTR(lemma, p.suffix("accusative", "(y)I"))
			return lexEntry{cat: "n", forms: map[string]string{"any": lemma, "acc": acc}}, true
		case "v":
			// No stored forms: the verb's tense comes from the learned harmony rule.
			return lexEntry{cat: "v", forms: map[string]string{}}, true
		}
	}
	return lexEntry{}, false
}
