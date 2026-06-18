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

// Op is one code construction: an operation paired with the surfaces it is read
// and written through. A primitive carries the operator symbol that runs it; a
// defined operation carries its parameters and the body that computes it, a
// function learned as one more construction.
type Op struct {
	Name   string       // the operation the meaning pole carries
	Symbol string       // the code form: the operator that runs it, for a primitive
	Word   string       // the literal form: the word it is taught by
	Params []string     // the parameters of a defined operation, in order
	Body   *Computation // the meaning of a defined operation over its parameters, nil for a primitive
}

// arity is how many operands the operation takes: a defined operation's is the
// count of its parameters, and a primitive is binary.
func (o Op) arity() int {
	if o.Body != nil {
		return len(o.Params)
	}
	return 2
}

// Def is a defined operation in the surface it is stored as: a name, the
// parameters it abstracts over in order, and its body as code. It is what a
// learned function persists to and is read back from, so a function survives a
// restart the way a seeded construction does.
type Def struct {
	Name   string
	Params []string
	Body   string // the body as code, e.g. "number * factor"
}

// Engine carries the code constructions and runs them both ways. It is built
// from the seeded primitives and the functions learned since, indexed by each
// face so a surface in any of them reaches the one computation.
type Engine struct {
	byWord   map[string]Op
	bySymbol map[string]Op
	byName   map[string]Op
}

