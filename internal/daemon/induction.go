package daemon

import (
	"fmt"
	"strings"
	"time"

	"golem/internal/events"
	"golem/internal/memory"
)

// Construction induction is the usage-based companion to concept and
// abstraction genesis: where those distil a thing from recurring shapes,
// this distils an argument-structure construction from recurring
// propositions. A relation told in enough propositions, its role slots
// filled by varied concepts across them, has shown an item-based
// construction whose slots have generalized over the recurrences, from the
// specific fillers first heard to open positions any concept may take. The
// induced construction is written as a construction node keyed by the
// relation, on one footing with the seed and competing by weight, so the
// inventory grows from what is heard and the seed's first guesses can be
// superseded by lived usage rather than carried forever.
//
// This is the bootstrap stage of a long arc. The induced construction
// re-derives the seeded argument structure for one relation from its own
// usage, which is item-based learning; generalizing across relations into a
// single abstract construction, and inducing genuinely novel structures a
// richer comprehension would surface, are the reach this leaves open.
const (
	inducedMinSituations = 3   // a construction takes several tellings to trust
	inducedMinFillers    = 2   // a slot is general only when varied fillers fill it
	inducedWeightBase    = 0.9 // entrenchment at the threshold, just under the seed
	inducedWeightPer     = 0.1 // each further telling entrenches the pattern
	inducedWeightCap     = 1.4 // below a deliberately taught construction
)

// growConstructions induces an argument-structure construction for each
// relation whose grounded propositions have shown a generalized slot
// pattern, writing or re-weighting a construction node the engine then runs.
// It returns how many were newly born.
func (d *Daemon) growConstructions() (born int) {
	concepts, err := d.store.Concepts()
	if err != nil {
		fmt.Fprintf(d.out, "construction induction error: %v\n", err)
		return 0
	}
	existing, err := d.store.NodesByKind(memory.KindConstruction)
	if err != nil {
		fmt.Fprintf(d.out, "construction induction error: %v\n", err)
		return 0
	}
	have := make(map[string]*memory.Fragment, len(existing))
	for _, n := range existing {
		have[n.Template] = n
	}

	for _, c := range concepts {
		if c.Category != "relation" {
			continue
		}
		lang, lemma := relationKey(c.Template)
		if lang == "" || lemma == "" {
			continue
		}
		grounds, err := d.store.LinksFromRel([]uint64{c.ID}, memory.RelGrounds)
		if err != nil {
			fmt.Fprintf(d.out, "construction induction error: %v\n", err)
			return born
		}
		moments := make([]uint64, 0, len(grounds))
		for _, g := range grounds {
			moments = append(moments, g.To)
		}
		if len(moments) < inducedMinSituations {
			continue
		}
		agents := distinctTargets(d, moments, memory.RelAgent)
		if len(agents) < inducedMinFillers {
			continue // the agent slot has not generalized
		}
		patients := distinctTargets(d, moments, memory.RelPatient)
		// The shape follows the slots that generalized: an open patient makes
		// it transitive, no patient at all makes it intransitive, and a
		// patient seen too narrowly waits for more varied usage.
		var value string
		switch {
		case len(patients) == 0:
			value = "agent/np rel/v"
		case len(patients) >= inducedMinFillers:
			value = "agent/np rel/v patient/np"
		default:
			continue
		}

		key := "acxn:" + lang + ":" + lemma
		weight := inducedWeightBase + inducedWeightPer*float64(len(moments))
		if weight > inducedWeightCap {
			weight = inducedWeightCap
		}
		if n := have[key]; n != nil {
			if weight != n.Weight {
				if err := d.store.SetWeight(n.ID, weight); err != nil {
					fmt.Fprintf(d.out, "construction re-weight error: %v\n", err)
					continue
				}
				d.event(events.Event{Ev: "grammar", ID: n.ID, Tpl: key, Weight: weight, Outcome: "induced"})
			}
			continue
		}
		node := &memory.Fragment{
			Kind:     memory.KindConstruction,
			Channel:  "grammar",
			Content:  value,
			Template: key,
			Lang:     lang,
			Category: "rel",
			Time:     time.Now(),
		}
		id, err := d.store.Encode(node)
		if err != nil {
			fmt.Fprintf(d.out, "construction mint error: %v\n", err)
			continue
		}
		if err := d.store.SetWeight(id, weight); err != nil {
			fmt.Fprintf(d.out, "construction weight error: %v\n", err)
			continue
		}
		d.event(events.Event{Ev: "grammar", ID: id, Tpl: key, Weight: weight, Outcome: "induced"})
		born++
	}
	return born
}

// relationKey unpacks a relation concept's template, "rel:<lang>:<lemma>",
// into the language and lemma the induced construction is keyed by.
func relationKey(template string) (lang, lemma string) {
	parts := strings.SplitN(template, ":", 3)
	if len(parts) != 3 || parts[0] != "rel" {
		return "", ""
	}
	return parts[1], parts[2]
}

// distinctTargets gathers the distinct concepts that filled a role across a
// relation's moments, the measure of whether that slot has generalized.
func distinctTargets(d *Daemon, moments []uint64, rel string) map[uint64]bool {
	edges, err := d.store.LinksFromRel(moments, rel)
	if err != nil {
		return nil
	}
	out := make(map[uint64]bool, len(edges))
	for _, e := range edges {
		out[e.To] = true
	}
	return out
}
