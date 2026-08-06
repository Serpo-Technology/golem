#!/usr/bin/env python3
"""Check the teaching scenes for the corpus faults that break learning.

These are not style nits. Each one has cost a re-grow at least once, and none of
them shows up as an error at teaching time: the lesson runs clean and the damage
only appears nights later, in a category that will not form or a noun that
resolves to its neighbour.

    python3 teaching/lint-scenes.py            # every scene
    python3 teaching/lint-scenes.py --scenes menagerie,flora
    python3 teaching/lint-scenes.py --quiet    # counts only, exit 1 if anything

What it looks for:

CAPTION-HEAD    A percept captioned with its own class as the head noun ("a small
                blue bird") is read as a basic-level NAME of that percept, so the
                class word never distils and its is-a resolves to a sibling.
                Caption by species, and keep the class word for the naming level.

MODIFIER        A taught word used as a modifier inside another percept's caption
                ("a green oak leaf") gives that word company with a percept it does
                not name, which is what smears a name across the room.

DUPLICATE       A caption named two different things, so neither name can settle on it.
                The looser form, one name over several captions of the same thing, is
                reported too: it splits the word's pointing, which is tolerable but is
                worth knowing before a grow.

TALK-FIRST      A word the corpus talks about BEFORE it shows anything mints a
                perceptless placeholder, self-named above what a grounded reading can
                earn, which forecloses its category for the rest of the grow. Order is
                the whole of it, so the scenes are walked in teaching order.
"""

import argparse
import importlib.util
import os
import re
import sys
from collections import defaultdict

REPO = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
TEACHING = os.path.join(REPO, "teaching")

_spec = importlib.util.spec_from_file_location("scoreisa", os.path.join(TEACHING, "score-isa.py"))
_s = importlib.util.module_from_spec(_spec)
_spec.loader.exec_module(_s)
capture, head, norm, OBSERVE, TELL, ISA = _s.capture, _s.head, _s.norm, _s.OBSERVE, _s.TELL, _s.ISA

STOP = {"a", "an", "the", "of", "and", "in", "on", "with", "is", "are", "it", "its"}


def read(scenes):
    """Replay the corpus and record what was shown, what was said, and in what order."""
    captions = {}                       # caption -> [words said over it]
    order = []                          # (kind, payload) in corpus order
    current = None
    for call in capture(scenes):
        m = OBSERVE.match(call)
        if m:
            current = norm(m.group(1))
            captions.setdefault(current, [])
            order.append(("show", current))
            continue
        m = TELL.match(call)
        if not m:
            continue
        lang, text = m.group(1) or "en", m.group(2).strip()
        if current and len(text.split()) <= 2 and lang == "en":
            w = head(text)
            captions[current].append(w)
            order.append(("say", (current, w)))
            continue
        # Every other telling is a clause ABOUT things. It is where a placeholder gets
        # minted, whether the clause states a kind ("a ball is a toy") or an event
        # ("a cat chased a bird"), so both have to be walked.
        if lang == "en":
            order.append(("utter", text.lower()))
    return captions, order


def lint(captions, order):
    """Return the faults, each as (check, detail) pairs grouped by check."""
    # A percept is shown several times over and named on every showing, while its class
    # is said on only one or two of them. So the name is the word said most often over
    # it, and everything else said over it is a class. A caption with nothing said over
    # it is a companion, shown only to keep the channel varied, and teaches nothing.
    classes = set()
    names = {}
    capName = {}
    for cap, words in captions.items():
        if not words:
            continue
        name = max(set(words), key=lambda w: (words.count(w), -words.index(w)))
        capName[cap] = name
        names.setdefault(name, set()).add(cap)
        classes.update(w for w in words if w != name)
    # A noun that names a percept of its own, which is what makes it hazardous inside
    # another percept's caption: the word gains company with a thing it does not name.
    nouns = {n for n, caps in names.items() if any(head(c) == n for c in caps)}

    out = defaultdict(list)

    for cap, words in captions.items():
        if not words:
            continue
        h = head(cap)
        # The rule is about a percept captioned by its OWN class, which is what makes
        # the naming pass read the class word as this percept's basic-level name.
        if h != capName[cap] and h in set(words):
            out["CAPTION-HEAD"].append(f"{cap!r} is headed by its own class {h!r}")
        for tok in re.sub(r"[^a-z ]", "", cap.lower()).split()[:-1]:
            # An adjective taught by contrast is MEANT to appear as a modifier; a noun
            # that names a percept of its own is not.
            if tok in STOP or tok == h or tok == capName[cap]:
                continue
            if tok in nouns:
                out["MODIFIER"].append(f"{cap!r} uses {tok!r}, which names a percept of its own")

    for name, caps in sorted(names.items()):
        if len(caps) > 1:
            out["DUPLICATE"].append(f"{name!r} is taught over {len(caps)} captions: "
                                    + ", ".join(sorted(caps)))
    # The harmful direction: one caption standing for two different things, so neither
    # name can settle on it. One name over several captions merely spreads its pointing
    # over percepts that are all genuinely its own.
    for cap, words in sorted(captions.items()):
        if not words:
            continue
        rivals = {w for w in words if w != capName[cap] and w not in classes}
        if rivals:
            out["DUPLICATE"].append(f"{cap!r} is named both {capName[cap]!r} and {sorted(rivals)}")

    # A noun the corpus only ever talks about, never shows, mints a perceptless node
    # that the grounded reading can then never outrank. Asked of the whole corpus
    # rather than of the running order, since the scenes carry no fixed sequence.
    # A word first MET in a telling mints a perceptless placeholder, self-named above
    # what any grounded reading can earn, and that placeholder then blocks its category
    # from ever forming. So the order matters, not just the totals: a word the corpus
    # shows at scene 9 but talks about at scene 2 is already lost by the time it is
    # shown. This walks the corpus in teaching order for exactly that reason.
    grounded = set()
    flagged = set()
    everShown = {head(c) for c in captions} | set(names) | classes
    for kind, payload in order:
        if kind == "show":
            grounded.add(head(payload))
        elif kind == "say":
            grounded.add(payload[1])
        elif kind == "utter":
            for tok in re.sub(r"[^a-z ]", "", payload).split():
                if tok in STOP or tok in grounded or tok in flagged:
                    continue
                if tok not in everShown:
                    continue    # a wholly perceptless vocabulary is taught by talk on purpose
                flagged.add(tok)
                out["TALK-FIRST"].append(
                    f"{tok!r} is talked about before anything shows it: {payload!r}")
    return out


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--scenes", default="")
    ap.add_argument("--quiet", action="store_true")
    ap.add_argument("--check", default="", help="only this check")
    args = ap.parse_args()
    # Teaching order, never alphabetical: this check is about what precedes what.
    order_file = os.path.join(TEACHING, "curriculum-order.txt")
    default = [l.strip() for l in open(order_file)] if os.path.exists(order_file) else \
              sorted(f[:-4] for f in os.listdir(TEACHING) if f.endswith(".zsh"))
    scenes = [s for s in args.scenes.split(",") if s] or [s for s in default if s]
    captions, order = read(scenes)
    out = lint(captions, order)
    total = sum(len(v) for v in out.values())
    print(f"{len(scenes)} scenes, {len(captions)} captions")
    for check in ("CAPTION-HEAD", "MODIFIER", "DUPLICATE", "TALK-FIRST"):
        hits = out.get(check, [])
        print(f"  {check:13} {len(hits)}")
        if args.quiet or (args.check and args.check != check):
            continue
        for h in hits:
            print(f"      {h}")
    return 1 if total else 0


if __name__ == "__main__":
    sys.exit(main())
