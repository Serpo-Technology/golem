package daemon

import (
	"fmt"
	"strings"
	"time"

	"golem/internal/events"
	"golem/internal/grammar"
	"golem/internal/memory"
)

// A justification trace is the proof behind a spoken claim, kept as graph
// structure: when GOLEM asserts a proposition, a trace moment records what the
// assertion stood on at that instant, so "why do you say that" reads the actual
// support rather than re-deriving a basis that may have drifted, and a repair
// can retract every claim that leaned on a fact it just corrected away.
//
// The trace is an episodic moment on its own channel. Its said-edges identify
// the asserted proposition without wearing the proposition shape (no grounds
// into it, no role edges out of it), so no walk that looks for observed facts
// can ever mistake the trace of a guess for the fact itself. Its drewon edges
// point at the support: the moment that holds the proposition, or the concept
// whose membership or resemblance lent the guess beside the lender's own
// moment. The evidence kind is never labeled; the reader derives it from the
// support fragment live (a told moment carries its source, a percept its
// channel), so the proof is always read from the graph as it stands. Traces
// are ordinary episodic citizens of consolidation: they decay nightly, fold at
// the merge when a claim is re-asserted, and hold nothing alive, since their
// rels appear in no prune predicate; when the evidence has faded, the reader
// says so.
const (
	claimChannel     = "claim"
	claimTraceWeight = 0.5
	justifyTraceCap  = 2 // traces read back per probe, so the proof stays short
)

// claimTemplate is the canonical key of a claim's trace: relation content and
// role ids, free of any tongue, so re-asserting the claim in any language
// refreshes the one trace instead of minting a twin. An is-a claim, which has
// no relation concept, carries the "isa" word in the relation slot.
func claimTemplate(relContent string, agentID, patientID uint64) string {
	return fmt.Sprintf("claim %s a%d p%d", relContent, agentID, patientID)
}

// traceClaim sets down the justification of a proposition GOLEM just asserted:
// one trace moment per claim, refreshed in place on re-assertion, its evidence
// linked as it stood at this speaking. spoken is the line as uttered, kept as
// the trace's readable content; when empty, the template stands in. A trace
// with no evidence still records that the claim was made, so the reader can
// admit what it has nothing to point to and repair can find the claim.
func (d *Daemon) traceClaim(relContent string, agentID, patientID uint64, spoken string, evidence []uint64) {
	if agentID == 0 || patientID == 0 || relContent == "" {
		return
	}
	tpl := claimTemplate(relContent, agentID, patientID)
	content := spoken
	if content == "" {
		content = tpl
	}
	trace, err := d.store.FragmentByTemplate(tpl)
	if err == nil && trace != nil && trace.Channel == claimChannel {
		// The claim again: the standing trace is the record, refreshed rather
		// than doubled, its weight restored and any newly-consulted support
		// joined to what it already points at.
		d.store.SetWeight(trace.ID, claimTraceWeight)
		d.store.UpdateContent(trace.ID, content, tpl)
	} else {
		trace = &memory.Fragment{Kind: memory.KindEpisodic, Channel: claimChannel,
			Content: content, Template: tpl, Weight: claimTraceWeight,
			Limbic: d.chem.Snapshot(), Time: time.Now()}
		if _, err := d.store.Encode(trace); err != nil {
			return
		}
		// The trace's said-relation is the relation concept, or the scale dimension when
		// the claim is a comparison ordering, so traceTuple reads a non-zero relation and
		// the reader does not misclassify a comparative claim as an is-a stub.
		rel := d.resolveRelation(relContent)
		if rel == nil {
			rel = d.scaleDimension(relContent)
		}
		if rel != nil {
			d.store.LinkRel(trace.ID, rel.ID, memory.RelSaidRel, claimTraceWeight)
		}
		d.store.LinkRel(trace.ID, agentID, memory.RelSaidAgent, claimTraceWeight)
		d.store.LinkRel(trace.ID, patientID, memory.RelSaidPatient, claimTraceWeight)
	}
	for _, ev := range evidence {
		if ev == 0 || ev == trace.ID {
			continue
		}
		if err := d.store.LinkRel(trace.ID, ev, memory.RelDrewOn, claimTraceWeight); err == nil {
			d.event(events.Event{Ev: "link", From: trace.ID, To: ev,
				Rel: memory.RelDrewOn, Weight: claimTraceWeight})
		}
	}
	d.noteTraced(trace.ID)
}

// noteTraced keeps the turn's traces in mind, newest answer only: the first
// trace of a turn displaces the previous turn's set, so "why do you say that"
// reads the proof of the last assertion and never a stale one.
func (d *Daemon) noteTraced(id uint64) {
	if !d.turnTraced {
		d.lastTrace = d.lastTrace[:0]
		d.turnTraced = true
	}
	d.lastTrace = append(d.lastTrace, id)
}

