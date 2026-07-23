#!/usr/bin/env zsh
# The natural world and the body, taught by telling: body parts, materials, weather,
# landforms, the sky, and gems, each placed under its kind and tied in by a relation
# or two. Builds on an existing world.
#
#   GOLEM=golem GAP=1 zsh nature-body.zsh
set -u
GOLEM=${GOLEM:-golem}
GAP=${GAP:-1}

log()  { print -r -- "$(date +%H:%M:%S) $*" }
sayw() { local i; for i in 1 2 3; do $GOLEM tell "$1" >/dev/null 2>&1 && { sleep 0.15; return 0 }; sleep 1; done; return 0 }
saytr(){ local i; for i in 1 2 3; do $GOLEM tell --lang tr "$1" >/dev/null 2>&1 && { sleep 0.15; return 0 }; sleep 1; done; return 0 }
fact() { sayw "$1"; sleep $GAP }
nap()  { log "(nap)"; $GOLEM sleep >/dev/null 2>&1; sleep "${BLOCKGAP:-6}" }
# noun kind: several tellings so a perceptless noun mints from its company.
kindof() { local n=$1 k=$2; fact "a $n is a $k"; fact "a $n is a $k"; }

$GOLEM status >/dev/null 2>&1 || { log "daemon unreachable"; exit 1 }
log "########## nature and body begin ##########"

# --- Body parts --------------------------------------------------------------
log "=== body parts ==="
for b in hand foot arm leg head eye ear nose mouth tooth finger heart lung bone skin; do
  kindof "$b" "body part"
done
nap
fact "a hand has a finger"; fact "a foot is on a leg"; fact "an eye is on a head"
fact "a mouth has a tooth"; fact "an arm has a hand"; fact "a head has an eye"
fact "a heart pumps a blood"; fact "a lung holds an air"; fact "a bone is hard"
saytr "el bir vücut parçasıdır"; saytr "göz bir vücut parçasıdır"; saytr "kalp bir vücut parçasıdır"
nap

# --- Materials ---------------------------------------------------------------
log "=== materials ==="
for m in wood glass metal stone cloth plastic paper leather rubber clay concrete; do
  kindof "$m" "material"
done
nap
fact "a chair is made of wood"; fact "a window is made of glass"; fact "a wall is made of stone"
fact "a shirt is made of cloth"; fact "a book is made of paper"; fact "a tire is made of rubber"
fact "wood came from a tree";   fact "paper came from a wood";   fact "glass is clear"
saytr "ahşap bir malzemedir"; saytr "cam bir malzemedir"; saytr "taş bir malzemedir"
nap

# --- Weather -----------------------------------------------------------------
log "=== weather ==="
for w in rain snow wind cloud storm fog frost thunder lightning hail; do
  kindof "$w" "weather"
done
nap
fact "a rain fell from a cloud"; fact "a snow is cold"; fact "a wind moved a leaf"
fact "a storm brought a rain";   fact "a fog hid a road"; fact "a thunder followed a lightning"
saytr "yağmur bir havadır"; saytr "kar soğuktur"; saytr "rüzgar bir yaprağı hareket ettirdi"
nap

# --- Landforms ---------------------------------------------------------------
log "=== landforms ==="
for l in mountain hill valley river lake ocean sea desert forest island cliff cave plain swamp; do
  kindof "$l" "landform"
done
nap
fact "a mountain is tall";      fact "a river flowed to a sea"; fact "a lake held a water"
fact "an island sat in an ocean"; fact "a desert is dry";       fact "a forest held a tree"
fact "a mountain is taller than a hill"; fact "an ocean is bigger than a lake"
fact "a river is longer than a stream"; fact "a valley lies between a mountain"
saytr "dağ bir arazidir"; saytr "nehir bir denize aktı"; saytr "orman bir ağaç tuttu"
nap

# --- The sky -----------------------------------------------------------------
log "=== sky ==="
for s in sun moon star planet comet cloud sky rainbow; do
  kindof "$s" "sky thing"
done
nap
fact "a sun gave a light"; fact "a moon circled an earth"; fact "a star shone at night"
fact "a sun is brighter than a moon"; fact "a sun is hotter than a star"
saytr "güneş bir ışık verdi"; saytr "ay bir dünyayı çevreledi"; saytr "yıldız gece parladı"
nap

# --- Gems and precious metals ------------------------------------------------
log "=== gems ==="
for g in diamond ruby emerald sapphire pearl gold silver bronze; do
  kindof "$g" "gem"
done
nap
fact "a diamond is hard";   fact "a gold is precious"; fact "a ruby is red"
fact "an emerald is green"; fact "a sapphire is blue"; fact "a diamond is harder than a stone"
fact "a gold is more precious than a copper"
saytr "elmas serttir"; saytr "altın değerlidir"; saytr "yakut kırmızıdır"
nap
log "########## nature and body complete ##########"
