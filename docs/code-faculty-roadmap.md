# Code faculty roadmap: toward grounded, meaningful code writing

The plan for taking GOLEM from arithmetic and unary functions to composing and
running real procedures, and eventually writing code that serves a goal stated in
language. This is a design doc: the staged slices below carry labels and a status
table, which belong here and nowhere else (not in commit messages, identifiers,
or code comments).

## Framing: the parser already provides the syntax

GOLEM does not learn Go syntax token by token. The code faculty reads real Go
through Go's own parser (`go/parser`), which is the innate syntax floor of the
faculty, the role the grammar engine plays for natural language. The whole Go
grammar is therefore already available. What the faculty *learns* is the meaning
pole: which computation each word or operator names, grounded by running it. So
the work is to grow the inventory of grounded constructions, grow the meaning
representation they compile to, and connect both to what GOLEM knows, never to
"teach syntax".

## Current state

Implemented in `internal/code` (`code.go`, `induce.go`, `function.go`,
`source.go`):

- Arithmetic primitives (add/subtract/multiply/divide), seeded as constructions.
- Unary user functions, told in language ("the double of a number is the product
  of the number and 2"), grounded by running and written as Go
  (`func double(number int) int { return number * 2 }`).
- Composition and nesting ("the square of the double of 3").
- One-step by-example induction ("the double of 3 is 6 and the double of 5 is
  10").
- Pseudocode (operator words: "6 plus 7").
- Honest gaps: an undefined function or an ill-formed computation admits the gap
  instead of leaking a compiler error.

The meaning representation (`code.Computation`) is an expression tree:
`{Op, Lit, Var, Args}`. A function (`code.Op`) is `{Name, Symbol, Word, Params,
Body}`, written by `ToFunc` as a single `return` expression over `int`
parameters.

### The ceiling

The ceiling is the meaning representation, which is expression-only:

- no statements, no variables or assignment, no sequencing;
- no control flow (conditionals, loops);
- no recursion;
- one type (`int`), no data structures (lists, maps, structs);
- user functions are unary (`Op.Params` is a slice, but the definition/call
  parse and the Go-writing handle one parameter).

"Meaningful code" needs this representation to grow. Each step below extends the
representation, adds comprehension (surface to meaning) and generation (meaning
to Go), and grounds the new construction by running it.

## Rails and the scope boundary

Every slice holds the project's rails:

- **Grounded by running.** A construction is grounded by executing it and reading
  the result back as a percept. This is the faculty's decisive advantage:
  correctness is decided by running the code, where a statistical writer can
  only resemble its training text.
- **Glass box.** The meaning is an explicit, auditable structure; the Go is
  generated from it; the run is recorded on the trail. No opaque step.
- **No hardcoding.** A capability is a learned or seeded construction plus the
  engine that runs it both ways, never a per-case matcher in the daemon. The test
  for each slice: a *new* function or pattern of the same shape works without new
  code.
- **Language decoupled.** The code faculty composes with comprehension in either
  tongue; growing it must not regress the conversational or reasoning faculties
  (the cross-tongue litmus stays green).
- **Honesty.** What cannot be computed or grounded is admitted, never faked.

**Scope boundary (the volume wall).** The grounded path fits problems where
running the code decides correctness: algorithmic code over well-defined data
(arithmetic, recursion, search, sort, transforms). Arbitrary application or
library code leans on vast API and convention knowledge that a low-volume,
grounded system does not hold by design. The target is GOLEM composing and
grounding algorithmic procedures from intent; writing arbitrary software stays
out of scope. Within that target it can be more trustworthy than a statistical
writer, because every step is grounded and verified.

## Staged plan

| Slice | Capability | Adds to the IR | Status |
|------|------------|----------------|--------|
| 1 | Binary / n-ary user functions | parse + write n parameters | not started |
| 2 | Multi-step induction | wider hypothesis search | not started |
| 3 | Conditionals | a branch node | not started |
| 4 | Recursion | a function decl with a self-call + base case | not started |
| 5 | Data and types | typed values, sequences | not started |
| 6 | Sequencing and iteration | statements; loops or fold | not started |
| 7 | Intent to procedure | a spec, decomposition, verification | not started |

### Slice 1: binary and n-ary user functions

"The X of A and B." `Op.Params` is already a slice; the definition parser, the
call parser, and `ToFunc` assume one parameter. Extend all three to n parameters.
Grounded by running ("the sum-with-double of 3 and 4"). Small, self-contained,
the flagged next feature.

### Slice 2: multi-step induction

Today induction finds a one-step rule, so cube (`n*n*n`) is correctly declined.
Widen the hypothesis search to compose two or more steps (and constants) under
the grounding check (a candidate is kept only if it runs correctly over every
example pair). Reuses the existing ground-by-running validation.

### Slice 3: conditionals

The first control flow. A construction like "the larger of A and B", or a told
rule with a condition, comprehends to a branch node in the IR and writes an `if`
(or a conditional expression). Grounded by running over cases that exercise both
branches. Decision to settle: surface form (a function over a comparison vs a
told "if ... then ..." construction).

### Slice 4: recursion

The first construction that needs a real procedure: a named function that calls
itself with a base case (factorial, fibonacci). The IR grows from expression-only
to a function declaration with a body that can reference its own name. `ToFunc`
writes the recursive form; grounding runs it on small inputs and on the base
case. This is the slice where "writes a program" becomes literally true.

### Slice 5: data and types

Typed values beyond `int` and at least one sequence type. "The sum of a list",
"the length of a list", "the first of a list". The IR gains typed operands and a
sequence value; generation writes the slice/loop or a reduction. Opens the door
to transforms over collections.

### Slice 6: sequencing and iteration

Multiple statements, local bindings, and bounded iteration. A design choice to
make with the architect: explicit loops, or express iteration as recursion plus
map/fold over slice 5's sequences (the second keeps the IR smaller and more
groundable). Either way, grounded by running.

### Slice 7: intent to procedure (the bridge)

The frontier. A goal stated in language is comprehended to a spec (examples,
or a relation over input and output), decomposed using the reasoning graph and
the construction inventory into a candidate procedure, emitted as code, grounded
by running, and checked against the spec. This is the association the goal asks
for: connecting "what to compute" (intent, world knowledge) to "how to compute
it" (constructions). It depends on slices 1 to 6 and is research-grade; it earns
its own design pass once the IR can express procedures.

## Per-slice invariants

Before any slice is called done:

- green `go test ./...` and the linux arm64 cross-build;
- the cross-tongue litmus and the reasoning frontier unregressed;
- a grounded-by-running demo of the new capability, plus an honest gap for the
  case it cannot yet handle;
- the no-hardcoding check (a fresh instance of the same shape works without new
  code);
- one commit, describing the change by what it does (no slice label).

Verify compute on a throwaway brain, never on a checkpoint: a run grounds a
`ch:code` percept whose digits normalize to a `<n>` node, which pollutes a brain
you intend to snapshot.

## Open questions to settle per slice

- Slice 3: conditional as a function over a comparison, or a told "if/then"
  construction?
- Slice 5: which sequence type, and how do types enter the surface (inferred, or
  named)?
- Slice 6: explicit loops, or recursion plus fold over sequences?
- Slice 7: how is a spec represented (examples vs a relation), and what counts as
  the procedure being verified against the goal?
