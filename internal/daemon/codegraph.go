package daemon

import (
	"sort"
	"strconv"
	"strings"

	"golem/internal/code"
	"golem/internal/memory"
)

// Code generation by graph traversal: a prototype of the north-star where a
// program's meaning is a SUBGRAPH the engine walks to emit source, rather than a
// Go type it renders. A node is one of five shapes: an OPERATION (grounded in its
// operation concept, with ordered operand0/operand1 edges) rendered as an infix
// expression; a CONDITIONAL (the comparison operator as its content, with named
// condLeft/condRight/condThen/condElse edges) rendered as the branch function
// literal; a LIST (grounded in the list concept, its elements held in ordered slot
// nodes) rendered as a composite literal; a CALL (grounded in a function concept, its
// arguments held in ordered slot nodes) inlined by walking the function's body; or a
// LEAF read by its content, a literal or a variable reference alike, since both emit
// their token. Operands recurse and are parenthesized so precedence is kept. It is
// grounded the same way every construction is, by running the source it emits, and
// checked against the typed render. This sits alongside the typed render; it proves
// the pattern over operations, variables, conditionals, ordered children, and calls.
//
// Ordered children (a list's elements, a call's arguments, a function's parameters)
// bind each child to its position through a distinct slot node: root --elem--> slot,
// slot --rank--> ordinal, slot --holds--> value. The slot is the per-occurrence
// binding, the ordinal the position the walk sorts by, the value what sits there.
// Distinct slots let one value node sit at more than one position, so a repeat
// ([]int{5, 5}) places the same item twice without the (from, to, rel) primary key
// colliding. This is positional coding: each item bound to a position, which a plain
// chain cannot do.
//
// A function is a graph: its concept carries a body subgraph and ordered parameter
// slots. A call inlines by traversal: walk the body under a binding of parameter names
// to the call's rendered arguments, the graph counterpart of the typed render's inline
// + substitute. The inlining covers a non-recursive call with simple arguments (a
// literal or a variable); a compound argument would need parenthesizing where it lands
// in an operand position, and a recursive call the self-call form rather than inlining.
const (
	operand0Rel = "operand0"
	operand1Rel = "operand1"

	// A conditional's parts are named roles, not ordered operands: the comparison's
	// two sides and the two branches. The comparison operator is the node's content.
	condLeftRel  = "condLeft"
	condRightRel = "condRight"
	condThenRel  = "condThen"
	condElseRel  = "condElse"

	// A map transforms a list into a list element by element: named roles like a
	// conditional's, the element variable, the list ranged over, and the per-element
	// body that reads the element as a free variable.
	mapVarRel  = "mapVar"
	mapListRel = "mapList"
	mapBodyRel = "mapBody"

	// A value-fold reduces a list to a value through an accumulator: named roles, the
	// accumulator name, the list param ranged over, the start (a const leaf, or
	// foldFirst marking the first element), the per-element step expression over the
	// accumulator and x, and the guard the step applies under (absent when plain).
	foldAccRel   = "foldAcc"
	foldParamRel = "foldParam"
	foldInitRel  = "foldInit"
	foldFirstRel = "foldFirst"
	foldStepRel  = "foldStep"
	foldGuardRel = "foldGuard"

	// A list-returning loop builds a result list from a list element by element: a
	// map appends a transform of each element, a filter appends the elements a
	// comparison keeps. Named roles, the list param ranged over, and either the map
	// step or the filter's comparison and constant. Its own shape, distinct from the
	// told map (which renders the out/IIFE form).
	listParamRel = "listParam"
	listStepRel  = "listStep"
	listCmpRel   = "listCmp"
	listConstRel = "listConst"

	// Ordered children bind each child to its position through a slot node, so order
	// is meaning and a value can repeat: root --elem--> slot, slot --rank--> ordinal,
	// slot --holds--> value. A list literal uses them; an n-ary call will reuse them.
	elemRel  = "elem"
	rankRel  = "rank"
	holdsRel = "holds"

	// A function is a graph too: its concept carries a body subgraph and its ordered
	// parameters (the slot walk again). A call grounds in the function concept and holds
	// its arguments as ordered children, then inlines by walking the body bound to them.
	paramRel = "param"
	bodyRel  = "body"
)

