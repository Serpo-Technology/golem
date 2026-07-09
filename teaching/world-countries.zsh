#!/usr/bin/env zsh
# A small world of nations, each with a distinct currency, capital, and language, so
# their frames run parallel: the same three relations toward different fillers. That
# parallel structure is what a structural analogy maps across -- the dollar of America
# answered by the peso of Mexico -- which the single-relation and offset analogies
# cannot reach. It is ordinary world knowledge that also happens to carry the frames
# the holographic map reads; as more such vocabulary grows, the mode reaches wider.
#
# A statement subject needs a determiner to parse, so each nation is named with "the";
# the analogy query uses the bare names, which parse on their own.
#
# Build on an existing world: restore the latest snapshot first.
#   GOLEM=golem zsh world-countries.zsh
set -u
GOLEM=${GOLEM:-golem}
GAP=${GAP:-2}

log()  { print -r -- "$(date +%H:%M:%S) $*" }
sayw() { local i; for i in 1 2 3; do $GOLEM tell "$1" >/dev/null 2>&1 && { sleep 0.2; return 0 }; sleep 1; done; return 0 }
fact() { log "fact: $1"; sayw "$1"; sleep $GAP }
nap()  { log "(nap: consolidate)"; $GOLEM sleep >/dev/null 2>&1; sleep "${BLOCKGAP:-8}" }

log "=== america: dollar, washington, english ==="
fact "the america minted the dollar"
fact "the america ruled the washington"
fact "the america spoke the english"
nap

log "=== mexico: peso, mexicocity, spanish ==="
fact "the mexico minted the peso"
fact "the mexico ruled the mexicocity"
fact "the mexico spoke the spanish"
nap

log "=== france: euro, paris, french ==="
fact "the france minted the euro"
fact "the france ruled the paris"
fact "the france spoke the french"
nap

log "=== japan: yen, tokyo, japanese ==="
fact "the japan minted the yen"
fact "the japan ruled the tokyo"
fact "the japan spoke the japanese"
nap

log "=== done: four nation frames taught ==="
