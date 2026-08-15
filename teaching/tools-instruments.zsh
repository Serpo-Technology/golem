#!/usr/bin/env zsh
# A wide workshop of tools and instruments across three kinds (handtool, powertool,
# instrument), each shown on its own percept, named, placed under its kind and under
# the tool superordinate, and tied into the world by who wields it. The kinds abstract
# from the shared naming; the use relations weave the tools to the people who work
# them. Builds on an existing world.
#
#   GOLEM=golem GAP=1 zsh tools-instruments.zsh
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
    (( i == 2 )) && sayw "$kind"; (( i == 3 )) && { sayw "tool"; sayw "object" }
    (( i == 4 )) && { [ -n "$tr" ] && saytr "$tr" }
    (( i == 5 )) && sayw "$kind"
    comp; sleep $GAP
  done
}
fact() { sayw "$1"; sleep $GAP }

$GOLEM status >/dev/null 2>&1 || { log "daemon unreachable"; exit 1 }
log "########## tools and instruments begin ##########"

# --- Hand tools --------------------------------------------------------------
teach "a heavy steel hammer"    hammer      handtool çekiç
teach "a sharp toothed saw"     saw         handtool testere
teach "a shiny adjustable wrench" wrench    handtool ""
teach "a yellow handled screwdriver" screwdriver handtool tornavida
teach "a small gripping pliers" pliers      handtool pense
teach "a sharp flat chisel"     chisel      handtool keski
teach "a broad iron axe"        axe         handtool balta
teach "a wide digging shovel"   shovel      handtool kürek
teach "a long metal rake"       rake        handtool tırmık
teach "a curved garden hoe"     hoe         handtool çapa
nap
teach "a rough metal file"      file        handtool eğe
teach "a round wooden mallet"   mallet      handtool tokmak
teach "a tight metal clamp"     clamp       handtool ""
teach "a bent iron crowbar"     crowbar     handtool ""
nap

# --- Power tools -------------------------------------------------------------
teach "a black cordless drill"  drill       powertool matkap
teach "an orange electric sander" sander    powertool ""
teach "a loud angle grinder"    grinder     powertool ""
teach "a roaring red chainsaw"  chainsaw    powertool ""
teach "a heavy pounding jackhammer" jackhammer powertool ""
teach "a blue cutting jigsaw"   jigsaw      powertool ""
nap

# --- Measuring and viewing instruments ---------------------------------------
teach "a long brass telescope"  telescope   device teleskop
teach "a silver lab microscope" microscope  device mikroskop
teach "a small brass compass"   compass     device pusula
teach "a straight wooden ruler" ruler       device cetvel
teach "a shiny weighing scale"  scale       device terazi
teach "a thin glass thermometer" thermometer device termometre
teach "a round wall barometer"  barometer   device barometre
teach "a round handheld magnifier" magnifier device büyüteç
teach "a clear half-circle protractor" protractor device iletki
teach "a round pressure gauge"  gauge       device ""
nap

# --- Use: people wield tools -------------------------------------------------
log "=== use ==="
fact "a carpenter used a hammer";   fact "a scientist used a microscope"; fact "a builder used a drill"
fact "a gardener used a rake";      fact "a plumber used a wrench";       fact "a mechanic used a screwdriver"
fact "a sailor used a compass";     fact "an astronomer used a telescope"; fact "a farmer used a hoe"
fact "a woodcutter used an axe";    fact "a doctor used a thermometer";   fact "a cook used a scale"
nap
log "########## tools and instruments complete ##########"
