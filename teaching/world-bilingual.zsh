#!/usr/bin/env zsh
# A coherent world grown bilingual from genesis: every animal and object is
# named in English AND Turkish on ONE percept in its own block, so each concept
# crystallizes with both names on a single node (the bilingual-clean recipe at
# scale, on the cross-tongue-binding engine). The animal category is named in
# both tongues, two gradable scales are unified across tongues, and the chase and
# fetch relations bind kovala to chase and getir to fetch. Grown from an EMPTY
# brain so the provenance is clean, no English layer to retrofit Turkish onto.
#
#   GOLEM=/tmp/golem-native GAP=2 zsh world-bilingual.zsh
#
# Run against a fresh empty daemon (senses off, gate open for teaching). The
# protocol: a scene observes one percept, names it in both tongues while it is
# the warm concept, draws one unique combinatorial companion, waits GAP; a noun
# is taught in its own block with a nap between, so its names stay on that one
# concept rather than drifting. Categories abstract from a second naming pass;
# the scales and relations are warmed in English then told in Turkish back to
# back so the Turkish form joins the same relation.
set -u

GOLEM=${GOLEM:-golem}
GAP=${GAP:-2}
log() { print -r -- "$(date +%H:%M:%S) $*" }

# The taught percept (fobs) is shown with attention directed at it, so a repeated showing
# is never predicted away and it accrues the showings a concept needs at tight pacing;
# companions and warmup throwaways stay ambient (obs) so habituation absorbs them without
# minting. The deliberate sleep in nap() forms what is ready regardless of how depleted
# sleep pressure is, so no recovery rest is needed.
obs()   { local i; for i in 1 2 3; do $GOLEM observe vision "$1" >/dev/null 2>&1 && { sleep 0.2; return 0 }; sleep 1; done; return 0 }
fobs()  { local i; for i in 1 2 3; do $GOLEM observe --force vision "$1" >/dev/null 2>&1 && { sleep 0.2; return 0 }; sleep 1; done; return 0 }
sayw()  { local i; for i in 1 2 3; do $GOLEM tell "$1" >/dev/null 2>&1 && { sleep 0.2; return 0 }; sleep 1; done; return 0 }
saytr() { local i; for i in 1 2 3; do $GOLEM tell --lang tr "$1" >/dev/null 2>&1 && { sleep 0.2; return 0 }; sleep 1; done; return 0 }
# Consolidate, then idle past the naming-recency window before the next block, so
# this block's percept ages out and the next block's name does not keep company
# with it.
nap()   { log "(nap: consolidate)"; $GOLEM sleep >/dev/null 2>&1; sleep "${BLOCKGAP:-8}" }

# Combinatorial companions (43 x 47, both prime, both indices step together so no
# pair repeats for 2021 draws): a varied scene-setter drawn AFTER each naming, so
# it flushes the warm percept before the next showing without entering the name's
# pre-tell company. It never recurs into a junk hub, and is no taught token.
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

# Bilingual block: one percept named in both tongues, then two fresh companions
# flush the scene so the name's company stays the one percept it was told over.
# Six showings; the block-gap nap isolates it from its neighbours.
bi() {
  local cap=$1 en=$2 tr=$3 i
  log "block: $en / $tr -> $cap"
  for i in 1 2 3 4 5 6; do fobs "$cap"; sayw "$en"; saytr "$tr"; companion; companion; sleep $GAP; done
  nap
}
narrate() { log "narrate: $1"; sayw "$1"; sleep $GAP }
narrate_tr() { log "narrate(tr): $1"; saytr "$1"; sleep $GAP }

$GOLEM status >/dev/null 2>&1 || { log "daemon unreachable at $GOLEM"; exit 1 }
log "########## bilingual world begins ##########"

