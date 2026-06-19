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

// slot is a child to render: a node to recurse into (under its own context), or a leaf
// already rendered.
type slot struct {
	child uint64
	ctx   any
	leaf  rendered
}

// construction renders one node: its ordered slots, and how to assemble them once each is
// rendered (false aborts the walk).
type construction struct {
	slots    []slot
	assemble func(parts []rendered) (rendered, bool)
}

// surface is a notation a meaning renders into. It selects the construction for a node
// under the threaded context (surface-specific; the Go surface carries the parameter
// binding and the recursive-function node, a language carries none).
type surface interface {
	construct(d *Daemon, g codeGraph, node uint64, ctx any) (construction, bool)
}

// traverse walks the meaning subgraph at node through a surface: select the node's
// construction, render each slot (recursing into a child under its context, or taking a
// rendered leaf), and assemble. The one walk both surfaces share.
func (d *Daemon) traverse(s surface, g codeGraph, node uint64, ctx any) (rendered, bool) {
	c, ok := s.construct(d, g, node, ctx)
	if !ok {
		return rendered{}, false
	}
	parts := make([]rendered, len(c.slots))
	for i, sl := range c.slots {
		if sl.child != 0 {
			r, rok := d.traverse(s, g, sl.child, sl.ctx)
			if !rok {
				return rendered{}, false
			}
			parts[i] = r
		} else {
			parts[i] = sl.leaf
		}
	}
	return c.assemble(parts)
}

// parenthesize wraps a compound (bare infix) source so it keeps its grouping in an
// operand position, the precedence care genOperand takes.
func parenthesize(r rendered) string {
	if r.compound {
		return "(" + r.src + ")"
	}
	return r.src
}

// codeCtx is the Go surface's threaded context: the parameter binding a call inlines
// under (empty at the top level) and the function node a recursive body is written
// inside. asCodeCtx reads it from the opaque context the driver threads.
type codeCtx struct {
	env  map[string]boundArg
	self uint64
}

func asCodeCtx(ctx any) codeCtx {
	if c, ok := ctx.(codeCtx); ok {
		return c
	}
	return codeCtx{}
}

// goSurface renders the Go code shapes: an operation as an infix expression, a list as a
// composite literal, a conditional as the branch function literal, a leaf as its token
// (a bound parameter as its argument), and a call by inlining its body under the binding
// of its parameters to its arguments. A recursive call cannot inline; its
// self-referential local-function form is written by genRecursive, the one shape the
// driver hands to a helper rather than walking. The whole reproduces genCode.
type goSurface struct{}

func (goSurface) construct(d *Daemon, g codeGraph, node uint64, ctx any) (construction, bool) {
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
			slots: []slot{{child: left, ctx: ctx}, {child: right, ctx: ctx}, {child: then, ctx: ctx}, {child: els, ctx: ctx}},
			assemble: func(parts []rendered) (rendered, bool) {
				return rendered{src: "(func() int { if " + parenthesize(parts[0]) + " " + cmp + " " +
					parenthesize(parts[1]) + " { return " + parts[2].src + " }; return " + parts[3].src + " }())"}, true
			},
		}, true
	}
	if firstDest(g, node, elemRel) != 0 {
		grounds := firstDest(g, node, memory.RelGrounds)
		if g.content(grounds) == "list" {
			ids, ok := d.orderedSlots(g, node, elemRel)
			if !ok {
				return construction{}, false
			}
			return construction{slots: childSlots(ids, ctx), assemble: func(parts []rendered) (rendered, bool) {
				return rendered{src: "[]int{" + joinRendered(parts) + "}"}, true
			}}, true
		}
		if firstDest(g, grounds, bodyRel) != 0 {
			return d.goCall(g, node, grounds, ctx)
		}
		return construction{}, false
	}
	left := firstDest(g, node, operand0Rel)
	right := firstDest(g, node, operand1Rel)
	if left == 0 && right == 0 {
		c := g.content(node)
		if c == "" {
			return construction{}, false
		}
		if bound, ok := asCodeCtx(ctx).env[c]; ok {
			return construction{assemble: func([]rendered) (rendered, bool) {
				return rendered{src: bound.src, compound: bound.compound}, true
			}}, true
		}
		return construction{assemble: func([]rendered) (rendered, bool) { return rendered{src: c}, true }}, true
	}
	sym := d.opSymbol(g.content(firstDest(g, node, memory.RelGrounds)))
	if left == 0 || right == 0 || sym == "" {
		return construction{}, false
	}
	return construction{
		slots: []slot{{child: left, ctx: ctx}, {child: right, ctx: ctx}},
		assemble: func(parts []rendered) (rendered, bool) {
			return rendered{src: parenthesize(parts[0]) + " " + sym + " " + parenthesize(parts[1]), compound: true}, true
		},
	}, true
}

