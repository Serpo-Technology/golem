#!/usr/bin/env zsh
# Four words the corpus states and never shows. It says that a ball is a sphere, that a
# box is a cube, that a sphere and a cube are shapes, and that a container, a furniture,
# an instrument, a tool and a vehicle are objects. A word that is only ever stated has no
# percept to mean, so the placeholder it mints stands above anything a grounded reading
# earns later, and the kinds the corpus does ground have no way up to it.
#
# This lesson says those words over the percepts the corpus has already shown and already
# named. Saying a rung over fresh things of its own does not do the job: the rung forms,
# grounded in its own new percepts, and the hundreds of things taught earlier still cannot
# reach it, because nothing was ever said over both. So every caption below is reproduced
# byte for byte from the lesson that first showed it, and the only words said over it are
# the name it already carries and the rungs standing above that name.
#
#   GOLEM=golem zsh llm-rungs-2.zsh
#
# Run it after the lessons that show these things and before the ones that state the
# rungs. It looks again at 182 things over 188 visits, since the six boxes are also
# containers and come round under both words. Each visit shows its thing three times,
# which runs about half an hour at the default pause. Two thirds of that is the waiting.
#
# PAUSE      seconds to clear the attention window at each change of percept
# BLOCKGAP   seconds after a nap
set -u

GOLEM=${GOLEM:-golem}
# Wider than the daemon's attention recency, so the percept before this one has left the
# window by the time this one is named.
PAUSE=${PAUSE:-6}

log()  { print -r -- "$(date +%H:%M:%S) $*" }
fobs() { local i; for i in 1 2 3; do $GOLEM observe --force vision "$1" >/dev/null 2>&1 && { sleep 0.15; return 0 }; sleep 1; done; return 0 }
sayw() { local i; for i in 1 2 3; do $GOLEM tell "$1" >/dev/null 2>&1 && { sleep 0.15; return 0 }; sleep 1; done; return 0 }
nap()  { log "(nap: consolidate)"; $GOLEM sleep >/dev/null 2>&1; sleep "${BLOCKGAP:-4}" }

