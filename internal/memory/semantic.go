// The semantic half of the store: word and concept nodes and the typed
// edges that bind them to experience, persisted beside the moments they
// abstract.

package memory

import (
	"strings"
	"time"

	"golem/internal/limbic"
)

// Concepts returns every concept node, oldest first.
func (s *Store) Concepts() ([]*Fragment, error) {
	return s.query(`SELECT `+fragmentCols+` FROM fragments WHERE kind = ? ORDER BY id`, KindConcept)
}

// ConceptByTemplate returns the concept that distils a template, or nil
// if none has been born.
func (s *Store) ConceptByTemplate(tpl string) (*Fragment, error) {
	frags, err := s.query(`SELECT `+fragmentCols+` FROM fragments
		WHERE kind = ? AND template = ? LIMIT 1`, KindConcept, tpl)
	if err != nil || len(frags) == 0 {
		return nil, err
	}
	return frags[0], nil
}

// EpisodesByExactShape returns the moments whose template or content is
// exactly the phrase, newest first. An exact shape is the strongest
// possible anchor a phrase can have.
func (s *Store) EpisodesByExactShape(phrase string, limit int) ([]*Fragment, error) {
	q := strings.ToLower(phrase)
	if q == "" {
		return nil, nil
	}
	return s.query(`SELECT `+fragmentCols+` FROM fragments
		WHERE kind = ? AND (lower(template) = ? OR lower(content) = ?)
		ORDER BY id DESC LIMIT ?`, KindEpisodic, q, q, limit)
}

// ConceptsByGloss returns concepts whose gloss or source template contains
// the phrase. This is how a spoken phrase reaches a concept no word names
// yet: by content, the way episodic recall already works.
func (s *Store) ConceptsByGloss(phrase string, limit int) ([]*Fragment, error) {
	q := strings.ToLower(phrase)
	if q == "" {
		return nil, nil
	}
	return s.query(`SELECT `+fragmentCols+` FROM fragments
		WHERE kind = ? AND (instr(lower(content), ?) > 0 OR instr(lower(template), ?) > 0)
		ORDER BY id LIMIT ?`, KindConcept, q, q, limit)
}

// utteranceChannel is speech: language about the world rather than the
// world itself. An utterance's shape mints words that name concepts and
// never becomes a concept, or a question asked often enough would
// distil into a thing.
const utteranceChannel = "tell"

// ConceptCandidates lists episodic shapes recurrent enough to deserve a
// concept and not yet distilled into one, most recurrent first.
// Utterances are excluded: only lived experience earns an abstraction.
func (s *Store) ConceptCandidates(minCount, limit int) ([]*Fragment, error) {
	if limit < 1 {
		return nil, nil
	}
	return s.query(`SELECT `+fragmentCols+` FROM fragments
		WHERE kind = ? AND count >= ? AND channel <> ?
		AND template NOT IN (SELECT template FROM fragments WHERE kind = ?)
		ORDER BY count DESC, id DESC LIMIT ?`,
		KindEpisodic, minCount, utteranceChannel, KindConcept, limit)
}

// Reground writes the missing grounds edges from every concept to the
// episodic fragments matching its template, and reports how many were
// added. It serves both newborn concepts (their first grounding) and old
// ones whose shape has recurred since the last night. Utterances never
// instantiate a concept, so they take no grounds edges.
func (s *Store) Reground(weight float64) (int, error) {
	res, err := s.db.Exec(`INSERT OR IGNORE INTO links (from_id, to_id, rel, weight)
		SELECT c.id, e.id, ?, ?
		FROM fragments c
		JOIN fragments e ON e.template = c.template AND e.kind = ? AND e.channel <> ?
		WHERE c.kind = ?`,
		RelGrounds, weight, KindEpisodic, utteranceChannel, KindConcept)
	if err != nil {
		return 0, err
	}
	n, err := res.RowsAffected()
	return int(n), err
}

// WordNode returns the node for a surface form in a language, minting it
// on first use. Reuse is practice: the count grows and the weight
// refreshes.
func (s *Store) WordNode(form, lang string, chem limbic.State, at time.Time) (*Fragment, bool, error) {
	frags, err := s.query(`SELECT `+fragmentCols+` FROM fragments
		WHERE kind = ? AND content = ? AND lang = ? LIMIT 1`, KindWord, form, lang)
	if err != nil {
		return nil, false, err
	}
	if len(frags) > 0 {
		f := frags[0]
		f.Count++
		f.Weight = 1.0
		if _, err := s.db.Exec(`UPDATE fragments SET count = ?, weight = 1.0 WHERE id = ?`,
			f.Count, f.ID); err != nil {
			return nil, false, err
		}
		return f, false, nil
	}
	f := &Fragment{
		Kind:    KindWord,
		Channel: "word",
		Content: form,
		Lang:    lang,
		Limbic:  chem,
		Time:    at,
	}
	f.Template = form
	if _, err := s.Encode(f); err != nil {
		return nil, false, err
	}
	return f, true, nil
}

