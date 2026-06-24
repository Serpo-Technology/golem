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
	caseLangs  map[string]bool                // lang -> roles come from case, not position (free order)
	cases      map[string]map[string]string   // lang -> case name -> ending (la)
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
	// surface holds the words a tongue renders an answer's surface with: the
	// definite marker, the coordinator, the pronoun a given referent takes by
	// role, the pro-verbs. Keyed lang then name, so the engine reads them rather
	// than carrying any in its code.
	surface map[string]map[string]string
	// learnedInfl holds inflection patterns induced from examples rather than
	// coded, keyed "lang:feature" (the seed of morphology as learned constructions).
	learnedInfl map[string]inflection
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
		categories:  map[string]map[string]string{},
		sortals:     map[string]map[string]string{},
		haps:        map[string]map[string]hapEntry{},
		verbs:       map[string]verbEN{},
		verbsTR:     map[string]verbTR{},
		verbFinals:  map[string]bool{},
		caseLangs:   map[string]bool{},
		cases:       map[string]map[string]string{},
		suffixes:    map[string]string{},
		surface:     map[string]map[string]string{},
		learnedInfl: map[string]inflection{},
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
			p.caseLangs[lang] = e.Value == "free"
		case "case":
			ensure(p.cases, lang)[name] = e.Value
		case "suffix":
			p.suffixes[name] = e.Value
		case "surface":
			ensure(p.surface, lang)[name] = e.Value
		case "infl":
			if rule, ok := decodeInflection(e.Value); ok {
				p.learnedInfl[lang+":"+name] = rule
			}
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
// event-verb forms reading as catHap. A category may carry a colon-suffixed
// refinement, "wh:what" naming the interrogative's type; the broad class
// before the colon is what classification reads, the refinement riding in a
// dedicated accessor the way an event verb's tense does.
func (p *Params) category(lang, form string) string {
	if _, ok := p.haps[lang][form]; ok {
		return catHap
	}
	cat := p.categories[lang][form]
	if i := strings.IndexByte(cat, ':'); i >= 0 {
		return cat[:i]
	}
	return cat
}

// doSupportQuestion reports whether the clause opens with do-support inversion:
// the auxiliary "do" before a deictic subject ("do you want the rose"), which
// forms a yes/no question rather than the command the bare imperative "do" opens
// ("do the dishes"). Only "do" is ambiguous this way; "does" and "did" are filler
// the polar path reads directly. It needs at least an auxiliary, a subject, and a
// verb, so a bare "do you" is not taken for one.
func (p *Params) doSupportQuestion(lang string, tokens []string) bool {
	if len(tokens) < 3 || strings.ToLower(tokens[0]) != "do" {
		return false
	}
	return p.category(lang, tokens[1]) == catDeixis
}

// relType reads a temporal relator's direction. English relators name their
// own ("after", "before"); a language whose relator is a different form
// carries the direction in the seed, "rel:after" for Turkish "sonra", so the
// mapping is data and not a parser fork.
func (p *Params) relType(lang, form string) string {
	if spec, ok := strings.CutPrefix(p.categories[lang][form], "rel:"); ok && spec != "" {
		return spec
	}
	return form
}

// speechMove reads the discourse move a speech-act verb carries from its seeded
// category, "speech:clarify" for "mean" and "speech:recall" for "say", so a verb
// about the talk routes to its move from data rather than a string match. A bare
// "speech" carries none.
func (p *Params) speechMove(lang, form string) string {
	if spec, ok := strings.CutPrefix(p.categories[lang][form], "speech:"); ok {
		return spec
	}
	return ""
}

// propertyName reads the relation property a predicate names from its seeded
// category, "property:transitive" for "transitive", so a clause predicating a
// rule of a relation routes to that property from data rather than a string
// match. A bare "property" carries none.
func (p *Params) propertyName(lang, form string) string {
	if spec, ok := strings.CutPrefix(p.categories[lang][form], "property:"); ok {
		return spec
	}
	return ""
}

