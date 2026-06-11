package grammar

import (
	"fmt"
	"strings"

	"golem/internal/limbic"
	"golem/internal/memory"
)

// Categories the innate grammar distinguishes. Content words take their
// category from their concept's grounding role instead (verb for shapes
// in command position, noun for recurring sensory objects).
const (
	catWh    = "wh"    // interrogatives: open a question
	catRel   = "rel"   // temporal relators: attach an anchor clause
	catAdv   = "adv"   // adverbs of habit
	catAgain = "again" // the repetition pro-verb modifier
	catPro   = "pro"   // pro-forms: resolve from the workspace
	catImp   = "imp"   // imperative openers: command force
	catDet   = "det"   // determiners: skipped during resolution
	catCop   = "cop"   // copulas: definition questions
	catHap   = "hap"   // happen-verbs: event questions
	catNeg   = "neg"   // negators: deny the predicate
	catCnt   = "cnt"   // count quantifiers: "how many" asks for a number
	catFn    = "fn"    // grammatical filler with no slot of its own
)

// sortalNouns restrict what kind of thing a question wants back, the
// "command" of "which command follows X". They are meta-vocabulary
// about GOLEM's own ontology. Plural forms invite more than one answer.
// This table is curriculum: the engine reads the memory copy.
var sortalNouns = map[string]string{
	"command": "command", "commands": "command",
	"action": "action", "actions": "action",
	"thing": "thing", "things": "thing",
	"event": "event", "events": "event",
	"step": "step", "steps": "step",
}

// innateCategory is the English closed class, the curriculum source for
// the fw:en grammar nodes. Event-verb forms (catHap) seed separately as
// hap:en nodes carrying their predicate and tense.
var innateCategory = map[string]string{
	"what": catWh, "when": catWh, "which": catWh, "where": catWh, "who": catWh, "why": catWh, "how": catWh,
	"after": catRel, "before": catRel,
	"usually": catAdv, "always": catAdv, "often": catAdv, "sometimes": catAdv,
	"again": catAgain,
	"it":    catPro, "that": catPro, "this": catPro, "them": catPro, "one": catPro,
	"do": catImp, "run": catImp, "execute": catImp, "show": catImp, "retry": catImp, "try": catImp,
	"the": catDet, "a": catDet, "an": catDet, "my": catDet, "your": catDet,
	"is": catCop, "are": catCop, "was": catCop, "were": catCop,
	"happens": catHap, "happened": catHap, "follows": catHap, "followed": catHap, "comes": catHap, "came": catHap,
	"follow": catHap, "come": catHap, "happen": catHap,
	"precedes": catHap, "preceded": catHap, "precede": catHap,
	"not": catNeg, "never": catNeg, "no": catNeg, "cannot": catNeg,
	"many": catCnt, "much": catCnt,
	"i": catFn, "you": catFn, "we": catFn, "me": catFn, "us": catFn,
	"to": catFn, "of": catFn, "in": catFn, "on": catFn, "at": catFn, "and": catFn, "or": catFn,
	"does": catFn, "did": catFn, "next": catFn, "then": catFn, "now": catFn, "yet": catFn,
	"please": catFn, "know": catFn, "about": catFn, "with": catFn, "for": catFn,
	"times": catFn, "time": catFn,
}

// happenVerbs maps each event-verb surface form to the predicate it
// fills and the tense it carries, so "followed" and "follows" land in
// one concept with the time told apart. Succession verbs collapse onto
// "follow"; precedence verbs onto its inverse. Curriculum for the
// hap:en nodes.
var happenVerbs = map[string]hapEntry{
	"happens": {"follow", false}, "happen": {"follow", false}, "happened": {"follow", true},
	"follows": {"follow", false}, "follow": {"follow", false}, "followed": {"follow", true},
	"comes": {"follow", false}, "come": {"follow", false}, "came": {"follow", true},
	"precedes": {"precede", false}, "precede": {"precede", false}, "preceded": {"precede", true},
}