// boundArg is a rendered argument bound to a parameter: its source and whether that
// source is a bare infix expression. The flag rides with the binding so that when a
// body uses the parameter as an operand, a compound argument is parenthesized there the
// way the typed render parenthesizes a substituted operation.
type boundArg struct {
	src      string
	compound bool
}

// genCodeFromGraph walks a code-meaning subgraph in the persistent store from its root
// and emits the Go expression it means. It is the entry with no names in scope and no
// function being emitted; genCode carries the parameter binding a call inlines under,
// the graph it reads, and self, the function node a recursive body is being written
// inside, so the one traverser serves a stored subgraph or a transient one alike.
func (d *Daemon) genCodeFromGraph(nodeID uint64) (string, bool) {
	src, _, ok := d.genCode(d.storeView(), nodeID, nil, 0)
	return src, ok
}

// genCode walks a code-meaning subgraph in g under a binding of names to already-
// rendered arguments (empty at the top level) and self, the function node a recursive
// body is being written inside (0 when none, so a call to self is a plain self-call). It
// returns the source, whether that source is a bare infix expression (compound) an
// operand position must parenthesize, and ok. A node is a conditional (named branches),
// an ordered-children node (a list literal or a call, told apart by what it grounds
// in), an operation (ordered operands), or a leaf. A leaf emits its bound argument when
// the binding holds its content -- carrying that argument's own compound flag, so a
// compound argument lands parenthesized where the body uses it -- else the content
// itself, a literal or a free variable.
func (d *Daemon) genCode(g codeGraph, nodeID uint64, env map[string]boundArg, self uint64) (string, bool, bool) {
	if firstDest(g, nodeID, condThenRel) != 0 {
		s, ok := d.genConditional(g, nodeID, env, self)
		return s, false, ok // a conditional renders as a parenthesized literal, a primary
	}
	if firstDest(g, nodeID, mapBodyRel) != 0 {
		s, ok := d.genMap(g, nodeID, env, self)
		return s, false, ok // a map renders as a parenthesized IIFE, a primary
	}
	if firstDest(g, nodeID, foldStepRel) != 0 {
		s, ok := d.genFold(g, nodeID, env, self)
		return s, false, ok // a fold renders as the loop statements its call wrapper holds
	}
	if firstDest(g, nodeID, listParamRel) != 0 {
		s, ok := d.genListLoop(g, nodeID, env, self)
		return s, false, ok // a list-returning loop renders as its statements too
	}
	if firstDest(g, nodeID, elemRel) != 0 {
		return d.genOrderedNode(g, nodeID, env, self)
	}
	left := firstDest(g, nodeID, operand0Rel)
	right := firstDest(g, nodeID, operand1Rel)
	if left == 0 && right == 0 {
		c := g.content(nodeID)
		if c == "" {
			return "", false, false
		}
		if bound, ok := env[c]; ok {
			return bound.src, bound.compound, true
		}
		return c, false, true // a literal or a free variable is a single token
	}
	sym := d.opSymbol(g.content(firstDest(g, nodeID, memory.RelGrounds)))
	l, lok := d.genOperand(g, left, env, self)
	r, rok := d.genOperand(g, right, env, self)
	if sym == "" || !lok || !rok {
		return "", false, false
	}
	return l + " " + sym + " " + r, true, true // an infix expression is compound
}

// genOperand renders an operand under the binding, wrapping a compound (bare infix)
// source in parentheses so the emitted expression keeps its grouping whatever the
// operators' precedence. The compound flag comes from the walk, so an inlined call or a
// substituted compound argument is parenthesized as faithfully as a literal operation.
func (d *Daemon) genOperand(g codeGraph, nodeID uint64, env map[string]boundArg, self uint64) (string, bool) {
	s, compound, ok := d.genCode(g, nodeID, env, self)
	if !ok {
		return "", false
	}
	if compound {
		return "(" + s + ")", true
	}
	return s, true
}

