package grammar

import "strings"

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