// Parse runs the innate grammar backward with the built-in parameter
// seed, for callers without a memory-backed grammar.
func Parse(utterance string, resolve func(string) *memory.Fragment) Frame {
	return Builtin().Parse(utterance, resolve)
}

// Parse runs the innate grammar backward: an utterance becomes a frame.
// The resolver is one learned half, mapping a surface phrase to its
// concept; the closed class, the event verbs, and the sortals are the
// other, read from the compiled parameters. The procedure alone is
// wired in: force and interrogative from the opening word, temporal
// qualifiers each owning their anchor, and content spans into thematic
// roles by their position against the verb.
func (p *Params) Parse(utterance string, resolve func(string) *memory.Fragment) Frame {
	p = p.get()
	tokens, question := Tokenize(utterance)
	f := Frame{Force: ForceStatement}
	if len(tokens) == 0 {
		return f
	}
	// The closed classes share no forms, so one hit decides the grammar
	// the utterance is read with. Word order is a parameter too: only a
	// verb-final setting engages the SOV parse.
	if p.detectTurkish(tokens) && p.verbFinal("tr") {
		return p.parseTurkish(tokens, question, resolve)
	}
	switch {
	case p.category("en", tokens[0]) == catWh:
		f.Force = ForceQuestion
		f.Wh = tokens[0]
	case p.category("en", tokens[0]) == catImp:
		f.Force = ForceCommand
	case question:
		f.Force = ForceQuestion
	}

	// Each relator opens a qualifier whose anchor runs to the next
	// relator, so "after X and before Y" parses as two qualifiers
	// instead of folding the second relator into the first anchor.
	main := tokens
	for i, tok := range tokens {
		if p.category("en", tok) != catRel {
			continue
		}
		main = tokens[:i]
		for i < len(tokens) {
			rel := tokens[i]
			j := i + 1
			for j < len(tokens) && p.category("en", tokens[j]) != catRel {
				j++
			}
			anchor := p.contentSpan(tokens[i+1 : j])
			q := Qualifier{Rel: rel, Anchor: resolveTerm(anchor, resolve)}
			if anchor != "" {
				f.Quals = append(f.Quals, q)
			}
			i = j
		}
		break
	}

	type span struct {
		text     string
		postVerb bool
	}
	var spans []span
	var current []string
	verbSeen := false
	flush := func() {
		if len(current) > 0 {
			spans = append(spans, span{strings.Join(current, " "), verbSeen})
			current = nil
		}
	}
	for _, tok := range main {
		// The do-auxiliary carries tense for the clause; "was" and
		// "were" carry it for the copula. Both leave the predicate to
		// the verb that follows.
		if tok == "did" {
			f.Past = true
			flush()
			continue
		}
		if base, ok := p.sortal("en", tok); ok {
			f.Sort = base
			f.SortPl = tok != base
			flush()
			continue
		}
		switch p.category("en", tok) {
		case catHap:
			// The span before the verb closes as the subject before the
			// flag flips; what follows is the object.
			flush()
			v, _ := p.hap("en", tok)
			f.Action = Term{Phrase: v.base}
			if v.past {
				f.Past = true
			}
			verbSeen = true
		case catCop:
			flush()
			f.Action = Term{Phrase: "be"}
			if tok == "was" || tok == "were" {
				f.Past = true
			}
			verbSeen = true
		case catCnt:
			f.Counting = true
			flush()
		case catPro:
			f.Pro = true
			flush()
		case catAgain:
			f.Again = true
			flush()
		case catAdv:
			f.Habit = true
			flush()
		case catNeg:
			f.Negate = true
			flush()
		case catWh, catRel, catImp, catDet, catFn:
			flush()
		default:
			current = append(current, tok)
		}
	}
	flush()
	// "How often" counts the way "how many" does.
	if f.Wh == "how" && f.Habit {
		f.Counting = true
	}

	// Position against the verb assigns the roles: a pre-verb span is
	// the subject, a post-verb span the object. A command's spans are
	// what the command acts on.
	for _, sp := range spans {
		term := resolveTerm(sp.text, resolve)
		switch {
		case f.Force == ForceCommand, sp.postVerb:
			if f.Patient.Empty() {
				f.Patient = term
			} else if f.Agent.Empty() {
				f.Agent = term
			}
		default:
			if f.Agent.Empty() {
				f.Agent = term
			} else if f.Patient.Empty() {
				f.Patient = term
			}
		}
		if term.Concept == nil && sp.text != "" {
			f.Unknown = append(f.Unknown, sp.text)
		}
	}
	for _, q := range f.Quals {
		if q.Anchor.Concept == nil && q.Anchor.Phrase != "" {
			f.Unknown = append(f.Unknown, q.Anchor.Phrase)
		}
	}
	return f
}

