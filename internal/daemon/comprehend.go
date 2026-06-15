package daemon

import (
	"fmt"
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
	// The proposition is in the graph now; the exchange remembers it set this
	// down, so a later "what about X" leans on what was just said.
	if patientID != 0 {
		d.recordThreadProp(rel.Content, agent.Concept.ID, patientID, agent.Concept.Content, patientGloss)
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
	questioned, cross := "patient", memory.RelPatient
	if knownRole == "patient" {
		questioned, cross = "agent", memory.RelAgent
	}
	past := frame.Past || d.gram.VerbPast(frame.Lang, verb)
	dests, err := d.walkableDests(known.ID)
	if err != nil {
		return "", false
	}
	// The answer is the thread's established co-arguments, newest first: "what
	// did the dog chase" after two tellings names both, and a fresh claim is
	// never lost behind the strongest memory. With nothing set down this exchange
	// the walk's strongest stands, so a cold question still reads the graph.
	fillers := d.threadFillers(rel.Content, known.ID, questioned)
	if len(fillers) == 0 {
		for i := range dests[cross] {
			if dests[cross][i].via == rel.Content {
				fillers = []walkDest{dests[cross][i]}
				break
			}
		}
	}
	// The relation and its tense are the thread an elliptical follow-up inherits.
	d.lastRole = &roleContext{rel: rel, knownRole: knownRole, verb: verb, past: past}
	if len(fillers) == 0 {
		// The relation is known but nothing filled the questioned role: the
		// honest answer is the gap, spoken through the older admission.
		return d.realizeAs(frame.Lang, grammar.Frame{Force: grammar.ForceStatement, Action: grammar.Term{Phrase: "know"},
			Negate: true, Past: past, Patient: grammar.Term{Phrase: frame.Wh}}, d.chem.Snapshot()), true
	}
	// Coordinate the co-arguments into one phrase, "the cat and the mouse", each
	// made definite on its own so the conjunction reads naturally, the freer
	// answer composed from several propositions rather than fixed to one clause.
	parts := make([]string, len(fillers))
	for i, f := range fillers {
		parts[i] = definiteNP(f.gloss)
	}
	line, ok := d.sayProposition(frame.Lang, rel.Content,
		map[string]string{knownRole: known.Content, questioned: strings.Join(parts, " and ")}, past)
	if !ok {
		return "", false
	}
	// The answer's referents become warm, so the conversational thread follows
	// what was just said.
	for _, f := range fillers {
		d.ws.excite(f.id, exciteSpeech)
	}
	d.warmSpoken(known.Content)
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
	if frame.Wh != "what" || frame.Agent.Concept == nil || !proVerbDo(raw) {
		return "", false
	}
	agent := frame.Agent.Concept
	dests, err := d.walkableDests(agent.ID)
	if err != nil || len(dests[memory.RelPatient]) == 0 {
		return "", false
	}
	// walkableDests sorts each crossing by weight, so the first is the
	// strongest proposition the agent stood as the agent of.
	filler := dests[memory.RelPatient][0]
	if filler.via == "" {
		return "", false
	}
	line, ok := d.sayProposition(frame.Lang, filler.via,
		map[string]string{"agent": agent.Content, "patient": filler.gloss}, frame.Past)
	if !ok {
		return "", false
	}
	d.ws.excite(filler.id, exciteSpeech)
	d.warmSpoken(agent.Content)
	return line, true
}

// proVerbDo reports whether a clause's main verb is the generic pro-verb,
// "what did the dog do" rather than "what did the dog chase": the last word
// is a form of "do", which stands in for the relation the question asks
// after.
func proVerbDo(raw string) bool {
	tokens, _ := grammar.Tokenize(raw)
	if len(tokens) == 0 {
		return false
	}
	switch tokens[len(tokens)-1] {
	case "do", "does":
		return true
	}
	return false
}

// sayProposition builds a meaning over a relation and its roles and hands it
// to the bidirectional engine, the one entry the relational answers speak
// through. Each role's gloss takes the definite article an answer wants. It
// returns false when no construction speaks the meaning, so the caller can
// fall back.
func (d *Daemon) sayProposition(lang, rel string, roles map[string]string, past bool) (string, bool) {
	terms := make(map[string]grammar.Term, len(roles))
	for role, gloss := range roles {
		terms[role] = grammar.Term{Phrase: definiteNP(gloss)}
	}
	return d.gram.Speak(lang, grammar.Meaning{
		Relation: grammar.Term{Phrase: rel},
		Roles:    terms,
		Past:     past,
	})
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

// definiteNP gives a concept's gloss the definite article an answer wants,
// unless the gloss already carries a determiner, so a bare "dog" speaks as
// "the dog" while "a dog" is left alone.
func definiteNP(gloss string) string {
	lead, _, _ := strings.Cut(gloss, " ")
	switch lead {
	case "a", "an", "the", "this", "that", "my", "your", "its":
		return gloss
	}
	return "the " + gloss
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

// roleFit scores a concept's affinity for a thematic role from what has
// filled that role before: a concept that has stood as the agent of past
// propositions, or whose feature signature overlaps those that have, fits
// the agent role, so best fit reads an animacy off lived usage rather than a
// hand-listed table. It is empty until propositions accumulate, so the word
// order alone binds the first tellings and the features sharpen the binding
// only once there is evidence.
func (d *Daemon) roleFit(c *memory.Fragment, role string) float64 {
	rel := memory.RelAgent
	if role == "patient" {
		rel = memory.RelPatient
	}
	// Direct history: the role edges arriving at this concept.
	direct := 0.0
	if edges, err := d.store.LinksToRel([]uint64{c.ID}, rel); err == nil {
		for _, e := range edges {
			direct += e.Weight
		}
	}
	return direct + d.roleKinship(c, rel)
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
