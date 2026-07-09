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
	// resApply is a find-then-apply selection: its source reach is relation-valued (the
	// relations connecting two query args), and for each such relation, in order, it walks an
	// apply reach from a third concept over that relation, returning the first apply that reaches
	// a non-empty set. It is the analogy fourth term (find the relation a->b, apply it from c).
	resApply = "apply"
	// resChain is a nested reach (a flat-map): its source reach yields concepts, and for EACH,
	// an apply reach walks from that concept (bound as $chained), the results unioned. It is the
	// multi-hop nested query ("who chased what the dog chased": the things the dog chased, then
	// the agents that stand to each). Unlike resApply the source yields concepts not relations,
	// each feeds the apply's START not its edge, and the fold is a union not a first-non-empty.
	resChain = "chain"
	// resSupport is the one VECTOR-grounded result kind: over a weight-bearing $like reach (the
	// concepts the subject resembles), it tests the queried relation at each and returns the
	// weighted-best support as a CONFIDENCE, plus the neighbour it rode on (the lender). It is the
	// resemblance guess -- always hedged, never firm. The guardrail keeps a $like reach out of the
	// sound (boolean/set) kinds, so a vector proposal can never be spoken as a firm fact.
	resSupport = "support"

	// The reach parameters, each a leaf on the reach node: the edge to walk, the role, whether
	// to close transitively, and an optional marker to filter facts by.
	reachEdgeRel   = "redge"
	reachRoleRel   = "rrole"
	reachTransRel  = "rtrans"
	reachFilterRel = "rfilter"
	// reachLandRel binds the concept a hop must land on, which makes a reach RELATION-VALUED: it
	// then yields the relation of each landing hop (reachRelSet) rather than the co-argument. It
	// is the connector of a find-then-apply rule (the relations by which one concept reaches another).
	reachLandRel = "rland"
	// reachKeepStartRel keeps the reach's start concept eligible as a landing of its own reach (a
	// reflexive hop back to the start), rather than excluding it. The closure reaches exclude the
	// start (a rank or transitive set is of OTHER concepts); a single-hop apply reach that must
	// count a fourth term equal to its start (c stands in the relation to c itself) sets it.
	reachKeepStartRel = "rkeep"

	// resSourceRel links a result node to the reach it reads; findRelRel and findArgRel give a
	// find result the relation to test at each reached concept and the argument to test against.
	resSourceRel = "rsource"
	findRelRel   = "rtestrel"
	findArgRel   = "rtestarg"
	// resApplyRel links a resApply root to its APPLY reach (the second reach, whose edge is the
	// relation the source reach discovered); the source (relation-valued connector) uses resSourceRel.
	resApplyRel = "rapply"

	// reachStartRel binds which query concept a reach walks from (the subject by default); it
	// lets a rule reach from the target instead, which is how a symmetric rule checks the
	// reversed fact. containsTargetRel binds which concept a contains looks for (the target by
	// default), the counterpart on the result side.
	reachStartRel     = "rstart"
	containsTargetRel = "ctarget"

	// The query bindings a leaf resolves to at evaluation, and the two special reach edges (the
	// queried relation, and any relation, for a marker-filtered reach that is not over one named
	// relation). A leaf that is none of these is a literal (a relation name, a role, a marker).
	bindFrom    = "$from"
	bindTo      = "$to"
	bindRel     = "$rel"
	bindPatient = "$patient"
	bindThird   = "$third"   // the third query concept, for a three-place inference (analogy's c)
	bindFound   = "$found"   // a reach edge bound to the relation a connector reach discovered
	bindChained = "$chained" // a reach start bound to each concept a source reach yields (multi-hop)
	bindRel2    = "$rel2"    // a second queried relation, the outer relation of a nested query
	anyEdge     = "$any"
	// bindLike is the VECTOR-grounded reach edge: it reaches by resemblance (featureSiblings), the
	// weight-bearing neighbours the resemblance guess proposes. It is walkable only by reachLikeSet
	// and read only by resSupport, and is inert in the sound reaches (the guardrail).
	bindLike = "$like"

	transYes = "yes"
	transNo  = "no"

	partWholeMarker = "part-whole"
)

// ruleQuery binds a compositional rule's free leaves for one evaluation.
type ruleQuery struct {
	from, to, patient uint64
	third             uint64 // the third input concept, for a three-place inference (analogy)
	chained           uint64 // the concept a nested reach is currently applying from (multi-hop)
	rel               string
	rel2              string // a second queried relation, the outer relation of a nested query
	foundRel          string // the relation a connector reach discovered, bound to a $found edge
}