// contentSpan joins a token run's open-class words, dropping the
// grammatical filler, so "the make test" and "make test" are one anchor.
func (p *Params) contentSpan(tokens []string) string {
	var out []string
	for _, tok := range tokens {
		switch p.category("en", tok) {
		case catDet, catFn, catAdv, catNeg:
			continue
		}
		out = append(out, tok)
	}
	return strings.Join(out, " ")
}

func resolveTerm(phrase string, resolve func(string) *memory.Fragment) Term {
	if phrase == "" {
		return Term{}
	}
	if resolve == nil {
		return Term{Phrase: phrase}
	}
	return Term{Concept: resolve(phrase), Phrase: phrase}
}

// Realize runs the innate grammar forward with the built-in parameter
// seed, for callers without a memory-backed grammar.
func Realize(f Frame, chem limbic.State) string {
	return Builtin().Realize(f, chem)
}

// Realize runs the innate grammar forward: a frame becomes a sentence,
// assembled as adjunct, subject, predicate, complement, never as a
// stored string. The frame's language picks the parameter setting; the
// chemistry reads the same either way. Oxytocin decides whether an
// inferential claim asserts ("usually follows") or hedges ("may
// follow"): a system whose judgement was recently corrected should
// sound like it.
func (p *Params) Realize(f Frame, chem limbic.State) string {
	p = p.get()
	if f.Lang == "tr" && p.verbFinal("tr") {
		return p.realizeTurkish(f, chem)
	}
	var b strings.Builder
	// Temporal adjuncts front the clause only when the predicate is the
	// ordered event itself; for "know" the anchors live inside the
	// embedded clause instead. Two qualifiers coordinate: "after X and
	// before Y, ...".
	rel, anchor := f.FirstQual()
	if rel != "" && !anchor.Empty() && f.Force != ForceQuestion &&
		(f.Action.Phrase == "follow" || f.Action.Phrase == "happen" || f.Action.Phrase == "run") {
		parts := make([]string, 0, len(f.Quals))
		for _, q := range f.Quals {
			if !q.Anchor.Empty() {
				parts = append(parts, relWord(q.Rel)+" "+q.Anchor.Speak())
			}
		}
		b.WriteString(strings.Join(parts, " and "))
		b.WriteString(", ")
	}

	subject := f.Agent.Speak()
	if subject == "" {
		subject = "I"
	}
	firstPerson := subject == "I"
	plural := f.Agent.Plural
	b.WriteString(subject)

	switch f.Action.Phrase {
	case "follow", "precede", "happen", "fail", "run":
		verb := f.Action.Phrase
		b.WriteString(" ")
		switch {
		case f.Negate:
			// A denial must never come out as its opposite. The
			// auxiliary carries agreement and tense; do-support works
			// the same for every verb here.
			b.WriteString(negatedVerb(p, verb, firstPerson, plural, f.Past))
		case chem.Oxytocin < limbic.OxytocinBaseline || f.Hedge:
			// A recently corrected system hedges its inferences.
			b.WriteString(hedgedVerb(p, verb, f.Past))
		case f.Habit:
			b.WriteString("usually ")
			b.WriteString(finiteVerb(p, verb, firstPerson, plural, f.Past))
		case f.Last:
			b.WriteString("last ")
			b.WriteString(finiteVerb(p, verb, firstPerson, plural, f.Past))
		default:
			b.WriteString(finiteVerb(p, verb, firstPerson, plural, f.Past))
		}
		if !f.Patient.Empty() {
			b.WriteString(" ")
			b.WriteString(f.Patient.Speak())
		}
		if f.At != "" {
			b.WriteString(" on ")
			b.WriteString(f.At)
		}
	case "know":
		// A knowledge claim is about now even when the gap it names is
		// about then: "I do not know what followed make test yet." The
		// tense lives in the embedded clause alone.
		b.WriteString(" ")
		if f.Negate {
			b.WriteString(negatedVerb(p, "know", firstPerson, plural, false))
		} else {
			b.WriteString(finiteVerb(p, "know", firstPerson, plural, false))
		}
		b.WriteString(" ")
		b.WriteString(embeddedClause(p, f))
		b.WriteString(" yet")
	case "be":
		if f.Force == ForceQuestion {
			return sentence("what is "+f.Patient.Speak(), "?")
		}
		b.WriteString(" ")
		be := beForm(firstPerson, plural, f.Past)
		if f.Negate {
			be += " not"
		}
		b.WriteString(be)
		b.WriteString(" ")
		b.WriteString(f.Patient.Speak())
	case "name":
		b.WriteString(" ")
		if f.Hedge {
			b.WriteString(hedgedVerb(p, "name", f.Past))
		} else {
			b.WriteString(finiteVerb(p, "name", firstPerson, plural, f.Past))
		}
		b.WriteString(" ")
		b.WriteString(f.Patient.Speak())
	case "propose":
		b.WriteString(" ")
		b.WriteString(finiteVerb(p, "propose", firstPerson, plural, f.Past))
		b.WriteString(" ")
		b.WriteString(f.Patient.Speak())
	case "see":
		// The experiential perfect: what the count of moments amounts to.
		b.WriteString(" have ")
		b.WriteString(pastPart(p, "see"))
		b.WriteString(" ")
		b.WriteString(f.Patient.Speak())
		b.WriteString(" ")
		b.WriteString(timesWord(f))
	}

	mark := "."
	if f.Force == ForceQuestion {
		mark = "?"
	}
	return sentence(b.String(), mark)
}