// polarEvidence gathers what a just-asserted proposition stands on: the moment
// that holds it, when one is stored (an observed fact, or a telling whose
// source the reader will name), and the lender beside the lender's own moment
// when a membership or resemblance carried the guess. A composed deduction
// (a transitive chain, a containment walk) stores none of its steps, so its
// trace stays bare and the reader owns up to reasoning it out.
func (d *Daemon) polarEvidence(relContent string, agentID, patientID, lender uint64) []uint64 {
	rel := d.resolveRelation(relContent)
	if rel == nil {
		// A comparison ordering grounds on its scale dimension, not a relation, and the
		// agent/patient the caller passes are the canonical greater/lesser pair, so the
		// moment reads in the direction the ordering was stored.
		rel = d.scaleDimension(relContent)
	}
	if rel == nil {
		return nil
	}
	var ev []uint64
	if m, ok := d.propositionMoment(rel, agentID, patientID); ok {
		ev = append(ev, m)
	}
	if lender != 0 {
		ev = append(ev, lender)
		if m, ok := d.propositionMoment(rel, lender, patientID); ok {
			ev = append(ev, m)
		}
	}
	return ev
}

// planJustify answers the epistemic probe: "why do you say that" or "how do
// you know" reads the trace of the last assertion and speaks its actual
// support. The ask phrases are seeded surfaces per tongue, matched whole, and
// the reply speaks the tongue whose phrase matched, so an untagged Turkish
// probe still lands. It returns false on any other clause.
func (d *Daemon) planJustify(frame grammar.Frame, raw string) (string, bool) {
	ask := strings.Join(strings.Fields(strings.ToLower(strings.Trim(raw, " ?.!"))), " ")
	if ask == "" {
		return "", false
	}
	lang := ""
	for _, lg := range []string{frame.Lang, "en", "tr", "la"} {
		if lg == "" {
			continue
		}
		if ask == d.gram.Word(lg, "justifyask") || ask == d.gram.Word(lg, "justifyask2") {
			lang = lg
			break
		}
	}
	if lang == "" {
		return "", false
	}
	// Reading the proof is not asserting: the traces stay in mind, so the
	// probe can be asked twice and answer twice.
	d.turnTraced = true
	if len(d.lastTrace) == 0 {
		return justifySurface(d, lang, "justifynone"), true
	}
	var lines []string
	seen := map[string]bool{}
	for i, tid := range d.lastTrace {
		if i >= justifyTraceCap {
			break
		}
		// Two facts proved the same way share one verdict: "You told me." once,
		// not once per fact.
		if line := d.justifyTrace(lang, tid); line != "" && !seen[line] {
			seen[line] = true
			lines = append(lines, line)
		}
	}
	if len(lines) == 0 {
		// The claim is on record but its support has faded or been overturned
		// since: the honest reading of a proof that is no longer there.
		return justifySurface(d, lang, "justifylost"), true
	}
	return strings.Join(lines, " "), true
}

// justifySurface speaks one seeded justification surface as a sentence. A
// tongue without one keeps the old silence, the same bargain every seeded
// surface makes.
func justifySurface(d *Daemon, lang, key string) string {
	w := d.gram.Word(lang, key)
	if w == "" {
		return ""
	}
	return upperFirst(w) + "."
}

// traceTuple reads a trace's identity off its said-edges: the relation concept,
// the agent, the patient, and the support it drew on. relID is zero for an is-a
// stub, which carries no relation concept.
func (d *Daemon) traceTuple(traceID uint64) (relID, agentID, patientID uint64, drewOn []uint64) {
	edges, err := d.store.LinksFrom([]uint64{traceID})
	if err != nil {
		return 0, 0, 0, nil
	}
	for _, e := range edges {
		switch e.Rel {
		case memory.RelSaidRel:
			relID = e.To
		case memory.RelSaidAgent:
			agentID = e.To
		case memory.RelSaidPatient:
			patientID = e.To
		case memory.RelDrewOn:
			drewOn = append(drewOn, e.To)
		}
	}
	return relID, agentID, patientID, drewOn
}