// genConditional renders a conditional node -- a comparison choosing between two
// branches -- as the parenthesized function literal the typed render uses, so a
// choice composes as an expression and runs (a bare leading function literal would
// not). The comparison operator is the node's content; the sides go through genOperand
// (so a compound side is parenthesized) and the branches render bare, the same shape
// the typed render takes.
func (d *Daemon) genConditional(g codeGraph, nodeID uint64, env map[string]boundArg, self uint64) (string, bool) {
	cmp := g.content(nodeID)
	if cmp != ">" && cmp != "<" && cmp != "==" {
		return "", false
	}
	left, lok := d.genOperand(g, firstDest(g, nodeID, condLeftRel), env, self)
	right, rok := d.genOperand(g, firstDest(g, nodeID, condRightRel), env, self)
	then, _, tok := d.genCode(g, firstDest(g, nodeID, condThenRel), env, self)
	els, _, eok := d.genCode(g, firstDest(g, nodeID, condElseRel), env, self)
	if !lok || !rok || !tok || !eok {
		return "", false
	}
	return "(func() int { if " + left + " " + cmp + " " + right +
		" { return " + then + " }; return " + els + " }())", true
}

// genMap renders a map node -- a transform of a list into a list element by element
// -- as the function literal the typed render uses, so it composes as an expression
// and runs. The element variable is the loop variable, the list and the body render
// bare (the body reads the element as a free variable), the same shape the typed
// render takes.
func (d *Daemon) genMap(g codeGraph, nodeID uint64, env map[string]boundArg, self uint64) (string, bool) {
	elem := g.content(firstDest(g, nodeID, mapVarRel))
	list, _, lok := d.genCode(g, firstDest(g, nodeID, mapListRel), env, self)
	body, _, bok := d.genCode(g, firstDest(g, nodeID, mapBodyRel), env, self)
	if elem == "" || !lok || !bok {
		return "", false
	}
	return "(func() []int { out := []int{}; for _, " + elem + " := range " + list +
		" { out = append(out, " + body + ") }; return out }())", true
}

// genFold renders a value-fold node -- a reduction of a list to a value through an
// accumulator -- as the loop statements the synthesizer's foldBody emits, regenerated
// from the structure so the walked source matches the stored Raw byte for byte. The
// step is rendered bare (it reads the accumulator and x as free variables); an
// unchanging accumulator from the first element collapses to a plain return.
func (d *Daemon) genFold(g codeGraph, nodeID uint64, env map[string]boundArg, self uint64) (string, bool) {
	acc := g.content(firstDest(g, nodeID, foldAccRel))
	param := g.content(firstDest(g, nodeID, foldParamRel))
	guard := g.content(firstDest(g, nodeID, foldGuardRel))
	step, _, sok := d.genCode(g, firstDest(g, nodeID, foldStepRel), env, self)
	if acc == "" || param == "" || !sok {
		return "", false
	}
	start := g.content(firstDest(g, nodeID, foldInitRel))
	if firstDest(g, nodeID, foldFirstRel) != 0 {
		start = param + "[0]"
	}
	return foldSource(acc, param, start, guard, step), true
}

// foldSource writes a value-fold's loop statements, the one place the foldBody shape
// is regenerated so the graph walk and the typed render agree to the character: an
// unchanging accumulator from the start is just a return; a guarded step assigns only
// when x compares to the accumulator; otherwise the step assigns each pass.
func foldSource(acc, param, start, guard, step string) string {
	if guard == "" && step == acc {
		return "return " + start
	}
	assign := acc + " = " + step
	if guard != "" {
		assign = "if x " + guard + " " + acc + " { " + assign + " }"
	}
	return acc + " := " + start + "; for _, x := range " + param + " { " + assign + " }; return " + acc
}

// rawCallSource wraps an imperative function's body statements in the func literal
// applied to its arguments, the form the typed render gives a Raw call so the
// statements run and yield their value.
func rawCallSource(op code.Op, body, args string) string {
	return "(func(" + sigParams(op) + ") " + goCodeType(op.Ret) + " { " + body + " })(" + args + ")"
}

