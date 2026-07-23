#!/usr/bin/env zsh
# Motion and action verbs that a mind never sees. Each verb is minted from the company
# of a subject and sometimes an object, the way an unknown verb is learned from the
# sentence it lands in. A few carry a Turkish twin. Builds on an existing world whose
# creatures, people, and objects these verbs act upon.
#
#   GOLEM=golem GAP=1 zsh actions-motion.zsh
set -u
GOLEM=${GOLEM:-golem}
GAP=${GAP:-1}

log()  { print -r -- "$(date +%H:%M:%S) $*" }
sayw() { local i; for i in 1 2 3; do $GOLEM tell "$1" >/dev/null 2>&1 && { sleep 0.15; return 0 }; sleep 1; done; return 0 }
saytr(){ local i; for i in 1 2 3; do $GOLEM tell --lang tr "$1" >/dev/null 2>&1 && { sleep 0.15; return 0 }; sleep 1; done; return 0 }
fact() { sayw "$1"; sleep $GAP }
nap()  { log "(nap)"; $GOLEM sleep >/dev/null 2>&1; sleep "${BLOCKGAP:-6}" }

$GOLEM status >/dev/null 2>&1 || { log "daemon unreachable"; exit 1 }
log "########## actions begin ##########"

# --- Creatures and people in motion ------------------------------------------
log "=== motion ==="
fact "a bird flew";          fact "a fish swam";          fact "a person walked"
fact "a dog ran";            fact "a horse galloped";     fact "a person jumped"
fact "a boy climbed a hill"; fact "a leaf fell";          fact "a person sat"
fact "a person stood"
nap

# --- Handling: carrying, pushing, holding ------------------------------------
log "=== handling ==="
fact "a person carried a box"; fact "a person pushed a cart"; fact "a horse pulled a cart"
fact "a person threw a ball";  fact "a child caught a ball"; fact "a hand held a cup"
fact "a person lifted a rock"; fact "a person dropped a cup"; fact "a mother carried a baby"
fact "a person pushed a door"
nap

# --- Making and altering things ----------------------------------------------
log "=== making ==="
fact "a builder built a house"; fact "a person broke a bottle"; fact "a person opened a door"
fact "a person closed a door";  fact "a baker cut a bread";     fact "a farmer dug a field"
fact "a builder built a wall";  fact "a person opened a box";   fact "a person cut a wood"
fact "a dog dug a ground"
nap

# --- The body's day ----------------------------------------------------------
log "=== household ==="
fact "a person poured a water"; fact "a rain poured";        fact "a person washed a shirt"
fact "a cook cooked a meal";    fact "a person ate a bread"; fact "a person drank a water"
fact "a person slept";          fact "a baby slept";         fact "a person woke"
fact "a person washed a cup"
nap

# --- Turkish threads ---------------------------------------------------------
saytr "bir kuş uçtu"; saytr "bir insan yürüdü"; saytr "bir köpek koştu"
saytr "bir insan bir ekmek yedi"; saytr "bir insan bir su içti"
nap
log "########## actions complete ##########"
