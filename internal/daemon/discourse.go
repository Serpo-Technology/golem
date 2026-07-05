package daemon

import (
	"fmt"
	"sort"
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

// planGreeting meets a greeting with the tongue's greeting and a farewell with
// its farewell, the social moves that open and close an exchange. The words
// route as grounded speech acts the way the trust verbs do, and the reply is
// the tongue's seeded greeting surface, so both sides of the move live in the
// graph rather than in the engine. It declines a clause that carries anything
// beyond the greeting itself, so "hello, what is a dog" still answers the
// question, and it leaves the topic untouched: a greeting opens the talk, it
// does not change what the talk is about.
func (d *Daemon) planGreeting(frame grammar.Frame) (string, bool) {
	if frame.Speech != "greet" && frame.Speech != "farewell" {
		return "", false
	}
	if frame.Wh != "" || !frame.Action.Empty() || frame.Inner != "" || frame.Property != "" {
		return "", false
	}
	word := d.gram.Word(frame.Lang, frame.Speech)
	if word == "" {
		return "", false
	}
	return upperFirst(word) + ".", true
}

// ackStatement is the minimal acknowledgment a plain telling earns: the tongue's
// "I see", so a statement audibly lands instead of passing in silence. The
// surface is a seeded word the nightly competition can bend, not a string fixed
// in the engine; a tongue without one keeps the old silence.
func (d *Daemon) ackStatement(lang string) string {
	word := d.gram.Word(lang, "isee")
	if word == "" {
		return ""
	}
	return upperFirst(word) + "."
}

// admitLost is the admission for a clause the grammar could not frame: the
// failure is spoken rather than silent, and the first unknown word is asked
// after, since a missing word is usually why the clause did not frame. The
// ask is not gated on curiosity here: total incomprehension is the one place
// a question back is always the right reply. A tongue without the seeded
// surface keeps the old silence.
func (d *Daemon) admitLost(frame grammar.Frame) string {
	word := d.gram.Word(frame.Lang, "lost")
	if word == "" {
		return ""
	}
	line := upperFirst(word) + "."
	for _, unknown := range frame.Unknown {
		if unknown == "" || !hasLetter(unknown) {
			continue
		}
		d.adoptGoal("learn", unknown)
		return line + " " + d.realizeAs(frame.Lang, grammar.Frame{Force: grammar.ForceQuestion,
			Action: grammar.Term{Phrase: "be"}, Patient: grammar.Term{Phrase: unknown}}, d.chem.Snapshot())
	}
	return line
}

// pendingReferent is a role question parked on an ambiguous pro-form: the
// relation and role it asked, and the referents that fit equally well. The
// next turn naming one of them answers the question it settles; any other
// turn drops it, a clarification unanswered being a topic changed.
type pendingReferent struct {
	rel        *memory.Fragment
	knownRole  string
	verb       string
	wh         string
	lang       string
	past       bool
	candidates []uint64
}

// askWhichReferent asks the speaker to settle a pro-form two referents fit
// equally well, the conflict veto applied to reference: picking on a hair's
// margin writes the wrong answer silently, and asking is what a listener
// does. The parked question is held for the reply. It declines in a tongue
// whose ask surfaces are not seeded, so the caller falls back to the pick.
func (d *Daemon) askWhichReferent(frame grammar.Frame, rel, a, b *memory.Fragment, knownRole, verb string) (string, bool) {
	lead := d.gram.Word(frame.Lang, "whichmean")
	or := d.gram.Word(frame.Lang, "or")
	if lead == "" || or == "" {
		return "", false
	}
	d.pendingReferent = &pendingReferent{rel: rel, knownRole: knownRole, verb: verb,
		wh: frame.Wh, lang: frame.Lang, past: frame.Past, candidates: []uint64{a.ID, b.ID}}
	return upperFirst(lead) + " the " + d.conceptName(a.ID, a.Content, frame.Lang) +
		" " + or + " the " + d.conceptName(b.ID, b.Content, frame.Lang) + "?", true
}

// planReferentReply reads the turn after an ambiguity question: a clause
// naming one of the parked candidates answers the role question the pro-form
// left open. Anything else lets the turn take its ordinary course.
func (d *Daemon) planReferentReply(frame grammar.Frame, raw string, p *pendingReferent) (string, bool) {
	chosen := frame.Agent.Concept
	if chosen == nil {
		chosen = frame.Patient.Concept
	}
	if chosen == nil {
		for _, t := range strings.Fields(strings.ToLower(strings.Trim(raw, " ?.!"))) {
			if c := d.resolveConceptLang(t, p.lang); c != nil && c.Kind == memory.KindConcept {
				chosen = c
				break
			}
		}
	}
	if chosen == nil {
		return "", false
	}
	match := false
	for _, id := range p.candidates {
		if chosen.ID == id {
			match = true
			break
		}
	}
	if !match {
		return "", false
	}
	synth := grammar.Frame{Force: grammar.ForceQuestion, Lang: p.lang, Wh: p.wh, Past: p.past}
	return d.answerRole(synth, p.rel, chosen, p.knownRole, p.verb)
}

// withInvite trails a teaching invitation after an admitted gap, so an unknown
// closes with an opening: the speaker is invited to fill it. The curiosity
// follow-up never asks after a word the admission already speaks, so when both
// fire they cover different gaps rather than doubling one.
func (d *Daemon) withInvite(lang, admission string) string {
	if admission == "" {
		return admission
	}
	word := d.gram.Word(lang, "invite")
	if word == "" {
		return admission
	}
	return admission + " " + upperFirst(word) + "."
}

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
	rel        string
	focusRole  string // "agent" or "patient": the role the focus plays
	coGloss    string
	coID       uint64
	weight     float64 // the fact's salience: its proposition's edge weight from the walk
	fromThread bool    // established this exchange, so kept whatever its entrenched weight
}