// justifyTrace renders one trace's proof from live state: the claim identity
// is read off the said-edges, the support off the drewon edges, and each piece
// speaks by what it is. It returns "" when nothing on the trace still stands,
// so the caller can admit the loss.
func (d *Daemon) justifyTrace(lang string, traceID uint64) string {
	relID, agentID, patientID, evidence := d.traceTuple(traceID)
	if agentID == 0 || patientID == 0 {
		return ""
	}
	relContent := ""
	if relID != 0 {
		if rf, err := d.store.FragmentsByIDs([]uint64{relID}); err == nil && len(rf) == 1 {
			relContent = rf[0].Content
		}
	}
	// An is-a claim carries no relation concept; its ground is the membership
	// itself, re-read live, the tally or the telling saying which kind it is.
	if relID == 0 {
		return d.justifyIsA(lang, agentID, patientID)
	}
	// A denial told since the claim stands over the stored support: the proof
	// no longer holds, so nothing here is offered as one.
	if af, err := d.fragmentsByIDsTiered([]uint64{agentID}); err == nil && len(af) == 1 {
		if d.propositionDenied(af[0], relContent, patientID) ||
			d.propositionDeniedCategorically(af[0], relContent, patientID) {
			return ""
		}
	}
	frags, err := d.fragmentsByIDsTiered(evidence)
	if err != nil {
		frags = nil
	}
	// The tie is spoken before the fact it lent and the fact before where it
	// came from, whatever order the store returned the support in.
	var lines []string
	for _, f := range frags {
		if f.Kind != memory.KindConcept {
			continue
		}
		if line := d.justifyLender(lang, agentID, f); line != "" {
			lines = append(lines, line)
		}
	}
	var verdict string
	for _, f := range frags {
		if f.Kind != memory.KindEpisodic {
			continue
		}
		// The moment that holds the claimed proposition itself justifies by
		// where it came from; a lender's moment also re-tells its clause,
		// the fact the guess rested on.
		a, p, rel, ok := d.claimAt(f.ID)
		if ok && !(a == agentID && p == patientID && rel == relContent) {
			if clause := d.momentClause(lang, a, p, rel); clause != "" {
				lines = append(lines, clause)
			}
		}
		if v := d.justifyProvenance(lang, f); v != "" {
			verdict = v
		}
	}
	if verdict != "" {
		lines = append(lines, verdict)
	}
	if len(lines) == 0 {
		// The trace stored no steps that still stand. A claim that still
		// derives is a composed deduction, owned as GOLEM's own reasoning; one
		// that no longer holds has lost its proof, and the caller admits it.
		af, aerr := d.fragmentsByIDsTiered([]uint64{agentID})
		pf, perr := d.fragmentsByIDsTiered([]uint64{patientID})
		if aerr == nil && perr == nil && len(af) == 1 && len(pf) == 1 {
			if belief, _, _, _ := d.assessProposition(af[0], relContent, pf[0]); belief == beliefObserved || belief == beliefInferred {
				return justifySurface(d, lang, "justifyreason")
			}
		}
		return ""
	}
	return strings.Join(lines, " ")
}

// justifyProvenance speaks how a supporting moment entered memory: a telling
// names its teller (the architect's untagged tellings are the addressee's), a
// moment from any perceptual channel was seen.
func (d *Daemon) justifyProvenance(lang string, m *memory.Fragment) string {
	if m.Channel != "tell" && m.Channel != "thought" {
		return justifySurface(d, lang, "justifysaw")
	}
	src, err := d.store.LinksFromRel([]uint64{m.ID}, memory.RelSource)
	if err != nil || len(src) == 0 {
		return justifySurface(d, lang, "justifytoldyou")
	}
	sf, err := d.store.FragmentsByIDs([]uint64{src[0].To})
	if err != nil || len(sf) != 1 {
		return justifySurface(d, lang, "justifytoldso")
	}
	told := d.gram.Word(lang, "justifytold")
	if told == "" {
		return justifySurface(d, lang, "justifytoldso")
	}
	name := d.conceptName(sf[0].ID, sf[0].Content, lang)
	return upperFirst(name) + " " + told + "."
}

// justifyLender speaks the tie that let a concept lend the claim: membership
// when the agent grounds under it ("A cat is an animal."), likeness otherwise
// ("The wolf is like a dog."), the same copular composition the guess
// explanations use.
func (d *Daemon) justifyLender(lang string, agentID uint64, lender *memory.Fragment) string {
	af, err := d.fragmentsByIDsTiered([]uint64{agentID})
	if err != nil || len(af) != 1 {
		return ""
	}
	agentName := d.conceptName(agentID, af[0].Content, lang)
	lenderName := d.conceptName(lender.ID, lender.Content, lang)
	if d.isA(agentID, lender.ID) || d.toldIsA(agentID, lender.ID) {
		return d.realizeAs(lang, grammar.Frame{Force: grammar.ForceStatement,
			Agent:   grammar.Term{Phrase: grammar.Indefinite(agentName)},
			Action:  grammar.Term{Phrase: "be"},
			Patient: grammar.Term{Phrase: grammar.Indefinite(lenderName)}}, d.chem.Snapshot())
	}
	like := strings.TrimSpace(d.gram.Word(lang, "islike") + " " + grammar.Indefinite(lenderName))
	return d.realizeAs(lang, grammar.Frame{Force: grammar.ForceStatement,
		Agent:   grammar.Term{Phrase: agentName},
		Action:  grammar.Term{Phrase: "be"},
		Patient: grammar.Term{Phrase: like}}, d.chem.Snapshot())
}

