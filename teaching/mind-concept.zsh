#!/usr/bin/env zsh
# GOLEM learns a cluster of concepts it can never perceive: the furniture of the
# mind -- idea, notion, plan, choice, question, goal, problem. None of these words
# names a percept, so each enters the graph through the telling itself, minted the
# way an unknown noun is, and takes its place from the company it keeps. An idea and
# a notion are used alike here: both shape a plan, both guide a choice, both spark a
# question. That shared use is the only signal that places them, and it places them
# near, so a fact told of one carries to the other as a hedged guess. This is what an
# abstract word's meaning is to a grounded mind: not a picture, but a position among
# the things it acts on.
#
# It builds on an existing world: restore the latest snapshot first, so the mind
# concepts land in a brain that already holds its animals, foods, and its own words.
#
#   GOLEM=golem zsh mind-concept.zsh
set -u
GOLEM=${GOLEM:-golem}
GAP=${GAP:-2}

log()  { print -r -- "$(date +%H:%M:%S) $*" }
sayw() { local i; for i in 1 2 3; do $GOLEM tell "$1" >/dev/null 2>&1 && { sleep 0.2; return 0 }; sleep 1; done; return 0 }
fact() { log "fact: $1"; sayw "$1"; sleep $GAP }
nap()  { log "(nap: consolidate)"; $GOLEM sleep >/dev/null 2>&1; sleep "${BLOCKGAP:-8}" }

log "=== the furniture of the mind: mould, guide, spark ==="

# Both an idea and a notion shape a plan: the first shared use, the column that
# starts to draw the two together in the usage space.
fact "an idea moulds a plan"
fact "a notion moulds a plan"

# Both guide a choice: a second shared use, deepening the likeness.
fact "an idea guides a choice"
fact "a notion guides a choice"

# Both spark a question: a third. With three uses in common and no feature between
# them, the idea and the notion now sit near in the usage space, near enough that
# one stands in for the other.
fact "an idea sparks a question"
fact "a notion sparks a question"
nap

# One thing told of only the idea, one of only the notion. Neither is ever told of
# the other, so each is the ground for an inference: does a notion shape a goal (the
# idea does), does an idea spark a problem (the notion does). The hedged yes rides
# the usage nearness the three shared uses built.
fact "an idea moulds a goal"
fact "a notion sparks a problem"
nap

log "=== done: the mind concepts taught ==="
