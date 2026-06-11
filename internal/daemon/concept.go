package daemon

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"golem/internal/events"
	"golem/internal/grammar"
	"golem/internal/memory"
	"golem/internal/senses"
	"golem/internal/wire"
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
	// Naming is cross-situational: a bond's strength is the reliability
	// of the pairing, not the count of meetings. The smoothing mass is
	// about one co-occurrence, so a single meeting reads as half
	// evidence; the gain scales reliability into link-weight range with
	// dopamine deciding how readily words bond at all; bonds that would
	// start below the write floor are not worth an edge.
	namesSmoothing    = 0.3
	namesGainBase     = 0.6
	namesDopamineGain = 0.75
	namesMaxWeight    = 0.95
	namesWriteFloor   = 0.1
	// A name well below the concept's firmest is a weak pointer, spoken
	// tentatively. The gap reads the reliability spread, so it tracks
	// how much less faithfully a word kept the concept's company, not
	// the shared dopamine gain that lifts every name alike.
	namesContrastGap = 0.12
)

// lemmaCategory derives a concept's syntactic category from its grounding
// role: a shape in command position is verb-like, a recurring sensory
// object is noun-like.
func lemmaCategory(proto *memory.Fragment) string {
	if proto.Channel == "shell" && strings.HasPrefix(proto.Content, senses.ActionPrefix) {
		return "verb"
	}
	return "noun"
}

// glossFor is the speakable face of a concept: its source template, with
// the command marker stripped from shapes that live in command position.
func glossFor(proto *memory.Fragment) string {
	if proto.Channel == "shell" && strings.HasPrefix(proto.Template, senses.ActionPrefix) {
		return strings.TrimSpace(strings.TrimPrefix(proto.Template, senses.ActionPrefix))
	}
	return proto.Template
}

