#!/usr/bin/env zsh
# A second, larger web of plain cross-domain facts, run over a world that already holds
# the concepts. Simple agent-verb-object propositions (low pollution risk, unlike the
# comparison chains) that thicken the graph: who does what, what is where, what is made
# of what, who works with what. Mints little, relates much -- the density that carries
# brain size. Run late.
#
#   GOLEM=golem GAP=2 zsh relations-web-2.zsh
set -u
GOLEM=${GOLEM:-golem}
GAP=${GAP:-2}

log()  { print -r -- "$(date +%H:%M:%S) $*" }
sayw() { local i; for i in 1 2 3; do $GOLEM tell "$1" >/dev/null 2>&1 && { sleep 0.15; return 0 }; sleep 1; done; return 0 }
fact() { sayw "$1"; sleep $GAP }
nap()  { log "(nap)"; $GOLEM sleep >/dev/null 2>&1; sleep "${BLOCKGAP:-6}" }
block() { local n=0; for f in "$@"; do fact "$f"; n=$((n+1)); done }

$GOLEM status >/dev/null 2>&1 || { log "daemon unreachable"; exit 1 }
log "########## relations web 2 begins ##########"

log "=== animals act ==="
block "a cat caught a mouse" "a dog barked at a stranger" "a bird built a nest" "a bee made a honey" \
  "a spider caught a fly" "a cow gave a milk" "a hen laid an egg" "a sheep grew a wool" \
  "a horse pulled a cart" "a dog guarded a house" "a cat climbed a tree" "a fish swam upstream" \
  "a bear caught a salmon" "a wolf howled at a moon" "a frog caught a fly" "an owl hunted a mouse"; nap

log "=== people work ==="
block "a farmer plowed a field" "a fisher mended a net" "a smith heated an iron" "a baker kneaded a dough" \
  "a weaver spun a thread" "a mason laid a brick" "a shepherd herded a sheep" "a hunter tracked a deer" \
  "a miller ground a grain" "a potter moulded a clay" "a carpenter sawed a plank" "a sailor tied a rope" \
  "a merchant sold a good" "a scholar read a book" "a doctor mixed a medicine" "a soldier carried a shield"; nap

log "=== things made of / hold ==="
block "a barrel held a wine" "a jar held a honey" "a sack held a grain" "a chest held a gold" \
  "a nest held an egg" "a cage held a bird" "a stable held a horse" "a pen held a sheep" \
  "a bridge crossed a river" "a road led to a town" "a path led to a forest" "a gate closed a wall" \
  "a roof covered a house" "a fence bounded a field" "a well held a water" "a lamp lit a room"; nap

log "=== nature acts ==="
block "a river carved a valley" "a wind moulded a dune" "a rain filled a lake" "a frost cracked a stone" \
  "a fire cleared a forest" "a flood covered a field" "a sun dried a soil" "a snow capped a mountain" \
  "a wave moulded a shore" "a root split a rock" "a vine climbed a wall" "a moss covered a stone" \
  "a cloud hid a sun" "a storm bent a tree" "a spring fed a stream" "a glacier carved a fjord"; nap

log "=== who has what ==="
block "a king has a crown" "a knight has a sword" "a bird has a wing" "a fish has a fin" \
  "a tree has a root" "a house has a door" "a book has a page" "a clock has a hand" \
  "a river has a bank" "a mountain has a peak" "a flower has a petal" "a wheel has a spoke" \
  "a ship has a sail" "a candle has a flame" "a bee has a sting" "a cat has a claw"; nap

log "=== part of ==="
block "a keel is part of a ship" "a blade is part of a knife" "a lens is part of a telescope" \
  "a hinge is part of a door" "a spoke is part of a wheel" "a barrel is part of a gun" \
  "a trunk is part of a tree" "a crust is part of a bread" "a yolk is part of an egg" \
  "a stem is part of a flower" "a beak is part of a bird" "a scale is part of a fish" \
  "a mane is part of a lion" "a shell is part of a turtle" "a tusk is part of an elephant"; nap
log "########## relations web 2 complete ##########"
