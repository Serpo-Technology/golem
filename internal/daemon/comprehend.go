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
// nounHead reads the head of a content span, the last open-class word a minted
// concept takes its name from ("a big model" -> "model"), lowercased; empty for a
// span too short to be a noun.
func nounHead(phrase string) string {
	fields := strings.Fields(strings.ToLower(strings.TrimSpace(phrase)))
	if len(fields) == 0 {
		return ""
	}
	head := fields[len(fields)-1]
	if len([]rune(head)) < 2 {
		return ""
	}
	return head
}

// mintNounConcept gives an unknown noun argument a concept of its own, the way an
// unknown verb gets a relation through relationConcept. The concept carries no
// features, since an abstract word names no percept, so it is the usage space that
// places it by its relational company, not the feature space. A function word is
// never minted, so a determiner or particle that slipped into a span is left alone.
// It returns the existing concept when the noun already names one.
func (d *Daemon) mintNounConcept(phrase, lang string) *memory.Fragment {
	// A numeral is a number, one concept per value however it arrives, so a
	// told claim about 17 and a computed fact about 17 meet on the same node.
	// It is read off the whole phrase before the head extraction, which reads
	// word heads and drops a bare digit.
	if n := strings.TrimSpace(phrase); numericLiteral(n) {
		if id := d.ensureNumberConcept(n); id != 0 {
			if frags, err := d.store.FragmentsByIDs([]uint64{id}); err == nil && len(frags) == 1 {
				return frags[0]
			}
		}
		return nil
	}
	head := nounHead(phrase)
	if head == "" || d.gram.IsFunctionWord(lang, head) {
		return nil
	}
	// A head that resolves only to grammar scaffolding is unresolved: the real
	// noun is minted so the teaching lands on a thing, not on a form pole.
	if c := d.resolveConcept(head); c != nil && !d.scaffoldingConcept(c.ID) {
		return c
	}
	c := &memory.Fragment{Kind: memory.KindConcept, Channel: "concept", Content: head,
		Template: "noun:" + lang + ":" + head, Category: "noun", Limbic: d.chem.Snapshot(), Time: time.Now()}
	if _, err := d.store.Encode(c); err != nil {
		fmt.Fprintf(d.out, "noun mint error: %v\n", err)
		return nil
	}
	d.nameConceptWord(c.ID, head, lang)
	return c
}

// acquirePatient resolves the object of a told proposition to a concept. An abstract
// noun names no percept, so the shared-percept path that unifies "dog" and "köpek"
// across tongues cannot reach it; the relation carries it instead. The relation has
// already bound across tongues by the time the object is read, so the object of the
// warm proposition over it, told in the other tongue, is the same thing this clause's
// object names: the foreign word becomes that concept's name in this tongue rather
// than minting a parallel concept. With no twin to bind, the object is minted fresh,
// as an unknown noun always was.
func (d *Daemon) acquirePatient(phrase, lang string, rel, agent *memory.Fragment) *memory.Fragment {
	lemma := d.nounLemma(phrase, lang)
	if lemma == "" || d.gram.IsFunctionWord(lang, lemma) {
		return nil
	}
	if c := d.resolveConcept(lemma); c != nil && !d.scaffoldingConcept(c.ID) {
		return c
	}
	if twin := d.crossTonguePatient(rel, agent, lang); twin != nil {
		d.nameConceptWord(twin.ID, lemma, lang)
		return twin
	}
	return d.mintNounConcept(lemma, lang)
}

// crossTonguePatient returns the object of the most recent thread proposition with
// the SAME agent AND relation, when that object is a noun this tongue does not yet
// name: the cross-tongue twin of the object now being read. Matching the whole
// proposition (agent + relation), not the relation alone, keeps "köpek X kovaladı"
// from binding X to whatever was last chased by anyone; it binds to what THIS dog
// chased. Speaking the same proposition in a new tongue back to back keeps that twin
// the most recent match, so the binding follows the adjacency it relies on.
func (d *Daemon) crossTonguePatient(rel, agent *memory.Fragment, lang string) *memory.Fragment {
	if rel == nil || agent == nil {
		return nil
	}
	for _, p := range d.threadProps {
		if p.rel != rel.Content || p.agent != agent.ID || p.patient == 0 {
			continue
		}
		frags, err := d.store.FragmentsByIDs([]uint64{p.patient})
		if err != nil || len(frags) != 1 {
			continue
		}
		if c := frags[0]; c.Category == "noun" && !d.conceptNamedIn(c.ID, lang) {
			return c
		}
	}
	return nil
}

// nounLemma reduces a noun phrase to the bare lemma that should name a concept,
// stripping a Turkish case ending the way the resolver does for a known noun, so an
// object minted or bound from "deseni" is named "desen", not the accusative it wore.
func (d *Daemon) nounLemma(phrase, lang string) string {
	head := nounHead(phrase)
	if lang == "tr" && head != "" {
		if stems := d.gram.CaseStems("tr", head); len(stems) > 0 {
			return stems[0]
		}
	}
	return head
}

// meaningsAgree reports whether two comprehended meanings bind the same relation and role
// concepts (and tense), the check the comprehension-traversal shadow uses to confirm the
// traversal reads what the coded path reads.
func meaningsAgree(a, b grammar.Meaning) bool {
	if a.Past != b.Past || !sameConcept(a.Relation.Concept, b.Relation.Concept) || len(a.Roles) != len(b.Roles) {
		return false
	}
	for role, ta := range a.Roles {
		if !sameConcept(ta.Concept, b.Roles[role].Concept) {
			return false
		}
	}
	return true
}

func sameConcept(a, b *memory.Fragment) bool {
	if a == nil || b == nil {
		return a == b
	}
	return a.ID == b.ID
}

// meaningGloss is a short surface for a meaning, for the shadow's mismatch log.
func meaningGloss(m grammar.Meaning) string {
	g := m.Relation.Phrase
	if a := m.Roles["agent"].Phrase; a != "" {
		g = a + " " + g
	}
	if p := m.Roles["patient"].Phrase; p != "" {
		g += " " + p
	}
	return g
}

// framesAgree reports whether two parsed frames carry the same force, interrogative, predicate, and
// question kind, the check the parse-traversal shadow uses to confirm the question traversal reads
// the frame the coded parser does. The comparative and magnitude flags distinguish an asked
// comparison from a plain copular question that shares the same force and predicate, so they are
// part of the recognition. A wh-question's roles are the asked-role extraction the planner reads off
// the tokens (analyzeRoleQuestion), not the frame's spans, which the coded parser leaves garbled for
// a learned verb; so the recognition is the check there, and the role concepts only for a non-wh
// frame (the copular and comparison questions, whose roles the planner reads from the frame).
func framesAgree(a, b grammar.Frame) bool {
	if a.Force != b.Force || a.Wh != b.Wh || a.Action.Phrase != b.Action.Phrase {
		return false
	}
	if a.Comparative != b.Comparative || a.Magnitude != b.Magnitude {
		return false
	}
	if a.Wh != "" {
		return true
	}
	return sameConcept(a.Agent.Concept, b.Agent.Concept) && sameConcept(a.Patient.Concept, b.Patient.Concept)
}

