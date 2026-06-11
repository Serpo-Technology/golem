package main

import (
	"strconv"
	"strings"
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

// The Turkish closed class. Categories reuse the innate set; the SOV
// parser reads them with the anchor on the other side of the relator.
var turkishCategory = map[string]string{
	"ne": catWh, "hangi": catWh, "kim": catWh, "neden": catWh, "niye": catWh,
	"nerede": catWh, "nasıl": catWh, "nedir": catWh,
	"sonra": catRel, "önce": catRel,
	"kaç": catCnt,
	"kez": catFn, "kere": catFn, "defa": catFn, "zaman": catFn,
	"genellikle": catAdv, "hep": catAdv, "bazen": catAdv,
	"asla": catNeg, "hiç": catNeg, "değil": catNeg,
	"olur": catHap, "oluyor": catHap, "oldu": catHap,
	"gelir": catHap, "geliyor": catHap, "geldi": catHap,
	"bir": catDet, "ve": catFn,
	"mi": catFn, "mı": catFn, "mu": catFn, "mü": catFn,
}

// Turkish sortals, the "komut" of "hangi komut": the same ontology
// vocabulary as the English set.
var turkishSortals = map[string]string{
	"komut": "command", "komutlar": "command",
	"eylem": "action", "eylemler": "action",
	"şey": "thing", "şeyler": "thing",
}

// turkishHappenVerbs carry the predicate and the tense, like their
// English counterparts.
var turkishHappenVerbs = map[string]struct {
	base string
	past bool
}{
	"olur": {"follow", false}, "oluyor": {"follow", false}, "oldu": {"follow", true},
	"gelir": {"follow", false}, "geliyor": {"follow", false}, "geldi": {"follow", true},
}

// detectTurkish reports whether an utterance is Turkish: any closed-class
// or sortal hit decides, since the two vocabularies share no forms.
func detectTurkish(tokens []string) bool {
	for _, tok := range tokens {
		if _, ok := turkishCategory[tok]; ok {
			return true
		}
		if _, ok := turkishSortals[tok]; ok {
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

// parseTurkish reads a Turkish utterance into the shared frame. SOV
// reverses the qualifier: the anchor span comes before its relator.
func parseTurkish(tokens []string, question bool, resolve func(string) *Fragment) Frame {
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
		if base, ok := turkishSortals[tok]; ok {
			f.Sort = base
			f.SortPl = tok != base
			flush()
			continue
		}
		if v, ok := turkishHappenVerbs[tok]; ok {
			flush()
			f.Action = Term{Phrase: v.base}
			if v.past {
				f.Past = true
			}
			verbSeen = true
			continue
		}
		switch turkishCategory[tok] {
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
		if f.Patient.empty() {
			f.Patient = term
		} else if f.Agent.empty() {
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

// ablativeTR builds "from X". Foreign names take the apostrophe Turkish
// writes before suffixes on proper nouns: "make test'ten".
func ablativeTR(w string) string {
	return w + "'" + dOrT(w) + harmony2(w) + "n"
}

// accusativeTR builds the definite object, buffer y after a vowel:
// "make test'i", "elma'yı".
func accusativeTR(w string) string {
	if endsVowelTR(w) {
		return w + "'y" + harmony4(w)
	}
	return w + "'" + harmony4(w)
}

// copulaTR attaches -DIr to a (Turkish) predicate noun: "eylemdir",
// "şeydir". No apostrophe: these are GOLEM's own words.
func copulaTR(w string) string {
	return w + dOrT(w) + harmony4(w) + "r"
}

// The digits as spoken, for harmonizing a suffix onto a clock time:
// "09:14'te" because "dört" ends voiceless and front.
var digitWordTR = [10]string{"sıfır", "bir", "iki", "üç", "dört", "beş", "altı", "yedi", "sekiz", "dokuz"}

// timeLocativeTR suffixes "at" onto a digit-final time by the spoken
// form of its last digit.
func timeLocativeTR(formatted string) string {
	r := []rune(formatted)
	last := r[len(r)-1]
	if last < '0' || last > '9' {
		return formatted + "'de"
	}
	spoken := digitWordTR[last-'0']
	return formatted + "'" + dOrT(spoken) + harmony2(spoken)
}

// The Turkish verb forms GOLEM speaks: aorist for habits, its negative,
// the simple past, and the potential for hedging.
type verbTR struct {
	aorist, aoristNeg, past, pastNeg, potential, hedgedPast string
}

var verbsTR = map[string]verbTR{
	"follow":  {"gelir", "gelmez", "geldi", "gelmedi", "gelebilir", "gelmiş olabilir"},
	"precede": {"gelir", "gelmez", "geldi", "gelmedi", "gelebilir", "gelmiş olabilir"},
	"happen":  {"olur", "olmaz", "oldu", "olmadı", "olabilir", "olmuş olabilir"},
}

func finiteVerbTR(base string, past, negate, hedge bool) string {
	v, ok := verbsTR[base]
	if !ok {
		v = verbsTR["happen"]
	}
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
func qualPhraseTR(q Qualifier) string {
	rel := "sonra"
	if q.Rel == "before" {
		rel = "önce"
	}
	return ablativeTR(q.Anchor.speak()) + " " + rel
}

// realizeTurkish runs the grammar forward under the Turkish parameters:
// adjuncts front, the verb closes the clause, and morphology carries
// what English carries with word order and auxiliaries.
func realizeTurkish(f Frame, chem LimbicState) string {
	var b strings.Builder
	hedge := chem.Oxytocin < oxytocinBaseline || f.Hedge

	switch f.Action.Phrase {
	case "follow", "precede", "happen", "run", "fail":
		var parts []string
		for _, q := range f.Quals {
			if !q.Anchor.empty() {
				parts = append(parts, qualPhraseTR(q))
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
		if !f.Agent.empty() {
			b.WriteString(f.Agent.speak())
			b.WriteString(" ")
		}
		if f.At != "" {
			b.WriteString(f.At)
			b.WriteString(" ")
		}
		b.WriteString(finiteVerbTR(f.Action.Phrase, f.Past, f.Negate, hedge))
	case "know":
		// "Make test'ten sonra ne gelir, henüz bilmiyorum."
		var gap []string
		for _, q := range f.Quals {
			if !q.Anchor.empty() {
				gap = append(gap, qualPhraseTR(q)+" ne gelir")
			}
		}
		switch {
		case len(gap) > 0:
			b.WriteString(strings.Join(gap, " ve "))
			b.WriteString(", henüz bilmiyorum")
		case !f.Patient.empty() && f.Patient.speak() != "why" && f.Patient.speak() != "who":
			b.WriteString(accusativeTR(f.Patient.speak()))
			b.WriteString(" henüz bilmiyorum")
		default:
			b.WriteString("henüz bilmiyorum")
		}
	case "be":
		if f.Force == ForceQuestion {
			return sentence(f.Patient.speak()+" nedir", "?")
		}
		b.WriteString(f.Agent.speak())
		b.WriteString(" ")
		words := strings.Fields(f.Patient.speak())
		if len(words) > 0 {
			words[len(words)-1] = copulaTR(words[len(words)-1])
		}
		b.WriteString(strings.Join(words, " "))
	case "name":
		// "testler, make test demek."
		b.WriteString(f.Agent.speak())
		b.WriteString(", ")
		b.WriteString(f.Patient.speak())
		b.WriteString(" demek")
		if f.Hedge {
			b.WriteString(" olabilir")
		}
	case "see":
		object := accusativeTR(f.Patient.speak())
		if f.Patient.speak() == "it" {
			object = "onu"
		}
		b.WriteString(object)
		b.WriteString(" ")
		b.WriteString(numberWordTR(f.Times))
		b.WriteString(" kez gördüm")
	case "propose":
		b.WriteString(f.Patient.speak())
		b.WriteString(" öneririm")
	}

	mark := "."
	if f.Force == ForceQuestion {
		mark = "?"
	}
	return sentence(b.String(), mark)
}
