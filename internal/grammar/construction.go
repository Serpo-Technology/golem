package grammar

import (
	"strings"

	"golem/internal/limbic"
)

// A construction is a form-and-meaning pattern lifted out of code and into
// the weighted graph: an intent on the meaning side, an ordered surface
// template with typed slots on the form side. The slot-filling engine, the
// morphology, and the word order stay innate here; only the pattern lives
// in memory, the same move Step 14 made for grammar parameters, raised from
// words to whole sentences. A meaning may have several constructions, and
// which one speaks reads from context and chemistry, the way oxytocin picks
// "usually follows" over "may follow".
type Construction struct {
	Intent  string     // the meaning side: which answer or remark this realizes
	Variant string     // distinguishes the alternative forms of one intent
	Lang    string     // the language it speaks
	Cond    string     // a chemistry condition that gates eligibility, empty for always
	Weight  float64    // entrenchment; the strongest eligible variant speaks
	Tokens  []CxnToken // the form side: literal runs and typed slots in order
}

// CxnToken is one piece of a template: a literal text run, or a named slot
// the engine fills from a binding.
type CxnToken struct {
	Lit  string
	Slot string
}

// ParseTemplate splits a surface template into literal runs and {slot}
// markers, so "I was thinking: {thought}" becomes the literal "I was
// thinking: " followed by the slot "thought". An unmatched brace stays
// literal rather than swallowing the rest of the line.
func ParseTemplate(s string) []CxnToken {
	var toks []CxnToken
	for len(s) > 0 {
		open := strings.IndexByte(s, '{')
		if open < 0 {
			toks = append(toks, CxnToken{Lit: s})
			break
		}
		if open > 0 {
			toks = append(toks, CxnToken{Lit: s[:open]})
		}
		rest := s[open:]
		close := strings.IndexByte(rest, '}')
		if close < 0 {
			toks = append(toks, CxnToken{Lit: rest})
			break
		}
		toks = append(toks, CxnToken{Slot: rest[1:close]})
		s = rest[close+1:]
	}
	return toks
}

// render fills the template's slots from the binding without raising the
// first letter: missing slots collapse to nothing and runs of space close
// up, but a nested clause stays lowercase so it can sit mid-sentence. The
// template carries its own terminal punctuation.
func (c Construction) render(slots map[string]string) string {
	var b strings.Builder
	for _, t := range c.Tokens {
		if t.Slot != "" {
			b.WriteString(slots[t.Slot])
			continue
		}
		b.WriteString(t.Lit)
	}
	return strings.Join(strings.Fields(b.String()), " ")
}

// Render fills the template and finishes the clause as a standalone
// sentence: the first letter rises, the template's own terminal
// punctuation stands, so "I was thinking: ..." stays unpunctuated where a
// full statement ends in a period. A construction nested by Compose uses
// render instead, so only the outermost clause is capitalized.
func (c Construction) Render(slots map[string]string) string {
	return sentence(c.render(slots), "")
}

// conditionMet reads a construction's chemistry gate. The vocabulary is
// small and innate: trust splits a confident assertion from a tentative
// hedge, and alarm splits a calm register from an aroused one. An unknown
// condition is treated as always eligible, so a typo in a taught
// construction never mutes it outright.
func conditionMet(cond string, chem limbic.State) bool {
	switch cond {
	case "":
		return true
	case "confident":
		return chem.Oxytocin >= limbic.OxytocinBaseline
	case "tentative":
		return chem.Oxytocin < limbic.OxytocinBaseline
	case "calm":
		return chem.Noradrenaline < 0.5
	case "aroused":
		return chem.Noradrenaline >= 0.5
	default:
		return true
	}
}

// selectVariant picks the construction that speaks for an intent: among
// the variants whose chemistry condition is met, the strongest entrenched
// one, ties broken by variant name for determinism. It is the one choice
// point, made the same way at every layer of a composition.
func selectVariant(variants []Construction, chem limbic.State) *Construction {
	var best *Construction
	for i := range variants {
		v := &variants[i]
		if !conditionMet(v.Cond, chem) {
			continue
		}
		if best == nil || v.Weight > best.Weight ||
			(v.Weight == best.Weight && v.Variant < best.Variant) {
			best = v
		}
	}
	return best
}

// Construct realizes one intent through the memory constructions, filling
// its slots with text. It returns false when no construction is seeded for
// the language, so the caller falls back to the innate realizer and a
// tongue with no construction still speaks.
func (p *Params) Construct(lang, intent string, slots map[string]string, chem limbic.State) (string, bool) {
	p = p.get()
	best := selectVariant(p.constructions[lang][intent], chem)
	if best == nil {
		return "", false
	}
	return best.Render(slots), true
}

// A Message is a layered binding the realizer renders by recursive
// slot-filling: an intent, and for each of its slots a filler that is
// either literal text or another message nested in the slot. Composing
// messages is how GOLEM says something it was never given whole, a clause
// built from constructions that were each seeded on their own.
type Message struct {
	Intent string
	Slots  map[string]Filler
}

