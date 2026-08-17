#!/usr/bin/env zsh
# The rungs the corpus states over things it never says them over.
#
# Same fault as the object and thing rungs, in what is left of it. A stated rung writes no
# grounding for the climb to use, so a percept reaches only the words some lesson SAID over
# it. The corpus states that a person is an animal and says animal over 130 percepts, none
# of them a person, so all 38 people climb to person and thing and stop. It states that a
# desk is a table and never says table over anything at all. This revisits 139 percepts
# other lessons already show and says the missing rung over each.
#
# NOTHING NEW IS SHOWN. Every caption below is reproduced byte for byte from the lesson that
# already teaches it, so the rung lands on the percept the learner holds rather than beside
# it. The wait belongs to the change of percept: a thing's showings sit together and the
# window is cleared once per thing, so a rung cannot smear onto the thing shown before it.
# The name is said on every showing and the rung only on the later two, so the rung gains
# the company it needs without unseating the name.
#
# The abstract tops are deliberately absent. A ball is a sphere and a sphere is a shape, but
# a ball is not a QUALITY, it has one, and no lesson says quality over anything shown.
#
#   GOLEM=golem zsh rungs-remaining.zsh
set -u
GOLEM=${GOLEM:-golem}
PAUSE=${PAUSE:-6}

log()  { print -r -- "$(date +%H:%M:%S) $*" }
fobs() { local i; for i in 1 2 3; do $GOLEM observe --force vision "$1" >/dev/null 2>&1 && { sleep 0.15; return 0 }; sleep 1; done; return 0 }
sayw() { local i; for i in 1 2 3; do $GOLEM tell "$1" >/dev/null 2>&1 && { sleep 0.15; return 0 }; sleep 1; done; return 0 }
nap()  { log "(nap: consolidate)"; $GOLEM sleep >/dev/null 2>&1; sleep "${BLOCKGAP:-6}" }

