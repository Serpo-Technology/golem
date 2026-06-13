package daemon

import (
	"fmt"
	"strings"
	"time"

	"golem/internal/events"
	"golem/internal/grammar"
	"golem/internal/memory"
)

// The grammar engine is the innate floor and lives in code; its data
// lives in the graph. Each parameter (a function word and its category,
// a verb and its principal parts, a suffix and its recipe, the
// word-order setting) is a grammar node in the same store as everything
// else, readable by dump and recall, seeded at full weight on first run
// through the same encode path every memory takes. Grammar nodes sit
// outside the nightly fade entirely: native grammar is the hardest
// thing to lose, and the only force that moves it is the competition
// below.
//
// The competition is how grammar bends to real usage with no special
// case: a closed-class form that the lexicon comes to claim (its word
// nodes accruing real naming mass through deliberate labeling) erodes
// the seeded node's weight night by night, and the compile drops a
// parameter whose weight has fallen under the lived mass. Usage that
// stops recovers the seed just as gradually.
const (
	grammarErosion     = 0.10 // nightly cost of sustained contrary usage
	grammarRecovery    = 0.02 // nightly healing once the usage fades
	grammarFloor       = 0.05
	grammarContestMass = 0.30 // naming mass below this is noise, no contest
)

// taughtConstructionWeight is the entrenchment a freshly taught
// construction takes: above the seeds, so a new way to say a thing is the
// one that speaks, until it is forgotten or out-weighed.
const taughtConstructionWeight = 1.5

// loadGrammar reads the grammar parameters and the sentence constructions
// out of memory, training both curricula into starter memory on first
// run, and compiles the working copy the engine reads. The compile is the
// cache: per-parse lookups are RAM map hits, rebuilt at wake after the
// nightly competition. A memory trained by an earlier build tops up:
// innate forms and constructions the curriculum has since gained are
// seeded in, while every parameter the store already holds keeps its
// learned weight.
func (d *Daemon) loadGrammar() error {
	gnodes, err := d.store.NodesByKind(memory.KindGrammar)
	if err != nil {
		return err
	}
	if _, err := d.seedParams(gnodes, memory.KindGrammar, grammar.Curriculum()); err != nil {
		return err
	}
	cnodes, err := d.store.NodesByKind(memory.KindConstruction)
	if err != nil {
		return err
	}
	if _, err := d.seedParams(cnodes, memory.KindConstruction, grammar.Constructions()); err != nil {
		return err
	}
	// recompileGrammar reloads both kinds from the store, so the nodes
	// just seeded are picked up without a second fetch here.
	return d.recompileGrammar()
}

// seedParams writes the missing entries of a curriculum into starter
// memory through the standard encode path, so the grammar seed and the
// construction seed are both first curricula and the seeding path is the
// one all bootstrapping uses. Encode stamps every node at maximal weight.
// Entries already held are left alone, which makes the same pass both the
// first boot and the top-up. The kind decides which curriculum a node
// belongs to; both ride the same columns.
func (d *Daemon) seedParams(held []*memory.Fragment, kind string, curriculum []grammar.Param) (int, error) {
	have := make(map[string]bool, len(held))
	for _, n := range held {
		have[n.Template] = true
	}
	trained := 0
	for _, p := range curriculum {
		if have[p.Key] {
			continue
		}
		f := &memory.Fragment{
			Kind:     kind,
			Channel:  "grammar",
			Content:  p.Value,
			Template: p.Key,
			Lang:     p.Lang,
			Category: p.Category,
			Time:     time.Now(),
		}
		if _, err := d.store.Encode(f); err != nil {
			return trained, err
		}
		trained++
	}
	return trained, nil
}

// recompileGrammar rebuilds the engine's working copy from the grammar
// nodes and the construction nodes the store now holds, each grammar
// form's lived naming mass standing against its seeded weight. Both the
// boot path and the nightly competition recompile through here, so the
// constructions are never dropped when the grammar re-weights.
func (d *Daemon) recompileGrammar() error {
	gnodes, err := d.store.NodesByKind(memory.KindGrammar)
	if err != nil {
		return err
	}
	cnodes, err := d.store.NodesByKind(memory.KindConstruction)
	if err != nil {
		return err
	}
	mass, err := d.store.NamesMassByForm()
	if err != nil {
		return err
	}
	// The grammar nodes carry the closed class, the constructions' siblings,
	// and the declarative grammar the general engine runs; each compile step
	// reads the slice it understands and ignores the rest.
	gparams := paramsOf(gnodes)
	d.gram = grammar.Compile(gparams, mass).SetConstructions(paramsOf(cnodes)).SetDecl(gparams)
	return nil
}