// genListLoop renders a list-returning loop node -- a synthesized map or filter -- as
// the loop statements its call wrapper holds, regenerated from the structure so the
// walked source matches the stored Raw. A map appends a transform of each element; a
// filter appends each element a comparison keeps.
func (d *Daemon) genListLoop(g codeGraph, nodeID uint64, env map[string]boundArg, self uint64) (string, bool) {
	param := g.content(firstDest(g, nodeID, listParamRel))
	if param == "" {
		return "", false
	}
	if step := firstDest(g, nodeID, listStepRel); step != 0 {
		s, _, ok := d.genCode(g, step, env, self)
		if !ok {
			return "", false
		}
		return listMapSource(param, s), true
	}
	cmp := g.content(firstDest(g, nodeID, listCmpRel))
	c := g.content(firstDest(g, nodeID, listConstRel))
	if cmp == "" || c == "" {
		return "", false
	}
	return listFilterSource(param, cmp, c), true
}

// listMapSource and listFilterSource write a list-returning loop's statements, the one
// place the mapLoop and filterBody shapes are regenerated so the graph walk and the
// typed render agree to the character: a map appends the per-element transform, a
// filter appends each element the comparison keeps.
func listMapSource(param, step string) string {
	return "b := []int{}; for _, x := range " + param + " { b = append(b, " + step + ") }; return b"
}

func listFilterSource(param, cmp, c string) string {
	return "b := []int{}; for _, x := range " + param + " { if x " + cmp + " " + c + " { b = append(b, x) } }; return b"
}

// genOrderedNode renders a node whose children are ordered -- a list literal or a
// call -- told apart by what it grounds in. The list concept gives a composite
// literal (a primary); a function concept (one carrying a body subgraph) gives a call.
func (d *Daemon) genOrderedNode(g codeGraph, nodeID uint64, env map[string]boundArg, self uint64) (string, bool, bool) {
	fn := firstDest(g, nodeID, memory.RelGrounds)
	if g.content(fn) == "list" {
		elems, ok := d.orderedChildren(g, nodeID, elemRel, env, self)
		if !ok {
			return "", false, false
		}
		parts := make([]string, len(elems))
		for i, e := range elems {
			parts[i] = e.src
		}
		return "[]int{" + strings.Join(parts, ", ") + "}", false, true
	}
	if firstDest(g, fn, bodyRel) != 0 {
		return d.genCall(g, nodeID, fn, env, self)
	}
	return "", false, false
}

// genCall renders a call. A call to the function currently being written (self) is a
// plain call to its recursive local variable. A call to a recursive function is written
// as the local-function form (genRecursive); a second recursive function reached while
// one is already being written is out of this step's scope and refused. Any other call
// is inlined: its parameters bound to the rendered arguments, its body walked in place,
// each argument keeping its compound flag so a compound one parenthesizes where used.
func (d *Daemon) genCall(g codeGraph, nodeID, fn uint64, env map[string]boundArg, self uint64) (string, bool, bool) {
	if fn == self {
		args, ok := d.orderedChildren(g, nodeID, elemRel, env, self)
		if !ok {
			return "", false, false
		}
		return g.content(fn) + "(" + joinSrc(args) + ")", false, true
	}
	// An imperative function (a synthesized loop) cannot inline; its body statements
	// are wrapped in the func literal applied to the arguments, the form the typed
	// render gives a Raw call. The body reads its parameter and accumulator as free
	// names, so it is walked with no binding.
	if op, ok := d.opByName(g.content(fn)); ok && op.Raw != "" {
		body := firstDest(g, fn, bodyRel)
		stmts, _, sok := d.genCode(g, body, nil, 0)
		args, aok := d.orderedChildren(g, nodeID, elemRel, env, self)
		if body == 0 || !sok || !aok {
			return "", false, false
		}
		return rawCallSource(op, stmts, joinSrc(args)), false, true
	}
	if d.isRecursive(g, fn) {
		if self != 0 {
			return "", false, false // a second recursive function while one is open
		}
		return d.genRecursive(g, nodeID, fn, env)
	}
	body := firstDest(g, fn, bodyRel)
	if body == 0 {
		return "", false, false
	}
	params, pok := d.orderedSlots(g, fn, paramRel)
	args, aok := d.orderedChildren(g, nodeID, elemRel, env, self)
	if !pok || !aok || len(params) != len(args) {
		return "", false, false
	}
	binding := make(map[string]boundArg, len(params))
	for i, p := range params {
		name := g.content(p)
		if name == "" {
			return "", false, false
		}
		binding[name] = args[i]
	}
	return d.genCode(g, body, binding, self)
}

