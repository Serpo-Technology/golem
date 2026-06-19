package daemon

import (
	"sort"
	"strconv"
	"strings"

	"golem/internal/code"
	"golem/internal/memory"
)

// Code generation by graph traversal: a prototype of the north-star where a
// program's meaning is a SUBGRAPH the engine walks to emit source, rather than a
// Go type it renders. A node is one of five shapes: an OPERATION (grounded in its
// operation concept, with ordered operand0/operand1 edges) rendered as an infix
// expression; a CONDITIONAL (the comparison operator as its content, with named
// condLeft/condRight/condThen/condElse edges) rendered as the branch function
// literal; a LIST (grounded in the list concept, its elements held in ordered slot
// nodes) rendered as a composite literal; a CALL (grounded in a function concept, its
// arguments held in ordered slot nodes) inlined by walking the function's body; or a
// LEAF read by its content, a literal or a variable reference alike, since both emit
// their token. Operands recurse and are parenthesized so precedence is kept. It is
// grounded the same way every construction is, by running the source it emits, and
// checked against the typed render. This sits alongside the typed render; it proves
// the pattern over operations, variables, conditionals, ordered children, and calls.
//
// Ordered children (a list's elements, a call's arguments, a function's parameters)
// bind each child to its position through a distinct slot node: root --elem--> slot,
// slot --rank--> ordinal, slot --holds--> value. The slot is the per-occurrence
// binding, the ordinal the position the walk sorts by, the value what sits there.
// Distinct slots let one value node sit at more than one position, so a repeat
// ([]int{5, 5}) places the same item twice without the (from, to, rel) primary key
// colliding. This is positional coding: each item bound to a position, which a plain
// chain cannot do.
//
// A function is a graph: its concept carries a body subgraph and ordered parameter
// slots. A call inlines by traversal: walk the body under a binding of parameter names
// to the call's rendered arguments, the graph counterpart of the typed render's inline
// + substitute. The inlining covers a non-recursive call with simple arguments (a
// literal or a variable); a compound argument would need parenthesizing where it lands
// in an operand position, and a recursive call the self-call form rather than inlining.
const (
	operand0Rel = "operand0"
	operand1Rel = "operand1"

	// A conditional's parts are named roles, not ordered operands: the comparison's
	// two sides and the two branches. The comparison operator is the node's content.
	condLeftRel  = "condLeft"
	condRightRel = "condRight"
	condThenRel  = "condThen"
	condElseRel  = "condElse"

	// Ordered children bind each child to its position through a slot node, so order
	// is meaning and a value can repeat: root --elem--> slot, slot --rank--> ordinal,
	// slot --holds--> value. A list literal uses them; an n-ary call will reuse them.
	elemRel  = "elem"
	rankRel  = "rank"
	holdsRel = "holds"

	// A function is a graph too: its concept carries a body subgraph and its ordered
	// parameters (the slot walk again). A call grounds in the function concept and holds
	// its arguments as ordered children, then inlines by walking the body bound to them.
	paramRel = "param"
	bodyRel  = "body"
)

// boundArg is a rendered argument bound to a parameter: its source and whether that
// source is a bare infix expression. The flag rides with the binding so that when a
// body uses the parameter as an operand, a compound argument is parenthesized there the
// way the typed render parenthesizes a substituted operation.
type boundArg struct {
	src      string
	compound bool
}

// genCodeFromGraph walks a code-meaning subgraph in the persistent store from its root
// and emits the Go expression it means. It is the entry with no names in scope; genCode
// carries the parameter binding a call inlines under and the graph it reads, so the one
// traverser serves a stored subgraph or a transient one alike.
func (d *Daemon) genCodeFromGraph(nodeID uint64) (string, bool) {
	src, _, ok := d.genCode(d.storeView(), nodeID, nil)
	return src, ok
}

// genCode walks a code-meaning subgraph in g under a binding of names to already-
// rendered arguments (the parameters a call has bound, empty at the top level). It
// returns the source, whether that source is a bare infix expression (compound) an
// operand position must parenthesize, and ok. A node is a conditional (named branches),
// an ordered-children node (a list literal or a call, told apart by what it grounds
// in), an operation (ordered operands), or a leaf. A leaf emits its bound argument when
// the binding holds its content -- carrying that argument's own compound flag, so a
// compound argument lands parenthesized where the body uses it -- else the content
// itself, a literal or a free variable.
func (d *Daemon) genCode(g codeGraph, nodeID uint64, env map[string]boundArg) (string, bool, bool) {
	if firstDest(g, nodeID, condThenRel) != 0 {
		s, ok := d.genConditional(g, nodeID, env)
		return s, false, ok // a conditional renders as a parenthesized literal, a primary
	}
	if firstDest(g, nodeID, elemRel) != 0 {
		return d.genOrderedNode(g, nodeID, env)
	}
	left := firstDest(g, nodeID, operand0Rel)
	right := firstDest(g, nodeID, operand1Rel)
	if left == 0 && right == 0 {
		c := g.content(nodeID)
		if c == "" {
			return "", false, false
		}
		if bound, ok := env[c]; ok {
			return bound.src, bound.compound, true
		}
		return c, false, true // a literal or a free variable is a single token
	}
	sym := d.opSymbol(g.content(firstDest(g, nodeID, memory.RelGrounds)))
	l, lok := d.genOperand(g, left, env)
	r, rok := d.genOperand(g, right, env)
	if sym == "" || !lok || !rok {
		return "", false, false
	}
	return l + " " + sym + " " + r, true, true // an infix expression is compound
}

