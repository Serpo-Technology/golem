package daemon

import (
	"golem/internal/memory"
)

// Code generation by graph traversal: the first prototype of the north-star where
// a program's meaning is a SUBGRAPH the engine walks to emit source, rather than a
// Go type it renders. A computation is a root node grounded in its operation with
// ordered operand edges (operand0, operand1); the walk reads the operation's
// symbol from the code engine, renders each operand (a literal leaf by its
// content, an operation by recursion, parenthesized so precedence is kept), and
// lays them out as the expression. It is grounded the same way every construction
// is, by running the source it emits. This sits alongside the typed render, not in
// place of it; it proves the pattern on the smallest shape. Two design questions
// it deliberately surfaces for the wider arc: ordered/n-ary children in the graph
// (positional rels do not scale) and where the shared traverser eventually lives.
const (
	operand0Rel = "operand0"
	operand1Rel = "operand1"
)

// genCodeFromGraph walks a code-meaning subgraph from its root and emits the Go
// expression it means. A node with operand edges is an operation; a node without is
// a literal leaf, read by its content.
func (d *Daemon) genCodeFromGraph(nodeID uint64) (string, bool) {
	left := d.firstDest(nodeID, operand0Rel)
	right := d.firstDest(nodeID, operand1Rel)
	if left == 0 && right == 0 {
		n := d.fragByID(nodeID)
		if n == nil || n.Content == "" {
			return "", false
		}
		return n.Content, true
	}
	op := d.fragByID(d.firstDest(nodeID, memory.RelGrounds))
	if op == nil {
		return "", false
	}
	sym := d.opSymbol(op.Content)
	l, lok := d.genCodeOperand(left)
	r, rok := d.genCodeOperand(right)
	if sym == "" || !lok || !rok {
		return "", false
	}
	return l + " " + sym + " " + r, true
}

// genCodeOperand renders an operand, wrapping a nested operation in parentheses so
// the emitted expression keeps its grouping whatever the operators' precedence, the
// same care the typed render takes.
func (d *Daemon) genCodeOperand(nodeID uint64) (string, bool) {
	s, ok := d.genCodeFromGraph(nodeID)
	if !ok {
		return "", false
	}
	if d.firstDest(nodeID, operand0Rel) != 0 {
		return "(" + s + ")", true
	}
	return s, true
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
