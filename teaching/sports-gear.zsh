#!/usr/bin/env zsh
# A locker of sporting equipment: many items across three kinds (ball, stick, gear),
# each shown on its own percept, named, placed under its kind and under the equipment
# superordinate, and tied into the world by how a player uses it. The kinds abstract
# from the shared naming; the play and wear relations weave the items to the athletes.
# Builds on an existing world.
#
#   GOLEM=golem GAP=1 zsh sports-gear.zsh
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
    (( i == 2 )) && sayw "$kind"; (( i == 3 )) && sayw "equipment"
    (( i == 4 )) && { [ -n "$tr" ] && saytr "$tr" }
    (( i == 5 )) && sayw "$kind"
    comp; sleep $GAP
  done
}
fact() { sayw "$1"; sleep $GAP }

$GOLEM status >/dev/null 2>&1 || { log "daemon unreachable"; exit 1 }
log "########## sports-gear begins ##########"

# --- Balls -------------------------------------------------------------------
teach "a muddy leather football"   football   ball ""
teach "a fuzzy yellow tennisball"  tennisball ball ""
teach "a swirled glass marble"     marble     ball misket
teach "a frozen rubber puck"       puck       ball ""
teach "a stitched white baseball"  baseball   ball ""
teach "a pebbled orange basketball" basketball ball ""
teach "a bright neon softball"     softball   ball ""
nap

# --- Sticks ------------------------------------------------------------------
teach "a slender steel javelin"    javelin stick cirit
teach "a strung graphite racket"   racket  stick raket
teach "a flat wooden paddle"       paddle  stick ""
teach "a polished tapered cue"     cue     stick isteka
teach "a heavy rubber mallet"      mallet  stick tokmak
teach "a curved yew bow"           bow     stick yay
teach "a chrome golf club"         club    stick ""
teach "a springy vaulting pole"    pole    stick sırık
teach "a long ashen oar"           oar     stick kürek
nap

# --- Gear --------------------------------------------------------------------
teach "a scratched racing helmet"  helmet    gear kask
teach "a padded batting glove"     glove     gear eldiven
teach "a fogged swimming goggles"  goggles   gear ""
teach "a bladed silver skate"      skate     gear paten
teach "a waxed downhill ski"       ski       gear kayak
teach "a decaled fiberglass snowboard" snowboard gear ""
nap
teach "a waxed foam surfboard"     surfboard gear ""
teach "a molded plastic shinguard" shinguard gear ""
teach "a striped mesh jersey"      jersey    gear forma
teach "a studded turf cleat"       cleat     gear krampon
teach "a webbed climbing harness"  harness   gear ""
teach "a tinted curved visor"      visor     gear ""
nap

# --- Use: players play, athletes wear ----------------------------------------
log "=== use: play and wear ==="
fact "a player kicked a football";  fact "a pitcher threw a baseball"; fact "a player bounced a basketball"
fact "a child rolled a marble";     fact "a golfer swung a club";      fact "an archer drew a bow"
fact "a rower pulled an oar";       fact "a thrower hurled a javelin"; fact "a skier wore a ski"
fact "a rider wore a helmet";       fact "a goalie wore a glove";      fact "a skater laced a skate"
nap
log "########## sports-gear complete ##########"
