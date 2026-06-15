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
			parts = append(parts, finiteVerb(p, verb, false, m.Roles["agent"].Plural, m.Past))
			continue
		}
		filler := m.Roles[s.Role]
		if filler.Empty() {
			return "", false
		}
		parts = append(parts, filler.Speak())
	}
	return sentence(strings.Join(parts, " "), "."), true
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
	spans, ok := p.argSpans(lang, utterance)
	if !ok {
		return Meaning{}, false
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
		// A noun phrase resolves on its surface, and on its case-stripped form
		// when the surface names nothing, so a case-marked Turkish object
		// ("kediyi") reaches the concept its stem names ("kedi"). An unmarked
		// word is left as it is.
		filler := Term{Concept: resolve(span), Phrase: span}
		if filler.Concept == nil {
			if base := p.stripCase(lang, span); base != span {
				if c := resolve(base); c != nil {
					filler = Term{Concept: c, Phrase: base}
				}
			}
		}
		m.Roles[slot.Role] = filler
	}
	p.bestFit(cxn, m, fit)
	return m, true
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
// canonical bare clause.
func (p *Params) argSpans(lang, utterance string) ([]string, bool) {
	tokens, _ := Tokenize(utterance)
	if len(tokens) == 0 {
		return nil, false
	}
	var dets []int
	for i, tok := range tokens {
		if p.category(lang, tok) == catDet {
			dets = append(dets, i)
		}
	}
	verbFinal := p.verbFinal(lang)
	switch {
	case len(dets) >= 2:
		d2 := dets[1]
		if verbFinal {
			np1 := p.contentSpan(tokens[:d2])
			np2 := p.contentSpan(tokens[d2 : len(tokens)-1])
			verb := tokens[len(tokens)-1]
			return []string{np1, np2, verb}, np1 != "" && np2 != "" && verb != ""
		}
		verb := tokens[d2-1]
		np1 := p.contentSpan(tokens[:d2-1])
		np2 := p.contentSpan(tokens[d2:])
		return []string{np1, verb, np2}, np1 != "" && np2 != "" && verb != ""
	case len(dets) == 1:
		verb := tokens[len(tokens)-1]
		np1 := p.contentSpan(tokens[:len(tokens)-1])
		return []string{np1, verb}, np1 != "" && verb != ""
	default:
		content := p.ContentTokens(utterance)
		return content, len(content) >= 2
	}
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
	p.argcxns = reg
	return p
}
