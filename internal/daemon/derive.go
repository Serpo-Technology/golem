package daemon

import (
	"strings"

	"golem/internal/grammar"
	"golem/internal/memory"
)

// Consolidation draws two kinds of conclusion about the day's concepts. The hedged kind
// (drawInferenceAbout) transfers what a resembling neighbour does as a guess. This is the FIRM kind:
// the SOUND deductions the day's facts newly enable -- a transitive chain closed, a category
// composed, a symmetric relation reversed, a containment carried through -- worked out unprompted and
// REMEMBERED, so GOLEM's knowledge grows while it rests. A derived fact is certain (it follows from
// observed facts by a sound rule), so it is spoken firm, not hedged; but it is kept apart from the
// observed graph and labeled as reasoned, so it is never mistaken for something seen. Crucially the
// store is an INDEX, not the authority: every time a derived fact is surfaced it is RE-VERIFIED live
// against the graph (soundlyDerivesFirm), so a premise retracted since means the deduction no longer
// holds and the fact silently stops being spoken -- stale-firm-assertion is impossible with no cascade
// wiring, because the firm answer was always recomputed, never trusted from a row.

const (
	// A night works out a bounded handful of new facts, so consolidation grows knowledge steadily
	// rather than sweeping the whole closure of the graph in one sleep.
	deriveCapPerTopic = 6
	deriveCapPerNight = 24

	// The census names at most a handful of worked-out facts, so "what do you know" stays a summary.
	derivedSummaryCap = 5

	derivedTransitive = "transitive"
)

// soundlyDerivesFirm reports whether a proposition is a FIRM DERIVED fact: a sound deduction
// assessProposition would answer "yes" for, but that the graph does not hold directly. It mirrors
// assessProposition's beliefObserved-via-deduction branches in the same order (a told negative gates
// everything; a direct fact is observed not derived; then categorical composition, a told-transitive
// chain, a symmetric reversal, a containment chain), but WITHOUT its hedged tail and so without the
// held-belief side effect that tail carries -- this is the side-effect-free gate the nightly draw, the
// re-verify, and the "how sure" reading all share, so the store can never disagree with the live
// answer.
func (d *Daemon) soundlyDerivesFirm(agent *memory.Fragment, relContent string, patient *memory.Fragment) bool {
	if agent == nil || patient == nil {
		return false
	}
	if d.propositionDenied(agent, relContent, patient.ID) {
		return false // a told negative denies it, the defeasible exception assessProposition weighs first
	}
	if d.propositionHolds(agent, relContent, patient.ID) {
		return false // directly observed: a fact in the graph, not a derived one
	}
	if d.propositionHoldsCategorically(agent, relContent, patient) {
		return true
	}
	if agent.ID != patient.ID {
		if rel := d.resolveRelation(relContent); rel != nil && d.relationIsTransitive(rel.ID) &&
			d.comparesTransitively(rel, agent.ID, patient.ID) {
			return true
		}
		if d.symmetricHolds(agent.ID, relContent, patient.ID) {
			return true
		}
		if relContent == containmentPrep && d.locativelyInside(agent.ID, patient.ID, locativeChainDepth) {
			return true
		}
	}
	return false
}

// derivesStill re-verifies a stored derived fact against the live graph, reading the current
// fragments so a merged or pruned argument reads as no longer derivable.
func (d *Daemon) derivesStill(f memory.DerivedFact) bool {
	frags, err := d.fragmentsByIDsTiered([]uint64{f.Agent, f.Patient})
	if err != nil || len(frags) != 2 {
		return false
	}
	byID := map[uint64]*memory.Fragment{frags[0].ID: frags[0], frags[1].ID: frags[1]}
	return d.soundlyDerivesFirm(byID[f.Agent], f.Rel, byID[f.Patient])
}

// verifiedDerived reads the ledger and keeps only the conclusions that still hold against the live
// graph, the re-verify every surfacing runs so a derived fact retracted at its root stops being shown.
// It does NOT prune the stale rows (a read must not write); the nightly reverifyDerived does that.
func (d *Daemon) verifiedDerived() []memory.DerivedFact {
	facts, err := d.store.DerivedFacts()
	if err != nil {
		return nil
	}
	var out []memory.DerivedFact
	for _, f := range facts {
		if d.derivesStill(f) {
			out = append(out, f)
		}
	}
	return out
}