// sliceToSet turns a reach's ordered result into the membership set some handlers return (a
// rank reads its size, an ordering its members), the order not being meaningful to them.
func sliceToSet(ids []uint64) map[uint64]bool {
	m := make(map[uint64]bool, len(ids))
	for _, id := range ids {
		m[id] = true
	}
	return m
}

// ruleResult is what evalRule returns, one field meaningful per result kind: hit for a
// contains or find (whether it holds/was found), id for a find (the matching concept), set
// for a set.
type ruleResult struct {
	kind     string
	hit      bool
	id       uint64
	set      []uint64
	foundRel string // for a resApply: the relation that carried through (the proportion's relation)
	// The confidence-carrying fields a resSupport (resemblance) rule returns, meaningful only for
	// it: a weighted-best confidence, the depth and source of the held chain it rode, whether it
	// rests on a supposition, and the lender (the neighbour the guess rode on, for the trace).
	conf     float64
	depth    int
	from     uint64
	supposed bool
	lender   uint64
}

// reachSpec authors one reach node. A compSpec's own edge/role/trans/filter/start/land describe
// a rule's SOURCE reach; apply carries an optional second reach for a find-then-apply rule.
type reachSpec struct {
	edge, role    string
	trans, filter string
	start, land   string
	keep          string // transYes to keep the start concept eligible as a reflexive landing
}

// compSpec describes a compositional rule to build: the result primitive, the (source) reach
// parameters, and (for a find) the relation and argument it tests, each an edge or binding
// leaf. It is the surface a rule is authored in before it is written into the graph.
type compSpec struct {
	result           string
	edge, role       string
	trans, filter    string
	testRel, testArg string
	start, ctarget   string     // reach start binding and contains target binding, empty for the defaults ($from, $to)
	land             string     // source-reach landing binding: makes it relation-valued (a connector)
	keep             string     // transYes to keep the source reach's start eligible as a reflexive landing
	apply            *reachSpec // a second reach for a resApply / resChain rule, nil otherwise
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
	w.link(root.ID, writeReach(w, reachSpec{edge: spec.edge, role: spec.role,
		trans: spec.trans, filter: spec.filter, start: spec.start, land: spec.land, keep: spec.keep}), resSourceRel)
	if spec.apply != nil {
		w.link(root.ID, writeReach(w, *spec.apply), resApplyRel)
	}
	if spec.ctarget != "" {
		w.link(root.ID, w.add(spec.ctarget), containsTargetRel)
	}
	if spec.result == resFind || spec.result == resSupport {
		w.link(root.ID, w.add(spec.testRel), findRelRel)
		w.link(root.ID, w.add(spec.testArg), findArgRel)
	}
	return root.ID
}

