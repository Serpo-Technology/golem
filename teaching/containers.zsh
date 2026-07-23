#!/usr/bin/env zsh
# A store of containers: many holders across three kinds (box, vessel, bag), each
# shown on its own percept, named, placed under its kind and under the container
# superordinate, and tied into the world by what it holds. The kinds abstract from
# the shared naming; the hold relations weave the holders to their contents. Every
# caption names the specific holder, never the bare kind. Builds on an existing world.
#
#   GOLEM=golem GAP=1 zsh containers.zsh
set -u
GOLEM=${GOLEM:-golem}
GAP=${GAP:-1}

log()  { print -r -- "$(date +%H:%M:%S) $*" }
obs()  { local i; for i in 1 2 3; do $GOLEM observe vision "$1" >/dev/null 2>&1 && { sleep 0.15; return 0 }; sleep 1; done; return 0 }
fobs() { local i; for i in 1 2 3; do $GOLEM observe --force vision "$1" >/dev/null 2>&1 && { sleep 0.15; return 0 }; sleep 1; done; return 0 }
sayw() { local i; for i in 1 2 3; do $GOLEM tell "$1" >/dev/null 2>&1 && { sleep 0.15; return 0 }; sleep 1; done; return 0 }
saytr(){ local i; for i in 1 2 3; do $GOLEM tell --lang tr "$1" >/dev/null 2>&1 && { sleep 0.15; return 0 }; sleep 1; done; return 0 }
nap()  { log "(nap)"; $GOLEM sleep >/dev/null 2>&1; sleep "${BLOCKGAP:-6}" }

adj=(drifting distant humming quiet busy faded gentle sudden hollow crisp damp dusty
     woven carved folded tangled looming narrow hidden sloping creaking shimmering idle
     weary jagged swaying flickering rippling glowing ticking whistling)
nn=(fountain archway corridor tunnel harbor plaza kiosk streetlamp balcony staircase
    courtyard terrace rooftop chimney beacon lighthouse pier canal footpath lane ravine
    summit lagoon marsh hallway alcove rotunda veranda parapet turret)
ci=0
comp() { obs "a $adj[$(( (ci % ${#adj[@]}) + 1 ))] $nn[$(( (ci % ${#nn[@]}) + 1 ))]"; ci=$((ci+1)) }

# teach: caption name subkind tr   -- five showings, named at three levels, bilingual.
teach() {
  local cap=$1 name=$2 kind=$3 tr=$4 i
  log "teach: $name ($kind)"
  for i in 1 2 3 4 5; do
    fobs "$cap"; sayw "$name"
    (( i == 2 )) && sayw "$kind"; (( i == 3 )) && sayw "container"
    (( i == 4 )) && { [ -n "$tr" ] && saytr "$tr" }
    (( i == 5 )) && sayw "$kind"
    comp; sleep $GAP
  done
}
fact() { sayw "$1"; sleep $GAP }

$GOLEM status >/dev/null 2>&1 || { log "daemon unreachable"; exit 1 }
log "########## containers begins ##########"

# --- Boxes -------------------------------------------------------------------
teach "a splintered wooden crate"  crate  box kasa
teach "a crushed cardboard carton" carton box ""
teach "a lacquered ebony casket"   casket box ""
teach "a rusty steel bin"          bin    box ""
teach "a bolted oak coffer"        coffer box ""
teach "a woven straw caddy"        caddy  box ""
nap

# --- Vessels -----------------------------------------------------------------
teach "a hooped oak barrel"        barrel   vessel fıçı
teach "a squat glass jar"          jar      vessel kavanoz
teach "a slender silver flask"     flask    vessel matara
teach "a labeled tin canister"     canister vessel ""
teach "a tall marble urn"          urn      vessel ""
teach "a painted clay amphora"     amphora  vessel amfora
teach "a chipped stoneware jug"    jug      vessel testi
nap
teach "a vast copper vat"          vat      vessel ""
teach "a sealed cedar cask"        cask     vessel ""
teach "a wide wooden tub"          tub      vessel ""
teach "a galvanized metal pail"    pail     vessel kova
teach "a long stone trough"        trough   vessel yalak
nap

# --- Bags --------------------------------------------------------------------
teach "a drawstring leather pouch" pouch    bag kese
teach "a scuffed canvas satchel"   satchel  bag ""
teach "a bulging burlap sack"      sack     bag çuval
teach "a lidded wicker hamper"     hamper   bag ""
teach "a striped cotton tote"      tote     bag ""
teach "a patched army knapsack"    knapsack bag ""
teach "a bulky nylon duffel"       duffel   bag ""
teach "a woven willow basket"      basket   bag sepet
teach "a heavy leather rucksack"   rucksack bag ""
teach "a beaded evening purse"     purse    bag ""
nap

# --- Use: holders hold contents ----------------------------------------------
log "=== use: hold ==="
fact "a barrel held a wine";     fact "a jar held a honey";      fact "a sack held a grain"
fact "a basket held a bread";    fact "a pouch held a coin";     fact "a flask held a water"
fact "a crate held a fruit";     fact "a canister held a flour"; fact "a satchel held a book"
fact "a trough held a water";    fact "an urn held an ash";      fact "a hamper held a cloth"
nap
log "########## containers complete ##########"
