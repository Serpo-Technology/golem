#!/usr/bin/env zsh
# Two more clusters of perceptless abstract concepts, taught the way the language
# model and the mind concepts were: through propositions alone, so each word enters
# the graph minted from the telling and takes its place from the company it keeps. In
# each cluster two near-synonyms share three uses, so they land near in the usage
# space and a fact told of one carries to the other as a hedged guess.
#
#   - CAUSE: a reason and a factor both drive an event, produce a change, trigger a
#     result. Told apart only at the end, each grounds an inference about the other.
#   - KNOWLEDGE: a fact and a claim both support a theory, state a view, prove a case.
#
# Build on an existing world: restore the latest snapshot first.
#
#   GOLEM=golem zsh abstract-domains.zsh
set -u
GOLEM=${GOLEM:-golem}
GAP=${GAP:-2}

log()  { print -r -- "$(date +%H:%M:%S) $*" }
sayw() { local i; for i in 1 2 3; do $GOLEM tell "$1" >/dev/null 2>&1 && { sleep 0.2; return 0 }; sleep 1; done; return 0 }
fact() { log "fact: $1"; sayw "$1"; sleep $GAP }
nap()  { log "(nap: consolidate)"; $GOLEM sleep >/dev/null 2>&1; sleep "${BLOCKGAP:-8}" }

log "=== cause: a reason and a factor, used alike ==="
fact "a reason drives an event"
fact "a factor drives an event"
fact "a reason produces a change"
fact "a factor produces a change"
fact "a reason triggers a result"
fact "a factor triggers a result"
nap
# One of each alone: the ground for the cross inference.
fact "a reason produces an effect"
fact "a factor triggers an outcome"
nap

log "=== knowledge: a fact and a claim, used alike ==="
fact "a fact supports a theory"
fact "a claim supports a theory"
fact "a fact states a view"
fact "a claim states a view"
fact "a fact proves a case"
fact "a claim proves a case"
nap
# One of each alone: the ground for the cross inference.
fact "a fact supports a truth"
fact "a claim proves an error"
nap

log "=== done: cause and knowledge concepts taught ==="
