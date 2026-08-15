#!/usr/bin/env zsh
# The stuff things are made of: many substances across four kinds (metal, stone, mineral,
# fuel), each shown on its own percept, named, placed under its kind and under the
# material superordinate, then tied to the work that wins and moulds them. The kinds
# abstract from the shared naming; the relations bind substance to labor. Builds on an
# existing world.
#
#   GOLEM=golem GAP=1 zsh materials-minerals.zsh
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
    (( i == 2 )) && sayw "$kind"; (( i == 3 )) && sayw "material"
    (( i == 4 )) && { [ -n "$tr" ] && saytr "$tr" }
    (( i == 5 )) && sayw "$kind"
    comp; sleep $GAP
  done
}
fact() { sayw "$1"; sleep $GAP }

$GOLEM status >/dev/null 2>&1 || { log "daemon unreachable"; exit 1 }
log "########## materials and minerals begins ##########"

# --- Metals ------------------------------------------------------------------
teach "a shiny yellow gold"        gold      metal altın
teach "a bright polished silver"   silver    metal gümüş
teach "a dull gray iron"           iron      metal demir
teach "a reddish brown copper"     copper    metal bakır
teach "a warm brown bronze"        bronze    metal bronz
teach "a hard blue steel"          steel     metal çelik
teach "a soft pale tin"            tin       metal kalay
teach "a heavy dark lead"          lead      metal kurşun
nap

# --- Stones ------------------------------------------------------------------
teach "a speckled gray granite"    granite   stone granit
teach "a veined white marble"      marble    stone mermer
teach "a pale layered limestone"   limestone stone kireçtaşı
teach "a flat dark slate"          slate     stone ""
teach "a rough tan sandstone"      sandstone stone kumtaşı
teach "a dark porous basalt"       basalt    stone bazalt
teach "a sharp gray flint"         flint     stone çakmaktaşı
teach "a soft white chalk"         chalk     stone tebeşir
nap

# --- Minerals ----------------------------------------------------------------
teach "a clear shiny quartz"       quartz    mineral kuvars
# The gems are said to be gems here, where they are shown, and not only in the
# lesson that talks about them. A category word never uttered over a thing earns no
# pointing, so it binds to whatever it was near: gem named a ship for a whole grow.
teach "a sparkling clear diamond"  diamond   gem     elmas
teach "a deep red ruby"            ruby      gem     yakut
teach "a rich green emerald"       emerald   gem     zümrüt
teach "a deep blue sapphire"       sapphire  gem     safir

# A gem is a mineral as well, and a word earns its membership by being said over the thing
# rather than beside it. The four above are said to be gems where they are shown; these two
# rounds say mineral over the same four, so neither reading is bought at the other's expense.
for g in "a sparkling clear diamond" "a deep red ruby" "a rich green emerald" "a deep blue sapphire"; do
  fobs "$g"; sayw "mineral"; comp; sleep $GAP
  fobs "$g"; sayw "mineral"; comp; sleep $GAP
done
teach "a fine white salt"          salt      mineral tuz
teach "a damp brown clay"          clay      mineral kil
teach "a fine yellow sand"         sand      mineral kum
nap

# --- Fuels -------------------------------------------------------------------
teach "a black lumpy coal"         coal      fuel kömür
teach "a dark crumbly peat"        peat      fuel turba
teach "a black brittle charcoal"   charcoal  fuel ""
teach "a bright yellow sulfur"     sulfur    fuel kükürt
nap

# --- Work on materials -------------------------------------------------------
log "=== labor ==="
fact "a smith forged an iron";     fact "a smith forged a steel";  fact "a miner dug a coal"
fact "a miner dug a gold";         fact "a jeweler cut a diamond"; fact "a jeweler set a ruby"
fact "a mason cut a granite";      fact "a mason laid a marble";   fact "a fire burned a coal"
fact "a smith moulded a bronze";    fact "a miner found a silver";  fact "a potter used a clay"
nap
log "########## materials and minerals complete ##########"
