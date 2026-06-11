package main

import (
	"sort"
	"strings"
)

// The message planner sits above the grammar and decides what to say: it
// answers a question from what is grounded, admits ignorance where
// nothing is, asks back when curiosity is high, and turns command force
// into a staged proposal under the same gates every proposal faces.
// Speech never executes anything by itself.
const (
	curiosityGate = 0.5 // dopamine above this makes GOLEM ask about unknowns
	// One co-occurrence is an anecdote. A single fresh successor edge
	// weighs successorBaseWeight, so the floor sits above it: claiming
	// "usually follows" takes either repetition or a practiced edge.
	successorFloor = 0.3
	// spokenPhraseCap keeps a raw one-off line from swallowing the
	// sentence built around it; concepts and templates stay short on
	// their own.
	spokenPhraseCap = 100
)

// resolveConcept maps a surface phrase to its strongest concept: first
// through words that name one, then by gloss content, the same fallback
// recall uses for phrases nobody taught yet.
func (d *Daemon) resolveConcept(phrase string) *Fragment {
	tokens, _ := tokenize(phrase)
	words, err := d.store.WordNodesByForms(tokens)
	if err == nil && len(words) > 0 {
		ids := make([]uint64, 0, len(words))
		for _, w := range words {
			ids = append(ids, w.ID)
		}
		names, err := d.store.LinksFromRel(ids, RelNames)
		if err == nil && len(names) > 0 {
			votes := map[uint64]float64{}
			for _, e := range names {
				votes[e.To] += e.Weight
			}
			var best uint64
			bestW := 0.0
			for id, w := range votes {
				if w > bestW || (w == bestW && id > best) {
					best, bestW = id, w
				}
			}
			if frags, err := d.store.FragmentsByIDs([]uint64{best}); err == nil && len(frags) == 1 {
				return frags[0]
			}
		}
	}
	if concepts, err := d.store.ConceptsByGloss(phrase, 1); err == nil && len(concepts) > 0 {
		return concepts[0]
	}
	return nil
}

// namingContrast measures how firmly a phrase names a concept against the
// concept's firmest name. The repeat bonus is the only part of a naming
// weight that varies per word; the dopamine that lifts every name the same
// cancels in the gap, so a large gap means this word kept the concept's
// company far less often than its best-established name and is a weak
// pointer, not a settled one.
func (d *Daemon) namingContrast(phrase string, conceptID uint64) (asked, top float64) {
	if into, err := d.store.LinksToRel([]uint64{conceptID}, RelNames); err == nil {
		for _, e := range into {
			if e.Weight > top {
				top = e.Weight
			}
		}
	}
	tokens, _ := tokenize(phrase)
	words, err := d.store.WordNodesByForms(tokens)
	if err != nil || len(words) == 0 {
		return 0, top
	}
	ids := make([]uint64, 0, len(words))
	for _, w := range words {
		ids = append(ids, w.ID)
	}
	names, err := d.store.LinksFromRel(ids, RelNames)
	if err != nil {
		return 0, top
	}
	for _, e := range names {
		if e.To == conceptID && e.Weight > asked {
			asked = e.Weight
		}
	}
	return asked, top
}

// anchorEpisodes resolves a term to the moments it stands for and to the
// face the answer should speak for them. An exact shape match outranks
// the concept the term's words voted for: greedy naming can point a
// word at a bystander concept, and an answer must anchor on the event
// the question named, never on what its words happen to be bonded to.
// Utterances never anchor at all, or asking "what follows make test"
// would anchor on the asking.
func (d *Daemon) anchorEpisodes(t Term) ([]uint64, Term) {
	if t.Phrase != "" {
		exact, err := d.store.EpisodesByExactShape(t.Phrase, 8)
		if err == nil {
			var ids []uint64
			for _, f := range exact {
				if f.Channel != "tell" {
					ids = append(ids, f.ID)
				}
			}
			if len(ids) > 0 {
				return ids, Term{Phrase: t.Phrase}
			}
		}
	}
	if t.Concept != nil {
		grounds, err := d.store.LinksFromRel([]uint64{t.Concept.ID}, RelGrounds)
		if err == nil && len(grounds) > 0 {
			ids := make([]uint64, 0, len(grounds))
			for _, e := range grounds {
				ids = append(ids, e.To)
			}
			return ids, t
		}
	}
	if t.Phrase == "" {
		return nil, t
	}
	frags, err := d.store.Recall(t.Phrase, 8)
	if err != nil {
		return nil, t
	}
	var ids []uint64
	for _, f := range frags {
		if f.Channel == "tell" {
			continue
		}
		ids = append(ids, f.ID)
	}
	if len(ids) > 0 {
		if anchored, err := d.store.FragmentsByIDs(ids[:1]); err == nil && len(anchored) == 1 {
			return ids, Term{Phrase: d.speakFor(anchored[0])}
		}
	}
	return ids, t
}

