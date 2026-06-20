package grammar

import "strings"

// Learned morphology: an inflection pattern induced from examples rather than coded. The
// engine's English rules (pastEN/thirdPersonEN) and Turkish harmony are innate; this is the
// seed of morphology as LEARNED constructions -- a feature paired with an affix, induced
// from a few (base, inflected) pairs the way functions are induced from example pairs, and
// applied by rule so it generalizes to a held-out word. A pattern here is a suffix
// substitution; vowel harmony (a suffix that bends to the stem's vowel) is the next slice,
// and the prefix scan is byte-wise, enough for the ASCII patterns this slice learns.

// inflection is a suffix substitution: from a base, delete the trailing del and append add.
type inflection struct {
	del, add string
}

// induceInflection learns the one suffix substitution that maps every (base, inflected)
// pair, the standard longest-common-prefix morphology learner: each pair's transform is to
// delete the base's tail past the shared prefix and append the inflected's tail. It is kept
// only when one rule fits all pairs (grounded by reproducing each), else declined -- the
// Occam guard against a set no single regular rule explains.
func induceInflection(pairs [][2]string) (inflection, bool) {
	if len(pairs) == 0 {
		return inflection{}, false
	}
	var rule inflection
	for i, pair := range pairs {
		base, infl := pair[0], pair[1]
		if base == "" || infl == "" {
			return inflection{}, false
		}
		n := commonPrefixLen(base, infl)
		r := inflection{del: base[n:], add: infl[n:]}
		if i == 0 {
			rule = r
		} else if r != rule {
			return inflection{}, false
		}
	}
	for _, pair := range pairs {
		if out, ok := applyInflection(rule, pair[0]); !ok || out != pair[1] {
			return inflection{}, false
		}
	}
	return rule, true
}

// applyInflection rewrites a base by the rule: strip the trailing del, append add. A base
// that does not end in del is outside the rule's domain.
func applyInflection(rule inflection, base string) (string, bool) {
	if !strings.HasSuffix(base, rule.del) {
		return "", false
	}
	return base[:len(base)-len(rule.del)] + rule.add, true
}

// commonPrefixLen is the length in bytes of the longest shared prefix of a and b.
func commonPrefixLen(a, b string) int {
	n := 0
	for n < len(a) && n < len(b) && a[n] == b[n] {
		n++
	}
	return n
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
