package daemon

import (
	"fmt"
	"math"
	"strconv"

	"golem/internal/memory"
	"gonum.org/v1/gonum/mat"
)

// embedDim caps the width of the learned space; the data caps it lower, since a
// brain with few concepts or few traits yields fewer real directions.
const embedDim = 32

// categoryTraitWeight is how strongly belonging to a category enters the matrix,
// against a feature's damped log1p (around 1). Membership is a stronger likeness
// signal than any single feature, so it weighs more: enough that concepts of one
// kind resemble each other above the blend floor even with no shared features (a
// golden lion finds the dog through "animal"), without so much that within-kind
// feature structure collapses. Calibrated so same-category concepts land near 0.6
// cosine, comfortably over the floor with room to spare.
const categoryTraitWeight = 6.0

// growEmbeddings learns each concept's place in a continuous space from its own
// grounded company, no outside model. It is LSA over the concept-by-trait
// matrix: factor that matrix, keep its leading directions, and a concept's
// coordinates are its row in the left factor. The traits are the concept's whole
// grounded company, not features alone: the features it carries, the categories
// it belongs to, and the relational roles it fills. Drawing on all three lets
// the latent factors place concepts that share a category or a use near each
// other even when their surface features do not overlap, the second-order
// resemblance that feature-token overlap misses, so a sparsely-featured concept
// still finds its kind. The space is opaque in its axes but grounded in its
// source: every coordinate traces back to a grounded edge, which traces back to
// lived moments. This is the proposer the brain grows for itself. It runs each
// night after the graph settles; under two concepts or with no traits it writes
// nothing.
func (d *Daemon) growEmbeddings() {
	concepts, err := d.store.Concepts()
	if err != nil {
		fmt.Fprintf(d.out, "embedding error: %v\n", err)
		return
	}
	if len(concepts) < 2 {
		return
	}
	ids := make([]uint64, len(concepts))
	rowOf := make(map[uint64]int, len(concepts))
	for i, c := range concepts {
		ids[i] = c.ID
		rowOf[c.ID] = i
	}

	colOf := map[string]int{}
	val := map[[2]int]float64{} // (row, col) -> final matrix value, strongest wins
	put := func(conceptID uint64, trait string, v float64) {
		r, ok := rowOf[conceptID]
		if !ok || v <= 0 {
			return
		}
		c, ok := colOf[trait]
		if !ok {
			c = len(colOf)
			colOf[trait] = c
		}
		if k := [2]int{r, c}; v > val[k] {
			val[k] = v
		}
	}

	// A feature the concept carries, the first-order trait. log1p tempers a
	// dominant feature, the count-based embedding's damping of raw frequency so one
	// loud trait does not swamp the geometry.
	if feats, err := d.store.LinksFromRel(ids, memory.RelFeature); err == nil {
		for _, e := range feats {
			put(e.From, "feat:"+strconv.FormatUint(e.To, 10), math.Log1p(e.Weight))
		}
	}
	// A category the concept belongs to: the abstraction that grounds in it. This
	// is the likeness feature overlap misses, so every animal shares one strong
	// column and the factors place them near regardless of their surface features.
	if cats, err := d.store.LinksToRel(ids, memory.RelGrounds); err == nil {
		for _, e := range cats {
			put(e.To, "cat:"+strconv.FormatUint(e.From, 10), categoryTraitWeight)
		}
	}

	if len(colOf) == 0 {
		return
	}
	nC, nF := len(concepts), len(colOf)
	data := make([]float64, nC*nF)
	for rc, v := range val {
		data[rc[0]*nF+rc[1]] = v
	}

	var svd mat.SVD
	if !svd.Factorize(mat.NewDense(nC, nF, data), mat.SVDThinU) {
		fmt.Fprintf(d.out, "embedding error: factorization did not converge\n")
		return
	}
	var u mat.Dense
	svd.UTo(&u)
	sv := svd.Values(nil)
	k := embedDim
	if len(sv) < k {
		k = len(sv)
	}
	if k == 0 {
		return
	}
	vecByID := make(map[uint64][]float32, len(concepts))
	for i, c := range concepts {
		vec := make([]float32, k)
		for j := 0; j < k; j++ {
			// Scaling a direction by the root of its singular value weights the
			// dominant factors, the standard latent-semantic coordinate.
			vec[j] = float32(u.At(i, j) * math.Sqrt(sv[j]))
		}
		vecByID[c.ID] = vec
		if err := d.store.SetVector(c.ID, vec); err != nil {
			fmt.Fprintf(d.out, "embedding error: %v\n", err)
			return
		}
	}
	d.embedRelationsByFillers(concepts, vecByID, k)
}

// embedRelationsByFillers places each relation among the concepts it relates. A
// relation carries no feature and belongs to no category, so the factorization
// leaves its row at the origin and its embedding is useless. Here it takes the
// average of its role-fillers' coordinates, the agents and patients of the
// moments it grounds in, so it lands in the same space as the things it takes as
// arguments. A clause whose verb names no relation can then propose the relation
// whose company its arguments most resemble. This runs after the feature-and-
// category coordinates are set and overwrites only relation vectors, so the
// geometry the resemblance path already leans on is left exactly as it was.
func (d *Daemon) embedRelationsByFillers(concepts []*memory.Fragment, vecByID map[uint64][]float32, k int) {
	for _, c := range concepts {
		if c.Category != "relation" {
			continue
		}
		centroid := make([]float64, k)
		n := 0
		for _, id := range d.relationFillers(c.ID) {
			v, ok := vecByID[id]
			if !ok || len(v) != k {
				continue
			}
			for j := 0; j < k; j++ {
				centroid[j] += float64(v[j])
			}
			n++
		}
		if n == 0 {
			continue // a relation never yet given an embedded argument stays unplaced
		}
		vec := make([]float32, k)
		for j := 0; j < k; j++ {
			vec[j] = float32(centroid[j] / float64(n))
		}
		if err := d.store.SetVector(c.ID, vec); err != nil {
			fmt.Fprintf(d.out, "embedding error: %v\n", err)
			return
		}
	}
}

// relationFillers returns the concepts that have filled a relation's roles: the
// agents and patients of the moments the relation grounds in. The same concept
// recurs once per moment it filled, so a frequent argument weighs more in the
// centroid, the usage the placement should reflect.
func (d *Daemon) relationFillers(relID uint64) []uint64 {
	moments, err := d.store.LinksFromRel([]uint64{relID}, memory.RelGrounds)
	if err != nil || len(moments) == 0 {
		return nil
	}
	momIDs := make([]uint64, 0, len(moments))
	for _, e := range moments {
		momIDs = append(momIDs, e.To)
	}
	var fillers []uint64
	for _, role := range []string{memory.RelAgent, memory.RelPatient} {
		edges, err := d.store.LinksFromRel(momIDs, role)
		if err != nil {
			continue
		}
		for _, e := range edges {
			fillers = append(fillers, e.To)
		}
	}
	return fillers
}