revisit() {
  local cap=$1; shift
  local nm=${cap##* } w i
  sleep $PAUSE
  fobs "$cap"; sayw "$nm"
  for i in 1 2; do
    fobs "$cap"; sayw "$nm"
    for w in "$@"; do sayw "$w"; done
  done
}

$GOLEM status >/dev/null 2>&1 || { log "daemon unreachable at $GOLEM"; exit 1 }
log "########## the remaining rungs begin ##########"


log "=== block 1 ==="
revisit "a armed downy octopus" "creature"
revisit "a barbed round morningstar" "bludgeon"
revisit "a bent scribbling writer" "animal"
revisit "a bent stitching tailor" "animal"
revisit "a blazing yellow sun" "shape"
revisit "a blue clawed lobster" "creature"
revisit "a bolted oak coffer" "box"
revisit "a brass key" "material"
revisit "a brass lantern" "material"
revisit "a bright blue vega" "shape"
revisit "a bright happy daughter" "animal"
revisit "a bright neon softball" "toy"
revisit "a bright polished silver" "gem"
revisit "a bright skipping girl" "animal" "child"
revisit "a bright tailed comet" "smallbody"
revisit "a brilliant white sirius" "shape"
revisit "a broad muscular smith" "animal"
revisit "a busy round cook" "animal"
revisit "a busy young builder" "animal"
revisit "a calm careful doctor" "animal"
revisit "a charred fallen meteorite" "smallbody"
revisit "a cheerful running boy" "animal" "child"
revisit "a cheerful young sister" "animal"
revisit "a chipped ceramic plate" "utensil"
revisit "a cluttered writing desk" "table"
revisit "a crushed cardboard carton" "box"
revisit "a deep wooden bowl" "utensil"
revisit "a dim orange titan" "moon"
revisit "a distant blazing rigel" "shape"
revisit "a dusty stooped miner" "animal"
revisit "a five-armed amber starfish" "creature"
revisit "a floating glowing jellyfish" "creature"
revisit "a frozen distant pluto" "smallbody"
revisit "a frozen rubber puck" "toy"
revisit "a fuzzy yellow tennisball" "toy"
nap

log "=== block 2 ==="
revisit "a giggling wobbling toddler" "animal" "child"
revisit "a golden lion" "mammal"
revisit "a gray gentle grandmother" "animal"
revisit "a gray wolf" "mammal"
revisit "a green bottle" "green"
revisit "a green frog" "amphibian"
revisit "a green pear" "food"
revisit "a heavy iron pan" "utensil"
revisit "a heavy oak drawbridge" "bridge"
revisit "a hooked steel halberd" "polearm"
revisit "a kind steady nurse" "animal"
revisit "a lacquered ebony casket" "box"
revisit "a lively young brother" "animal"
revisit "a long arched viaduct" "bridge"
revisit "a long orange carrot" "food"
revisit "a long pale squid" "creature"
revisit "a long wooden spear" "polearm"
revisit "a massive blunt warhammer" "bludgeon"
revisit "a muddy leather football" "activity" "toy"
revisit "a narrow wooden footbridge" "bridge"
revisit "a open-mouthed loud singer" "animal"
revisit "a pale round luna" "moon"
revisit "a pallid steamed rice" "grain"
revisit "a patient standing teacher" "animal"
revisit "a pebbled orange basketball" "toy"
revisit "a plump cheerful baker" "animal"
revisit "a polished granite counter" "table"
revisit "a proud crowned king" "animal"
revisit "a purple grape" "food"
revisit "a quiet watchful hunter" "animal"
revisit "a red apple" "food"
revisit "a regal crowned queen" "animal"
revisit "a robed grave judge" "animal"
nap

log "=== block 3 ==="
revisit "a rocky tumbling asteroid" "smallbody"
revisit "a rough knotted club" "bludgeon"
revisit "a round clay plate" "utensil"
revisit "a round copper coin" "circle" "material" "shape"
revisit "a round dwarf ceres" "smallbody"
revisit "a round red ball" "shape" "sphere" "toy"
revisit "a round wooden shield" "bludgeon"
revisit "a rusty steel bin" "box"
revisit "a scarred oily workbench" "table"
revisit "a seated unhurried driver" "animal"
revisit "a sharp sweeping glaive" "polearm"
revisit "a shiny gold ring" "shape"
revisit "a shiny red apple" "food"
revisit "a shiny weighing scale" "tool"
revisit "a shiny yellow gold" "gem"
revisit "a short thick cudgel" "bludgeon"
revisit "a sideways scuttling crab" "creature"
revisit "a slanted oak lectern" "table"
revisit "a sleek straw-coloured salamander" "amphibian"
revisit "a slow spiral snail" "bug"
revisit "a small brass nightstand" "table"
revisit "a small iron nail" "material"
revisit "a small lumpy phobos" "moon"
revisit "a small smiling baby" "animal" "child"
revisit "a smiling near friend" "animal"
revisit "a smooth icy europa" "moon"
revisit "a soft slimy slug" "bug"
revisit "a sparkling clear diamond" "shape"
revisit "a spattered absorbed painter" "animal"
revisit "a spiked iron mace" "bludgeon"
revisit "a splintered wooden crate" "box"
revisit "a spotted deer" "mammal"
revisit "a spotted volcanic io" "moon"
revisit "a squat porcelain teapot" "utensil"
revisit "a steady northern polaris" "shape"
nap

log "=== block 4 ==="
revisit "a stern armored soldier" "animal"
revisit "a stitched white baseball" "toy"
revisit "a stout infantry pike" "polearm"
revisit "a stout jolly uncle" "animal"
revisit "a streaking bright meteor" "smallbody"
revisit "a striped drifting balloon" "toy"
revisit "a sturdy young son" "animal"
revisit "a swinging chained flail" "bludgeon"
revisit "a tall bearded sailor" "animal"
revisit "a tall calm father" "animal"
revisit "a tall jousting lance" "polearm"
revisit "a thick blue book" "sign"
revisit "a thick golden honey" "food"
revisit "a three pronged trident" "polearm"
revisit "a tiny dark deimos" "moon"
revisit "a tiny gray snail" "bug"
revisit "a tiny sleeping infant" "animal" "child"
revisit "a uniformed alert pilot" "animal"
revisit "a veined white marble" "toy"
revisit "a warm brown bronze" "gem"
revisit "a warm gentle aunt" "animal"
revisit "a warm smiling mother" "animal"
revisit "a warty russet toad" "amphibian"
revisit "a waving nearby neighbor" "animal"
revisit "a weathered patient fisher" "animal"
revisit "a weathered strong farmer" "animal"
revisit "a whistling steel kettle" "utensil"
revisit "a white egg" "food"
revisit "a wide concrete overpass" "bridge"
revisit "a woven straw caddy" "box"
revisit "a yellow banana" "food"
revisit "an old roman aqueduct" "bridge"
revisit "an orange carrot" "food"
nap

log "########## the remaining rungs done ##########"
