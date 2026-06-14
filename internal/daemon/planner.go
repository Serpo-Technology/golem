package daemon

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"golem/internal/attention"
	"golem/internal/events"
	"golem/internal/grammar"
	"golem/internal/limbic"
	"golem/internal/memory"
	"golem/internal/senses"
	"golem/wire"
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
	// animacyBindBonus is how much a pro-form's animacy bends its binding:
	// a fitting referent's activation is lifted by this share and a clashing
	// one cut by it, enough to overtake an equally warm rival without
	// overriding a far warmer one.
	animacyBindBonus = 0.5
)

// resolveConcept maps a surface phrase to its strongest concept: first
// through words that name one, then by gloss content, the same fallback
// recall uses for phrases nobody taught yet.
func (d *Daemon) resolveConcept(phrase string) *memory.Fragment {
	tokens, _ := grammar.Tokenize(phrase)
	words, err := d.store.WordNodesByForms(tokens)
	if err == nil && len(words) > 0 {
		ids := make([]uint64, 0, len(words))
		for _, w := range words {
			ids = append(ids, w.ID)
		}
		names, err := d.store.LinksFromRel(ids, memory.RelNames)
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
// concept's firmest name. Reliability is the only part of a naming weight
// that varies per word; the dopamine gain that lifts every name the same
// cancels in the gap, so a large gap means this word kept the concept's
// company far less faithfully than its best-established name and is a
// weak pointer, not a settled one.
func (d *Daemon) namingContrast(phrase string, conceptID uint64) (asked, top float64) {
	if into, err := d.store.LinksToRel([]uint64{conceptID}, memory.RelNames); err == nil {
		for _, e := range into {
			if e.Weight > top {
				top = e.Weight
			}
		}
	}
	tokens, _ := grammar.Tokenize(phrase)
	words, err := d.store.WordNodesByForms(tokens)
	if err != nil || len(words) == 0 {
		return 0, top
	}
	ids := make([]uint64, 0, len(words))
	for _, w := range words {
		ids = append(ids, w.ID)
	}
	names, err := d.store.LinksFromRel(ids, memory.RelNames)
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
func (d *Daemon) anchorEpisodes(t grammar.Term) ([]uint64, grammar.Term) {
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
				return ids, grammar.Term{Phrase: t.Phrase}
			}
		}
	}
	if t.Concept != nil {
		grounds, err := d.store.LinksFromRel([]uint64{t.Concept.ID}, memory.RelGrounds)
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
			return ids, grammar.Term{Phrase: d.speakFor(anchored[0])}
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
	var edges []memory.LinkEdge
	var err error
	forward := rel != "before"
	if forward {
		edges, err = d.store.LinksFromRel(ids, memory.RelSuccessor)
	} else {
		edges, err = d.store.LinksToRel(ids, memory.RelSuccessor)
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
	byID := make(map[uint64]*memory.Fragment, len(targets))
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
func (d *Daemon) speakFor(f *memory.Fragment) string {
	if c, err := d.store.ConceptByTemplate(f.Template); err == nil && c != nil {
		d.ws.excite(c.ID, exciteRecall)
		return wire.Clip(c.Content, spokenPhraseCap)
	}
	return wire.Clip(strings.TrimSpace(strings.TrimPrefix(f.Content, senses.ActionPrefix)), spokenPhraseCap)
}

// warmestVerb returns the hottest verb-like concept in working memory,
// the referent of "do it again"; skip steps past the warmest matches,
// so "the other one" reaches the second.
func (d *Daemon) warmestVerb(skip int) *memory.Fragment {
	return d.warmConceptWhere(skip, func(f *memory.Fragment) bool { return f.Category == "verb" })
}

// warmConcept returns the workspace's hottest concept of any category,
// the referent of a bare "it" or "that"; skip steps past the warmest.
func (d *Daemon) warmConcept(skip int) *memory.Fragment {
	return d.warmConceptWhere(skip, func(f *memory.Fragment) bool { return true })
}

func (d *Daemon) warmConceptWhere(skip int, keep func(*memory.Fragment) bool) *memory.Fragment {
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
	byID := make(map[uint64]*memory.Fragment, len(frags))
	for _, f := range frags {
		byID[f.ID] = f
	}
	for _, e := range entries {
		f := byID[e.id]
		if f == nil || f.Kind != memory.KindConcept || !keep(f) {
			continue
		}
		if skip > 0 {
			skip--
			continue
		}
		return f
	}
	return nil
}

// warmTopic resolves a pro-form against working memory by best fit: the most
// active concept that fits the pronoun's animacy, "it" leaning to the
// inanimate referent and "she" to the animate, read from the role each last
// played. With no animacy asked, or no role to read, it is the warmest, the
// way a bare "it" and "the other one" resolved before. The workspace carrying
// this referent across turns is what holds the thread, and resolving is itself
// a touch, so the topic stays current for the turn after.
func (d *Daemon) warmTopic(alt bool, animacy string) grammar.Term {
	skip := 0
	if alt {
		skip = 1
	}
	if c := d.warmReferent(skip, animacy); c != nil {
		d.ws.excite(c.ID, exciteSpeech)
		return grammar.Term{Concept: c, Phrase: c.Content}
	}
	return grammar.Term{}
}

// warmReferent ranks the warm concepts for a pro-form: activation is the base,
// and when the pronoun asks for an animacy a concept's lean shifts its score,
// so a slightly cooler but fitting referent can win while a clear winner on
// warmth alone still stands. Skip steps past the top, "the other one" reaching
// the next.
func (d *Daemon) warmReferent(skip int, animacy string) *memory.Fragment {
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
	byID := make(map[uint64]*memory.Fragment, len(frags))
	for _, f := range frags {
		byID[f.ID] = f
	}
	type scored struct {
		f     *memory.Fragment
		score float64
		heat  float64
	}
	ranked := make([]scored, 0, len(entries))
	for _, e := range entries {
		f := byID[e.id]
		if f == nil || f.Kind != memory.KindConcept {
			continue
		}
		score := e.heat
		if animacy != "" {
			score *= 1 + animacyBindBonus*d.animacyLean(f.ID, animacy)
		}
		ranked = append(ranked, scored{f, score, e.heat})
	}
	sort.Slice(ranked, func(i, j int) bool {
		if ranked[i].score != ranked[j].score {
			return ranked[i].score > ranked[j].score
		}
		if ranked[i].heat != ranked[j].heat {
			return ranked[i].heat > ranked[j].heat
		}
		return d.ws.touchOf(ranked[i].f.ID) > d.ws.touchOf(ranked[j].f.ID)
	})
	if skip >= len(ranked) {
		return nil
	}
	return ranked[skip].f
}

// animacyLean scores how well a concept fits a wanted animacy, from the role
// it last played: an agent leans animate, a patient inanimate, the things that
// act being the animate ones. The lean is positive when the concept fits the
// pronoun, negative when it clashes, zero when no role has been seen, so the
// binding falls back to pure warmth.
func (d *Daemon) animacyLean(id uint64, animacy string) float64 {
	lean := 0.0
	switch d.ws.roleOf(id) {
	case "agent":
		lean = 1
	case "patient":
		lean = -1
	}
	if animacy == "inanimate" {
		lean = -lean
	}
	return lean
}

// warmSpoken warms the concept behind a shape the answer actually
// spoke. Saying is the strongest touch there is, so the conversational
// thread follows the answer: after "make deploy usually follows," a
// bare "it" means the deploy.
func (d *Daemon) warmSpoken(template string) {
	if c, err := d.store.ConceptByTemplate(template); err == nil && c != nil {
		d.ws.excite(c.ID, exciteSpeech)
	}
}

// trackTopic follows the subject of the exchange across turns: the dominant
// warm concept is the topic, sustained on the decaying field the inner loop
// already biases toward, so a subject stays current until something displaces
// it. When a new concept takes the field the change is noticed, recorded on the
// trail, so the shift can be seen rather than passing silently.
func (d *Daemon) trackTopic() {
	c := d.warmConcept(0)
	if c == nil {
		return
	}
	if d.topic != 0 && d.topic != c.ID {
		d.event(events.Event{Ev: "topic", From: d.topic, To: c.ID})
	}
	d.topic = c.ID
}

// conceptDecision weighs the commands grounding a verb concept, the way
// decide weighs the commands linked to a cue. It lets a resolved concept
// propose without any cue text having been taught.
func (d *Daemon) conceptDecision(c *memory.Fragment) (Decision, error) {
	dec := Decision{Cue: c.Content, CueTemplate: attention.Normalize(c.Content)}
	grounds, err := d.store.LinksFromRel([]uint64{c.ID}, memory.RelGrounds)
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
	byID := make(map[uint64]*memory.Fragment, len(frags))
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

// realizeAs answers in the language the question arrived in: the
// frame's language is the one knob the planner sets on every reply.
func (d *Daemon) realizeAs(lang string, f grammar.Frame, chem limbic.State) string {
	f.Lang = lang
	return d.gram.Realize(f, chem)
}

// plan acts on one parsed frame. It returns the sentence GOLEM says
// (empty when silence is the honest reply) and, for command force, the
// staged proposal. All planning reads memory; the side effects are
// workspace warmth, the pending proposal slot, and a comprehended
// proposition written into the graph.
func (d *Daemon) plan(frame grammar.Frame, raw string, momentID uint64) (string, *wire.DecisionInfo) {
	terms := []grammar.Term{frame.Agent, frame.Patient}
	for _, q := range frame.Quals {
		terms = append(terms, q.Anchor)
	}
	for _, t := range terms {
		if t.Concept != nil {
			d.ws.excite(t.Concept.ID, exciteSpeech)
		}
	}

	// "And then" is a move on the warm proposition, not a clause: walk its
	// successor and speak what came next.
	if isThenMove(raw) {
		return d.discourseWalk(frame, "after", d.chem.Snapshot()), nil
	}

	switch frame.Force {
	case grammar.ForceQuestion:
		return d.planQuestion(frame, raw), nil
	case grammar.ForceCommand:
		return d.planCommand(frame, raw)
	default:
		// Comprehension carries a told proposition into the graph the way
		// perception carries a percept. A statement understood as a
		// proposition is taught, not queried, so the curiosity question is
		// held back for the verb it just grounded.
		if d.comprehendStatement(frame, raw, momentID) {
			return "", nil
		}
		// A statement teaches; GOLEM only speaks up when curiosity about
		// an unknown word wins, and an asked question is held as a goal.
		if len(frame.Unknown) > 0 && d.chem.Snapshot().Dopamine >= curiosityGate {
			d.adoptGoal("learn", frame.Unknown[0])
			return d.realizeAs(frame.Lang, grammar.Frame{Force: grammar.ForceQuestion, Action: grammar.Term{Phrase: "be"},
				Patient: grammar.Term{Phrase: frame.Unknown[0]}}, d.chem.Snapshot()), nil
		}
		return "", nil
	}
}

// planQuestion answers from grounded memory or admits it cannot. The
// interrogative routes the question: what and which ask along order or
// for definitions, when asks for a time, where for a channel, how many
// for a count, who and why name gaps GOLEM genuinely has, and a
// question about thinking reads the inner loop. Unresolved words can
// earn an appended follow-up question when curiosity is high: deciding
// what to say next includes asking.
func (d *Daemon) planQuestion(frame grammar.Frame, raw string) string {
	return d.withFollowUp(frame, d.planAnswer(frame, raw))
}

// withFollowUp appends the curiosity question when the exchange left a
// word pointing at nothing: GOLEM holds the thread by asking after the
// gap instead of only answering around it. A phrase the answer already
// speaks was not a gap, whether it anchored the reply or names the very
// thing the admission is about.
func (d *Daemon) withFollowUp(frame grammar.Frame, answer string) string {
	if len(frame.Unknown) == 0 || d.chem.Snapshot().Dopamine < curiosityGate {
		return answer
	}
	spoken := strings.ToLower(answer)
	for _, unknown := range frame.Unknown {
		if unknown == "" || strings.Contains(spoken, strings.ToLower(unknown)) {
			continue
		}
		// Asking is more than a reflex: the open question is held as a
		// goal, and the drives keep hunting it until it resolves.
		d.adoptGoal("learn", unknown)
		ask := d.realizeAs(frame.Lang, grammar.Frame{Force: grammar.ForceQuestion,
			Action:  grammar.Term{Phrase: "be"},
			Patient: grammar.Term{Phrase: unknown}}, d.chem.Snapshot())
		if answer == "" {
			return ask
		}
		return answer + " " + ask
	}
	return answer
}

func (d *Daemon) planAnswer(frame grammar.Frame, raw string) string {
	chem := d.chem.Snapshot()

	// "What were you thinking" reads the inner loop, the same report the
	// introspect act gives.
	if frame.Action.Phrase == "think" {
		line := d.introspectLine()
		d.finishAct(actIntrospect, "", line, d.lastThought != "", "asked")
		return line
	}

	// A question about a relation's role is answered from the proposition
	// graph through the engine: the walk crosses to the co-argument and the
	// construction speaks the proposition back, not a hardcoded reply.
	if line, ok := d.planRoleQuestion(frame, raw); ok {
		return line
	}
	// "What did the dog do" asks the relation itself: the walk reads the
	// agent's strongest proposition and the engine speaks the whole clause.
	if line, ok := d.planActivityQuestion(frame, raw); ok {
		return line
	}

	// A relation carried by the verb is a qualifier in disguise: "which
	// command follows X" asks after X, "what preceded X" asks before it.
	quals := frame.Quals
	if len(quals) == 0 && !frame.Patient.Empty() &&
		(frame.Action.Phrase == "follow" || frame.Action.Phrase == "precede") {
		rel := "after"
		if frame.Action.Phrase == "precede" {
			rel = "before"
		}
		quals = []grammar.Qualifier{{Rel: rel, Anchor: frame.Patient}}
	}

	switch {
	case frame.Wh == "why":
		// "Why" is a move on the warm proposition: walk to what precedes it,
		// the antecedent in the learned sequence, GOLEM's grounded answer where
		// it has no causal model. With nothing preceding, the honest gap.
		return d.discourseWalk(frame, "before", chem)
	case frame.Wh == "who":
		// No agents modeled: the honest answer is the gap itself.
		return d.realizeAs(frame.Lang, grammar.Frame{Force: grammar.ForceStatement, Action: grammar.Term{Phrase: "know"},
			Negate: true, Patient: grammar.Term{Phrase: frame.Wh}}, chem)
	case frame.Counting:
		return d.planCount(frame, quals, chem)
	case frame.Wh == "when", frame.Wh == "where":
		return d.planWhence(frame, quals, chem)
	}

	// "What happens after X", "what comes before X", "which command
	// follows X and precedes Y."
	if len(quals) > 0 && !quals[0].Anchor.Empty() {
		return d.planSuccession(frame, quals, chem)
	}

	// "What is X": definitions read the concept layer. A bare pro-form
	// ("what is it?", "what is the other one?") resolves against what is
	// still warm from the turns before.
	topic := frame.Patient
	if topic.Empty() {
		topic = frame.Agent
	}
	if topic.Empty() && (frame.Pro || frame.Alt) {
		topic = d.warmTopic(frame.Alt, frame.ProType)
	}
	if topic.Concept != nil {
		c := topic.Concept
		tally, _ := d.store.GroundingTally()
		// A relation reified as a concept is an action, the same kind a verb
		// names, so "what is chase" answers that chase is an action and not a
		// thing.
		action := c.Category == "verb" || c.Category == "relation"
		kind := "thing"
		if action {
			kind = "action"
		}
		kindWord := grammar.Indefinite(kind)
		if frame.Lang == "tr" {
			kindWord = "bir şey"
			if action {
				kindWord = "bir eylem"
			}
		}
		if topic.Phrase != "" && !strings.EqualFold(topic.Phrase, c.Content) {
			// The asked word and the concept differ: name the pointer. Greedy
			// early naming bonds bystander words too, so a word that names the
			// concept far more weakly than its firmest name is spoken as a
			// tentative pointer rather than asserted as fact. A tentative
			// pointer does not borrow the concept's tally as if it were the
			// word's own.
			asked, top := d.namingContrast(topic.Phrase, c.ID)
			tentative := asked > 0 && top-asked > namesContrastGap
			parts := []string{d.realizeAs(frame.Lang, grammar.Frame{Force: grammar.ForceStatement,
				Agent:  grammar.Term{Phrase: topic.Phrase},
				Action: grammar.Term{Phrase: "name"}, Patient: grammar.Term{Phrase: c.Content},
				Hedge: tentative}, chem)}
			if n := tally[c.ID]; n > 0 && !tentative {
				parts = append(parts, d.countClause(frame.Lang, n, chem))
			}
			return strings.Join(parts, " ")
		}
		// A firm definition, composed: the definition clause, and when
		// experience stands behind the concept the count clause nests beside
		// it. The sentence is built from constructions, not a fixed template,
		// and the composition reads back on the trail as the constructions
		// that made it.
		msg := grammar.Message{Intent: "described", Slots: map[string]grammar.Filler{
			"defn": {Sub: &grammar.Message{Intent: "definition",
				Slots: grammar.TextSlots(map[string]string{"subject": c.Content, "kind": kindWord})}},
		}}
		if n := tally[c.ID]; n > 0 {
			msg.Slots["count"] = grammar.Filler{Sub: &grammar.Message{Intent: "seen_count",
				Slots: grammar.TextSlots(map[string]string{"object": "it", "count": grammar.TimesWord(n)})}}
		}
		if line, ok := d.compose(frame.Lang, msg); ok {
			return line
		}
		// With no constructions the innate realizer assembles the same
		// answer, so the engine stays the floor under the learned patterns.
		parts := []string{d.definitionClause(frame.Lang, c.Content, kindWord, chem)}
		if n := tally[c.ID]; n > 0 {
			parts = append(parts, d.countClause(frame.Lang, n, chem))
		}
		return strings.Join(parts, " ")
	}
	if topic.Phrase != "" {
		return d.realizeAs(frame.Lang, grammar.Frame{Force: grammar.ForceStatement, Action: grammar.Term{Phrase: "know"},
			Negate: true, Patient: grammar.Term{Phrase: topic.Phrase}}, chem)
	}
	return d.realizeAs(frame.Lang, grammar.Frame{Force: grammar.ForceStatement, Action: grammar.Term{Phrase: "know"},
		Negate: true, Patient: grammar.Term{Phrase: "that"}}, chem)
}

// discourseWalk reads a follow-up as a move on the warm proposition: it takes
// the topic named or, lacking one, the warm referent, and walks the succession
// relation from it, "before" for why (the antecedent) and "after" for and-then
// (the consequent), so a connective runs on the walk already built rather than
// a parser of dialogue acts. With no proposition in mind it is the honest gap.
func (d *Daemon) discourseWalk(frame grammar.Frame, rel string, chem limbic.State) string {
	topic := frame.Patient
	if topic.Empty() {
		topic = frame.Agent
	}
	if topic.Empty() {
		topic = d.warmTopic(frame.Alt, frame.ProType)
	}
	if topic.Empty() {
		wh := frame.Wh
		if wh == "" {
			wh = "why"
		}
		return d.realizeAs(frame.Lang, grammar.Frame{Force: grammar.ForceStatement, Action: grammar.Term{Phrase: "know"},
			Negate: true, Patient: grammar.Term{Phrase: wh}}, chem)
	}
	return d.planSuccession(frame, []grammar.Qualifier{{Rel: rel, Anchor: topic}}, chem)
}

// isThenMove reports whether an utterance is the bare successor connective,
// "then" or "and then", a discourse move asking what came next rather than a
// clause of its own.
func isThenMove(raw string) bool {
	tokens, _ := grammar.Tokenize(raw)
	if len(tokens) == 0 {
		return false
	}
	hasThen := false
	for _, t := range tokens {
		switch t {
		case "then":
			hasThen = true
		case "and":
		default:
			return false
		}
	}
	return hasThen
}

// planSuccession answers order questions: the qualifiers pick the
// anchors, the successor edges vote, a second qualifier intersects, the
// sortal restricts the kind of answer, and a plural sortal invites a
// coordinated list.
func (d *Daemon) planSuccession(frame grammar.Frame, quals []grammar.Qualifier, chem limbic.State) string {
	anchors, anchorTerm := d.anchorEpisodes(quals[0].Anchor)
	outQuals := []grammar.Qualifier{{Rel: quals[0].Rel, Anchor: anchorTerm}}

	var ranked []successorRank
	if len(anchors) > 0 {
		ranked = d.successorsOf(anchors, quals[0].Rel)
	}

	// "After X and before Y": what follows X is kept only when it also
	// precedes Y.
	if len(quals) > 1 && !quals[1].Anchor.Empty() {
		second, secondTerm := d.anchorEpisodes(quals[1].Anchor)
		outQuals = append(outQuals, grammar.Qualifier{Rel: quals[1].Rel, Anchor: secondTerm})
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
			if strings.HasPrefix(r.template, senses.ActionPrefix) {
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
		return d.realizeAs(frame.Lang, grammar.Frame{Force: grammar.ForceStatement, Action: grammar.Term{Phrase: "know"},
			Negate: true, Past: frame.Past, Quals: outQuals}, chem)
	}

	// A plural sortal asks for the field, capped at three; everything
	// else takes the winner. What the answer names becomes the warmest
	// referent, so the thread follows the answer.
	subjects := []string{strong[0].speak}
	d.warmSpoken(strong[0].template)
	if frame.SortPl {
		for _, r := range strong[1:] {
			if len(subjects) == 3 {
				break
			}
			subjects = append(subjects, r.speak)
			d.warmSpoken(r.template)
		}
	}
	subject := grammar.Term{Phrase: grammar.JoinAnd(subjects), Plural: len(subjects) > 1}

	if quals[0].Rel == "before" && len(outQuals) == 1 {
		// "Make test usually precedes make deploy." The pattern is a
		// construction; the verb group inside it is conjugated by the
		// engine, so morphology stays innate while the frame is learned.
		slots := map[string]string{
			"subject":   subject.Speak(),
			"anchor":    anchorTerm.Speak(),
			"vp_assert": d.gram.VerbPhrase("precede", subject.Plural, frame.Past, "habit"),
			"vp_hedge":  d.gram.VerbPhrase("precede", subject.Plural, frame.Past, "hedge"),
		}
		if line, ok := d.construct(frame.Lang, "succession_before", slots); ok {
			return line
		}
		return d.realizeAs(frame.Lang, grammar.Frame{
			Force:   grammar.ForceStatement,
			Agent:   subject,
			Action:  grammar.Term{Phrase: "precede"},
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
	slots := map[string]string{
		"adjunct":   successionAdjunct(outQuals),
		"subject":   subject.Speak(),
		"vp_assert": d.gram.VerbPhrase(verb, subject.Plural, frame.Past, "habit"),
		"vp_hedge":  d.gram.VerbPhrase(verb, subject.Plural, frame.Past, "hedge"),
	}
	if line, ok := d.construct(frame.Lang, "succession_after", slots); ok {
		return line
	}
	return d.realizeAs(frame.Lang, grammar.Frame{
		Force:  grammar.ForceStatement,
		Agent:  subject,
		Action: grammar.Term{Phrase: verb},
		Habit:  true,
		Past:   frame.Past,
		Quals:  outQuals,
	}, chem)
}

// definitionClause speaks what kind of thing a concept is, through the
// definition construction with the innate realizer as the fallback when
// none is seeded.
func (d *Daemon) definitionClause(lang, subject, kindWord string, chem limbic.State) string {
	if line, ok := d.construct(lang, "definition",
		map[string]string{"subject": subject, "kind": kindWord}); ok {
		return line
	}
	return d.realizeAs(lang, grammar.Frame{Force: grammar.ForceStatement,
		Agent:  grammar.Term{Phrase: subject},
		Action: grammar.Term{Phrase: "be"}, Patient: grammar.Term{Phrase: kindWord}}, chem)
}

// countClause speaks how much experience stands behind a concept, through
// the seen_count construction with the innate realizer as the fallback.
func (d *Daemon) countClause(lang string, n int, chem limbic.State) string {
	if line, ok := d.construct(lang, "seen_count",
		map[string]string{"object": "it", "count": grammar.TimesWord(n)}); ok {
		return line
	}
	return d.realizeAs(lang, grammar.Frame{Force: grammar.ForceStatement,
		Action:  grammar.Term{Phrase: "see"},
		Patient: grammar.Term{Phrase: "it"}, Times: n}, chem)
}

// successionAdjunct fronts the temporal qualifiers of a succession answer
// into the construction's adjunct slot: "after make test", or "after make
// test and before make deploy" when two anchors coordinate. The engine
// fronts the same span when it realizes the fallback frame.
func successionAdjunct(quals []grammar.Qualifier) string {
	parts := make([]string, 0, len(quals))
	for _, q := range quals {
		if q.Anchor.Empty() {
			continue
		}
		rel := "after"
		if q.Rel == "before" {
			rel = "before"
		}
		parts = append(parts, rel+" "+q.Anchor.Speak())
	}
	return strings.Join(parts, " and ")
}

// whenceAnchor picks the term a when/where/count question is about: the
// qualifier's anchor when one was spoken, otherwise the patient or the
// subject, otherwise the warm referent a pro-form points at ("when did
// it happen?").
func (d *Daemon) whenceAnchor(frame grammar.Frame, quals []grammar.Qualifier) grammar.Term {
	if len(quals) > 0 && !quals[0].Anchor.Empty() {
		return quals[0].Anchor
	}
	if !frame.Patient.Empty() {
		return frame.Patient
	}
	if frame.Agent.Empty() && (frame.Pro || frame.Alt) {
		return d.warmTopic(frame.Alt, frame.ProType)
	}
	return frame.Agent
}

// planWhence answers when and where: the newest moment the anchor
// stands for supplies its time or its channel.
func (d *Daemon) planWhence(frame grammar.Frame, quals []grammar.Qualifier, chem limbic.State) string {
	topic := d.whenceAnchor(frame, quals)
	ids, face := d.anchorEpisodes(topic)
	if len(ids) == 0 {
		return d.realizeAs(frame.Lang, grammar.Frame{Force: grammar.ForceStatement, Action: grammar.Term{Phrase: "know"},
			Negate: true, Patient: grammar.Term{Phrase: frame.Wh}}, chem)
	}
	frags, err := d.fragmentsByIDsTiered(ids)
	if err != nil || len(frags) == 0 {
		return d.realizeAs(frame.Lang, grammar.Frame{Force: grammar.ForceStatement, Action: grammar.Term{Phrase: "know"},
			Negate: true, Patient: grammar.Term{Phrase: frame.Wh}}, chem)
	}
	newest := frags[0]
	for _, f := range frags {
		if f.Time.After(newest.Time) {
			newest = f
		}
	}
	d.warmSpoken(newest.Template)
	at := "the " + newest.Channel + " channel"
	last := false
	if frame.Wh == "when" {
		at = newest.Time.Format("Jan 2 at 15:04")
		last = true
	}
	// "Make test last happened on Jun 11 at 09:14." / "Make test
	// happened on the shell channel."
	return d.realizeAs(frame.Lang, grammar.Frame{
		Force:  grammar.ForceStatement,
		Agent:  face,
		Action: grammar.Term{Phrase: "happen"},
		Past:   true,
		Last:   last,
		At:     at,
	}, chem)
}

// planCount answers "how many times" and "how often" with the tally of
// moments behind the anchor, prototype counts included.
func (d *Daemon) planCount(frame grammar.Frame, quals []grammar.Qualifier, chem limbic.State) string {
	topic := d.whenceAnchor(frame, quals)
	ids, face := d.anchorEpisodes(topic)
	if len(ids) > 0 {
		if frags, err := d.fragmentsByIDsTiered(ids); err == nil && len(frags) > 0 {
			n := 0
			for _, f := range frags {
				n += f.Count
			}
			return d.realizeAs(frame.Lang, grammar.Frame{Force: grammar.ForceStatement, Action: grammar.Term{Phrase: "see"},
				Patient: face, Times: n}, chem)
		}
	}
	return d.realizeAs(frame.Lang, grammar.Frame{Force: grammar.ForceStatement, Action: grammar.Term{Phrase: "know"},
		Negate: true, Patient: grammar.Term{Phrase: frame.Wh}}, chem)
}

// stageProposal parks a decision in the pending slot behind the usual
// approve and deny, records it on the trail, and returns its wire form.
// Speech, the propose act, and the propose verb all stage through here.
func (d *Daemon) stageProposal(dec Decision) *wire.DecisionInfo {
	di := decisionInfo(dec)
	outcome := "proposed"
	if dec.Conflict {
		outcome = "conflict"
	}
	d.event(events.Event{Ev: "proposal", Cue: dec.CueTemplate, Action: di.Proposed, Outcome: outcome})
	d.pending = &dec
	return di
}

// planCommand acts on command force. A faculty verb drives the matching
// internal act directly; anything else becomes a staged proposal, by
// taught cue first, then through the concept the words or the workspace
// point at. A proposal waits for approval exactly like one made with
// golem propose; speech never runs an external command by itself.
func (d *Daemon) planCommand(frame grammar.Frame, raw string) (string, *wire.DecisionInfo) {
	chem := d.chem.Snapshot()
	switch frame.Action.Phrase {
	case "recall":
		return d.planRecall(frame, chem), nil
	case "consolidate":
		outcome, fruitful := d.actConsolidateNow()
		d.finishAct(actConsolidate, "", outcome, fruitful, "asked")
		return "I consolidated: " + outcome + ".", nil
	case "introspect":
		line := d.introspectLine()
		d.finishAct(actIntrospect, "", line, d.lastThought != "", "asked")
		return line, nil
	}
	dec, err := d.decide(raw)
	if err == nil && dec.Proposed == nil {
		var c *memory.Fragment
		switch {
		case frame.Patient.Concept != nil:
			c = frame.Patient.Concept
		case frame.Alt:
			c = d.warmestVerb(1)
		case frame.Pro || frame.Again:
			c = d.warmestVerb(0)
		}
		if c != nil {
			if cdec, cerr := d.conceptDecision(c); cerr == nil && cdec.Proposed != nil {
				dec = cdec
			}
		}
	}

	if dec.Proposed == nil {
		return d.realizeAs(frame.Lang, grammar.Frame{Force: grammar.ForceStatement, Action: grammar.Term{Phrase: "know"},
			Negate: true, Patient: grammar.Term{Phrase: "how to " + raw}}, chem), nil
	}

	di := d.stageProposal(dec)
	if dec.Conflict {
		return dec.Question, di
	}
	say := d.realizeAs(frame.Lang, grammar.Frame{Force: grammar.ForceStatement, Action: grammar.Term{Phrase: "propose"},
		Patient: grammar.Term{Phrase: dec.Proposed.Command}}, chem)
	return say, di
}

// planRecall drives the recall faculty from speech. "Remember this as a
// bell" is the naming shape: the pro-form picks the warm referent and
// the patient word comes to name it. Everything else retrieves: the
// topic is the patient, or the warm referent a pro-form points at, and
// the reply speaks the strongest moment found.
func (d *Daemon) planRecall(frame grammar.Frame, chem limbic.State) string {
	if frame.Pro && !frame.Patient.Empty() {
		return d.nameWarmConcept(frame.Patient.Phrase, frame.Lang, chem)
	}
	topic := frame.Patient
	if topic.Empty() && (frame.Pro || frame.Alt) {
		topic = d.warmTopic(frame.Alt, frame.ProType)
	}
	if topic.Empty() {
		return d.realizeAs(frame.Lang, grammar.Frame{Force: grammar.ForceStatement, Action: grammar.Term{Phrase: "know"},
			Negate: true, Patient: grammar.Term{Phrase: "what to recall"}}, chem)
	}
	query := topic.Phrase
	if topic.Concept != nil {
		query = topic.Concept.Content
	}
	seeds, outcome, fruitful := d.actRecallNow(query)
	d.finishAct(actRecall, query, outcome, fruitful, "asked")
	if len(seeds) == 0 {
		return d.realizeAs(frame.Lang, grammar.Frame{Force: grammar.ForceStatement, Action: grammar.Term{Phrase: "know"},
			Negate: true, Patient: grammar.Term{Phrase: query}}, chem)
	}
	say := "I remember: " + d.speakFor(seeds[0]) + "."
	if len(seeds) > 1 {
		say += fmt.Sprintf(" It keeps company with %d more moment(s).", len(seeds)-1)
	}
	return say
}

// nameWarmConcept binds a spoken word to the workspace's warm referent,
// the deliberate half of grounding: "remember this as a bell" makes
// bell name whatever is most present. The bond is firm because the
// labeling is explicit, the same standing a mental-teach name gets.
func (d *Daemon) nameWarmConcept(phrase, lang string, chem limbic.State) string {
	c := d.warmConcept(0)
	if c == nil {
		return d.realizeAs(lang, grammar.Frame{Force: grammar.ForceStatement, Action: grammar.Term{Phrase: "know"},
			Negate: true, Patient: grammar.Term{Phrase: "what you mean"}}, chem)
	}
	if lang == "" {
		lang = "en"
	}
	named := false
	for _, form := range grammar.AllTokens(phrase) {
		w, _, err := d.store.WordNode(form, lang, chem, time.Now())
		if err != nil {
			fmt.Fprintf(d.out, "word mint error: %v\n", err)
			continue
		}
		if err := d.store.SetLinkRel(w.ID, c.ID, memory.RelNames, opNameWeight); err != nil {
			fmt.Fprintf(d.out, "naming link error: %v\n", err)
			continue
		}
		d.event(events.Event{Ev: "name", From: w.ID, To: c.ID, Rel: memory.RelNames, Weight: opNameWeight})
		named = true
	}
	if !named {
		return d.realizeAs(lang, grammar.Frame{Force: grammar.ForceStatement, Action: grammar.Term{Phrase: "know"},
			Negate: true, Patient: grammar.Term{Phrase: "what you mean"}}, chem)
	}
	d.ws.excite(c.ID, exciteSpeech)
	d.finishAct(actGround, phrase, "named "+c.Content, true, "asked")
	return d.realizeAs(lang, grammar.Frame{Force: grammar.ForceStatement,
		Agent:   grammar.Term{Phrase: phrase},
		Action:  grammar.Term{Phrase: "name"},
		Patient: grammar.Term{Phrase: c.Content}}, chem)
}
