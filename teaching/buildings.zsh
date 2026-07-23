#!/usr/bin/env zsh
# The built world: many structures across four kinds (dwelling, tower, bridge, place),
# each shown on its own percept, named, placed under its kind and under the building
# superordinate, then set to use by who lives in one and what one spans. The kinds
# abstract from the shared naming; the relations put the structures to work. Builds on an
# existing world.
#
#   GOLEM=golem GAP=1 zsh buildings.zsh
set -u
GOLEM=${GOLEM:-golem}
GAP=${GAP:-1}

log()  { print -r -- "$(date +%H:%M:%S) $*" }
obs()  { local i; for i in 1 2 3; do $GOLEM observe vision "$1" >/dev/null 2>&1 && { sleep 0.15; return 0 }; sleep 1; done; return 0 }
fobs() { local i; for i in 1 2 3; do $GOLEM observe --force vision "$1" >/dev/null 2>&1 && { sleep 0.15; return 0 }; sleep 1; done; return 0 }
sayw() { local i; for i in 1 2 3; do $GOLEM tell "$1" >/dev/null 2>&1 && { sleep 0.15; return 0 }; sleep 1; done; return 0 }
saytr(){ local i; for i in 1 2 3; do $GOLEM tell --lang tr "$1" >/dev/null 2>&1 && { sleep 0.15; return 0 }; sleep 1; done; return 0 }
nap()  { log "(nap)"; $GOLEM sleep >/dev/null 2>&1; sleep "${BLOCKGAP:-6}" }

adj=(drifting quiet busy faded gentle sudden hollow crisp woven folded tangled humming
     idle weary swaying rippling glowing ticking distant narrow looming hidden creaking
     flickering whistling murmuring winding fraying gleaming brimming)
nn=(basket lantern ribbon kettle cushion satchel teapot candle mirror blanket sketch
    melody whisper puzzle lattice garland bundle parcel thimble spindle bobbin trinket
    goblet flagon scroll quill inkwell locket pendant charm)
ci=0
comp() { obs "a $adj[$(( (ci % ${#adj[@]}) + 1 ))] $nn[$(( (ci % ${#nn[@]}) + 1 ))]"; ci=$((ci+1)) }

# teach: caption name subkind tr   -- five showings, named at three levels, bilingual.
teach() {
  local cap=$1 name=$2 kind=$3 tr=$4 i
  log "teach: $name ($kind)"
  for i in 1 2 3 4 5; do
    fobs "$cap"; sayw "$name"
    (( i == 2 )) && sayw "$kind"; (( i == 3 )) && sayw "building"
    (( i == 4 )) && { [ -n "$tr" ] && saytr "$tr" }
    (( i == 5 )) && sayw "$kind"
    comp; sleep $GAP
  done
}
fact() { sayw "$1"; sleep $GAP }

$GOLEM status >/dev/null 2>&1 || { log "daemon unreachable"; exit 1 }
log "########## buildings begins ##########"

# --- Dwellings ---------------------------------------------------------------
teach "a cozy red house"           house      dwelling ev
teach "a small thatched cottage"   cottage    dwelling ""
teach "a rustic wooden cabin"      cabin      dwelling kulübe
teach "a grand royal palace"       palace     dwelling saray
teach "a tall gray castle"         castle     dwelling kale
teach "a tiny round hut"           hut        dwelling ""
teach "a large elegant mansion"    mansion    dwelling konak
teach "a bright seaside villa"     villa      dwelling ""
nap

# --- Towers ------------------------------------------------------------------
teach "a tall striped lighthouse"  lighthouse tower ""
teach "a white spinning windmill"  windmill   tower ""
teach "a slender pointed minaret"  minaret    tower minare
teach "a round gray silo"          silo       tower ""
teach "a tall carved obelisk"      obelisk    tower dikilitaş
teach "a sharp white steeple"      steeple    tower ""
teach "a thin soaring spire"       spire      tower ""
nap

# --- Bridges -----------------------------------------------------------------
teach "a long arched viaduct"      viaduct    bridge viyadük
teach "an old roman aqueduct"      aqueduct   bridge ""
teach "a narrow wooden footbridge" footbridge bridge ""
teach "a heavy oak drawbridge"     drawbridge bridge ""
teach "a wide concrete overpass"   overpass   bridge ""
nap

# --- Places ------------------------------------------------------------------
teach "a big red barn"             barn       place ahır
teach "a stout old mill"           mill       place değirmen
teach "an ancient columned temple" temple     place tapınak
teach "an old white church"        church     place kilise
teach "a domed blue mosque"        mosque     place cami
teach "a smoky brick factory"      factory    place fabrika
teach "a vast empty warehouse"     warehouse  place depo
teach "a tall wooden granary"      granary    place ambar
nap

# --- Structures in use -------------------------------------------------------
log "=== in use ==="
fact "a person lived in a house";  fact "a king lived in a palace";   fact "a farmer lived in a cottage"
fact "a bridge crossed a river";   fact "a viaduct crossed a valley"; fact "a lighthouse warned a ship"
fact "a windmill ground a grain";  fact "a monk prayed in a temple";  fact "a crowd filled a church"
fact "a factory made a good";      fact "a warehouse held a crate";   fact "a barn stored a hay"
nap
log "########## buildings complete ##########"
