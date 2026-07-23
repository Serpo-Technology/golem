#!/usr/bin/env zsh
# Widen an already-bilingual world: two new kinds (plants, foods), a new gradable
# scale (weight), and a cross-kind relation (eat) that ties the animals to the
# foods. Taught onto a warmed brain that already holds the animals, objects, the
# size and speed scales, and chase/fetch, so the new concepts join the same
# graph and the reasoning (is-a, comparison, the eat relation) spans a bigger
# world. Each noun is bilingual on one percept in its own block, the clean recipe
# at scale; categories abstract from a second naming pass; the scale and the
# relation are warmed in English then told in Turkish so each tongue's form joins
# the same edge.
#
#   GOLEM=/tmp/golem-native GAP=2 zsh world-breadth.zsh
#
# Run against a daemon over a copy of the workhorse brain, senses off, gate open.
set -u

GOLEM=${GOLEM:-golem}
GAP=${GAP:-2}
log() { print -r -- "$(date +%H:%M:%S) $*" }

# The taught percept (fobs) is attention-directed at full salience so a repeated showing is
# still learned; companions stay ambient (obs) so habituation absorbs them without minting.
obs()   { local i; for i in 1 2 3; do $GOLEM observe vision "$1" >/dev/null 2>&1 && { sleep 0.2; return 0 }; sleep 1; done; return 0 }
fobs()  { local i; for i in 1 2 3; do $GOLEM observe --force vision "$1" >/dev/null 2>&1 && { sleep 0.2; return 0 }; sleep 1; done; return 0 }
sayw()  { local i; for i in 1 2 3; do $GOLEM tell "$1" >/dev/null 2>&1 && { sleep 0.2; return 0 }; sleep 1; done; return 0 }
saytr() { local i; for i in 1 2 3; do $GOLEM tell --lang tr "$1" >/dev/null 2>&1 && { sleep 0.2; return 0 }; sleep 1; done; return 0 }
nap()   { log "(nap: consolidate)"; $GOLEM sleep >/dev/null 2>&1; sleep "${BLOCKGAP:-8}" }

# Combinatorial companions: a varied scene-setter drawn after each naming to
# flush the warm percept before the next showing, never recurring, never a token.
adj=(drifting distant humming quiet busy faded gentle sudden hollow crisp
     damp dusty woven carved folded tangled looming narrow hidden sloping
     creaking shimmering idle weary jagged swaying flickering rippling glowing ticking
     whistling bustling winding drowsy restless gleaming murmuring fleeting wandering lingering
     brooding teeming sprawling)
nn=(fountain archway corridor tunnel harbor plaza kiosk streetlamp balcony staircase
    courtyard terrace rooftop chimney beacon lighthouse pier canal footpath lane
    ravine summit lagoon marsh hallway alcove rotunda veranda parapet turret
    cloister atrium esplanade boulevard promenade viaduct aqueduct cistern reservoir embankment
    quay wharf jetty breakwater causeway overpass underpass)
