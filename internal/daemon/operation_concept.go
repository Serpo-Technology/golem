package daemon

import (
	"time"

	"golem/internal/code"
	"golem/internal/grammar"
	"golem/internal/memory"
)

// An operation the code faculty runs is seeded as a concept, named by the words that
// ask for it: its teaching word ("sum"), the operator-register word ("plus"), and its
// forms in other tongues. So which operation a computation names is resolved through
// the graph, cross-tongue and teachable, rather than a private English map kept in the
// engine, the same way a faculty's verb resolves to its command concept.

// operationWords names each arithmetic primitive beyond the "the <word> of" teaching
// surface its concept already carries: the operator-register word, the verb a bare
// imperative names the operation by ("add 6 and 7"), and a word in each other
// tongue, so a computation reads cross-tongue and the operator register is a
// grounded naming edge rather than a hardcoded table.
var operationWords = map[string]map[string][]string{
	"add":      {"en": {"plus", "add"}, "tr": {"topla", "artı"}, "la": {"adde"}},
	"subtract": {"en": {"minus", "subtract"}, "tr": {"çıkar", "eksi"}, "la": {"deme"}},
	"multiply": {"en": {"times", "multiply"}, "tr": {"çarp"}, "la": {"multiplica"}},
	"divide":   {"en": {"over", "divide"}, "tr": {"böl"}, "la": {"divide"}},
}

// comparisonOps are the comparisons a conditional's test names, grounded operations
// like the arithmetic primitives but kept off the call-head resolver: a relator word
// names one ("above" -> gt) and its operator symbol drives the branch, never a
// value-producing call. eq carries no relator word; it is the bare copula's default
// comparison, equality.
var comparisonOps = []struct {
	name, symbol string
	words        map[string][]string
}{
	{"gt", ">", map[string][]string{"en": {"above"}}},
	{"lt", "<", map[string][]string{"en": {"below"}}},
	{"eq", "==", nil},
}

// extremalOps are the extremal reductions a property goal names, the maximum and
// the minimum of a list. Like the comparisons they are grounded concepts, but kept
// in extremalConcept apart from both opConcept and cmpConcept, so an extremal word
// ("greatest") never reads as a call head or a conditional relator. Each carries
// the comparison it extremizes by, by name; the symbol is read from that grounded
// comparison concept, not written here. "largest"/"least" are left out so they do
// not collide with a largest reduction taught by example.
var extremalOps = []struct {
	name, cmp string
	words     map[string][]string
}{
	{"max", "gt", map[string][]string{"en": {"greatest", "maximum", "highest"}}},
	{"min", "lt", map[string][]string{"en": {"smallest", "minimum", "lowest"}}},
}

// propertyOps are the properties a goal names, each a concept whose meaning is the
// conjunction of grounded relations a function must satisfy. Like the extremals they are
// grounded concepts kept in propertyConcept apart from the other namespaces, so a property
// word ("magnitude") never reads as a call head, a relator, or an extremal. The relations
// are built from grounded operations (multiply) and grounded comparison symbols read from
// the comparison concepts, so the property's meaning is grounded data, not a literal here;
// the procedure that satisfies it is discovered by running, never written.
var propertyOps = []struct {
	name  string
	words map[string][]string
	build func(eq, lt string) code.Property
}{
	// The magnitude: the output squares to the input squared (so it is the number or its
	// negation) and is not below zero (so the non-negative one is chosen). Non-negativity is
	// the forbidden region "below zero", read through the grounded lt comparison, so no
	// "at least" comparison is invented.
	{"magnitude", map[string][]string{"en": {"magnitude", "size"}}, func(eq, lt string) code.Property {
		square := func(v code.Computation) code.Computation {
			return code.Computation{Op: "multiply", Args: []code.Computation{v, v}}
		}
		return code.Property{
			Equational: []code.Relation{{Left: square(code.Computation{Var: code.ResultVar}), Cmp: eq, Right: square(code.Computation{Var: inducedParam})}},
			Ordering:   &code.Relation{Left: code.Computation{Var: code.ResultVar}, Cmp: lt, Right: code.Computation{Lit: "0"}},
		}
	}},
}

