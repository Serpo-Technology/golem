package daemon

import (
	"time"

	"golem/internal/memory"
)

// Resemblance-based inference is the one VECTOR-grounded inference: where fact, category, and the
// sound deductions all miss, a concept the agent resembles that stands in the same relation lends
// a hedged guess ("the wolf resembles the dog, and the dog chased the cat, so probably the wolf
// did"). It is the last reasoning walk lifted onto the one interpreter, and the one that puts
// CONFIDENCE into the rule language: a resSupport rule reaches by likeness ($like -> featureSiblings)
// and returns the weighted-best support as a confidence, never a firm answer. The invariant
// machinery it runs on stays in Go (featureSiblings, propositionSupport, the held self-seed and
// recording, the inference floor), the same way gatherFacts stays the floor a graph reach runs on.
// The glass-box boundary holds by construction: a $like reach is inert in every sound result kind
// (the reachSet/reachRelSet guard), and resSupport maps only to a hedged belief in assessProposition,
// so a vector proposal can never be spoken as a firm fact.

// resembleRuleTemplate keys the one resemblance rule: a resSupport over a $like reach in the agent
// role, testing the queried relation ($rel) at the queried patient.
const resembleRuleTemplate = "rule:resemble:support"

// resembleSpec is the resemblance composition: reach the concepts the subject resembles ($like),
// and at each test whether it stands in the queried relation to the queried patient, returning the
// weighted-best support. testRel/testArg bind to the query so one rule serves every relation.
func resembleSpec() compSpec {
	return compSpec{result: resSupport, edge: bindLike, role: agentRole, trans: transNo,
		testRel: bindRel, testArg: bindPatient}
}

// resembleRule returns the cached root of the resemblance rule, ensuring it once on first use.
func (d *Daemon) resembleRule() uint64 {
	return d.compRule(resembleRuleTemplate, resembleSpec())
}

// resembleSupport reads the weighted-best resemblance support for a proposition (the neighbour the
// agent most resembles that stands in the relation, and how strongly), by running the resemblance
// rule against the graph; resembleSupportGo is the fallback when no rule is stored. It is the
// composable part of the resemblance guess: the caller seeds it with a held self-conclusion,
// thresholds it, records it, and speaks it hedged.
func (d *Daemon) resembleSupport(agent *memory.Fragment, relContent string, patient *memory.Fragment) (ruleResult, bool) {
	if root := d.resembleRule(); root != 0 {
		if r, ok := d.evalRule(root, ruleQuery{from: agent.ID, patient: patient.ID, rel: relContent}); ok {
			return r, true
		}
	}
	return d.resembleSupportGo(agent, relContent, patient)
}

// resembleSupportGo is the hardcoded weighted-max over the agent's feature siblings, the fallback
// for the resemblance rule: for each neighbour (bar the patient) it weights the support the graph
// or a held guess gives the proposition by the resemblance strength, and keeps the best. The rule
// reproduces it, verified over the resemblance fixtures. ok is false only when the vector read
// errors, so the caller yields the honest gap without weighing a held self-conclusion.
func (d *Daemon) resembleSupportGo(agent *memory.Fragment, relContent string, patient *memory.Fragment) (ruleResult, bool) {
	sibs, err := d.featureSiblings(agent.ID, time.Now())
	if err != nil {
		return ruleResult{}, false
	}
	best, bestDepth, bestFrom, bestSupposed, bestSib := 0.0, 0, uint64(0), false, uint64(0)
	for _, sib := range sibs {
		if sib.id == patient.ID {
			continue
		}
		conf, depth, from, supposed, ok := d.propositionSupport(sib.id, relContent, patient.ID)
		if !ok {
			continue
		}
		if cand := sib.weight * conf; cand > best {
			best, bestDepth, bestFrom, bestSupposed, bestSib = cand, depth, from, supposed, sib.id
		}
	}
	return ruleResult{kind: resSupport, conf: best, depth: bestDepth, from: bestFrom,
		supposed: bestSupposed, lender: bestSib}, true
}
