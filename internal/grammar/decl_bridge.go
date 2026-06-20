package grammar

import (
	"strings"

	"golem/internal/limbic"
)

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
	agent, patient := m.Roles["agent"], m.Roles["patient"]
	if patient.Empty() {
		return "", false // transitive only
	}
	subj, ok := casedNoun(p, g, lang, "agent", agent, false)
	if !ok {
		return "", false
	}
	obj, ok := casedNoun(p, g, lang, "patient", patient, true)
	if !ok {
		return "", false
	}
	// The verb stacks its morphemes, each a learned rule the engine folds in order. The clause
	// carries the features; the verb leaf forms itself from them.
	clause := fs{}
	if m.Past {
		// The past agrees with the subject's person, the suffix riding after the tense, and the
		// negation goes ahead of both.
		clause["tense"] = "past"
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
	} else {
		// The present is the aorist: -r after a vowel, the harmonic -Ir after a consonant, with
		// no person suffix, the way the coded present forms it. The negative aorist (-mAz) is a
		// separate fused form, deferred to the coded path.
		if m.Negate {
			return "", false
		}
		tense := "aorist"
		if !endsVowelTR(verb) {
			tense = "aoristC"
		}
		if !p.HasInflection(lang, tense) {
			return "", false
		}
		clause["tense"] = tense
	}
	root := Phrase("s", clause, map[string]*GenNode{
		"subj": subj,
		"obj":  obj,
		"v":    Leaf(verb, fs{"cat": "v"}),
	})
	surface, _, gok := p.Generate(lang, root)
	if !gok {
		return "", false
	}
	return Sentence(surface), true
}

