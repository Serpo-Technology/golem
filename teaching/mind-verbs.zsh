#!/usr/bin/env zsh
# GOLEM baseline pass: the verbs of mind. This grounds the experiential and
# epistemic relations that learning-to-learn and dialogue both rest on -- want,
# see, like, know -- as ordinary relations between an animate experiencer and a
# thing, bilingual, so the self and the user can later be taught as things that
# want and know. It builds on an existing world: restore the latest snapshot
# first, since the animals and foods it leans on are already grounded there.
#
#   GOLEM=golem zsh mind-verbs.zsh
set -u
GOLEM=${GOLEM:-golem}
GAP=${GAP:-2}

log()   { print -r -- "$(date +%H:%M:%S) $*" }
sayw()  { local i; for i in 1 2 3; do $GOLEM tell "$1" >/dev/null 2>&1 && { sleep 0.2; return 0 }; sleep 1; done; return 0 }
saytr() { local i; for i in 1 2 3; do $GOLEM tell --lang tr "$1" >/dev/null 2>&1 && { sleep 0.2; return 0 }; sleep 1; done; return 0 }
nap()   { log "(nap: consolidate)"; $GOLEM sleep >/dev/null 2>&1; sleep "${BLOCKGAP:-8}" }
# A relation must bind one tongue's verb to the other's before a consolidation
# clears the workspace, so the two tellings are adjacent (the bi_rel recipe).
bi_rel() { log "rel: $1 / $2"; sayw "$1"; saytr "$2"; sleep $GAP }

# want, love, and watch are open words here: see and know are already GOLEM's own
# (it perceives, and it says "I do not know"), so they are reserved and a fresh
# relation cannot take them. These three carry the experiential floor instead.
log "=== verbs of mind: want, love, watch (bilingual) ==="

# want: an animate experiencer desires a thing.
bi_rel "a dog wants a carrot"      "köpek havucu ister"
bi_rel "a cat wants a cheese"      "kedi peyniri ister"
bi_rel "a bear wants a honey"      "ayı balı ister"
nap

# love: an animate experiencer is fond of a thing.
bi_rel "a rabbit loves a carrot"   "tavşan havucu sever"
bi_rel "a mouse loves a cheese"    "fare peyniri sever"
bi_rel "a bear loves a honey"      "ayı balı sever"
nap

# watch: an animate experiencer keeps its eyes on another.
bi_rel "a cat watches a mouse"     "kedi fareyi izler"
bi_rel "a wolf watches a rabbit"   "kurt tavşanı izler"
bi_rel "a lion watches a deer"     "aslan geyiği izler"
nap

log "=== done: verbs of mind taught ==="
