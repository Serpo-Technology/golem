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

// ensureArgConstructions represents each argument-structure construction as a concept carrying its
// form pole in the graph: an ordered slot per argument (its content the citation rank), each slot
// linked to its role, syntactic type, and feature preference, plus the predicate role. The roles,
// syntactic types, and features are a small shared vocabulary reused across constructions. It is
// seeded from the same seed that is the floor, idempotent and self-healing at boot, and runs before
// the compile so the derive can shadow the graph against the compiled constructions. The
// construction concepts sit outside the grown layer.
func (d *Daemon) ensureArgConstructions() error {
	d.argConstructions = map[uint64]string{}
	d.argConstructionParts = map[uint64]bool{}
	vocab := func(kind, value string) uint64 {
		if value == "" {
			return 0
		}
		id := d.ensureConstructionNode("cxn:"+kind+":"+value, value, "")
		if id != 0 {
			d.argConstructionParts[id] = true
		}
		return id
	}
	linkVocab := func(slot uint64, rel, kind, value string) error {
		if vid := vocab(kind, value); vid != 0 {
			return d.store.LinkRel(slot, vid, rel, 1)
		}
		return nil
	}
	for lang, cxns := range grammar.ParseArgConstructions(grammar.Constructions()) {
		for _, c := range cxns {
			cid := d.ensureConstructionNode("acxn:"+lang+":"+c.Name, c.Name, lang)
			if cid == 0 {
				continue
			}
			d.argConstructions[cid] = lang + ":" + c.Name
			if pid := vocab("role", c.Pred); pid != 0 {
				if err := d.store.LinkRel(cid, pid, cxnPredRel, 1); err != nil {
					return err
				}
			}
			for i, s := range c.Slots {
				sid := d.ensureConstructionNode(fmt.Sprintf("acxn:%s:%s/%d", lang, c.Name, i), strconv.Itoa(i), lang)
				if sid == 0 {
					continue
				}
				d.argConstructionParts[sid] = true
				if err := d.store.LinkRel(cid, sid, cxnSlotRel, 1); err != nil {
					return err
				}
				if err := linkVocab(sid, cxnRoleRel, "role", s.Role); err != nil {
					return err
				}
				if err := linkVocab(sid, cxnSynRel, "syn", s.Syn); err != nil {
					return err
				}
				if err := linkVocab(sid, cxnPreferRel, "prefer", s.Prefer); err != nil {
					return err
				}
			}
		}
	}
	return nil
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

// shadowArgConstructions rebuilds each construction from its graph subgraph and compares it to the
// compiled, seed-derived one, logging any divergence. It proves the graph representation round-trips
// the form pole the engine walks before the graph is made the source; the compiled constructions
// still drive, so this changes nothing. It finds each construction concept in the current store by
// its template rather than a cached id, so it stays correct when the brain is swapped under the
// daemon by a restore, and is silent for a brain that does not hold the construction concepts yet.
func (d *Daemon) shadowArgConstructions() {
	g := d.storeView()
	for lang, cxns := range grammar.ParseArgConstructions(grammar.Constructions()) {
		for _, seed := range cxns {
			c, err := d.store.ConceptByTemplate("acxn:" + lang + ":" + seed.Name)
			if err != nil || c == nil {
				continue // this brain does not represent the construction in the graph yet
			}
			built := d.argConstructionFromGraph(g, c.ID, lang, seed.Name)
			var compiled grammar.ArgConstruction
			for _, x := range d.gram.ArgConstructions(lang) {
				if x.Name == seed.Name {
					compiled = x
					break
				}
			}
			if !argConstructionsEqual(built, compiled) {
				fmt.Fprintf(d.out, "argcxn mismatch: %s:%s graph %s vs seed %s\n", lang, seed.Name, slotsGloss(built), slotsGloss(compiled))
			}
		}
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

// argConstructionsEqual compares two constructions on the structure the graph holds: the name, the
// language, the predicate role, and the slots in order (role, syntactic type, feature). The weight
// is the seed's metadata, not yet represented in the graph, so it is left out of the comparison.
func argConstructionsEqual(a, b grammar.ArgConstruction) bool {
	if a.Name != b.Name || a.Lang != b.Lang || a.Pred != b.Pred || len(a.Slots) != len(b.Slots) {
		return false
	}
	for i := range a.Slots {
		if a.Slots[i] != b.Slots[i] {
			return false
		}
	}
	return true
}

// slotsGloss renders a construction's slots for the shadow's mismatch log.
func slotsGloss(c grammar.ArgConstruction) string {
	parts := make([]string, len(c.Slots))
	for i, s := range c.Slots {
		parts[i] = s.Role + "/" + s.Syn + "/" + s.Prefer
	}
	return "[" + strings.Join(parts, " ") + "] pred=" + c.Pred
}
