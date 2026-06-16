package daemon

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"golem/internal/events"
	"golem/internal/grammar"
	"golem/internal/memory"
)

// Comprehension carries meaning inward the way perception does: a told
// statement that parses as a proposition through the argument-structure
// constructions is written into the graph, the relation grounded in the
// moment of its telling and the moment pointing at what filled each role.
// So "a dog chased a cat" leaves a chase the brain can be asked about, and
// the walk that crosses a relation has a proposition to cross. This is the
// producer reified relations were left waiting on.
const comprehendWeight = 0.8

// comprehendStatement runs the construction engine over a told statement and
// writes the proposition it yields. It returns true when one was written, so
// the planner knows the utterance was understood as an assertion rather than
// left as a gap to ask about. A statement that fits no argument construction,
// or whose subject grounds in nothing, writes nothing and returns false, so
// the planner falls back to its older handling.
func (d *Daemon) comprehendStatement(frame grammar.Frame, raw string, momentID uint64) bool {
	if momentID == 0 {
		return false
	}
	m, ok := d.gram.Comprehend(frame.Lang, raw, d.resolveConcept, d.roleFit)
	if !ok {
		return false
	}
	agent := m.Roles["agent"]
	if agent.Concept == nil {
		return false // the subject grounds in nothing; not yet a proposition to write
	}
	rel, err := d.relationConcept(m.Relation, frame.Lang)
	if err != nil || rel == nil {
		fmt.Fprintf(d.out, "relation mint error: %v\n", err)
		return false
	}
	var patientID uint64
	var patientGloss string
	if patient := m.Roles["patient"]; patient.Concept != nil {
		patientID = patient.Concept.ID
		patientGloss = patient.Concept.Content
	}
	if err := d.store.Propose(rel.ID, momentID, agent.Concept.ID, patientID, comprehendWeight); err != nil {
		fmt.Fprintf(d.out, "propose error: %v\n", err)
		return false
	}
	d.noteFocus(agent.Concept) // a told proposition leaves its subject as the topic
	// The proposition is in the graph now; the exchange remembers it set this
	// down, so a later "what about X" leans on what was just said.
	if patientID != 0 {
		d.recordThreadProp(rel.Content, agent.Concept.ID, patientID, agent.Concept.Content, patientGloss)
		// What was a guess is now seen: retire any inferred belief about the same
		// proposition, in mind and in long-term memory, so observation supersedes
		// inference rather than standing beside it.
		d.held.forget(rel.Content, agent.Concept.ID, patientID)
		if err := d.store.ForgetBelief(rel.Content, agent.Concept.ID, patientID); err != nil {
			fmt.Fprintf(d.out, "belief retire error: %v\n", err)
		}
	}
	// Warm what it bound, recording the role each filler played so the discourse
	// set can read its lean, and put the role edges on the trail so the write is
	// auditable like a naming is.
	d.ws.excite(rel.ID, exciteSpeech)
	d.ws.exciteRole(agent.Concept.ID, exciteSpeech, "agent")
	d.event(events.Event{Ev: "link", From: momentID, To: agent.Concept.ID, Rel: memory.RelAgent, Weight: comprehendWeight})
	if patientID != 0 {
		d.ws.exciteRole(patientID, exciteSpeech, "patient")
		d.event(events.Event{Ev: "link", From: momentID, To: patientID, Rel: memory.RelPatient, Weight: comprehendWeight})
	}

	// A gap in what was bound is a learning signal, not a failure: a role
	// whose filler grounds in nothing is held as a goal so the drives hunt
	// it. Comprehension would otherwise swallow the gap, having understood
	// the proposition it sat in, so the signal the older curiosity path
	// raised is preserved here, behind the same curiosity gate.
	if d.chem.Snapshot().Dopamine >= curiosityGate {
		for _, role := range []string{"agent", "patient"} {
			if t := m.Roles[role]; t.Concept == nil && t.Phrase != "" {
				d.adoptGoal("learn", t.Phrase)
				break
			}
		}
	}
	return true
}

// momentBoundRoles reports whether a moment carries the role edges of a
// proposition, the agent comprehension bound to it. It tells the tell handler
// the utterance was understood as a proposition, whose words ground in their
// roles rather than in what was merely warm beside them.
func (d *Daemon) momentBoundRoles(momentID uint64) bool {
	edges, err := d.store.LinksFromRel([]uint64{momentID}, memory.RelAgent)
	return err == nil && len(edges) > 0
}

// planRoleQuestion answers a question about a role of a relation by walking
// the proposition graph and generating the answer through the engine, rather
// than selecting a reply from a fixed menu. "What did the dog chase" asks for
// the patient of a chase the dog was the agent of; "who chased the cat" asks
// for the agent of a chase whose patient was the cat. The walk crosses the
// relation to the co-argument, and the construction speaks the proposition
// back, the same construction comprehension read forward. It returns false
// when the question is not a relational role question, so the planner's older
// handling stands.
func (d *Daemon) planRoleQuestion(frame grammar.Frame, raw string) (string, bool) {
	if frame.Wh != "what" && frame.Wh != "who" && frame.Wh != "which" {
		return "", false
	}
	rel, known, knownRole, verb := d.analyzeRoleQuestion(frame.Lang, raw)
	if rel == nil || known == nil {
		return "", false
	}
	return d.answerRole(frame, rel, known, knownRole, verb)
}

// answerRole walks a relation from the named argument to the questioned role and
// speaks the proposition, the shared body of a role question asked in full and
// one inherited by an elliptical follow-up. It leaves the relation as the
// discourse thread so "what about X" can ask the same of another referent.
func (d *Daemon) answerRole(frame grammar.Frame, rel, known *memory.Fragment, knownRole, verb string) (string, bool) {
	d.noteFocus(known) // the asked-about concept is the turn's subject, the topic to keep
	past := frame.Past || d.gram.VerbPast(frame.Lang, verb)
	// The relation and its tense are the thread an elliptical follow-up inherits.
	d.lastRole = &roleContext{rel: rel, knownRole: knownRole, verb: verb, past: past}
	// The answer is the focus's propositions for this relation, fresh thread
	// claims winning over the entrenched memory and the co-arguments coordinated
	// into one clause: "the dog chased the cat and the mouse". The same walk and
	// renderer every answer uses, here filtered to the one relation asked.
	facts := d.gatherFacts(known, knownRole, rel.Content)
	line, ok := d.renderFacts(frame.Lang, d.conceptName(known.ID, known.Content, frame.Lang), facts, past, false)
	if !ok {
		// Nothing observed filled the questioned role. A supposition in play is
		// reasoned under first (the conditional answer), then a hedged guess from a
		// near neighbour that did stand in the relation, before the gap is admitted.
		if supposed, ok := d.heldRoleGuess(frame, known, knownRole, rel.Content, past, false); ok {
			d.warmSpoken(known.Content)
			return supposed, true
		}
		if guess, ok := d.inferRole(frame, known, knownRole, rel.Content, past, false); ok {
			d.warmSpoken(known.Content)
			return guess, true
		}
		// The relation is known but nothing filled the questioned role: the
		// honest answer is the gap, spoken through the older admission.
		return d.realizeAs(frame.Lang, grammar.Frame{Force: grammar.ForceStatement, Action: grammar.Term{Phrase: "know"},
			Negate: true, Past: past, Patient: grammar.Term{Phrase: frame.Wh}}, d.chem.Snapshot()), true
	}
	for _, f := range facts {
		d.ws.excite(f.coID, exciteSpeech)
	}
	d.warmSpoken(known.Content)
	d.claimFacts(known, facts) // record what this answer asserts, so a later question stays consistent with it
	// A supposition that adds to what was observed trails the observed clause, the
	// focus carried as a pronoun and the conditional lead marking it contingent.
	if supposed, ok := d.heldRoleGuess(frame, known, knownRole, rel.Content, past, true); ok {
		return line + " " + supposed, true
	}
	return line, true
}

