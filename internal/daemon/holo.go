package daemon

import (
	"fmt"
	"hash/fnv"
	"math/rand"
	"strconv"

	"golem/internal/memory"
	"golem/internal/vsa"
)

// GOLEM answers a proportion two ways already: symbolically, by finding the told relation that holds
// a->b and applying it to c; and by the offset a shared regularity leaves in the kind space. Both
// miss the STRUCTURAL analogy -- the dollar of America answered by the peso of Mexico -- where no one
// relation connects the pair and no single axis carries the move, yet both entities carry the SAME
// frame of roles toward DIFFERENT fillers. Holographic binding holds that frame in one vector: a
// concept's record is its roles bound to their fillers and superposed, and the fourth term falls out
// of mapping one record onto another by the algebra, no edge walked, no role named. It is the third
// and weakest analogy mode, a pure vector proposal the graph cannot dispose on, so it is only ever a
// hedged guess. This file grows the records nightly; analogy_holo.go runs the query and speaks it.

const (
	// holoDim is the width of the holographic space. A bundle recovers its fillers cleanly well past
	// any real frame's size at this width (see internal/vsa capacity), and the bind cost that scales
	// with it is paid only at the nightly encode, never on a query.
	holoDim = 512

	// A concept earns a record only when its frame is rich enough to map FROM: at least this many
	// fillers across at least this many distinct relations. A chain (one relation, many fillers) or a
	// hub is not a mappable frame, so both fall below the bar and stay unencoded -- which is also what
	// bounds the nightly bind cost, since most concepts do not qualify.
	holoMinFillers   = 2
	holoMinRelations = 2
)

// holoSeed derives a stable 64-bit seed from named parts, so an atom is the same random draw every
// night forever: the codebook a query cleans against is reconstructed on demand, never stored, and so
// never drifts.
func holoSeed(parts ...string) int64 {
	h := fnv.New64a()
	for _, p := range parts {
		h.Write([]byte(p))
		h.Write([]byte{0})
	}
	return int64(h.Sum64())
}

// holoAtom is a concept's atom in the holographic codebook: a deterministic random vector keyed to
// its fragment id, so cleanup can only ever name a concept that exists (grounded, never fabricated).
func (d *Daemon) holoAtom(id uint64) vsa.HV {
	return vsa.New(holoDim, rand.New(rand.NewSource(holoSeed("concept", strconv.FormatUint(id, 10)))))
}

// holoRoleAtom is the binding key for one role of one relation: the slot a co-argument fills of a
// given relation. Keyed on the relation id and the slot, so two entities related the same way share
// the identical key -- the algebraic condition that lets a mapping cancel the shared role and leave
// the analogous filler. A symmetric relation collapses its two slots to one undirected key.
func (d *Daemon) holoRoleAtom(relID uint64, slot string) vsa.HV {
	return vsa.New(holoDim, rand.New(rand.NewSource(holoSeed("role", strconv.FormatUint(relID, 10), slot))))
}

