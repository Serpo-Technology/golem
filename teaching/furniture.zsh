#!/usr/bin/env zsh
# A house of furniture: many pieces across four kinds (seat, table, storage, bed),
# each shown on its own percept, named, placed under its kind and under the
# furniture superordinate, and tied into the world by who sits on it and what rests
# on it. The kinds abstract from the shared naming; the sit and rest relations weave
# the pieces to the people and things they hold. Builds on an existing world.
#
#   GOLEM=golem GAP=1 zsh furniture.zsh
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
    (( i == 2 )) && sayw "$kind"; (( i == 3 )) && sayw "furniture"
    (( i == 4 )) && { [ -n "$tr" ] && saytr "$tr" }
    (( i == 5 )) && sayw "$kind"
    comp; sleep $GAP
  done
}
fact() { sayw "$1"; sleep $GAP }

$GOLEM status >/dev/null 2>&1 || { log "daemon unreachable"; exit 1 }
log "########## furniture begins ##########"

# --- Seats -------------------------------------------------------------------
teach "a slatted oak chair"        chair    seat sandalye
teach "a tall wooden stool"        stool    seat tabure
teach "a long stone bench"         bench    seat ""
teach "a plush velvet sofa"        sofa     seat kanepe
teach "a worn leather armchair"    armchair seat koltuk
teach "a gilded royal throne"      throne   seat taht
teach "a sagging corduroy couch"   couch    seat ""
teach "a padded tilting recliner"  recliner seat ""
nap

# --- Tables ------------------------------------------------------------------
teach "a cluttered writing desk"   desk       table ""
teach "a polished granite counter" counter    table tezgah
teach "a scarred oily workbench"   workbench  table ""
teach "a small brass nightstand"   nightstand table ""
teach "a slanted oak lectern"      lectern    table ""
nap

# --- Storage -----------------------------------------------------------------
teach "a wide oak dresser"         dresser   storage ""
teach "a tall glass cabinet"       cabinet   storage ""
teach "a painted corner cupboard"  cupboard  storage ""
teach "a narrow floating shelf"    shelf     storage raf
teach "a towering walnut bookcase" bookcase  storage kitaplık
teach "a mirrored double wardrobe" wardrobe  storage gardırop
teach "a banded iron chest"        chest     storage sandık
teach "a long mahogany sideboard"  sideboard storage ""
teach "a dented metal locker"      locker    storage ""
nap

# --- Beds --------------------------------------------------------------------
teach "a slatted pine crib"        crib     bed beşik
teach "a stacked metal bunk"       bunk     bed ""
teach "a rocking wicker cradle"    cradle   bed ""
teach "a swaying rope hammock"     hammock  bed hamak
teach "a folding canvas cot"       cot      bed ""
teach "a frilly white bassinet"    bassinet bed ""
nap

# --- Use: people sit, things rest --------------------------------------------
log "=== use: sit and rest ==="
fact "a person sat on a chair";  fact "a child sat on a stool";      fact "a guest sat on a sofa"
fact "a king sat on a throne";   fact "a cat curled on an armchair"; fact "a worker sat on a bench"
fact "a book rested on a shelf"; fact "a lamp stood on a desk";      fact "a plate lay on a counter"
fact "a baby slept in a crib";   fact "a coat hung in a wardrobe";   fact "a tool lay on a workbench"
nap
log "########## furniture complete ##########"
