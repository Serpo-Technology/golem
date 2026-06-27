package daemon

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"golem/internal/grammar"
	"golem/internal/memory"
)

// The declarative grammar is the general realization engine's knowledge: phrase rules naming a
// clause's daughter order and the agreement that flows between them, read from seed strings. The
// engine itself (one unification primitive over a meaning tree) is the irreducible floor; these
// edges move the rules it reads into the graph, the form-meaning counterpart of the argument-structure
// constructions. A phrase rule concept carries one ordered slot per daughter (each linked to its role),
// a head edge, and a flow node per agreement constraint (its feature the content, its source and target
// roles the edges). The seed stays the floor: the graph is seeded from it at boot and the derive rewrites
// each rule's form pole from the graph in place, the graph authoritative, a disagreement logged.
const declGrammarCategory = "declgrammar"

const (
	declSlotRel     = "declslot"     // a phrase rule to one of its ordered daughter slots
	declRoleRel     = "declrole"     // a slot to the role it stands for (subj, vp, obj)
	declHeadRel     = "declhead"     // a phrase rule to the daughter whose features become its own
	declFlowRel     = "declflow"     // a phrase rule to one of its agreement-flow nodes
	declFlowFromRel = "declflowfrom" // a flow to the daughter the feature flows from
	declFlowToRel   = "declflowto"   // a flow to the daughter the feature flows to

	declCatRel  = "declcat"  // a word node to its realizer category (cop, aux, pron)
	declFormRel = "declform" // a word node to one of its inflected form nodes
	declKeyRel  = "declkey"  // a form node to the agreement value that selects it (sg, pl, past)
)

// declSyn is a phrase rule's form pole as the daemon reads and writes it: the daughter order, the
// agreement flows, and the head. It is the daemon-side mirror of the grammar engine's synRule, kept
// here so the dissolution works at the seed-string level without touching the grammar package.
type declSyn struct {
	order []string
	flows []declFlow
	head  string
}

// declFlow is one agreement constraint: a feature copied from one daughter to another.
type declFlow struct{ feat, from, to string }

// declLex is a lexical entry's form pole as the daemon reads and writes it: the realizer category
// and the inflected forms keyed by the agreement value that selects each. It rides on the word's own
// node (a function word's fw node, or a created lexeme node), so a word's forms live with the word.
type declLex struct {
	cat   string
	forms map[string]string
}

// ensureDeclGrammar represents each seed phrase rule as a concept carrying its form pole in the graph,
// then registers every declarative-grammar concept the graph holds so the exclusion maps and the
// enumeration cover the seeds and any taught since. It is seeded from the curriculum that stays the
// floor, idempotent and self-healing at boot, and runs before the compile so the derive reads each rule
// from the graph. The concepts sit outside the grown layer.
func (d *Daemon) ensureDeclGrammar() error {
	d.declRules = map[uint64]string{}
	d.declParts = map[uint64]bool{}
	gnodes, err := d.store.NodesByKind(memory.KindGrammar)
	if err != nil {
		return err
	}
	byTemplate := make(map[string]*memory.Fragment, len(gnodes))
	for _, n := range gnodes {
		byTemplate[n.Template] = n
	}
	for _, p := range grammar.Curriculum() {
		parts := strings.SplitN(p.Key, ":", 3)
		if len(parts) != 3 {
			continue
		}
		lang, name := parts[1], parts[2]
		switch parts[0] {
		case "syn":
			if _, err := d.buildDeclSynConcept(lang, name, parseDeclSyn(p.Value)); err != nil {
				return err
			}
		case "lex":
			// A function word's forms ride on its own fw node, so a word's inflection lives with the
			// word. A lex entry with no fw node keeps its seed floor: a content example (dog, chase)
			// whose forms the open-vocabulary minter derives, or a load-bearing orphan (be, may) given
			// a created lexeme node in a later step.
			if host := byTemplate["fw:"+lang+":"+name]; host != nil {
				if err := d.buildDeclLexOnNode(host.ID, lang, name, parseDeclLex(p.Value)); err != nil {
					return err
				}
			}
		}
	}
	return d.registerGraphDeclGrammar()
}

