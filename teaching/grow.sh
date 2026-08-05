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
#   config    defaults to teaching/always-online.json
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
CFG=${2:-$REPO/teaching/always-online.json}
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

say "building the image from the working tree"
docker build -t "golem:$LABEL" "$REPO" >/dev/null || { say "FATAL build failed"; exit 1; }

docker rm -f "$NAME" >/dev/null 2>&1
docker run -d --name "$NAME" -v "$CFG:/root/.golem/config.json:ro" "golem:$LABEL" sleep infinity >/dev/null
docker exec "$NAME" sh -c 'rm -f /root/.golem/memory.db*'
start_daemon
say "growing '$LABEL' from empty over ${#SCENES[@]} scenes, config $(basename "$CFG")"

for s in "${SCENES[@]}"; do
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
say "grow driver complete: $OUT/$LABEL-final.db"
