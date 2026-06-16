package daemon

import (
	"fmt"
	"math"
	"slices"
	"sort"
	"strings"
	"time"

	"golem/internal/events"
	"golem/internal/limbic"
	"golem/internal/memory"
	"golem/internal/senses"
	"golem/wire"
)

// The inner loop is replay run while awake. When perception is quiet and
// sleep pressure is building but sleep has not yet fired (the brain's
// default-mode condition, mind-wandering when not task-engaged), the
// warmest concept in the workspace takes one thought-step through the
// lived graph and the result feeds back into the workspace. Activation
// chains across ticks instead of dying when a function returns; the
// chaining is thinking and the feedback is the self-reference.
//
// Thought bottoms out in one executable operation, walk(R, S): traverse
// relation R from the seed and rank the destinations by signal S. The
// relations are the ones the graph holds (temporal, successor, grounds,
// names) and the signals are the ones already tracked (weight, stress,
// saliency, recency), so the space of thought-types is their cross
// product and a new relation or signal yields a new kind of thought with
// no new code. Free association is walk(temporal, weight), expectation
// is walk(successor, weight), stress recollection is walk(grounds,
// stress), deliberation is walk(grounds, saliency); nothing in the code
// names those kinds, they are settings.
//
// Which setting fires is not fixed either. Every operation computes on
// the same snapshot and the loop arbitrates winner-take-all, with each
// candidate weighed by the chemistry's appetite for its signal, by the
// learned worth of its operation, and by how warm any concept naming
// that operation sits in the workspace. Mood, learning, and what was
// just said about thinking are what select the kind of thought.
//
// Each committed step hands the seed's warmth on at a discount and
// fatigues the seed, so a chain moves instead of circling and damps to
// nothing unless something it touches is worth feeling again. A thought
// re-enters perception on its own channel through the same surprise gate
// everything else faces: a routine association passes unremembered, a
// novel combination is encoded, and that encoding is an insight GOLEM
// can later recall having had. A remembered thought is fruit: it
// reinforces the operation that produced it, the operant loop turned
// inward, so GOLEM learns which of its own operations are worth running.
// Thoughts never reset the quiet timers and never cue habits: the mind's
// own activity is not the world's.
const (
	thoughtChannel    = "thought"
	utteranceChannel  = "tell"
	thinkDamping      = 0.7  // warmth handed to an association, per step
	thinkFatigue      = 0.4  // what remains on a seed after thinking from it
	thinkSeedFloor    = 0.10 // embers below this do not seed a thought
	thinkFanout       = 2    // associations fed back per step
	thinkAssocFloor   = 0.05 // edges weaker than this carry no association
	thoughtRefractory = 24   // recent re-entered thoughts guarded against repeat
)

// The signals a walk can rank by. Weight is the lived strength of the
// path, stress is the noradrenaline the destination was encoded under,
// saliency is the dopamine-scaled pull of an actionable grounding, and
// recency is closeness in time.
const (
	sigWeight   = "weight"
	sigStress   = "stress"
	sigSaliency = "saliency"
	sigRecency  = "recency"
)

// thoughtOp is one setting of the think primitive: traverse relation Rel
// from the seed and rank the destinations by signal Sig.
type thoughtOp struct {
	rel string
	sig string
}

// template is the operation's stable face in the graph and the trail.
func (op thoughtOp) template() string { return "walk(" + op.rel + "," + op.sig + ")" }

// gloss is the operation's speakable face.
func (op thoughtOp) gloss() string { return "walk " + op.rel + " by " + op.sig }

var thinkRels = []string{memory.RelTemporal, memory.RelSuccessor, memory.RelGrounds,
	memory.RelNames, memory.RelFeature, memory.RelAgent, memory.RelPatient}
var thinkSigs = []string{sigWeight, sigStress, sigSaliency, sigRecency}

// thoughtOps is the whole operation space, the innate floor of thought:
// every relation crossed with every signal, in a fixed order.
func thoughtOps() []thoughtOp {
	out := make([]thoughtOp, 0, len(thinkRels)*len(thinkSigs))
	for _, rel := range thinkRels {
		for _, sig := range thinkSigs {
			out = append(out, thoughtOp{rel: rel, sig: sig})
		}
	}
	return out
}

