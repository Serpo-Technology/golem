package daemon

import (
	"fmt"
	"sort"
	"time"

	"golem/internal/memory"
	"golem/internal/senses"
	"golem/internal/wire"
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

// A thought round is the parallel substrate under a serial arbiter:
// the loop builds an immutable snapshot of what is warm, hand-coded
// specialists (a spreader, a predictor, a recaller, a decider) compute
// on it concurrently and post one candidate each through one channel,
// and the loop arbitrates winner-take-all and commits every mutation
// single-threaded: the snapshot-in, candidate-out, loop-commits pattern
// the execution layer already proved. Specialists never touch the
// store or the daemon, which is what keeps concurrent thought from
// racing the graph.

// thoughtSnapshot is everything a specialist may read: the warm seed
// and its one-step neighborhood, prefetched on the loop.
type thoughtSnapshot struct {
	epoch     uint64
	seed      uint64
	seedGloss string
	seedHeat  float64
	assoc     []assocEdge
	sharp     sharpMemory
	plan      deliberation
}

// sharpMemory is the seed's most stressful grounded moment, the
// recaller's material.
type sharpMemory struct {
	content string
	na      float64
	ok      bool
}

// deliberation is the seed's actionable face when it is verb-like: the
// command its groundings would propose, never staged and never run.
type deliberation struct {
	command  string
	saliency float64
	conflict bool
	ok       bool
}

// thoughtCandidate is one specialist's offer: a thought as perception
// would see it, the workspace changes it implies, and the saliency the
// arbiter weighs.
type thoughtCandidate struct {
	epoch    uint64
	source   string
	content  string
	seed     uint64
	seedHeat float64
	warm     []uint64
	strength float64
	ok       bool
}

// startThinkRound snapshots the warm seed and fans the specialists out.
// One round in flight at a time; the round is stamped with the current
// epoch so sleep can invalidate it.
func (d *Daemon) startThinkRound() {
	if d.thinking > 0 {
		return
	}
	seeds := d.ws.warmest(1)
	if len(seeds) == 0 || seeds[0].heat < thinkSeedFloor {
		return
	}
	seedFrags, err := d.store.FragmentsByIDs([]uint64{seeds[0].id})
	if err != nil || len(seedFrags) != 1 {
		return
	}
	seed := seedFrags[0]
	sn := thoughtSnapshot{
		epoch:     d.thinkEpoch,
		seed:      seed.ID,
		seedGloss: seed.Content,
		seedHeat:  seeds[0].heat,
	}
	if sn.assoc, err = d.associations(seed.ID); err != nil {
		fmt.Fprintf(d.out, "thinking error: %v\n", err)
		return
	}
	sn.sharp = d.sharpestGrounding(seed.ID)
	if seed.Category == "verb" {
		if dec, err := d.conceptDecision(seed); err == nil && dec.Proposed != nil {
			sn.plan = deliberation{
				command:  dec.Proposed.Command,
				saliency: dec.Proposed.Saliency,
				conflict: dec.Conflict,
				ok:       true,
			}
		}
	}

	d.thinkSeed, d.thinkSeedHeat = sn.seed, sn.seedHeat
	specialists := []func(thoughtSnapshot) thoughtCandidate{
		spreadThought, predictThought, recallThought, deliberateThought,
	}
	d.thinking = len(specialists)
	for _, specialist := range specialists {
		go func(think func(thoughtSnapshot) thoughtCandidate) {
			d.thinkDone <- think(sn)
		}(specialist)
	}
}

// sharpestGrounding returns the most stressful moment grounding a
// concept, for the recaller.
func (d *Daemon) sharpestGrounding(seed uint64) sharpMemory {
	grounds, err := d.store.LinksFromRel([]uint64{seed}, memory.RelGrounds)
	if err != nil || len(grounds) == 0 {
		return sharpMemory{}
	}
	ids := make([]uint64, 0, len(grounds))
	for _, g := range grounds {
		ids = append(ids, g.To)
	}
	frags, err := d.fragmentsByIDsTiered(ids)
	if err != nil || len(frags) == 0 {
		return sharpMemory{}
	}
	best := frags[0]
	for _, f := range frags {
		if f.Limbic.Noradrenaline > best.Limbic.Noradrenaline {
			best = f
		}
	}
	return sharpMemory{content: wire.Clip(best.Content, 80), na: best.Limbic.Noradrenaline, ok: true}
}

// absorbThought reabsorbs one specialist's candidate; when the round is
// complete the loop arbitrates and commits. A round that finishes into
// a busy moment is discarded whole: thinking yields to perception.
func (d *Daemon) absorbThought(cand thoughtCandidate) {
	d.thinking--
	if cand.ok && cand.epoch == d.thinkEpoch {
		d.thinkRound = append(d.thinkRound, cand)
	}
	if d.thinking > 0 {
		return
	}
	round := d.thinkRound
	d.thinkRound = nil
	if time.Since(d.lastObs) < d.cfg.InputSilence() {
		return
	}
	if len(round) == 0 {
		// A seed with nowhere to go cools without a thought, so the
		// workspace cannot ruminate on a dead end forever.
		d.ws.cool(d.thinkSeed, thinkFatigue)
		return
	}
	winner := round[0]
	for _, c := range round[1:] {
		if c.strength > winner.strength ||
			(c.strength == winner.strength && c.source < winner.source) {
			winner = c
		}
	}
	d.ws.cool(winner.seed, thinkFatigue)
	for _, id := range winner.warm {
		d.ws.excite(id, winner.seedHeat*thinkDamping)
	}
	d.process(senses.Observation{Channel: thoughtChannel, Content: winner.content, At: time.Now()}, false, true)
}

// settleThinking completes the open think round the way the Run loop
// would, for loopless drivers: demos and tests.
func (d *Daemon) settleThinking() {
	for d.thinking > 0 {
		d.absorbThought(<-d.thinkDone)
	}
}

// spreadThought is free association: the strongest step away from the
// seed, however the lived graph carries it.
func spreadThought(sn thoughtSnapshot) thoughtCandidate {
	if len(sn.assoc) == 0 {
		return thoughtCandidate{epoch: sn.epoch, source: "spreader"}
	}
	top := sn.assoc[0]
	warm := make([]uint64, 0, thinkFanout)
	for i, a := range sn.assoc {
		if i == thinkFanout {
			break
		}
		warm = append(warm, a.concept)
	}
	return thoughtCandidate{
		epoch: sn.epoch, source: "spreader", ok: true,
		content: fmt.Sprintf("thought: %s ~ %s", sn.seedGloss, top.gloss),
		seed:    sn.seed, seedHeat: sn.seedHeat, warm: warm,
		strength: top.strength * sn.seedHeat,
	}
}

// predictThought follows order: the strongest successor continuation,
// preferred slightly over plain co-occurrence because lived sequence is
// the brain's favored next step.
func predictThought(sn thoughtSnapshot) thoughtCandidate {
	for _, a := range sn.assoc {
		if !a.ordered {
			continue
		}
		return thoughtCandidate{
			epoch: sn.epoch, source: "predictor", ok: true,
			content: fmt.Sprintf("thought: after %s, %s", sn.seedGloss, a.gloss),
			seed:    sn.seed, seedHeat: sn.seedHeat, warm: []uint64{a.concept},
			strength: a.strength * sn.seedHeat * 1.05,
		}
	}
	return thoughtCandidate{epoch: sn.epoch, source: "predictor"}
}

// recallThought dwells: the seed's sharpest grounded moment resurfaces,
// quieter than an active association.
func recallThought(sn thoughtSnapshot) thoughtCandidate {
	if !sn.sharp.ok || sn.sharp.na == 0 {
		return thoughtCandidate{epoch: sn.epoch, source: "recaller"}
	}
	return thoughtCandidate{
		epoch: sn.epoch, source: "recaller", ok: true,
		content: fmt.Sprintf("thought: that time: %s", sn.sharp.content),
		seed:    sn.seed, seedHeat: sn.seedHeat,
		strength: sn.sharp.na * sn.seedHeat * 0.8,
	}
}

// deliberateThought notices that a warm verb concept is actionable. It
// only ever thinks so: nothing is staged, nothing runs, and a thought
// never cues a habit.
func deliberateThought(sn thoughtSnapshot) thoughtCandidate {
	if !sn.plan.ok || sn.plan.conflict {
		return thoughtCandidate{epoch: sn.epoch, source: "decider"}
	}
	saliency := sn.plan.saliency
	if saliency > 1 {
		saliency = 1
	}
	return thoughtCandidate{
		epoch: sn.epoch, source: "decider", ok: true,
		content: fmt.Sprintf("thought: could run %s", sn.plan.command),
		seed:    sn.seed, seedHeat: sn.seedHeat,
		strength: saliency * sn.seedHeat * 0.6,
	}
}
