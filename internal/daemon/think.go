package daemon

import (
	"fmt"
	"sort"
	"time"

	"golem/internal/memory"
	"golem/internal/senses"
)

// The inner loop is replay run while awake. When perception is quiet and
// sleep pressure is building but sleep has not yet fired (the brain's
// default-mode condition, mind-wandering when not task-engaged), the
// warmest concept in the workspace associates one step through the lived
// graph and the result feeds back into the workspace. Activation chains
// across ticks instead of dying when a function returns; the chaining is
// thinking and the feedback is the self-reference.
//
// Thought is free association at first: the spread follows the strong
// edges, which are the regularities GOLEM actually lived. Each step
// hands the seed's warmth on at a discount and fatigues the seed, so a
// chain moves instead of circling and damps to nothing unless something
// it touches is worth feeling again. A thought re-enters perception on
// its own channel through the same surprise gate everything else faces:
// a routine association passes unremembered, a novel combination is
// encoded, and that encoding is an insight GOLEM can later recall having
// had. Thoughts never reset the quiet timers and never cue habits: the
// mind's own activity is not the world's.
const (
	thoughtChannel  = "thought"
	thinkDamping    = 0.7  // warmth handed to an association, per step
	thinkFatigue    = 0.4  // what remains on a seed after thinking from it
	thinkSeedFloor  = 0.10 // embers below this do not seed a thought
	thinkFanout     = 2    // associations fed back per step
	thinkAssocFloor = 0.05 // edges weaker than this carry no association
)

// assocEdge is one association reachable from a concept in a single
// thought-step: the concept it lands on, how strongly the lived graph
// carries it, and whether order (a successor edge) rather than plain
// co-occurrence carried it.
type assocEdge struct {
	concept  uint64
	gloss    string
	strength float64
	ordered  bool
}

// mindWandering reports the default-mode condition: nothing surprising
// for the calm window, no fresh input, sleep not yet triggered, and
// something still warm to think from.
func (d *Daemon) mindWandering() bool {
	return time.Since(d.lastSurprise) > d.cfg.CalmBeforeSleep() &&
		time.Since(d.lastObs) >= d.cfg.InputSilence() &&
		!d.readyToSleep() &&
		len(d.ws.warmest(1)) > 0
}

// associations walks one thought-step from a seed concept: through the
// episodes that ground it, along their temporal and successor edges, to
// the concepts grounding what those episodes kept company with.
// Strength aggregates edge weight scaled by the far episode's own
// weight, so association follows what was lived and still matters.
func (d *Daemon) associations(seed uint64) ([]assocEdge, error) {
	grounds, err := d.store.LinksFromRel([]uint64{seed}, memory.RelGrounds)
	if err != nil || len(grounds) == 0 {
		return nil, err
	}
	eIDs := make([]uint64, 0, len(grounds))
	for _, g := range grounds {
		eIDs = append(eIDs, g.To)
	}
	edges, err := d.store.LinksFrom(eIDs)
	if err != nil {
		return nil, err
	}
	var farIDs []uint64
	for _, e := range edges {
		if e.Rel == memory.RelTemporal || e.Rel == memory.RelSuccessor {
			farIDs = append(farIDs, e.To)
		}
	}
	far, err := d.fragmentsByIDsTiered(farIDs)
	if err != nil {
		return nil, err
	}
	byID := make(map[uint64]*memory.Fragment, len(far))
	for _, f := range far {
		byID[f.ID] = f
	}

	concepts := map[string]*memory.Fragment{} // template -> concept, cached per walk
	agg := map[uint64]*assocEdge{}
	for _, e := range edges {
		if e.Rel != memory.RelTemporal && e.Rel != memory.RelSuccessor {
			continue
		}
		f := byID[e.To]
		if f == nil || f.Kind != memory.KindEpisodic ||
			f.Channel == "tell" || f.Channel == thoughtChannel {
			continue
		}
		c, seen := concepts[f.Template]
		if !seen {
			c, err = d.store.ConceptByTemplate(f.Template)
			if err != nil {
				return nil, err
			}
			concepts[f.Template] = c
		}
		if c == nil || c.ID == seed {
			continue
		}
		a := agg[c.ID]
		if a == nil {
			a = &assocEdge{concept: c.ID, gloss: c.Content}
			agg[c.ID] = a
		}
		a.strength += e.Weight * f.Weight
		if e.Rel == memory.RelSuccessor {
			a.ordered = true
		}
	}

	out := make([]assocEdge, 0, len(agg))
	for _, a := range agg {
		if a.strength >= thinkAssocFloor {
			out = append(out, *a)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].strength != out[j].strength {
			return out[i].strength > out[j].strength
		}
		return out[i].concept < out[j].concept
	})
	return out, nil
}

// thinkStep is one step of the inner loop: the warmest concept hands a
// share of its warmth to its strongest associations, steps back, and
// the transition re-enters perception as a thought.
func (d *Daemon) thinkStep() {
	seeds := d.ws.warmest(1)
	if len(seeds) == 0 || seeds[0].heat < thinkSeedFloor {
		return
	}
	seed := seeds[0]
	seedFrags, err := d.store.FragmentsByIDs([]uint64{seed.id})
	if err != nil || len(seedFrags) != 1 {
		return
	}
	assoc, err := d.associations(seed.id)
	if err != nil {
		fmt.Fprintf(d.out, "thinking error: %v\n", err)
		return
	}
	// A seed with nowhere to go cools without a thought, so the
	// workspace cannot ruminate on a dead end forever.
	d.ws.cool(seed.id, thinkFatigue)
	if len(assoc) == 0 {
		return
	}

	for i, a := range assoc {
		if i == thinkFanout {
			break
		}
		d.ws.excite(a.concept, seed.heat*thinkDamping)
	}

	// The step itself is a perceivable event: the strongest transition,
	// phrased by order when order carried it. The surprise gate decides
	// whether it is worth remembering.
	content := fmt.Sprintf("thought: %s ~ %s", seedFrags[0].Content, assoc[0].gloss)
	if assoc[0].ordered {
		content = fmt.Sprintf("thought: after %s, %s", seedFrags[0].Content, assoc[0].gloss)
	}
	d.process(senses.Observation{Channel: thoughtChannel, Content: content, At: time.Now()}, false, true)
}
