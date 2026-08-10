#!/usr/bin/env zsh
# People at their work, shown and named, each placed under the person they are.
#
#   GOLEM=golem GAP=1 zsh trades.zsh
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
log "########## trades begins ##########"
teach "a busy young builder" builder person ""
teach "a seated unhurried driver" driver person "şoför"
teach "a weathered patient fisher" fisher person ""
teach "a smiling near friend" friend person "arkadaş"
teach "a quiet watchful hunter" hunter person "avcı"
teach "a robed grave judge" judge person "yargıç"
teach "a dusty stooped miner" miner person ""
teach "a waving nearby neighbor" neighbor person "komşu"
teach "a kind steady nurse" nurse person "hemşire"
teach "a spattered absorbed painter" painter person "ressam"
teach "a uniformed alert pilot" pilot person "pilot"
teach "a open-mouthed loud singer" singer person "şarkıcı"
teach "a bent stitching tailor" tailor person "terzi"
teach "a bent scribbling writer" writer person "yazar"

nap
log "########## trades complete ##########"