// paramsOf reads a set of grammar or construction nodes back into the
// parameters the engine compiles from.
func paramsOf(nodes []*memory.Fragment) []grammar.Param {
	params := make([]grammar.Param, 0, len(nodes))
	for _, n := range nodes {
		params = append(params, grammar.Param{
			Key:      n.Template,
			Value:    n.Content,
			Lang:     n.Lang,
			Category: n.Category,
			Weight:   n.Weight,
		})
	}
	return params
}

// teachConstruction mints a new way to say an intent: a construction node
// whose template the planner will fill, entrenched above the seeds so it
// is the one that speaks. The variant key is made unique per intent, so a
// second teaching of the same intent adds an alternative rather than
// overwriting the first. It is the construction twin of the mental-teach.
func (d *Daemon) teachConstruction(intent, template, lang string) error {
	if lang == "" {
		lang = "en"
	}
	intent = strings.TrimSpace(intent)
	template = strings.TrimSpace(template)
	if intent == "" || template == "" {
		return fmt.Errorf("teach --say needs an intent and a template")
	}
	if strings.ContainsAny(intent, " :") {
		return fmt.Errorf("an intent is one word, not %q", intent)
	}
	prefix := "cxn:" + lang + ":" + intent + ":"
	existing, err := d.store.NodesByKind(memory.KindConstruction)
	if err != nil {
		return err
	}
	n := 0
	for _, e := range existing {
		if strings.HasPrefix(e.Template, prefix+"taught") {
			n++
		}
	}
	key := fmt.Sprintf("%staught%d", prefix, n)
	node := &memory.Fragment{
		Kind:     memory.KindConstruction,
		Channel:  "grammar",
		Content:  template,
		Template: key,
		Lang:     lang,
		Time:     time.Now(),
	}
	id, err := d.store.Encode(node)
	if err != nil {
		return err
	}
	if err := d.store.SetWeight(id, taughtConstructionWeight); err != nil {
		return err
	}
	d.event(events.Event{Ev: "grammar", ID: id, Tpl: key, Weight: taughtConstructionWeight})
	return d.recompileGrammar()
}

// grammarCompetition is the nightly weighted competition between seeded
// parameters and lived usage, run during consolidation: a closed-class
// form whose lexical naming mass is real erodes its grammar node, a
// form the lexicon has let go heals back toward full entrenchment, and
// the working grammar recompiles from what is left standing.
func (d *Daemon) grammarCompetition() {
	nodes, err := d.store.NodesByKind(memory.KindGrammar)
	if err != nil {
		fmt.Fprintf(d.out, "grammar competition error: %v\n", err)
		return
	}
	mass, err := d.store.NamesMassByForm()
	if err != nil {
		fmt.Fprintf(d.out, "grammar competition error: %v\n", err)
		return
	}
	for _, n := range nodes {
		kind, _, _ := strings.Cut(n.Template, ":")
		if kind != "fw" && kind != "sortal" && kind != "hap" {
			continue // only forms compete with the lexicon
		}
		w := n.Weight
		switch {
		case mass[n.Lang+":"+n.Content] >= grammarContestMass:
			w -= grammarErosion
			if w < grammarFloor {
				w = grammarFloor
			}
		case n.Weight < 1.0:
			w += grammarRecovery
			if w > 1.0 {
				w = 1.0
			}
		default:
			continue
		}
		if w == n.Weight {
			continue
		}
		n.Weight = w
		if err := d.store.SetWeight(n.ID, w); err != nil {
			fmt.Fprintf(d.out, "grammar re-weight error: %v\n", err)
			continue
		}
		d.event(events.Event{Ev: "grammar", ID: n.ID, Tpl: n.Template, Weight: w})
	}
	if err := d.recompileGrammar(); err != nil {
		fmt.Fprintf(d.out, "grammar compile error: %v\n", err)
	}
}
