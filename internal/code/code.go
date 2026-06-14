// Package code is GOLEM learning to program the way it learns any language,
// through a construction with two poles: the form is the syntax, the meaning is
// the computation, and the grounding is execution, what the code does when it
// runs in the Step 35 runtime. Pseudocode is the meaning made precise, literal
// language is the teaching surface, source is the executable form, and the run
// is the percept. One engine runs the constructions both ways: comprehending a
// surface yields its computation, and generating from a computation yields the
// surface, so GOLEM reads and writes programs from the one inventory.
//
// The code surface is real Go: an expression is read by the language's own
// parser, the innate syntax floor of this faculty the way the grammar engine is
// the floor for natural language, and the operation each operator names is a
// seeded construction, so the engine comprehends genuine source and grounds it
// by running. A few arithmetic constructions are seeded as data and compete by
// weight like the rest of the grammar; growing the meaning pole past
// expressions to statements and declarations is the long arc, and the
// principle, code as grounded form-meaning constructions run both ways, is what
// is fixed here.
package code

import (
	"go/ast"
	"go/parser"
	"sort"
	"strings"
)

// Computation is the meaning pole of a program, pseudocode made precise: an
// expression over an operation and its operands, or a bare literal. It is what
// comprehending a surface yields and what generating one reads, the bridge
// between the literal a computation is taught in and the source it runs as.
type Computation struct {
	Op   string        // the operation: the construction's name, empty for a literal
	Lit  string        // the literal value, when Op is empty
	Args []Computation // the operands of an operation
}

// Op is one code construction: an operation paired with the code symbol that
// runs it and the word a person teaches it by, a form-meaning pair read both
// ways like any construction.
type Op struct {
	Name   string // the operation the meaning pole carries
	Symbol string // the code form: the operator that runs it
	Word   string // the literal form: the word it is taught by
}

// Engine carries the code constructions and runs them both ways. It is built
// from the seeded constructions, indexed by each of the three faces so a
// surface in any of them reaches the one computation.
type Engine struct {
	byWord   map[string]Op
	bySymbol map[string]Op
	byName   map[string]Op
}

// NewEngine indexes a set of code constructions for bidirectional use.
func NewEngine(ops []Op) *Engine {
	e := &Engine{byWord: map[string]Op{}, bySymbol: map[string]Op{}, byName: map[string]Op{}}
	for _, o := range ops {
		e.byWord[o.Word] = o
		e.bySymbol[o.Symbol] = o
		e.byName[o.Name] = o
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
// meaning. A literal teaching surface ("the sum of 6 and 7") and a code surface
// ("6 + 7") both yield the same computation, so GOLEM understands a program
// however it is given. It returns false when no construction matches.
func (e *Engine) Comprehend(text string) (Computation, bool) {
	if c, ok := e.comprehendLiteral(strings.Fields(text)); ok {
		return c, true
	}
	return e.comprehendCode(text)
}

// comprehendLiteral reads the teaching surface "the <word> of <a> and <b>".
func (e *Engine) comprehendLiteral(toks []string) (Computation, bool) {
	if len(toks) != 6 || toks[0] != "the" || toks[2] != "of" || toks[4] != "and" {
		return Computation{}, false
	}
	op, ok := e.byWord[toks[1]]
	if !ok {
		return Computation{}, false
	}
	return Computation{Op: op.Name, Args: []Computation{{Lit: toks[3]}, {Lit: toks[5]}}}, true
}

// comprehendCode reads the executable surface as real Go: the language's own
// parser turns the source into a syntax tree, and fromExpr walks it into the
// computation, so a nested, parenthesized expression of any depth comprehends,
// not only a flat "<a> <symbol> <b>". The operator still has to be one a seeded
// construction claims, so the engine reads genuine Go but only the operations it
// has been taught.
func (e *Engine) comprehendCode(text string) (Computation, bool) {
	expr, err := parser.ParseExpr(text)
	if err != nil {
		return Computation{}, false
	}
	return e.fromExpr(expr)
}

// fromExpr walks a Go expression tree into a computation: a literal is a value,
// a parenthesized expression is its inside, a signed literal is one value, and a
// binary expression is the operation its operator names over its two sides. An
// operator no construction claims, or a shape the meaning pole does not reach
// yet, fails cleanly rather than guessing.
func (e *Engine) fromExpr(expr ast.Expr) (Computation, bool) {
	switch x := expr.(type) {
	case *ast.BasicLit:
		return Computation{Lit: x.Value}, true
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
// to source: add(6, 7) becomes "6 + 7", ready to run in the sandbox.
func (e *Engine) ToCode(c Computation) (string, bool) {
	if c.Op == "" {
		return c.Lit, c.Lit != ""
	}
	op, ok := e.byName[c.Op]
	if !ok || len(c.Args) != 2 {
		return "", false
	}
	a, aok := e.codeOperand(c.Args[0])
	b, bok := e.codeOperand(c.Args[1])
	if !aok || !bok {
		return "", false
	}
	return a + " " + op.Symbol + " " + b, true
}

// codeOperand generates an operand, wrapping a compound one in parentheses so
// the source it makes carries the same grouping as the computation does and runs
// to the same value whatever the operators' precedence.
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

// ToLiteral generates the teaching surface from a computation, the meaning pole
// run the other way: add(6, 7) becomes "the sum of 6 and 7".
func (e *Engine) ToLiteral(c Computation) (string, bool) {
	if c.Op == "" {
		return c.Lit, c.Lit != ""
	}
	op, ok := e.byName[c.Op]
	if !ok || len(c.Args) != 2 {
		return "", false
	}
	a, aok := e.ToLiteral(c.Args[0])
	b, bok := e.ToLiteral(c.Args[1])
	if !aok || !bok {
		return "", false
	}
	return "the " + op.Word + " of " + a + " and " + b, true
}
