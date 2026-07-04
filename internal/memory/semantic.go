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

// FragmentByTemplate finds the newest fragment of any kind carrying a template,
// the lookup a computed fact is recalled through ("computed:add:6:7"). The
// template index serves it the way it serves the concept lookup.
func (s *Store) FragmentByTemplate(tpl string) (*Fragment, error) {
	frags, err := s.query(`SELECT `+fragmentCols+` FROM fragments
		WHERE template = ? ORDER BY id DESC LIMIT 1`, tpl)
	if err != nil || len(frags) == 0 {
		return nil, err
	}
	return frags[0], nil
}

// WordByTemplate returns the word node a template keys, or nil if none.
// A closed-class lexeme that names no concept (the copula, a modal) is held
// as a word node under a "lexeme:" template; this finds it by that template
// the way ConceptByTemplate finds a concept, current after a restore.
func (s *Store) WordByTemplate(tpl string) (*Fragment, error) {
	frags, err := s.query(`SELECT `+fragmentCols+` FROM fragments
		WHERE kind = ? AND template = ? LIMIT 1`, KindWord, tpl)
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

// Internal channels carry the mind rather than the world: speech is
// language about experience, thought is the inner loop's own activity.
// Neither shape ever becomes a concept (a question asked often enough
// would distil into a thing, a habitual thought into a fact), and
// neither counts as company when words are weighed.
const (
	utteranceChannel = "tell"
	thoughtChannel   = "thought"
	// categoryClass marks a grammatical function concept (a case or topic node), which
	// names nothing and is kept out of a word's naming company.
	categoryClass = "class"
)

// ConceptCandidates lists episodic shapes recurrent enough to deserve a
// concept and not yet distilled into one, most recurrent first.
// Utterances are excluded: only lived experience earns an abstraction.
func (s *Store) ConceptCandidates(minCount, limit int) ([]*Fragment, error) {
	if limit < 1 {
		return nil, nil
	}
	return s.query(`SELECT `+fragmentCols+` FROM fragments
		WHERE kind = ? AND count >= ? AND channel NOT IN (?, ?)
		AND template NOT IN (SELECT template FROM fragments WHERE kind = ?)
		ORDER BY count DESC, id DESC LIMIT ?`,
		KindEpisodic, minCount, utteranceChannel, thoughtChannel, KindConcept, limit)
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
		JOIN fragments e ON e.template = c.template AND e.kind = ? AND e.channel NOT IN (?, ?)
		WHERE c.kind = ?`,
		RelGrounds, weight, KindEpisodic, utteranceChannel, thoughtChannel, KindConcept)
	if err != nil {
		return 0, err
	}
	n, err := res.RowsAffected()
	return int(n), err
}

// Propose records one instance of a relation: the relation grounds in the
// moment that instantiates it, and the moment points at what filled each role,
// the agent at what acted and the patient at what it acted on. So "a dog
// chased a cat" grounds chase in the moment with the dog as agent and the cat
// as patient, the structure a proposition stands on. A zero role is left
// unbound, so an intransitive instance carries an agent and no patient.
func (s *Store) Propose(relation, moment, agent, patient uint64, weight float64) error {
	if err := s.LinkRel(relation, moment, RelGrounds, weight); err != nil {
		return err
	}
	if agent != 0 {
		if err := s.LinkRel(moment, agent, RelAgent, weight); err != nil {
			return err
		}
	}
	if patient != 0 {
		if err := s.LinkRel(moment, patient, RelPatient, weight); err != nil {
			return err
		}
	}
	return nil
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

// FeatureNode returns the node for one interpretable feature, minting
// it on first use. Features are shared across concepts: one node per
// attribute is what makes overlap a plain graph walk.
func (s *Store) FeatureNode(feature string, at time.Time) (*Fragment, error) {
	frags, err := s.query(`SELECT `+fragmentCols+` FROM fragments
		WHERE kind = ? AND template = ? LIMIT 1`, KindFeature, feature)
	if err != nil {
		return nil, err
	}
	if len(frags) > 0 {
		return frags[0], nil
	}
	f := &Fragment{
		Kind:     KindFeature,
		Channel:  "feature",
		Content:  feature,
		Template: feature,
		Time:     at,
	}
	if _, err := s.Encode(f); err != nil {
		return nil, err
	}
	return f, nil
}

// FeaturesByNames returns the feature nodes matching the given
// attribute strings, for overlap lookups at recall time.
func (s *Store) FeaturesByNames(names []string) ([]*Fragment, error) {
	if len(names) == 0 {
		return nil, nil
	}
	args := make([]any, 0, len(names)+1)
	marks := make([]string, len(names))
	args = append(args, KindFeature)
	for i, n := range names {
		marks[i] = "?"
		args = append(args, n)
	}
	return s.query(`SELECT `+fragmentCols+` FROM fragments
		WHERE kind = ? AND template IN (`+strings.Join(marks, ",")+`)`, args...)
}

// DropRelFrom removes every edge of one relation leaving the given
// fragments. The nightly feature pass owns its edges outright: each
// signature is rewritten whole, so a dissolved attribute disappears
// instead of fading.
func (s *Store) DropRelFrom(ids []uint64, rel string) error {
	if len(ids) == 0 {
		return nil
	}
	args := make([]any, 0, len(ids)+1)
	marks := make([]string, len(ids))
	for i, id := range ids {
		marks[i] = "?"
		args = append(args, id)
	}
	args = append(args, rel)
	_, err := s.db.Exec(`DELETE FROM links
		WHERE from_id IN (`+strings.Join(marks, ",")+`) AND rel = ?`, args...)
	return err
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
// graph. A word keeps company with a concept two ways: through a lived
// episode the concept grounds, and directly with a concept that was
// only held in mind when the word was said (attention-grounding, which a
// higher abstraction needs since it grounds in concepts, not episodes).
// Utterance episodes count for nothing on either side: a word always
// keeps company with its own saying, and that says nothing about the
// world.
func (s *Store) NamingPairs() ([]NamingPair, error) {
	// A word names a thing, never a grammatical class. A function node like fn:locative,
	// minted from a tongue's case morphology ("kediyi", "kediden"), keeps a noun's company
	// through every cased mention and, reached through the direct branch, would dominate
	// that noun's naming company and dilute its concentration on the percept it names below
	// the abstraction threshold, splitting the noun into a parallel abstraction. Class
	// concepts are excluded from naming company so a noun is named by the percepts and
	// nouns it co-occurs with, not the cases it inflects through.
	rows, err := s.db.Query(`
		WITH company AS (
			SELECT l1.from_id AS wid, c.id AS cid, l1.weight AS wgt
			FROM links l1
			JOIN fragments wd ON wd.id = l1.from_id AND wd.kind = ?
			JOIN fragments e ON e.id = l1.to_id AND e.kind = ? AND e.channel NOT IN (?, ?)
			JOIN links l2 ON l2.to_id = e.id AND l2.rel = ?
			JOIN fragments c ON c.id = l2.from_id AND c.kind = ? AND c.category <> ?
			WHERE l1.rel = ?
			UNION ALL
			SELECT l.from_id AS wid, l.to_id AS cid, l.weight AS wgt
			FROM links l
			JOIN fragments wd ON wd.id = l.from_id AND wd.kind = ?
			JOIN fragments c ON c.id = l.to_id AND c.kind = ? AND c.category <> ?
			WHERE l.rel = ?
		),
		wt AS (SELECT wid, SUM(wgt) AS total FROM company GROUP BY wid)
		SELECT company.wid, company.cid, SUM(company.wgt), wt.total,
		       COALESCE((SELECT weight FROM links
		                 WHERE from_id = company.wid AND to_id = company.cid AND rel = ?), 0)
		FROM company
		JOIN wt ON wt.wid = company.wid
		GROUP BY company.wid, company.cid`,
		KindWord, KindEpisodic, utteranceChannel, thoughtChannel, RelGrounds, KindConcept, categoryClass, RelTemporal,
		KindWord, KindConcept, categoryClass, RelTemporal,
		RelNames)
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

// OstensionPair is one word-concept pairing the abstraction pass weighs apart from
// the naming company: the ostensive salience a word accrued toward a concept, summed
// over the percept episodes the concept grounds. A word points at one percept when it
// names it, so a name concentrates its ostension on its referent while a category
// spreads it across the members it was said over.
type OstensionPair struct {
	Word    uint64
	Concept uint64
	Mass    float64
}

// OstensionPairs aggregates a word's ostensive salience onto the concepts the pointed-at
// percepts ground. A word keeps ostension with a concept through a lived episode the
// concept grounds, the freshly attended percept of the naming that earned the edge.
// Utterance and thought episodes count for nothing, the same exclusions naming makes,
// so a word's ostension is its company with what it was shown over, never with its own
// saying.
func (s *Store) OstensionPairs() ([]OstensionPair, error) {
	rows, err := s.db.Query(`
		SELECT l1.from_id AS wid, c.id AS cid, SUM(l1.weight)
		FROM links l1
		JOIN fragments wd ON wd.id = l1.from_id AND wd.kind = ?
		JOIN fragments e ON e.id = l1.to_id AND e.kind = ? AND e.channel NOT IN (?, ?)
		JOIN links l2 ON l2.to_id = e.id AND l2.rel = ?
		JOIN fragments c ON c.id = l2.from_id AND c.kind = ? AND c.category <> ?
		WHERE l1.rel = ?
		GROUP BY l1.from_id, c.id`,
		KindWord, KindEpisodic, utteranceChannel, thoughtChannel, RelGrounds, KindConcept, categoryClass, RelOstends)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []OstensionPair
	for rows.Next() {
		var p OstensionPair
		if err := rows.Scan(&p.Word, &p.Concept, &p.Mass); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// NamesMassByForm sums each word form's naming mass per language: the
// total weight of the names edges its word nodes carry, keyed
// "lang:form". The grammar competition reads it as the strength of
// lived usage standing against a seeded parameter.
func (s *Store) NamesMassByForm() (map[string]float64, error) {
	rows, err := s.db.Query(`SELECT w.lang, w.content, SUM(l.weight)
		FROM fragments w
		JOIN links l ON l.from_id = w.id AND l.rel = ?
		WHERE w.kind = ?
		GROUP BY w.lang, w.content`, RelNames, KindWord)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make(map[string]float64)
	for rows.Next() {
		var lang, form string
		var mass float64
		if err := rows.Scan(&lang, &form, &mass); err != nil {
			return nil, err
		}
		out[lang+":"+form] = mass
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
