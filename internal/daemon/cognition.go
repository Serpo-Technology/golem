package daemon

import (
	"fmt"
	"sort"
	"time"

	"golem/internal/memory"
	"golem/wire"
)

// cognitionPairCap bounds the resemblance pairs the inspection view reports, the
// strongest kept, so a large brain returns a readable summary rather than every
// pair it holds.
const cognitionPairCap = 40

// cognitionInfo reads GOLEM's reasoning state for the inspection view: how
// richly the concepts resemble one another, and what it believes and has kept.
// Resemblance is the headline because it is the gate, an inference fires only
// when a pair crosses the floor, so a growing count above the floor is a brain
// becoming able to reason. The held beliefs are working memory's inferred layer,
// the promoted ones long-term memory's, both held apart from the observed graph.
func (d *Daemon) cognitionInfo() (*wire.CognitionInfo, error) {
	concepts, err := d.store.Concepts()
	if err != nil {
		return nil, err
	}
	featCount, _ := d.store.CountKind(memory.KindFeature)
	content := make(map[uint64]string, len(concepts))
	for _, c := range concepts {
		content[c.ID] = c.Content
	}
	// Label a concept by its own content, not by the word that names it: naming
	// can bond many words to one concept, so naming-based labels collapse
	// distinct concepts onto a shared name and hide what actually resembles what.
	gloss := func(id uint64) string {
		if c := content[id]; c != "" {
			return c
		}
		return d.conceptName(id, "", "en")
	}

	// Resemblance, the stronger of the two directions for each pair, so the report
	// shows how near the concepts sit and how many cross the floor to reason on.
	now := time.Now()
	best := map[[2]uint64]float64{}
	for _, c := range concepts {
		sibs, err := d.featureSiblings(c.ID, now)
		if err != nil {
			continue
		}
		for _, s := range sibs {
			key := pairKey(c.ID, s.id)
			if s.weight > best[key] {
				best[key] = s.weight
			}
		}
	}
	pairs := make([]*wire.ResemblancePair, 0, len(best))
	above := 0
	for key, w := range best {
		pairs = append(pairs, &wire.ResemblancePair{A: gloss(key[0]), B: gloss(key[1]), Weight: w})
		if w >= inferenceFloor {
			above++
		}
	}
	sort.Slice(pairs, func(i, j int) bool { return pairs[i].Weight > pairs[j].Weight })
	if len(pairs) > cognitionPairCap {
		pairs = pairs[:cognitionPairCap]
	}

	// The features named universal, the ones resemblance passes over, so the
	// reader can see why a dense feature graph is not a dense resemblance.
	var universal []string
	if len(d.universalFeats) > 0 {
		fids := make([]uint64, 0, len(d.universalFeats))
		for id := range d.universalFeats {
			fids = append(fids, id)
		}
		if feats, err := d.store.FragmentsByIDs(fids); err == nil {
			sort.Slice(feats, func(i, j int) bool {
				return d.universalFeats[feats[i].ID] > d.universalFeats[feats[j].ID]
			})
			for _, f := range feats {
				universal = append(universal, fmt.Sprintf("%s %.2f", f.Content, d.universalFeats[f.ID]))
			}
		}
	}

	var held []*wire.BeliefInfo
	for _, p := range d.held.props {
		held = append(held, d.beliefInfo(p))
	}
	sort.SliceStable(held, func(i, j int) bool { return held[i].Conf > held[j].Conf })

	var promoted []*wire.BeliefInfo
	if bs, err := d.store.Beliefs(); err == nil {
		for _, b := range bs {
			promoted = append(promoted, &wire.BeliefInfo{
				Agent: d.conceptName(b.Agent, b.AgentGloss, "en"), Rel: b.Rel,
				Patient: d.conceptName(b.Patient, b.PatGloss, "en"),
				Conf:    b.Conf, Depth: b.Depth, Status: "inferred", Kept: true})
		}
	}

	return &wire.CognitionInfo{
		Concepts:    len(concepts),
		Features:    featCount,
		Floor:       inferenceFloor,
		AboveFloor:  above,
		Resemblance: pairs,
		Universal:   universal,
		Held:        held,
		Promoted:    promoted,
	}, nil
}

// beliefInfo renders one held proposition for the wire, naming its arguments in
// English and reporting how it is held.
func (d *Daemon) beliefInfo(p heldProp) *wire.BeliefInfo {
	status := "inferred"
	if p.status == heldHypothetical {
		status = "hypothetical"
	}
	return &wire.BeliefInfo{
		Agent:   d.conceptName(p.agent, p.agentGloss, "en"),
		Rel:     p.rel,
		Patient: d.conceptName(p.patient, p.patientGloss, "en"),
		Conf:    p.conf,
		Depth:   p.depth,
		Status:  status,
		Voiced:  p.voiced,
		Kept:    p.persistent,
	}
}

// pairKey orders two ids so a concept pair has one key whichever way it is read.
func pairKey(a, b uint64) [2]uint64 {
	if a < b {
		return [2]uint64{a, b}
	}
	return [2]uint64{b, a}
}
