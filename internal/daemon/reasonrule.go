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

	// inheritRule grounds a rule node in the inheritance primitive: reach the categories a
	// concept belongs to and test the queried relation at each, the composed two-relation
	// inference (reach is-a, then check the relation) that transitivity's single reach is not.
	inheritRule = "inherit"

	// isAEdge is the edge an inheritance rule reaches over: the is-a membership hierarchy, so
	// a member reaches the categories above it. Stored on the rule as the reach edge, the
	// counterpart of the role a reach rule stores, so the walk it follows is graph data.
	isAEdge = "is-a"

	// transitiveRuleTemplate and inheritRuleTemplate key the generic rules, so each is found
	// and reused rather than stored per relation.
	transitiveRuleTemplate = "rule:transitive"
	inheritRuleTemplate    = "rule:inherit"
)

// ensureReasonRule returns the root of a durable reasoning-rule subgraph keyed by template,
// creating it once if absent: a node grounding in the rule's primitive with its one operand
// (a reach rule's role, an inherit rule's reach edge), held in a code subgraph the nightly
// prune spares like any other. A rule is generic (the relation it works over is bound at
// evaluation time from the query), so one object serves every relation of its kind.
func (d *Daemon) ensureReasonRule(template, primitive, operand string) uint64 {
	if id := d.reasonRuleByTemplate(template); id != 0 {
		return id
	}
	root := &memory.Fragment{Kind: memory.KindCodeNode, Channel: "grammar", Category: "code",
		Template: template, Weight: taughtConstructionWeight, Time: time.Now()}
	if _, err := d.store.Encode(root); err != nil {
		return 0
	}
	w := storeCodeWriter{d}
	w.link(root.ID, w.add(primitive), memory.RelGrounds)
	w.link(root.ID, w.add(operand), operand0Rel)
	return root.ID
}

func (d *Daemon) ensureTransitivityRule() uint64 {
	return d.ensureReasonRule(transitiveRuleTemplate, reachRule, agentRole)
}

func (d *Daemon) ensureInheritanceRule() uint64 {
	return d.ensureReasonRule(inheritRuleTemplate, inheritRule, isAEdge)
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

// transitivityRule returns the cached root of the transitive-closure rule, ensuring it once
// on first use so the comparison path runs the graph-stored rule without a per-query scan. It
// is called only on the single request loop, so the cache write is unsynchronized by design.
func (d *Daemon) transitivityRule() uint64 {
	if d.transitivityRuleID == 0 {
		d.transitivityRuleID = d.ensureTransitivityRule()
	}
	return d.transitivityRuleID
}

// inheritanceRule returns the cached root of the inheritance rule, ensuring it once on first
// use so the inheritance path runs the graph-stored rule without a per-query scan. Called
// only on the single request loop, so the cache write is unsynchronized by design.
func (d *Daemon) inheritanceRule() uint64 {
	if d.inheritanceRuleID == 0 {
		d.inheritanceRuleID = d.ensureInheritanceRule()
	}
	return d.inheritanceRuleID
}

// evalInheritRule evaluates an inheritance rule against the live graph: it reaches the
// categories the agent belongs to (over the is-a edge the rule stores) and tests the queried
// relation at each, returning the first category that holds it, defeasibly. It reproduces
// categoryInheritsGo, but driven by the stored rule (its kind and reach edge read from the
// graph) rather than hardcoded, so a second inference kind runs on the one rule interpreter.
// ok is false when the node is not an inheritance rule over is-a, so a caller falls back to
// the Go handler. The deny-filtering that makes inheritance defeasible rides in the reused
// propositionHolds (through gatherFacts), so the exception discipline is preserved unchanged.
func (d *Daemon) evalInheritRule(ruleRoot uint64, relContent string, agentID, patientID uint64) (catID uint64, found, ok bool) {
	g := d.storeView()
	if g.content(firstDest(g, ruleRoot, memory.RelGrounds)) != inheritRule {
		return 0, false, false
	}
	if g.content(firstDest(g, ruleRoot, operand0Rel)) != isAEdge {
		return 0, false, false // a reach edge the evaluator does not yet interpret
	}
	for _, cid := range d.categoriesOf(agentID) {
		cf, err := d.fragmentsByIDsTiered([]uint64{cid})
		if err != nil || len(cf) != 1 {
			continue
		}
		if d.propositionHolds(cf[0], relContent, patientID) {
			return cid, true, true
		}
	}
	return 0, false, true
}

// ruleReachRole reads the role a reach rule follows from its subgraph, false when the node is
// not a reach rule (it grounds in something else), so a caller falls back to the Go handler.
func (d *Daemon) ruleReachRole(g codeGraph, ruleRoot uint64) (string, bool) {
	if g.content(firstDest(g, ruleRoot, memory.RelGrounds)) != reachRule {
		return "", false
	}
	role := g.content(firstDest(g, ruleRoot, operand0Rel))
	return role, role != ""
}

// evalReachRule evaluates a reach rule against the live graph, the reasoning counterpart of
// genCode running a code subgraph: it reads the role the rule follows from the graph, then
// chains that role-directed relation transitively from one concept to another, returning
// whether the target is reached. It reproduces comparesTransitivelyGo, but with the role read
// from the stored rule rather than hardcoded, so the walk itself is graph-stored data. ok is
// false when the node is not a reach rule, so a caller falls back to the Go handler.
func (d *Daemon) evalReachRule(ruleRoot uint64, relContent string, fromID, toID uint64) (holds, ok bool) {
	g := d.storeView()
	role, ok := d.ruleReachRole(g, ruleRoot)
	if !ok {
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

// evalReachSet evaluates a reach rule for the whole set a concept reaches, the collecting
// counterpart of evalReachRule: it walks the same role-directed relation transitively and
// returns every concept reached (a concept's place on a scale, how many it stands over),
// reproducing comparativeReachGo with the role read from the rule. ok is false when the node
// is not a reach rule, so a caller falls back to the Go handler.
func (d *Daemon) evalReachSet(ruleRoot uint64, relContent string, fromID uint64) (map[uint64]bool, bool) {
	g := d.storeView()
	role, ok := d.ruleReachRole(g, ruleRoot)
	if !ok {
		return nil, false
	}
	below := map[uint64]bool{}
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
			below[f.coID] = true
			if !seen[f.coID] {
				seen[f.coID] = true
				stack = append(stack, f.coID)
			}
		}
	}
	delete(below, fromID)
	return below, true
}
