package daemon

import (
	"golem/internal/memory"
)

// Code generation by graph traversal: a prototype of the north-star where a
// program's meaning is a SUBGRAPH the engine walks to emit source, rather than a
// Go type it renders. A node is one of three shapes: an OPERATION (grounded in its
// operation concept, with ordered operand0/operand1 edges) rendered as an infix
// expression; a CONDITIONAL (the comparison operator as its content, with named
// condLeft/condRight/condThen/condElse edges) rendered as the branch function
// literal; or a LEAF read by its content -- a literal or a variable reference
// alike, since both emit their token. Operands recurse and are parenthesized so
// precedence is kept. It is grounded the same way every construction is, by running
// the source it emits, and checked against the typed render. This sits alongside
// the typed render; it proves the pattern (now over operations, variables, and
// conditionals -- a whole conditional function body). The open design question for
// the wider arc: ordered/n-ary children in the graph (calls and list literals;
// positional rels do not scale -- an order field on the edge or operand-slot nodes).
const (
	operand0Rel = "operand0"
	operand1Rel = "operand1"

	// A conditional's parts are named roles, not ordered operands: the comparison's
	// two sides and the two branches. The comparison operator is the node's content.
	condLeftRel  = "condLeft"
	condRightRel = "condRight"
	condThenRel  = "condThen"
	condElseRel  = "condElse"
)

// genCodeFromGraph walks a code-meaning subgraph from its root and emits the Go
// expression it means: a conditional node (named branches) as a choice, an
// operation node (ordered operands) as an infix expression, and a leaf as its
// content -- a literal or a variable reference alike, since both emit their token.
func (d *Daemon) genCodeFromGraph(nodeID uint64) (string, bool) {
	if d.firstDest(nodeID, condThenRel) != 0 {
		return d.genCondFromGraph(nodeID)
	}
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

// genCondFromGraph renders a conditional node -- a comparison choosing between two
// branches -- as the parenthesized function literal the typed render uses, so a
// choice composes as an expression and runs (a bare leading function literal would
// not). The comparison operator is the node's content; the sides and branches are
// named roles.
func (d *Daemon) genCondFromGraph(nodeID uint64) (string, bool) {
	n := d.fragByID(nodeID)
	if n == nil || (n.Content != ">" && n.Content != "<" && n.Content != "==") {
		return "", false
	}
	left, lok := d.genCodeOperand(d.firstDest(nodeID, condLeftRel))
	right, rok := d.genCodeOperand(d.firstDest(nodeID, condRightRel))
	then, tok := d.genCodeFromGraph(d.firstDest(nodeID, condThenRel))
	els, eok := d.genCodeFromGraph(d.firstDest(nodeID, condElseRel))
	if !lok || !rok || !tok || !eok {
		return "", false
	}
	return "(func() int { if " + left + " " + n.Content + " " + right +
		" { return " + then + " }; return " + els + " }())", true
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
