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
	weight        REAL    NOT NULL,
	count         INTEGER NOT NULL DEFAULT 1
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
);
CREATE TABLE IF NOT EXISTS assoc (
	tpl_a TEXT    NOT NULL,
	tpl_b TEXT    NOT NULL,
	count INTEGER NOT NULL,
	PRIMARY KEY (tpl_a, tpl_b)
);
CREATE TABLE IF NOT EXISTS calib (
	channel TEXT PRIMARY KEY,
	n       INTEGER NOT NULL,
	mean    REAL    NOT NULL,
	vari    REAL    NOT NULL
);
CREATE TABLE IF NOT EXISTS habits (
	cue       TEXT    NOT NULL,
	action    TEXT    NOT NULL,
	approvals INTEGER NOT NULL,
	denials   INTEGER NOT NULL,
	status    TEXT    NOT NULL,
	PRIMARY KEY (cue, action)
);
CREATE TABLE IF NOT EXISTS audit (
	at      INTEGER NOT NULL,
	cue     TEXT    NOT NULL,
	action  TEXT    NOT NULL,
	outcome TEXT    NOT NULL
);`
	if _, err := db.Exec(schema); err != nil {
		db.Close()
		return nil, err
	}
	// Databases written before consolidation lack the count column;
	// CREATE TABLE IF NOT EXISTS leaves an existing table untouched, so
	// the column is added in place.
	var hasCount int
	if err := db.QueryRow(
		`SELECT COUNT(*) FROM pragma_table_info('fragments') WHERE name = 'count'`).Scan(&hasCount); err != nil {
		db.Close()
		return nil, err
	}
	if hasCount == 0 {
		if _, err := db.Exec(`ALTER TABLE fragments ADD COLUMN count INTEGER NOT NULL DEFAULT 1`); err != nil {
			db.Close()
			return nil, err
		}
	}
	return &Store{db: db}, nil
}

func (s *Store) Close() error { return s.db.Close() }

// Encode stores a new moment with full starting weight and stamps its ID.
func (s *Store) Encode(f *Fragment) (uint64, error) {
	f.Weight = 1.0
	if f.Count < 1 {
		f.Count = 1
	}
	res, err := s.db.Exec(`INSERT INTO fragments
		(channel, content, template, surprise, dopamine, noradrenaline, serotonin, at, weight, count)
		VALUES (?,?,?,?,?,?,?,?,?,?)`,
		f.Channel, f.Content, f.Template, f.Surprise,
		f.Limbic.Dopamine, f.Limbic.Noradrenaline, f.Limbic.Serotonin,
		f.Time.UnixNano(), f.Weight, f.Count)
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

// Nightly fading of the association graph: edges lose a little weight each
// sleep and die below the floor, so only paths that recall keeps walking
// stay strong. The assoc table is capped by evicting never-confirmed pairs
// once it grows past assocCap.
const (
	linkNightlyKeep = 0.98
	linkFloor       = 0.05
	assocCap        = 5000
)

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
	if _, err := s.db.Exec(`UPDATE links SET weight = weight * ?`, linkNightlyKeep); err != nil {
		return int(pruned), err
	}
	if _, err := s.db.Exec(`DELETE FROM links
		WHERE weight <= ?
		   OR from_id NOT IN (SELECT id FROM fragments)
		   OR to_id   NOT IN (SELECT id FROM fragments)`, linkFloor); err != nil {
		return int(pruned), err
	}
	var assocRows int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM assoc`).Scan(&assocRows); err == nil && assocRows > assocCap {
		s.db.Exec(`DELETE FROM assoc WHERE count <= 1`)
	}
	return int(pruned), nil
}

const fragmentCols = `id, channel, content, template, surprise,
	dopamine, noradrenaline, serotonin, at, weight, count`

