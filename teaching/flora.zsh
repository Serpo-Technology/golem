#!/usr/bin/env zsh
# A wide flora: trees, flowers, fruits, and vegetables, each shown, named, placed
# under its kind and under the plant superordinate, and tied in by what grows on what
# and what has what colour. Builds on an existing world.
#
#   GOLEM=golem GAP=1 zsh flora.zsh
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
     woven carved folded tangled looming narrow hidden sloping creaking shimmering idle)
nn=(fountain archway corridor tunnel harbor plaza kiosk streetlamp balcony staircase
    courtyard terrace rooftop chimney beacon lighthouse pier canal footpath lane ravine)
ci=0
comp() { obs "a $adj[$(( (ci % ${#adj[@]}) + 1 ))] $nn[$(( (ci % ${#nn[@]}) + 1 ))]"; ci=$((ci+1)) }

teach() {
  local cap=$1 name=$2 kind=$3 tr=$4 i
  log "teach: $name ($kind)"
  for i in 1 2 3 4 5; do
    fobs "$cap"; sayw "$name"
    (( i == 2 )) && sayw "$kind"; (( i == 3 )) && sayw "plant"
    (( i == 4 )) && { [ -n "$tr" ] && saytr "$tr" }
    (( i == 5 )) && sayw "$kind"
    comp; sleep $GAP
  done
}
fact() { sayw "$1"; sleep $GAP }

$GOLEM status >/dev/null 2>&1 || { log "daemon unreachable"; exit 1 }
log "########## flora begins ##########"

# --- Trees -------------------------------------------------------------------
teach "a tall green oak"     oak    tree meşe
teach "a tall dark pine"     pine   tree çam
teach "a white slender birch" birch tree huş
teach "a broad green maple"  maple  tree akçaağaç
teach "a weeping green willow" willow tree söğüt
teach "a tall green palm"    palm   tree palmiye
teach "a dark green cedar"   cedar  tree sedir
nap

# --- Flowers -----------------------------------------------------------------
teach "a red garden rose"    rose    flower gül
teach "a yellow tall tulip"  tulip   flower lale
teach "a white round daisy"  daisy   flower papatya
teach "a purple tall lily"   lily    flower zambak
teach "a blue small violet"  violet  flower menekşe
teach "a tall yellow sunflower" sunflower flower ayçiçeği
teach "a red wild poppy"     poppy   flower gelincik
teach "a pink garden orchid" orchid  flower orkide
nap

# --- Fruits ------------------------------------------------------------------
teach "a round red apple"    apple  fruit elma
teach "a yellow curved banana" banana fruit muz
teach "a round orange orange" orange fruit portakal
teach "a small red cherry"   cherry fruit kiraz
teach "a purple round grape" grape  fruit üzüm
teach "a yellow soft pear"   pear   fruit armut
teach "a red juicy strawberry" strawberry fruit çilek
teach "a green sour lime"    lime   fruit misket
teach "a yellow sour lemon"  lemon  fruit limon
teach "a brown fuzzy kiwi"   kiwi   fruit kivi
nap

# --- Vegetables --------------------------------------------------------------
teach "a long orange carrot" carrot vegetable havuç
teach "a round brown potato" potato vegetable patates
teach "a round white onion"  onion  vegetable soğan
teach "a green round pea"    pea    vegetable bezelye
teach "a red round tomato"   tomato vegetable domates
teach "a green leafy lettuce" lettuce vegetable marul
teach "a green long cucumber" cucumber vegetable salatalık
teach "a purple round eggplant" eggplant vegetable patlıcan
nap

# --- Relations: what grows on what, colours -----------------------------------
log "=== growth and colour ==="
fact "an apple grew on an oak"; fact "a cherry grew on a tree"; fact "a lemon grew on a tree"
fact "a rose grew in a garden"; fact "a tulip grew in a garden"; fact "a grape grew on a vine"
fact "a potato grew in a soil"; fact "a carrot grew in a soil";  fact "an acorn grew on an oak"
nap
fact "a rose is red";   fact "a sunflower is yellow"; fact "a violet is blue"
fact "a lemon is yellow"; fact "a lime is green";     fact "a cherry is red"
fact "a leaf is green"; fact "a carrot is orange";    fact "an eggplant is purple"
nap
log "########## flora complete ##########"