// successorRank is one aggregated answer to "what follows": a shape, its
// summed edge weight, and a speakable face.
type successorRank struct {
	template string
	speak    string
	weight   float64
}

// successorsOf aggregates the successor edges leaving (or entering, for
// "before") the given moments, grouped by the shape they point at. The
// strongest shape is the habit the question asked about. Utterances are
// excluded from the answer: a question asked seconds after an event is
// genuinely that event's successor in the graph, but "what follows X"
// asks about the world, and an answer that names the asking would chase
// its own tail.
func (d *Daemon) successorsOf(ids []uint64, rel string) []successorRank {
	var edges []LinkEdge
	var err error
	forward := rel != "before"
	if forward {
		edges, err = d.store.LinksFromRel(ids, RelSuccessor)
	} else {
		edges, err = d.store.LinksToRel(ids, RelSuccessor)
	}
	if err != nil || len(edges) == 0 {
		return nil
	}
	targetIDs := make([]uint64, 0, len(edges))
	for _, e := range edges {
		if forward {
			targetIDs = append(targetIDs, e.To)
		} else {
			targetIDs = append(targetIDs, e.From)
		}
	}
	targets, err := d.fragmentsByIDsTiered(targetIDs)
	if err != nil {
		return nil
	}
	byID := make(map[uint64]*Fragment, len(targets))
	for _, f := range targets {
		byID[f.ID] = f
	}
	agg := map[string]*successorRank{}
	for _, e := range edges {
		id := e.To
		if !forward {
			id = e.From
		}
		f := byID[id]
		if f == nil || f.Channel == "tell" {
			continue
		}
		r := agg[f.Template]
		if r == nil {
			r = &successorRank{template: f.Template, speak: d.speakFor(f)}
			agg[f.Template] = r
		}
		r.weight += e.Weight * f.Weight
	}
	ranked := make([]successorRank, 0, len(agg))
	for _, r := range agg {
		ranked = append(ranked, *r)
	}
	sort.Slice(ranked, func(i, j int) bool {
		if ranked[i].weight != ranked[j].weight {
			return ranked[i].weight > ranked[j].weight
		}
		return ranked[i].template < ranked[j].template
	})
	return ranked
}

// speakFor is a fragment's face in a sentence: its concept's gloss when
// the shape has one, otherwise the raw content with the command marker
// stripped, clipped so one long log line cannot swallow the sentence.
func (d *Daemon) speakFor(f *Fragment) string {
	if c, err := d.store.ConceptByTemplate(f.Template); err == nil && c != nil {
		d.ws.excite(c.ID, exciteRecall)
		return clip(c.Content, spokenPhraseCap)
	}
	return clip(strings.TrimSpace(strings.TrimPrefix(f.Content, actionPrefix)), spokenPhraseCap)
}

// warmestVerb returns the hottest verb-like concept in working memory,
// the referent of "do it again."
func (d *Daemon) warmestVerb() *Fragment {
	entries := d.ws.warmest(workspaceBudget)
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
	byID := make(map[uint64]*Fragment, len(frags))
	for _, f := range frags {
		byID[f.ID] = f
	}
	for _, e := range entries {
		if f := byID[e.id]; f != nil && f.Category == "verb" {
			return f
		}
	}
	return nil
}

