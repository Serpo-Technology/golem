#!/usr/bin/env zsh
# A Turkish layer over the grown English world: each concept already has an
# English name on its percept; this binds the Turkish name to the SAME percept,
# so one concept answers in either tongue. The chase and fetch relations and the
# size scale are unified across tongues by warming the English relation and
# telling the Turkish one back to back, the way kovala joins chase.
#
#   GOLEM=/tmp/golem-native GAP=2 zsh bilingualize.zsh
#
# Run against a daemon holding the English world (world-clean snapshot). It only
# ADDS Turkish names and cross-tongue relation links; the English layer and the
# reasoning over it are untouched. A noun is taught onto its existing percept in
# its own block with a nap between, so its Turkish name stays a name on that one
# concept rather than drifting; the relation unification is back to back with no
# nap so the warmed relation is still in mind when the Turkish verb is told.
set -u

GOLEM=${GOLEM:-golem}
GAP=${GAP:-2}
log() { print -r -- "$(date +%H:%M:%S) $*" }

obs()   { local i; for i in 1 2 3; do $GOLEM observe vision "$1" >/dev/null 2>&1 && { sleep 0.4; return 0 }; sleep 1; done; return 0 }
sayw()  { local i; for i in 1 2 3; do $GOLEM tell "$1" >/dev/null 2>&1 && { sleep 0.4; return 0 }; sleep 1; done; return 0 }
saytr() { local i; for i in 1 2 3; do $GOLEM tell --lang tr "$1" >/dev/null 2>&1 && { sleep 0.4; return 0 }; sleep 1; done; return 0 }
nap()   { log "(nap: consolidate)"; $GOLEM sleep >/dev/null 2>&1; sleep 2 }

# Fresh companions so no scene-setter recurs into a junk hub (the gate is off, so
# diversity is only a safety margin here).
ci=0
companion() { obs "a passing shape $ci"; ci=$((ci+1)) }

# Teach a Turkish name onto an existing percept: show the percept, name it in
# Turkish while it is the warm concept, six showings so the name lands firm.
bind_tr() {
  local cap=$1 name=$2 i
  log "bind: $name -> $cap"
  for i in 1 2 3 4 5 6; do obs "$cap"; saytr "$name"; companion; done
}

$GOLEM status >/dev/null 2>&1 || { log "daemon unreachable at $GOLEM"; exit 1 }
log "########## bilingualize begins ##########"

# --- Base nouns: bind the Turkish name to each English percept ---------------
log "=== base: dog cat mouse rabbit ==="
bind_tr "a small brown dog" "köpek"; nap
bind_tr "a small black cat" "kedi"; nap
bind_tr "a small gray mouse" "fare"; nap
bind_tr "a gray rabbit" "tavşan"; nap

# --- The wider animal cast ---------------------------------------------------
log "=== animals: wolf bird fish horse snail frog bear lion owl deer ==="
bind_tr "a gray wolf" "kurt"; nap
bind_tr "a small blue bird" "kuş"; nap
bind_tr "a silver fish" "balık"; nap
bind_tr "a large brown horse" "at"; nap
bind_tr "a tiny gray snail" "salyangoz"; nap
bind_tr "a green frog" "kurbağa"; nap
bind_tr "a large black bear" "ayı"; nap
bind_tr "a golden lion" "aslan"; nap
bind_tr "a brown owl" "baykuş"; nap
bind_tr "a spotted deer" "geyik"; nap

# --- A few objects -----------------------------------------------------------
log "=== objects: ball box cup book plate ==="
bind_tr "a round red ball" "top"; nap
bind_tr "a square wooden box" "kutu"; nap
bind_tr "a white cup" "fincan"; nap
bind_tr "a thick blue book" "kitap"; nap
bind_tr "a round clay plate" "tabak"; nap

# --- Relations unified across tongues (warm English, tell Turkish, no nap) ----
# The English relation is still in mind when the Turkish verb is first told, so
# kovala binds to chase and getir to fetch rather than minting a parallel one.
log "=== relations: chase<->kovala, fetch<->getir ==="
sayw "a dog chased a cat"; saytr "köpek kediyi kovaladı"
sayw "a dog fetched a ball"; saytr "köpek topu getirdi"
nap

# --- The size scale unified across tongues -----------------------------------
# Warm "bigger", then the Turkish comparative "daha büyük" joins the same scale,
# so a size question asked in Turkish reads the English-told chain.
log "=== size: bigger <-> daha büyük ==="
sayw "a lion is bigger than a horse"; saytr "aslan attan daha büyük"
nap

log "########## bilingualize complete ##########"
