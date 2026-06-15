#!/usr/bin/env bash
# Snapshot or restore GOLEM's memory from the running container. A snapshot is
# the brain's fluid "weights": a self-contained sqlite copy kept on this machine
# so a threshold reached is never lost. Save stops the daemon briefly for a
# clean, checkpointed copy, then brings it back.
#
#   snapshot.sh save <label>     checkpoint the brain to ~/.golem/snapshots/<label>-<ts>.db
#   snapshot.sh restore <file>   load a snapshot back into the container
#   snapshot.sh list             list snapshots, newest last
set -euo pipefail

CONTAINER=${CONTAINER:-golem-brain}
DIR=${SNAP_DIR:-$HOME/.golem/snapshots}
mkdir -p "$DIR"

cmd=${1:-}
case "$cmd" in
  save)
    label=${2:-snapshot}
    ts=$(date +%Y%m%d-%H%M%S)
    dest="$DIR/$label-$ts.db"
    docker exec "$CONTAINER" golem sleep >/dev/null 2>&1 || true
    docker stop "$CONTAINER" >/dev/null
    docker cp "$CONTAINER:/root/.golem/memory.db" "$dest"
    docker start "$CONTAINER" >/dev/null
    printf 'saved %s (%s)\n' "$dest" "$(du -h "$dest" | cut -f1)"
    ;;
  restore)
    src=${2:?usage: snapshot.sh restore <file>}
    docker stop "$CONTAINER" >/dev/null
    docker cp "$src" "$CONTAINER:/root/.golem/memory.db"
    # The previous brain's write-ahead log, applied to the restored database,
    # corrupts it (sqlite reports "disk image is malformed"). Overwrite the WAL
    # and shared-memory files with empty ones; sqlite rebuilds them clean on
    # start.
    empty=$(mktemp); : > "$empty"
    docker cp "$empty" "$CONTAINER:/root/.golem/memory.db-wal"
    docker cp "$empty" "$CONTAINER:/root/.golem/memory.db-shm"
    rm -f "$empty"
    docker start "$CONTAINER" >/dev/null
    printf 'restored %s\n' "$(basename "$src")"
    ;;
  list)
    ls -1trh "$DIR" 2>/dev/null || echo "no snapshots yet"
    ;;
  *)
    echo "usage: snapshot.sh {save <label> | restore <file> | list}"
    exit 1
    ;;
esac
