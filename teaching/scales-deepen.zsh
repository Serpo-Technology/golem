#!/usr/bin/env zsh
# Long comparison chains and the is-a spine, run over a world that already holds the
# concepts. Each chain lays down a comparative relation the reasoner reads transitively
# and whose converse it discovers, and each is-a edge deepens the hierarchy. It mints
# almost nothing and relates a great deal, so the graph grows dense. Run late.
#
#   GOLEM=golem GAP=1 zsh scales-deepen.zsh
set -u
GOLEM=${GOLEM:-golem}
GAP=${GAP:-1}

log()  { print -r -- "$(date +%H:%M:%S) $*" }
sayw() { local i; for i in 1 2 3; do $GOLEM tell "$1" >/dev/null 2>&1 && { sleep 0.15; return 0 }; sleep 1; done; return 0 }
fact() { sayw "$1"; sleep $GAP }
nap()  { log "(nap)"; $GOLEM sleep >/dev/null 2>&1; sleep "${BLOCKGAP:-6}" }
# chain adj a b c ... : "a is <adj>er than b", "b is <adj>er than c", ...
chain() { local adj=$1; shift; while (( $# >= 2 )); do fact "a $1 is $adj than a $2"; shift; done }

$GOLEM status >/dev/null 2>&1 || { log "daemon unreachable"; exit 1 }
log "########## scales and spine begin ##########"

# --- Size --------------------------------------------------------------------
log "=== size ==="
chain "bigger" whale elephant horse cow bear dog cat mouse ant
chain "bigger" mountain hill rock stone pebble
chain "bigger" ocean sea lake pond puddle
chain "bigger" tree bush flower grass moss
nap

# --- Speed -------------------------------------------------------------------
log "=== speed ==="
chain "faster" cheetah horse dog human turtle snail
chain "faster" plane train car bicycle
chain "faster" eagle sparrow bee
nap

# --- Height, weight, hardness, temperature -----------------------------------
log "=== more scales ==="
chain "taller" giraffe elephant horse dog cat
chain "taller" skyscraper tower house
chain "heavier" elephant horse dog cat mouse
chain "harder" diamond metal stone wood clay
chain "hotter" sun fire lava water ice
chain "brighter" sun star moon candle
nap

# --- The is-a spine ----------------------------------------------------------
log "=== is-a spine ==="
fact "a mammal is an animal";  fact "a bird is an animal";  fact "a fish is an animal"
fact "an insect is an animal"; fact "a reptile is an animal"; fact "an amphibian is an animal"
fact "a tree is a plant";      fact "a flower is a plant";  fact "a vegetable is a plant"
fact "a fruit is a plant";     fact "an animal is a living thing"; fact "a plant is a living thing"
nap
fact "a tool is an object";    fact "a vehicle is an object"; fact "a furniture is an object"
fact "an instrument is an object"; fact "a container is an object"; fact "an object is a thing"
fact "a living thing is a thing"; fact "a place is a thing";   fact "a person is an animal"
fact "a colour is a quality";  fact "a shape is a quality";  fact "a feeling is an idea"
nap

# --- Antonym pairs (opposites) ------------------------------------------------
log "=== opposites ==="
fact "a big opposes a small";  fact "a hot opposes a cold";  fact "a fast opposes a slow"
fact "a hard opposes a soft";  fact "a light opposes a dark"; fact "a wet opposes a dry"
fact "a high opposes a low";   fact "a rich opposes a poor";  fact "a young opposes an old"
fact "a strong opposes a weak"; fact "a happy opposes a sad"; fact "a full opposes an empty"
nap
log "########## scales and spine complete ##########"
