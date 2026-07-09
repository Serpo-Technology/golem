package memory

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
	count         INTEGER NOT NULL DEFAULT 1,
	mass          REAL    NOT NULL DEFAULT 1.0,
	kind          TEXT    NOT NULL DEFAULT 'episodic',
	lang          TEXT    NOT NULL DEFAULT '',
	category      TEXT    NOT NULL DEFAULT '',
	edited        INTEGER NOT NULL DEFAULT 0
);
CREATE TABLE IF NOT EXISTS links (
	from_id INTEGER NOT NULL,
	to_id   INTEGER NOT NULL,
	rel     TEXT    NOT NULL DEFAULT 'temporal',
	weight  REAL    NOT NULL,
	PRIMARY KEY (from_id, to_id, rel)
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
CREATE TABLE IF NOT EXISTS succ (
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
);
CREATE TABLE IF NOT EXISTS media (
	fragment_id INTEGER PRIMARY KEY,
	hash        TEXT    NOT NULL,
	mime        TEXT    NOT NULL DEFAULT ''
);
CREATE TABLE IF NOT EXISTS vectors (
	fragment_id INTEGER PRIMARY KEY,
	vec         BLOB    NOT NULL
);
CREATE TABLE IF NOT EXISTS usage_vectors (
	fragment_id INTEGER NOT NULL,
	role        TEXT    NOT NULL,
	vec         BLOB    NOT NULL,
	PRIMARY KEY (fragment_id, role)
);
CREATE TABLE IF NOT EXISTS holo_vectors (
	fragment_id INTEGER PRIMARY KEY,
	vec         BLOB    NOT NULL
);
CREATE TABLE IF NOT EXISTS beliefs (
	rel         TEXT    NOT NULL,
	agent       INTEGER NOT NULL,
	patient     INTEGER NOT NULL,
	agent_gloss TEXT    NOT NULL DEFAULT '',
	pat_gloss   TEXT    NOT NULL DEFAULT '',
	conf        REAL    NOT NULL,
	depth       INTEGER NOT NULL DEFAULT 0,
	PRIMARY KEY (rel, agent, patient)
);
CREATE TABLE IF NOT EXISTS derived (
	rel         TEXT    NOT NULL,
	agent       INTEGER NOT NULL,
	patient     INTEGER NOT NULL,
	agent_gloss TEXT    NOT NULL DEFAULT '',
	pat_gloss   TEXT    NOT NULL DEFAULT '',
	kind        TEXT    NOT NULL,
	PRIMARY KEY (rel, agent, patient)
);
CREATE TABLE IF NOT EXISTS feature_universal (
	feature_id INTEGER PRIMARY KEY,
	frac       REAL    NOT NULL
);
CREATE TABLE IF NOT EXISTS colour_prototypes (
	cell TEXT PRIMARY KEY,
	r    REAL NOT NULL,
	g    REAL NOT NULL,
	b    REAL NOT NULL
);
CREATE TABLE IF NOT EXISTS source_keys (
	name   TEXT PRIMARY KEY,
	pubkey TEXT NOT NULL
);`
	if _, err := db.Exec(schema); err != nil {
		db.Close()
		return nil, err
	}
	if err := migrate(db); err != nil {
		db.Close()
		return nil, err
	}
	// Secondary indexes, created after migration so the columns they cover exist on an
	// upgraded brain. The fragments PK is only the id and the links PK leads with
	// from_id, so a lookup by template (ConceptByTemplate, called per word every
	// consolidation and on every comprehension) or a walk by to_id (categoriesOf and
	// the is-a hierarchy, the nightly prune subqueries) was a full table scan. These
	// turn those hot paths into index lookups, which scales both the night and the
	// per-utterance reasoning as the graph grows.
	indexes := `
CREATE INDEX IF NOT EXISTS idx_fragments_template ON fragments(template);
CREATE INDEX IF NOT EXISTS idx_fragments_kind ON fragments(kind);
CREATE INDEX IF NOT EXISTS idx_links_to ON links(to_id, rel);`
	if _, err := db.Exec(indexes); err != nil {
		db.Close()
		return nil, err
	}
	return &Store{db: db}, nil
}

// migrate brings a database written by an earlier build up to the current
// schema. CREATE TABLE IF NOT EXISTS leaves existing tables untouched, so
// missing columns are added in place; the links table is rebuilt when it
// predates edge relations, because its primary key had to grow.
func migrate(db *sql.DB) error {
	hasColumn := func(table, column string) (bool, error) {
		var n int
		err := db.QueryRow(
			`SELECT COUNT(*) FROM pragma_table_info(?) WHERE name = ?`, table, column).Scan(&n)
		return n > 0, err
	}

	addColumns := []struct{ name, decl string }{
		{"count", `count INTEGER NOT NULL DEFAULT 1`},
		{"mass", `mass REAL NOT NULL DEFAULT 1.0`},
		{"kind", `kind TEXT NOT NULL DEFAULT 'episodic'`},
		{"lang", `lang TEXT NOT NULL DEFAULT ''`},
		{"category", `category TEXT NOT NULL DEFAULT ''`},
		{"edited", `edited INTEGER NOT NULL DEFAULT 0`},
	}
	massExisted, err := hasColumn("fragments", "mass")
	if err != nil {
		return err
	}
	for _, col := range addColumns {
		ok, err := hasColumn("fragments", col.name)
		if err != nil {
			return err
		}
		if !ok {
			if _, err := db.Exec(`ALTER TABLE fragments ADD COLUMN ` + col.decl); err != nil {
				return err
			}
		}
	}
	if !massExisted {
		// A brain written before the mass column has every fragment at the ADD-COLUMN default (1.0),
		// which would strand an already-recurrent shape below the genesis threshold. Backfill mass to
		// count so genesis stays exactly count-gated for what the old brain already accumulated.
		if _, err := db.Exec(`UPDATE fragments SET mass = count`); err != nil {
			return err
		}
	}

	ok, err := hasColumn("links", "rel")
	if err != nil {
		return err
	}
	if !ok {
		// Every pre-relation edge was woven by co-occurrence.
		_, err := db.Exec(`
			CREATE TABLE links_new (
				from_id INTEGER NOT NULL,
				to_id   INTEGER NOT NULL,
				rel     TEXT    NOT NULL DEFAULT 'temporal',
				weight  REAL    NOT NULL,
				PRIMARY KEY (from_id, to_id, rel)
			);
			INSERT INTO links_new (from_id, to_id, rel, weight)
				SELECT from_id, to_id, 'temporal', weight FROM links;
			DROP TABLE links;
			ALTER TABLE links_new RENAME TO links;`)
		if err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) Close() error { return s.db.Close() }

// Encode stores a new node and stamps its ID. The starting weight is
// full unless the caller pre-set a reduced one (untrusted channels
// remember lightly); most encodes are moments, and word and concept
// nodes pass their kind in.
func (s *Store) Encode(f *Fragment) (uint64, error) {
	if f.Weight <= 0 || f.Weight > 1 {
		f.Weight = 1.0
	}
	if f.Count < 1 {
		f.Count = 1
	}
	if f.Mass <= 0 {
		// A conscious occurrence weighs a full unit per count, so mass defaults to the count and
		// genesis stays exactly count-gated; only the graded gate lowers an occurrence's mass.
		f.Mass = float64(f.Count)
	}
	if f.Kind == "" {
		f.Kind = KindEpisodic
	}
	res, err := s.db.Exec(`INSERT INTO fragments
		(channel, content, template, surprise, dopamine, noradrenaline, serotonin, at, weight, count, mass, kind, lang, category, edited)
		VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		f.Channel, f.Content, f.Template, f.Surprise,
		f.Limbic.Dopamine, f.Limbic.Noradrenaline, f.Limbic.Serotonin,
		f.Time.UnixNano(), f.Weight, f.Count, f.Mass, f.Kind, f.Lang, f.Category, boolToInt(f.Edited))
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
//
// Only episodic nodes live under the nightly fragment decay. Words and
// concepts are semantic memory: they die by disconnection instead, once
// every edge that justified them has faded away.
func (s *Store) Sleep(decay, floor float64) (int, error) {
	if _, err := s.db.Exec(
		`UPDATE fragments SET weight = weight * (1.0 - ? / (1.0 + surprise))
		 WHERE kind = ?`, decay, KindEpisodic); err != nil {
		return 0, err
	}
	res, err := s.db.Exec(`DELETE FROM fragments WHERE weight <= ? AND kind = ?`, floor, KindEpisodic)
	if err != nil {
		return 0, err
	}
	pruned, err := res.RowsAffected()
	if err != nil {
		return 0, err
	}
	// An edge into grammar scaffolding (a construction's slot, a phrase rule's daughter, a word's
	// form) is structural rather than associative: it defines the pattern, it does not fade with
	// disuse. So it is spared the nightly decay and stands at full weight, which keeps a taught
	// grammar pattern intact across nights instead of thinning toward the link floor. A naming
	// bond is spared too: the nightly recount is the one authority over names and moves every bond
	// whose word still has lived company, so the generic decay would only thin the bonds the
	// recount holds frozen once their word's company has starved, surrendering an earned name to
	// disuse the recount is deliberately keeping.
	// An edge touching a code node is structural too: it holds a piece of a learned
	// function's meaning graph, which the engine walks on load to reconstruct the body. So
	// every edge into or out of a code node is spared the nightly decay and stands at full
	// weight, keeping the durable code subgraph intact across nights rather than thinning it
	// toward the link floor where the orphan-link drop would erase the function.
	if _, err := s.db.Exec(`UPDATE links SET weight = weight * ?
		WHERE rel <> ? AND rel <> ?
		AND to_id NOT IN (SELECT id FROM fragments WHERE category IN ('construction', 'declgrammar', 'class', 'answer', 'source', 'number', 'participant'))
		AND to_id   NOT IN (SELECT id FROM fragments WHERE kind = ?)
		AND from_id NOT IN (SELECT id FROM fragments WHERE kind = ?)`,
		linkNightlyKeep, RelNames, RelComparativeForm, KindCodeNode, KindCodeNode); err != nil {
		return int(pruned), err
	}
	dropOrphanLinks := func() error {
		_, err := s.db.Exec(`DELETE FROM links
			WHERE weight <= ?
			   OR from_id NOT IN (SELECT id FROM fragments)
			   OR to_id   NOT IN (SELECT id FROM fragments)`, linkFloor)
		return err
	}
	if err := dropOrphanLinks(); err != nil {
		return int(pruned), err
	}
	// An utterance never instantiates a thing, so a grounds edge from a
	// concept to a tell or thought moment is a leftover that lets a concept
	// born from a question die of disconnection on the same night. A relation
	// is the exception: a narrated event is a real instance of the relation it
	// states, so a relation grounding in the moment it was told in stands, and
	// the proposition comprehension wrote persists the way a perceived one
	// would rather than being purged before it can be consolidated. A told comparison is
	// likewise a real instance of the ordering on its scale, so a moment grounding a scale
	// dimension (the target of a pole's scale edge) is spared the same way the relation is,
	// or every told comparison would disconnect from its scale on the first night.
	if _, err := s.db.Exec(`DELETE FROM links WHERE rel = ?
		AND to_id IN (SELECT id FROM fragments WHERE channel IN (?, ?))
		AND from_id NOT IN (SELECT id FROM fragments WHERE category = ?)
		AND from_id NOT IN (SELECT to_id FROM links WHERE rel = ?)`,
		RelGrounds, utteranceChannel, thoughtChannel, "relation", RelScale); err != nil {
		return int(pruned), err
	}
	// A word that no edge touches names nothing and recalls nothing; a
	// concept that neither grounds in a percept nor stands in a proposition
	// stands for nothing. An abstract word names no percept but is held by the
	// company it keeps, so a concept that fills a role survives with no grounding.
	// What stands for nothing fades, and the edges that pointed at it follow.
	if _, err := s.db.Exec(`DELETE FROM fragments WHERE kind = ?
		AND id NOT IN (SELECT from_id FROM links)
		AND id NOT IN (SELECT to_id FROM links)`, KindWord); err != nil {
		return int(pruned), err
	}
	// Grammar scaffolding (an argument-structure construction, a declarative phrase rule or
	// lexical form, a grammatical class) is the grammar living in the graph rather than a
	// perceptual concept; it stands for a learned form-meaning fact, not a thing seen, so it
	// is spared the grounding test. This is what lets a grammar pattern taught into the graph
	// survive the night as a fluid weight rather than fade and re-grow only from the seed.
	if _, err := s.db.Exec(`DELETE FROM fragments WHERE kind = ?
		AND category NOT IN ('construction', 'declgrammar', 'class', 'answer', 'source', 'number', 'participant')
		AND id NOT IN (SELECT from_id FROM links WHERE rel = ?)
			AND id NOT IN (SELECT to_id FROM links WHERE rel = ? OR rel = ?)
			AND id NOT IN (SELECT to_id FROM links WHERE rel = ?
				AND from_id IN (SELECT id FROM fragments WHERE category = 'participant'))`,
		KindConcept, RelGrounds, RelAgent, RelPatient, RelToldIsA); err != nil {
		return int(pruned), err
	}
	// A feature no concept carries describes nothing; it follows its
	// last bearer out of the graph.
	if _, err := s.db.Exec(`DELETE FROM fragments WHERE kind = ?
		AND id NOT IN (SELECT to_id FROM links WHERE rel = ?)`, KindFeature, RelFeature); err != nil {
		return int(pruned), err
	}
	if err := dropOrphanLinks(); err != nil {
		return int(pruned), err
	}
	var assocRows int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM assoc`).Scan(&assocRows); err == nil && assocRows > assocCap {
		s.db.Exec(`DELETE FROM assoc WHERE count <= 1`)
	}
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM succ`).Scan(&assocRows); err == nil && assocRows > assocCap {
		s.db.Exec(`DELETE FROM succ WHERE count <= 1`)
	}
	return int(pruned), nil
}