// buildDeclLexOnNode writes a word's lex pole onto its node: a realizer-category edge and an inflected
// form node per agreement value, each form node carrying the surface form and linked to the value that
// selects it through a shared key vocabulary. It is the find-or-create used to seed the forms and (later)
// to teach them, so a word's forms live with the word.
func (d *Daemon) buildDeclLexOnNode(hostID uint64, lang, name string, lx declLex) error {
	if cid := d.ensureDeclNode("declcat:"+lx.cat, lx.cat, ""); cid != 0 {
		if err := d.store.LinkRel(hostID, cid, declCatRel, 1); err != nil {
			return err
		}
	}
	for key, surface := range lx.forms {
		fid := d.ensureDeclNode("lexform:"+lang+":"+name+"/"+key, surface, lang)
		if fid == 0 {
			continue
		}
		if err := d.store.LinkRel(hostID, fid, declFormRel, 1); err != nil {
			return err
		}
		if kid := d.ensureDeclNode("declkey:"+key, key, ""); kid != 0 {
			if err := d.store.LinkRel(fid, kid, declKeyRel, 1); err != nil {
				return err
			}
		}
	}
	return nil
}

// buildDeclSynConcept writes one phrase rule's form pole into the graph: a concept holding an ordered
// slot per daughter (each linked to its role through a shared role vocabulary), a head edge, and a flow
// node per agreement constraint. It is the find-or-create used both to seed the rules and to teach a new
// one, so a rule learned into the graph is built the same way the seed ones are.
func (d *Daemon) buildDeclSynConcept(lang, name string, r declSyn) (uint64, error) {
	role := func(value string) uint64 {
		if value == "" {
			return 0
		}
		return d.ensureDeclNode("declrole:"+value, value, "")
	}
	rid := d.ensureDeclNode("syn:"+lang+":"+name, name, lang)
	if rid == 0 {
		return 0, nil
	}
	for i, slot := range r.order {
		sid := d.ensureDeclNode(fmt.Sprintf("syn:%s:%s/%d", lang, name, i), strconv.Itoa(i), lang)
		if sid == 0 {
			continue
		}
		if err := d.store.LinkRel(rid, sid, declSlotRel, 1); err != nil {
			return rid, err
		}
		if vid := role(slot); vid != 0 {
			if err := d.store.LinkRel(sid, vid, declRoleRel, 1); err != nil {
				return rid, err
			}
		}
	}
	if vid := role(r.head); vid != 0 {
		if err := d.store.LinkRel(rid, vid, declHeadRel, 1); err != nil {
			return rid, err
		}
	}
	for j, f := range r.flows {
		fid := d.ensureDeclNode(fmt.Sprintf("syn:%s:%s/flow/%d", lang, name, j), f.feat, lang)
		if fid == 0 {
			continue
		}
		if err := d.store.LinkRel(rid, fid, declFlowRel, 1); err != nil {
			return rid, err
		}
		if vid := role(f.from); vid != 0 {
			if err := d.store.LinkRel(fid, vid, declFlowFromRel, 1); err != nil {
				return rid, err
			}
		}
		if vid := role(f.to); vid != 0 {
			if err := d.store.LinkRel(fid, vid, declFlowToRel, 1); err != nil {
				return rid, err
			}
		}
	}
	return rid, nil
}

// ensureDeclNode is the find-or-create for a declarative-grammar scaffolding concept (a phrase rule, a
// slot, a flow, or a role vocabulary word), keyed by its template and held in the distinct
// "declgrammar" category so it cannot collide with a grown noun.
func (d *Daemon) ensureDeclNode(template, content, lang string) uint64 {
	if c, err := d.store.ConceptByTemplate(template); err == nil && c != nil {
		return c.ID
	}
	c := &memory.Fragment{Kind: memory.KindConcept, Channel: "concept", Content: content,
		Template: template, Category: declGrammarCategory, Lang: lang, Limbic: d.chem.Snapshot(), Time: time.Now()}
	if _, err := d.store.Encode(c); err != nil {
		fmt.Fprintf(d.out, "declarative grammar concept error: %v\n", err)
		return 0
	}
	return c.ID
}

