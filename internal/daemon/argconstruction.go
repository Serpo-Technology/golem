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

// The argument-structure constructions (transitive, intransitive) are the bidirectional
// form-meaning pairings Speak and Comprehend both walk. They live in the grammar as seed-loaded
// structs; these edges represent each one as a concept carrying its form pole in the graph, the
// form-meaning counterpart of the category-class concepts. A construction concept has one ordered
// slot per argument; each slot links to its role, syntactic type, and feature-preference word, and
// the construction marks which role is the predicate. The seed stays the floor: the graph is
// seeded from it at boot and the derive shadows the graph against the compiled constructions, so
// the representation is proven before it is made the source.
const (
	cxnSlotRel   = "cxnslot"   // a construction to one of its ordered argument slots
	cxnRoleRel   = "cxnrole"   // a slot to the thematic role it fills (agent, patient, rel)
	cxnSynRel    = "cxnsyn"    // a slot to the syntactic type it expects (np, v)
	cxnPreferRel = "cxnprefer" // a slot to the feature its best fit favors (animate)
	cxnPredRel   = "cxnpred"   // a construction to the role that carries the relation
)

// ensureArgConstructions represents each seed argument-structure construction as a concept carrying
// its form pole in the graph, then registers every construction concept the graph holds (the seeds
// plus any taught since) so the exclusion maps and the enumeration cover both. It is seeded from the
// same seed that stays the floor, idempotent and self-healing at boot, and runs before the compile so
// the derive reads each construction from the graph. The construction concepts sit outside the grown
// layer.
func (d *Daemon) ensureArgConstructions() error {
	d.argConstructions = map[uint64]string{}
	d.argConstructionParts = map[uint64]bool{}
	for _, cxns := range grammar.ParseArgConstructions(grammar.Constructions()) {
		for _, c := range cxns {
			if _, err := d.buildArgConstructionConcept(c, c.Weight); err != nil {
				return err
			}
		}
	}
	return d.registerGraphConstructions()
}

// buildArgConstructionConcept writes one construction's form pole into the graph: a concept (at the
// given entrenchment weight) carrying its predicate role and an ordered slot per argument, each slot
// linked to its role, syntactic type, and feature preference through a shared vocabulary. It is the
// find-or-create used both to seed the constructions and to teach a new one, so a construction
// learned into the graph is built the same way the seed ones are.
func (d *Daemon) buildArgConstructionConcept(c grammar.ArgConstruction, weight float64) (uint64, error) {
	vocab := func(kind, value string) uint64 {
		if value == "" {
			return 0
		}
		return d.ensureConstructionNode("cxn:"+kind+":"+value, value, "")
	}
	cid := d.ensureConstructionNode("acxn:"+c.Lang+":"+c.Name, c.Name, c.Lang)
	if cid == 0 {
		return 0, nil
	}
	if err := d.store.SetWeight(cid, weight); err != nil {
		return cid, err
	}
	if pid := vocab("role", c.Pred); pid != 0 {
		if err := d.store.LinkRel(cid, pid, cxnPredRel, 1); err != nil {
			return cid, err
		}
	}
	for i, s := range c.Slots {
		sid := d.ensureConstructionNode(fmt.Sprintf("acxn:%s:%s/%d", c.Lang, c.Name, i), strconv.Itoa(i), c.Lang)
		if sid == 0 {
			continue
		}
		if err := d.store.LinkRel(cid, sid, cxnSlotRel, 1); err != nil {
			return cid, err
		}
		for _, attr := range []struct{ rel, kind, value string }{
			{cxnRoleRel, "role", s.Role}, {cxnSynRel, "syn", s.Syn}, {cxnPreferRel, "prefer", s.Prefer},
		} {
			if vid := vocab(attr.kind, attr.value); vid != 0 {
				if err := d.store.LinkRel(sid, vid, attr.rel, 1); err != nil {
					return cid, err
				}
			}
		}
	}
	return cid, nil
}

// registerGraphConstructions populates the exclusion maps from every construction-category concept
// the graph holds: a construction concept (template "acxn:<lang>:<name>") into argConstructions
// keyed by "<lang>:<name>", and a slot or a vocabulary node into argConstructionParts. So a
// construction taught into the graph is discovered at boot, kept out of the grown layer, and reached
// by the enumeration, alongside the seeded ones.
func (d *Daemon) registerGraphConstructions() error {
	concepts, err := d.store.Concepts()
	if err != nil {
		return err
	}
	for _, c := range concepts {
		if c.Category != "construction" {
			continue
		}
		if rest, ok := strings.CutPrefix(c.Template, "acxn:"); ok && !strings.Contains(rest, "/") {
			d.argConstructions[c.ID] = rest // "<lang>:<name>"
		} else {
			d.argConstructionParts[c.ID] = true // a slot ("acxn:.../N") or a vocabulary node ("cxn:...")
		}
	}
	return nil
}

// enumerateGraphArgConstructions appends a parameter for every construction the graph holds as a
// concept but no construction node carries, so a construction taught into the graph (with no seed or
// induced node) reaches the engine. Its form pole comes from the subgraph and its weight from the
// concept, so a taught construction is entrenched and selected like any other. A construction that a
// node already carries is left to deriveArgConstructions, which reads it from the graph in place.
func (d *Daemon) enumerateGraphArgConstructions(cnodes []*memory.Fragment, cparams []grammar.Param) []grammar.Param {
	if len(d.argConstructions) == 0 {
		return cparams
	}
	have := make(map[string]bool, len(cnodes))
	for _, n := range cnodes {
		have[n.Template] = true
	}
	g := d.storeView()
	for id, label := range d.argConstructions {
		lang, name, ok := strings.Cut(label, ":")
		if !ok {
			continue
		}
		template := "acxn:" + lang + ":" + name
		if have[template] {
			continue // a construction node carries this; the in-place derive reads it from the graph
		}
		frags, err := d.store.FragmentsByIDs([]uint64{id})
		if err != nil || len(frags) != 1 {
			continue
		}
		built := d.argConstructionFromGraph(g, id, lang, name)
		if len(built.Slots) == 0 {
			continue // a stale id (a brain swapped under the daemon) resolves to no slots; skip it
		}
		cparams = append(cparams, grammar.Param{
			Key: template, Value: serializeArgSlots(built.Slots), Lang: lang,
			Category: built.Pred, Weight: frags[0].Weight,
		})
	}
	return cparams
}