// WordNodesByForms returns the word nodes matching any of the given
// surface forms, in every language at once: which language a form came
// from does not matter when the goal is the concept behind it.
func (s *Store) WordNodesByForms(forms []string) ([]*Fragment, error) {
	if len(forms) == 0 {
		return nil, nil
	}
	args := make([]any, 0, len(forms)+1)
	marks := make([]string, len(forms))
	args = append(args, KindWord)
	for i, w := range forms {
		marks[i] = "?"
		args = append(args, strings.ToLower(w))
	}
	return s.query(`SELECT `+fragmentCols+` FROM fragments
		WHERE kind = ? AND lower(content) IN (`+strings.Join(marks, ",")+`)`, args...)
}

// LinksToRel returns the edges of one relation arriving at the given
// fragments.
func (s *Store) LinksToRel(ids []uint64, rel string) ([]LinkEdge, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	args := make([]any, 0, len(ids)+1)
	marks := make([]string, len(ids))
	for i, id := range ids {
		marks[i] = "?"
		args = append(args, id)
	}
	args = append(args, rel)
	rows, err := s.db.Query(`SELECT from_id, to_id, rel, weight FROM links
		WHERE to_id IN (`+strings.Join(marks, ",")+`) AND rel = ?`, args...)
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

// NamingPair is one word-concept pairing the naming pass should weigh:
// the co-occurrence mass between the word and the concept's grounded
// episodes, the word's total episodic company for the denominator, and
// the bond's current strength (zero when new). Mass survives the merge
// pass because absorbed edges fold onto prototypes with summed weights.
type NamingPair struct {
	Word      uint64
	Concept   uint64
	Cooc      float64 // co-occurrence mass with this concept's episodes
	WordTotal float64 // the word's mass across all lived episodes
	Existing  float64
}

// NamingPairs aggregates word-to-concept co-occurrence over the whole
// graph. Utterance episodes count for nothing on either side: a word
// always keeps company with its own saying, and that company says
// nothing about the world.
func (s *Store) NamingPairs() ([]NamingPair, error) {
	rows, err := s.db.Query(`
		WITH wt AS (
			SELECT l.from_id AS wid, SUM(l.weight) AS total
			FROM links l
			JOIN fragments w2 ON w2.id = l.from_id AND w2.kind = ?
			JOIN fragments e2 ON e2.id = l.to_id AND e2.kind = ? AND e2.channel <> ?
			WHERE l.rel = ?
			GROUP BY l.from_id
		)
		SELECT w.id, c.id, SUM(l1.weight), wt.total,
		       COALESCE((SELECT weight FROM links
		                 WHERE from_id = w.id AND to_id = c.id AND rel = ?), 0)
		FROM fragments w
		JOIN links l1 ON l1.from_id = w.id AND l1.rel = ?
		JOIN fragments e ON e.id = l1.to_id AND e.kind = ? AND e.channel <> ?
		JOIN links l2 ON l2.to_id = e.id AND l2.rel = ?
		JOIN fragments c ON c.id = l2.from_id AND c.kind = ?
		JOIN wt ON wt.wid = w.id
		WHERE w.kind = ?
		GROUP BY w.id, c.id`,
		KindWord, KindEpisodic, utteranceChannel, RelTemporal,
		RelNames, RelTemporal, KindEpisodic, utteranceChannel,
		RelGrounds, KindConcept, KindWord)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []NamingPair
	for rows.Next() {
		var p NamingPair
		if err := rows.Scan(&p.Word, &p.Concept, &p.Cooc, &p.WordTotal, &p.Existing); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// GroundingTally returns, per concept, the total occurrences its grounded
// episodes stand for, prototype counts included.
func (s *Store) GroundingTally() (map[uint64]int, error) {
	rows, err := s.db.Query(`SELECT l.from_id, SUM(e.count)
		FROM links l JOIN fragments e ON e.id = l.to_id
		WHERE l.rel = ? GROUP BY l.from_id`, RelGrounds)
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
