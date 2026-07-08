package daemon

import (
	"time"

	"golem/internal/memory"
)

// A reasoning rule is an inference lifted out of hardcoded Go into a durable, walkable graph
// object the daemon runs against the live graph, the reasoning twin of a learned function's
// code subgraph. It reuses the same KindCodeNode storage and the same walk as the code
// faculty, but its leaves are graph queries (reach the concepts a concept stands to in a
// role over a relation) rather than arithmetic, and it is evaluated against the graph rather
// than compiled to Go, since the sandbox that runs code cannot see the store.
//
// Transitivity is the first rule lifted. Told "before is transitive", GOLEM already records
// the property as graph data (a RelTransitive marker, transitivity.go); the chaining walk
// that property fires (comparesTransitively) stays the authoritative handler, and this makes
// a graph-stored twin of it, proven to reproduce the handler before any switch-over. What
// becomes learnable is the walk's shape: the role it follows is read from the stored rule
// rather than hardcoded, so inheritance and part-whole reach follow as sibling rules that
// differ only in their stored role, not in new Go.

const (
	// reachRule grounds a rule node in the graph-query primitive that reaches the concepts a
	// concept stands to in a role over a relation, the reasoning counterpart of an arithmetic
	// operation. Its one operand is the role the reach follows.
	reachRule = "reach"

	// agentRole is the role a transitive ordering follows: the canonical greater-over-lesser
	// (or before-over-after) orderings write the standing concept as the agent, so the reach
	// walks from a concept to the concepts it stands over.
	agentRole = "agent"

	// transitiveRuleTemplate keys the one generic transitive-closure rule, so it is found and
	// reused for every relation told transitive rather than stored once per relation.
	transitiveRuleTemplate = "rule:transitive"
)

// ensureTransitivityRule returns the root of the durable transitive-closure rule subgraph,
// creating it once if absent: a reach node following the agent role, held in a code subgraph
// the nightly prune spares like any other. The rule is generic (the relation it chains is
// bound at evaluation time from the query), so one object serves every transitive relation.
func (d *Daemon) ensureTransitivityRule() uint64 {
	if id := d.reasonRuleByTemplate(transitiveRuleTemplate); id != 0 {
		return id
	}
	root := &memory.Fragment{Kind: memory.KindCodeNode, Channel: "grammar", Category: "code",
		Template: transitiveRuleTemplate, Weight: taughtConstructionWeight, Time: time.Now()}
	if _, err := d.store.Encode(root); err != nil {
		return 0
	}
	w := storeCodeWriter{d}
	w.link(root.ID, w.add(reachRule), memory.RelGrounds)
	w.link(root.ID, w.add(agentRole), operand0Rel)
	return root.ID
}

// reasonRuleByTemplate finds a reasoning-rule subgraph root by its template, 0 when none is
// stored yet. A rule root is a code node, never pruned and never in the concept or word
// space, so it is reached by template rather than the concept lookup.
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

// evalReachRule evaluates a reach rule against the live graph, the reasoning counterpart of
// genCode running a code subgraph: it reads the rule's shape (the reach primitive and the
// role it follows) from the graph, then chains that role-directed relation transitively from
// one concept to another, returning whether the target is reached. It reproduces
// comparesTransitively, but with the role read from the stored rule rather than hardcoded, so
// the walk itself is graph-stored data. ok is false when the node is not a reach rule, so a
// caller falls back to the Go handler.
func (d *Daemon) evalReachRule(ruleRoot uint64, relContent string, fromID, toID uint64) (holds, ok bool) {
	g := d.storeView()
	if g.content(firstDest(g, ruleRoot, memory.RelGrounds)) != reachRule {
		return false, false
	}
	role := g.content(firstDest(g, ruleRoot, operand0Rel))
	if role == "" {
		return false, false
	}
	seen := map[uint64]bool{fromID: true}
	stack := []uint64{fromID}
	for len(stack) > 0 {
		cur := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		frags, err := d.store.FragmentsByIDs([]uint64{cur})
		if err != nil || len(frags) != 1 {
			continue
		}
		for _, f := range d.gatherFacts(frags[0], role, relContent) {
			if f.coID == toID {
				return true, true
			}
			if !seen[f.coID] {
				seen[f.coID] = true
				stack = append(stack, f.coID)
			}
		}
	}
	return false, true
}
