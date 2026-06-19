package daemon

import (
	"golem/internal/code"
	"golem/internal/memory"
)

// materializeCode writes a comprehended computation into the scratch graph as the
// subgraph the traverser reads, the inverse of genCode. It mirrors the read shapes: a
// literal or variable leaf, a binary primitive operation, a conditional, a list literal,
// and a call of a defined function (materialized as a graph: the function's ordered
// parameters and its body). The traverser parenthesizes a compound operand faithfully,
// so a call may sit in any position and its arguments may be compound. selfName is the
// function whose body is being written, fnNodes the functions open above it: a call to
// selfName grounds in its own node (a recursive self-reference the traverser writes as a
// local function), and a call back to a different open function (mutual recursion) is
// refused, the one call shape the shadow skips.
func (d *Daemon) materializeCode(g *scratchGraph, c code.Computation, selfName string, fnNodes map[string]uint64) (uint64, bool) {
	// A leaf carries its token as content. An "each" reference lives only inside a map
	// body, which is not materialized, so it is not a leaf here.
	if c.Op == "" && c.Cond == nil && c.Map == nil && !c.Each {
		if c.Var != "" {
			return g.add(c.Var), true
		}
		if c.Lit != "" {
			return g.add(c.Lit), true
		}
		return 0, false
	}
	// A conditional: the comparison operator as content, its two sides and two branches
	// as named edges.
	if c.Cond != nil {
		root := g.add(c.Cond.Cmp)
		if d.materializeInto(g, root, condLeftRel, c.Cond.Left, selfName, fnNodes) &&
			d.materializeInto(g, root, condRightRel, c.Cond.Right, selfName, fnNodes) &&
			d.materializeInto(g, root, condThenRel, c.Cond.Then, selfName, fnNodes) &&
			d.materializeInto(g, root, condElseRel, c.Cond.Else, selfName, fnNodes) {
			return root, true
		}
		return 0, false
	}
	// A list literal: a root grounding in the list concept, its elements held in ordered
	// slots.
	if c.Op == "list" {
		root := g.add("")
		g.link(root, g.add("list"), memory.RelGrounds)
		if d.materializeSlots(g, root, elemRel, c.Args, selfName, fnNodes) {
			return root, true
		}
		return 0, false
	}
	// A binary primitive operation: a root grounding in the operation concept with its
	// two operands.
	if len(c.Args) == 2 && d.opSymbol(c.Op) != "" {
		root := g.add("")
		g.link(root, g.add(c.Op), memory.RelGrounds)
		if d.materializeInto(g, root, operand0Rel, c.Args[0], selfName, fnNodes) &&
			d.materializeInto(g, root, operand1Rel, c.Args[1], selfName, fnNodes) {
			return root, true
		}
		return 0, false
	}
	// A call of a defined function.
	if d.isCall(c) {
		return d.materializeCall(g, c, selfName, fnNodes)
	}
	return 0, false
}

// materializeCall builds a call subgraph. A call to the function whose body is being
// written (selfName) grounds in that same function node, the recursive self-reference;
// a call back to a different open function is mutual recursion, out of scope, refused.
// Otherwise the function is built as a graph (its ordered parameter names and its body,
// the body written with the function itself as the current self) and the call grounds in
// it with its ordered arguments, which stay in the caller's scope.
func (d *Daemon) materializeCall(g *scratchGraph, c code.Computation, selfName string, fnNodes map[string]uint64) (uint64, bool) {
	if c.Op == selfName {
		call := g.add("")
		g.link(call, fnNodes[selfName], memory.RelGrounds)
		if d.materializeSlots(g, call, elemRel, c.Args, selfName, fnNodes) {
			return call, true
		}
		return 0, false
	}
	if _, open := fnNodes[c.Op]; open {
		return 0, false // mutual recursion is out of scope
	}
	op, ok := d.opByName(c.Op)
	if !ok || op.Body == nil || op.Raw != "" || len(op.Params) != len(c.Args) {
		return 0, false
	}
	fn := g.add(c.Op)
	fnNodes[c.Op] = fn
	defer delete(fnNodes, c.Op)

	for i, p := range op.Params {
		g.attachSlot(fn, paramRel, i, g.add(p))
	}
	if !d.materializeInto(g, fn, bodyRel, *op.Body, c.Op, fnNodes) {
		return 0, false
	}
	call := g.add("")
	g.link(call, fn, memory.RelGrounds)
	if !d.materializeSlots(g, call, elemRel, c.Args, selfName, fnNodes) {
		return 0, false
	}
	return call, true
}

// materializeInto materializes c and links it from parent under rel, false on failure.
func (d *Daemon) materializeInto(g *scratchGraph, parent uint64, rel string, c code.Computation, selfName string, fnNodes map[string]uint64) bool {
	id, ok := d.materializeCode(g, c, selfName, fnNodes)
	if !ok {
		return false
	}
	g.link(parent, id, rel)
	return true
}

// materializeSlots materializes each child and attaches it to parent as an ordered slot.
func (d *Daemon) materializeSlots(g *scratchGraph, parent uint64, rel string, children []code.Computation, selfName string, fnNodes map[string]uint64) bool {
	for i, child := range children {
		id, ok := d.materializeCode(g, child, selfName, fnNodes)
		if !ok {
			return false
		}
		g.attachSlot(parent, rel, i, id)
	}
	return true
}

// isCall reports whether c is a call of a defined function: a named operation that is
// neither a primitive (no operator symbol) nor a list literal.
func (d *Daemon) isCall(c code.Computation) bool {
	return c.Op != "" && c.Op != "list" && d.opSymbol(c.Op) == ""
}

// traverseCode generates Go from a comprehended computation by graph traversal: it
// materializes the computation as a transient subgraph and walks it through the shared
// traverser's Go surface for the source. This is the production generator on the runcode
// path, so the one traverser that renders language renders code here too. ok is false
// when the shape is not one the materializer builds yet (a map), so the caller falls back
// to the typed render.
func (d *Daemon) traverseCode(comp code.Computation) (string, bool) {
	g := newScratchGraph()
	root, ok := d.materializeCode(g, comp, "", map[string]uint64{})
	if !ok {
		return "", false
	}
	r, rok := d.traverse(goSurface{}, g, root, nil)
	return r.src, rok
}
