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
	g := p.decl[lang]
	if g == nil {
		return "", false
	}
	verb := m.Relation.Speak()
	if verb == "" {
		return "", false
	}
	if p.Article(lang) == "" {
		// A tongue that marks definiteness by case rather than an article (Turkish), where the
		// verb stacks negation as one more learned morpheme.
		return p.speakDeclCased(g, lang, m, verb)
	}
	if m.Negate {
		return "", false // English do-support negation is not in the declarative grammar yet
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
		np, ok := nounPhrase(p, g, lang, s.Role, m.Roles[s.Role])
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
	if !m.Past {
		return "", false
	}
	agent, patient := m.Roles["agent"], m.Roles["patient"]
	if agent.Empty() || agent.Given || len(agent.With) > 0 || agent.Speak() == "" {
		return "", false
	}
	if patient.Empty() || patient.Given || len(patient.With) > 0 || !patient.Definite || patient.Speak() == "" {
		return "", false
	}
	if hasDeterminer(patient.Speak()) {
		return "", false // a name already carrying a determiner takes no accusative here
	}
	// The verb stacks its morphemes, each a learned rule the engine folds in order: tense, then
	// the subject's person, and negation ahead of both. The clause carries the features; the
	// verb leaf forms itself from them.
	clause := fs{"tense": "past"}
	if !p.HasInflection(lang, "past") {
		return "", false // the verb morphology must be a learned rule, else fall back
	}
	switch p.trPersonOf(agent.Speak()) {
	case trPerson1sg:
		clause["pers"] = "pers1sg"
	case trPerson2sg:
		clause["pers"] = "pers2sg"
	case trPerson1pl:
		clause["pers"] = "pers1pl"
	}
	if pers := clause["pers"]; pers != "" && !p.HasInflection(lang, pers) {
		return "", false
	}
	if m.Negate {
		if !p.HasInflection(lang, "neg") {
			return "", false
		}
		clause["pol"] = "neg"
	}
	root := Phrase("s", clause, map[string]*GenNode{
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
func nounPhrase(p *Params, g *DeclGrammar, lang, role string, t Term) (*GenNode, bool) {
	if t.Empty() {
		return nil, false
	}
	// A referent already in play speaks as the role's pronoun.
	if t.Given {
		pron := p.Pronoun(lang, role)
		if pron == "" {
			return nil, false
		}
		if _, ok := g.lex[pron]; !ok {
			return nil, false
		}
		return Leaf(pron, fs{"cat": "pron", "num": "sg"}), true
	}
	if !t.Definite {
		return nil, false // indefinite noun phrases are a later stage
	}
	head, ok := definiteNoun(p, g, lang, t.Speak(), t.Plural)
	if !ok {
		return nil, false
	}
	if len(t.With) == 0 {
		return head, true
	}
	// Coordinated co-arguments: the head and each, joined by the tongue's coordinator.
	items := []*GenNode{head}
	for _, w := range t.With {
		np, ok := definiteNoun(p, g, lang, w, false)
		if !ok {
			return nil, false
		}
		items = append(items, np)
	}
	coord := p.Coordinator(lang)
	if coord == "" {
		return nil, false
	}
	if _, ok := g.lex[coord]; !ok {
		return nil, false
	}
	return coordNode(coord, items), true
}

// definiteNoun builds a definite noun phrase: the tongue's determiner and the noun, the noun
// carrying its number and category so the engine can mint an entry for a learned word.
func definiteNoun(p *Params, g *DeclGrammar, lang, word string, plural bool) (*GenNode, bool) {
	if word == "" {
		return nil, false
	}
	if hasDeterminer(word) {
		return nil, false // a name already carrying a determiner is left to the coded path
	}
	art := p.Article(lang)
	if art == "" {
		return nil, false // a tongue marking definiteness by case is a later stage
	}
	if _, ok := g.lex[art]; !ok {
		return nil, false
	}
	num := "sg"
	if plural {
		num = "pl"
	}
	return Phrase("np", fs{"num": num}, map[string]*GenNode{
		"det": Leaf(art, nil),
		"n":   Leaf(word, fs{"num": num, "cat": "n"}),
	}), true
}

// coordNode right-nests a coordination over the items, so "the cat and the mouse and the bird"
// reads as a chain of the coordinator rule, the coordinator between each pair.
func coordNode(coord string, items []*GenNode) *GenNode {
	node := items[len(items)-1]
	for i := len(items) - 2; i >= 0; i-- {
		node = Phrase("coord", nil, map[string]*GenNode{
			"a": items[i],
			"c": Leaf(coord, fs{"cat": "c"}),
			"b": node,
		})
	}
	return node
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
