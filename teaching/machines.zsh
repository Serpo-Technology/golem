#!/usr/bin/env zsh
# A wide workshop of machines across three kinds (powerplant, mechanism, appliance),
# each shown on its own percept, named, placed under its kind and under the machine
# superordinate, and tied into the world by the work it does. The kinds abstract from
# the shared naming; the work and drive relations weave the machines to the loads they
# move. Builds on an existing world.
#
#   GOLEM=golem GAP=1 zsh machines.zsh
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
    (( i == 2 )) && sayw "$kind"; (( i == 3 )) && sayw "machine"
    (( i == 4 )) && { [ -n "$tr" ] && saytr "$tr" }
    (( i == 5 )) && sayw "$kind"
    comp; sleep $GAP
  done
}
fact() { sayw "$1"; sleep $GAP }

$GOLEM status >/dev/null 2>&1 || { log "daemon unreachable"; exit 1 }
log "########## machines begins ##########"

# --- Powerplants -------------------------------------------------------------
teach "a roaring steam engine"    engine     powerplant motor
teach "a small electric motor"    motor      powerplant ""
teach "a spinning wind turbine"   turbine    powerplant türbin
teach "a humming brass dynamo"    dynamo     powerplant dinamo
teach "a loud diesel generator"   generator  powerplant jeneratör
teach "a heavy air compressor"    compressor powerplant kompresör
teach "a sealed nuclear reactor"  reactor    powerplant reaktör
nap

# --- Mechanisms --------------------------------------------------------------
teach "a sliding metal piston"    piston     mechanism piston
teach "a toothed iron gear"       gear       mechanism dişli
teach "a hanging rope pulley"     pulley     mechanism makara
teach "a long steel lever"        lever      mechanism kaldıraç
teach "a bent hand crank"         crank      mechanism ""
teach "a rounded rotating cam"    cam        mechanism ""
teach "a straight steel axle"     axle       mechanism ""
teach "a heavy spinning flywheel" flywheel   mechanism ""
teach "a clicking metal ratchet"  ratchet    mechanism ""
nap

# --- Appliances --------------------------------------------------------------
teach "a rusty water pump"        pump       appliance pompa
teach "a tall lifting crane"      crane      appliance vinç
teach "a geared steel winch"      winch      appliance ""
teach "a long moving conveyor"    conveyor   appliance ""
teach "a glowing hot furnace"     furnace    appliance ""
teach "a huge iron boiler"        boiler     appliance kazan
teach "a turning stone mill"      mill       appliance değirmen
teach "a heavy hydraulic press"   press      appliance ""
teach "a whirring metal fan"      fan        appliance ""
teach "a rising cabin elevator"   elevator   appliance asansör
nap

# --- Work: machines do work --------------------------------------------------
log "=== work ==="
fact "an engine powered a car";     fact "a motor spun a wheel";        fact "a turbine spun a blade"
fact "a pump moved a water";        fact "a generator made a current";  fact "a crane lifted a beam"
fact "a winch pulled a rope";       fact "a mill ground a grain";       fact "a boiler heated a water"
fact "a gear turned a shaft";       fact "a piston pushed a rod";       fact "a conveyor carried a box"
nap

# --- Drive: machines run and move things -------------------------------------
log "=== drive ==="
fact "an engine drove a pump";      fact "a turbine drove a generator"; fact "a motor drove a fan"
fact "a crank turned a gear";       fact "a pulley raised a load";      fact "a lever lifted a rock"
fact "a piston drove a crank";      fact "a furnace fed a boiler";      fact "a dynamo charged a battery"
nap
log "########## machines complete ##########"
