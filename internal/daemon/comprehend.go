package daemon

import (
	"fmt"
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
	if patient := m.Roles["patient"]; patient.Concept != nil {
		patientID = patient.Concept.ID
	}
	if err := d.store.Propose(rel.ID, momentID, agent.Concept.ID, patientID, comprehendWeight); err != nil {
		fmt.Fprintf(d.out, "propose error: %v\n", err)
		return false
	}
	// The proposition is in the graph now; warm what it bound and record the
	// role edges on the trail, so the write is auditable like a naming is.
	d.ws.excite(rel.ID, exciteSpeech)
	d.ws.excite(agent.Concept.ID, exciteSpeech)
	d.event(events.Event{Ev: "link", From: momentID, To: agent.Concept.ID, Rel: memory.RelAgent, Weight: comprehendWeight})
	if patientID != 0 {
		d.ws.excite(patientID, exciteSpeech)
		d.event(events.Event{Ev: "link", From: momentID, To: patientID, Rel: memory.RelPatient, Weight: comprehendWeight})
	}
	return true
}

// relationConcept resolves the relation a proposition binds into, minting it
// when the verb is new. A relation is a concept like any other, grounded,
// named, and weighted, but it lives under a key that marks it a relation and
// carries the role slots its instances fill. The verb's lemma names it, so a
// later telling resolves to the same relation rather than minting a second,
// and a noun that happens to share the surface is not mistaken for it.
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
	// The verb's lemma names the relation, so the next telling resolves here.
	if w, _, err := d.store.WordNode(lemma, lang, d.chem.Snapshot(), now); err == nil {
		if err := d.store.SetLinkRel(w.ID, c.ID, memory.RelNames, opNameWeight); err == nil {
			d.event(events.Event{Ev: "name", From: w.ID, To: c.ID, Rel: memory.RelNames, Weight: opNameWeight})
		}
	}
	return c, nil
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
