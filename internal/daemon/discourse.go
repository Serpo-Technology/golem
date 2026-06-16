package daemon

import (
	"strings"

	"golem/internal/grammar"
	"golem/internal/memory"
)

// The discourse layer holds the thread of an exchange as conceptual structure,
// not as text. A role question leaves its relation in mind, so an elliptical
// follow-up can ask the same thing of a new referent: "what did the cat chase"
// then "what about the dog" is one thread, the relation carried over and only
// the referent swapped. The relation and role are reasoned over the graph; no
// words from the first turn are reused, so the thread holds across a switch of
// tongue.

// roleContext is the thread a role question leaves behind: the relation asked
// about, which role the named argument played, and the verb that named it.
type roleContext struct {
	rel       *memory.Fragment
	knownRole string
	verb      string
	past      bool
}

// threadProp is a proposition the current exchange has set down: a relation
// between an agent and a patient. Recall leans toward it, so a fresh claim wins
// the conversation without rewriting the long-term weight it would take many
// tellings to earn. It is how the talk prefers what was just said over what is
// worn deepest into the graph.
type threadProp struct {
	rel          string
	agent        uint64
	patient      uint64
	agentGloss   string
	patientGloss string
}

const threadPropCap = 16

// topicTrailCap bounds how many displaced subjects the talk keeps to return to,
// so a long conversation can step back a few topics without hoarding every one.
const topicTrailCap = 8

// claim is a proposition GOLEM has asserted this exchange and how firmly: whether
// it said the proposition holds (affirmed) or not (denied), and whether it said so
// firmly ("Yes") or as a guess ("Probably"). It is the speaker's-side record of
// the discourse, so GOLEM can stand by what it committed to.
type claim struct {
	rel            string
	agent, patient uint64
	affirmed       bool
	hedged         bool
}

const claimsCap = 16

// recallClaim returns GOLEM's standing claim about a proposition this exchange, or
// nil when it has made none, so a fresh assertion can be checked against it.
func (d *Daemon) recallClaim(rel string, agent, patient uint64) *claim {
	for i := range d.claims {
		if c := &d.claims[i]; c.rel == rel && c.agent == agent && c.patient == patient {
			return c
		}
	}
	return nil
}

// recordClaim sets down what GOLEM has just asserted, one standing entry per
// proposition (the latest commitment), newest first and capped, so a re-assertion
// updates rather than piles up.
func (d *Daemon) recordClaim(rel string, agent, patient uint64, affirmed, hedged bool) {
	if rel == "" || agent == 0 || patient == 0 {
		return
	}
	if c := d.recallClaim(rel, agent, patient); c != nil {
		c.affirmed, c.hedged = affirmed, hedged
		return
	}
	d.claims = append([]claim{{rel: rel, agent: agent, patient: patient, affirmed: affirmed, hedged: hedged}}, d.claims...)
	if len(d.claims) > claimsCap {
		d.claims = d.claims[:claimsCap]
	}
}

// claimFacts records the propositions a fact answer asserts as firm affirmations,
// and reports whether GOLEM had already claimed them all firmly this exchange, so
// a re-asked question can be noted as already answered. A fact that only firms up
// a prior guess is not "as I said": it is an upgrade.
func (d *Daemon) claimFacts(focus *memory.Fragment, facts []focusFact) (allPrior bool) {
	allPrior = len(facts) > 0
	for _, f := range facts {
		agent, patient := focus.ID, f.coID
		if f.focusRole == "patient" {
			agent, patient = f.coID, focus.ID
		}
		if prior := d.recallClaim(f.rel, agent, patient); prior == nil || !prior.affirmed || prior.hedged {
			allPrior = false
		}
		d.recordClaim(f.rel, agent, patient, true, false)
	}
	return allPrior
}

// ackIfSaid prefixes an answer with the tongue's "as I said" when GOLEM is
// repeating a claim it already made, so a re-asked question is noted rather than
// replayed cold. With no such word seeded, the answer stands plain.
func (d *Daemon) ackIfSaid(lang, line string, said bool) string {
	if said {
		if as := d.gram.Word(lang, "asisaid"); as != "" {
			return upperFirst(as) + ", " + lowerFirst(line)
		}
	}
	return line
}

