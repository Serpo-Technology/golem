package daemon

import "golem/internal/memory"

// A multi-hop nested query ("who chased what the dog chased") composes two relational hops: the
// inner clause resolves to the things a subject stands to over one relation, and the outer asks
// which concepts stand to each of those over another. It is the last hardcoded reasoning walk
// lifted into the compositional rule language, and the shape that needed a NESTED reach: a
// resChain whose source reach yields the inner concepts and whose apply reach walks from each of
// them (bound as $chained) over a second relation ($rel2), the results unioned. The parse of the
// nested clause (which the grammar cannot handle standalone) stays the floor in planMultiHop; the
// two-hop composition is the rule.

// multiHopRuleTemplate keys the one nested-query rule: a resChain over the inner relation ($rel)
// in the agent role, applying the outer relation ($rel2) in the patient role from each reached
// concept.
const multiHopRuleTemplate = "rule:multihop:chain"

// multiHopSpec is the nested-query composition: the source reach walks the inner subject ($from)
// over the inner relation ($rel) in the agent role to its patients, and the apply reach walks
// each of those ($chained) over the outer relation ($rel2) in the patient role to the agents
// standing to it. Both reaches keep their start, so a reflexive fact (a concept standing in the
// relation to itself) carries through exactly as the hardcoded walk counts it.
func multiHopSpec() compSpec {
	return compSpec{
		result: resChain, edge: bindRel, role: agentRole, trans: transNo, keep: transYes,
		apply: &reachSpec{edge: bindRel2, role: patientRole, trans: transNo, start: bindChained, keep: transYes},
	}
}

// multiHopRule returns the cached root of the nested-query rule, ensuring it once on first use.
func (d *Daemon) multiHopRule() uint64 {
	return d.compRule(multiHopRuleTemplate, multiHopSpec())
}

// multiHopReach answers a nested query by running the rule against the graph: the agents that
// stand to what the inner subject reached, plus the first inner concept (the focus a following
// pronoun resolves to). It falls back to the Go walk when no rule is stored. The returned facts
// carry each agent's gloss so the caller renders unnamed concepts the way the walk does.
func (d *Daemon) multiHopReach(innerSubj uint64, innerRel, outerRel string) ([]focusFact, uint64, bool) {
	root := d.multiHopRule()
	if root == 0 {
		return d.multiHopReachGo(innerSubj, innerRel, outerRel)
	}
	r, ok := d.evalRule(root, ruleQuery{from: innerSubj, rel: innerRel, rel2: outerRel})
	if !ok {
		return d.multiHopReachGo(innerSubj, innerRel, outerRel)
	}
	return d.factsFromIDs(r.set), d.firstInnerMid(innerSubj, innerRel), true
}

// multiHopReachGo is the hardcoded two-hop walk, the fallback for the nested-query rule: hop 1
// gathers the inner subject's patients over the inner relation, hop 2 the agents standing to each
// over the outer relation, unioned in walk order. The rule reproduces it, verified over the
// nested-query fixtures.
func (d *Daemon) multiHopReachGo(innerSubj uint64, innerRel, outerRel string) ([]focusFact, uint64, bool) {
	frags, err := d.store.FragmentsByIDs([]uint64{innerSubj})
	if err != nil || len(frags) != 1 {
		return nil, 0, false
	}
	seen := map[uint64]bool{}
	var facts []focusFact
	var focus uint64
	for _, mid := range d.gatherFacts(frags[0], agentRole, innerRel) { // hop 1: the subject's patients
		mf, err := d.fragmentsByIDsTiered([]uint64{mid.coID})
		if err != nil || len(mf) != 1 {
			continue
		}
		if focus == 0 {
			focus = mf[0].ID
		}
		for _, ag := range d.gatherFacts(mf[0], patientRole, outerRel) { // hop 2: who stood to each
			if !seen[ag.coID] {
				seen[ag.coID] = true
				facts = append(facts, ag)
			}
		}
	}
	return facts, focus, true
}

// firstInnerMid is the first concept the inner hop reaches (the thing the subject did), the focus
// a nested query leaves for a following pronoun. It reproduces the Go walk's focus, which is the
// first patient of the inner subject over the inner relation that reads back as a fragment.
func (d *Daemon) firstInnerMid(innerSubj uint64, innerRel string) uint64 {
	frags, err := d.store.FragmentsByIDs([]uint64{innerSubj})
	if err != nil || len(frags) != 1 {
		return 0
	}
	for _, mid := range d.gatherFacts(frags[0], agentRole, innerRel) {
		mf, err := d.fragmentsByIDsTiered([]uint64{mid.coID})
		if err == nil && len(mf) == 1 {
			return mf[0].ID
		}
	}
	return 0
}

// factsFromIDs rebuilds the focus facts a rule's reached set renders as: each concept's id and
// its gloss, its content verbatim as the walk's role-crossing co-argument gloss is (unclipped),
// so a name borrow reads the same fallback the Go walk gave it. The rule returns bare ids to
// stay concept-typed; this re-attaches the gloss the renderer wants.
func (d *Daemon) factsFromIDs(ids []uint64) []focusFact {
	frags, err := d.fragmentsByIDsTiered(ids)
	if err != nil {
		return nil
	}
	byID := make(map[uint64]*memory.Fragment, len(frags))
	for _, f := range frags {
		byID[f.ID] = f
	}
	out := make([]focusFact, 0, len(ids))
	for _, id := range ids {
		f, ok := byID[id]
		if !ok {
			continue
		}
		out = append(out, focusFact{coID: id, coGloss: f.Content})
	}
	return out
}
