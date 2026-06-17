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
	catWh     = "wh"     // interrogatives: open a question
	catRel    = "rel"    // temporal relators: attach an anchor clause
	catAdv    = "adv"    // adverbs of habit
	catAgain  = "again"  // the repetition pro-verb modifier
	catPro    = "pro"    // pro-forms: resolve from the workspace
	catOth    = "oth"    // "other": shift a pro-form past the warmest referent
	catImp    = "imp"    // imperative openers: command force
	catDet    = "det"    // determiners: skipped during resolution
	catCop    = "cop"    // copulas: definition questions
	catHap    = "hap"    // happen-verbs: event questions
	catNeg    = "neg"    // negators: deny the predicate
	catCnt    = "cnt"    // count quantifiers: "how many" asks for a number
	catFn     = "fn"     // grammatical filler with no slot of its own
	catTime   = "time"   // a temporal noun that compounds with an interrogative
	catDeixis = "deixis" // deictic pronouns: a span resolving to a participant
	catSpeech = "speech" // speech-act verbs: the clause is about the talk itself
	catThan   = "than"   // the comparative standard marker: "bigger THAN the cat"
)

// imperativeIntents maps the faculty verbs onto the acts they invoke:
// an utterance opening with one carries an intent over GOLEM's own
// action space rather than a cue for an external command. The mapping
// is engine procedure; whether a form still parses as imperative is the
// memory-backed closed class's call.
var imperativeIntents = map[string]string{
	"recall": "recall", "remember": "recall",
	"consolidate": "consolidate", "tidy": "consolidate",
	"introspect": "introspect",
}

// Parse runs the innate grammar backward with the built-in parameter
// seed, for callers without a memory-backed grammar.
func Parse(utterance string, resolve func(string) *memory.Fragment) Frame {
	return Builtin().Parse(utterance, resolve)
}

// detectLanguage reads the language an utterance is in from the seeded
// closed classes, which share no forms across languages, so a hit names the
// tongue. It returns the seeded language whose function words and sortals
// the tokens hit most, or the empty string when none does, so a third
// language is detected from its bootstrap alone and not a new code path.
func (p *Params) detectLanguage(tokens []string) string {
	best, bestHits := "", 0
	for _, lang := range sortedKeys(p.categories) {
		hits := 0
		for _, tok := range tokens {
			if p.category(lang, tok) != "" {
				hits++
			} else if _, ok := p.sortal(lang, tok); ok {
				hits++
			}
		}
		if hits > bestHits {
			best, bestHits = lang, hits
		}
	}
	return best
}

// Parse runs the innate grammar backward: an utterance becomes a frame. It
// reads the language from the seed (the closed classes share no forms across
// tongues) and hands the tokens to the one language-general procedure, the
// default tongue taken when none is detected.
func (p *Params) Parse(utterance string, resolve func(string) *memory.Fragment) Frame {
	p = p.get()
	tokens, question := Tokenize(utterance)
	if len(tokens) == 0 {
		return Frame{Force: ForceStatement}
	}
	lang := p.detectLanguage(tokens)
	if lang == "" {
		lang = "en"
	}
	return p.parse(lang, tokens, question, resolve)
}

