package grammar

import (
	"sort"
	"strings"
	"sync"
)

// The engine is the innate floor and stays in code; its data lives in
// memory. A Params is the working copy of that data: the closed class
// and its categories, the sortal nouns, the event-verb forms, the
// verbs' principal parts, the word-order setting, and the suffix
// recipes, compiled from grammar nodes read out of the store. The
// compile is the cache: per-parse lookups are map hits, and the daemon
// recompiles at wake after the nightly competition has re-weighted the
// nodes.

// Param is one grammar parameter on its way between memory and the
// engine: the node's template is the key, its content the value, and
// its weight the entrenchment the competition reads.
type Param struct {
	Key      string // "fw:en:what", "verb:en:follow", "order:tr", "suffix:tr:ablative"
	Value    string
	Lang     string
	Category string
	Weight   float64
}

// hapEntry is one event-verb surface form: the predicate it fills and
// the tense it carries.
type hapEntry struct {
	base string
	past bool
}

// Params is the learned half of the grammar, compiled for the engine.
type Params struct {
	categories map[string]map[string]string   // lang -> form -> category
	sortals    map[string]map[string]string   // lang -> form -> kind
	haps       map[string]map[string]hapEntry // lang -> form -> predicate and tense
	verbs      map[string]verbEN              // base -> principal parts (en)
	verbsTR    map[string]verbTR              // base -> form set (tr)
	verbFinals map[string]bool                // lang -> the verb closes the clause (sov)
	suffixes   map[string]string              // name -> recipe (tr)
	// constructions holds the sentence patterns the planner fills, keyed
	// language then intent; set by SetConstructions after Compile.
	constructions map[string]map[string][]Construction
	// decl holds the declarative grammar the general engine runs, keyed by
	// language; set by SetDecl after Compile.
	decl map[string]*DeclGrammar
	// argcxns holds the argument-structure constructions the bidirectional
	// engine runs, keyed by language; set by SetArgConstructions.
	argcxns map[string][]ArgConstruction
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// parseHap unpacks the predicate and tense the seed packs into an event
// verb's category field, "<base>" or "<base>:past".
func parseHap(category string) hapEntry {
	base, tense, _ := strings.Cut(category, ":")
	return hapEntry{base: base, past: tense == "past"}
}

// Compile builds the engine's working copy from parameters read out of
// memory. lexMass carries each form's lived naming mass keyed
// "lang:form"; a seeded closed-class entry holds its slot only while it
// outweighs that mass, which is the weighted competition: grammar bends
// to sustained contrary usage with no special case.
func Compile(entries []Param, lexMass map[string]float64) *Params {
	p := &Params{
		categories: map[string]map[string]string{},
		sortals:    map[string]map[string]string{},
		haps:       map[string]map[string]hapEntry{},
		verbs:      map[string]verbEN{},
		verbsTR:    map[string]verbTR{},
		verbFinals: map[string]bool{},
		suffixes:   map[string]string{},
	}
	for _, e := range entries {
		kind, lang, name, ok := splitKey(e.Key)
		if !ok {
			continue
		}
		outweighed := lexMass[lang+":"+name] >= e.Weight
		switch kind {
		case "fw":
			if outweighed {
				continue
			}
			ensure(p.categories, lang)[name] = e.Category
		case "sortal":
			if outweighed {
				continue
			}
			ensure(p.sortals, lang)[name] = e.Category
		case "hap":
			if outweighed {
				continue
			}
			ensure(p.haps, lang)[name] = parseHap(e.Category)
		case "verb":
			switch lang {
			case "tr":
				parts := strings.Split(e.Value, "|")
				if len(parts) == 6 {
					p.verbsTR[name] = verbTR{parts[0], parts[1], parts[2], parts[3], parts[4], parts[5]}
				}
			default:
				parts := strings.Fields(e.Value)
				if len(parts) == 4 {
					p.verbs[name] = verbEN{parts[0], parts[1], parts[2], parts[3]}
				}
			}
		case "order":
			p.verbFinals[lang] = e.Value == "sov"
		case "suffix":
			p.suffixes[name] = e.Value
		}
	}
	return p
}

func splitKey(key string) (kind, lang, name string, ok bool) {
	parts := strings.SplitN(key, ":", 3)
	switch len(parts) {
	case 2: // "order:tr"
		return parts[0], parts[1], "", true
	case 3:
		return parts[0], parts[1], parts[2], true
	}
	return "", "", "", false
}

func ensure[V any](m map[string]map[string]V, lang string) map[string]V {
	inner, ok := m[lang]
	if !ok {
		inner = map[string]V{}
		m[lang] = inner
	}
	return inner
}

var builtinOnce = sync.OnceValue(func() *Params {
	return Compile(Curriculum(), nil).
		SetConstructions(Constructions()).
		SetDecl(Curriculum()).
		SetArgConstructions(Constructions())
})

// Builtin is the grammar compiled straight from the curriculum, for
// callers without a memory-backed copy: the package-level Parse and
// Realize, and the tests that pin engine behavior.
func Builtin() *Params {
	return builtinOnce()
}

// get makes a nil Params usable: the builtin seed is the default.
func (p *Params) get() *Params {
	if p == nil {
		return Builtin()
	}
	return p
}

// category resolves a form's closed-class category in a language, with
// event-verb forms reading as catHap.
func (p *Params) category(lang, form string) string {
	if _, ok := p.haps[lang][form]; ok {
		return catHap
	}
	return p.categories[lang][form]
}

func (p *Params) sortal(lang, form string) (string, bool) {
	base, ok := p.sortals[lang][form]
	return base, ok
}

func (p *Params) hap(lang, form string) (hapEntry, bool) {
	h, ok := p.haps[lang][form]
	return h, ok
}

// isFunctionWord reports whether a form belongs to the working closed
// class, the set ContentTokens keeps out of the lexicon.
func (p *Params) isFunctionWord(form string) bool {
	return p.category("en", form) != ""
}

// verb returns a verb's principal parts, regular by rule when memory
// holds no entry.
func (p *Params) verb(base string) verbEN {
	if v, ok := p.verbs[base]; ok {
		return v
	}
	return regularVerbEN(base)
}

// verbTurkish returns a Turkish verb's form set, falling back to the
// generic event verb when memory holds no entry.
func (p *Params) verbTurkish(base string) verbTR {
	if v, ok := p.verbsTR[base]; ok {
		return v
	}
	if v, ok := p.verbsTR["happen"]; ok {
		return v
	}
	return verbTR{}
}

// verbFinal reports a language's word-order setting: true when the
// verb closes the clause (SOV).
func (p *Params) verbFinal(lang string) bool {
	return p.verbFinals[lang]
}

// suffix returns a suffix recipe by name, with the curriculum shape as
// the fallback.
func (p *Params) suffix(name, fallback string) string {
	if r, ok := p.suffixes[name]; ok {
		return r
	}
	return fallback
}

// IsContentWord reports whether a surface form is an open-class word in a
// language: long enough to carry content and outside the closed class. The
// comprehension engine reads it to pick which tokens fill role slots,
// keeping the determiners and the auxiliaries out.
func (p *Params) IsContentWord(lang, form string) bool {
	p = p.get()
	return len(form) >= 2 && p.category(lang, form) == ""
}

// Lemma recovers a verb's base form from an inflected surface, exposing the
// reverse morphology so a caller outside the package can key a relation on
// one lemma across its inflections.
func (p *Params) Lemma(lang, surface string) string {
	return p.get().verbLemma(lang, surface)
}

// VerbPast reads an inflected verb's tense, exposing the same judgement
// comprehension uses so a caller can settle the tense of an answer it
// generates from a question's verb.
func (p *Params) VerbPast(lang, surface string) bool {
	return p.get().verbPast(lang, surface)
}

// ContentTokens returns the distinct open-class words of an utterance,
// the ones eligible to become word nodes and point at concepts.
func (p *Params) ContentTokens(s string) []string {
	p = p.get()
	tokens, _ := Tokenize(s)
	seen := map[string]bool{}
	var out []string
	for _, tok := range tokens {
		if len(tok) < 2 || p.isFunctionWord(tok) || seen[tok] {
			continue
		}
		seen[tok] = true
		out = append(out, tok)
	}
	return out
}
