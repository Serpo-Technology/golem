package grammar

import (
	"fmt"
	"strings"
	"unicode"

	"golem/internal/limbic"
	"golem/internal/memory"
)

// Categories the innate grammar distinguishes. Content words take their
// category from their concept's grounding role instead (verb for shapes
// in command position, noun for recurring sensory objects).
const (
	catWh     = "wh"       // interrogatives: open a question
	catRel    = "rel"      // temporal relators: attach an anchor clause
	catAdv    = "adv"      // adverbs of habit
	catAgain  = "again"    // the repetition pro-verb modifier
	catPro    = "pro"      // pro-forms: resolve from the workspace
	catOth    = "oth"      // "other": shift a pro-form past the warmest referent
	catImp    = "imp"      // imperative openers: command force
	catDet    = "det"      // determiners: skipped during resolution
	catCop    = "cop"      // copulas: definition questions
	catHap    = "hap"      // happen-verbs: event questions
	catNeg    = "neg"      // negators: deny the predicate
	catCnt    = "cnt"      // count quantifiers: "how many" asks for a number
	catFn     = "fn"       // grammatical filler with no slot of its own
	catTime   = "time"     // a temporal noun that compounds with an interrogative
	catDeixis = "deixis"   // deictic pronouns: a span resolving to a participant
	catSpeech = "speech"   // speech-act verbs: the clause is about the talk itself
	catThan   = "than"     // the comparative standard marker: "bigger THAN the cat"
	catProp   = "property" // a relation-property predicate: "before is TRANSITIVE"
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
	return p.ParseHint(utterance, "", resolve)
}

// Detect names the tongue an utterance's closed class reveals, "" when no
// function word settles it. It exposes the same judgement ParseHint opens
// with, so a caller can tell an undetected clause (open-class only, left to
// morphology) from one detection has already placed.
func (p *Params) Detect(utterance string) string {
	p = p.get()
	tokens, _ := Tokenize(utterance)
	return p.detectLanguage(tokens)
}

