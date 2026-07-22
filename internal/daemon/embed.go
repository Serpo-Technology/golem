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

// A relation's role is placed by prototype POINTS, the mean of its fillers plus a pair one
// standard deviation out along each leading spread direction, so a role whose fillers cluster in
// two crowds is matched by the nearer crowd rather than the midpoint between them. rProtoMax caps
// the spread directions kept, protoEnergyFloor is the share of total variance a direction must
// carry to earn a prototype pair, and protoSigma is how far along it the pair sits.
const (
	rProtoMax        = 3
	protoEnergyFloor = 0.10
	protoSigma       = 1.0
)

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
	concepts, err := d.grownConcepts()
	if err != nil {
		fmt.Fprintf(d.out, "embedding error: %v\n", err)
		return
	}
	if len(concepts) < 2 {
		return
	}
	in := d.prefetchEmbeddingInputs(concepts)
	d.applyEmbeddings(computeEmbeddings(concepts, in))
}

// growEmbeddingsOverlapped runs the factorization off the committer while during()
// runs on it, then applies the vectors. The factorization is the nap's heaviest
// compute and touches no store once its inputs are prefetched, and during() (the
// holographic encode) reads none of the vectors, so the two run at once and the
// committer is blocked for the longer of the two rather than their sum. Every vector
// write still lands on the committer, after during(), so a later pass that reads
// resemblance sees the same fresh vectors it did before.
func (d *Daemon) growEmbeddingsOverlapped(during func()) {
	concepts, err := d.grownConcepts()
	if err != nil {
		fmt.Fprintf(d.out, "embedding error: %v\n", err)
		during()
		return
	}
	if len(concepts) < 2 {
		during()
		return
	}
	in := d.prefetchEmbeddingInputs(concepts)
	done := make(chan []embWrite, 1)
	go func() { done <- computeEmbeddings(concepts, in) }()
	during()
	d.applyEmbeddings(<-done)
}

// embInputs holds every store read the embedding factorization needs, gathered on
// the committer so the factorization runs off it without touching the store: the
// kind traits (a concept's features and category memberships) and, per relation, the
// concepts that filled its agent and patient roles.
type embInputs struct {
	featEdges   []memory.LinkEdge
	catEdges    []memory.LinkEdge
	roleFillers map[uint64]roleFill
}

type roleFill struct{ agents, patients []uint64 }

// embWrite is one vector the factorization produced, applied on the committer. An
// empty role is a kind vector (SetVector); a named role is a usage vector, either a
// concept's own usage place (usageSelf) or a relation's role prototype.
type embWrite struct {
	id   uint64
	role string
	vec  []float32
}

// prefetchEmbeddingInputs gathers on the committer every edge the factorization
// reads, so computeEmbeddings can run without the store. relationRoleFillers is read
// once per relation here and shared by both the role prototypes and the usage
// factorization, which each read it separately before.
func (d *Daemon) prefetchEmbeddingInputs(concepts []*memory.Fragment) *embInputs {
	ids := make([]uint64, len(concepts))
	for i, c := range concepts {
		ids[i] = c.ID
	}
	in := &embInputs{roleFillers: map[uint64]roleFill{}}
	in.featEdges, _ = d.store.LinksFromRel(ids, memory.RelFeature)
	in.catEdges, _ = d.store.LinksToRel(ids, memory.RelGrounds)
	for _, c := range concepts {
		if c.Category != "relation" {
			continue
		}
		agents, patients := d.relationRoleFillers(c.ID)
		in.roleFillers[c.ID] = roleFill{agents: agents, patients: patients}
	}
	return in
}

// applyEmbeddings writes the factored vectors on the committer.
func (d *Daemon) applyEmbeddings(writes []embWrite) {
	for _, w := range writes {
		var err error
		if w.role == "" {
			err = d.store.SetVector(w.id, w.vec)
		} else {
			err = d.store.SetUsageVector(w.id, w.role, w.vec)
		}
		if err != nil {
			fmt.Fprintf(d.out, "embedding error: %v\n", err)
			return
		}
	}
}

