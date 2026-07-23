#!/usr/bin/env zsh
# A wide armory of old arms across four kinds (blade, polearm, ranged, bludgeon), each
# shown on its own percept, named, placed under its kind and under the weapon
# superordinate, and tied into the world by who bears it and what it strikes. The kinds
# abstract from the shared naming; the wield and battle relations weave the arms to the
# fighters who carry them. Builds on an existing world.
#
#   GOLEM=golem GAP=1 zsh oldweapons.zsh
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
    (( i == 2 )) && sayw "$kind"; (( i == 3 )) && sayw "weapon"
    (( i == 4 )) && { [ -n "$tr" ] && saytr "$tr" }
    (( i == 5 )) && sayw "$kind"
    comp; sleep $GAP
  done
}
fact() { sayw "$1"; sleep $GAP }

$GOLEM status >/dev/null 2>&1 || { log "daemon unreachable"; exit 1 }
log "########## old weapons begins ##########"

# --- Blades ------------------------------------------------------------------
teach "a sharp steel sword"       sword      blade kılıç
teach "a slim pointed dagger"     dagger     blade hançer
teach "a curved cavalry saber"    saber      blade ""
teach "a thin dueling rapier"     rapier     blade ""
teach "a broad heavy cutlass"     cutlass    blade ""
teach "a wide chopping machete"   machete    blade ""
nap

# --- Polearms ----------------------------------------------------------------
teach "a long wooden spear"       spear      polearm mızrak
teach "a tall jousting lance"     lance      polearm ""
teach "a hooked steel halberd"    halberd    polearm ""
teach "a stout infantry pike"     pike       polearm ""
teach "a three pronged trident"   trident    polearm ""
teach "a sharp sweeping glaive"   glaive     polearm ""
nap

# --- Ranged ------------------------------------------------------------------
teach "a bent horn bow"           bow        ranged yay
teach "a heavy loaded crossbow"   crossbow   ranged ""
teach "a woven leather sling"     sling      ranged ""
teach "a huge wooden catapult"    catapult   ranged mancınık
teach "a light throwing javelin"  javelin    ranged cirit
teach "a giant bolt ballista"     ballista   ranged ""
teach "a small feathered dart"    dart       ranged ""
nap

# --- Bludgeons ---------------------------------------------------------------
teach "a spiked iron mace"        mace       bludgeon ""
teach "a swinging chained flail"  flail      bludgeon ""
teach "a massive blunt warhammer" warhammer  bludgeon ""
teach "a rough knotted club"      club       bludgeon ""
teach "a barbed round morningstar" morningstar bludgeon ""
teach "a short thick cudgel"      cudgel     bludgeon ""
teach "a round wooden shield"     shield     bludgeon kalkan
nap

# --- Wield: fighters bear arms -----------------------------------------------
log "=== wield ==="
fact "a soldier held a sword";      fact "a knight held a lance";       fact "a guard held a spear"
fact "an archer drew a bow";        fact "a hunter drew a crossbow";    fact "a warrior swung a mace"
fact "a soldier raised a shield";   fact "a fencer held a rapier";      fact "a pikeman held a pike"
fact "a slinger whirled a sling";   fact "a gladiator held a trident";  fact "a soldier hurled a javelin"
nap

# --- Battle: arms strike in war ----------------------------------------------
log "=== battle ==="
fact "a sword cut a rope";          fact "a spear pierced a shield";    fact "an arrow flew from a bow"
fact "a catapult flung a stone";    fact "a dagger pierced a hide";     fact "a mace crushed a helmet"
fact "a lance struck a rider";      fact "a sword clashed on a shield"; fact "a bow launched an arrow"
nap
log "########## old weapons complete ##########"
