#!/usr/bin/env zsh
# GOLEM learns an abstract concept it can never perceive: a language model. None
# of these words name a percept -- model, llm, word, pattern, text -- so each
# enters the graph through the telling itself, minted the way an unknown verb is,
# and takes its place from the company it keeps. A model and an llm are used
# alike here: both predict, both learn, both read. That shared use is the only
# signal that places them, and it places them near, so a fact told of one carries
# to the other as a hedged guess. This is what an abstract word's meaning is to a
# grounded mind: not a picture, but a position among the things it acts on.
#
# It builds on an existing world: restore the latest snapshot first, so the
# concept lands in a brain that already holds its animals, foods, and the verbs
# of mind.
#
#   GOLEM=golem zsh llm-concept.zsh
set -u
GOLEM=${GOLEM:-golem}
GAP=${GAP:-2}

log()  { print -r -- "$(date +%H:%M:%S) $*" }
sayw() { local i; for i in 1 2 3; do $GOLEM tell "$1" >/dev/null 2>&1 && { sleep 0.2; return 0 }; sleep 1; done; return 0 }
fact() { log "fact: $1"; sayw "$1"; sleep $GAP }
nap()  { log "(nap: consolidate)"; $GOLEM sleep >/dev/null 2>&1; sleep "${BLOCKGAP:-8}" }

log "=== what a language model is: predict, learn, read ==="

# Both the model and the llm predict a word: the first shared use, the column
# that starts to draw the two together in the usage space.
fact "a model predicts a word"
fact "an llm predicts a word"

# Both learn a pattern: a second shared use, deepening the likeness.
fact "a model learns a pattern"
fact "an llm learns a pattern"

# Both read a text: a third. With three uses in common and no feature between
# them, the model and the llm now sit near in the usage space, near enough that
# one stands in for the other.
fact "a model reads a text"
fact "an llm reads a text"
nap

# One thing told of only the model, one of only the llm. Neither is ever told of
# the other, so each is the ground for an inference: does the llm learn a rule
# (the model does), does the model read a sentence (the llm does). The hedged yes
# rides the usage nearness the three shared uses built.
fact "a model learns a rule"
fact "an llm reads a sentence"
nap

log "=== done: the language model concept taught ==="
