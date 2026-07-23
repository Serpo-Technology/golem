#!/usr/bin/env zsh
# Food and drink: dishes, drinks, and ingredients, each placed under its kind and
# tied in by what it is made from and who makes it. Taught by telling. Builds on an
# existing world that holds the fruits, vegetables, and animals these come from.
#
#   GOLEM=golem GAP=1 zsh food-drink.zsh
set -u
GOLEM=${GOLEM:-golem}
GAP=${GAP:-1}

log()  { print -r -- "$(date +%H:%M:%S) $*" }
sayw() { local i; for i in 1 2 3; do $GOLEM tell "$1" >/dev/null 2>&1 && { sleep 0.15; return 0 }; sleep 1; done; return 0 }
saytr(){ local i; for i in 1 2 3; do $GOLEM tell --lang tr "$1" >/dev/null 2>&1 && { sleep 0.15; return 0 }; sleep 1; done; return 0 }
fact() { sayw "$1"; sleep $GAP }
nap()  { log "(nap)"; $GOLEM sleep >/dev/null 2>&1; sleep "${BLOCKGAP:-6}" }
kindof() { local n=$1 k=$2; fact "a $n is a $k"; fact "a $n is a $k"; }

$GOLEM status >/dev/null 2>&1 || { log "daemon unreachable"; exit 1 }
log "########## food and drink begin ##########"

# --- Dishes ------------------------------------------------------------------
log "=== dishes ==="
for d in bread soup salad stew pie cake pizza pasta rice porridge sandwich pancake omelette dumpling; do
  kindof "$d" "dish"
done
nap
fact "a bread is made of flour"; fact "a soup is made of water"; fact "a salad is made of lettuce"
fact "a pie is made of fruit";   fact "a cake is made of sugar"; fact "a pizza is made of cheese"
fact "a stew is made of meat";   fact "a porridge is made of oat"; fact "a pasta is made of wheat"
fact "a baker baked a bread";    fact "a cook cooked a soup";      fact "a chef made a cake"
saytr "ekmek bir yemektir"; saytr "çorba bir yemektir"; saytr "pasta bir yemektir"
nap

# --- Drinks ------------------------------------------------------------------
log "=== drinks ==="
for d in water milk tea coffee juice wine beer soda cocoa lemonade; do
  kindof "$d" "drink"
done
nap
fact "a milk came from a cow";    fact "a juice came from a fruit"; fact "a wine came from a grape"
fact "a tea is hot";              fact "a soda is cold";            fact "a coffee is bitter"
fact "a person drank a water";    fact "a child drank a milk";      fact "a lemonade is sour"
saytr "su bir içecektir"; saytr "süt bir içecektir"; saytr "çay sıcaktır"
nap

# --- Ingredients -------------------------------------------------------------
log "=== ingredients ==="
for i in flour sugar salt butter oil egg cheese wheat oat rice yeast pepper spice cream; do
  kindof "$i" "ingredient"
done
nap
fact "a flour came from a wheat"; fact "a butter came from a milk"; fact "a cheese came from a milk"
fact "a sugar is sweet";          fact "a salt is salty";           fact "a pepper is hot"
fact "an egg came from a hen";    fact "an oil came from a seed";   fact "a cream is thick"
saytr "un buğdaydan geldi"; saytr "şeker tatlıdır"; saytr "tuz tuzludur"
nap
log "########## food and drink complete ##########"
