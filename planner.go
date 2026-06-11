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
	for _, t := range []Term{frame.Agent, frame.Patient, frame.Anchor} {
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

// planQuestion answers from grounded memory or admits it cannot.
func (d *Daemon) planQuestion(frame Frame) string {
	chem := d.chem.Snapshot()

	// "What happens after X" and "what comes before X."
	if frame.Rel != "" && !frame.Anchor.empty() {
		anchors, anchorTerm := d.anchorEpisodes(frame.Anchor)
		if len(anchors) > 0 {
			ranked := d.successorsOf(anchors, frame.Rel)
			if len(ranked) > 0 && ranked[0].weight >= successorFloor {
				if frame.Rel == "before" {
					// "make test usually precedes make deploy."
					return Realize(Frame{
						Force:   ForceStatement,
						Agent:   Term{Phrase: ranked[0].speak},
						Action:  Term{Phrase: "precede"},
						Habit:   true,
						Patient: anchorTerm,
					}, chem)
				}
				// "After make test, make deploy usually follows."
				return Realize(Frame{
					Force:  ForceStatement,
					Agent:  Term{Phrase: ranked[0].speak},
					Action: Term{Phrase: "follow"},
					Habit:  true,
					Rel:    frame.Rel,
					Anchor: anchorTerm,
				}, chem)
			}
		}
		return Realize(Frame{Force: ForceStatement, Action: Term{Phrase: "know"},
			Negate: true, Rel: frame.Rel, Anchor: anchorTerm}, chem)
	}

	// "What is X": definitions read the concept layer.
	topic := frame.Patient
	if topic.empty() {
		topic = frame.Agent
	}
	if topic.Concept != nil {
		c := topic.Concept
		tally, _ := d.store.GroundingTally()
		kindWord := "a thing"
		if c.Category == "verb" {
			kindWord = "an action"
		}
		var parts []string
		if topic.Phrase != "" && !strings.EqualFold(topic.Phrase, c.Content) {
			// The asked word and the concept differ: name the pointer.
			parts = append(parts, Realize(Frame{Force: ForceStatement,
				Agent:  Term{Phrase: topic.Phrase},
				Action: Term{Phrase: "name"}, Patient: Term{Phrase: c.Content}}, chem))
		} else {
			parts = append(parts, Realize(Frame{Force: ForceStatement,
				Agent:  Term{Phrase: c.Content},
				Action: Term{Phrase: "be"}, Patient: Term{Phrase: kindWord}}, chem))
		}
		if n := tally[c.ID]; n > 0 {
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
