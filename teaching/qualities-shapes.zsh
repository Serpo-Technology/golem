#!/usr/bin/env zsh
# Colours, shapes, and textures: the qualities things carry, taught by telling, each
# placed under its kind and hung on the things that bear it. Builds on an existing
# world of objects and creatures.
#
#   GOLEM=golem GAP=1 zsh qualities-shapes.zsh
set -u
GOLEM=${GOLEM:-golem}
GAP=${GAP:-1}

log()  { print -r -- "$(date +%H:%M:%S) $*" }
sayw() { local i; for i in 1 2 3; do $GOLEM tell "$1" >/dev/null 2>&1 && { sleep 0.15; return 0 }; sleep 1; done; return 0 }
saytr(){ local i; for i in 1 2 3; do $GOLEM tell --lang tr "$1" >/dev/null 2>&1 && { sleep 0.15; return 0 }; sleep 1; done; return 0 }
fact() { sayw "$1"; sleep $GAP }
nap()  { log "(nap)"; $GOLEM sleep >/dev/null 2>&1; sleep "${BLOCKGAP:-6}" }
kindof() { local n=$1 k=$2; fact "a $n is a $k"; fact "a $n is a $k"; }

$GOLEM status >/dev/null 2>&1 || { log "daemon unreachable"; exit 1 }
log "########## qualities begin ##########"

# --- Colours -----------------------------------------------------------------
log "=== colours ==="
for c in red blue green yellow orange purple pink brown black white gray gold silver; do
  kindof "$c" "colour"
done
nap
fact "a rose is red";     fact "a sky is blue";    fact "a grass is green"
fact "a sun is yellow";   fact "a snow is white";  fact "a coal is black"
fact "a red is warmer than a blue"; fact "a black is darker than a white"
saytr "kırmızı bir renktir"; saytr "mavi bir renktir"; saytr "yeşil bir renktir"
nap

# --- Shapes ------------------------------------------------------------------
log "=== shapes ==="
for s in circle square triangle rectangle oval star diamond cube sphere cone cylinder ring; do
  kindof "$s" "shape"
done
nap
fact "a wheel is a circle";   fact "a window is a rectangle"; fact "a ball is a sphere"
fact "a box is a cube";       fact "a coin is a circle";      fact "a roof is a triangle"
fact "a circle is round";     fact "a square has a corner";   fact "a triangle has a corner"
saytr "daire bir şekildir"; saytr "kare bir şekildir"; saytr "üçgen bir şekildir"
nap

# --- Textures and qualities --------------------------------------------------
log "=== textures ==="
for t in soft hard smooth rough wet dry hot cold sharp dull heavy light bright dark; do
  kindof "$t" "quality"
done
nap
fact "a stone is hard";   fact "a pillow is soft";  fact "a glass is smooth"
fact "a fire is hot";     fact "an ice is cold";    fact "a knife is sharp"
fact "a rock is heavier than a feather"; fact "a sun is brighter than a moon"
fact "a silk is smoother than a bark";   fact "a desert is drier than a swamp"
saytr "taş serttir"; saytr "yastık yumuşaktır"; saytr "ateş sıcaktır"
nap
log "########## qualities complete ##########"
