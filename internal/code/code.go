// Package code is GOLEM learning to program the way it learns any language,
// through a construction with two poles: the form is the syntax, the meaning is
// the computation, and the grounding is execution, what the code does when it
// runs in the runtime. Pseudocode is the meaning made precise, literal language
// is the teaching surface, source is the executable form, and the run is the
// percept. One engine runs the constructions both ways: comprehending a surface
// yields its computation, and generating from a computation yields the surface,
// so GOLEM reads and writes programs from the one inventory.
//
// The code surface is real Go: an expression is read by the language's own
// parser, the innate syntax floor of this faculty the way the grammar engine is
// the floor for natural language, and the operation each operator names is a
// seeded construction, so the engine comprehends genuine source and grounds it
// by running. A few arithmetic constructions are seeded as data and compete by
// weight like the rest of the grammar. A function is a construction GOLEM is
// taught later: a new operation named by a word, defined as a composition over a
// parameter, called where a number would sit, and written out as the Go function
// it means. The principle, code as grounded form-meaning constructions run both
// ways, is what is fixed here.
package code

import (
	"go/ast"
	"go/parser"
	"reflect"
	"sort"
	"strings"
)

// Computation is the meaning pole of a program, pseudocode made precise: an
// expression over an operation and its operands, a bare literal, a reference to
// the parameter of the function being defined, or a conditional choosing between
// two computations on a comparison. It is what comprehending a surface yields and
// what generating one reads, the bridge between the literal a computation is
// taught in and the source it runs as.
type Computation struct {
	Op   string        // the operation: the construction's name, empty for a literal or a variable
	Lit  string        // the literal value, when Op and Var are empty
	Var  string        // a reference to a parameter, inside a defined operation's body
	Args []Computation // the operands of an operation
	Cond *Conditional  // a choice between two computations, when set
	Map  *Mapping      // a transform of a list into a list, element by element, when set
	Each bool          // a reference to the element of a map, "each number", before it is lifted into a Mapping
}

// Conditional is a computation that chooses between two branches on a comparison:
// "the larger of A and B" is A when A is above B, else B. The comparison is the
// only control the faculty needs to express selection and, with a self-call, a
// base case. Cmp is the comparison operator (">", "<", "=="); Left and Right are
// its operands; Then is taken when it holds, Else otherwise.
type Conditional struct {
	Cmp         string
	Left, Right Computation
	Then, Else  Computation
}

// Mapping is a computation that transforms a list into a list element by element:
// "the doubled of a list of numbers is the double of each number" applies the
// per-element body to each element of the list, collecting the results. Var names
// the element, List is the list ranged over, Body the computation over the
// element.
type Mapping struct {
	Var  string
	List Computation
	Body Computation
}

// Op is one code construction: an operation paired with the surfaces it is read
// and written through. A primitive carries the operator symbol that runs it; a
// defined operation carries its parameters and the body that computes it, a
// function learned as one more construction.
type Op struct {
	Name       string       // the operation the meaning pole carries
	Symbol     string       // the code form: the operator that runs it, for a primitive
	Word       string       // the literal form: the word it is taught by
	Params     []string     // the parameters of a defined operation, in order
	ParamTypes []string     // each parameter's type ("int", "list", or "string"), parallel to Params; empty means int
	Body       *Computation // the meaning of a defined operation over its parameters, nil for a primitive
	Raw        string       // the Go statements of an imperative function (a reduction or a synthesized loop), when set
	Ret        string       // an imperative function's return type ("int" or "list"), parallel to Raw
	// A string primitive carries per-operand types and an emit template rather than
	// an operator symbol. In gives each operand's type, Out the result type, and Emit
	// how the operation lowers to import-free Go: "infix:+" for a binary join (concat,
	// prepend), "slice:last" for a[len(a)-1:], "slice:init" for a[:len(a)-1]. The
	// slice forms are ASCII-scoped (byte slices), correct for ASCII English stems and
	// never reused where a multibyte surface is cut. Emit non-empty is the mark that
	// an operation is a string primitive (isStrPrim); such an op leaves Symbol empty so
	// the "+" it emits stays owned by code:add on the raw-source parse path.
	In   []string // operand types; empty defaults to the binary-int arithmetic case
	Out  string   // result type; empty defaults to int
	Emit string   // the import-free Go template a string primitive lowers to
}

// isStrPrim reports whether an operation is a string primitive: one that carries an
// emit template rather than an operator symbol, so the four sites that once read
// symbol-emptiness as the binary-int discriminator classify it explicitly instead.
func (o Op) isStrPrim() bool { return o.Emit != "" }

// emitsPrimary reports whether a string primitive lowers to a Go primary expression
// (a slice expression a[len(a)-1:], which needs no grouping as an operand) rather than
// an infix join (a + b, which an operand position parenthesizes). It keeps the typed
// render's operand parenthesization in lockstep with the graph twin's compound flag.
func (o Op) emitsPrimary() bool { return strings.HasPrefix(o.Emit, "slice:") }

// Arity is how many operands the operation takes, exported so the daemon can record
// it on the operation's concept for the resolver to return.
func (o Op) Arity() int { return o.arity() }

// arity is how many operands the operation takes: an imperative or defined
// operation as many as it has parameters, a primitive is binary.
func (o Op) arity() int {
	switch {
	case len(o.In) != 0:
		return len(o.In) // a string primitive takes as many operands as it declares
	case o.Raw != "":
		return len(o.Params)
	case o.Body != nil:
		return len(o.Params)
	default:
		return 2
	}
}

// paramType reports a parameter's type by position, "int" unless the operation
// declared it a list.
func (o Op) paramType(i int) string {
	if i < len(o.ParamTypes) && o.ParamTypes[i] != "" {
		return o.ParamTypes[i]
	}
	return "int"
}

// Def is a defined operation in the surface it is stored as: a name, the
// parameters it abstracts over in order, and its body as code. It is what a
// learned function persists to and is read back from, so a function survives a
// restart the way a seeded construction does.
type Def struct {
	Name       string
	Params     []string
	ParamTypes []string // parallel to Params, "int" or "list"; empty means all int
	Body       string   // the body as code, e.g. "number * factor"
}

// The value types the faculty distinguishes.
const (
	typeInt  = "int"
	typeList = "list"
	typeStr  = "string"

	listWord = "list" // the literal surface, "the list of 1 and 2 and 3"
	eachWord = "each" // the element of a map, "the double of each number"
)

// isQuoted reports whether a literal's stored form is a Go string literal, its
// leading double quote the durable type marker fromExpr preserves verbatim. A number
// ("5") or a signed number ("-5") never carries one, so none reclassifies.
func isQuoted(lit string) bool { return len(lit) > 0 && lit[0] == '"' }

// ListWord is the list constructor's operation name, exported so a caller that
// builds a list computation from a surface the grammar parsed names it without a
// literal of its own.
const ListWord = listWord

// OpRef identifies the operation a word names: its internal name, its operator
// symbol (empty for a defined op), and its arity. It is the operation's identity
// without its body, what a resolver returns, which the engine then looks up by name
// for the body.
type OpRef struct {
	Name   string
	Symbol string
	Arity  int
}

// Resolver maps a surface word to the operation it names. The engine takes one so
// which-operation is decided outside it, by the daemon's grounded concept graph
// (cross-tongue, teachable), rather than a private English map. With none set, the
// engine resolves against the words its own operations carry, so it still reads a
// computation standalone.
type Resolver func(word string) (OpRef, bool)

// BodyParser reads a function body into the computation it means with the
// parameters in scope, the grounded substitute for the engine's own English body
// scan. The daemon supplies one that parses the body through the grammar; with
// none the engine reads the body itself, so a standalone engine still learns a
// told function. It returns false for a surface it does not read, so the engine's
// own reader serves as the fallback.
type BodyParser func(body string, params []string, selfName string, selfArity int) (Computation, bool)