// ensureOperations seeds each operation the engine carries as a concept and loads the
// resolver index. Idempotent like the participants and commands: the concept and its
// naming edges are reused when present, re-minted after a consolidation prunes them.
func (d *Daemon) ensureOperations() {
	d.opConcept = map[uint64]code.OpRef{}
	d.stringOpConcept = map[uint64]code.OpRef{}
	d.cmpConcept = map[uint64]string{}
	d.extremalConcept = map[uint64]string{}
	d.propertyConcept = map[uint64]code.Property{}
	d.cmpDefault = ""
	if d.code == nil {
		return
	}
	for _, op := range d.code.Ops() {
		id := d.ensureOperationConcept("op:"+op.Name, op.Word)
		if id == 0 {
			continue
		}
		ref := code.OpRef{Name: op.Name, Symbol: op.Symbol, Arity: op.Arity()}
		// A string primitive is recorded apart, in stringOpConcept, so its word never
		// resolves as a value-producing call head in ordinary talk; a morphology rule
		// reaches it structurally, not by resolving a spoken word.
		if op.Emit != "" {
			d.stringOpConcept[id] = ref
		} else {
			d.opConcept[id] = ref
		}
		if op.Word != "" {
			d.nameConceptWord(id, op.Word, "en")
		}
		// The operator symbol names the operation the way its words do: a graph
		// citizen ("+" naming op:add) resolved by the ordinary naming walk, so
		// written arithmetic reads through the same grounded lookup as speech
		// and a taught alternative can outweigh it. No symbol table stands
		// between the surface and the concept.
		if op.Symbol != "" {
			d.nameConceptWord(id, op.Symbol, "en")
		}
		for lang, words := range operationWords[op.Name] {
			for _, w := range words {
				d.nameConceptWord(id, w, lang)
			}
		}
	}
	// The comparisons are grounded the same way but recorded apart, in cmpConcept, so
	// a relator word resolves only in a conditional's test and never reads as a call
	// head. They do not depend on the engine's op set, so they are seeded outright.
	for _, cmp := range comparisonOps {
		id := d.ensureOperationConcept("op:"+cmp.name, cmp.name)
		if id == 0 {
			continue
		}
		d.cmpConcept[id] = cmp.symbol
		if cmp.name == "eq" {
			d.cmpDefault = cmp.symbol
		}
		for lang, words := range cmp.words {
			for _, w := range words {
				d.nameConceptWord(id, w, lang)
			}
		}
	}
	// The extremal reductions are grounded the same way, recorded apart in
	// extremalConcept, after the comparisons so each can read its comparison's symbol
	// from the grounded concept rather than carry a literal.
	for _, ex := range extremalOps {
		sym, ok := d.cmpSymbolByName(ex.cmp)
		if !ok {
			continue
		}
		id := d.ensureOperationConcept("op:"+ex.name, ex.name)
		if id == 0 {
			continue
		}
		d.extremalConcept[id] = sym
		for lang, words := range ex.words {
			for _, w := range words {
				d.nameConceptWord(id, w, lang)
			}
		}
	}
	// The properties are grounded the same way, recorded apart in propertyConcept, after the
	// comparisons so each relation reads its comparison's symbol from the grounded concept
	// rather than a literal.
	eq, eqok := d.cmpSymbolByName("eq")
	lt, ltok := d.cmpSymbolByName("lt")
	if eqok && ltok {
		for _, pr := range propertyOps {
			id := d.ensureOperationConcept("op:"+pr.name, pr.name)
			if id == 0 {
				continue
			}
			d.propertyConcept[id] = pr.build(eq, lt)
			for lang, words := range pr.words {
				for _, w := range words {
					d.nameConceptWord(id, w, lang)
				}
			}
		}
	}
}