// planActivityQuestion answers "what did the dog do": the relation itself is
// asked, not a role of a named one. The agent is given and the verb is the
// generic pro-verb, so the walk reads the strongest proposition the agent
// took part in off its role edges, and the engine speaks the whole clause
// back, "the dog chased a cat," from a meaning structure rather than a fixed
// reply. It returns false when the clause is not of this shape, so the older
// handling stands.
func (d *Daemon) planActivityQuestion(frame grammar.Frame, raw string) (string, bool) {
	if frame.Wh != "what" || !d.proVerbDo(frame.Lang, raw) {
		return "", false
	}
	agent := d.activityAgent(frame, raw)
	if agent == nil {
		return "", false
	}
	d.noteFocus(agent) // "what did X do" keeps X the topic
	// The tense is the pro-verb's: "what did the dog do" is past, "what does the
	// dog do" present, read from the last word's tense so a Turkish "ne yaptı"
	// answers in the past where "ne yapar" would not.
	past := frame.Past
	if tokens, _ := grammar.Tokenize(raw); len(tokens) > 0 {
		past = past || d.gram.VerbPast(frame.Lang, tokens[len(tokens)-1])
	}
	// "What did the dog do" reports the dog's activities across the relations it
	// took part in: the same gather and renderer, filtered to the agent role with
	// no single relation, so each relation becomes a clause and the agent is
	// carried as "it" after the first.
	facts := d.gatherFacts(agent, "agent", "")
	line, ok := d.renderFacts(frame.Lang, d.conceptName(agent.ID, agent.Content, frame.Lang), facts, past, false)
	if !ok {
		// No observed activity. Reason under a supposition in play, then offer a
		// hedged guess from a near neighbour, before falling through.
		if supposed, ok := d.heldRoleGuess(frame, agent, "agent", "", past, false); ok {
			d.warmSpoken(agent.Content)
			return supposed, true
		}
		if guess, ok := d.inferRole(frame, agent, "agent", "", past, false); ok {
			d.warmSpoken(agent.Content)
			return guess, true
		}
		return "", false
	}
	for _, f := range facts {
		d.ws.excite(f.coID, exciteSpeech)
	}
	d.warmSpoken(agent.Content)
	d.claimFacts(agent, facts) // record what this answer asserts, for consistency with later questions
	if supposed, ok := d.heldRoleGuess(frame, agent, "agent", "", past, true); ok {
		return line + " " + supposed, true
	}
	return line, true
}

// planDescribe answers an open "tell me about X": it says what the focus is and
// then what is salient about it, its identity followed by the propositions it
// took part in, gathered by the one walk with no filter and spoken through the
// one renderer. It is the general case the role and activity questions are
// filters of. It returns false when the turn is not such a request or the focus
// names nothing GOLEM knows.
func (d *Daemon) planDescribe(frame grammar.Frame, raw string) (string, bool) {
	ref, ok := describeReferent(raw)
	if !ok {
		return "", false
	}
	focus := d.resolveConcept(ref)
	if focus == nil {
		return "", false
	}
	d.turnFocus = focus.ID // "tell me about X" makes X the subject, resolved apart from the frame
	return d.describeConcept(frame, focus), true
}

// describeConcept says what a concept is and what it has been part of: its kind,
// then the propositions it stands in (the thread winning over the entrenched
// graph), or a hedged guess from a near neighbour when nothing is observed. It is
// the renderer behind "tell me about X" and a recall of the subject in mind.
func (d *Daemon) describeConcept(frame grammar.Frame, focus *memory.Fragment) string {
	chem := d.chem.Snapshot()
	kind := "thing"
	if focus.Category == "verb" || focus.Category == "relation" {
		kind = "action"
	}
	parts := []string{d.definitionClause(frame.Lang, d.conceptName(focus.ID, focus.Content, frame.Lang), grammar.Indefinite(kind), chem)}
	if facts := d.gatherFacts(focus, "", ""); len(facts) > 0 {
		if line, ok := d.renderFacts(frame.Lang, d.conceptName(focus.ID, focus.Content, frame.Lang), facts, true, true); ok {
			parts = append(parts, line)
		}
		for _, f := range facts {
			d.ws.excite(f.coID, exciteSpeech)
		}
		d.claimFacts(focus, facts) // a description asserts these facts too; record them
	} else if guess, ok := d.inferRole(frame, focus, "agent", "", true, true); ok {
		// Nothing observed about the focus: offer what a near neighbour does as a
		// hedged guess at what this one likely does, the property counterpart of
		// the kind guess "what is X" makes. The focus was just named, so the guess
		// carries it as a pronoun.
		parts = append(parts, guess)
	}
	d.warmSpoken(focus.Content)
	return strings.Join(parts, " ")
}

// planTopicRecall answers a turn that asks what the talk is on, or was on before:
// "what were we talking about" describes the subject in mind; "what about before
// that" steps back to the subject the trail kept and returns to it, so a
// conversation can pick up a thread it set down. It returns false when the turn is
// not such a question, or there is no subject (or none to step back to).
func (d *Daemon) planTopicRecall(frame grammar.Frame, raw string) (string, bool) {
	back, ok := isTopicRecall(raw)
	if !ok {
		return "", false
	}
	var id uint64
	if back {
		if len(d.topicTrail) == 0 {
			return "", false
		}
		id = d.topicTrail[0]
		d.turnFocus = id // stepping back makes the prior subject the one in play again
	} else {
		id = d.topic
	}
	if id == 0 {
		return "", false
	}
	frags, err := d.store.FragmentsByIDs([]uint64{id})
	if err != nil || len(frags) != 1 {
		return "", false
	}
	return d.describeConcept(frame, frags[0]), true
}