// orderedSlots returns the held nodes of a node's ordered children in position order,
// each reached through a slot carrying its ordinal (rank) and the value it holds. The
// slot is the per-occurrence binding, so one node can be held at more than one position
// without the edge's primary key colliding. Parameters read the held nodes by name;
// arguments and list elements render them.
func (d *Daemon) orderedSlots(g codeGraph, nodeID uint64, rel string) ([]uint64, bool) {
	slots := g.dests(nodeID, rel)
	if len(slots) == 0 {
		return nil, false
	}
	type held struct {
		rank int
		id   uint64
	}
	items := make([]held, 0, len(slots))
	for _, slot := range slots {
		id := firstDest(g, slot, holdsRel)
		rank, err := strconv.Atoi(g.content(firstDest(g, slot, rankRel)))
		if id == 0 || err != nil {
			return nil, false
		}
		items = append(items, held{rank: rank, id: id})
	}
	sort.Slice(items, func(i, j int) bool { return items[i].rank < items[j].rank })
	out := make([]uint64, len(items))
	for i, it := range items {
		out[i] = it.id
	}
	return out, true
}

// orderedChildren renders a node's ordered children in position order, each held value
// walked under the binding and keeping its compound flag, so a child may be a literal,
// a variable, or a nested sub-meaning. A list's elements and a call's arguments share
// it (the list takes the sources, a call also takes the flags for operand hygiene).
func (d *Daemon) orderedChildren(g codeGraph, nodeID uint64, rel string, env map[string]boundArg, self uint64) ([]boundArg, bool) {
	ids, ok := d.orderedSlots(g, nodeID, rel)
	if !ok {
		return nil, false
	}
	out := make([]boundArg, len(ids))
	for i, id := range ids {
		src, compound, ok := d.genCode(g, id, env, self)
		if !ok {
			return nil, false
		}
		out[i] = boundArg{src: src, compound: compound}
	}
	return out, true
}

// joinSrc joins rendered children by their sources, the form a call's argument list and
// the argument list of the local-function form both take.
func joinSrc(args []boundArg) string {
	parts := make([]string, len(args))
	for i, a := range args {
		parts[i] = a.src
	}
	return strings.Join(parts, ", ")
}

// genRecursive writes a call to a recursive function as the local-function form the
// typed render uses: a function-typed local variable assigned a closure that calls
// itself through the variable, then invoked on the arguments. The body is written as
// statements with self set to the function node, so a self-call inside it is a plain
// call. The whole thing is a parenthesized IIFE, a primary that yields its value.
func (d *Daemon) genRecursive(g codeGraph, nodeID, fn uint64, env map[string]boundArg) (string, bool, bool) {
	name := g.content(fn)
	op, ok := d.opByName(name)
	body := firstDest(g, fn, bodyRel)
	if !ok || body == 0 {
		return "", false, false
	}
	stmts, sok := d.genBody(g, body, fn)
	args, aok := d.orderedChildren(g, nodeID, elemRel, env, 0)
	if !sok || !aok {
		return "", false, false
	}
	rt := goCodeType(d.inferType(g, body, paramTypeMap(op)))
	return "(func() " + rt + " { var " + name + " " + funcType(op, rt) + "; " +
		name + " = func(" + sigParams(op) + ") " + rt + " { " + stmts + " }; " +
		"return " + name + "(" + joinSrc(args) + ") }())", false, true
}

