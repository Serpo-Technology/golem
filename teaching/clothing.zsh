#!/usr/bin/env zsh
# A wide wardrobe of clothing across four kinds (garment, footwear, headwear,
# accessory), each shown on its own percept, named, placed under its kind and under
# the clothing superordinate, and tied into the world by who wears it and what it
# covers. The kinds abstract from the shared naming; the wear and cover relations
# weave the garments to the people they dress. Builds on an existing world.
#
#   GOLEM=golem GAP=1 zsh clothing.zsh
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
    (( i == 2 )) && sayw "$kind"; (( i == 3 )) && sayw "clothing"
    (( i == 4 )) && { [ -n "$tr" ] && saytr "$tr" }
    (( i == 5 )) && sayw "$kind"
    comp; sleep $GAP
  done
}
fact() { sayw "$1"; sleep $GAP }

$GOLEM status >/dev/null 2>&1 || { log "daemon unreachable"; exit 1 }
log "########## clothing begins ##########"

# --- Garments ----------------------------------------------------------------
teach "a blue cotton shirt"     shirt    garment gömlek
teach "a long wool coat"        coat     garment palto
teach "a red silk dress"        dress    garment elbise
teach "a pleated plaid skirt"   skirt    garment etek
teach "a cozy knit sweater"     sweater  garment kazak
teach "a black leather jacket"  jacket   garment ceket
teach "a gray woolen trousers"  trousers garment pantolon
teach "a blue denim shorts"     shorts   garment şort
teach "a white frilly blouse"   blouse   garment bluz
teach "a green padded vest"     vest     garment yelek
nap
teach "a soft velvet robe"      robe     garment ""
teach "a long evening gown"     gown     garment ""
nap

# --- Footwear ----------------------------------------------------------------
teach "a shiny black shoe"      shoe     footwear ayakkabı
teach "a tall leather boot"     boot     footwear çizme
teach "a brown open sandal"     sandal   footwear sandalet
teach "a striped woolen sock"   sock     footwear çorap
teach "a fuzzy pink slipper"    slipper  footwear terlik
teach "a white canvas sneaker"  sneaker  footwear ""
nap

# --- Headwear ----------------------------------------------------------------
teach "a wide straw hat"        hat      headwear şapka
teach "a red baseball cap"      cap      headwear ""
teach "a hard safety helmet"    helmet   headwear kask
teach "a gray drawstring hood"  hood     headwear kapüşon
teach "a white wrapped turban"  turban   headwear ""
teach "a soft blue beret"       beret    headwear bere
nap

# --- Accessories -------------------------------------------------------------
teach "a long striped scarf"    scarf    accessory atkı
teach "a warm leather glove"    glove    accessory eldiven
teach "a brown leather belt"    belt     accessory kemer
teach "a shiny gold ring"       ring     accessory yüzük
teach "a silver wrist watch"    watch    accessory saat
teach "a red silk tie"          tie      accessory kravat
teach "a beaded silver necklace" necklace accessory kolye
nap

# --- Wear: people put on clothing --------------------------------------------
log "=== wear ==="
fact "a person wore a coat";     fact "a person wore a hat";    fact "a person wore a shoe"
fact "a child wore a sweater";   fact "a woman wore a dress";   fact "a man wore a tie"
fact "a person wore a glove";    fact "a person wore a boot";   fact "a person wore a scarf"
nap

# --- Cover and comfort: clothing dresses the body ----------------------------
log "=== cover ==="
fact "a coat kept a person warm"; fact "a scarf kept a person warm"; fact "a glove covered a hand"
fact "a hat covered a head";      fact "a boot covered a foot";      fact "a sock covered a foot"
fact "a ring adorned a finger";   fact "a belt held a trousers";     fact "a helmet protected a head"
nap
log "########## clothing complete ##########"
