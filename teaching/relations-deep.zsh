#!/usr/bin/env zsh
# A second dense web of cross-domain facts laid over the world the earlier scripts grew:
# workers in their workplaces, animals and the sounds they make, containers and what
# they hold, parts and their wholes, causes and what they bring, and long chains of
# comparison a reasoner can walk. It mints few concepts and many relations. Run last,
# over a world that already holds the concepts it ties together.
#
#   GOLEM=golem GAP=1 zsh relations-deep.zsh
set -u
GOLEM=${GOLEM:-golem}
GAP=${GAP:-1}

log()  { print -r -- "$(date +%H:%M:%S) $*" }
sayw() { local i; for i in 1 2 3; do $GOLEM tell "$1" >/dev/null 2>&1 && { sleep 0.15; return 0 }; sleep 1; done; return 0 }
fact() { sayw "$1"; sleep $GAP }
nap()  { log "(nap)"; $GOLEM sleep >/dev/null 2>&1; sleep "${BLOCKGAP:-6}" }

$GOLEM status >/dev/null 2>&1 || { log "daemon unreachable"; exit 1 }
log "########## relations deep begins ##########"

# --- Workers in their workplaces ---------------------------------------------
log "=== worker works in place ==="
fact "a doctor worked in a hospital"; fact "a teacher worked in a school"; fact "a baker worked in a bakery"
fact "a smith worked in a forge";     fact "a judge worked in a court";    fact "a cook worked in a kitchen"
fact "a sailor worked on a ship";     fact "a miner worked in a mine";     fact "a farmer worked in a barn"
nap

# --- Animals and the sounds they make ----------------------------------------
log "=== animal makes sound ==="
fact "a dog barked";    fact "a cat meowed";  fact "a cow mooed"
fact "a horse neighed"; fact "a lion roared"; fact "a bird sang"
fact "a duck quacked";  fact "a bee buzzed";  fact "a snake hissed"
nap

# --- Containers and what they hold -------------------------------------------
log "=== container holds content ==="
fact "a bottle held a water"; fact "a basket held a bread"; fact "a cup held a water"
fact "a box held a book";     fact "a jar held a honey";   fact "a bag held a coin"
fact "a pot held a meat";     fact "a barn held a cow"
nap

# --- Parts and their wholes --------------------------------------------------
log "=== part of whole ==="
fact "a leaf is part of a tree";  fact "a feather is part of a bird"; fact "a wing is part of a bird"
fact "a tail is part of a dog";   fact "a fin is part of a fish";     fact "a horn is part of a cow"
fact "a roof is part of a house"; fact "a wall is part of a house"
nap

# --- Causes and what they bring ----------------------------------------------
log "=== cause brings effect ==="
fact "a rain made a flood"; fact "a fire made a smoke";  fact "a sun made a light"
fact "a cloud made a rain"; fact "a river made a flood"; fact "a fire made a heat"
fact "a sun made a shadow"; fact "a wind made a wave"
nap

# --- Long chains of comparison -----------------------------------------------
log "=== comparison chains ==="
fact "a whale is bigger than an elephant"; fact "an elephant is bigger than a dog"
fact "a dog is bigger than a mouse";       fact "a mouse is bigger than an ant"
fact "a cheetah is faster than a horse";   fact "a horse is faster than a person"
fact "a giraffe is taller than a person";  fact "a person is taller than a dog"
nap
log "########## relations deep complete ##########"
