package main

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

// Store is the persistent hippocampus: fragments live in sqlite so memory
// survives restarts and crashes. Every encode commits immediately, so a hard
// kill loses at most the observation in flight. The links table is the
// durable home of fragment associations, and model_counts holds each
// channel predictor's state between runs (flushed at sleep and shutdown).
type Store struct {
	db *sql.DB
}

func OpenStore(path string) (*Store, error) {
	dsn := fmt.Sprintf("file:%s?_pragma=journal_mode(WAL)&_pragma=synchronous(NORMAL)&_pragma=busy_timeout(5000)", path)
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	// A single connection keeps the brain single-writer and sidesteps
	// SQLITE_BUSY between concurrent statements.
	db.SetMaxOpenConns(1)

	schema := `
CREATE TABLE IF NOT EXISTS fragments (
	id            INTEGER PRIMARY KEY AUTOINCREMENT,
	channel       TEXT    NOT NULL,
	content       TEXT    NOT NULL,
	template      TEXT    NOT NULL,
	surprise      REAL    NOT NULL,
	dopamine      REAL    NOT NULL,
	noradrenaline REAL    NOT NULL,
	serotonin     REAL    NOT NULL,
	at            INTEGER NOT NULL,
	weight        REAL    NOT NULL
);
CREATE TABLE IF NOT EXISTS links (
	from_id INTEGER NOT NULL,
	to_id   INTEGER NOT NULL,
	weight  REAL    NOT NULL,
	PRIMARY KEY (from_id, to_id)
);
CREATE TABLE IF NOT EXISTS model_counts (
	channel  TEXT NOT NULL,
	template TEXT NOT NULL,
	count    REAL NOT NULL,
	PRIMARY KEY (channel, template)
);`
	if _, err := db.Exec(schema); err != nil {
		db.Close()
		return nil, err
	}
	return &Store{db: db}, nil
}

func (s *Store) Close() error { return s.db.Close() }

// Encode stores a new moment with full starting weight and stamps its ID.
func (s *Store) Encode(f *Fragment) (uint64, error) {
	f.Weight = 1.0
	res, err := s.db.Exec(`INSERT INTO fragments
		(channel, content, template, surprise, dopamine, noradrenaline, serotonin, at, weight)
		VALUES (?,?,?,?,?,?,?,?,?)`,
		f.Channel, f.Content, f.Template, f.Surprise,
		f.Limbic.Dopamine, f.Limbic.Noradrenaline, f.Limbic.Serotonin,
		f.Time.UnixNano(), f.Weight)
	if err != nil {
		return 0, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, err
	}
	f.ID = uint64(id)
	return f.ID, nil
}

// Sleep mirrors Hippocampus.Sleep in SQL: decay every weight (slower for
// sharper memories), prune what has faded, and drop links whose endpoints
// are gone. The decay law here and the one in memory.go must stay
// identical; TestDecayLawMatchesHippocampus pins them together.
func (s *Store) Sleep(decay, floor float64) (int, error) {
	if _, err := s.db.Exec(
		`UPDATE fragments SET weight = weight * (1.0 - ? / (1.0 + surprise))`, decay); err != nil {
		return 0, err
	}
	res, err := s.db.Exec(`DELETE FROM fragments WHERE weight <= ?`, floor)
	if err != nil {
		return 0, err
	}
	pruned, err := res.RowsAffected()
	if err != nil {
		return 0, err
	}
	if _, err := s.db.Exec(`DELETE FROM links
		WHERE from_id NOT IN (SELECT id FROM fragments)
		   OR to_id   NOT IN (SELECT id FROM fragments)`); err != nil {
		return int(pruned), err
	}
	return int(pruned), nil
}

const fragmentCols = `id, channel, content, template, surprise,
	dopamine, noradrenaline, serotonin, at, weight`

