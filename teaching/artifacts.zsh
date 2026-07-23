#!/usr/bin/env zsh
# A wide world of made things: tools, furniture, vehicles, clothing, containers,
# instruments, and buildings, each shown, named, placed under its kind and under the
# thing superordinate, and tied in by what it is used for and made of. Builds on an
# existing world.
#
#   GOLEM=golem GAP=1 zsh artifacts.zsh
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

# teach: caption name kind tr  -- placed under its kind and under "thing".
teach() {
  local cap=$1 name=$2 kind=$3 tr=$4 i
  log "teach: $name ($kind)"
  for i in 1 2 3 4 5; do
    fobs "$cap"; sayw "$name"
    (( i == 2 )) && sayw "$kind"; (( i == 3 )) && sayw "thing"
    (( i == 4 )) && { [ -n "$tr" ] && saytr "$tr" }
    (( i == 5 )) && sayw "$kind"
    comp; sleep $GAP
  done
}
fact() { sayw "$1"; sleep $GAP }

$GOLEM status >/dev/null 2>&1 || { log "daemon unreachable"; exit 1 }
log "########## artifacts begins ##########"

# --- Tools -------------------------------------------------------------------
teach "a heavy steel hammer"   hammer tool çekiç
teach "a sharp steel saw"      saw    tool testere
teach "a small steel drill"    drill  tool matkap
teach "a red steel wrench"     wrench tool anahtar
teach "a sharp steel axe"      axe    tool balta
teach "a small steel screwdriver" screwdriver tool tornavida
teach "a long wooden shovel"   shovel tool kürek
teach "a metal garden rake"    rake   tool tırmık
nap

# --- Furniture ---------------------------------------------------------------
teach "a wooden brown chair"   chair  furniture sandalye
teach "a low round coffeetable"    coffeetable  furniture sehpa
teach "a soft brown couch"     couch  furniture kanepe
teach "a tall wooden shelf"    shelf  furniture raf
teach "a wide soft mattress"        mattress    furniture şilte
teach "a wooden brown desk"    desk   furniture sıra
teach "a tall wooden cabinet"  cabinet furniture dolap
nap

# --- Vehicles ----------------------------------------------------------------
teach "a red fast car"         car    vehicle araba
teach "a large blue truck"     truck  vehicle kamyon
teach "a yellow long bus"      bus    vehicle otobüs
teach "a long silver train"    train  vehicle tren
teach "a small wooden boat"    boat   vehicle tekne
teach "a large white plane"    plane  vehicle uçak
teach "a red two-wheeled bicycle" bicycle vehicle bisiklet
teach "a fast black motorcycle" motorcycle vehicle motosiklet
nap

# --- Clothing ----------------------------------------------------------------
teach "a blue cotton shirt"    shirt  clothing gömlek
teach "a warm wool coat"       coat   clothing palto
teach "a round brown hat"      hat    clothing şapka
teach "a black leather shoe"   shoe   clothing ayakkabı
teach "a warm wool glove"      glove  clothing eldiven
teach "a blue denim trouser"   trouser clothing pantolon
teach "a long red scarf"       scarf  clothing atkı
nap

# --- Containers, instruments, buildings --------------------------------------
teach "a round clay pot"       pot    container tencere
teach "a glass water bottle"   bottle container şişe
teach "a woven straw basket"   basket container sepet
teach "a brown leather knapsack"    knapsack    container ""
nap
teach "a brown wooden drum"    drum   instrument davul
teach "a silver metal flute"   flute  instrument flüt
teach "a wooden brown guitar"  guitar instrument gitar
teach "a golden brass trumpet" trumpet instrument trompet
teach "a black grand piano"    piano  instrument piyano
nap
teach "a tall brick house"     house  building ev
teach "a tall stone obelisk"     obelisk  building ""
teach "a long stone causeway"    causeway building ""
teach "a large stone castle"   castle building kale
teach "a tall glass skyscraper" skyscraper building gökdelen
nap

# --- Relations: use and material ---------------------------------------------
log "=== use and material ==="
fact "a hammer drove a nail"; fact "a saw cut a wood"; fact "an axe split a log"
fact "a key opened a lock";   fact "a car carried a person"; fact "a boat crossed a river"
fact "a plane crossed a sky"; fact "a drum made a sound";    fact "a flute made a music"
fact "a knife cut a bread";   fact "a pot held a soup";      fact "a basket held a fruit"
nap
fact "a chair is wooden";  fact "a hammer is steel"; fact "a shirt is cotton"
fact "a bottle is glass";  fact "a coat is wool";    fact "a castle is stone"
fact "a boat is wooden";   fact "a window is glass"; fact "a coin is copper"
nap
log "########## artifacts complete ##########"