# --- Warmup: prime the fresh predictor with varied throwaways ---------------
# Each shape is shown once, so none crystallizes into a concept that could later
# keep company with a name; they only settle the predictor's first-block lock.
log "=== warmup ==="
for s in "a plain wooden crate" "a dim stone arch" "a faint metal rail" "a worn cloth sheet" \
         "a pale glass pane" "a low brick ledge"; do obs "$s"; sleep 1; done; nap

# --- Animals, each bilingual on its percept in its own block -----------------
log "=== animals (bilingual) ==="
bi "a small brown dog"   dog    köpek
bi "a small black cat"   cat    kedi
bi "a small gray mouse"  mouse  fare
bi "a gray rabbit"       rabbit tavşan
bi "a gray wolf"         wolf   kurt
bi "a small blue jay"   jay   alakarga
bi "a silver sardine"       sardine   sardalya
bi "a large brown horse" horse  at
bi "a tiny gray snail"   snail  salyangoz
bi "a green frog"        frog   kurbağa
bi "a large black bear"  bear   ayı
bi "a golden lion"       lion   aslan
bi "a brown owl"         owl    baykuş
bi "a spotted deer"      deer   geyik

# --- Objects, bilingual ------------------------------------------------------
log "=== objects (bilingual) ==="
bi "a round red bead"    bead   boncuk
bi "a square wooden case" case    sandık
bi "a white cup"         cup    fincan
bi "a thick blue book"   book   kitap
bi "a round clay plate"  plate  tabak

# --- The animal category, named in both tongues over the members -------------
# A second naming pass: each animal percept named "animal" and "hayvan", rotated
# so the category gathers a spread core and abstracts (in each tongue), while the
# basic names taught above stay concentrated on their percepts.
log "=== animal / hayvan over the cast ==="
cat_scene() { fobs "$1"; sayw animal; saytr hayvan; companion; companion; sleep $GAP }
catset=(
  "a small brown dog" "a small gray mouse" "a gray rabbit" "a small black cat"
  "a gray wolf" "a small blue jay" "a silver sardine" "a large brown horse"
  "a tiny gray snail" "a green frog" "a large black bear" "a golden lion"
  "a brown owl" "a spotted deer"
)
for round in 1 2 3; do
  for c in $catset; do cat_scene "$c"; done
done
nap

# --- Size scale, unified across tongues --------------------------------------
# The English chain gives "bigger" transitively; one "smaller" telling discovers
# the converse. Each English comparative is followed back to back by its Turkish
# form so "daha büyük" joins the same relation as "bigger".
log "=== size: lion > horse > dog > cat > mouse > snail (bilingual) ==="
narrate "a lion is bigger than a horse"; narrate_tr "aslan attan daha büyük"
narrate "a horse is bigger than a dog";  narrate_tr "at köpekten daha büyük"
narrate "a dog is bigger than a cat";    narrate_tr "köpek kediden daha büyük"
narrate "a cat is bigger than a mouse";  narrate_tr "kedi fareden daha büyük"
narrate "a mouse is bigger than a snail"; narrate_tr "fare salyangozdan daha büyük"
narrate "a snail is smaller than a mouse"   # discovers smaller <-> bigger
nap

# --- Speed scale -------------------------------------------------------------
log "=== speed: horse > dog > snail ==="
narrate "a horse is faster than a dog"
narrate "a dog is faster than a snail"
narrate "a dog is slower than a horse"      # discovers slower <-> faster
nap

# --- Relations, unified across tongues (warm English, tell Turkish, no nap) ---
log "=== chase<->kovala, fetch<->getir ==="
narrate "a dog chased a cat"; saytr "köpek kediyi kovaladı"
narrate "a cat chased a mouse"; saytr "kedi fareyi kovaladı"
narrate "a dog chased a rabbit"
narrate "a mouse chased a cat"
narrate "a wolf chased a rabbit"
narrate "a cat chased a bird"
narrate "a bear chased a fish"
narrate "a dog fetched a ball"; saytr "köpek topu getirdi"
narrate "a bird fetched a ribbon"
nap

log "########## bilingual world complete ##########"