// writeReach writes one reach node and its parameter leaves, returning its id. Every rule's
// reach (a source reach, or a resApply's apply reach) is written through it, so a reach's shape
// lives in one place. filter/start/land are optional (an empty leaf is omitted, read as absent).
func writeReach(w storeCodeWriter, rs reachSpec) uint64 {
	reach := w.add("")
	w.link(reach, w.add(reachRule), memory.RelGrounds)
	w.link(reach, w.add(rs.edge), reachEdgeRel)
	w.link(reach, w.add(rs.role), reachRoleRel)
	w.link(reach, w.add(rs.trans), reachTransRel)
	if rs.filter != "" {
		w.link(reach, w.add(rs.filter), reachFilterRel)
	}
	if rs.start != "" {
		w.link(reach, w.add(rs.start), reachStartRel)
	}
	if rs.land != "" {
		w.link(reach, w.add(rs.land), reachLandRel)
	}
	if rs.keep != "" {
		w.link(reach, w.add(rs.keep), reachKeepStartRel)
	}
	return reach
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
	if kind == resApply {
		return d.evalApply(g, root, reach, q)
	}
	if kind == resSupport {
		return d.evalSupport(g, root, reach, q)
	}
	reached, ok := d.reachSet(reach, q)
	if !ok {
		return ruleResult{}, false
	}
	switch kind {
	case resContains:
		target := q.to
		if b := g.content(firstDest(g, root, containsTargetRel)); b != "" {
			target = d.resolveArgBinding(b, q)
		}
		for _, id := range reached {
			if id == target {
				return ruleResult{kind: kind, hit: true}, true
			}
		}
		return ruleResult{kind: kind}, true
	case resSet:
		return ruleResult{kind: kind, set: reached}, true
	case resChain:
		// A nested reach: the source set is the first hop's reached concepts, and the apply reach
		// walks from each (bound as $chained), the results unioned in walk order and deduped. This
		// is the multi-hop query's two composed hops, the outer reach run once per inner concept.
		apply := firstDest(g, root, resApplyRel)
		if apply == 0 {
			return ruleResult{}, false
		}
		seen := map[uint64]bool{}
		var out []uint64
		for _, mid := range reached {
			qq := q
			qq.chained = mid
			sub, ok := d.reachSet(apply, qq)
			if !ok {
				return ruleResult{}, false
			}
			for _, id := range sub {
				if !seen[id] {
					seen[id] = true
					out = append(out, id)
				}
			}
		}
		return ruleResult{kind: kind, set: out}, true
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

// evalApply runs a find-then-apply (resApply) rule: the source reach is RELATION-VALUED (a
// connector), so it yields the relations connecting the query's endpoints; for each, in order,
// the apply reach walks from the third concept over that relation, and the first apply that
// reaches a non-empty set wins. This is the analogy walk (find the relation a->b, apply it from
// c, return the fourth terms) expressed as a composition, the multi-candidate loop living here:
// it tries every connecting relation and returns the first that carries through, admitting the
// gap honestly (hit false) when none does.
func (d *Daemon) evalApply(g codeGraph, root, connector uint64, q ruleQuery) (ruleResult, bool) {
	apply := firstDest(g, root, resApplyRel)
	if apply == 0 {
		return ruleResult{}, false
	}
	cands, ok := d.reachRelSet(connector, q)
	if !ok {
		return ruleResult{}, false
	}
	for _, rel := range cands {
		qq := q
		qq.foundRel = rel
		set, ok := d.reachSet(apply, qq)
		if !ok {
			return ruleResult{}, false
		}
		if len(set) > 0 {
			return ruleResult{kind: resApply, hit: true, foundRel: rel, set: set}, true
		}
	}
	return ruleResult{kind: resApply}, true
}

// reachSet walks a reach node from the query's subject and returns the concepts reached, in
// walk order and deduplicated (the subject excluded unless the reach keeps it): the shared step
// of every compositional rule. It reads the edge, role, transitivity, optional marker filter,
// and the keep-start flag off the node, so which walk it runs is graph data. The traversal
// mirrors the hardcoded walks (a stack, dedup by first sight), so a reach reproduces categoriesOf
// / comparativeReach / partWholes exactly for the parameters those use.
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
	// The guardrail: a $like (resemblance) reach is vector-grounded and weight-bearing, so it is
	// never walked as a sound concept reach. Returning false (not an empty set) makes a firm kind
	// over $like fall back rather than answer a firm no, so a vector proposal cannot be spoken firm.
	if edge == bindLike {
		return nil, false
	}
	trans := g.content(firstDest(g, reach, reachTransRel)) == transYes
	filter := g.content(firstDest(g, reach, reachFilterRel))
	keepStart := g.content(firstDest(g, reach, reachKeepStartRel)) == transYes
	start := q.from
	if b := g.content(firstDest(g, reach, reachStartRel)); b != "" {
		start = d.resolveArgBinding(b, q)
	}
	var reached []uint64
	// seen guards against re-walking a concept; the start is pre-seen so a closure reach is of
	// OTHER concepts, unless the reach keeps its start (then a reflexive hop back to it counts).
	seen := map[uint64]bool{}
	if !keepStart {
		seen[start] = true
	}
	stack := []uint64{start}
	for len(stack) > 0 {
		cur := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		for _, nb := range d.reachStep(cur, edge, role, filter, q) {
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
// of categoriesOf's step; any other edge gathers cur's facts over the relation edgeToRel reads
// from the edge leaf and query, in the role, keeping only those whose relation carries the
// marker when one is set.
func (d *Daemon) reachStep(cur uint64, edge, role, filter string, q ruleQuery) []uint64 {
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
	rel := edgeToRel(edge, q)
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

// edgeToRel resolves a reach's edge leaf to the relation it walks: the queried relation for
// $rel, every relation for $any, the relation a connector reach discovered for $found, else the
// literal named relation the leaf holds.
func edgeToRel(edge string, q ruleQuery) string {
	switch edge {
	case bindRel:
		return q.rel
	case anyEdge:
		return ""
	case bindFound:
		return q.foundRel
	case bindRel2:
		return q.rel2
	default:
		return edge
	}
}

// reachRelSet walks a RELATION-VALUED reach: rather than the concepts a start reaches, it
// returns the RELATIONS by which the start reaches a bound landing concept (the reach's rland
// leaf), in gatherFacts order. It is the connector of a find-then-apply rule -- the "find the
// relation between two concepts" step -- the relation-valued twin of reachSet, and it stands on
// relationBetween the way reachSet stands on reachStep. ok is false when the node is not a
// relation-valued reach (no rland), so a caller falls back.
func (d *Daemon) reachRelSet(reach uint64, q ruleQuery) ([]string, bool) {
	g := d.storeView()
	if g.content(firstDest(g, reach, memory.RelGrounds)) != reachRule {
		return nil, false
	}
	edge := g.content(firstDest(g, reach, reachEdgeRel))
	role := g.content(firstDest(g, reach, reachRoleRel))
	landLeaf := g.content(firstDest(g, reach, reachLandRel))
	if edge == "" || role == "" || landLeaf == "" {
		return nil, false
	}
	if edge == bindLike { // the resemblance reach never feeds a connector (the guardrail)
		return nil, false
	}
	start := q.from
	if b := g.content(firstDest(g, reach, reachStartRel)); b != "" {
		start = d.resolveArgBinding(b, q)
	}
	return d.relationBetween(start, d.resolveArgBinding(landLeaf, q), role, edgeToRel(edge, q)), true
}

// relationBetween returns the relations by which fromID stands to toID in a role, the invariant
// graph-query step reachRelSet stands on (the twin of reachStep). It reads fromID's facts in the
// role over the given relation (empty = any) and keeps the relation of each that lands on toID,
// in gatherFacts order and once each (gatherFacts dedups by relation/role/id), so a connector's
// candidate order matches the hardcoded analogy walk exactly.
func (d *Daemon) relationBetween(fromID, toID uint64, role, rel string) []string {
	frags, err := d.store.FragmentsByIDs([]uint64{fromID})
	if err != nil || len(frags) != 1 {
		return nil
	}
	var out []string
	for _, f := range d.gatherFacts(frags[0], role, rel) {
		if f.coID == toID {
			out = append(out, f.rel)
		}
	}
	return out
}

// reachLikeSet walks a VECTOR-grounded resemblance reach: the concepts the start resembles, each
// with the resemblance weight, read from featureSiblings (the invariant vector+feature step this
// reach stands on, the counterpart of reachStep's gatherFacts). It is the sole entry to the
// weight-bearing neighbourhood and is read only by evalSupport, so a resemblance proposal is
// walled off from the sound reaches. ok is false when the node is not a $like reach or the vector
// read errors, so a caller falls back.
func (d *Daemon) reachLikeSet(reach uint64, q ruleQuery) ([]walkDest, bool) {
	g := d.storeView()
	if g.content(firstDest(g, reach, memory.RelGrounds)) != reachRule {
		return nil, false
	}
	if g.content(firstDest(g, reach, reachEdgeRel)) != bindLike {
		return nil, false
	}
	start := q.from
	if b := g.content(firstDest(g, reach, reachStartRel)); b != "" {
		start = d.resolveArgBinding(b, q)
	}
	sibs, err := d.featureSiblings(start, time.Now())
	if err != nil {
		return nil, false
	}
	return sibs, true
}

// evalSupport runs a resemblance (resSupport) rule: over the neighbours the $like reach yields, it
// tests the queried relation at each (propositionSupport, the invariant graph/held dispose step),
// weights each support by the resemblance strength, and returns the weighted-best as a confidence
// plus the neighbour it rode on. The weighted-max IS the composable part of the resemblance guess;
// the held self-seed, the inference floor, and the working-memory recording stay in the caller
// (assessProposition), the invariant machinery this rule runs on. It never sets hit, so a
// resemblance proposal is a confidence, never a firm answer.
func (d *Daemon) evalSupport(g codeGraph, root, reach uint64, q ruleQuery) (ruleResult, bool) {
	sibs, ok := d.reachLikeSet(reach, q)
	if !ok {
		return ruleResult{}, false
	}
	testRel := d.resolveRelBinding(g.content(firstDest(g, root, findRelRel)), q)
	arg := d.resolveArgBinding(g.content(firstDest(g, root, findArgRel)), q)
	best, bestDepth, bestFrom, bestSupposed, bestSib := 0.0, 0, uint64(0), false, uint64(0)
	for _, sib := range sibs {
		if sib.id == arg { // the patient itself is not evidence that the agent acts on it
			continue
		}
		conf, depth, from, supposed, ok := d.propositionSupport(sib.id, testRel, arg)
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
	case bindThird:
		return q.third
	case bindChained:
		return q.chained
	}
	return 0
}