// genOperand renders an operand under the binding, wrapping a compound (bare infix)
// source in parentheses so the emitted expression keeps its grouping whatever the
// operators' precedence. The compound flag comes from the walk, so an inlined call or a
// substituted compound argument is parenthesized as faithfully as a literal operation.
func (d *Daemon) genOperand(g codeGraph, nodeID uint64, env map[string]boundArg) (string, bool) {
	s, compound, ok := d.genCode(g, nodeID, env)
	if !ok {
		return "", false
	}
	if compound {
		return "(" + s + ")", true
	}
	return s, true
}

// genConditional renders a conditional node -- a comparison choosing between two
// branches -- as the parenthesized function literal the typed render uses, so a
// choice composes as an expression and runs (a bare leading function literal would
// not). The comparison operator is the node's content; the sides go through genOperand
// (so a compound side is parenthesized) and the branches render bare, the same shape
// the typed render takes.
func (d *Daemon) genConditional(g codeGraph, nodeID uint64, env map[string]boundArg) (string, bool) {
	cmp := g.content(nodeID)
	if cmp != ">" && cmp != "<" && cmp != "==" {
		return "", false
	}
	left, lok := d.genOperand(g, firstDest(g, nodeID, condLeftRel), env)
	right, rok := d.genOperand(g, firstDest(g, nodeID, condRightRel), env)
	then, _, tok := d.genCode(g, firstDest(g, nodeID, condThenRel), env)
	els, _, eok := d.genCode(g, firstDest(g, nodeID, condElseRel), env)
	if !lok || !rok || !tok || !eok {
		return "", false
	}
	return "(func() int { if " + left + " " + cmp + " " + right +
		" { return " + then + " }; return " + els + " }())", true
}

// genOrderedNode renders a node whose children are ordered -- a list literal or a
// call -- told apart by what it grounds in. The list concept gives a composite
// literal (a primary); a function concept (one carrying a body subgraph) gives an
// inlined call, whose compound flag is the inlined body's, so the call parenthesizes in
// an operand position exactly when the body would.
func (d *Daemon) genOrderedNode(g codeGraph, nodeID uint64, env map[string]boundArg) (string, bool, bool) {
	fn := firstDest(g, nodeID, memory.RelGrounds)
	if g.content(fn) == "list" {
		elems, ok := d.orderedChildren(g, nodeID, elemRel, env)
		if !ok {
			return "", false, false
		}
		parts := make([]string, len(elems))
		for i, e := range elems {
			parts[i] = e.src
		}
		return "[]int{" + strings.Join(parts, ", ") + "}", false, true
	}
	if firstDest(g, fn, bodyRel) != 0 {
		return d.genCall(g, nodeID, fn, env)
	}
	return "", false, false
}

// genCall inlines a call by traversal: it binds the function's ordered parameters to
// the call's rendered arguments and walks the function's body under that binding, the
// graph counterpart of the typed render's inline + substitute. Arguments are rendered
// in the caller's binding, each keeping its compound flag, so the body parenthesizes a
// compound argument where it uses the parameter as an operand. A body leaf that names a
// parameter emits its argument, and the call expands to the body in place.
func (d *Daemon) genCall(g codeGraph, nodeID, fn uint64, env map[string]boundArg) (string, bool, bool) {
	body := firstDest(g, fn, bodyRel)
	if body == 0 {
		return "", false, false
	}
	params, pok := d.orderedSlots(g, fn, paramRel)
	args, aok := d.orderedChildren(g, nodeID, elemRel, env)
	if !pok || !aok || len(params) != len(args) {
		return "", false, false
	}
	binding := make(map[string]boundArg, len(params))
	for i, p := range params {
		name := g.content(p)
		if name == "" {
			return "", false, false
		}
		binding[name] = args[i]
	}
	return d.genCode(g, body, binding)
}

// orderedSlots returns the held nodes of a node's ordered children in position order,
// each reached through a slot carrying its ordinal (rank) and the value it holds. The
// slot is the per-occurrence binding, so one node can be held at more than one position
// without the edge's primary key colliding. Parameters read the held nodes by name;
// arguments and list elements render them.
func (d *Daemon) orderedSlots(g codeGraph, nodeID uint64, rel string) ([]uint64, bool) {
	slots := g.dests(nodeID, rel)
	if len(slots) == 0 {
		return nil, false
	}
	type held struct {
		rank int
		id   uint64
	}
	items := make([]held, 0, len(slots))
	for _, slot := range slots {
		id := firstDest(g, slot, holdsRel)
		rank, err := strconv.Atoi(g.content(firstDest(g, slot, rankRel)))
		if id == 0 || err != nil {
			return nil, false
		}
		items = append(items, held{rank: rank, id: id})
	}
	sort.Slice(items, func(i, j int) bool { return items[i].rank < items[j].rank })
	out := make([]uint64, len(items))
	for i, it := range items {
		out[i] = it.id
	}
	return out, true
}