const (
	factCoArgCap  = 4 // co-arguments coordinated within one clause
	factClauseCap = 3 // relation clauses joined in a focused answer
	// describeClauseCap backstops a description against a pathological hub: a concept reached
	// by dozens of equally strong facts still stops at a paragraph. Within it, salience decides
	// the real length, so a sparsely grounded concept reads short and a richly grounded one
	// reads long.
	describeClauseCap = 8
)

// salienceFraction is where a description's tail is judged to have thinned: a fact weaker than
// this slice of the strongest is dropped, so the answer runs as long as the concept is richly
// connected and stops once the remaining facts are faint beside what was already said.
const salienceFraction = 0.15

// salientFacts orders a focus's facts by salience and stops when it thins: the facts
// established this exchange are kept whatever their entrenched weight (the talk just raised
// them), and the rest are taken strongest first until one falls below salienceFraction of the
// strongest or the ceiling is reached. So the length of a description follows how richly the
// focus is grounded rather than a fixed count.
func salientFacts(facts []focusFact, ceiling int) []focusFact {
	var thread, graph []focusFact
	for _, f := range facts {
		if f.fromThread {
			thread = append(thread, f)
		} else {
			graph = append(graph, f)
		}
	}
	sort.SliceStable(graph, func(i, j int) bool { return graph[i].weight > graph[j].weight })
	out := thread
	for _, f := range graph {
		if len(out) >= ceiling {
			break
		}
		if len(graph) > 0 && f.weight < graph[0].weight*salienceFraction {
			break
		}
		out = append(out, f)
	}
	return out
}

// aspectKey buckets a fact by the facet of the focus it speaks to, so a description can group
// like with like: what it is compared on, where it is, what it did, what was done to it. The
// bucket is read from the relation's grounded kind and the focus's role, never from a surface
// word, so the grouping is the same whatever tongue the description is later spoken in.
func (d *Daemon) aspectKey(f focusFact) string {
	switch {
	case d.factIsComparative(f.rel):
		return "comparison"
	case d.factIsLocative(f.rel):
		return "location"
	default:
		return "action:" + f.focusRole
	}
}