// cmpSymbolByName reads the operator symbol of a comparison concept by its name,
// "gt" -> ">", from the grounded comparison concept rather than a literal, so an
// extremal that turns on a comparison carries the same symbol the comparison does.
func (d *Daemon) cmpSymbolByName(name string) (string, bool) {
	c, err := d.store.ConceptByTemplate("op:" + name)
	if err != nil || c == nil {
		return "", false
	}
	sym, ok := d.cmpConcept[c.ID]
	return sym, ok
}

// ensureOperationConcept mints an operation concept, distinct in category so a word
// that names an operation is never mistaken for a noun or a relation in ordinary talk.
// The content falls back to the name when the operation has no teaching word.
func (d *Daemon) ensureOperationConcept(template, content string) uint64 {
	if content == "" {
		content = template
	}
	if c, err := d.store.ConceptByTemplate(template); err == nil && c != nil {
		return c.ID
	}
	c := &memory.Fragment{Kind: memory.KindConcept, Channel: "concept", Content: content,
		Template: template, Category: "operation", Limbic: d.chem.Snapshot(), Time: time.Now()}
	if _, err := d.store.Encode(c); err != nil {
		return 0
	}
	return c.ID
}

// resolveOperation maps a surface word to the operation it names, the grounded
// substitute for the engine's English word map. It resolves the word to a concept
// (cross-tongue, with case-stripping, like any other word) and returns the operation
// that concept names. This is what the engine consults through its resolver.
func (d *Daemon) resolveOperation(word string) (code.OpRef, bool) {
	c := d.resolveConcept(word)
	if c == nil {
		return code.OpRef{}, false
	}
	ref, ok := d.opConcept[c.ID]
	return ref, ok
}

// opHead resolves an operation word for the grammar's call parser: the canonical
// name the call carries and the arity that bounds its operands, both read off the
// operation's concept. It is resolveOperation in the shape the parser takes, so
// which operation a word names stays grounded and cross-tongue while the grammar
// holds only the form. A comparison word does not resolve here (comparisons live
// in cmpConcept), so a relator never reads as a call head.
func (d *Daemon) opHead(word string) (name string, arity int, ok bool) {
	ref, ok := d.resolveOperation(word)
	if !ok {
		return "", 0, false
	}
	return ref.Name, ref.Arity, true
}

// cmpRelator resolves a comparison relator word to the operator symbol it names,
// the empty word the bare copula's default comparison (equality). It is the
// grounded substitute for a word-to-operator table, reading the comparison concept
// a relator names; ok is false for a word that names no comparison, so the grammar
// reads the test another way.
func (d *Daemon) cmpRelator(relator string) (string, bool) {
	if relator == "" {
		return d.cmpDefault, d.cmpDefault != ""
	}
	c := d.resolveConcept(relator)
	if c == nil {
		return "", false
	}
	sym, ok := d.cmpConcept[c.ID]
	return sym, ok
}

// extremalRelator resolves an extremal word to the comparison symbol the reduction
// it names turns on, reading the extremal concept the word names. It is a namespace
// apart from opHead and cmpRelator, consulted only by ParseExtremal, so an extremal
// word never reads as a call head or a conditional relator. ok is false for a word
// that names no extremal.
func (d *Daemon) extremalRelator(word string) (string, bool) {
	c := d.resolveConcept(word)
	if c == nil {
		return "", false
	}
	sym, ok := d.extremalConcept[c.ID]
	return sym, ok
}

// propertyKind resolves a property word to the property concept it names, for the grammar's
// property-goal parse: it returns the concept's name and whether the word names a property,
// a namespace apart from the operation, comparison, and extremal resolvers. The grammar uses
// only the resolution; the daemon reads the property's meaning with propertyFor.
func (d *Daemon) propertyKind(word string) (string, bool) {
	c := d.resolveConcept(word)
	if c == nil {
		return "", false
	}
	if _, ok := d.propertyConcept[c.ID]; ok {
		return c.Content, true
	}
	return "", false
}