func (d *Daemon) comprehendStatement(frame grammar.Frame, raw string, momentID uint64) bool {
	d.followUp = "" // set only when this telling answers a question left open
	if momentID == 0 {
		return false
	}
	// How far this telling is trusted, from the source it was tagged with: the
	// architect's is full (an untagged local telling too), a peer's is its own. The
	// proposition it lays down is written at that trust, so a low-trust claim enters
	// weak and defeasible, entrenching only by corroboration, never at the firmness a
	// grounded observation carries.
	trust := d.momentTrust(momentID)
	w := comprehendWeight * trust
	// A comparison is parsed apart from the copular clause, so it is stored from the
	// frame's roles rather than re-read from the surface, which the argument
	// splitter would mistake (it has no "than" construction).
	if frame.Comparative {
		return d.storeComparison(frame, momentID, w)
	}
	// A Turkish locative clause marks place by the case and carries no verb
	// ("yaprak ağaçta"), so the verb-based comprehension would mistake the cased
	// ground for the predicate. Read it as a place before that path.
	if frame.Lang == "tr" && d.comprehendLocativeTR(raw, momentID) {
		return true
	}
	// A Latin locative clause places the figure with a preposition ("rosa in mensa
	// est"); the preposition and copula carry no role the case parser reads, so the
	// place is comprehended before it.
	if frame.Lang == "la" && d.comprehendLocativeLA(raw, momentID) {
		return true
	}
	// A Latin comparison ("rosa maior quam viola") orders two concepts on a scale
	// rather than relating them as a clause, so it is read as a comparison first.
	if frame.Lang == "la" && d.comprehendComparisonLA(raw, momentID) {
		return true
	}
	// A Turkish possession clause marks holding by case and the existential "var"
	// ("ağacın yaprağı var"), no verb, so it too is read before the verb path.
	if frame.Lang == "tr" && d.comprehendPossessionTR(raw, momentID) {
		return true
	}
	// A part-of clause ("the leaf is part of the tree") names a part-whole, which the argument
	// parse would mistake for a verb clause, so the construction is read before that path.
	if frame.Lang == "en" && !frame.Comparative && d.comprehendPartWhole(raw, momentID) {
		return true
	}
	m, ok := d.gram.Comprehend(frame.Lang, raw, d.resolveIn(frame.Lang), d.roleFit)
	if !ok {
		return false
	}
	// The construction-traversal comprehender (the inverse of Speak) drives where it reads the same
	// meaning the coded path does. It walks the same bidirectional argument construction and chooses
	// it by how the grounded spans fit its slots' roles (selectArgByFit), the reasoned counterpart of
	// the coded path's arity selection; its meaning is preferred and the coded Comprehend becomes the
	// cross-check (logged on disagreement) and the fallback for a clause the traversal declines. On
	// agreement the two meanings are identical: they share the span segmentation, the relation lemma,
	// and resolveCasedNoun, differing only in how the construction is selected, so the role spans the
	// minting reads are the same. So preferring the walk is behaviour-preserving while making it the
	// source of the read.
	if walked, wok := d.gram.ComprehendByTraversal(frame.Lang, raw, d.resolveIn(frame.Lang), d.roleFit); wok {
		if meaningsAgree(walked, m) {
			m = walked
		} else {
			fmt.Fprintf(d.out, "comprehension traversal mismatch: walked %q vs coded %q\n", meaningGloss(walked), meaningGloss(m))
		}
	}
	// A clause naming a conversation participant is a deictic assertion ("I chased
	// the cat", "kediyi kovaladım"): the speaker fixed the roles by the pronoun or
	// the verb's agreement, so word order is authoritative here as it is for a
	// correction. A participant carries no features, so role-fit would otherwise
	// unseat it for whatever featured concept reads as the better agent, storing
	// the cat as having chased the speaker. Re-read it with word order alone.
	if d.meaningNamesParticipant(m) {
		if m2, ok2 := d.gram.Comprehend(frame.Lang, raw, d.resolveIn(frame.Lang), nil); ok2 {
			m = m2
		}
	}
	// Two arguments whose feature signatures barely differ give role-fit nothing to
	// read: each resembles the other's past role-fillers about as much as its own, so
	// a hair's difference in kinship would swap a clause the order stated plainly ("a
	// wolf chased a rabbit" becoming the rabbit chasing). When the fillers are that
	// alike, re-read with word order alone, the same recourse a participant clause takes.
	if d.argsIndistinctForFit(m) {
		if m2, ok2 := d.gram.Comprehend(frame.Lang, raw, d.resolveIn(frame.Lang), nil); ok2 {
			m = m2
		}
	}
	agent := m.Roles["agent"]
	// A role that resolved to grammar scaffolding is unresolved: the clause is
	// about a thing, and the mint below gives it one, rather than binding the
	// proposition onto a form pole a substring happened to hit.
	if agent.Concept != nil && d.scaffoldingConcept(agent.Concept.ID) {
		agent.Concept = nil
	}
	if p := m.Roles["patient"]; p.Concept != nil && d.scaffoldingConcept(p.Concept.ID) {
		p.Concept = nil
		m.Roles["patient"] = p
	}
	if agent.Concept == nil {
		// Only a transitive content proposition acquires a noun ("a model predicts a
		// word"): a clause with no object is a copula predication ("the gizmo is
		// broken") or an intransitive, not a proposition to mint a subject from, so
		// its unknown is left to curiosity, as it was before acquisition reached the
		// noun side. The clause was read as a proposition and yielded none, so it
		// lays no naming either: the abandonment is marked for the tell handler.
		if m.Roles["patient"].Phrase == "" {
			d.statementAbandoned = true
			return false
		}
		// An unknown noun subject is minted as its own concept, the same acquisition
		// from a telling the unknown verb gets, so an abstract word like "model" that
		// names no percept enters the graph and the usage space places it. A subject
		// that is no mintable noun (a stray particle) is still no proposition.
		if agent.Concept = d.mintNounConcept(agent.Phrase, frame.Lang); agent.Concept == nil {
			d.statementAbandoned = true
			return false
		}
	}
	// A faculty word standing as the subject is a command the grammar misread as a
	// statement, its verb not being a seeded imperative opener. It is not a
	// proposition: declining it lets the faculty fallback route the clause and keeps
	// its object from being minted as a bogus relation.
	if _, isCommand := d.commandName[agent.Concept.ID]; isCommand {
		return false
	}
	// An unknown verb is learned here: relationConcept mints it as a new relation.
	// Resemblance to a known relation belongs on the question path, where a verb
	// cannot be learned and a fuzzy match aids retrieval, never on this teaching
	// path, where it would read a new verb as an old one and block acquisition.
	rel, err := d.relationConcept(m.Relation, frame.Lang)
	if err != nil || rel == nil {
		fmt.Fprintf(d.out, "relation mint error: %v\n", err)
		return false
	}
	// A relation spoken as a copula with a locative preposition ("the leaf is on
	// the tree") places its figure at its ground. The construction teaches the
	// relation's locativity: it is marked on the relation concept, so a "where"
	// query reads it and the clause renders with the preposition rather than as a
	// verb. The mark is idempotent, refreshed each time the construction is used.
	if d.gram.LocativePrep(frame.Lang, m.Relation.Phrase) {
		d.markLocative(rel.ID)
	}
	// An English possession verb ("the tree has a leaf") marks its relation
	// possessive, so a Turkish "var" clause unifies onto it and renders by the case.
	if possessionVerb(m.Relation.Phrase) {
		d.markPossessive(rel.ID)
	}
	var patientID uint64
	var patientGloss string
	patient := m.Roles["patient"]
	if patient.Concept == nil && patient.Phrase != "" {
		// An unknown noun object is acquired, so "a model predicts a word" grounds the
		// word it predicts as well as the model that predicts it. Where the relation
		// has already bound across tongues, the object binds to its cross-tongue twin
		// rather than minting a parallel concept; otherwise it is minted fresh.
		patient.Concept = d.acquirePatient(patient.Phrase, frame.Lang, rel, agent.Concept)
	}
	if patient.Concept != nil {
		patientID = patient.Concept.ID
		patientGloss = patient.Concept.Content
	}
	// A negated proposition ("the cat does not eat the plant") is a standing denial, not an
	// observation: it is recorded as a direct denial edge from the agent to the patient under the
	// relation rather than proposed as a fact, so it stays out of the positive walks yet is honoured
	// when the proposition is later assessed, overriding an inherited default. A denial with no
	// patient (an intransitive negative) is acknowledged but not yet stored.
	if frame.Negate {
		if patientID != 0 {
			if err := d.store.LinkRel(agent.Concept.ID, patientID, denyRel(rel.Content), w); err != nil {
				fmt.Fprintf(d.out, "denial store error: %v\n", err)
				return false
			}
			d.event(events.Event{Ev: "link", From: agent.Concept.ID, To: patientID, Rel: denyRel(rel.Content), Weight: w})
		}
		d.noteFocus(agent.Concept)
		return true
	}
	// Grounded disposes, a low-trust source only proposes: if GOLEM already holds a
	// grounded denial that contradicts a peer's claim, the claim is doubted. It is still
	// written, weak and source-tagged, so GOLEM remembers the claim was made and can
	// judge the source's track record by it, while the grounded denial keeps standing
	// over every read. The architect's tellings (full trust) are never doubted this way.
	doubted := false
	if trust < architectTrust && patient.Concept != nil {
		if b, _, _, _ := d.assessProposition(agent.Concept, rel.Content, patient.Concept); b == beliefDenied {
			doubted = true
			d.held.add(heldProp{
				status:       heldDoubted,
				rel:          rel.Content,
				agent:        agent.Concept.ID,
				patient:      patient.Concept.ID,
				agentGloss:   agent.Concept.Content,
				patientGloss: patient.Concept.Content,
				conf:         1,
			})
		}
	}
	if err := d.store.Propose(rel.ID, momentID, agent.Concept.ID, patientID, w); err != nil {
		fmt.Fprintf(d.out, "propose error: %v\n", err)
		return false
	}
	d.noteFocus(agent.Concept) // a told proposition leaves its subject as the topic
	if doubted {
		// The claim is on record and held as a doubt; GOLEM does not take it up as a
		// fact, so it neither leads the thread nor retires what it already believes.
		d.followUp = "I have heard otherwise, so I doubt that."
	} else {
		// If this telling answers a question GOLEM had to leave open, it follows up
		// with the answer it can now give rather than receiving the fact in silence.
		d.followUp = d.resolveOpenQuestions(frame.Lang, rel.Content, agent.Concept.ID, patientID)
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
	}
	// Warm what it bound, recording the role each filler played so the discourse
	// set can read its lean, and put the role edges on the trail so the write is
	// auditable like a naming is.
	d.ws.excite(rel.ID, exciteSpeech)
	d.ws.exciteRole(agent.Concept.ID, exciteSpeech, "agent")
	d.event(events.Event{Ev: "link", From: momentID, To: agent.Concept.ID, Rel: memory.RelAgent, Weight: w})
	if patientID != 0 {
		d.ws.exciteRole(patientID, exciteSpeech, "patient")
		d.event(events.Event{Ev: "link", From: momentID, To: patientID, Rel: memory.RelPatient, Weight: w})
	}
	// A role beyond the agent and patient that a construction binds (a recipient, an instrument)
	// is grounded the same way, written as a role edge labelled by its name, so a clause a learned
	// construction reads into more than two roles keeps every grounded role. The role name is the
	// edge, as the agent and patient ones are ("agent", "patient"); an unresolved extra role is
	// left for a later acquisition, the way the patient gap is.
	for role, t := range m.Roles {
		if role == "agent" || role == "patient" || t.Concept == nil {
			continue
		}
		if err := d.store.LinkRel(momentID, t.Concept.ID, role, w); err != nil {
			fmt.Fprintf(d.out, "role link error: %v\n", err)
			continue
		}
		d.ws.exciteRole(t.Concept.ID, exciteSpeech, role)
		d.event(events.Event{Ev: "link", From: momentID, To: t.Concept.ID, Rel: role, Weight: w})
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

// resembleAccept is the floor cosine for a known argument to count as resembling a
// relation's role-company. Above it the question path treats the relation as a
// candidate and disambiguates by which one the argument actually has facts for.
const resembleAccept = 0.6

// scoredRel is a relation and how much a clause's known argument resembles its
// role-company.
type scoredRel struct {
	id    uint64
	score float64
}

// rankRelationsByCompany scores every verbal relation by how much the given
// arguments resemble its role-centroids in the usage space, the agent against the
// agent-centroid and the patient against the patient-centroid, strongest first.
// With one role given (a question's gap leaves the other zero) it ranks on that
// role alone. The directional, role-aware match tells a predator-chases-prey
// relation from a symmetric one a mixed centroid could not.
func (d *Daemon) rankRelationsByCompany(agentID, patientID uint64) []scoredRel {
	var agentVec, patientVec []float32
	if agentID != 0 {
		agentVec, _ = d.store.Vector(agentID)
	}
	if patientID != 0 {
		patientVec, _ = d.store.Vector(patientID)
	}
	if agentVec == nil && patientVec == nil {
		return nil
	}
	agentCentroids, _ := d.store.UsageVectorsByRole(memory.RelAgent)
	patientCentroids, _ := d.store.UsageVectorsByRole(memory.RelPatient)
	var ranked []scoredRel
	for relID := range agentCentroids {
		if !d.verbalRelation(relID) {
			continue
		}
		var sum, roles float64
		if agentVec != nil {
			if ac, ok := agentCentroids[relID]; ok {
				sum += memory.Cosine(agentVec, ac)
				roles++
			}
		}
		if patientVec != nil {
			if pc, ok := patientCentroids[relID]; ok {
				sum += memory.Cosine(patientVec, pc)
				roles++
			}
		}
		if roles == 0 {
			continue
		}
		ranked = append(ranked, scoredRel{relID, sum / roles})
	}
	sort.Slice(ranked, func(i, j int) bool {
		if ranked[i].score != ranked[j].score {
			return ranked[i].score > ranked[j].score
		}
		return ranked[i].id < ranked[j].id
	})
	return ranked
}

// verbalRelation reports whether a relation can be the reading of a plain
// subject-verb-object clause, the only construction the resemblance fallback
// covers. A relation that carries its own construction is excluded: a locative or a
// possession. A gradable comparison is no longer a relation at all (it is an ordering
// on a scale), so it never reaches this ranking; the check keeps a verb from being read
// as "on" or "has" just because its arguments keep the same company those relations do.
func (d *Daemon) verbalRelation(relID uint64) bool {
	for _, mark := range []string{memory.RelLocative, memory.RelPossessive} {
		if e, err := d.store.LinksFromRel([]uint64{relID}, mark); err == nil && len(e) > 0 {
			return false
		}
	}
	return true
}

// meaningNamesParticipant reports whether a comprehended clause fills a role with
// a conversation participant, the signal that the clause is a deictic assertion
// whose roles the speaker fixed and word order should hold.
func (d *Daemon) meaningNamesParticipant(m grammar.Meaning) bool {
	for _, role := range []string{"agent", "patient"} {
		if t := m.Roles[role]; t.Concept != nil && d.isParticipant(t.Concept.ID) {
			return true
		}
	}
	return false
}

// storeComparison writes a told comparison "the cat is bigger than the mouse" as a
// canonical ordering Proposed on the gradable scale, decomposing the comparative surface
// into its pole and reading the un-gated pole role for the direction (invariant I1). The
// subject over standard for a greater pole, standard over subject for a lesser one, so one
// greater-over-lesser ordering per scale reads the same from both poles. It admits the gap
// (returns false, never denies, never mints) when the surface decomposes to no gradable
// pole, so a word never taught a scale binds nothing (invariant I4).
func (d *Daemon) storeComparison(frame grammar.Frame, momentID uint64, w float64) bool {
	subj, std := frame.Agent.Concept, frame.Patient.Concept
	// A numeral side of a told ordering grounds in its number concept, minted
	// on first contact, so the claim lands on the same node a computed check
	// reads and the nightly trust pass can judge the teller against the run.
	if subj == nil && numericLiteral(frame.Agent.Phrase) {
		subj = d.mintNounConcept(frame.Agent.Phrase, frame.Lang)
	}
	if std == nil && numericLiteral(frame.Patient.Phrase) {
		std = d.mintNounConcept(frame.Patient.Phrase, frame.Lang)
	}
	if subj == nil || std == nil {
		return false
	}
	pole, scaleID, sign, ok := d.comparativeToPole(frame.Action.Phrase, frame.Lang)
	if !ok {
		return false // the surface names no gradable pole: admit the gap, mint nothing
	}
	sf, err := d.store.FragmentsByIDs([]uint64{scaleID})
	if err != nil || len(sf) != 1 {
		return false
	}
	scale := sf[0]
	greater, lesser := canonicalOrdering(sign, subj, std)
	if err := d.store.Propose(scaleID, momentID, greater.ID, lesser.ID, w); err != nil {
		fmt.Fprintf(d.out, "propose error: %v\n", err)
		return false
	}
	d.noteFocus(subj) // the original subject stays the topic (invariant I2)
	d.recordThreadProp(scale.Content, greater.ID, lesser.ID, greater.Content, lesser.Content)
	d.ws.excite(scaleID, exciteSpeech)
	d.ws.exciteRole(greater.ID, exciteSpeech, "agent")
	d.ws.exciteRole(lesser.ID, exciteSpeech, "patient")
	d.event(events.Event{Ev: "link", From: momentID, To: greater.ID, Rel: memory.RelAgent, Weight: w})
	d.event(events.Event{Ev: "link", From: momentID, To: lesser.ID, Rel: memory.RelPatient, Weight: w})
	_ = pole
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

// planMultiRoleQuestion answers a question that gives more than one role of a relation and asks
// for another: "what did the dog show the cat" gives the agent and the recipient and asks the
// patient. It finds the proposition the given arguments all stand in and speaks it back whole, so
// the answer reads off the same grounded roles the construction stored. The roles are discovered
// from the proposition itself rather than parsed from position, so a clause a learned construction
// read into three roles is answered without naming the roles in code. It declines a question with
// fewer than two given arguments to the single-role path, and a question whose given arguments do
// not all meet in one proposition to the gap admission.
func (d *Daemon) planMultiRoleQuestion(frame grammar.Frame, raw string) (string, bool) {
	if frame.Wh != "what" && frame.Wh != "who" && frame.Wh != "which" {
		return "", false
	}
	rel, knowns := d.multiRoleQuestionParts(frame.Lang, raw)
	if rel == nil || len(knowns) < 2 {
		return "", false // the single-role path handles a question that gives one argument
	}
	roles, ok := d.matchProposition(rel, knowns)
	if !ok {
		return "", false // the given arguments never met in one proposition; let the gap stand
	}
	m := grammar.Meaning{
		Relation: grammar.Term{Phrase: d.relationName(rel.Content, frame.Lang)},
		Lang:     frame.Lang,
		Past:     frame.Past,
		Roles:    map[string]grammar.Term{},
	}
	for role, id := range roles {
		frags, err := d.store.FragmentsByIDs([]uint64{id})
		if err != nil || len(frags) != 1 {
			return "", false
		}
		m.Roles[role] = grammar.Term{Phrase: d.conceptName(id, frags[0].Content, frame.Lang), Definite: true}
	}
	line, ok := d.gram.Speak(frame.Lang, m)
	if !ok {
		return "", false
	}
	d.noteFocus(knowns[0])
	return line, true
}

// multiRoleQuestionParts reads a role question's relation and every argument it gives: the relation
// is the content word whose lemma names one, the given arguments the content words that ground in a
// non-relation concept. The asked role is left out, so "what did the dog show the cat" yields the
// show relation and the dog and the cat.
func (d *Daemon) multiRoleQuestionParts(lang, raw string) (rel *memory.Fragment, knowns []*memory.Fragment) {
	tokens, _ := grammar.Tokenize(raw)
	for _, t := range tokens {
		// A closed-class surface whose lemma names a learned relation is kept,
		// the same rescue the single-role reader gives a taught verb like "run".
		if !d.gram.IsContentWord(lang, t) && !d.gram.IsDeictic(lang, t) &&
			d.resolveRelationSurface(lang, t) == nil {
			continue
		}
		if rel == nil {
			if r := d.resolveRelationSurface(lang, t); r != nil {
				rel = r
				continue
			}
		}
		if c := d.resolveConcept(t); c != nil && c.Category != "relation" {
			knowns = append(knowns, c)
		}
	}
	return rel, knowns
}

// matchProposition finds a proposition of the relation that holds every given argument in some role
// and carries at least one more, returning all its roles so the answer can speak it whole. The roles
// are read off the moment's edges, each labelled by its name, and only concept fillers count, so a
// moment's succession links are not mistaken for roles. It returns false when no proposition holds
// all the given arguments together.
func (d *Daemon) matchProposition(rel *memory.Fragment, knowns []*memory.Fragment) (map[string]uint64, bool) {
	grounds, err := d.store.LinksFromRel([]uint64{rel.ID}, memory.RelGrounds)
	if err != nil {
		return nil, false
	}
	for _, g := range grounds {
		edges, err := d.store.LinksFrom([]uint64{g.To})
		if err != nil {
			continue
		}
		ids := make([]uint64, 0, len(edges))
		for _, e := range edges {
			ids = append(ids, e.To)
		}
		frags, err := d.store.FragmentsByIDs(ids)
		if err != nil {
			continue
		}
		isConcept := make(map[uint64]bool, len(frags))
		for _, f := range frags {
			isConcept[f.ID] = f.Kind == memory.KindConcept
		}
		roles := map[string]uint64{}
		present := map[uint64]bool{}
		for _, e := range edges {
			if isConcept[e.To] {
				roles[e.Rel] = e.To
				present[e.To] = true
			}
		}
		allGiven := true
		for _, k := range knowns {
			if !present[k.ID] {
				allGiven = false
				break
			}
		}
		if allGiven && len(roles) > len(knowns) {
			return roles, true
		}
	}
	return nil, false
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
	rel, known, rival, knownRole, verb := d.analyzeRoleQuestion(frame.Lang, raw)
	if rel != nil && known != nil {
		// Two referents fit the pro-form equally: ask which was meant rather
		// than write an answer on a hair's margin. The question is parked so
		// the next turn naming one of them answers it.
		if rival != nil {
			if ask, ok := d.askWhichReferent(frame, rel, known, rival, knownRole, verb); ok {
				return ask, true
			}
		}
		return d.answerRole(frame, rel, known, knownRole, verb)
	}
	// The verb names no relation. Read the question through the relation its known
	// argument's company most resembles, and answer hedged. Resemblance lives here,
	// on the question path: a query verb cannot be learned, so a fuzzy match aids
	// retrieval, where on the teaching path it would read a new verb as an old one.
	if cand, k, kRole, ok := d.resembleRoleQuestion(frame, raw); ok {
		return d.answerRoleResembled(frame, cand, k, kRole)
	}
	return "", false
}

// resembleRoleQuestion reads a role question whose verb names no relation: it finds
// the known argument and the relation whose company that argument most resembles,
// so a paraphrased or unknown verb can still retrieve. The role comes from
// position, as on the exact path: the known argument before the verb is the agent
// and the wh-word the patient, the verb before it reverses them.
func (d *Daemon) resembleRoleQuestion(frame grammar.Frame, raw string) (rel, known *memory.Fragment, knownRole string, ok bool) {
	tokens, _ := grammar.Tokenize(raw)
	var content []string
	for _, t := range tokens {
		if d.gram.IsContentWord(frame.Lang, t) || d.gram.IsDeictic(frame.Lang, t) {
			content = append(content, t)
		}
	}
	knownIdx, verbIdx := -1, -1
	for i, t := range content {
		if known == nil {
			if c := d.resolveConcept(t); c != nil && c.Category == "noun" {
				known, knownIdx = c, i
				continue
			}
		}
		if verbIdx < 0 && d.resolveConcept(t) == nil {
			verbIdx = i // an unknown word standing in the verb slot
		}
	}
	if known == nil || verbIdx < 0 {
		return nil, nil, "", false
	}
	knownRole = "agent"
	if verbIdx < knownIdx {
		knownRole = "patient"
	}
	var agentID, patientID uint64
	if knownRole == "agent" {
		agentID = known.ID
	} else {
		patientID = known.ID
	}
	// Among the relations the known argument resembles, answer from the strongest
	// one it actually has a fact for: retrieval needs the fact, and a fact the
	// argument genuinely stands in disambiguates better than a thin company margin,
	// where a question gives only one role and the other relations crowd it.
	for _, r := range d.rankRelationsByCompany(agentID, patientID) {
		if r.score < resembleAccept {
			break
		}
		frs, err := d.store.FragmentsByIDs([]uint64{r.id})
		if err != nil || len(frs) != 1 {
			continue
		}
		if len(d.gatherFacts(known, knownRole, frs[0].Content)) > 0 {
			return frs[0], known, knownRole, true
		}
	}
	return nil, nil, "", false
}

// answerRoleResembled answers a role question from a relation the verb only
// resembled, hedged, so the retrieval is offered as a guess: "Probably, the wolf
// chased the rabbit." It declines when the resembling relation holds no fact for
// the known argument, so the ordinary gap admission stands.
func (d *Daemon) answerRoleResembled(frame grammar.Frame, rel, known *memory.Fragment, knownRole string) (string, bool) {
	d.noteFocus(known)
	facts := d.gatherFacts(known, knownRole, rel.Content)
	if len(facts) == 0 {
		return "", false
	}
	name := d.conceptName(known.ID, known.Content, frame.Lang)
	line, ok := d.renderFacts(frame.Lang, name, facts, frame.Past, false)
	if !ok {
		return "", false
	}
	hedge := d.gram.Word(frame.Lang, "probably")
	if hedge == "" {
		return line, true
	}
	return upperFirst(hedge) + ", " + lowerFirst(line), true
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
		// The relation is known but nothing filled the questioned role: the honest
		// answer is the gap, spoken through the older admission and trailed by the
		// teaching invitation, and the question is held open so a later telling
		// that fills it can be followed up.
		d.recordOpenQuestion(rel.Content, known.ID, knownRole)
		return d.withInvite(frame.Lang, d.realizeAs(frame.Lang, grammar.Frame{Force: grammar.ForceStatement, Action: grammar.Term{Phrase: "know"},
			Negate: true, Past: past, Patient: grammar.Term{Phrase: d.gram.WhSurface(frame.Lang, frame.Wh)}}, d.chem.Snapshot())), true
	}
	for _, f := range facts {
		d.ws.excite(f.coID, exciteSpeech)
	}
	// A report resting only on tellings below full trust is hedged the way the polar
	// answer is: the fact is on record, but from a source GOLEM does not fully trust,
	// so the clause leads with the tongue's hedge rather than reading as witnessed.
	trusted := false
	for _, f := range facts {
		a, p := known.ID, f.coID
		if f.focusRole == "patient" {
			a, p = f.coID, known.ID
		}
		if d.propositionTrust(a, f.rel, p) >= architectTrust {
			trusted = true
			break
		}
	}
	if !trusted {
		line = upperFirst(d.gram.Word(frame.Lang, "probably")) + ", " + lowerFirst(line)
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

// planMultiHop answers a nested question "who chased what the dog chased" by
// composing two relational hops. An object-fronted clause like "what the dog
// chased" does not parse on its own (the parser leans on an auxiliary to flag the
// verb), so the pieces are read straight from the tokens: the inner clause is its
// subject plus its final verb, and the outer is the leading interrogative plus its
// verb. The first hop walks the inner relation from its subject to its patients;
// the second walks the outer relation to the agents that stand to each. It admits
// the gap when a part does not resolve or no fact carries through.
func (d *Daemon) planMultiHop(frame grammar.Frame, raw string) (string, bool) {
	if frame.Inner == "" {
		return "", false
	}
	dontKnow := func() (string, bool) {
		return d.realizeAs(frame.Lang, grammar.Frame{Force: grammar.ForceStatement, Action: grammar.Term{Phrase: "know"},
			Negate: true, Patient: grammar.Term{Phrase: d.gram.WhSurface(frame.Lang, frame.Wh)}}, d.chem.Snapshot()), true
	}
	all, _ := grammar.Tokenize(raw)
	inner, _ := grammar.Tokenize(frame.Inner)
	if len(inner) < 3 || len(all) < len(inner)+2 {
		return dontKnow()
	}
	// Inner "what the dog chased": the interrogative fronts the object, so the
	// subject is the agent and the final verb names the relation.
	innerRel := d.resolveRelation(d.gram.Lemma(frame.Lang, inner[len(inner)-1]))
	innerSubj := d.resolveConcept(strings.Join(inner[1:len(inner)-1], " "))
	// Outer "who chased ...": the leading interrogative asks the agent, its verb the
	// relation, and the inner answer fills the patient.
	outer := all[:len(all)-len(inner)]
	outerRel := d.resolveRelation(d.gram.Lemma(frame.Lang, outer[len(outer)-1]))
	if innerRel == nil || innerSubj == nil || outerRel == nil {
		return dontKnow()
	}
	// The two-hop composition now runs as the nested-query rule (multiHopReach), the graph-walked
	// counterpart of the hop-1-then-hop-2 walk; multiHopReachGo is the fallback. The parse above
	// stays here because a nested clause does not parse standalone.
	facts, focus, ok := d.multiHopReach(innerSubj.ID, innerRel.Content, outerRel.Content)
	if !ok || len(facts) == 0 {
		return dontKnow()
	}
	var names []string
	for _, f := range facts {
		names = append(names, d.conceptName(f.coID, f.coGloss, frame.Lang))
	}
	if focus != 0 {
		if ff, err := d.fragmentsByIDsTiered([]uint64{focus}); err == nil && len(ff) == 1 {
			d.noteFocus(ff[0])
		}
	}
	return upperFirst(d.coordinatedNP(frame.Lang, names)) + ".", true
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
	// "Do" asks for actions, so a state the gather also turns up is left out: a comparison
	// answers "how big", a placement answers "where", neither is a thing the agent did.
	actions := facts[:0]
	for _, f := range facts {
		if d.factIsComparative(f.rel) || d.factIsLocative(f.rel) {
			continue
		}
		actions = append(actions, f)
	}
	facts = actions
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
	// A describe move is named by its verb ("describe the dog", "köpeği anlat"). Its subject is
	// the clause's patient, read off the frame the way a clarify reads its own, so it fires in
	// any tongue the verb is seeded in. The "tell me about" and "know about" phrasings still
	// read the raw surface until they route off the recall and know verbs.
	focus := d.describeFocus(frame)
	if focus == nil {
		ref, ok := describeReferent(raw)
		if !ok {
			return "", false
		}
		if focus = d.resolveConcept(ref); focus == nil {
			return "", false
		}
	}
	d.turnFocus = focus.ID
	return d.describeConcept(frame, focus), true
}

// describeFocus is the concept a describe move points at: the patient of a clause whose verb
// is a describe speech act ("describe the dog"), resolved from the parsed concept or its
// phrase. A participant patient ("describe yourself") is not a describe target here. The
// recall verb pointed at a thing ("tell me about X") still reads the surface, deferred.
func (d *Daemon) describeFocus(frame grammar.Frame) *memory.Fragment {
	if frame.Speech != "describe" {
		return nil
	}
	return d.patientFocus(frame)
}

// patientFocus resolves a clause's patient to the concept it names, through the tongue's case
// stems, nil when the patient is empty or a conversation participant. It is the focus a move
// pointed at a thing acts on: the describe verb's object, the "what do you know about X" topic.
func (d *Daemon) patientFocus(frame grammar.Frame) *memory.Fragment {
	focus := frame.Patient.Concept
	if focus == nil && frame.Patient.Phrase != "" {
		focus = d.resolveCased(frame.Patient.Phrase, frame.Lang)
	}
	if focus == nil || d.isParticipant(focus.ID) {
		return nil
	}
	return focus
}

// resolveCased resolves a phrase to its concept, falling back to the case stems of the
// clause's tongue so a cased object ("köpeği", "puellam") names the same concept its bare
// form does. A patient read off a clause keeps its surface case, which the bare resolve does
// not see through.
func (d *Daemon) resolveCased(phrase, lang string) *memory.Fragment {
	if r := d.resolveConceptLang(phrase, lang); r != nil {
		return r
	}
	for _, base := range d.gram.CaseStems(lang, phrase) {
		if r := d.resolveConceptLang(base, lang); r != nil {
			return r
		}
	}
	return nil
}

// describeConcept says what a concept is and what it has been part of: its kind,
// then the propositions it stands in (the thread winning over the entrenched
// graph), or a hedged guess from a near neighbour when nothing is observed. It is
// the renderer behind "tell me about X" and a recall of the subject in mind.
// sortalWord names the bare kind a definition predicates, "X is a ___", in the
// asked tongue: a thing or an action, "bir şey"/"bir eylem" in Turkish, "res"/
// "actio" in Latin, so a description does not leak the English sortal.
func sortalWord(lang, kind string) string {
	switch lang {
	case "tr":
		if kind == "action" {
			return "bir eylem"
		}
		return "bir şey"
	case "la":
		if kind == "action" {
			return "actio"
		}
		return "res"
	}
	return grammar.Indefinite(kind)
}

func (d *Daemon) describeConcept(frame grammar.Frame, focus *memory.Fragment) string {
	chem := d.chem.Snapshot()
	action := focus.Category == "verb" || focus.Category == "relation"
	kind := "thing"
	if action {
		kind = "action"
	}
	kindWord := sortalWord(frame.Lang, kind)
	// The grounded kind speaks over the bare sortal here as it does in the
	// definition: a description of the fox opens "Fox is an animal", and only
	// a concept the hierarchy holds nothing for stays a thing.
	if !action {
		if cat := d.groundedKindOf(focus.ID); cat != nil {
			catName := d.conceptName(cat.ID, cat.Content, frame.Lang)
			if frame.Lang == "tr" {
				kindWord = "bir " + catName
			} else {
				kindWord = grammar.Indefinite(catName)
			}
		}
	}
	name := d.conceptName(focus.ID, focus.Content, frame.Lang)
	parts := []string{d.definitionClause(frame.Lang, name, kindWord, chem)}
	// What is observed about the focus, elaborated across more of its relations than a focused
	// question answers, the firm part of the description.
	observed := map[string]bool{}
	if facts := d.gatherFacts(focus, "", ""); len(facts) > 0 {
		// Salience decides how much is said (strongest first, tail dropped once it thins,
		// capped only against a hub); the plan then orders the kept facts by aspect so like is
		// spoken with like. Both steps work on meaning; the words come at realization.
		ordered := d.planDescription(salientFacts(facts, describeClauseCap))
		if line, ok := d.renderFactsCapped(frame.Lang, name, ordered, true, true, describeClauseCap); ok {
			parts = append(parts, line)
		}
		for _, f := range facts {
			d.ws.excite(f.coID, exciteSpeech)
			observed[f.rel] = true // the full set marks what is observed, so the hedge below adds only fresh relations
		}
		d.claimFacts(focus, ordered) // a description asserts what it actually said
	}
	// Then what the focus probably also does, from a near neighbour or a category it belongs
	// to, for relations it has not been observed in, so the description shades from what is
	// known to what is likely and stops at what can be grounded. When nothing is observed this
	// hedged guess is the whole of what can be said, the focus carried as a pronoun after the
	// definition just spoken.
	if hedged, ok := d.hedgedElaboration(frame, focus, observed); ok {
		parts = append(parts, hedged)
	}
	d.warmSpoken(focus.Content)
	return strings.Join(parts, " ")
}

// hedgedElaboration offers what a focus probably also does, beyond what was observed of it: the
// facts a near neighbour stands in (or a category it belongs to does), filtered to relations
// the focus has not been seen in, rendered behind the tongue's "probably". It returns false
// when no fresh likely fact can be drawn, so a description never pads with an empty guess.
func (d *Daemon) hedgedElaboration(frame grammar.Frame, focus *memory.Fragment, observed map[string]bool) (string, bool) {
	kept, _, ok := d.inferredFacts(focus, "agent", "")
	if !ok {
		if kept, ok = d.inheritedFacts(focus, "agent", ""); !ok {
			return "", false
		}
	}
	fresh := kept[:0]
	for _, f := range kept {
		if !observed[f.rel] {
			fresh = append(fresh, f)
		}
	}
	if len(fresh) == 0 {
		return "", false
	}
	// One clause only: the hedge prefixes the line, so a second guessed clause would trail
	// behind it reading as firm. The single likeliest guess keeps the shade honest.
	line, ok := d.renderFactsCapped(frame.Lang, d.conceptName(focus.ID, focus.Content, frame.Lang), fresh, true, true, 1)
	if !ok {
		return "", false
	}
	hedge := d.hedgeWord(frame.Lang, topConf(fresh))
	if hedge == "" {
		return line, true
	}
	return upperFirst(hedge) + ", " + lowerFirst(line), true
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

// planClarify answers a request to say more about what GOLEM just said ("what do
// you mean"). When the last thing it said was a guess, it explains the reasoning:
// the resemblance the guess rode on and the fact behind it. Otherwise it
// elaborates the subject in mind, its kind and what it has been part of. It
// returns false when the turn is not such a request or nothing is in mind.
func (d *Daemon) planClarify(frame grammar.Frame) (string, bool) {
	// The clause is a clarify move when its verb is one ("what do you mean",
	// "explain"). The same verb naming a concrete patient ("I meant the cat") is a
	// repair of the referent, so it is left to that path rather than caught here.
	if frame.Speech != "clarify" {
		return "", false
	}
	if frame.Patient.Concept != nil && !d.isParticipant(frame.Patient.Concept.ID) {
		return "", false
	}
	// A guess just made is clarified by saying why: "the rabbit is like a mouse,
	// and the mouse chased the cat." The reasoning is rendered for English; other
	// tongues fall back to elaborating the subject until a resemblance clause is
	// built for them.
	if frame.Lang == "en" && len(d.claims) > 0 {
		if c := d.claims[0]; c.affirmed && c.hedged {
			if line, ok := d.explainGuess(frame.Lang, c); ok {
				return line, true
			}
		}
	}
	if d.topic == 0 {
		return "", false
	}
	frags, err := d.store.FragmentsByIDs([]uint64{d.topic})
	if err != nil || len(frags) != 1 {
		return "", false
	}
	return d.describeConcept(frame, frags[0]), true
}

// planRepairFact handles a correction of a claim GOLEM made: "no, the dog chased
// the rabbit" retracts the conflicting claim, in mind, in long-term memory, in the
// graph, and in the discourse record, then teaches the corrected fact and confirms
// its updated understanding. It returns false when the turn is not a correction or
// its clause is not a proposition, so an ordinary statement still teaches plainly.
func (d *Daemon) planRepairFact(frame grammar.Frame, raw string, momentID uint64) (string, bool) {
	clause, ok := correctionClause(raw)
	if !ok || momentID == 0 {
		return "", false
	}
	// A correction states its direction explicitly, so role-fit is given no say
	// (as with a polar question): "no, the dog chased the rabbit" means the dog
	// chased the rabbit, not the order role-fit would prefer on a trained graph.
	m, ok := d.gram.Comprehend(frame.Lang, clause, d.resolveIn(frame.Lang), nil)
	if !ok {
		return "", false
	}
	agent, patient := m.Roles["agent"], m.Roles["patient"]
	if agent.Concept == nil || patient.Concept == nil {
		return "", false // not a proposition to correct toward
	}
	rel, err := d.relationConcept(m.Relation, frame.Lang)
	if err != nil || rel == nil {
		return "", false
	}
	// A claim GOLEM made about the same subject and relation, but a different
	// object, is what the correction overturns. Retract it everywhere it is held.
	for _, c := range append([]claim(nil), d.claims...) {
		if !c.affirmed || c.agent != agent.Concept.ID || c.rel != rel.Content || c.patient == patient.Concept.ID {
			continue
		}
		d.held.forget(c.rel, c.agent, c.patient)
		if err := d.store.ForgetBelief(c.rel, c.agent, c.patient); err != nil {
			fmt.Fprintf(d.out, "belief retire error: %v\n", err)
		}
		retracted := d.retractProposition(c.rel, c.agent, c.patient)
		d.removeThreadProp(c.rel, c.agent, c.patient)
		d.removeClaim(c.rel, c.agent, c.patient)
		// The retracted claim's own proof goes with it, and every claim that
		// drew on the retracted moments is cut at its support: the guesses
		// that leaned on the overturned fact retract too, not only the claims
		// that named it.
		for _, t := range d.claimTraces(c.rel, c.agent, c.patient) {
			d.dropTrace(t)
		}
		d.cutSupport(retracted)
	}
	// Write the corrected fact in the asked direction, and remember the exchange
	// set it down; any guess about it retires, now that it is observed.
	if err := d.store.Propose(rel.ID, momentID, agent.Concept.ID, patient.Concept.ID, comprehendWeight); err != nil {
		fmt.Fprintf(d.out, "propose error: %v\n", err)
		return "", false
	}
	d.noteFocus(agent.Concept)
	d.recordThreadProp(rel.Content, agent.Concept.ID, patient.Concept.ID, agent.Concept.Content, patient.Concept.Content)
	d.held.forget(rel.Content, agent.Concept.ID, patient.Concept.ID)
	d.store.ForgetBelief(rel.Content, agent.Concept.ID, patient.Concept.ID)
	// Confirm the updated understanding.
	isee := d.gram.Word(frame.Lang, "isee")
	confirm := ""
	if af, err := d.store.FragmentsByIDs([]uint64{agent.Concept.ID}); err == nil && len(af) == 1 {
		if line, ok := d.renderFacts(frame.Lang, d.conceptName(agent.Concept.ID, agent.Concept.Content, frame.Lang),
			d.gatherFacts(af[0], "agent", rel.Content), true, false); ok {
			confirm = line
		}
	}
	switch {
	case isee != "" && confirm != "":
		return upperFirst(isee) + ". " + confirm, true
	case confirm != "":
		return confirm, true
	case isee != "":
		return upperFirst(isee) + ".", true
	default:
		return "", true
	}
}

// retractProposition removes a proposition from the graph: it finds the moments in
// which the agent acted on the patient under the relation and drops their role
// edges, so a fact corrected away no longer reads back. It returns the moments it
// retracted, so the claims that drew on them can be cut with the fact.
func (d *Daemon) retractProposition(rel string, agent, patient uint64) []uint64 {
	relC := d.resolveRelation(rel)
	moments, err := d.store.LinksToRel([]uint64{agent}, memory.RelAgent)
	if err != nil {
		return nil
	}
	var retracted []uint64
	for _, mm := range moments {
		pats, err := d.store.LinksFromRel([]uint64{mm.From}, memory.RelPatient)
		if err != nil {
			continue
		}
		hasPatient := false
		for _, p := range pats {
			if p.To == patient {
				hasPatient = true
				break
			}
		}
		if !hasPatient {
			continue
		}
		if relC != nil {
			// The relation grounds INTO the moment (Propose lays rel -> moment),
			// so the moment's grounding is read from its incoming edges; read
			// outgoing, this check matched nothing and the retraction was a
			// silent no-op the discourse thread masked within the exchange.
			grounds, err := d.store.LinksToRel([]uint64{mm.From}, memory.RelGrounds)
			if err != nil {
				continue
			}
			matches := false
			for _, g := range grounds {
				if g.From == relC.ID {
					matches = true
					break
				}
			}
			if !matches {
				continue
			}
		}
		d.store.DropRelFrom([]uint64{mm.From}, memory.RelAgent)
		d.store.DropRelFrom([]uint64{mm.From}, memory.RelPatient)
		retracted = append(retracted, mm.From)
	}
	return retracted
}

// planIsA answers an is-a question against the category hierarchy: "is a cat an
// animal" holds because the animal category grounds in the cat, climbing the chain
// so "is a cat a thing" holds through animal. An identity ("is a cat a cat") holds,
// and anything else is denied plainly. It returns false when the turn is not an
// is-a question or its terms ground in nothing.
func (d *Daemon) planIsA(frame grammar.Frame) (string, bool) {
	// An is-a question is a copular question with two grounded arguments: "is a cat
	// an animal". The copula and the arguments are read from comprehension, not a
	// surface template, so any phrasing that parses to them is answered alike. One
	// grounded argument is a definition ("what is a cat"), left to its own path.
	if frame.Force != grammar.ForceQuestion || frame.Action.Phrase != "be" {
		return "", false
	}
	// A copula question carrying a locative preposition asks a place ("is the leaf
	// on the tree"), not a category, so it is left to the polar path which reads the
	// preposition and assesses the placed proposition.
	if frame.LocPrep != "" {
		return "", false
	}
	// Subject-auxiliary inversion puts the subject ("a cat") in the patient slot
	// and the predicate ("an animal") in the agent slot, so the member is the
	// patient and the category the agent. If the slots came the other way, the
	// abstraction is the category whichever side it landed.
	member, cat := frame.Patient.Concept, frame.Agent.Concept
	if member == nil || cat == nil {
		return "", false
	}
	if d.isCategory(member) && !d.isCategory(cat) {
		member, cat = cat, member
	}
	// Grounded membership answers firmly; a membership only TOLD answers a hedged yes,
	// since it rests on the telling, not on what GOLEM has seen the thing to be. A
	// participant's told identity needs no grown category: "are you a daemon" reads
	// the defeasible edge the telling laid on the self.
	grounded := member.ID == cat.ID || (d.isCategory(cat) && d.isA(member.ID, cat.ID))
	told := !grounded && (d.isCategory(cat) || d.isParticipant(member.ID)) && d.toldIsA(member.ID, cat.ID)
	yes := grounded || told
	// An affirmed membership leaves a bare trace: the membership machinery is
	// its own record, so the probe re-reads it live (the tally, or the telling)
	// rather than a stored copy. The "isa" word keys the claim, since no
	// relation concept carries a membership.
	if yes {
		d.traceClaim("isa", member.ID, cat.ID, "", nil)
	}
	// A participant speaks in its own person: "Probably, I am a daemon", never
	// "an I is a daemon", the article and agreement of a thing-noun.
	if d.isParticipant(member.ID) {
		word := upperFirst(d.gram.Word(frame.Lang, "yes"))
		if told {
			word = upperFirst(d.gram.Word(frame.Lang, "probably"))
		} else if !yes {
			word = upperFirst(d.gram.Word(frame.Lang, "no"))
		}
		return word + ", " + lowerFirst(d.sayParticipantIsA(frame.Lang, member.ID, cat, yes, told)), true
	}
	memberName := grammar.Indefinite(d.conceptName(member.ID, member.Content, frame.Lang))
	catName := grammar.Indefinite(d.conceptName(cat.ID, cat.Content, frame.Lang))
	clause := d.realizeAs(frame.Lang, grammar.Frame{Force: grammar.ForceStatement,
		Agent: grammar.Term{Phrase: memberName}, Action: grammar.Term{Phrase: "be"},
		Patient: grammar.Term{Phrase: catName}, Negate: !yes}, d.chem.Snapshot())
	if told {
		return upperFirst(d.gram.Word(frame.Lang, "probably")) + ", " + lowerFirst(clause), true
	}
	word := d.gram.Word(frame.Lang, "yes")
	if !yes {
		word = d.gram.Word(frame.Lang, "no")
	}
	return upperFirst(word) + ", " + lowerFirst(clause), true
}

// toldIsA reports whether GOLEM was told the member belongs to the category, directly or
// through a told membership in a category grounded under it (told a zebra is an animal,
// and an animal grounded under a thing, a zebra is told to be a thing too). It reads the
// told-membership edges alone, never the groundings the categories are built from, so a
// told fact answers defeasibly without joining the grounded hierarchy.
func (d *Daemon) toldIsA(memberID, catID uint64) bool {
	edges, err := d.store.LinksFromRel([]uint64{memberID}, memory.RelToldIsA)
	if err != nil {
		return false
	}
	for _, e := range edges {
		if e.To == catID || d.isA(e.To, catID) {
			return true
		}
	}
	return false
}

// planToldIsA records a membership GOLEM is told: "a zebra is an animal" links the zebra
// to the animal category defeasibly, so an is-a question reads a hedged yes, WITHOUT the
// told fact feeding the grounded category machinery (a told membership is never a
// grounding, so it cannot pull a thing into a category the way a lived percept does). It
// fires only on a copular statement whose subject is a known concept and whose predicate
// is a category the subject does not already ground under; a naming ("they are fruit"), a
// comparison, a placement, or an already-grounded membership passes through. The record is
// gated and scaled by the source's trust: a source GOLEM has come to distrust (below the
// trust a freshly-met peer starts at) cannot inject a category membership at all, and a
// low-trust source's edge is written weak so it fades unless the standing is earned, the
// same defeasible treatment every told claim gets.
func (d *Daemon) planToldIsA(frame grammar.Frame, momentID uint64) (string, bool) {
	if frame.Force != grammar.ForceStatement || frame.Action.Phrase != "be" ||
		frame.Negate || frame.Comparative || frame.LocPrep != "" {
		return "", false
	}
	trust := d.momentTrust(momentID)
	if trust < defaultSourceTrust {
		return "", false // a distrusted source's category claim is not taken as membership
	}
	member, cat := frame.Agent.Concept, frame.Patient.Concept
	// A participant can be TOLD an identity over a plain noun: "you are a
	// daemon" needs no grown category, since the self is one concrete thing
	// and the predicate names what it is said to be. The membership is held
	// on the same defeasible edge, so it answers hedged like any telling.
	selfIdentity := member != nil && d.isParticipant(member.ID)
	if selfIdentity {
		if cat == nil {
			cat = d.mintNounConcept(frame.Patient.Phrase, frame.Lang)
		}
		if cat == nil || d.isCategory(cat) && d.isA(member.ID, cat.ID) ||
			cat.Category == "relation" || d.isParticipant(cat.ID) {
			return "", false
		}
	} else {
		if cat == nil || !d.isCategory(cat) {
			return "", false // the predicate is not a known category: a naming or a property, not membership
		}
		if member != nil && (d.isCategory(member) || member.ID == cat.ID || d.isA(member.ID, cat.ID)) {
			return "", false // a category subject, an identity, or already grounded: leave it
		}
		if member == nil {
			// The thing is new: introduce it as a concept so the told membership has something
			// to hold, the way an ordinary told proposition mints an unknown subject.
			member = d.mintNounConcept(frame.Agent.Phrase, frame.Lang)
			if member == nil {
				return "", false
			}
		}
	}
	if err := d.store.LinkRel(member.ID, cat.ID, memory.RelToldIsA, comprehendWeight*trust); err != nil {
		return "", false
	}
	d.noteFocus(member)
	if selfIdentity {
		return upperFirst(d.gram.Word(frame.Lang, "isee")) + ". " +
			d.sayParticipantIsA(frame.Lang, member.ID, cat, true, true), true
	}
	memberName := grammar.Indefinite(d.conceptName(member.ID, member.Content, frame.Lang))
	catName := grammar.Indefinite(d.conceptName(cat.ID, cat.Content, frame.Lang))
	clause := d.realizeAs(frame.Lang, grammar.Frame{Force: grammar.ForceStatement,
		Agent: grammar.Term{Phrase: memberName}, Action: grammar.Term{Phrase: "be"},
		Patient: grammar.Term{Phrase: catName}}, d.chem.Snapshot())
	return upperFirst(d.gram.Word(frame.Lang, "isee")) + " " + lowerFirst(clause), true
}

// planComparative answers a comparison "is the cat bigger than the mouse" against
// the comparisons it was told, walked transitively so an ordering chained across
// several tellings is followed: the cat is bigger than the mouse and the mouse
// than the snail, so the cat is bigger than the snail. The ordering lives in the
// told facts, not a scale spelled in code, and the walk is sound, so the answer is
// firm like the categorical deduction. It returns false when the clause is not a
// comparison question, so the older handling stands.
func (d *Daemon) planComparative(frame grammar.Frame) (string, bool) {
	if !frame.Comparative || frame.Force != grammar.ForceQuestion {
		return "", false
	}
	// Bare numerals settle by computing when the gradable has a taught pole:
	// GOLEM's own run outranks any told ordering, so the check runs first.
	if line, ok := d.compareNumbers(frame); ok {
		return line, true
	}
	subj, std := frame.Agent.Concept, frame.Patient.Concept
	if subj == nil || std == nil {
		return "", false // a side of the comparison grounds in nothing; admit the gap
	}
	d.noteFocus(subj)
	gap := func() (string, bool) {
		// Nothing places the two on this dimension: no pole taught, no scale, or
		// the surface names no gradable. GOLEM admits the gap rather than deny an
		// ordering it never heard (invariant I4).
		return d.realizeAs(frame.Lang, grammar.Frame{Force: grammar.ForceStatement,
			Action: grammar.Term{Phrase: "know"}, Negate: true}, d.chem.Snapshot()), true
	}
	pole, scaleID, sign, ok := d.comparativeToPole(frame.Action.Phrase, frame.Lang)
	if !ok {
		return gap()
	}
	sf, err := d.store.FragmentsByIDs([]uint64{scaleID})
	if err != nil || len(sf) != 1 {
		return gap()
	}
	scale := sf[0]
	// The walk runs in the canonical greater-over-lesser direction: an lt sign swaps its
	// walk pair (the standard is the greater). Realization keeps the original subject,
	// standard, and asked pole for a genuine lesser pole (invariant I2), but a "less"
	// inversion of a greater pole (its own role disagrees with the flipped sign) speaks
	// the greater surface with subject and standard swapped, "Y is more X than Z".
	walkFrom, walkTo := subj.ID, std.ID
	if sign == "lt" {
		walkFrom, walkTo = std.ID, subj.ID
	}
	holds := subj.ID != std.ID && d.comparesTransitively(scale, walkFrom, walkTo)
	surface := d.realizeComparative(pole, frame.Lang, "")
	if surface == "" {
		return gap()
	}
	saySubj, sayStd := subj, std
	if own, ok := d.poleSign(pole); ok && own != sign {
		saySubj, sayStd = std, subj
	}
	line := d.sayComparison(frame.Lang, saySubj, surface, sayStd, holds)
	// An affirmed ordering leaves its proof, keyed on the scale and the canonical
	// greater/lesser pair the walk affirmed, so the evidence reads the right moment.
	if holds {
		greater, lesser := canonicalOrdering(sign, subj, std)
		d.traceClaim(scale.Content, greater.ID, lesser.ID, line,
			d.polarEvidence(scale.Content, greater.ID, lesser.ID, 0))
	}
	return line, true
}

// planMagnitude answers "how much bigger is X than Y" with the degree read from the
// told order: the rank gap between the two, sorted into a qualitative word. When the
// subject does not stand above the standard there is no degree to give, so it
// answers the plain comparison instead (denied or admitted).
func (d *Daemon) planMagnitude(frame grammar.Frame) (string, bool) {
	if !frame.Magnitude || frame.Force != grammar.ForceQuestion {
		return "", false
	}
	// Bare numerals answer with the metric amount their difference computes,
	// not a rank bucket, when the gradable has a taught pole.
	if line, ok := d.magnitudeNumbers(frame); ok {
		return line, true
	}
	subj, std := frame.Agent.Concept, frame.Patient.Concept
	if subj == nil || std == nil {
		return "", false
	}
	d.noteFocus(subj)
	dontKnow := func() (string, bool) {
		return d.realizeAs(frame.Lang, grammar.Frame{Force: grammar.ForceStatement,
			Action: grammar.Term{Phrase: "know"}, Negate: true}, d.chem.Snapshot()), true
	}
	pole, scaleID, sign, ok := d.comparativeToPole(frame.Action.Phrase, frame.Lang)
	if !ok {
		return dontKnow()
	}
	sf, err := d.store.FragmentsByIDs([]uint64{scaleID})
	if err != nil || len(sf) != 1 {
		return dontKnow()
	}
	scale := sf[0]
	surface := d.realizeComparative(pole, frame.Lang, "")
	if surface == "" {
		return dontKnow()
	}
	// The magnitude reads the rank gap in the canonical direction: an lt sign swaps its
	// walk pair so the reach is measured greater-to-lesser (invariant I2). A "less"
	// inversion of a greater pole speaks the greater surface with the pair swapped.
	fromID, toID := subj.ID, std.ID
	if sign == "lt" {
		fromID, toID = std.ID, subj.ID
	}
	saySubj, sayStd := subj, std
	if own, ok := d.poleSign(pole); ok && own != sign {
		saySubj, sayStd = std, subj
	}
	gap, ok := d.comparativeMagnitude(scale, fromID, toID)
	if !ok {
		return d.sayComparison(frame.Lang, saySubj, surface, sayStd, false), true
	}
	return d.sayMagnitude(frame.Lang, saySubj, surface, sayStd, degreeBucket(gap)), true
}

// comparesTransitively reports whether the from-concept stands above the to-concept on a
// scale or a told-transitive relation, chained transitively. The chaining walk now lives in
// the graph as the transitivity rule: it runs against the store through evalReachRule when
// the rule is present (the switch-over that makes this reasoning graph-authoritative), and
// the Go walk below serves as the fallback when no rule is stored or its shape is not one the
// evaluator runs. The rule reproduces the Go walk, verified over the comparison fixtures.
func (d *Daemon) comparesTransitively(frag *memory.Fragment, fromID, toID uint64) bool {
	if root := d.compRule(transContainsRuleTemplate, transContainsSpec()); root != 0 {
		if r, ok := d.evalRule(root, ruleQuery{from: fromID, to: toID, rel: frag.Content}); ok {
			return r.hit
		}
	}
	return d.comparesTransitivelyGo(frag, fromID, toID)
}

// comparesTransitivelyGo is the hardcoded chaining walk, the fallback for the transitivity
// rule above. The order is read from the canonical greater-over-lesser orderings Proposed on
// the scale (the greater stands as agent), so a single generic agent-direction walk serves
// it. A converse ("smaller") query swaps its walk pair before calling, so it reads the same
// orderings the greater pole wrote rather than a reversed second pole. The same walk answers
// an ordinary told-transitive relation, whose content is the relation rather than a scale.
func (d *Daemon) comparesTransitivelyGo(frag *memory.Fragment, fromID, toID uint64) bool {
	seen := map[uint64]bool{fromID: true}
	stack := []uint64{fromID}
	for len(stack) > 0 {
		cur := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		frags, err := d.store.FragmentsByIDs([]uint64{cur})
		if err != nil || len(frags) != 1 {
			continue
		}
		reach := func(id uint64) bool {
			if id == toID {
				return true
			}
			if !seen[id] {
				seen[id] = true
				stack = append(stack, id)
			}
			return false
		}
		// The things cur was told to exceed directly.
		for _, f := range d.gatherFacts(frags[0], "agent", frag.Content) {
			if reach(f.coID) {
				return true
			}
		}
	}
	return false
}

// comparativeReach returns every concept a concept stands above on a scale, directly or
// through the chain, so a concept's PLACE on the scale (its rank) is read from the canonical
// orderings. Like comparesTransitively it runs the graph-stored rule (evalReachSet, the
// collecting counterpart) when present, and falls back to the Go walk below otherwise.
func (d *Daemon) comparativeReach(frag *memory.Fragment, fromID uint64) map[uint64]bool {
	if root := d.compRule(transSetRuleTemplate, transSetSpec()); root != 0 {
		if r, ok := d.evalRule(root, ruleQuery{from: fromID, rel: frag.Content}); ok {
			return sliceToSet(r.set)
		}
	}
	return d.comparativeReachGo(frag, fromID)
}

// comparativeReachGo is the hardcoded collecting walk, the fallback for the reach rule above.
// It is the same walk comparesTransitivelyGo runs, collecting the whole set rather than
// stopping at one target, so a concept's rank (how many it exceeds) reads from the orderings.
func (d *Daemon) comparativeReachGo(frag *memory.Fragment, fromID uint64) map[uint64]bool {
	below := map[uint64]bool{}
	seen := map[uint64]bool{fromID: true}
	stack := []uint64{fromID}
	for len(stack) > 0 {
		cur := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		frags, err := d.store.FragmentsByIDs([]uint64{cur})
		if err != nil || len(frags) != 1 {
			continue
		}
		step := func(id uint64) {
			below[id] = true
			if !seen[id] {
				seen[id] = true
				stack = append(stack, id)
			}
		}
		for _, f := range d.gatherFacts(frags[0], "agent", frag.Content) {
			step(f.coID)
		}
	}
	delete(below, fromID)
	return below
}

// comparativeMagnitude is the ordinal distance the subject stands above the
// standard on a scale: the difference in how many each exceeds, read from the
// canonical order. The comparisons are ordinal, so this is a rank gap, not a
// metric amount, the honest degree the data supports. It returns false when the
// subject does not stand above the standard at all (nothing to measure).
func (d *Daemon) comparativeMagnitude(rel *memory.Fragment, subjID, stdID uint64) (int, bool) {
	if subjID == stdID || !d.comparesTransitively(rel, subjID, stdID) {
		return 0, false
	}
	return len(d.comparativeReach(rel, subjID)) - len(d.comparativeReach(rel, stdID)), true
}

// degreeBucket sorts a rank gap into a qualitative degree, since ordinal data can
// only say how far apart, not how many times: a one-step gap is slight, a wide gap
// is great, the middle plain. The buckets name a degree the tongue then speaks.
func degreeBucket(gap int) string {
	switch {
	case gap <= 1:
		return "slight"
	case gap >= 3:
		return "great"
	}
	return ""
}

// factIsComparative reports whether a gathered fact's relation is a gradable
// comparison, now that a comparison is an ordering on a scale rather than a fact hung
// off a per-tongue relation. Its content names a scale dimension (a concept with poles
// hung on it by RelScale), so it speaks as "X is bigger than Y" through the comparison
// query rather than as a plain verb ("X biggered Y") in a fact list.
func (d *Daemon) factIsComparative(relContent string) bool {
	return d.isScale(relContent)
}

// analogize completes a proportion a:b::c:?: it finds the relation that holds from
// a to b in the told facts, then the concepts c stands to in that same relation,
// the fourth term. Grounded and glass-box: the relation is explicit, no vector
// offset, so the analogy is the relation a:b shares with c:?. It returns the
// relation and c's co-arguments under it, or false when a stands in no told
// relation to b, or c stands in none of the relations a does. The find-then-apply
// walk now lives in the graph as the analogy rule and runs through the one evaluator
// (evalRule over the analogy composition); the Go walk (analogizeGo) is the fallback
// when no rule is stored or its shape is not one the evaluator runs.
func (d *Daemon) analogize(a, b, c *memory.Fragment) (string, []focusFact, bool) {
	if a == nil || b == nil || c == nil {
		return "", nil, false
	}
	if root := d.analogyRule(); root != 0 {
		if r, ok := d.evalRule(root, ruleQuery{from: a.ID, to: b.ID, third: c.ID}); ok {
			if !r.hit {
				return "", nil, false
			}
			return r.foundRel, d.gatherFacts(c, "agent", r.foundRel), true
		}
	}
	return d.analogizeGo(a, b, c)
}

// analogizeGo is the hardcoded find-then-apply walk, the fallback for the analogy rule above:
// it returns the first relation that holds a->b and that c also stands in, with c's
// co-arguments under it. The rule reproduces it, verified over the analogy fixtures.
func (d *Daemon) analogizeGo(a, b, c *memory.Fragment) (string, []focusFact, bool) {
	for _, ab := range d.gatherFacts(a, "agent", "") {
		if ab.coID != b.ID {
			continue // the relation must be the one that actually holds a -> b
		}
		if cf := d.gatherFacts(c, "agent", ab.rel); len(cf) > 0 {
			return ab.rel, cf, true
		}
	}
	return "", nil, false
}

// planAnalogy answers a proportion "A is to B as C is to what" with the fourth
// term: the relation that holds A->B applied to C. It admits the gap when a term
// grounds in nothing or no relation carries through, rather than force an answer.
func (d *Daemon) planAnalogy(frame grammar.Frame) (string, bool) {
	if !frame.Analogy || frame.Force != grammar.ForceQuestion {
		return "", false
	}
	a, b, c := frame.Agent.Concept, frame.Patient.Concept, frame.Third.Concept
	dontKnow := func() string {
		return d.realizeAs(frame.Lang, grammar.Frame{Force: grammar.ForceStatement,
			Action: grammar.Term{Phrase: "know"}, Negate: true}, d.chem.Snapshot())
	}
	if a == nil || b == nil || c == nil {
		return dontKnow(), true
	}
	d.noteFocus(c)
	_, ans, ok := d.analogize(a, b, c)
	if !ok {
		return dontKnow(), true
	}
	seen := map[uint64]bool{}
	var names []string
	for _, f := range ans {
		if !seen[f.coID] {
			seen[f.coID] = true
			names = append(names, d.conceptName(f.coID, f.coGloss, frame.Lang))
		}
	}
	if len(names) == 0 {
		return dontKnow(), true
	}
	return upperFirst(d.coordinatedNP(frame.Lang, names)) + ".", true
}

// sayComparison speaks a comparison's answer, the affirmation word and the clause the
// question stated, denied when the ordering does not hold: "Yes, the cat is bigger than
// the mouse" or "No, the cat is not bigger than the mouse". The rel is the realized
// comparative surface of the asked pole, spliced with the original subject and standard
// so a swapped walk pair never inverts the spoken clause (invariant I2). The copula and
// the standard marker come from the tongue's data.
func (d *Daemon) sayComparison(lang string, subj *memory.Fragment, rel string, std *memory.Fragment, holds bool) string {
	if lang == "tr" {
		// Turkish marks the standard ablative and follows the adjective with "değil"
		// to deny, no copula: "kedi fareden büyük", "kedi fareden büyük değil".
		clause := d.conceptName(subj.ID, subj.Content, lang) + " " +
			d.gram.Ablative(lang, d.conceptName(std.ID, std.Content, lang)) + " " + rel
		if !holds {
			clause += " " + d.gram.Word(lang, "notcompare")
		}
		word := d.gram.Word(lang, "yes")
		if !holds {
			word = d.gram.Word(lang, "no")
		}
		return upperFirst(word) + ", " + strings.TrimSpace(clause) + "."
	}
	def := func(name string) string {
		if a := d.gram.Article(lang); a != "" {
			return a + " " + name
		}
		return name
	}
	than := d.gram.Word(lang, "than")
	pred := strings.TrimSpace(rel + " " + than + " " + def(d.conceptName(std.ID, std.Content, lang)))
	clause := d.realizeAs(lang, grammar.Frame{Force: grammar.ForceStatement,
		Agent:   grammar.Term{Phrase: def(d.conceptName(subj.ID, subj.Content, lang))},
		Action:  grammar.Term{Phrase: "be"},
		Patient: grammar.Term{Phrase: pred}, Negate: !holds}, d.chem.Snapshot())
	word := d.gram.Word(lang, "yes")
	if !holds {
		word = d.gram.Word(lang, "no")
	}
	return upperFirst(word) + ", " + lowerFirst(clause)
}

// comparativeClause renders "SUBJ is REL than STD" as a plain statement ("the lion is bigger
// than the mouse"), the comparison spoken as a fact in a description rather than as the yes/no
// a question gets. When several gradable relations share a standard the predicates coordinate
// into one clause ("the mouse is lighter and slower than the cat"): the standard is named once
// and the directions are joined by the tongue's coordinator. Likewise several standards in one
// direction coordinate ("lighter than the cat and the dog"). The subject is "it" when a clause
// was already spoken. It returns false when the tongue has no comparison surface for the fact.
func (d *Daemon) comparativeClause(lang, subj string, dirs, stds []string, given bool) (string, bool) {
	if len(dirs) == 0 || len(stds) == 0 {
		return "", false
	}
	// Each direction is a pole's content ("big", "small"), language-agnostic through the
	// grouping and contrast logic; it realizes to the tongue's comparative surface here,
	// so the same plan reads in any tongue the poles carry a form in.
	surfaces := make([]string, 0, len(dirs))
	for _, dir := range dirs {
		pole := d.resolveConcept(dir)
		surface := d.realizeComparative(pole, lang, "")
		if surface == "" {
			return "", false
		}
		surfaces = append(surfaces, surface)
	}
	rel := d.joinCoordinated(lang, surfaces)
	if lang == "tr" {
		abl := make([]string, len(stds))
		for i, s := range stds {
			abl[i] = d.gram.Ablative(lang, s)
		}
		clause := subj + " " + d.joinCoordinated(lang, abl) + " " + rel
		return grammar.Sentence(strings.TrimSpace(clause)), true
	}
	if lang != "en" {
		return "", false // other tongues read a comparison through their own query for now
	}
	subjPhrase := definiteForm(lang, subj)
	if given {
		if pron := d.gram.Pronoun(lang, "agent"); pron != "" {
			subjPhrase = pron
		}
	}
	defs := make([]string, len(stds))
	for i, s := range stds {
		defs[i] = definiteForm(lang, s)
	}
	pred := strings.TrimSpace(rel + " " + d.gram.Word(lang, "than") + " " + d.joinCoordinated(lang, defs))
	line := d.realizeAs(lang, grammar.Frame{Force: grammar.ForceStatement,
		Agent:   grammar.Term{Phrase: subjPhrase},
		Action:  grammar.Term{Phrase: "be"},
		Patient: grammar.Term{Phrase: pred}}, d.chem.Snapshot())
	if line == "" {
		return "", false
	}
	return line, true
}

// joinCoordinated joins predicates with the tongue's coordinator ("lighter and slower", "daha
// hafif ve daha yavaş"), a comma before the last when there are three or more.
func (d *Daemon) joinCoordinated(lang string, items []string) string {
	switch len(items) {
	case 0:
		return ""
	case 1:
		return items[0]
	case 2:
		return items[0] + " " + d.gram.Coordinator(lang) + " " + items[1]
	default:
		return strings.Join(items[:len(items)-1], ", ") + " " + d.gram.Coordinator(lang) + " " + items[len(items)-1]
	}
}

// comparisonDir is the pole a comparison is spoken from, given as the pole's content
// ("big", "small") so the direction key stays language-agnostic through the grouping and
// contrast logic and realizes to a surface only at the clause. The focus is the greater
// pole when it stood as the greater (agent) on the scale, the lesser when it stood below
// (patient), so the same stored ordering reads "bigger" or "smaller".
func (d *Daemon) comparisonDir(f focusFact) string {
	side := "greater"
	if f.focusRole == "patient" {
		side = "lesser"
	}
	if p := d.scalePole(f.rel, side); p != nil {
		return p.Content
	}
	return f.rel
}

// comparisonScale names the scale a direction (a pole's content) lies on, the same for a
// pole and its opposite ("big" and "small" share one scale), so the two poles cluster in a
// description and a switch between them reads as a contrast. It reads the dimension via the
// pole's RelScale rather than a lexicographic proxy.
func (d *Daemon) comparisonScale(dir string) string {
	pole := d.resolveConcept(dir)
	if pole == nil {
		return dir
	}
	if scaleID, ok := d.scaleOf(pole); ok {
		if sf, err := d.store.FragmentsByIDs([]uint64{scaleID}); err == nil && len(sf) == 1 {
			return sf[0].Content
		}
	}
	return dir
}

// opposesDir reports whether two comparison directions (pole contents) are the opposite
// poles of one scale ("big" against "small"), the discourse relation a contrast connective
// realizes. They oppose when they share a scale and carry different pole roles.
func (d *Daemon) opposesDir(a, b string) bool {
	if a == "" || b == "" || a == b {
		return false
	}
	pa, pb := d.resolveConcept(a), d.resolveConcept(b)
	if pa == nil || pb == nil {
		return false
	}
	sa, oka := d.scaleOf(pa)
	sb, okb := d.scaleOf(pb)
	if !oka || !okb || sa != sb {
		return false
	}
	// The poles oppose only when they name opposite ends of the shared scale, so a pair of
	// same-role synonyms on one scale is not a contrast.
	ra, rok := d.poleSign(pa)
	rb, rbok := d.poleSign(pb)
	return rok && rbok && ra != rb
}

// sayMagnitude speaks a comparison's degree: the clause the question asked with the
// degree word set before the gradable relation, "The lion is much bigger than the
// mouse". A middling gap takes no word, reading as the plain comparative. The
// degree word, copula, and standard marker come from the tongue's data.
func (d *Daemon) sayMagnitude(lang string, subj *memory.Fragment, rel string, std *memory.Fragment, degree string) string {
	def := func(name string) string {
		if a := d.gram.Article(lang); a != "" {
			return a + " " + name
		}
		return name
	}
	deg := ""
	if degree != "" {
		deg = d.gram.Word(lang, degree)
		// A degree the tongue has no word for stands as itself: a computed
		// metric amount ("5") speaks literally where "slight" maps to
		// "slightly".
		if deg == "" {
			deg = degree
		}
	}
	than := d.gram.Word(lang, "than")
	pred := strings.TrimSpace(strings.TrimSpace(deg+" "+rel) + " " + than + " " + def(d.conceptName(std.ID, std.Content, lang)))
	clause := d.realizeAs(lang, grammar.Frame{Force: grammar.ForceStatement,
		Agent:   grammar.Term{Phrase: def(d.conceptName(subj.ID, subj.Content, lang))},
		Action:  grammar.Term{Phrase: "be"},
		Patient: grammar.Term{Phrase: pred}}, d.chem.Snapshot())
	return upperFirst(clause)
}

// inferenceBasis finds the neighbour a guess rode on: the concept the agent most
// resembles that observably stands in the relation to the patient, the "like" that
// explains a "probably". It returns false when no resembling neighbour does.
func (d *Daemon) inferenceBasis(agentID uint64, rel string, patientID uint64) (*memory.Fragment, bool) {
	sibs, err := d.featureSiblings(agentID, time.Now())
	if err != nil {
		return nil, false
	}
	sort.SliceStable(sibs, func(i, j int) bool { return sibs[i].weight > sibs[j].weight })
	for _, s := range sibs {
		if s.weight < inferenceFloor {
			break
		}
		if s.id == patientID {
			continue
		}
		sf, err := d.fragmentsByIDsTiered([]uint64{s.id})
		if err != nil || len(sf) != 1 {
			continue
		}
		if d.propositionHolds(sf[0], rel, patientID) {
			return sf[0], true
		}
	}
	return nil, false
}

// explainGuess voices why GOLEM guessed a proposition: the concept the agent is
// like, and what that concept did, so a hedged answer can be justified rather than
// only repeated. It returns false when the guess has no observed neighbour to point
// to (a chained or supposition-backed guess, or one whose basis has since changed).
func (d *Daemon) explainGuess(lang string, c claim) (string, bool) {
	agentFrags, err := d.store.FragmentsByIDs([]uint64{c.agent})
	if err != nil || len(agentFrags) != 1 {
		return "", false
	}
	agentName := d.conceptName(c.agent, agentFrags[0].Content, lang)
	// A category-inheritance guess rests on membership, not resemblance: the agent
	// belongs to a category that stands in the relation, so the guess is explained
	// by the is-a and the category-level fact. Tried first, the order
	// assessProposition draws the guess in.
	if catID, ok := d.categoryInherits(agentFrags[0], c.rel, c.patient); ok {
		if line, ok := d.explainByCategory(lang, agentName, catID, c.rel); ok {
			return line, true
		}
	}
	sib, ok := d.inferenceBasis(c.agent, c.rel, c.patient)
	if !ok {
		return "", false
	}
	sibName := d.conceptName(sib.ID, sib.Content, lang)
	// "<agent> is like a <sibling>." The copula and the seeded "like" carry it.
	like := strings.TrimSpace(d.gram.Word(lang, "islike") + " " + grammar.Indefinite(sibName))
	res := d.realizeAs(lang, grammar.Frame{Force: grammar.ForceStatement,
		Agent: grammar.Term{Phrase: agentName}, Action: grammar.Term{Phrase: "be"},
		Patient: grammar.Term{Phrase: like}}, d.chem.Snapshot())
	// "<sibling> chased <patient>." The fact the guess rests on.
	if ev, ok := d.renderFacts(lang, sibName, d.gatherFacts(sib, "agent", c.rel), true, false); ok {
		return res + " " + ev, true
	}
	return res, true
}

// explainByCategory voices the basis of a category-inheritance guess: the is-a
// that places the agent in the category, then the category-level fact the guess
// inherited. "Cat is an animal. The animal chased the mouse." The membership is
// the firm part, the inherited fact the defeasible part, so the explanation is
// honest about what carried the "probably."
func (d *Daemon) explainByCategory(lang, agentName string, catID uint64, rel string) (string, bool) {
	catFrags, err := d.store.FragmentsByIDs([]uint64{catID})
	if err != nil || len(catFrags) != 1 {
		return "", false
	}
	catName := d.conceptName(catID, catFrags[0].Content, lang)
	isA := d.realizeAs(lang, grammar.Frame{Force: grammar.ForceStatement,
		Agent: grammar.Term{Phrase: agentName}, Action: grammar.Term{Phrase: "be"},
		Patient: grammar.Term{Phrase: grammar.Indefinite(catName)}}, d.chem.Snapshot())
	if ev, ok := d.renderFacts(lang, catName, d.gatherFacts(catFrags[0], "agent", rel), true, false); ok {
		return isA + " " + ev, true
	}
	return isA, true
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
	m, ok := d.gram.Comprehend(frame.Lang, clause, d.resolveIn(frame.Lang), nil)
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
	belief, conf, supposed, lender := d.assessProposition(agent.Concept, rel.Content, patient.Concept)
	return d.answerPolar(frame.Lang, agent.Concept, rel, patient.Concept, past, belief, supposed, lender, conf), true
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
	m, ok := d.gram.Comprehend(frame.Lang, clause, d.resolveIn(frame.Lang), nil)
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

// denyRel is the edge a told negative is recorded under: a relation-specific denial, distinct from
// the positive role edges, so a denial stands in the graph without entering the walks that read
// what an agent does.
func denyRel(relContent string) string { return "deny:" + relContent }

// propositionDenied reports whether the agent was told it does not stand in the relation to the
// patient: a standing denial edge from the agent to the patient under the relation. It is the
// negative the defeasible reasoning honours over an inherited or resembled default.
func (d *Daemon) propositionDenied(agent *memory.Fragment, relContent string, patientID uint64) bool {
	if patientID == 0 {
		return false
	}
	edges, err := d.store.LinksFromRel([]uint64{agent.ID}, denyRel(relContent))
	if err != nil {
		return false
	}
	for _, e := range edges {
		if e.To == patientID {
			return true
		}
	}
	return false
}

// propositionDeniedCategorically reads whether a category the agent belongs to was told it does not
// stand in the relation to the patient: told "mammals do not eat the plant", a cow (a mammal)
// inherits the denial. It is the negative counterpart of categoryInherits, the defeasible exception
// carried at the category level, so a member denies what its category was told it does not do.
func (d *Daemon) propositionDeniedCategorically(agent *memory.Fragment, relContent string, patientID uint64) bool {
	for _, catID := range d.categoriesOf(agent.ID) {
		cf, err := d.fragmentsByIDsTiered([]uint64{catID})
		if err != nil || len(cf) != 1 {
			continue
		}
		if d.propositionDenied(cf[0], relContent, patientID) {
			return true
		}
	}
	return false
}

// isCategory reports whether a concept is an abstraction over members, a category
// in the is-a hierarchy rather than a thing in it.
func (d *Daemon) isCategory(c *memory.Fragment) bool {
	if c == nil {
		return false
	}
	_, ok := abstractionLemma(c.Template)
	return ok
}

// categoryMembers returns the concepts an abstraction grounds in, the things it is
// a category of: animal grounds in the cat, the dog, the mouse, the rabbit.
func (d *Daemon) categoryMembers(catID uint64) []uint64 {
	edges, err := d.store.LinksFromRel([]uint64{catID}, memory.RelGrounds)
	if err != nil {
		return nil
	}
	out := make([]uint64, 0, len(edges))
	for _, e := range edges {
		out = append(out, e.To)
	}
	return out
}

// categoriesOf returns the categories a concept belongs to, directly and up the
// is-a chain (the cat is an animal, an animal is a thing), so a member's whole
// lineage is in hand for a query or an inheritance. The climb is the composition:
// a fact of membership chained onto a fact of membership.
func (d *Daemon) categoriesOf(memberID uint64) []uint64 {
	var out []uint64
	seen := map[uint64]bool{memberID: true}
	stack := []uint64{memberID}
	for len(stack) > 0 {
		m := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		cats, err := d.store.LinksToRel([]uint64{m}, memory.RelGrounds)
		if err != nil {
			continue
		}
		for _, e := range cats { // the abstractions that ground in m are its categories
			if !seen[e.From] {
				seen[e.From] = true
				out = append(out, e.From)
				stack = append(stack, e.From) // climb to the category's own categories
			}
		}
	}
	return out
}

// isA reports whether a concept is a member of a category, directly or up the is-a
// chain, so a member belongs to every category above it.
func (d *Daemon) isA(memberID, catID uint64) bool {
	for _, c := range d.categoriesOf(memberID) {
		if c == catID {
			return true
		}
	}
	return false
}

// categoryInherits reads whether a category the agent belongs to stands in the relation to
// the patient: told that the animal chased the mouse, a cat (an animal) probably did too. It
// composes membership with a category-level fact, defeasibly. The composition now lives in
// the graph as the inheritance rule: it runs through evalInheritRule when the rule is present
// (the reasoning is graph-authoritative), and the Go walk below serves as the fallback. The
// rule reproduces the Go walk, verified over the inheritance fixtures.
func (d *Daemon) categoryInherits(agent *memory.Fragment, relContent string, patientID uint64) (uint64, bool) {
	if root := d.compRule(inheritRuleTemplate, inheritSpec()); root != 0 {
		if r, ok := d.evalRule(root, ruleQuery{from: agent.ID, patient: patientID, rel: relContent}); ok {
			return r.id, r.hit
		}
	}
	return d.categoryInheritsGo(agent, relContent, patientID)
}

// categoryInheritsGo is the hardcoded composition, the fallback for the inheritance rule
// above: it climbs the agent's categories and returns the first that stands in the relation
// to the patient, for the explanation, and whether one was found.
func (d *Daemon) categoryInheritsGo(agent *memory.Fragment, relContent string, patientID uint64) (uint64, bool) {
	for _, catID := range d.categoriesOf(agent.ID) {
		cf, err := d.fragmentsByIDsTiered([]uint64{catID})
		if err != nil || len(cf) != 1 {
			continue
		}
		if d.propositionHolds(cf[0], relContent, patientID) {
			return catID, true
		}
	}
	return 0, false
}

// inheritedFacts finds what a concept's categories do in the asked role, the
// defeasible counterpart of inferredFacts: a member inherits its category's
// propositions, hedged. "What did the rabbit chase" reaches the mouse through the
// animal the rabbit belongs to. It returns false when no category fills the role.
func (d *Daemon) inheritedFacts(known *memory.Fragment, knownRole, relContent string) ([]focusFact, bool) {
	var out []focusFact
	seen := map[uint64]bool{}
	for _, catID := range d.categoriesOf(known.ID) {
		cf, err := d.fragmentsByIDsTiered([]uint64{catID})
		if err != nil || len(cf) != 1 {
			continue
		}
		for _, f := range d.gatherFacts(cf[0], knownRole, relContent) {
			if f.coID == known.ID || seen[f.coID] {
				continue
			}
			seen[f.coID] = true
			out = append(out, f)
		}
	}
	return out, len(out) > 0
}

// propositionHoldsCategorically reads whether a proposition holds once a category
// in one of its roles is read as any member of it: the dog chased an animal because
// it chased a cat, an animal chased the cat because the dog did. It composes a
// proposition with a fact of membership, both grounded, so the conclusion is sound.
func (d *Daemon) propositionHoldsCategorically(agent *memory.Fragment, relContent string, patient *memory.Fragment) bool {
	if d.isCategory(patient) {
		for _, m := range d.categoryMembers(patient.ID) {
			if m != agent.ID && d.propositionHolds(agent, relContent, m) {
				return true
			}
		}
	}
	if d.isCategory(agent) {
		for _, m := range d.categoryMembers(agent.ID) {
			if m == patient.ID {
				continue
			}
			if mf, err := d.fragmentsByIDsTiered([]uint64{m}); err == nil && len(mf) == 1 &&
				d.propositionHolds(mf[0], relContent, patient.ID) {
				return true
			}
		}
	}
	// Both roles are categories: chain two is-a hops. An animal ate a food because the
	// rabbit (an animal) ate the carrot (a food). It holds when a grounded fact has an
	// agent that is a member of the agent-category and a co-argument that is a member of
	// the patient-category. Both hops are grounded, so the conclusion is as sound as the
	// single-role case. Each agent-member's facts are gathered once and the co-argument
	// tested for patient-category membership, so no member cross-product is materialised.
	if d.isCategory(agent) && d.isCategory(patient) {
		for _, m := range d.categoryMembers(agent.ID) {
			mf, err := d.fragmentsByIDsTiered([]uint64{m})
			if err != nil || len(mf) != 1 {
				continue
			}
			for _, f := range d.gatherFacts(mf[0], "agent", relContent) {
				if f.coID != m && d.isA(f.coID, patient.ID) {
					return true
				}
			}
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
	beliefDenied                     // told not to hold, so it does not, overriding any default
)

// inferenceFloor is how near a neighbour must be for its proposition to support
// an inference about the asked agent; below it, resemblance is too thin to guess on.
const inferenceFloor = 0.3

// roleFitIndistinctFloor is how alike two arguments must be before role-fit is set
// aside in favour of word order: above it their feature signatures are too close for
// kinship to tell which fills which role (the gray wolf and the gray rabbit), so the
// order that named them is the surer signal. Below it, role-fit still corrects a
// genuinely misplaced subject, which resembles its own kind, not the co-argument.
const roleFitIndistinctFloor = 0.6

// categoryInheritConf is the confidence a member inherits from a category-level
// fact: firmer than a typical resemblance guess, since membership is a surer tie
// than feature overlap, but still hedged, since a category's doing binds no one
// member.
const categoryInheritConf = 0.7

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
// lender is the concept whose membership or resemblance carried an inferred
// answer (zero elsewhere), surfaced so the claim's trace can point at the very
// basis used rather than one re-derived later.
func (d *Daemon) assessProposition(agent *memory.Fragment, relContent string, patient *memory.Fragment) (belief propBelief, conf float64, supposed bool, lender uint64) {
	// A told negative is authoritative: "the cat does not eat plants" denies the proposition even
	// where the cat would inherit it as an animal. So a standing denial is honoured before any
	// positive is weighed, the defeasible exception overriding the default.
	if d.propositionDenied(agent, relContent, patient.ID) {
		return beliefDenied, 1, false, 0
	}
	if d.propositionHolds(agent, relContent, patient.ID) {
		// A fact heard only from a source below full trust is a proposal on record,
		// not something GOLEM saw or the architect told: it answers hedged, at the
		// teller's trust, until a fully trusted telling or entrenchment firms it.
		if trust := d.propositionTrust(agent.ID, relContent, patient.ID); trust < architectTrust {
			return beliefInferred, trust, false, 0
		}
		return beliefObserved, 1, false, 0
	}
	// A category in a role is met by any member that fills it: "did the dog chase
	// an animal" holds because the dog chased a cat and a cat is an animal. The
	// deduction is sound, so it answers as firmly as the fact it rests on.
	if d.propositionHoldsCategorically(agent, relContent, patient) {
		return beliefObserved, 1, false, 0
	}
	// A transitive relation chains its told orderings: told "before is transitive",
	// A before B and B before C entail A before C. The relation must have been told
	// it is transitive, so this never fires for an ordinary relation (chase does not
	// chain); the walk is the same reachability the comparative reads, run here for a
	// relation whose transitivity is a learned property rather than a given of the
	// construction. The deduction is sound, so it answers as firmly as the orderings.
	if agent.ID != patient.ID {
		if rel := d.resolveRelation(relContent); rel != nil && d.relationIsTransitive(rel.ID) &&
			d.comparesTransitively(rel, agent.ID, patient.ID) {
			return beliefObserved, 1, false, 0
		}
	}
	// A symmetric relation holds both ways: told "meet is symmetric", the cat meeting the dog
	// answers that the dog met the cat. The relation must have been told it is symmetric, so
	// this never fires for an ordinary relation; the reversed fact is read by running the
	// stored compositional symmetric rule against the graph, the deduction as firm as the fact.
	if agent.ID != patient.ID && d.symmetricHolds(agent.ID, relContent, patient.ID) {
		return beliefObserved, 1, false, 0
	}
	// Containment composes across placements of different kinds: a thing on or in something
	// that is in a place is in that place ("the leaf is on the tree, the tree is in the
	// forest, so the leaf is in the forest"). The chain of locative grounds is walked for the
	// containment query; the deduction is sound, so it answers as firmly as the placements.
	if agent.ID != patient.ID && relContent == containmentPrep {
		if d.locativelyInside(agent.ID, patient.ID, locativeChainDepth) {
			return beliefObserved, 1, false, 0
		}
	}
	// A thing standing in a relation to itself is not a claim worth guessing, so
	// a self-proposition is left unknown rather than inferred from a neighbour.
	if agent.ID == patient.ID {
		return beliefUnknown, 0, false, 0
	}
	// A category the agent belongs to was told it does not stand in the relation: told "mammals do
	// not eat the plant", a cow (a mammal) denies it. The category-level negative overrides the
	// inherited default below, the defeasible exception inherited the way the default would be, but
	// it is weighed after the firm positives above, so an observed fact about the member still wins.
	if d.propositionDeniedCategorically(agent, relContent, patient.ID) {
		return beliefDenied, 1, false, 0
	}
	// A category the agent belongs to standing in the relation lends it, hedged:
	// the cat is an animal, and the animal chased the mouse, so probably the cat
	// did. Membership is a firmer tie than resemblance, so it is tried first, but a
	// category's doing binds no one member, so the conclusion is held as a guess.
	if catID, ok := d.categoryInherits(agent, relContent, patient.ID); ok {
		d.held.add(heldProp{rel: relContent, agent: agent.ID, patient: patient.ID,
			agentGloss: agent.Content, patientGloss: patient.Content,
			status: heldInferred, conf: categoryInheritConf, depth: 0, voiced: true})
		return beliefInferred, categoryInheritConf, false, catID
	}
	// Resemblance proposes the neighbour, the graph disposes: the weighted-best support a concept
	// the agent resembles gives this proposition, run as the resemblance rule (the composable part)
	// against the graph, the Go weighted-max the fallback. A vector read that errors is the honest
	// gap, weighed before the held self-conclusion as the source order has it.
	r, ok := d.resembleSupport(agent, relContent, patient)
	if !ok {
		return beliefUnknown, 0, false, 0
	}
	best, bestDepth, bestFrom, bestSupposed, bestSib := 0.0, 0, uint64(0), false, uint64(0)
	// A standing conclusion about this very proposition is evidence for it, whether GOLEM
	// believes it or is only supposing it, weighed against the resemblance support.
	if p := d.held.findID(relContent, agent.ID, patient.ID, heldInferred); p != nil {
		best, bestDepth = p.conf, p.depth
	}
	if p := d.held.findID(relContent, agent.ID, patient.ID, heldHypothetical); p != nil && p.conf > best {
		best, bestDepth, bestSupposed = p.conf, p.depth, true
	}
	// The resemblance support wins only if it strictly beats the held self-conclusion, so a held
	// belief at position zero keeps a tie: the exact order the inline walk seeded and updated in.
	if r.conf > best {
		best, bestDepth, bestFrom, bestSupposed, bestSib = r.conf, r.depth, r.from, r.supposed, r.lender
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
		return beliefInferred, best, bestSupposed, bestSib
	}
	return beliefUnknown, 0, false, 0
}

// inferRole answers a role question GOLEM holds no observed fact for from a fact
// the asked concept inherits, the wh-question counterpart of the category-level
// polar guess. A category the asked one belongs to that stood in the relation in
// the questioned role lends its co-argument: "what did the rabbit chase" reaches
// the mouse through the animal the rabbit is, defeasibly. Feature resemblance to a
// neighbour is deliberately not a source here: borrowing an unrelated neighbour's
// co-argument would fabricate the very object the question asks for and speak it
// under the asked concept's name, asserting a proposition the graph does not hold.
// A wh-object answer must rest on a grounded tie (the concept's own fact, gathered
// before this is reached, or a category it inherits), not on mere likeness, so an
// ungrounded subject yields the honest gap rather than an invented object. The same
// gather and renderer the observed answer uses, here run over an inherited fact.
// relContent filters to one relation, "" leaves it open for "what did X do".
// continuation carries the focus as the tongue's pronoun, for a guess that trails a
// clause already naming it. It returns the rendered line and true when a category
// supplied such a fact, so the caller offers the gap only when none did.
func (d *Daemon) inferRole(frame grammar.Frame, known *memory.Fragment, knownRole, relContent string, past, continuation bool) (string, bool) {
	kept, ok := d.inheritedFacts(known, knownRole, relContent)
	if !ok {
		return "", false
	}
	line, ok := d.renderFacts(frame.Lang, d.conceptName(known.ID, known.Content, frame.Lang), kept, past, continuation)
	if !ok {
		return "", false
	}
	hedge := d.hedgeWord(frame.Lang, topConf(kept))
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
		// What the neighbour is observed to do, transferred as a fresh guess. The
		// states the comparison and place questions own stay out, as they do in
		// every fact appendix: a neighbour's ordering does not transfer by
		// likeness, and a volunteered one would render its relation as a verb
		// ("the fox biggered the mouse").
		for _, f := range d.gatherFacts(&memory.Fragment{ID: sib.id}, "agent", "") {
			if f.coID == topic.ID || observed[coArg{f.rel, f.coID}] ||
				d.factIsComparative(f.rel) || d.factIsLocative(f.rel) {
				continue
			}
			d.held.add(heldProp{rel: f.rel, agent: topic.ID, patient: f.coID,
				agentGloss: topic.Content, patientGloss: f.coGloss,
				status: heldInferred, conf: sib.weight, depth: 0})
			drew = true
		}
		// What the neighbour is only believed to do, transferred a hop further out.
		for _, p := range d.held.byAgent(sib.id, heldInferred) {
			if p.patient == topic.ID || p.depth >= heldMaxDepth || observed[coArg{p.rel, p.patient}] ||
				d.factIsComparative(p.rel) || d.factIsLocative(p.rel) {
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
		// Nothing concluded to offer; perhaps GOLEM has NOTICED an un-named kind:
		// concepts that sit together in the learned space but were never given a
		// common word. It proposes one, hedged, for the speaker to name.
		return d.proposeKind(lang)
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
	hedge := d.hedgeWord(lang, p.conf)
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
func (d *Daemon) answerPolar(lang string, agent, rel, patient *memory.Fragment, past bool, belief propBelief, supposed bool, lender uint64, conf float64) string {
	// What GOLEM is about to commit to, weighed against what it has already said
	// this exchange. A supposition is contingent, so it is neither recorded nor
	// held against: it never becomes a claim GOLEM stands by.
	acked := false
	if !supposed {
		affirm := belief == beliefObserved || belief == beliefInferred
		hedged := belief == beliefInferred
		if prior := d.recallClaim(rel.Content, agent.ID, patient.ID); prior != nil {
			if prior.affirmed && !affirm && belief != beliefDenied {
				// GOLEM affirmed this proposition earlier and nothing told this exchange
				// overturns it; it stands by its claim rather than quietly contradict
				// itself. But it stands by it as a HEDGE: an affirmation now held only on
				// memory of having said it, no longer grounded live, is at most "probably",
				// never restored to a firm "yes". A told negative does overturn it, so a
				// denial is exempt; a live-grounded upgrade keeps belief at beliefObserved
				// and never reaches here, so the "probably" -> "yes" path is unaffected.
				affirm, hedged = true, true
				belief = beliefInferred
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
		word = d.hedgeWord(lang, conf) // the hedge marks the clause a guess, graded by how sure
	default:
		word = d.gram.Word(lang, "no")
		negate = true
	}
	var clause string
	var ok bool
	if d.relationIsLocative(rel.ID) {
		// A locative proposition answers as a copula with its preposition ("the leaf
		// is on the tree"), the same clause the figure's "where" reads, denied for a
		// "no".
		clause, ok = d.locativeClause(lang,
			d.conceptName(agent.ID, agent.Content, lang),
			rel.Content,
			d.conceptName(patient.ID, patient.Content, lang),
			false, negate, d.chem.Snapshot())
	} else {
		meaning := grammar.Meaning{
			Relation: grammar.Term{Phrase: d.relationName(rel.Content, lang)},
			Roles: map[string]grammar.Term{
				"agent":   {Phrase: d.conceptName(agent.ID, agent.Content, lang), Definite: true},
				"patient": {Phrase: d.objectCase(d.conceptName(patient.ID, patient.Content, lang), lang), Definite: true},
			},
			Past:   past,
			Negate: negate,
		}
		clause, ok = d.gram.Speak(lang, meaning)
		if !ok {
			// A tongue the coded realizer does not cover (Latin) speaks through the
			// declarative engine instead, which cases the nouns from the graph.
			clause, ok = d.gram.SpeakDecl(lang, meaning)
		}
	}
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
	// An affirmed claim leaves its proof: the moment that holds it, or the
	// lender the guess rode in on, as they stood at this speaking, so the
	// probe reads the actual support rather than a basis re-derived later. A
	// supposition is contingent and leaves no durable record.
	if !supposed && (belief == beliefObserved || belief == beliefInferred) {
		d.traceClaim(rel.Content, agent.ID, patient.ID, answer,
			d.polarEvidence(rel.Content, agent.ID, patient.ID, lender))
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
		// A deictic pronoun ("what did you do") names a participant the activity is
		// asked after, so it is kept where the closed-class filter would drop it.
		if !d.gram.IsContentWord(frame.Lang, t) && !d.gram.IsDeictic(frame.Lang, t) {
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
	forms := tokens
	if len(tokens) > 1 {
		// A multi-word relation (the Turkish comparative "daha büyük") is named as one
		// form, so the whole phrase is tried alongside its tokens.
		forms = append([]string{strings.Join(tokens, " ")}, tokens...)
	}
	words, err := d.store.WordNodesByForms(forms)
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
// resolveRelationSurface resolves the relation an inflected verb surface
// names, trying the lemma then the tongue's further stem candidates, so a
// Turkish aorist question verb ("yer") reaches the relation its stem names
// the way the past form does.
func (d *Daemon) resolveRelationSurface(lang, t string) *memory.Fragment {
	if c := d.resolveRelation(d.gram.Lemma(lang, t)); c != nil {
		return c
	}
	for _, s := range d.gram.StemCandidates(lang, t) {
		if c := d.resolveRelation(s); c != nil {
			return c
		}
	}
	return nil
}

func (d *Daemon) analyzeRoleQuestion(lang, raw string) (rel, known, rival *memory.Fragment, knownRole, verb string) {
	tokens, _ := grammar.Tokenize(raw)
	var content []string
	proIdx := -1
	for _, t := range tokens {
		// A deictic pronoun is closed-class but names a participant, so it can be
		// the known argument of "what did you chase"; keep it beside the open-class
		// words the question splits into. A closed-class surface whose lemma names
		// a learned relation is kept too: "run" is a seeded imperative, but once
		// taught as a verb the graph's relation rescues it, so "what do you run"
		// still finds its relation.
		if d.gram.IsContentWord(lang, t) || d.gram.IsDeictic(lang, t) ||
			d.resolveRelationSurface(lang, t) != nil {
			content = append(content, t)
			continue
		}
		// A pro-form ("it", "they") stands where a known argument would: it is
		// kept beside the content words so position still assigns the roles, and
		// resolved from working memory once the role it fills is read.
		if proIdx < 0 && d.gram.WordClass(lang, t) == "pro" {
			proIdx = len(content)
			content = append(content, t)
		}
	}
	verbIdx := -1
	for i, t := range content {
		// A pronoun is never the predicate: even where a garbled clause once
		// minted a relation under a deictic surface, the question's verb is
		// sought among the content words alone.
		if d.gram.IsDeictic(lang, t) || i == proIdx {
			continue
		}
		if c := d.resolveRelationSurface(lang, t); c != nil {
			verbIdx, rel, verb = i, c, t
			break
		}
	}
	if verbIdx < 0 {
		return nil, nil, nil, "", ""
	}
	for i, t := range content {
		if i == verbIdx || i == proIdx {
			continue
		}
		if c := d.resolveConcept(t); c != nil && c.Category != "relation" && !d.scaffoldingConcept(c.ID) {
			known = c
			break
		}
	}
	knownRole = "agent" // an argument precedes the verb; the wh-word is the object
	if verbIdx == 0 {
		knownRole = "patient" // the wh-word is the subject; the known argument is the patient
	}
	// A question whose only argument is the pro-form reads its referent from
	// working memory: the warm concept that lately played the role the pronoun
	// stands in, so "what did it chase" asks after the fox that just chased.
	// A rival that fits within the conflict margin rides along, so the caller
	// can ask which was meant.
	if known == nil && proIdx >= 0 {
		known, rival = d.roleReferent(knownRole, rel.ID)
	}
	if known == nil {
		return nil, nil, nil, "", ""
	}
	return rel, known, rival, knownRole, verb
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
	// A deictic pronoun names a participant, never a relation. A clause the parser
	// garbled can seat "you" as the predicate, and minting rel:en:you would then have
	// every later deictic question read the pronoun as its verb and break the whole
	// deictic register, the same wholesale failure the comparative marker would cause.
	if d.gram.IsDeictic(lang, lemma) {
		return nil, nil
	}
	// The grammatical core never names a relation either: a garbled copular
	// clause can seat "are" as the predicate, and minting rel:en:are breaks
	// the copula for the whole brain. The refusal is by grammatical class, so
	// an imperative-capable surface like "run" stays teachable as a verb and
	// the locative prepositions keep naming their placement relations.
	switch d.gram.WordClass(lang, lemma) {
	case "cop", "wh", "det", "pro", "cnt", "neg":
		return nil, nil
	case "fn":
		if !d.gram.LocativePrep(lang, lemma) {
			return nil, nil
		}
	}
	// The comparative marker names no relation. A "than" ("daha") that reaches this path is a
	// mis-parsed comparison, not a verb; minting a relation for it (rel:en:than) would then
	// have the parser read every later comparison as that relation and break comparison
	// wholesale. Refuse just the marker (locative prepositions like "in"/"on" do name their
	// relations and must pass), so the malformed proposition binds no relation rather than
	// entrenching the marker as one.
	if marker := d.gram.Word(lang, "than"); marker != "" && strings.EqualFold(lemma, marker) {
		return nil, nil
	}
	// A gradable comparative surface names a pole on a scale, never a relation. A
	// comparison frame is written by storeComparison and never reaches here, but a
	// malformed clause could seat "bigger" as a predicate on the general-proposition
	// path; minting rel:en:bigger would fork the very atomic relation the scale model
	// dissolved. Refuse it structurally, so a comparative surface binds nothing here.
	if d.isGradablePoleSurface(lemma, lang) {
		return nil, nil
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
// above the attention gate AND freshly attended, the relation a verb new to a
// tongue binds to so it names what was just spoken of. Fresh attention, not raw
// lingering warmth, is the signal, so a stale relation does not capture a new
// verb; members it might subsume are ignored, the warmest relation wins.
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
	now := time.Now()
	for _, f := range frags {
		if f.Category != "relation" {
			continue
		}
		// Warm but not freshly attended: a relation lingering from an earlier scene, or
		// mulled over, does not take a new tongue's verb as its name; only a relation
		// genuinely just spoken of does, the same fresh-attention gate a told noun uses.
		// Otherwise a stale warm relation would fuse a brand-new verb onto the wrong one.
		if t, ok := d.ws.seenAt(f.ID); !ok || now.Sub(t) > attentionRecency {
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
	return d.conceptNamedIn(relID, lang)
}

// conceptNamedIn reports whether a word of the given tongue already names the concept,
// relation or noun alike. It guards cross-tongue binding the same way for both: a
// tongue that already speaks the concept does not take a second word for it from
// warmth, so two distinct words of one tongue stay two concepts, while a tongue with
// no name for it yet may give it its first.
func (d *Daemon) conceptNamedIn(id uint64, lang string) bool {
	namers, err := d.store.LinksToRel([]uint64{id}, memory.RelNames)
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
	w, _, err := d.mintWordNode(lemma, lang, d.chem.Snapshot(), time.Now())
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

// argsIndistinctForFit reports whether a transitive meaning's two arguments resemble
// each other so closely that role-fit cannot tell which fills which role. When they do,
// the word order that named them is authoritative; role-fit, which reads feature kinship
// to past role-fillers, would otherwise swap two look-alikes (the gray wolf and the gray
// rabbit) on a hair's difference. Distinct fillers (a faint or zero mutual resemblance)
// leave role-fit free to correct a misplaced subject, which resembles its own kind.
func (d *Daemon) argsIndistinctForFit(m grammar.Meaning) bool {
	a, p := m.Roles["agent"], m.Roles["patient"]
	if a.Concept == nil || p.Concept == nil || a.Concept.ID == p.Concept.ID {
		return false
	}
	sibs, err := d.featureSiblings(a.Concept.ID, time.Now())
	if err != nil {
		return false
	}
	for _, s := range sibs {
		if s.id == p.Concept.ID {
			return s.weight >= roleFitIndistinctFloor
		}
	}
	return false
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

// roleHeld reports whether a concept has stood in a thematic role on record,
// the presence the grammar's order-defense consults: a filler that has
// already held the role it was seated in keeps the seat the teller gave it.
// A boolean, never a count, so the swap feedback the role-count fit once fed
// cannot return.
func (d *Daemon) roleHeld(c *memory.Fragment, role string) bool {
	if c == nil {
		return false
	}
	rel := memory.RelAgent
	if role == "patient" {
		rel = memory.RelPatient
	}
	edges, err := d.store.LinksToRel([]uint64{c.ID}, rel)
	return err == nil && len(edges) > 0
}
