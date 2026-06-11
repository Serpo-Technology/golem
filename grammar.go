package main

import (
	"fmt"
	"strings"
)

// GOLEM's language bet: the grammar ships built in and only the lexicon
// is learned. This file is the innate half. The closed class below is
// the seed the roadmap flags as a forced deviation: function words
// scaffold parsing and realization and cannot be induced from a handful
// of examples, so they arrive with their categories wired.
//
// Everything outside the closed class is a content word. Content words
// become word nodes in the graph, and meaning accrues on the concepts
// they come to name, never on the forms themselves.

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
// about GOLEM's own ontology, so they ship with the grammar instead of
// competing with content words for grounding. Plural forms invite more
// than one answer.
var sortalNouns = map[string]string{
	"command": "command", "commands": "command",
	"action": "action", "actions": "action",
	"thing": "thing", "things": "thing",
	"event": "event", "events": "event",
	"step": "step", "steps": "step",
}

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

func isFunctionWord(w string) bool {
	_, ok := innateCategory[w]
	return ok
}

// happenVerbs maps each event-verb surface form to the predicate it
// fills and the tense it carries, so "followed" and "follows" land in
// one concept with the time told apart. Succession verbs collapse onto
// "follow"; precedence verbs onto its inverse.
var happenVerbs = map[string]struct {
	base string
	past bool
}{
	"happens": {"follow", false}, "happen": {"follow", false}, "happened": {"follow", true},
	"follows": {"follow", false}, "follow": {"follow", false}, "followed": {"follow", true},
	"comes": {"follow", false}, "come": {"follow", false}, "came": {"follow", true},
	"precedes": {"precede", false}, "precede": {"precede", false}, "preceded": {"precede", true},
}

// tokenize lowercases an utterance and splits it into word tokens,
// trimming punctuation off both ends of each token so "Deploy," and
// "deploy" are one form. Word-internal marks survive, which keeps
// shapes like "go.mod" whole. A question mark anywhere is reported
// separately because it carries force, not content.
func tokenize(s string) (tokens []string, question bool) {
	question = strings.Contains(s, "?")
	for field := range strings.FieldsSeq(strings.ToLower(s)) {
		tok := strings.TrimFunc(field, func(r rune) bool {
			return !(r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r > 127)
		})
		if tok != "" {
			tokens = append(tokens, tok)
		}
	}
	return tokens, question
}

// contentTokens returns the distinct open-class words of an utterance,
// the ones eligible to become word nodes and point at concepts.
func contentTokens(s string) []string {
	tokens, _ := tokenize(s)
	seen := map[string]bool{}
	var out []string
	for _, tok := range tokens {
		if len(tok) < 2 || isFunctionWord(tok) || seen[tok] {
			continue
		}
		seen[tok] = true
		out = append(out, tok)
	}
	return out
}

// Force is the illocutionary force of an utterance: what the speaker is
// doing with it.
type Force string

const (
	ForceStatement Force = "statement"
	ForceQuestion  Force = "question"
	ForceCommand   Force = "command"
)

// Term is one filled thematic role: a concept when the phrase grounded
// to one, otherwise just the surface phrase, which is what lets the
// planner admit it does not know. Number lives on the term so the verb
// can agree with its subject instead of guessing.
type Term struct {
	Concept *Fragment // nil when nothing grounds the phrase
	Phrase  string
	Plural  bool
}

func (t Term) empty() bool { return t.Concept == nil && t.Phrase == "" }

// speak is the term's surface form for realization: the concept's gloss
// when grounded, the raw phrase when not.
func (t Term) speak() string {
	if t.Concept != nil {
		return t.Concept.Content
	}
	return t.Phrase
}

