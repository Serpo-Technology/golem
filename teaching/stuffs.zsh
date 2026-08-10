#!/usr/bin/env zsh
# What grows to be eaten and one thing dug from the sea, shown and named.
#
# The materials belong here by rights and are left out on purpose: this corpus says what
# a thing is made of by putting the stuff in front of it, and a leather in its own right
# would turn every knapsack and coffer already taught into a caption modified by a thing.
#
#   GOLEM=golem GAP=1 zsh stuffs.zsh
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

# teach: caption name kind tr  -- the caption's HEAD is the name, which is what the naming
# pass reads off it. Five showings, named at its own level and at its kind, bilingual.
teach() {
  local cap=$1 name=$2 kind=$3 tr=$4 i
  log "teach: $name ($kind)"
  for i in 1 2 3 4 5; do
    fobs "$cap"; sayw "$name"
    (( i == 2 )) && sayw "$kind"
    
    (( i == 4 )) && { [ -n "$tr" ] && saytr "$tr" }
    (( i == 5 )) && sayw "$kind"
    comp; sleep $GAP
  done
}

$GOLEM status >/dev/null 2>&1 || { log "daemon unreachable"; exit 1 }
log "########## stuffs begins ##########"
teach "a russet capped acorn" acorn nut "meşe palamudu"
teach "a pale egg-shaped almond" almond nut "badem"
teach "a glossy russet chestnut" chestnut nut "kestane"
teach "a round small hazelnut" hazelnut nut "fındık"
teach "a papery tan peanut" peanut nut "yerfıstığı"
teach "a bearded pale barley" barley grain "arpa"
teach "a straw-coloured kerneled corn" corn grain "mısır"
teach "a flaked pale oat" oat grain "yulaf"
teach "a dusky slender rye" rye grain "çavdar"
teach "a golden ripe wheat" wheat grain "buğday"
teach "a round pallid pearl" pearl gem "inci"

nap
log "########## stuffs complete ##########"
