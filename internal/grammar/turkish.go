package grammar

import (
	"strconv"
	"strings"

	"golem/internal/limbic"
	"golem/internal/memory"
)

// Turkish, the second language: the same concept-frame read and spoken
// under a different parameter setting. Word order is SOV with the
// temporal adjunct fronted, relators follow their anchors instead of
// preceding them, and grammar lives in suffixes stacked on the stem,
// shaped by vowel harmony and voicing assimilation. Harmony runs on
// spelling here, the same no-phonology deviation the roadmap flags for
// English: input is text, so the last written vowel decides.
//
// Anchors are command names, foreign words in Turkish orthography, so
// case suffixes attach with an apostrophe the way Turkish writes
// suffixed proper names: "make test'ten sonra", "make deploy'dan once".

// detectTurkish reports whether an utterance is Turkish: any closed-class
// or sortal hit decides, since the two vocabularies share no forms.
func (p *Params) detectTurkish(tokens []string) bool {
	for _, tok := range tokens {
		if p.category("tr", tok) != "" {
			return true
		}
		if _, ok := p.sortal("tr", tok); ok {
			return true
		}
	}
	return false
}

// stripCaseTR removes a case suffix from an anchor word: the apostrophe
// form of a suffixed foreign name ("test'ten" to "test"), or a bare
// ablative/locative ending when the word plainly carries one.
func stripCaseTR(w string) string {
	if i := strings.IndexRune(w, '\''); i > 0 {
		return w[:i]
	}
	for _, suf := range []string{"dan", "den", "tan", "ten"} {
		if strings.HasSuffix(w, suf) && len(w) > len(suf)+2 {
			return w[:len(w)-len(suf)]
		}
	}
	return w
}

// stripCase removes a case suffix from a noun-phrase span so a comprehended
// role-filler resolves to its base concept. Turkish marks the definite
// object accusative and a temporal anchor ablative, while English marks no
// case, so this is the identity outside Turkish. The comprehension engine
// calls it as a fallback when the surface span names nothing, leaving an
// unmarked word untouched.
func (p *Params) stripCase(lang, phrase string) string {
	if lang != "tr" {
		return phrase
	}
	words := strings.Fields(phrase)
	if len(words) == 0 {
		return phrase
	}
	words[len(words)-1] = stripCaseAccTR(words[len(words)-1])
	return strings.Join(words, " ")
}

// stripCaseAccTR removes the accusative a definite object wears, as well as
// the ablative and locative an anchor may carry, so "kediyi" resolves to
// "kedi". The accusative is the high vowel, with the buffer y after a vowel
// stem. It is a best effort: applied only when the marked form failed to
// resolve, so it never strips a word it would not then ground.
func stripCaseAccTR(w string) string {
	w = stripCaseTR(w)
	for _, suf := range []string{"yı", "yi", "yu", "yü"} {
		if strings.HasSuffix(w, suf) && len([]rune(w)) > 3 {
			return strings.TrimSuffix(w, suf)
		}
	}
	for _, suf := range []string{"ı", "i", "u", "ü"} {
		if strings.HasSuffix(w, suf) && len([]rune(w)) > 2 {
			return strings.TrimSuffix(w, suf)
		}
	}
	return w
}

// turkishVerbStem recovers a Turkish verb's stem from an inflected surface,
// stripping a past-tense suffix so a relation heard as "kovaladı" keys on
// "kovala". It is the Turkish twin of the English reverse morphology, a best
// effort over the regular paradigm; a seeded event verb is resolved by the
// closed class before this is consulted.
func turkishVerbStem(surface string) string {
	for _, suf := range []string{"dı", "di", "du", "dü", "tı", "ti", "tu", "tü"} {
		if strings.HasSuffix(surface, suf) && len([]rune(surface)) > 3 {
			return strings.TrimSuffix(surface, suf)
		}
	}
	return surface
}

