#!/usr/bin/env zsh
# Three more clusters of perceptless abstract concepts, taught through propositions
# like the mind, cause, and knowledge concepts. In each cluster two near-synonyms
# share three uses, so they land near in the usage space and a fact told of one
# carries to the other as a hedged guess.
#
#   - PURPOSE:  an aim and a purpose both drive an effort, guide a task, serve a quest.
#   - DESIRE:   a hope and a wish both nourish a dream, stir a mood, spur an urge.
#   - VALUE:    a value and a merit both raise a quality, weigh a standard, mark a level.
#
# Build on an existing world: restore the latest snapshot first.
#
#   GOLEM=golem zsh abstract-domains-2.zsh
set -u
GOLEM=${GOLEM:-golem}
GAP=${GAP:-2}

log()  { print -r -- "$(date +%H:%M:%S) $*" }
sayw() { local i; for i in 1 2 3; do $GOLEM tell "$1" >/dev/null 2>&1 && { sleep 0.2; return 0 }; sleep 1; done; return 0 }
fact() { log "fact: $1"; sayw "$1"; sleep $GAP }
nap()  { log "(nap: consolidate)"; $GOLEM sleep >/dev/null 2>&1; sleep "${BLOCKGAP:-8}" }

log "=== purpose: an aim and a purpose, used alike ==="
fact "an aim drives an effort"
fact "a purpose drives an effort"
fact "an aim guides a task"
fact "a purpose guides a task"
fact "an aim serves a quest"
fact "a purpose serves a quest"
nap
fact "an aim drives a mission"
fact "a purpose guides a deal"
nap

log "=== desire: a hope and a wish, used alike ==="
fact "a hope nourishes a dream"
fact "a wish nourishes a dream"
fact "a hope stirs a mood"
fact "a wish stirs a mood"
fact "a hope spurs an urge"
fact "a wish spurs an urge"
nap
fact "a hope nourishes a spirit"
fact "a wish stirs a plan"
nap

log "=== value: a value and a merit, used alike ==="
fact "a value raises a quality"
fact "a merit raises a quality"
fact "a value weighs a standard"
fact "a merit weighs a standard"
fact "a value marks a level"
fact "a merit marks a level"
nap
fact "a value raises a grade"
fact "a merit marks a rank"
nap

log "=== done: purpose, desire, value concepts taught ==="