// conceptDecision weighs the commands grounding a verb concept, the way
// decide weighs the commands linked to a cue. It lets a resolved concept
// propose without any cue text having been taught.
func (d *Daemon) conceptDecision(c *Fragment) (Decision, error) {
	dec := Decision{Cue: c.Content, CueTemplate: Normalize(c.Content)}
	grounds, err := d.store.LinksFromRel([]uint64{c.ID}, RelGrounds)
	if err != nil || len(grounds) == 0 {
		return dec, err
	}
	ids := make([]uint64, 0, len(grounds))
	for _, e := range grounds {
		ids = append(ids, e.To)
	}
	frags, err := d.store.FragmentsByIDs(ids)
	if err != nil {
		return dec, err
	}
	byID := make(map[uint64]*Fragment, len(frags))
	for _, f := range frags {
		byID[f.ID] = f
	}
	cands := map[string]*Candidate{}
	for _, e := range grounds {
		f := byID[e.To]
		if f == nil {
			continue
		}
		cmd, ok := commandOf(f)
		if !ok {
			continue
		}
		cand := cands[cmd]
		if cand == nil {
			cand = &Candidate{Command: cmd}
			cands[cmd] = cand
		}
		cand.Saliency += e.Weight * f.Weight
		cand.Members = append(cand.Members, f.ID)
		cand.Edges = append(cand.Edges, e)
	}
	if len(cands) == 0 {
		return dec, nil
	}
	drive := dopamineFloor + d.chem.Snapshot().Dopamine
	ranked := make([]*Candidate, 0, len(cands))
	for _, cand := range cands {
		cand.Saliency *= drive
		ranked = append(ranked, cand)
	}
	sort.Slice(ranked, func(i, j int) bool {
		if ranked[i].Saliency != ranked[j].Saliency {
			return ranked[i].Saliency > ranked[j].Saliency
		}
		return ranked[i].Command < ranked[j].Command
	})
	top := ranked[0]
	if top.Saliency < actionFloor {
		return dec, nil
	}
	dec.Proposed = top
	dec.Alternatives = ranked[1:]
	if len(ranked) >= 2 {
		margin := (top.Saliency - ranked[1].Saliency) / top.Saliency
		if margin < d.cfg.ConflictMargin {
			dec.Conflict = true
			dec.Question = conflictQuestion(c.Content, ranked)
			return dec, nil
		}
	}
	h, err := d.store.LoadHabit(dec.CueTemplate, top.Command)
	if err != nil {
		return dec, err
	}
	dec.Status = h.Status
	return dec, nil
}

// plan acts on one parsed frame. It returns the sentence GOLEM says
// (empty when silence is the honest reply) and, for command force, the
// staged proposal. All planning reads memory; the only side effects are
// workspace warmth and the pending proposal slot.
func (d *Daemon) plan(frame Frame, raw string) (string, *DecisionInfo) {
	terms := []Term{frame.Agent, frame.Patient}
	for _, q := range frame.Quals {
		terms = append(terms, q.Anchor)
	}
	for _, t := range terms {
		if t.Concept != nil {
			d.ws.excite(t.Concept.ID, exciteSpeech)
		}
	}

	switch frame.Force {
	case ForceQuestion:
		return d.planQuestion(frame), nil
	case ForceCommand:
		return d.planCommand(frame, raw)
	default:
		// A statement teaches; GOLEM only speaks up when curiosity about
		// an unknown word wins.
		if len(frame.Unknown) > 0 && d.chem.Snapshot().Dopamine >= curiosityGate {
			return Realize(Frame{Force: ForceQuestion, Action: Term{Phrase: "be"},
				Patient: Term{Phrase: frame.Unknown[0]}}, d.chem.Snapshot()), nil
		}
		return "", nil
	}
}

// planQuestion answers from grounded memory or admits it cannot. The
// interrogative routes the question: what and which ask along order or
// for definitions, when asks for a time, where for a channel, how many
// for a count, and who and why name gaps GOLEM genuinely has.
func (d *Daemon) planQuestion(frame Frame) string {
	chem := d.chem.Snapshot()

	// A relation carried by the verb is a qualifier in disguise: "which
	// command follows X" asks after X, "what preceded X" asks before it.
	quals := frame.Quals
	if len(quals) == 0 && !frame.Patient.empty() &&
		(frame.Action.Phrase == "follow" || frame.Action.Phrase == "precede") {
		rel := "after"
		if frame.Action.Phrase == "precede" {
			rel = "before"
		}
		quals = []Qualifier{{Rel: rel, Anchor: frame.Patient}}
	}

	switch {
	case frame.Wh == "why", frame.Wh == "who":
		// No causal model and no agents: the honest answer is the gap
		// itself.
		return Realize(Frame{Force: ForceStatement, Action: Term{Phrase: "know"},
			Negate: true, Patient: Term{Phrase: frame.Wh}}, chem)
	case frame.Counting:
		return d.planCount(frame, quals, chem)
	case frame.Wh == "when", frame.Wh == "where":
		return d.planWhence(frame, quals, chem)
	}

	// "What happens after X", "what comes before X", "which command
	// follows X and precedes Y."
	if len(quals) > 0 && !quals[0].Anchor.empty() {
		return d.planSuccession(frame, quals, chem)
	}

	// "What is X": definitions read the concept layer.
	topic := frame.Patient
	if topic.empty() {
		topic = frame.Agent
	}
	if topic.Concept != nil {
		c := topic.Concept
		tally, _ := d.store.GroundingTally()
		kind := "thing"
		if c.Category == "verb" {
			kind = "action"
		}
		kindWord := indefinite(kind)
		tentative := false
		var parts []string
		if topic.Phrase != "" && !strings.EqualFold(topic.Phrase, c.Content) {
			// The asked word and the concept differ: name the pointer. Greedy
			// early naming bonds bystander words too, so a word that names the
			// concept far more weakly than its firmest name is spoken as a
			// tentative pointer rather than asserted as fact.
			asked, top := d.namingContrast(topic.Phrase, c.ID)
			tentative = asked > 0 && top-asked > namesContrastGap
			parts = append(parts, Realize(Frame{Force: ForceStatement,
				Agent:  Term{Phrase: topic.Phrase},
				Action: Term{Phrase: "name"}, Patient: Term{Phrase: c.Content},
				Hedge: tentative}, chem))
		} else {
			parts = append(parts, Realize(Frame{Force: ForceStatement,
				Agent:  Term{Phrase: c.Content},
				Action: Term{Phrase: "be"}, Patient: Term{Phrase: kindWord}}, chem))
		}
		// A firm answer counts the experience behind the concept; a tentative
		// pointer does not borrow that concept's tally as if it were the
		// word's own.
		if n := tally[c.ID]; n > 0 && !tentative {
			parts = append(parts, Realize(Frame{Force: ForceStatement,
				Action:  Term{Phrase: "see"},
				Patient: Term{Phrase: "it"}, Times: n}, chem))
		}
		return strings.Join(parts, " ")
	}
	if topic.Phrase != "" {
		return Realize(Frame{Force: ForceStatement, Action: Term{Phrase: "know"},
			Negate: true, Patient: Term{Phrase: topic.Phrase}}, chem)
	}
	return Realize(Frame{Force: ForceStatement, Action: Term{Phrase: "know"},
		Negate: true, Patient: Term{Phrase: "that"}}, chem)
}