// registerGraphDeclGrammar populates the exclusion maps from every declarative-grammar concept the
// graph holds: a phrase rule (template "syn:<lang>:<name>") into declRules keyed by its template, and a
// slot, flow, or role vocabulary node into declParts. So a rule taught into the graph is discovered at
// boot, kept out of the grown layer, and reached by the enumeration, alongside the seeded ones.
func (d *Daemon) registerGraphDeclGrammar() error {
	concepts, err := d.store.Concepts()
	if err != nil {
		return err
	}
	for _, c := range concepts {
		if c.Category != declGrammarCategory {
			continue
		}
		if (strings.HasPrefix(c.Template, "syn:") || strings.HasPrefix(c.Template, "lex:")) && !strings.Contains(c.Template, "/") {
			d.declRules[c.ID] = c.Template
		} else {
			d.declParts[c.ID] = true
		}
	}
	return nil
}

// deriveDeclGrammar rewrites each phrase rule parameter's form pole from the rule concept the graph
// holds for it, so the engine reads the daughter order, the agreement flows, and the head from the
// graph rather than the seed string. A rule with no concept in the graph (a brain that does not
// represent it yet) keeps its stored value, the seed standing as the floor. While the seed still
// carries the form pole, a derived pole that disagrees is logged as a tripwire that the graph and the
// seed have parted; the graph is authoritative. It finds the concept by template in the current store,
// so it stays correct when a restore swaps the brain under the daemon. A grammar node and its parameter
// line up by index (paramsOf preserves order).
func (d *Daemon) deriveDeclGrammar(gnodes []*memory.Fragment, gparams []grammar.Param) {
	if len(d.declRules) == 0 {
		return
	}
	g := d.storeView()
	for i, n := range gnodes {
		if !strings.HasPrefix(n.Template, "syn:") {
			continue
		}
		c, err := d.store.ConceptByTemplate(n.Template)
		if err != nil || c == nil {
			continue // not represented yet: the stored rule stands
		}
		value := serializeDeclSyn(d.declSynFromGraph(g, c.ID))
		if value != serializeDeclSyn(parseDeclSyn(gparams[i].Value)) {
			fmt.Fprintf(d.out, "declgrammar mismatch: %s graph %q vs seed %q\n", n.Template, value, gparams[i].Value)
		}
		gparams[i].Value = value
	}
}

// declSynFromGraph reads a phrase rule back out of its subgraph: the daughters in rank order, the head,
// and the agreement flows. It is the inverse of buildDeclSynConcept, walked through the same node-and-edge
// view the code generation reads.
func (d *Daemon) declSynFromGraph(g codeGraph, id uint64) declSyn {
	r := declSyn{}
	type ranked struct {
		rank int
		role string
	}
	var slots []ranked
	for _, sid := range g.dests(id, declSlotRel) {
		rank, _ := strconv.Atoi(g.content(sid))
		role := ""
		if rr := g.dests(sid, declRoleRel); len(rr) == 1 {
			role = g.content(rr[0])
		}
		slots = append(slots, ranked{rank, role})
	}
	sort.Slice(slots, func(i, j int) bool { return slots[i].rank < slots[j].rank })
	for _, s := range slots {
		r.order = append(r.order, s.role)
	}
	if h := g.dests(id, declHeadRel); len(h) == 1 {
		r.head = g.content(h[0])
	}
	for _, fid := range g.dests(id, declFlowRel) {
		f := declFlow{feat: g.content(fid)}
		if ff := g.dests(fid, declFlowFromRel); len(ff) == 1 {
			f.from = g.content(ff[0])
		}
		if tt := g.dests(fid, declFlowToRel); len(tt) == 1 {
			f.to = g.content(tt[0])
		}
		r.flows = append(r.flows, f)
	}
	return r
}

// parseDeclSyn reads a phrase rule from its seed value ("order | flows | head"), the daemon-side mirror
// of the grammar engine's parseSynRule, so the graph is seeded from the same string the engine compiles.
func parseDeclSyn(value string) declSyn {
	sections := strings.Split(value, "|")
	r := declSyn{}
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
			r.flows = append(r.flows, declFlow{feat: feat, from: strings.TrimSpace(from), to: strings.TrimSpace(to)})
		}
	}
	if len(sections) > 2 && strings.TrimSpace(sections[2]) != "" {
		r.head = strings.TrimSpace(sections[2])
	} else if len(r.order) > 0 {
		r.head = r.order[len(r.order)-1]
	}
	return r
}

