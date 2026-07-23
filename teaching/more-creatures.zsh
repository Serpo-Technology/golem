#!/usr/bin/env zsh
# More of the living world, taught by telling for speed: sea creatures, more birds and
# mammals, amphibians, and minibeasts, each placed under its kind and the animal
# superordinate and tied in by diet and home. Builds on the menagerie.
#
#   GOLEM=golem GAP=1 zsh more-creatures.zsh
set -u
GOLEM=${GOLEM:-golem}
GAP=${GAP:-1}

log()  { print -r -- "$(date +%H:%M:%S) $*" }
sayw() { local i; for i in 1 2 3; do $GOLEM tell "$1" >/dev/null 2>&1 && { sleep 0.15; return 0 }; sleep 1; done; return 0 }
saytr(){ local i; for i in 1 2 3; do $GOLEM tell --lang tr "$1" >/dev/null 2>&1 && { sleep 0.15; return 0 }; sleep 1; done; return 0 }
fact() { sayw "$1"; sleep $GAP }
nap()  { log "(nap)"; $GOLEM sleep >/dev/null 2>&1; sleep "${BLOCKGAP:-6}" }
# name kind: place under a kind and under animal, twice each so it mints.
animal() { local n=$1 k=$2; fact "a $n is a $k"; fact "a $n is an animal"; fact "a $n is a $k"; }

$GOLEM status >/dev/null 2>&1 || { log "daemon unreachable"; exit 1 }
log "########## more creatures begin ##########"

# --- Sea creatures -----------------------------------------------------------
log "=== sea creatures ==="
animal shark fish;   animal seal mammal;   animal octopus creature; animal crab creature
animal lobster creature; animal jellyfish creature; animal starfish creature; animal walrus mammal
animal penguin bird; animal seahorse fish; animal squid creature; animal otter mammal
nap
fact "a shark ate a seal";   fact "an octopus lived in an ocean"; fact "a crab walked on a sand"
fact "a penguin lived in an ice"; fact "a seal ate a fish";       fact "a whale is bigger than a shark"
nap

# --- More mammals ------------------------------------------------------------
log "=== more mammals ==="
animal camel mammal; animal deer mammal;  animal wolf mammal;   animal lion mammal
animal leopard mammal; animal rhino mammal; animal hippo mammal; animal bat mammal
animal hedgehog mammal; animal beaver mammal; animal donkey mammal; animal buffalo mammal
nap
fact "a camel lived in a desert"; fact "a wolf hunted a deer"; fact "a lion is stronger than a wolf"
fact "a leopard ran fast";        fact "a bat flew at night";  fact "a beaver built a dam"
saytr "aslan bir memelidir"; saytr "kurt bir geyik avladı"; saytr "deve çölde yaşadı"
nap

# --- More birds --------------------------------------------------------------
log "=== more birds ==="
animal hawk bird;   animal falcon bird; animal pigeon bird; animal peacock bird
animal woodpecker bird; animal pelican bird; animal seagull bird; animal ostrich bird
animal turkey bird; animal goose bird;  animal heron bird;  animal magpie bird
nap
fact "a hawk hunted a mouse"; fact "an ostrich ran fast"; fact "a seagull flew over a sea"
fact "a peacock has a feather"; fact "a goose flew south"; fact "an eagle is bigger than a sparrow"
nap

# --- Amphibians and minibeasts -----------------------------------------------
log "=== amphibians and bugs ==="
animal frog amphibian; animal toad amphibian; animal newt amphibian; animal salamander amphibian
animal spider bug; animal worm bug; animal snail bug; animal slug bug
animal moth insect; animal wasp insect; animal ladybug insect; animal cricket insect
nap
fact "a frog lived in a pond"; fact "a spider spun a web"; fact "a worm lived in a soil"
fact "a frog ate a fly";       fact "a wasp has a sting";  fact "a snail is slower than a rabbit"
saytr "kurbağa bir gölette yaşadı"; saytr "örümcek bir ağ ördü"; saytr "solucan toprakta yaşadı"
nap
log "########## more creatures complete ##########"
