package grammar

import (
	"strconv"
	"strings"
)

// English inflection, hand-rolled. The realizer never bolts an "-s" onto
// a verb or an "s" onto a noun: agreement is computed from person,
// number, and tense against the tables here, and the irregulars live
// beside the rules. The vocabulary is GOLEM's own speech, so the tables
// are small and exact rather than large and approximate.

// verbEN is one verb's principal parts. Part is the past participle,
// which the perfect and the hedged past build on.
type verbEN struct {
	base, third, past, part string
}

// finiteVerb conjugates a verb for its subject: third person singular
// present takes the -s form, everything else in the present takes the
// base, and the past ignores person the way English does.
func finiteVerb(p *Params, base string, firstPerson, plural, past bool) string {
	v := p.verb(base)
	if past {
		return v.past
	}
	if firstPerson || plural {
		return v.base
	}
	return v.third
}

// VerbPhrase realizes the predicate's verb group a construction places in
// a slot, so the pattern can live in memory while the morphology stays
// innate. The mode picks the aspect: "habit" prefixes the adverb onto the
// finite form ("usually follows"), "hedge" keeps the modal ("may follow"),
// and "plain" is the bare finite. Succession subjects are third person, so
// only number bends agreement here.
func (p *Params) VerbPhrase(lemma string, plural, past bool, mode string) string {
	p = p.get()
	switch mode {
	case "hedge":
		return hedgedVerb(p, lemma, past)
	case "habit":
		return "usually " + finiteVerb(p, lemma, false, plural, past)
	default:
		return finiteVerb(p, lemma, false, plural, past)
	}
}

// negatedVerb denies a predicate with do-support, agreement carried by
// the auxiliary: "does not follow", "do not know", "did not follow".
func negatedVerb(p *Params, base string, firstPerson, plural, past bool) string {
	aux := "does"
	switch {
	case past:
		aux = "did"
	case firstPerson || plural:
		aux = "do"
	}
	return aux + " not " + verbBase(p, base)
}

// hedgedVerb keeps a tentative claim tentative in both tenses: "may
// follow" for the open present, "may have followed" for the settled
// past.
func hedgedVerb(p *Params, base string, past bool) string {
	if past {
		return "may have " + pastPart(p, base)
	}
	return "may " + verbBase(p, base)
}

func verbBase(p *Params, base string) string {
	return p.verb(base).base
}

func pastPart(p *Params, base string) string {
	return p.verb(base).part
}

// beForm is the copula, the one verb whose agreement survives the past.
func beForm(firstPerson, plural, past bool) string {
	switch {
	case past && plural:
		return "were"
	case past:
		return "was"
	case plural:
		return "are"
	case firstPerson:
		return "am"
	default:
		return "is"
	}
}

// pluralIrregularEN are the noun plurals no rule reaches.
var pluralIrregularEN = map[string]string{
	"child":  "children",
	"person": "people",
	"man":    "men",
	"woman":  "women",
	"foot":   "feet",
	"tooth":  "teeth",
	"mouse":  "mice",
	"datum":  "data",
	"sheep":  "sheep",
	"series": "series",
}

// pluralEN inflects a noun for number: the irregular table first, then
// the spelling rules (sibilant -es, consonant-y -ies, -f/-fe -ves),
// then plain -s.
func pluralEN(noun string) string {
	if p, ok := pluralIrregularEN[noun]; ok {
		return p
	}
	switch {
	case strings.HasSuffix(noun, "s"), strings.HasSuffix(noun, "x"),
		strings.HasSuffix(noun, "z"), strings.HasSuffix(noun, "ch"),
		strings.HasSuffix(noun, "sh"):
		return noun + "es"
	case strings.HasSuffix(noun, "y") && len(noun) > 1 && !isVowelLetter(rune(noun[len(noun)-2])):
		return noun[:len(noun)-1] + "ies"
	case strings.HasSuffix(noun, "fe"):
		return noun[:len(noun)-2] + "ves"
	case strings.HasSuffix(noun, "f"):
		return noun[:len(noun)-1] + "ves"
	default:
		return noun + "s"
	}
}

func isVowelLetter(r rune) bool {
	switch r {
	case 'a', 'e', 'i', 'o', 'u':
		return true
	}
	return false
}

// Words whose spelling and sound disagree about the article: vowel
// letters said with a consonant glide take "a", silent-h words take
// "an". Matched by prefix so "user" covers "users".
var aDespiteVowel = []string{"user", "unit", "uniform", "unique", "unive", "usual", "one", "once", "euro", "ewe"}
var anDespiteConsonant = []string{"hour", "honest", "honor", "honour", "heir"}

// Indefinite chooses a or an by the following sound, never by the
// letter alone, and returns the article with its noun phrase.
func Indefinite(phrase string) string {
	w := strings.ToLower(strings.TrimSpace(phrase))
	if w == "" {
		return phrase
	}
	art := "a"
	if isVowelLetter(rune(w[0])) {
		art = "an"
	}
	for _, p := range aDespiteVowel {
		if strings.HasPrefix(w, p) {
			art = "a"
		}
	}
	for _, p := range anDespiteConsonant {
		if strings.HasPrefix(w, p) {
			art = "an"
		}
	}
	return art + " " + phrase
}

// numberWord speaks a small count the way prose does and falls back to
// digits where prose would too.
func numberWord(n int) string {
	words := []string{"zero", "one", "two", "three", "four", "five",
		"six", "seven", "eight", "nine", "ten", "eleven", "twelve"}
	if n >= 0 && n < len(words) {
		return words[n]
	}
	return strconv.Itoa(n)
}

// JoinAnd coordinates a list the way a clause needs it: "a", "a and b",
// "a, b, and c".
func JoinAnd(items []string) string {
	switch len(items) {
	case 0:
		return ""
	case 1:
		return items[0]
	case 2:
		return items[0] + " and " + items[1]
	default:
		return strings.Join(items[:len(items)-1], ", ") + ", and " + items[len(items)-1]
	}
}
