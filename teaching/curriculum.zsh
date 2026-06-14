#!/usr/bin/env zsh
# GOLEM teaching curriculum: one continuous ostensive session that grows a
# grounded vocabulary, three categories, an adjective, two Turkish words, a
# temporal sequence, a superordinate, and a handful of relations. Merged from
# the day-three through day-six lessons and augmented with relation narration
# for the comprehension engine.
#
# Run it against a reachable daemon, in the container or out:
#   GOLEM=golem GAP=15 zsh curriculum.zsh
#
# GOLEM   the CLI binary (default: golem on PATH)
# GAP     seconds between scenes, the calibration spacing (default 15)
#
# The protocol: a scene observes one vision percept, names it, then draws two
# unique combinatorial companion percepts so no scene-setter ever recurs often
# enough to mint a junk hub, then waits GAP. Naps consolidate. The primitives
# retry and always return 0, so a call that lands mid-consolidation never
# aborts the lesson.
set -u

GOLEM=${GOLEM:-golem}
GAP=${GAP:-15}

log() { print -r -- "$(date +%H:%M:%S) $*" }

obs() {
  local i
  for i in 1 2 3; do
    $GOLEM observe vision "$1" >/dev/null 2>&1 && { sleep 1; return 0 }
    log "  retry observe ($i): $1"; sleep 2
  done
  log "  WARN observe failed 3x: $1"; sleep 1; return 0
}
sayw() {
  local i
  for i in 1 2 3; do
    $GOLEM tell "$1" >/dev/null 2>&1 && { sleep 1; return 0 }
    log "  retry tell ($i): $1"; sleep 2
  done
  log "  WARN tell failed 3x: $1"; sleep 1; return 0
}
saytr() {
  local i
  for i in 1 2 3; do
    $GOLEM tell --lang tr "$1" >/dev/null 2>&1 && { sleep 1; return 0 }
    log "  retry tell-tr ($i): $1"; sleep 2
  done
  log "  WARN tell-tr failed 3x: $1"; sleep 1; return 0
}
nap() { log "(nap: consolidate)"; $GOLEM sleep >/dev/null 2>&1 || log "  WARN sleep failed"; sleep 2 }

# Companion captions are generated combinatorially (43 adjectives x 47 nouns,
# both prime, so no pair repeats for 2021 draws) and drawn sequentially, so a
# long session never repeats a companion. None is a taught token, a Turkish
# form, or a word the curriculum teaches. companion() sets CMP and advances ci
# in the caller's shell; call it bare, never as $(companion).
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
CMP=""
companion() {
  local na=${#adj[@]} nb=${#nn[@]}
  CMP="a $adj[$(( (ci % na) + 1 ))] $nn[$(( (ci % nb) + 1 ))]"
  ci=$((ci+1))
}

scene()       { log "scene: $2"; obs "$1"; sayw "$2"; companion; obs "$CMP"; companion; obs "$CMP"; sleep $GAP }
scene_named() { log "scene: $2 + $3"; obs "$1"; sayw "$2"; sayw "$3"; companion; obs "$CMP"; companion; obs "$CMP"; sleep $GAP }
scene_named3(){ log "scene: $2 + $3 + $4"; obs "$1"; sayw "$2"; sayw "$3"; sayw "$4"; companion; obs "$CMP"; companion; obs "$CMP"; sleep $GAP }
scene_tr()    { log "scene(tr): $2"; obs "$1"; saytr "$2"; companion; obs "$CMP"; companion; obs "$CMP"; sleep $GAP }

$GOLEM status >/dev/null 2>&1 || { log "daemon unreachable at $GOLEM"; exit 1 }
log "########## curriculum begins ##########"

# --- Vocabulary: five everyday objects -------------------------------------
log "=== vocab: cat bottle ribbon lantern spoon ==="
v1() { case $1 in
  C) scene "a small black cat" "cat";;   B) scene "a green bottle" "bottle";;
  R) scene "a green ribbon" "ribbon";;   L) scene "a brass lantern" "lantern";;
  S) scene "a wooden spoon" "spoon";; esac }
for x in C B R L S R S C B L L C S R B; do v1 $x; done; nap
for x in B L R C S S R B L C C L B S R; do v1 $x; done; nap

# --- Category: animal over dog mouse rabbit cat ----------------------------
log "=== animal over dog mouse rabbit cat ==="
a() { case $1 in
  D) scene_named "a small brown dog" "dog" "animal";;
  M) scene_named "a small gray mouse" "mouse" "animal";;
  R) scene_named "a gray rabbit" "rabbit" "animal";;
  C) scene_named "a small black cat" "cat" "animal";; esac }
