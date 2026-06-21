package daemon

import (
	"fmt"
	"sort"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

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
	// A deictic pronoun points at a participant, not a thing in the world: from the
	// speaker's side "I" is the user and "you" is GOLEM, so a clause about the
	// speakers grounds in them and is reasoned over like any other.
	if id := d.deicticParticipant(phrase); id != 0 {
		if frags, err := d.store.FragmentsByIDs([]uint64{id}); err == nil && len(frags) == 1 {
			return frags[0]
		}
	}
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
			votedIDs := make([]uint64, 0, len(names))
			for _, e := range names {
				if votes[e.To] == 0 {
					votedIDs = append(votedIDs, e.To)
				}
				votes[e.To] += e.Weight
			}
			tokSet := make(map[string]bool, len(tokens))
			for _, t := range tokens {
				tokSet[t] = true
			}
			frags, err := d.store.FragmentsByIDs(votedIDs)
			if err == nil && len(frags) > 0 {
				// An abstraction is the referent of its OWN word; a word that names
				// an abstraction it does not match holds an inherited member-to-
				// superordinate name (cat names the animal abstraction at 0.8), and
				// refers to its own specific concept, not the category. So such an
				// inherited abstraction is set aside, and the noun resolves to the
				// percept it directly names. A category word still resolves to its
				// abstraction, which it matches. The any-fallback keeps a word that
				// names only an inherited abstraction resolving rather than failing.
				// A basic word names a thing, not a class of it. A percept's
				// basic kind is the head of its caption ("a small black cat" is a
				// cat), so when a candidate percept is basically an L, an
				// "abs:<lang>:L" concept is a basic-level shadow of that percept,
				// not a category above it: heavy company can grow such a shadow
				// from a noun that only ever named one kind. It is set aside the
				// same as an inherited name, so the noun resolves to the percept it
				// labels. A true superordinate ("animal") heads no percept's
				// caption, so its abstraction still wins.
				perceptHead := map[string]bool{}
				for _, f := range frags {
					if _, ok := abstractionLemma(f.Template); ok {
						continue
					}
					if f.Category == "relation" {
						continue // a relation is not a thing with a basic-level name
					}
					if fields := strings.Fields(f.Content); len(fields) > 0 {
						perceptHead[strings.ToLower(fields[len(fields)-1])] = true
					}
				}
				var best, bestAny uint64
				var bestW, bestAnyW float64
				for _, f := range frags {
					w := votes[f.ID]
					if w > bestAnyW || (w == bestAnyW && f.ID > bestAny) {
						bestAny, bestAnyW = f.ID, w
					}
					if lemma, ok := abstractionLemma(f.Template); ok && (!tokSet[lemma] || perceptHead[lemma]) {
						continue
					}
					if w > bestW || (w == bestW && f.ID > best) {
						best, bestW = f.ID, w
					}
				}
				if best == 0 {
					best = bestAny
				}
				for _, f := range frags {
					if f.ID == best {
						return f
					}
				}
			}
		}
	}
	if concepts, err := d.store.ConceptsByGloss(phrase, 1); err == nil && len(concepts) > 0 {
		return concepts[0]
	}
	return nil
}

// abstractionLemma reads the word an abstraction generalizes, the lemma in its
// "abs:<lang>:<lemma>" template, so a resolver can tell whether a word names an
// abstraction as its own (a category word) or only by inheritance from a member
// it grounds. It returns false for a percept concept, which carries no such
// template.
func abstractionLemma(template string) (string, bool) {
	if !strings.HasPrefix(template, "abs:") {
		return "", false
	}
	parts := strings.SplitN(template, ":", 3)
	if len(parts) != 3 || parts[2] == "" {
		return "", false
	}
	return parts[2], true
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
	// The subject the turn explicitly raised holds the field over a neighbour the
	// reply happened to warm: a definition about the cat keeps the cat the topic,
	// not the animal its answer reached for. With no subject named (an open "what
	// else"), the dominant warm concept stands in.
	next := d.turnFocus
	d.turnFocus = 0
	if next == 0 {
		// A turn that raised no subject of its own ("what else", "go on") keeps the
		// one already in play rather than drifting to whatever the reply left warm.
		// Only with no topic yet does the warm field bootstrap one.
		if d.topic != 0 {
			return
		}
		if c := d.warmConcept(0); c != nil {
			next = c.ID
		}
	}
	if next == 0 || next == d.topic {
		return
	}
	if d.topic != 0 {
		// The displaced subject goes onto the trail, newest first and distinct, so
		// the talk can return to what it was on before.
		d.topicTrail = append([]uint64{d.topic}, removeID(d.topicTrail, d.topic)...)
		if len(d.topicTrail) > topicTrailCap {
			d.topicTrail = d.topicTrail[:topicTrailCap]
		}
	}
	d.topicTrail = removeID(d.topicTrail, next) // returning to a trailed subject lifts it off the trail
	d.event(events.Event{Ev: "topic", From: d.topic, To: next})
	d.topic = next
}