// verifiedDerivedAbout is verifiedDerived scoped to one agent, for a description of that concept.
func (d *Daemon) verifiedDerivedAbout(agent uint64) []memory.DerivedFact {
	facts, err := d.store.DerivedAbout(agent)
	if err != nil {
		return nil
	}
	var out []memory.DerivedFact
	for _, f := range facts {
		if d.derivesStill(f) {
			out = append(out, f)
		}
	}
	return out
}

// reverifyDerived drops every derived row that no longer holds against the settled graph, the nightly
// hygiene that keeps the ledger a pure function of the graph so it does not accrete stale conclusions.
// It runs on the sleep path (single-writer), where a write is safe. Returns how many it pruned.
func (d *Daemon) reverifyDerived() int {
	facts, err := d.store.DerivedFacts()
	if err != nil {
		return 0
	}
	dropped := 0
	for _, f := range facts {
		if !d.derivesStill(f) {
			d.store.ForgetDerived(f.Rel, f.Agent, f.Patient)
			dropped++
		}
	}
	return dropped
}

// deriveFirmAbout works out and remembers the sound conclusions a concept newly stands in as agent,
// bounded by the budget. This first kind is the told-transitive closure: for each relation the concept
// was told is transitive, the concepts it reaches through the chain but was not directly told it
// exceeds are firm derived facts. It returns how many fresh conclusions it drew.
func (d *Daemon) deriveFirmAbout(topic *memory.Fragment, budget int) int {
	if budget <= 0 || topic.Kind != memory.KindConcept || d.isSystemConcept(topic.ID) {
		return 0
	}
	if budget > deriveCapPerTopic {
		budget = deriveCapPerTopic
	}
	drawn := 0
	seenRel := map[string]bool{}
	for _, f := range d.gatherFacts(topic, "agent", "") {
		if drawn >= budget {
			break
		}
		if seenRel[f.rel] {
			continue
		}
		seenRel[f.rel] = true
		rel := d.resolveRelation(f.rel)
		if rel == nil || !d.relationIsTransitive(rel.ID) {
			continue // only a relation told transitive chains; an ordinary one derives nothing
		}
		reached := d.comparativeReach(rel, topic.ID)
		ids := make([]uint64, 0, len(reached))
		for id := range reached {
			if id != topic.ID {
				ids = append(ids, id)
			}
		}
		frags, err := d.fragmentsByIDsTiered(ids)
		if err != nil {
			continue
		}
		for _, pf := range frags {
			if drawn >= budget {
				break
			}
			// The gate re-confirms soundness AND that the fact is not already observed or denied,
			// so only a genuinely new, sound conclusion is kept.
			if !d.soundlyDerivesFirm(topic, f.rel, pf) {
				continue
			}
			if has, err := d.store.HasDerived(f.rel, topic.ID, pf.ID); err != nil || has {
				continue
			}
			if err := d.store.SaveDerived(memory.DerivedFact{Rel: f.rel, Agent: topic.ID, Patient: pf.ID,
				AgentGloss: topic.Content, PatGloss: pf.Content, Kind: derivedTransitive}); err != nil {
				continue
			}
			drawn++
		}
	}
	return drawn
}

// derivedFocusFacts turns a concept's re-verified derived facts into the focusFact shape the fact
// renderers speak. A derived fact is never a directly observed one (the gate excludes an observed
// proposition), so it always adds something the seen tier did not say, even on a relation the focus is
// also observed in (seen to outrank the dog, reasoned to outrank the mouse).
func (d *Daemon) derivedFocusFacts(agent uint64) []focusFact {
	var facts []focusFact
	for _, df := range d.verifiedDerivedAbout(agent) {
		facts = append(facts, focusFact{rel: df.Rel, focusRole: "agent", coGloss: df.PatGloss, coID: df.Patient})
	}
	return facts
}

