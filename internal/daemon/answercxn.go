package daemon

import (
	"fmt"
	"strings"
	"time"

	"golem/internal/grammar"
	"golem/internal/memory"
)

// The answer constructions are the template-filling realizers Compose runs: an intent (a definition, a
// seen-count, a succession) in a chemistry-conditioned variant maps to a template whose {slots} the
// planner fills. They live in the grammar as seed-loaded strings; these edges represent each as a concept
// carrying its template in the graph, the realization counterpart of the argument-structure and
// declarative constructions. The seed stays the floor: the graph is seeded from it at boot and the derive
// rewrites each template from the graph in place, the graph authoritative, a disagreement logged.
const answerCxnCategory = "answer"

const ansCondRel = "anscond" // an answer construction to the chemistry condition that selects its variant

// ensureAnswerConstructions represents each seed answer template as a concept carrying its template,
// then registers every answer concept the graph holds so the exclusion maps and the enumeration cover the
// seeds and any taught since. Seeded from the seed that stays the floor, idempotent at boot, and run
// before the compile so the derive reads each template from the graph.
func (d *Daemon) ensureAnswerConstructions() error {
	d.answerConstructions = map[uint64]string{}
	d.answerConstructionParts = map[uint64]bool{}
	for _, p := range grammar.Constructions() {
		rest, ok := strings.CutPrefix(p.Key, "cxn:")
		if !ok {
			continue // only the answer templates, not the argument-structure (acxn:) or code (code:) ones
		}
		d.buildAnswerConstructionConcept(rest, p.Value, p.Category, p.Weight)
	}
	return d.registerGraphAnswerConstructions()
}

// buildAnswerConstructionConcept writes one answer template into the graph: a concept whose content is
// the template and which links to the chemistry condition that selects its variant. It is the
// find-or-create used both to seed the templates and to teach a new one.
func (d *Daemon) buildAnswerConstructionConcept(rest, template, cond string, weight float64) uint64 {
	lang, _, _ := strings.Cut(rest, ":")
	cid := d.ensureAnswerNode("anscxn:"+rest, template, lang)
	if cid == 0 {
		return 0
	}
	if weight > 0 {
		if err := d.store.SetWeight(cid, weight); err != nil {
			fmt.Fprintf(d.out, "answer construction weight error: %v\n", err)
		}
	}
	if cond != "" {
		if vid := d.ensureAnswerNode("anscond:"+cond, cond, ""); vid != 0 {
			if err := d.store.LinkRel(cid, vid, ansCondRel, 1); err != nil {
				fmt.Fprintf(d.out, "answer construction cond error: %v\n", err)
			}
		}
	}
	return cid
}

// ensureAnswerNode is the find-or-create for an answer-construction scaffolding concept (a template or a
// condition word), keyed by its template and held in the distinct "answer" category so it cannot collide
// with a grown noun or the other constructions' vocabulary.
func (d *Daemon) ensureAnswerNode(template, content, lang string) uint64 {
	if c, err := d.store.ConceptByTemplate(template); err == nil && c != nil {
		return c.ID
	}
	c := &memory.Fragment{Kind: memory.KindConcept, Channel: "concept", Content: content,
		Template: template, Category: answerCxnCategory, Lang: lang, Limbic: d.chem.Snapshot(), Time: time.Now()}
	if _, err := d.store.Encode(c); err != nil {
		fmt.Fprintf(d.out, "answer construction concept error: %v\n", err)
		return 0
	}
	return c.ID
}

// registerGraphAnswerConstructions populates the exclusion maps from every answer-category concept the
// graph holds: a template concept (template "anscxn:<lang>:<intent>:<variant>") into answerConstructions
// keyed by its "cxn:" parameter key, and a condition vocabulary node into answerConstructionParts.
func (d *Daemon) registerGraphAnswerConstructions() error {
	concepts, err := d.store.Concepts()
	if err != nil {
		return err
	}
	for _, c := range concepts {
		if c.Category != answerCxnCategory {
			continue
		}
		if rest, ok := strings.CutPrefix(c.Template, "anscxn:"); ok {
			d.answerConstructions[c.ID] = "cxn:" + rest
		} else {
			d.answerConstructionParts[c.ID] = true
		}
	}
	return nil
}

// deriveAnswerConstructions rewrites each answer template parameter from the concept the graph holds for
// it, so Compose reads the template and its chemistry condition from the graph rather than the seed
// string. A template with no concept keeps its stored value, the seed the floor. The concept is found by
// template in the current store, so it stays correct after a restore swaps the brain. A construction node
// and its parameter line up by index; a derived template that disagrees is logged, the graph driving.
func (d *Daemon) deriveAnswerConstructions(cnodes []*memory.Fragment, cparams []grammar.Param) {
	if len(d.answerConstructions) == 0 {
		return
	}
	for i, n := range cnodes {
		rest, ok := strings.CutPrefix(n.Template, "cxn:")
		if !ok {
			continue
		}
		c, err := d.store.ConceptByTemplate("anscxn:" + rest)
		if err != nil || c == nil {
			continue // not represented yet: the stored template stands as the floor
		}
		template, cond := c.Content, d.answerCondOf(c.ID)
		if template != cparams[i].Value || cond != cparams[i].Category {
			fmt.Fprintf(d.out, "answer cxn mismatch: %s graph %q/%q vs seed %q/%q\n", n.Template, template, cond, cparams[i].Value, cparams[i].Category)
		}
		cparams[i].Value = template
		cparams[i].Category = cond
	}
}

// answerCondOf reads the chemistry condition an answer construction links to, "" when it has none.
func (d *Daemon) answerCondOf(id uint64) string {
	g := d.storeView()
	if v := g.dests(id, ansCondRel); len(v) == 1 {
		return g.content(v[0])
	}
	return ""
}

// enumerateGraphAnswerConstructions appends a parameter for every answer template the graph holds as a
// concept but no construction node carries, so a template taught into the graph reaches Compose with no
// seed edit. A template a node already carries is left to the in-place derive.
func (d *Daemon) enumerateGraphAnswerConstructions(cnodes []*memory.Fragment, cparams []grammar.Param) []grammar.Param {
	if len(d.answerConstructions) == 0 {
		return cparams
	}
	have := make(map[string]bool, len(cnodes))
	for _, n := range cnodes {
		have[n.Template] = true
	}
	for id, key := range d.answerConstructions {
		if have[key] {
			continue // a construction node carries this template; the in-place derive reads it
		}
		frags, err := d.store.FragmentsByIDs([]uint64{id})
		if err != nil || len(frags) != 1 {
			continue
		}
		lang := ""
		if rest, ok := strings.CutPrefix(key, "cxn:"); ok {
			lang, _, _ = strings.Cut(rest, ":")
		}
		cparams = append(cparams, grammar.Param{Key: key, Value: frags[0].Content, Lang: lang,
			Category: d.answerCondOf(id), Weight: frags[0].Weight})
	}
	return cparams
}