func scanFragment(rows *sql.Rows) (*Fragment, error) {
	var f Fragment
	var at int64
	if err := rows.Scan(&f.ID, &f.Channel, &f.Content, &f.Template, &f.Surprise,
		&f.Limbic.Dopamine, &f.Limbic.Noradrenaline, &f.Limbic.Serotonin,
		&at, &f.Weight, &f.Count); err != nil {
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

// Refresh is retrieval practice: recalled memories get their weight
// restored, so what gets used survives consolidation. The hot tier calls
// it for its own hits, so practice reaches the archive no matter which
// tier answered.
func (s *Store) Refresh(ids []uint64) error {
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
	if err := s.Refresh([]uint64{f.ID}); err != nil {
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
	if err := s.Refresh(ids); err != nil {
		return nil, err
	}
	return frags, nil
}

func (s *Store) Dump() ([]*Fragment, error) {
	return s.query(`SELECT ` + fragmentCols + ` FROM fragments ORDER BY id`)
}

// StrongSince returns the strongest fragments encoded since a moment, for
// sleep replay. Same-day fragments mostly share full weight, so ties
// break toward the sharpest surprise.
func (s *Store) StrongSince(since time.Time, floor float64, limit int) ([]*Fragment, error) {
	return s.query(`SELECT `+fragmentCols+` FROM fragments
		WHERE at >= ? AND weight >= ?
		ORDER BY weight DESC, surprise DESC, id DESC LIMIT ?`,
		since.UnixNano(), floor, limit)
}

func (s *Store) Count() (int, error) {
	var n int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM fragments`).Scan(&n)
	return n, err
}

// Sleep merges near-duplicates: once a shape has recurred mergeThreshold
// times, its occurrences collapse into a prototype. The newest occurrence
// survives as the prototype and
// carries the group's total count; the single most surprising other
// occurrence stays behind as an exemplar, linked to the prototype; the
// rest are absorbed. Absorbed fragments' edges fold into the prototype
// with their weights summed (capped at 1.0), so groundings and recall
// paths keep their strength through the merge.
const (
	mergeThreshold     = 3
	exemplarLinkWeight = 0.6
)

// MergeDuplicates runs one merge pass and reports how many fragments were
// absorbed into prototypes.
func (s *Store) MergeDuplicates() (int, error) {
	type group struct{ channel, template string }
	rows, err := s.db.Query(`SELECT channel, template FROM fragments
		GROUP BY channel, template HAVING COUNT(*) >= ?`, mergeThreshold)
	if err != nil {
		return 0, err
	}
	var groups []group
	for rows.Next() {
		var g group
		if err := rows.Scan(&g.channel, &g.template); err != nil {
			rows.Close()
			return 0, err
		}
		groups = append(groups, g)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return 0, err
	}

	merged := 0
	for _, g := range groups {
		n, err := s.mergeGroup(g.channel, g.template)
		if err != nil {
			return merged, err
		}
		merged += n
	}
	return merged, nil
}

func (s *Store) mergeGroup(channel, template string) (int, error) {
	type member struct {
		id       uint64
		surprise float64
		weight   float64
		count    int
	}
	rows, err := s.db.Query(`SELECT id, surprise, weight, count FROM fragments
		WHERE channel = ? AND template = ? ORDER BY at DESC, id DESC`, channel, template)
	if err != nil {
		return 0, err
	}
	var members []member
	for rows.Next() {
		var m member
		if err := rows.Scan(&m.id, &m.surprise, &m.weight, &m.count); err != nil {
			rows.Close()
			return 0, err
		}
		members = append(members, m)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return 0, err
	}
	if len(members) < mergeThreshold {
		return 0, nil
	}

	proto := members[0]
	exemplar := members[1]
	for _, m := range members[2:] {
		if m.surprise > exemplar.surprise {
			exemplar = m
		}
	}
	fold := map[uint64]bool{proto.id: true}
	total, maxWeight := proto.count, proto.weight
	var absorbed []uint64
	for _, m := range members[1:] {
		if m.id == exemplar.id {
			continue
		}
		fold[m.id] = true
		absorbed = append(absorbed, m.id)
		total += m.count
		if m.weight > maxWeight {
			maxWeight = m.weight
		}
	}

	// Aggregate every edge touching the fold set onto the prototype.
	// Edges inside the fold set are self-association and drop.
	ids := make([]any, 0, len(fold))
	marks := make([]string, 0, len(fold))
	for id := range fold {
		ids = append(ids, id)
		marks = append(marks, "?")
	}
	in := strings.Join(marks, ",")
	rows, err = s.db.Query(`SELECT from_id, to_id, weight FROM links
		WHERE from_id IN (`+in+`) OR to_id IN (`+in+`)`, append(append([]any{}, ids...), ids...)...)
	if err != nil {
		return 0, err
	}
	outW := map[uint64]float64{}
	inW := map[uint64]float64{}
	for rows.Next() {
		var e LinkEdge
		if err := rows.Scan(&e.From, &e.To, &e.Weight); err != nil {
			rows.Close()
			return 0, err
		}
		switch {
		case fold[e.From] && fold[e.To]:
		case fold[e.From]:
			outW[e.To] += e.Weight
		default:
			inW[e.From] += e.Weight
		}
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return 0, err
	}

	tx, err := s.db.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`DELETE FROM links WHERE from_id IN (`+in+`) OR to_id IN (`+in+`)`,
		append(append([]any{}, ids...), ids...)...); err != nil {
		return 0, err
	}
	put := func(from, to uint64, w float64) error {
		if w > 1 {
			w = 1
		}
		_, err := tx.Exec(`INSERT INTO links (from_id, to_id, weight) VALUES (?,?,?)
			ON CONFLICT(from_id, to_id) DO UPDATE SET weight = max(weight, excluded.weight)`,
			from, to, w)
		return err
	}
	for to, w := range outW {
		if err := put(proto.id, to, w); err != nil {
			return 0, err
		}
	}
	for from, w := range inW {
		if err := put(from, proto.id, w); err != nil {
			return 0, err
		}
	}
	// The exemplar pointer: the prototype and its kept exemplar stay
	// reachable from each other.
	if err := put(proto.id, exemplar.id, exemplarLinkWeight); err != nil {
		return 0, err
	}
	if err := put(exemplar.id, proto.id, exemplarLinkWeight); err != nil {
		return 0, err
	}
	if len(absorbed) > 0 {
		am := make([]string, len(absorbed))
		av := make([]any, len(absorbed))
		for i, id := range absorbed {
			am[i] = "?"
			av[i] = id
		}
		if _, err := tx.Exec(`DELETE FROM fragments WHERE id IN (`+strings.Join(am, ",")+`)`, av...); err != nil {
			return 0, err
		}
	}
	if _, err := tx.Exec(`UPDATE fragments SET count = ?, weight = ? WHERE id = ?`,
		total, maxWeight, proto.id); err != nil {
		return 0, err
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return len(absorbed), nil
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

// LinkEdge is one directed association edge between fragments.
type LinkEdge struct {
	From   uint64
	To     uint64
	Weight float64
}

// Link records an association edge. A repeated link keeps the strongest
// weight it has ever been given.
func (s *Store) Link(from, to uint64, weight float64) error {
	_, err := s.db.Exec(`INSERT INTO links (from_id, to_id, weight) VALUES (?,?,?)
		ON CONFLICT(from_id, to_id) DO UPDATE SET weight = max(weight, excluded.weight)`,
		from, to, weight)
	return err
}

// LinksFrom returns every edge leaving the given fragments.
func (s *Store) LinksFrom(ids []uint64) ([]LinkEdge, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	args := make([]any, len(ids))
	marks := make([]string, len(ids))
	for i, id := range ids {
		args[i] = id
		marks[i] = "?"
	}
	rows, err := s.db.Query(`SELECT from_id, to_id, weight FROM links
		WHERE from_id IN (`+strings.Join(marks, ",")+`)`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []LinkEdge
	for rows.Next() {
		var e LinkEdge
		if err := rows.Scan(&e.From, &e.To, &e.Weight); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// StrengthenLinks is retrieval practice for edges: every traversed link,
// in both directions, gains a little weight, capped at 1.0. This is the
// counterpart of the nightly link decay.
func (s *Store) StrengthenLinks(edges []LinkEdge, delta float64) error {
	for _, e := range edges {
		if _, err := s.db.Exec(`UPDATE links SET weight = min(1.0, weight + ?)
			WHERE (from_id = ? AND to_id = ?) OR (from_id = ? AND to_id = ?)`,
			delta, e.From, e.To, e.To, e.From); err != nil {
			return err
		}
	}
	return nil
}

// NudgeLinks shifts a set of edges in both directions, clamped to [0, 1].
// Approval strengthens the cue-to-action path; denial weakens it.
func (s *Store) NudgeLinks(edges []LinkEdge, delta float64) error {
	for _, e := range edges {
		if _, err := s.db.Exec(`UPDATE links SET weight = max(0.0, min(1.0, weight + ?))
			WHERE (from_id = ? AND to_id = ?) OR (from_id = ? AND to_id = ?)`,
			delta, e.From, e.To, e.To, e.From); err != nil {
			return err
		}
	}
	return nil
}

// BumpAssoc counts one more co-occurrence of a template pair, order
// insensitive, and returns the running count. The count is how the system
// notices that an association is being confirmed by repetition.
func (s *Store) BumpAssoc(a, b string) (int, error) {
	if a > b {
		a, b = b, a
	}
	var count int
	err := s.db.QueryRow(`INSERT INTO assoc (tpl_a, tpl_b, count) VALUES (?,?,1)
		ON CONFLICT(tpl_a, tpl_b) DO UPDATE SET count = count + 1
		RETURNING count`, a, b).Scan(&count)
	return count, err
}

// FragmentsByIDs fetches specific fragments; the order of the result is
// not significant.
func (s *Store) FragmentsByIDs(ids []uint64) ([]*Fragment, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	args := make([]any, len(ids))
	marks := make([]string, len(ids))
	for i, id := range ids {
		args[i] = id
		marks[i] = "?"
	}
	return s.query(`SELECT `+fragmentCols+` FROM fragments
		WHERE id IN (`+strings.Join(marks, ",")+`)`, args...)
}

// LinkCounts returns each fragment's out-degree, for display.
func (s *Store) LinkCounts() (map[uint64]int, error) {
	rows, err := s.db.Query(`SELECT from_id, COUNT(*) FROM links GROUP BY from_id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make(map[uint64]int)
	for rows.Next() {
		var id uint64
		var n int
		if err := rows.Scan(&id, &n); err != nil {
			return nil, err
		}
		out[id] = n
	}
	return out, rows.Err()
}

// Checkpoint folds the write-ahead log back into the main database file,
// keeping the on-disk footprint honest. Sleep is the natural moment.
func (s *Store) Checkpoint() error {
	_, err := s.db.Exec(`PRAGMA wal_checkpoint(TRUNCATE)`)
	return err
}

// Habit is the durable record of how a cue-and-command pair has been
// judged: enough consistent approvals graduate it to autonomy, any denial
// demotes it.
type Habit struct {
	Cue       string
	Action    string
	Approvals int
	Denials   int
	Status    string
}

const (
	HabitProposing = "proposing"
	HabitGraduated = "graduated"
	HabitDemoted   = "demoted"
)

// LoadHabit returns the stored judgement for a pair, or a zero-value
// proposing habit if the pair is new.
func (s *Store) LoadHabit(cue, action string) (Habit, error) {
	h := Habit{Cue: cue, Action: action, Status: HabitProposing}
	err := s.db.QueryRow(`SELECT approvals, denials, status FROM habits WHERE cue = ? AND action = ?`,
		cue, action).Scan(&h.Approvals, &h.Denials, &h.Status)
	if err == sql.ErrNoRows {
		return h, nil
	}
	return h, err
}

func (s *Store) SaveHabit(h Habit) error {
	_, err := s.db.Exec(`INSERT INTO habits (cue, action, approvals, denials, status)
		VALUES (?,?,?,?,?)
		ON CONFLICT(cue, action) DO UPDATE SET
			approvals = excluded.approvals, denials = excluded.denials, status = excluded.status`,
		h.Cue, h.Action, h.Approvals, h.Denials, h.Status)
	return err
}

// GraduatedCues lists the cue templates of every graduated habit, the
// trigger set for automatic cueing.
func (s *Store) GraduatedCues() ([]string, error) {
	rows, err := s.db.Query(`SELECT DISTINCT cue FROM habits WHERE status = ?`, HabitGraduated)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var cue string
		if err := rows.Scan(&cue); err != nil {
			return nil, err
		}
		out = append(out, cue)
	}
	return out, rows.Err()
}

// AllHabits returns every grounded pair, graduated first.
func (s *Store) AllHabits() ([]Habit, error) {
	rows, err := s.db.Query(`SELECT cue, action, approvals, denials, status FROM habits
		ORDER BY status, approvals DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Habit
	for rows.Next() {
		var h Habit
		if err := rows.Scan(&h.Cue, &h.Action, &h.Approvals, &h.Denials, &h.Status); err != nil {
			return nil, err
		}
		out = append(out, h)
	}
	return out, rows.Err()
}

// Audit appends one decision outcome to the durable trail.
func (s *Store) Audit(at int64, cue, action, outcome string) error {
	_, err := s.db.Exec(`INSERT INTO audit (at, cue, action, outcome) VALUES (?,?,?,?)`,
		at, cue, action, outcome)
	return err
}

// AuditEntry is one line of the decision trail.
type AuditEntry struct {
	At      int64
	Cue     string
	Action  string
	Outcome string
}

func (s *Store) AuditTail(limit int) ([]AuditEntry, error) {
	// rowid breaks same-second ties so the newest entry is always first.
	rows, err := s.db.Query(`SELECT at, cue, action, outcome FROM audit
		ORDER BY at DESC, rowid DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []AuditEntry
	for rows.Next() {
		var e AuditEntry
		if err := rows.Scan(&e.At, &e.Cue, &e.Action, &e.Outcome); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// CalibState is one channel's persisted saliency calibration.
type CalibState struct {
	N    int
	Mean float64
	Vari float64
}

// SaveCalib persists one channel's saliency calibration. It is flushed on
// the same schedule as the predictor models.
func (s *Store) SaveCalib(channel string, n int, mean, vari float64) error {
	_, err := s.db.Exec(`INSERT INTO calib (channel, n, mean, vari) VALUES (?,?,?,?)
		ON CONFLICT(channel) DO UPDATE SET n=excluded.n, mean=excluded.mean, vari=excluded.vari`,
		channel, n, mean, vari)
	return err
}

// LoadCalibs returns every persisted channel calibration.
func (s *Store) LoadCalibs() (map[string]CalibState, error) {
	rows, err := s.db.Query(`SELECT channel, n, mean, vari FROM calib`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make(map[string]CalibState)
	for rows.Next() {
		var ch string
		var cs CalibState
		if err := rows.Scan(&ch, &cs.N, &cs.Mean, &cs.Vari); err != nil {
			return nil, err
		}
		out[ch] = cs
	}
	return out, rows.Err()
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