// opGains is the innate loudness prior: how strongly each (relation,
// signal) pairing speaks when its signal is fully present. Association
// and order carry the lived weights, order slightly preferred because
// lived sequence is the brain's favored next step; dwelling on a
// grounding and noticing an action are quieter than active association;
// resemblance through shared features is quieter still, and names are
// the quietest, a murmur about vocabulary.
var opGains = map[string]map[string]float64{
	memory.RelTemporal:  {sigWeight: 1.00, sigStress: 0.45, sigSaliency: 0, sigRecency: 0.50},
	memory.RelSuccessor: {sigWeight: 1.05, sigStress: 0.45, sigSaliency: 0, sigRecency: 0.50},
	memory.RelGrounds:   {sigWeight: 0.50, sigStress: 0.80, sigSaliency: 0.60, sigRecency: 0.55},
	memory.RelNames:     {sigWeight: 0.50, sigStress: 0.30, sigSaliency: 0, sigRecency: 0.35},
	memory.RelFeature:   {sigWeight: 0.70, sigStress: 0.30, sigSaliency: 0, sigRecency: 0.40},
	memory.RelAgent:     {sigWeight: 0.80, sigStress: 0.45, sigSaliency: 0.30, sigRecency: 0.45},
	memory.RelPatient:   {sigWeight: 0.80, sigStress: 0.45, sigSaliency: 0.30, sigRecency: 0.45},
}

// appetite is the chemistry's pull toward a signal, 1.0 at baseline so a
// resting mind weighs the operations by their innate gains alone. Alarm
// pulls thinking toward stressful memory and the just-now and away from
// idle association; reward chemistry pulls it toward action.
func appetite(chem limbic.State, sig string) float64 {
	a := 1.0
	switch sig {
	case sigWeight:
		a = 1.0 - 0.6*(chem.Noradrenaline-0.1)
	case sigStress:
		a = 1.0 + 1.6*(chem.Noradrenaline-0.1)
	case sigSaliency:
		a = 1.0 + 1.2*(chem.Dopamine-0.2)
	case sigRecency:
		a = 1.0 + 0.8*(chem.Noradrenaline-0.1)
	}
	if a < 0.1 {
		a = 0.1
	}
	return a
}

// Operant learning over the operation space: a thought surprising enough
// to be remembered reinforces the operation that produced it, a routine
// one wears it slightly, and the learned worth lives as the operation
// node's weight in the graph, bounded so no operation is ever lost or
// runaway. A warm concept naming an operation biases selection toward
// it, which is what lets a taught word like "worry" steer the mind.
const (
	opFruitDelta  = 0.15
	opBarrenDelta = 0.02
	opWeightCeil  = 2.0
	opWeightFloor = 0.25
	opHeatGain    = 2.0
	opNameWeight  = 0.9 // word-to-concept and concept-to-operation bonds from an explicit mental-teach
)

// walkDest is one destination a walk can land on, with every signal
// measured so any (relation, signal) setting can rank it. degree counts
// the edges that led here, the breadth that marks a junk hub everything
// connects to, so the walk can discount it.
type walkDest struct {
	id      uint64 // concept to warm; zero when the destination is not a workspace referent
	gloss   string
	kind    string
	via     string // the relation crossed to reach here, when this is a relation step
	weight  float64
	stress  float64
	recency float64
	degree  int
}

// hubDamp discounts a destination by how many edges reach it, so salience
// is information, not frequency: a node reached by a few strong paths keeps
// its pull while a hub everything connects to is divided down and stops
// dominating the walk. A handful of edges is ordinary association and goes
// untouched; only past that does the logarithmic damping bite, so a focused
// neighbor is left alone and only a true hub is held back.
const hubDampFree = 6

