package grammar

import (
	"strings"

	"golem/internal/memory"
)

// An argument-structure construction is a two-poled form-and-meaning pair:
// a form pole of ordered, typed slots (the word order and the syntactic
// shape of each argument) and a meaning pole of thematic roles over a
// relation (the predicate and what it binds). One inventory of role names
// is shared between the poles, so a single engine runs the construction in
// both directions: meaning to form is generation, form to meaning is
// comprehension, the same construction read each way. Where the
// answer-template Construction keys a surface string by a bare intent
// label, this carries a grounded role structure as its meaning, the
// deepening the comprehension engine stands on.
type ArgConstruction struct {
	Name   string    // the pattern it names: "transitive", "intransitive"
	Lang   string    // the language it speaks and parses
	Weight float64   // entrenchment; the strongest binding construction wins
	Pred   string    // the role that carries the relation, the predicate
	Slots  []ArgSlot // the form pole, in this language's citation order
}

// ArgSlot is one slot of the form pole: a thematic role realized as a
// syntactic type (a noun phrase, a verb), with an optional feature the
// best-fit binding favors in this role's filler, so an animate concept is
// the better agent.
type ArgSlot struct {
	Role   string // the thematic role this slot fills: agent, patient, rel
	Syn    string // the syntactic type the surface expects: np, v
	Prefer string // a feature best fit favors in the filler, empty for none
}

// argRoles lists the non-predicate roles a construction binds, the
// arguments a meaning must supply for the construction to speak it.
func (c ArgConstruction) argRoles() []string {
	out := make([]string, 0, len(c.Slots))
	for _, s := range c.Slots {
		if s.Role != c.Pred {
			out = append(out, s.Role)
		}
	}
	return out
}

// Meaning is a proposition made concrete: a relation over thematic roles,
// each filled by a concept where grounding found one and a bare phrase
// where it did not. It is the deepened meaning pole as data, the structure
// comprehension produces from an utterance and generation realizes back
// into one, the same shape written into the graph as the proposition the
// brain can be asked about.
type Meaning struct {
	Relation Term            // the predicate; the relation the roles bind into
	Roles    map[string]Term // thematic role -> filler
	Past     bool            // the clause is about a settled moment
	Negate   bool            // the relation is denied, "did not chase"
	Lang     string          // the language it arrived in, or speaks back
}

// Speak realizes a proposition through the argument-structure
// constructions: the meaning's roles pick the construction that binds them,
// its slots lay out in the language's word order, each noun-phrase role
// speaks its filler, and the predicate role is conjugated by the morphology
// engine. This is the one engine run meaning to form. It returns false when
// no construction binds the meaning's roles, so the caller can fall back to
// the procedural realizer rather than speak a broken clause.
func (p *Params) Speak(lang string, m Meaning) (string, bool) {
	p = p.get()
	if lang == "" {
		lang = "en"
	}
	cxn, ok := p.selectArg(lang, m)
	if !ok {
		return "", false
	}
	parts := make([]string, 0, len(cxn.Slots))
	for _, s := range p.layout(lang, cxn) {
		if s.Role == cxn.Pred {
			verb := m.Relation.Speak()
			if verb == "" {
				return "", false
			}
			// The Turkish past agrees with its subject's person, so a clause about a
			// participant ("ben kediyi kovaladim") conjugates to it.
			person := p.trPersonOf(m.Roles["agent"].Speak())
			parts = append(parts, p.conjugate(lang, verb, m.Roles["agent"].Plural, m.Past, m.Negate, person))
			continue
		}
		filler := m.Roles[s.Role]
		if filler.Empty() {
			return "", false
		}
		parts = append(parts, p.renderFiller(lang, s.Role, filler))
	}
	return sentence(strings.Join(parts, " "), "."), true
}

// conjugate inflects a relation's verb for the tongue: the Turkish morphology
// for Turkish, the English for the rest, and the negative in either. The
// morphology is per-language code (vowel harmony and do-support are not
// strings), but the words it works on come from data.
func (p *Params) conjugate(lang, base string, plural, past, negate bool, person trPerson) string {
	if lang == "tr" {
		return finiteVerbTR(p, base, past, negate, false, person)
	}
	// English marks only the present third-person singular, with -s; first and
	// second person and the plural take the base form ("I want", "you want").
	bare := plural || person == trPerson1sg || person == trPerson2sg || person == trPerson1pl
	if negate {
		// English denies with do-support: "did not chase", "do/does not chase".
		switch {
		case past:
			return "did not " + base
		case bare:
			return "do not " + base
		default:
			return "does not " + base
		}
	}
	return finiteVerb(p, base, bare, plural, past)
}

