package daemon

import (
	"math"

	"golem/internal/memory"
	"golem/internal/vsa"
)

// The holographic analogy is the third and weakest way GOLEM completes a proportion a:b::c:?, reached
// only where the symbolic relation and the kind-space offset both come up empty. It maps the frame of
// one entity onto another's by binding algebra alone: a is a filler in b's frame, and the filler that
// plays the same role in c's frame is the fourth term -- the dollar of America answered by the peso of
// Mexico, with no relation named and no edge walked. It is a pure vector PROPOSAL the graph cannot
// dispose on (no told fact connects the pair), so it is always hedged, always grounded in a real
// concept the codebook names, floor-gated, and never written to the graph.

// A holographic answer must stand out as an OUTLIER against the whole codebook, not merely clear a
// fixed cosine. The cleanup cosine of a correct structural map is set by the frame's richness and does
// not grow with the brain, but the runner-up is an order statistic over the codebook's random atoms
// and rises with the concept count (~0.07 over a dozen, ~0.15 over a thousand). A fixed margin
// calibrated on a small brain would therefore make the mode inert on a large one. So the gate is
// stated in the codebook's own units: the top match must sit many standard deviations above the mean
// candidate cosine (a clear outlier, not the tail of the noise) AND stand clear of the runner-up by
// enough of those deviations that it is alone up there. A near-zero query names nothing regardless.
const (
	vsaMinCosine   = 0.05 // a query with no real signal proposes nothing, whatever its z-score
	vsaZFloor      = 4.0  // the top must sit this many std devs above the mean candidate cosine
	vsaZMargin     = 1.2  // and clear the runner-up by this many, so it stands alone above the noise
	vsaMinCodebook = 6    // too few candidates to estimate the noise distribution: no confident outlier
)

// holoAnalogy completes a:b::c:? by mapping b's frame onto c's: it binds a's atom into c's record and
// unbinds by b's record, so the role that carries a in b leaves its counterpart filler in c, then
// cleans that noisy vector to the concept that stands out from the codebook. ok is false when either
// frame is unencoded (nothing to map between), a stored record's width has gone stale, or no candidate
// stands out as a clear outlier (the honest gap, never a forced guess). The returned id is always a
// real concept the codebook names, never fabricated; the query reads records and derives atoms, and
// writes nothing.
func (d *Daemon) holoAnalogy(a, b, c *memory.Fragment) (uint64, bool) {
	if a == nil || b == nil || c == nil {
		return 0, false
	}
	recB, okB := d.store.Holo(b.ID)
	recC, okC := d.store.Holo(c.ID)
	if !okB || !okC {
		return 0, false // without both frames there is nothing to map between
	}
	if len(recB) != holoDim || len(recC) != holoDim {
		return 0, false // a record from a prior width is not one to bind against: gap until re-encoded
	}
	// Bind a into c's frame, unbind by b's frame: the role that binds a in b is the role whose filler
	// in c falls out, plus the noise cleanup removes.
	query := vsa.Unbind(vsa.Bind(d.holoAtom(a.ID), f32ToHV(recC)), f32ToHV(recB))

	concepts, err := d.grownConcepts()
	if err != nil {
		return 0, false
	}
	exclude := map[uint64]bool{a.ID: true, b.ID: true, c.ID: true}
	var topID uint64
	topScore, secondScore := math.Inf(-1), math.Inf(-1)
	var sum, sumSq float64
	n := 0
	for _, cand := range concepts {
		if exclude[cand.ID] {
			continue // the proportion's own terms are never its fourth term
		}
		score := vsa.Cosine(query, d.holoAtom(cand.ID))
		sum += score
		sumSq += score * score
		n++
		switch {
		case score > topScore:
			secondScore = topScore
			topScore = score
			topID = cand.ID
		case score > secondScore:
			secondScore = score
		}
	}
	if topID == 0 || n < vsaMinCodebook || topScore < vsaMinCosine {
		return 0, false
	}
	mean := sum / float64(n)
	variance := sumSq/float64(n) - mean*mean
	if variance <= 0 {
		return 0, false // a degenerate codebook (all alike) can name no outlier
	}
	std := math.Sqrt(variance)
	if (topScore-mean)/std < vsaZFloor || (topScore-secondScore)/std < vsaZMargin {
		return 0, false // not a clear outlier above the codebook noise: the honest gap
	}
	return topID, true
}
