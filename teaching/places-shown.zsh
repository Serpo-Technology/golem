#!/usr/bin/env zsh
# The places shown, so their names mean something they have seen.
#
# geography teaches fifty places by telling alone: "the toronto is a city", "the nile
# flowed through the egypt". A word only ever talked about owns no percept, and the
# cross-tongue reuse hands such a word the one named percept it happens to keep company
# with, at full naming strength. Measured on the deliverable: toronto came to name an
# elevator, barcelona a pigeon, alexandria a turkey, arctic a sunflower, so the brain
# answered "what is a pigeon" with "a pigeon is a toronto" and "is a pigeon a bird" with
# "yes, a barcelona is a bird". Ninety seven words were bound that way.
#
# A place shown owns its own percept, and a word that owns one is not a word in search of
# a referent, so the reuse passes it by. The kind rides the later showings the way every
# other rung does, which is also what lets "is toronto a city" climb rather than rest on
# the told fact alone.
#
# Each caption is headed by the place itself, never by its kind: a caption ending in its
# own class is read as a basic-level name and forecloses the class, which is the rule the
# whole corpus is built on. So "a bustling quarter toronto", never "a canadian city".
#
#   GOLEM=golem zsh places-shown.zsh
set -u
GOLEM=${GOLEM:-golem}
PAUSE=${PAUSE:-6}

log()  { print -r -- "$(date +%H:%M:%S) $*" }
fobs() { local i; for i in 1 2 3; do $GOLEM observe --force vision "$1" >/dev/null 2>&1 && { sleep 0.15; return 0 }; sleep 1; done; return 0 }
sayw() { local i; for i in 1 2 3; do $GOLEM tell "$1" >/dev/null 2>&1 && { sleep 0.15; return 0 }; sleep 1; done; return 0 }
nap()  { log "(nap: consolidate)"; $GOLEM sleep >/dev/null 2>&1; sleep "${BLOCKGAP:-6}" }

# The wait belongs to the change of percept: a told word binds to whatever is still inside
# the attention window, so a name said over one place while the place before it is still in
# view is a name said over both. The three showings of one place sit together and need no
# wait between them.
# SIX showings, not three. Three is enough to re-ground a percept that already exists, which
# is what the rung lessons do, but these percepts are NEW: the merge pass keeps the sharpest
# occurrence out as an exemplar, so genesis wants five or six showings before a concept forms
# at all. At three the captions sat in the store as episodes and no concept was minted, and
# the place names gained nothing. The name is said on every showing and the kind only on the
# last three, so the kind gains its company without ever outweighing the name.
show() {
  local cap=$1 name=$2 kind=$3 i
  sleep $PAUSE
  for i in 1 2 3; do fobs "$cap"; sayw "$name"; done
  for i in 4 5 6; do fobs "$cap"; sayw "$name"; sayw "$kind"; done
}

$GOLEM status >/dev/null 2>&1 || { log "daemon unreachable at $GOLEM"; exit 1 }
log "########## places begins ##########"

log "=== city ==="
show "a sprawling skyline newyork" "newyork" "city"
show "a terraced harbour chicago" "chicago" "city"
show "a bustling quarter toronto" "toronto" "city"
show "a sunlit avenue riodejaneiro" "riodejaneiro" "city"
show "a windswept rooftop munich" "munich" "city"
show "a sleepy market milan" "milan" "city"
show "a crowded skyline barcelona" "barcelona" "city"
show "a gleaming harbour lyon" "lyon" "city"
show "a foggy quarter manchester" "manchester" "city"
show "a hilly avenue glasgow" "glasgow" "city"
nap
show "a walled rooftop naples" "naples" "city"
show "a lively market hamburg" "hamburg" "city"
show "a humid skyline kyoto" "kyoto" "city"
show "a breezy harbour shanghai" "shanghai" "city"
show "a dusty quarter mumbai" "mumbai" "city"
show "a restless avenue busan" "busan" "city"
show "a tiled rooftop alexandria" "alexandria" "city"
show "a leafy market mombasa" "mombasa" "city"
show "a narrow skyline lagos" "lagos" "city"
show "a winding harbour sydney" "sydney" "city"
nap
show "an open quarter istanbul" "istanbul" "city"
show "a distant avenue izmir" "izmir" "city"
show "a tidy rooftop saintpetersburg" "saintpetersburg" "city"
show "a noisy market geneva" "geneva" "city"

log "=== river ==="
show "a swift channel nile" "nile" "river"
show "a muddy current amazon" "amazon" "river"
show "a winding bank thames" "thames" "river"
show "a shallow bend seine" "seine" "river"
show "a glassy channel rhine" "rhine" "river"
show "a reedy current danube" "danube" "river"
nap
show "a silted bank ganges" "ganges" "river"
show "a braided bend yangtze" "yangtze" "river"
show "a sluggish channel volga" "volga" "river"
show "a brisk current tiber" "tiber" "river"
show "a turbid bank ebro" "ebro" "river"
show "a wide bend mississippi" "mississippi" "river"

log "=== mountain ==="
show "a jagged ridge everest" "everest" "mountain"
show "a snowcapped slope alps" "alps" "mountain"
show "a craggy summit andes" "andes" "mountain"
show "a barren face kilimanjaro" "kilimanjaro" "mountain"
nap
show "a lofty ridge fuji" "fuji" "mountain"
show "a steep slope elbrus" "elbrus" "mountain"
show "a rugged summit atlas" "atlas" "mountain"
show "a bare face urals" "urals" "mountain"

log "=== sea ==="
show "a restless swell atlantic" "atlantic" "sea"
show "a stormy tide pacific" "pacific" "sea"
show "a salty expanse mediterranean" "mediterranean" "sea"
show "a glassy horizon baltic" "baltic" "sea"
show "an icy reach caspian" "caspian" "sea"
show "an open brine arctic" "arctic" "sea"
nap
nap

log "########## places done ##########"