// Frame is the concept-frame: the small typed structure the parser fills
// and the realizer reads, the interface both halves of the grammar
// share. Thematic roles hold concepts where grounding found them; the
// temporal qualifier carries "what follows X" shapes; Unknown keeps the
// words that resolved to nothing.
type Frame struct {
	Force    Force
	Lang     string      // the language the utterance arrived in; answers go back the same way
	Wh       string      // the interrogative that opened a question: what, which, when, where, why, who, how
	Agent    Term        // who or what the clause is about
	Action   Term        // the predicate; for innate verbs only Phrase is set
	Patient  Term        // what is acted on or asked about
	Quals    []Qualifier // temporal adjuncts in spoken order; "after X and before Y" is two
	Sort     string      // sortal restriction from "which command": the kind of answer asked for
	SortPl   bool        // the sortal was plural, inviting more than one answer
	Counting bool        // "how many" or "how often": the answer is a count
	Past     bool        // tense: the clause is about a settled moment, "followed" against "follows"
	Negate   bool        // the predicate is denied
	Hedge    bool        // the claim is tentative ("may name")
	Habit    bool        // habitual aspect: the clause states a regularity ("usually")
	Times    int         // occurrence count, for "I have seen it N times"
	At       string      // a time or place complement, preformatted by the planner
	Last     bool        // the most recent occurrence: "last happened"
	Pro      bool        // the patient was a pro-form; resolve it from the workspace
	Again    bool        // repetition was requested
	Unknown  []string
}

// Qualifier is one temporal adjunct: a relator and the anchor it
// attaches to.
type Qualifier struct {
	Rel    string
	Anchor Term
}

// firstQual returns the leading temporal qualifier, the common case.
func (f Frame) firstQual() (string, Term) {
	if len(f.Quals) == 0 {
		return "", Term{}
	}
	return f.Quals[0].Rel, f.Quals[0].Anchor
}