// renderFiller speaks a role's filler from the tongue's data: a referent in
// play takes the tongue's pronoun for the role, a definite one its definite
// marking, and coordinated co-arguments its coordinator. With no flags it is
// the filler's bare surface, so a caller that pre-rendered its phrase is
// unchanged. This is where the answer surface a tongue carries is applied,
// rather than spelled into the generator.
func (p *Params) renderFiller(lang, role string, t Term) string {
	if t.Given {
		if pr := p.Pronoun(lang, role); pr != "" {
			return pr
		}
	}
	mark := func(np string) string {
		if t.Definite {
			return p.definiteNP(lang, role, np)
		}
		return np
	}
	head := mark(t.Speak())
	if len(t.With) == 0 {
		return head
	}
	parts := []string{head}
	for _, w := range t.With {
		parts = append(parts, mark(w))
	}
	coord := p.Coordinator(lang)
	if coord == "" {
		return strings.Join(parts, " ")
	}
	return strings.Join(parts, " "+coord+" ")
}

// definiteNP gives a noun phrase the tongue's definite marking. A tongue with a
// definite article prepends it; Turkish has none and marks the definite OBJECT
// by the accusative case ("kedi" -> "kediyi"), the subject staying nominative.
// A phrase already carrying a determiner is left alone.
func (p *Params) definiteNP(lang, role, np string) string {
	if np == "" || hasDeterminer(np) {
		return np
	}
	if art := p.Article(lang); art != "" {
		return art + " " + np
	}
	if lang == "tr" && role == "patient" {
		return accusativeCommonTR(p, np)
	}
	return np
}

// Conjugate, DefiniteNP, and Sentence expose the generation morphology for a caller
// rendering a clause's slots through the shared traverser, so a slot is inflected and
// marked by the same engine Speak uses rather than reimplemented. They read the compiled
// parameters and change nothing; Conjugate defaults the Turkish person agreement to third
// singular, the form a clause about a thing takes.
func (p *Params) Conjugate(lang, base string, plural, past, negate bool) string {
	return p.get().conjugate(lang, base, plural, past, negate, trPerson3sg)
}

func (p *Params) DefiniteNP(lang, role, np string) string {
	return p.get().definiteNP(lang, role, np)
}

// VerbFinal reports a language's word order, so a caller laying a clause's slots out
// through the shared traverser puts the verb where the language does (SOV closes with it).
func (p *Params) VerbFinal(lang string) bool {
	return p.get().verbFinal(lang)
}

func Sentence(s string) string {
	return sentence(s, ".")
}

// hasDeterminer reports whether a phrase already opens with a determiner or a
// pronoun, so the definite marker is not stacked onto one.
func hasDeterminer(np string) bool {
	lead, _, _ := strings.Cut(np, " ")
	switch strings.ToLower(lead) {
	case "a", "an", "the", "this", "that", "my", "your", "its",
		"it", "they", "he", "she", "we", "you", "i", "me", "us", "o", "onu":
		return true
	}
	return false
}

// selectArg picks the construction whose argument roles are exactly the
// roles a meaning supplies, the strongest entrenched one among them, ties
// broken by name for determinism. Matching the role set is how a transitive
// meaning reaches the transitive construction and an intransitive one the
// intransitive, with no intent label to dispatch on.
func (p *Params) selectArg(lang string, m Meaning) (ArgConstruction, bool) {
	want := make(map[string]bool, len(m.Roles))
	for r, t := range m.Roles {
		if !t.Empty() {
			want[r] = true
		}
	}
	var best ArgConstruction
	found := false
	for _, c := range p.argcxns[lang] {
		roles := c.argRoles()
		if len(roles) != len(want) {
			continue
		}
		matched := true
		for _, r := range roles {
			if !want[r] {
				matched = false
				break
			}
		}
		if !matched {
			continue
		}
		if !found || c.Weight > best.Weight || (c.Weight == best.Weight && c.Name < best.Name) {
			best, found = c, true
		}
	}
	return best, found
}

// layout sequences a construction's slots by the language's word-order
// parameter: a verb-medial setting speaks them in citation order, a
// verb-final one moves the predicate to the close while the arguments keep
// their relative order, so "agent rel patient" becomes "agent patient rel"
// and the same construction speaks SVO and SOV with no new code.
func (p *Params) layout(lang string, c ArgConstruction) []ArgSlot {
	if !p.verbFinal(lang) {
		return c.Slots
	}
	out := make([]ArgSlot, 0, len(c.Slots))
	var pred ArgSlot
	havePred := false
	for _, s := range c.Slots {
		if s.Role == c.Pred {
			pred, havePred = s, true
			continue
		}
		out = append(out, s)
	}
	if havePred {
		out = append(out, pred)
	}
	return out
}

