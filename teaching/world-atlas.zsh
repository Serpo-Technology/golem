#!/usr/bin/env zsh
# A wider atlas of nations, each carrying the same frame toward different fillers:
# a currency minted, a capital ruled, a language spoken, and borders shared with its
# neighbours. The shared fillers (the euro across the eurozone, a continent across a
# region) become hubs, and the borders weave the nations into one graph, so the world
# grows dense in relations, not just wide in names. Builds on an existing world; the
# four nations of world-countries are left out so nothing is retaught.
#
#   GOLEM=golem GAP=1 zsh world-atlas.zsh
set -u
GOLEM=${GOLEM:-golem}
GAP=${GAP:-1}

log()  { print -r -- "$(date +%H:%M:%S) $*" }
sayw() { local i; for i in 1 2 3; do $GOLEM tell "$1" >/dev/null 2>&1 && { sleep 0.15; return 0 }; sleep 1; done; return 0 }
fact() { sayw "$1"; sleep $GAP }
nap()  { log "(nap)"; $GOLEM sleep >/dev/null 2>&1; sleep "${BLOCKGAP:-6}" }

# name capital currency language
frames=(
  "germany berlin euro german"      "italy rome euro italian"
  "spain madrid euro spanish"       "portugal lisbon euro portuguese"
  "greece athens euro greek"        "ireland dublin euro irish"
  "netherlands amsterdam euro dutch" "belgium brussels euro flemish"
  "austria vienna euro german"      "finland helsinki euro finnish"
  "britain london pound english"    "sweden stockholm krona swedish"
  "norway oslo krone norwegian"     "denmark copenhagen krone danish"
  "poland warsaw zloty polish"      "russia moscow ruble russian"
  "turkey ankara lira turkish"      "india delhi rupee hindi"
  "china beijing yuan chinese"      "korea seoul won korean"
  "brazil brasilia real portuguese" "canada ottawa dollar english"
  "egypt cairo pound arabic"        "kenya nairobi shilling swahili"
  "nigeria abuja naira english"     "australia canberra dollar english"
  "argentina buenosaires peso spanish" "chile santiago peso spanish"
  "switzerland bern franc german"   "thailand bangkok baht thai"
)
i=0
for row in "${frames[@]}"; do
  set -- ${(z)row}
  name=$1 cap=$2 cur=$3 lang=$4
  log "atlas: $name ($cap, $cur, $lang)"
  fact "the $name minted the $cur"
  fact "the $name ruled the $cap"
  fact "the $name spoke the $lang"
  i=$((i+1)); (( i % 5 == 0 )) && nap
done
nap

# Borders: a chain and a few crossings, weaving the nations into one connected graph.
log "=== borders ==="
borders=(
  "france germany" "germany poland" "germany austria" "germany belgium"
  "germany netherlands" "germany switzerland" "france spain" "france italy"
  "france belgium" "spain portugal" "italy austria" "italy switzerland"
  "poland russia" "norway sweden" "sweden finland" "finland russia"
  "belgium netherlands" "austria italy" "greece turkey" "turkey russia"
  "india china" "china russia" "china korea" "brazil argentina"
  "argentina chile" "canada america" "egypt kenya" "kenya nigeria"
)
i=0
for row in "${borders[@]}"; do
  set -- ${(z)row}
  fact "the $1 bordered the $2"
  i=$((i+1)); (( i % 8 == 0 )) && nap
done
nap
log "########## atlas complete ##########"
