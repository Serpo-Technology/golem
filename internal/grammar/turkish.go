package grammar

import (
	"strconv"
	"strings"

	"golem/internal/limbic"
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

// caseStems lists the base forms a case-marked span might reduce to: the
// literal case-stripped stem and, for Turkish, that stem with its final
// consonant hardened back from the softening the suffix applied ("köpeği" ->
// "köpeğ" -> "köpek"). The caller resolves each in turn and keeps the first
// that names a concept, so a stem that genuinely ends in the voiced consonant
// is reached by the literal form before the hardened candidate is tried.
func (p *Params) caseStems(lang, span string) []string {
	base := p.stripCase(lang, span)
	out := []string{base}
	if lang == "tr" {
		words := strings.Fields(base)
		if len(words) > 0 {
			if hard := unsoftenFinalTR(words[len(words)-1]); hard != words[len(words)-1] {
				words[len(words)-1] = hard
				out = append(out, strings.Join(words, " "))
			}
		}
	}
	return out
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
	s := surface
	// A finite past may carry a person suffix after the tense: -m (I), -n (you),
	// -k (we). Shed it when a past-tense suffix sits beneath, so an agreed verb
	// ("kovaladın", "kovaladım") reduces to the same stem as the bare third person.
	if r := []rune(s); len(r) > 4 {
		switch r[len(r)-1] {
		case 'm', 'n', 'k':
			if trimmed := string(r[:len(r)-1]); endsPastTR(trimmed) {
				s = trimmed
			}
		}
	}
	for _, suf := range []string{"dı", "di", "du", "dü", "tı", "ti", "tu", "tü"} {
		if strings.HasSuffix(s, suf) && len([]rune(s)) > 3 {
			return strings.TrimSuffix(s, suf)
		}
	}
	return s
}

// endsPastTR reports whether a form ends in the Turkish past-tense suffix -DI, the
// marker the verb stemmer strips, so a person suffix is shed only over a real past.
func endsPastTR(s string) bool {
	for _, suf := range []string{"dı", "di", "du", "dü", "tı", "ti", "tu", "tü"} {
		if strings.HasSuffix(s, suf) {
			return true
		}
	}
	return false
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

// polysyllabicTR reports whether a stem carries more than one vowel, the
// condition under which Turkish softens a final stop before a vowel suffix; a
// monosyllable keeps its stop ("at" -> "atı", not "adı").
func polysyllabicTR(w string) bool {
	n := 0
	for _, r := range strings.ToLower(w) {
		if _, ok := vowelsTR[r]; ok {
			n++
		}
	}
	return n > 1
}

// softenFinalTR voices a polysyllabic stem's final stop when a vowel attaches
// to it, Turkish consonant softening: p/ç/t/k -> b/c/d/ğ, so "köpek" before the
// accusative vowel surfaces "köpeğ" ("köpeği") and "kitap" "kitab" ("kitabı").
// A vowel-final or other-consonant-final stem, and any monosyllable, is left
// alone, as the rule spares them.
func softenFinalTR(w string) string {
	r := []rune(w)
	if len(r) == 0 || !polysyllabicTR(w) {
		return w
	}
	switch r[len(r)-1] {
	case 'p':
		r[len(r)-1] = 'b'
	case 'ç':
		r[len(r)-1] = 'c'
	case 't':
		r[len(r)-1] = 'd'
	case 'k':
		r[len(r)-1] = 'ğ'
	default:
		return w
	}
	return string(r)
}

// unsoftenFinalTR reverses softenFinalTR for comprehension, hardening a final
// voiced consonant back to the stop the suffix softened ("köpeğ" -> "köpek").
// It is offered as a candidate stem only, the caller resolving it against the
// lexicon, so a word that genuinely ends in the voiced consonant ("dağ") is
// unharmed when its literal stem resolves first.
func unsoftenFinalTR(w string) string {
	r := []rune(w)
	if len(r) == 0 {
		return w
	}
	switch r[len(r)-1] {
	case 'b':
		r[len(r)-1] = 'p'
	case 'c':
		r[len(r)-1] = 'ç'
	case 'd':
		r[len(r)-1] = 't'
	case 'ğ':
		r[len(r)-1] = 'k'
	default:
		return w
	}
	return string(r)
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

// accusativeCommonTR is the accusative of an ordinary (not proper) noun, so no
// apostrophe: "kedi" -> "kediyi", "fare" -> "fareyi", "köpek" -> "köpeği". When
// the vowel attaches straight onto a consonant-final stem, that stem's final
// stop softens; a vowel-final stem takes the buffer y and keeps its shape.
func accusativeCommonTR(p *Params, w string) string {
	recipe := p.get().suffix("accusative", "(y)I")
	stem := w
	if !endsVowelTR(w) {
		stem = softenFinalTR(w)
	}
	return stem + applySuffixTR(w, recipe)
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

// regularVerbTR conjugates a learned verb with no seeded paradigm by the
// regular rules: the past is the stem plus -DI ("kovala" -> "kovaladı"), the
// negative inserts -mA ("kovalamadı"), the aorist is -r after a vowel or -Ir
// otherwise ("kovalar"), and its negative is -mAz ("kovalamaz"). The suffix
// recipes are data; this runs the regular floor for a verb the curriculum never
// spelled principal parts for.
func regularVerbTR(p *Params, stem string, past, negate bool) string {
	if negate {
		stem += applySuffixTR(stem, p.suffix("verbneg", "mA"))
		if !past {
			return stem + "z" // aorist negative: -mAz
		}
	}
	if past {
		return stem + applySuffixTR(stem, p.suffix("verbpast", "DI"))
	}
	if endsVowelTR(stem) {
		return stem + "r"
	}
	return stem + applySuffixTR(stem, p.suffix("aorist", "Ir"))
}

// trPerson is the grammatical person a finite verb agrees with. The third person
// is the unmarked default the engine speaks for an ordinary subject; the others
// are the conversation participants, who carry an agreement suffix.
type trPerson int

const (
	trPerson3sg trPerson = iota
	trPerson1sg          // ben
	trPerson2sg          // sen
	trPerson1pl          // biz
)

// trPersonOf reads the person a subject's surface carries, so a Turkish past verb
// agrees with it: "ben" is first person, "sen" second, "biz" first plural, and
// anything else the bare third person the engine already speaks.
func (p *Params) trPersonOf(surface string) trPerson {
	switch strings.ToLower(strings.TrimSpace(surface)) {
	case p.surfaceWord("tr", "firstperson"):
		return trPerson1sg
	case p.surfaceWord("tr", "addressee"):
		return trPerson2sg
	case p.surfaceWord("tr", "firstpersonpl"):
		return trPerson1pl
	}
	return trPerson3sg
}

// pastPersonSuffixTR is the agreement suffix a Turkish past verb takes: -m (I),
// -n (you), -k (we). The third person is unmarked.
func pastPersonSuffixTR(person trPerson) string {
	switch person {
	case trPerson1sg:
		return "m"
	case trPerson2sg:
		return "n"
	case trPerson1pl:
		return "k"
	}
	return ""
}

// finiteVerbTR conjugates a verb and agrees a past form with its subject's person.
func finiteVerbTR(p *Params, base string, past, negate, hedge bool, person trPerson) string {
	form := finiteVerbFormTR(p, base, past, negate, hedge)
	if past && !hedge {
		form += pastPersonSuffixTR(person)
	}
	return form
}

func finiteVerbFormTR(p *Params, base string, past, negate, hedge bool) string {
	if _, seeded := p.verbsTR[base]; !seeded {
		return regularVerbTR(p, base, past, negate) // a learned verb: the regular paradigm
	}
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
		b.WriteString(finiteVerbTR(p, f.Action.Phrase, f.Past, f.Negate, hedge, p.trPersonOf(f.Agent.Speak())))
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
