package memory

// A universal feature is one carried by nearly every concept: that a percept
// arrived on vision, or carried some stress, is true of all of them and so
// distinguishes none. Such a feature is not dropped, because the fact it records
// is real and may discriminate again once the brain gains a sense that makes it
// rare. It is named, here, as universal: the fraction of concepts that carry it
// is stored beside it, so the resemblance walk can pass it by while the feature
// itself stays in the graph for any reader to see.

// SetFeatureUniversal replaces the stored universality of every feature with a
// fresh reading, the fraction of concepts that carry each, recomputed each night
// from the current grounding. A nil or empty map clears the table, the right
// state for a brain with nothing universal yet.
func (s *Store) SetFeatureUniversal(frac map[uint64]float64) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM feature_universal`); err != nil {
		tx.Rollback()
		return err
	}
	stmt, err := tx.Prepare(`INSERT INTO feature_universal (feature_id, frac) VALUES (?, ?)`)
	if err != nil {
		tx.Rollback()
		return err
	}
	for id, f := range frac {
		if _, err := stmt.Exec(id, f); err != nil {
			stmt.Close()
			tx.Rollback()
			return err
		}
	}
	stmt.Close()
	return tx.Commit()
}

// FeatureUniversal reads how universal each feature is, the fraction of concepts
// carrying it, loaded into the resemblance walk's skip set at boot and refreshed
// at consolidation.
func (s *Store) FeatureUniversal() (map[uint64]float64, error) {
	rows, err := s.db.Query(`SELECT feature_id, frac FROM feature_universal`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[uint64]float64{}
	for rows.Next() {
		var id uint64
		var frac float64
		if err := rows.Scan(&id, &frac); err != nil {
			return nil, err
		}
		out[id] = frac
	}
	return out, rows.Err()
}