// embeddedClause renders the thing not known: a patient phrase, or the
// wh-clauses the temporal qualifiers asked about, inflected for the
// tense the question carried and coordinated when there are two.
func embeddedClause(p *Params, f Frame) string {
	if !f.Patient.Empty() {
		return f.Patient.Speak()
	}
	var parts []string
	for _, q := range f.Quals {
		if q.Anchor.Empty() {
			continue
		}
		verb := "follow"
		if q.Rel == "before" {
			verb = "precede"
		}
		parts = append(parts, "what "+finiteVerb(p, verb, false, false, f.Past)+" "+q.Anchor.Speak())
	}
	if len(parts) > 0 {
		return strings.Join(parts, " and ")
	}
	return "that"
}

// timesWord counts occurrences grammatically, so the realizer never
// emits "1 times."
func timesWord(f Frame) string {
	switch f.Times {
	case 1:
		return "once"
	case 2:
		return "twice"
	default:
		return fmt.Sprintf("%d times", f.Times)
	}
}

func relWord(rel string) string {
	if rel == "before" {
		return "before"
	}
	return "after"
}

// sentence finishes a clause: first letter up, terminal mark on, unless
// the content already ends with one.
func sentence(s, mark string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	r := []rune(s)
	if r[0] >= 'a' && r[0] <= 'z' {
		r[0] = r[0] - 'a' + 'A'
	}
	s = string(r)
	if strings.HasSuffix(s, ".") || strings.HasSuffix(s, "?") || strings.HasSuffix(s, "!") {
		return s
	}
	return s + mark
}