// propertyFor resolves a property word to the grounded property it denotes, the conjunction
// of relations a discovered function must satisfy, read from the property concept the word
// names.
func (d *Daemon) propertyFor(word string) (code.Property, bool) {
	c := d.resolveConcept(word)
	if c == nil {
		return code.Property{}, false
	}
	p, ok := d.propertyConcept[c.ID]
	return p, ok
}

// parseBodyGrammar reads a function body through the grammar, the daemon's grounded
// reading the code engine consults instead of its own English body scan. The head
// resolver also names the function being defined, so a self-call reads as a call
// before the function is in the inventory (recursion); the comparison resolver
// names a conditional's relator. The body is read in English, the tongue the
// definition surface takes. It returns false for a surface the grammar does not yet
// read, so the engine's own reader serves.
func (d *Daemon) parseBodyGrammar(body string, params []string, selfName string, selfArity int) (code.Computation, bool) {
	if d.gram == nil {
		return code.Computation{}, false
	}
	head := func(word string) (string, int, bool) {
		if selfName != "" && word == selfName {
			return selfName, selfArity, true
		}
		return d.opHead(word)
	}
	app, ok := d.gram.ParseBody("en", body, params, head, d.cmpRelator)
	if !ok {
		return code.Computation{}, false
	}
	return applicationToComputation(app), true
}

// applicationToComputation walks the application the grammar parsed into the
// computation it means, a structural copy: a node is an operation over its
// operands, a leaf a literal value, a Var a parameter reference, and a Var marked
// Each the element of a map the definition path then lifts. Arity was enforced as
// the call parsed, so the walk cannot fail on it.
func applicationToComputation(a grammar.Application) code.Computation {
	switch {
	case a.Var != "":
		return code.Computation{Var: a.Var, Each: a.Each}
	case a.Cond != nil:
		return code.Computation{Cond: &code.Conditional{
			Cmp:   a.Cond.Cmp,
			Left:  applicationToComputation(a.Cond.Left),
			Right: applicationToComputation(a.Cond.Right),
			Then:  applicationToComputation(a.Cond.Then),
			Else:  applicationToComputation(a.Cond.Else),
		}}
	case a.Head == "" && !a.List:
		return code.Computation{Lit: a.Lit}
	}
	args := make([]code.Computation, len(a.Args))
	for i, arg := range a.Args {
		args[i] = applicationToComputation(arg)
	}
	if a.List {
		return code.Computation{Op: code.ListWord, Args: args}
	}
	return code.Computation{Op: a.Head, Args: args}
}

// comprehendCall reads a computation call into its meaning, the grammar reading
// the operation-application surface ("the sum of 6 and 7") and the code engine
// the rest: an expression in operator words ("6 plus 7"), raw Go ("6 + 7"), or a
// list literal. The grammar parse is tried first and its application walked into a
// computation; when the surface opens no operation call the code engine reads it.
// So the operation a call names resolves through the concept graph, cross-tongue,
// while every surface the engine read still comprehends. This is the single point
// the daemon's computation paths comprehend a call, so the live answer and the
// learning-loop close never part on two parsers.
func (d *Daemon) comprehendCall(lang, phrase string) (code.Computation, *scratchGraph, uint64, bool) {
	if d.code == nil {
		return code.Computation{}, nil, 0, false
	}
	if app, ok := d.gram.ParseApplication(lang, phrase, d.opHead); ok {
		return d.comprehendApplication(app)
	}
	// The bare imperative "double 4" / "add 6 and 7" names the operation with the
	// leading word and applies it to the operands that follow, the same call without
	// the genitive scaffolding.
	if app, ok := d.gram.ParseBareCall(lang, phrase, d.opHead); ok {
		return d.comprehendApplication(app)
	}
	// The operator register "3 + 4" / "3 plus 4" stands the operation between its
	// operands; the head resolves through the same grounded naming, so a symbol
	// reads as a call exactly when the graph names an operation with it.
	if app, ok := d.gram.ParseInfixCall(lang, phrase, d.opHead); ok {
		return d.comprehendApplication(app)
	}
	comp, ok := d.code.Comprehend(phrase)
	return comp, nil, 0, ok
}

