#!/usr/bin/env zsh
# Qualities a person can hold and states a person can be in, none of them ever seen.
# Each is minted from the company its tellings keep. A category holds it, a person shows
# or seeks or fears it, and comparisons place it on a scale against its opposite. A few
# carry a Turkish twin. Builds on an existing world.
#
#   GOLEM=golem GAP=1 zsh qualities-abstract.zsh
set -u
GOLEM=${GOLEM:-golem}
GAP=${GAP:-1}

log()  { print -r -- "$(date +%H:%M:%S) $*" }
sayw() { local i; for i in 1 2 3; do $GOLEM tell "$1" >/dev/null 2>&1 && { sleep 0.15; return 0 }; sleep 1; done; return 0 }
saytr(){ local i; for i in 1 2 3; do $GOLEM tell --lang tr "$1" >/dev/null 2>&1 && { sleep 0.15; return 0 }; sleep 1; done; return 0 }
fact() { sayw "$1"; sleep $GAP }
nap()  { log "(nap)"; $GOLEM sleep >/dev/null 2>&1; sleep "${BLOCKGAP:-6}" }

$GOLEM status >/dev/null 2>&1 || { log "daemon unreachable"; exit 1 }
log "########## qualities begin ##########"

# --- Virtues: a quality a person shows ---------------------------------------
log "=== qualities ==="
qual=(courage honesty patience kindness wisdom justice mercy loyalty honor
      generosity humility faith grace strength)
for q in $qual; do
  fact "$q is a quality"; fact "a person shows $q"
done
nap

# --- Good states: a state a person seeks -------------------------------------
log "=== states sought ==="
good=(health sleep rest peace wealth safety freedom life)
for s in $good; do
  fact "$s is a state"; fact "a person seeks $s"
done
nap

# --- Hard states: a state a person fears -------------------------------------
log "=== states feared ==="
hard=(hunger thirst war poverty sickness danger slavery death)
for s in $hard; do
  fact "$s is a state"; fact "a person fears $s"
done
nap

# --- Placing them on scales --------------------------------------------------
log "=== comparisons ==="
fact "courage is stronger than fear";   fact "peace is better than war"
fact "health is better than sickness";  fact "wealth is better than poverty"
fact "freedom is better than slavery";  fact "safety is better than danger"
fact "patience is stronger than anger"; fact "love is stronger than fear"
nap

# --- Turkish threads ---------------------------------------------------------
saytr "barış bir durumdur"; saytr "savaş bir durumdur"; saytr "sağlık bir durumdur"
saytr "barış savaştan iyidir"; saytr "bir insan barış arar"
nap
log "########## qualities complete ##########"
