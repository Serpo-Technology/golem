#!/usr/bin/env zsh
# A foods domain grown onto the bilingual world brain: six foods, each named in
# English AND Turkish on one percept in its own block, then a second naming pass
# distils a food / yiyecek category over them. It broadens what the brain can
# talk about with a vocabulary cluster of its own, separate from the animal cast,
# so a second category forms and resemblance can be checked for whether the foods
# cluster apart from the animals. Run against a daemon already holding the world:
#
#   GOLEM=/tmp/golem-native GAP=2 BLOCKGAP=2 zsh world-foods.zsh
#
# The protocol matches world-bilingual.zsh: a scene forces a percept past the
# attention gate, names it in both tongues while it is the warm concept, draws two
# fresh combinatorial companions to flush the scene, and waits GAP; each food is
# its own block with a nap between so its names stay on that one concept. The
# category abstracts from the second naming pass over the food concepts.
set -u

GOLEM=${GOLEM:-golem}
GAP=${GAP:-2}
log() { print -r -- "$(date +%H:%M:%S) $*" }

# The taught percept (fobs) is attention-directed at full salience so a repeated showing is
# still learned; companions and warmup throwaways stay ambient (obs) so habituation absorbs
# them without minting.
obs()   { local i; for i in 1 2 3; do $GOLEM observe vision "$1" >/dev/null 2>&1 && { sleep 0.2; return 0 }; sleep 1; done; return 0 }
fobs()  { local i; for i in 1 2 3; do $GOLEM observe --force vision "$1" >/dev/null 2>&1 && { sleep 0.2; return 0 }; sleep 1; done; return 0 }
sayw()  { local i; for i in 1 2 3; do $GOLEM tell "$1" >/dev/null 2>&1 && { sleep 0.2; return 0 }; sleep 1; done; return 0 }
saytr() { local i; for i in 1 2 3; do $GOLEM tell --lang tr "$1" >/dev/null 2>&1 && { sleep 0.2; return 0 }; sleep 1; done; return 0 }
nap()   { log "(nap: consolidate)"; $GOLEM sleep >/dev/null 2>&1; sleep "${BLOCKGAP:-8}" }

# Combinatorial companions (43 x 47, both prime, indices step together so no pair
# repeats), drawn after each naming to flush the warm percept; never a taught token.
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

# Bilingual block: one percept named in both tongues, two fresh companions flush
# the scene, six showings, the block-gap nap isolating it from its neighbours.
bi() {
  local cap=$1 en=$2 tr=$3 i
  log "block: $en / $tr -> $cap"
  for i in 1 2 3 4 5 6; do fobs "$cap"; sayw "$en"; saytr "$tr"; companion; companion; sleep $GAP; done
  nap
}

$GOLEM status >/dev/null 2>&1 || { log "daemon unreachable at $GOLEM"; exit 1 }
log "########## foods domain begins ##########"

# --- Warmup: settle the predictor with varied throwaways --------------------
log "=== warmup ==="
for s in "a plain wooden crate" "a dim stone arch" "a faint metal rail" "a worn cloth sheet"; do
  obs "$s"; sleep 1
done; nap

# --- Foods, each bilingual on its percept in its own block -------------------
log "=== foods (bilingual) ==="
bi "a red apple"      apple  elma
bi "a yellow banana"  banana muz
bi "an orange carrot" carrot havuç
bi "a green pear"     pear   armut
bi "a white egg"      egg    yumurta
bi "a purple grape"   grape  üzüm

# --- The food category, named in both tongues over the members --------------
# A second naming pass: each food percept named "food" and "yiyecek", rotated so
# the category gathers a spread core and abstracts in each tongue, while the basic
# names above stay concentrated on their percepts.
log "=== food / yiyecek over the foods ==="
food_scene() { fobs "$1"; sayw food; saytr yiyecek; companion; companion; sleep $GAP }
foodset=(
  "a red apple" "a yellow banana" "an orange carrot"
  "a green pear" "a white egg" "a purple grape"
)
for round in 1 2 3; do
  for c in $foodset; do food_scene "$c"; done
done
nap

log "########## foods domain complete ##########"