// planPolar answers a yes/no question by checking the proposition it states
// against the graph rather than asserting it. "Did the dog chase the cat" and
// "köpek kediyi kovaladı mı" each strip their question marker, comprehend the
// bare clause into a proposition, and read whether the graph holds it: the
// answer affirms or denies through the same construction, "Yes, the dog chased
// the cat" or "Hayır, köpek kediyi kovalamadı", the denial running the negative
// morphology a "no" needs. It returns false when the utterance is not a polar
// question or its proposition grounds in nothing, so the older handling stands
// and nothing is written as a fact.
func (d *Daemon) planPolar(frame grammar.Frame, raw string) (string, bool) {
	clause, pastAux, ok := polarClause(frame.Lang, raw)
	if !ok {
		return "", false
	}
	// The shape is a yes/no question, so from here it is always answered, never
	// passed on to be written as a fact: a question that fell through to the
	// statement path would assert its own words, and a marker like the Turkish
	// "mı" stripped above would be minted as a noun. When the clause cannot be
	// resolved into a known proposition, the honest answer is the gap.
	gap := func() (string, bool) {
		return d.realizeAs(frame.Lang, grammar.Frame{Force: grammar.ForceStatement,
			Action: grammar.Term{Phrase: "know"}, Negate: true}, d.chem.Snapshot()), true
	}
	// A polar question asks about a directed proposition: "did the cat chase the
	// mouse" is a question about the cat chasing the mouse, not the mouse chasing
	// the cat. The asker's word order fixes which referent is agent, so role-fit
	// is given no say here; letting it reorder by learned animacy would answer a
	// different question than the one asked.
	m, ok := d.gram.Comprehend(frame.Lang, clause, d.resolveConcept, nil)
	if !ok {
		return gap()
	}
	agent, patient := m.Roles["agent"], m.Roles["patient"]
	if agent.Concept == nil || patient.Concept == nil {
		return gap() // a referent grounds in nothing; admit rather than guess
	}
	d.noteFocus(agent.Concept) // a question about the cat chasing keeps the cat the topic
	rel := d.resolveRelation(m.Relation.Phrase)
	if rel == nil {
		// The referents are known but the relation is not, so the proposition
		// cannot hold. Deny it plainly.
		return upperFirst(d.gram.Word(frame.Lang, "no")) + ".", true
	}
	// English carries the tense on the auxiliary it led with; Turkish on the verb
	// the particle trailed, read back by Comprehend. Either way the answer matches.
	past := pastAux || m.Past
	belief, _, supposed := d.assessProposition(agent.Concept, rel.Content, patient.Concept)
	return d.answerPolar(frame.Lang, agent.Concept, rel, patient.Concept, past, belief, supposed), true
}

// planSuppose entertains a supposition without asserting it: "suppose the wolf
// chased the cat" comprehends its clause to a proposition and holds it in working
// memory as hypothetical, so questions reason under it while it stands and
// nothing it implies is ever written to memory. It is the entry to GOLEM
// reasoning about something it does not believe. The clause is comprehended with
// word order authoritative, like a polar question, so the supposition is the one
// posed rather than one role-fit reordered. It returns false when the utterance
// is not a supposition, so ordinary handling stands. "Never mind" releases it.
func (d *Daemon) planSuppose(frame grammar.Frame, raw string) (string, bool) {
	clause, ok := supposeClause(frame.Lang, raw)
	if !ok {
		return "", false
	}
	cannot := func() (string, bool) {
		return d.realizeAs(frame.Lang, grammar.Frame{Force: grammar.ForceStatement,
			Action: grammar.Term{Phrase: "know"}, Negate: true, Patient: grammar.Term{Phrase: "that"}}, d.chem.Snapshot()), true
	}
	m, ok := d.gram.Comprehend(frame.Lang, clause, d.resolveConcept, nil)
	if !ok {
		return cannot()
	}
	agent, patient := m.Roles["agent"], m.Roles["patient"]
	if agent.Concept == nil || patient.Concept == nil {
		return cannot()
	}
	rel := d.resolveRelation(m.Relation.Phrase)
	if rel == nil {
		return cannot()
	}
	past := m.Past
	d.held.add(heldProp{rel: rel.Content, agent: agent.Concept.ID, patient: patient.Concept.ID,
		agentGloss: agent.Concept.Content, patientGloss: patient.Concept.Content,
		status: heldHypothetical, conf: 1, depth: 0})
	// Echo the supposition back so it is clear what is being entertained, the
	// same construction every proposition speaks through, marked a supposition.
	echo, _ := d.gram.Speak(frame.Lang, grammar.Meaning{
		Relation: grammar.Term{Phrase: d.relationName(rel.Content, frame.Lang)},
		Roles: map[string]grammar.Term{
			"agent":   {Phrase: d.conceptName(agent.Concept.ID, agent.Concept.Content, frame.Lang), Definite: true},
			"patient": {Phrase: d.conceptName(patient.Concept.ID, patient.Concept.Content, frame.Lang), Definite: true},
		},
		Past: past,
	})
	return supposeAck(frame.Lang, echo), true
}

// planRelease ends a supposition: "never mind" drops every hypothetical held
// under it, and with it everything that was only contingent on it, so GOLEM
// returns to reasoning from what it believes. It leaves inferred beliefs
// standing. It returns false when the utterance is not a release.
func (d *Daemon) planRelease(frame grammar.Frame, raw string) (string, bool) {
	if !isRelease(raw) {
		return "", false
	}
	d.held.clearStatus(heldHypothetical)
	ack := "All right."
	if frame.Lang == "tr" {
		ack = "Peki."
	}
	return ack, true
}

// planContinue is the open invitation to say more: "go on", "what else", a turn
// with no subject of its own. GOLEM volunteers what it has concluded about the
// topic in mind and not yet said, holding the focus across the turn and choosing
// what to add, so sustained topic and initiative meet in one move. With nothing
// left it says so. It returns false when the turn is not an invitation to go on.
func (d *Daemon) planContinue(frame grammar.Frame, raw string) (string, bool) {
	if !isContinue(raw) {
		return "", false
	}
	if line, ok := d.volunteerRemark(frame.Lang); ok {
		return line, true
	}
	if frame.Lang == "tr" {
		return "Hakkında ekleyecek başka bir şeyim yok.", true
	}
	return "I have nothing more to add about it.", true
}

// isContinue reports whether an utterance is a bare invitation to continue on the
// topic in mind, carrying no subject of its own.
func isContinue(raw string) bool {
	s := strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(strings.ToLower(raw)), "?"))
	s = strings.TrimSuffix(s, ".")
	for _, p := range []string{"go on", "tell me more", "what else", "anything else", "what else about it", "and what else", "başka", "devam et", "başka ne"} {
		if s == p {
			return true
		}
	}
	return false
}

// supposeClause reads the clause of a supposition, the proposition after a
// "suppose"/"imagine"/"what if" lead, with any trailing mark dropped. It returns
// false when the utterance carries no such lead.
func supposeClause(lang, raw string) (string, bool) {
	s := strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(strings.ToLower(raw)), "?"))
	leads := []string{"suppose that ", "suppose ", "imagine that ", "imagine ", "what if "}
	if lang == "tr" {
		leads = []string{"diyelim ki ", "varsayalım ki "}
	}
	for _, lead := range leads {
		if rest, ok := strings.CutPrefix(s, lead); ok {
			if rest = strings.TrimSpace(rest); rest != "" {
				return rest, true
			}
		}
	}
	return "", false
}