func hubDamp(degree int) float64 {
	if degree <= hubDampFree {
		return 1
	}
	return 1 + math.Log2(float64(degree)/float64(hubDampFree))
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

// A thought round is the parallel substrate under a serial arbiter: the
// loop builds an immutable snapshot of what is warm, every operation in
// the space computes on it concurrently and posts one candidate through
// one channel, and the loop arbitrates winner-take-all and commits every
// mutation single-threaded: the snapshot-in, candidate-out, loop-commits
// pattern the execution layer already proved. Operations never touch the
// store or the daemon, which is what keeps concurrent thought from
// racing the graph.

// thoughtSnapshot is everything an operation may read: the warm seed,
// its prefetched neighborhood per relation, the chemistry, the policy
// state (learned worth and naming warmth per operation), and the body
// state the acts weigh (clutter, the pending slot, the last thought),
// all gathered on the loop.
type thoughtSnapshot struct {
	epoch     uint64
	seed      uint64
	seedGloss string
	seedHeat  float64
	chem      limbic.State
	dests     map[string][]walkDest
	plan      deliberation
	opWorth   map[string]float64
	opHeat    map[string]float64

	clutter        float64 // fresh encodes since the last consolidation, scaled
	canConsolidate bool    // enough time and clutter for an unprompted night
	pendingHeld    bool    // a proposal is already awaiting the operator
	introRecency   float64 // closeness of the last committed thought

	// Drives bias the act gate: per-act multipliers with the reasons
	// that earned them, and the held goal's topic for the recall act to
	// aim at.
	driveBias map[string]float64
	driveWhy  map[string]string
	goalTopic string
}

// worth reports the learned preference for an operation, 1.0 until
// experience has moved it.
func (sn thoughtSnapshot) worth(tpl string) float64 {
	if w, ok := sn.opWorth[tpl]; ok {
		return w
	}
	return 1.0
}

// deliberation is the seed's actionable face when it is verb-like: the
// command its groundings would propose, never staged and never run.
type deliberation struct {
	command  string
	saliency float64
	conflict bool
	ok       bool
}

// thoughtCandidate is one operation's offer: a thought as perception
// would see it (or an act the loop would perform), the workspace
// changes it implies, and the priority the arbiter weighs.
type thoughtCandidate struct {
	epoch    uint64
	op       string // the producing operation's template, for fruit and the trail
	content  string
	arg      string // the act's target, when the operation is an act
	why      string // the drive that pushed it, when one did
	seed     uint64
	seedHeat float64
	warm     []uint64
	strength float64
	ok       bool
}

// opPriority composes one candidate's claim on ignition: the measured
// signal, scaled by the innate gain of the pairing, the chemistry's
// appetite for the signal, the operation's learned worth, and the warmth
// of any concept taught to name the operation. State selects the kind of
// thought; no specialist is named anywhere.
func opPriority(sn thoughtSnapshot, op thoughtOp, sig float64) float64 {
	return sig * opGains[op.rel][op.sig] * appetite(sn.chem, op.sig) *
		sn.worth(op.template()) * (1 + opHeatGain*sn.opHeat[op.template()])
}

// startThinkRound snapshots the warm seed and fans the operation space
// out. One round in flight at a time; the round is stamped with the
// current epoch so sleep can invalidate it.
func (d *Daemon) startThinkRound() {
	if d.thinking > 0 {
		return
	}
	// The held intention settles before anything is selected: a goal the
	// world has answered should reward and clear, not keep steering.
	d.checkGoal()
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
		chem:      d.chem.Snapshot(),
	}
	if sn.dests, err = d.walkableDests(seed.ID); err != nil {
		fmt.Fprintf(d.out, "thinking error: %v\n", err)
		return
	}
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
	sn.opWorth, sn.opHeat = d.opPolicy()
	sn.clutter = clutterOf(d.encodesSinceSleep)
	sn.canConsolidate = d.encodesSinceSleep >= actClutterMin &&
		time.Since(d.lastSleep) >= d.cfg.SleepInterval()/4
	sn.pendingHeld = d.pending != nil
	if d.lastThought != "" {
		sn.introRecency = recencyOf(time.Now(), d.lastThoughtAt)
	}
	seedNameless := false
	if seed.Kind == memory.KindConcept {
		if names, err := d.store.LinksToRel([]uint64{seed.ID}, memory.RelNames); err == nil {
			seedNameless = len(names) == 0
		}
	}
	sn.driveBias, sn.driveWhy = driveMaps(d.computeDrives(time.Now(), seedNameless))
	if d.goal != nil {
		sn.goalTopic = d.goal.topic
	}

	d.thinkSeed, d.thinkSeedHeat = sn.seed, sn.seedHeat
	panel := thoughtOps()
	acts := d.actPanel(time.Now())
	d.thinking = len(panel) + len(acts)
	for _, op := range panel {
		go func(op thoughtOp) {
			d.thinkDone <- walkThought(sn, op)
		}(op)
	}
	for _, op := range acts {
		go func(op actOp) {
			d.thinkDone <- actThought(sn, op)
		}(op)
	}
}

