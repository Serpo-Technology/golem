#!/usr/bin/env zsh
# Abstract vocabulary a mind can never see, taught by telling: emotions, times,
# directions, professions, and qualities. Each enters through the company its
# tellings keep -- a category it belongs to and a relation or two it stands in -- the
# way an unknown verb is minted. None names a percept. Builds on an existing world.
#
#   GOLEM=golem GAP=1 zsh abstractions-plus.zsh
set -u
GOLEM=${GOLEM:-golem}
GAP=${GAP:-1}

log()  { print -r -- "$(date +%H:%M:%S) $*" }
sayw() { local i; for i in 1 2 3; do $GOLEM tell "$1" >/dev/null 2>&1 && { sleep 0.15; return 0 }; sleep 1; done; return 0 }
saytr(){ local i; for i in 1 2 3; do $GOLEM tell --lang tr "$1" >/dev/null 2>&1 && { sleep 0.15; return 0 }; sleep 1; done; return 0 }
fact() { sayw "$1"; sleep $GAP }
nap()  { log "(nap)"; $GOLEM sleep >/dev/null 2>&1; sleep "${BLOCKGAP:-6}" }

$GOLEM status >/dev/null 2>&1 || { log "daemon unreachable"; exit 1 }
log "########## abstractions begin ##########"

# --- Emotions: each a feeling a person feels ---------------------------------
log "=== emotions ==="
emo=(joy fear anger hope grief love pride shame calm worry)
for e in $emo; do
  fact "$e is a feeling"; fact "a person feels $e"
done
nap
fact "joy is good";  fact "grief is bad";   fact "fear is bad";  fact "hope is good"
fact "love is good"; fact "anger is bad";   fact "calm is good"
fact "joy is happier than grief"; fact "hope is stronger than fear"
saytr "sevinç bir duygudur"; saytr "korku bir duygudur"; saytr "öfke bir duygudur"
nap

# --- Times: each a time that passes ------------------------------------------
log "=== time ==="
tim=(hour day week month year morning noon evening night season)
for t in $tim; do
  fact "$t is a time"; fact "a time passes"
done
nap
fact "a day has an hour";   fact "a week has a day";    fact "a month has a week"
fact "a year has a month";  fact "a morning is a time"; fact "a night follows an evening"
fact "a day is longer than an hour"; fact "a year is longer than a month"
saytr "gün bir zamandır"; saytr "yıl bir zamandır"; saytr "gece bir zamandır"
nap

# --- Directions --------------------------------------------------------------
log "=== directions ==="
dir=(north south east west up down left right)
for d in $dir; do fact "$d is a direction"; done
nap
fact "the north opposes the south"; fact "the east opposes the west"
fact "the up opposes the down";     fact "the left opposes the right"
fact "a compass shows a north";     fact "a sun rose in the east"
nap

# --- Professions: each a person with a job -----------------------------------
log "=== professions ==="
pro=(teacher doctor farmer baker builder driver painter singer writer sailor
     cook nurse pilot judge soldier hunter fisher tailor smith miner)
for p in $pro; do
  fact "a $p is a person"; fact "a $p has a job"
done
nap
fact "a teacher taught a student";  fact "a doctor healed a patient"; fact "a farmer grew a crop"
fact "a baker baked a bread";       fact "a builder built a house";  fact "a driver drove a car"
fact "a painter painted a wall";    fact "a singer sang a song";     fact "a writer wrote a book"
fact "a sailor sailed a boat";      fact "a cook cooked a meal";     fact "a pilot flew a plane"
fact "a hunter hunted a deer";      fact "a fisher caught a fish";   fact "a smith forged a blade"
nap
saytr "öğretmen bir insandır"; saytr "doktor bir insandır"; saytr "çiftçi bir insandır"
saytr "bir aşçı bir yemek pişirdi"; saytr "bir çiftçi bir ekin yetiştirdi"
nap

# --- Qualities: gradable adjectives on scales --------------------------------
log "=== qualities ==="
fact "a giant is bigger than a dwarf";   fact "a river is longer than a stream"
fact "a mountain is taller than a hill"; fact "a stone is harder than a clay"
fact "a feather is lighter than a rock"; fact "a fire is hotter than an ice"
fact "a night is darker than a day";     fact "a king is richer than a beggar"
fact "a cheetah is faster than a snail"; fact "an ocean is deeper than a pond"
nap
log "########## abstractions complete ##########"