// isRelease reports whether an utterance ends a supposition: "never mind" and
// its kin, kept clear of the forget faculty's words so releasing a thought never
// deletes a concept.
func isRelease(raw string) bool {
	s := strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(strings.ToLower(raw)), "."))
	for _, p := range []string{"never mind", "nevermind", "never mind that", "boş ver", "neyse"} {
		if s == p {
			return true
		}
	}
	return false
}

// supposeAck speaks back what is being supposed, the proposition led (English) or
// trailed (Turkish) by the tongue's supposing word, so the entertained clause is
// confirmed without being asserted.
func supposeAck(lang, echo string) string {
	echo = strings.TrimSuffix(strings.TrimSpace(echo), ".")
	if lang == "tr" {
		return upperFirst(echo) + " diyelim."
	}
	return "Supposing " + lowerFirst(echo) + "."
}

// propositionHolds reads whether the graph holds a proposition: that the agent
// stands in the relation to the patient. It crosses the same role edges every
// answer walks, the agent's co-arguments under the relation, and reports whether
// the patient is among them, so a polar question is settled against grounded
// memory and not a fixed table. The gather is unranked, so a true but faint
// proposition is found where a salience-capped answer might not have spoken it.
func (d *Daemon) propositionHolds(agent *memory.Fragment, relContent string, patientID uint64) bool {
	for _, f := range d.gatherFacts(agent, "agent", relContent) {
		if f.coID == patientID {
			return true
		}
	}
	return false
}

// propBelief is how firmly GOLEM holds a proposition it was asked about.
type propBelief int

const (
	beliefUnknown  propBelief = iota // neither observed nor inferable
	beliefObserved                   // the graph holds it, a fact
	beliefInferred                   // not observed, but a near neighbour did it, a guess
)

// inferenceFloor is how near a neighbour must be for its proposition to support
// an inference about the asked agent; below it, resemblance is too thin to guess on.
const inferenceFloor = 0.3

// propositionSupport reads how firmly a concept stands in a relation to a
// patient, for an inference leaning on it. Full when the graph holds it
// outright; attenuated when GOLEM only holds it as an inferred belief in working
// memory, so a chain that runs through a guess is weaker than one through a
// fact; absent otherwise. It returns the derivation depth of the support and the
// held belief it rode, so the conclusion drawn from it records how far it stands
// from observation and what it was drawn from, and so a chain stops before it
// runs past heldMaxDepth.
func (d *Daemon) propositionSupport(agentID uint64, relContent string, patientID uint64) (conf float64, depth int, from uint64, supposed, ok bool) {
	if d.propositionHolds(&memory.Fragment{ID: agentID}, relContent, patientID) {
		return 1, 0, 0, false, true
	}
	if p := d.held.findID(relContent, agentID, patientID, heldInferred); p != nil && p.depth < heldMaxDepth {
		return p.conf * heldStepKeep, p.depth + 1, p.id, false, true
	}
	// A supposition supports a guess the same way a belief does while it stands,
	// but the guess it yields is contingent on it: the supposed flag rides back so
	// the conclusion is held as hypothetical and never written as a belief.
	if p := d.held.findID(relContent, agentID, patientID, heldHypothetical); p != nil && p.depth < heldMaxDepth {
		return p.conf * heldStepKeep, p.depth + 1, p.id, true, true
	}
	return 0, 0, 0, false, false
}

// assessProposition reads how GOLEM holds a proposition. Observed when the graph
// holds it outright. Inferred when a concept the agent resembles stands in the
// same relation to the patient, in the graph or already in mind: the resemblance
// layer proposes the neighbour and the support layer disposes by checking whether
// that neighbour actually did it or is believed to, so the answer is a calibrated
// guess earned from a near neighbour rather than a fact. Unknown otherwise.
//
// A conclusion it draws is held in working memory, so a later step can build on
// it: the wolf guessed to chase the cat lets the coyote, which resembles the
// wolf, be guessed to chase it too, a hop further from observation and a notch
// less sure. GOLEM's own standing conclusion about the very proposition counts as
// evidence for it, so a re-ask is consistent and the belief stays warm. The
// confidence is the resemblance the guess rode in on, attenuated once per hop.
func (d *Daemon) assessProposition(agent *memory.Fragment, relContent string, patient *memory.Fragment) (belief propBelief, conf float64, supposed bool) {
	if d.propositionHolds(agent, relContent, patient.ID) {
		return beliefObserved, 1, false
	}
	// A thing standing in a relation to itself is not a claim worth guessing, so
	// a self-proposition is left unknown rather than inferred from a neighbour.
	if agent.ID == patient.ID {
		return beliefUnknown, 0, false
	}
	sibs, err := d.featureSiblings(agent.ID, time.Now())
	if err != nil {
		return beliefUnknown, 0, false
	}
	best, bestDepth, bestFrom, bestSupposed := 0.0, 0, uint64(0), false
	// A standing conclusion about this very proposition is evidence for it,
	// whether GOLEM believes it or is only supposing it.
	if p := d.held.findID(relContent, agent.ID, patient.ID, heldInferred); p != nil {
		best, bestDepth = p.conf, p.depth
	}
	if p := d.held.findID(relContent, agent.ID, patient.ID, heldHypothetical); p != nil && p.conf > best {
		best, bestDepth, bestSupposed = p.conf, p.depth, true
	}
	for _, sib := range sibs {
		// The patient itself is not evidence that the agent acts on it.
		if sib.id == patient.ID {
			continue
		}
		conf, depth, from, supposed, ok := d.propositionSupport(sib.id, relContent, patient.ID)
		if !ok {
			continue
		}
		if cand := sib.weight * conf; cand > best {
			best, bestDepth, bestFrom, bestSupposed = cand, depth, from, supposed
		}
	}
	if best >= inferenceFloor {
		// A conclusion drawn from fact or belief is held as a belief a later step
		// can build on; one that leans on a supposition is left unrecorded,
		// contingent on the supposition and gone with it.
		if !bestSupposed {
			var from []uint64
			if bestFrom != 0 {
				from = []uint64{bestFrom}
			}
			d.held.add(heldProp{rel: relContent, agent: agent.ID, patient: patient.ID,
				agentGloss: agent.Content, patientGloss: patient.Content,
				status: heldInferred, conf: best, depth: bestDepth, from: from, voiced: true})
		}
		return beliefInferred, best, bestSupposed
	}
	return beliefUnknown, 0, false
}

