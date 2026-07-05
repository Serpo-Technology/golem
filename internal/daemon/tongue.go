package daemon

import (
	"golem/internal/grammar"
)

// sniffTurkish reports whether an untagged clause that closed-class detection
// could not place reads as Turkish, from grounded evidence rather than a
// language table: a token whose word node lives only in the Turkish tongue, a
// case-marked form whose stem resolves to a known concept, or an inflected
// verb whose Turkish stem names a known relation. A clause with no such
// evidence keeps the English default, so a fresh brain is unchanged until its
// Turkish lexicon has actually grown.
func (d *Daemon) sniffTurkish(content string) bool {
	tokens, _ := grammar.Tokenize(content)
	if len(tokens) == 0 {
		return false
	}
	if words, err := d.store.WordNodesByForms(tokens); err == nil {
		hasTR, hasOther := map[string]bool{}, map[string]bool{}
		for _, w := range words {
			if w.Lang == "tr" {
				hasTR[w.Content] = true
			} else {
				hasOther[w.Content] = true
			}
		}
		for form := range hasTR {
			if !hasOther[form] {
				return true
			}
		}
	}
	for _, t := range tokens {
		for _, base := range d.gram.CaseStems("tr", t) {
			if base != t && d.resolveConceptLang(base, "tr") != nil {
				return true
			}
		}
		if lemma := d.gram.Lemma("tr", t); lemma != t && d.resolveRelation(lemma) != nil {
			return true
		}
	}
	return false
}
