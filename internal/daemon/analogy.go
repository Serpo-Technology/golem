package daemon

// Analogy completes a proportion a:b::c:? -- "the cat is to the mouse as the dog is to what" --
// by finding the relation that holds from a to b and applying it to c, the fourth term. It is
// the last major inference lifted out of a hardcoded walk into the compositional rule language:
// a find-then-apply rule whose SOURCE reach is relation-valued (the relations connecting a and
// b) and whose APPLY reach walks c over each, the first that carries through winning. So the
// analogy is stored data run by the one evaluator rather than a bespoke Go walk, and the
// relation-valued reach and $found edge it needs are reusable primitives a future inference can
// recompose (relation identification, relational transfer).

// analogyRuleTemplate keys the one compositional analogy rule: a resApply whose source reach
// discovers the relations $from->$to and whose apply reach walks $third over each.
const analogyRuleTemplate = "rule:analogy:apply"

// analogySpec is the analogy composition: a resApply whose source reach is the relation-valued
// connector (any relation in the agent role, landing on $to, so it yields the relations $from
// stands in to $to) and whose apply reach walks from $third over the discovered $found relation.
func analogySpec() compSpec {
	return compSpec{
		result: resApply, edge: anyEdge, role: agentRole, trans: transNo, land: bindTo,
		apply: &reachSpec{edge: bindFound, role: agentRole, trans: transNo, start: bindThird},
	}
}

// analogyRule returns the cached root of the compositional analogy rule, ensuring it once on
// first use like the other reasoning rules.
func (d *Daemon) analogyRule() uint64 {
	return d.compRule(analogyRuleTemplate, analogySpec())
}