// Engine carries the code constructions and runs them both ways. It is built from
// the seeded primitives and the functions learned since, indexed by name and by
// operator symbol; which word names an operation is the resolver's, not a map kept
// here.
type Engine struct {
	words     map[string]OpRef // the operations' own words, the resolver's default
	resolve   Resolver         // word -> operation; the daemon's grounded resolver supersedes the default
	bodyParse BodyParser       // body -> computation; the daemon's grammar reader supersedes the engine's own
	synthSelf string           // the function being synthesized, left out of its own search so it is not defined as itself
	bySymbol  map[string]Op
	byName    map[string]Op
}

// NewEngine indexes the primitive constructions, then the defined ones, whose
// bodies are read with the primitives already in hand so a function built from
// them comprehends. A definition whose body does not comprehend is dropped
// rather than indexed half-formed.
func NewEngine(prims []Op, defs []Def) *Engine {
	e := &Engine{words: map[string]OpRef{}, bySymbol: map[string]Op{}, byName: map[string]Op{}}
	for _, o := range prims {
		e.index(o)
	}
	// The operator words are the pseudocode register of the arithmetic primitives.
	// The default resolver knows them so a standalone engine reads "6 plus 7"; the
	// daemon's grounded resolver carries them as cross-tongue naming edges instead.
	for word, sym := range map[string]string{"plus": "+", "minus": "-", "times": "*", "over": "/"} {
		if o, ok := e.bySymbol[sym]; ok {
			e.words[word] = OpRef{Name: o.Name, Symbol: o.Symbol, Arity: o.arity()}
		}
	}
	for _, d := range defs {
		// A synthesized imperative function is stored as its Go statements behind a
		// "raw:<ret> " marker; it reloads as the raw op it is rather than through the
		// expression reader.
		if ret, raw, ok := rawDef(d.Body); ok {
			e.index(Op{Name: d.Name, Word: d.Name, Params: d.Params, ParamTypes: d.ParamTypes, Ret: ret, Raw: raw})
			continue
		}
		body, ok := e.bodyFromSource(d.Body)
		if !ok {
			continue
		}
		e.index(Op{Name: d.Name, Word: d.Name, Params: d.Params, ParamTypes: d.ParamTypes, Body: &body})
	}
	return e
}

// index records an operation in the engine's lookups: by name always, by operator
// symbol when it has one (a primitive), and by the words its surface carries (the
// resolver's default, used when no grounded resolver is set).
func (e *Engine) index(o Op) {
	if o.Word != "" {
		e.words[o.Word] = OpRef{Name: o.Name, Symbol: o.Symbol, Arity: o.arity()}
	}
	if o.Symbol != "" {
		e.bySymbol[o.Symbol] = o
	}
	e.byName[o.Name] = o
}

// SetResolver installs the operation resolver the comprehension path consults, the
// daemon's grounded word-to-operation lookup over the concept graph. It supersedes
// the engine's own word index, so a tongue or a synonym named on an operation
// concept reaches the operation with no change here.
func (e *Engine) SetResolver(r Resolver) { e.resolve = r }

// SetSynthSelf names the function currently being synthesized, which the search
// then leaves out of its own building blocks, so an already-learned operation is not
// re-defined as a trivial call to itself. The caller sets it around a synthesis and
// clears it after; the empty string is the unset state.
func (e *Engine) SetSynthSelf(name string) { e.synthSelf = name }

// SetBodyParser installs the body parser the definition path consults, the
// daemon's grounded reading of a function body through the grammar. It is
// cross-checked against the engine's own reader so the move cannot regress what
// parsed before, and the engine's reader still serves when the grammar declines a
// surface it does not yet read.
func (e *Engine) SetBodyParser(b BodyParser) { e.bodyParse = b }

// bodyOf reads a function body into its computation through the grounded parser
// when one is installed, cross-checked against the engine's own reader: the
// grounded reading is served when the two agree, the engine's when they differ or
// when the grounded parser declines (a surface it does not yet read, like a map),
// so the dissolution cannot regress what parsed before. With no parser installed
// the engine reads the body alone.
func (e *Engine) bodyOf(toks, params []string, selfName string, selfArity int) (Computation, bool) {
	own, ownOK := e.parseBody(toks, params, selfName, selfArity)
	if e.bodyParse == nil {
		return own, ownOK
	}
	if parsed, ok := e.bodyParse(strings.Join(toks, " "), params, selfName, selfArity); ok {
		if !ownOK || sameComputation(parsed, own) {
			return parsed, true
		}
		return own, true // the readers disagree; keep the engine's, the conservative floor
	}
	return own, ownOK
}

// opRef resolves a word to the operation it names, through the installed resolver
// when there is one, else the engine's own operation words.
func (e *Engine) opRef(word string) (OpRef, bool) {
	if e.resolve != nil {
		return e.resolve(word)
	}
	ref, ok := e.words[word]
	return ref, ok
}

// sameComputation reports whether two computations are structurally identical, the
// agreement test between the grounded body parse and the engine's own reading.
func sameComputation(a, b Computation) bool { return reflect.DeepEqual(a, b) }

// rawDefPrefix marks a stored body as the Go statements of a synthesized
// imperative function, the return type following the marker ahead of the body.
const rawDefPrefix = "raw:"

// rawDef reads a stored synthesized body into its return type and statements, the
// inverse of how DefSource writes one; it declines a body that is not raw.
func rawDef(body string) (ret, raw string, ok bool) {
	rest, ok := strings.CutPrefix(body, rawDefPrefix)
	if !ok {
		return "", "", false
	}
	sp := strings.SplitN(rest, " ", 2)
	if len(sp) != 2 || sp[1] == "" {
		return "", "", false
	}
	return sp[0], sp[1], true
}