// ParseHint is Parse with a fallback tongue for when the closed class does not
// reveal one. A clause of only open-class words (a Turkish comparison "kedi
// fareden büyük", whose case marking is morphological, not a function word) detects
// as no language; the caller's tag then settles it, so the parse runs in the right
// tongue rather than defaulting to English and missing its constructions.
func (p *Params) ParseHint(utterance, fallback string, resolve func(string) *memory.Fragment) Frame {
	p = p.get()
	tokens, question := Tokenize(utterance)
	if len(tokens) == 0 {
		return Frame{Force: ForceStatement}
	}
	// An explicit tongue tag is authoritative: the speaker said which language this
	// is. Detection is the fallback for an untagged utterance, where the closed
	// classes name the tongue. Reading detection first would let a word that spells
	// a foreign function word ("at", a Turkish horse, is also English "at") tip a
	// tagged Turkish clause to English and lose its construction.
	lang := fallback
	if lang == "" {
		lang = p.detectLanguage(tokens)
	}
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
		if p.doSupportQuestion(lang, tokens) {
			// "do" also fronts a yes/no question by do-support: before a subject it
			// inverts for a question ("do you want the rose"), where the bare
			// imperative ("do the dishes") commands. The polar path reads the clause.
			f.Force = ForceQuestion
		} else {
			f.Force = ForceCommand
			// A faculty verb opens an intent over GOLEM's own acts; the generic
			// openers (do, run) leave the whole utterance as the cue.
			if intent, ok := imperativeIntents[tokens[0]]; ok {
				f.Action = Term{Phrase: intent}
			}
		}
	}
	// A verb-medial clause that opens with a copula is a yes/no question by
	// subject-auxiliary inversion ("is a cat an animal"), the question the
	// statement "a cat is an animal" asks once its subject steps back behind the
	// verb. A verb-final tongue forms the question another way, so it is left out.
	if !verbFinal && p.category(lang, tokens[0]) == catCop {
		f.Force = ForceQuestion
	}
	// "who chased what the dog chased": a nested question, the asked role of the
	// outer clause filled by an inner one. A second interrogative mid-clause opens
	// the embedded clause; it is split off and the outer parsed alone, the inner
	// kept for the planner to resolve first and feed back in.
	if outer, inner, ok := p.splitNested(lang, tokens); ok {
		f = p.parse(lang, outer, question, resolve)
		f.Inner = inner
		return f
	}
	// "A is to B as C is to what": a proportion. Its four terms span two clauses
	// joined by "as", a shape the clause-at-a-time parse below cannot hold, so it is
	// read apart into the three known terms; the fourth is what the question asks.
	if a, b, c, ok := p.proportionParts(lang, tokens); ok {
		f.Analogy, f.Force = true, ForceQuestion
		f.Agent = resolveTerm(a, resolve)
		f.Patient = resolveTerm(b, resolve)
		f.Third = resolveTerm(c, resolve)
		return f
	}
	// "how much bigger is X than Y": a comparison asked for its DEGREE, not its
	// truth. It opens with the degree interrogative, so it is read before the plain
	// comparison below (which would mistake the subject for the relation).
	if rel, subject, standard, ok := p.magnitudeParts(lang, tokens); ok {
		f.Comparative, f.Magnitude, f.Force = true, true, ForceQuestion
		f.Action = Term{Phrase: rel}
		f.Agent = resolveTerm(subject, resolve)
		f.Patient = resolveTerm(standard, resolve)
		return f
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
	// Turkish marks the comparison's standard by ablative case and drops the copula
	// ("kedi fareden büyük"), so it is read its own way; the reasoning over the
	// ordering is the same once the roles are filled.
	if lang == "tr" {
		if rel, subject, standard, ask, ok := p.trComparativeParts(tokens); ok {
			f.Comparative = true
			if ask {
				f.Force = ForceQuestion
			}
			f.Action = Term{Phrase: rel}
			f.Agent = resolveTerm(subject, resolve)
			f.Patient = resolveTerm(standard, resolve)
			return f
		}
		// Turkish drops the copula and asks an is-a with the interrogative particle
		// ("kedi bir hayvan mı"), a shape verb-medial subject-auxiliary inversion cannot
		// reach. The subject fills the patient and the predicate the agent, the post-copula
		// order the is-a planner reads, so the reasoning is the same once the roles are set.
		if member, category, ok := p.trCopularQuestionParts(tokens); ok {
			f.Force = ForceQuestion
			f.Action = Term{Phrase: "be"}
			f.Patient = resolveTerm(member, resolve)
			f.Agent = resolveTerm(category, resolve)
			return f
		}
	}

	type span struct {
		text     string
		postVerb bool
		topic    bool // the object of a topic preposition ("de X"): the patient whatever its position
	}
	var spans []span
	var current []string
	verbSeen := false
	topicNext := false
	flush := func() {
		if len(current) > 0 {
			spans = append(spans, span{strings.Join(current, " "), verbSeen, topicNext})
			current = nil
			topicNext = false
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
			// A locative preposition in a copula clause marks the clause a place
			// question ("is the leaf on the tree"), not a category one. The word
			// itself fills no slot, so it is still flushed; the mark routes the
			// clause away from the is-a reading to the place reading.
			if cat == catFn && p.LocativePrep(lang, tok) {
				f.LocPrep = tok
			}
			// A topic preposition ("quid de cane scis") names what the clause is about; its
			// object fills the patient, so the span after it is marked, the word itself flushed.
			topic := cat == catFn && p.TopicPrep(lang, tok)
			flush()
			if topic {
				topicNext = true
			}
		case catSpeech:
			// A speech-act verb is the predicate, but the move it carries (clarify,
			// recall) is about the talk rather than the world, so a clause built on
			// one is routed as a discourse move instead of read as a fact.
			flush()
			f.Speech = p.speechMove(lang, tok)
			verbSeen = true
		case catProp:
			// A property predicated of a relation ("before is transitive") names a
			// learned rule about the relation, not a thing acted on, so it is read off
			// the frame and the clause routed to learn the property rather than written
			// as a fact. The subject stays the only content span, the relation it marks.
			flush()
			f.Property = p.propertyName(lang, tok)
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
		if sp.topic {
			if f.Patient.Empty() {
				f.Patient = term
			}
		} else if verbFinal || f.Force == ForceCommand || sp.postVerb {
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

// comprehendCopularQuestion reads a yes/no copula question formed by subject-auxiliary inversion
// ("is a cat an animal") into its frame by walking the construction: the fronted copula marks the
// question and the predicate "be", and the two noun phrases after it are the subject and the
// complement, filling the patient then the agent the way the coded role assignment does for a
// post-verb span. It is the traversal counterpart for this question shape, the inverse of how the
// copula is generated; it declines a copula clause that carries a place or a comparison (those are
// their own constructions). It returns false when the clause is not a plain copular question.
func (p *Params) comprehendCopularQuestion(lang, utterance string, resolve func(string) *memory.Fragment) (Frame, bool) {
	p = p.get()
	if lang == "" {
		lang = "en"
	}
	if p.verbFinal(lang) {
		return Frame{}, false // subject-auxiliary inversion is a verb-medial form
	}
	tokens, _ := Tokenize(utterance)
	if len(tokens) < 3 || p.category(lang, tokens[0]) != catCop {
		return Frame{}, false
	}
	f := Frame{Force: ForceQuestion, Action: Term{Phrase: "be"}}
	if lang != "en" {
		f.Lang = lang
	}
	if tokens[0] == "was" || tokens[0] == "were" {
		f.Past = true
	}
	// Segment the noun phrases after the copula, a determiner opening each. A place preposition or
	// a comparison marker means a locative or comparative clause, its own construction, declined.
	var nps []string
	var cur []string
	for _, tok := range tokens[1:] {
		switch cat := p.category(lang, tok); {
		case cat == catThan || p.LocativePrep(lang, tok) || p.TopicPrep(lang, tok):
			return Frame{}, false
		case cat == catCop || cat == catWh:
			return Frame{}, false // a second verb or a wh: not this shape
		case cat == catDet:
			if len(cur) > 0 {
				nps = append(nps, strings.Join(cur, " "))
				cur = nil
			}
		default:
			cur = append(cur, tok)
		}
	}
	if len(cur) > 0 {
		nps = append(nps, strings.Join(cur, " "))
	}
	if len(nps) != 2 {
		return Frame{}, false
	}
	// Both noun phrases sit after the fronted copula, so the first fills the patient and the
	// second the agent, the post-verb order the coded role assignment uses.
	f.Patient = resolveTerm(nps[0], resolve)
	f.Agent = resolveTerm(nps[1], resolve)
	return f, true
}

// comprehendWhQuestion reads a wh-fronted role question ("who chased the cat", "what did the dog
// chase") into its frame's recognition: the force and the interrogative. The fronted wh word marks
// the question and which role is asked; the verb and the known argument are read off the tokens by
// the planner (analyzeRoleQuestion), already a grounded walk, so the recognition is what the frame
// supplies. It declines a fused copula wh (the definition question "nedir") and a wh followed by a
// copula (a definition or place question), which are their own shapes. It returns false when the
// clause is not a plain wh-fronted role question.
func (p *Params) comprehendWhQuestion(lang, utterance string, resolve func(string) *memory.Fragment) (Frame, bool) {
	p = p.get()
	if lang == "" {
		lang = "en"
	}
	tokens, _ := Tokenize(utterance)
	if len(tokens) < 2 || p.category(lang, tokens[0]) != catWh {
		return Frame{}, false
	}
	whType, copula := p.whInfo(lang, tokens[0])
	if copula {
		return Frame{}, false // a fused copula wh is the definition question, its own shape
	}
	for _, tok := range tokens[1:] {
		if p.category(lang, tok) == catCop {
			return Frame{}, false // a copula makes it a definition or place question, not a role one
		}
	}
	f := Frame{Force: ForceQuestion, Wh: whType}
	if lang != "en" {
		f.Lang = lang
	}
	return f, true
}

// comprehendComparativeQuestion reads an asked comparison ("is the cat bigger than the mouse",
// "kedi fareden büyük mü") into its frame. The comparative is its own construction, a gradable
// relation between two arguments with the standard marked off (English "than", Turkish the ablative
// case), and inversion makes it a question: a fronted copula in English, a trailing question
// particle in Turkish. So the recognition routes through the same comparative-construction walk the
// coded parse reads (comparativeParts / trComparativeParts), both category-driven, and keeps only the
// asked form: the relation fills the predicate, the subject the agent, the standard the patient, the
// way planComparative consumes them. It declines the bare statement comparison (its own path stores
// it) and any clause that is not a comparison.
func (p *Params) comprehendComparativeQuestion(lang, utterance string, resolve func(string) *memory.Fragment) (Frame, bool) {
	p = p.get()
	if lang == "" {
		lang = "en"
	}
	tokens, _ := Tokenize(utterance)
	var rel, subject, standard string
	var ask, ok bool
	if lang == "tr" {
		rel, subject, standard, ask, ok = p.trComparativeParts(tokens)
	} else {
		rel, subject, standard, ask, ok = p.comparativeParts(lang, tokens)
	}
	if !ok || !ask {
		return Frame{}, false // not a comparison, or the bare statement form
	}
	f := Frame{Force: ForceQuestion, Comparative: true, Action: Term{Phrase: rel}}
	if lang != "en" {
		f.Lang = lang
	}
	f.Agent = resolveTerm(subject, resolve)
	f.Patient = resolveTerm(standard, resolve)
	return f, true
}

// comprehendMagnitudeQuestion reads a comparison asked for its DEGREE ("how much bigger is the lion
// than the mouse") into its frame. It is the comparison construction opened by the degree
// interrogative ("how much") instead of inverted for truth, so it carries both Comparative and
// Magnitude, and the relation, subject, and standard fill the same roles planMagnitude reads. The
// recognition routes through magnitudeParts, the same "how much" walk the coded parse reads, which
// is keyed on the English degree opener and so declines the other tongues. It declines any clause
// not opened that way (a plain comparison, a copular question).
func (p *Params) comprehendMagnitudeQuestion(lang, utterance string, resolve func(string) *memory.Fragment) (Frame, bool) {
	p = p.get()
	if lang == "" {
		lang = "en"
	}
	tokens, _ := Tokenize(utterance)
	rel, subject, standard, ok := p.magnitudeParts(lang, tokens)
	if !ok {
		return Frame{}, false
	}
	f := Frame{Force: ForceQuestion, Comparative: true, Magnitude: true, Action: Term{Phrase: rel}}
	if lang != "en" {
		f.Lang = lang
	}
	f.Agent = resolveTerm(subject, resolve)
	f.Patient = resolveTerm(standard, resolve)
	return f, true
}

// ParseByTraversal reads a question to its frame by walking the grounded constructions, the
// question-side counterpart of ComprehendByTraversal. It handles the copular yes/no question, the
// asked comparison and its degree question, and the wh-fronted role question; the remaining special
// forms and the do-support polars are still the coded parser's, so it declines them. The degree
// question is tried before the plain comparison, the precedence the coded parse keeps so the degree
// opener is not mistaken for the subject. It is shadowed against the coded Parse, demonstrated before
// the cutover.
func (p *Params) ParseByTraversal(lang, utterance string, resolve func(string) *memory.Fragment) (Frame, bool) {
	if f, ok := p.comprehendCopularQuestion(lang, utterance, resolve); ok {
		return f, true
	}
	if f, ok := p.comprehendMagnitudeQuestion(lang, utterance, resolve); ok {
		return f, true
	}
	if f, ok := p.comprehendComparativeQuestion(lang, utterance, resolve); ok {
		return f, true
	}
	return p.comprehendWhQuestion(lang, utterance, resolve)
}

// comparativeParts reads a comparison "X is bigger than Y" into its relation, its
// subject, and its standard (after "than"), and whether it is asked (the copula
// inverted to the front). The relation is the gradable word before "than"; an
// analytic comparative marks it with "more"/"less" instead of "-er", which is
// normalized to "more <adjective>" so the comparative is one relation however it
// was formed. "Less" is the grammatical inverse of "more" ("X less clever than Y"
// is "Y more clever than X"), so its subject and standard are swapped. It returns
// false when the clause is not a comparison: no standard marker, no copula before
// it, or no gradable word between them.
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
	// The gradable word sits just before "than"; an analytic comparative puts
	// "more" or "less" before it, which is folded into the relation.
	relStart, invert := than-1, false
	if than-2 > cop {
		switch tokens[than-2] {
		case "more":
			relStart = than - 2
		case "less":
			relStart, invert = than-2, true
		}
	}
	if relStart < than-1 {
		rel = "more " + tokens[than-1] // analytic comparative, normalized
	} else {
		rel = tokens[than-1] // synthetic "-er" comparative
	}
	standard = p.contentSpan(tokens[than+1:])
	if cop == 0 {
		// The copula led the clause: subject-auxiliary inversion makes it a question,
		// the subject sitting between the copula and the comparative.
		subject, ask = p.contentSpan(tokens[cop+1:relStart]), true
	} else {
		subject = p.contentSpan(tokens[:cop])
	}
	if invert {
		subject, standard = standard, subject // "X less clever than Y" is "Y more clever than X"
	}
	if rel == "" || subject == "" || standard == "" {
		return "", "", "", false, false
	}
	return rel, subject, standard, ask, true
}

// splitNested separates a nested question into its outer clause and the embedded
// inner one: a second interrogative, with a clause before it, opens an embedded
// clause that fills the outer's asked role ("who chased | what the dog chased").
// It returns false when there is no second interrogative or nothing precedes it.
func (p *Params) splitNested(lang string, tokens []string) (outer []string, inner string, ok bool) {
	var whs []int
	for i, tok := range tokens {
		if p.category(lang, tok) == catWh {
			whs = append(whs, i)
		}
	}
	if len(whs) < 2 || whs[1] < 2 {
		return nil, "", false
	}
	return tokens[:whs[1]], strings.Join(tokens[whs[1]:], " "), true
}

// proportionParts reads "A is to B as C is to <what>" into its three known terms,
// the first pair and the third whose fourth is asked. The clause "X is to Y" names
// a relation by example rather than word, and "as" pivots one such clause against
// another. It returns false when the shape is absent: no pivot, or a half that is
// not an "is to" clause.
func (p *Params) proportionParts(lang string, tokens []string) (a, b, c string, ok bool) {
	as := -1
	for i, tok := range tokens {
		if tok == "as" { // the proportional pivot
			as = i
			break
		}
	}
	if as <= 0 || as >= len(tokens)-1 {
		return "", "", "", false
	}
	a, b, ok1 := p.isToParts(lang, tokens[:as])
	c, _, ok2 := p.isToParts(lang, tokens[as+1:]) // the right half's fourth term is the asked one
	if !ok1 || !ok2 {
		return "", "", "", false
	}
	return a, b, c, true
}

// isToParts reads one "X is to Y" half of a proportion: the copula with "to" after
// it splits the two terms. The right half's Y is the asked "what", so an empty Y is
// allowed there; only X is required of either half.
func (p *Params) isToParts(lang string, toks []string) (x, y string, ok bool) {
	cop := -1
	for i, tok := range toks {
		if p.category(lang, tok) == catCop {
			cop = i
			break
		}
	}
	if cop < 1 || cop+1 >= len(toks) || toks[cop+1] != "to" {
		return "", "", false
	}
	x = p.contentSpan(toks[:cop])
	y = p.contentSpan(toks[cop+2:])
	if x == "" {
		return "", "", false
	}
	return x, y, true
}

// magnitudeParts reads "how much bigger is X than Y" into its relation (the
// gradable word after the degree opener), subject, and standard. The opener is the
// degree interrogative "how much" (a wh-how followed by the quantity word), so a
// tongue forms the degree question its own way. It returns false when the clause is
// not so opened, or lacks the copula or standard the shape needs.
func (p *Params) magnitudeParts(lang string, tokens []string) (rel, subject, standard string, ok bool) {
	if len(tokens) < 6 {
		return "", "", "", false
	}
	if wh, _ := p.whInfo(lang, tokens[0]); wh != "how" {
		return "", "", "", false
	}
	if p.category(lang, tokens[1]) != catCnt { // "much", the quantity word "how" leans on
		return "", "", "", false
	}
	than := -1
	for i, tok := range tokens {
		if p.category(lang, tok) == catThan {
			than = i
			break
		}
	}
	if than < 4 || than == len(tokens)-1 {
		return "", "", "", false
	}
	cop := -1
	for i := 2; i < than; i++ {
		if p.category(lang, tokens[i]) == catCop {
			cop = i
			break
		}
	}
	if cop < 3 { // need a gradable word between "how much" and the copula
		return "", "", "", false
	}
	rel = p.contentSpan(tokens[2:cop])
	subject = p.contentSpan(tokens[cop+1 : than])
	standard = p.contentSpan(tokens[than+1:])
	if rel == "" || subject == "" || standard == "" {
		return "", "", "", false
	}
	return rel, subject, standard, true
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

// Realize realizes a frame with the built-in seed, for callers without a memory-backed grammar:
// the declarative engine where it composes the clause (the copula, the event verbs), the coded
// residual where the construction is an idiom (know, see, name, propose).
func Realize(f Frame, chem limbic.State) string {
	p := Builtin()
	if s, ok := p.RealizeDecl(f, chem); ok {
		return s
	}
	return p.Realize(f, chem)
}

// RealizeDecl realizes a frame through the declarative engine with the built-in seed, for
// callers without a memory-backed grammar.
func RealizeDecl(f Frame, chem limbic.State) (string, bool) {
	return Builtin().RealizeDecl(f, chem)
}

// Realize is the coded residual for the idiomatic constructions the declarative engine does not
// compose: knowledge claims ("I do not know what followed make test yet"), the experiential
// perfect ("I have seen that twice"), naming, and proposing. The compositional clauses (the
// copula and the event verbs) generate through RealizeDecl instead; this is reached only when
// that declines. The frame's language picks the parameter setting; the chemistry reads the same.
func (p *Params) Realize(f Frame, chem limbic.State) string {
	p = p.get()
	if f.Lang == "tr" && p.verbFinal("tr") {
		return p.realizeTurkish(f, chem)
	}
	var b strings.Builder
	subject := f.Agent.Speak()
	if subject == "" {
		subject = "I"
	}
	firstPerson := subject == "I"
	plural := f.Agent.Plural
	b.WriteString(subject)

	switch f.Action.Phrase {
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
		// The "yet" belongs to the admitted gap, the not-known-so-far; a
		// positive knowledge claim ("I know 79 animals") carries none.
		if f.Negate {
			b.WriteString(" yet")
		}
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
// the content already ends with one. The first letter is cased by rune, so a
// clause a tongue opens with an accented letter rises too (the Turkish "üzüm" ->
// "Üzüm"), not only an ASCII one.
func sentence(s, mark string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	r := []rune(s)
	r[0] = unicode.ToUpper(r[0])
	s = string(r)
	if strings.HasSuffix(s, ".") || strings.HasSuffix(s, "?") || strings.HasSuffix(s, "!") {
		return s
	}
	return s + mark
}
