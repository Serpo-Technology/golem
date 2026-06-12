package daemon

import (
	"sort"

	"golem/internal/memory"
)

// recallTiered answers a query from the hot working set first, topping up
// from the archive and promoting what it fetched. Hot hits still write
// their retrieval practice through to the archive, so a hard kill cannot
// lose the refresh. The decision loop deliberately stays on the archive
// path: what a cue means should not bend toward whatever happens to be
// cached.
func (d *Daemon) recallTiered(query string, limit int) ([]*memory.Fragment, error) {
	// Gather from both tiers before trimming, so a hot-tier thought cannot
	// crowd out a grounded recollection that lives only in the archive.
	hot := d.hot.Search(query, limit*2)
	cold, err := d.store.Recall(query, limit*2)
	if err != nil {
		return nil, err
	}
	seen := make(map[uint64]bool, len(hot)+len(cold))
	merged := make([]*memory.Fragment, 0, len(hot)+len(cold))
	for _, f := range append(hot, cold...) {
		if !seen[f.ID] {
			seen[f.ID] = true
			merged = append(merged, f)
		}
	}
	// A thought re-enters memory carrying a concept's gloss verbatim, so it
	// substring-matches the query. Self-talk stays recallable but ranks
	// below grounded recollections, so it cannot outrank experience.
	sort.SliceStable(merged, func(i, j int) bool {
		return merged[i].Channel != thoughtChannel && merged[j].Channel == thoughtChannel
	})
	if len(merged) > limit {
		merged = merged[:limit]
	}
	ids := make([]uint64, 0, len(merged))
	for _, f := range merged {
		f.Weight = 1.0
		ids = append(ids, f.ID)
		d.hot.Put(f)
	}
	if err := d.store.Refresh(ids); err != nil {
		return nil, err
	}
	return merged, nil
}

// fragmentsByIDsTiered fetches by ID through the working set, promoting
// archive hits: a moment that recall walked to is working memory now.
func (d *Daemon) fragmentsByIDsTiered(ids []uint64) ([]*memory.Fragment, error) {
	out := make([]*memory.Fragment, 0, len(ids))
	var missing []uint64
	for _, id := range ids {
		if f, ok := d.hot.Get(id); ok {
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
			d.hot.Put(f)
		}
		out = append(out, cold...)
	}
	return out, nil
}
