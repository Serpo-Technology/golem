package main

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

// The concept layer is GOLEM's semantic hub: an amodal node between the
// word and the experience, so meaning accrues per concept across episodes
// and across languages. Words point at concepts through names edges;
// concepts point at the episodes that instantiate them through grounds
// edges. Many words may name one concept, which is all the machinery
// cross-lingual synonymy needs.
//
// Concepts are born during consolidation, the same pass that merges
// recurring shapes into prototypes: a shape that has recurred enough to
// earn a prototype has earned an abstraction. Serotonin paces how many
// may be born in one night, and dopamine gates how readily a word bonds
// to a concept the next time naming runs.
const (
	conceptBudgetScale = 16 // concepts mintable in one night at full serotonin
	groundsWeight      = 0.8
	namesBaseWeight    = 0.25
	namesDopamineGain  = 0.5
	namesRepeatBonus   = 0.08 // per co-occurring grounded episode beyond the first
	namesMaxWeight     = 0.95
)

// lemmaCategory derives a concept's syntactic category from its grounding
// role: a shape in command position is verb-like, a recurring sensory
// object is noun-like.
func lemmaCategory(proto *Fragment) string {
	if proto.Channel == "shell" && strings.HasPrefix(proto.Content, actionPrefix) {
		return "verb"
	}
	return "noun"
}

// glossFor is the speakable face of a concept: its source template, with
// the command marker stripped from shapes that live in command position.
func glossFor(proto *Fragment) string {
	if proto.Channel == "shell" && strings.HasPrefix(proto.Template, actionPrefix) {
		return strings.TrimSpace(strings.TrimPrefix(proto.Template, actionPrefix))
	}
	return proto.Template
}

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

// ConceptCandidates lists episodic shapes recurrent enough to deserve a
// concept and not yet distilled into one, most recurrent first.
func (s *Store) ConceptCandidates(minCount, limit int) ([]*Fragment, error) {
	if limit < 1 {
		return nil, nil
	}
	return s.query(`SELECT `+fragmentCols+` FROM fragments
		WHERE kind = ? AND count >= ?
		AND template NOT IN (SELECT template FROM fragments WHERE kind = ?)
		ORDER BY count DESC, id DESC LIMIT ?`,
		KindEpisodic, minCount, KindConcept, limit)
}

// Reground writes the missing grounds edges from every concept to the
// episodic fragments matching its template, and reports how many were
// added. It serves both newborn concepts (their first grounding) and old
// ones whose shape has recurred since the last night.
func (s *Store) Reground(weight float64) (int, error) {
	res, err := s.db.Exec(`INSERT OR IGNORE INTO links (from_id, to_id, rel, weight)
		SELECT c.id, e.id, ?, ?
		FROM fragments c
		JOIN fragments e ON e.template = c.template AND e.kind = ?
		WHERE c.kind = ?`,
		RelGrounds, weight, KindEpisodic, KindConcept)
	if err != nil {
		return 0, err
	}
	n, err := res.RowsAffected()
	return int(n), err
}

