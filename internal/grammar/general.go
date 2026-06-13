package grammar

import "strings"

// A general realization engine: one combination primitive, feature-
// structure unification, over declarative rules read from memory. Where
// the procedural realizer bakes English agreement and word order into
// code, this shrinks the innate floor to a single language-agnostic
// operation and moves the linguistic knowledge into inspectable nodes. A
// phrase rule names its daughters' surface order (the word order) and the
// features that flow between them (the agreement); a lexical entry gives a
// lemma its category and its inflected forms. The engine unifies, selects,
// and linearizes, recursing to any depth, and fails to the simpler engine
// rather than emit a broken sentence.

// fs is an atomic feature structure: a flat map of feature to value, which
// is all the agreement here needs (category, number). Unification is the
// one general operation, so a clause is built by merging the feature
// structures of its parts and a conflict is a failure, not a guess. It is
// an alias for a plain map, so a caller outside the package builds one
// without naming an internal type.
type fs = map[string]string

func copyFS(a fs) fs {
	if a == nil {
		return fs{}
	}
	out := make(fs, len(a))
	for k, v := range a {
		out[k] = v
	}
	return out
}

// unifyFS merges two feature structures, failing when they disagree on a
// shared feature. This is the whole engine's decision procedure.
func unifyFS(a, b fs) (fs, bool) {
	out := copyFS(a)
	for k, v := range b {
		if cur, ok := out[k]; ok && cur != v {
			return nil, false
		}
		out[k] = v
	}
	return out, true
}

// featFlow is one agreement constraint: a feature value copied from one
// daughter to another, the verb taking its number from the subject. It is
// declarative data, read from a rule node, not a line of code.
type featFlow struct{ feat, from, to string }

// synRule is one phrase rule: the surface order of its daughter roles (the
// word order), the features that flow between them (the agreement), and
// which daughter's features become the phrase's own (the head). All three
// are read from a memory node, so editing the node edits the grammar.
type synRule struct {
	order []string
	flows []featFlow
	head  string
}

// lexEntry is one lexical item: its category and its surface forms keyed
// by the agreement value that selects them ("sg", "pl", or "any").
type lexEntry struct {
	cat   string
	forms map[string]string
}

// DeclGrammar is a language's declarative grammar compiled for the engine:
// its phrase rules and its lexicon, both read from memory nodes.
type DeclGrammar struct {
	rules map[string]synRule
	lex   map[string]lexEntry
}

// GenNode is a node of the meaning tree the engine realizes: a phrase
// (Rule set, Kids filled by role) or a lexical leaf (Lemma set). Feats
// carries the features the meaning imposes, such as the number of a
// subject, which agreement then flows to the verb.
type GenNode struct {
	Rule  string
	Lemma string
	Feats fs
	Kids  map[string]*GenNode
}

// Leaf builds a lexical node with optional imposed features.
func Leaf(lemma string, feats fs) *GenNode {
	return &GenNode{Lemma: lemma, Feats: feats}
}

// Phrase builds a phrase node from a rule name and its role-to-child
// bindings, with optional imposed features.
func Phrase(rule string, feats fs, kids map[string]*GenNode) *GenNode {
	return &GenNode{Rule: rule, Feats: feats, Kids: kids}
}

// Generate realizes a meaning tree through a language's declarative
// grammar, returning the surface string and the trace of rules and lexical
// items that built it, leaves first, so the sentence reads back as the
// structures that made it. It returns false when the grammar cannot build
// the tree, so the caller falls back to the procedural realizer.
func (p *Params) Generate(lang string, root *GenNode) (string, []string, bool) {
	g := p.get().decl[lang]
	if g == nil || root == nil {
		return "", nil, false
	}
	surface, _, trace, ok := g.gen(root)
	return surface, trace, ok
}

