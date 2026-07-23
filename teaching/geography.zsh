#!/usr/bin/env zsh
# The physical world named: cities, rivers, mountains, and seas, each placed under its
# kind and tied to the country it sits in, so the atlas gains a layer of places under
# the nations. Taught by telling. Builds on the atlas.
#
#   GOLEM=golem GAP=1 zsh geography.zsh
set -u
GOLEM=${GOLEM:-golem}
GAP=${GAP:-1}

log()  { print -r -- "$(date +%H:%M:%S) $*" }
sayw() { local i; for i in 1 2 3; do $GOLEM tell "$1" >/dev/null 2>&1 && { sleep 0.15; return 0 }; sleep 1; done; return 0 }
fact() { sayw "$1"; sleep $GAP }
nap()  { log "(nap)"; $GOLEM sleep >/dev/null 2>&1; sleep "${BLOCKGAP:-6}" }
kindof() { local n=$1 k=$2; fact "the $n is a $k"; fact "the $n is a $k"; }

$GOLEM status >/dev/null 2>&1 || { log "daemon unreachable"; exit 1 }
log "########## geography begins ##########"

# --- Cities, each in a country -----------------------------------------------
log "=== cities ==="
# city country
cities=(
  "newyork america" "chicago america" "toronto canada" "riodejaneiro brazil"
  "munich germany" "milan italy" "barcelona spain" "lyon france"
  "manchester britain" "glasgow britain" "naples italy" "hamburg germany"
  "kyoto japan" "shanghai china" "mumbai india" "busan korea"
  "alexandria egypt" "mombasa kenya" "lagos nigeria" "sydney australia"
  "istanbul turkey" "izmir turkey" "saintpetersburg russia" "geneva switzerland"
)
i=0
for row in "${cities[@]}"; do
  set -- ${(z)row}
  kindof "$1" city
  fact "the $1 sits in the $2"
  i=$((i+1)); (( i % 6 == 0 )) && nap
done
nap

# --- Rivers ------------------------------------------------------------------
log "=== rivers ==="
rivers=("nile egypt" "amazon brazil" "thames britain" "seine france"
        "rhine germany" "danube austria" "ganges india" "yangtze china"
        "volga russia" "tiber italy" "ebro spain" "mississippi america")
i=0
for row in "${rivers[@]}"; do
  set -- ${(z)row}
  kindof "$1" river
  fact "the $1 flowed through the $2"
  i=$((i+1)); (( i % 6 == 0 )) && nap
done
nap
fact "the amazon is longer than the thames"; fact "the nile is longer than the seine"
fact "a river flowed to a sea"; fact "the volga is a river"
nap

# --- Mountains ---------------------------------------------------------------
log "=== mountains ==="
mts=("everest asia" "alps europe" "andes america" "kilimanjaro africa"
     "fuji japan" "elbrus russia" "atlas africa" "urals russia")
i=0
for row in "${mts[@]}"; do
  set -- ${(z)row}
  kindof "$1" mountain
  fact "the $1 rises in the $2"
  i=$((i+1)); (( i % 6 == 0 )) && nap
done
nap
fact "the everest is taller than the alps"; fact "the everest is the tallest mountain"
fact "a mountain is taller than a hill"
nap

# --- Seas --------------------------------------------------------------------
log "=== seas ==="
for s in atlantic pacific mediterranean baltic caspian arctic; do
  kindof "$s" sea
done
nap
fact "the pacific is bigger than the baltic"; fact "the mediterranean touched the egypt"
fact "the atlantic touched the america"; fact "a sea holds a water"; fact "a ship crossed the atlantic"
nap
log "########## geography complete ##########"