// WordNode returns the node for a surface form in a language, minting it
// on first use. Reuse is practice: the count grows and the weight
// refreshes.
func (s *Store) WordNode(form, lang string, limbic LimbicState, at time.Time) (*Fragment, bool, error) {
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
		Limbic:  limbic,
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

// NamingPair is one word-concept bond the naming pass should consider:
// the word has kept company with this many episodes that ground the
// concept, and Existing is the bond's current strength (zero when new).
type NamingPair struct {
	Word     uint64
	Concept  uint64
	Episodes int
	Existing float64
}

// NamingPairs aggregates word-to-concept co-occurrence over the whole
// graph: for every word node, the concepts grounding the episodes the
// word has bound to.
func (s *Store) NamingPairs() ([]NamingPair, error) {
	rows, err := s.db.Query(`
		SELECT w.id, c.id, COUNT(DISTINCT l1.to_id),
		       COALESCE((SELECT weight FROM links
		                 WHERE from_id = w.id AND to_id = c.id AND rel = ?), 0)
		FROM fragments w
		JOIN links l1 ON l1.from_id = w.id AND l1.rel = ?
		JOIN links l2 ON l2.to_id = l1.to_id AND l2.rel = ?
		JOIN fragments c ON c.id = l2.from_id AND c.kind = ?
		WHERE w.kind = ?
		GROUP BY w.id, c.id`,
		RelNames, RelTemporal, RelGrounds, KindConcept, KindWord)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []NamingPair
	for rows.Next() {
		var p NamingPair
		if err := rows.Scan(&p.Word, &p.Concept, &p.Episodes, &p.Existing); err != nil {
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

// growConcepts is concept genesis, run during sleep: shapes that have
// recurred into prototypes are distilled into concept nodes, every
// concept is grounded to the episodes matching its shape, and words bond
// to the concepts their episodes ground. Serotonin decides how many
// concepts one night may mint; dopamine decides how eagerly words bond.
func (d *Daemon) growConcepts() (born, named int) {
	budget := int(d.chem.Serotonin() * conceptBudgetScale)
	candidates, err := d.store.ConceptCandidates(mergeThreshold, budget)
	if err != nil {
		fmt.Fprintf(d.out, "concept genesis error: %v\n", err)
		return 0, 0
	}
	for _, proto := range candidates {
		c := &Fragment{
			Kind:     KindConcept,
			Channel:  "concept",
			Content:  glossFor(proto),
			Template: proto.Template,
			Category: lemmaCategory(proto),
			Limbic:   d.chem.Snapshot(),
			Time:     time.Now(),
		}
		if _, err := d.store.Encode(c); err != nil {
			fmt.Fprintf(d.out, "concept mint error: %v\n", err)
			continue
		}
		born++
		d.event(Event{Ev: "concept", ID: c.ID, Tpl: clip(c.Template, 120), Outcome: c.Category})
	}

	if _, err := d.store.Reground(groundsWeight); err != nil {
		fmt.Fprintf(d.out, "reground error: %v\n", err)
	}

	pairs, err := d.store.NamingPairs()
	if err != nil {
		fmt.Fprintf(d.out, "naming error: %v\n", err)
		return born, 0
	}
	da := d.chem.Snapshot().Dopamine
	for _, p := range pairs {
		w := namesBaseWeight + namesDopamineGain*da + namesRepeatBonus*float64(p.Episodes-1)
		if w > namesMaxWeight {
			w = namesMaxWeight
		}
		if w <= p.Existing {
			continue
		}
		if err := d.store.LinkRel(p.Word, p.Concept, RelNames, w); err != nil {
			fmt.Fprintf(d.out, "naming link error: %v\n", err)
			continue
		}
		if p.Existing == 0 {
			named++
			d.event(Event{Ev: "name", From: p.Word, To: p.Concept, Rel: RelNames, Weight: w})
		}
	}
	return born, named
}

// mintWords stores or refreshes word nodes for the content words of an
// utterance and binds each to the tell moment and to the moments the
// utterance bound to, so the night's naming pass can see which
// experiences the words kept company with.
func (d *Daemon) mintWords(utterance, lang string, tellID uint64, anchors []recentMoment) []*Fragment {
	if lang == "" {
		lang = "en"
	}
	forms := contentTokens(utterance)
	if len(forms) == 0 {
		return nil
	}
	da := d.chem.Snapshot().Dopamine
	w := linkBaseWeight + linkDopamineGain*da
	var words []*Fragment
	for _, form := range forms {
		word, minted, err := d.store.WordNode(form, lang, d.chem.Snapshot(), time.Now())
		if err != nil {
			fmt.Fprintf(d.out, "word mint error: %v\n", err)
			continue
		}
		if minted {
			d.event(Event{Ev: "word", ID: word.ID, Channel: lang, Tpl: form})
		}
		targets := make([]uint64, 0, len(anchors)+1)
		if tellID != 0 {
			targets = append(targets, tellID)
		}
		for _, a := range anchors {
			if a.id != word.ID {
				targets = append(targets, a.id)
			}
		}
		for _, id := range targets {
			if err := d.store.Link(word.ID, id, w); err != nil {
				fmt.Fprintf(d.out, "word link error: %v\n", err)
				continue
			}
			d.store.Link(id, word.ID, w)
		}
		words = append(words, word)
	}
	return words
}

// conceptInfos assembles the wire view of the concept layer: every
// concept with the words that name it, the experience that grounds it,
// and its workspace heat.
func (d *Daemon) conceptInfos() ([]*ConceptInfo, error) {
	concepts, err := d.store.Concepts()
	if err != nil {
		return nil, err
	}
	tally, err := d.store.GroundingTally()
	if err != nil {
		return nil, err
	}
	ids := make([]uint64, 0, len(concepts))
	for _, c := range concepts {
		ids = append(ids, c.ID)
	}
	names, err := d.store.LinksToRel(ids, RelNames)
	if err != nil {
		return nil, err
	}
	wordIDs := make([]uint64, 0, len(names))
	for _, e := range names {
		wordIDs = append(wordIDs, e.From)
	}
	words, err := d.store.FragmentsByIDs(wordIDs)
	if err != nil {
		return nil, err
	}
	wordByID := make(map[uint64]*Fragment, len(words))
	for _, w := range words {
		wordByID[w.ID] = w
	}
	namesOf := make(map[uint64][]string)
	for _, e := range names {
		if w := wordByID[e.From]; w != nil {
			namesOf[e.To] = append(namesOf[e.To], fmt.Sprintf("%s(%s)", w.Content, w.Lang))
		}
	}
	infos := make([]*ConceptInfo, 0, len(concepts))
	for _, c := range concepts {
		sort.Strings(namesOf[c.ID])
		infos = append(infos, &ConceptInfo{
			ID:       c.ID,
			Gloss:    c.Content,
			Category: c.Category,
			Names:    namesOf[c.ID],
			Grounded: tally[c.ID],
			Heat:     d.ws.heat(c.ID),
		})
	}
	return infos, nil
}

// warmInfos resolves the workspace's hottest concepts into glosses, for
// status display.
func (d *Daemon) warmInfos(n int) []*WarmInfo {
	entries := d.ws.warmest(n)
	if len(entries) == 0 {
		return nil
	}
	ids := make([]uint64, 0, len(entries))
	for _, e := range entries {
		ids = append(ids, e.id)
	}
	frags, err := d.store.FragmentsByIDs(ids)
	if err != nil {
		return nil
	}
	gloss := make(map[uint64]string, len(frags))
	for _, f := range frags {
		gloss[f.ID] = f.Content
	}
	out := make([]*WarmInfo, 0, len(entries))
	for _, e := range entries {
		if g, ok := gloss[e.id]; ok {
			out = append(out, &WarmInfo{Gloss: g, Heat: e.heat})
		}
	}
	return out
}

// warmConcepts excites the concepts the given words name: speaking about
// something is what keeps it in working memory.
func (d *Daemon) warmConcepts(words []*Fragment, amount float64) {
	if len(words) == 0 {
		return
	}
	ids := make([]uint64, 0, len(words))
	for _, w := range words {
		ids = append(ids, w.ID)
	}
	names, err := d.store.LinksFromRel(ids, RelNames)
	if err != nil {
		return
	}
	for _, e := range names {
		d.ws.excite(e.To, amount)
	}
}

// semanticSeeds reads a query through the lexicon: its words name
// concepts, the concepts ground episodes, and those episodes join the
// recall as seeds even when they share no substring with the query. The
// walked names and grounds edges gain practice like any recalled path.
// The mediating word and concept nodes come back too, so a reply can
// show the path meaning took.
func (d *Daemon) semanticSeeds(query string) ([]*Fragment, []*FragmentInfo) {
	tokens, _ := tokenize(query)
	words, err := d.store.WordNodesByForms(tokens)
	if err != nil {
		fmt.Fprintf(d.out, "semantic recall error: %v\n", err)
		return nil, nil
	}

	var walked []LinkEdge
	conceptIDs := map[uint64]bool{}
	wordIDs := make([]uint64, 0, len(words))
	for _, w := range words {
		wordIDs = append(wordIDs, w.ID)
	}
	names, err := d.store.LinksFromRel(wordIDs, RelNames)
	if err != nil {
		fmt.Fprintf(d.out, "semantic recall error: %v\n", err)
		return nil, nil
	}
	for _, e := range names {
		conceptIDs[e.To] = true
		walked = append(walked, e)
	}
	// A phrase no word names yet can still reach a concept by content,
	// the same fallback episodic recall has always used.
	if glossed, err := d.store.ConceptsByGloss(query, 3); err == nil {
		for _, c := range glossed {
			conceptIDs[c.ID] = true
		}
	}
	if len(conceptIDs) == 0 {
		return nil, nil
	}

	cids := make([]uint64, 0, len(conceptIDs))
	for id := range conceptIDs {
		cids = append(cids, id)
	}
	grounds, err := d.store.LinksFromRel(cids, RelGrounds)
	if err != nil {
		fmt.Fprintf(d.out, "semantic recall error: %v\n", err)
		return nil, nil
	}
	episodeIDs := make([]uint64, 0, len(grounds))
	for _, e := range grounds {
		episodeIDs = append(episodeIDs, e.To)
		walked = append(walked, e)
	}
	episodes, err := d.fragmentsByIDsTiered(episodeIDs)
	if err != nil {
		fmt.Fprintf(d.out, "semantic recall error: %v\n", err)
		return nil, nil
	}
	// Reaching an episode through meaning is retrieval practice for the
	// episode and for every edge the meaning walked, and it warms the
	// concepts it walked through.
	if err := d.store.Refresh(episodeIDs); err == nil {
		for _, f := range episodes {
			f.Weight = 1.0
		}
	}
	if len(walked) > 0 {
		d.store.StrengthenLinks(walked, linkPracticeDelta)
	}
	for id := range conceptIDs {
		d.ws.excite(id, exciteRecall)
	}

	var mediators []*FragmentInfo
	for _, w := range words {
		info := fragmentInfo(w)
		info.Activation = 0.9
		mediators = append(mediators, info)
	}
	if concepts, err := d.store.FragmentsByIDs(cids); err == nil {
		for _, c := range concepts {
			info := fragmentInfo(c)
			info.Activation = 0.9
			mediators = append(mediators, info)
		}
	}
	return episodes, mediators
}