// computeEmbeddings factors the whole embedding without touching the store: the kind
// space from features and categories, the relation role prototypes read off the kind
// vectors, and the usage space from relational company. The kind and usage spaces are
// separate, so the usage placement runs even for a brain whose concepts carry no
// features at all, where the kind factorization has nothing to work on. It is pure,
// so it can run on a goroutine while the committer does other work.
func computeEmbeddings(concepts []*memory.Fragment, in *embInputs) []embWrite {
	vecByID, k, writes := computeKindEmbeddings(concepts, in.featEdges, in.catEdges)
	// Each relation's roles are placed by prototype points near their filler crowds,
	// read off the kind vectors, so a directional relation reads differently from a
	// symmetric one. A role no embedded filler took is left unplaced.
	for _, c := range concepts {
		if c.Category != "relation" {
			continue
		}
		rf := in.roleFillers[c.ID]
		if v := roleProtoVector(rf.agents, vecByID, k); v != nil {
			writes = append(writes, embWrite{c.ID, memory.RelAgent, v})
		}
		if v := roleProtoVector(rf.patients, vecByID, k); v != nil {
			writes = append(writes, embWrite{c.ID, memory.RelPatient, v})
		}
	}
	writes = append(writes, computeUsageEmbeddings(concepts, in.roleFillers)...)
	return writes
}

// computeKindEmbeddings is the feature-and-category half: LSA over the concept-by-
// trait matrix, factor it, keep its leading directions, and a concept's coordinates
// are its row in the left factor. A category column is the likeness feature overlap
// misses, so every animal shares one strong column. It returns the kind vectors keyed
// by concept (used to place the relation roles) and the writes for concepts that
// carried a trait; a concept with a zero row is left out of the kind space so its
// relational company can place it in the usage space instead.
func computeKindEmbeddings(concepts []*memory.Fragment, featEdges, catEdges []memory.LinkEdge) (map[uint64][]float32, int, []embWrite) {
	rowOf := make(map[uint64]int, len(concepts))
	for i, c := range concepts {
		rowOf[c.ID] = i
	}
	colOf := map[string]int{}
	val := map[[2]int]float64{}   // (row, col) -> final matrix value, strongest wins
	hasTrait := map[uint64]bool{} // a concept that contributed at least one trait
	put := func(conceptID uint64, trait string, v float64) {
		r, ok := rowOf[conceptID]
		if !ok || v <= 0 {
			return
		}
		hasTrait[conceptID] = true
		c, ok := colOf[trait]
		if !ok {
			c = len(colOf)
			colOf[trait] = c
		}
		if key := [2]int{r, c}; v > val[key] {
			val[key] = v
		}
	}
	// A feature the concept carries, tempered by log1p so one loud trait does not
	// swamp the geometry; and a category it belongs to, the likeness feature overlap
	// misses, so every animal shares one strong column.
	for _, e := range featEdges {
		put(e.From, "feat:"+strconv.FormatUint(e.To, 10), math.Log1p(e.Weight))
	}
	for _, e := range catEdges {
		put(e.To, "cat:"+strconv.FormatUint(e.From, 10), categoryTraitWeight)
	}
	empty := map[uint64][]float32{}
	if len(colOf) == 0 {
		return empty, 0, nil
	}
	nC, nF := len(concepts), len(colOf)
	data := make([]float64, nC*nF)
	for rc, v := range val {
		data[rc[0]*nF+rc[1]] = v
	}
	var svd mat.SVD
	if !svd.Factorize(mat.NewDense(nC, nF, data), mat.SVDThinU) {
		return empty, 0, nil
	}
	var u mat.Dense
	svd.UTo(&u)
	sv := svd.Values(nil)
	k := embedDim
	if len(sv) < k {
		k = len(sv)
	}
	if k == 0 {
		return empty, 0, nil
	}
	vecByID := make(map[uint64][]float32, len(concepts))
	var writes []embWrite
	for i, c := range concepts {
		vec := make([]float32, k)
		for j := 0; j < k; j++ {
			// Scaling a direction by the root of its singular value weights the
			// dominant factors, the standard latent-semantic coordinate.
			vec[j] = float32(u.At(i, j) * math.Sqrt(sv[j]))
		}
		vecByID[c.ID] = vec
		// A concept that carried no trait has a zero row: leave it unwritten so the
		// kind space does not claim it and the usage space can. It still enters the
		// relation prototypes below through vecByID.
		if !hasTrait[c.ID] {
			continue
		}
		writes = append(writes, embWrite{c.ID, "", vec})
	}
	return vecByID, k, writes
}