ci=0
companion() { local na=${#adj[@]} nb=${#nn[@]}; obs "a $adj[$(( (ci % na) + 1 ))] $nn[$(( (ci % nb) + 1 ))]"; ci=$((ci+1)) }

bi() {
  local cap=$1 en=$2 tr=$3 i
  log "block: $en / $tr -> $cap"
  for i in 1 2 3 4 5 6; do fobs "$cap"; sayw "$en"; saytr "$tr"; companion; companion; sleep $GAP; done
  nap
}
narrate() { log "narrate: $1"; sayw "$1"; sleep $GAP }
narrate_tr() { log "narrate(tr): $1"; saytr "$1"; sleep $GAP }
# A relation in both tongues, told back to back with no pause between the forms.
# The Turkish verb binds to the English relation only while that relation is the
# warm one in the workspace, so the two tellings must be adjacent. A pause risks a
# consolidation pass clearing the workspace, after which the Turkish verb mints a
# parallel relation instead of joining the English one.
bi_rel() { log "rel: $1 / $2"; sayw "$1"; saytr "$2"; sleep $GAP }
# Re-ground an animal already known, shown on its own percept in its own block so
# its name stays the firmest. A relation names no noun, but the cross-situational
# naming recount still reads the company each word kept, so an animal mentioned
# only inside relation clauses (never shown) drifts toward the percepts it was
# warmed beside. Showing it again, blocked and napped like a first teaching, keeps
# its own percept the strongest company its name has.
review() { local cap=$1 en=$2 tr=$3 i; log "review: $en / $tr"; for i in 1 2 3 4; do fobs "$cap"; sayw "$en"; saytr "$tr"; companion; companion; sleep $GAP; done; nap }

$GOLEM status >/dev/null 2>&1 || { log "daemon unreachable at $GOLEM"; exit 1 }
log "########## world breadth begins ##########"

# --- Plants, each bilingual on its percept in its own block ------------------
log "=== plants (bilingual) ==="
bi "a tall green elm"   elm   ağaç
bi "a bright red poppy" poppy gelincik
bi "a green grass tuft"  grass  çimen
bi "a red garden rose"   rose   gül
bi "a green oak leaf"    leaf   yaprak

# --- Foods, bilingual --------------------------------------------------------
log "=== foods (bilingual) ==="
bi "a shiny red apple"    apple  elma
bi "a long orange carrot" carrot havuç
bi "a yellow cheese wedge" cheese peynir
bi "a thick golden honey" honey  bal

# --- The plant category, named in both tongues over the members --------------
log "=== plant / bitki over the cast ==="
plant_scene() { fobs "$1"; sayw plant; saytr bitki; companion; companion; sleep $GAP }
plantset=("a tall green elm" "a bright red poppy" "a green grass tuft" "a red garden rose" "a green oak leaf")
for round in 1 2 3; do for c in $plantset; do plant_scene "$c"; done; done
nap

# --- The food category -------------------------------------------------------
log "=== food / yiyecek over the cast ==="
food_scene() { fobs "$1"; sayw food; saytr yiyecek; companion; companion; sleep $GAP }
foodset=("a shiny red apple" "a long orange carrot" "a yellow cheese wedge" "a thick golden honey")
for round in 1 2 3; do for c in $foodset; do food_scene "$c"; done; done
nap

# --- Weight scale, unified across tongues ------------------------------------
# The English chain gives "heavier" transitively; one "lighter" telling discovers
# the converse. Each comparative is followed by its Turkish form so "daha ağır"
# joins the same relation as "heavier".
log "=== weight: horse > dog > cat > mouse (bilingual) ==="
bi_rel "a horse is heavier than a dog"  "at köpekten daha ağır"
bi_rel "a dog is heavier than a cat"    "köpek kediden daha ağır"
bi_rel "a cat is heavier than a mouse"  "kedi fareden daha ağır"
narrate "a mouse is lighter than a cat"   # discovers lighter <-> heavier
nap

# --- The eat relation, animals to foods, unified across tongues --------------
log "=== eat <-> ye, animals to foods ==="
bi_rel "a rabbit ate a carrot" "tavşan havucu yedi"
bi_rel "a mouse ate a cheese"  "fare peyniri yedi"
bi_rel "a bear ate a honey"    "ayı balı yedi"
narrate "a horse ate a grass"
narrate "a deer ate a leaf"
nap

# --- Re-ground the animal cast --------------------------------------------------
# The weight and eat relations name the animals without showing them, so each
# animal's name kept company with the others it was warmed beside. A grounding pass
# on every animal restores its own percept as the firmest company its name has,
# before the final consolidation reads the naming from that company.
log "=== re-ground the animal cast ==="
review "a small gray mouse"  mouse  fare
review "a small brown dog"   dog    köpek
review "a small black cat"   cat    kedi
review "a large brown horse" horse  at
review "a gray rabbit"       rabbit tavşan
review "a large black bear"  bear   ayı
review "a spotted deer"      deer   geyik
review "a golden lion"       lion   aslan
review "a gray wolf"         wolf   kurt
review "a tiny gray snail"   snail  salyangoz

log "########## world breadth complete ##########"
