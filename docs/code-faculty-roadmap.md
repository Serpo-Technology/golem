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

## Rails, reach, and the one limit

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

### Reach: what teaching covers

There is no kind of code the faculty is barred from writing or grounding.
Capability is a function of acquired knowledge, of two sorts.

- *Conceptual / necessary* knowledge composes and emerges. The logic of an
  algorithm (apply the chosen operation, dispatch, recurse, fold over a
  sequence) follows from grounded concepts the way a reasoned conclusion does.
  This is the part the faculty does well and verifiably, and it is the hard,
  valuable part.
- *Conventional / contingent* knowledge must be acquired; it does not follow from
  the concepts. That parsing a number is spelled `strconv.Atoi` with signature
  `(int, error)`, that output goes through `fmt.Println`, the package and import
  shape: no depth of understanding what parsing *is* yields the name. This is
  vocabulary, and there is a lot of it for code that touches the world. A
  statistical writer did not derive it either; it memorized it from a huge
  corpus. The gap is dictionary size, and it moves outward as GOLEM is taught or
  reads and grounds libraries itself.

So a program is emergent logic wrapped in acquired vocabulary. An algorithm is
mostly logic and little vocabulary, so it emerges well; application code is
vocabulary-heavy, so it leans on how much has been acquired.

### Grounding the messy cases is also teachable

"Run it and check a fixed output" is one grounding method among several, and the
rest are procedures the faculty can hold and apply:

- randomness: seed the generator for a reproducible run, or check an invariant
  that holds whatever the value (a shuffle is still a permutation), or run many
  times and check the distribution;
- time: inject a fixed clock;
- filesystem and IO: run in a sandbox and read the effect back, the percept being
  the observed state change;
- a network or external service: stub it with a fixture and verify the code
  against that.

Side effects and non-determinism do not put code out of reach; they change
*which* grounding procedure applies, and the procedure is knowledge like any
other.

### Reducing the volume

Hand-teaching every library fact does not scale. The path that does, and that
fits the self-build aim, is for GOLEM to ground a library function by reading its
Go source and running it, the way it reads any expression. That turns "teach ten
thousand APIs" into "acquire them by reading": still acquisition, far more
scalable, still grounded.

### The one limit teaching cannot pre-empt

Verification is after the fact. A model is built from outcomes already
experienced; the first time a genuinely novel outcome occurs (a clock that runs
backward, an undocumented status), it is met by the model held *before* it, and
may be handled wrongly. The update happens *from* that encounter and improves the
next run; the run that taught it has already happened. This after-the-fact
learning binds every learner, human, statistical, or grounded, the same way.

GOLEM is not rigid here: it revises from surprise online (surprise drives
encoding, concepts abstract from instances, beliefs are defeasible and get
overturned by observation), which a frozen-weights model cannot do at runtime.
Nor is this a missing fact: the outcome had not happened yet, so there was
nothing to know. Judging a fresh outcome as a real new case or as noise is the
same after-the-fact judgment, settled by repetition or by a reason, with the
fallibility a person has.

### The practical boundary is cost and guarantee

Nothing is permanently out of reach. What the faculty can write and ground is set
by how much vocabulary and grounding technique it has acquired, which teaching
and self-reading move outward. What it can *guarantee at a given moment* is set
by whether a run can decide correctness then; the open world can always present
an outcome that is learned only once it occurs. Within reach the faculty is more
trustworthy than a statistical writer, because every step is an explicit,
grounded, verifiable structure, where a statistical writer offers only a
confidence.

## Staged plan

| Slice | Capability | Adds to the IR | Status |
|------|------------|----------------|--------|
| 1 | Binary / n-ary user functions | parse + write n parameters | done |
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