// NewEngine indexes the primitive constructions, then the defined ones, whose
// bodies are read with the primitives already in hand so a function built from
// them comprehends. A definition whose body does not comprehend is dropped
// rather than indexed half-formed.
func NewEngine(prims []Op, defs []Def) *Engine {
	e := &Engine{byWord: map[string]Op{}, bySymbol: map[string]Op{}, byName: map[string]Op{}}
	for _, o := range prims {
		e.byWord[o.Word] = o
		e.bySymbol[o.Symbol] = o
		e.byName[o.Name] = o
	}
	for _, d := range defs {
		body, ok := e.bodyFromSource(d.Body)
		if !ok {
			continue
		}
		op := Op{Name: d.Name, Word: d.Name, Params: d.Params, Body: &body}
		e.byWord[op.Word] = op
		e.byName[op.Name] = op
	}
	return e
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
		if _, ok := e.byWord[toks[i+1]]; !ok {
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
		if _, ok := spokenOps[strings.ToLower(t)]; ok {
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

// spokenOps maps the words a person says an operator with to the symbol that
// runs it, the pseudocode register of the same operations: "6 plus 7" is read
// as "6 + 7" and "3 times 4" as "3 * 4".
var spokenOps = map[string]string{"plus": "+", "minus": "-", "times": "*", "over": "/"}

// comprehendSpoken reads the operator words into their symbols and lets the Go
// parser take it from there, so a computation spoken in words carries the same
// precedence and grouping the symbols would. With no operator word present it
// declines, so the code path reads a plain expression unchanged.
func (e *Engine) comprehendSpoken(text string) (Computation, bool) {
	toks := strings.Fields(text)
	replaced := false
	for i, t := range toks {
		if sym, ok := spokenOps[strings.ToLower(t)]; ok {
			toks[i] = sym
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
	c, n, ok := e.literalOperand(toks, nil)
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
// lets a function call sit where a number would.
func (e *Engine) literalOperand(toks []string, params []string) (Computation, int, bool) {
	if len(toks) == 0 {
		return Computation{}, 0, false
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
	op, ok := e.byWord[toks[1]]
	if !ok || op.arity() < 1 {
		return Computation{}, 0, false
	}
	consumed := 3 // "the <word> of"
	rest := toks[3:]
	args := make([]Computation, 0, op.arity())
	for i := 0; i < op.arity(); i++ {
		if i > 0 {
			if len(rest) == 0 || rest[0] != "and" {
				return Computation{}, 0, false
			}
			rest = rest[1:]
			consumed++
		}
		arg, n, ok := e.literalOperand(rest, params)
		if !ok {
			return Computation{}, 0, false
		}
		args = append(args, arg)
		rest = rest[n:]
		consumed += n
	}
	return Computation{Op: op.Name, Args: args}, consumed, true
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
	if len(toks) < 7 || toks[0] != "the" || toks[2] != "of" {
		return Op{}, false
	}
	name := toks[1]
	// Read the parameters: one or more "a <param>" groups joined by "and", up to
	// "is", which opens the body.
	var params []string
	i := 3
	for {
		if i+1 >= len(toks) || (toks[i] != "a" && toks[i] != "an") {
			return Op{}, false
		}
		params = append(params, toks[i+1])
		i += 2
		if i >= len(toks) {
			return Op{}, false
		}
		if toks[i] == "is" {
			i++
			break
		}
		if toks[i] != "and" {
			return Op{}, false
		}
		i++
	}
	bodyStart := i
	if name == "" || len(params) == 0 || bodyStart >= len(toks) {
		return Op{}, false
	}
	// A name may not shadow a seeded primitive, by its name or its word.
	if op, taken := e.byName[name]; taken && op.Body == nil {
		return Op{}, false
	}
	if op, taken := e.byWord[name]; taken && op.Body == nil {
		return Op{}, false
	}
	// Each parameter must be distinct, not the name, and not an operation word the
	// body would misread.
	seen := map[string]bool{}
	for _, p := range params {
		if p == "" || p == name || seen[p] {
			return Op{}, false
		}
		seen[p] = true
		if _, taken := e.byWord[p]; taken {
			return Op{}, false
		}
	}
	body, ok := e.parseBody(toks[bodyStart:], params)
	if !ok {
		return Op{}, false
	}
	// The body must be a genuine function of every parameter, with no dead ones.
	for _, p := range params {
		if !usesVar(body, p) {
			return Op{}, false
		}
	}
	return Op{Name: name, Word: name, Params: params, Body: &body}, true
}

// parseBody reads a function body with the parameters in scope: a conditional
// "<then> if <cond> else <else>", or a plain arithmetic computation. It tolerates
// a comma before "else" and fails unless the body comprehends and consumes every
// token, so a half-formed body is not half-learned.
func (e *Engine) parseBody(toks, params []string) (Computation, bool) {
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
		body, n, ok := e.literalOperand(clean, params)
		if !ok || n != len(clean) {
			return Computation{}, false
		}
		return body, true
	}
	if ifAt == 0 || elseAt <= ifAt+1 || elseAt == len(clean)-1 {
		return Computation{}, false
	}
	then, n, ok := e.literalOperand(clean[:ifAt], params)
	if !ok || n != ifAt {
		return Computation{}, false
	}
	cond, ok := e.comparison(clean[ifAt+1:elseAt], params)
	if !ok {
		return Computation{}, false
	}
	els, m, ok := e.literalOperand(clean[elseAt+1:], params)
	if !ok || m != len(clean)-elseAt-1 {
		return Computation{}, false
	}
	cond.Then, cond.Else = then, els
	return Computation{Cond: cond}, true
}

// comparison reads a conditional's test into the operator and its operands: "<A>
// is above <B>" is A > B, "<A> is below <B>" is A < B, and a bare "<A> is <B>" is
// equality. The branches are left empty for parseBody to fill.
func (e *Engine) comparison(toks, params []string) (*Conditional, bool) {
	left, n, ok := e.literalOperand(toks, params)
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
	right, m, ok := e.literalOperand(rest, params)
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
	default:
		return Computation{}, false
	}
}

// ToCode generates the executable form from a computation, the meaning pole run
// to source: add(6, 7) becomes "6 + 7". A call of a defined operation is inlined,
// its body with the arguments bound to its parameters, so the source runs in the
// runtime without a function declaration to carry along.
func (e *Engine) ToCode(c Computation) (string, bool) {
	if c.Cond != nil {
		return e.condCode(c.Cond)
	}
	if c.Var != "" {
		return c.Var, true
	}
	if c.Op == "" {
		return c.Lit, c.Lit != ""
	}
	op, ok := e.byName[c.Op]
	if !ok {
		return "", false
	}
	if op.Body != nil {
		inlined, ok := e.inline(op, c.Args)
		if !ok {
			return "", false
		}
		return e.ToCode(inlined)
	}
	if len(c.Args) != 2 {
		return "", false
	}
	a, aok := e.codeOperand(c.Args[0])
	b, bok := e.codeOperand(c.Args[1])
	if !aok || !bok {
		return "", false
	}
	return a + " " + op.Symbol + " " + b, true
}

// condCode writes a conditional as an immediately-invoked function literal, so a
// choice is an expression that composes anywhere a value does ("the double of the
// larger of 3 and 4") and runs in the interpreter like any other expression. The
// readable function form is written by ToFunc instead, for the declaration a
// person reads.
func (e *Engine) condCode(c *Conditional) (string, bool) {
	test, ok := e.condTest(c)
	if !ok {
		return "", false
	}
	then, tok := e.ToCode(c.Then)
	els, eok := e.ToCode(c.Else)
	if !tok || !eok {
		return "", false
	}
	// Parenthesized so the call yields its value as an expression (a bare leading
	// function literal evaluates to nothing) and composes safely as an operand.
	return "(func() int { if " + test + " { return " + then + " }; return " + els + " }())", true
}

// condTest writes a conditional's comparison, "number > count", the guard the
// branch turns on. The operands are written as operands so a compared expression
// keeps its grouping.
func (e *Engine) condTest(c *Conditional) (string, bool) {
	if c.Cmp != ">" && c.Cmp != "<" && c.Cmp != "==" {
		return "", false
	}
	left, lok := e.codeOperand(c.Left)
	right, rok := e.codeOperand(c.Right)
	if !lok || !rok {
		return "", false
	}
	return left + " " + c.Cmp + " " + right, true
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

// codeOperand generates an operand, wrapping a compound one in parentheses so
// the source it makes carries the same grouping as the computation does and runs
// to the same value whatever the operators' precedence. A call of a defined
// operation is compound too, wrapped around the body it inlines to.
func (e *Engine) codeOperand(c Computation) (string, bool) {
	s, ok := e.ToCode(c)
	if !ok {
		return "", false
	}
	if c.Op != "" {
		return "(" + s + ")", true
	}
	// A conditional already renders as a parenthesized function-literal call, so it
	// is safe as an operand without another pair of parentheses.
	return s, true
}

// ToFunc writes a defined operation as the Go function it means: its body becomes
// the return expression over its parameters, so "double" is written "func
// double(number int) int { return number * 2 }". A conditional body is written as
// the branch a person reads, "if number > count { return number }; return count",
// rather than the function-literal form ToCode composes with. A primitive has no
// body to write and fails.
func (e *Engine) ToFunc(name string) (string, bool) {
	op, ok := e.byName[name]
	if !ok || op.Body == nil {
		return "", false
	}
	params := make([]string, len(op.Params))
	for i, p := range op.Params {
		params[i] = p + " int"
	}
	sig := "func " + name + "(" + strings.Join(params, ", ") + ") int { "
	if op.Body.Cond != nil {
		c := op.Body.Cond
		test, tok := e.condTest(c)
		then, thok := e.ToCode(c.Then)
		els, eok := e.ToCode(c.Else)
		if !tok || !thok || !eok {
			return "", false
		}
		return sig + "if " + test + " { return " + then + " }; return " + els + " }", true
	}
	bodyCode, ok := e.ToCode(*op.Body)
	if !ok {
		return "", false
	}
	return sig + "return " + bodyCode + " }", true
}

// DefSource reads a defined operation back into the surface it is stored as, the
// parameters and the body, so the daemon can persist a learned function as a
// construction. An arithmetic body is stored as the code it generates, which
// reads back through the Go parser. A conditional is stored in a structured form,
// "if <cmp> ; <left> ; <right> ; <then> ; <else>", each part arithmetic code, so
// the branch reloads without the parser having to read a function literal back.
func (e *Engine) DefSource(op Op) (params []string, body string, ok bool) {
	if op.Body == nil || len(op.Params) == 0 {
		return nil, "", false
	}
	body, ok = e.bodySource(*op.Body)
	if !ok {
		return nil, "", false
	}
	return op.Params, body, true
}

// condBodyPrefix marks a stored body as a conditional, ahead of its parts.
const condBodyPrefix = "if "

// bodySep separates a stored conditional's parts, a sequence no arithmetic body
// contains so the parts split back cleanly.
const bodySep = " ; "

// bodySource serializes a body for storage: arithmetic as its code, a conditional
// as the structured form bodyFromSource reads back.
func (e *Engine) bodySource(c Computation) (string, bool) {
	if c.Cond == nil {
		return e.ToCode(c)
	}
	parts := []Computation{c.Cond.Left, c.Cond.Right, c.Cond.Then, c.Cond.Else}
	codes := make([]string, 0, len(parts)+1)
	codes = append(codes, c.Cond.Cmp)
	for _, p := range parts {
		s, ok := e.ToCode(p)
		if !ok {
			return "", false
		}
		codes = append(codes, s)
	}
	return condBodyPrefix + strings.Join(codes, bodySep), true
}

// bodyFromSource reads a stored body back into its computation, the inverse of
// bodySource: a conditional from its structured parts, anything else through the
// Go parser as arithmetic.
func (e *Engine) bodyFromSource(body string) (Computation, bool) {
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
