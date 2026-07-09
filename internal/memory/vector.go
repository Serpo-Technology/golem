package memory

import (
	"encoding/binary"
	"math"
	"sort"
)

// The vector layer is the associative substrate beside the symbolic graph: an
// embedding per fragment that places it among the others, so "similar" can be
// nearness in a space rather than overlap of discrete feature tokens. The
// embeddings are learned by the brain from its own grounded company (the daemon's
// nightly factorization, no outside model); the store only keeps the vectors and
// measures distance. This is the layer that lets analogy and generalization
// propose, while the graph still disposes (composes and grounds).

// VectorMatch is one fragment and how near its embedding sits to a target.
type VectorMatch struct {
	ID    uint64
	Score float64 // cosine similarity in [-1, 1]
}

// SetVector stores a fragment's embedding, replacing any prior one. The vector
// is kept as little-endian float32, so its width travels with the bytes.
func (s *Store) SetVector(fragmentID uint64, vec []float32) error {
	_, err := s.db.Exec(
		`INSERT OR REPLACE INTO vectors (fragment_id, vec) VALUES (?, ?)`,
		fragmentID, encodeVec(vec))
	return err
}

// Vector reads a fragment's embedding; ok is false when it has none.
func (s *Store) Vector(fragmentID uint64) (vec []float32, ok bool) {
	var blob []byte
	if err := s.db.QueryRow(`SELECT vec FROM vectors WHERE fragment_id = ?`, fragmentID).Scan(&blob); err != nil {
		return nil, false
	}
	return decodeVec(blob), true
}

// NearestVectors finds the fragments of a kind whose embeddings sit nearest a
// target by cosine, the seed excluded, the strongest first. It is a brute-force
// scan, which a small brain can afford; an index is the optimization a large one
// would want. A kind of "" scans every embedded fragment.
func (s *Store) NearestVectors(target []float32, kind string, limit int, exclude uint64) ([]VectorMatch, error) {
	query := `SELECT v.fragment_id, v.vec FROM vectors v JOIN fragments f ON f.id = v.fragment_id`
	args := []any{}
	if kind != "" {
		query += ` WHERE f.kind = ?`
		args = append(args, kind)
	}
	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []VectorMatch
	for rows.Next() {
		var id uint64
		var blob []byte
		if err := rows.Scan(&id, &blob); err != nil {
			return nil, err
		}
		if id == exclude {
			continue
		}
		out = append(out, VectorMatch{ID: id, Score: Cosine(target, decodeVec(blob))})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Score != out[j].Score {
			return out[i].Score > out[j].Score
		}
		return out[i].ID < out[j].ID
	})
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

// Cosine is the similarity of two embeddings, 0 when either is empty or zero or
// the widths disagree, so a mismatch reads as unrelated rather than panicking.
func Cosine(a, b []float32) float64 {
	if len(a) == 0 || len(a) != len(b) {
		return 0
	}
	var dot, na, nb float64
	for i := range a {
		dot += float64(a[i]) * float64(b[i])
		na += float64(a[i]) * float64(a[i])
		nb += float64(b[i]) * float64(b[i])
	}
	if na == 0 || nb == 0 {
		return 0
	}
	return dot / (math.Sqrt(na) * math.Sqrt(nb))
}

// SetUsageVector stores a fragment's place in the usage space for one role, the
// company it keeps as the filler of that role. A relation carries an "agent" and a
// "patient" vector, kept apart so a directional relation reads differently from a
// symmetric one; this space sits beside the feature-and-category vectors and never
// touches them, so the kind geometry is left as it was.
func (s *Store) SetUsageVector(fragmentID uint64, role string, vec []float32) error {
	_, err := s.db.Exec(
		`INSERT OR REPLACE INTO usage_vectors (fragment_id, role, vec) VALUES (?, ?, ?)`,
		fragmentID, role, encodeVec(vec))
	return err
}

// UsageVector reads a fragment's usage vector for a role; ok is false when it has
// none.
func (s *Store) UsageVector(fragmentID uint64, role string) (vec []float32, ok bool) {
	var blob []byte
	if err := s.db.QueryRow(`SELECT vec FROM usage_vectors WHERE fragment_id = ? AND role = ?`,
		fragmentID, role).Scan(&blob); err != nil {
		return nil, false
	}
	return decodeVec(blob), true
}

// UsageVectorsByRole returns every fragment's usage vector for a role, so a caller
// can rank all relations against a clause's filler in one pass.
func (s *Store) UsageVectorsByRole(role string) (map[uint64][]float32, error) {
	rows, err := s.db.Query(`SELECT fragment_id, vec FROM usage_vectors WHERE role = ?`, role)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[uint64][]float32{}
	for rows.Next() {
		var id uint64
		var blob []byte
		if err := rows.Scan(&id, &blob); err != nil {
			return nil, err
		}
		out[id] = decodeVec(blob)
	}
	return out, rows.Err()
}

// SetHolo stores a concept's holographic record, the one fixed-width vector that
// holds its whole relational frame (its roles bound to their fillers, superposed).
// It sits beside the kind and usage embeddings as a third proposer space and never
// touches them, so placing a concept by its bound frame disturbs neither geometry.
func (s *Store) SetHolo(fragmentID uint64, vec []float32) error {
	_, err := s.db.Exec(
		`INSERT OR REPLACE INTO holo_vectors (fragment_id, vec) VALUES (?, ?)`,
		fragmentID, encodeVec(vec))
	return err
}

// Holo reads a concept's holographic record; ok is false when it has none, which is
// the common case: only a concept whose frame carries enough distinct fillers to map
// is encoded, so most concepts have no record and a query over them admits the gap.
func (s *Store) Holo(fragmentID uint64) (vec []float32, ok bool) {
	var blob []byte
	if err := s.db.QueryRow(`SELECT vec FROM holo_vectors WHERE fragment_id = ?`, fragmentID).Scan(&blob); err != nil {
		return nil, false
	}
	return decodeVec(blob), true
}

// DeleteHolo drops a concept's holographic record, so a concept whose frame has
// thinned below the encode threshold since the last night stops offering a stale
// record for a mapping to read.
func (s *Store) DeleteHolo(fragmentID uint64) error {
	_, err := s.db.Exec(`DELETE FROM holo_vectors WHERE fragment_id = ?`, fragmentID)
	return err
}

// SweepHoloOrphans drops holographic records for fragments the graph no longer holds,
// so a concept pruned by a nightly consolidation (and thus never revisited by the
// re-encode) leaves no lingering row. Ids never repeat, so a swept record can never be
// misread as a live concept's.
func (s *Store) SweepHoloOrphans() error {
	_, err := s.db.Exec(`DELETE FROM holo_vectors WHERE fragment_id NOT IN (SELECT id FROM fragments)`)
	return err
}

func encodeVec(vec []float32) []byte {
	b := make([]byte, 4*len(vec))
	for i, f := range vec {
		binary.LittleEndian.PutUint32(b[4*i:], math.Float32bits(f))
	}
	return b
}

func decodeVec(b []byte) []float32 {
	vec := make([]float32, len(b)/4)
	for i := range vec {
		vec[i] = math.Float32frombits(binary.LittleEndian.Uint32(b[4*i:]))
	}
	return vec
}
