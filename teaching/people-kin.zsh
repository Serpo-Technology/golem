#!/usr/bin/env zsh
# A household and village of people, each shown on its own percept, named, and placed
# under a kind (relative, worker, child) and under the person superordinate. The kinds
# abstract from the naming the members share, since no caption ever carries the kind or
# the superordinate word. A closing set of facts ties the people together by what they
# do for one another. Builds on an existing world.
#
#   GOLEM=golem GAP=1 zsh people-kin.zsh
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

# teach shows a captioned percept five times, names it, and places it under its kind
# and under person. The caption never carries the kind or the superordinate word, so
# those abstract from the naming alone. A Turkish name is added when one is given.
teach() {
  local cap=$1 name=$2 kind=$3 tr=$4 i
  log "teach: $name ($kind)"
  for i in 1 2 3 4 5; do
    fobs "$cap"; sayw "$name"
    (( i == 2 )) && sayw "$kind"; (( i == 3 )) && sayw "person"
    (( i == 4 )) && { [ -n "$tr" ] && saytr "$tr" }
    (( i == 5 )) && sayw "$kind"
    comp; sleep $GAP
  done
}
fact() { sayw "$1"; sleep $GAP }

$GOLEM status >/dev/null 2>&1 || { log "daemon unreachable"; exit 1 }
log "########## people begin ##########"

# --- Relatives ---------------------------------------------------------------
teach "a warm smiling mother"      mother      relative anne
teach "a tall calm father"         father      relative baba
teach "a cheerful young sister"    sister      relative ""
teach "a lively young brother"     brother     relative ""
teach "a bright happy daughter"    daughter    relative kız
teach "a sturdy young son"         son         relative oğul
teach "a warm gentle aunt"         aunt        relative teyze
teach "a stout jolly uncle"        uncle       relative amca
teach "a gray gentle grandmother"  grandmother relative büyükanne
nap

# --- Workers -----------------------------------------------------------------
teach "a patient standing teacher" teacher     worker öğretmen
teach "a calm careful doctor"      doctor      worker doktor
teach "a weathered strong farmer"  farmer      worker çiftçi
teach "a plump cheerful baker"     baker       worker fırıncı
teach "a broad muscular smith"     smith       worker demirci
teach "a tall bearded sailor"      sailor      worker denizci
teach "a stern armored soldier"    soldier     worker asker
teach "a proud crowned king"       king        worker kral
teach "a regal crowned queen"      queen       worker kraliçe
teach "a busy round cook"          cook        worker aşçı
nap

# --- Children ----------------------------------------------------------------
teach "a small smiling baby"       baby        child bebek
teach "a cheerful running boy"     boy         child oğlan
teach "a bright skipping girl"     girl        child ""
teach "a tiny sleeping infant"     infant      child ""
teach "a giggling wobbling toddler" toddler    child ""
nap

# --- People act for one another ----------------------------------------------
log "=== relations among people ==="
fact "a mother loved a child";    fact "a father held a baby";     fact "a teacher taught a child"
fact "a sister helped a brother"; fact "a doctor healed a child";  fact "an uncle carried a toddler"
fact "a soldier guarded a king";  fact "a king trusted a soldier"; fact "a grandmother loved a girl"
nap

# --- Turkish threads ---------------------------------------------------------
saytr "anne bir insandır"; saytr "baba bir insandır"; saytr "çocuk bir insandır"
saytr "kral bir insandır"; saytr "doktor bir insandır"
nap
log "########## people complete ##########"
