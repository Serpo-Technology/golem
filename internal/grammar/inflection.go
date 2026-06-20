package grammar

import "strings"

// Learned morphology: an inflection pattern induced from examples rather than coded. The
// engine's English rules (pastEN) and Turkish harmony are innate; this is the seed of
// morphology as LEARNED constructions -- a feature paired with an affix, induced from a few
// (base, inflected) pairs the way functions are induced from example pairs, and applied by
// rule so it generalizes to a held-out word.
//
// A pattern is either a fixed suffix substitution {del, add} or a HARMONIC one: a suffix
// whose vowel bends to the stem's last vowel (the Turkish -DI past: kovaladı / geldi /
// okudu / gördü). The vowel FEATURE table is seeded phonetics, the floor; the harmony RULE
// (a consonant frame around a vowel that agrees in backness, and rounding for the 4-way
// high vowels) is what is LEARNED from the examples. The work is rune-aware, since the
// vowels it harmonizes are multibyte. Consonant assimilation (the d/t of bak->baktı) is a
// further refinement; this slice harmonizes the vowel.

// inflection is a suffix substitution: delete the trailing del, then append either the fixed
// add or, when harmonic, frameBefore + the harmonizing vowel + frameAfter.
type inflection struct {
	del string
	add string

	harmonic    bool
	frameBefore string
	frameAfter  string
	kind        string // "high" (4-way ı/i/u/ü) or "low" (2-way a/e)
}

// vowelFeat is a vowel's harmony features; the table is seeded phonetics, the floor the
// learned rule stands on.
type vowelFeat struct{ back, round bool }

var vowelFeats = map[rune]vowelFeat{
	'a': {true, false}, 'ı': {true, false}, 'o': {true, true}, 'u': {true, true},
	'e': {false, false}, 'i': {false, false}, 'ö': {false, true}, 'ü': {false, true},
}

// harmonyVowel is the suffix vowel a stem vowel calls for: a high vowel matching its
// backness and rounding (4-way), or a low vowel matching only its backness (2-way).
func harmonyVowel(kind string, f vowelFeat) rune {
	if kind == "low" {
		if f.back {
			return 'a'
		}
		return 'e'
	}
	switch {
	case f.back && f.round:
		return 'u'
	case f.back:
		return 'ı'
	case f.round:
		return 'ü'
	default:
		return 'i'
	}
}

// lastVowel is the features of a stem's last vowel, and whether it has one.
func lastVowel(stem string) (vowelFeat, bool) {
	var last vowelFeat
	found := false
	for _, r := range stem {
		if f, ok := vowelFeats[r]; ok {
			last, found = f, true
		}
	}
	return last, found
}

// runePrefix and runeSuffix are the longest shared prefix and suffix, rune-aware so a
// multibyte vowel is never split.
func runePrefix(a, b string) string {
	ar, br := []rune(a), []rune(b)
	n := 0
	for n < len(ar) && n < len(br) && ar[n] == br[n] {
		n++
	}
	return string(ar[:n])
}

func runeSuffix(a, b string) string {
	ar, br := []rune(a), []rune(b)
	n := 0
	for n < len(ar) && n < len(br) && ar[len(ar)-1-n] == br[len(br)-1-n] {
		n++
	}
	return string(ar[len(ar)-n:])
}

// induceInflection learns the one pattern that maps every (base, inflected) pair: a fixed
// suffix when the added tails agree, else a harmonic suffix when they differ only in a vowel
// that agrees with each stem's last vowel. It is kept only when one rule reproduces all
// pairs, else declined -- the Occam guard against a set no single rule explains.
func induceInflection(pairs [][2]string) (inflection, bool) {
	if len(pairs) == 0 {
		return inflection{}, false
	}
	dels := make([]string, len(pairs))
	adds := make([]string, len(pairs))
	for i, pair := range pairs {
		base, infl := pair[0], pair[1]
		if base == "" || infl == "" {
			return inflection{}, false
		}
		p := []rune(runePrefix(base, infl))
		dels[i] = string([]rune(base)[len(p):])
		adds[i] = string([]rune(infl)[len(p):])
	}
	for i := 1; i < len(dels); i++ {
		if dels[i] != dels[0] {
			return inflection{}, false // the stem edit differs; no single rule
		}
	}
	del := dels[0]

	fixed := true
	for i := 1; i < len(adds); i++ {
		if adds[i] != adds[0] {
			fixed = false
			break
		}
	}
	if fixed {
		rule := inflection{del: del, add: adds[0]}
		if grounds(rule, pairs) {
			return rule, true
		}
		return inflection{}, false
	}
	return induceHarmonic(del, adds, pairs)
}

