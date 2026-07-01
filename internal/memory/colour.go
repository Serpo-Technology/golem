package memory

// Colour prototypes are the learned centres of GOLEM's colour categories: the colour a
// cell has come to stand for, seeded from the born-in palette and moved toward what the
// eye is shown and told. The eye classifies a colour to its nearest prototype, so where
// one colour ends and the next begins is learned from experience rather than a fixed
// table, the way a tongue carves the spectrum its own way.

// ColourPrototypes returns every cell's learned centre colour, keyed by cell.
func (s *Store) ColourPrototypes() (map[string][3]float64, error) {
	rows, err := s.db.Query(`SELECT cell, r, g, b FROM colour_prototypes`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string][3]float64{}
	for rows.Next() {
		var cell string
		var c [3]float64
		if err := rows.Scan(&cell, &c[0], &c[1], &c[2]); err != nil {
			return nil, err
		}
		out[cell] = c
	}
	return out, rows.Err()
}

// ColourPrototype reads one cell's learned centre colour.
func (s *Store) ColourPrototype(cell string) (c [3]float64, ok bool) {
	row := s.db.QueryRow(`SELECT r, g, b FROM colour_prototypes WHERE cell = ?`, cell)
	if err := row.Scan(&c[0], &c[1], &c[2]); err != nil {
		return [3]float64{}, false
	}
	return c, true
}

// SetColourPrototype writes a cell's centre colour, moving it as experience refines the
// category.
func (s *Store) SetColourPrototype(cell string, c [3]float64) error {
	_, err := s.db.Exec(
		`INSERT INTO colour_prototypes (cell, r, g, b) VALUES (?,?,?,?)
		 ON CONFLICT(cell) DO UPDATE SET r = excluded.r, g = excluded.g, b = excluded.b`,
		cell, c[0], c[1], c[2])
	return err
}

// SeedColourPrototype writes a cell's centre colour only if it has none yet, so a boot
// lays down the born-in prior without clobbering what experience has since learned.
func (s *Store) SeedColourPrototype(cell string, c [3]float64) error {
	_, err := s.db.Exec(
		`INSERT OR IGNORE INTO colour_prototypes (cell, r, g, b) VALUES (?,?,?,?)`,
		cell, c[0], c[1], c[2])
	return err
}