// LocativePrep reports whether a form is a seeded locative preposition (on, in,
// under), the closed class whose use as a copula's complement marks its relation
// locative. The locative refinement rides on the function-word category ("fn:
// locative"), so the broad class the parser reads is still "fn" and the word is
// filled and flushed exactly as before; only this accessor reads the refinement.
func (p *Params) LocativePrep(lang, form string) bool {
	p = p.get()
	if spec, ok := strings.CutPrefix(p.categories[lang][form], "fn:"); ok {
		return spec == "locative"
	}
	return false
}

// TopicPrep reports whether a form is a seeded topic preposition (Latin "de"), the closed
// class that names the thing a clause is about, so its object fills the patient whatever its
// position. The refinement rides on the function-word category ("fn:topic"), so the broad
// class the parser reads is still "fn"; only this accessor reads the refinement.
func (p *Params) TopicPrep(lang, form string) bool {
	p = p.get()
	if spec, ok := strings.CutPrefix(p.categories[lang][form], "fn:"); ok {
		return spec == "topic"
	}
	return false
}

// Existential reports whether a form is the seeded existential that closes a
// Turkish location or possession clause ("var"), the word that stands in for the
// copula English would use ("there is", "X has").
func (p *Params) Existential(lang, form string) bool {
	p = p.get()
	return p.categories[lang][form] == "exist"
}

// Postposition reads the English preposition a Turkish spatial postposition maps to,
// "üstünde" -> "on" from its seeded "postp:on" category, so the precise place a
// postposition names joins the same relation its English preposition does. The empty
// string and false mean the form is no postposition.
func (p *Params) Postposition(lang, form string) (string, bool) {
	p = p.get()
	if spec, ok := strings.CutPrefix(p.categories[lang][form], "postp:"); ok {
		return spec, true
	}
	return "", false
}

// PostpositionFor returns the Turkish postposition that names a preposition's place,
// "on" -> "üstünde", for generation. The empty string and false mean the tongue has
// no postposition for it (so the general locative case is used instead).
func (p *Params) PostpositionFor(lang, prep string) (string, bool) {
	p = p.get()
	want := "postp:" + prep
	for form, cat := range p.categories[lang] {
		if cat == want {
			return form, true
		}
	}
	return "", false
}

// proAnimacy reads a pro-form's animacy preference from its seeded category,
// "pro:inanimate" for "it" and "pro:animate" for "she", so a pronoun's
// binding favors the referent it fits. A bare "pro" carries none.
func (p *Params) proAnimacy(lang, form string) string {
	if spec, ok := strings.CutPrefix(p.categories[lang][form], "pro:"); ok {
		return spec
	}
	return ""
}

// whInfo reads an interrogative's type and whether it carries the copula.
// English wh words name their own type, so "what" is read from the surface;
// a language whose interrogative is a different form carries the type in the
// seed, "wh:what" for Turkish "ne" and "wh:what:be" for the copular "nedir",
// so the translation lives in data and not in a parser fork.
func (p *Params) whInfo(lang, form string) (whType string, copula bool) {
	spec, ok := strings.CutPrefix(p.categories[lang][form], "wh:")
	if !ok {
		return form, false
	}
	parts := strings.Split(spec, ":")
	whType = parts[0]
	if whType == "" {
		whType = form
	}
	copula = len(parts) > 1 && parts[1] == "be"
	return whType, copula
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
	return p.isFunctionWordLang("en", form)
}

// isFunctionWordLang is isFunctionWord aware of the utterance's tongue: a form
// is closed class when it is one in that tongue or in English, so a Turkish
// particle ("mı") is kept out of the lexicon when a Turkish clause is tokenized
// rather than minted as a content word for want of an English category. Without
// this a question particle accrues naming mass on every asking until it
// outweighs its own seed and turns into a spurious noun.
func (p *Params) isFunctionWordLang(lang, form string) bool {
	if p.category(lang, form) != "" {
		return true
	}
	// English is the bootstrap tongue, so its closed class is the fallback filter
	// when the utterance is English or its tongue was not detected. For a known
	// other tongue, that tongue's closed class is authoritative: a content word
	// that happens to spell an English function word ("at", a Turkish horse) is
	// not filtered out of its own language.
	if lang == "" || lang == "en" {
		return p.category("en", form) != ""
	}
	return false
}