// momentClause re-tells the proposition a moment holds, past, the way the why
// answers speak a told cause. It returns "" when the tongue cannot speak it.
func (d *Daemon) momentClause(lang string, agentID, patientID uint64, relContent string) string {
	pair, err := d.fragmentsByIDsTiered([]uint64{agentID, patientID})
	if err != nil || len(pair) != 2 {
		return ""
	}
	gloss := map[uint64]string{}
	for _, f := range pair {
		gloss[f.ID] = d.conceptName(f.ID, f.Content, lang)
	}
	meaning := grammar.Meaning{
		Relation: grammar.Term{Phrase: d.relationName(relContent, lang)},
		Lang:     lang,
		Past:     true,
		Roles: map[string]grammar.Term{
			"agent":   {Phrase: gloss[agentID], Definite: true},
			"patient": {Phrase: d.objectCase(gloss[patientID], lang), Definite: true},
		},
	}
	line, ok := d.gram.Speak(lang, meaning)
	if !ok {
		line, ok = d.gram.SpeakDecl(lang, meaning)
	}
	if !ok {
		return ""
	}
	return line
}

// justifyIsA grounds an is-a claim live: a membership GOLEM has seen speaks
// its tally, one it was only told owns the telling. The stub trace stores no
// evidence because the membership machinery is its own record, re-read here
// so the proof is always current. A membership reached only by climbing the derived
// kind ladder has no record of its own to speak, so nothing comes back and the reader
// admits it has nothing to point at.
func (d *Daemon) justifyIsA(lang string, memberID, catID uint64) string {
	if d.isA(memberID, catID) || memberID == catID {
		if tally, err := d.store.GroundingTally(); err == nil && tally[memberID] > 0 {
			if line := d.countClause(lang, tally[memberID], d.chem.Snapshot()); line != "" {
				return line
			}
		}
		return justifySurface(d, lang, "justifysaw")
	}
	if d.toldIsA(memberID, catID) {
		return justifySurface(d, lang, "justifytoldso")
	}
	return ""
}

// claimTraces finds the trace of a proposition by its canonical key, the one
// entry traceClaim keeps per claim, so a retraction can take the record with
// the claim.
func (d *Daemon) claimTraces(relContent string, agentID, patientID uint64) []uint64 {
	trace, err := d.store.FragmentByTemplate(claimTemplate(relContent, agentID, patientID))
	if err != nil || trace == nil || trace.Channel != claimChannel {
		return nil
	}
	return []uint64{trace.ID}
}

// dropTrace removes a trace and forgets it from the turn's record, so a
// retracted claim leaves no proof behind to read back.
func (d *Daemon) dropTrace(id uint64) {
	d.store.DeleteByID(id)
	kept := d.lastTrace[:0]
	for _, t := range d.lastTrace {
		if t != id {
			kept = append(kept, t)
		}
	}
	d.lastTrace = kept
}

// cutSupport retracts every claim that drew on the given moments: the trace
// names the claim it recorded, and a claim whose support was just corrected
// away goes with it, out of working memory, the belief store, the discourse
// record, and the trace itself. This is how a repair reaches the guesses that
// leaned on the fact it overturned, not only the claims that named it.
func (d *Daemon) cutSupport(moments []uint64) {
	if len(moments) == 0 {
		return
	}
	dependents, err := d.store.LinksToRel(moments, memory.RelDrewOn)
	if err != nil {
		return
	}
	for _, dep := range dependents {
		edges, err := d.store.LinksFrom([]uint64{dep.From})
		if err != nil {
			continue
		}
		var relID, agentID, patientID uint64
		for _, e := range edges {
			switch e.Rel {
			case memory.RelSaidRel:
				relID = e.To
			case memory.RelSaidAgent:
				agentID = e.To
			case memory.RelSaidPatient:
				patientID = e.To
			}
		}
		if relID != 0 && agentID != 0 && patientID != 0 {
			if rf, err := d.store.FragmentsByIDs([]uint64{relID}); err == nil && len(rf) == 1 {
				d.held.forget(rf[0].Content, agentID, patientID)
				d.store.ForgetBelief(rf[0].Content, agentID, patientID)
				d.removeClaim(rf[0].Content, agentID, patientID)
			}
		}
		d.dropTrace(dep.From)
	}
}