// parse is the one parser for every language: the closed class, the event
// verbs, and the sortals are read by the detected language, and the word
// order parameter decides the rest. Where the verb sits sets the relator's
// side (a preposition before its anchor in a verb-medial language, a
// postposition after it in a verb-final one) and the order roles fill (the
// subject first before the verb, the object first after it or in a verb-final
// clause). The interrogative's type and a relator's direction are read from
// the seed, so a tongue whose wh-word or relator is a different form needs no
// code. The resolver is the learned half, mapping a phrase to its concept.
func (p *Params) parse(lang string, tokens []string, question bool, resolve func(string) *memory.Fragment) Frame {
	verbFinal := p.verbFinal(lang)
	f := Frame{Force: ForceStatement}
	if lang != "en" {
		f.Lang = lang
	}
	if question {
		f.Force = ForceQuestion
	}
	// An imperative opener commands; the wh-opener is read inline below, so a
	// verb-final language that forms its interrogative mid-clause reads alike.
	if p.category(lang, tokens[0]) == catImp {
		f.Force = ForceCommand
		// A faculty verb opens an intent over GOLEM's own acts; the generic
		// openers (do, run) leave the whole utterance as the cue.
		if intent, ok := imperativeIntents[tokens[0]]; ok {
			f.Action = Term{Phrase: intent}
		}
	}
	// A verb-medial clause that opens with a copula is a yes/no question by
	// subject-auxiliary inversion ("is a cat an animal"), the question the
	// statement "a cat is an animal" asks once its subject steps back behind the
	// verb. A verb-final tongue forms the question another way, so it is left out.
	if !verbFinal && p.category(lang, tokens[0]) == catCop {
		f.Force = ForceQuestion
	}
	// "X is bigger than Y": a comparison. Its predicate is a gradable relation
	// between two arguments, not the copular predication the rest of an "is" clause
	// is, so it is parsed apart into agent, relation, and standard. Inversion makes
	// it a question ("is the cat bigger than the mouse").
	if rel, subject, standard, ask, ok := p.comparativeParts(lang, tokens); ok {
		f.Comparative = true
		if ask {
			f.Force = ForceQuestion
		}
		f.Action = Term{Phrase: rel}
		f.Agent = resolveTerm(subject, resolve)
		f.Patient = resolveTerm(standard, resolve)
		return f
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
	// In a verb-medial language a relator precedes its anchor, so the anchor
	// is gathered after the relator until the next one; a verb-final language
	// takes the span already gathered as the anchor.
	prepRel := ""
	var prepToks []string
	closePrep := func() {
		if prepRel == "" {
			return
		}
		if anchor := p.contentSpan(prepToks); anchor != "" {
			f.Quals = append(f.Quals, Qualifier{Rel: prepRel, Anchor: resolveTerm(anchor, resolve)})
		}
		prepRel, prepToks = "", nil
	}

	for i := 0; i < len(tokens); i++ {
		tok := tokens[i]
		cat := p.category(lang, tok)

		if prepRel != "" { // gathering a preposition's anchor
			if cat == catRel {
				closePrep()
				prepRel = p.relType(lang, tok)
			} else {
				prepToks = append(prepToks, tok)
			}
			continue
		}

		// The do-auxiliary carries tense for the clause; sortals name the kind
		// of answer; an event verb is the predicate and flips the verb flag.
		if tok == "did" {
			f.Past = true
			flush()
			continue
		}
		if base, ok := p.sortal(lang, tok); ok {
			f.Sort = base
			f.SortPl = tok != base
			flush()
			continue
		}
		if v, ok := p.hap(lang, tok); ok {
			flush()
			if f.Force != ForceCommand || f.Action.Empty() {
				f.Action = Term{Phrase: v.base}
			}
			if v.past {
				f.Past = true
			}
			verbSeen = true
			continue
		}

		switch cat {
		case catRel:
			if verbFinal {
				// Postposition: the span just gathered is the anchor, its case
				// suffix shed from the final word.
				if len(current) > 0 {
					current[len(current)-1] = stripCaseTR(current[len(current)-1])
					anchor := strings.Join(current, " ")
					current = nil
					f.Quals = append(f.Quals, Qualifier{Rel: p.relType(lang, tok), Anchor: resolveTerm(anchor, resolve)})
				}
			} else {
				flush()
				prepRel = p.relType(lang, tok)
			}
		case catWh:
			f.Force = ForceQuestion
			whType, copula := p.whInfo(lang, tok)
			f.Wh = whType
			if copula && (f.Force != ForceCommand || f.Action.Empty()) {
				f.Action = Term{Phrase: "be"}
				verbSeen = true
			}
			// A verb-final language may form "when" as "what" plus a time
			// noun ("ne zaman"); the compound is read from the seed.
			if whType == "what" && i+1 < len(tokens) && p.category(lang, tokens[i+1]) == catTime {
				f.Wh = "when"
				i++
			}
			flush()
		case catCop:
			flush()
			if f.Force != ForceCommand || f.Action.Empty() {
				f.Action = Term{Phrase: "be"}
			}
			if tok == "was" || tok == "were" {
				f.Past = true
			}
			verbSeen = true
		case catCnt:
			f.Counting = true
			// Where the count word is the only interrogative, it carries the
			// question; a verb-medial clause already has its wh-opener.
			if verbFinal {
				f.Force = ForceQuestion
			}
			flush()
		case catPro:
			f.Pro = true
			if anim := p.proAnimacy(lang, tok); anim != "" {
				f.ProType = anim
			}
			flush()
		case catOth:
			f.Alt = true
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
		case catImp, catDet, catFn, catTime:
			flush()
		case catSpeech:
			// A speech-act verb is the predicate, but the move it carries (clarify,
			// recall) is about the talk rather than the world, so a clause built on
			// one is routed as a discourse move instead of read as a fact.
			flush()
			f.Speech = p.speechMove(lang, tok)
			verbSeen = true
		case catDeixis:
			// A deictic pronoun fills a role like a noun, but resolves to a
			// conversation participant rather than a thing named in the world.
			current = append(current, tok)
		default:
			current = append(current, tok)
		}
	}
	flush()
	closePrep()
	// "How often" counts the way "how many" does.
	if f.Wh == "how" && f.Habit {
		f.Counting = true
	}

	// Position against the verb assigns the roles. A verb-final clause, a
	// command, and a post-verb span fill the object first; a verb-medial
	// pre-verb span fills the subject first. A case-marked pre-verb span sheds
	// its suffix to reach the concept.
	for _, sp := range spans {
		text := sp.text
		if verbFinal && !sp.postVerb {
			words := strings.Fields(text)
			if len(words) > 0 {
				words[len(words)-1] = stripCaseTR(words[len(words)-1])
				text = strings.Join(words, " ")
			}
		}
		term := resolveTerm(text, resolve)
		if verbFinal || f.Force == ForceCommand || sp.postVerb {
			if f.Patient.Empty() {
				f.Patient = term
			} else if f.Agent.Empty() {
				f.Agent = term
			}
		} else if f.Agent.Empty() {
			f.Agent = term
		} else if f.Patient.Empty() {
			f.Patient = term
		}
		if term.Concept == nil && text != "" {
			f.Unknown = append(f.Unknown, text)
		}
	}
	for _, q := range f.Quals {
		if q.Anchor.Concept == nil && q.Anchor.Phrase != "" {
			f.Unknown = append(f.Unknown, q.Anchor.Phrase)
		}
	}
	return f
}

// comparativeParts reads a comparison "X is bigger than Y" into its relation
// (the gradable word before "than"), its subject, and its standard (after
// "than"), and whether it is asked (the copula inverted to the front). It returns
// false when the clause is not a comparison: no standard marker, no copula before
// it, or no gradable word between them. The standard marker is read from the
// tongue's seed, so a tongue marks comparison its own way.
func (p *Params) comparativeParts(lang string, tokens []string) (rel, subject, standard string, ask, ok bool) {
	than := -1
	for i, tok := range tokens {
		if p.category(lang, tok) == catThan {
			than = i
			break
		}
	}
	if than <= 0 || than == len(tokens)-1 {
		return "", "", "", false, false
	}
	cop := -1
	for i := 0; i < than-1; i++ {
		if p.category(lang, tokens[i]) == catCop {
			cop = i
			break
		}
	}
	if cop < 0 {
		return "", "", "", false, false
	}
	rel = tokens[than-1] // the gradable word sits just before the standard marker
	standard = p.contentSpan(tokens[than+1:])
	if cop == 0 {
		// The copula led the clause: subject-auxiliary inversion makes it a question,
		// the subject sitting between the copula and the gradable word.
		subject, ask = p.contentSpan(tokens[cop+1:than-1]), true
	} else {
		subject = p.contentSpan(tokens[:cop])
	}
	if rel == "" || subject == "" || standard == "" {
		return "", "", "", false, false
	}
	return rel, subject, standard, ask, true
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
	return TimesWord(f.Times)
}

// TimesWord renders an occurrence count the way prose does, so a
// construction slot reads "once" or "twice" before falling back to
// digits. The realizer and the planner share it.
func TimesWord(n int) string {
	switch n {
	case 1:
		return "once"
	case 2:
		return "twice"
	default:
		return fmt.Sprintf("%d times", n)
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
