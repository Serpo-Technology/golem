package daemon

import (
	"sort"
	"strconv"
	"strings"

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

// genCodeFromGraph walks a code-meaning subgraph from its root and emits the Go
// expression it means. It is the entry with no names in scope; genCode carries the
// parameter binding a call inlines under.
func (d *Daemon) genCodeFromGraph(nodeID uint64) (string, bool) {
	return d.genCode(nodeID, nil)
}

// genCode walks a code-meaning subgraph under a binding of names to already-rendered
// sources (the parameters a call has bound to its arguments, empty at the top level).
// A node is a conditional (named branches), an ordered-children node (a list literal
// or a call, told apart by what it grounds in), an operation (ordered operands), or a
// leaf. A leaf emits its bound source when the binding holds its content, else the
// content itself -- a literal or a free variable alike.
func (d *Daemon) genCode(nodeID uint64, env map[string]string) (string, bool) {
	if d.firstDest(nodeID, condThenRel) != 0 {
		return d.genConditional(nodeID, env)
	}
	if d.firstDest(nodeID, elemRel) != 0 {
		return d.genOrderedNode(nodeID, env)
	}
	left := d.firstDest(nodeID, operand0Rel)
	right := d.firstDest(nodeID, operand1Rel)
	if left == 0 && right == 0 {
		n := d.fragByID(nodeID)
		if n == nil || n.Content == "" {
			return "", false
		}
		if bound, ok := env[n.Content]; ok {
			return bound, true
		}
		return n.Content, true
	}
	op := d.fragByID(d.firstDest(nodeID, memory.RelGrounds))
	if op == nil {
		return "", false
	}
	sym := d.opSymbol(op.Content)
	l, lok := d.genOperand(left, env)
	r, rok := d.genOperand(right, env)
	if sym == "" || !lok || !rok {
		return "", false
	}
	return l + " " + sym + " " + r, true
}

// genOperand renders an operand under the binding, wrapping a nested operation in
// parentheses so the emitted expression keeps its grouping whatever the operators'
// precedence, the same care the typed render takes.
func (d *Daemon) genOperand(nodeID uint64, env map[string]string) (string, bool) {
	s, ok := d.genCode(nodeID, env)
	if !ok {
		return "", false
	}
	if d.firstDest(nodeID, operand0Rel) != 0 {
		return "(" + s + ")", true
	}
	return s, true
}

// genConditional renders a conditional node -- a comparison choosing between two
// branches -- as the parenthesized function literal the typed render uses, so a
// choice composes as an expression and runs (a bare leading function literal would
// not). The comparison operator is the node's content; the sides and branches are
// named roles, each walked under the binding.
func (d *Daemon) genConditional(nodeID uint64, env map[string]string) (string, bool) {
	n := d.fragByID(nodeID)
	if n == nil || (n.Content != ">" && n.Content != "<" && n.Content != "==") {
		return "", false
	}
	left, lok := d.genOperand(d.firstDest(nodeID, condLeftRel), env)
	right, rok := d.genOperand(d.firstDest(nodeID, condRightRel), env)
	then, tok := d.genCode(d.firstDest(nodeID, condThenRel), env)
	els, eok := d.genCode(d.firstDest(nodeID, condElseRel), env)
	if !lok || !rok || !tok || !eok {
		return "", false
	}
	return "(func() int { if " + left + " " + n.Content + " " + right +
		" { return " + then + " }; return " + els + " }())", true
}

// genOrderedNode renders a node whose children are ordered -- a list literal or a
// call -- told apart by what it grounds in. The list concept gives a composite
// literal; a function concept (one carrying a body subgraph) gives an inlined call.
func (d *Daemon) genOrderedNode(nodeID uint64, env map[string]string) (string, bool) {
	fn := d.firstDest(nodeID, memory.RelGrounds)
	concept := d.fragByID(fn)
	if concept == nil {
		return "", false
	}
	if concept.Content == "list" {
		elems, ok := d.orderedChildren(nodeID, elemRel, env)
		if !ok {
			return "", false
		}
		return "[]int{" + strings.Join(elems, ", ") + "}", true
	}
	if d.firstDest(fn, bodyRel) != 0 {
		return d.genCall(nodeID, fn, env)
	}
	return "", false
}

// genCall inlines a call by traversal: it binds the function's ordered parameters to
// the call's rendered arguments and walks the function's body under that binding, the
// graph counterpart of the typed render's inline + substitute. Arguments are rendered
// in the caller's binding; the body then sees only its parameters, so a body leaf that
// names a parameter emits its argument and the call expands to the body in place.
func (d *Daemon) genCall(nodeID, fn uint64, env map[string]string) (string, bool) {
	body := d.firstDest(fn, bodyRel)
	if body == 0 {
		return "", false
	}
	params, pok := d.orderedSlots(fn, paramRel)
	args, aok := d.orderedChildren(nodeID, elemRel, env)
	if !pok || !aok || len(params) != len(args) {
		return "", false
	}
	binding := make(map[string]string, len(params))
	for i, p := range params {
		name := d.fragByID(p)
		if name == nil || name.Content == "" {
			return "", false
		}
		binding[name.Content] = args[i]
	}
	return d.genCode(body, binding)
}

// orderedSlots returns the held nodes of a node's ordered children in position order,
// each reached through a slot carrying its ordinal (rank) and the value it holds. The
// slot is the per-occurrence binding, so one node can be held at more than one position
// without the edge's primary key colliding. Parameters read the held nodes by name;
// arguments and list elements render them.
func (d *Daemon) orderedSlots(nodeID uint64, rel string) ([]uint64, bool) {
	slots, err := d.store.LinksFromRel([]uint64{nodeID}, rel)
	if err != nil || len(slots) == 0 {
		return nil, false
	}
	type held struct {
		rank int
		id   uint64
	}
	items := make([]held, 0, len(slots))
	for _, slot := range slots {
		ordinal := d.fragByID(d.firstDest(slot.To, rankRel))
		id := d.firstDest(slot.To, holdsRel)
		if ordinal == nil || id == 0 {
			return nil, false
		}
		rank, err := strconv.Atoi(ordinal.Content)
		if err != nil {
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
// walked under the binding so a child may be a literal, a variable, or a nested
// sub-meaning. A list's elements and a call's arguments share it.
func (d *Daemon) orderedChildren(nodeID uint64, rel string, env map[string]string) ([]string, bool) {
	ids, ok := d.orderedSlots(nodeID, rel)
	if !ok {
		return nil, false
	}
	out := make([]string, len(ids))
	for i, id := range ids {
		src, ok := d.genCode(id, env)
		if !ok {
			return nil, false
		}
		out[i] = src
	}
	return out, true
}

// firstDest is the first node a relation edge leaves the node for, 0 when none.
func (d *Daemon) firstDest(nodeID uint64, rel string) uint64 {
	edges, err := d.store.LinksFromRel([]uint64{nodeID}, rel)
	if err != nil || len(edges) == 0 {
		return 0
	}
	return edges[0].To
}

// fragByID reads one fragment by id, nil when absent.
func (d *Daemon) fragByID(id uint64) *memory.Fragment {
	if id == 0 {
		return nil
	}
	frags, err := d.store.FragmentsByIDs([]uint64{id})
	if err != nil || len(frags) != 1 {
		return nil
	}
	return frags[0]
}

// opSymbol is the operator symbol of a seeded operation by name ("add" -> "+"),
// read from the code engine so the walk reuses the one operator inventory.
func (d *Daemon) opSymbol(name string) string {
	if d.code == nil {
		return ""
	}
	for _, o := range d.code.Ops() {
		if o.Name == name {
			return o.Symbol
		}
	}
	return ""
}
