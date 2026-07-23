#!/usr/bin/env zsh
# The made world of people: places they gather, the arts, games and sport, and the
# abstract things they trade and live by. Taught by telling, each placed under its
# kind and tied into the web of professions, tools, and countries already known.
#
#   GOLEM=golem GAP=1 zsh society.zsh
set -u
GOLEM=${GOLEM:-golem}
GAP=${GAP:-1}

log()  { print -r -- "$(date +%H:%M:%S) $*" }
sayw() { local i; for i in 1 2 3; do $GOLEM tell "$1" >/dev/null 2>&1 && { sleep 0.15; return 0 }; sleep 1; done; return 0 }
saytr(){ local i; for i in 1 2 3; do $GOLEM tell --lang tr "$1" >/dev/null 2>&1 && { sleep 0.15; return 0 }; sleep 1; done; return 0 }
fact() { sayw "$1"; sleep $GAP }
nap()  { log "(nap)"; $GOLEM sleep >/dev/null 2>&1; sleep "${BLOCKGAP:-6}" }
kindof() { local n=$1 k=$2; fact "a $n is a $k"; fact "a $n is a $k"; }

$GOLEM status >/dev/null 2>&1 || { log "daemon unreachable"; exit 1 }
log "########## society begins ##########"

# --- Places people gather ----------------------------------------------------
log "=== places ==="
for p in school hospital market bank court library museum theater park station church factory; do
  kindof "$p" "place"
done
nap
fact "a teacher worked in a school"; fact "a doctor worked in a hospital"; fact "a judge worked in a court"
fact "a person bought in a market";  fact "a book sat in a library";      fact "a train stopped at a station"
fact "a city has a market";          fact "a town has a school";          fact "a village has a church"
saytr "okul bir yerdir"; saytr "hastane bir yerdir"; saytr "pazar bir yerdir"
nap

# --- The arts ----------------------------------------------------------------
log "=== arts ==="
for a in music dance painting story poem song play film sculpture drawing; do
  kindof "$a" "art"
done
nap
fact "a singer made a music"; fact "a writer made a story"; fact "a painter made a painting"
fact "a poet wrote a poem";   fact "a dancer performed a dance"; fact "a song has a music"
fact "a story has a word";    fact "a music is beautiful";       fact "a poem has a line"
saytr "müzik bir sanattır"; saytr "hikaye bir sanattır"; saytr "resim bir sanattır"
nap

# --- Games and sport ---------------------------------------------------------
log "=== games ==="
for g in game sport race chess football tennis swimming running boxing cycling; do
  kindof "$g" "activity"
done
nap
fact "a player played a game";  fact "a runner won a race"; fact "a team played a football"
fact "a chess needs a board";   fact "a tennis needs a ball"; fact "a sport needs a skill"
fact "a race has a winner";     fact "a football has a goal"; fact "a swimmer swam in a pool"
saytr "oyun bir etkinliktir"; saytr "yarış bir etkinliktir"; saytr "futbol bir etkinliktir"
nap

# --- Abstract things people live by ------------------------------------------
log "=== abstract ==="
for x in money law peace war power freedom justice knowledge truth wealth trade wisdom; do
  kindof "$x" "idea"
done
nap
fact "a money bought a good";      fact "a law kept a peace";   fact "a war broke a peace"
fact "a king held a power";        fact "a trade moved a money"; fact "a knowledge came from a study"
fact "a peace is better than a war"; fact "a freedom is precious"; fact "a justice needs a law"
fact "a merchant made a wealth";   fact "a scholar sought a knowledge"; fact "a truth beats a lie"
saytr "para bir fikirdir"; saytr "barış bir fikirdir"; saytr "adalet bir yasaya ihtiyaç duyar"
nap

# --- Language ----------------------------------------------------------------
log "=== language ==="
for w in word letter sentence question answer name number book page story language; do
  kindof "$w" "sign"
done
nap
fact "a sentence has a word"; fact "a word has a letter"; fact "a book has a page"
fact "a question needs an answer"; fact "a name names a thing"; fact "a page has a line"
saytr "kelime bir işarettir"; saytr "harf bir işarettir"; saytr "kitap bir sayfaya sahiptir"
nap
log "########## society complete ##########"
