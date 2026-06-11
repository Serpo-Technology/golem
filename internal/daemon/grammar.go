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

// loadGrammar reads the grammar parameters out of memory, training the
// curriculum into starter memory on first run, and compiles the working
// copy the engine reads. The compile is the cache: per-parse lookups
// are RAM map hits, rebuilt at wake after the nightly competition.
func (d *Daemon) loadGrammar() error {
	nodes, err := d.store.NodesByKind(memory.KindGrammar)
	if err != nil {
		return err
	}
	if len(nodes) == 0 {
		if err := d.trainGrammar(); err != nil {
			return err
		}
		if nodes, err = d.store.NodesByKind(memory.KindGrammar); err != nil {
			return err
		}
	}
	return d.compileGrammar(nodes)
}

// trainGrammar writes the innate curriculum into starter memory through
// the standard encode path, so the grammar seed is the first curriculum
// and the seeding path is the one all bootstrapping uses. Encode stamps
// every node at maximal weight.
func (d *Daemon) trainGrammar() error {
	for _, p := range grammar.Curriculum() {
		f := &memory.Fragment{
			Kind:     memory.KindGrammar,
			Channel:  "grammar",
			Content:  p.Value,
			Template: p.Key,
			Lang:     p.Lang,
			Category: p.Category,
			Time:     time.Now(),
		}
		if _, err := d.store.Encode(f); err != nil {
			return err
		}
	}
	return nil
}

// compileGrammar builds the engine's working copy from grammar nodes,
// with each form's lived naming mass standing against its seeded
// weight.
func (d *Daemon) compileGrammar(nodes []*memory.Fragment) error {
	mass, err := d.store.NamesMassByForm()
	if err != nil {
		return err
	}
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
	d.gram = grammar.Compile(params, mass)
	return nil
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
	if err := d.compileGrammar(nodes); err != nil {
		fmt.Fprintf(d.out, "grammar compile error: %v\n", err)
	}
}
