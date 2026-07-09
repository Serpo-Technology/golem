package daemon

import (
	"testing"

	"golem/internal/memory"
)

// A relation whose role is filled by two disjoint crowds is placed by a prototype near EACH, not
// the midpoint near neither, so an argument from either crowd matches it strongly where the mean
// alone would fail. Sign-safety holds: an argument opposite the whole cloud scores below zero.
func TestSetRoleProtosSplitsTwoCrowds(t *testing.T) {
	d, s := testDaemon(t)
	const k = 4
	// Lobe A near e0, lobe B near e1 -- two disjoint filler crowds.
	vecByID := map[uint64][]float32{
		1: {1, 0, 0, 0}, 2: {0.9, 0.1, 0, 0}, 3: {1, 0, 0.1, 0}, // A
		4: {0, 1, 0, 0}, 5: {0.1, 0.9, 0, 0}, 6: {0, 1, 0.1, 0}, // B
	}
	fillers := []uint64{1, 2, 3, 4, 5, 6}
	const relID = 100
	d.setRoleProtos(relID, memory.RelAgent, fillers, vecByID, k)

	blob, ok := s.UsageVector(relID, memory.RelAgent)
	if !ok {
		t.Fatal("the role should be placed")
	}
	if len(blob)%k != 0 || len(blob)/k < 3 {
		t.Fatalf("a two-crowd role should hold the mean plus a spread pair (>=3 prototypes): %d floats", len(blob))
	}
	mean := blob[0:k]

	argA := []float32{1, 0, 0, 0}
	argB := []float32{0, 1, 0, 0}
	if got, meanCos := maxProtoCosine(argA, blob), memory.Cosine(argA, mean); got <= meanCos || got < 0.9 {
		t.Fatalf("crowd A should match a prototype above the mean: max=%.3f mean=%.3f", got, meanCos)
	}
	if got, meanCos := maxProtoCosine(argB, blob), memory.Cosine(argB, mean); got <= meanCos || got < 0.9 {
		t.Fatalf("crowd B should match a prototype above the mean: max=%.3f mean=%.3f", got, meanCos)
	}
	// An argument opposite the whole filler cloud must not score high (sign-safe, not a projection norm).
	if got := maxProtoCosine([]float32{-1, -1, 0, 0}, blob); got >= 0 {
		t.Fatalf("an opposite argument must score below zero, got %.3f", got)
	}
}

// A tight unimodal role degrades to the mean: an argument in the crowd matches near the old
// cosine-to-mean and an unrelated argument stays low, so the richer placement adds no spurious
// score. And an old single-vector (mean-only) placement reads exactly as the plain cosine to it.
func TestSetRoleProtosUnimodalAndLegacy(t *testing.T) {
	d, s := testDaemon(t)
	const k = 4
	vecByID := map[uint64][]float32{
		1: {1, 0.05, 0, 0}, 2: {0.95, 0, 0.05, 0}, 3: {1, -0.05, 0, 0}, // one tight crowd near e0
	}
	const relID = 200
	d.setRoleProtos(relID, memory.RelAgent, []uint64{1, 2, 3}, vecByID, k)
	blob, _ := s.UsageVector(relID, memory.RelAgent)
	mean := blob[0:k]

	arg := []float32{1, 0, 0, 0}
	if got, meanCos := maxProtoCosine(arg, blob), memory.Cosine(arg, mean); got < meanCos-1e-4 || got < 0.9 {
		t.Fatalf("a unimodal role should match near the mean, no regression: max=%.3f mean=%.3f", got, meanCos)
	}
	if got := maxProtoCosine([]float32{0, 0, 1, 0}, blob); got > 0.3 {
		t.Fatalf("an unrelated argument should stay low, no spurious prototype inflation: %.3f", got)
	}

	// A legacy single-vector placement (a brain embedded before the spread points) reads as the
	// plain cosine to that one vector: maxProtoCosine reduces to Cosine when the blob is one chunk.
	legacy := []float32{1, 0, 0, 0}
	if err := s.SetUsageVector(300, memory.RelAgent, legacy); err != nil {
		t.Fatal(err)
	}
	one, _ := s.UsageVector(300, memory.RelAgent)
	if maxProtoCosine(arg, one) != memory.Cosine(arg, one) {
		t.Fatal("a single-vector placement must read identically to the plain cosine")
	}
}