// bestFitMargin is how decisively feature fit must beat the word order
// before it overrides position: the form pole is the strong prior, so only
// a clear preference (an animate filler stranded in the patient slot)
// reassigns the roles. Equal fits leave position standing.
const bestFitMargin = 0.2

// bestFitAgentFactor is how much better an agent the order's patient must be
// before it unseats the order's agent: half again, not a fixed step. Feature
// kin lifts every animate filler's agent fit on a trained graph, so a fixed
// margin a fresh brain clears is swamped once the numbers are large; a multiple
// holds the bar proportional. "a dog chased a cat" keeps the dog as agent even
// when the cat has edged ahead, while "a rock chased a dog" still yields the
// agent role to the dog the order stranded.
const bestFitAgentFactor = 1.5

// Comprehend runs the argument-structure constructions form to meaning, the
// one engine reversed. The utterance's surface spans fill the
// construction's slots in the language's word order, the same layout
// generation lays out by; each noun-phrase span resolves to the concept it
// names and the predicate span to the relation's lemma, so a told "a dog
// chased a cat" yields the proposition chase(agent: dog, patient: cat).
// When a feature-fit scorer is given, best fit refines the role assignment
// the order proposed, so an animate concept stranded as the patient is read
// as the agent instead. It returns false when no construction's shape fits
// the utterance, so the caller can fall back to the frame parser.
func (p *Params) Comprehend(lang, utterance string, resolve func(string) *memory.Fragment, fit func(*memory.Fragment, string) float64) (Meaning, bool) {
	p = p.get()
	if lang == "" {
		lang = "en"
	}
	spans, ok := p.argSpans(lang, utterance, resolve)
	if !ok {
		return Meaning{}, false
	}
	// A case-driven language (Latin) reads roles from the case endings, not the slot
	// order, so any permutation of the words yields the one proposition.
	if p.caseDriven(lang) {
		return p.comprehendByCase(lang, spans, resolve)
	}
	// Turkish drops a subject the verb's agreement already carries: "kediyi
	// kovaladım" is "(I) chased the cat", the -m marking first person. Recover it
	// when the verb (last in a verb-final clause) agrees with a person and no
	// nominative pronoun was spoken, so the clause reads as the transitive it is
	// rather than mistaking the object for a subject.
	if lang == "tr" && len(spans) > 0 {
		if person := pastVerbPersonTR(spans[len(spans)-1]); person != trPerson3sg && !p.hasSubjectPronounTR(spans) {
			if pronoun := p.personPronounTR(person); pronoun != "" {
				spans = append([]string{pronoun}, spans...)
			}
		}
	}
	cxn, ok := p.selectArgByArity(lang, len(spans))
	if !ok {
		return Meaning{}, false
	}
	order := p.layout(lang, cxn)
	if len(order) != len(spans) {
		return Meaning{}, false
	}
	m := Meaning{Roles: map[string]Term{}, Lang: lang}
	for i, slot := range order {
		span := spans[i]
		if slot.Role == cxn.Pred {
			lemma := p.verbLemma(lang, span)
			m.Relation = Term{Concept: resolve(lemma), Phrase: lemma}
			m.Past = p.verbPast(lang, span)
			continue
		}
		m.Roles[slot.Role] = p.resolveCasedNoun(lang, span, resolve)
	}
	p.bestFit(cxn, m, fit)
	return m, true
}

// resolveCasedNoun grounds a noun span to its concept: the surface first, then its case-stripped
// stems when the surface names nothing, so a case-marked Turkish object ("kediyi") reaches the
// concept its stem names ("kedi"). The stem may need its softened stop hardened back ("köpeği" ->
// "köpeğ" -> "köpek"), tried after the literal stem so a word genuinely ending in the voiced
// consonant resolves first. An unmarked word is left as it is. Both comprehension paths share it.
func (p *Params) resolveCasedNoun(lang, span string, resolve func(string) *memory.Fragment) Term {
	if c := resolve(span); c != nil {
		return Term{Concept: c, Phrase: span}
	}
	for _, base := range p.caseStems(lang, span) {
		if base == span {
			continue
		}
		if c := resolve(base); c != nil {
			return Term{Concept: c, Phrase: base}
		}
	}
	return Term{Phrase: span}
}

