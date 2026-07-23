#!/usr/bin/env zsh
# A wide fleet of vehicles across four kinds (landcar, watercraft, aircraft, rail),
# each shown on its own percept, named, placed under its kind and under the vehicle
# superordinate, and tied into the world by what it carries and where it goes. The
# kinds abstract from the shared naming; the carry and cross relations weave the
# vehicles to people and places. Builds on an existing world.
#
#   GOLEM=golem GAP=1 zsh vehicles.zsh
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
    (( i == 2 )) && sayw "$kind"; (( i == 3 )) && sayw "vehicle"
    (( i == 4 )) && { [ -n "$tr" ] && saytr "$tr" }
    (( i == 5 )) && sayw "$kind"
    comp; sleep $GAP
  done
}
fact() { sayw "$1"; sleep $GAP }

$GOLEM status >/dev/null 2>&1 || { log "daemon unreachable"; exit 1 }
log "########## vehicles begins ##########"

# --- Land vehicles -----------------------------------------------------------
teach "a red racing car"        car        landcar araba
teach "a big blue truck"        truck      landcar kamyon
teach "a long yellow bus"       bus        landcar otobüs
teach "a slim silver bicycle"   bicycle    landcar bisiklet
teach "a loud black motorcycle" motorcycle landcar motosiklet
teach "an old wooden wagon"     wagon      landcar ""
teach "a white delivery van"    van        landcar ""
teach "a green farm tractor"    tractor    landcar traktör
teach "a bright yellow taxi"    taxi       landcar taksi
teach "a rugged green jeep"     jeep       landcar ""
nap

# --- Watercraft --------------------------------------------------------------
teach "a small wooden boat"     boat       watercraft tekne
teach "a huge steel ship"       ship       watercraft gemi
teach "a narrow birch canoe"    canoe      watercraft kano
teach "a crowded white ferry"   ferry      watercraft feribot
teach "a black diving submarine" submarine watercraft denizaltı
teach "a white billowing sailboat" sailboat watercraft yelkenli
teach "a sleek luxury yacht"    yacht      watercraft yat
teach "a flat rubber raft"      raft       watercraft sal
teach "a bright orange kayak"   kayak      watercraft ""
nap

# --- Aircraft ----------------------------------------------------------------
teach "a silver soaring plane"  plane      aircraft uçak
teach "a fast military jet"     jet        aircraft jet
teach "a hovering red helicopter" helicopter aircraft helikopter
teach "a quiet white glider"    glider     aircraft planör
teach "a striped drifting balloon" balloon aircraft balon
teach "a long silver airship"   airship    aircraft ""
teach "a tall white rocket"     rocket     aircraft roket
nap

# --- Rail --------------------------------------------------------------------
teach "a long speeding train"   train      rail tren
teach "a red city tram"         tram       rail tramvay
teach "a fast underground subway" subway   rail metro
teach "an old electric trolley" trolley    rail ""
teach "a sleek elevated monorail" monorail rail ""
nap

# --- Carry: vehicles carry people and loads ----------------------------------
log "=== carry ==="
fact "a car carried a person";      fact "a bus carried a person";  fact "a truck carried a load"
fact "a train carried a passenger"; fact "a ferry carried a car";   fact "a wagon carried a hay"
fact "a van carried a parcel";      fact "a taxi carried a rider";  fact "a ship carried a cargo"
nap

# --- Travel: vehicles cross and move through places ---------------------------
log "=== travel ==="
fact "a ship crossed an ocean";     fact "a plane flew in a sky";   fact "a boat floated on a water"
fact "a bicycle rolled on a road";  fact "a helicopter hovered in an air"; fact "a ferry crossed a river"
fact "a submarine dived in an ocean"; fact "a rocket flew to a moon"; fact "a train ran on a track"
nap
log "########## vehicles complete ##########"
