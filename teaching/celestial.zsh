#!/usr/bin/env zsh
# The night sky in bodies: many objects across four kinds (star, planet, moon,
# smallbody), each shown on its own percept, named by its own name, placed under its kind
# and under the space superordinate, then set in motion by what circles what. The kinds
# abstract from the shared naming; the relations wheel the bodies around one another.
# Builds on an existing world.
#
#   GOLEM=golem GAP=1 zsh celestial.zsh
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
    (( i == 2 )) && sayw "$kind"; (( i == 3 )) && sayw "space"
    (( i == 4 )) && { [ -n "$tr" ] && saytr "$tr" }
    (( i == 5 )) && sayw "$kind"
    comp; sleep $GAP
  done
}
fact() { sayw "$1"; sleep $GAP }

$GOLEM status >/dev/null 2>&1 || { log "daemon unreachable"; exit 1 }
log "########## celestial begins ##########"

# --- Stars -------------------------------------------------------------------
teach "a blazing yellow sun"       sun     star güneş
teach "a brilliant white sirius"   sirius  star ""
teach "a bright blue vega"         vega    star ""
teach "a distant blazing rigel"    rigel   star ""
teach "a steady northern polaris"  polaris star ""
nap

# --- Planets -----------------------------------------------------------------
teach "a swift cratered mercury"   mercury planet merkür
teach "a bright veiled venus"      venus   planet venüs
teach "a rusty red mars"           mars    planet ""
teach "a giant banded jupiter"     jupiter planet jüpiter
teach "a ringed pale saturn"       saturn  planet satürn
teach "a deep blue neptune"        neptune planet neptün
teach "a tilted teal uranus"       uranus  planet uranüs
nap

# --- Moons -------------------------------------------------------------------
teach "a pale round luna"          luna    moon ay
teach "a small lumpy phobos"       phobos  moon ""
teach "a tiny dark deimos"         deimos  moon ""
teach "a dim orange titan"         titan   moon ""
teach "a smooth icy europa"        europa  moon ""
teach "a spotted volcanic io"      io      moon ""
nap

# --- Small bodies ------------------------------------------------------------
teach "a bright tailed comet"      comet     smallbody ""
teach "a rocky tumbling asteroid"  asteroid  smallbody ""
teach "a streaking bright meteor"  meteor    smallbody ""
teach "a charred fallen meteorite" meteorite smallbody ""
teach "a frozen distant pluto"     pluto     smallbody plüton
teach "a round dwarf ceres"        ceres     smallbody ""
nap

# --- What wheels around what -------------------------------------------------
log "=== orbits ==="
fact "a planet circled a sun";     fact "a moon circled a planet";   fact "a comet neared a sun"
fact "an asteroid struck a moon";  fact "a meteor crossed a sky";    fact "a saturn wore a ring"
fact "a jupiter held a moon";      fact "a mars neared an asteroid"; fact "a luna circled a planet"
fact "a pluto orbited a sun";      fact "a phobos circled a mars";   fact "a sun lit a planet"
nap
log "########## celestial complete ##########"