const fragmentCols = `id, channel, content, template, surprise,
	dopamine, noradrenaline, serotonin, at, weight, count, kind, lang, category, edited, mass`

func scanFragment(rows *sql.Rows) (*Fragment, error) {
	var f Fragment
	var at int64
	var edited int64
	if err := rows.Scan(&f.ID, &f.Channel, &f.Content, &f.Template, &f.Surprise,
		&f.Limbic.Dopamine, &f.Limbic.Noradrenaline, &f.Limbic.Serotonin,
		&at, &f.Weight, &f.Count, &f.Kind, &f.Lang, &f.Category, &edited, &f.Mass); err != nil {
		return nil, err
	}
	f.Time = time.Unix(0, at)
	f.Edited = edited != 0
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

// NodesByKind returns every node of one kind, oldest first. Operation
// and grammar nodes are few, so callers load them whole.
func (s *Store) NodesByKind(kind string) ([]*Fragment, error) {
	return s.query(`SELECT `+fragmentCols+` FROM fragments WHERE kind = ? ORDER BY id`, kind)
}

// SetWeight writes one node's weight outright. Operation and grammar
// nodes carry learned parameters in their weight, owned by the passes
// that recompute them, so the write goes down as well as up.
func (s *Store) SetWeight(id uint64, w float64) error {
	_, err := s.db.Exec(`UPDATE fragments SET weight = ? WHERE id = ?`, w, id)
	return err
}

// MarkEdited records that a fragment was hand-touched through the editor, so a
// shaped memory can be told from a lived one when the mark is used.
func (s *Store) MarkEdited(id uint64) error {
	_, err := s.db.Exec(`UPDATE fragments SET edited = 1 WHERE id = ?`, id)
	return err
}

// UpdateContent splices a fragment's content and the template prediction runs
// on, marking it edited in the same write. It is the raw scalpel the editor's
// surgery mode uses; the caller logs every such write to the trail.
func (s *Store) UpdateContent(id uint64, content, template string) error {
	_, err := s.db.Exec(`UPDATE fragments SET content = ?, template = ?, edited = 1 WHERE id = ?`,
		content, template, id)
	return err
}

// UpdateCategory rewrites a fragment's grammatical category. A seeded closed-class
// word's category is structural, not learned, so when the seed re-classes a word the
// stored node is reconciled to the floor at boot, the way a fresh word is seeded.
func (s *Store) UpdateCategory(id uint64, category string) error {
	_, err := s.db.Exec(`UPDATE fragments SET category = ? WHERE id = ?`, category, id)
	return err
}

// DeleteByID removes one fragment by its id and every edge touching it,
// reporting whether a node was there to go. It is deletion in the raw, distinct
// from Forget's template sweep: it takes exactly the node named.
func (s *Store) DeleteByID(id uint64) (int, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`DELETE FROM links WHERE from_id = ? OR to_id = ?`, id, id); err != nil {
		return 0, err
	}
	res, err := tx.Exec(`DELETE FROM fragments WHERE id = ?`, id)
	if err != nil {
		return 0, err
	}
	n, _ := res.RowsAffected()
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return int(n), nil
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

// DeleteByTemplate removes every node holding a template key and the edges
// touching them, reporting how many nodes went. A construction or grammar
// template is a unique key, so this forgets one parameter outright; a
// concrete shape's template is shared by its episodes and their concept,
// so forgetting it takes the whole trace, the true-forget a demoted node
// never gives.
func (s *Store) DeleteByTemplate(tpl string) (int, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`DELETE FROM links WHERE from_id IN (SELECT id FROM fragments WHERE template = ?)
		OR to_id IN (SELECT id FROM fragments WHERE template = ?)`, tpl, tpl); err != nil {
		return 0, err
	}
	res, err := tx.Exec(`DELETE FROM fragments WHERE template = ?`, tpl)
	if err != nil {
		return 0, err
	}
	n, _ := res.RowsAffected()
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return int(n), nil
}