// Parse runs the innate grammar backward: an utterance becomes a frame.
// The resolver is the learned half, mapping a surface phrase to its
// concept; everything else here is wired in. The grammar is small on
// purpose: force and interrogative from the opening word, temporal
// qualifiers each owning their anchor, and content spans into thematic
// roles by their position against the verb.
func Parse(utterance string, resolve func(string) *Fragment) Frame {
	tokens, question := tokenize(utterance)
	f := Frame{Force: ForceStatement}
	if len(tokens) == 0 {
		return f
	}
	// The closed classes share no forms, so one hit decides the grammar
	// the utterance is read with.
	if detectTurkish(tokens) {
		return parseTurkish(tokens, question, resolve)
	}
	switch {
	case innateCategory[tokens[0]] == catWh:
		f.Force = ForceQuestion
		f.Wh = tokens[0]
	case innateCategory[tokens[0]] == catImp:
		f.Force = ForceCommand
	case question:
		f.Force = ForceQuestion
	}

	// Each relator opens a qualifier whose anchor runs to the next
	// relator, so "after X and before Y" parses as two qualifiers
	// instead of folding the second relator into the first anchor.
	main := tokens
	for i, tok := range tokens {
		if innateCategory[tok] != catRel {
			continue
		}
		main = tokens[:i]
		for i < len(tokens) {
			rel := tokens[i]
			j := i + 1
			for j < len(tokens) && innateCategory[tokens[j]] != catRel {
				j++
			}
			anchor := contentSpan(tokens[i+1 : j])
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
		if base, ok := sortalNouns[tok]; ok {
			f.Sort = base
			f.SortPl = tok != base
			flush()
			continue
		}
		switch innateCategory[tok] {
		case catHap:
			// The span before the verb closes as the subject before the
			// flag flips; what follows is the object.
			flush()
			v := happenVerbs[tok]
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
			if f.Patient.empty() {
				f.Patient = term
			} else if f.Agent.empty() {
				f.Agent = term
			}
		default:
			if f.Agent.empty() {
				f.Agent = term
			} else if f.Patient.empty() {
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
func contentSpan(tokens []string) string {
	var out []string
	for _, tok := range tokens {
		switch innateCategory[tok] {
		case catDet, catFn, catAdv, catNeg:
			continue
		}
		out = append(out, tok)
	}
	return strings.Join(out, " ")
}

func resolveTerm(phrase string, resolve func(string) *Fragment) Term {
	if phrase == "" {
		return Term{}
	}
	if resolve == nil {
		return Term{Phrase: phrase}
	}
	return Term{Concept: resolve(phrase), Phrase: phrase}
}

// Realize runs the innate grammar forward: a frame becomes a sentence,
// assembled as adjunct, subject, predicate, complement, never as a
// stored string. The frame's language picks the parameter setting; the
// chemistry reads the same either way. Oxytocin decides whether an
// inferential claim asserts ("usually follows") or hedges ("may
// follow"): a system whose judgement was recently corrected should
// sound like it.
func Realize(f Frame, chem LimbicState) string {
	if f.Lang == "tr" {
		return realizeTurkish(f, chem)
	}
	var b strings.Builder
	// Temporal adjuncts front the clause only when the predicate is the
	// ordered event itself; for "know" the anchors live inside the
	// embedded clause instead. Two qualifiers coordinate: "after X and
	// before Y, ...".
	rel, anchor := f.firstQual()
	if rel != "" && !anchor.empty() && f.Force != ForceQuestion &&
		(f.Action.Phrase == "follow" || f.Action.Phrase == "happen" || f.Action.Phrase == "run") {
		parts := make([]string, 0, len(f.Quals))
		for _, q := range f.Quals {
			if !q.Anchor.empty() {
				parts = append(parts, relWord(q.Rel)+" "+q.Anchor.speak())
			}
		}
		b.WriteString(strings.Join(parts, " and "))
		b.WriteString(", ")
	}

	subject := f.Agent.speak()
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
			b.WriteString(negatedVerb(verb, firstPerson, plural, f.Past))
		case chem.Oxytocin < oxytocinBaseline || f.Hedge:
			// A recently corrected system hedges its inferences.
			b.WriteString(hedgedVerb(verb, f.Past))
		case f.Habit:
			b.WriteString("usually ")
			b.WriteString(finiteVerb(verb, firstPerson, plural, f.Past))
		case f.Last:
			b.WriteString("last ")
			b.WriteString(finiteVerb(verb, firstPerson, plural, f.Past))
		default:
			b.WriteString(finiteVerb(verb, firstPerson, plural, f.Past))
		}
		if !f.Patient.empty() {
			b.WriteString(" ")
			b.WriteString(f.Patient.speak())
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
			b.WriteString(negatedVerb("know", firstPerson, plural, false))
		} else {
			b.WriteString(finiteVerb("know", firstPerson, plural, false))
		}
		b.WriteString(" ")
		b.WriteString(embeddedClause(f))
		b.WriteString(" yet")
	case "be":
		if f.Force == ForceQuestion {
			return sentence("what is "+f.Patient.speak(), "?")
		}
		b.WriteString(" ")
		be := beForm(firstPerson, plural, f.Past)
		if f.Negate {
			be += " not"
		}
		b.WriteString(be)
		b.WriteString(" ")
		b.WriteString(f.Patient.speak())
	case "name":
		b.WriteString(" ")
		if f.Hedge {
			b.WriteString(hedgedVerb("name", f.Past))
		} else {
			b.WriteString(finiteVerb("name", firstPerson, plural, f.Past))
		}
		b.WriteString(" ")
		b.WriteString(f.Patient.speak())
	case "propose":
		b.WriteString(" ")
		b.WriteString(finiteVerb("propose", firstPerson, plural, f.Past))
		b.WriteString(" ")
		b.WriteString(f.Patient.speak())
	case "see":
		// The experiential perfect: what the count of moments amounts to.
		b.WriteString(" have ")
		b.WriteString(pastPart("see"))
		b.WriteString(" ")
		b.WriteString(f.Patient.speak())
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
func embeddedClause(f Frame) string {
	if !f.Patient.empty() {
		return f.Patient.speak()
	}
	var parts []string
	for _, q := range f.Quals {
		if q.Anchor.empty() {
			continue
		}
		verb := "follow"
		if q.Rel == "before" {
			verb = "precede"
		}
		parts = append(parts, "what "+finiteVerb(verb, false, false, f.Past)+" "+q.Anchor.speak())
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