// walkableDests prefetches the seed's one-step neighborhood per
// relation, every signal measured, so the operations can run without
// touching the store. Grounds destinations are the seed's own
// instantiations: episodes to dwell on (and, for an abstract concept,
// the concepts of its constellation, which the walk may warm). Temporal
// and successor destinations are the concepts of what those episodes
// kept company with, strength aggregated the way the lived graph carries
// it. Names destinations are the words that name the seed.
func (d *Daemon) walkableDests(seed uint64) (map[string][]walkDest, error) {
	now := time.Now()
	dests := map[string][]walkDest{}

	grounds, err := d.store.LinksFromRel([]uint64{seed}, memory.RelGrounds)
	if err != nil {
		return nil, err
	}
	eIDs := make([]uint64, 0, len(grounds))
	gw := make(map[uint64]float64, len(grounds))
	for _, g := range grounds {
		eIDs = append(eIDs, g.To)
		gw[g.To] = g.Weight
	}
	grounded, err := d.fragmentsByIDsTiered(eIDs)
	if err != nil {
		return nil, err
	}
	var episodeIDs []uint64
	for _, f := range grounded {
		if f.Channel == utteranceChannel || f.Channel == thoughtChannel {
			continue
		}
		dst := walkDest{
			gloss:   wire.Clip(f.Content, 80),
			kind:    f.Kind,
			weight:  gw[f.ID] * f.Weight,
			stress:  f.Limbic.Noradrenaline,
			recency: recencyOf(now, f.Time),
		}
		switch f.Kind {
		case memory.KindEpisodic:
			episodeIDs = append(episodeIDs, f.ID)
		case memory.KindConcept:
			// An abstract concept grounds in concepts; unfolding it is a
			// step the chain can continue from.
			dst.id = f.ID
		default:
			continue
		}
		dests[memory.RelGrounds] = append(dests[memory.RelGrounds], dst)
	}

	edges, err := d.store.LinksFrom(episodeIDs)
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
	agg := map[string]map[uint64]*walkDest{
		memory.RelTemporal:  {},
		memory.RelSuccessor: {},
	}
	for _, e := range edges {
		if e.Rel != memory.RelTemporal && e.Rel != memory.RelSuccessor {
			continue
		}
		f := byID[e.To]
		if f == nil || f.Kind != memory.KindEpisodic ||
			f.Channel == utteranceChannel || f.Channel == thoughtChannel {
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
		a := agg[e.Rel][c.ID]
		if a == nil {
			a = &walkDest{id: c.ID, gloss: c.Content, kind: c.Kind}
			agg[e.Rel][c.ID] = a
		}
		a.weight += e.Weight * f.Weight
		a.degree++
		if f.Limbic.Noradrenaline > a.stress {
			a.stress = f.Limbic.Noradrenaline
		}
		if r := recencyOf(now, f.Time); r > a.recency {
			a.recency = r
		}
	}
	for rel, byConcept := range agg {
		for _, a := range byConcept {
			// Salience by information: a destination reached by many edges is
			// a hub, and dividing its mass by that breadth keeps the inner
			// loop from circling whatever everything happens to touch.
			a.weight /= hubDamp(a.degree)
			if a.weight >= thinkAssocFloor {
				dests[rel] = append(dests[rel], *a)
			}
		}
	}

	names, err := d.store.LinksToRel([]uint64{seed}, memory.RelNames)
	if err != nil {
		return nil, err
	}
	wordIDs := make([]uint64, 0, len(names))
	nw := make(map[uint64]float64, len(names))
	for _, e := range names {
		wordIDs = append(wordIDs, e.From)
		nw[e.From] = e.Weight
	}
	words, err := d.fragmentsByIDsTiered(wordIDs)
	if err != nil {
		return nil, err
	}
	for _, w := range words {
		dests[memory.RelNames] = append(dests[memory.RelNames], walkDest{
			gloss:   w.Content,
			kind:    w.Kind,
			weight:  nw[w.ID] * w.Weight,
			stress:  w.Limbic.Noradrenaline,
			recency: recencyOf(now, w.Time),
		})
	}

	siblings, err := d.featureSiblings(seed, now)
	if err != nil {
		return nil, err
	}
	dests[memory.RelFeature] = siblings

	// Relation crossings: a step from the seed through a relation to its
	// co-argument. As agent the seed reaches what it acted on, the patient of
	// the shared moment; as patient, what acted on it, the agent. The relation
	// that grounds the moment names the crossing. The role edges stay empty
	// until comprehension writes propositions, so this is dormant until then.
	roleCross := func(seedRole, partnerRole, key string) error {
		held, err := d.store.LinksToRel([]uint64{seed}, seedRole)
		if err != nil {
			return err
		}
		for _, m := range held {
			via := ""
			if g, err := d.store.LinksToRel([]uint64{m.From}, memory.RelGrounds); err == nil && len(g) > 0 {
				if rf, err := d.fragmentsByIDsTiered([]uint64{g[0].From}); err == nil && len(rf) > 0 {
					via = rf[0].Content
				}
			}
			partners, err := d.store.LinksFromRel([]uint64{m.From}, partnerRole)
			if err != nil {
				return err
			}
			for _, p := range partners {
				pf, err := d.fragmentsByIDsTiered([]uint64{p.To})
				if err != nil || len(pf) == 0 || pf[0].ID == seed {
					continue
				}
				f := pf[0]
				dests[key] = append(dests[key], walkDest{
					id:      f.ID,
					gloss:   f.Content,
					kind:    f.Kind,
					via:     via,
					weight:  m.Weight * p.Weight * f.Weight,
					stress:  f.Limbic.Noradrenaline,
					recency: recencyOf(now, f.Time),
				})
			}
		}
		return nil
	}
	if err := roleCross(memory.RelAgent, memory.RelPatient, memory.RelPatient); err != nil {
		return nil, err
	}
	if err := roleCross(memory.RelPatient, memory.RelAgent, memory.RelAgent); err != nil {
		return nil, err
	}

	for rel := range dests {
		list := dests[rel]
		sort.Slice(list, func(i, j int) bool {
			if list[i].weight != list[j].weight {
				return list[i].weight > list[j].weight
			}
			return list[i].gloss < list[j].gloss
		})
	}
	return dests, nil
}

// featureSiblings finds the concepts a seed resembles: those sharing
// its feature attributes, scored by the overlapping shares against the
// seed's whole signature, so resemblance lives in [0, 1] and a shared
// channel or one common word never outshouts lived association. Walking
// to one is analogy along the sparse layer, while the thought itself
// still originates from and lands on named nodes, so provenance
// survives the resemblance.
func (d *Daemon) featureSiblings(seed uint64, now time.Time) ([]walkDest, error) {
	mine, err := d.store.LinksFromRel([]uint64{seed}, memory.RelFeature)
	if err != nil {
		return nil, err
	}
	var out []walkDest
	if len(mine) > 0 {
		featIDs := make([]uint64, 0, len(mine))
		share := make(map[uint64]float64, len(mine))
		mass := 0.0
		for _, e := range mine {
			featIDs = append(featIDs, e.To)
			share[e.To] = e.Weight
			mass += e.Weight
		}
		bearers, err := d.store.LinksToRel(featIDs, memory.RelFeature)
		if err != nil {
			return nil, err
		}
		overlap := map[uint64]float64{}
		for _, e := range bearers {
			if e.From == seed {
				continue
			}
			w := e.Weight
			if share[e.To] < w {
				w = share[e.To]
			}
			overlap[e.From] += w / mass
		}
		ids := make([]uint64, 0, len(overlap))
		for id, w := range overlap {
			if w >= thinkAssocFloor {
				ids = append(ids, id)
			}
		}
		siblings, err := d.fragmentsByIDsTiered(ids)
		if err != nil {
			return nil, err
		}
		for _, f := range siblings {
			if f.Kind != memory.KindConcept {
				continue
			}
			out = append(out, walkDest{
				id:      f.ID,
				gloss:   f.Content,
				kind:    f.Kind,
				weight:  overlap[f.ID],
				stress:  f.Limbic.Noradrenaline,
				recency: recencyOf(now, f.Time),
			})
		}
	}
	// The vector layer blends in: a seed with an embedding draws its nearest
	// neighbours in the space, so resemblance is geometric where embeddings
	// exist and shared-token where they do not. With no embeddings learned yet
	// this adds nothing, so the symbolic resemblance stands unchanged.
	return d.blendVectorSiblings(seed, out, now), nil
}

const (
	vectorSiblingsCap = 8   // nearest neighbours the vector layer proposes per seed
	vectorSimFloor    = 0.5 // a neighbour must be at least this near to count as resembling
)

// blendVectorSiblings folds the vector layer's nearest concepts into a seed's
// resemblance set: where an embedding exists, the percepts nearest it in the
// space join the shared-token siblings, taking the stronger weight where both
// name the same concept. The embedder lives outside the brain, so this only
// measures distance over coordinates it was handed. A seed with no embedding,
// the case until embeddings are learned, passes the set through untouched, so
// the symbolic walk is unchanged until the associative layer is populated.
func (d *Daemon) blendVectorSiblings(seed uint64, out []walkDest, now time.Time) []walkDest {
	vec, ok := d.store.Vector(seed)
	if !ok {
		return out
	}
	matches, err := d.store.NearestVectors(vec, memory.KindConcept, vectorSiblingsCap, seed)
	if err != nil {
		return out
	}
	at := make(map[uint64]int, len(out))
	for i, ws := range out {
		at[ws.id] = i
	}
	var want []uint64
	for _, m := range matches {
		if m.Score >= vectorSimFloor {
			want = append(want, m.ID)
		}
	}
	frags, err := d.fragmentsByIDsTiered(want)
	if err != nil {
		return out
	}
	byID := make(map[uint64]*memory.Fragment, len(frags))
	for _, f := range frags {
		byID[f.ID] = f
	}
	for _, m := range matches {
		if m.Score < vectorSimFloor {
			continue
		}
		f := byID[m.ID]
		if f == nil || f.Kind != memory.KindConcept {
			continue
		}
		if i, exists := at[m.ID]; exists {
			if m.Score > out[i].weight {
				out[i].weight = m.Score
			}
			continue
		}
		out = append(out, walkDest{
			id:      f.ID,
			gloss:   f.Content,
			kind:    f.Kind,
			weight:  m.Score,
			stress:  f.Limbic.Noradrenaline,
			recency: recencyOf(now, f.Time),
		})
	}
	return out
}

// growConceptVectors lifts the vector layer from percepts to concepts: a
// concept's embedding is the mean of the embeddings of what it grounds in, the
// percepts a transducer placed in the space for a perceptual concept, the
// sub-concepts for an abstraction. The associative coordinate each percept was
// given rolls up into the concept distilled from it, so the resemblance walk has
// concept-level neighbours to draw. It runs each night, after grounding settles,
// so a concept's place tracks what it grounds in. With no percept embeddings,
// the case until an embedder feeds them, this writes nothing and the concept
// graph stands as it was. Cosine is scale-free, so the unnormalized mean serves.
func (d *Daemon) growConceptVectors() {
	concepts, err := d.store.Concepts()
	if err != nil {
		fmt.Fprintf(d.out, "concept vector error: %v\n", err)
		return
	}
	for _, c := range concepts {
		grounds, err := d.store.LinksFromRel([]uint64{c.ID}, memory.RelGrounds)
		if err != nil || len(grounds) == 0 {
			continue
		}
		var sum []float32
		n := 0
		for _, g := range grounds {
			v, ok := d.store.Vector(g.To)
			if !ok {
				continue
			}
			if sum == nil {
				sum = make([]float32, len(v))
			}
			if len(v) != len(sum) {
				continue
			}
			for i := range v {
				sum[i] += v[i]
			}
			n++
		}
		if n == 0 {
			continue
		}
		for i := range sum {
			sum[i] /= float32(n)
		}
		if err := d.store.SetVector(c.ID, sum); err != nil {
			fmt.Fprintf(d.out, "concept vector error: %v\n", err)
		}
	}
}

// recencyOf scores closeness in time into (0, 1]: the just-now is 1 and
// an hour-old moment is half as near.
func recencyOf(now, at time.Time) float64 {
	age := now.Sub(at).Hours()
	if age < 0 {
		age = 0
	}
	return 1 / (1 + age)
}

// signalOf reads one signal off a destination.
func signalOf(dst walkDest, sig string) float64 {
	switch sig {
	case sigStress:
		return dst.stress
	case sigRecency:
		return dst.recency
	case sigSaliency:
		return 0 // saliency lives on deliberation, not on plain destinations
	default:
		return dst.weight
	}
}

// walkThought is the think primitive: one operation applied to the
// snapshot. It picks the destination its signal ranks highest, renders
// the thought, and prices it for the arbiter. Deliberation, the
// (grounds, saliency) setting, reads the prefetched plan instead of a
// destination list and only ever thinks that something is runnable.
func walkThought(sn thoughtSnapshot, op thoughtOp) thoughtCandidate {
	none := thoughtCandidate{epoch: sn.epoch, op: op.template()}
	if op.rel == memory.RelGrounds && op.sig == sigSaliency {
		if !sn.plan.ok || sn.plan.conflict {
			return none
		}
		saliency := sn.plan.saliency
		if saliency > 1 {
			saliency = 1
		}
		return thoughtCandidate{
			epoch: sn.epoch, op: op.template(), ok: true,
			content: fmt.Sprintf("thought: could run %s", sn.plan.command),
			seed:    sn.seed, seedHeat: sn.seedHeat,
			strength: opPriority(sn, op, saliency) * sn.seedHeat,
		}
	}

	dests := sn.dests[op.rel]
	best, bestSig := -1, 0.0
	for i, dst := range dests {
		if s := signalOf(dst, op.sig); s > bestSig {
			best, bestSig = i, s
		}
	}
	if best < 0 {
		return none
	}
	top := dests[best]

	var warm []uint64
	switch op.rel {
	case memory.RelTemporal:
		// Free association fans out a little, so a chain can branch.
		for _, dst := range dests {
			if len(warm) == thinkFanout {
				break
			}
			if dst.id != 0 {
				warm = append(warm, dst.id)
			}
		}
	default:
		if top.id != 0 {
			warm = append(warm, top.id)
		}
	}

	return thoughtCandidate{
		epoch: sn.epoch, op: op.template(), ok: true,
		content: renderThought(op, sn.seedGloss, top),
		seed:    sn.seed, seedHeat: sn.seedHeat, warm: warm,
		strength: opPriority(sn, op, bestSig) * sn.seedHeat,
	}
}

// renderThought is the thought's face in perception, shaped by the
// relation that carried it: order speaks as expectation, co-occurrence
// as association, a grounding as a remembered moment (or, for an
// abstract concept's constellation, as association again), a shared
// signature as resemblance, and a name as vocabulary.
func renderThought(op thoughtOp, seedGloss string, dst walkDest) string {
	switch {
	case op.rel == memory.RelSuccessor:
		return fmt.Sprintf("thought: after %s, %s", seedGloss, dst.gloss)
	case op.rel == memory.RelNames:
		return fmt.Sprintf("thought: %s is called %s", seedGloss, dst.gloss)
	case op.rel == memory.RelFeature:
		return fmt.Sprintf("thought: %s is like %s", seedGloss, dst.gloss)
	case op.rel == memory.RelPatient:
		if dst.via != "" {
			return fmt.Sprintf("thought: %s %s %s", seedGloss, dst.via, dst.gloss)
		}
		return fmt.Sprintf("thought: %s acts on %s", seedGloss, dst.gloss)
	case op.rel == memory.RelAgent:
		if dst.via != "" {
			return fmt.Sprintf("thought: %s %s %s", dst.gloss, dst.via, seedGloss)
		}
		return fmt.Sprintf("thought: %s acts on %s", dst.gloss, seedGloss)
	case op.rel == memory.RelGrounds && dst.kind != memory.KindConcept:
		return fmt.Sprintf("thought: that time: %s", dst.gloss)
	default:
		return fmt.Sprintf("thought: %s ~ %s", seedGloss, dst.gloss)
	}
}

// absorbThought reabsorbs one operation's candidate; when the round is
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
			(c.strength == winner.strength && c.op < winner.op) {
			winner = c
		}
	}
	d.ws.cool(winner.seed, thinkFatigue)
	// An act that won the round is performed rather than re-entered: the
	// loop fires the faculty the way a CLI verb would, single-threaded.
	if name, ok := actNameOf(winner.op); ok {
		d.performAct(name, winner)
		return
	}
	// A thought already re-entered in the recent rounds is rumination. A
	// repeat is the most barren thought there is, so wear its operation the
	// way any routine step does, but neither re-encode nor re-broadcast it,
	// so one line cannot pile up as memory.
	if d.thoughtRuminates(winner.content) {
		d.noteThoughtFruit(winner, wire.ObserveResult{})
		return
	}
	for _, id := range winner.warm {
		d.ws.excite(id, winner.seedHeat*thinkDamping)
	}
	d.lastThought = winner.content
	d.lastThoughtAt = time.Now()
	d.rememberThought(winner.content)
	res := d.process(senses.Observation{Channel: thoughtChannel, Content: winner.content, At: time.Now()}, false, true)
	d.noteThoughtFruit(winner, res)
}

