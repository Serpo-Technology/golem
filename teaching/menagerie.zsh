#!/usr/bin/env zsh
# A wide menagerie: many creatures across five kinds (mammal, bird, fish, insect,
# reptile), each shown on its own percept, named, placed under its kind and under the
# animal superordinate, and tied into the world by what it eats and where it lives.
# The kinds abstract from the shared naming; the eat and live relations weave the
# animals to the foods and habitats. Builds on an existing world.
#
#   GOLEM=golem GAP=1 zsh menagerie.zsh
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
    (( i == 2 )) && sayw "$kind"; (( i == 3 )) && sayw "animal"
    (( i == 4 )) && { [ -n "$tr" ] && saytr "$tr" }
    (( i == 5 )) && sayw "$kind"
    comp; sleep $GAP
  done
}
fact() { sayw "$1"; sleep $GAP }

$GOLEM status >/dev/null 2>&1 || { log "daemon unreachable"; exit 1 }
log "########## menagerie begins ##########"

# --- Mammals -----------------------------------------------------------------
teach "a large gray elephant"   elephant mammal fil
teach "a tall spotted giraffe"  giraffe  mammal zürafa
teach "a striped orange tiger"  tiger    mammal kaplan
teach "a heavy brown bear"      bear     mammal ayı
teach "a woolly white sheep"    sheep    mammal koyun
teach "a pink farm pig"         pig      mammal domuz
teach "a small brown squirrel"  squirrel mammal sincap
teach "a gray hopping kangaroo" kangaroo mammal kanguru
teach "a black striped zebra"   zebra    mammal zebra
teach "a small red fox"         fox      mammal tilki
nap
teach "a large gray whale"      whale    mammal balina
teach "a playful gray dolphin"  dolphin  mammal yunus
teach "a brown farm cow"        cow      mammal inek
teach "a woolly brown goat"     goat     mammal keçi
teach "a small brown monkey"    monkey   mammal maymun
nap

# --- Birds -------------------------------------------------------------------
teach "a small brown sparrow"   sparrow bird serçe
teach "a large brown eagle"     eagle   bird kartal
teach "a white gliding swan"    swan    bird kuğu
teach "a green talking parrot"  parrot  bird papağan
teach "a black cawing crow"     crow    bird karga
teach "a red robin"        robin   bird nakış
teach "a small gray owl"        owl     bird baykuş
teach "a white farm duck"       duck    bird ördek
teach "a pink standing flamingo" flamingo bird flamingo
teach "a small blue kingfisher" kingfisher bird yalıçapkını
nap

# --- Fish --------------------------------------------------------------------
teach "a silver swimming salmon" salmon fish somon
teach "a small striped bass"     bass   fish levrek
teach "a large blue tuna"        tuna   fish tonbalığı
teach "a long green eel"         eel    fish yılanbalığı
teach "a flat gray flounder"     flounder fish pisi
teach "a small orange goldfish"  goldfish fish japonbalığı
nap

# --- Insects -----------------------------------------------------------------
teach "a small black ant"        ant    insect karınca
teach "a striped yellow bee"     bee    insect arı
teach "an orange winged butterfly" butterfly insect kelebek
teach "a green jumping grasshopper" grasshopper insect çekirge
teach "a small red beetle"       beetle insect böcek
teach "a black buzzing fly"      fly    insect sinek
nap

# --- Reptiles ----------------------------------------------------------------
teach "a green scaly lizard"     lizard reptile kertenkele
teach "a long green snake"       snake  reptile yılan
teach "a slow green turtle"      turtle reptile kaplumbağa
teach "a large green crocodile"  crocodile reptile timsah
teach "a small brown gecko"      gecko  reptile geko
nap

# --- Diet: animals eat foods and plants --------------------------------------
log "=== diet: eat ==="
fact "an elephant ate a grass"; fact "a giraffe ate a leaf"; fact "a tiger ate a meat"
fact "a bear ate a honey";      fact "a sheep ate a grass"; fact "a squirrel ate a nut"
fact "a whale ate a fish";      fact "a dolphin ate a fish"; fact "a cow ate a grass"
fact "a monkey ate a banana";   fact "an owl ate a mouse";   fact "an eagle ate a rabbit"
fact "a bee ate a honey";       fact "a snake ate a mouse";  fact "a fox ate a rabbit"
nap

# --- Habitat: animals live in places -----------------------------------------
log "=== habitat: live ==="
fact "a whale lived in an ocean";  fact "a salmon lived in a river"; fact "a dolphin lived in an ocean"
fact "a lizard lived in a desert"; fact "a bear lived in a forest";  fact "an owl lived in a forest"
fact "a camel lived in a desert";  fact "a snake lived in a desert"; fact "a fish lived in a river"
fact "a monkey lived in a jungle"; fact "a tiger lived in a jungle"; fact "a sparrow lived in a tree"
nap
log "########## menagerie complete ##########"
