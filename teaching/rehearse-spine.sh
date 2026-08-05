#!/usr/bin/env bash
# Rebuild the is-a spine after a long front-loaded grow.
#
# A grow that teaches each domain once and moves on erodes its early
# percept-grounded categories: as later scenes stop mentioning them, the
# naming episodes fade, the category's core company thins, and the nightly
# abstraction pass grounds fewer members until a class that still names
# itself grounds almost nothing and its is-a answers turn to No. Re-showing
# the percepts rebuilds the grounding; re-telling facts does not, since a
# spoken sentence is an utterance and utterances are excluded from naming
# company.
#
# This tool replays every taught percept (re-show) and every name/is-a
# (re-tell) across all scenes, minus companions, in two rounds with periodic
# naps, so the whole spine reconsolidates in one pass without the tail
# eroding the head. Run it once at the end of a grow.
#
# Pacing is load-bearing, not politeness. A told word binds to every percept
# still inside the daemon's attention window, so a replay that fires scene
# after scene with no pause hands each category word the whole run of things
# shown before it, and the naming company it rebuilds is uniform noise. So a
# percept that differs from the one before it waits out the window first; a
# re-show of the same percept does not, being the same thing looked at again.
#
#   GOLEM=golem bash teaching/rehearse-spine.sh <scene1> <scene2> ...
#
# PAUSE   seconds to clear the attention window at each change of percept
set -u
GOLEM=${GOLEM:-golem}
# Wider than the daemon's attention recency, so the previous percept has left
# the window by the time the next one is named.
PAUSE=${PAUSE:-6}
REPO="$(cd "$(dirname "$0")/.." && pwd)"
WORK="$(mktemp -d)"; trap 'rm -rf "$WORK"' EXIT
# mock that records each golem call, so a dry run yields the taught atoms
printf '#!/bin/sh\nprintf "%%s\\n" "$*" >> "$CAPTURE"\nexit 0\n' > "$WORK/golem"
printf '#!/bin/sh\nexit 0\n' > "$WORK/sleep"
chmod +x "$WORK/golem" "$WORK/sleep"
: > "$WORK/atoms"
for s in "$@"; do
  f="$REPO/teaching/$s.zsh"; [ -f "$f" ] || continue
  export CAPTURE="$WORK/cap"; : > "$CAPTURE"
  PATH="$WORK:$PATH" GOLEM=golem GAP=0 BLOCKGAP=0 zsh "$f" >/dev/null 2>&1
  grep -E '^(observe|tell)' "$CAPTURE" >> "$WORK/atoms"
done
# Gather each percept's own atoms together, keeping their order within the percept and
# the percepts in the order the corpus first shows them. The corpus returns to a thing
# five times across a block, so replaying in corpus order means the window has to be
# cleared at nearly every showing; gathered, it is cleared once per thing. That is the
# difference between a pause for each of 3500 showings and one for each of 624 things,
# and it costs nothing, since what a percept needs is its own showings and the words
# said over them, not the neighbours it happened to sit between.
awk '
  /^observe --force vision / {
    cap = substr($0, length("observe --force vision ") + 1)
    if (!(cap in seen)) { seen[cap] = ++n; order[n] = cap }
    buf[cap] = buf[cap] $0 "\n"; next
  }
  /^tell / { if (cap != "") buf[cap] = buf[cap] $0 "\n"; next }
  END { for (i = 1; i <= n; i++) printf "%s", buf[order[i]] }
' "$WORK/atoms" > "$WORK/grouped" && mv "$WORK/grouped" "$WORK/atoms"
# replay: forced observes -> re-show, names/is-a -> re-tell, skip companions
fobs(){ local i; for i in 1 2 3; do $GOLEM observe --force vision "$1" >/dev/null 2>&1 && return 0; sleep 1; done; }
sayw(){ local i; for i in 1 2 3; do $GOLEM tell "$1" >/dev/null 2>&1 && return 0; sleep 1; done; }
saytr(){ local i; for i in 1 2 3; do $GOLEM tell --lang tr "$1" >/dev/null 2>&1 && return 0; sleep 1; done; }
nap(){ $GOLEM sleep >/dev/null 2>&1; sleep 2; }
replay() {
  local n=0 prev="" shown=""
  while IFS= read -r call; do
    local line=""
    case "$call" in
      "observe --force vision "*) line="fobs ${call#observe --force vision }" ;;
      "observe vision "*) continue ;;
      "tell --lang tr "*) local t="${call#tell --lang tr }"
        # A bare Turkish naming, the other tongue's half of the spine. Dropping it
        # rehearses the English names while the Turkish ones fade, which quietly
        # unpicks the cross-tongue bond a grow spent its whole length building.
        [ "$(printf '%s' "$t" | wc -w)" -le 2 ] || continue
        line="saytr $t" ;;
      "tell "*) local a="${call#tell }"; case "$a" in --*) continue;; esac
        case " $a " in *" is a "*|*" is an "*) : ;; *) [ "$(printf '%s' "$a" | wc -w)" -le 2 ] || continue ;; esac
        line="sayw $a" ;;
      *) continue ;;
    esac
    [ "$line" = "$prev" ] && continue; prev="$line"
    case "$line" in
      fobs*) [ "$line" = "$shown" ] || sleep "$PAUSE"; shown="$line" ;;
    esac
    ${line%% *} "${line#* }"
    n=$((n+1)); [ $((n % STEP)) -eq 0 ] && nap
  done < "$WORK/atoms"
  nap
}
$GOLEM status >/dev/null 2>&1 || { echo "daemon unreachable"; exit 1; }
# Nap interval scales with corpus size: aim for ~40 naps per round. Napping too
# often (a fixed small interval over a large corpus) erodes the categories shown
# early in a round before it ends, so a broad spine needs a wider stride.
TOTAL=$(wc -l < "$WORK/atoms"); STEP=$((TOTAL / 40)); [ "$STEP" -lt 150 ] && STEP=150
echo "rehearsing spine over $TOTAL atoms, two rounds, nap every $STEP, ${PAUSE}s at each change of percept"
replay; replay
echo "spine rehearsal complete"