// ComprehendByTraversal reads a clause to a meaning by walking the SAME argument construction
// that Speak walks to generate one, in reverse: where Speak selects a construction, lays its
// slots out in word order, and renders each role's filler, this segments the surface in that
// layout, binds each slot's span to the concept it names, and the predicate span to the relation.
// It differs from the coded Comprehend in ONE way that matters as the grammar grows: the
// construction is chosen by how the GROUNDED spans fit its slots' roles (selectArgByFit), the way
// generation chooses by the role set, rather than by the span count alone. So the form-meaning
// pairing is one bidirectional source of truth walked both ways. This is the additive traversal
// counterpart of the coded parser; it recovers a Turkish pro-dropped subject and reads Latin's
// case-bound roles the way the coded path does, so it now walks the construction across all three
// tongues. It returns false when no construction's grounded shape fits.
func (p *Params) ComprehendByTraversal(lang, utterance string, resolve func(string) *memory.Fragment, fit func(*memory.Fragment, string) float64) (Meaning, bool) {
	p = p.get()
	if lang == "" {
		lang = "en"
	}
	spans, ok := p.argSpans(lang, utterance, resolve)
	if !ok {
		return Meaning{}, false
	}
	// A case-driven tongue (Latin) binds roles by the case endings, not the slot order, so the
	// construction is walked by reading each span's case rather than its position. That walk is
	// the same in either direction, so it dispatches to it the way the coded path does.
	if p.caseDriven(lang) {
		return p.comprehendByCase(lang, spans, resolve)
	}
	// Turkish drops a subject its verb's agreement carries ("kediyi kovaladım" is "(I) chased the
	// cat", the -m marking first person). Recover the implied pronoun as the first span, the way
	// the coded path does, so the clause walks as the transitive it is.
	if lang == "tr" && len(spans) > 0 {
		if person := pastVerbPersonTR(spans[len(spans)-1]); person != trPerson3sg && !p.hasSubjectPronounTR(spans) {
			if pronoun := p.personPronounTR(person); pronoun != "" {
				spans = append([]string{pronoun}, spans...)
			}
		}
	}
	cxn, order, ok := p.selectArgByFit(lang, spans, resolve, fit)
	if !ok {
		return Meaning{}, false
	}
	m := Meaning{Roles: map[string]Term{}, Lang: lang}
	for i, slot := range order {
		span := spans[i]
		if slot.Role == cxn.Pred {
			lemma := p.verbLemma(lang, span)
			m.Relation = Term{Concept: resolve(lemma), Phrase: lemma}
			m.Past = p.verbPast(lang, span)
			continue
		}
		m.Roles[slot.Role] = p.resolveCasedNoun(lang, span, resolve)
	}
	p.bestFit(cxn, m, fit)
	return m, true
}

// selectArgByFit picks the construction the grounded spans best fill: among the constructions
// whose laid-out slots match the span count, the entrenched one, ties broken by how well the
// resolved concepts fit its non-predicate slots' roles. So comprehension reasons over the
// grounding to choose its construction, the form-meaning counterpart of how generation's selectArg
// chooses by the role set, rather than by arity alone. It returns the construction and its slots
// in surface-layout order.
func (p *Params) selectArgByFit(lang string, spans []string, resolve func(string) *memory.Fragment, fit func(*memory.Fragment, string) float64) (ArgConstruction, []ArgSlot, bool) {
	var best ArgConstruction
	var bestOrder []ArgSlot
	bestScore := 0.0
	found := false
	for _, cxn := range p.argcxns[lang] {
		order := p.layout(lang, cxn)
		if len(order) != len(spans) {
			continue
		}
		score := 0.0
		if fit != nil {
			for i, slot := range order {
				if slot.Role == cxn.Pred {
					continue
				}
				if c := resolve(spans[i]); c != nil {
					score += fit(c, slot.Role)
				}
			}
		}
		if !found || cxn.Weight > best.Weight || (cxn.Weight == best.Weight && score > bestScore) {
			best, bestOrder, bestScore, found = cxn, order, score, true
		}
	}
	return best, bestOrder, found
}

