#!/usr/bin/env zsh
# A dense web of cross-domain facts that ties the whole world together: professions to
# tools and places, animals to foods and homes, materials to made things, countries to
# continents, and long is-a and comparison chains. It mints few new concepts and many
# relations, so the graph grows dense where the vocabulary scripts grow it wide. Run
# last, over a world that already holds all the concepts it relates.
#
#   GOLEM=golem GAP=1 zsh relations-web.zsh
set -u
GOLEM=${GOLEM:-golem}
GAP=${GAP:-1}

log()  { print -r -- "$(date +%H:%M:%S) $*" }
sayw() { local i; for i in 1 2 3; do $GOLEM tell "$1" >/dev/null 2>&1 && { sleep 0.15; return 0 }; sleep 1; done; return 0 }
fact() { sayw "$1"; sleep $GAP }
nap()  { log "(nap)"; $GOLEM sleep >/dev/null 2>&1; sleep "${BLOCKGAP:-6}" }

$GOLEM status >/dev/null 2>&1 || { log "daemon unreachable"; exit 1 }
log "########## relations web begins ##########"

# --- Professions use tools ---------------------------------------------------
log "=== profession uses tool ==="
fact "a smith used a hammer";    fact "a builder used a saw";   fact "a farmer used a shovel"
fact "a cook used a pot";        fact "a painter used a brush"; fact "a sailor used a boat"
fact "a soldier used a sword";   fact "a doctor used a needle"; fact "a tailor used a needle"
fact "a miner used an axe";      fact "a gardener used a rake"; fact "a carpenter used a drill"
nap

# --- Continents hold countries -----------------------------------------------
log "=== continent holds country ==="
fact "a europe held a germany";  fact "a europe held a france"; fact "a europe held a spain"
fact "an asia held a china";     fact "an asia held a japan";   fact "an asia held an india"
fact "an africa held an egypt";  fact "an africa held a kenya"; fact "an africa held a nigeria"
fact "an america held a canada"; fact "an america held a brazil"; fact "an america held a mexico"
nap

# --- Materials make things ---------------------------------------------------
log "=== material makes thing ==="
fact "a wood made a chair";  fact "a wood made a table";  fact "a wood made a boat"
fact "a metal made a hammer"; fact "a metal made a knife"; fact "a metal made a coin"
fact "a stone made a wall";  fact "a stone made a castle"; fact "a glass made a bottle"
fact "a cloth made a shirt"; fact "a leather made a shoe"; fact "a clay made a pot"
nap

# --- Animals and their kinds (deeper is-a chains) ----------------------------
log "=== is-a chains ==="
fact "a mammal is an animal";  fact "a bird is an animal";  fact "a fish is an animal"
fact "an insect is an animal"; fact "a reptile is an animal"; fact "an animal is a living thing"
fact "a tree is a plant";      fact "a flower is a plant";  fact "a plant is a living thing"
fact "a living thing is a thing"; fact "a tool is a thing";  fact "a vehicle is a thing"
fact "a dog is a mammal";      fact "an eagle is a bird";   fact "a salmon is a fish"
nap

# --- Part-whole --------------------------------------------------------------
log "=== part-whole ==="
fact "a wheel is part of a car";  fact "a wing is part of a plane"; fact "a root is part of a tree"
fact "a petal is part of a flower"; fact "a page is part of a book"; fact "a room is part of a house"
fact "a branch is part of a tree"; fact "a leg is part of a chair"; fact "a handle is part of a door"
fact "a string is part of a guitar"; fact "a key is part of a piano"; fact "a sail is part of a boat"
nap

# --- Comparisons across domains (scales) -------------------------------------
log "=== comparisons ==="
fact "an elephant is bigger than a mouse"; fact "a whale is bigger than a dolphin"
fact "an eagle is faster than a sparrow";  fact "a tiger is stronger than a cat"
fact "a plane is faster than a car";       fact "a train is longer than a bus"
fact "a mountain is higher than a tower";  fact "an ocean is deeper than a lake"
fact "a diamond is harder than a gold";    fact "a lion is fiercer than a rabbit"
fact "a giraffe is taller than a zebra";   fact "a truck is heavier than a bicycle"
nap

# --- Who does what (agents and actions) --------------------------------------
log "=== agents and actions ==="
fact "a bird flew in a sky";     fact "a fish swam in a sea";   fact "a snake crawled on a ground"
fact "a horse ran in a field";   fact "a bee flew to a flower"; fact "a spider spun a web"
fact "a river flowed to a sea";  fact "a sun rose in a morning"; fact "a rain watered a plant"
fact "a farmer fed an animal";   fact "a mother held a child";  fact "a dog guarded a house"
nap
log "########## relations web complete ##########"
