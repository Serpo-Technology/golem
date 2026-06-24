package daemon

import (
	"fmt"
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

// openQuestion is a question raised this exchange that GOLEM could not answer: the
// relation it was asked about, the concept it was asked about, and the role that
// concept plays (the other role is the gap). Held so a later telling that fills the
// gap can be connected back to the question that wanted it.
type openQuestion struct {
	rel       string
	known     uint64
	knownRole string
}

const openQuestionCap = 8

// recordOpenQuestion remembers a question GOLEM admitted it could not answer, one
// entry per distinct question, so the answer can be volunteered when it arrives.
func (d *Daemon) recordOpenQuestion(rel string, known uint64, knownRole string) {
	if known == 0 || knownRole == "" {
		return
	}
	for _, q := range d.openQuestions {
		if q.rel == rel && q.known == known && q.knownRole == knownRole {
			return
		}
	}
	d.openQuestions = append([]openQuestion{{rel: rel, known: known, knownRole: knownRole}}, d.openQuestions...)
	if len(d.openQuestions) > openQuestionCap {
		d.openQuestions = d.openQuestions[:openQuestionCap]
	}
}

// resolveOpenQuestions checks a freshly told proposition against the questions left
// open, and when it answers one, removes it and renders the answer GOLEM can now
// give, so a telling that fills a gap is followed up rather than passing silently.
// It returns the empty string when nothing open is answered.
func (d *Daemon) resolveOpenQuestions(lang, rel string, agent, patient uint64) string {
	for i, q := range d.openQuestions {
		if q.rel != "" && q.rel != rel {
			continue
		}
		answered := (q.knownRole == "agent" && q.known == agent && patient != 0) ||
			(q.knownRole == "patient" && q.known == patient)
		if !answered {
			continue
		}
		d.openQuestions = append(d.openQuestions[:i], d.openQuestions[i+1:]...)
		return d.followUpAnswer(lang, q)
	}
	return ""
}

// followUpAnswer speaks the answer to a question that was open, now that GOLEM can,
// led by the tongue's "now I know" so the telling is heard as the answer to what
// was asked rather than an echo of what was just said.
func (d *Daemon) followUpAnswer(lang string, q openQuestion) string {
	frags, err := d.store.FragmentsByIDs([]uint64{q.known})
	if err != nil || len(frags) != 1 {
		return ""
	}
	known := frags[0]
	facts := d.gatherFacts(known, q.knownRole, q.rel)
	line, ok := d.renderFacts(lang, d.conceptName(known.ID, known.Content, lang), facts, true, false)
	if !ok {
		return ""
	}
	if now := d.gram.Word(lang, "nowknow"); now != "" {
		return upperFirst(now) + " " + lowerFirst(line)
	}
	return line
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
	// A participant is spoken from GOLEM's side, the mirror of how it was heard:
	// GOLEM names itself "I", the one it speaks with "you", the two "we", so the
	// pronoun the user said as "you" comes back as "I".
	if name := d.participantName(id, lang); name != "" {
		return name
	}
	edges, err := d.store.LinksToRel([]uint64{id}, memory.RelNames)
	if err != nil || len(edges) == 0 {
		return d.borrowedName(id, content, lang)
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
		return d.borrowedName(id, content, lang)
	}
	return best
}

// nameBorrowThreshold is how near a neighbor must sit before its word is borrowed for a
// concept the graph cannot name: the vector proposes a word only when it is confidently
// close, so a loosely related neighbor does not put words in a concept's mouth.
const nameBorrowThreshold = 0.5

// borrowedName proposes a word for a concept the graph cannot name in a language: it takes
// the concept's embedding and borrows the nearest named neighbor's word, the vector layer
// proposing where the naming edges are silent. It returns the concept's content when there
// is no embedding or no near-enough named neighbor, so naming never regresses below the
// gloss it had before.
func (d *Daemon) borrowedName(id uint64, content, lang string) string {
	vec, ok := d.store.Vector(id)
	if !ok {
		return content
	}
	matches, err := d.store.NearestVectors(vec, memory.KindConcept, 8, id)
	if err != nil {
		return content
	}
	for _, m := range matches {
		if m.Score < nameBorrowThreshold {
			break // sorted strongest first, so nothing further clears the bar
		}
		if w := d.namingWord(m.ID, lang); w != "" {
			return w
		}
	}
	return content
}

// namingWord is the firmest word naming a concept in a language, "" when none. It is the
// direct naming lookup a borrow reads on a neighbor, without the participant, own-name, or
// borrow handling conceptName layers on, so a borrow cannot recurse into another borrow.
func (d *Daemon) namingWord(id uint64, lang string) string {
	edges, err := d.store.LinksToRel([]uint64{id}, memory.RelNames)
	if err != nil || len(edges) == 0 {
		return ""
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
		return ""
	}
	best, bestW := "", 0.0
	for _, w := range words {
		if w.Kind != memory.KindWord || w.Lang != lang {
			continue
		}
		if best == "" || weight[w.ID] > bestW {
			best, bestW = w.Content, weight[w.ID]
		}
	}
	return best
}

// participantName speaks a conversation participant from GOLEM's side: itself as
// the first person, the one it addresses as the second, the two together as the
// first-person plural. It returns "" for any other concept, and never matches when
// the participants are unset (id 0), so naming falls through to the graph.
func (d *Daemon) participantName(id uint64, lang string) string {
	switch {
	case id == 0:
		return ""
	case id == d.selfID:
		return d.gram.Word(lang, "firstperson")
	case id == d.userID:
		return d.gram.Word(lang, "addressee")
	case id == d.bothID:
		return d.gram.Word(lang, "firstpersonpl")
	}
	return ""
}

// objectCase gives a participant filler its object-case surface for the patient
// role where the tongue distinguishes it: English "I" becomes "me" and "we"
// becomes "us", so a participant in object position reads "the dog chased me"
// with its correct case where it would otherwise read "chased I".
// The mapping is read from the tongue's participant surfaces, the subject form
// to its object form; a filler that is no participant, or a tongue that cases
// the object by morphology instead (Turkish marks it accusative downstream), is
// returned unchanged.
func (d *Daemon) objectCase(name, lang string) string {
	for _, p := range []string{"firstperson", "addressee", "firstpersonpl"} {
		if d.gram.Word(lang, p) != name {
			continue
		}
		if obj := d.gram.Word(lang, p+".obj"); obj != "" {
			return obj
		}
		return name
	}
	return name
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
		// A locative relation speaks as a copula with a preposition ("the leaf is on
		// the tree"), not as a verb. The figure is the relation's agent and the
		// ground its patient, so the focus is the figure when it stood as agent and
		// the ground when it stood as patient; the co-arguments fill the other side.
		if d.factIsLocative(k.rel) {
			prep := k.rel // the canonical preposition the tongue renders by
			given := continuation || len(clauses) > 0
			for _, co := range names {
				figure, ground := focusName, co
				figGiven := given
				if k.role == "patient" {
					figure, ground, figGiven = co, focusName, false
				}
				if cl, ok := d.locativeClause(lang, figure, prep, ground, figGiven, false, d.chem.Snapshot()); ok {
					clauses = append(clauses, cl)
				}
			}
			continue
		}
		if d.factIsComparative(k.rel) {
			continue // a comparison answers through its own query, not as a verb here
		}
		// The focus plays k.role and the co-arguments fill the other. A participant
		// in the patient role takes its object case ("the dog chased me"), so the
		// patient-side filler is cased before the clause is built.
		focusIsPatient := k.role == "patient"
		coNames := names
		if !focusIsPatient {
			coNames = make([]string, len(names))
			for i, n := range names {
				coNames[i] = d.objectCase(n, lang)
			}
		}
		fName := focusName
		if focusIsPatient {
			fName = d.objectCase(focusName, lang)
		}
		// The meaning is language-free: the focus and the coordinated
		// co-arguments are named, marked definite or given, and the engine
		// renders the article, coordinator, and pronoun from the tongue's data.
		co := grammar.Term{Phrase: coNames[0], With: coNames[1:], Definite: true}
		focus := grammar.Term{Phrase: fName, Definite: true}
		if continuation || len(clauses) > 0 {
			focus = grammar.Term{Given: true} // already in play; the tongue's pronoun
		}
		roles := map[string]grammar.Term{}
		if k.role == "agent" {
			roles["agent"], roles["patient"] = focus, co
		} else {
			roles["agent"], roles["patient"] = co, focus
		}
		meaning := grammar.Meaning{Relation: grammar.Term{Phrase: d.relationName(k.rel, lang)}, Roles: roles, Past: past}
		// The declarative engine drives the clause when it can speak the meaning; the coded
		// Speak stays as the fallback for the shapes it does not yet cover, and as a tripwire
		// that logs any divergence on the shapes it does.
		line, ok := d.gram.Speak(lang, meaning)
		alt, dok := d.gram.SpeakDecl(lang, meaning)
		switch {
		case dok && ok && alt != line:
			fmt.Fprintf(d.out, "declgen mismatch: %q vs Speak %q\n", alt, line)
		case dok && d.shadowVerbose:
			fmt.Fprintf(d.out, "declgen ok\n")
		case !dok && ok && d.shadowVerbose:
			fmt.Fprintf(d.out, "declgen decline: %q\n", line)
		}
		if dok {
			line, ok = alt, true
		}
		if ok {
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
func (d *Daemon) planRecallSaid(frame grammar.Frame) (string, bool) {
	// A recall-said move asks what the exchange has set down: a verb of saying, in
	// a question. Asked of GOLEM ("what did you say") it would be after GOLEM's own
	// words, a different report, so the move is the speaker asking what was told.
	if frame.Speech != "recall" || frame.Force != grammar.ForceQuestion {
		return "", false
	}
	if frame.Agent.Concept != nil && frame.Agent.Concept.ID == d.selfID {
		return "", false
	}
	if len(d.threadProps) == 0 {
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
	// A continuation ("what about the dog") and a repair ("no, I meant the dog")
	// both inherit the last question's relation and ask it of a new referent; the
	// difference is only what prompted the swap. The repair names its referent
	// through the "mean" verb, read from the frame; the continuation is still read
	// from the surface.
	var known *memory.Fragment
	if frame.Speech == "clarify" && frame.Patient.Concept != nil && !d.isParticipant(frame.Patient.Concept.ID) {
		known = frame.Patient.Concept
	} else if ref, ok := aboutReferent(raw); ok {
		known = d.resolveConcept(ref)
	}
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
	// A pro-drop tongue carries the subject on the verb, so a two-word clause (a
	// verb and its object) is a whole question. English needs an auxiliary and a
	// subject besides the verb, so it is guarded again before its switch below.
	if len(tokens) < 2 {
		return "", false, false
	}
	if lang == "tr" {
		last := strings.ToLower(tokens[len(tokens)-1])
		switch last {
		case "mı", "mi", "mu", "mü":
			return strings.Join(tokens[:len(tokens)-1], " "), false, true
		}
		// In the aorist the particle fuses with the person ("ister misin" = want +
		// mi + you), so the suffix carries the subject when it is dropped. The
		// matching pronoun is restored before the clause is read, unless the clause
		// already names its subject.
		if pron, fused := trFusedInterrogative[last]; fused {
			rest := tokens[:len(tokens)-1]
			if len(rest) == 0 {
				return "", false, false
			}
			if !trSubjectPronoun[strings.ToLower(rest[0])] {
				rest = append([]string{pron}, rest...)
			}
			return strings.Join(rest, " "), false, true
		}
		return "", false, false
	}
	if lang == "la" {
		// Latin marks a yes/no question with the enclitic -ne on a word ("amatne
		// puella rosam"); strip it and the free-order clause reads as the proposition.
		for i, tok := range tokens {
			if low := strings.ToLower(tok); strings.HasSuffix(low, "ne") && len([]rune(low)) > 3 {
				tokens[i] = tok[:len(tok)-len("ne")]
				return strings.Join(tokens, " "), false, true
			}
		}
		return "", false, false
	}
	if len(tokens) < 3 {
		return "", false, false // English needs an auxiliary, a subject, and a verb
	}
	switch strings.ToLower(tokens[0]) {
	case "did":
		return strings.Join(tokens[1:], " "), true, true
	case "does":
		return strings.Join(tokens[1:], " "), false, true
	case "do":
		// "do" is also the imperative opener ("do the dishes"); it forms a yes/no
		// question only as do-support, before a subject pronoun ("do you want the
		// rose"). Stripped, the clause reads as the proposition to check.
		if subjectPronoun[strings.ToLower(tokens[1])] {
			return strings.Join(tokens[1:], " "), false, true
		}
	}
	return "", false, false
}

// subjectPronoun is the closed set of deictic pronouns that stand as a clause's
// subject, the company that tells do-support ("do you want X", a question) from
// the bare imperative "do". The third-person pronouns are left out: they ground in
// no participant here, and "do it" is an imperative, not a question.
var subjectPronoun = map[string]bool{"i": true, "you": true, "we": true}

// trFusedInterrogative maps a Turkish question particle fused with a person suffix
// to the subject pronoun that person drops. The aorist asks "ister misin" (want +
// mi + you) where the bare particle alone would be third person, so the fused
// suffix carries the dropped subject: misin -> you, miyim -> I, miyiz -> we, each
// across the four-way vowel harmony.
var trFusedInterrogative = map[string]string{
	"mısın": "sen", "misin": "sen", "musun": "sen", "müsün": "sen",
	"mısınız": "sen", "misiniz": "sen", "musunuz": "sen", "müsünüz": "sen",
	"mıyım": "ben", "miyim": "ben", "muyum": "ben", "müyüm": "ben",
	"mıyız": "biz", "miyiz": "biz", "muyuz": "biz", "müyüz": "biz",
}

// trSubjectPronoun is the deictic subject set, so a fused particle's person is not
// restored a second time when the clause already names its subject ("sen ... misin").
var trSubjectPronoun = map[string]bool{"ben": true, "sen": true, "biz": true}

// describeReferent reads the focus of an open request to say what is known about something.
// The "describe X" and "what do you know about X" phrasings route off the describe and know
// verbs now in every tongue (the subject the parsed patient, the Latin de-object included);
// only the "tell me about" recall phrasings still read the surface until they route off the
// recall verb too.
func describeReferent(raw string) (string, bool) {
	s := strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(strings.ToLower(raw)), "?"))
	for _, lead := range []string{"tell me about ", "tell me of ", "what can you tell me about "} {
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

// correctionClause reads the clause of a correction, "the dog chased the rabbit"
// from "no, the dog chased the rabbit" or "actually the dog chased the rabbit", the
// leading marker dropped, so a turn that fixes a claim is told from a plain
// statement. It returns false when the turn carries no correction marker.
func correctionClause(raw string) (string, bool) {
	s := strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(raw), "."))
	low := strings.ToLower(s)
	for _, lead := range []string{"no, ", "actually, ", "actually "} {
		if strings.HasPrefix(low, lead) {
			if rest := strings.TrimSpace(s[len(lead):]); rest != "" {
				return rest, true
			}
		}
	}
	return "", false
}

// removeThreadProp drops a proposition from the discourse record, used when a
// correction retracts what was established so the old fact no longer wins recall.
func (d *Daemon) removeThreadProp(rel string, agent, patient uint64) {
	kept := d.threadProps[:0]
	for _, p := range d.threadProps {
		if p.rel == rel && p.agent == agent && p.patient == patient {
			continue
		}
		kept = append(kept, p)
	}
	d.threadProps = kept
}

// removeClaim drops a claim from the speaker's-side record, used when a correction
// retracts what GOLEM had committed to.
func (d *Daemon) removeClaim(rel string, agent, patient uint64) {
	kept := d.claims[:0]
	for _, c := range d.claims {
		if c.rel == rel && c.agent == agent && c.patient == patient {
			continue
		}
		kept = append(kept, c)
	}
	d.claims = kept
}