// computeUsageEmbeddings places each concept in the usage space by its relational
// company: which role of which relation it fills. It factors the concept-by-(relation,
// role) matrix on its own, so a concept with no grounded features -- an abstract word
// that names no percept -- still gets coordinates from how it is used, where the kind
// embedding leaves it at the origin. A concept that fills no role is left unplaced.
func computeUsageEmbeddings(concepts []*memory.Fragment, roleFillers map[uint64]roleFill) []embWrite {
	rowOf := make(map[uint64]int, len(concepts))
	for i, c := range concepts {
		rowOf[c.ID] = i
	}
	colOf := map[string]int{}
	val := map[[2]int]float64{}
	hasCompany := map[uint64]bool{}
	put := func(conceptID uint64, trait string) {
		r, ok := rowOf[conceptID]
		if !ok {
			return
		}
		c, ok := colOf[trait]
		if !ok {
			c = len(colOf)
			colOf[trait] = c
		}
		val[[2]int{r, c}]++
		hasCompany[conceptID] = true
	}
	for _, c := range concepts {
		if c.Category != "relation" {
			continue
		}
		rel := strconv.FormatUint(c.ID, 10)
		rf := roleFillers[c.ID]
		for _, a := range rf.agents {
			put(a, "a:"+rel)
		}
		for _, p := range rf.patients {
			put(p, "p:"+rel)
		}
	}
	if len(colOf) == 0 {
		return nil
	}
	nC, nF := len(concepts), len(colOf)
	data := make([]float64, nC*nF)
	for rc, v := range val {
		data[rc[0]*nF+rc[1]] = math.Log1p(v) // damp a loud co-occurrence
	}
	var svd mat.SVD
	if !svd.Factorize(mat.NewDense(nC, nF, data), mat.SVDThinU) {
		return nil
	}
	var u mat.Dense
	svd.UTo(&u)
	sv := svd.Values(nil)
	k := embedDim
	if len(sv) < k {
		k = len(sv)
	}
	if k == 0 {
		return nil
	}
	var writes []embWrite
	for i, c := range concepts {
		if !hasCompany[c.ID] {
			continue // no relational role filled; nothing to place by
		}
		vec := make([]float32, k)
		for j := 0; j < k; j++ {
			vec[j] = float32(u.At(i, j) * math.Sqrt(sv[j]))
		}
		writes = append(writes, embWrite{c.ID, usageSelf, vec})
	}
	return writes
}

// usageSelf is the role under which a concept's own usage vector sits in the usage
// space, beside the agent and patient centroids the relations keep there.
const usageSelf = "self"

// conceptUsageResemblance is the cosine of two concepts' usage vectors, how alike
// the relational company they keep is. It is the abstract-word counterpart to
// feature resemblance: two concepts used the same way, filling the same role of the
// same relations, read as near even with no features in common. Zero when either
// has not been placed in the usage space.
func (d *Daemon) conceptUsageResemblance(a, b uint64) float64 {
	if a == b {
		return 1
	}
	va, oka := d.store.UsageVector(a, usageSelf)
	vb, okb := d.store.UsageVector(b, usageSelf)
	if !oka || !okb {
		return 0
	}
	return memory.Cosine(va, vb)
}

// setRoleProtos places a relation's role in the usage space from the kind vectors of
// its fillers (roleProtoVector) and writes it. It is the store-writing wrapper kept
// for direct callers; the nightly pass computes the vector off the committer and
// applies it through applyEmbeddings instead. A role no embedded filler took is left
// unplaced.
func (d *Daemon) setRoleProtos(relID uint64, role string, fillers []uint64, vecByID map[uint64][]float32, k int) {
	vec := roleProtoVector(fillers, vecByID, k)
	if vec == nil {
		return
	}
	if err := d.store.SetUsageVector(relID, role, vec); err != nil {
		fmt.Fprintf(d.out, "usage embedding error: %v\n", err)
	}
}