// Ops lists the operations the engine carries, sorted by name, so a caller can
// enumerate the constructions the way induction by example does to find which
// one a set of examples means.
func (e *Engine) Ops() []Op {
	out := make([]Op, 0, len(e.byName))
	for _, o := range e.byName {
		out = append(out, o)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// Comprehend reads a surface into its computation, the form pole run to the
// meaning. A literal teaching surface ("the sum of 6 and 7"), a code surface
// ("6 + 7"), and a function call ("the double of 6") all yield a computation, so
// GOLEM understands a program however it is given. It returns false when no
// construction matches.
func (e *Engine) Comprehend(text string) (Computation, bool) {
	if c, ok := e.comprehendLiteral(strings.Fields(text)); ok {
		return c, true
	}
	if c, ok := e.comprehendSpoken(text); ok {
		return c, true
	}
	return e.comprehendCode(text)
}

// UnknownOp reports the first operation a computation surface names that no
// construction claims. Each operation in the literal teaching surface is named
// "the <word> of ...", so a call of a function GOLEM has not learned ("the cube
// of 3") is found here. The caller can then admit the gap by name where it would
// otherwise run the words as a broken program. A surface naming only operations
// it knows, or one not phrased as a call at all, returns false.
func (e *Engine) UnknownOp(text string) (string, bool) {
	toks := strings.Fields(text)
	for i := 0; i+2 < len(toks); i++ {
		if toks[i] != "the" || toks[i+2] != "of" {
			continue
		}
		if _, ok := e.opRef(toks[i+1]); !ok {
			return toks[i+1], true
		}
	}
	return "", false
}

// IsComputationSurface reports whether the text is phrased as a request to
// compute: the literal teaching surface, which opens with "the" and joins its
// operands with "of", or an expression said in operator words ("6 plus 7"). It
// tells a computation that did not comprehend (ill-formed, or a call of an
// operation GOLEM lacks) apart from a raw program handed in to run, so the
// first is answered with the gap and the second is left to run as the source it
// already is.
func (e *Engine) IsComputationSurface(text string) bool {
	toks := strings.Fields(text)
	for _, t := range toks {
		if ref, ok := e.opRef(strings.ToLower(t)); ok && ref.Symbol != "" {
			return true
		}
	}
	if len(toks) >= 3 && toks[0] == "the" {
		for _, t := range toks[1:] {
			if t == "of" {
				return true
			}
		}
	}
	return false
}

// comprehendSpoken reads an operator word into its symbol and lets the Go parser
// take it from there, so a computation spoken in words carries the same precedence
// and grouping the symbols would. A word names an operator when it resolves to an
// operation that has a symbol; with none present it declines, so the code path
// reads a plain expression unchanged.
func (e *Engine) comprehendSpoken(text string) (Computation, bool) {
	toks := strings.Fields(text)
	replaced := false
	for i, t := range toks {
		if ref, ok := e.opRef(strings.ToLower(t)); ok && ref.Symbol != "" {
			toks[i] = ref.Symbol
			replaced = true
		}
	}
	if !replaced {
		return Computation{}, false
	}
	return e.comprehendCode(strings.Join(toks, " "))
}

// comprehendLiteral reads the teaching surface, composed: an operand is a number
// or another "the <word> of ...", so a computation built from many operations
// comprehends as one, the meaning pole read the way ToLiteral writes it. The
// surface opens with an operation, and the parse must consume every token, so a
// binary operation handed a third operand ("the sum of 2 and 3 and 4") is the
// ill-formed surface it is and fails on the tail it cannot place.
func (e *Engine) comprehendLiteral(toks []string) (Computation, bool) {
	if len(toks) == 0 || toks[0] != "the" {
		return Computation{}, false
	}
	c, n, ok := e.literalOperand(toks, nil, "", 0)
	if !ok || n != len(toks) {
		return Computation{}, false
	}
	return c, true
}

// isParam reports whether a word is one of the parameters in scope inside a
// function body, so "the number" reads as a reference to it rather than the
// opening of an operation.
func isParam(params []string, word string) bool {
	for _, p := range params {
		if p == word {
			return true
		}
	}
	return false
}

// literalOperand reads one operand from the front of toks and reports how many
// tokens it spanned. A bare token is a literal; "the <param>" inside a function
// body is a reference to one of the parameters in scope; "the <word> of <A> and
// <B> and ..." is the operation its word names over as many operands as it
// takes, each read the same way, the recursion that lets the surface nest and
// lets a function call sit where a number would. selfName, with selfArity, is the
// function being defined, so its own body can call it (recursion) before it is in
// the inventory.
func (e *Engine) literalOperand(toks, params []string, selfName string, selfArity int) (Computation, int, bool) {
	if len(toks) == 0 {
		return Computation{}, 0, false
	}
	// "each <name>" is the element of a map, "the double of each number"; it is
	// lifted into a Mapping over the list parameter once the body is read.
	if toks[0] == eachWord && len(toks) >= 2 {
		return Computation{Var: toks[1], Each: true}, 2, true
	}
	// A parameter reference, "the number", takes precedence over reading "the"
	// as the opening of an operation.
	if len(toks) >= 2 && toks[0] == "the" && isParam(params, toks[1]) {
		return Computation{Var: toks[1]}, 2, true
	}
	if toks[0] != "the" {
		return Computation{Lit: toks[0]}, 1, true
	}
	if len(toks) < 3 || toks[2] != "of" {
		return Computation{}, 0, false
	}
	// A list literal, "the list of 1 and 2 and 3", reads as many operands as are
	// joined by "and", variadic where an operation is fixed-arity.
	if toks[1] == listWord {
		consumed := 3 // "the list of"
		rest := toks[3:]
		var elems []Computation
		for {
			el, n, ok := e.literalOperand(rest, params, selfName, selfArity)
			if !ok {
				return Computation{}, 0, false
			}
			elems = append(elems, el)
			rest, consumed = rest[n:], consumed+n
			if len(rest) == 0 || rest[0] != "and" {
				break
			}
			rest, consumed = rest[1:], consumed+1
		}
		return Computation{Op: listWord, Args: elems}, consumed, true
	}
	opName, arity := "", 0
	if selfName != "" && toks[1] == selfName {
		opName, arity = selfName, selfArity
	} else if ref, ok := e.opRef(toks[1]); ok && ref.Arity >= 1 {
		opName, arity = ref.Name, ref.Arity
	} else {
		return Computation{}, 0, false
	}
	consumed := 3 // "the <word> of"
	rest := toks[3:]
	args := make([]Computation, 0, arity)
	for i := 0; i < arity; i++ {
		if i > 0 {
			if len(rest) == 0 || rest[0] != "and" {
				return Computation{}, 0, false
			}
			rest = rest[1:]
			consumed++
		}
		arg, n, ok := e.literalOperand(rest, params, selfName, selfArity)
		if !ok {
			return Computation{}, 0, false
		}
		args = append(args, arg)
		rest = rest[n:]
		consumed += n
	}
	return Computation{Op: opName, Args: args}, consumed, true
}

// defHeader reads the structural shape of a function definition, "the <name> of
// <params> is", into the name, the parameters with their types, and the index the
// body begins at. A parameter is a noun introduced by "a" or "an", "a <noun>" a
// number and "a list of <noun>" a list, the declarations joined by "and" up to
// "is". It recognizes the shape and nothing more, so the definition reader and the
// failed-attempt check read the same surface; whether the definition is sound (its
// name free, its parameters used, its body well-typed) is the caller's to judge.
func (e *Engine) defHeader(toks []string) (name string, params, paramTypes []string, bodyStart int, ok bool) {
	if len(toks) < 7 || toks[0] != "the" || toks[2] != "of" {
		return "", nil, nil, 0, false
	}
	name = toks[1]
	i := 3
	for {
		if i+1 >= len(toks) || (toks[i] != "a" && toks[i] != "an") {
			return "", nil, nil, 0, false
		}
		if toks[i+1] == listWord && i+3 < len(toks) && toks[i+2] == "of" {
			params = append(params, toks[i+3])
			paramTypes = append(paramTypes, typeList)
			i += 4
		} else {
			params = append(params, toks[i+1])
			paramTypes = append(paramTypes, typeInt)
			i += 2
		}
		if i >= len(toks) {
			return "", nil, nil, 0, false
		}
		if toks[i] == "is" {
			i++
			break
		}
		if toks[i] != "and" {
			return "", nil, nil, 0, false
		}
		i++
	}
	if name == "" || len(params) == 0 || i >= len(toks) {
		return "", nil, nil, 0, false
	}
	return name, params, paramTypes, i, true
}

// IsDefinitionAttempt reports whether a turn is shaped like a function definition
// whose body computes over its parameters, the mark of a definition the caller
// admits a gap for rather than grounds as a world fact once the definition reader
// has declined it on a soundness check (a name that shadows a primitive, a type
// error). It runs the same shape recognizer and the same body reader the definition
// path runs, then requires the body to use a parameter. A generic fact ("the friend
// of a man is a dog", "the color of a rose is red") has a predicate that names no
// parameter (it fails the body read, or reads as a bare constant), so it is not
// taken for a definition and keeps grounding. It returns the name so the caller can
// admit the gap by it.
func (e *Engine) IsDefinitionAttempt(text string) (string, bool) {
	toks := strings.Fields(text)
	name, params, _, bodyStart, ok := e.defHeader(toks)
	if !ok {
		return "", false
	}
	body, ok := e.bodyOf(toks[bodyStart:], params, name, len(params))
	if !ok {
		return "", false
	}
	// A definition computes over its inputs, so the body must use a parameter. This
	// is what separates a failed definition from a constant predicate ("is red"),
	// which a brain reads as a fact, not a function it could not work out.
	for _, p := range params {
		if usesVar(body, p) {
			return name, true
		}
	}
	return "", false
}

// ComprehendDefinition reads a function taught in language into the operation it
// defines. A unary "the double of a number is the product of the number and 2"
// and an n-ary "the blend of a number and a factor is the product of the number
// and the factor" both parse: the name is the word after "the", the parameters
// are the nouns each introduced by "a" and joined by "and" up to "is", and the
// body is the computation after "is", read with the parameters in scope so "the
// number" is a reference to one. The body must comprehend, must consume every
// word after "is", and must use every parameter, so a half-formed or constant
// "function", or one with a dead parameter, fails rather than being half-learned.
func (e *Engine) ComprehendDefinition(text string) (Op, bool) {
	toks := strings.Fields(text)
	name, params, paramTypes, bodyStart, ok := e.defHeader(toks)
	if !ok {
		return Op{}, false
	}
	// A name may not shadow a seeded primitive, by its name or its word.
	if op, taken := e.byName[name]; taken && op.Body == nil {
		return Op{}, false
	}
	if e.Primitive(name) {
		return Op{}, false
	}
	// Each parameter must be distinct and not the name. A parameter may share a
	// word with an operation (a "count", a "first"): inside the body a parameter
	// reference is read before an operation, so "the count" is the parameter and is
	// never misread as the operation it happens to name, and the definition need not
	// be refused for it. Refusing would lose ordinary parameter nouns on a brain
	// rich enough that they collide with a learned operation.
	seen := map[string]bool{}
	for _, p := range params {
		if p == "" || p == name || seen[p] {
			return Op{}, false
		}
		seen[p] = true
		if p == listWord || p == eachWord {
			return Op{}, false // a parameter may not shadow a surface marker the body reads
		}
	}
	body, ok := e.bodyOf(toks[bodyStart:], params, name, len(params))
	if !ok {
		return Op{}, false
	}
	// A body that names "each <element>" is a transform: lift it into a map over
	// the list parameter, so "the double of each number" maps double across the
	// list. The lift runs before the parameter-use check, since the list parameter
	// is used by the map it is lifted into.
	if lifted, ok := liftMap(body, params, paramTypes); ok {
		body = lifted
	} else if eachReferenced(body) {
		return Op{}, false // an "each" the lift could not resolve
	}
	// The body must be a genuine function of every parameter, with no dead ones.
	for _, p := range params {
		if !usesVar(body, p) {
			return Op{}, false
		}
	}
	// The body must type-check against the parameters' types, so a definition that
	// would run to a broken program (a list operation on a number, say) is refused.
	env := make(map[string]string, len(params))
	for j, p := range params {
		env[p] = paramTypes[j]
	}
	if _, ok := e.typeOf(body, env, name); !ok {
		return Op{}, false
	}
	return Op{Name: name, Word: name, Params: params, ParamTypes: paramTypes, Body: &body}, true
}

// parseBody reads a function body with the parameters in scope: a conditional
// "<then> if <cond> else <else>", or a plain arithmetic computation. It tolerates
// a comma before "else" and fails unless the body comprehends and consumes every
// token, so a half-formed body is not half-learned.
func (e *Engine) parseBody(toks, params []string, selfName string, selfArity int) (Computation, bool) {
	clean := make([]string, len(toks))
	for i, t := range toks {
		clean[i] = strings.TrimSuffix(t, ",")
	}
	ifAt, elseAt := -1, -1
	for i, t := range clean {
		switch {
		case t == "if" && ifAt < 0:
			ifAt = i
		case t == "else" && ifAt >= 0:
			elseAt = i
		}
		if elseAt >= 0 {
			break
		}
	}
	if ifAt < 0 {
		body, n, ok := e.literalOperand(clean, params, selfName, selfArity)
		if !ok || n != len(clean) {
			return Computation{}, false
		}
		return body, true
	}
	if ifAt == 0 || elseAt <= ifAt+1 || elseAt == len(clean)-1 {
		return Computation{}, false
	}
	then, n, ok := e.literalOperand(clean[:ifAt], params, selfName, selfArity)
	if !ok || n != ifAt {
		return Computation{}, false
	}
	cond, ok := e.comparison(clean[ifAt+1:elseAt], params, selfName, selfArity)
	if !ok {
		return Computation{}, false
	}
	els, m, ok := e.literalOperand(clean[elseAt+1:], params, selfName, selfArity)
	if !ok || m != len(clean)-elseAt-1 {
		return Computation{}, false
	}
	cond.Then, cond.Else = then, els
	return Computation{Cond: cond}, true
}

// comparison reads a conditional's test into the operator and its operands: "<A>
// is above <B>" is A > B, "<A> is below <B>" is A < B, and a bare "<A> is <B>" is
// equality. The branches are left empty for parseBody to fill.
func (e *Engine) comparison(toks, params []string, selfName string, selfArity int) (*Conditional, bool) {
	left, n, ok := e.literalOperand(toks, params, selfName, selfArity)
	if !ok || n >= len(toks) || toks[n] != "is" {
		return nil, false
	}
	rest := toks[n+1:]
	cmp := "=="
	if len(rest) > 0 {
		switch rest[0] {
		case "above":
			cmp, rest = ">", rest[1:]
		case "below":
			cmp, rest = "<", rest[1:]
		}
	}
	right, m, ok := e.literalOperand(rest, params, selfName, selfArity)
	if !ok || m != len(rest) {
		return nil, false
	}
	return &Conditional{Cmp: cmp, Left: left, Right: right}, true
}

// usesVar reports whether a parameter is referenced anywhere in a body, the test
// that a definition is a function of its parameter and not a constant dressed as
// one. It descends through a conditional's comparison and both branches, so a
// parameter used only to choose between two constants still counts as used.
func usesVar(c Computation, name string) bool {
	if c.Map != nil {
		return usesVar(c.Map.List, name) || usesVar(c.Map.Body, name)
	}
	if c.Cond != nil {
		return usesVar(c.Cond.Left, name) || usesVar(c.Cond.Right, name) ||
			usesVar(c.Cond.Then, name) || usesVar(c.Cond.Else, name)
	}
	if c.Var == name {
		return true
	}
	for _, a := range c.Args {
		if usesVar(a, name) {
			return true
		}
	}
	return false
}

// references reports whether a computation calls the operation named, descending
// through arguments and a conditional. It is how a self-call is found, the mark
// that a definition is recursive.
func references(c Computation, name string) bool {
	if c.Map != nil {
		return references(c.Map.List, name) || references(c.Map.Body, name)
	}
	if c.Cond != nil {
		return references(c.Cond.Left, name) || references(c.Cond.Right, name) ||
			references(c.Cond.Then, name) || references(c.Cond.Else, name)
	}
	if c.Op == name {
		return true
	}
	for _, a := range c.Args {
		if references(a, name) {
			return true
		}
	}
	return false
}

// recursive reports whether a defined operation calls itself, so it is run as a
// self-referential local function rather than inlined, since an inline of a
// recursive body would expand forever.
func (e *Engine) recursive(op Op) bool {
	return op.Body != nil && references(*op.Body, op.Name)
}

// liftMap turns a body that names "each <element>" into a map over the list
// parameter: the element variable is the each-reference's name, the list is the
// single list parameter, and the per-element body reads each-references as the
// element. It returns false when the body names no element, names more than one,
// has no single list parameter to map over, or the element would shadow a
// parameter, so a transform with nowhere to map is refused.
func liftMap(body Computation, params, paramTypes []string) (Computation, bool) {
	set := map[string]bool{}
	collectEach(body, set)
	if len(set) != 1 {
		return Computation{}, false
	}
	var v string
	for k := range set {
		v = k
	}
	listParam := ""
	for i, p := range params {
		if p == v {
			return Computation{}, false
		}
		if i < len(paramTypes) && paramTypes[i] == typeList {
			if listParam != "" {
				return Computation{}, false
			}
			listParam = p
		}
	}
	if listParam == "" {
		return Computation{}, false
	}
	return Computation{Map: &Mapping{Var: v, List: Computation{Var: listParam}, Body: clearEach(body)}}, true
}

// eachReferenced reports whether a body still names "each <element>", the mark of
// a transform the lift must resolve.
func eachReferenced(c Computation) bool {
	set := map[string]bool{}
	collectEach(c, set)
	return len(set) > 0
}

// collectEach gathers the element names a body names through "each <element>".
func collectEach(c Computation, set map[string]bool) {
	switch {
	case c.Each:
		set[c.Var] = true
	case c.Cond != nil:
		collectEach(c.Cond.Left, set)
		collectEach(c.Cond.Right, set)
		collectEach(c.Cond.Then, set)
		collectEach(c.Cond.Else, set)
	default:
		for _, a := range c.Args {
			collectEach(a, set)
		}
	}
}

// clearEach reads each "each <element>" reference as a plain reference to the
// element, the form the per-element body of a map takes.
func clearEach(c Computation) Computation {
	switch {
	case c.Each:
		return Computation{Var: c.Var}
	case c.Cond != nil:
		return Computation{Cond: &Conditional{Cmp: c.Cond.Cmp,
			Left: clearEach(c.Cond.Left), Right: clearEach(c.Cond.Right),
			Then: clearEach(c.Cond.Then), Else: clearEach(c.Cond.Else)}}
	case len(c.Args) > 0:
		args := make([]Computation, len(c.Args))
		for i, a := range c.Args {
			args[i] = clearEach(a)
		}
		return Computation{Op: c.Op, Args: args}
	default:
		return c
	}
}

// typeOf computes a computation's value type ("int" or "list") and checks that
// every operation is given the kind of value it takes: a list operation a list, a
// primitive and arithmetic a number, a function its declared parameter types. env
// gives the parameters' types in scope; self names the function being defined,
// whose own calls yield a number. It returns false on any mismatch, so a
// type-inconsistent computation is refused before it is run to a broken program.
func (e *Engine) typeOf(c Computation, env map[string]string, self string) (string, bool) {
	switch {
	case c.Map != nil:
		if lt, ok := e.typeOf(c.Map.List, env, self); !ok || lt != typeList {
			return "", false
		}
		inner := make(map[string]string, len(env)+1)
		for k, v := range env {
			inner[k] = v
		}
		inner[c.Map.Var] = typeInt // an element of a list of numbers is a number
		if bt, ok := e.typeOf(c.Map.Body, inner, self); !ok || bt != typeInt {
			return "", false
		}
		return typeList, true
	case c.Each:
		return typeInt, true // an element reference, before it is lifted into a map
	case c.Cond != nil:
		lt, lok := e.typeOf(c.Cond.Left, env, self)
		rt, rok := e.typeOf(c.Cond.Right, env, self)
		tt, tok := e.typeOf(c.Cond.Then, env, self)
		et, eok := e.typeOf(c.Cond.Else, env, self)
		if !lok || !rok || !tok || !eok {
			return "", false
		}
		// The test compares two operands of one type: an ordering (">"/"<") over
		// numbers, or equality ("==") over numbers or strings. The two branches share
		// a type T, ints for an arithmetic choice or strings for a morphology cascade,
		// and the conditional yields T.
		if lt != rt || tt != et {
			return "", false
		}
		if c.Cond.Cmp != "==" && lt != typeInt {
			return "", false
		}
		return tt, true
	case c.Var != "":
		t, ok := env[c.Var]
		return t, ok
	case c.Op == "":
		if c.Lit == "" {
			return "", false
		}
		if isQuoted(c.Lit) {
			return typeStr, true
		}
		return typeInt, true
	case c.Op == listWord:
		for _, a := range c.Args {
			if t, ok := e.typeOf(a, env, self); !ok || t != typeInt {
				return "", false
			}
		}
		return typeList, true
	}
	if self != "" && c.Op == self {
		for _, a := range c.Args {
			if _, ok := e.typeOf(a, env, self); !ok {
				return "", false
			}
		}
		return typeInt, true
	}
	op, ok := e.byName[c.Op]
	if !ok {
		return "", false
	}
	if op.Raw != "" { // a synthesized function takes its declared parameters, returns its type
		if len(c.Args) != len(op.Params) {
			return "", false
		}
		for i, a := range c.Args {
			if t, ok := e.typeOf(a, env, self); !ok || t != op.paramType(i) {
				return "", false
			}
		}
		if op.Ret == typeList {
			return typeList, true
		}
		return typeInt, true
	}
	if op.isStrPrim() { // a string primitive takes its declared operand types, returns Out
		if len(c.Args) != len(op.In) {
			return "", false
		}
		for i, a := range c.Args {
			if t, ok := e.typeOf(a, env, self); !ok || t != op.In[i] {
				return "", false
			}
		}
		return op.Out, true
	}
	want := 2
	if op.Body != nil {
		want = len(op.Params)
	}
	if len(c.Args) != want {
		return "", false
	}
	for i, a := range c.Args {
		expect := typeInt
		if op.Body != nil {
			expect = op.paramType(i)
		}
		if t, ok := e.typeOf(a, env, self); !ok || t != expect {
			return "", false
		}
	}
	// A defined operation returns whatever its body yields, so a transform that
	// returns a list reads as a list to whatever consumes it; a primitive returns a
	// number.
	if op.Body != nil {
		return e.retType(op)
	}
	return typeInt, true
}

// TypeCheck reports whether a computation is type-consistent, so a computation
// that would run to a broken program (a list operation on a number, say) is
// refused before it is run.
func (e *Engine) TypeCheck(c Computation) bool {
	_, ok := e.typeOf(c, nil, "")
	return ok
}

// ReturnType reports the type a call of the named operation yields, "int" or "list", so a
// caller type-checking over a representation other than the Computation (the substrate
// graph) can read an operation's result type without re-deriving its body. A synthesized
// operation carries its return type, a defined one yields its body's type, and a primitive
// returns a number.
func (e *Engine) ReturnType(name string) (string, bool) {
	op, ok := e.byName[name]
	if !ok {
		return "", false
	}
	if op.Raw != "" {
		if op.Ret == typeList {
			return typeList, true
		}
		return typeInt, true
	}
	if op.Body != nil {
		return e.retType(op)
	}
	if op.Out != "" { // a string primitive declares its result type
		return op.Out, true
	}
	return typeInt, true
}

// comprehendCode reads the executable surface as real Go: the language's own
// parser turns the source into a syntax tree, and fromExpr walks it into the
// computation, so a nested, parenthesized expression of any depth comprehends.
// The operator still has to be one a construction claims, so the engine reads
// genuine Go but only the operations it has been taught, and an identifier is a
// parameter the way a function body reads back.
func (e *Engine) comprehendCode(text string) (Computation, bool) {
	expr, err := parser.ParseExpr(text)
	if err != nil {
		return Computation{}, false
	}
	return e.fromExpr(expr)
}

// fromExpr walks a Go expression tree into a computation: a literal is a value,
// an identifier is a parameter reference, a parenthesized expression is its
// inside, a signed literal is one value, and a binary expression is the
// operation its operator names over its two sides. An operator no construction
// claims, or a shape the meaning pole does not reach, fails cleanly.
func (e *Engine) fromExpr(expr ast.Expr) (Computation, bool) {
	switch x := expr.(type) {
	case *ast.BasicLit:
		return Computation{Lit: x.Value}, true
	case *ast.Ident:
		return Computation{Var: x.Name}, true
	case *ast.ParenExpr:
		return e.fromExpr(x.X)
	case *ast.UnaryExpr:
		inner, ok := x.X.(*ast.BasicLit)
		if !ok || (x.Op.String() != "-" && x.Op.String() != "+") {
			return Computation{}, false
		}
		return Computation{Lit: x.Op.String() + inner.Value}, true
	case *ast.BinaryExpr:
		op, ok := e.bySymbol[x.Op.String()]
		if !ok {
			return Computation{}, false
		}
		a, aok := e.fromExpr(x.X)
		b, bok := e.fromExpr(x.Y)
		if !aok || !bok {
			return Computation{}, false
		}
		return Computation{Op: op.Name, Args: []Computation{a, b}}, true
	case *ast.CallExpr:
		// A call to a named function, "factorial(number - 1)", the form a self-call
		// reads back from storage as, so a recursive body reloads as the call it is
		// rather than expanding.
		id, ok := x.Fun.(*ast.Ident)
		if !ok {
			return Computation{}, false
		}
		args := make([]Computation, 0, len(x.Args))
		for _, a := range x.Args {
			ac, aok := e.fromExpr(a)
			if !aok {
				return Computation{}, false
			}
			args = append(args, ac)
		}
		return Computation{Op: id.Name, Args: args}, true
	default:
		return Computation{}, false
	}
}

// ToCode generates the executable form from a computation, the meaning pole run
// to source: add(6, 7) becomes "6 + 7". A call of a non-recursive defined
// operation is inlined; a recursive one is written as a self-referential local
// function and called, so the source runs in the runtime as a self-contained
// expression either way, with no top-level declaration to carry along.
func (e *Engine) ToCode(c Computation) (string, bool) {
	return e.render(c, "", false)
}

// render is the one code generator, parameterized by the name of the function
// whose body is being written (so a call to that name is a self-call, written as
// a plain call to the local recursive variable) and by whether this is for
// storage (flat), where a recursive call is kept as a plain call to read back
// rather than expanded into a function literal.
func (e *Engine) render(c Computation, self string, flat bool) (string, bool) {
	if c.Map != nil {
		if flat {
			return "", false // a map is stored structurally, not as runnable code
		}
		list, lok := e.render(c.Map.List, self, false)
		body, bok := e.render(c.Map.Body, self, false)
		if !lok || !bok {
			return "", false
		}
		return "(func() []int { " + mapLoop("out", c.Map.Var, list, body) + " }())", true
	}
	if c.Each {
		return c.Var, true // the element, inside a map's per-element body
	}
	if c.Cond != nil {
		if flat {
			return "", false // a nested conditional is not stored in this form
		}
		return e.genCond(c.Cond, self)
	}
	if c.Var != "" {
		return c.Var, true
	}
	if c.Op == "" {
		return c.Lit, c.Lit != ""
	}
	if c.Op == listWord {
		args, ok := e.renderArgs(c.Args, self, flat)
		if !ok {
			return "", false
		}
		if flat {
			return listWord + "(" + args + ")", true
		}
		return "[]int{" + args + "}", true
	}
	if self != "" && c.Op == self {
		args, ok := e.renderArgs(c.Args, self, flat)
		if !ok {
			return "", false
		}
		return c.Op + "(" + args + ")", true
	}
	op, ok := e.byName[c.Op]
	if !ok {
		return "", false
	}
	if op.Raw != "" {
		// An imperative function (a list reduction or a synthesized loop) runs as
		// its statements in a function literal applied to the arguments, so the loop
		// composes as an expression. A call to it stores flat as a plain call, the way
		// the body of a function that uses one is serialized; the function's own raw
		// body is stored separately by DefSource, not inlined here.
		args, ok := e.renderArgs(c.Args, self, flat)
		if !ok {
			return "", false
		}
		if flat {
			return c.Op + "(" + args + ")", true
		}
		return "(func(" + op.sigParams() + ") " + goType(op.Ret) + " { " + op.Raw + " })(" + args + ")", true
	}
	if e.recursive(op) {
		if flat {
			args, ok := e.renderArgs(c.Args, self, flat)
			if !ok {
				return "", false
			}
			return c.Op + "(" + args + ")", true
		}
		return e.genRecursiveCall(op, c.Args, self)
	}
	if op.Body != nil {
		// The written form keeps a defined operation as a call, so a function
		// synthesized by composing acquired vocabulary is stored and shown in those
		// terms ("square(square(number))") rather than flattened to arithmetic. The
		// runnable form inlines it instead, since the runtime carries no declaration
		// to call.
		if flat {
			args, ok := e.renderArgs(c.Args, self, flat)
			if !ok {
				return "", false
			}
			return c.Op + "(" + args + ")", true
		}
		inlined, ok := e.inline(op, c.Args)
		if !ok {
			return "", false
		}
		return e.render(inlined, self, flat)
	}
	if op.isStrPrim() {
		return e.renderStrPrim(op, c.Args, self, flat)
	}
	if len(c.Args) != 2 {
		return "", false
	}
	a, aok := e.renderOperand(c.Args[0], self, flat)
	b, bok := e.renderOperand(c.Args[1], self, flat)
	if !aok || !bok {
		return "", false
	}
	return a + " " + op.Symbol + " " + b, true
}

// renderStrPrim writes a string primitive to its import-free Go form by its emit
// template: an infix join "a + b" for concat and prepend, a byte slice "a[len(a)-1:]"
// for lastChar and "a[:len(a)-1]" for dropLast. The operands go through renderOperand
// so a nested infix operand is parenthesized while a slice operand stays a primary,
// the same parenthesization the graph twin's genStrPrim applies.
func (e *Engine) renderStrPrim(op Op, args []Computation, self string, flat bool) (string, bool) {
	if sym, ok := strings.CutPrefix(op.Emit, "infix:"); ok {
		if len(args) != 2 {
			return "", false
		}
		a, aok := e.renderOperand(args[0], self, flat)
		b, bok := e.renderOperand(args[1], self, flat)
		if !aok || !bok {
			return "", false
		}
		return a + " " + sym + " " + b, true
	}
	if len(args) != 1 {
		return "", false
	}
	a, aok := e.renderOperand(args[0], self, flat)
	if !aok {
		return "", false
	}
	switch op.Emit {
	case "slice:last":
		return a + "[len(" + a + ")-1:]", true
	case "slice:init":
		return a + "[:len(" + a + ")-1]", true
	}
	return "", false
}

// renderArgs writes a call's argument list.
func (e *Engine) renderArgs(args []Computation, self string, flat bool) (string, bool) {
	parts := make([]string, len(args))
	for i, a := range args {
		s, ok := e.render(a, self, flat)
		if !ok {
			return "", false
		}
		parts[i] = s
	}
	return strings.Join(parts, ", "), true
}

// renderOperand writes an operand, wrapping a compound one in parentheses so the
// source keeps its grouping whatever the operators' precedence. A call (a
// self-call or a recursive function) and a conditional already render as a
// primary expression, a call or a parenthesized literal, so they need none.
func (e *Engine) renderOperand(c Computation, self string, flat bool) (string, bool) {
	s, ok := e.render(c, self, flat)
	if !ok {
		return "", false
	}
	if c.Op != "" && !e.rendersAsCall(c, self) {
		return "(" + s + ")", true
	}
	return s, true
}

// rendersAsCall reports whether a computation renders as a call or a
// parenthesized literal, a primary expression that needs no grouping as an
// operand: a self-call, an imperative function (a reduction or a synthesized
// loop, which renders as a parenthesized function literal applied to its
// argument), or a call to a recursive function.
func (e *Engine) rendersAsCall(c Computation, self string) bool {
	if c.Op == "" {
		return false
	}
	if c.Op == listWord || (self != "" && c.Op == self) {
		return true
	}
	op, ok := e.byName[c.Op]
	if !ok {
		return false
	}
	if op.isStrPrim() {
		return op.emitsPrimary() // a slice expression is a primary; an infix join is not
	}
	return op.Raw != "" || e.recursive(op)
}

// genCond writes a conditional as a parenthesized immediately-invoked function
// literal, so a choice is an expression that composes anywhere a value does and
// yields its value when run (a bare leading function literal evaluates to
// nothing). The readable function form is written by ToFunc instead.
func (e *Engine) genCond(c *Conditional, self string) (string, bool) {
	test, tok := e.genTest(c, self)
	then, thok := e.render(c.Then, self, false)
	els, eok := e.render(c.Else, self, false)
	if !tok || !thok || !eok {
		return "", false
	}
	return "(func() " + goType(e.condType(c, self)) + " { if " + test + " { return " + then + " }; return " + els + " }())", true
}

// condType is the value type a conditional yields, the type its two branches share,
// so genCond writes the immediately-invoked literal returning that type rather than a
// hardcoded int. A string cascade returns string, an arithmetic choice int. It reads
// the then branch's type with the function's own parameter types in scope (a morphology
// rule's body is inlined before it is rendered, so the branches are grounded in literals
// and env may be nil), falling back to int when the type does not resolve.
func (e *Engine) condType(c *Conditional, self string) string {
	if t, ok := e.typeOf(c.Then, e.selfEnv(self), self); ok {
		return t
	}
	return typeInt
}

// selfEnv gives the parameter types of the function currently being written, so a
// conditional rendered inside a not-yet-inlined body (ToFunc) still types its branches;
// an empty name or an unknown function yields no bindings.
func (e *Engine) selfEnv(self string) map[string]string {
	if self == "" {
		return nil
	}
	op, ok := e.byName[self]
	if !ok || len(op.Params) == 0 {
		return nil
	}
	env := make(map[string]string, len(op.Params))
	for i, p := range op.Params {
		env[p] = op.paramType(i)
	}
	return env
}

// genTest writes a conditional's comparison, "number > count", the guard the
// branch turns on.
func (e *Engine) genTest(c *Conditional, self string) (string, bool) {
	if c.Cmp != ">" && c.Cmp != "<" && c.Cmp != "==" {
		return "", false
	}
	left, lok := e.renderOperand(c.Left, self, false)
	right, rok := e.renderOperand(c.Right, self, false)
	if !lok || !rok {
		return "", false
	}
	return left + " " + c.Cmp + " " + right, true
}

// genRecursiveCall writes a call to a recursive function as a self-contained
// expression: a local variable of the function's type, assigned the function
// literal (which then calls itself through the variable), invoked on the
// arguments. Parenthesized so it yields its value and composes as an operand.
func (e *Engine) genRecursiveCall(op Op, args []Computation, outerSelf string) (string, bool) {
	if len(args) != len(op.Params) {
		return "", false
	}
	stmts, sok := e.bodyStmts(op, false) // the runnable form inlines the operations it composes
	argCode, aok := e.renderArgs(args, outerSelf, false)
	ret, rok := e.retType(op)
	if !sok || !aok || !rok {
		return "", false
	}
	rt := goType(ret)
	return "(func() " + rt + " { var " + op.Name + " " + op.funcType(rt) + "; " +
		op.Name + " = func(" + op.sigParams() + ") " + rt + " { " + stmts + " }; " +
		"return " + op.Name + "(" + argCode + ") }())", true
}

// goType is a value type's Go form.
func goType(t string) string {
	switch t {
	case typeList:
		return "[]int"
	case typeStr:
		return "string"
	}
	return "int"
}

// retType is a defined operation's return type, the type its body evaluates to,
// so a transform that yields a list is written returning a slice.
func (e *Engine) retType(op Op) (string, bool) {
	if op.Body == nil {
		return "", false
	}
	env := make(map[string]string, len(op.Params))
	for i, p := range op.Params {
		env[p] = op.paramType(i)
	}
	return e.typeOf(*op.Body, env, op.Name)
}

// sigParams writes a function's parameter list with Go types, "numbers []int".
func (op Op) sigParams() string {
	parts := make([]string, len(op.Params))
	for i, p := range op.Params {
		parts[i] = p + " " + goType(op.paramType(i))
	}
	return strings.Join(parts, ", ")
}

// funcType writes a function's type, "func([]int) int", for the local variable a
// recursive call declares; ret is its return type's Go form.
func (op Op) funcType(ret string) string {
	parts := make([]string, len(op.Params))
	for i := range op.Params {
		parts[i] = goType(op.paramType(i))
	}
	return "func(" + strings.Join(parts, ", ") + ") " + ret
}

// bodyStmts writes the statements inside a function's braces, the function's own
// name in scope so a self-call is a plain call: a conditional becomes the branch
// form "if test { return then }; return else", any other body a single return.
func (e *Engine) bodyStmts(op Op, flat bool) (string, bool) {
	if op.Body.Cond != nil {
		c := op.Body.Cond
		test, tok := e.genTest(c, op.Name)
		then, thok := e.render(c.Then, op.Name, flat)
		els, eok := e.render(c.Else, op.Name, flat)
		if !tok || !thok || !eok {
			return "", false
		}
		return "if " + test + " { return " + then + " }; return " + els, true
	}
	body, ok := e.render(*op.Body, op.Name, flat)
	if !ok {
		return "", false
	}
	return "return " + body, true
}

// inline binds a defined operation's arguments to its parameters and returns its
// body with the bindings substituted, the call replaced by what it computes.
func (e *Engine) inline(op Op, args []Computation) (Computation, bool) {
	if op.Body == nil || len(args) != len(op.Params) {
		return Computation{}, false
	}
	binding := make(map[string]Computation, len(op.Params))
	for i, p := range op.Params {
		binding[p] = args[i]
	}
	return substitute(*op.Body, binding), true
}

// substitute replaces each parameter reference in a body with the argument bound
// to it, leaving literals and operations otherwise intact and descending through
// a conditional's comparison and both branches.
func substitute(c Computation, binding map[string]Computation) Computation {
	if c.Map != nil {
		return Computation{Map: &Mapping{
			Var:  c.Map.Var,
			List: substitute(c.Map.List, binding),
			Body: substitute(c.Map.Body, binding),
		}}
	}
	if c.Cond != nil {
		return Computation{Cond: &Conditional{
			Cmp:   c.Cond.Cmp,
			Left:  substitute(c.Cond.Left, binding),
			Right: substitute(c.Cond.Right, binding),
			Then:  substitute(c.Cond.Then, binding),
			Else:  substitute(c.Cond.Else, binding),
		}}
	}
	if c.Var != "" {
		if v, ok := binding[c.Var]; ok {
			return v
		}
		return c
	}
	if c.Op == "" {
		return c
	}
	args := make([]Computation, len(c.Args))
	for i, a := range c.Args {
		args[i] = substitute(a, binding)
	}
	return Computation{Op: c.Op, Args: args}
}

// ToFunc writes a defined operation as the Go function it means: "double" is
// written "func double(number int) int { return number * 2 }", a conditional as
// the branch a person reads, and a recursive one calls itself by name. The
// function's own name is in scope as the body is written, so a self-call is a
// plain call. A primitive has no body to write and fails.
func (e *Engine) ToFunc(name string) (string, bool) {
	op, ok := e.byName[name]
	if !ok {
		return "", false
	}
	if op.Raw != "" { // a synthesized function is its statements as written
		return "func " + name + "(" + op.sigParams() + ") " + goType(op.Ret) + " { " + op.Raw + " }", true
	}
	if op.Body == nil {
		return "", false
	}
	// The written form keeps the operations a body composes as calls; a map has no
	// such call form (it is stored structurally), so its runnable form is written.
	stmts, ok := e.bodyStmts(op, true)
	if !ok {
		stmts, ok = e.bodyStmts(op, false)
	}
	ret, rok := e.retType(op)
	if !ok || !rok {
		return "", false
	}
	return "func " + name + "(" + op.sigParams() + ") " + goType(ret) + " { " + stmts + " }", true
}

// DefSource reads a defined operation back into the surface it is stored as, the
// parameters and the body, so the daemon can persist a learned function as a
// construction. An arithmetic body is stored as the code it generates, which
// reads back through the Go parser. A conditional is stored in a structured form,
// "if <cmp> ; <left> ; <right> ; <then> ; <else>", each part code, so the branch
// reloads without the parser having to read a function literal back. A self-call
// is kept as a call, "factorial(number - 1)", so a recursive body reads back as
// the call it is rather than expanding.
func (e *Engine) DefSource(op Op) (params []string, body string, ok bool) {
	if len(op.Params) == 0 {
		return nil, "", false
	}
	if op.Raw != "" { // a synthesized function stores its return type and statements
		return op.Params, rawDefPrefix + op.Ret + " " + op.Raw, true
	}
	if op.Body == nil {
		return nil, "", false
	}
	body, ok = e.bodySource(*op.Body, op.Name)
	if !ok {
		return nil, "", false
	}
	return op.Params, body, true
}

// condBodyPrefix marks a stored body as a conditional, ahead of its parts;
// mapBodyPrefix marks one as a map; morphBodyPrefix marks one as a string /
// conditional cascade stored structurally.
const condBodyPrefix = "if "
const mapBodyPrefix = "map "
const morphBodyPrefix = "morph "

// isStringBody reports whether a computation runs over the string primitives, so a
// morphology rule (a nested conditional cascade whose leaves are string primitives) is
// serialized structurally rather than as re-parsed Go text. Re-parsing "a + b" would
// misread a concat as add, and a nested conditional cannot store flat, so a string body
// takes the structural morph encoding instead.
func (e *Engine) isStringBody(c Computation) bool {
	if c.Cond != nil {
		return e.isStringBody(c.Cond.Left) || e.isStringBody(c.Cond.Right) ||
			e.isStringBody(c.Cond.Then) || e.isStringBody(c.Cond.Else)
	}
	if c.Op != "" && c.Op != listWord {
		if op, ok := e.byName[c.Op]; ok && op.isStrPrim() {
			return true
		}
	}
	for _, a := range c.Args {
		if e.isStringBody(a) {
			return true
		}
	}
	return false
}

// morphEncode serializes a string / conditional computation as a parenthesized
// s-expression, so a nested morphology rule round-trips through storage without the Go
// parser (where "a + b" would misread concat as add and a nested conditional could not
// store flat). A conditional is (if <cmp> left right then else), an operation
// (op <name> args...), a parameter reference (var <name>), a literal (lit <quoted>); the
// quoted literal keeps its quotes, and a space inside it (the TR "daha ") is preserved by
// the tokenizer that reads it back.
func morphEncode(c Computation) (string, bool) {
	switch {
	case c.Map != nil || c.Each:
		return "", false // a morphology body carries no map or element reference
	case c.Cond != nil:
		l, lok := morphEncode(c.Cond.Left)
		r, rok := morphEncode(c.Cond.Right)
		t, tok := morphEncode(c.Cond.Then)
		el, eok := morphEncode(c.Cond.Else)
		if !lok || !rok || !tok || !eok {
			return "", false
		}
		return "(if " + c.Cond.Cmp + " " + l + " " + r + " " + t + " " + el + ")", true
	case c.Var != "":
		return "(var " + c.Var + ")", true
	case c.Op == "":
		if c.Lit == "" {
			return "", false
		}
		return "(lit " + c.Lit + ")", true
	default:
		parts := make([]string, 0, len(c.Args)+2)
		parts = append(parts, "op", c.Op)
		for _, a := range c.Args {
			s, ok := morphEncode(a)
			if !ok {
				return "", false
			}
			parts = append(parts, s)
		}
		return "(" + strings.Join(parts, " ") + ")", true
	}
}

// morphTokens splits a morph s-expression into tokens: the parentheses, a quoted string
// as one token (its inner space kept), and a bare identifier or symbol. A literal in a
// morphology rule carries no embedded quote or paren, so the tokenizer needs no escaping.
func morphTokens(s string) []string {
	var toks []string
	for i := 0; i < len(s); {
		switch ch := s[i]; {
		case ch == ' ':
			i++
		case ch == '(' || ch == ')':
			toks = append(toks, string(ch))
			i++
		case ch == '"':
			j := i + 1
			for j < len(s) && s[j] != '"' {
				j++
			}
			if j < len(s) {
				j++ // include the closing quote
			}
			toks = append(toks, s[i:j])
			i = j
		default:
			j := i
			for j < len(s) && s[j] != ' ' && s[j] != '(' && s[j] != ')' {
				j++
			}
			toks = append(toks, s[i:j])
			i = j
		}
	}
	return toks
}

// morphDecode reads one morph s-expression node from toks at pos, returning the
// computation, the position past it, and ok, the inverse of morphEncode. It is a plain
// recursive descent over the (head ...) forms morphEncode writes.
func morphDecode(toks []string, pos int) (Computation, int, bool) {
	if pos >= len(toks) || toks[pos] != "(" {
		return Computation{}, pos, false
	}
	pos++ // consume "("
	if pos >= len(toks) {
		return Computation{}, pos, false
	}
	head := toks[pos]
	pos++
	switch head {
	case "lit":
		if pos+1 >= len(toks) || toks[pos+1] != ")" {
			return Computation{}, pos, false
		}
		lit := toks[pos]
		return Computation{Lit: lit}, pos + 2, true
	case "var":
		if pos+1 >= len(toks) || toks[pos+1] != ")" {
			return Computation{}, pos, false
		}
		name := toks[pos]
		return Computation{Var: name}, pos + 2, true
	case "if":
		if pos >= len(toks) {
			return Computation{}, pos, false
		}
		cmp := toks[pos]
		pos++
		sides := make([]Computation, 4)
		for i := 0; i < 4; i++ {
			c, np, ok := morphDecode(toks, pos)
			if !ok {
				return Computation{}, pos, false
			}
			sides[i], pos = c, np
		}
		if pos >= len(toks) || toks[pos] != ")" {
			return Computation{}, pos, false
		}
		return Computation{Cond: &Conditional{Cmp: cmp, Left: sides[0], Right: sides[1], Then: sides[2], Else: sides[3]}}, pos + 1, true
	case "op":
		if pos >= len(toks) {
			return Computation{}, pos, false
		}
		name := toks[pos]
		pos++
		var args []Computation
		for pos < len(toks) && toks[pos] != ")" {
			c, np, ok := morphDecode(toks, pos)
			if !ok {
				return Computation{}, pos, false
			}
			args = append(args, c)
			pos = np
		}
		if pos >= len(toks) {
			return Computation{}, pos, false
		}
		return Computation{Op: name, Args: args}, pos + 1, true
	}
	return Computation{}, pos, false
}

// bodySep separates a stored body's parts, a sequence no arithmetic body contains
// so the parts split back cleanly.
const bodySep = " ; "

// bodySource serializes a body for storage with the function's own name in scope:
// arithmetic and inlined calls as code, a conditional and a map as the structured
// forms bodyFromSource reads back, and a self-call kept as a call.
func (e *Engine) bodySource(c Computation, self string) (string, bool) {
	if e.isStringBody(c) {
		enc, ok := morphEncode(c)
		if !ok {
			return "", false
		}
		return morphBodyPrefix + enc, true
	}
	switch {
	case c.Map != nil:
		list, lok := e.render(c.Map.List, self, true)
		body, bok := e.render(c.Map.Body, self, true)
		if !lok || !bok {
			return "", false
		}
		return mapBodyPrefix + strings.Join([]string{c.Map.Var, list, body}, bodySep), true
	case c.Cond != nil:
		parts := []Computation{c.Cond.Left, c.Cond.Right, c.Cond.Then, c.Cond.Else}
		codes := []string{c.Cond.Cmp}
		for _, p := range parts {
			s, ok := e.render(p, self, true)
			if !ok {
				return "", false
			}
			codes = append(codes, s)
		}
		return condBodyPrefix + strings.Join(codes, bodySep), true
	default:
		return e.render(c, self, true)
	}
}

// bodyFromSource reads a stored body back into its computation, the inverse of
// bodySource: a conditional or a map from its structured parts, anything else
// through the Go parser as arithmetic.
func (e *Engine) bodyFromSource(body string) (Computation, bool) {
	if rest, ok := strings.CutPrefix(body, morphBodyPrefix); ok {
		toks := morphTokens(rest)
		c, pos, ok := morphDecode(toks, 0)
		if !ok || pos != len(toks) {
			return Computation{}, false
		}
		return c, true
	}
	if rest, ok := strings.CutPrefix(body, mapBodyPrefix); ok {
		parts := strings.SplitN(rest, bodySep, 3)
		if len(parts) != 3 || parts[0] == "" {
			return Computation{}, false
		}
		list, lok := e.comprehendCode(parts[1])
		inner, bok := e.comprehendCode(parts[2])
		if !lok || !bok {
			return Computation{}, false
		}
		return Computation{Map: &Mapping{Var: parts[0], List: list, Body: inner}}, true
	}
	rest, isCond := strings.CutPrefix(body, condBodyPrefix)
	if !isCond {
		return e.comprehendCode(body)
	}
	parts := strings.Split(rest, bodySep)
	if len(parts) != 5 {
		return Computation{}, false
	}
	cmp := parts[0]
	if cmp != ">" && cmp != "<" && cmp != "==" {
		return Computation{}, false
	}
	sides := make([]Computation, 4)
	for i, p := range parts[1:] {
		c, ok := e.comprehendCode(p)
		if !ok {
			return Computation{}, false
		}
		sides[i] = c
	}
	return Computation{Cond: &Conditional{Cmp: cmp, Left: sides[0], Right: sides[1], Then: sides[2], Else: sides[3]}}, true
}

// ToLiteral generates the teaching surface from a computation, the meaning pole
// run the other way: add(6, 7) becomes "the sum of 6 and 7", and double(6)
// becomes "the double of 6".
func (e *Engine) ToLiteral(c Computation) (string, bool) {
	if c.Var != "" {
		return "the " + c.Var, true
	}
	if c.Op == "" {
		return c.Lit, c.Lit != ""
	}
	op, ok := e.byName[c.Op]
	if !ok || len(c.Args) != op.arity() {
		return "", false
	}
	parts := make([]string, len(c.Args))
	for i, a := range c.Args {
		s, ok := e.ToLiteral(a)
		if !ok {
			return "", false
		}
		parts[i] = s
	}
	return "the " + op.Word + " of " + strings.Join(parts, " and "), true
}