// Forget removes every node holding a template key and the edges touching it,
// then sweeps any name-word, feature, or moment the cut leaves with no edges
// at all. So deleting a concept also clears the name that pointed only at it
// and the moments that were only about it, in one pass, while shared
// neighbors and every other concept keep their edges and survive. The count
// returned includes the swept satellites.
func (s *Store) Forget(tpl string) (int, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	// The neighbors of the doomed nodes, gathered before the cut so each can
	// be re-checked once its edge to the deleted node is gone.
	rows, err := tx.Query(`SELECT from_id FROM links WHERE to_id IN (SELECT id FROM fragments WHERE template = ?)
		UNION SELECT to_id FROM links WHERE from_id IN (SELECT id FROM fragments WHERE template = ?)`, tpl, tpl)
	if err != nil {
		return 0, err
	}
	var neighbors []uint64
	for rows.Next() {
		var id uint64
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return 0, err
		}
		neighbors = append(neighbors, id)
	}
	rows.Close()
	if _, err := tx.Exec(`DELETE FROM links WHERE from_id IN (SELECT id FROM fragments WHERE template = ?)
		OR to_id IN (SELECT id FROM fragments WHERE template = ?)`, tpl, tpl); err != nil {
		return 0, err
	}
	res, err := tx.Exec(`DELETE FROM fragments WHERE template = ?`, tpl)
	if err != nil {
		return 0, err
	}
	n, _ := res.RowsAffected()
	// A name-word, feature, or moment now touching nothing existed only to
	// describe what is gone, so it goes with it. Concepts and machinery are
	// spared even when the cut leaves them momentarily isolated, so forgetting
	// one concept never silently deletes another.
	for _, id := range neighbors {
		var cnt int
		if err := tx.QueryRow(`SELECT COUNT(*) FROM links WHERE from_id = ? OR to_id = ?`, id, id).Scan(&cnt); err != nil {
			return 0, err
		}
		if cnt > 0 {
			continue
		}
		swept, err := tx.Exec(`DELETE FROM fragments WHERE id = ? AND kind IN (?, ?, ?)`,
			id, KindWord, KindFeature, KindEpisodic)
		if err != nil {
			return 0, err
		}
		if k, _ := swept.RowsAffected(); k > 0 {
			n += k
		}
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return int(n), nil
}