// planSuccession answers order questions: the qualifiers pick the
// anchors, the successor edges vote, a second qualifier intersects, the
// sortal restricts the kind of answer, and a plural sortal invites a
// coordinated list.
func (d *Daemon) planSuccession(frame Frame, quals []Qualifier, chem LimbicState) string {
	anchors, anchorTerm := d.anchorEpisodes(quals[0].Anchor)
	outQuals := []Qualifier{{Rel: quals[0].Rel, Anchor: anchorTerm}}

	var ranked []successorRank
	if len(anchors) > 0 {
		ranked = d.successorsOf(anchors, quals[0].Rel)
	}

	// "After X and before Y": what follows X is kept only when it also
	// precedes Y.
	if len(quals) > 1 && !quals[1].Anchor.empty() {
		second, secondTerm := d.anchorEpisodes(quals[1].Anchor)
		outQuals = append(outQuals, Qualifier{Rel: quals[1].Rel, Anchor: secondTerm})
		within := map[string]bool{}
		if len(second) > 0 {
			for _, r := range d.successorsOf(second, quals[1].Rel) {
				within[r.template] = true
			}
		}
		kept := ranked[:0]
		for _, r := range ranked {
			if within[r.template] {
				kept = append(kept, r)
			}
		}
		ranked = kept
	}

	// A sortal narrows the kind: "which command" wants only shapes that
	// live in command position.
	if frame.Sort == "command" || frame.Sort == "action" {
		kept := ranked[:0]
		for _, r := range ranked {
			if strings.HasPrefix(r.template, actionPrefix) {
				kept = append(kept, r)
			}
		}
		ranked = kept
	}

	var strong []successorRank
	for _, r := range ranked {
		if r.weight >= successorFloor {
			strong = append(strong, r)
		}
	}
	if len(strong) == 0 {
		return Realize(Frame{Force: ForceStatement, Action: Term{Phrase: "know"},
			Negate: true, Past: frame.Past, Quals: outQuals}, chem)
	}

	// A plural sortal asks for the field, capped at three; everything
	// else takes the winner.
	subjects := []string{strong[0].speak}
	if frame.SortPl {
		for _, r := range strong[1:] {
			if len(subjects) == 3 {
				break
			}
			subjects = append(subjects, r.speak)
		}
	}
	subject := Term{Phrase: joinAnd(subjects), Plural: len(subjects) > 1}

	if quals[0].Rel == "before" && len(outQuals) == 1 {
		// "make test usually precedes make deploy."
		return Realize(Frame{
			Force:   ForceStatement,
			Agent:   subject,
			Action:  Term{Phrase: "precede"},
			Habit:   true,
			Past:    frame.Past,
			Patient: anchorTerm,
		}, chem)
	}
	// "After make test, make deploy usually follows." With a second
	// qualifier the verb turns neutral: "after X and before Y, Z usually
	// happens."
	verb := "follow"
	if len(outQuals) > 1 {
		verb = "happen"
	}
	return Realize(Frame{
		Force:  ForceStatement,
		Agent:  subject,
		Action: Term{Phrase: verb},
		Habit:  true,
		Past:   frame.Past,
		Quals:  outQuals,
	}, chem)
}