// recordThreadProp remembers a proposition the exchange just established, newest
// first, keeping only the last few so an old turn does not haunt a new one. The
// glosses are kept too, so the answer can be spoken from the thread even when
// the walk has dropped the proposition.
func (d *Daemon) recordThreadProp(rel string, agent, patient uint64, agentGloss, patientGloss string) {
	if rel == "" || agent == 0 || patient == 0 {
		return
	}
	d.threadProps = append([]threadProp{{rel: rel, agent: agent, patient: patient, agentGloss: agentGloss, patientGloss: patientGloss}}, d.threadProps...)
	if len(d.threadProps) > threadPropCap {
		d.threadProps = d.threadProps[:threadPropCap]
	}
}

// The generator answers by walking the graph and the thread for what is salient
// about a focus concept, then speaking the propositions it gathers. One walk
// feeds every answer: a role question filters it to one relation, an activity
// question takes them all, a definition leads with the identity and adds a fresh
// fact, an open "tell me about X" applies no filter. The walk is a chain over
// meaning, not words, so the answer is composed from grounded propositions and
// rendered at the language edge, never authored.

// focusFact is one grounded thing about a focus concept: a relation it takes
// part in, the role the focus plays in it, and the co-argument filling the
// other role. The thread and the walk both yield these.
type focusFact struct {
	rel       string
	focusRole string // "agent" or "patient": the role the focus plays
	coGloss   string
	coID      uint64
}

const (
	factCoArgCap  = 4 // co-arguments coordinated within one clause
	factClauseCap = 3 // relation clauses joined in one answer
)

// conceptName is a concept's speakable face in a tongue: the firmest word that
// names it there, so an answer says "the dog" or, in Turkish, "köpek" rather
// than the raw percept caption the concept distilled from. It falls back to the
// concept's own content when no word of the tongue names it, so a concept known
// only by its shape still speaks.
func (d *Daemon) conceptName(id uint64, content, lang string) string {
	edges, err := d.store.LinksToRel([]uint64{id}, memory.RelNames)
	if err != nil || len(edges) == 0 {
		return content
	}
	weight := map[uint64]float64{}
	for _, e := range edges {
		if e.Weight > weight[e.From] {
			weight[e.From] = e.Weight
		}
	}
	ids := make([]uint64, 0, len(weight))
	for wid := range weight {
		ids = append(ids, wid)
	}
	words, err := d.store.FragmentsByIDs(ids)
	if err != nil {
		return content
	}
	best, bestW := "", 0.0
	ownW := -1.0
	for _, w := range words {
		if w.Kind != memory.KindWord || w.Lang != lang {
			continue
		}
		cand := weight[w.ID]
		if w.Content == content {
			ownW = cand // the concept's own name, the lemma of a category
		}
		if best == "" || cand > bestW {
			best, bestW = w.Content, cand
		}
	}
	// A concept answers to its own name. A superordinate is named by its own word
	// and, through inheritance, by its dominant member's word, which heavy use can
	// grow past the category's own ("cat" naming the animal abstraction above
	// "animal"); glossing by the member would speak the category as one of its
	// members. So when the concept's own name names it firmly, at least half as
	// firmly as its firmest namer, it speaks by that. A percept's caption is no
	// single word, so this only ever favors a category's lemma, never a basic name.
	if ownW >= 0 && ownW >= 0.5*bestW {
		return content
	}
	if best == "" {
		return content
	}
	return best
}

// relationName is a relation's verb in a tongue: the firmest word naming the
// relation there, found from the relation's content, so a Turkish answer
// conjugates "kovala" where an English one conjugates "chase". It falls back to
// the content when no name in the tongue is found.
func (d *Daemon) relationName(content, lang string) string {
	rel := d.resolveRelation(content)
	if rel == nil {
		return content
	}
	return d.conceptName(rel.ID, content, lang)
}