// comprehendByCase reads a clause whose roles come from case, not position: the
// finite verb is the predicate, the nominative noun the agent, the accusative noun
// the patient, in whatever order they were spoken. So every permutation of "puella
// rosam amat" yields the one proposition girl-loves-rose. It returns false when no
// finite verb is present.
func (p *Params) comprehendByCase(lang string, spans []string, resolve func(string) *memory.Fragment) (Meaning, bool) {
	m := Meaning{Roles: map[string]Term{}, Lang: lang}
	var nouns []string
	person := ""
	for _, span := range spans {
		if m.Relation.Phrase == "" && p.finiteVerbByCase(lang, span) {
			lemma := p.verbLemma(lang, span)
			m.Relation = Term{Concept: resolve(lemma), Phrase: lemma}
			m.Past = p.verbPast(lang, span)
			_, person, _ = latinVerb(span)
			continue
		}
		nouns = append(nouns, span)
	}
	if m.Relation.Phrase == "" {
		return Meaning{}, false
	}
	for _, span := range nouns {
		role, lemma := p.caseRole(lang, span)
		filler := Term{Concept: resolve(lemma), Phrase: lemma}
		if filler.Concept == nil && lemma != span {
			if c := resolve(span); c != nil {
				filler = Term{Concept: c, Phrase: span}
			}
		}
		m.Roles[role] = filler
	}
	// Pro-drop: Latin leaves the subject to the verb's person. With no nominative
	// agent named and a first or second person verb, the agent is the speaker or the
	// addressee, recovered as the pronoun the resolver maps to the participant.
	if m.Roles["agent"].Concept == nil {
		if pron := p.personPronounLatin(person); pron != "" {
			m.Roles["agent"] = Term{Concept: resolve(pron), Phrase: pron}
		}
	}
	return m, true
}

// personPronounLatin is the nominative pronoun a pro-dropped Latin subject recovers
// to, read from the seeded surfaces; the third person carries no participant, so it is
// left for an explicit subject.
func (p *Params) personPronounLatin(person string) string {
	switch person {
	case "1sg":
		return p.surfaceWord("la", "firstperson")
	case "2sg":
		return p.surfaceWord("la", "addressee")
	case "1pl":
		return p.surfaceWord("la", "firstpersonpl")
	}
	return ""
}

// caseRole reads the role a case-marked noun fills and the lemma it names: the
// accusative ending marks the patient, the bare nominative the agent. The endings
// are the seeded case data, so the rule is a fact about the language, not the parser.
func (p *Params) caseRole(lang, span string) (role, lemma string) {
	// Latin marks the patient with the accusative of the first two declensions (-am, -um); the
	// bare nominative is the agent. The stem reduces to the nominative the noun is named by.
	if lang == "la" {
		if strings.HasSuffix(span, "am") && len([]rune(span)) > 3 {
			return "patient", strings.TrimSuffix(span, "am") + "a"
		}
		if strings.HasSuffix(span, "um") && len([]rune(span)) > 3 {
			return "patient", strings.TrimSuffix(span, "um") + "us"
		}
		return "agent", span
	}
	if acc := p.caseEnding(lang, "accusative"); acc != "" && strings.HasSuffix(span, acc) {
		return "patient", strings.TrimSuffix(span, acc)
	}
	return "agent", span
}

// latinPresent lists the present-active endings of the first conjugation (the a-stem)
// and the second (the e-stem), longest first so a plural ending is tried before the
// short singular it contains. Each carries the person it marks and the third-singular
// ending the lemma normalizes to, so every person of a verb keys the one relation.
var latinPresent = []struct{ ending, person, lemma string }{
	{"emus", "1pl", "et"}, {"etis", "2pl", "et"}, {"ent", "3pl", "et"},
	{"eo", "1sg", "et"}, {"es", "2sg", "et"}, {"et", "3sg", "et"},
	{"amus", "1pl", "at"}, {"atis", "2pl", "at"}, {"ant", "3pl", "at"},
	{"as", "2sg", "at"}, {"at", "3sg", "at"}, {"o", "1sg", "at"},
}

// latinVerb reads a Latin present-active verb: the lemma normalized to the
// third-singular form (so every person keys the one relation) and the person it
// carries. ok is false when the span is no such verb. The present of the first two
// conjugations; further conjugations and tenses refine it.
func latinVerb(span string) (lemma, person string, ok bool) {
	for _, e := range latinPresent {
		if strings.HasSuffix(span, e.ending) && len(span) > len(e.ending)+1 {
			return span[:len(span)-len(e.ending)] + e.lemma, e.person, true
		}
	}
	return "", "", false
}