// comprehendApplication lowers a grounded grammar application to its meaning: the
// computation, and the substrate graph built straight from the application when the build
// covers the shape. The graph is generation's source when present; it is nil for a shape
// the build declines, and the caller falls back to materializing the computation.
func (d *Daemon) comprehendApplication(app grammar.Application) (code.Computation, *scratchGraph, uint64, bool) {
	comp := applicationToComputation(app)
	if g, root, ok := d.applicationToGraph(app); ok {
		return comp, g, root, true
	}
	return comp, nil, 0, true
}

// applicationToGraph builds the code-meaning subgraph a comprehended call denotes directly
// from the grounded grammar application, so comprehension lands in the substrate graph
// without the Computation type, the inverse of how generation walks a graph to a surface.
// It mirrors the node and edge vocabulary materializeCode writes, for the shapes it covers
// (a literal leaf, a binary primitive operation, a list literal, and a defined-function
// call, nested), and declines the rest (a conditional, a parameter or map element), which
// the shadow then skips, the way the materializer skipped its own unbuilt shapes early on.
func (d *Daemon) applicationToGraph(app grammar.Application) (*scratchGraph, uint64, bool) {
	g := newScratchGraph()
	root, ok := d.buildAppGraph(g, app)
	if !ok {
		return nil, 0, false
	}
	return g, root, true
}

// buildAppGraph writes one application node into the scratch graph and returns it,
// recursing on the operands. The head is already the canonical operation, resolved through
// the concept graph as the call parsed, so the build reads structure alone and matches no
// surface word.
func (d *Daemon) buildAppGraph(g *scratchGraph, a grammar.Application) (uint64, bool) {
	// A conditional, a parameter, or a map element belongs to a definition body, not a
	// computed call; it is left to a later slice and declined here.
	if a.Cond != nil || a.Var != "" {
		return 0, false
	}
	// A literal leaf carries its value as content.
	if a.Head == "" && !a.List {
		if a.Lit == "" {
			return 0, false
		}
		return g.add(a.Lit), true
	}
	// A list literal: a root grounding in the list concept, its elements held in ordered
	// slots, so one value may repeat across positions without colliding.
	if a.List {
		root := g.add("")
		g.link(root, g.add(code.ListWord), memory.RelGrounds)
		for i, el := range a.Args {
			child, ok := d.buildAppGraph(g, el)
			if !ok {
				return 0, false
			}
			g.attachSlot(root, elemRel, i, child)
		}
		return root, true
	}
	// A binary primitive operation: a root grounding in the operation concept with its two
	// operands. A defined function falls to the call case below.
	if len(a.Args) == 2 && d.opSymbol(a.Head) != "" {
		left, lok := d.buildAppGraph(g, a.Args[0])
		right, rok := d.buildAppGraph(g, a.Args[1])
		if !lok || !rok {
			return 0, false
		}
		root := g.add("")
		g.link(root, g.add(a.Head), memory.RelGrounds)
		g.link(root, left, operand0Rel)
		g.link(root, right, operand1Rel)
		return root, true
	}
	// A defined-function call: the function comes from the stored op, the surface carrying
	// only the head and the arguments, so the function is built as a graph and the call
	// grounds in it with its arguments. The body's own conditionals, recursion, and folds
	// ride through the shared function materializer, so a call to any defined function is
	// covered without the call surface naming those shapes.
	op, ok := d.opByName(a.Head)
	if !ok || len(op.Params) != len(a.Args) {
		return 0, false
	}
	fn, fok := d.materializeFnNode(g, op, map[string]uint64{})
	if !fok {
		return 0, false
	}
	call := g.add("")
	g.link(call, fn, memory.RelGrounds)
	for i, arg := range a.Args {
		child, cok := d.buildAppGraph(g, arg)
		if !cok {
			return 0, false
		}
		g.attachSlot(call, elemRel, i, child)
	}
	return call, true
}