// inferRole answers a role question GOLEM holds no observed fact for from
// resemblance, the wh-question counterpart of the polar guess. The concept the
// asked one most resembles, that stood in the relation in the questioned role,
// lends its co-argument: "what did the wolf chase", unobserved, is answered
// "Probably, the wolf chased the cat" when the wolf resembles the dog and the
// dog chased the cat. Resemblance proposes the neighbour, the neighbour's
// gathered facts are spoken back under the asked concept's name, and the hedge
// marks the guess. The same gather and renderer the observed answer uses, here
// run over a near neighbour. relContent filters to one relation, "" leaves it
// open for "what did X do". continuation carries the focus as the tongue's
// pronoun, for a guess that trails a clause already naming it ("A wolf is a
// thing. Probably, it chased the cat"). It returns the rendered line and true
// when a near enough neighbour had such a fact, so the caller offers the gap
// only when none did.
func (d *Daemon) inferRole(frame grammar.Frame, known *memory.Fragment, knownRole, relContent string, past, continuation bool) (string, bool) {
	kept, _, ok := d.inferredFacts(known, knownRole, relContent)
	if !ok {
		return "", false
	}
	line, ok := d.renderFacts(frame.Lang, d.conceptName(known.ID, known.Content, frame.Lang), kept, past, continuation)
	if !ok {
		return "", false
	}
	hedge := d.gram.Word(frame.Lang, "probably")
	if hedge == "" {
		return line, true
	}
	return upperFirst(hedge) + ", " + lowerFirst(line), true
}

// inferredFacts finds the propositions a concept probably stands in, from the
// nearest neighbour that actually stands in them: the facts the strongest
// resembling concept has in the questioned role, with the asker dropped from its
// own co-arguments, and the resemblance they rode in on. It is the fact-finding
// behind a wh-guess, shared so the same propositions a guess renders can also be
// held in mind and volunteered. It returns false when no near enough neighbour
// has such a fact.
func (d *Daemon) inferredFacts(known *memory.Fragment, knownRole, relContent string) ([]focusFact, float64, bool) {
	sibs, err := d.featureSiblings(known.ID, time.Now())
	if err != nil {
		return nil, 0, false
	}
	// Strongest resemblance first, so the guess rides the nearest neighbour that
	// actually stood in the relation.
	sort.SliceStable(sibs, func(i, j int) bool { return sibs[i].weight > sibs[j].weight })
	for _, sib := range sibs {
		if sib.weight < inferenceFloor {
			break
		}
		facts := d.gatherFacts(&memory.Fragment{ID: sib.id}, knownRole, relContent)
		// The asked concept is not evidence that it stands in the relation to
		// itself, so a co-argument that is the asker is dropped.
		kept := facts[:0]
		for _, f := range facts {
			if f.coID != known.ID {
				kept = append(kept, f)
			}
		}
		if len(kept) == 0 {
			continue
		}
		return kept, sib.weight, true
	}
	return nil, 0, false
}

// heldRoleGuess answers a role question from what GOLEM is supposing rather than
// what it observed: a held hypothetical in which the asked concept stands in the
// asked role lends its co-argument, spoken back behind the conditional lead so a
// supposed conclusion is never taken for a believed or an observed one. It is the
// wh counterpart of the polar "If so, probably ..." answer, so a supposition
// reasons across every question shape and not only yes/no. relContent filters to
// one relation, "" leaves it open ("what did X do"); continuation carries the
// focus as a pronoun when a clause already named it. It returns false when nothing
// supposed fills the asked role.
func (d *Daemon) heldRoleGuess(frame grammar.Frame, known *memory.Fragment, knownRole, relContent string, past, continuation bool) (string, bool) {
	var facts []focusFact
	for _, p := range d.held.props {
		if p.status != heldHypothetical {
			continue
		}
		if relContent != "" && p.rel != relContent {
			continue
		}
		var coID uint64
		var coGloss string
		switch {
		case knownRole == "agent" && p.agent == known.ID:
			coID, coGloss = p.patient, p.patientGloss
		case knownRole == "patient" && p.patient == known.ID:
			coID, coGloss = p.agent, p.agentGloss
		default:
			continue
		}
		if coID == known.ID {
			continue
		}
		facts = append(facts, focusFact{rel: p.rel, focusRole: knownRole, coGloss: coGloss, coID: coID})
	}
	if len(facts) == 0 {
		return "", false
	}
	line, ok := d.renderFacts(frame.Lang, d.conceptName(known.ID, known.Content, frame.Lang), facts, past, continuation)
	if !ok {
		return "", false
	}
	hedge := d.gram.Word(frame.Lang, "probably")
	cond := d.gram.Word(frame.Lang, "ifso")
	switch {
	case cond != "" && hedge != "":
		return upperFirst(cond) + ", " + hedge + " " + lowerFirst(line), true
	case cond != "":
		return upperFirst(cond) + ", " + lowerFirst(line), true
	case hedge != "":
		return upperFirst(hedge) + ", " + lowerFirst(line), true
	default:
		return line, true
	}
}

// drawInferenceAbout concludes something about a concept on GOLEM's own account,
// held in mind as an unvoiced belief rather than spoken: what the nearest
// neighbour does, transferred to the concept that resembles it. The neighbour's
// observed actions transfer as fresh guesses one hop from the graph; the
// neighbour's own held beliefs transfer a hop further out, attenuated and
// depth-bounded, so the idle loop builds a chain across rounds the way a question
// chains within one. The conclusions ride the resemblance, the held ones their
// support's confidence as well, and are deposited for initiative to offer.
func (d *Daemon) drawInferenceAbout(topic *memory.Fragment) {
	sibs, err := d.featureSiblings(topic.ID, time.Now())
	if err != nil {
		return
	}
	sort.SliceStable(sibs, func(i, j int) bool { return sibs[i].weight > sibs[j].weight })
	// What the topic already does observably is known, not a conclusion to draw.
	// Gathered once, it keeps a transferred fact the topic already stands in from
	// being deposited as a guess and later volunteered, hedged, as if GOLEM were
	// unsure of something it has seen.
	type coArg struct {
		rel string
		id  uint64
	}
	observed := map[coArg]bool{}
	for _, f := range d.gatherFacts(topic, "agent", "") {
		observed[coArg{f.rel, f.coID}] = true
	}
	for _, sib := range sibs {
		if sib.weight < inferenceFloor {
			break
		}
		drew := false
		// What the neighbour is observed to do, transferred as a fresh guess.
		for _, f := range d.gatherFacts(&memory.Fragment{ID: sib.id}, "agent", "") {
			if f.coID == topic.ID || observed[coArg{f.rel, f.coID}] {
				continue
			}
			d.held.add(heldProp{rel: f.rel, agent: topic.ID, patient: f.coID,
				agentGloss: topic.Content, patientGloss: f.coGloss,
				status: heldInferred, conf: sib.weight, depth: 0})
			drew = true
		}
		// What the neighbour is only believed to do, transferred a hop further out.
		for _, p := range d.held.byAgent(sib.id, heldInferred) {
			if p.patient == topic.ID || p.depth >= heldMaxDepth || observed[coArg{p.rel, p.patient}] {
				continue
			}
			d.held.add(heldProp{rel: p.rel, agent: topic.ID, patient: p.patient,
				agentGloss: topic.Content, patientGloss: p.patientGloss,
				status: heldInferred, conf: sib.weight * p.conf * heldStepKeep, depth: p.depth + 1,
				from: []uint64{p.id}})
			drew = true
		}
		if drew {
			return
		}
	}
}

