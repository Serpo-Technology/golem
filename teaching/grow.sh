#!/usr/bin/env bash
# Grow a brain from empty, one scene at a time, checkpointing as it goes.
#
# An engine change invalidates every brain grown before it, so a brain worth
# keeping is grown end to end on ONE frozen engine. This drives that: it builds
# the image from the working tree, teaches each scene in order into a fresh
# container, and saves a checkpoint after each one, so a grow that goes wrong is
# diagnosable at the scene it went wrong in rather than only at the end.
#
#   bash teaching/grow.sh <label> [<config.json>] [<scene>...]
#
#   label     names the snapshot directory under ~/.golem/snapshots/
#   config    defaults to teaching/ostensive.json
#   scenes    default to the order in teaching/curriculum-order.txt
#
# GAP and BLOCKGAP pace the scenes as they do for the lessons themselves.
# Set REHEARSE=0 to skip the closing spine rehearsal.
#
# The rehearsal at the end is not optional in spirit: a long grow erodes what it
# taught first, and re-showing every percept is what rebuilds the pointing the
# naming and category passes read. Skip it only to inspect the erosion itself.
set -u
REPO="$(cd "$(dirname "$0")/.." && pwd)"
LABEL=${1:?usage: grow.sh <label> [config.json] [scene...]}
# The scenes lean on the pointing passes: a word is shown its members and read off what
# it was shown. They stand on by default and both teaching configs write them out, so a
# grow records the reading it was taught under rather than inheriting it silently.
CFG=${2:-$REPO/teaching/ostensive.json}
# The config is bind-mounted, and docker reads a relative path as a named volume rather
# than a file, so it fails with a message about volume names that says nothing about the
# real mistake. Resolve it here and say plainly when it is not there.
[ -f "$CFG" ] || { printf 'FATAL config %s not found\n' "$CFG"; exit 1; }
CFG="$(cd "$(dirname "$CFG")" && pwd)/$(basename "$CFG")"
shift 2 2>/dev/null || shift 1
SCENES=("$@")
if [ ${#SCENES[@]} -eq 0 ]; then
  while read -r s; do [ -n "$s" ] && SCENES+=("$s"); done < "$REPO/teaching/curriculum-order.txt"
fi

NAME="golem-grow-$LABEL"
OUT="$HOME/.golem/snapshots/$LABEL"
GAP=${GAP:-1}
BLOCKGAP=${BLOCKGAP:-6}
mkdir -p "$OUT"

say() { printf '%s %s\n' "$(date +%H:%M:%S)" "$*"; }

# The container runs `sleep infinity` rather than the daemon, so the daemon can be
# stopped and restarted around each checkpoint without losing the container. Every
# restart therefore has to re-exec `golem run` and wait for it: a driver that skips
# the wait teaches into a daemon that is not listening yet, and every scene after
# the first exits at its own unreachable-daemon guard in seconds, silently.
start_daemon() {
  docker exec -d "$NAME" golem run
  for _ in $(seq 1 120); do
    docker exec "$NAME" golem status >/dev/null 2>&1 && return 0
    sleep 1
  done
  say "FATAL daemon did not come up"; exit 1
}
stop_daemon() { docker exec "$NAME" sh -c 'pkill -9 golem >/dev/null 2>&1; sleep 1'; }

checkpoint() {
  stop_daemon
  docker cp "$NAME:/root/.golem/memory.db" "$OUT/after-$1.db" 2>/dev/null \
    || say "WARN could not save checkpoint $1"
  start_daemon
}

# One tag, reused. A tag named after the grow is never dangling, so a run of these
# leaves a pile of images no prune can see, and by the next day nothing says which
# engine any of them held. Two arms of a comparison MUST share an engine besides:
# build once, then run the second arm with BUILD=0 against the very same image.
IMAGE=${IMAGE:-golem:grow}
if [ "${BUILD:-1}" = 1 ]; then
  say "building $IMAGE from the working tree"
  docker build -t "$IMAGE" "$REPO" >/dev/null || { say "FATAL build failed"; exit 1; }
else
  docker image inspect "$IMAGE" >/dev/null 2>&1 || { say "FATAL $IMAGE not built"; exit 1; }
  say "reusing $IMAGE unbuilt, so this arm runs the engine the last one did"
fi

docker rm -f "$NAME" >/dev/null 2>&1
docker run -d --name "$NAME" -v "$CFG:/root/.golem/config.json:ro" "$IMAGE" sleep infinity >/dev/null

# The image carries the engine and the scenes together, so two arms that differ in what
# they teach would otherwise differ in engine too, and the comparison would measure both
# at once. PATCH names a directory of scene files copied over /src/teaching once the
# container is up: the binary stays the one the image was built with, and only the lesson
# changes. Point it at the older scenes to grow the control for a corpus change.
if [ -n "${PATCH:-}" ]; then
  [ -d "$PATCH" ] || { say "FATAL PATCH directory $PATCH not found"; exit 1; }
  n=0
  for f in "$PATCH"/*.zsh; do
    [ -e "$f" ] || break
    docker cp "$f" "$NAME:/src/teaching/$(basename "$f")" >/dev/null \
      || { say "FATAL could not patch $(basename "$f")"; exit 1; }
    n=$((n + 1))
  done
  say "patched $n scene(s) over the image, so this arm teaches a different corpus on the same engine"
fi

# A grow runs for hours and lands a checkpoint after every scene, so a run cut short by a
# restarted engine or a closed lid should not have to begin again from nothing. With
# RESUME=1 the scenes whose checkpoints are already on disk are taken as taught, the last
# of them is loaded back in, and teaching picks up at the next one. Only an unbroken run
# from the first scene counts: a checkpoint after a gap was grown from a brain this run
# never made, and continuing from it would teach one history on top of another.
SKIP=0
LAST=
if [ "${RESUME:-0}" = 1 ]; then
  for s in "${SCENES[@]}"; do
    [ -f "$OUT/after-$s.db" ] || break
    SKIP=$((SKIP + 1)); LAST=$s
  done
fi
docker exec "$NAME" sh -c 'rm -f /root/.golem/memory.db*'
if [ -n "$LAST" ]; then
  say "resuming after $LAST, $SKIP scene(s) already on disk"
  docker cp "$OUT/after-$LAST.db" "$NAME:/root/.golem/memory.db" >/dev/null \
    || { say "FATAL could not load $OUT/after-$LAST.db"; exit 1; }
fi
start_daemon
say "growing '$LABEL' from empty over ${#SCENES[@]} scenes, config $(basename "$CFG")"

taught=0
for s in "${SCENES[@]}"; do
  taught=$((taught + 1))
  [ "$taught" -le "$SKIP" ] && continue
  [ -f "$REPO/teaching/$s.zsh" ] || { say "skip $s (no such scene)"; continue; }
  say "teaching $s"
  # The image carries the whole tree at /src, so the scenes and the rehearsal tool
  # are already there and already agree with the engine they were built beside.
  docker exec "$NAME" env GOLEM=golem GAP="$GAP" BLOCKGAP="$BLOCKGAP" \
    zsh "/src/teaching/$s.zsh" >/dev/null 2>&1
  checkpoint "$s"
  size=$(docker exec "$NAME" sh -c 'wc -c < /root/.golem/memory.db' 2>/dev/null | tr -d ' ')
  say "===== done $s (${size:-?} bytes)"
done

if [ "${REHEARSE:-1}" = 1 ]; then
  say "rehearsing the spine over every scene"
  docker exec "$NAME" env GOLEM=golem bash /src/teaching/rehearse-spine.sh "${SCENES[@]}" >/dev/null 2>&1
  checkpoint "rehearsed"
fi

stop_daemon
docker cp "$NAME:/root/.golem/memory.db" "$OUT/$LABEL-final.db" 2>/dev/null
# The brain is on disk now and the container holds nothing else worth keeping. Set
# KEEP=1 to leave it up for a post mortem on a grow that went wrong.
[ "${KEEP:-0}" = 1 ] || docker rm -f "$NAME" >/dev/null 2>&1
say "grow driver complete: $OUT/$LABEL-final.db"