// latinNominativeCandidates lists the nominative forms a cased Latin noun of the first two
// declensions might reduce to: the accusative -am and -um and the ablative -o shed back to the
// -a and -us a noun is named by ("lupum"/"lupo" -> "lupus", "rosam" -> "rosa"). The third
// declension's nominative is not predictable from the stem (leo/leonem), so it is left for an
// explicit naming. The resolver tries each in turn after the literal form.
func latinNominativeCandidates(span string) []string {
	switch {
	case strings.HasSuffix(span, "am") && len([]rune(span)) > 3:
		return []string{strings.TrimSuffix(span, "am") + "a"}
	case strings.HasSuffix(span, "um") && len([]rune(span)) > 3:
		return []string{strings.TrimSuffix(span, "um") + "us"}
	case strings.HasSuffix(span, "o") && len([]rune(span)) > 2:
		return []string{strings.TrimSuffix(span, "o") + "us"}
	}
	return nil
}

// latinAccusative forms the accusative a first- or second-declension Latin patient wears from
// its nominative (-a -> -am, -us -> -um). A noun of another declension keeps its nominative,
// the honest limit until its stem is known.
func latinAccusative(nom string) string {
	switch {
	case strings.HasSuffix(nom, "us"):
		return strings.TrimSuffix(nom, "us") + "um"
	case strings.HasSuffix(nom, "a"):
		return nom + "m"
	}
	return nom
}

// finiteVerbByCase reports whether a span is the finite verb of a case-marked clause,
// told apart from the case-marked nouns by its conjugation ending (Latin's nouns wear
// their case vowels, its verbs the person endings).
func (p *Params) finiteVerbByCase(lang, span string) bool {
	_, _, ok := latinVerb(span)
	return ok
}

// bestFit reorders the role assignment by feature fit when one is clearly
// better placed elsewhere, the constraint satisfaction the feature
// signatures supply. For the two-argument case it weighs the agent and
// patient as bound against the swap, and swaps only when the swap wins by a
// margin, so the word order stands unless the features overrule it
// decisively (an animate concept is the better agent). Roles are mutated in
// place on the meaning's map.
func (p *Params) bestFit(cxn ArgConstruction, m Meaning, fit func(*memory.Fragment, string) float64) {
	if fit == nil {
		return
	}
	args := cxn.argRoles()
	if len(args) != 2 {
		return
	}
	a, b := args[0], args[1]
	ta, tb := m.Roles[a], m.Roles[b]
	if ta.Concept == nil || tb.Concept == nil {
		return
	}
	asIs := fit(ta.Concept, a) + fit(tb.Concept, b)
	swapped := fit(tb.Concept, a) + fit(ta.Concept, b)
	// The word order is overruled only when the filler it seated as agent is no
	// better suited to that role than to patient, an inanimate or stranded
	// concept. A perfectly good agent the order named first prefers the agent
	// role, so it is not unseated by another that has merely been an agent more
	// often: "a dog chased a rabbit" stays the dog chasing, not the reverse.
	orderAgentMisplaced := fit(ta.Concept, a) <= fit(ta.Concept, b)
	// A filler the features say nothing about, a thing just introduced by the
	// telling, is unknown rather than misplaced: with no kinship either way
	// there is no evidence against the teller's order, and any grounded
	// co-argument would clear the relative bar over a zero baseline, so the
	// order stands until the new thing has features to weigh.
	if fit(ta.Concept, a) == 0 && fit(ta.Concept, b) == 0 {
		orderAgentMisplaced = false
	}
	// And the swap must promote a clearly better agent. A heavily grounded
	// concept fits both roles highly through its feature kin, so on a trained
	// graph the order's agent can read as fitting the patient role by a hair
	// while still being a strong agent, and a co-argument can edge it for the
	// agent role without being the right subject. Requiring the order's patient
	// to out-fit it by half again keeps the order standing in that near tie: the
	// dog stays the chaser because the order named it, not undone by the cat
	// nosing ahead as an agent.
	candidateBetterAgent := fit(tb.Concept, a) > fit(ta.Concept, a)*bestFitAgentFactor
	if orderAgentMisplaced && candidateBetterAgent && swapped > asIs+bestFitMargin {
		m.Roles[a], m.Roles[b] = tb, ta
	}
}