// thoughtRuminates reports whether this exact thought re-entered perception
// within the recent refractory window.
func (d *Daemon) thoughtRuminates(content string) bool {
	for _, t := range d.recentThoughts {
		if t == content {
			return true
		}
	}
	return false
}

// rememberThought records a re-entered thought in the bounded refractory
// ring, evicting the oldest.
func (d *Daemon) rememberThought(content string) {
	d.recentThoughts = append(d.recentThoughts, content)
	if len(d.recentThoughts) > thoughtRefractory {
		d.recentThoughts = d.recentThoughts[len(d.recentThoughts)-thoughtRefractory:]
	}
}

// noteOpFruit is the operant loop turned inward: an operation whose
// product was worth keeping (an encoded insight, an act that surfaced or
// changed something) gains worth, and a barren one wears a little. The
// learned worth lives as the operation node's weight, bounded on both
// sides. A fruitful thought also takes a grounds edge from its operation
// to the insight, so the trail shows which operation produced it.
func (d *Daemon) noteOpFruit(tpl string, fruitful bool, insightID uint64) {
	node := d.opNode(tpl)
	if node == nil {
		return
	}
	if fruitful {
		node.Weight += opFruitDelta
		if node.Weight > opWeightCeil {
			node.Weight = opWeightCeil
		}
		if insightID != 0 {
			if err := d.store.LinkRel(node.ID, insightID, memory.RelGrounds, groundsWeight); err != nil {
				fmt.Fprintf(d.out, "thought grounding error: %v\n", err)
			}
			d.event(events.Event{Ev: "thought", Tpl: tpl, ID: insightID,
				Weight: node.Weight, Outcome: "insight"})
		}
	} else {
		node.Weight -= opBarrenDelta
		if node.Weight < opWeightFloor {
			node.Weight = opWeightFloor
		}
	}
	if err := d.store.SetWeight(node.ID, node.Weight); err != nil {
		fmt.Fprintf(d.out, "thought worth error: %v\n", err)
	}
}

