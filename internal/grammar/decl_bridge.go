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
	cxn, ok := p.selectArg(lang, m)
	if !ok {
		return "", false
	}
	verb := m.Relation.Speak()
	if verb == "" {
		return "", false
	}
	if _, seeded := g.lex[verb]; !seeded {
		return "", false // an unseeded verb has no declarative lexical entry
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
	vpKids := map[string]*GenNode{"v": Leaf(verb, nil)}
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

// nounPhrase builds a definite np GenNode for a role's filler, declining on the cases the
// seeded declarative grammar does not yet render: a referent in play (a pronoun), a
// coordinated co-argument, an indefinite or bare phrase, or a word with no lexical entry. The
// noun carries its number, and the phrase takes the tongue's definite determiner.
func nounPhrase(p *Params, g *DeclGrammar, lang string, t Term) (*GenNode, bool) {
	if t.Empty() || t.Given || len(t.With) > 0 || !t.Definite {
		return nil, false
	}
	word := t.Speak()
	if _, ok := g.lex[word]; !ok {
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
	return Phrase("np", fs{"num": num}, map[string]*GenNode{
		"det": Leaf(art, nil),
		"n":   Leaf(word, fs{"num": num}),
	}), true
}