// gen is the recursive core: unify the node's imposed features into a
// lexical form at a leaf, or generate a phrase by resolving agreement
// flows, generating each daughter, and linearizing by the word order.
func (g *DeclGrammar) gen(n *GenNode) (string, fs, []string, bool) {
	if n == nil {
		return "", nil, nil, false
	}
	if n.Rule == "" {
		e, ok := g.lex[n.Lemma]
		if !ok {
			return "", nil, nil, false
		}
		feats, ok := unifyFS(fs{"cat": e.cat}, n.Feats)
		if !ok {
			return "", nil, nil, false
		}
		form := e.forms[feats["num"]]
		if form == "" {
			form = e.forms["any"]
		}
		if form == "" {
			return "", nil, nil, false
		}
		return form, feats, []string{"lex:" + n.Lemma}, true
	}

	r, ok := g.rules[n.Rule]
	if !ok {
		return "", nil, nil, false
	}

	// The head daughter inherits the phrase's own imposed features, so a
	// plural noun phrase makes its head noun plural.
	inject := map[string]fs{}
	if r.head != "" && len(n.Feats) > 0 {
		inject[r.head] = copyFS(n.Feats)
	}

	// Agreement is resolved before word order: a daughter is generated only
	// once every feature flowing into it has its source, then the surface is
	// laid out in the rule's order. Separating the two is what lets word
	// order be edited without disturbing agreement.
	done := map[string]fs{}
	surf := map[string]string{}
	var trace []string
	pending := append([]string{}, r.order...)
	for len(pending) > 0 {
		var still []string
		progressed := false
		for _, role := range pending {
			ready := true
			for _, fl := range r.flows {
				if fl.to == role {
					if _, got := done[fl.from]; !got {
						ready = false
					}
				}
			}
			if !ready {
				still = append(still, role)
				continue
			}
			kid := n.Kids[role]
			if kid == nil {
				return "", nil, nil, false
			}
			feats := copyFS(kid.Feats)
			if inj, ok := inject[role]; ok {
				if merged, ok := unifyFS(feats, inj); ok {
					feats = merged
				} else {
					return "", nil, nil, false
				}
			}
			for _, fl := range r.flows {
				if fl.to == role && done[fl.from][fl.feat] != "" {
					if merged, ok := unifyFS(feats, fs{fl.feat: done[fl.from][fl.feat]}); ok {
						feats = merged
					} else {
						return "", nil, nil, false
					}
				}
			}
			child := &GenNode{Rule: kid.Rule, Lemma: kid.Lemma, Feats: feats, Kids: kid.Kids}
			s, f, tr, ok := g.gen(child)
			if !ok {
				return "", nil, nil, false
			}
			done[role], surf[role] = f, s
			trace = append(trace, tr...)
			progressed = true
		}
		if !progressed {
			return "", nil, nil, false // a cyclic agreement flow has no resolution
		}
		pending = still
	}

	parts := make([]string, 0, len(r.order))
	for _, role := range r.order {
		parts = append(parts, surf[role])
	}
	return strings.Join(parts, " "), done[r.head], append(trace, "syn:"+n.Rule), true
}

// SetDecl installs the declarative grammar compiled from the syn and lex
// parameters, returning the same Params for chaining onto Compile. A syn
// node's value is "order | flows | head"; a lex node's is
// "cat=<c>;<agr>=<form>;...". Both ride the same grammar nodes as the rest
// of the language faculty.
func (p *Params) SetDecl(entries []Param) *Params {
	decl := map[string]*DeclGrammar{}
	ensureDecl := func(lang string) *DeclGrammar {
		g := decl[lang]
		if g == nil {
			g = &DeclGrammar{rules: map[string]synRule{}, lex: map[string]lexEntry{}}
			decl[lang] = g
		}
		return g
	}
	for _, e := range entries {
		kind, lang, name, ok := splitKey(e.Key)
		if !ok || name == "" {
			continue
		}
		switch kind {
		case "syn":
			ensureDecl(lang).rules[name] = parseSynRule(e.Value)
		case "lex":
			ensureDecl(lang).lex[name] = parseLexEntry(e.Value)
		}
	}
	p.decl = decl
	return p
}

// parseSynRule reads a phrase rule from its node value: the daughter order,
// the agreement flows, and the head role (defaulting to the last daughter).
func parseSynRule(value string) synRule {
	sections := strings.Split(value, "|")
	r := synRule{}
	if len(sections) > 0 {
		r.order = strings.Fields(sections[0])
	}
	if len(sections) > 1 {
		for _, f := range strings.Split(sections[1], ",") {
			f = strings.TrimSpace(f)
			if f == "" {
				continue
			}
			feat, route, ok := strings.Cut(f, ":")
			if !ok {
				continue
			}
			from, to, ok := strings.Cut(route, ">")
			if !ok {
				continue
			}
			r.flows = append(r.flows, featFlow{feat: feat, from: strings.TrimSpace(from), to: strings.TrimSpace(to)})
		}
	}
	if len(sections) > 2 && strings.TrimSpace(sections[2]) != "" {
		r.head = strings.TrimSpace(sections[2])
	} else if len(r.order) > 0 {
		r.head = r.order[len(r.order)-1]
	}
	return r
}

// parseLexEntry reads a lexical item from its node value: its category and
// its inflected forms keyed by the agreement value that selects each.
func parseLexEntry(value string) lexEntry {
	e := lexEntry{forms: map[string]string{}}
	for _, field := range strings.Split(value, ";") {
		field = strings.TrimSpace(field)
		if field == "" {
			continue
		}
		key, val, ok := strings.Cut(field, "=")
		if !ok {
			continue
		}
		if key == "cat" {
			e.cat = val
			continue
		}
		e.forms[key] = val
	}
	return e
}
