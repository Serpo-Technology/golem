#!/usr/bin/env zsh
# A wide consort of musical instruments across four kinds (stringinstrument,
# windinstrument, percussion, keyboard), each shown on its own percept, named, placed
# under its kind and under the instrument superordinate, and tied into the world by who
# plays it and what sound it makes. The kinds abstract from the shared naming; the play
# and sound relations weave the instruments to the musicians who voice them. Builds on
# an existing world.
#
#   GOLEM=golem GAP=1 zsh instruments-music.zsh
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
    (( i == 2 )) && sayw "$kind"; (( i == 3 )) && sayw "instrument"
    (( i == 4 )) && { [ -n "$tr" ] && saytr "$tr" }
    (( i == 5 )) && sayw "$kind"
    comp; sleep $GAP
  done
}
fact() { sayw "$1"; sleep $GAP }

$GOLEM status >/dev/null 2>&1 || { log "daemon unreachable"; exit 1 }
log "########## musical instruments begin ##########"

# --- String instruments ------------------------------------------------------
teach "a polished acoustic guitar" guitar   stringinstrument gitar
teach "a glossy wooden violin"  violin      stringinstrument keman
teach "a large brown cello"     cello       stringinstrument çello
teach "a tall golden harp"      harp        stringinstrument arp
teach "a twangy round banjo"    banjo       stringinstrument ""
teach "a warm reddish viola"    viola       stringinstrument viyola
teach "a small pear-shaped mandolin" mandolin stringinstrument mandolin
teach "a tiny Hawaiian ukulele" ukulele     stringinstrument ""
teach "an old carved lute"      lute        stringinstrument ut
nap

# --- Wind instruments --------------------------------------------------------
teach "a silver slender flute"  flute       windinstrument flüt
teach "a black slim clarinet"   clarinet    windinstrument klarnet
teach "a shiny brass trumpet"   trumpet     windinstrument trompet
teach "a coiled brass horn"     horn        windinstrument korno
teach "a golden curved saxophone" saxophone windinstrument saksofon
teach "a slim wooden oboe"      oboe        windinstrument obua
teach "a sliding brass trombone" trombone   windinstrument trombon
teach "a huge brass tuba"       tuba        windinstrument tuba
nap

# --- Percussion --------------------------------------------------------------
teach "a round wooden drum"     drum        percussion davul
teach "a shiny brass cymbal"    cymbal      percussion zil
teach "a jingling round tambourine" tambourine percussion tef
teach "a colorful wooden xylophone" xylophone percussion ksilofon
teach "a huge hanging gong"     gong        percussion gong
teach "a small steel triangle"  triangle    percussion ""
teach "a speckled shaking maraca" maraca    percussion marakas
nap

# --- Keyboard ----------------------------------------------------------------
teach "a shiny black grand piano" piano     keyboard piyano
teach "a tall wooden pipe organ" organ      keyboard org
teach "an ornate antique harpsichord" harpsichord keyboard klavsen
teach "a red folding accordion" accordion   keyboard akordeon
nap

# --- Play: musicians voice instruments ---------------------------------------
log "=== play ==="
fact "a musician played a guitar";  fact "a musician played a piano";  fact "a violinist played a violin"
fact "a person played a flute";     fact "a musician played a drum";   fact "a musician played a violin"
fact "a drummer played a drum";     fact "a pianist played a piano";   fact "a musician played a harp"
nap

# --- Sound: instruments make music -------------------------------------------
log "=== sound ==="
fact "a drum made a sound";      fact "a trumpet made a sound";  fact "a piano made a sound"
fact "a harp made a sound";      fact "a flute made a sound";    fact "a guitar made a music"
fact "a violin made a music";    fact "a cymbal made a sound";   fact "a band played a music"
nap
log "########## musical instruments complete ##########"