// parseTurkish reads a Turkish utterance into the shared frame. SOV
// reverses the qualifier: the anchor span comes before its relator.
func (p *Params) parseTurkish(tokens []string, question bool, resolve func(string) *memory.Fragment) Frame {
	f := Frame{Force: ForceStatement, Lang: "tr"}
	if question {
		f.Force = ForceQuestion
	}

	var current []string
	verbSeen := false
	type span struct {
		text     string
		postVerb bool
	}
	var spans []span
	flush := func() {
		if len(current) > 0 {
			spans = append(spans, span{strings.Join(current, " "), verbSeen})
			current = nil
		}
	}

	for i := 0; i < len(tokens); i++ {
		tok := tokens[i]
		if base, ok := p.sortal("tr", tok); ok {
			f.Sort = base
			f.SortPl = tok != base
			flush()
			continue
		}
		if v, ok := p.hap("tr", tok); ok {
			flush()
			f.Action = Term{Phrase: v.base}
			if v.past {
				f.Past = true
			}
			verbSeen = true
			continue
		}
		switch p.category("tr", tok) {
		case catRel:
			// The span just gathered is this relator's anchor, its case
			// suffix stripped from the final word.
			rel := "after"
			if tok == "önce" {
				rel = "before"
			}
			if len(current) > 0 {
				current[len(current)-1] = stripCaseTR(current[len(current)-1])
				anchor := strings.Join(current, " ")
				current = nil
				f.Quals = append(f.Quals, Qualifier{Rel: rel, Anchor: resolveTerm(anchor, resolve)})
			}
		case catWh:
			f.Force = ForceQuestion
			switch tok {
			case "ne":
				f.Wh = "what"
				// "ne zaman" asks when; the bare "ne" asks what.
				if i+1 < len(tokens) && tokens[i+1] == "zaman" {
					f.Wh = "when"
					i++
				}
			case "nedir":
				f.Wh = "what"
				f.Action = Term{Phrase: "be"}
				verbSeen = true
			case "hangi":
				f.Wh = "which"
			case "kim":
				f.Wh = "who"
			case "neden", "niye":
				f.Wh = "why"
			case "nerede":
				f.Wh = "where"
			case "nasıl":
				f.Wh = "how"
			}
			flush()
		case catCnt:
			f.Counting = true
			f.Force = ForceQuestion
			flush()
		case catAdv:
			f.Habit = true
			flush()
		case catNeg:
			f.Negate = true
			flush()
		case catDet, catFn:
			flush()
		default:
			current = append(current, tok)
		}
	}
	flush()

	for _, sp := range spans {
		text := sp.text
		if !sp.postVerb {
			// A pre-verb span may still wear its case from a question
			// like "make test'i kim çalıştırdı".
			words := strings.Fields(text)
			words[len(words)-1] = stripCaseTR(words[len(words)-1])
			text = strings.Join(words, " ")
		}
		term := resolveTerm(text, resolve)
		if f.Patient.Empty() {
			f.Patient = term
		} else if f.Agent.Empty() {
			f.Agent = term
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

// Vowel harmony. The last written vowel decides: twofold harmony picks
// a or e (locative, ablative), fourfold picks the high vowel that
// matches in backness and rounding (accusative, the -DIr copula).
var vowelsTR = map[rune]struct {
	back, round bool
}{
	'a': {true, false}, 'ı': {true, false}, 'o': {true, true}, 'u': {true, true},
	'e': {false, false}, 'i': {false, false}, 'ö': {false, true}, 'ü': {false, true},
}

func lastVowelTR(w string) (rune, bool) {
	var last rune
	found := false
	for _, r := range strings.ToLower(w) {
		if _, ok := vowelsTR[r]; ok {
			last = r
			found = true
		}
	}
	return last, found
}

func harmony2(w string) string {
	v, ok := lastVowelTR(w)
	if ok && vowelsTR[v].back {
		return "a"
	}
	return "e"
}

func harmony4(w string) string {
	v, ok := lastVowelTR(w)
	if !ok {
		return "i"
	}
	switch {
	case vowelsTR[v].back && vowelsTR[v].round:
		return "u"
	case vowelsTR[v].back:
		return "ı"
	case vowelsTR[v].round:
		return "ü"
	default:
		return "i"
	}
}

// endsVoicelessTR reports the voicing of the final consonant, which
// assimilates the D of -DE, -DEn, and -DIr to t.
func endsVoicelessTR(w string) bool {
	r := []rune(strings.ToLower(w))
	if len(r) == 0 {
		return false
	}
	switch r[len(r)-1] {
	case 'f', 's', 't', 'k', 'ç', 'ş', 'h', 'p':
		return true
	}
	return false
}

func endsVowelTR(w string) bool {
	r := []rune(strings.ToLower(w))
	if len(r) == 0 {
		return false
	}
	_, ok := vowelsTR[r[len(r)-1]]
	return ok
}

// dOrT picks the suffix-initial consonant by assimilation.
func dOrT(w string) string {
	if endsVoicelessTR(w) {
		return "t"
	}
	return "d"
}

// applySuffixTR realizes a suffix recipe against a stem. The recipe is
// the parameter; this interpreter is the innate floor: D assimilates to
// d or t by the stem's final voicing, A takes twofold vowel harmony, I
// fourfold, a leading (y) is the buffer consonant after a vowel, and
// any other letter is literal.
func applySuffixTR(w, recipe string) string {
	var b strings.Builder
	rest := recipe
	if cut, ok := strings.CutPrefix(rest, "(y)"); ok {
		if endsVowelTR(w) {
			b.WriteString("y")
		}
		rest = cut
	}
	for _, r := range rest {
		switch r {
		case 'D':
			b.WriteString(dOrT(w))
		case 'A':
			b.WriteString(harmony2(w))
		case 'I':
			b.WriteString(harmony4(w))
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

// ablativeTR builds "from X". Foreign names take the apostrophe Turkish
// writes before suffixes on proper nouns: "make test'ten".
func ablativeTR(p *Params, w string) string {
	return w + "'" + applySuffixTR(w, p.get().suffix("ablative", "DAn"))
}

// accusativeTR builds the definite object, buffer y after a vowel:
// "make test'i", "elma'yı".
func accusativeTR(p *Params, w string) string {
	return w + "'" + applySuffixTR(w, p.get().suffix("accusative", "(y)I"))
}

// copulaTR attaches -DIr to a (Turkish) predicate noun: "eylemdir",
// "şeydir". No apostrophe: these are GOLEM's own words.
func copulaTR(p *Params, w string) string {
	return w + applySuffixTR(w, p.get().suffix("copula", "DIr"))
}

// The digits as spoken, for harmonizing a suffix onto a clock time:
// "09:14'te" because "dört" ends voiceless and front.
var digitWordTR = [10]string{"sıfır", "bir", "iki", "üç", "dört", "beş", "altı", "yedi", "sekiz", "dokuz"}

// timeLocativeTR suffixes "at" onto a digit-final time by the spoken
// form of its last digit.
func timeLocativeTR(p *Params, formatted string) string {
	r := []rune(formatted)
	last := r[len(r)-1]
	if last < '0' || last > '9' {
		return formatted + "'de"
	}
	spoken := digitWordTR[last-'0']
	return formatted + "'" + applySuffixTR(spoken, p.get().suffix("locative", "DA"))
}

// The Turkish verb forms GOLEM speaks: aorist for habits, its negative,
// the simple past, and the potential for hedging. Curriculum for the
// verb:tr nodes.
type verbTR struct {
	aorist, aoristNeg, past, pastNeg, potential, hedgedPast string
}

func finiteVerbTR(p *Params, base string, past, negate, hedge bool) string {
	v := p.verbTurkish(base)
	switch {
	case hedge && past:
		return v.hedgedPast
	case hedge:
		return v.potential
	case negate && past:
		return v.pastNeg
	case negate:
		return v.aoristNeg
	case past:
		return v.past
	default:
		return v.aorist
	}
}

func numberWordTR(n int) string {
	words := []string{"sıfır", "bir", "iki", "üç", "dört", "beş",
		"altı", "yedi", "sekiz", "dokuz", "on"}
	if n >= 0 && n < len(words) {
		return words[n]
	}
	return strconv.Itoa(n)
}

// qualPhraseTR renders one temporal adjunct, anchor first: "make
// test'ten sonra".
func qualPhraseTR(p *Params, q Qualifier) string {
	rel := "sonra"
	if q.Rel == "before" {
		rel = "önce"
	}
	return ablativeTR(p, q.Anchor.Speak()) + " " + rel
}

// realizeTurkish runs the grammar forward under the Turkish parameters:
// adjuncts front, the verb closes the clause, and morphology carries
// what English carries with word order and auxiliaries.
func (p *Params) realizeTurkish(f Frame, chem limbic.State) string {
	var b strings.Builder
	hedge := chem.Oxytocin < limbic.OxytocinBaseline || f.Hedge

	switch f.Action.Phrase {
	case "follow", "precede", "happen", "run", "fail":
		var parts []string
		for _, q := range f.Quals {
			if !q.Anchor.Empty() {
				parts = append(parts, qualPhraseTR(p, q))
			}
		}
		if len(parts) > 0 {
			b.WriteString(strings.Join(parts, " ve "))
			b.WriteString(" ")
		}
		if f.Habit && !f.Negate && !hedge {
			b.WriteString("genellikle ")
		}
		if f.Last {
			b.WriteString("en son ")
		}
		if !f.Agent.Empty() {
			b.WriteString(f.Agent.Speak())
			b.WriteString(" ")
		}
		if f.At != "" {
			b.WriteString(f.At)
			b.WriteString(" ")
		}
		b.WriteString(finiteVerbTR(p, f.Action.Phrase, f.Past, f.Negate, hedge))
	case "know":
		// "Make test'ten sonra ne gelir, henüz bilmiyorum."
		var gap []string
		for _, q := range f.Quals {
			if !q.Anchor.Empty() {
				gap = append(gap, qualPhraseTR(p, q)+" ne gelir")
			}
		}
		switch {
		case len(gap) > 0:
			b.WriteString(strings.Join(gap, " ve "))
			b.WriteString(", henüz bilmiyorum")
		case !f.Patient.Empty() && f.Patient.Speak() != "why" && f.Patient.Speak() != "who":
			b.WriteString(accusativeTR(p, f.Patient.Speak()))
			b.WriteString(" henüz bilmiyorum")
		default:
			b.WriteString("henüz bilmiyorum")
		}
	case "be":
		if f.Force == ForceQuestion {
			return sentence(f.Patient.Speak()+" nedir", "?")
		}
		b.WriteString(f.Agent.Speak())
		b.WriteString(" ")
		words := strings.Fields(f.Patient.Speak())
		if len(words) > 0 {
			words[len(words)-1] = copulaTR(p, words[len(words)-1])
		}
		b.WriteString(strings.Join(words, " "))
	case "name":
		// "testler, make test demek."
		b.WriteString(f.Agent.Speak())
		b.WriteString(", ")
		b.WriteString(f.Patient.Speak())
		b.WriteString(" demek")
		if f.Hedge {
			b.WriteString(" olabilir")
		}
	case "see":
		object := accusativeTR(p, f.Patient.Speak())
		if f.Patient.Speak() == "it" {
			object = "onu"
		}
		b.WriteString(object)
		b.WriteString(" ")
		b.WriteString(numberWordTR(f.Times))
		b.WriteString(" kez gördüm")
	case "propose":
		b.WriteString(f.Patient.Speak())
		b.WriteString(" öneririm")
	}

	mark := "."
	if f.Force == ForceQuestion {
		mark = "?"
	}
	return sentence(b.String(), mark)
}
