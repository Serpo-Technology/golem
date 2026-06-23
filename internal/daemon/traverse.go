package daemon

import (
	"strings"

	"golem/internal/code"
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
	if firstDest(g, node, mapBodyRel) != 0 {
		elem := g.content(firstDest(g, node, mapVarRel))
		list := firstDest(g, node, mapListRel)
		body := firstDest(g, node, mapBodyRel)
		if elem == "" || list == 0 || body == 0 {
			return construction{}, false
		}
		return construction{
			slots: []slot{{child: list, ctx: ctx}, {child: body, ctx: ctx}},
			assemble: func(parts []rendered) (rendered, bool) {
				return rendered{src: "(func() []int { out := []int{}; for _, " + elem +
					" := range " + parts[0].src + " { out = append(out, " + parts[1].src +
					") }; return out }())"}, true
			},
		}, true
	}
	if firstDest(g, node, foldStepRel) != 0 {
		acc := g.content(firstDest(g, node, foldAccRel))
		param := g.content(firstDest(g, node, foldParamRel))
		guard := g.content(firstDest(g, node, foldGuardRel))
		step := firstDest(g, node, foldStepRel)
		if acc == "" || param == "" || step == 0 {
			return construction{}, false
		}
		start := g.content(firstDest(g, node, foldInitRel))
		if firstDest(g, node, foldFirstRel) != 0 {
			start = param + "[0]"
		}
		return construction{
			slots: []slot{{child: step, ctx: ctx}},
			assemble: func(parts []rendered) (rendered, bool) {
				return rendered{src: foldSource(acc, param, start, guard, parts[0].src)}, true
			},
		}, true
	}
	if firstDest(g, node, listParamRel) != 0 {
		param := g.content(firstDest(g, node, listParamRel))
		if param == "" {
			return construction{}, false
		}
		if step := firstDest(g, node, listStepRel); step != 0 {
			return construction{slots: []slot{{child: step, ctx: ctx}}, assemble: func(parts []rendered) (rendered, bool) {
				return rendered{src: listMapSource(param, parts[0].src)}, true
			}}, true
		}
		cmp := g.content(firstDest(g, node, listCmpRel))
		c := g.content(firstDest(g, node, listConstRel))
		if cmp == "" || c == "" {
			return construction{}, false
		}
		return construction{assemble: func([]rendered) (rendered, bool) {
			return rendered{src: listFilterSource(param, cmp, c)}, true
		}}, true
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
	if op, ok := d.opByName(g.content(fn)); ok && op.Raw != "" {
		return d.goRawCall(g, node, fn, op, ctx)
	}
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

// goRawCall renders a call to an imperative function (a synthesized loop), which
// cannot inline: its body statements are wrapped in the func literal applied to the
// arguments, the form the typed render gives a Raw call. The arguments render in the
// caller's context; the body reads its parameter and accumulator as free names, so it
// is walked with none.
func (d *Daemon) goRawCall(g codeGraph, node, fn uint64, op code.Op, ctx any) (construction, bool) {
	body := firstDest(g, fn, bodyRel)
	args, aok := d.orderedSlots(g, node, elemRel)
	if body == 0 || !aok {
		return construction{}, false
	}
	return construction{slots: childSlots(args, ctx), assemble: func(parts []rendered) (rendered, bool) {
		r, rok := d.traverse(goSurface{}, g, body, nil)
		if !rok {
			return rendered{}, false
		}
		return rendered{src: rawCallSource(op, r.src, joinRendered(parts))}, true
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

// nlSurface renders a transitive clause from a stored proposition: a moment its relation
// grounds into, with agent and patient edges to the argument CONCEPTS. The relation is
// read by the reverse grounds edge (a proposition stores the relation grounding into the
// telling moment), and tense is supplied as a render parameter the way renderFacts
// supplies it. Each role concept resolves to a word by the graph -- its firmest naming
// word in the language, or a participant's pronoun -- the same conceptName every answer
// uses, so the proposition's concepts are the meaning and the words are looked up. The
// slots then lay out in the language's word order (the verb closing a verb-final clause),
// the verb is conjugated and the arguments marked definite by grammar's morphology (the
// answer-render marking renderFacts uses), and co-arguments on a role coordinate, the same
// Speak does. A given referent's pronoun is a discourse choice carried across clauses, not
// the single proposition's structure, so it stays with the answer path for now.
type nlSurface struct {
	lang string
	past bool
	// given is the role of a referent already in play, spoken as the language's pronoun
	// (the anaphora renderFacts does across clauses); empty when both roles are named.
	given string
}

func (s nlSurface) construct(d *Daemon, g codeGraph, node uint64, _ any) (construction, bool) {
	lang := s.lang
	if lang == "" {
		lang = "en"
	}
	relID := d.groundedRelation(node)
	verb := d.conceptName(relID, g.content(relID), lang)
	agents := d.roleWords(g, node, memory.RelAgent, lang)
	patients := d.roleWords(g, node, memory.RelPatient, lang)
	if verb == "" || len(agents) == 0 || len(patients) == 0 {
		return construction{}, false
	}
	coord := d.gram.Coordinator(lang)
	agent := rendered{src: d.roleLeaf(lang, "agent", agents, s.given, coord)}
	verbLeaf := rendered{src: d.gram.Conjugate(lang, verb, len(agents) > 1, s.past, false)}
	patient := rendered{src: d.roleLeaf(lang, "patient", patients, s.given, coord)}
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

// roleLeaf renders a role's filler: the in-play referent's pronoun when the role is the
// given one, else its concept words marked definite and coordinated, the same choice
// renderFiller makes from a Term's flags.
func (d *Daemon) roleLeaf(lang, role string, words []string, given, coord string) string {
	if role == given {
		return d.gram.Pronoun(lang, role)
	}
	marked := make([]string, len(words))
	for i, w := range words {
		marked[i] = d.gram.DefiniteNP(lang, role, w)
	}
	return joinCoord(marked, coord)
}

// groundedRelation is the relation concept a proposition's moment grounds: a proposition
// stores the relation grounding into the moment (chase grounds in the telling), so the
// relation is the concept whose grounds edge points at the moment, read in reverse.
func (d *Daemon) groundedRelation(moment uint64) uint64 {
	edges, err := d.store.LinksToRel([]uint64{moment}, memory.RelGrounds)
	if err != nil || len(edges) == 0 {
		return 0
	}
	return edges[0].From
}

// speakProposition renders a stored proposition to a clause by walking it through the
// shared traverser's language surface, the NL counterpart of traverseCode. Tense is
// supplied the way renderFacts supplies it. It shadows the answer path's per-clause render
// today; the switch onto it is a later, deliberate step.
func (d *Daemon) speakProposition(momentID uint64, lang string, past bool, given string) (string, bool) {
	r, ok := d.traverse(nlSurface{lang: lang, past: past, given: given}, d.storeView(), momentID, nil)
	return r.src, ok
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
