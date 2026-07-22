#!/usr/bin/env zsh
# A coherent world grown on top of the base curriculum: more animals under the
# animal category, a few objects, two gradable orderings (size and speed) with a
# single reversed telling each so the converse pole is discovered, and a handful
# of actions. Turkish names converge on the same percepts. Run against a daemon
# that already holds the base vocabulary (cat dog mouse rabbit animal, chase):
#
#   GOLEM=/tmp/golem-native GAP=3 zsh world.zsh
#
# The protocol matches curriculum.zsh: a scene observes one percept, names it,
# draws two combinatorial companions so no scene-setter mints a junk hub, then
# waits GAP; naps consolidate; comparisons and actions are narrated once the
# things they relate are grounded. The primitives retry and return 0.
set -u

GOLEM=${GOLEM:-golem}
GAP=${GAP:-3}
log() { print -r -- "$(date +%H:%M:%S) $*" }

obs()   { local i; for i in 1 2 3; do $GOLEM observe vision "$1" >/dev/null 2>&1 && { sleep 1; return 0 }; sleep 2; done; return 0 }
# The taught percept is shown with attention directed at it (full salience) so a repeated
# showing is still learned once habituation would otherwise sink it below consciousness;
# companions stay ambient. A no-op under the quiet room, the fix under always-online.
fobs()  { local i; for i in 1 2 3; do $GOLEM observe --force vision "$1" >/dev/null 2>&1 && { sleep 1; return 0 }; sleep 2; done; return 0 }
sayw()  { local i; for i in 1 2 3; do $GOLEM tell "$1" >/dev/null 2>&1 && { sleep 1; return 0 }; sleep 2; done; return 0 }
saytr() { local i; for i in 1 2 3; do $GOLEM tell --lang tr "$1" >/dev/null 2>&1 && { sleep 1; return 0 }; sleep 2; done; return 0 }
nap()   { log "(nap: consolidate)"; $GOLEM sleep >/dev/null 2>&1; sleep 2 }

# Combinatorial companions (43 x 47, both prime), drawn so no pair repeats.
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
ci=0; CMP=""
companion() { local na=${#adj[@]} nb=${#nn[@]}; CMP="a $adj[$(( (ci % na) + 1 ))] $nn[$(( (ci % nb) + 1 ))]"; ci=$((ci+1)) }

scene()       { log "scene: $2"; fobs "$1"; sayw "$2"; companion; obs "$CMP"; companion; obs "$CMP"; sleep $GAP }
scene_named() { log "scene: $2 + $3"; fobs "$1"; sayw "$2"; sayw "$3"; companion; obs "$CMP"; companion; obs "$CMP"; sleep $GAP }
scene_tr()    { log "scene(tr): $2"; fobs "$1"; saytr "$2"; companion; obs "$CMP"; companion; obs "$CMP"; sleep $GAP }
narrate()     { log "narrate: $1"; sayw "$1"; sleep $GAP }

$GOLEM status >/dev/null 2>&1 || { log "daemon unreachable at $GOLEM"; exit 1 }
log "########## world begins ##########"

# --- More animals, each named and placed under the animal category ---------
log "=== animals: wolf bird fish horse snail frog bear lion owl deer ==="
cap() { case $1 in
  W) print "a gray wolf";;       B) print "a small blue bird";;  F) print "a silver fish";;
  H) print "a large brown horse";; S) print "a tiny gray snail";; G) print "a green frog";;
  E) print "a large black bear";; L) print "a golden lion";;     O) print "a brown owl";;
  D) print "a spotted deer";; esac }
name() { case $1 in
  W) print wolf;; B) print bird;; F) print fish;; H) print horse;; S) print snail;;
  G) print frog;; E) print bear;; L) print lion;; O) print owl;; D) print deer;; esac }
an() { scene_named "$(cap $1)" "$(name $1)" "animal" }
for x in W B F H S  G E L O D  H S W L B  E D F O G  L H E W S; do an $x; done; nap
for x in B O D G F  S W H L E  D F B G O  W L S H E; do an $x; done; nap

# --- A few objects for breadth ---------------------------------------------
log "=== objects: ball box cup book plate ==="
ob() { case $1 in
  A) scene "a round red ball" "ball";;  X) scene "a square wooden box" "box";;
  C) scene "a white cup" "cup";;        K) scene "a thick blue book" "book";;
  P) scene "a round clay plate" "plate";; esac }
for x in A X C K P  X K A P C  C P X A K  K A P C X; do ob $x; done; nap

# --- Turkish names converge on the same animal percepts --------------------
log "=== Turkish: kurt kuş at balık aslan ==="
tr() { case $1 in
  K) scene_tr "a gray wolf" "kurt";;   U) scene_tr "a small blue bird" "kuş";;
  A) scene_tr "a large brown horse" "at";; B) scene_tr "a silver fish" "balık";;
  L) scene_tr "a golden lion" "aslan";; esac }
for x in K U A B L  U B K L A  A L U B K  K B A U L; do tr $x; done; nap

# --- Size: one descending chain, then a single reversed telling ------------
# Each pair is grounded, so the comparison writes a proposition on the gradable
# relation. The chain alone gives "bigger" transitively; the one "smaller"
# telling lets GOLEM discover the two are converse poles of one scale, so a
# "smaller" question then reads the whole chain reversed.
log "=== size: lion > horse > dog > cat > mouse > snail ==="
narrate "a lion is bigger than a horse"
narrate "a horse is bigger than a dog"
narrate "a dog is bigger than a cat"
narrate "a cat is bigger than a mouse"
narrate "a mouse is bigger than a snail"
narrate "a snail is smaller than a mouse"   # the reversed pair: discovers smaller <-> bigger
nap

# --- Speed: a second, independent scale, with its own reversed telling ------
log "=== speed: horse > dog > snail ==="
narrate "a horse is faster than a dog"
narrate "a dog is faster than a snail"
narrate "a dog is slower than a horse"      # discovers slower <-> faster
nap

# --- Base relations: the canonical chasings the reasoning queries rest on --
# Re-narrated here so a brain grown from the relation-free clean snapshot holds
# the same chase facts; on a brain that already has them this only reinforces.
log "=== base relations: dog/cat/mouse/rabbit chasings ==="
for s in "a dog chased a cat" "a cat chased a mouse" "a dog chased a rabbit" "a mouse chased a cat" \
         "a dog chased a cat" "a cat chased a mouse"; do narrate "$s"; done
nap

# --- Actions: chase and fetch over the wider cast --------------------------
log "=== actions: chase, fetch ==="
narrate "a wolf chased a rabbit"
narrate "a cat chased a bird"
narrate "a bear chased a fish"
narrate "a dog fetched a ball"
narrate "a bird fetched a ribbon"
nap

log "########## world complete ##########"