// reasonedElaboration is the FIRM middle tier of a description, between what the focus is seen to do
// and what it probably also does: the sound conclusions GOLEM has worked out about it, spoken plainly
// (a sound deduction is certain, so it is not hedged) but labeled as reasoned, so it reads as
// something worked out rather than seen. It marks the relations it speaks so the hedged tier below
// does not guess at them again. English-first; other tongues fall through to the seen and hedged tiers
// until their reasoning lead is chosen. Returns false when nothing has been reasoned about the focus.
func (d *Daemon) reasonedElaboration(frame grammar.Frame, focus *memory.Fragment, observed map[string]bool) (string, bool) {
	if frame.Lang != "en" {
		return "", false
	}
	facts := d.derivedFocusFacts(focus.ID)
	if len(facts) == 0 {
		return "", false
	}
	line, ok := d.renderFactsCapped(frame.Lang, d.conceptName(focus.ID, focus.Content, frame.Lang),
		facts, true, true, describeClauseCap)
	if !ok {
		return "", false
	}
	for _, f := range facts {
		observed[f.rel] = true // spoken as reasoned now, so the hedge below does not re-guess it
	}
	return "I have reasoned that " + lowerFirst(line), true
}

// derivedSummaryLine is the census clause for what GOLEM has worked out: the re-verified derived facts
// grouped by their agent and spoken as named clauses ("the lion outranks the wolf"), so "what do you
// know" shows the knowledge grown by reasoning, distinct from what was told. English-first; empty when
// nothing has been derived or the tongue is not yet phrased.
func (d *Daemon) derivedSummaryLine(lang string) string {
	if lang != "en" {
		return ""
	}
	facts, err := d.store.DerivedFacts()
	if err != nil {
		return ""
	}
	// Collect a bounded sample and re-verify lazily: the census names at most derivedSummaryCap
	// agents with a couple of facts each, so re-verify (a live graph walk per row) stops as soon as
	// enough have survived rather than re-verifying the whole accumulated ledger for a short summary.
	const factsPerAgent = 2
	byAgent := map[uint64][]focusFact{}
	var order []uint64
	for _, df := range facts {
		known := byAgent[df.Agent] != nil
		if !known && len(order) >= derivedSummaryCap {
			continue // enough distinct agents already; do not re-verify rows for new ones
		}
		if known && len(byAgent[df.Agent]) >= factsPerAgent {
			continue // enough facts named for this agent already
		}
		if !d.derivesStill(df) {
			continue
		}
		if !known {
			order = append(order, df.Agent)
		}
		byAgent[df.Agent] = append(byAgent[df.Agent],
			focusFact{rel: df.Rel, focusRole: "agent", coGloss: df.PatGloss, coID: df.Patient})
	}
	var clauses []string
	for _, ag := range order {
		frags, err := d.fragmentsByIDsTiered([]uint64{ag})
		if err != nil || len(frags) != 1 {
			continue
		}
		name := d.conceptName(ag, frags[0].Content, lang)
		if line, ok := d.renderFactsCapped(lang, name, byAgent[ag], true, false, factsPerAgent); ok {
			clauses = append(clauses, lowerFirst(strings.TrimRight(line, ".")))
		}
	}
	if len(clauses) == 0 {
		return ""
	}
	return "I have worked out that " + grammar.JoinAnd(clauses)
}

// reasonedSureLine answers "how sure are you" for a sound deduction: certain, and reasoned. A composed
// deduction leaves no moment of its own, so the evidence tally reads empty and would otherwise admit
// "it's a guess"; but a sound conclusion is certain, and owning that it was reasoned keeps it honest
// and distinct from a seen fact. Reuses the seeded certainty adverb and the reasoning surface, so it
// speaks in any tongue.
func (d *Daemon) reasonedSureLine(lang string) string {
	reason := justifySurface(d, lang, "justifyreason")
	certain := d.gram.Word(lang, "certain")
	if certain == "" {
		return reason
	}
	if reason == "" {
		return upperFirst(certain) + "."
	}
	// justifySurface already ends the reasoning surface with a period, so only the certainty clause
	// needs one added.
	return upperFirst(certain) + ". " + upperFirst(reason)
}