// genBody writes a recursive function's body as the statements inside its braces, with
// self the function node so a self-call is a plain call: a conditional becomes the
// branch form "if test { return then }; return else", any other body a single return.
// The parameters are in scope as themselves, so the body is walked with no binding.
func (d *Daemon) genBody(g codeGraph, body, self uint64) (string, bool) {
	if firstDest(g, body, condThenRel) != 0 {
		cmp := g.content(body)
		if cmp != ">" && cmp != "<" && cmp != "==" {
			return "", false
		}
		left, lok := d.genOperand(g, firstDest(g, body, condLeftRel), nil, self)
		right, rok := d.genOperand(g, firstDest(g, body, condRightRel), nil, self)
		then, _, tok := d.genCode(g, firstDest(g, body, condThenRel), nil, self)
		els, _, eok := d.genCode(g, firstDest(g, body, condElseRel), nil, self)
		if !lok || !rok || !tok || !eok {
			return "", false
		}
		return "if " + left + " " + cmp + " " + right + " { return " + then + " }; return " + els, true
	}
	src, _, ok := d.genCode(g, body, nil, self)
	if !ok {
		return "", false
	}
	return "return " + src, true
}

// isRecursive reports whether a function's body calls the function itself, the mark of a
// call that is written as a self-referential local function rather than inlined.
func (d *Daemon) isRecursive(g codeGraph, fn uint64) bool {
	return d.reaches(g, firstDest(g, fn, bodyRel), fn, map[uint64]bool{})
}

// reaches reports whether a node, or anything under its structural edges, grounds in
// target -- how a body is searched for a call back to its own function. It does not
// cross the grounding edge, so the search does not loop through the function it seeks.
func (d *Daemon) reaches(g codeGraph, nodeID, target uint64, seen map[uint64]bool) bool {
	if nodeID == 0 || seen[nodeID] {
		return false
	}
	seen[nodeID] = true
	if firstDest(g, nodeID, memory.RelGrounds) == target {
		return true
	}
	for _, rel := range []string{operand0Rel, operand1Rel, condLeftRel, condRightRel, condThenRel, condElseRel, mapListRel, mapBodyRel, foldStepRel, listStepRel, elemRel, holdsRel} {
		for _, dest := range g.dests(nodeID, rel) {
			if d.reaches(g, dest, target, seen) {
				return true
			}
		}
	}
	return false
}

// inferType is the value type a body evaluates to ("int" or "list"), read from the
// graph: a conditional takes its chosen branch's type, a list its own, an operation a
// number, and a leaf a number or, for a parameter reference, the parameter's type. It
// matches the engine's type for the shapes a function returns, the return type the
// recursive form is written with.
func (d *Daemon) inferType(g codeGraph, nodeID uint64, paramTypes map[string]string) string {
	if then := firstDest(g, nodeID, condThenRel); then != 0 {
		return d.inferType(g, then, paramTypes)
	}
	if firstDest(g, nodeID, mapBodyRel) != 0 {
		return "list" // a map transforms a list into a list
	}
	if firstDest(g, nodeID, elemRel) != 0 {
		if g.content(firstDest(g, nodeID, memory.RelGrounds)) == "list" {
			return "list"
		}
		return "int" // a call, taken as a number in the common case
	}
	if firstDest(g, nodeID, operand0Rel) != 0 {
		return "int" // arithmetic
	}
	if t, ok := paramTypes[g.content(nodeID)]; ok {
		return t
	}
	return "int" // a literal
}

// goCodeType is a value type's Go form, a list written as a slice.
func goCodeType(t string) string {
	if t == "list" {
		return "[]int"
	}
	return "int"
}

// paramTypeAt is a parameter's declared type, "int" when none is recorded.
func paramTypeAt(op code.Op, i int) string {
	if i < len(op.ParamTypes) && op.ParamTypes[i] != "" {
		return op.ParamTypes[i]
	}
	return "int"
}

// paramTypeMap maps each parameter name to its type, for reading a body's value type.
func paramTypeMap(op code.Op) map[string]string {
	m := make(map[string]string, len(op.Params))
	for i, p := range op.Params {
		m[p] = paramTypeAt(op, i)
	}
	return m
}

// sigParams writes a function's parameter list with Go types, "number int".
func sigParams(op code.Op) string {
	parts := make([]string, len(op.Params))
	for i, p := range op.Params {
		parts[i] = p + " " + goCodeType(paramTypeAt(op, i))
	}
	return strings.Join(parts, ", ")
}