for x in D M R C  M C D R  R D C M  C R M D; do a $x; done; nap

# --- Adjective: green by contrast across carriers --------------------------
log "=== green by contrast: leaf bottle ribbon ==="
for cap in "a green leaf" "a green bottle" "a green ribbon" \
           "a green ribbon" "a green leaf" "a green bottle"; do
  scene "$cap" "green"
done; nap

# --- Turkish: kedi sise kurdele converge on the same concepts --------------
log "=== Turkish: kedi şişe kurdele ==="
t() { case $1 in
  K) scene_tr "a small black cat" "kedi";;
  S) scene_tr "a green bottle" "şişe";;
  R) scene_tr "a green ribbon" "kurdele";; esac }
for x in K S R  S R K  R K S  K R S; do t $x; done; nap

# --- Metals: vocabulary then the category ----------------------------------
log "=== review animals, ground key lantern, mint coin nail knife ==="
scene_named "a small brown dog"  "dog"    "animal"
scene_named "a small gray mouse" "mouse"  "animal"
scene_named "a gray rabbit"      "rabbit" "animal"
scene_named "a small black cat"  "cat"    "animal"
scene "a brass key"     "key"
scene "a brass lantern" "lantern"
mv() { case $1 in
  C) scene "a round copper coin" "coin";;
  N) scene "a small iron nail"   "nail";;
  K) scene "a small steel knife" "knife";; esac }
for x in C N K N C K N C K C N K N K C K N C; do mv $x; done; nap

log "=== metal over key lantern coin nail ==="
m() { case $1 in
  K) scene_named "a brass key"         "key"     "metal";;
  L) scene_named "a brass lantern"     "lantern" "metal";;
  C) scene_named "a round copper coin" "coin"    "metal";;
  N) scene_named "a small iron nail"   "nail"    "metal";; esac }
for x in K L C N L N K C K C N L N C L K; do m $x; done; nap

# --- Plants: vocabulary then the category ----------------------------------
log "=== plant vocab: flower tree grass branch stone ==="
pv() { case $1 in
  F) scene "a tall yellow flower" "flower";;
  T) scene "a tall green tree"    "tree";;
  G) scene "a tall white grass"   "grass";;
  B) scene "a tall brown branch"  "branch";;
  S) scene "a tall gray stone"    "stone";; esac }
for x in F T B S G  T G S F B  G B F T S  B S G F T  S F T B G  T G B S F; do pv $x; done
log "=== Turkish: çiçek ağaç converge on flower tree ==="
for x in çiçek ağaç çiçek ağaç çiçek ağaç; do
  case $x in
  çiçek) scene_tr "a tall yellow flower" "çiçek";;
  ağaç)  scene_tr "a tall green tree"    "ağaç";; esac
done; nap

log "=== plant over flower tree grass branch ==="
p() { case $1 in
  F) scene_named "a tall yellow flower" "flower" "plant";;
  T) scene_named "a tall green tree"    "tree"   "plant";;
  G) scene_named "a tall white grass"   "grass"  "plant";;
  B) scene_named "a tall brown branch"  "branch" "plant";; esac }
for x in F T G B  G B F T  B F T G  T G B F; do p $x; done; nap

# --- Sequence: the door opens, then the bell rings -------------------------
log "=== sequence: the door opens -> the bell rings ==="
seq_round() {
  log "seq: door -> bell"
  obs "the door opens"; sleep 3; obs "the bell rings"
  companion; obs "$CMP"; companion; obs "$CMP"; sleep $GAP
}
seq_round; seq_round; nap

# --- Superordinate: thing over dog(animal) key(metal) tree(plant) ----------
log "=== thing over dog key tree, naming all three levels at once ==="
th() { case $1 in
  D) scene_named3 "a small brown dog" "dog"  "animal" "thing";;
  K) scene_named3 "a brass key"       "key"  "metal"  "thing";;
  T) scene_named3 "a tall green tree" "tree" "plant"  "thing";; esac }
for x in D K T  K T D  T D K  D K T  K T D; do th $x; done; nap

# --- Relations: narrate chasings so the chase relation grounds -------------
# Augmentation for the comprehension engine: a transitive utterance parses
# into chaser and chased and is written as a proposition the graph can answer.
log "=== relations: narrate chasings (dog/cat/mouse/rabbit already grounded) ==="
for s in "a dog chased a cat"    "a cat chased a mouse"  "a dog chased a rabbit" \
         "a cat chased a mouse"  "a dog chased a cat"    "a mouse chased a cat" \
         "a dog chased a cat"    "a cat chased a mouse"; do
  log "narrate: $s"; sayw "$s"; sleep $GAP
done; nap

log "########## curriculum complete ##########"