// goCall renders a call. A recursive function is handed to genRecursive (its
// local-function form is not a slot layout). A non-recursive one inlines: the driver
// renders its arguments, and assemble binds them to the parameters and walks the body
// under that binding, so a body leaf that names a parameter emits its argument.
func (d *Daemon) goCall(g codeGraph, node, fn uint64, ctx any) (construction, bool) {
	cc := asCodeCtx(ctx)
	if d.isRecursive(g, fn) {
		return construction{assemble: func([]rendered) (rendered, bool) {
			src, compound, ok := d.genRecursive(g, node, fn, cc.env)
			return rendered{src: src, compound: compound}, ok
		}}, true
	}
	body := firstDest(g, fn, bodyRel)
	params, pok := d.orderedSlots(g, fn, paramRel)
	args, aok := d.orderedSlots(g, node, elemRel)
	if body == 0 || !pok || !aok || len(params) != len(args) {
		return construction{}, false
	}
	return construction{slots: childSlots(args, ctx), assemble: func(parts []rendered) (rendered, bool) {
		binding := make(map[string]boundArg, len(params))
		for i, p := range params {
			name := g.content(p)
			if name == "" {
				return rendered{}, false
			}
			binding[name] = boundArg{src: parts[i].src, compound: parts[i].compound}
		}
		return d.traverse(goSurface{}, g, body, codeCtx{env: binding, self: cc.self})
	}}, true
}

// childSlots makes a child slot per node, each rendered under the same context.
func childSlots(ids []uint64, ctx any) []slot {
	slots := make([]slot, len(ids))
	for i, id := range ids {
		slots[i] = slot{child: id, ctx: ctx}
	}
	return slots
}

// joinRendered joins rendered parts by their sources, the form a comma-separated list of
// elements or arguments takes.
func joinRendered(parts []rendered) string {
	out := make([]string, len(parts))
	for i, p := range parts {
		out[i] = p.src
	}
	return strings.Join(out, ", ")
}

// nlSurface renders a transitive clause from a proposition node: a moment grounded in
// its relation, with agent and patient edges to the argument CONCEPTS (the moment's
// content marks past tense). Each role concept resolves to a word by the graph -- its
// firmest naming word in the language, or a participant's pronoun -- the same conceptName
// every answer uses, so the proposition's concepts are the meaning and the words are
// looked up, not pre-rendered. The slots then lay out in the language's word order (the
// verb closing a verb-final clause), the verb is conjugated and the object marked
// definite by grammar's morphology, and co-arguments on a role coordinate, the same Speak
// does. A given referent's pronoun is a discourse choice carried by the meaning, not the
// proposition's structure, so it stays with Speak's Meaning render for now.
type nlSurface struct {
	lang string
}

func (s nlSurface) construct(d *Daemon, g codeGraph, node uint64, _ any) (construction, bool) {
	lang := s.lang
	if lang == "" {
		lang = "en"
	}
	relNode := firstDest(g, node, memory.RelGrounds)
	verb := d.conceptName(relNode, g.content(relNode), lang)
	agents := d.roleWords(g, node, memory.RelAgent, lang)
	patients := d.roleWords(g, node, memory.RelPatient, lang)
	if verb == "" || len(agents) == 0 || len(patients) == 0 {
		return construction{}, false
	}
	past := g.content(node) == "past"
	coord := d.gram.Coordinator(lang)
	object := make([]string, len(patients))
	for i, p := range patients {
		object[i] = d.gram.DefiniteNP(lang, "patient", p)
	}
	agent := rendered{src: joinCoord(agents, coord)}
	verbLeaf := rendered{src: d.gram.Conjugate(lang, verb, len(agents) > 1, past, false)}
	patient := rendered{src: joinCoord(object, coord)}
	slots := []slot{{leaf: agent}, {leaf: verbLeaf}, {leaf: patient}}
	if d.gram.VerbFinal(lang) {
		slots = []slot{{leaf: agent}, {leaf: patient}, {leaf: verbLeaf}}
	}
	return construction{slots: slots, assemble: func(parts []rendered) (rendered, bool) {
		words := make([]string, len(parts))
		for i, p := range parts {
			words[i] = p.src
		}
		return rendered{src: grammar.Sentence(strings.Join(words, " "))}, true
	}}, true
}

// roleWords resolves the concepts a role's edges leave a node for to their words in the
// language, in storage order, so a role with more than one filler coordinates them. The
// resolution is conceptName, the graph's own concept-to-word lookup (its firmest naming
// word, or a participant's pronoun, falling back to the node's content when nothing names
// it), so the surface reads concepts and the words come from the graph.
func (d *Daemon) roleWords(g codeGraph, node uint64, rel, lang string) []string {
	dests := g.dests(node, rel)
	out := make([]string, 0, len(dests))
	for _, dst := range dests {
		if w := d.conceptName(dst, g.content(dst), lang); w != "" {
			out = append(out, w)
		}
	}
	return out
}

// joinCoord joins fillers with the language's coordinator, "the cat and the mouse".
func joinCoord(items []string, coord string) string {
	if coord == "" {
		return strings.Join(items, " ")
	}
	return strings.Join(items, " "+coord+" ")
}
