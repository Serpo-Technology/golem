package memory

import (
	"sort"
	"strings"
)

// The hot tier is GOLEM's working set: the fragments most recently
// encoded or recalled, held in RAM in front of the sqlite archive. Recall
// answers from here first and promotes what it has to fetch from cold
// storage. The tier is owned by the brain loop, so it needs no lock, and
// it is cleared at sleep because consolidation (decay, pruning, merging)
// rewrites the archive underneath any cached copy.
const hotBudget = 256

type HotTier struct {
	frags map[uint64]*Fragment
	touch map[uint64]int64
	clock int64
}

func NewHotTier() *HotTier {
	return &HotTier{frags: make(map[uint64]*Fragment), touch: make(map[uint64]int64)}
}

func (h *HotTier) Len() int { return len(h.frags) }

func (h *HotTier) Clear() {
	h.frags = make(map[uint64]*Fragment)
	h.touch = make(map[uint64]int64)
}

// Put inserts or refreshes a fragment in the working set, evicting the
// least recently touched entry once the budget is exceeded.
func (h *HotTier) Put(f *Fragment) {
	if f == nil || f.ID == 0 {
		return
	}
	h.clock++
	h.frags[f.ID] = f
	h.touch[f.ID] = h.clock
	if len(h.frags) <= hotBudget {
		return
	}
	var coldest uint64
	var oldest int64 = -1
	for id, at := range h.touch {
		if oldest == -1 || at < oldest {
			coldest, oldest = id, at
		}
	}
	delete(h.frags, coldest)
	delete(h.touch, coldest)
}

// Get returns a working-set fragment and counts the access as a touch.
func (h *HotTier) Get(id uint64) (*Fragment, bool) {
	f, ok := h.frags[id]
	if ok {
		h.clock++
		h.touch[id] = h.clock
	}
	return f, ok
}

// Search matches the query against content and template, ranked the way
// the archive ranks recall: most stressful first. Only moments answer;
// word and concept nodes promoted by graph walks are not recall hits
// themselves.
func (h *HotTier) Search(query string, limit int) []*Fragment {
	q := strings.ToLower(query)
	if q == "" {
		return nil
	}
	var out []*Fragment
	for _, f := range h.frags {
		if f.Kind != "" && f.Kind != KindEpisodic {
			continue
		}
		if strings.Contains(strings.ToLower(f.Content), q) ||
			strings.Contains(strings.ToLower(f.Template), q) {
			out = append(out, f)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		// Grounded recollections rank above thought-channel self-talk, so
		// the limit cut keeps experience over the inner loop's chatter.
		if ti, tj := out[i].Channel == thoughtChannel, out[j].Channel == thoughtChannel; ti != tj {
			return tj
		}
		if out[i].Limbic.Noradrenaline != out[j].Limbic.Noradrenaline {
			return out[i].Limbic.Noradrenaline > out[j].Limbic.Noradrenaline
		}
		return out[i].ID > out[j].ID
	})
	if len(out) > limit {
		out = out[:limit]
	}
	for _, f := range out {
		h.clock++
		h.touch[f.ID] = h.clock
	}
	return out
}
