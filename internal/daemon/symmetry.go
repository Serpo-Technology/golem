package daemon

import "golem/internal/memory"

// Symmetry is a relation property GOLEM is told and then reasons with, the sibling of
// transitivity: told "meet is symmetric", the cat meeting the dog entails the dog meeting the
// cat. The property is recorded as a self-edge on the relation concept (markSymmetric), and
// the inference it fires is a graph-stored compositional rule run against the graph rather
// than a hardcoded walk: a check of the reversed fact, expressed as a reach from the query's
// target that looks for its subject. So a relation's symmetry is taught in language and
// answered by running stored data, the teach-a-rule loop closed for an inference the code did
// not have built in. It is new, so it fires only for a relation explicitly told symmetric and
// regresses nothing.

// symmetricRuleTemplate keys the one generic compositional symmetric rule, bound to the
// queried relation at evaluation, so one object serves every symmetric relation.
const symmetricRuleTemplate = "rule:comp:symmetric"

// markSymmetric records that a relation is symmetric: a self-edge on the relation concept,
// matched to the transitive marker's weight so the learned property persists across naps.
func (d *Daemon) markSymmetric(relID uint64) {
	d.store.SetLinkRel(relID, relID, memory.RelSymmetric, transitiveWeight)
}

// relationIsSymmetric reports whether a relation has been told it is symmetric.
func (d *Daemon) relationIsSymmetric(relID uint64) bool {
	edges, err := d.store.LinksFromRel([]uint64{relID}, memory.RelSymmetric)
	return err == nil && len(edges) > 0
}

// symmetricRule returns the cached root of the compositional symmetric rule, ensuring it once
// on first use: contains(reach(edge=$rel, role=agent, single-hop, start=$to), target=$from),
// which reads as "does the reversed fact hold" -- reach from the query's target over the
// queried relation and see whether it lands on the subject.
func (d *Daemon) symmetricRule() uint64 {
	if d.symmetricRuleID == 0 {
		d.symmetricRuleID = d.ensureCompRule(symmetricRuleTemplate, compSpec{
			result: resContains, edge: bindRel, role: agentRole, trans: transNo,
			start: bindTo, ctarget: bindFrom,
		})
	}
	return d.symmetricRuleID
}

// symmetricHolds reports whether the agent stands in the relation to the patient by symmetry:
// the relation was told symmetric and the reversed fact (patient to agent) holds, read by
// running the compositional symmetric rule against the graph. It fires only for a relation
// explicitly told symmetric, so an ordinary relation never symmetrizes and nothing regresses.
func (d *Daemon) symmetricHolds(agentID uint64, relContent string, patientID uint64) bool {
	rel := d.resolveRelation(relContent)
	if rel == nil || !d.relationIsSymmetric(rel.ID) {
		return false
	}
	root := d.symmetricRule()
	if root == 0 {
		return false
	}
	r, ok := d.evalRule(root, ruleQuery{from: agentID, to: patientID, rel: relContent})
	return ok && r.hit
}
