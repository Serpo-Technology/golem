#!/usr/bin/env zsh
# A broad sweep of everyday things to widen the world further: kitchen and household
# objects, grains and nuts, family roles, and toys, each placed under its kind and tied
# in by a relation. Taught by telling for speed. A buffer of breadth over an existing
# world.
#
#   GOLEM=golem GAP=1 zsh more-things.zsh
set -u
GOLEM=${GOLEM:-golem}
GAP=${GAP:-1}

log()  { print -r -- "$(date +%H:%M:%S) $*" }
sayw() { local i; for i in 1 2 3; do $GOLEM tell "$1" >/dev/null 2>&1 && { sleep 0.15; return 0 }; sleep 1; done; return 0 }
saytr(){ local i; for i in 1 2 3; do $GOLEM tell --lang tr "$1" >/dev/null 2>&1 && { sleep 0.15; return 0 }; sleep 1; done; return 0 }
fact() { sayw "$1"; sleep $GAP }
nap()  { log "(nap)"; $GOLEM sleep >/dev/null 2>&1; sleep "${BLOCKGAP:-5}" }
kindof() { local n=$1 k=$2; fact "a $n is a $k"; fact "a $n is a $k"; }

$GOLEM status >/dev/null 2>&1 || { log "daemon unreachable"; exit 1 }
log "########## more things begin ##########"

# --- Kitchen -----------------------------------------------------------------
log "=== kitchen ==="
for k in spoon fork knife plate cup bowl kettle pan oven stove fridge sink teapot ladle; do
  kindof "$k" "utensil"
done
nap
fact "a knife cut a bread"; fact "a spoon held a soup"; fact "a cup held a tea"
fact "a pan fried an egg";  fact "an oven baked a cake"; fact "a fridge kept a milk"
saytr "kaşık bir mutfak aletidir"; saytr "bıçak bir mutfak aletidir"; saytr "tabak bir mutfak aletidir"
nap

# --- Household ---------------------------------------------------------------
log "=== household ==="
for h in lamp clock mirror door window carpet curtain pillow blanket broom bucket candle key; do
  kindof "$h" "household thing"
done
nap
fact "a lamp gave a light";   fact "a clock showed a time"; fact "a mirror showed a face"
fact "a key opened a door";   fact "a broom swept a floor"; fact "a candle burned a wax"
saytr "lamba bir ışık verdi"; saytr "saat bir zamanı gösterdi"; saytr "anahtar bir kapıyı açtı"
nap

# --- Grains and nuts ---------------------------------------------------------
log "=== grains and nuts ==="
for g in wheat rice corn oat barley rye; do kindof "$g" "grain"; done
for n in walnut almond peanut hazelnut chestnut acorn; do kindof "$n" "nut"; done
nap
fact "a bread came from a wheat"; fact "a rice grew in a field"; fact "a squirrel ate a walnut"
fact "an oat fed a horse";        fact "a corn grew tall";       fact "an acorn came from an oak"
nap

# --- Family and people -------------------------------------------------------
log "=== family ==="
for p in mother father sister brother child baby friend neighbor king queen; do
  kindof "$p" "person"
done
nap
fact "a mother held a baby";   fact "a father loved a child"; fact "a king ruled a country"
fact "a queen wore a crown";   fact "a friend helped a friend"; fact "a sister has a brother"
saytr "anne bir insandır"; saytr "baba bir insandır"; saytr "kral bir ülkeyi yönetti"
nap

# --- Toys and play -----------------------------------------------------------
log "=== toys ==="
for t in ball doll kite balloon puzzle marble top swing slide; do
  kindof "$t" "toy"
done
nap
fact "a child played a ball";  fact "a kite flew in a wind"; fact "a balloon held an air"
fact "a doll is a toy";        fact "a child rode a swing";  fact "a marble is round"
nap
log "########## more things complete ##########"