// gatherFacts collects what is salient about a focus: the propositions it takes
// part in, freshly established ones from the discourse thread winning over the
// entrenched memory. role filters to the role the focus plays ("" for either),
// rel to one relation ("" for any). A relation the thread spoke of this
// exchange is reported from the thread and not diluted by the entrenched
// crossings; a relation only the long-term graph holds is read from the walk.
// The result is salience-ordered, thread first, and deduped by relation, role,
// and co-argument.
func (d *Daemon) gatherFacts(focus *memory.Fragment, role, rel string) []focusFact {
	wantRole := func(r string) bool { return role == "" || role == r }
	wantRel := func(r string) bool { return rel == "" || rel == r }
	type key struct {
		rel, role string
		id        uint64
	}
	seen := map[key]bool{}
	var out []focusFact
	threadCovered := map[string]bool{} // rel|role the thread has spoken of
	add := func(rl, frole, gloss string, id uint64) {
		if id == 0 || rl == "" || !wantRole(frole) || !wantRel(rl) {
			return
		}
		k := key{rl, frole, id}
		if seen[k] {
			return
		}
		seen[k] = true
		out = append(out, focusFact{rel: rl, focusRole: frole, coGloss: gloss, coID: id})
	}
	for _, p := range d.threadProps {
		if p.agent == focus.ID {
			add(p.rel, "agent", p.patientGloss, p.patient)
			threadCovered[p.rel+"|agent"] = true
		}
		if p.patient == focus.ID {
			add(p.rel, "patient", p.agentGloss, p.agent)
			threadCovered[p.rel+"|patient"] = true
		}
	}
	dests, err := d.walkableDests(focus.ID)
	if err != nil {
		return out
	}
	for _, dst := range dests[memory.RelPatient] { // the focus as agent, reaching its patients
		if !threadCovered[dst.via+"|agent"] {
			add(dst.via, "agent", dst.gloss, dst.id)
		}
	}
	for _, dst := range dests[memory.RelAgent] { // the focus as patient, reaching its agents
		if !threadCovered[dst.via+"|patient"] {
			add(dst.via, "patient", dst.gloss, dst.id)
		}
	}
	return out
}

// renderFacts speaks a gathered set as coordinated clauses: the co-arguments of
// one relation join in a single clause ("chased the cat and the mouse"), and
// clauses of different relations follow in turn. A repeated subject is carried
// as a pronoun in English ("it") and as the name again in tongues whose pronoun
// case this generator does not yet inflect, so a Turkish answer reads cleanly
// rather than splicing in an English "it". continuation means the focus was
// already named just before (after a definition), so the first clause carries
// it too. The focus fills its role and the co-arguments the other; the caps
// keep an answer from running on. It returns false when nothing renders.
func (d *Daemon) renderFacts(lang, focusName string, facts []focusFact, past, continuation bool) (string, bool) {
	type key struct{ rel, role string }
	var order []key
	coargs := map[key][]string{}
	for _, f := range facts {
		k := key{f.rel, f.focusRole}
		if _, ok := coargs[k]; !ok {
			if len(order) >= factClauseCap {
				continue
			}
			order = append(order, k)
		}
		if len(coargs[k]) < factCoArgCap {
			// Two concepts a split left under one name, or one concept two tongues
			// name alike, would otherwise list the name twice ("the cat and the
			// cat"). Coordinate each distinct name once.
			name := d.conceptName(f.coID, f.coGloss, lang)
			seen := false
			for _, had := range coargs[k] {
				if had == name {
					seen = true
					break
				}
			}
			if !seen {
				coargs[k] = append(coargs[k], name)
			}
		}
	}
	var clauses []string
	for _, k := range order {
		names := coargs[k]
		// The meaning is language-free: the focus and the coordinated
		// co-arguments are named, marked definite or given, and the engine
		// renders the article, coordinator, and pronoun from the tongue's data.
		co := grammar.Term{Phrase: names[0], With: names[1:], Definite: true}
		focus := grammar.Term{Phrase: focusName, Definite: true}
		if continuation || len(clauses) > 0 {
			focus = grammar.Term{Given: true} // already in play; the tongue's pronoun
		}
		roles := map[string]grammar.Term{}
		if k.role == "agent" {
			roles["agent"], roles["patient"] = focus, co
		} else {
			roles["agent"], roles["patient"] = co, focus
		}
		if line, ok := d.gram.Speak(lang, grammar.Meaning{Relation: grammar.Term{Phrase: d.relationName(k.rel, lang)}, Roles: roles, Past: past}); ok {
			clauses = append(clauses, line)
		}
	}
	if len(clauses) == 0 {
		return "", false
	}
	return strings.Join(clauses, " "), true
}