// whenceAnchor picks the term a when/where/count question is about: the
// qualifier's anchor when one was spoken, otherwise the patient or the
// subject.
func whenceAnchor(frame Frame, quals []Qualifier) Term {
	if len(quals) > 0 && !quals[0].Anchor.empty() {
		return quals[0].Anchor
	}
	if !frame.Patient.empty() {
		return frame.Patient
	}
	return frame.Agent
}

// planWhence answers when and where: the newest moment the anchor
// stands for supplies its time or its channel.
func (d *Daemon) planWhence(frame Frame, quals []Qualifier, chem LimbicState) string {
	topic := whenceAnchor(frame, quals)
	ids, face := d.anchorEpisodes(topic)
	if len(ids) == 0 {
		return Realize(Frame{Force: ForceStatement, Action: Term{Phrase: "know"},
			Negate: true, Patient: Term{Phrase: frame.Wh}}, chem)
	}
	frags, err := d.fragmentsByIDsTiered(ids)
	if err != nil || len(frags) == 0 {
		return Realize(Frame{Force: ForceStatement, Action: Term{Phrase: "know"},
			Negate: true, Patient: Term{Phrase: frame.Wh}}, chem)
	}
	newest := frags[0]
	for _, f := range frags {
		if f.Time.After(newest.Time) {
			newest = f
		}
	}
	at := "the " + newest.Channel + " channel"
	last := false
	if frame.Wh == "when" {
		at = newest.Time.Format("Jan 2 at 15:04")
		last = true
	}
	// "Make test last happened on Jun 11 at 09:14." / "Make test
	// happened on the shell channel."
	return Realize(Frame{
		Force:  ForceStatement,
		Agent:  face,
		Action: Term{Phrase: "happen"},
		Past:   true,
		Last:   last,
		At:     at,
	}, chem)
}

// planCount answers "how many times" and "how often" with the tally of
// moments behind the anchor, prototype counts included.
func (d *Daemon) planCount(frame Frame, quals []Qualifier, chem LimbicState) string {
	topic := whenceAnchor(frame, quals)
	ids, face := d.anchorEpisodes(topic)
	if len(ids) > 0 {
		if frags, err := d.fragmentsByIDsTiered(ids); err == nil && len(frags) > 0 {
			n := 0
			for _, f := range frags {
				n += f.Count
			}
			return Realize(Frame{Force: ForceStatement, Action: Term{Phrase: "see"},
				Patient: face, Times: n}, chem)
		}
	}
	return Realize(Frame{Force: ForceStatement, Action: Term{Phrase: "know"},
		Negate: true, Patient: Term{Phrase: frame.Wh}}, chem)
}

// planCommand turns command force into a staged proposal: by taught cue
// first, then through the concept the words or the workspace point at.
// The proposal waits for approval exactly like one made with golem
// propose; speech never runs a command by itself.
func (d *Daemon) planCommand(frame Frame, raw string) (string, *DecisionInfo) {
	chem := d.chem.Snapshot()
	dec, err := d.decide(raw)
	if err == nil && dec.Proposed == nil {
		var c *Fragment
		switch {
		case frame.Patient.Concept != nil:
			c = frame.Patient.Concept
		case frame.Pro || frame.Again:
			c = d.warmestVerb()
		}
		if c != nil {
			if cdec, cerr := d.conceptDecision(c); cerr == nil && cdec.Proposed != nil {
				dec = cdec
			}
		}
	}

	if dec.Proposed == nil {
		return Realize(Frame{Force: ForceStatement, Action: Term{Phrase: "know"},
			Negate: true, Patient: Term{Phrase: "how to " + raw}}, chem), nil
	}

	di := decisionInfo(dec)
	outcome := "proposed"
	if dec.Conflict {
		outcome = "conflict"
	}
	d.event(Event{Ev: "proposal", Cue: dec.CueTemplate, Action: di.Proposed, Outcome: outcome})
	d.pending = &dec

	if dec.Conflict {
		return dec.Question, di
	}
	say := Realize(Frame{Force: ForceStatement, Action: Term{Phrase: "propose"},
		Patient: Term{Phrase: dec.Proposed.Command}}, chem)
	return say, di
}
