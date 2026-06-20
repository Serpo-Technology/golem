package daemon

import (
	"strings"

	"golem/internal/code"
	"golem/internal/memory"
)

// Comprehension by graph traversal: the inverse of the generation driver. Where genCode
// walks a code-meaning subgraph to emit a surface, this reads a code surface and builds
// the subgraph it means, so the Computation type is bypassed on the way in as it is on the
// way out. The same op-constructions drive both directions: generation reads an op's symbol
// to render, comprehension reads its word and arity to parse, the one inventory read each
// way. It sits alongside code.Comprehend (the parser and the oracle), and is proven by
// round trip: a comprehended graph rendered back through genCode equals what the engine
// comprehends and renders. This first cut reads literals, binary arithmetic, and lists, the
// shapes goSurface began with; defined-function calls and conditionals come later.

// comprehendCodeGraph parses one operand from the front of toks into the scratch graph and
// reports how many tokens it spanned: a bare token is a literal leaf, "the list of A and
// B ..." a list node with ordered element slots, and "the <word> of A and B" a binary
// operation grounding in the op the word names, its operands parsed the same way. It fails
// on a surface it cannot place, the way literalOperand does.
func (d *Daemon) comprehendCodeGraph(g *scratchGraph, toks []string) (uint64, int, bool) {
	if len(toks) == 0 {
		return 0, 0, false
	}
	if toks[0] != "the" {
		return g.add(toks[0]), 1, true // a bare literal
	}
	if len(toks) < 3 || toks[2] != "of" {
		return 0, 0, false
	}
	if toks[1] == "list" {
		root := g.add("")
		g.link(root, g.add("list"), memory.RelGrounds)
		consumed, rest, i := 3, toks[3:], 0
		for {
			el, n, ok := d.comprehendCodeGraph(g, rest)
			if !ok {
				return 0, 0, false
			}
			g.attachSlot(root, elemRel, i, el)
			rest, consumed, i = rest[n:], consumed+n, i+1
			if len(rest) == 0 || rest[0] != "and" {
				break
			}
			rest, consumed = rest[1:], consumed+1
		}
		return root, consumed, true
	}
	op, ok := d.opByWord(toks[1])
	if !ok || op.Symbol == "" || opArity(op) != 2 {
		return 0, 0, false // a defined-function call or unknown word, not parsed yet
	}
	consumed, rest := 3, toks[3:]
	left, n, ok := d.comprehendCodeGraph(g, rest)
	if !ok {
		return 0, 0, false
	}
	rest, consumed = rest[n:], consumed+n
	if len(rest) == 0 || rest[0] != "and" {
		return 0, 0, false
	}
	rest, consumed = rest[1:], consumed+1
	right, n, ok := d.comprehendCodeGraph(g, rest)
	if !ok {
		return 0, 0, false
	}
	consumed += n
	root := g.add("")
	g.link(root, g.add(op.Name), memory.RelGrounds)
	g.link(root, left, operand0Rel)
	g.link(root, right, operand1Rel)
	return root, consumed, true
}

// parseCodeToGraph comprehends a whole code surface into a fresh scratch graph, requiring
// every token to be placed, and returns the graph and its root.
func (d *Daemon) parseCodeToGraph(surface string) (*scratchGraph, uint64, bool) {
	toks := strings.Fields(surface)
	g := newScratchGraph()
	root, consumed, ok := d.comprehendCodeGraph(g, toks)
	if !ok || consumed != len(toks) {
		return nil, 0, false
	}
	return g, root, true
}

// opByWord finds a seeded or learned operation by the word it is named with ("sum" ->
// add), read from the code engine so comprehension reuses the one operation inventory
// generation renders from.
func (d *Daemon) opByWord(word string) (code.Op, bool) {
	if d.code == nil {
		return code.Op{}, false
	}
	for _, o := range d.code.Ops() {
		if o.Word == word {
			return o, true
		}
	}
	return code.Op{}, false
}

// opArity is how many operands an operation takes, read from its exported shape: a defined
// or synthesized function takes its parameters, a primitive is binary.
func opArity(o code.Op) int {
	if o.Raw != "" || o.Body != nil {
		return len(o.Params)
	}
	return 2
}