// induceHarmonic learns a suffix whose vowel harmonizes: the added tails share a consonant
// frame around exactly one vowel, and that vowel is what each stem's last vowel calls for
// (high or low). It tries the 4-way high harmony first, then the 2-way low.
func induceHarmonic(del string, adds []string, pairs [][2]string) (inflection, bool) {
	before, after := adds[0], adds[0]
	for _, a := range adds[1:] {
		before = runePrefix(before, a)
		after = runeSuffix(after, a)
	}
	bn, an := len([]rune(before)), len([]rune(after))
	for _, kind := range []string{"high", "low"} {
		fits := true
		for i, a := range adds {
			ar := []rune(a)
			if len(ar) != bn+1+an { // exactly one rune between the frames
				fits = false
				break
			}
			v := ar[bn]
			if _, isVowel := vowelFeats[v]; !isVowel {
				fits = false
				break
			}
			stemF, has := lastVowel(pairs[i][0])
			if !has || v != harmonyVowel(kind, stemF) {
				fits = false
				break
			}
		}
		if fits {
			rule := inflection{del: del, harmonic: true, frameBefore: before, frameAfter: after, kind: kind}
			if grounds(rule, pairs) {
				return rule, true
			}
		}
	}
	return inflection{}, false
}

// grounds checks a rule reproduces every example, the "ground by running" the function
// induction uses.
func grounds(rule inflection, pairs [][2]string) bool {
	for _, pair := range pairs {
		if out, ok := applyInflection(rule, pair[0]); !ok || out != pair[1] {
			return false
		}
	}
	return true
}

// applyInflection rewrites a base by the rule: strip the trailing del, then append the fixed
// suffix or the harmonic one (its vowel chosen by the stem's last vowel). A base outside the
// rule's domain (wrong ending, or no vowel for a harmonic suffix) fails.
func applyInflection(rule inflection, base string) (string, bool) {
	if !strings.HasSuffix(base, rule.del) {
		return "", false
	}
	br := []rune(base)
	stem := string(br[:len(br)-len([]rune(rule.del))])
	if !rule.harmonic {
		return stem + rule.add, true
	}
	f, has := lastVowel(stem)
	if !has {
		return "", false
	}
	return stem + rule.frameBefore + string(harmonyVowel(rule.kind, f)) + rule.frameAfter, true
}

// LearnInflection induces an inflection pattern for a language and feature from example
// pairs and stores it, returning whether one was learned.
func (p *Params) LearnInflection(lang, feature string, pairs [][2]string) bool {
	p = p.get()
	rule, ok := induceInflection(pairs)
	if !ok {
		return false
	}
	if p.learnedInfl == nil {
		p.learnedInfl = map[string]inflection{}
	}
	p.learnedInfl[lang+":"+feature] = rule
	return true
}

// Inflect applies the learned inflection for a language and feature to a base, false when
// none is learned or the base falls outside the rule's domain.
func (p *Params) Inflect(lang, feature, base string) (string, bool) {
	p = p.get()
	rule, ok := p.learnedInfl[lang+":"+feature]
	if !ok {
		return "", false
	}
	return applyInflection(rule, base)
}

// encode serializes a rule as a grammar node's value, tagged and pipe-delimited so a fixed
// and a harmonic suffix both round-trip through memory.
func (r inflection) encode() string {
	if r.harmonic {
		return strings.Join([]string{"harm", r.del, r.frameBefore, r.frameAfter, r.kind}, "|")
	}
	return strings.Join([]string{"fixed", r.del, r.add}, "|")
}

// decodeInflection reads a stored rule back, false when the value is not one.
func decodeInflection(s string) (inflection, bool) {
	parts := strings.Split(s, "|")
	switch {
	case len(parts) == 3 && parts[0] == "fixed":
		return inflection{del: parts[1], add: parts[2]}, true
	case len(parts) == 5 && parts[0] == "harm" && (parts[4] == "high" || parts[4] == "low"):
		return inflection{del: parts[1], harmonic: true, frameBefore: parts[2], frameAfter: parts[3], kind: parts[4]}, true
	}
	return inflection{}, false
}

// InduceInflection induces a rule from example pairs and returns its stored form, false when
// no single rule fits. The daemon writes the value under an "infl:<lang>:<feature>" grammar
// node, so a taught inflection becomes a fluid weight that recompiles into the grammar.
func InduceInflection(pairs [][2]string) (string, bool) {
	rule, ok := induceInflection(pairs)
	if !ok {
		return "", false
	}
	return rule.encode(), true
}