// deicticParticipant maps a deictic pronoun to the participant it points at from
// the speaker's side: the one speaking says "I" of the user and "you" of GOLEM,
// "we" of the two together. The mapping is read from the graph (each deixis word
// is linked to its participant concept), so it is a learnable relation, not a
// table in code; it returns 0 for anything not linked.
func (d *Daemon) deicticParticipant(phrase string) uint64 {
	return d.deicticByForm[strings.ToLower(strings.TrimSpace(phrase))]
}

// isParticipant reports whether a concept is one of the innate conversation
// participants (GOLEM, the speaker, the two together). They are concepts GOLEM
// reasons over, but stand apart from the grown perceptual layer.
func (d *Daemon) isParticipant(id uint64) bool {
	return id != 0 && (id == d.selfID || id == d.userID || id == d.bothID)
}

// noteFocus records the concept a turn settled on as its subject, so the topic
// follows what the turn was about rather than what its answer warmed. The answer
// path knows the real subject (a role question's known concept, a statement's
// agent) where the frame's first guess may not, so the path's note wins.
func (d *Daemon) noteFocus(c *memory.Fragment) {
	if c != nil {
		d.turnFocus = c.ID
	}
}

// removeID returns the ids without any occurrence of one, in a fresh slice so the
// input is left untouched.
func removeID(ids []uint64, id uint64) []uint64 {
	out := make([]uint64, 0, len(ids))
	for _, x := range ids {
		if x != id {
			out = append(out, x)
		}
	}
	return out
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
	// The declarative engine realizes the compositional clauses (the copula and the event
	// verbs); the coded Realize remains only for the idioms it declines (know, see, name,
	// propose), whose slots are computed rather than composed.
	if alt, ok := d.gram.RealizeDecl(f, chem); ok {
		return alt
	}
	if d.shadowVerbose {
		fmt.Fprintf(d.out, "framegen decline: %s\n", f.Action.Phrase)
	}
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

	// The subject the turn explicitly names anchors the topic, so it follows what
	// was raised rather than what the reply warmed: the agent is an action's
	// subject, a definition or "what is X" names its subject as the patient. A
	// turn that names neither (an open "what else") leaves the topic to the field.
	d.turnFocus = 0
	if frame.Agent.Concept != nil {
		d.turnFocus = frame.Agent.Concept.ID
	} else if frame.Patient.Concept != nil {
		d.turnFocus = frame.Patient.Concept.ID
	}

	// "And then" is a move on the warm proposition, not a clause: walk its
	// successor and speak what came next.
	if isThenMove(raw) {
		return d.discourseWalk(frame, "after", d.chem.Snapshot()), nil
	}

	// A supposition is entertained, never asserted, so it is caught before the
	// statement path would write its clause as a fact: "suppose the wolf chased
	// the cat" holds the proposition as hypothetical, and "never mind" releases it.
	if ans, ok := d.planSuppose(frame, raw); ok {
		return ans, nil
	}
	if ans, ok := d.planRelease(frame, raw); ok {
		return ans, nil
	}

	// "Go on", "what else": an open invitation to elaborate the topic in mind.
	// GOLEM volunteers a conclusion it has drawn and not yet voiced.
	if ans, ok := d.planContinue(frame, raw); ok {
		return ans, nil
	}

	// "What have I told you": report the propositions the exchange has set down,
	// the discourse memory read back so the talk can be asked what it has covered.
	if ans, ok := d.planRecallSaid(frame); ok {
		return ans, nil
	}

	// "What were we talking about" (or before that): name the subject in mind, or
	// step back to one the trail kept, so a conversation can pick up where it was.
	if ans, ok := d.planTopicRecall(frame, raw); ok {
		return ans, nil
	}

	// "What do you mean": elaborate the subject in mind rather than read the turn
	// as a question of its own.
	if ans, ok := d.planClarify(frame); ok {
		return ans, nil
	}

	// Naming a kind GOLEM has offered ("they are fruit" after it noticed a cluster)
	// grounds it as a category. Fires only while an offer stands, so it does not
	// shadow an ordinary statement.
	if ans, ok := d.planNameKind(frame); ok {
		return ans, nil
	}

	// An elliptical follow-up to a role question inherits its relation and asks
	// it of a new referent: "what about the dog" after "what did the cat chase"
	// walks chase from the dog. The thread is read and cleared here; a role
	// answer below re-sets it, so a further "what about the bird" can follow.
	thread := d.lastRole
	d.lastRole = nil
	if ans, ok := d.completeRoleEllipsis(frame, raw, thread); ok {
		return ans, nil
	}

	// "Tell me about X" is the open request: say what the focus is and what it
	// has been part of, the same walk and renderer the focused questions are
	// filters of, here with no filter.
	if line, ok := d.planDescribe(frame, raw); ok {
		return line, nil
	}

	// "Before is transitive": a rule predicated of a relation, learned as a
	// property of it and read back the same way. It runs before the statement path
	// so the rule is recorded on the relation rather than written as a fact, and
	// before is-a since its subject is a relation, not a member of a category.
	if ans, ok := d.planProperty(frame); ok {
		return ans, nil
	}

	// "Do you know X": a reflexive question about what GOLEM has grounded, read
	// from its tokens because "do" opens an imperative and "know" is a function
	// word, so the clause would otherwise be mis-handled as a command. It runs
	// before the polar and command paths so that misreading never surfaces.
	if ans, ok := d.planEpistemic(raw); ok {
		return ans, nil
	}

	// A yes/no question is checked, not asserted: "did the dog chase the cat"
	// reads whether the graph holds the proposition and answers yes or no. It
	// runs before the force switch so a polar clause is never written as a fact,
	// whether it carried a question mark or not.
	// "Ağacın yaprağı var mı": a Turkish possession question, answered from the
	// stored proposition before the verb-clause paths, which have no "var" reading.
	if ans, ok := d.planPossessionPolarTR(frame, raw); ok {
		return ans, nil
	}
	// "Is the leaf on the tree": a copula question carrying a place, answered from
	// the placed proposition before the is-a reading, which the preposition rules out.
	if ans, ok := d.planLocativePolar(frame); ok {
		return ans, nil
	}
	// "Is a cat an animal": an is-a question, recognized from its comprehended
	// frame (a copular question over two concepts) and settled against the category
	// hierarchy rather than the relation graph a polar question reads.
	if ans, ok := d.planIsA(frame); ok {
		return ans, nil
	}

	// "Who chased what the dog chased": a nested question, the inner clause resolved
	// first and its answer fed into the outer, composing two relational hops.
	if ans, ok := d.planMultiHop(frame, raw); ok {
		return ans, nil
	}

	// "The cat is to the mouse as the dog is to what": a proportion, answered by the
	// relation the first pair shares carried onto the third term.
	if ans, ok := d.planAnalogy(frame); ok {
		return ans, nil
	}

	// "How much bigger is the cat than the mouse": the degree of a comparison, read
	// before the plain comparison since a magnitude clause is also comparative.
	if ans, ok := d.planMagnitude(frame); ok {
		return ans, nil
	}

	// "Is the cat bigger than the mouse": a comparison, settled against the told
	// orderings walked transitively rather than the relation graph a polar reads.
	if ans, ok := d.planComparative(frame); ok {
		return ans, nil
	}
	// The Latin comparison carries its comparative in a word the English-or-Turkish
	// frame parser does not read, so it is settled from the raw clause before the
	// polar path strips its enclitic and reads it as a yes/no.
	if ans, ok := d.planLatinComparison(frame, raw); ok {
		return ans, nil
	}

	if line, ok := d.planPolar(frame, raw); ok {
		return line, nil
	}

	// "No, the dog chased the rabbit": a correction of a claim GOLEM made, not a
	// plain statement. The conflicting prior is retracted and the new fact taught.
	if ans, ok := d.planRepairFact(frame, raw, momentID); ok {
		return ans, nil
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
			// A statement teaches in silence, unless it answered a question left open,
			// in which case GOLEM follows up with the answer it can now give.
			return d.followUp, nil
		}
		// A clause that did not teach, whose opening word names a faculty, is a
		// command the grammar did not recognize as one: its verb is not a seeded
		// imperative opener, so it was read as a subject. Route it to the faculty
		// before the failure is treated as an unknown word. This is what makes a
		// faculty synonym work the moment it names the faculty, with no seed edit.
		if line, handled := d.routeFacultyFallback(frame, raw); handled {
			return line, nil
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
		// A bare numeral is an operand, not a word whose meaning is missing, so
		// the answer to a computation does not trail a question after its
		// digits. Curiosity is for the lexicon.
		if !hasLetter(unknown) {
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

	// A Latin role question carries its interrogative in a word the English-or-Turkish
	// frame parser does not read, so it is answered from the raw clause through its own
	// path, the way the Latin polar question is.
	if line, ok := d.planLatinWh(frame, raw); ok {
		return line
	}
	// "What is on the tree" asks which figures hold a place at the ground: the
	// preposition filters the locative relations so the answer names the things in
	// that place, before the role-question and definition paths that would drop it.
	if line, ok := d.planLocativeWh(frame, chem); ok {
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
	case frame.Wh == "where":
		// "Where is X" reads the figure's place from its locative relations; an
		// event "where did X happen" has no such relation and falls through to the
		// occurrence answer.
		if ans, ok := d.planWhere(frame, chem); ok {
			return ans
		}
		return d.planWhence(frame, quals, chem)
	case frame.Wh == "when":
		return d.planWhence(frame, quals, chem)
	}

	// "What happens after X", "what comes before X", "which command
	// follows X and precedes Y."
	if len(quals) > 0 && !quals[0].Anchor.Empty() {
		return d.planSuccession(frame, quals, chem)
	}

	// A computation asked is computed, not looked up: "what is the sum of 6
	// and 7" comprehends to its meaning, runs in the runtime, and the value is
	// spoken. With the runtime closed or the words not a computation it returns
	// nothing and the definition path below answers.
	if line, ok := d.planComputation(frame, raw); ok {
		return line
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
		d.noteFocus(c) // "what is X" keeps X the topic, not a neighbour the answer reached for
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
		// A definition carries a fresh fact when the exchange just set one down:
		// what the concept did or had done to it this turn is appended, read from
		// the thread so the generator only reports what was established.
		withFact := func(answer string) string {
			if fact, ok := d.threadFactClause(frame.Lang, c); ok {
				return answer + " " + fact
			}
			return answer
		}
		// A firm definition that can only call the concept a generic thing carries
		// a hedged guess at what it is, from the better-known thing it resembles, so
		// "what is a wolf" seen once reaches past "a thing" to "probably a dog". The
		// guess gates itself on resemblance and grounding, a no-op for a firmly-known
		// thing or one without a near neighbour.
		withKind := func(answer string) string {
			if guess, ok := d.kindGuessClause(frame, c, action, tally); ok {
				return answer + " " + guess
			}
			return answer
		}
		// A concept's content is the English caption it distilled from, so the
		// pointer "X names <caption>" cannot be spoken in another tongue without
		// splicing English in. In a non-English answer the definition is firm
		// instead (its kind), so "what is köpek" reads "Köpek bir şey" rather than
		// leaking "a brown dog".
		nonEnglish := frame.Lang != "" && frame.Lang != "en"
		if topic.Phrase != "" && !strings.EqualFold(topic.Phrase, c.Content) && !nonEnglish {
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
			return withFact(strings.Join(parts, " "))
		}
		// A firm definition, composed: the definition clause, and when
		// experience stands behind the concept the count clause nests beside
		// it. The sentence is built from constructions, not a fixed template,
		// and the composition reads back on the trail as the constructions
		// that made it.
		msg := grammar.Message{Intent: "described", Slots: map[string]grammar.Filler{
			"defn": {Sub: &grammar.Message{Intent: "definition",
				Slots: grammar.TextSlots(map[string]string{"subject": d.conceptName(c.ID, c.Content, frame.Lang), "kind": kindWord})}},
		}}
		if n := tally[c.ID]; n > 0 {
			msg.Slots["count"] = grammar.Filler{Sub: &grammar.Message{Intent: "seen_count",
				Slots: grammar.TextSlots(map[string]string{"object": "it", "count": grammar.TimesWord(n)})}}
		}
		if line, ok := d.compose(frame.Lang, msg); ok {
			return withKind(withFact(line))
		}
		// With no constructions the innate realizer assembles the same
		// answer, so the engine stays the floor under the learned patterns.
		parts := []string{d.definitionClause(frame.Lang, d.conceptName(c.ID, c.Content, frame.Lang), kindWord, chem)}
		if n := tally[c.ID]; n > 0 {
			parts = append(parts, d.countClause(frame.Lang, n, chem))
		}
		return withKind(withFact(strings.Join(parts, " ")))
	}
	if topic.Phrase != "" {
		return d.realizeAs(frame.Lang, grammar.Frame{Force: grammar.ForceStatement, Action: grammar.Term{Phrase: "know"},
			Negate: true, Patient: grammar.Term{Phrase: topic.Phrase}}, chem)
	}
	return d.realizeAs(frame.Lang, grammar.Frame{Force: grammar.ForceStatement, Action: grammar.Term{Phrase: "know"},
		Negate: true, Patient: grammar.Term{Phrase: "that"}}, chem)
}

// planComputation answers a computation posed as a question: "what is the sum
// of 6 and 7" has its question lead-in stripped, comprehends to its meaning,
// runs in the runtime, and the value is spoken. It returns false when the words
// carry no number, are not a computation the code faculty claims, or the
// runtime is closed, so "what is a dog" still reaches the definition path and a
// production brain with the runtime shut answers the honest gap.
func (d *Daemon) planComputation(frame grammar.Frame, raw string) (string, bool) {
	if d.code == nil {
		return "", false
	}
	phrase := stripAsk(raw)
	// A computation grounds on number literals; without a digit the words are
	// a definition question and the runtime is never spun.
	if phrase == "" || !strings.ContainsAny(phrase, "0123456789") {
		return "", false
	}
	comp, ok := d.code.Comprehend(phrase)
	if !ok {
		return "", false
	}
	src, ok := d.code.ToCode(comp)
	if !ok {
		return "", false
	}
	res := d.runProgram(src)
	if res.Err != "" || res.Value == "" {
		return "", false
	}
	// The value is spoken back against the phrase asked, and the run itself is
	// already recorded on the trail, so the answer reads as a grounded act and
	// not a stored fact.
	connective := "is"
	if frame.Lang == "tr" {
		connective = "eder"
	}
	return upperFirst(phrase) + " " + connective + " " + res.Value + ".", true
}

// stripAsk removes a leading question lead-in so the computation surface is left
// bare for the code faculty: "what is the sum of 6 and 7" becomes "the sum of 6
// and 7", and "what is 6 + 7" becomes "6 + 7". A trailing question mark is
// dropped with it.
func stripAsk(raw string) string {
	s := strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(raw), "?"))
	low := strings.ToLower(s)
	for _, lead := range []string{"what is the value of ", "what is ", "what's ", "what are ", "whats ", "how much is ", "how many is "} {
		if strings.HasPrefix(low, lead) {
			return strings.TrimSpace(s[len(lead):])
		}
	}
	return s
}