// orderedChildren renders a node's ordered children in position order, each held value
// walked under the binding and keeping its compound flag, so a child may be a literal,
// a variable, or a nested sub-meaning. A list's elements and a call's arguments share
// it (the list takes the sources, a call also takes the flags for operand hygiene).
func (d *Daemon) orderedChildren(g codeGraph, nodeID uint64, rel string, env map[string]boundArg) ([]boundArg, bool) {
	ids, ok := d.orderedSlots(g, nodeID, rel)
	if !ok {
		return nil, false
	}
	out := make([]boundArg, len(ids))
	for i, id := range ids {
		src, compound, ok := d.genCode(g, id, env)
		if !ok {
			return nil, false
		}
		out[i] = boundArg{src: src, compound: compound}
	}
	return out, true
}

// opByName finds a seeded or learned operation by name, read from the code engine so
// the walk and the materializer reuse the one operation inventory.
func (d *Daemon) opByName(name string) (code.Op, bool) {
	if d.code == nil {
		return code.Op{}, false
	}
	for _, o := range d.code.Ops() {
		if o.Name == name {
			return o, true
		}
	}
	return code.Op{}, false
}

// opSymbol is the operator symbol of a primitive operation by name ("add" -> "+"),
// empty when the name is not a primitive (a defined function carries no symbol).
func (d *Daemon) opSymbol(name string) string {
	op, ok := d.opByName(name)
	if !ok {
		return ""
	}
	return op.Symbol
}

// codeGraph is the read view the traverser needs: a node's content, and the nodes a
// relation leaves it for. The persistent store and a transient scratch graph both
// satisfy it, so the traverser walks "a graph" rather than a particular store.
type codeGraph interface {
	content(id uint64) string
	dests(id uint64, rel string) []uint64
}

// firstDest is the first node a relation leaves the node for, 0 when none.
func firstDest(g codeGraph, nodeID uint64, rel string) uint64 {
	ds := g.dests(nodeID, rel)
	if len(ds) == 0 {
		return 0
	}
	return ds[0]
}

// storeGraph adapts the persistent store to the read view, reading a node's content
// and a relation's destinations from memory. d.storeView is the production graph.
type storeGraph struct{ s *memory.Store }

func (d *Daemon) storeView() storeGraph { return storeGraph{s: d.store} }

func (g storeGraph) content(id uint64) string {
	if id == 0 {
		return ""
	}
	frags, err := g.s.FragmentsByIDs([]uint64{id})
	if err != nil || len(frags) != 1 {
		return ""
	}
	return frags[0].Content
}

func (g storeGraph) dests(id uint64, rel string) []uint64 {
	edges, err := g.s.LinksFromRel([]uint64{id}, rel)
	if err != nil {
		return nil
	}
	out := make([]uint64, len(edges))
	for i, e := range edges {
		out[i] = e.To
	}
	return out
}

// scratchGraph is a transient in-memory code graph. The shadow check materializes a
// comprehended computation into one, walks it, and discards it, so the check leaves no
// scaffolding behind in memory. Ids start at 1, leaving 0 as the "no node" sentinel
// the traverser reads as absence.
type scratchGraph struct {
	next     uint64
	contents map[uint64]string
	edges    map[uint64]map[string][]uint64
}

func newScratchGraph() *scratchGraph {
	return &scratchGraph{contents: map[uint64]string{}, edges: map[uint64]map[string][]uint64{}}
}

// add mints a node holding content and returns its id. link adds a relation edge.
func (g *scratchGraph) add(content string) uint64 {
	g.next++
	g.contents[g.next] = content
	return g.next
}

func (g *scratchGraph) link(from, to uint64, rel string) {
	if g.edges[from] == nil {
		g.edges[from] = map[string][]uint64{}
	}
	g.edges[from][rel] = append(g.edges[from][rel], to)
}

// attachSlot binds held to parent as the rank-th ordered child under rel: a distinct
// slot node carrying the ordinal and holding the value, the write side of the slot walk
// orderedSlots reads.
func (g *scratchGraph) attachSlot(parent uint64, rel string, rank int, held uint64) {
	slot := g.add("")
	g.link(parent, slot, rel)
	g.link(slot, g.add(strconv.Itoa(rank)), rankRel)
	g.link(slot, held, holdsRel)
}

func (g *scratchGraph) content(id uint64) string { return g.contents[id] }

func (g *scratchGraph) dests(id uint64, rel string) []uint64 {
	if g.edges[id] == nil {
		return nil
	}
	return g.edges[id][rel]
}