// noteThoughtFruit judges one committed thought: encoded means insight,
// anything else means routine wear.
func (d *Daemon) noteThoughtFruit(winner thoughtCandidate, res wire.ObserveResult) {
	d.noteOpFruit(winner.op, res.Conscious && res.FragmentID != 0, res.FragmentID)
}

// opNodes lazily loads the operation nodes the graph holds, keyed by
// template. Operations are minted on first use, so the map starts with
// whatever earlier runs learned.
func (d *Daemon) opNodes() map[string]*memory.Fragment {
	if d.ops == nil {
		d.ops = make(map[string]*memory.Fragment)
		if nodes, err := d.store.NodesByKind(memory.KindOperation); err == nil {
			for _, n := range nodes {
				d.ops[n.Template] = n
			}
		} else {
			fmt.Fprintf(d.out, "operation load error: %v\n", err)
		}
	}
	return d.ops
}

// opNode resolves an operation's node, minting it at baseline worth on
// first use. The operation space itself is innate; the node is where its
// learned worth and its names live, inspectable like everything else.
func (d *Daemon) opNode(tpl string) *memory.Fragment {
	ops := d.opNodes()
	if n, ok := ops[tpl]; ok {
		return n
	}
	gloss, ok := opGloss(tpl)
	if !ok {
		return nil
	}
	n := &memory.Fragment{
		Kind:     memory.KindOperation,
		Channel:  "mind",
		Content:  gloss,
		Template: tpl,
		Category: "verb",
		Limbic:   d.chem.Snapshot(),
		Time:     time.Now(),
	}
	if _, err := d.store.Encode(n); err != nil {
		fmt.Fprintf(d.out, "operation mint error: %v\n", err)
		return nil
	}
	ops[tpl] = n
	return n
}

