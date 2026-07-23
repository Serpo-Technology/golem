#!/usr/bin/env zsh
# A wide playroom of toys across three kinds (figure, plaything, puzzle), each shown
# on its own percept, named, placed under its kind and under the toy superordinate,
# and tied into the world by who plays with it and how it moves. The kinds abstract
# from the shared naming; the play and motion relations weave the toys to the children
# who hold them. Builds on an existing world.
#
#   GOLEM=golem GAP=1 zsh toys.zsh
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
    (( i == 2 )) && sayw "$kind"; (( i == 3 )) && sayw "toy"
    (( i == 4 )) && { [ -n "$tr" ] && saytr "$tr" }
    (( i == 5 )) && sayw "$kind"
    comp; sleep $GAP
  done
}
fact() { sayw "$1"; sleep $GAP }

$GOLEM status >/dev/null 2>&1 || { log "daemon unreachable"; exit 1 }
log "########## toys begins ##########"

# --- Figures -----------------------------------------------------------------
teach "a smiling painted doll"    doll        figure bebek
teach "a dangling cloth puppet"   puppet      figure kukla
teach "a fuzzy brown teddy"       teddy       figure ""
teach "a stringed wooden marionette" marionette figure ""
teach "a tiny pink dollhouse"     dollhouse   figure ""
teach "a carved white rockinghorse" rockinghorse figure ""
teach "a small glazed figurine"   figurine    figure ""
nap

# --- Playthings --------------------------------------------------------------
teach "a bright paper kite"       kite        plaything uçurtma
teach "a spinning striped top"    top         plaything topaç
teach "a swirled glass marble"    marble      plaything misket
teach "a red bouncing yoyo"       yoyo        plaything ""
teach "a small metal jack"        jack        plaything ""
teach "a round wooden hoop"       hoop        plaything çember
nap
teach "a loud plastic rattle"     rattle      plaything çıngırak
teach "a floating blue balloon"   balloon     plaything balon
teach "a shrill silver whistle"   whistle     plaything düdük
teach "a bent forked slingshot"   slingshot   plaything sapan
teach "a colorful whirling pinwheel" pinwheel plaything ""
teach "a bright twisting kaleidoscope" kaleidoscope plaything ""
nap

# --- Puzzles -----------------------------------------------------------------
teach "a bright wooden blocks"    blocks      puzzle ""
teach "a scattered cardboard jigsaw" jigsaw   puzzle yapboz
teach "a bright cubic rubik"      rubik       puzzle ""
teach "a flat angular tangram"    tangram     puzzle ""
teach "a winding printed maze"    maze        puzzle labirent
teach "a black spotted domino"    domino      puzzle ""
teach "a wooden sliding abacus"   abacus      puzzle ""
nap

# --- Play: children play with toys -------------------------------------------
log "=== play ==="
fact "a child played with a doll";  fact "a child played with a top";   fact "a boy played with a kite"
fact "a girl played with a puppet"; fact "a baby played with a rattle"; fact "a child played with a yoyo"
fact "a boy played with a marble";  fact "a girl played with a jigsaw"; fact "a child played with a blocks"
fact "a kid played with a hoop";    fact "a girl played with a dollhouse"; fact "a boy played with a slingshot"
nap

# --- Motion: toys move and fly -----------------------------------------------
log "=== motion ==="
fact "a kite flew in a wind";       fact "a balloon floated in an air"; fact "a top spun on a floor"
fact "a yoyo bounced on a string";  fact "a marble rolled on a floor";  fact "a hoop rolled on a path"
fact "a kite soared in a sky";      fact "a balloon drifted in a sky";  fact "a pinwheel spun in a breeze"
nap
log "########## toys complete ##########"
