package main

import (
	"sort"
	"sync"
)

// Hippocampus stores fragments and supports limbic-keyed retrieval and the
// sleep/prune cycle. The mutex is here so that when you later have multiple
// sensory goroutines writing memories concurrently, this already holds up.
type Hippocampus struct {
	mu        sync.Mutex
	fragments map[uint64]*Fragment
	nextID    uint64
}

func NewHippocampus() *Hippocampus {
	return &Hippocampus{fragments: make(map[uint64]*Fragment)}
}

// Encode stores a new moment, stamps it with an ID and a full starting weight,
// and returns the ID.
func (h *Hippocampus) Encode(f *Fragment) uint64 {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.nextID++
	f.ID = h.nextID
	f.Weight = 1.0
	h.fragments[f.ID] = f
	return f.ID
}

// RecallByStress returns the fragment with the highest noradrenaline at
// encoding — i.e. "show me the moment when things felt wrong." This is the
// point: GOLEM retrieves by how it FELT, not by string match.
func (h *Hippocampus) RecallByStress() *Fragment {
	h.mu.Lock()
	defer h.mu.Unlock()
	var best *Fragment
	for _, f := range h.fragments {
		if best == nil || f.Limbic.Noradrenaline > best.Limbic.Noradrenaline {
			best = f
		}
	}
	return best
}

// Sleep runs one consolidation pass: decay every weight, then prune whatever
// has faded below the floor. High-surprise memories resist decay (a small
// limbic-tagging bonus), so the dull stuff fades and the sharp stuff persists.
func (h *Hippocampus) Sleep(decay, floor float64) (pruned int) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for id, f := range h.fragments {
		f.Weight *= (1 - decay)
		f.Weight += f.Surprise * 0.01 // limbic tagging: surprising = sticky
		if f.Weight <= floor {
			delete(h.fragments, id)
			pruned++
		}
	}
	return pruned
}

func (h *Hippocampus) Count() int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return len(h.fragments)
}

// Dump returns fragments oldest-first, for the demo printout.
func (h *Hippocampus) Dump() []*Fragment {
	h.mu.Lock()
	defer h.mu.Unlock()
	out := make([]*Fragment, 0, len(h.fragments))
	for _, f := range h.fragments {
		out = append(out, f)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}
