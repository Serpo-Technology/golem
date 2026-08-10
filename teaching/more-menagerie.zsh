#!/usr/bin/env zsh
# Creatures the corpus named and never showed: each on its own percept, named,
# placed under its kind and under the animal above it, in both tongues.
#
#   GOLEM=golem GAP=1 zsh more-menagerie.zsh
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
    (( i == 3 )) && sayw "animal"
    (( i == 4 )) && { [ -n "$tr" ] && saytr "$tr" }
    (( i == 5 )) && sayw "$kind"
    comp; sleep $GAP
  done
}

$GOLEM status >/dev/null 2>&1 || { log "daemon unreachable"; exit 1 }
log "########## more-menagerie begins ##########"

super="animal"
teach "a small russet bat" bat mammal "yarasa"
teach "a flat-tailed dripping beaver" beaver mammal ""
teach "a shaggy sooty buffalo" buffalo mammal "bizon"
teach "a humped humped camel" camel mammal "deve"
teach "a ashen patient donkey" donkey mammal "eşek"
teach "a spiny small hedgehog" hedgehog mammal "kirpi"
teach "a wide ashen hippo" hippo mammal ""
teach "a spotted golden leopard" leopard mammal "leopar"
teach "a sleek streamside otter" otter mammal ""
teach "a horned ashen rhino" rhino mammal ""
teach "a sleek ashen seal" seal mammal "fok"
teach "a tusked weighty walrus" walrus mammal ""
teach "a hooked swift falcon" falcon bird "şahin"
teach "a sharp-eyed russet hawk" hawk bird "atmaca"
teach "a pallid long-necked goose" goose bird "kaz"
teach "a still tall heron" heron bird ""
teach "a black-and-white magpie" magpie bird "saksağan"
teach "a long-legged tall ostrich" ostrich bird "devekuşu"
teach "a bright-tailed proud peacock" peacock bird "tavuskuşu"
teach "a pouched coastal pelican" pelican bird "pelikan"
teach "a sooty waddling penguin" penguin bird "penguen"
teach "a ashen city pigeon" pigeon bird "güvercin"
teach "a pallid coastal seagull" seagull bird "martı"
teach "a striped woodland woodpecker" woodpecker bird "ağaçkakan"
teach "a plump farm turkey" turkey bird "hindi"
teach "a ashen sleek shark" shark fish "köpekbalığı"
teach "a curled tiny seahorse" seahorse fish ""
teach "a chirping small cricket" cricket insect "cırcırböceği"
teach "a spotted scarlet ladybug" ladybug insect "uğurböceği"
teach "a dusty pale moth" moth insect "güve"
teach "a striped stinging wasp" wasp insect "eşekarısı"
teach "a eight-legged patient spider" spider bug "örümcek"
teach "a downy rosy worm" worm bug "solucan"
teach "a armed downy octopus" octopus mollusk "ahtapot"
teach "a long pale squid" squid mollusk "kalamar"
teach "a five-armed amber starfish" starfish creature "denizyıldızı"
teach "a spotted damp newt" newt amphibian ""
teach "a sleek straw-coloured salamander" salamander amphibian "semender"
teach "a warty russet toad" toad amphibian "kurbağa"

nap
log "########## more-menagerie complete ##########"