// funcType writes a function's type, "func(int) int", for the recursive local variable.
func funcType(op code.Op, ret string) string {
	parts := make([]string, len(op.Params))
	for i := range op.Params {
		parts[i] = goCodeType(paramTypeAt(op, i))
	}
	return "func(" + strings.Join(parts, ", ") + ") " + ret
}

// opByName finds a seeded or learned operation by name, read from the code engine so
// the walk and the materializer reuse the one operation inventory.
func (d *Daemon) opByName(name string) (code.Op, bool) {
	if d.code == nil {
		return code.Op{}, false
	}
	for _, o := range d.code.Ops() {
		if o.Name == name {
			return o, true
		}
	}
	return code.Op{}, false
}

// opSymbol is the operator symbol of a primitive operation by name ("add" -> "+"),
// empty when the name is not a primitive (a defined function carries no symbol).
func (d *Daemon) opSymbol(name string) string {
	op, ok := d.opByName(name)
	if !ok {
		return ""
	}
	return op.Symbol
}

// codeGraph is the read view the traverser needs: a node's content, and the nodes a
// relation leaves it for. The persistent store and a transient scratch graph both
// satisfy it, so the traverser walks "a graph" rather than a particular store.
type codeGraph interface {
	content(id uint64) string
	dests(id uint64, rel string) []uint64
}

// firstDest is the first node a relation leaves the node for, 0 when none.
func firstDest(g codeGraph, nodeID uint64, rel string) uint64 {
	ds := g.dests(nodeID, rel)
	if len(ds) == 0 {
		return 0
	}
	return ds[0]
}

// storeGraph adapts the persistent store to the read view, reading a node's content
// and a relation's destinations from memory. d.storeView is the production graph.
type storeGraph struct{ s *memory.Store }

func (d *Daemon) storeView() storeGraph { return storeGraph{s: d.store} }

func (g storeGraph) content(id uint64) string {
	if id == 0 {
		return ""
	}
	frags, err := g.s.FragmentsByIDs([]uint64{id})
	if err != nil || len(frags) != 1 {
		return ""
	}
	return frags[0].Content
}

func (g storeGraph) dests(id uint64, rel string) []uint64 {
	edges, err := g.s.LinksFromRel([]uint64{id}, rel)
	if err != nil {
		return nil
	}
	out := make([]uint64, len(edges))
	for i, e := range edges {
		out[i] = e.To
	}
	return out
}

// scratchGraph is a transient in-memory code graph. The shadow check materializes a
// comprehended computation into one, walks it, and discards it, so the check leaves no
// scaffolding behind in memory. Ids start at 1, leaving 0 as the "no node" sentinel
// the traverser reads as absence.
type scratchGraph struct {
	next     uint64
	contents map[uint64]string
	edges    map[uint64]map[string][]uint64
}

func newScratchGraph() *scratchGraph {
	return &scratchGraph{contents: map[uint64]string{}, edges: map[uint64]map[string][]uint64{}}
}

// add mints a node holding content and returns its id. link adds a relation edge.
func (g *scratchGraph) add(content string) uint64 {
	g.next++
	g.contents[g.next] = content
	return g.next
}

func (g *scratchGraph) link(from, to uint64, rel string) {
	if g.edges[from] == nil {
		g.edges[from] = map[string][]uint64{}
	}
	g.edges[from][rel] = append(g.edges[from][rel], to)
}

// attachSlot binds held to parent as the rank-th ordered child under rel: a distinct
// slot node carrying the ordinal and holding the value, the write side of the slot walk
// orderedSlots reads.
func (g *scratchGraph) attachSlot(parent uint64, rel string, rank int, held uint64) {
	slot := g.add("")
	g.link(parent, slot, rel)
	g.link(slot, g.add(strconv.Itoa(rank)), rankRel)
	g.link(slot, held, holdsRel)
}

func (g *scratchGraph) content(id uint64) string { return g.contents[id] }

func (g *scratchGraph) dests(id uint64, rel string) []uint64 {
	if g.edges[id] == nil {
		return nil
	}
	return g.edges[id][rel]
}
