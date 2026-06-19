package daemon

import (
	"golem/internal/code"
	"golem/internal/memory"
)

// materializeCode writes a comprehended computation into the scratch graph as the
// subgraph the traverser reads, the inverse of genCode. It mirrors the read shapes: a
// literal or variable leaf, a binary primitive operation, a conditional, a list
// literal, and a call of a defined function (materialized as a graph: the function's
// ordered parameters and its body). seen carries the functions being materialized so a
// recursive call is refused rather than looped.
//
// The shadow stays within the inliner's reach: a defined call may not sit in an operand
// or comparison position (an inlined call there would need parenthesizing the traverser
// does not yet apply), and a call's arguments must be simple leaves (a compound argument
// needs the same hygiene). Those shapes return false, so the shadow skips them rather
// than reporting a false gap.
func (d *Daemon) materializeCode(g *scratchGraph, c code.Computation, seen map[string]bool) (uint64, bool) {
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
	// as named edges. A call in a comparison side is refused; the branches render bare,
	// so a call there composes.
	if c.Cond != nil {
		if d.isCall(c.Cond.Left) || d.isCall(c.Cond.Right) {
			return 0, false
		}
		root := g.add(c.Cond.Cmp)
		if d.materializeInto(g, root, condLeftRel, c.Cond.Left, seen) &&
			d.materializeInto(g, root, condRightRel, c.Cond.Right, seen) &&
			d.materializeInto(g, root, condThenRel, c.Cond.Then, seen) &&
			d.materializeInto(g, root, condElseRel, c.Cond.Else, seen) {
			return root, true
		}
		return 0, false
	}
	// A list literal: a root grounding in the list concept, its elements held in ordered
	// slots. Elements render bare, so a call element composes.
	if c.Op == "list" {
		root := g.add("")
		g.link(root, g.add("list"), memory.RelGrounds)
		if d.materializeSlots(g, root, elemRel, c.Args, seen) {
			return root, true
		}
		return 0, false
	}
	// A binary primitive operation: a root grounding in the operation concept with its
	// two operands. A call in an operand is refused, as in a comparison side.
	if len(c.Args) == 2 && d.opSymbol(c.Op) != "" {
		if d.isCall(c.Args[0]) || d.isCall(c.Args[1]) {
			return 0, false
		}
		root := g.add("")
		g.link(root, g.add(c.Op), memory.RelGrounds)
		if d.materializeInto(g, root, operand0Rel, c.Args[0], seen) &&
			d.materializeInto(g, root, operand1Rel, c.Args[1], seen) {
			return root, true
		}
		return 0, false
	}
	// A call of a defined function.
	if d.isCall(c) {
		return d.materializeCall(g, c, seen)
	}
	return 0, false
}

// materializeCall builds a call subgraph: the function as a graph (its ordered parameter
// names and its body) and the call grounding in it with ordered arguments. It is refused
// unless the arguments are simple leaves and the function is known, non-recursive, and
// has a materializable body, keeping the shadow within what genCall inlines.
func (d *Daemon) materializeCall(g *scratchGraph, c code.Computation, seen map[string]bool) (uint64, bool) {
	if seen[c.Op] {
		return 0, false // a recursive call is not inlined
	}
	op, ok := d.opByName(c.Op)
	if !ok || op.Body == nil || op.Raw != "" || len(op.Params) != len(c.Args) {
		return 0, false
	}
	for _, a := range c.Args {
		if !isLeaf(a) {
			return 0, false // a compound argument needs operand hygiene not yet built
		}
	}
	seen[c.Op] = true
	defer delete(seen, c.Op)

	fn := g.add(c.Op)
	for i, p := range op.Params {
		g.attachSlot(fn, paramRel, i, g.add(p))
	}
	if !d.materializeInto(g, fn, bodyRel, *op.Body, seen) {
		return 0, false
	}
	call := g.add("")
	g.link(call, fn, memory.RelGrounds)
	if !d.materializeSlots(g, call, elemRel, c.Args, seen) {
		return 0, false
	}
	return call, true
}

// materializeInto materializes c and links it from parent under rel, false on failure.
func (d *Daemon) materializeInto(g *scratchGraph, parent uint64, rel string, c code.Computation, seen map[string]bool) bool {
	id, ok := d.materializeCode(g, c, seen)
	if !ok {
		return false
	}
	g.link(parent, id, rel)
	return true
}

// materializeSlots materializes each child and attaches it to parent as an ordered slot.
func (d *Daemon) materializeSlots(g *scratchGraph, parent uint64, rel string, children []code.Computation, seen map[string]bool) bool {
	for i, child := range children {
		id, ok := d.materializeCode(g, child, seen)
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

// isLeaf reports whether c is a literal or a variable reference, the shape that emits a
// single token and so needs no parenthesizing wherever it lands.
func isLeaf(c code.Computation) bool {
	return c.Op == "" && c.Cond == nil && c.Map == nil && !c.Each && (c.Lit != "" || c.Var != "")
}

// shadowCheck closes the loop on a comprehended computation: it materializes the
// computation as a transient subgraph and walks it the way the prototype walks any
// graph, returning the source. ok is false when the shape is not one the materializer
// builds yet, so the caller skips the comparison rather than reporting a false gap.
func (d *Daemon) shadowCheck(comp code.Computation) (string, bool) {
	g := newScratchGraph()
	root, ok := d.materializeCode(g, comp, map[string]bool{})
	if !ok {
		return "", false
	}
	return d.genCode(g, root, nil)
}
