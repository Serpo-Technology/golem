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
// expression over an operation and its operands, a bare literal, or a reference
// to the parameter of the function being defined. It is what comprehending a
// surface yields and what generating one reads, the bridge between the literal a
// computation is taught in and the source it runs as.
type Computation struct {
	Op   string        // the operation: the construction's name, empty for a literal or a variable
	Lit  string        // the literal value, when Op and Var are empty
	Var  string        // a reference to a parameter, inside a defined operation's body
	Args []Computation // the operands of an operation
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
// parameter it abstracts over, and its body as code. It is what a learned
// function persists to and is read back from, so a function survives a restart
// the way a seeded construction does.
type Def struct {
	Name  string
	Param string
	Body  string // the body as code, e.g. "number * 2"
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
		body, ok := e.comprehendCode(d.Body)
		if !ok {
			continue
		}
		op := Op{Name: d.Name, Word: d.Name, Params: []string{d.Param}, Body: &body}
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
	c, n, ok := e.literalOperand(toks, "")
	if !ok || n != len(toks) {
		return Computation{}, false
	}
	return c, true
}

// literalOperand reads one operand from the front of toks and reports how many
// tokens it spanned. A bare token is a literal; "the <param>" inside a function
// body is a reference to the parameter; "the <word> of <A> and <B> and ..." is
// the operation its word names over as many operands as it takes, each read the
// same way, the recursion that lets the surface nest and lets a function call
// sit where a number would.
func (e *Engine) literalOperand(toks []string, param string) (Computation, int, bool) {
	if len(toks) == 0 {
		return Computation{}, 0, false
	}
	// A parameter reference, "the number", takes precedence over reading "the"
	// as the opening of an operation.
	if param != "" && len(toks) >= 2 && toks[0] == "the" && toks[1] == param {
		return Computation{Var: param}, 2, true
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
		arg, n, ok := e.literalOperand(rest, param)
		if !ok {
			return Computation{}, 0, false
		}
		args = append(args, arg)
		rest = rest[n:]
		consumed += n
	}
	return Computation{Op: op.Name, Args: args}, consumed, true
}

// ComprehendDefinition reads a function taught in language, "the double of a
// number is the product of the number and 2", into the operation it defines: the
// name is the word after "the", the parameter is the noun introduced by "a", and
// the body is the computation after "is", read with the parameter in scope so
// "the number" is its reference. The body must comprehend, must consume every
// word after "is", and must actually use the parameter, so a half-formed or
// constant "function" fails rather than being half-learned.
func (e *Engine) ComprehendDefinition(text string) (Op, bool) {
	toks := strings.Fields(text)
	if len(toks) < 7 || toks[0] != "the" || toks[2] != "of" || toks[5] != "is" {
		return Op{}, false
	}
	if toks[3] != "a" && toks[3] != "an" {
		return Op{}, false
	}
	name, param := toks[1], toks[4]
	if name == "" || param == "" || name == param {
		return Op{}, false
	}
	// A function may not shadow a seeded primitive, by its name or its word, and
	// its parameter may not be an operation word the body would misread.
	if op, taken := e.byName[name]; taken && op.Body == nil {
		return Op{}, false
	}
	if op, taken := e.byWord[name]; taken && op.Body == nil {
		return Op{}, false
	}
	if _, taken := e.byWord[param]; taken {
		return Op{}, false
	}
	body, n, ok := e.literalOperand(toks[6:], param)
	if !ok || n != len(toks)-6 || !usesVar(body, param) {
		return Op{}, false
	}
	return Op{Name: name, Word: name, Params: []string{param}, Body: &body}, true
}

// usesVar reports whether a parameter is referenced anywhere in a body, the test
// that a definition is a function of its parameter and not a constant dressed as
// one.
func usesVar(c Computation, name string) bool {
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
// to it, leaving literals and operations otherwise intact.
func substitute(c Computation, binding map[string]Computation) Computation {
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
	return s, true
}

// ToFunc writes a defined operation as the Go function it means: its body becomes
// the return expression over its parameters, so "double" is written "func
// double(number int) int { return number * 2 }". A primitive has no body to write
// and fails.
func (e *Engine) ToFunc(name string) (string, bool) {
	op, ok := e.byName[name]
	if !ok || op.Body == nil {
		return "", false
	}
	bodyCode, ok := e.ToCode(*op.Body)
	if !ok {
		return "", false
	}
	params := make([]string, len(op.Params))
	for i, p := range op.Params {
		params[i] = p + " int"
	}
	return "func " + name + "(" + strings.Join(params, ", ") + ") int { return " + bodyCode + " }", true
}

// DefSource reads a defined operation back into the surface it is stored as, the
// parameter and the body as code, so the daemon can persist a learned function
// as a construction.
func (e *Engine) DefSource(op Op) (param, body string, ok bool) {
	if op.Body == nil || len(op.Params) != 1 {
		return "", "", false
	}
	body, ok = e.ToCode(*op.Body)
	if !ok {
		return "", "", false
	}
	return op.Params[0], body, true
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
