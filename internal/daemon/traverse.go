package daemon

import (
	"strings"

	"golem/internal/grammar"
	"golem/internal/memory"
)

// One traverser, two surfaces. A meaning subgraph is walked once, here, and a surface
// (Go, a language) renders each node by selecting its construction: the ordered slots to
// render and how to assemble them into the surface. The walk, the slot order, and the
// recursion are the shared skeleton; the surface supplies what a node means in its
// notation. This proves the north-star shape -- one traverser, surfaces as plug-ins --
// alongside the production genCode and grammar.Speak, whose output it reproduces.

// rendered is a slot's surface text and whether it is a bare infix expression an operand
// position must parenthesize (a Go concern; a language leaves it false).
type rendered struct {
	src      string
	compound bool
}

// slot is a child to render: a node to recurse into, or a leaf already rendered.
type slot struct {
	child uint64
	leaf  rendered
}

// construction renders one node: its ordered slots, and how to assemble them once each
// is rendered.
type construction struct {
	slots    []slot
	assemble func(parts []rendered) rendered
}

// surface is a notation a meaning renders into. It selects the construction for a node.
type surface interface {
	construct(d *Daemon, g codeGraph, node uint64) (construction, bool)
}

// traverse walks the meaning subgraph at node through a surface: select the node's
// construction, render each slot (recursing into a child, or taking a rendered leaf), and
// assemble. The one walk both surfaces share.
func (d *Daemon) traverse(s surface, g codeGraph, node uint64) (rendered, bool) {
	c, ok := s.construct(d, g, node)
	if !ok {
		return rendered{}, false
	}
	parts := make([]rendered, len(c.slots))
	for i, sl := range c.slots {
		if sl.child != 0 {
			r, rok := d.traverse(s, g, sl.child)
			if !rok {
				return rendered{}, false
			}
			parts[i] = r
		} else {
			parts[i] = sl.leaf
		}
	}
	return c.assemble(parts), true
}

// parenthesize wraps a compound (bare infix) source so it keeps its grouping in an
// operand position, the precedence care genOperand takes.
func parenthesize(r rendered) string {
	if r.compound {
		return "(" + r.src + ")"
	}
	return r.src
}

// goSurface renders the Go code shapes: an operation as an infix expression, a list as a
// composite literal, a conditional as the branch function literal, and a leaf as its
// token. Calls and recursion thread the parameter binding and the self node, so they stay
// in genCode for now; this surface covers the binding-free shapes and reproduces it.
type goSurface struct{}

func (goSurface) construct(d *Daemon, g codeGraph, node uint64) (construction, bool) {
	if firstDest(g, node, condThenRel) != 0 {
		cmp := g.content(node)
		left := firstDest(g, node, condLeftRel)
		right := firstDest(g, node, condRightRel)
		then := firstDest(g, node, condThenRel)
		els := firstDest(g, node, condElseRel)
		if (cmp != ">" && cmp != "<" && cmp != "==") || left == 0 || right == 0 || then == 0 || els == 0 {
			return construction{}, false
		}
		return construction{
			slots: []slot{{child: left}, {child: right}, {child: then}, {child: els}},
			assemble: func(parts []rendered) rendered {
				return rendered{src: "(func() int { if " + parenthesize(parts[0]) + " " + cmp + " " +
					parenthesize(parts[1]) + " { return " + parts[2].src + " }; return " + parts[3].src + " }())"}
			},
		}, true
	}
	if firstDest(g, node, elemRel) != 0 {
		if g.content(firstDest(g, node, memory.RelGrounds)) != "list" {
			return construction{}, false // a call threads env/self, a later slice
		}
		ids, ok := d.orderedSlots(g, node, elemRel)
		if !ok {
			return construction{}, false
		}
		slots := make([]slot, len(ids))
		for i, id := range ids {
			slots[i] = slot{child: id}
		}
		return construction{slots: slots, assemble: func(parts []rendered) rendered {
			items := make([]string, len(parts))
			for i, p := range parts {
				items[i] = p.src
			}
			return rendered{src: "[]int{" + strings.Join(items, ", ") + "}"}
		}}, true
	}
	left := firstDest(g, node, operand0Rel)
	right := firstDest(g, node, operand1Rel)
	if left == 0 && right == 0 {
		c := g.content(node)
		if c == "" {
			return construction{}, false
		}
		return construction{assemble: func([]rendered) rendered { return rendered{src: c} }}, true
	}
	sym := d.opSymbol(g.content(firstDest(g, node, memory.RelGrounds)))
	if left == 0 || right == 0 || sym == "" {
		return construction{}, false
	}
	return construction{
		slots: []slot{{child: left}, {child: right}},
		assemble: func(parts []rendered) rendered {
			return rendered{src: parenthesize(parts[0]) + " " + sym + " " + parenthesize(parts[1]), compound: true}
		},
	}, true
}

// nlSurface renders an English transitive clause from a proposition node: a moment
// grounded in its relation, with agent and patient edges to the argument concepts (the
// moment's content marks past tense). The slots lay out in citation order; the verb is
// conjugated and the object marked definite by grammar's morphology, the same Speak
// applies, and the clause is assembled into a sentence.
type nlSurface struct {
	lang string
}

func (s nlSurface) construct(d *Daemon, g codeGraph, node uint64) (construction, bool) {
	verb := g.content(firstDest(g, node, memory.RelGrounds))
	agent := g.content(firstDest(g, node, memory.RelAgent))
	patient := g.content(firstDest(g, node, memory.RelPatient))
	if verb == "" || agent == "" || patient == "" {
		return construction{}, false
	}
	lang := s.lang
	if lang == "" {
		lang = "en"
	}
	past := g.content(node) == "past"
	return construction{
		slots: []slot{
			{leaf: rendered{src: agent}},
			{leaf: rendered{src: d.gram.Conjugate(lang, verb, false, past, false)}},
			{leaf: rendered{src: d.gram.DefiniteNP(lang, "patient", patient)}},
		},
		assemble: func(parts []rendered) rendered {
			words := make([]string, len(parts))
			for i, p := range parts {
				words[i] = p.src
			}
			return rendered{src: grammar.Sentence(strings.Join(words, " "))}
		},
	}, true
}
