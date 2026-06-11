package main

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

type hotTier struct {
	frags map[uint64]*Fragment
	touch map[uint64]int64
	clock int64
}

func newHotTier() *hotTier {
	return &hotTier{frags: make(map[uint64]*Fragment), touch: make(map[uint64]int64)}
}

func (h *hotTier) len() int { return len(h.frags) }

func (h *hotTier) clear() {
	h.frags = make(map[uint64]*Fragment)
	h.touch = make(map[uint64]int64)
}

// put inserts or refreshes a fragment in the working set, evicting the
// least recently touched entry once the budget is exceeded.
func (h *hotTier) put(f *Fragment) {
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

// get returns a working-set fragment and counts the access as a touch.
func (h *hotTier) get(id uint64) (*Fragment, bool) {
	f, ok := h.frags[id]
	if ok {
		h.clock++
		h.touch[id] = h.clock
	}
	return f, ok
}

// search matches the query against content and template, ranked the way
// the archive ranks recall: most stressful first. Only moments answer;
// word and concept nodes promoted by graph walks are not recall hits
// themselves.
func (h *hotTier) search(query string, limit int) []*Fragment {
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

// recallTiered answers a query from the hot working set first, topping up
// from the archive and promoting what it fetched. Hot hits still write
// their retrieval practice through to the archive, so a hard kill cannot
// lose the refresh. The decision loop deliberately stays on the archive
// path: what a cue means should not bend toward whatever happens to be
// cached.
func (d *Daemon) recallTiered(query string, limit int) ([]*Fragment, error) {
	hits := d.hot.search(query, limit)
	if len(hits) > 0 {
		ids := make([]uint64, len(hits))
		for i, f := range hits {
			ids[i] = f.ID
			f.Weight = 1.0
		}
		if err := d.store.Refresh(ids); err != nil {
			return nil, err
		}
	}
	if len(hits) >= limit {
		return hits, nil
	}
	cold, err := d.store.Recall(query, limit)
	if err != nil {
		return nil, err
	}
	seen := make(map[uint64]bool, len(hits))
	for _, f := range hits {
		seen[f.ID] = true
	}
	for _, f := range cold {
		if seen[f.ID] || len(hits) >= limit {
			continue
		}
		hits = append(hits, f)
		d.hot.put(f)
	}
	return hits, nil
}

// fragmentsByIDsTiered fetches by ID through the working set, promoting
// archive hits: a moment that recall walked to is working memory now.
func (d *Daemon) fragmentsByIDsTiered(ids []uint64) ([]*Fragment, error) {
	out := make([]*Fragment, 0, len(ids))
	var missing []uint64
	for _, id := range ids {
		if f, ok := d.hot.get(id); ok {
			out = append(out, f)
			continue
		}
		missing = append(missing, id)
	}
	if len(missing) > 0 {
		cold, err := d.store.FragmentsByIDs(missing)
		if err != nil {
			return nil, err
		}
		for _, f := range cold {
			d.hot.put(f)
		}
		out = append(out, cold...)
	}
	return out, nil
}
