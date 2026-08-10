#!/usr/bin/env zsh
# Food and drink shown on the plate and in the glass, each named and placed
# under the dish or drink it is.
#
#   GOLEM=golem GAP=1 zsh dishes-and-drinks.zsh
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
log "########## dishes-and-drinks begins ##########"
teach "a warm crusty bread" bread dish "ekmek"
teach "a iced layered cake" cake dish "pasta"
teach "a steamed folded dumpling" dumpling dish "mantı"
teach "a folded straw-coloured omelette" omelette dish ""
teach "a flat golden pancake" pancake dish ""
teach "a twisted parched pasta" pasta dish "makarna"
teach "a round baked pie" pie dish ""
teach "a round sliced pizza" pizza dish ""
teach "a thick warm porridge" porridge dish ""
teach "a pallid steamed rice" rice dish "pilav"
teach "a leafy tossed salad" salad dish "salata"
teach "a stacked cut sandwich" sandwich dish ""
teach "a scalding clear soup" soup dish "çorba"
teach "a thick russet stew" stew dish "güveç"
teach "a foaming amber beer" beer drink "bira"
teach "a scalding dusky cocoa" cocoa drink ""
teach "a sooty steaming coffee" coffee drink "kahve"
teach "a chilled amber juice" juice drink ""
teach "a cloudy chilled lemonade" lemonade drink ""
teach "a pallid chilled milk" milk drink "süt"
teach "a fizzy sweet soda" soda drink ""
teach "a scalding amber tea" tea drink "çay"
teach "a clear chilled water" water drink "su"
teach "a dusky scarlet wine" wine drink "şarap"

nap
log "########## dishes-and-drinks complete ##########"