// threadFactClause speaks what the exchange just established about a concept, a
// proposition it took part in, so a definition can carry a fresh fact: asked
// what a dog is right after "a dog chased a cat", the answer adds "It chased a
// black cat". The concept fills its role as "it"; the co-argument keeps its
// gloss. The fact is read from the discourse thread, never composed, so the
// generator reports only what was set down. It returns false when the thread
// holds nothing about the concept.
func (d *Daemon) threadFactClause(lang string, c *memory.Fragment) (string, bool) {
	var fresh []focusFact
	for _, p := range d.threadProps {
		switch c.ID {
		case p.agent:
			fresh = append(fresh, focusFact{rel: p.rel, focusRole: "agent", coGloss: p.patientGloss, coID: p.patient})
		case p.patient:
			fresh = append(fresh, focusFact{rel: p.rel, focusRole: "patient", coGloss: p.agentGloss, coID: p.agent})
		}
		if len(fresh) > 0 {
			break // the freshest fact only, spoken as one clause
		}
	}
	// A continuation of the definition that just named the concept, so it is
	// carried as a pronoun (English) or named again (other tongues).
	return d.renderFacts(lang, d.conceptName(c.ID, c.Content, lang), fresh, true, true)
}

// planRecallSaid answers "what have I told you": GOLEM reports the propositions
// the exchange has set down, grouped by their agent and spoken in the order they
// were established, so the talk can be asked what it has covered. It reads the
// discourse memory, never the entrenched graph, so it reports only what was said
// this exchange. It returns false when the turn is not that question or nothing
// has been established yet, so an empty exchange falls through to the older path.
func (d *Daemon) planRecallSaid(frame grammar.Frame, raw string) (string, bool) {
	if !isRecallSaid(raw) || len(d.threadProps) == 0 {
		return "", false
	}
	// Oldest first, so the recall reads in the order the exchange built it.
	var agents []uint64
	seen := map[uint64]bool{}
	for i := len(d.threadProps) - 1; i >= 0; i-- {
		if a := d.threadProps[i].agent; !seen[a] {
			seen[a] = true
			agents = append(agents, a)
		}
	}
	var clauses []string
	for _, a := range agents {
		var facts []focusFact
		gloss := ""
		for i := len(d.threadProps) - 1; i >= 0; i-- {
			if p := d.threadProps[i]; p.agent == a {
				facts = append(facts, focusFact{rel: p.rel, focusRole: "agent", coGloss: p.patientGloss, coID: p.patient})
				gloss = p.agentGloss
			}
		}
		if line, ok := d.renderFacts(frame.Lang, d.conceptName(a, gloss, frame.Lang), facts, true, false); ok {
			clauses = append(clauses, line)
		}
	}
	if len(clauses) == 0 {
		return "", false
	}
	return strings.Join(clauses, " "), true
}

// isRecallSaid reports whether a turn asks GOLEM to report back what it has been
// told this exchange, distinct from "what do you know about X", which describes a
// concept from the whole graph.
func isRecallSaid(raw string) bool {
	s := strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(strings.ToLower(raw)), "?"))
	for _, p := range []string{
		"what have i told you", "what did i tell you", "what have i told you so far",
		"what have i said", "what did i say", "what do you remember",
	} {
		if s == p {
			return true
		}
	}
	return false
}