// serializeDeclSyn writes a phrase rule back into the seed's "order | flows | head" form, the inverse of
// parseDeclSyn, so the form pole derived from the graph feeds the same compile the seed string did. The
// flows are sorted into a canonical order, so the comparison the derive makes is structural, not
// sensitive to the graph's edge iteration order or the seed's incidental whitespace.
func serializeDeclSyn(r declSyn) string {
	fs := append([]declFlow{}, r.flows...)
	sort.Slice(fs, func(i, j int) bool {
		if fs[i].feat != fs[j].feat {
			return fs[i].feat < fs[j].feat
		}
		if fs[i].from != fs[j].from {
			return fs[i].from < fs[j].from
		}
		return fs[i].to < fs[j].to
	})
	flows := make([]string, 0, len(fs))
	for _, f := range fs {
		flows = append(flows, f.feat+":"+f.from+">"+f.to)
	}
	return strings.Join(r.order, " ") + " | " + strings.Join(flows, ",") + " | " + r.head
}

// deriveDeclLex rewrites each lexical parameter's form pole from the forms the word's own node carries,
// so the realizer reads a word's category and inflected forms from the graph rather than the seed string.
// A lex entry whose word node carries no forms (a content example the minter derives, or an orphan not yet
// given a node) keeps its stored value, the seed standing as the floor. The host node is found by template
// in the current grammar nodes, so it stays correct when a restore swaps the brain. A grammar node and its
// parameter line up by index (paramsOf preserves order); a derived pole that disagrees logs a tripwire.
func (d *Daemon) deriveDeclLex(gnodes []*memory.Fragment, gparams []grammar.Param) {
	byTemplate := make(map[string]*memory.Fragment, len(gnodes))
	for _, n := range gnodes {
		byTemplate[n.Template] = n
	}
	g := d.storeView()
	for i, n := range gnodes {
		parts := strings.SplitN(n.Template, ":", 3)
		if len(parts) != 3 || parts[0] != "lex" {
			continue
		}
		host := byTemplate["fw:"+parts[1]+":"+parts[2]]
		if host == nil {
			continue // no word node carries the forms: the seed value stands as the floor
		}
		lx := d.declLexFromGraph(g, host.ID)
		if lx.cat == "" || len(lx.forms) == 0 {
			continue // the node does not carry the lex pole yet
		}
		value := serializeDeclLex(lx)
		if value != serializeDeclLex(parseDeclLex(gparams[i].Value)) {
			fmt.Fprintf(d.out, "declgrammar lex mismatch: %s graph %q vs seed %q\n", n.Template, value, gparams[i].Value)
		}
		gparams[i].Value = value
	}
}

// declLexFromGraph reads a word's lex pole back off its node: the realizer category and the inflected
// forms keyed by their agreement value. It is the inverse of buildDeclLexOnNode, walked through the same
// node-and-edge view the rest of the dissolution reads.
func (d *Daemon) declLexFromGraph(g codeGraph, hostID uint64) declLex {
	lx := declLex{forms: map[string]string{}}
	if c := g.dests(hostID, declCatRel); len(c) == 1 {
		lx.cat = g.content(c[0])
	}
	for _, fid := range g.dests(hostID, declFormRel) {
		key := ""
		if k := g.dests(fid, declKeyRel); len(k) == 1 {
			key = g.content(k[0])
		}
		if key != "" {
			lx.forms[key] = g.content(fid)
		}
	}
	return lx
}

// parseDeclLex reads a lexical entry from its seed value ("cat=<c>;<key>=<form>;..."), the daemon-side
// mirror of the grammar engine's parseLexEntry, so the word's node is seeded from the same string the
// engine compiles.
func parseDeclLex(value string) declLex {
	lx := declLex{forms: map[string]string{}}
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
			lx.cat = val
			continue
		}
		lx.forms[key] = val
	}
	return lx
}

// serializeDeclLex writes a lexical entry back into the seed's "cat=<c>;<key>=<form>;..." form, the
// inverse of parseDeclLex, with the forms in a canonical key order so the comparison the derive makes is
// structural rather than sensitive to map iteration or the seed's field order.
func serializeDeclLex(lx declLex) string {
	keys := make([]string, 0, len(lx.forms))
	for k := range lx.forms {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys)+1)
	parts = append(parts, "cat="+lx.cat)
	for _, k := range keys {
		parts = append(parts, k+"="+lx.forms[k])
	}
	return strings.Join(parts, ";")
}