// roleProtoVector places a relation's role by SIGNED PROTOTYPE POINTS in the kind space: the mean
// of its fillers' kind vectors, plus a pair one standard deviation out along each leading spread
// direction (a right singular vector of the centered filler matrix). So a role whose fillers form
// two clusters is placed by a point near EACH, not the midpoint near neither, and a match reads
// the nearest prototype. The mean is stored FIRST, so a match never scores below the old
// cosine-to-mean; a role with one tight crowd stores essentially the mean alone. The prototypes
// are real points in the kind space, so an argument's kind vector is matched by cosine as before,
// and storing both the plus and the minus of each direction makes the placement invariant to the
// singular vectors' arbitrary sign. It is pure: no store, so the nightly pass runs it off the
// committer. A role no embedded filler took returns a nil vector, left unplaced.
func roleProtoVector(fillers []uint64, vecByID map[uint64][]float32, k int) []float32 {
	rows := make([][]float64, 0, len(fillers))
	for _, id := range fillers {
		v, ok := vecByID[id]
		if !ok || len(v) != k {
			continue
		}
		row := make([]float64, k)
		for j := 0; j < k; j++ {
			row[j] = float64(v[j])
		}
		rows = append(rows, row)
	}
	n := len(rows)
	if n == 0 {
		return nil
	}
	mean := make([]float64, k)
	for _, row := range rows {
		for j := 0; j < k; j++ {
			mean[j] += row[j]
		}
	}
	for j := 0; j < k; j++ {
		mean[j] /= float64(n)
	}
	protos := append([]float64(nil), mean...) // the mean first: a match never falls below cosine-to-mean
	if n >= 2 {
		centered := make([]float64, n*k)
		for i, row := range rows {
			for j := 0; j < k; j++ {
				centered[i*k+j] = row[j] - mean[j]
			}
		}
		var svd mat.SVD
		if svd.Factorize(mat.NewDense(n, k, centered), mat.SVDThinV) {
			var v mat.Dense
			svd.VTo(&v)
			sv := svd.Values(nil)
			total := 0.0
			for _, s := range sv {
				total += s * s
			}
			r := rProtoMax
			if len(sv) < r {
				r = len(sv)
			}
			for i := 0; i < r && total > 0; i++ {
				// Singular values descend, so once one direction is too faint nothing further clears the floor.
				if sv[i] <= 0 || (sv[i]*sv[i])/total < protoEnergyFloor {
					break
				}
				std := sv[i] / math.Sqrt(float64(n))
				plus := make([]float64, k)
				minus := make([]float64, k)
				for j := 0; j < k; j++ {
					off := protoSigma * std * v.At(j, i)
					plus[j] = mean[j] + off
					minus[j] = mean[j] - off
				}
				protos = append(protos, plus...)
				protos = append(protos, minus...)
			}
		}
	}
	vec := make([]float32, len(protos))
	for i, x := range protos {
		vec[i] = float32(x)
	}
	return vec
}

// relationRoleFillers returns the concepts that have filled a relation's agent and
// patient roles, kept apart, by walking the moments it grounds in. The same
// concept recurs once per moment it filled, so a frequent argument weighs more in
// the centroid, the usage the placement should reflect.
func (d *Daemon) relationRoleFillers(relID uint64) (agents, patients []uint64) {
	moments, err := d.store.LinksFromRel([]uint64{relID}, memory.RelGrounds)
	if err != nil || len(moments) == 0 {
		return nil, nil
	}
	momIDs := make([]uint64, 0, len(moments))
	for _, e := range moments {
		momIDs = append(momIDs, e.To)
	}
	if edges, err := d.store.LinksFromRel(momIDs, memory.RelAgent); err == nil {
		for _, e := range edges {
			agents = append(agents, e.To)
		}
	}
	if edges, err := d.store.LinksFromRel(momIDs, memory.RelPatient); err == nil {
		for _, e := range edges {
			patients = append(patients, e.To)
		}
	}
	return agents, patients
}