// promoteBeliefs carries the night's earned conclusions into long-term memory
// and clears the passing ones. A belief re-drawn often enough is saved to the
// belief store and marked persistent in working memory, so it survives the sleep
// that clears the rest. It stays in the held layer, never the observed graph, so
// a promoted belief is still read as a guess ("probably") and never as something
// seen ("yes"): the separation is what keeps the promotion honest.
func (d *Daemon) promoteBeliefs() {
	for _, p := range d.held.earned(promoteReinforce) {
		if err := d.store.SaveBelief(memory.Belief{Rel: p.rel, Agent: p.agent, Patient: p.patient,
			AgentGloss: p.agentGloss, PatGloss: p.patientGloss, Conf: p.conf, Depth: p.depth}); err != nil {
			fmt.Fprintf(d.out, "belief save error: %v\n", err)
			continue
		}
		// Mark the same conclusion persistent in mind, so it is kept rather than
		// cleared when the passing thoughts are dropped below.
		d.held.add(heldProp{rel: p.rel, agent: p.agent, patient: p.patient,
			agentGloss: p.agentGloss, patientGloss: p.patientGloss,
			status: heldInferred, conf: p.conf, depth: p.depth, voiced: true, persistent: true})
	}
	d.held.keepPersistent()
}

// loadBeliefs restores the conclusions long-term memory kept into working memory,
// the morning after a night and the first thing on waking from a saved brain, so
// a believed proposition is in mind again and reads as a belief. Called where the
// workspace is rebuilt.
func (d *Daemon) loadBeliefs() {
	beliefs, err := d.store.Beliefs()
	if err != nil {
		fmt.Fprintf(d.out, "belief load error: %v\n", err)
		return
	}
	for _, b := range beliefs {
		d.held.add(heldProp{rel: b.Rel, agent: b.Agent, patient: b.Patient,
			agentGloss: b.AgentGloss, patientGloss: b.PatGloss,
			status: heldInferred, conf: b.Conf, depth: b.Depth, voiced: true, persistent: true})
	}
}

// volunteerRemark offers something GOLEM has concluded about the topic in mind
// but not yet said, drawing a fresh conclusion when nothing is held. It is
// initiative grounded in working memory: GOLEM speaks for itself, once per
// conclusion, and falls silent when it has nothing left to add. It returns false
// when there is no topic or nothing left to volunteer.
func (d *Daemon) volunteerRemark(lang string) (string, bool) {
	if d.topic == 0 {
		return "", false
	}
	p := d.held.unvoicedAbout(d.topic)
	if p == nil {
		if frags, err := d.fragmentsByIDsTiered([]uint64{d.topic}); err == nil && len(frags) == 1 {
			d.drawInferenceAbout(frags[0])
		}
		p = d.held.unvoicedAbout(d.topic)
	}
	if p == nil {
		return "", false
	}
	line := d.speakHeld(lang, p)
	if line == "" {
		return "", false
	}
	d.held.markVoiced(p.id)
	d.ws.excite(p.patient, exciteSpeech)
	d.recordClaim(p.rel, p.agent, p.patient, true, true) // a volunteered belief is a hedged claim
	return line, true
}

// speakHeld renders a held belief as a hedged clause, the shape an inferred
// answer takes, so a volunteered conclusion is offered as the guess it is.
func (d *Daemon) speakHeld(lang string, p *heldProp) string {
	clause, ok := d.gram.Speak(lang, grammar.Meaning{
		Relation: grammar.Term{Phrase: d.relationName(p.rel, lang)},
		Roles: map[string]grammar.Term{
			"agent":   {Phrase: d.conceptName(p.agent, p.agentGloss, lang), Definite: true},
			"patient": {Phrase: d.conceptName(p.patient, p.patientGloss, lang), Definite: true},
		},
		Past: true,
	})
	if !ok {
		return ""
	}
	hedge := d.gram.Word(lang, "probably")
	if hedge == "" {
		return clause
	}
	return upperFirst(hedge) + ", " + lowerFirst(clause)
}

// nearestConcept returns the concept a seed most resembles together with that
// resemblance, the single strongest feature sibling at or above the inference
// floor, or false when none is near enough. It is the one-neighbour read the
// kind guess leans on, where inferRole wants the strongest neighbour that also
// stood in a given relation.
func (d *Daemon) nearestConcept(seed uint64) (*memory.Fragment, float64, bool) {
	sibs, err := d.featureSiblings(seed, time.Now())
	if err != nil {
		return nil, 0, false
	}
	best := walkDest{}
	for _, sib := range sibs {
		if sib.weight > best.weight {
			best = sib
		}
	}
	if best.id == 0 || best.weight < inferenceFloor {
		return nil, 0, false
	}
	frags, err := d.fragmentsByIDsTiered([]uint64{best.id})
	if err != nil || len(frags) == 0 {
		return nil, 0, false
	}
	return frags[0], best.weight, true
}

// kindGuessClause offers a hedged guess at what a thinly-known thing IS, from
// the better-known thing it most resembles: "what is a wolf", seen once and
// resembling the dog, gains "Probably, it is a dog." It fires only for a
// generic thing, never an action, which already names its kind; only when a
// neighbour is near enough; and only when that neighbour is better grounded
// than the asked concept, so a thing GOLEM knows firmly is never reclassified
// as something it merely resembles. The guess is a definition clause naming the
// neighbour as the kind, led by the tongue's hedge and carrying the asked thing
// as a pronoun where the tongue has a bare one.
func (d *Daemon) kindGuessClause(frame grammar.Frame, c *memory.Fragment, action bool, tally map[uint64]int) (string, bool) {
	if action {
		return "", false
	}
	nb, _, ok := d.nearestConcept(c.ID)
	if !ok {
		return "", false
	}
	// Guess toward the better-known thing: a concept seen no more than the
	// neighbour it resembles is the uncertain one, so the neighbour lends its
	// identity. A thing known as well or better stands on its own.
	if tally[c.ID] >= tally[nb.ID] {
		return "", false
	}
	name := d.conceptName(nb.ID, nb.Content, frame.Lang)
	kindWord := grammar.Indefinite(name)
	if frame.Lang != "" && frame.Lang != "en" {
		kindWord = "bir " + name
	}
	subject := d.gram.Word(frame.Lang, "pronoun")
	if subject == "" {
		subject = d.conceptName(c.ID, c.Content, frame.Lang)
	}
	clause := d.definitionClause(frame.Lang, subject, kindWord, d.chem.Snapshot())
	hedge := d.gram.Word(frame.Lang, "probably")
	if hedge == "" {
		return clause, true
	}
	return upperFirst(hedge) + ", " + lowerFirst(clause), true
}