// argSpans segments an utterance into the ordered surface spans an
// argument-structure construction's slots bind to, in the language's word
// order so generation and comprehension read the layout the same way. The
// determiners mark the noun-phrase boundaries: two open a transitive
// clause, the verb taking the slot the order leaves it (between the phrases
// for SVO, after them for SOV); one opens an intransitive clause with the
// verb at the close; none falls back to the open-class tokens in order, the
// canonical bare clause. A single determiner can also open a transitive
// whose object stands bare (a mass or plural noun: "a zebra ate grass"),
// which the count alone reads as intransitive; the graph settles it: when a
// token past the subject resolves to a learned relation, it is the
// predicate and the trailing content the object.
func (p *Params) argSpans(lang, utterance string, resolve func(string) *memory.Fragment) ([]string, bool) {
	tokens, _ := Tokenize(utterance)
	if len(tokens) == 0 {
		return nil, false
	}
	// A determiner anchors a noun phrase's start; in a verb-medial tongue a bare
	// deictic pronoun anchors the same way without one, so "you chased the cat"
	// yields the subject a determiner count alone would miss. A verb-final tongue
	// marks definiteness by case rather than articles, so its arguments come through
	// the content-token path, where a pronoun stays a content token rather than an
	// anchor that would miscount the clause.
	verbFinal := p.verbFinal(lang)
	var anchors []int
	for i, tok := range tokens {
		switch p.category(lang, tok) {
		case catDet:
			anchors = append(anchors, i)
		case catDeixis:
			if !verbFinal {
				anchors = append(anchors, i)
			}
		}
	}
	switch {
	case len(anchors) >= 2:
		d2 := anchors[1]
		if verbFinal {
			np1 := p.contentSpan(tokens[:d2])
			np2 := p.contentSpan(tokens[d2 : len(tokens)-1])
			verb := tokens[len(tokens)-1]
			return []string{np1, np2, verb}, np1 != "" && np2 != "" && verb != ""
		}
		// A verb-medial clause splits into the subject before the verb, the verb, and one noun
		// phrase per determiner after it, so a clause with more arguments than the transitive yields
		// a span apiece: "the dog gave the cat the bone" reads as agent, verb, recipient, patient.
		// The verb sits just before the second determiner.
		verb := tokens[d2-1]
		spans := []string{p.contentSpan(tokens[:d2-1]), verb}
		ok := spans[0] != "" && verb != ""
		for i := 1; i < len(anchors); i++ {
			end := len(tokens)
			if i+1 < len(anchors) {
				end = anchors[i+1]
			}
			np := p.contentSpan(tokens[anchors[i]:end])
			spans = append(spans, np)
			ok = ok && np != ""
		}
		return spans, ok
	case len(anchors) == 1:
		// Verb-medial with content trailing a token the graph knows as a relation:
		// the clause is a transitive with a bare object, split at that predicate.
		// The close-verb intransitive ("a dog sleeps") is untouched: nothing
		// trails its verb, so the scan finds no object span and falls through.
		if !verbFinal && resolve != nil {
			for i := anchors[0] + 1; i < len(tokens); i++ {
				lemma := p.verbLemma(lang, tokens[i])
				c := resolve(lemma)
				if c == nil || c.Category != "relation" {
					continue
				}
				subj := p.contentSpan(tokens[:i])
				obj := p.contentSpan(tokens[i+1:])
				if subj != "" && obj != "" {
					return []string{subj, tokens[i], obj}, true
				}
				break
			}
		}
		verb := tokens[len(tokens)-1]
		np1 := p.contentSpan(tokens[:len(tokens)-1])
		return []string{np1, verb}, np1 != "" && verb != ""
	default:
		content := p.argContentTokens(lang, utterance)
		return content, len(content) >= 2
	}
}

// argContentTokens returns an utterance's open-class words plus its deictic
// pronouns, the arguments a clause splits into when no determiner anchors them. A
// pronoun is kept where the closed-class filter would drop it, so a verb-final
// clause with a pronoun argument ("sen kediyi kovaladin") still yields every span.
func (p *Params) argContentTokens(lang, s string) []string {
	p = p.get()
	tokens, _ := Tokenize(s)
	seen := map[string]bool{}
	var out []string
	for _, tok := range tokens {
		if len(tok) < 2 || seen[tok] {
			continue
		}
		if p.isFunctionWordLang(lang, tok) && p.category(lang, tok) != catDeixis {
			continue
		}
		seen[tok] = true
		out = append(out, tok)
	}
	return out
}

// selectArgByArity picks the construction with the given number of slots,
// the strongest entrenched one, ties broken by name. Arity is how a
// three-span clause reaches the transitive construction and a two-span one
// the intransitive, with no intent label to dispatch on.
func (p *Params) selectArgByArity(lang string, n int) (ArgConstruction, bool) {
	var best ArgConstruction
	found := false
	for _, c := range p.argcxns[lang] {
		if len(c.Slots) != n {
			continue
		}
		if !found || c.Weight > best.Weight || (c.Weight == best.Weight && c.Name < best.Name) {
			best, found = c, true
		}
	}
	return best, found
}

