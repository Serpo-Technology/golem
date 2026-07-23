#!/usr/bin/env zsh
# The weather and the sky: many phenomena across four kinds (precipitation, cloudform,
# wind, skylight), each shown on its own percept, named, placed under its kind and under
# the weather superordinate, then tied together by what a storm brings and what a cloud
# hides. The kinds abstract from the shared naming; the relations weave sky to ground.
# Builds on an existing world.
#
#   GOLEM=golem GAP=1 zsh weather-sky.zsh
set -u
GOLEM=${GOLEM:-golem}
GAP=${GAP:-1}

log()  { print -r -- "$(date +%H:%M:%S) $*" }
obs()  { local i; for i in 1 2 3; do $GOLEM observe vision "$1" >/dev/null 2>&1 && { sleep 0.15; return 0 }; sleep 1; done; return 0 }
fobs() { local i; for i in 1 2 3; do $GOLEM observe --force vision "$1" >/dev/null 2>&1 && { sleep 0.15; return 0 }; sleep 1; done; return 0 }
sayw() { local i; for i in 1 2 3; do $GOLEM tell "$1" >/dev/null 2>&1 && { sleep 0.15; return 0 }; sleep 1; done; return 0 }
saytr(){ local i; for i in 1 2 3; do $GOLEM tell --lang tr "$1" >/dev/null 2>&1 && { sleep 0.15; return 0 }; sleep 1; done; return 0 }
nap()  { log "(nap)"; $GOLEM sleep >/dev/null 2>&1; sleep "${BLOCKGAP:-6}" }

adj=(drifting quiet busy faded gentle sudden hollow crisp woven folded tangled humming
     idle weary swaying rippling glowing ticking distant narrow looming hidden creaking
     flickering whistling murmuring winding fraying gleaming brimming)
nn=(basket lantern ribbon kettle cushion satchel teapot candle mirror blanket sketch
    melody whisper puzzle lattice garland bundle parcel thimble spindle bobbin trinket
    goblet flagon scroll quill inkwell locket pendant charm)
ci=0
comp() { obs "a $adj[$(( (ci % ${#adj[@]}) + 1 ))] $nn[$(( (ci % ${#nn[@]}) + 1 ))]"; ci=$((ci+1)) }

# teach: caption name subkind tr   -- five showings, named at three levels, bilingual.
teach() {
  local cap=$1 name=$2 kind=$3 tr=$4 i
  log "teach: $name ($kind)"
  for i in 1 2 3 4 5; do
    fobs "$cap"; sayw "$name"
    (( i == 2 )) && sayw "$kind"; (( i == 3 )) && sayw "weather"
    (( i == 4 )) && { [ -n "$tr" ] && saytr "$tr" }
    (( i == 5 )) && sayw "$kind"
    comp; sleep $GAP
  done
}
fact() { sayw "$1"; sleep $GAP }

$GOLEM status >/dev/null 2>&1 || { log "daemon unreachable"; exit 1 }
log "########## weather and sky begins ##########"

# --- Precipitation -----------------------------------------------------------
teach "a steady silver rain"       rain      precipitation yağmur
teach "a soft white snow"          snow      precipitation kar
teach "a hard bouncing hail"       hail      precipitation dolu
teach "a cold gray sleet"          sleet     precipitation ""
teach "a fine light drizzle"       drizzle   precipitation ""
teach "a heavy drumming downpour"  downpour  precipitation sağanak
teach "a crisp glittering frost"   frost     precipitation kırağı
nap

# --- Cloud forms -------------------------------------------------------------
teach "a puffy white cloud"        cloud     cloudform bulut
teach "a thick damp fog"           fog       cloudform sis
teach "a pale drifting mist"       mist      cloudform pus
teach "a faint yellow haze"        haze      cloudform ""
teach "a dark raging storm"        storm     cloudform fırtına
nap

# --- Wind --------------------------------------------------------------------
teach "a soft warm breeze"         breeze    wind esinti
teach "a howling fierce gale"      gale      wind ""
teach "a sharp sudden gust"        gust      wind ""
teach "a spinning dusty whirlwind" whirlwind wind hortum
teach "a brief violent squall"     squall    wind ""
nap

# --- Sky light ---------------------------------------------------------------
teach "a bright arching rainbow"   rainbow   skylight gökkuşağı
teach "a rosy glowing sunrise"     sunrise   skylight gündoğumu
teach "a fiery orange sunset"      sunset    skylight günbatımı
teach "a blush pink dawn"          dawn      skylight şafak
teach "a dim purple dusk"          dusk      skylight ""
nap
teach "a deep blue twilight"       twilight  skylight alacakaranlık
teach "a shimmering green aurora"  aurora    skylight ""
teach "a jagged bright lightning"  lightning skylight şimşek
teach "a loud rolling thunder"     thunder   skylight ""
nap

# --- What the sky does -------------------------------------------------------
log "=== sky in motion ==="
fact "a storm brought a rain";     fact "a storm brought a thunder"; fact "a cloud hid a sun"
fact "a cloud brought a rain";     fact "a lightning lit a sky";     fact "a thunder followed a lightning"
fact "a breeze moved a cloud";     fact "a gale bent a tree";        fact "a fog covered a valley"
fact "a rainbow followed a rain";  fact "a sunrise followed a dawn"; fact "a dusk followed a sunset"
nap
log "########## weather and sky complete ##########"
