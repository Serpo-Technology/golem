#!/usr/bin/env zsh
# A wide reef of sea creatures across four kinds (mollusk, crustacean, cnidarian,
# fish), each shown on its own percept, named, placed under its kind and under the
# animal superordinate, and tied into the world by where it lives and what preys on
# it. The kinds abstract from the shared naming; the live and eat relations weave the
# creatures to their waters and hunters. Extends the existing animal category.
#
#   GOLEM=golem GAP=1 zsh sealife.zsh
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
    (( i == 2 )) && sayw "$kind"; (( i == 3 )) && sayw "animal"
    (( i == 4 )) && { [ -n "$tr" ] && saytr "$tr" }
    (( i == 5 )) && sayw "$kind"
    comp; sleep $GAP
  done
}
fact() { sayw "$1"; sleep $GAP }

$GOLEM status >/dev/null 2>&1 || { log "daemon unreachable"; exit 1 }
log "########## sealife begins ##########"

# --- Mollusks ----------------------------------------------------------------
teach "a ridged buried clam"      clam       mollusk ""
teach "a rough pearl oyster"      oyster     mollusk istiridye
teach "a dark clustered mussel"   mussel     mollusk midye
teach "a slow spiral snail"       snail      mollusk salyangoz
teach "a soft slimy slug"         slug       mollusk ""
teach "a pale hovering cuttlefish" cuttlefish mollusk ""
teach "a coiled chambered nautilus" nautilus mollusk ""
nap

# --- Crustaceans -------------------------------------------------------------
teach "a sideways scuttling crab" crab       crustacean yengeç
teach "a blue clawed lobster"     lobster    crustacean ıstakoz
teach "a tiny pink shrimp"        shrimp     crustacean karides
teach "a curled glassy prawn"     prawn      crustacean ""
teach "a crusty clinging barnacle" barnacle  crustacean ""
teach "a brown freshwater crayfish" crayfish crustacean kerevit
teach "a faint swarming krill"    krill      crustacean ""
nap

# --- Cnidarians --------------------------------------------------------------
teach "a branching red coral"     coral      cnidarian mercan
teach "a waving purple anemone"   anemone    cnidarian ""
teach "a floating glowing jellyfish" jellyfish cnidarian denizanası
teach "a small green hydra"       hydra      cnidarian ""
nap

# --- Fish --------------------------------------------------------------------
teach "a long slithering eel"     eel        fish yılanbalığı
teach "a flat winged ray"         ray        fish ""
teach "a silver darting herring"  herring    fish ringa
teach "a striped quick mackerel"  mackerel   fish uskumru
teach "a large speckled cod"      cod        fish morina
teach "a small shiny sardine"     sardine    fish sardalya
teach "a broad mottled halibut"   halibut    fish ""
teach "a thin sandy sole"         sole       fish ""
nap

# --- Habitat: sea creatures live in the water --------------------------------
log "=== habitat: live ==="
fact "a crab lived in a sea";       fact "a lobster lived in a reef";   fact "a clam lived in a seabed"
fact "an oyster lived in an estuary"; fact "a coral lived in a reef";   fact "an eel lived in a rock"
fact "a jellyfish lived in an ocean"; fact "a snail lived in a tidepool"; fact "a mackerel lived in a sea"
fact "a herring lived in a sea";    fact "a shrimp lived in a bay";     fact "an anemone lived in a reef"
nap

# --- Diet: hunters eat sea creatures -----------------------------------------
log "=== diet: eat ==="
fact "a whale ate a herring";       fact "a whale ate a krill";         fact "a dolphin ate a mackerel"
fact "a crab ate a clam";           fact "a seal ate a herring";        fact "a gull ate a crab"
fact "a cod ate a shrimp";          fact "an eel ate a sardine";        fact "a ray ate a mussel"
fact "an otter ate an oyster";      fact "a shark ate a mackerel";      fact "a lobster ate a snail"
nap
log "########## sealife complete ##########"