# revisit <caption> <rung> [<rung> ...]
#
# One thing, looked at again. The percept's own name is the last word of its caption,
# which is how the corpus captions every one of them, and it is said on all three
# showings so the name keeps the thing. The rungs ride the second and third, which is
# company enough for the nightly pass to abstract them and far too little to unseat the
# name. Nothing new is introduced: the caption, the name and the intermediate rung are
# all words the earlier lesson already said over this same percept.
#
# THE WAIT BELONGS TO THE CHANGE OF PERCEPT. A told word binds to whatever is still
# inside the attention window, so a rung said over one thing while the thing before it
# is still in the window is a rung said over both, and a block that fires percept, word,
# percept, word with no pause hands the rung the whole run of things shown before it.
# Showing the same thing again needs no wait, being the same thing looked at twice, so
# a percept's three showings sit together and the window is cleared once per thing.
revisit() {
  local cap=$1; shift
  local nm=${cap##* } w i
  log "revisit: $nm ($*)"
  sleep $PAUSE
  fobs "$cap"; sayw "$nm"
  for i in 1 2; do
    fobs "$cap"; sayw "$nm"
    for w in "$@"; do sayw "$w"; done
  done
}

$GOLEM status >/dev/null 2>&1 || { log "daemon unreachable at $GOLEM"; exit 1 }
log "########## llm-rungs-2 begins ##########"

# The corpus also states that a diamond, a ring and a star are shapes, and all three are
# left out below. The word in the stated fact means the figure; the percept the corpus
# shows under that same word is a different thing. Its diamond is named diamond, gem,
# material and mineral, its ring is worn on a hand, and its five stars are the sun and
# four suns named in the night sky. Saying shape over those would ground the word in a
# mineral, an ornament and a burning sun, and every later question about shapes would
# read them back. Ball and box carry no such split. The corpus shows round things
# under ball and box-shaped things under box, which is the sense the stated fact uses, so
# shape is said over those and over nothing else.
#
# The puck is the one thing under ball that is left out. The corpus names it a ball for
# the game it belongs to, and what it shows is a flat disc, so it is the single percept
# there that would teach sphere a counterexample.

# --- Round things: sphere over the seven the corpus already calls balls -------------
log "=== round things ==="
revisit "a bright neon softball" ball sphere shape
revisit "a fuzzy yellow tennisball" ball sphere shape
revisit "a muddy leather football" ball sphere shape
revisit "a pebbled orange basketball" ball sphere shape
revisit "a round red ball" sphere shape
revisit "a stitched white baseball" ball sphere shape
revisit "a swirled glass marble" ball sphere shape
nap

# --- Square things: cube over the six the corpus already calls boxes ----------------
log "=== square things ==="
revisit "a bolted oak coffer" box cube shape
revisit "a crushed cardboard carton" box cube shape
revisit "a lacquered ebony casket" box cube shape
revisit "a rusty steel bin" box cube shape
revisit "a splintered wooden crate" box cube shape
revisit "a woven straw caddy" box cube shape
nap

# --- Made things: object over the 175 percepts under the five kinds the corpus calls
# objects. Each of them already carries its kind word, so the rung is said beside the
# word it stands above, on the thing both were said over.

log "=== object over the containers ==="
for cap in \
  "a beaded evening purse" \
  "a bolted oak coffer" \
  "a brown leather knapsack" \
  "a bulging burlap sack" \
  "a bulky nylon duffel" \
  "a chipped stoneware jug" \
  "a crushed cardboard carton" \
  "a drawstring leather pouch" \
  "a galvanized metal pail" \
  "a glass water bottle" \
  "a heavy leather rucksack" \
  "a hooped oak barrel" \
  "a labeled tin canister" \
  "a lacquered ebony casket" \
  "a lidded wicker hamper" \
  "a long stone trough" \
  "a painted clay amphora" \
  "a patched army knapsack" \
  "a round clay pot" \
  "a rusty steel bin" \
  "a scuffed canvas satchel" \
  "a sealed cedar cask" \
  "a slender silver flask" \
  "a splintered wooden crate" \
  "a squat glass jar" \
  "a striped cotton tote" \
  "a tall marble urn" \
  "a vast copper vat" \
  "a wide wooden tub" \
  "a woven straw basket" \
  "a woven straw caddy" \
  "a woven willow basket"
; do revisit "$cap" container object; done
nap

log "=== object over the furniture ==="
for cap in \
  "a banded iron chest" \
  "a cluttered writing desk" \
  "a dented metal locker" \
  "a folding canvas cot" \
  "a frilly white bassinet" \
  "a gilded royal throne" \
  "a long mahogany sideboard" \
  "a long stone bench" \
  "a low round coffeetable" \
  "a mirrored double wardrobe" \
  "a narrow floating shelf" \
  "a padded tilting recliner" \
  "a painted corner cupboard" \
  "a plush velvet sofa" \
  "a polished granite counter" \
  "a rocking wicker cradle" \
  "a sagging corduroy couch" \
  "a scarred oily workbench" \
  "a slanted oak lectern" \
  "a slatted oak chair" \
  "a slatted pine crib" \
  "a small brass nightstand" \
  "a soft brown couch" \
  "a stacked metal bunk" \
  "a swaying rope hammock" \
  "a tall glass cabinet" \
  "a tall wooden cabinet" \
  "a tall wooden shelf" \
  "a tall wooden stool" \
  "a towering walnut bookcase" \
  "a wide oak dresser" \
  "a wide soft mattress" \
  "a wooden brown chair" \
  "a wooden brown desk" \
  "a worn leather armchair"
; do revisit "$cap" furniture object; done
nap

log "=== object over the instruments ==="
for cap in \
  "a black grand piano" \
  "a black slim clarinet" \
  "a brown wooden drum" \
  "a coiled brass horn" \
  "a colorful wooden xylophone" \
  "a glossy wooden violin" \
  "a golden brass trumpet" \
  "a golden curved saxophone" \
  "a huge brass tuba" \
  "a huge hanging gong" \
  "a jingling round tambourine" \
  "a large brown cello" \
  "a polished acoustic guitar" \
  "a red folding accordion" \
  "a round wooden drum" \
  "a shiny black grand piano" \
  "a shiny brass cymbal" \
  "a shiny brass trumpet" \
  "a silver metal flute" \
  "a silver slender flute" \
  "a sliding brass trombone" \
  "a slim wooden oboe" \
  "a small pear-shaped mandolin" \
  "a small steel chime" \
  "a speckled shaking maraca" \
  "a tall golden harp" \
  "a tall wooden pipe organ" \
  "a tiny hawaiian ukulele" \
  "a twangy round banjo" \
  "a warm reddish viola" \
  "a wooden brown guitar" \
  "an old carved lute" \
  "an ornate antique harpsichord"
; do revisit "$cap" instrument object; done
nap

log "=== object over the tools ==="
for cap in \
  "a bent iron crowbar" \
  "a black cordless drill" \
  "a blue cutting jigsaw" \
  "a broad iron axe" \
  "a clear half-circle protractor" \
  "a curved garden hoe" \
  "a heavy pounding jackhammer" \
  "a heavy steel hammer" \
  "a long brass telescope" \
  "a long metal rake" \
  "a long wooden shovel" \
  "a loud angle grinder" \
  "a metal garden rake" \
  "a red steel wrench" \
  "a roaring red chainsaw" \
  "a rough metal file" \
  "a round handheld magnifier" \
  "a round pressure gauge" \
  "a round wall barometer" \
  "a round wooden mallet" \
  "a sharp flat chisel" \
  "a sharp steel axe" \
  "a sharp steel saw" \
  "a sharp toothed saw" \
  "a shiny adjustable wrench" \
  "a shiny weighing scale" \
  "a silver lab microscope" \
  "a small brass compass" \
  "a small gripping pliers" \
  "a small steel drill" \
  "a small steel screwdriver" \
  "a straight wooden ruler" \
  "a thin glass thermometer" \
  "a tight metal clamp" \
  "a wide digging shovel" \
  "a yellow handled screwdriver" \
  "an orange electric sander"
; do revisit "$cap" tool object; done
nap

log "=== object over the vehicles ==="
for cap in \
  "a big blue truck" \
  "a black diving submarine" \
  "a bright orange kayak" \
  "a bright yellow taxi" \
  "a crowded white ferry" \
  "a fast black motorcycle" \
  "a fast military jet" \
  "a fast underground subway" \
  "a flat rubber raft" \
  "a green farm tractor" \
  "a hovering red helicopter" \
  "a huge steel ship" \
  "a large blue truck" \
  "a large white plane" \
  "a long silver airship" \
  "a long silver train" \
  "a long speeding train" \
  "a long yellow bus" \
  "a loud black motorcycle" \
  "a narrow birch canoe" \
  "a quiet white glider" \
  "a red city tram" \
  "a red fast car" \
  "a red racing car" \
  "a red two-wheeled bicycle" \
  "a rugged green jeep" \
  "a silver soaring plane" \
  "a sleek elevated monorail" \
  "a sleek luxury yacht" \
  "a slim silver bicycle" \
  "a small wooden boat" \
  "a striped drifting balloon" \
  "a tall white rocket" \
  "a white billowing sailboat" \
  "a white delivery van" \
  "a yellow long bus" \
  "an old electric trolley" \
  "an old wooden wagon"
; do revisit "$cap" vehicle object; done
nap

log "########## llm-rungs-2 complete ##########"
