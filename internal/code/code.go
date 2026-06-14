// Package code is GOLEM learning to program the way it learns any language,
// through a construction with two poles: the form is the syntax, the meaning is
// the computation, and the grounding is execution, what the code does when it
// runs in the Step 35 runtime. Pseudocode is the meaning made precise, literal
// language is the teaching surface, source is the executable form, and the run
// is the percept. One engine runs the constructions both ways: comprehending a
// surface yields its computation, and generating from a computation yields the
// surface, so GOLEM reads and writes programs from the one inventory.
//
// This is the bootstrap of a programming language, a few arithmetic
// constructions seeded as data and competing by weight like the rest of the
// grammar. Growing it to a language that comprehends arbitrary source is the
// long arc; the principle, code as grounded form-meaning constructions run both
// ways, is what is fixed here.
package code

import "strings"

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

// Comprehend reads a surface into its computation, the form pole run to the
// meaning. A literal teaching surface ("the sum of 6 and 7") and a code surface
// ("6 + 7") both yield the same computation, so GOLEM understands a program
// however it is given. It returns false when no construction matches.
func (e *Engine) Comprehend(text string) (Computation, bool) {
	if c, ok := e.comprehendLiteral(strings.Fields(text)); ok {
		return c, true
	}
	return e.comprehendCode(strings.Fields(text))
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

// comprehendCode reads the executable surface "<a> <symbol> <b>".
func (e *Engine) comprehendCode(toks []string) (Computation, bool) {
	if len(toks) != 3 {
		return Computation{}, false
	}
	op, ok := e.bySymbol[toks[1]]
	if !ok {
		return Computation{}, false
	}
	return Computation{Op: op.Name, Args: []Computation{{Lit: toks[0]}, {Lit: toks[2]}}}, true
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
	a, aok := e.ToCode(c.Args[0])
	b, bok := e.ToCode(c.Args[1])
	if !aok || !bok {
		return "", false
	}
	return a + " " + op.Symbol + " " + b, true
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