// answerPolar speaks a yes/no answer at the confidence GOLEM holds it. An
// observed proposition affirms plainly ("Yes, the dog chased the cat"); an
// unknown one denies ("No, the cat did not chase the dog"), the negative
// morphology running for the "no"; an inferred one leads with the tongue's hedge
// ("Probably, the wolf chased the cat") and speaks the affirmed clause, so the
// guess is offered as a guess and never as a fact. A guess that rode a
// supposition is marked contingent on it ("If so, probably the cat chased the
// dog"), so it is never mistaken for something GOLEM believes outright. Word and
// clause both come from the tongue's data.
func (d *Daemon) answerPolar(lang string, agent, rel, patient *memory.Fragment, past bool, belief propBelief, supposed bool) string {
	// What GOLEM is about to commit to, weighed against what it has already said
	// this exchange. A supposition is contingent, so it is neither recorded nor
	// held against: it never becomes a claim GOLEM stands by.
	acked := false
	if !supposed {
		affirm := belief == beliefObserved || belief == beliefInferred
		hedged := belief == beliefInferred
		if prior := d.recallClaim(rel.Content, agent.ID, patient.ID); prior != nil {
			if prior.affirmed && !affirm {
				// GOLEM affirmed this proposition earlier and nothing told this
				// exchange overturns it; it stands by its claim rather than quietly
				// contradict itself, at the confidence it first gave.
				affirm, hedged = true, prior.hedged
				if hedged {
					belief = beliefInferred
				} else {
					belief = beliefObserved
				}
			}
			// The same claim again, at the same confidence, is noted as such; a guess
			// that has firmed into a known fact ("probably" -> "yes") is an upgrade
			// stated plainly, not a repetition.
			acked = prior.affirmed == affirm && prior.hedged == hedged
		}
		d.recordClaim(rel.Content, agent.ID, patient.ID, affirm, hedged)
	}

	var word string
	negate := false
	switch belief {
	case beliefObserved:
		word = d.gram.Word(lang, "yes")
	case beliefInferred:
		word = d.gram.Word(lang, "probably") // the hedge marks the clause a guess
	default:
		word = d.gram.Word(lang, "no")
		negate = true
	}
	clause, ok := d.gram.Speak(lang, grammar.Meaning{
		Relation: grammar.Term{Phrase: d.relationName(rel.Content, lang)},
		Roles: map[string]grammar.Term{
			"agent":   {Phrase: d.conceptName(agent.ID, agent.Content, lang), Definite: true},
			"patient": {Phrase: d.conceptName(patient.ID, patient.Content, lang), Definite: true},
		},
		Past:   past,
		Negate: negate,
	})
	var answer string
	switch {
	case !ok:
		// No construction bound the proposition; the bare word still answers.
		answer = upperFirst(word) + "."
	case belief == beliefInferred && supposed && d.gram.Word(lang, "ifso") != "":
		// A guess that leans on a supposition follows only if the supposition does:
		// the conditional lead carries that, the hedge still marking it a guess.
		answer = upperFirst(d.gram.Word(lang, "ifso")) + ", " + lowerFirst(word) + " " + lowerFirst(clause)
	case word == "":
		answer = clause
	default:
		answer = upperFirst(word) + ", " + lowerFirst(clause)
	}
	// Asked again, GOLEM notes it has already answered rather than replaying it cold.
	return d.ackIfSaid(lang, answer, acked)
}

// activityAgent finds whose activity "what did X do" asks after: the frame's
// agent when the parser bound one, else the first content word that names a
// concept, so a tongue whose "X ne yaptı" shape the frame parser does not bind
// an agent for still reaches its agent. The wh-word and the pro-verb name no
// concept, so they are skipped without a special case.
func (d *Daemon) activityAgent(frame grammar.Frame, raw string) *memory.Fragment {
	if frame.Agent.Concept != nil {
		return frame.Agent.Concept
	}
	tokens, _ := grammar.Tokenize(raw)
	for _, t := range tokens {
		if !d.gram.IsContentWord(frame.Lang, t) {
			continue
		}
		if c := d.resolveConcept(t); c != nil && c.Category != "relation" {
			return c
		}
	}
	return nil
}

// proVerbDo reports whether a clause's main verb is the tongue's generic
// pro-verb, "what did the dog do" rather than "what did the dog chase": the
// last word is one of the tongue's pro-verbs, which stands in for the relation
// the question asks after. The pro-verbs are read from the tongue's data.
func (d *Daemon) proVerbDo(lang, raw string) bool {
	tokens, _ := grammar.Tokenize(raw)
	if len(tokens) == 0 {
		return false
	}
	last := tokens[len(tokens)-1]
	for _, pv := range d.gram.ProVerbs(lang) {
		if last == pv {
			return true
		}
	}
	return false
}

// resolveRelation finds the relation a word names, preferring it over any
// other concept the word also points at. A verb's form can come to name a
// spurious noun the constellation pass distilled from its arguments' company,
// and the naming vote would return that noun; a role question needs the
// relation, so it asks for that kind directly.
func (d *Daemon) resolveRelation(phrase string) *memory.Fragment {
	tokens, _ := grammar.Tokenize(phrase)
	words, err := d.store.WordNodesByForms(tokens)
	if err != nil || len(words) == 0 {
		return nil
	}
	ids := make([]uint64, 0, len(words))
	for _, w := range words {
		ids = append(ids, w.ID)
	}
	names, err := d.store.LinksFromRel(ids, memory.RelNames)
	if err != nil || len(names) == 0 {
		return nil
	}
	targets := make([]uint64, 0, len(names))
	weight := make(map[uint64]float64, len(names))
	for _, e := range names {
		if weight[e.To] == 0 {
			targets = append(targets, e.To)
		}
		weight[e.To] += e.Weight
	}
	frags, err := d.store.FragmentsByIDs(targets)
	if err != nil {
		return nil
	}
	var best *memory.Fragment
	bestW := 0.0
	for _, f := range frags {
		if f.Category != "relation" {
			continue
		}
		if best == nil || weight[f.ID] > bestW {
			best, bestW = f, weight[f.ID]
		}
	}
	return best
}

// namesRelation reports whether a word form names a relation, directly or
// through its lemma. The relation is keyed on the lemma, so an inflected
// surface ("chased") reaches it only through its lemma; checking both
// recognizes every inflection of a verb as the verb. It is the one test the
// abstraction pass and the naming pass share, so a verb is treated the same
// way wherever the constellation machinery would otherwise read its arguments
// as its company.
func (d *Daemon) namesRelation(content, lang string) bool {
	return d.resolveRelation(content) != nil || d.resolveRelation(d.gram.Lemma(lang, content)) != nil
}

// analyzeRoleQuestion reads a relational role question into the relation it
// asks about, the argument it gives, and which role that argument fills. The
// verb is the content word whose lemma names a grounded relation; the other
// content word is the known argument. Where the verb sits decides the rest:
// a verb leading the clause makes the wh-word the subject, so the known
// argument is the patient ("who chased the cat"), while a verb after an
// intervening noun makes the wh-word the object, so the known argument is the
// agent ("what did the dog chase").
func (d *Daemon) analyzeRoleQuestion(lang, raw string) (rel, known *memory.Fragment, knownRole, verb string) {
	tokens, _ := grammar.Tokenize(raw)
	var content []string
	for _, t := range tokens {
		if d.gram.IsContentWord(lang, t) {
			content = append(content, t)
		}
	}
	verbIdx := -1
	for i, t := range content {
		if c := d.resolveRelation(d.gram.Lemma(lang, t)); c != nil {
			verbIdx, rel, verb = i, c, t
			break
		}
	}
	if verbIdx < 0 {
		return nil, nil, "", ""
	}
	for i, t := range content {
		if i == verbIdx {
			continue
		}
		if c := d.resolveConcept(t); c != nil && c.Category != "relation" {
			known = c
			break
		}
	}
	if known == nil {
		return nil, nil, "", ""
	}
	if verbIdx == 0 {
		return rel, known, "patient", verb // the wh-word is the subject; the known argument is the patient
	}
	return rel, known, "agent", verb // an argument precedes the verb; the wh-word is the object
}