// IsFunctionWord exposes the closed-class test to callers outside the grammar, so
// the daemon can tell a content word it might mint as a concept from a determiner
// or particle it must not.
func (p *Params) IsFunctionWord(lang, form string) bool {
	return p.get().isFunctionWordLang(lang, form)
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

// caseDriven reports whether a language assigns thematic roles by case rather than
// by position, so word order is free (Latin). Comprehension reads the case endings
// for the roles instead of the slot layout.
func (p *Params) caseDriven(lang string) bool {
	return p.caseLangs[lang]
}

// caseEnding returns the seeded ending a language marks a case with ("m" for the
// Latin first-declension accusative), empty when the case is unmarked (nominative)
// or unseeded.
func (p *Params) caseEnding(lang, name string) string {
	return p.cases[lang][name]
}

// surfaceWord reads a tongue's surface word by name, empty when the tongue
// seeds none, so the engine and the generator carry no answer surface of their
// own.
func (p *Params) surfaceWord(lang, name string) string {
	return p.get().surface[lang][name]
}

// Article returns a tongue's definite marker, the word an answer prepends to a
// definite noun phrase. Empty when the tongue marks definiteness otherwise (the
// Turkish accusative), so nothing is prepended.
func (p *Params) Article(lang string) string { return p.surfaceWord(lang, "article") }

// Coordinator returns the word a tongue joins coordinated co-arguments with.
func (p *Params) Coordinator(lang string) string { return p.surfaceWord(lang, "coord") }

// Pronoun returns the pronoun a referent already in play takes in a role, by
// role first ("pronoun.agent") then the tongue's default ("pronoun").
func (p *Params) Pronoun(lang, role string) string {
	if pr := p.surfaceWord(lang, "pronoun."+role); pr != "" {
		return pr
	}
	return p.surfaceWord(lang, "pronoun")
}

// ProVerbs returns a tongue's generic pro-verbs, the verbs of "what did X do".
func (p *Params) ProVerbs(lang string) []string {
	return strings.Fields(p.surfaceWord(lang, "proverb"))
}

// Word returns a tongue's surface word by name (yes, no, ...), so a caller
// reads the word from the tongue's data rather than spelling it.
func (p *Params) Word(lang, name string) string { return p.surfaceWord(lang, name) }

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

// IsDeictic reports whether a surface form is a deictic pronoun in a language.
// It names a conversation participant a clause can be about ("you chased the
// cat"), so a caller scanning for a clause's arguments keeps it where the
// closed-class filter of IsContentWord would drop it.
func (p *Params) IsDeictic(lang, form string) bool {
	return p.get().category(lang, form) == catDeixis
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

// StemCandidates lists the base forms an inflected surface might reduce to: the
// verb lemma and, for a cased tongue, the case stems (with a softened stop
// hardened). The surface itself is excluded. The minting path tries these to
// reinforce a known base rather than store a fresh node per inflection, so the
// lexicon keys on the lemma; resolving each candidate filters the wrong
// reductions, since a mangled stem names nothing.
func (p *Params) StemCandidates(lang, surface string) []string {
	p = p.get()
	seen := map[string]bool{surface: true}
	var out []string
	add := func(s string) {
		if s != "" && !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	add(p.verbLemma(lang, surface))
	for _, s := range p.caseStems(lang, surface) {
		add(s)
	}
	return out
}

// ContentTokens returns the distinct open-class words of an utterance,
// the ones eligible to become word nodes and point at concepts.
func (p *Params) ContentTokens(s string) []string {
	return p.ContentTokensLang("en", s)
}

// ContentTokensLang is ContentTokens for an utterance in a known tongue, so the
// tongue's closed class is filtered too. The minting path passes the told
// language; the default keeps English for callers that have no tongue in hand.
func (p *Params) ContentTokensLang(lang, s string) []string {
	p = p.get()
	tokens, _ := Tokenize(s)
	seen := map[string]bool{}
	var out []string
	for _, tok := range tokens {
		if len(tok) < 2 || p.isFunctionWordLang(lang, tok) || seen[tok] {
			continue
		}
		seen[tok] = true
		out = append(out, tok)
	}
	return out
}
