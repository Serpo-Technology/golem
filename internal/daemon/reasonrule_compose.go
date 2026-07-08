package daemon

import (
	"time"

	"golem/internal/memory"
)

// A compositional reasoning rule expresses an inference as a small graph of composable
// primitives the daemon walks and evaluates against the live graph, rather than a rule kind
// with its own Go evaluator. A rule is a RESULT over a REACH: the reach walks from a query
// concept over an edge in a role (single-hop or transitively, optionally keeping only facts
// whose relation carries a marker), and the result reads the reached set (does it hold the
// query target, the whole set, or the first reached concept that stands in a relation to a
// query argument). Every parameter is a graph leaf, so a NEW inference is a stored
// composition of these primitives, no new Go: the three lifted walks (transitive comparison,
// category inheritance, part-whole) are each one composition, and so is an inference no
// handler provides (a patient-direction chain, a single-hop membership), which is the point.
//
// The query binds the rule's free leaves at evaluation: $from the subject, $to the target,
// $rel the queried relation, $patient the object an inheritance test checks. This generalizes
// the three per-kind evaluators; it runs in shadow here (they stay live and authoritative)
// and is verified to reproduce them and to run a novel composition as pure data.

const (
	// The result primitives a rule root grounds in: contains asks whether the reached set holds
	// the query target (a yes/no chain), set returns the whole reached set (a rank, a part
	// list), find returns the first reached concept that stands in the tested relation to the
	// tested argument (an inherited fact).
	resContains = "contains"
	resSet      = "set"
	resFind     = "find"

	// The reach parameters, each a leaf on the reach node: the edge to walk, the role, whether
	// to close transitively, and an optional marker to filter facts by.
	reachEdgeRel   = "redge"
	reachRoleRel   = "rrole"
	reachTransRel  = "rtrans"
	reachFilterRel = "rfilter"

	// resSourceRel links a result node to the reach it reads; findRelRel and findArgRel give a
	// find result the relation to test at each reached concept and the argument to test against.
	resSourceRel = "rsource"
	findRelRel   = "rtestrel"
	findArgRel   = "rtestarg"

	// The query bindings a leaf resolves to at evaluation, and the two special reach edges (the
	// queried relation, and any relation, for a marker-filtered reach that is not over one named
	// relation). A leaf that is none of these is a literal (a relation name, a role, a marker).
	bindFrom    = "$from"
	bindTo      = "$to"
	bindRel     = "$rel"
	bindPatient = "$patient"
	anyEdge     = "$any"

	transYes = "yes"
	transNo  = "no"

	partWholeMarker = "part-whole"
)

// ruleQuery binds a compositional rule's free leaves for one evaluation.
type ruleQuery struct {
	from, to, patient uint64
	rel               string
}

// ruleResult is what evalRule returns, one field meaningful per result kind: hit for a
// contains or find (whether it holds/was found), id for a find (the matching concept), set
// for a set.
type ruleResult struct {
	kind string
	hit  bool
	id   uint64
	set  []uint64
}

// compSpec describes a compositional rule to build: the result primitive, the reach
// parameters, and (for a find) the relation and argument it tests, each an edge or binding
// leaf. It is the surface a rule is authored in before it is written into the graph.
type compSpec struct {
	result           string
	edge, role       string
	trans, filter    string
	testRel, testArg string
}

// ensureCompRule creates (once) a durable compositional rule subgraph keyed by template and
// returns its root: a result node over a reach node with each parameter a leaf, held in a
// code subgraph the nightly prune spares. A find rule also stores the relation and argument
// it tests. It is the compositional counterpart of ensureReasonRule.
func (d *Daemon) ensureCompRule(template string, spec compSpec) uint64 {
	if id := d.reasonRuleByTemplate(template); id != 0 {
		return id
	}
	root := &memory.Fragment{Kind: memory.KindCodeNode, Channel: "grammar", Category: "code",
		Template: template, Weight: taughtConstructionWeight, Time: time.Now()}
	if _, err := d.store.Encode(root); err != nil {
		return 0
	}
	w := storeCodeWriter{d}
	w.link(root.ID, w.add(spec.result), memory.RelGrounds)
	reach := w.add("")
	w.link(reach, w.add(reachRule), memory.RelGrounds)
	w.link(reach, w.add(spec.edge), reachEdgeRel)
	w.link(reach, w.add(spec.role), reachRoleRel)
	w.link(reach, w.add(spec.trans), reachTransRel)
	if spec.filter != "" {
		w.link(reach, w.add(spec.filter), reachFilterRel)
	}
	w.link(root.ID, reach, resSourceRel)
	if spec.result == resFind {
		w.link(root.ID, w.add(spec.testRel), findRelRel)
		w.link(root.ID, w.add(spec.testArg), findArgRel)
	}
	return root.ID
}

