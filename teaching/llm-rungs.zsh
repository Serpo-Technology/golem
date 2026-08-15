#!/usr/bin/env zsh
# This lesson shows round things, square things, and made things, so that four words the
# corpus so far only states can abstract from what they were said over. The corpus states that a
# ball is a sphere, that a box is a cube, that a sphere and a cube are shapes, and that a
# tool, a vehicle, a furniture, an instrument and a container are objects. A word that is
# only stated has no percept to mean, and the placeholder it mints outranks any grounded
# reading that comes later. So sphere, cube, shape and object are said here over the things
# themselves, each of them over many different percepts, because the nightly pass tells a
# class from a name by how far the word's showings spread: a word that lands mostly on one
# percept stays that percept's name.
#
# Every caption names one specific thing and never the class it belongs to. The naming pass
# reads the head noun of a caption as the basic name of what was shown, so a caption headed
# by its own class hands that class word away as a name, and the category it was meant to
# build never forms. The class words are kept for the naming, where they can be said over a
# whole spread of different things.
#
#   GOLEM=golem GAP=1 zsh llm-rungs.zsh
set -u
GOLEM=${GOLEM:-golem}
GAP=${GAP:-1}

log()  { print -r -- "$(date +%H:%M:%S) $*" }
obs()  { local i; for i in 1 2 3; do $GOLEM observe vision "$1" >/dev/null 2>&1 && { sleep 0.15; return 0 }; sleep 1; done; return 0 }
fobs() { local i; for i in 1 2 3; do $GOLEM observe --force vision "$1" >/dev/null 2>&1 && { sleep 0.15; return 0 }; sleep 1; done; return 0 }
sayw() { local i; for i in 1 2 3; do $GOLEM tell "$1" >/dev/null 2>&1 && { sleep 0.15; return 0 }; sleep 1; done; return 0 }
saytr(){ local i; for i in 1 2 3; do $GOLEM tell --lang tr "$1" >/dev/null 2>&1 && { sleep 0.15; return 0 }; sleep 1; done; return 0 }
nap()  { log "(nap)"; $GOLEM sleep >/dev/null 2>&1; sleep "${BLOCKGAP:-6}" }

# The companion scenery is shown between the namings to keep the channel varied. Nothing is
# ever said over it, so none of it is named. The two lists are stepped by one counter and their
# lengths share no factor, so a pair never comes round twice in a run and no token is seen
# often enough to settle into a concept of its own.
adj=(misty breezy shaded sunlit windswept moonlit overgrown cobbled gravelled puddled
     deserted bustling echoing sprawling cramped airy murky hazy frosty balmy sultry
     blustery drizzly starlit lamplit gloomy cheerful weathered peeling crumbling brambly
     ferny thorny grassy pebbly silty chalky loamy briny foggy smoky snowy sleety windy
     stormy sunny cloudy)
nn=(underpass cloister esplanade quayside breakwater boardwalk mezzanine stairwell
    vestibule atrium colonnade portico gatehouse slipway crosswalk waterfront hilltop
    culvert spillway millpond towpath embankment escarpment foreshore headland promontory
    moorland coppice hedgerow paddock skyline treeline riverbank sandbar mudflat saltmarsh
    backstreet sidestreet crossroads roundabout forecourt backyard driveway walkway gangway
    laneway)
ci=0
comp() { obs "a $adj[$(( (ci % ${#adj[@]}) + 1 ))] $nn[$(( (ci % ${#nn[@]}) + 1 ))]"; ci=$((ci+1)) }

# teach: caption name rung top tr
#
# Five showings of one percept. The name is said on every one of them, so it wins the
# percept and stays its basic name. The rung above it and the rung above that are each said
# on two showings, which is enough company to ground them and far too little to outrank the
# name. The Turkish naming rides the fourth showing, where the English rungs are quiet.
teach() {
  local cap=$1 name=$2 rung=$3 top=$4 tr=$5 i
  log "teach: $name ($rung, $top)"
  for i in 1 2 3 4 5; do
    fobs "$cap"; sayw "$name"
    (( i == 2 )) && sayw "$rung"
    (( i == 3 )) && sayw "$top"
    (( i == 4 )) && { [ -n "$tr" ] && saytr "$tr" }
    (( i == 5 )) && { sayw "$rung"; sayw "$top" }
    comp; sleep $GAP
  done
}

$GOLEM status >/dev/null 2>&1 || { log "daemon unreachable"; exit 1 }
log "########## llm-rungs begins ##########"

# --- Round things: nine percepts for sphere, and the first nine for shape ----------------
log "=== round things ==="
teach "a tilted schoolroom globe"   globe      sphere shape yerküre
teach "a drifting soapy bubble"     bubble     sphere shape kabarcık
teach "a simmered peppery meatball" meatball   sphere shape köfte
teach "a packed slushy snowball"    snowball   sphere shape kartopu
teach "a pitted rusted cannonball"  cannonball sphere shape gülle
teach "a glossy speckled gumball"   gumball    sphere shape ""
teach "a ridged autumn pumpkin"     pumpkin    sphere shape balkabağı
teach "a hairy husked coconut"      coconut    sphere shape ""
teach "a blushing tart grapefruit"  grapefruit sphere shape greyfurt
nap

# --- Square things: eight percepts for cube, and eight more for shape --------------------
log "=== square things ==="
teach "a dotted ivory dice"         dice        cube shape zar
teach "a tufted velvet ottoman"     ottoman     cube shape puf
teach "a mossy sealed terrarium"    terrarium   cube shape ""
teach "a rough grey cinderblock"    cinderblock cube shape ""
teach "a bound fodder bale"         bale        cube shape balya
teach "a tiny striped matchbox"     matchbox    cube shape ""
teach "a carved alabaster plinth"   plinth      cube shape ""
teach "a bolted riveted strongbox"  strongbox   cube shape ""
nap

# --- Made things: fifteen percepts for object, three under each kind the corpus calls one -
log "=== made things: tools and vehicles ==="
teach "a gritty pointed trowel"     trowel   tool    object mala
teach "a greasy notched spanner"    spanner  tool    object ""
teach "a slim tapered awl"          awl      tool    object ""
teach "a lacquered slender gondola" gondola  vehicle object ""
teach "a dented mint scooter"       scooter  vehicle object ""
teach "a gilded jingling sleigh"    sleigh   vehicle object kızak
nap

log "=== made things: furniture and instruments ==="
teach "a rumpled sagging futon"     futon     furniture  object ""
teach "a scratched walnut bureau"   bureau    furniture  object ""
teach "a mirrored towering armoire" armoire   furniture  object ""
teach "a resonant fretted zither"   zither    instrument object ""
teach "a chromed pocket harmonica"  harmonica instrument object mızıka
teach "a polished reedy bassoon"    bassoon   instrument object ""
nap

log "=== made things: containers ==="
teach "a faceted crystal decanter"  decanter container object sürahi
teach "a dented vacuum thermos"     thermos  container object termos
teach "a fluted frosted carafe"     carafe   container object ""
nap
log "########## llm-rungs complete ##########"