// planDescription orders a description's facts into a coherent sequence while touching no
// language: the facts established this exchange lead, then the rest group by aspect with the
// strongest-grounded aspect first and salience order kept within each group. So like is spoken
// with like rather than the aspects interleaving by raw strength. The plan is over meaning
// alone (the relation's kind, the role, the salience); the realizer adds the words per tongue,
// so the same plan reads in any tongue the facts can be spoken in.
func (d *Daemon) planDescription(facts []focusFact) []focusFact {
	type group struct {
		facts []focusFact
		peak  float64
	}
	var thread []focusFact
	var groups []*group
	byKey := map[string]*group{}
	for _, f := range facts {
		if f.fromThread {
			thread = append(thread, f)
			continue
		}
		k := d.aspectKey(f)
		g := byKey[k]
		if g == nil {
			g = &group{}
			byKey[k] = g
			groups = append(groups, g)
		}
		g.facts = append(g.facts, f)
		if f.weight > g.peak {
			g.peak = f.weight
		}
	}
	sort.SliceStable(groups, func(i, j int) bool { return groups[i].peak > groups[j].peak })
	// Within the comparisons, cluster the two poles of one scale so a switch between them
	// ("bigger than the snail", then "smaller than the lion") falls adjacent and reads as a
	// contrast at realization. The clustering is by the scale, a grounded property, not a word.
	if g := byKey["comparison"]; g != nil {
		sort.SliceStable(g.facts, func(i, j int) bool {
			return d.comparisonScale(d.comparisonDir(g.facts[i])) < d.comparisonScale(d.comparisonDir(g.facts[j]))
		})
	}
	out := thread
	for _, g := range groups {
		out = append(out, g.facts...)
	}
	return out
}

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
	add := func(rl, frole, gloss string, id uint64, weight float64, thread bool) {
		if id == 0 || rl == "" || !wantRole(frole) || !wantRel(rl) {
			return
		}
		k := key{rl, frole, id}
		if seen[k] {
			return
		}
		seen[k] = true
		// A denied proposition is no fact to speak: the told negative stands
		// over the stored positive here as it does on the polar path, so a
		// retracted or denied fact stops surfacing in role and describe answers.
		agent, patientID := focus, id
		if frole == "patient" {
			if frags, err := d.fragmentsByIDsTiered([]uint64{id}); err == nil && len(frags) == 1 {
				agent, patientID = frags[0], focus.ID
			} else {
				agent = nil // unreadable co-argument: keep the fact rather than judge it
			}
		}
		if agent != nil && (d.propositionDenied(agent, rl, patientID) ||
			d.propositionDeniedCategorically(agent, rl, patientID)) {
			return
		}
		out = append(out, focusFact{rel: rl, focusRole: frole, coGloss: gloss, coID: id, weight: weight, fromThread: thread})
	}
	for _, p := range d.threadProps {
		if p.agent == focus.ID {
			add(p.rel, "agent", p.patientGloss, p.patient, 0, true)
			threadCovered[p.rel+"|agent"] = true
		}
		if p.patient == focus.ID {
			add(p.rel, "patient", p.agentGloss, p.agent, 0, true)
			threadCovered[p.rel+"|patient"] = true
		}
	}
	dests, err := d.walkableDests(focus.ID)
	if err != nil {
		return out
	}
	for _, dst := range dests[memory.RelPatient] { // the focus as agent, reaching its patients
		if !threadCovered[dst.via+"|agent"] {
			add(dst.via, "agent", dst.gloss, dst.id, dst.weight, false)
		}
	}
	for _, dst := range dests[memory.RelAgent] { // the focus as patient, reaching its agents
		if !threadCovered[dst.via+"|patient"] {
			add(dst.via, "patient", dst.gloss, dst.id, dst.weight, false)
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
	return d.renderFactsCapped(lang, focusName, facts, past, continuation, factClauseCap)
}

// renderFactsCapped is renderFacts with the clause limit as a parameter, so a focused answer
// keeps the terse cap while a description can run longer, up to the count its salience-ordered
// facts fill.
func (d *Daemon) renderFactsCapped(lang, focusName string, facts []focusFact, past, continuation bool, maxClauses int) (string, bool) {
	type key struct{ rel, role string }
	var order []key
	coargs := map[key][]string{}
	for _, f := range facts {
		k := key{f.rel, f.focusRole}
		if _, ok := coargs[k]; !ok {
			if len(order) >= maxClauses {
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
	// Comparisons that share a standard speak as one predicate ("lighter and slower than the
	// cat"), so the standard is named once. The grouping is by the standard concept and the
	// direction each is spoken with (meaning); the groups order by scale so the two poles of one
	// scale stay adjacent and a switch between them still reads as a contrast at realization.
	type compGroup struct {
		name  string
		dirs  []string
		scale string
	}
	var compGroups []*compGroup
	byStd := map[string]*compGroup{}
	for _, k := range order {
		if !d.factIsComparative(k.rel) {
			continue
		}
		dir := k.rel
		if k.role == "patient" {
			opp := d.oppositeRelations(d.resolveRelation(k.rel).ID)
			if len(opp) == 0 {
				continue
			}
			dir = opp[0]
		}
		scale := d.comparisonScale(dir)
		for _, name := range coargs[k] {
			g := byStd[name]
			if g == nil {
				g = &compGroup{name: name, scale: scale}
				byStd[name] = g
				compGroups = append(compGroups, g)
			}
			has := false
			for _, dd := range g.dirs {
				if dd == dir {
					has = true
					break
				}
			}
			if !has {
				g.dirs = append(g.dirs, dir)
			}
			if scale < g.scale {
				g.scale = scale
			}
		}
	}
	sort.SliceStable(compGroups, func(i, j int) bool { return compGroups[i].scale < compGroups[j].scale })

	// Actions over the same subject and object coordinate their verbs into one clause ("loved and
	// ate the cheese"), so only the verb differs. Group the plain-action keys by the role and the
	// co-argument set; a group of several verbs speaks once, the verbs joined at realization.
	actionVerbs := map[string][]string{}
	for _, k := range order {
		if d.factIsLocative(k.rel) || d.factIsComparative(k.rel) {
			continue
		}
		gk := actionGroupKey(k.role, coargs[k])
		actionVerbs[gk] = append(actionVerbs[gk], k.rel)
	}

	var clauses []string
	compRendered := false              // the comparison block is emitted as a whole at its first key
	emittedAction := map[string]bool{} // a subject-object pair's verbs are spoken together, once
	priorCompDir := ""                 // the pole the last comparison was spoken with, to read a contrast off the next
	actionRun := 0                     // consecutive actions by the focus, so a further one reads as an addition
	for _, k := range order {
		names := coargs[k]
		// A locative relation speaks as a copula with a preposition ("the leaf is on
		// the tree"), not as a verb. The figure is the relation's agent and the
		// ground its patient, so the focus is the figure when it stood as agent and
		// the ground when it stood as patient; the co-arguments fill the other side.
		if d.factIsLocative(k.rel) {
			actionRun = 0 // a placement is not an action, so it breaks an addition run
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
		// A gradable relation speaks as a comparison ("the lion is bigger than the mouse"), not
		// a verb. The focus is the greater pole when it stood as agent and the lesser when it
		// stood as patient, so the lesser is spoken with the relation's opposite ("smaller").
		// Comparisons that share a standard coordinate into one clause; opposite poles of one
		// scale read as a contrast. The whole block is emitted at its first key, so the grouped
		// order holds rather than the key order.
		if d.factIsComparative(k.rel) {
			actionRun = 0 // a comparison is not an action, so it breaks an addition run
			if compRendered {
				continue
			}
			compRendered = true
			given := continuation || len(clauses) > 0
			for i := 0; i < len(compGroups); i++ {
				g := compGroups[i]
				dirs := g.dirs
				stds := []string{g.name}
				// Several standards the focus stands to in one direction coordinate ("lighter than
				// the cat and the dog"). They share a scale, so the sort placed them adjacent; gather
				// the run of single-pole groups in this direction.
				if len(dirs) == 1 {
					for i+1 < len(compGroups) && len(compGroups[i+1].dirs) == 1 && compGroups[i+1].dirs[0] == dirs[0] {
						i++
						stds = append(stds, compGroups[i].name)
					}
				}
				cl, ok := d.comparativeClause(lang, focusName, dirs, stds, given)
				if !ok {
					continue
				}
				// A switch to the opposite pole of the same scale reads as a contrast: "bigger
				// than the snail, but smaller than the lion." A coordinated group spans several
				// scales, so only a single-pole group carries a pole the next can oppose. The
				// relation is meaning (the opposite pole), the connective the tongue's seeded word.
				if len(dirs) == 1 && d.opposesDir(dirs[0], priorCompDir) {
					if c := d.gram.Word(lang, "contrast"); c != "" {
						cl = upperFirst(c) + " " + lowerFirst(cl)
					}
				}
				clauses = append(clauses, cl)
				if len(dirs) == 1 {
					priorCompDir = dirs[0]
				} else {
					priorCompDir = ""
				}
				given = true
			}
			continue
		}
		// A tongue can only state a fact in the words it has learned. When the relation has no
		// verb in a non-English tongue, the clause is left unsaid there rather than rendered as
		// the foreign content under the tongue's case morphology; English is the floor every
		// concept is named in, so it is not gated.
		if lang != "en" {
			if rel := d.resolveRelation(k.rel); rel == nil || !d.relationNamedIn(rel.ID, lang) {
				continue
			}
		}
		// Actions over the same subject and object speak together at the first of them, so a later
		// verb over the same pair does not open a clause of its own.
		gk := actionGroupKey(k.role, names)
		if emittedAction[gk] {
			continue
		}
		emittedAction[gk] = true
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
		// Several actions over the same subject and object coordinate their verbs into the one
		// clause. The clause is built for the first verb, then the others join it through the
		// coordinator ("loved and ate the cheese"). Each verb is conjugated by the same morphology
		// the clause used, so the join lands on the verb the clause speaks; if it does not match,
		// the single clause stands rather than a broken splice.
		if verbs := actionVerbs[gk]; ok && len(verbs) > 1 {
			conj := make([]string, len(verbs))
			for i, v := range verbs {
				conj[i] = d.gram.Conjugate(lang, d.relationName(v, lang), false, past, false)
			}
			phrase := d.joinCoordinated(lang, conj)
			if at := strings.Index(line, conj[0]); at >= 0 {
				line = line[:at] + phrase + line[at+len(conj[0]):]
			}
		}
		if ok {
			// A further action by the same subject reads as an addition. The focus is the
			// constant agent across these clauses, so a second action is something it also
			// did; the marker is spoken once, on the first continuation, the rest understood
			// as part of the same run. The discourse relation is meaning (same subject, another
			// action); the connective is the tongue's seeded word, so the plan reads in any tongue.
			if k.role == "agent" {
				if actionRun == 1 {
					if w := d.gram.Word(lang, "addition"); w != "" {
						line = d.placeAdditive(lang, w, line)
					}
				}
				actionRun++
			} else {
				actionRun = 0
			}
			clauses = append(clauses, line)
		}
	}
	if len(clauses) == 0 {
		return "", false
	}
	return strings.Join(clauses, " "), true
}

// placeAdditive sets the additive connective where the tongue's word order wants it. A verb-final
// tongue takes it at the front (Turkish "Ayrıca o ... kovaladı"); a verb-medial tongue takes it
// after the subject, which for a continuation is the given pronoun, a single word ("It also
// watched the deer"). The discourse relation is the same either way; only the placement is the
// tongue's, like the connective word itself.
func (d *Daemon) placeAdditive(lang, word, clause string) string {
	if d.gram.VerbFinal(lang) {
		return upperFirst(word) + " " + lowerFirst(clause)
	}
	subject, rest, found := strings.Cut(clause, " ")
	if !found {
		return upperFirst(word) + " " + lowerFirst(clause)
	}
	return subject + " " + word + " " + rest
}

// actionGroupKey identifies the actions that share a subject and object: the role the focus plays
// and the co-argument set, so verbs over the same pair group together however the facts were
// ordered. The co-arguments are sorted so the key does not depend on the order they were gathered.
func actionGroupKey(role string, coargs []string) string {
	names := append([]string(nil), coargs...)
	sort.Strings(names)
	return role + "\x00" + strings.Join(names, "\x00")
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