// ensureConstructionNode is the find-or-create for a construction-scaffolding concept (a
// construction, a slot, or a role/syn/prefer vocabulary word), keyed by its template and held in
// the distinct "construction" category so it cannot collide with a grown noun.
func (d *Daemon) ensureConstructionNode(template, content, lang string) uint64 {
	if c, err := d.store.ConceptByTemplate(template); err == nil && c != nil {
		return c.ID
	}
	c := &memory.Fragment{Kind: memory.KindConcept, Channel: "concept", Content: content,
		Template: template, Category: "construction", Lang: lang, Limbic: d.chem.Snapshot(), Time: time.Now()}
	if _, err := d.store.Encode(c); err != nil {
		fmt.Fprintf(d.out, "construction concept error: %v\n", err)
		return 0
	}
	return c.ID
}

// deriveArgConstructions rewrites each construction parameter's form pole from the construction
// concept the graph holds for it, so the engine reads the slots, their roles, and the predicate from
// the graph rather than the seed string. A construction with no concept in the graph (a per-verb one
// induced from usage, or a brain that does not represent it yet) keeps its stored value, and the
// weight is the stored parameter's throughout, so entrenchment and the induced constructions are
// untouched. While the seed still carries the form pole, a derived pole that disagrees is logged as a
// tripwire that the graph and the seed have parted; the graph is authoritative. It finds the concept
// by template in the current store, so it stays correct when a restore swaps the brain under the
// daemon. A construction node and its parameter line up by index (paramsOf preserves order).
func (d *Daemon) deriveArgConstructions(cnodes []*memory.Fragment, cparams []grammar.Param) {
	if len(d.argConstructions) == 0 {
		return
	}
	g := d.storeView()
	for i, n := range cnodes {
		rest, ok := strings.CutPrefix(n.Template, "acxn:")
		if !ok {
			continue
		}
		lang, name, ok := strings.Cut(rest, ":")
		if !ok || name == "" {
			continue
		}
		c, err := d.store.ConceptByTemplate(n.Template)
		if err != nil || c == nil {
			continue // induced or not yet represented: the stored construction stands
		}
		built := d.argConstructionFromGraph(g, c.ID, lang, name)
		value := serializeArgSlots(built.Slots)
		if value != cparams[i].Value || built.Pred != cparams[i].Category {
			fmt.Fprintf(d.out, "argcxn mismatch: %s graph %q/%q vs seed %q/%q\n", n.Template, value, built.Pred, cparams[i].Value, cparams[i].Category)
		}
		cparams[i].Value = value
		cparams[i].Category = built.Pred
	}
}

// argConstructionFromGraph reads a construction back out of its graph subgraph: the predicate role,
// and the slots in rank order, each slot's role, syntactic type, and feature preference resolved
// through the shared vocabulary. It is the inverse of ensureArgConstructions, walked through the
// same node-and-edge view the code generation reads.
func (d *Daemon) argConstructionFromGraph(g codeGraph, id uint64, lang, name string) grammar.ArgConstruction {
	c := grammar.ArgConstruction{Name: name, Lang: lang}
	if preds := g.dests(id, cxnPredRel); len(preds) == 1 {
		c.Pred = g.content(preds[0])
	}
	type ranked struct {
		rank int
		slot grammar.ArgSlot
	}
	var slots []ranked
	for _, sid := range g.dests(id, cxnSlotRel) {
		rank, _ := strconv.Atoi(g.content(sid))
		s := grammar.ArgSlot{}
		if r := g.dests(sid, cxnRoleRel); len(r) == 1 {
			s.Role = g.content(r[0])
		}
		if y := g.dests(sid, cxnSynRel); len(y) == 1 {
			s.Syn = g.content(y[0])
		}
		if f := g.dests(sid, cxnPreferRel); len(f) == 1 {
			s.Prefer = g.content(f[0])
		}
		slots = append(slots, ranked{rank, s})
	}
	sort.Slice(slots, func(i, j int) bool { return slots[i].rank < slots[j].rank })
	for _, r := range slots {
		c.Slots = append(c.Slots, r.slot)
	}
	return c
}

// serializeArgSlots writes a construction's slots back into the seed's citation-order form
// ("agent/np/animate rel/v patient/np"), the inverse of the parse, so the form pole derived from the
// graph feeds the same compile the seed string did. A trailing empty field is dropped; a feature with
// no syntactic type keeps the empty middle ("role//prefer") so the parse reads it back the same.
func serializeArgSlots(slots []grammar.ArgSlot) string {
	fields := make([]string, len(slots))
	for i, s := range slots {
		parts := []string{s.Role}
		if s.Syn != "" || s.Prefer != "" {
			parts = append(parts, s.Syn)
		}
		if s.Prefer != "" {
			parts = append(parts, s.Prefer)
		}
		fields[i] = strings.Join(parts, "/")
	}
	return strings.Join(fields, " ")
}
