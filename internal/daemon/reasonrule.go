package daemon

import "golem/internal/memory"

// A reasoning rule is an inference lifted out of hardcoded Go into a durable, walkable graph
// object the daemon runs against the live graph, the reasoning twin of a learned function's
// code subgraph. It reuses the KindCodeNode storage and the walk of the code faculty, but its
// leaves are graph queries (reach the concepts a concept stands to in a role over a relation)
// rather than arithmetic, and it is interpreted against the graph rather than compiled to Go,
// since the sandbox that runs code cannot see the store.
//
// Every rule is a composition of the same primitives (a result over a reach, see
// reasonrule_compose.go) run by the one generic evalRule; a rule kind is a compSpec, not a
// bespoke evaluator. The four built-in inferences -- transitive comparison, category
// inheritance, part-whole, and the taught symmetry -- are each one such composition, built by
// compRule and cached per template, and their handlers run them with the Go walk kept as the
// fallback for a shape the rule does not reconstruct.

const (
	// reachRule grounds a reach node in the graph-query primitive that reaches the concepts a
	// concept stands to in a role over a relation, the reasoning counterpart of an arithmetic
	// operation.
	reachRule = "reach"

	// agentRole is the role the built-in inferences reach in: the canonical orderings and the
	// is-a and part-whole facts all write the standing concept as the agent. patientRole is its
	// counterpart, the role a nested query's outer hop reaches (the agents standing to a concept).
	agentRole   = "agent"
	patientRole = "patient"

	// isAEdge is the membership edge an inheritance reach climbs, so a member reaches the
	// categories above it.
	isAEdge = "is-a"

	// The templates the built-in rules are stored and found under, one per (reach, result) an
	// inference needs: transitive comparison as a yes/no chain and as a reached set, inheritance
	// as a find over is-a, part-whole as a marker-filtered set.
	transContainsRuleTemplate = "rule:transitive:contains"
	transSetRuleTemplate      = "rule:transitive:set"
	inheritRuleTemplate       = "rule:inherit:find"
	partWholeRuleTemplate     = "rule:partwhole:set"
)

// The compositional specs of the built-in inferences: transitive comparison reaches the query
// relation in the agent role and closes it (yes/no or the whole set); inheritance reaches the
// is-a hierarchy and tests the query relation at each category against the patient; part-whole
// is a single-hop, marker-filtered reach over any relation.
func transContainsSpec() compSpec {
	return compSpec{result: resContains, edge: bindRel, role: agentRole, trans: transYes}
}

func transSetSpec() compSpec {
	return compSpec{result: resSet, edge: bindRel, role: agentRole, trans: transYes}
}

func inheritSpec() compSpec {
	return compSpec{result: resFind, edge: isAEdge, role: agentRole, trans: transYes,
		testRel: bindRel, testArg: bindPatient}
}

func partWholeSpec() compSpec {
	return compSpec{result: resSet, edge: anyEdge, role: agentRole, trans: transNo,
		filter: partWholeMarker}
}

// reasonRuleByTemplate finds a reasoning-rule subgraph root by its template, 0 when none is
// stored yet. A rule root is a code node, never pruned and never in the concept or word space,
// so it is reached by template rather than the concept lookup.
func (d *Daemon) reasonRuleByTemplate(tmpl string) uint64 {
	nodes, err := d.store.NodesByKind(memory.KindCodeNode)
	if err != nil {
		return 0
	}
	for _, n := range nodes {
		if n.Template == tmpl {
			return n.ID
		}
	}
	return 0
}

// compRule returns the root of the compositional rule for a template, ensuring it once on
// first use and caching it, so a reasoning handler runs the graph-stored rule without a scan
// per query. Called only on the single request loop, so the cache write is unsynchronized by
// design; the cache is lazily made on first use.
func (d *Daemon) compRule(template string, spec compSpec) uint64 {
	if d.reasonRuleID == nil {
		d.reasonRuleID = map[string]uint64{}
	}
	if id := d.reasonRuleID[template]; id != 0 {
		return id
	}
	id := d.ensureCompRule(template, spec)
	d.reasonRuleID[template] = id
	return id
}
