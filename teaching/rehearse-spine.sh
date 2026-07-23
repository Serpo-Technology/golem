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
#   GOLEM=golem bash teaching/rehearse-spine.sh <scene1> <scene2> ...
set -u
GOLEM=${GOLEM:-golem}
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
# replay: forced observes -> re-show, names/is-a -> re-tell, skip companions
fobs(){ local i; for i in 1 2 3; do $GOLEM observe --force vision "$1" >/dev/null 2>&1 && return 0; sleep 1; done; }
sayw(){ local i; for i in 1 2 3; do $GOLEM tell "$1" >/dev/null 2>&1 && return 0; sleep 1; done; }
nap(){ $GOLEM sleep >/dev/null 2>&1; sleep 2; }
replay() {
  local n=0 prev=""
  while IFS= read -r call; do
    local line=""
    case "$call" in
      "observe --force vision "*) line="fobs ${call#observe --force vision }" ;;
      "observe vision "*) continue ;;
      "tell --lang tr "*) continue ;;
      "tell "*) local a="${call#tell }"; case "$a" in --*) continue;; esac
        case " $a " in *" is a "*|*" is an "*) : ;; *) [ "$(printf '%s' "$a" | wc -w)" -le 2 ] || continue ;; esac
        line="sayw $a" ;;
      *) continue ;;
    esac
    [ "$line" = "$prev" ] && continue; prev="$line"
    ${line%% *} "${line#* }"
    n=$((n+1)); [ $((n % 150)) -eq 0 ] && nap
  done < "$WORK/atoms"
  nap
}
$GOLEM status >/dev/null 2>&1 || { echo "daemon unreachable"; exit 1; }
echo "rehearsing spine over $(wc -l < "$WORK/atoms") atoms, two rounds"
replay; replay
echo "spine rehearsal complete"
