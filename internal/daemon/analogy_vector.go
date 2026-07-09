package daemon

import (
	"strings"

	"golem/internal/memory"
)

// A proportion a:b::c:? can hold on a regularity no told relation names -- king is to queen as
// man is to woman, where the graph carries no edge from one to the other. The symbolic analogy
// (an explicit relation a->b applied to c) finds nothing there. The kind space can: if the pair
// a->b share an offset, the fourth term is the concept nearest c plus that offset, the vector
// arithmetic that reads a regularity off the geometry. It is a pure vector PROPOSAL -- the graph
// cannot dispose on an unnamed regularity, so it is never firm -- and it serves only where the
// grounded symbolic path came up empty, spoken at the weaker "perhaps" shade a thin guess reads
// at. Grounded in an actual concept and gated by a nearness floor, so it proposes a real fourth
// term or admits the gap, never fabricates one.

// analogyOffsetFloor is how near the fourth-term concept must sit to the offset target to be
// proposed; below it the geometry names no clear fourth term and the honest answer is the gap.
const analogyOffsetFloor = 0.5

// vectorAnalogy completes a:b::c:? by the offset the first pair shares in the kind space: the
// concept nearest c + (b - a), the analogy terms themselves excluded. ok is false when a term has
// no kind vector (a featureless abstract word cannot ride the geometry) or nothing sits near
// enough to the offset target.
func (d *Daemon) vectorAnalogy(a, b, c *memory.Fragment) (uint64, bool) {
	va, oka := d.store.Vector(a.ID)
	vb, okb := d.store.Vector(b.ID)
	vc, okc := d.store.Vector(c.ID)
	if !oka || !okb || !okc || len(va) != len(vb) || len(va) != len(vc) {
		return 0, false
	}
	target := make([]float32, len(va))
	for i := range target {
		target[i] = vc[i] + vb[i] - va[i]
	}
	matches, err := d.store.NearestVectors(target, memory.KindConcept, 4, c.ID)
	if err != nil {
		return 0, false
	}
	for _, m := range matches {
		if m.ID == a.ID || m.ID == b.ID {
			continue // the proportion's own terms are not its fourth term
		}
		if m.Score < analogyOffsetFloor {
			return 0, false // nothing near the offset target is an honest gap, not a forced guess
		}
		return m.ID, true
	}
	return 0, false
}

// sayVectorAnalogy speaks a vector-offset proportion's fourth term as a thin guess: "Perhaps the
// woman." -- the weaker hedge shade, since it rests on the geometry alone with no fact to firm it.
// ok is false when the fourth term has no name in the tongue: a nameless concept is no fourth term
// to offer, so the caller admits the gap rather than speak an empty "Perhaps the ."
func (d *Daemon) sayVectorAnalogy(lang string, dID uint64) (string, bool) {
	name := strings.TrimSpace(d.conceptName(dID, "", lang))
	if name == "" {
		return "", false
	}
	hedge := d.gram.Word(lang, "maybe")
	if hedge == "" {
		hedge = "perhaps"
	}
	return upperFirst(hedge) + " " + d.coordinatedNP(lang, []string{name}) + ".", true
}
