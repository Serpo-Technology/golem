package daemon

import (
	"fmt"
	"math"

	"golem/internal/memory"
	"gonum.org/v1/gonum/mat"
)

// embedDim caps the width of the learned space; the data caps it lower, since a
// brain with few concepts or few features yields fewer real directions.
const embedDim = 32

// growEmbeddings learns each concept's place in a continuous space from its own
// grounded company, no outside model. It is LSA over the concept-by-feature
// matrix: factor that matrix, keep its leading directions, and a concept's
// coordinates are its row in the left factor. Concepts with similar feature
// signatures land near, and the latent factors put concepts that keep
// similar-but-not-identical company near too, the second-order resemblance that
// discrete token overlap misses. The space is opaque in its axes but grounded in
// its source: every coordinate traces back to the features it was built from,
// which trace back to lived moments. This is the proposer the brain grows for
// itself. It runs each night after features settle; under two concepts or with
// no features it writes nothing.
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
	edges, err := d.store.LinksFromRel(ids, memory.RelFeature)
	if err != nil || len(edges) == 0 {
		return
	}
	colOf := map[uint64]int{}
	for _, e := range edges {
		if _, ok := colOf[e.To]; !ok {
			colOf[e.To] = len(colOf)
		}
	}
	nC, nF := len(concepts), len(colOf)
	data := make([]float64, nC*nF)
	for _, e := range edges {
		r, ok := rowOf[e.From]
		if !ok {
			continue
		}
		// log1p tempers a dominant feature, the count-based embedding's damping
		// of raw frequency so one loud trait does not swamp the geometry.
		data[r*nF+colOf[e.To]] = math.Log1p(e.Weight)
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
	for i, c := range concepts {
		vec := make([]float32, k)
		for j := 0; j < k; j++ {
			// Scaling a direction by the root of its singular value weights the
			// dominant factors, the standard latent-semantic coordinate.
			vec[j] = float32(u.At(i, j) * math.Sqrt(sv[j]))
		}
		if err := d.store.SetVector(c.ID, vec); err != nil {
			fmt.Fprintf(d.out, "embedding error: %v\n", err)
			return
		}
	}
}