// evalRule evaluates a compositional rule against the live graph under a query binding: it
// reads the result kind the root grounds in, walks its reach to a set, and reads that set the
// way the kind says. It is the one generic evaluator the per-kind ones (evalReachRule,
// evalInheritRule, evalPartWholeRule) generalize to, so a rule that composes these primitives
// in a shape no handler provides runs here with no new Go. ok is false when the node is not a
// compositional rule, so a caller falls back.
func (d *Daemon) evalRule(root uint64, q ruleQuery) (ruleResult, bool) {
	g := d.storeView()
	kind := g.content(firstDest(g, root, memory.RelGrounds))
	reach := firstDest(g, root, resSourceRel)
	if reach == 0 {
		return ruleResult{}, false
	}
	reached, ok := d.reachSet(reach, q)
	if !ok {
		return ruleResult{}, false
	}
	switch kind {
	case resContains:
		for _, id := range reached {
			if id == q.to {
				return ruleResult{kind: kind, hit: true}, true
			}
		}
		return ruleResult{kind: kind}, true
	case resSet:
		return ruleResult{kind: kind, set: reached}, true
	case resFind:
		testRel := d.resolveRelBinding(g.content(firstDest(g, root, findRelRel)), q)
		arg := d.resolveArgBinding(g.content(firstDest(g, root, findArgRel)), q)
		for _, cid := range reached {
			cf, err := d.fragmentsByIDsTiered([]uint64{cid})
			if err != nil || len(cf) != 1 {
				continue
			}
			if d.propositionHolds(cf[0], testRel, arg) {
				return ruleResult{kind: kind, hit: true, id: cid}, true
			}
		}
		return ruleResult{kind: kind}, true
	}
	return ruleResult{}, false
}

// reachSet walks a reach node from the query's subject and returns the concepts reached, in
// walk order and deduplicated (the subject itself excluded): the shared step of every
// compositional rule. It reads the edge, role, transitivity, and optional marker filter off
// the node, so which walk it runs is graph data. The traversal mirrors the hardcoded walks
// (a stack, dedup by first sight), so a reach reproduces categoriesOf / comparativeReach /
// partWholes exactly for the parameters those use.
func (d *Daemon) reachSet(reach uint64, q ruleQuery) ([]uint64, bool) {
	g := d.storeView()
	if g.content(firstDest(g, reach, memory.RelGrounds)) != reachRule {
		return nil, false
	}
	edge := g.content(firstDest(g, reach, reachEdgeRel))
	role := g.content(firstDest(g, reach, reachRoleRel))
	if edge == "" || role == "" {
		return nil, false
	}
	trans := g.content(firstDest(g, reach, reachTransRel)) == transYes
	filter := g.content(firstDest(g, reach, reachFilterRel))
	var reached []uint64
	seen := map[uint64]bool{q.from: true}
	stack := []uint64{q.from}
	for len(stack) > 0 {
		cur := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		for _, nb := range d.reachStep(cur, edge, role, filter, q.rel) {
			if seen[nb] {
				continue
			}
			seen[nb] = true
			reached = append(reached, nb)
			if trans {
				stack = append(stack, nb)
			}
		}
	}
	return reached, true
}

// reachStep is one hop of a reach: the concepts cur stands to over the edge in the role. The
// is-a edge climbs the membership hierarchy (the abstractions that ground in cur), the twin
// of categoriesOf's step; any other edge gathers cur's facts (the queried relation for $rel,
// every relation for $any, a named relation otherwise) in the role, keeping only those whose
// relation carries the marker when one is set.
func (d *Daemon) reachStep(cur uint64, edge, role, filter, queryRel string) []uint64 {
	if edge == isAEdge {
		cats, err := d.store.LinksToRel([]uint64{cur}, memory.RelGrounds)
		if err != nil {
			return nil
		}
		out := make([]uint64, 0, len(cats))
		for _, e := range cats {
			out = append(out, e.From)
		}
		return out
	}
	rel := ""
	switch edge {
	case bindRel:
		rel = queryRel
	case anyEdge:
		rel = ""
	default:
		rel = edge // a literal named relation
	}
	frags, err := d.store.FragmentsByIDs([]uint64{cur})
	if err != nil || len(frags) != 1 {
		return nil
	}
	var out []uint64
	for _, f := range d.gatherFacts(frags[0], role, rel) {
		if filter == "" || d.markerHolds(filter, f.rel) {
			out = append(out, f.coID)
		}
	}
	return out
}

// markerHolds reports whether a relation carries a reach filter's marker: today only the
// part-whole marker, the test partWholesGo applies, so a marker-filtered reach reproduces it.
func (d *Daemon) markerHolds(marker, relContent string) bool {
	switch marker {
	case partWholeMarker:
		return d.factIsPartWhole(relContent)
	}
	return false
}

// resolveRelBinding reads a find rule's tested relation: the queried relation when the leaf is
// the $rel binding, else the literal relation the leaf names.
func (d *Daemon) resolveRelBinding(leaf string, q ruleQuery) string {
	if leaf == bindRel {
		return q.rel
	}
	return leaf
}

// resolveArgBinding reads a find rule's tested argument from the query binding the leaf names.
func (d *Daemon) resolveArgBinding(leaf string, q ruleQuery) uint64 {
	switch leaf {
	case bindPatient:
		return q.patient
	case bindTo:
		return q.to
	case bindFrom:
		return q.from
	}
	return 0
}
