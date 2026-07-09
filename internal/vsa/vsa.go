// Package vsa is a spike on Vector Symbolic Architecture: Plate's Holographic Reduced
// Representations (HRR), a way to hold STRUCTURE -- a proposition's roles and fillers, a concept's
// relational profile -- in one fixed-width dense vector, and to query it by relational SHAPE
// rather than by walking edges. It is GOLEM's weakest mode (structural analogy) explored on a
// synthetic dense codebook, to measure whether the substrate is worth wiring in.
//
// An atom (a concept or a role) is a random dense vector. Four operations compose them:
//   Bind    (circular convolution)  ties a role to its filler; the result resembles neither, so a
//                                   bound pair hides its parts until unbound.
//   Unbind  (circular correlation)  recovers a filler from its role, approximately, plus noise.
//   Bundle  (addition)              superposes many bindings in one vector, each still recoverable.
//   Cleanup (nearest codebook atom) snaps a noisy query back to a known concept, or reports a weak
//                                   match a caller can gate on.
//
// It fits the substrate discipline: DETERMINISTIC (a seeded codebook is reproducible), INSPECTABLE
// (every vector traces to the atoms it was built from), and PROPOSE/DISPOSE (a holographic query
// PROPOSES a noisy vector; cleanup DISPOSES it against the known codebook, hedged by its cosine).
package vsa

import (
	"math"
	"math/rand"
	"sort"
)

// HV is a hypervector: a point in a high-dimensional space an atom or a structure lives at.
type HV []float64

// New draws a random atom of dimension d, its elements ~ N(0, 1/d) so its expected norm is ~1, the
// random vector a concept or role is represented by. The rng seeds the draw, so a codebook is
// reproducible night to night.
func New(d int, rng *rand.Rand) HV {
	v := make(HV, d)
	s := 1.0 / math.Sqrt(float64(d))
	for i := range v {
		v[i] = rng.NormFloat64() * s
	}
	return v
}

// Bind is circular convolution, the reversible tying of a role to its filler:
// c[k] = sum_j a[j] * b[(k-j) mod d]. The bound vector resembles neither operand.
func Bind(a, b HV) HV {
	d := len(a)
	c := make(HV, d)
	for k := 0; k < d; k++ {
		var s float64
		for j := 0; j < d; j++ {
			s += a[j] * b[((k-j)%d+d)%d]
		}
		c[k] = s
	}
	return c
}

// involution reverses a hypervector so convolving with it is correlation, binding's approximate
// inverse: inv[k] = b[(d-k) mod d], the zeroth component fixed.
func involution(b HV) HV {
	d := len(b)
	inv := make(HV, d)
	for k := 0; k < d; k++ {
		inv[k] = b[(d-k)%d]
	}
	return inv
}

// Unbind recovers the filler bound under a role: Unbind(Bind(role, filler), role) ~= filler, plus
// the noise cleanup removes. It is Bind with the role's involution (circular correlation).
func Unbind(c, role HV) HV { return Bind(c, involution(role)) }

// Bundle superposes hypervectors by addition, holding a set in one vector, each member still
// recoverable by its binding up to the capacity the dimension affords.
func Bundle(vs ...HV) HV {
	if len(vs) == 0 {
		return nil
	}
	out := make(HV, len(vs[0]))
	for _, v := range vs {
		for i := range out {
			out[i] += v[i]
		}
	}
	return out
}

// Cosine is the similarity of two hypervectors, 0 on a zero or mismatched vector.
func Cosine(a, b HV) float64 {
	if len(a) == 0 || len(a) != len(b) {
		return 0
	}
	var dot, na, nb float64
	for i := range a {
		dot += a[i] * b[i]
		na += a[i] * a[i]
		nb += b[i] * b[i]
	}
	if na == 0 || nb == 0 {
		return 0
	}
	return dot / (math.Sqrt(na) * math.Sqrt(nb))
}

// Match is a codebook atom a query resembled and how strongly.
type Match struct {
	Name  string
	Score float64
}

// Cleanup ranks the codebook atoms a (noisy) query most resembles, strongest first: the dispose
// step that snaps a holographic result back to known concepts, the top match the recovered filler
// and its cosine the confidence a caller gates on.
func Cleanup(v HV, codebook map[string]HV) []Match {
	out := make([]Match, 0, len(codebook))
	for name, atom := range codebook {
		out = append(out, Match{name, Cosine(v, atom)})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Score != out[j].Score {
			return out[i].Score > out[j].Score
		}
		return out[i].Name < out[j].Name
	})
	return out
}

// Nearest names the single codebook atom a query most resembles, with its cosine.
func Nearest(v HV, codebook map[string]HV) (string, float64) {
	if m := Cleanup(v, codebook); len(m) > 0 {
		return m[0].Name, m[0].Score
	}
	return "", 0
}