// relationConcept resolves the relation a proposition binds into, minting it
// when the verb is new. A relation is a concept like any other, grounded,
// named, and weighted; the verb's lemma names it, so a later telling resolves
// to the same relation rather than minting a second, and a noun that happens to
// share the surface is not mistaken for it. The lemma's concept is found across
// tongues: a verb reaches its relation through naming edges, not a per-language
// key, so once two tongues are bound to one relation they both reach it.
func (d *Daemon) relationConcept(rel grammar.Term, lang string) (*memory.Fragment, error) {
	if rel.Concept != nil && rel.Concept.Category == "relation" {
		return rel.Concept, nil
	}
	lemma := rel.Phrase
	if lemma == "" && rel.Concept != nil {
		lemma = rel.Concept.Content
	}
	if lemma == "" {
		return nil, nil
	}
	if lang == "" {
		lang = "en"
	}
	// A relation this verb already names, in any tongue, is the one to reuse.
	if c := d.resolveRelation(lemma); c != nil {
		return c, nil
	}
	// A verb new to this tongue binds to the relation just spoken of: a relation
	// warm in mind and not yet named here takes this word as its name in this
	// tongue rather than spawning a parallel relation. This is attention
	// grounding on the relation side, the same ostensive path a noun takes when
	// it is named for what is held in mind, so chase discussed in one tongue and
	// told again in another is the one relation and the thread survives the
	// change of tongue. A relation already named here keeps its name, so two
	// distinct verbs of one tongue stay two relations.
	if warm := d.warmestRelation(); warm != nil && !d.relationNamedIn(warm.ID, lang) {
		d.nameRelationInTongue(warm, lemma, lang)
		return warm, nil
	}
	tpl := "rel:" + lang + ":" + lemma
	if c, err := d.store.ConceptByTemplate(tpl); err != nil || c != nil {
		return c, err
	}
	now := time.Now()
	c := &memory.Fragment{
		Kind:     memory.KindConcept,
		Channel:  "concept",
		Content:  lemma,
		Template: tpl,
		Category: "relation",
		Limbic:   d.chem.Snapshot(),
		Time:     now,
	}
	if _, err := d.store.Encode(c); err != nil {
		return nil, err
	}
	d.event(events.Event{Ev: "concept", ID: c.ID, Tpl: tpl, Outcome: "relation"})
	d.nameRelationInTongue(c, lemma, lang)
	return c, nil
}

// warmestRelation returns the warmest relation concept held in the workspace
// above the attention gate, the relation a verb new to a tongue binds to so it
// names what was just spoken of. Raw warmth is the signal, how present the
// relation is, regardless of any members it might subsume.
func (d *Daemon) warmestRelation() *memory.Fragment {
	warm := d.ws.warmest(workspaceBudget)
	ids := make([]uint64, 0, len(warm))
	heat := make(map[uint64]float64, len(warm))
	for _, e := range warm {
		if e.heat < attentionGroundingGate {
			continue
		}
		ids = append(ids, e.id)
		heat[e.id] = e.heat
	}
	if len(ids) == 0 {
		return nil
	}
	frags, err := d.store.FragmentsByIDs(ids)
	if err != nil {
		return nil
	}
	var best *memory.Fragment
	for _, f := range frags {
		if f.Category != "relation" {
			continue
		}
		if best == nil || heat[f.ID] > heat[best.ID] {
			best = f
		}
	}
	return best
}

// relationNamedIn reports whether a word of the given tongue already names the
// relation. It is the guard on attention binding: a tongue that already speaks
// the relation does not give it a second word from warmth, so two verbs of one
// tongue stay two relations, while a tongue with no name for it yet may give it
// its first.
func (d *Daemon) relationNamedIn(relID uint64, lang string) bool {
	namers, err := d.store.LinksToRel([]uint64{relID}, memory.RelNames)
	if err != nil || len(namers) == 0 {
		return false
	}
	ids := make([]uint64, 0, len(namers))
	for _, e := range namers {
		ids = append(ids, e.From)
	}
	words, err := d.store.FragmentsByIDs(ids)
	if err != nil {
		return false
	}
	for _, w := range words {
		if w.Lang == lang {
			return true
		}
	}
	return false
}

// nameRelationInTongue bonds a verb's word in one tongue to the relation it
// names, the naming edge a later telling in that tongue resolves through.
func (d *Daemon) nameRelationInTongue(rel *memory.Fragment, lemma, lang string) {
	w, _, err := d.store.WordNode(lemma, lang, d.chem.Snapshot(), time.Now())
	if err != nil {
		return
	}
	if err := d.store.SetLinkRel(w.ID, rel.ID, memory.RelNames, opNameWeight); err == nil {
		d.event(events.Event{Ev: "name", From: w.ID, To: rel.ID, Rel: memory.RelNames, Weight: opNameWeight})
	}
}

// roleFit scores a concept's affinity for a thematic role by ANIMACY, read off
// how much its feature signature overlaps the concepts that have filled the role:
// a concept that resembles past agents fits the agent role, so best fit corrects
// an implausible parse (an inanimate seated as agent) without a hand-listed
// table. It reads resemblance, NOT a concept's own role-count, on purpose: a
// concept's history of standing as a patient does not make it a patient now, and
// counting it would feed back, a thing once chased reading as never a chaser,
// unseating a clear subject. Word order binds the roles; animacy only rescues the
// parse the order clearly got wrong, and only once features distinguish animate
// from inanimate.
func (d *Daemon) roleFit(c *memory.Fragment, role string) float64 {
	rel := memory.RelAgent
	if role == "patient" {
		rel = memory.RelPatient
	}
	return d.roleKinship(c, rel)
}

// roleKinship measures how much a concept resembles the concepts that have
// filled a role, so a never-seen agent that shares the feature signature of
// past agents still fits the role. It reads the feature layer through the
// resemblance the walk already computes, and sums the resemblance of those
// neighbors that have stood in the role, the feature signatures constraining
// the binding without a hand-listed animacy table.
func (d *Daemon) roleKinship(c *memory.Fragment, rel string) float64 {
	sibs, err := d.featureSiblings(c.ID, time.Now())
	if err != nil || len(sibs) == 0 {
		return 0
	}
	sibIDs := make([]uint64, 0, len(sibs))
	resemblance := make(map[uint64]float64, len(sibs))
	for _, s := range sibs {
		sibIDs = append(sibIDs, s.id)
		resemblance[s.id] = s.weight
	}
	edges, err := d.store.LinksToRel(sibIDs, rel)
	if err != nil {
		return 0
	}
	score := 0.0
	for _, e := range edges {
		score += resemblance[e.To]
	}
	return score
}