// buildHoloRecord bundles a concept's frame into one holographic record: for each proposition it
// stands in, the co-argument's atom bound under the role that co-argument fills of that relation. It
// returns the record and the counts a caller gates on (distinct fillers, distinct relations); a
// concept whose frame is too thin to map from returns a nil record. The walk reuses gatherFacts, so a
// denied proposition is already filtered out and never enters the frame; determinism rests on the
// fact set being graph-determined (a discourse-thread fact is a committed proposition, so it adds no
// role-filler pair the graph walk does not already carry).
func (d *Daemon) buildHoloRecord(c *memory.Fragment) (rec vsa.HV, fillers, relations int) {
	type pair struct {
		relID  uint64
		slot   string
		filler uint64
	}
	var pairs []pair
	seenFiller := map[uint64]bool{}
	seenRel := map[uint64]bool{}
	for _, f := range d.gatherFacts(c, "", "") {
		rel := d.resolveRelation(f.rel)
		if rel == nil {
			continue // a relation whose word names no concept cannot key an atom
		}
		slot := memory.RelAgent // the focus is the patient, so the co-argument fills the agent slot
		if f.focusRole == "agent" {
			slot = memory.RelPatient // the focus is the agent, so the co-argument fills the patient slot
		}
		if d.relationIsSymmetric(rel.ID) {
			slot = "sym" // a symmetric relation binds both directions under one undirected role
		}
		pairs = append(pairs, pair{rel.ID, slot, f.coID})
		seenFiller[f.coID] = true
		seenRel[rel.ID] = true
	}
	// Gate before binding, so the O(d^2) convolution is never paid on a frame that will not be encoded.
	if len(seenFiller) < holoMinFillers || len(seenRel) < holoMinRelations {
		return nil, len(seenFiller), len(seenRel)
	}
	binds := make([]vsa.HV, len(pairs))
	for i, p := range pairs {
		binds[i] = vsa.Bind(d.holoRoleAtom(p.relID, p.slot), d.holoAtom(p.filler))
	}
	return vsa.Bundle(binds...), len(seenFiller), len(seenRel)
}

// growHolographic re-encodes every concept's holographic record from its current frame, the third
// nightly embedding beside the kind and usage spaces. It runs on a settled graph, so a record always
// reflects the propositions that survived the night; a concept whose frame has thinned below the
// mapping threshold has its stale record dropped. Determinism is free: atoms are pure functions of id
// and slot, so an unchanged frame re-encodes to the same bytes.
func (d *Daemon) growHolographic() {
	concepts, err := d.grownConcepts()
	if err != nil {
		fmt.Fprintf(d.out, "holographic error: %v\n", err)
		return
	}
	// Encode from the SETTLED GRAPH, not the discourse-thread-overlaid view. gatherFacts folds in
	// d.threadProps and marks each relation-role the day's conversation touched as covered, which
	// SUPPRESSES the graph walk of that relation-role and so drops any sibling filler the thread did
	// not mention -- making a record depend on the last exchange. The sleep path is single-writer, so
	// clearing the thread for the encode and restoring it after is safe, and leaves the record a pure
	// function of the graph.
	savedThread := d.threadProps
	d.threadProps = nil
	defer func() { d.threadProps = savedThread }()

	built, totalFillers := 0, 0
	for _, c := range concepts {
		rec, fillers, _ := d.buildHoloRecord(c)
		if rec == nil {
			// Not a mappable frame (or no longer one): keep no stale record for a query to read.
			d.store.DeleteHolo(c.ID)
			continue
		}
		if err := d.store.SetHolo(c.ID, hvToF32(rec)); err != nil {
			fmt.Fprintf(d.out, "holographic error: %v\n", err)
			return
		}
		built++
		totalFillers += fillers
	}
	// A concept pruned this same night is gone from the concept list above, so its record was never
	// revisited; sweep those orphans so the table tracks the living graph rather than accreting rows.
	if err := d.store.SweepHoloOrphans(); err != nil {
		fmt.Fprintf(d.out, "holographic error: %v\n", err)
	}
	if built > 0 {
		fmt.Fprintf(d.out, "encoded %d holographic record(s) over %d filler(s)\n", built, totalFillers)
	}
}

// hvToF32 narrows a hypervector to the float32 the store keeps; the dominant holographic noise is
// bundle crosstalk, not float precision, so the narrower store costs nothing measurable against it.
func hvToF32(v vsa.HV) []float32 {
	out := make([]float32, len(v))
	for i, x := range v {
		out[i] = float32(x)
	}
	return out
}

// f32ToHV widens a stored record back to a hypervector for the algebra.
func f32ToHV(v []float32) vsa.HV {
	out := make(vsa.HV, len(v))
	for i, x := range v {
		out[i] = float64(x)
	}
	return out
}