// upperFirst capitalizes a phrase's leading letter so the spoken answer opens
// like a sentence; a phrase beginning with a digit or symbol is left untouched.
// It reads the first rune, so an accented letter a tongue opens a word with (the
// Turkish "üzüm" -> "Üzüm") capitalizes too, not only an ASCII one. The Turkish
// dotted-i casing (i -> İ) is locale-specific and not distinguished here; no
// answer-initial word triggers it today.
func upperFirst(s string) string {
	if s == "" {
		return s
	}
	r, size := utf8.DecodeRuneInString(s)
	up := unicode.ToUpper(r)
	if up == r {
		return s
	}
	return string(up) + s[size:]
}

// lowerFirst lowercases a phrase's leading letter, the inverse of upperFirst, so
// a clause the engine already capitalized can follow an affirmation word inside
// one sentence: "Yes, the dog ..." carries "The dog ..." as its tail.
func lowerFirst(s string) string {
	if s == "" {
		return s
	}
	// English "I" stays capitalized even mid-sentence, so a clause opening with the
	// first-person pronoun is left as it stands ("Yes, I chased the cat").
	if s == "I" || strings.HasPrefix(s, "I ") || strings.HasPrefix(s, "I'") {
		return s
	}
	r, size := utf8.DecodeRuneInString(s)
	low := unicode.ToLower(r)
	if low == r {
		return s
	}
	return string(low) + s[size:]
}

// hasLetter reports whether a token carries any letter, the mark of a word that
// could have a meaning to learn, as opposed to a bare numeral or symbol.
func hasLetter(s string) bool {
	for _, r := range s {
		if unicode.IsLetter(r) {
			return true
		}
	}
	return false
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
	case "recall", "consolidate", "introspect":
		return d.runBuiltinCommand(frame.Action.Phrase, frame, chem), nil
	}
	// A reworded or translated imperative reaches the same faculty by resemblance,
	// before the grounded-command path tries to read it as a taught command.
	if line, handled := d.routeBuiltinCommand(frame, raw, chem); handled {
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