// Filler is one slot's value: a literal string, or a nested message the
// engine renders first and substitutes.
type Filler struct {
	Text string
	Sub  *Message
}

// TextSlots wraps a plain slot map as fillers, the common leaf case where
// every slot is literal text.
func TextSlots(m map[string]string) map[string]Filler {
	out := make(map[string]Filler, len(m))
	for k, v := range m {
		out[k] = Filler{Text: v}
	}
	return out
}

// compose is the recursive core: it returns the raw, uncapitalized
// rendering so a nested clause stays lowercase mid-sentence, with the
// trace of constructions that built it, leaves first.
func (p *Params) compose(lang string, m Message, chem limbic.State) (string, []string, bool) {
	slots := make(map[string]string, len(m.Slots))
	var trace []string
	// Resolve nested slots in name order, so the trace is deterministic
	// even though the binding is a map.
	for _, name := range sortedKeys(m.Slots) {
		f := m.Slots[name]
		if f.Sub == nil {
			slots[name] = f.Text
			continue
		}
		text, sub, ok := p.compose(lang, *f.Sub, chem)
		if !ok {
			return "", nil, false
		}
		slots[name] = text
		trace = append(trace, sub...)
	}
	best := selectVariant(p.constructions[lang][m.Intent], chem)
	if best == nil {
		return "", nil, false
	}
	return best.render(slots), append(trace, m.Intent+":"+best.Variant), true
}

// Compose renders a layered message: every nested filler is composed first
// and its text substituted, then the message's own intent selects a
// construction and fills its slots, so a novel combination still comes out
// grammatical. Only the outermost clause is capitalized, so a nested one
// reads naturally mid-sentence. It returns the trace of constructions that
// built the sentence, so the composition reads back as what made it. A
// missing construction at any layer returns false, so the whole falls back
// to the innate realizer rather than speaking a half-built clause.
func (p *Params) Compose(lang string, m Message, chem limbic.State) (string, []string, bool) {
	line, trace, ok := p.get().compose(lang, m, chem)
	if !ok {
		return "", nil, false
	}
	return sentence(line, ""), trace, true
}

// Constructions is the innate construction seed, the first sentence-pattern
// curriculum the trainer writes into starter memory beside the grammar
// parameters. Once seeded, the engine reads the memory copy, and a taught
// construction competes with these by weight.
//
// The succession and definition answers and the introspection report move
// out of the realizer to here; the verb group inside a succession slot is
// still conjugated by the engine, so morphology stays innate while the
// frame around it is learned. The assert and hedge variants carry the same
// oxytocin split the realizer used to make in code.
func Constructions() []Param {
	seed := func(intent, variant, cond, template string) Param {
		return Param{
			Key:      "cxn:en:" + intent + ":" + variant,
			Value:    template,
			Lang:     "en",
			Category: cond,
			Weight:   1.0,
		}
	}
	return []Param{
		seed("introspect", "report", "", "I was thinking: {thought}"),
		seed("introspect_idle", "plain", "", "I have not been thinking about anything."),
		seed("definition", "plain", "", "{subject} is {kind}."),
		seed("seen_count", "plain", "", "I have seen {object} {count}."),
		// A composition: the definition clause and, when there is one, the
		// count clause nest in its slots. With the count absent the slot
		// collapses, so the same pattern says both "X is a thing." and "X
		// is a thing. I have seen it twice."
		seed("described", "full", "", "{defn} {count}"),
		seed("succession_after", "assert", "confident", "{adjunct}, {subject} {vp_assert}."),
		seed("succession_after", "hedge", "tentative", "{adjunct}, {subject} {vp_hedge}."),
		seed("succession_before", "assert", "confident", "{subject} {vp_assert} {anchor}."),
		seed("succession_before", "hedge", "tentative", "{subject} {vp_hedge} {anchor}."),
	}
}

// SetConstructions installs the construction registry compiled from cxn
// parameters, keyed language then intent, and returns the same Params for
// chaining onto Compile. Each key is "cxn:<lang>:<intent>:<variant>"; the
// value is the template, the category the chemistry condition, the weight
// the entrenchment.
func (p *Params) SetConstructions(entries []Param) *Params {
	reg := map[string]map[string][]Construction{}
	for _, e := range entries {
		rest, ok := strings.CutPrefix(e.Key, "cxn:")
		if !ok {
			continue
		}
		parts := strings.SplitN(rest, ":", 3)
		if len(parts) != 3 {
			continue
		}
		lang, intent, variant := parts[0], parts[1], parts[2]
		if reg[lang] == nil {
			reg[lang] = map[string][]Construction{}
		}
		reg[lang][intent] = append(reg[lang][intent], Construction{
			Intent:  intent,
			Variant: variant,
			Lang:    lang,
			Cond:    e.Category,
			Weight:  e.Weight,
			Tokens:  ParseTemplate(e.Value),
		})
	}
	p.constructions = reg
	return p
}