// growConcepts is concept genesis, run during sleep: shapes that have
// recurred into prototypes are distilled into concept nodes, every
// concept is grounded to the episodes matching its shape, and words bond
// to the concepts their episodes ground. Serotonin decides how many
// concepts one night may mint; dopamine decides how eagerly words bond.
func (d *Daemon) growConcepts() (born, named int) {
	budget := int(d.chem.Serotonin() * conceptBudgetScale)
	candidates, err := d.store.ConceptCandidates(memory.MergeThreshold, budget)
	if err != nil {
		fmt.Fprintf(d.out, "concept genesis error: %v\n", err)
		return 0, 0
	}
	for _, proto := range candidates {
		c := &memory.Fragment{
			Kind:     memory.KindConcept,
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
		d.event(events.Event{Ev: "concept", ID: c.ID, Tpl: wire.Clip(c.Template, 120), Outcome: c.Category})
	}

	if _, err := d.store.Reground(groundsWeight); err != nil {
		fmt.Fprintf(d.out, "reground error: %v\n", err)
	}

	pairs, err := d.store.NamingPairs()
	if err != nil {
		fmt.Fprintf(d.out, "naming error: %v\n", err)
		return born, 0
	}
	conceptTotal := map[uint64]float64{}
	for _, p := range pairs {
		conceptTotal[p.Concept] += p.Cooc
	}
	gain := namesGainBase + namesDopamineGain*d.chem.Snapshot().Dopamine
	for _, p := range pairs {
		// Cross-situational reliability: the share of the word's company
		// that lands on this concept, blended with the share of the
		// concept's company that is this word. A promiscuous word splits
		// its mass across its targets and bonds weakly to each; a word
		// paired with one concept nearly every time keeps almost all of
		// its mass there. The bond is set from the fresh ratio, down as
		// well as up, so a word that drifts to new company loosens its
		// old bonds at the next consolidation.
		pcw := p.Cooc / (p.WordTotal + namesSmoothing)
		pwc := p.Cooc / (conceptTotal[p.Concept] + namesSmoothing)
		w := math.Sqrt(pcw*pwc) * gain
		if w > namesMaxWeight {
			w = namesMaxWeight
		}
		if p.Existing == 0 && w < namesWriteFloor {
			continue
		}
		if err := d.store.SetLinkRel(p.Word, p.Concept, memory.RelNames, w); err != nil {
			fmt.Fprintf(d.out, "naming link error: %v\n", err)
			continue
		}
		if p.Existing == 0 {
			named++
			d.event(events.Event{Ev: "name", From: p.Word, To: p.Concept, Rel: memory.RelNames, Weight: w})
		}
	}
	return born, named
}

// mintWords stores or refreshes word nodes for the content words of an
// utterance and binds each to the tell moment and to the moments the
// utterance bound to, so the night's naming pass can see which
// experiences the words kept company with.
func (d *Daemon) mintWords(utterance, lang string, tellID uint64, anchors []recentMoment) []*memory.Fragment {
	if lang == "" {
		lang = "en"
	}
	forms := grammar.ContentTokens(utterance)
	if len(forms) == 0 {
		return nil
	}
	da := d.chem.Snapshot().Dopamine
	w := linkBaseWeight + linkDopamineGain*da
	var words []*memory.Fragment
	for _, form := range forms {
		word, minted, err := d.store.WordNode(form, lang, d.chem.Snapshot(), time.Now())
		if err != nil {
			fmt.Fprintf(d.out, "word mint error: %v\n", err)
			continue
		}
		if minted {
			d.event(events.Event{Ev: "word", ID: word.ID, Channel: lang, Tpl: form})
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
func (d *Daemon) conceptInfos() ([]*wire.ConceptInfo, error) {
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
	names, err := d.store.LinksToRel(ids, memory.RelNames)
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
	wordByID := make(map[uint64]*memory.Fragment, len(words))
	for _, w := range words {
		wordByID[w.ID] = w
	}
	namesOf := make(map[uint64][]string)
	for _, e := range names {
		if w := wordByID[e.From]; w != nil {
			namesOf[e.To] = append(namesOf[e.To], fmt.Sprintf("%s(%s)", w.Content, w.Lang))
		}
	}
	infos := make([]*wire.ConceptInfo, 0, len(concepts))
	for _, c := range concepts {
		sort.Strings(namesOf[c.ID])
		infos = append(infos, &wire.ConceptInfo{
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
func (d *Daemon) warmInfos(n int) []*wire.WarmInfo {
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
	out := make([]*wire.WarmInfo, 0, len(entries))
	for _, e := range entries {
		if g, ok := gloss[e.id]; ok {
			out = append(out, &wire.WarmInfo{Gloss: g, Heat: e.heat})
		}
	}
	return out
}

// warmConcepts excites the concepts the given words name: speaking about
// something is what keeps it in working memory.
func (d *Daemon) warmConcepts(words []*memory.Fragment, amount float64) {
	if len(words) == 0 {
		return
	}
	ids := make([]uint64, 0, len(words))
	for _, w := range words {
		ids = append(ids, w.ID)
	}
	names, err := d.store.LinksFromRel(ids, memory.RelNames)
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
func (d *Daemon) semanticSeeds(query string) ([]*memory.Fragment, []*wire.FragmentInfo) {
	tokens, _ := grammar.Tokenize(query)
	words, err := d.store.WordNodesByForms(tokens)
	if err != nil {
		fmt.Fprintf(d.out, "semantic recall error: %v\n", err)
		return nil, nil
	}

	var walked []memory.LinkEdge
	conceptIDs := map[uint64]bool{}
	wordIDs := make([]uint64, 0, len(words))
	for _, w := range words {
		wordIDs = append(wordIDs, w.ID)
	}
	names, err := d.store.LinksFromRel(wordIDs, memory.RelNames)
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
	grounds, err := d.store.LinksFromRel(cids, memory.RelGrounds)
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

	var mediators []*wire.FragmentInfo
	for _, w := range words {
		info := wire.NewFragmentInfo(w)
		info.Activation = 0.9
		mediators = append(mediators, info)
	}
	if concepts, err := d.store.FragmentsByIDs(cids); err == nil {
		for _, c := range concepts {
			info := wire.NewFragmentInfo(c)
			info.Activation = 0.9
			mediators = append(mediators, info)
		}
	}
	return episodes, mediators
}
