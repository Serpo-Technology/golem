package daemon

import (
	"golem/internal/memory"
)

// recallTiered answers a query from the hot working set first, topping up
// from the archive and promoting what it fetched. Hot hits still write
// their retrieval practice through to the archive, so a hard kill cannot
// lose the refresh. The decision loop deliberately stays on the archive
// path: what a cue means should not bend toward whatever happens to be
// cached.
func (d *Daemon) recallTiered(query string, limit int) ([]*memory.Fragment, error) {
	hits := d.hot.Search(query, limit)
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
		d.hot.Put(f)
	}
	return hits, nil
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
