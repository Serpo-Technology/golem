package daemon

import (
	"golem/internal/code"
	"golem/internal/memory"
)

// materializeCode writes a comprehended computation into the scratch graph as the
// subgraph the traverser reads, the inverse of genCode. It covers the arithmetic
// shapes: a literal or a variable as a leaf, and a binary primitive operation as a
// node grounding in the operation concept with its two operands as ordered operand
// edges. A richer shape (a conditional, a list, a defined call, a map) returns false,
// so the shadow skips it rather than guess; those are the materializer's next widening.
func (d *Daemon) materializeCode(g *scratchGraph, c code.Computation) (uint64, bool) {
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
	// A binary primitive operation: only a primitive carries an operator symbol, so a
	// defined call or a list (no symbol) falls through to the skip below.
	if len(c.Args) == 2 && d.opSymbol(c.Op) != "" {
		left, lok := d.materializeCode(g, c.Args[0])
		right, rok := d.materializeCode(g, c.Args[1])
		if !lok || !rok {
			return 0, false
		}
		root := g.add("")
		g.link(root, g.add(c.Op), memory.RelGrounds)
		g.link(root, left, operand0Rel)
		g.link(root, right, operand1Rel)
		return root, true
	}
	return 0, false
}

// shadowCheck closes the loop on a comprehended computation: it materializes the
// computation as a transient subgraph and walks it the way the prototype walks any
// graph, returning the source. ok is false when the shape is not one the materializer
// builds yet, so the caller skips the comparison rather than reporting a false gap.
func (d *Daemon) shadowCheck(comp code.Computation) (string, bool) {
	g := newScratchGraph()
	root, ok := d.materializeCode(g, comp)
	if !ok {
		return "", false
	}
	return d.genCode(g, root, nil)
}