func scanFragment(rows *sql.Rows) (*Fragment, error) {
	var f Fragment
	var at int64
	if err := rows.Scan(&f.ID, &f.Channel, &f.Content, &f.Template, &f.Surprise,
		&f.Limbic.Dopamine, &f.Limbic.Noradrenaline, &f.Limbic.Serotonin,
		&at, &f.Weight); err != nil {
		return nil, err
	}
	f.Time = time.Unix(0, at)
	return &f, nil
}

func (s *Store) query(q string, args ...any) ([]*Fragment, error) {
	rows, err := s.db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*Fragment
	for rows.Next() {
		f, err := scanFragment(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, f)
	}
	return out, rows.Err()
}

// refresh is retrieval practice: recalled memories get their weight
// restored, so what gets used survives consolidation.
func (s *Store) refresh(ids []uint64) error {
	for _, id := range ids {
		if _, err := s.db.Exec(`UPDATE fragments SET weight = 1.0 WHERE id = ?`, id); err != nil {
			return err
		}
	}
	return nil
}

// RecallByStress returns the moment with the highest noradrenaline at
// encoding: "show me when things felt wrong."
func (s *Store) RecallByStress() (*Fragment, error) {
	frags, err := s.query(`SELECT ` + fragmentCols + ` FROM fragments
		ORDER BY noradrenaline DESC, id DESC LIMIT 1`)
	if err != nil || len(frags) == 0 {
		return nil, err
	}
	f := frags[0]
	if err := s.refresh([]uint64{f.ID}); err != nil {
		return nil, err
	}
	f.Weight = 1.0
	return f, nil
}

// Recall returns the most stressful moments whose content or template
// contains the query. Substring match is the fallback until link-based
// retrieval exists; the limbic ordering is already the real thing.
func (s *Store) Recall(query string, limit int) ([]*Fragment, error) {
	q := strings.ToLower(query)
	frags, err := s.query(`SELECT `+fragmentCols+` FROM fragments
		WHERE instr(lower(content), ?) > 0 OR instr(lower(template), ?) > 0
		ORDER BY noradrenaline DESC, id DESC LIMIT ?`, q, q, limit)
	if err != nil {
		return nil, err
	}
	ids := make([]uint64, 0, len(frags))
	for _, f := range frags {
		ids = append(ids, f.ID)
		f.Weight = 1.0
	}
	if err := s.refresh(ids); err != nil {
		return nil, err
	}
	return frags, nil
}

func (s *Store) Dump() ([]*Fragment, error) {
	return s.query(`SELECT ` + fragmentCols + ` FROM fragments ORDER BY id`)
}

func (s *Store) Count() (int, error) {
	var n int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM fragments`).Scan(&n)
	return n, err
}

// SaveModel persists one channel predictor's counts, replacing what was
// there. Called at sleep and shutdown, so a hard kill costs at most the
// learning since the last nap; fragments themselves are never at risk.
func (s *Store) SaveModel(channel string, counts map[string]float64) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`DELETE FROM model_counts WHERE channel = ?`, channel); err != nil {
		return err
	}
	for tpl, c := range counts {
		if _, err := tx.Exec(`INSERT INTO model_counts (channel, template, count) VALUES (?,?,?)`,
			channel, tpl, c); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *Store) LoadModel(channel string) (map[string]float64, error) {
	rows, err := s.db.Query(`SELECT template, count FROM model_counts WHERE channel = ?`, channel)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	counts := make(map[string]float64)
	for rows.Next() {
		var tpl string
		var c float64
		if err := rows.Scan(&tpl, &c); err != nil {
			return nil, err
		}
		counts[tpl] = c
	}
	return counts, rows.Err()
}

// ModelChannels lists the channels with persisted predictor state.
func (s *Store) ModelChannels() ([]string, error) {
	rows, err := s.db.Query(`SELECT DISTINCT channel FROM model_counts`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var ch string
		if err := rows.Scan(&ch); err != nil {
			return nil, err
		}
		out = append(out, ch)
	}
	return out, rows.Err()
}