// verbPast reads an inflected verb's tense: a seeded event verb carries its
// own, a form matching a seeded verb's principal parts takes the part's
// tense, and an unknown form reads as past on the regular -ed ending, so a
// novel "blorped" is settled while "blorps" is not.
func (p *Params) verbPast(lang, surface string) bool {
	if h, ok := p.hap(lang, surface); ok {
		return h.past
	}
	if lang == "tr" {
		// A Turkish verb is past when it carries the -DI suffix the stemmer
		// strips, so "kovaladı" reads past while the aorist "kovalar" does not.
		return turkishVerbStem(surface) != surface
	}
	if lang == "en" || lang == "" {
		for _, v := range p.verbs {
			switch surface {
			case v.past, v.part:
				return true
			case v.base, v.third:
				return false
			}
		}
	}
	return strings.HasSuffix(surface, "ed")
}

// verbLemma recovers a verb's base form from an inflected surface form, the
// reverse of the regular conjugation, so a relation heard as "chased",
// "chases", or "chasing" keys on the one lemma "chase". A seeded event verb
// yields its known base; otherwise the regular endings are stripped and a
// silent e restored where the spelling rules dropped one. It is a best
// effort over the regular paradigm: an irregular form keeps its surface,
// and a grounded relation is matched by the resolver before a lemma is
// minted from it.
func (p *Params) verbLemma(lang, surface string) string {
	if h, ok := p.hap(lang, surface); ok {
		return h.base
	}
	if lang == "tr" {
		return turkishVerbStem(surface)
	}
	if lang == "la" {
		if lemma, _, ok := latinVerb(surface); ok {
			return lemma
		}
	}
	for _, v := range p.verbs {
		switch surface {
		case v.base, v.third, v.past, v.part:
			return v.base
		}
	}
	s := surface
	switch {
	case strings.HasSuffix(s, "ies") && len(s) > 4:
		return s[:len(s)-3] + "y"
	case strings.HasSuffix(s, "ied") && len(s) > 4:
		return s[:len(s)-3] + "y"
	case strings.HasSuffix(s, "ing") && len(s) > 5:
		return restoreSilentE(s[:len(s)-3])
	case strings.HasSuffix(s, "ed") && len(s) > 3:
		return restoreSilentE(s[:len(s)-2])
	case strings.HasSuffix(s, "es") && len(s) > 3:
		return restoreSilentE(s[:len(s)-2])
	case strings.HasSuffix(s, "s") && len(s) > 2 && !strings.HasSuffix(s, "ss"):
		return s[:len(s)-1]
	}
	return s
}

// SetArgConstructions installs the argument-structure constructions
// compiled from acxn parameters, keyed by language, and returns the same
// Params for chaining onto Compile. Each key is "acxn:<lang>:<name>"; the
// value lists the form slots in citation order as "role/syn" (with an
// optional "/prefer" feature), and the category names the predicate role.
func (p *Params) SetArgConstructions(entries []Param) *Params {
	p.argcxns = ParseArgConstructions(entries)
	return p
}

// ParseArgConstructions reads the argument-structure constructions out of the construction
// parameters into their form-meaning structures, keyed by language. Each key is
// "acxn:<lang>:<name>"; the value lists the form slots in citation order as "role/syn" (with an
// optional "/prefer" feature), and the category names the predicate role. It is exported so the
// same parse feeds the engine and a caller representing the constructions in the graph.
func ParseArgConstructions(entries []Param) map[string][]ArgConstruction {
	reg := map[string][]ArgConstruction{}
	for _, e := range entries {
		rest, ok := strings.CutPrefix(e.Key, "acxn:")
		if !ok {
			continue
		}
		lang, name, ok := strings.Cut(rest, ":")
		if !ok || name == "" {
			continue
		}
		c := ArgConstruction{Name: name, Lang: lang, Weight: e.Weight, Pred: e.Category}
		for _, field := range strings.Fields(e.Value) {
			parts := strings.Split(field, "/")
			slot := ArgSlot{Role: parts[0]}
			if len(parts) > 1 {
				slot.Syn = parts[1]
			}
			if len(parts) > 2 {
				slot.Prefer = parts[2]
			}
			c.Slots = append(c.Slots, slot)
		}
		reg[lang] = append(reg[lang], c)
	}
	return reg
}

// ArgConstructions returns the argument-structure constructions for a language, read-only, for a
// caller cross-checking the constructions held in the graph against the compiled ones.
func (p *Params) ArgConstructions(lang string) []ArgConstruction {
	return p.get().argcxns[lang]
}