// opGloss resolves any operation template, walk or act, to its speakable
// face, refusing shapes outside the innate space.
func opGloss(tpl string) (string, bool) {
	if op, ok := opByTemplate(tpl); ok {
		return op.gloss(), true
	}
	if name, ok := actNameOf(tpl); ok {
		return name, true
	}
	return "", false
}

// opByTemplate parses a walk template back into its setting, refusing
// shapes outside the innate space.
func opByTemplate(tpl string) (thoughtOp, bool) {
	body, ok := strings.CutPrefix(tpl, "walk(")
	if !ok || !strings.HasSuffix(body, ")") {
		return thoughtOp{}, false
	}
	rel, sig, ok := strings.Cut(strings.TrimSuffix(body, ")"), ",")
	if !ok {
		return thoughtOp{}, false
	}
	gains, ok := opGains[rel]
	if !ok {
		return thoughtOp{}, false
	}
	if _, ok := gains[sig]; !ok {
		return thoughtOp{}, false
	}
	return thoughtOp{rel: rel, sig: sig}, true
}

// opPolicy gathers the per-operation policy state for a round: learned
// worth from the operation nodes, and the workspace warmth of every
// concept taught to name an operation, so a spoken "worry" leans the
// next thought toward the operation it names.
func (d *Daemon) opPolicy() (worth, heat map[string]float64) {
	ops := d.opNodes()
	worth = make(map[string]float64, len(ops))
	heat = make(map[string]float64, len(ops))
	if len(ops) == 0 {
		return worth, heat
	}
	ids := make([]uint64, 0, len(ops))
	byID := make(map[uint64]string, len(ops))
	for tpl, n := range ops {
		worth[tpl] = n.Weight
		ids = append(ids, n.ID)
		byID[n.ID] = tpl
	}
	into, err := d.store.LinksToRel(ids, memory.RelGrounds)
	if err != nil {
		return worth, heat
	}
	for _, e := range into {
		if h := d.ws.heat(e.From); h > heat[byID[e.To]] {
			heat[byID[e.To]] = h
		}
	}
	return worth, heat
}

