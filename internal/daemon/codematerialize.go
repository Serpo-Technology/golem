package daemon

import (
	"fmt"
	"strconv"
	"strings"

	"golem/internal/code"
	"golem/internal/memory"
)

// materializeCode writes a comprehended computation into the scratch graph as the
// subgraph the traverser reads, the inverse of genCode. It mirrors the read shapes: a
// literal or variable leaf, a binary primitive operation, a conditional, a map, a list
// literal, and a call of a defined function (materialized as a graph: the function's
// ordered parameters and its body). The traverser parenthesizes a compound operand faithfully,
// so a call may sit in any position and its arguments may be compound. selfName is the
// function whose body is being written, fnNodes the functions open above it: a call to
// selfName grounds in its own node (a recursive self-reference the traverser writes as a
// local function), and a call back to a different open function (mutual recursion) is
// refused, the one call shape the shadow skips.
func (d *Daemon) materializeCode(g codeWriter, c code.Computation, selfName string, fnNodes map[string]uint64) (uint64, bool) {
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
	// A map: a root grounding in the map concept, with the element name, the list ranged
	// over, and the per-element body as named edges. The body reads the element as a free
	// leaf (the loop variable), materialized in the same scope.
	if c.Map != nil {
		root := g.add("")
		g.link(root, g.add("map"), memory.RelGrounds)
		g.link(root, g.add(c.Map.Var), mapVarRel)
		if d.materializeInto(g, root, mapListRel, c.Map.List, selfName, fnNodes) &&
			d.materializeInto(g, root, mapBodyRel, c.Map.Body, selfName, fnNodes) {
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
	// A string primitive: a root grounding in the operation concept with its operands
	// (one for a slice, two for an infix join), recognized by the operation's emit
	// template rather than an operator symbol, so a unary slice is not mis-read as a
	// call by len(c.Args) == 2.
	if op, ok := d.opByName(c.Op); ok && op.Emit != "" {
		if len(c.Args) != len(op.In) {
			return 0, false
		}
		root := g.add("")
		g.link(root, g.add(c.Op), memory.RelGrounds)
		rels := []string{operand0Rel, operand1Rel}
		for i, a := range c.Args {
			if !d.materializeInto(g, root, rels[i], a, selfName, fnNodes) {
				return 0, false
			}
		}
		return root, true
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
// Otherwise the function is built as a graph (materializeFnNode) and the call grounds in it
// with its ordered arguments, which stay in the caller's scope.
func (d *Daemon) materializeCall(g codeWriter, c code.Computation, selfName string, fnNodes map[string]uint64) (uint64, bool) {
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
	if !ok || len(op.Params) != len(c.Args) {
		return 0, false
	}
	fn, ok := d.materializeFnNode(g, op, fnNodes)
	if !ok {
		return 0, false
	}
	call := g.add("")
	g.link(call, fn, memory.RelGrounds)
	if !d.materializeSlots(g, call, elemRel, c.Args, selfName, fnNodes) {
		return 0, false
	}
	return call, true
}

// materializeFnNode builds a defined function as a graph and returns the node a call
// grounds in: its ordered parameter names and its body, the body written with the function
// as the current self so a self-call inside it becomes the recursive self-reference. An
// imperative function (a synthesized loop) is built from its parsed Raw structure instead.
// The function is the same however the call reached it, so this is shared by a comprehended
// call (from a Computation) and a comprehension graph built straight from the grammar.
func (d *Daemon) materializeFnNode(g codeWriter, op code.Op, fnNodes map[string]uint64) (uint64, bool) {
	if op.Raw != "" {
		return d.materializeRawFnNode(g, op, fnNodes)
	}
	if op.Body == nil {
		return 0, false
	}
	fn := g.add(op.Name)
	fnNodes[op.Name] = fn
	defer delete(fnNodes, op.Name)
	for i, p := range op.Params {
		g.attachSlot(fn, paramRel, i, g.add(p))
	}
	if !d.materializeInto(g, fn, bodyRel, *op.Body, op.Name, fnNodes) {
		return 0, false
	}
	return fn, true
}

// materializeRawFnNode builds the function node for an imperative function (a synthesized
// reduction or list loop): its single list parameter and a body recovered from the Raw
// statements, a value-fold (the accumulator, the start, the step, the guard) by parseFold
// or a list loop (a map's transform, a filter's comparison and constant) by parseListLoop.
// The traverser wraps a call to it in the applied func literal the typed render gives a Raw
// call. It refuses a shape the parsers do not recognize, so the call dispatch falls back to
// the typed render rather than mis-building.
func (d *Daemon) materializeRawFnNode(g codeWriter, op code.Op, fnNodes map[string]uint64) (uint64, bool) {
	if len(op.Params) != 1 {
		return 0, false
	}
	param := op.Params[0]
	var body uint64
	var bok bool
	switch op.Ret {
	case "int":
		body, bok = d.materializeFold(g, op.Raw, param, fnNodes)
	case "list":
		body, bok = d.materializeListLoop(g, op.Raw, param, fnNodes)
	}
	if !bok {
		return 0, false
	}
	fn := g.add(op.Name)
	g.attachSlot(fn, paramRel, 0, g.add(param))
	g.link(fn, body, bodyRel)
	return fn, true
}

// materializeFold builds a value-fold body node from the synthesizer's Raw statements:
// the accumulator, the start (a const or the first element), the step expression, and
// the guard, recovered by parseFold. It returns the fold node and false when the Raw
// is not a fold parseFold recognizes.
func (d *Daemon) materializeFold(g codeWriter, raw, param string, fnNodes map[string]uint64) (uint64, bool) {
	acc, first, initLit, step, guard, ok := d.parseFold(raw, param)
	if !ok {
		return 0, false
	}
	fold := g.add("")
	g.link(fold, g.add("fold"), memory.RelGrounds)
	g.link(fold, g.add(acc), foldAccRel)
	g.link(fold, g.add(param), foldParamRel)
	if first {
		g.link(fold, g.add(""), foldFirstRel)
	} else {
		g.link(fold, g.add(initLit), foldInitRel)
	}
	if guard != "" {
		g.link(fold, g.add(guard), foldGuardRel)
	}
	if !d.materializeInto(g, fold, foldStepRel, step, "", fnNodes) {
		return 0, false
	}
	return fold, true
}

// materializeListLoop builds a list-returning loop body node from the synthesizer's
// Raw statements: a map carries its per-element transform, a filter its comparison and
// constant, recovered by parseListLoop. It returns the list-loop node and false when
// the Raw is not a map or filter parseListLoop recognizes.
func (d *Daemon) materializeListLoop(g codeWriter, raw, param string, fnNodes map[string]uint64) (uint64, bool) {
	filter, cmp, c, step, ok := d.parseListLoop(raw, param)
	if !ok {
		return 0, false
	}
	loop := g.add("")
	g.link(loop, g.add("listloop"), memory.RelGrounds)
	g.link(loop, g.add(param), listParamRel)
	if filter {
		g.link(loop, g.add(cmp), listCmpRel)
		g.link(loop, g.add(c), listConstRel)
		return loop, true
	}
	if !d.materializeInto(g, loop, listStepRel, step, "", fnNodes) {
		return 0, false
	}
	return loop, true
}

// parseFold recovers a value-fold from the synthesizer's Raw statements, the inverse
// of the engine's foldBody: the accumulator name, whether the start is the first
// element, the constant start otherwise, the per-element step expression (read by the
// engine's own reader), and the guard the step applies under. It reads only the closed
// set of shapes foldBody emits -- a bare "return <param>[0]" for the unchanging
// first-element fold, or "<acc> := <start>; for _, x := range <param> { <assign> };
// return <acc>" with a plain or guarded assign -- and returns false for anything else,
// so an unrecognized loop declines to the typed render rather than being misread.
func (d *Daemon) parseFold(raw, param string) (acc string, first bool, initLit string, step code.Computation, guard string, ok bool) {
	if !strings.Contains(raw, "; for _, x := range ") {
		if rest, found := strings.CutPrefix(raw, "return "); found && rest == param+"[0]" {
			return "a", true, "", code.Computation{Var: "a"}, "", true
		}
		return "", false, "", code.Computation{}, "", false
	}
	head, rest, found := strings.Cut(raw, "; for _, x := range "+param+" { ")
	if !found {
		return "", false, "", code.Computation{}, "", false
	}
	acc, start, found := strings.Cut(head, " := ")
	if !found || acc == "" {
		return "", false, "", code.Computation{}, "", false
	}
	assign, found := strings.CutSuffix(rest, " }; return "+acc)
	if !found {
		return "", false, "", code.Computation{}, "", false
	}
	if start == param+"[0]" {
		first = true
	} else {
		initLit = start
	}
	stepStr := ""
	if inner, isGuarded := strings.CutPrefix(assign, "if x "); isGuarded {
		g, body, gok := strings.Cut(inner, " "+acc+" { ")
		if !gok {
			return "", false, "", code.Computation{}, "", false
		}
		guard = g
		body, found = strings.CutSuffix(body, " }")
		if !found {
			return "", false, "", code.Computation{}, "", false
		}
		stepStr, found = strings.CutPrefix(body, acc+" = ")
	} else {
		stepStr, found = strings.CutPrefix(assign, acc+" = ")
	}
	if !found {
		return "", false, "", code.Computation{}, "", false
	}
	step, ok = d.code.Comprehend(stepStr)
	if !ok {
		return "", false, "", code.Computation{}, "", false
	}
	return acc, first, initLit, step, guard, true
}

// parseListLoop recovers a list-returning loop from the synthesizer's Raw statements,
// the inverse of mapBody/filterBody: a map appends a per-element transform, a filter
// appends each element a comparison keeps. It strips the fixed b/x skeleton and reads
// the middle as either a guarded append of the element (the filter's comparison and
// integer constant) or an append of a transform expression (the map step, read by the
// engine's reader). It returns false for anything else, so an unrecognized loop
// declines to the typed render rather than being misread.
func (d *Daemon) parseListLoop(raw, param string) (filter bool, cmp, c string, step code.Computation, ok bool) {
	inner, found := strings.CutPrefix(raw, "b := []int{}; for _, x := range "+param+" { ")
	if !found {
		return false, "", "", code.Computation{}, false
	}
	body, found := strings.CutSuffix(inner, " }; return b")
	if !found {
		return false, "", "", code.Computation{}, false
	}
	if test, isFilter := strings.CutPrefix(body, "if x "); isFilter {
		test, found = strings.CutSuffix(test, " { b = append(b, x) }")
		if !found {
			return false, "", "", code.Computation{}, false
		}
		cmp, c, found = strings.Cut(test, " ")
		if !found {
			return false, "", "", code.Computation{}, false
		}
		if _, err := strconv.Atoi(c); err != nil {
			return false, "", "", code.Computation{}, false
		}
		return true, cmp, c, code.Computation{}, true
	}
	stepStr, found := strings.CutPrefix(body, "b = append(b, ")
	if !found {
		return false, "", "", code.Computation{}, false
	}
	stepStr, found = strings.CutSuffix(stepStr, ")")
	if !found {
		return false, "", "", code.Computation{}, false
	}
	step, ok = d.code.Comprehend(stepStr)
	if !ok {
		return false, "", "", code.Computation{}, false
	}
	return false, "", "", step, true
}

// materializeInto materializes c and links it from parent under rel, false on failure.
func (d *Daemon) materializeInto(g codeWriter, parent uint64, rel string, c code.Computation, selfName string, fnNodes map[string]uint64) bool {
	id, ok := d.materializeCode(g, c, selfName, fnNodes)
	if !ok {
		return false
	}
	g.link(parent, id, rel)
	return true
}

// materializeSlots materializes each child and attaches it to parent as an ordered slot.
func (d *Daemon) materializeSlots(g codeWriter, parent uint64, rel string, children []code.Computation, selfName string, fnNodes map[string]uint64) bool {
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
// when a shape does not materialize (a Raw loop the parsers do not recognize), so the
// caller falls back to the typed render.
func (d *Daemon) traverseCode(comp code.Computation) (string, bool) {
	g := newScratchGraph()
	root, ok := d.materializeCode(g, comp, "", map[string]uint64{})
	if !ok {
		return "", false
	}
	r, rok := d.traverse(goSurface{}, g, root, nil)
	return r.src, rok
}

// generateCode produces the runnable Go for a comprehended computation by graph traversal.
// When comprehension built the substrate graph straight from the grounded grammar (g is
// non-nil), that graph is walked for the source, so the meaning comprehension landed in the
// graph drives generation with no re-materialize. Otherwise the computation is materialized
// onto a subgraph and walked, the path a surface the grammar build does not cover takes (the
// operator or raw-Go expression, or a told map). The typed render stays only a cross-check
// and a fallback: the walk is used whenever it succeeds, a render that disagrees is logged as
// a tripwire while the walk still serves, and the render serves a shape no walk builds yet.
// It returns false when neither path produces source, so the caller runs the surface it holds.
func (d *Daemon) generateCode(comp code.Computation, g *scratchGraph, root uint64) (string, bool) {
	if g != nil {
		if r, ok := d.traverse(goSurface{}, g, root, nil); ok {
			if typed, tok := d.code.ToCode(comp); tok && r.src != typed {
				fmt.Fprintf(d.out, "comprehension graph mismatch: grammar %q vs render %q\n", r.src, typed)
			}
			return r.src, true
		}
	}
	walked, wok := d.traverseCode(comp)
	generated, gok := d.code.ToCode(comp)
	switch {
	case wok:
		if gok && walked != generated {
			fmt.Fprintf(d.out, "codegraph mismatch: walked %q vs render %q\n", walked, generated)
		}
		return walked, true
	case gok:
		return generated, true
	}
	return "", false
}

// typeChecks reports whether a comprehended computation is type-consistent. When
// comprehension built the substrate graph (g is non-nil), the check reads the graph, so the
// type check runs over the same structure generation walks rather than the Computation. The
// typed check stays the cross-check, logged on a disagreement; a surface with no graph (the
// operator or raw-Go form, a told map) is checked the prior way.
func (d *Daemon) typeChecks(comp code.Computation, g *scratchGraph, root uint64) bool {
	if g == nil {
		return d.code.TypeCheck(comp)
	}
	ok := d.typeCheckGraph(g, root)
	if typed := d.code.TypeCheck(comp); typed != ok {
		fmt.Fprintf(d.out, "typecheck graph mismatch: graph %v vs render %v\n", ok, typed)
	}
	return ok
}

// typeCheckGraph reports whether the code-meaning subgraph is type-consistent, the inverse
// reading of the type the engine's typeOf computes over a Computation. It walks the shapes a
// comprehension graph holds at the top level (a literal leaf, a list, a binary primitive, a
// call) and reads a called function's parameter and result types from the operation
// inventory, so a body it does not walk still constrains the call.
func (d *Daemon) typeCheckGraph(g *scratchGraph, root uint64) bool {
	_, ok := d.typeOfGraph(g, root)
	return ok
}

// typeOfGraph computes a node's type, "int" or "list", false when it is ill-typed. A
// top-level leaf is a literal number (a parameter reference lives only in a body, which the
// call case does not walk). A list takes integer elements. A binary primitive takes two
// integers. A call's arguments must match the operation's parameter types, and it yields the
// operation's return type, both read from the inventory rather than the body.
func (d *Daemon) typeOfGraph(g codeGraph, node uint64) (string, bool) {
	grounds := firstDest(g, node, memory.RelGrounds)
	if grounds == 0 {
		content := g.content(node)
		if content == "" {
			return "", false
		}
		if content[0] == '"' {
			return "string", true // a quoted literal leaf
		}
		return "int", true
	}
	name := g.content(grounds)
	if name == code.ListWord {
		elems, ok := d.orderedSlots(g, node, elemRel)
		if !ok {
			return "", false
		}
		for _, el := range elems {
			if t, tok := d.typeOfGraph(g, el); !tok || t != "int" {
				return "", false
			}
		}
		return "list", true
	}
	op, ok := d.opByName(name)
	if !ok {
		return "", false
	}
	if op.Emit != "" { // a string primitive: operand0/operand1 typed by In, result Out
		rels := []string{operand0Rel, operand1Rel}
		if len(op.In) > len(rels) {
			return "", false
		}
		for i, want := range op.In {
			t, tok := d.typeOfGraph(g, firstDest(g, node, rels[i]))
			if !tok || t != want {
				return "", false
			}
		}
		return op.Out, true
	}
	if d.opSymbol(name) != "" {
		l, lok := d.typeOfGraph(g, firstDest(g, node, operand0Rel))
		r, rok := d.typeOfGraph(g, firstDest(g, node, operand1Rel))
		if !lok || !rok || l != "int" || r != "int" {
			return "", false
		}
		return "int", true
	}
	args, ok := d.orderedSlots(g, node, elemRel)
	if !ok || len(args) != len(op.Params) {
		return "", false
	}
	for i, a := range args {
		expect := "int"
		if i < len(op.ParamTypes) && op.ParamTypes[i] != "" {
			expect = op.ParamTypes[i]
		}
		if t, tok := d.typeOfGraph(g, a); !tok || t != expect {
			return "", false
		}
	}
	return d.code.ReturnType(name)
}