// casedNoun builds a role's node for a case-marking tongue: a referent in play speaks as its
// pronoun (already the right case), a definite object takes the accusative, the subject is bare
// nominative, and coordinated co-arguments join under the coordinator, each marked alike.
func casedNoun(p *Params, g *DeclGrammar, lang, role string, t Term, accusative bool) (*GenNode, bool) {
	if t.Empty() {
		return nil, false
	}
	if t.Given {
		pron := p.Pronoun(lang, role)
		if pron == "" {
			return nil, false
		}
		return Leaf(pron, fs{"cat": "n"}), true // o / onu, already carrying its case
	}
	if accusative && !t.Definite {
		return nil, false // an indefinite object stays nominative, a later stage
	}
	mk := func(w string) (*GenNode, bool) {
		if w == "" || hasDeterminer(w) {
			return nil, false
		}
		f := fs{"cat": "n"}
		if accusative {
			f = fs{"cat": "n", "case": "acc"}
		}
		return Leaf(w, f), true
	}
	head, ok := mk(t.Speak())
	if !ok {
		return nil, false
	}
	if len(t.With) == 0 {
		return head, true
	}
	items := []*GenNode{head}
	for _, w := range t.With {
		np, ok := mk(w)
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

// RealizeDecl realizes a Frame through the declarative engine, the data-driven dual of Realize
// for the clauses it covers. This handles the copula statement ("the dog is an animal", "make
// test bir eylemdir") and declines every other shape, so a caller can shadow it against Realize
// and fall back. The copula's agent and complement arrive already rendered, so they ride as
// verbatim leaves; what the engine supplies is the clause order and the copula form (is / are /
// was / were, the Turkish -DIr) from grammar data. chem is taken to match Realize's signature;
// the copula reads none.
func (p *Params) RealizeDecl(f Frame, chem limbic.State) (string, bool) {
	p = p.get()
	lang := f.Lang
	if lang == "" {
		lang = "en"
	}
	if p.decl[lang] == nil {
		return "", false
	}
	switch f.Action.Phrase {
	case "be":
		return p.realizeCopula(lang, f)
	case "follow", "precede", "happen", "run", "fail":
		return p.realizeEvent(lang, f, chem)
	}
	return "", false
}

// realizeCopula builds the copula clause, statement or question, through the engine. The agent
// and complement ride verbatim (the frame hands them already rendered); the copula form
// (is/are/was/were, the Turkish -DIr) and the clause order come from grammar data.
func (p *Params) realizeCopula(lang string, f Frame) (string, bool) {
	if len(f.Quals) > 0 || f.Hedge || f.Habit || f.Last {
		return "", false // the copula branch ignores these; declining keeps the shadow honest
	}
	comp := f.Patient.Speak()
	if comp == "" {
		return "", false
	}
	if f.Force == ForceQuestion {
		// "What is X?" / "X nedir?": the asked thing rides verbatim, the interrogative frame
		// from grammar data. Turkish fronts the thing and closes with "nedir", English fronts
		// the wh word and the copula.
		var root *GenNode
		if p.Article(lang) == "" {
			root = Phrase("qcop", nil, map[string]*GenNode{
				"comp": Leaf(comp, fs{"cat": "comp"}),
				"q":    Leaf("nedir", fs{"cat": "q"}),
			})
		} else {
			root = Phrase("qcop", nil, map[string]*GenNode{
				"wh":   Leaf("what", fs{"cat": "wh"}),
				"cop":  Leaf("be", fs{"cat": "cop", "num": "sg"}),
				"comp": Leaf(comp, fs{"cat": "comp"}),
			})
		}
		surface, _, ok := p.Generate(lang, root)
		if !ok {
			return "", false
		}
		return sentence(surface, "?"), true
	}
	if f.Force != ForceStatement {
		return "", false
	}
	if f.Agent.Empty() || f.Agent.Given {
		return "", false
	}
	subjPhrase := f.Agent.Speak()
	if subjPhrase == "" {
		return "", false
	}
	if p.Article(lang) == "" {
		// A tongue whose copula is a suffix: the Turkish -DIr on the complement's last word.
		if f.Negate {
			return "", false // Turkish copula negation is a separate form, deferred
		}
		words := strings.Fields(comp)
		words[len(words)-1] = copulaTR(p, words[len(words)-1])
		root := Phrase("scop", nil, map[string]*GenNode{
			"subj": Leaf(subjPhrase, fs{"cat": "comp"}),
			"comp": Leaf(strings.Join(words, " "), fs{"cat": "comp"}),
		})
		surface, _, ok := p.Generate(lang, root)
		if !ok {
			return "", false
		}
		return Sentence(surface), true
	}
	// An article tongue with a copula word (English is / are / was / were).
	if subjPhrase == "I" {
		return "", false // the first-person copula (am) is not seeded, deferred
	}
	num := "sg"
	if f.Agent.Plural {
		num = "pl"
	}
	copLemma := "be"
	if f.Negate {
		copLemma = "be_neg" // the " not" rides in the seeded form, no do-support
	}
	clause := fs{}
	if f.Past {
		clause["tense"] = "past"
	}
	root := Phrase("scop", clause, map[string]*GenNode{
		"subj": Leaf(subjPhrase, fs{"cat": "comp", "num": num}),
		"cop":  Leaf(copLemma, fs{"cat": "cop"}),
		"comp": Leaf(comp, fs{"cat": "comp"}),
	})
	surface, _, ok := p.Generate(lang, root)
	if !ok {
		return "", false
	}
	return Sentence(surface), true
}

// realizeEvent builds the plain event clause ("the dog follows the cat") through the engine,
// reusing the proposition clause's rules so the verb's form comes from the same minted and
// learned morphology while the agent and patient ride verbatim. It declines the aspected,
// negated, hedged, place-marked, adjuncted, and Turkish shapes, each a later stage, so the
// shadow stays honest.
func (p *Params) realizeEvent(lang string, f Frame, chem limbic.State) (string, bool) {
	if f.Force != ForceStatement {
		return "", false
	}
	if p.Article(lang) == "" {
		return "", false // Turkish event verbs are a suppletive paradigm, a later stage
	}
	if f.At != "" {
		return "", false // the place complement is a later stage
	}
	if f.Agent.Empty() || f.Agent.Given {
		return "", false
	}
	subject := f.Agent.Speak()
	if subject == "" {
		return "", false
	}
	// The first person and the plural both take the base form, the way finiteVerb does.
	num := "sg"
	if subject == "I" || f.Agent.Plural {
		num = "pl"
	}
	clause := fs{}
	if f.Past {
		clause["tense"] = "past"
	}
	hasObj := !f.Patient.Empty()
	// The verb group follows the coded order of precedence: negation, then the hedge a corrected
	// system reaches for, then the habitual or last adverb, then the plain finite verb.
	hedge := !f.Negate && (f.Hedge || chem.Oxytocin < limbic.OxytocinBaseline)
	var vpRule string
	var vpKids map[string]*GenNode
	switch {
	case f.Negate:
		// Do-support: the auxiliary carries agreement and tense, the main verb stays its base.
		vpKids = map[string]*GenNode{
			"aux": Leaf("do", fs{"cat": "aux"}),
			"neg": Leaf("not", fs{"cat": "neg"}),
			"v":   Leaf(f.Action.Phrase, fs{"cat": "v", "num": "pl"}),
		}
		vpRule = "vpni"
		if hasObj {
			vpRule = "vpn"
			vpKids["obj"] = Leaf(f.Patient.Speak(), fs{"cat": "comp"})
		}
	case hedge:
		// The modal carries no agreement; the present takes the base, the past "have" plus the
		// participle, which for these regular verbs is the past form.
		vpKids = map[string]*GenNode{"modal": Leaf("may", fs{"cat": "modal"})}
		if f.Past {
			vpKids["aux"] = Leaf("have", fs{"cat": "aux"})
			vpKids["v"] = Leaf(f.Action.Phrase, fs{"cat": "v", "tense": "past"})
			vpRule = "vpmpi"
			if hasObj {
				vpRule = "vpmp"
				vpKids["obj"] = Leaf(f.Patient.Speak(), fs{"cat": "comp"})
			}
		} else {
			vpKids["v"] = Leaf(f.Action.Phrase, fs{"cat": "v", "num": "pl"})
			vpRule = "vpmi"
			if hasObj {
				vpRule = "vpm"
				vpKids["obj"] = Leaf(f.Patient.Speak(), fs{"cat": "comp"})
			}
		}
	default:
		// The habitual and the last occurrence prefix an adverb; the habit wins when both are set.
		adv := ""
		if f.Habit {
			adv = "usually"
		} else if f.Last {
			adv = "last"
		}
		vpKids = map[string]*GenNode{"v": Leaf(f.Action.Phrase, fs{"cat": "v"})}
		if hasObj {
			vpKids["obj"] = Leaf(f.Patient.Speak(), fs{"cat": "comp"})
		}
		if adv != "" {
			vpKids["adv"] = Leaf(adv, fs{"cat": "adv"})
		}
		switch {
		case adv != "" && hasObj:
			vpRule = "vpa"
		case adv != "":
			vpRule = "vpai"
		case hasObj:
			vpRule = "vpt"
		default:
			vpRule = "vp"
		}
	}
	root := Phrase("s", clause, map[string]*GenNode{
		"subj": Leaf(subject, fs{"cat": "comp", "num": num}),
		"vp":   Phrase(vpRule, nil, vpKids),
	})
	if len(f.Quals) > 0 {
		// The temporal adjuncts front the clause, but only for the verbs Realize fronts; the
		// others drop them, so decline rather than risk a silent divergence. Each is the relator
		// and its anchor, coordinated, the whole closed by a comma the layout joins to the clause.
		if f.Action.Phrase != "follow" && f.Action.Phrase != "happen" && f.Action.Phrase != "run" {
			return "", false
		}
		parts := make([]string, 0, len(f.Quals))
		for _, q := range f.Quals {
			if q.Anchor.Empty() {
				return "", false
			}
			parts = append(parts, relWord(q.Rel)+" "+q.Anchor.Speak())
		}
		root = Phrase("sadj", nil, map[string]*GenNode{
			"adj": Leaf(strings.Join(parts, " and ")+",", fs{"cat": "comp"}),
			"s":   root,
		})
	}
	surface, _, ok := p.Generate(lang, root)
	if !ok {
		return "", false
	}
	return Sentence(surface), true
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
	if cat == "comp" {
		// A pre-rendered phrase (a copula's agent or complement) rides verbatim as one form.
		return lexEntry{cat: "comp", forms: map[string]string{"any": lemma}}, true
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