// teachThought is the mental-teach: as teach grounds a cue to a command,
// this grounds a concept to a mental operation and bonds the word to
// that concept, so a word can name a kind of thinking ("worry" for
// grounding by stress, "expect" for the successor walk) or a whole
// faculty ("tidy" for the consolidate act), and saying it leans the
// inner loop toward that operation.
func (d *Daemon) teachThought(word, rel, sig, lang string) error {
	var tpl string
	switch {
	case rel == relAct:
		if !slices.Contains(actNames, sig) {
			return fmt.Errorf("unknown act %q (one of: %s)", sig, strings.Join(actNames, ", "))
		}
		tpl = actOp{name: sig}.template()
	case opGains[rel] == nil:
		return fmt.Errorf("unknown relation %q (one of: %s, %s)", rel,
			strings.Join(thinkRels, ", "), relAct)
	case !slices.Contains(thinkSigs, sig):
		return fmt.Errorf("unknown signal %q (one of: %s)", sig, strings.Join(thinkSigs, ", "))
	default:
		tpl = thoughtOp{rel: rel, sig: sig}.template()
	}
	node := d.opNode(tpl)
	if node == nil {
		return fmt.Errorf("the operation could not be minted; check the daemon log")
	}

	cTpl := "op:" + tpl
	c, err := d.store.ConceptByTemplate(cTpl)
	if err != nil {
		return err
	}
	if c == nil {
		c = &memory.Fragment{
			Kind:     memory.KindConcept,
			Channel:  "concept",
			Content:  node.Content,
			Template: cTpl,
			Category: "verb",
			Limbic:   d.chem.Snapshot(),
			Time:     time.Now(),
		}
		if _, err := d.store.Encode(c); err != nil {
			return err
		}
		d.event(events.Event{Ev: "concept", ID: c.ID, Tpl: cTpl, Outcome: "operation"})
	}
	if err := d.store.LinkRel(c.ID, node.ID, memory.RelGrounds, opNameWeight); err != nil {
		return err
	}
	if lang == "" {
		lang = "en"
	}
	w, _, err := d.store.WordNode(strings.ToLower(word), lang, d.chem.Snapshot(), time.Now())
	if err != nil {
		return err
	}
	if err := d.store.SetLinkRel(w.ID, c.ID, memory.RelNames, opNameWeight); err != nil {
		return err
	}
	d.event(events.Event{Ev: "name", From: w.ID, To: c.ID, Rel: memory.RelNames, Weight: opNameWeight})
	// Naming a kind of thinking is talking about it: the concept warms,
	// and the next quiet stretch leans its way.
	d.ws.excite(c.ID, exciteSpeech)
	return nil
}

// settleThinking completes the open think round the way the Run loop
// would, for loopless drivers: demos and tests.
func (d *Daemon) settleThinking() {
	for d.thinking > 0 {
		d.absorbThought(<-d.thinkDone)
	}
}