// completeRoleEllipsis answers an elliptical follow-up to a role question by
// inheriting its relation and asking it of a new referent: after "what did the
// cat chase", "what about the dog" walks chase from the dog. The relation and
// role come from the thread, the referent from the follow-up. It returns false
// when there is no role thread, the turn is not such a follow-up, or the
// referent names nothing GOLEM knows.
func (d *Daemon) completeRoleEllipsis(frame grammar.Frame, raw string, thread *roleContext) (string, bool) {
	if thread == nil {
		return "", false
	}
	ref, ok := aboutReferent(raw)
	if !ok {
		return "", false
	}
	known := d.resolveConcept(ref)
	if known == nil {
		return "", false
	}
	// The tense is part of the inherited question: "what did the cat chase" was
	// past, so the follow-up about the dog is past too.
	frame.Past = frame.Past || thread.past
	return d.answerRole(frame, thread.rel, known, thread.knownRole, thread.verb)
}

// polarClause reads a yes/no question into the bare clause it asks about and
// whether the utterance is one. English leads with an auxiliary ("did the dog
// chase the cat"), Turkish closes with the question particle ("köpek kediyi
// kovaladı mı"). The marker is removed so the rest comprehends as a proposition
// to check against the graph; pastAux carries the English auxiliary's tense, the
// Turkish tense riding on the verb itself.
func polarClause(lang, raw string) (clause string, pastAux, ok bool) {
	s := strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(raw), "?"))
	tokens := strings.Fields(s)
	if len(tokens) < 3 {
		return "", false, false
	}
	if lang == "tr" {
		switch strings.ToLower(tokens[len(tokens)-1]) {
		case "mı", "mi", "mu", "mü":
			return strings.Join(tokens[:len(tokens)-1], " "), false, true
		}
		return "", false, false
	}
	switch strings.ToLower(tokens[0]) {
	case "did":
		return strings.Join(tokens[1:], " "), true, true
	case "does":
		return strings.Join(tokens[1:], " "), false, true
	}
	return "", false, false
}

// describeReferent reads the focus of an open request to say what is known
// about something, the noun in "tell me about the dog" or "describe a cat",
// with any article dropped. It returns false when the turn is not such a
// request.
func describeReferent(raw string) (string, bool) {
	s := strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(strings.ToLower(raw)), "?"))
	for _, lead := range []string{"tell me about ", "tell me of ", "describe ", "what can you tell me about ", "what do you know about "} {
		rest, ok := strings.CutPrefix(s, lead)
		if !ok {
			continue
		}
		rest = strings.TrimSpace(rest)
		for _, art := range []string{"a ", "an ", "the "} {
			rest = strings.TrimPrefix(rest, art)
		}
		if rest = strings.TrimSpace(rest); rest != "" {
			return rest, true
		}
	}
	return "", false
}

// isTopicRecall reports whether a turn asks what the talk is on (back=false) or
// what it was on before the current subject (back=true). The "before" forms are
// matched first, so "what were we talking about before" steps back rather than
// naming the current subject.
func isTopicRecall(raw string) (back bool, ok bool) {
	s := strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(strings.ToLower(raw)), "?"))
	for _, p := range []string{
		"what were we talking about before", "what were we talking about before that",
		"what were we discussing before", "what about before that", "and before that", "before that",
	} {
		if s == p {
			return true, true
		}
	}
	for _, p := range []string{
		"what were we talking about", "what are we talking about",
		"what were we discussing", "what are we discussing", "what is the topic", "what's the topic",
	} {
		if s == p {
			return false, true
		}
	}
	return false, false
}

// aboutReferent reads the referent of an elliptical follow-up, the noun in "what
// about a dog" or "how about the cat", with any article dropped. It returns
// false when the turn is not one of those continuations.
func aboutReferent(raw string) (string, bool) {
	s := strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(strings.ToLower(raw)), "?"))
	for _, lead := range []string{"what about ", "how about ", "and what about ", "and how about "} {
		rest, ok := strings.CutPrefix(s, lead)
		if !ok {
			continue
		}
		rest = strings.TrimSpace(rest)
		for _, art := range []string{"a ", "an ", "the "} {
			rest = strings.TrimPrefix(rest, art)
		}
		if rest = strings.TrimSpace(rest); rest != "" {
			return rest, true
		}
	}
	return "", false
}
