#!/usr/bin/env zsh
# A cabinet of kitchenware: many implements across three kinds (cookware, utensil,
# dishware), each shown on its own percept, named, placed under its kind and under
# the kitchenware superordinate, and tied into the world by what a cook does with it
# and what it holds. The kinds abstract from the shared naming; the stir and hold
# relations weave the implements to the cooks and the food. Builds on an existing world.
#
#   GOLEM=golem GAP=1 zsh kitchenware.zsh
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
    (( i == 2 )) && sayw "$kind"; (( i == 3 )) && sayw "kitchenware"
    (( i == 4 )) && { [ -n "$tr" ] && saytr "$tr" }
    (( i == 5 )) && sayw "$kind"
    comp; sleep $GAP
  done
}
fact() { sayw "$1"; sleep $GAP }

$GOLEM status >/dev/null 2>&1 || { log "daemon unreachable"; exit 1 }
log "########## kitchenware begins ##########"

# --- Cookware ----------------------------------------------------------------
teach "a dented copper pot"        pot      cookware tencere
teach "a heavy iron pan"           pan      cookware tava
teach "a whistling steel kettle"   kettle   cookware çaydanlık
teach "a greasy cast skillet"      skillet  cookware ""
teach "a bubbling black cauldron"  cauldron cookware kazan
teach "a rounded carbon wok"       wok      cookware ""
teach "a flat ridged griddle"      griddle  cookware ""
teach "a small enamel saucepan"    saucepan cookware ""
nap

# --- Utensils ----------------------------------------------------------------
teach "a deep silver ladle"        ladle      utensil kepçe
teach "a flat rubber spatula"      spatula    utensil ""
teach "a looped wire whisk"        whisk      utensil ""
teach "a sharp steel grater"       grater     utensil rende
teach "a fine mesh sieve"          sieve      utensil elek
teach "a perforated metal colander" colander  utensil süzgeç
teach "a springy steel tongs"      tongs      utensil maşa
teach "a nimble swivel peeler"     peeler     utensil ""
teach "a sturdy potato masher"     masher     utensil ""
teach "a smooth maple rollingpin"  rollingpin utensil oklava
nap

# --- Dishware ----------------------------------------------------------------
teach "a chipped ceramic plate"    plate   dishware tabak
teach "a deep wooden bowl"         bowl    dishware kase
teach "a delicate painted saucer"  saucer  dishware ""
teach "a chunky clay mug"          mug     dishware kupa
teach "a jeweled golden goblet"    goblet  dishware kadeh
teach "a tall glazed pitcher"      pitcher dishware sürahi
teach "a squat porcelain teapot"   teapot  dishware demlik
teach "a broad oval platter"       platter dishware ""
teach "a lidded soup tureen"       tureen  dishware ""
teach "a tiny fluted ramekin"      ramekin dishware ""
nap

# --- Use: cooks handle, wares hold -------------------------------------------
log "=== use: cook and hold ==="
fact "a cook stirred a pot";     fact "a chef flipped a pan";    fact "a cook filled a kettle"
fact "a chef seasoned a skillet"; fact "a ladle scooped a broth"; fact "a grater shredded a cheese"
fact "a plate held a meal";      fact "a bowl held a soup";      fact "a mug held a coffee"
fact "a pitcher held a water";   fact "a teapot held a tea";     fact "a goblet held a wine"
nap
log "########## kitchenware complete ##########"