// SetInfo records a node's information, the diversity of its grounding, in
// the surprise column, which a concept does not otherwise use. Salience and
// persistence read it so the system weights a node by how informative it
// is rather than how often it has recurred.
func (s *Store) SetInfo(id uint64, info float64) error {
	_, err := s.db.Exec(`UPDATE fragments SET surprise = ? WHERE id = ?`, info, id)
	return err
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
	frags, err := s.query(`SELECT `+fragmentCols+` FROM fragments
		WHERE kind = ?
		ORDER BY noradrenaline DESC, id DESC LIMIT 1`, KindEpisodic)
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
// contains the query. Substring match finds the literal moments; the
// semantic walk through words and concepts widens it to paraphrases.
func (s *Store) Recall(query string, limit int) ([]*Fragment, error) {
	q := strings.ToLower(query)
	// Grounded recollections rank above the inner loop's self-talk, which
	// substring-matches a query by carrying a concept's gloss verbatim. The
	// demotion is in the ORDER BY so it survives the LIMIT cut: a low-stress
	// sighting is kept over a high-stress thought. Thoughts stay recallable.
	frags, err := s.query(`SELECT `+fragmentCols+` FROM fragments
		WHERE kind = ? AND (instr(lower(content), ?) > 0 OR instr(lower(template), ?) > 0)
		ORDER BY (channel = ?) ASC, noradrenaline DESC, id DESC LIMIT ?`,
		KindEpisodic, q, q, thoughtChannel, limit)
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

// StrongSince returns the strongest moments encoded since a time, for
// sleep replay. Same-day fragments mostly share full weight, so ties
// break toward the sharpest surprise. Only episodic nodes replay; words
// and concepts are not moments.
func (s *Store) StrongSince(since time.Time, floor float64, limit int) ([]*Fragment, error) {
	return s.query(`SELECT `+fragmentCols+` FROM fragments
		WHERE at >= ? AND weight >= ? AND kind = ?
		ORDER BY weight DESC, surprise DESC, id DESC LIMIT ?`,
		since.UnixNano(), floor, KindEpisodic, limit)
}

func (s *Store) Count() (int, error) {
	var n int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM fragments`).Scan(&n)
	return n, err
}

// CountKind reports how many nodes of one kind are held.
func (s *Store) CountKind(kind string) (int, error) {
	var n int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM fragments WHERE kind = ?`, kind).Scan(&n)
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
	MergeThreshold     = 3
	exemplarLinkWeight = 0.6
)

// MergeDuplicates runs one merge pass and reports how many fragments were
// absorbed into prototypes. Only moments merge; word and concept nodes
// are already one node per form.
func (s *Store) MergeDuplicates() (int, error) {
	type group struct{ channel, template string }
	rows, err := s.db.Query(`SELECT channel, template FROM fragments
		WHERE kind = '`+KindEpisodic+`'
		GROUP BY channel, template HAVING COUNT(*) >= ?`, MergeThreshold)
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
		mass     float64
	}
	rows, err := s.db.Query(`SELECT id, surprise, weight, count, mass FROM fragments
		WHERE channel = ? AND template = ? AND kind = '`+KindEpisodic+`'
		ORDER BY at DESC, id DESC`, channel, template)
	if err != nil {
		return 0, err
	}
	var members []member
	for rows.Next() {
		var m member
		if err := rows.Scan(&m.id, &m.surprise, &m.weight, &m.count, &m.mass); err != nil {
			rows.Close()
			return 0, err
		}
		members = append(members, m)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return 0, err
	}
	if len(members) < MergeThreshold {
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
	total, maxWeight, totalMass := proto.count, proto.weight, proto.mass
	var absorbed []uint64
	for _, m := range members[1:] {
		if m.id == exemplar.id {
			continue
		}
		fold[m.id] = true
		absorbed = append(absorbed, m.id)
		total += m.count
		totalMass += m.mass
		if m.weight > maxWeight {
			maxWeight = m.weight
		}
	}

	// Aggregate every edge touching the fold set onto the prototype,
	// per relation, so a grounding stays a grounding and an order stays
	// an order. Edges inside the fold set are self-association and drop.
	ids := make([]any, 0, len(fold))
	marks := make([]string, 0, len(fold))
	for id := range fold {
		ids = append(ids, id)
		marks = append(marks, "?")
	}
	in := strings.Join(marks, ",")
	rows, err = s.db.Query(`SELECT from_id, to_id, rel, weight FROM links
		WHERE from_id IN (`+in+`) OR to_id IN (`+in+`)`, append(append([]any{}, ids...), ids...)...)
	if err != nil {
		return 0, err
	}
	type relEnd struct {
		id  uint64
		rel string
	}
	outW := map[relEnd]float64{}
	inW := map[relEnd]float64{}
	for rows.Next() {
		var e LinkEdge
		if err := rows.Scan(&e.From, &e.To, &e.Rel, &e.Weight); err != nil {
			rows.Close()
			return 0, err
		}
		switch {
		case fold[e.From] && fold[e.To]:
		case fold[e.From]:
			outW[relEnd{e.To, e.Rel}] += e.Weight
		default:
			inW[relEnd{e.From, e.Rel}] += e.Weight
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
	put := func(from, to uint64, rel string, w float64) error {
		if w > 1 {
			w = 1
		}
		_, err := tx.Exec(`INSERT INTO links (from_id, to_id, rel, weight) VALUES (?,?,?,?)
			ON CONFLICT(from_id, to_id, rel) DO UPDATE SET weight = max(weight, excluded.weight)`,
			from, to, rel, w)
		return err
	}
	for end, w := range outW {
		if err := put(proto.id, end.id, end.rel, w); err != nil {
			return 0, err
		}
	}
	for end, w := range inW {
		if err := put(end.id, proto.id, end.rel, w); err != nil {
			return 0, err
		}
	}
	// The exemplar pointer: the prototype and its kept exemplar stay
	// reachable from each other.
	if err := put(proto.id, exemplar.id, RelTemporal, exemplarLinkWeight); err != nil {
		return 0, err
	}
	if err := put(exemplar.id, proto.id, RelTemporal, exemplarLinkWeight); err != nil {
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
	if _, err := tx.Exec(`UPDATE fragments SET count = ?, weight = ?, mass = ? WHERE id = ?`,
		total, maxWeight, totalMass, proto.id); err != nil {
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

// LinkEdge is one directed, typed association edge between fragments.
type LinkEdge struct {
	From   uint64
	To     uint64
	Rel    string
	Weight float64
}

// Link records a co-occurrence edge. A repeated link keeps the strongest
// weight it has ever been given.
func (s *Store) Link(from, to uint64, weight float64) error {
	return s.LinkRel(from, to, RelTemporal, weight)
}

// LinkRel records an edge of the given relation, keeping the strongest
// weight the pair has been given under that relation.
func (s *Store) LinkRel(from, to uint64, rel string, weight float64) error {
	_, err := s.db.Exec(`INSERT INTO links (from_id, to_id, rel, weight) VALUES (?,?,?,?)
		ON CONFLICT(from_id, to_id, rel) DO UPDATE SET weight = max(weight, excluded.weight)`,
		from, to, rel, weight)
	return err
}

// SetLinkRel writes an edge's weight outright, down as well as up. A
// consolidation pass that recomputes a relation from counts owns its
// edges: practice and decay are overwritten by the fresh truth.
func (s *Store) SetLinkRel(from, to uint64, rel string, weight float64) error {
	_, err := s.db.Exec(`INSERT INTO links (from_id, to_id, rel, weight) VALUES (?,?,?,?)
		ON CONFLICT(from_id, to_id, rel) DO UPDATE SET weight = excluded.weight`,
		from, to, rel, weight)
	return err
}

// DeleteLinksFrom removes every edge of one relation leaving a fragment, the outright
// delete the upsert link API otherwise lacks. Re-rooting a code subgraph uses it to clear
// the stale coderoot before pointing the construction at the freshly materialized one.
func (s *Store) DeleteLinksFrom(from uint64, rel string) error {
	_, err := s.db.Exec(`DELETE FROM links WHERE from_id = ? AND rel = ?`, from, rel)
	return err
}

// SetLinkRelBatch upserts many edges with SetLinkRel's semantics in a single
// transaction over a prepared statement, so a consolidation pass that rewrites
// thousands of edges pays one transaction's overhead instead of one per edge. The
// store is single-connection, so the batch briefly holds the connection; call it
// only from the single-threaded nightly consolidation, not the live request path.
func (s *Store) SetLinkRelBatch(edges []LinkEdge) error {
	if len(edges) == 0 {
		return nil
	}
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	stmt, err := tx.Prepare(`INSERT INTO links (from_id, to_id, rel, weight) VALUES (?,?,?,?)
		ON CONFLICT(from_id, to_id, rel) DO UPDATE SET weight = excluded.weight`)
	if err != nil {
		_ = tx.Rollback()
		return err
	}
	for _, e := range edges {
		if _, err := stmt.Exec(e.From, e.To, e.Rel, e.Weight); err != nil {
			_ = stmt.Close()
			_ = tx.Rollback()
			return err
		}
	}
	_ = stmt.Close()
	return tx.Commit()
}

// LinksFrom returns every edge leaving the given fragments, whatever its
// relation.
func (s *Store) LinksFrom(ids []uint64) ([]LinkEdge, error) {
	return s.linksFromWhere(ids, "")
}

// LinksFromRel returns the edges of one relation leaving the given
// fragments.
func (s *Store) LinksFromRel(ids []uint64, rel string) ([]LinkEdge, error) {
	return s.linksFromWhere(ids, rel)
}

func (s *Store) linksFromWhere(ids []uint64, rel string) ([]LinkEdge, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	args := make([]any, len(ids))
	marks := make([]string, len(ids))
	for i, id := range ids {
		args[i] = id
		marks[i] = "?"
	}
	q := `SELECT from_id, to_id, rel, weight FROM links
		WHERE from_id IN (` + strings.Join(marks, ",") + `)`
	if rel != "" {
		q += ` AND rel = ?`
		args = append(args, rel)
	}
	rows, err := s.db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []LinkEdge
	for rows.Next() {
		var e LinkEdge
		if err := rows.Scan(&e.From, &e.To, &e.Rel, &e.Weight); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// StrengthenLinks is retrieval practice for edges: every traversed link
// gains a little weight, capped at 1.0, along with its reverse edge of the
// same relation when one exists. This is the counterpart of the nightly
// link decay. A directed relation has no reverse row, so practice cannot
// mint one.
func (s *Store) StrengthenLinks(edges []LinkEdge, delta float64) error {
	for _, e := range edges {
		if _, err := s.db.Exec(`UPDATE links SET weight = min(1.0, weight + ?)
			WHERE rel = ? AND ((from_id = ? AND to_id = ?) OR (from_id = ? AND to_id = ?))`,
			delta, e.Rel, e.From, e.To, e.To, e.From); err != nil {
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
			WHERE rel = ? AND ((from_id = ? AND to_id = ?) OR (from_id = ? AND to_id = ?))`,
			delta, e.Rel, e.From, e.To, e.To, e.From); err != nil {
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

// BumpSucc counts one more ordered occurrence of a template pair (a came
// first, b followed) and returns the running count. Order sensitivity is
// the point: it is what lets successor edges strengthen asymmetrically.
func (s *Store) BumpSucc(a, b string) (int, error) {
	var count int
	err := s.db.QueryRow(`INSERT INTO succ (tpl_a, tpl_b, count) VALUES (?,?,1)
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

// AllLinks returns the whole association graph, for export.
func (s *Store) AllLinks() ([]LinkEdge, error) {
	rows, err := s.db.Query(`SELECT from_id, to_id, rel, weight FROM links`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []LinkEdge
	for rows.Next() {
		var e LinkEdge
		if err := rows.Scan(&e.From, &e.To, &e.Rel, &e.Weight); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
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

// SetSourceKey binds a source name to its base64 public key, so a peer's
// identity is provable across restarts. Replacing an existing key rotates it.
func (s *Store) SetSourceKey(name, pubkey string) error {
	_, err := s.db.Exec(`INSERT INTO source_keys (name, pubkey) VALUES (?, ?)
		ON CONFLICT(name) DO UPDATE SET pubkey = excluded.pubkey`, name, pubkey)
	return err
}

// DeleteSourceKey removes a source's enrolled key, revoking its ability to
// prove that identity.
func (s *Store) DeleteSourceKey(name string) error {
	_, err := s.db.Exec(`DELETE FROM source_keys WHERE name = ?`, name)
	return err
}

// SourceKeys reads every enrolled source key, the map the daemon decodes at
// boot so a peer's signature can be checked against its own key.
func (s *Store) SourceKeys() (map[string]string, error) {
	rows, err := s.db.Query(`SELECT name, pubkey FROM source_keys`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]string{}
	for rows.Next() {
		var name, pubkey string
		if err := rows.Scan(&name, &pubkey); err != nil {
			return nil, err
		}
		out[name] = pubkey
	}
	return out, rows.Err()
}

// AuditByCue returns one cue's whole trail, oldest first, so a reader can
// fold its history forward: what was recorded, in the order it happened.
func (s *Store) AuditByCue(cue string) ([]AuditEntry, error) {
	rows, err := s.db.Query(`SELECT at, cue, action, outcome FROM audit
		WHERE cue = ? ORDER BY at ASC, rowid ASC`, cue)
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
