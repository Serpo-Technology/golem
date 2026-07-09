package vsa

import (
	"fmt"
	"math/rand"
	"testing"
)

// codebook mints a named atom per word from a seeded generator, the fixed vocabulary the
// structures below are built from and cleaned back to.
func codebook(d int, seed int64, names ...string) map[string]HV {
	rng := rand.New(rand.NewSource(seed))
	cb := map[string]HV{}
	for _, n := range names {
		cb[n] = New(d, rng)
	}
	return cb
}

// A proposition's roles and fillers live in one bundled, bound vector, and each filler is
// recovered by unbinding its role and cleaning to the nearest atom: structure held holographically
// and read back by relational shape, no edge walked.
func TestRoleFillerRetrieval(t *testing.T) {
	const d = 512
	cb := codebook(d, 1, "agent", "rel", "patient", "dog", "chase", "cat")
	record := Bundle(
		Bind(cb["agent"], cb["dog"]),
		Bind(cb["rel"], cb["chase"]),
		Bind(cb["patient"], cb["cat"]),
	)
	for role, want := range map[string]string{"agent": "dog", "rel": "chase", "patient": "cat"} {
		got, score := Nearest(Unbind(record, cb[role]), cb)
		if got != want {
			t.Fatalf("role %q should recover %q, got %q (%.2f)", role, want, got, score)
		}
	}
}

// Structural analogy without naming the role: the Mexican analog of the dollar is the peso,
// recovered by the holographic mapping alone -- the dollar bound into the Mexico record, unbound by
// the USA record, cancels the shared roles and leaves the peso. This is Kanerva's "What is the
// Dollar of Mexico?", GOLEM's weakest mode (analogy by relational shape) done in vector algebra.
func TestStructuralAnalogyByMapping(t *testing.T) {
	const d = 1024
	cb := codebook(d, 2, "country", "capital", "money", "usa", "wdc", "dollar", "mexico", "mxc", "peso")
	usa := Bundle(Bind(cb["country"], cb["usa"]), Bind(cb["capital"], cb["wdc"]), Bind(cb["money"], cb["dollar"]))
	mex := Bundle(Bind(cb["country"], cb["mexico"]), Bind(cb["capital"], cb["mxc"]), Bind(cb["money"], cb["peso"]))

	// dollar's analog in Mexico, by mapping alone (no role named): dollar (x) mex, unbound by usa.
	if got, score := Nearest(Unbind(Bind(cb["dollar"], mex), usa), cb); got != "peso" {
		t.Fatalf("the dollar of Mexico should be the peso, got %q (%.2f)", got, score)
	}
	// The capital analog the same way.
	if got, score := Nearest(Unbind(Bind(cb["wdc"], mex), usa), cb); got != "mxc" {
		t.Fatalf("the capital of Mexico should be mxc, got %q (%.2f)", got, score)
	}
}

// Feasibility measurement: how many role-filler pairs a bundle can hold before cleanup stops
// recovering them, across dimensions. The spike's honest number -- capacity grows with dimension,
// and past it the superposition is too crowded to read.
func TestCapacity(t *testing.T) {
	for _, d := range []int{256, 512, 1024, 2048} {
		line := fmt.Sprintf("d=%-5d", d)
		var lowLoad int
		for _, n := range []int{3, 6, 12, 24} {
			rng := rand.New(rand.NewSource(int64(d*100 + n)))
			cb := map[string]HV{}
			roles := make([]HV, n)
			pairs := make([]HV, n)
			for i := 0; i < n; i++ {
				roles[i] = New(d, rng)
				f := New(d, rng)
				cb[fmt.Sprintf("f%d", i)] = f
				pairs[i] = Bind(roles[i], f)
			}
			record := Bundle(pairs...)
			hits := 0
			for i := 0; i < n; i++ {
				if got, _ := Nearest(Unbind(record, roles[i]), cb); got == fmt.Sprintf("f%d", i) {
					hits++
				}
			}
			if n == 3 {
				lowLoad = hits
			}
			line += fmt.Sprintf("  n=%d:%d/%d", n, hits, n)
		}
		t.Log(line)
		// A baseline the spike must clear: at a light load a bundle recovers all its fillers.
		if lowLoad != 3 {
			t.Fatalf("d=%d should recover all 3 fillers at a light load, got %d", d, lowLoad)
		}
	}
}
