#!/usr/bin/env python3
"""Build the question set a brain is judged by, from what the corpus actually teaches.

Seven questions decide nothing. Two runs of one config disagreed on one of them, so the set
resolved about a single item, and a flag was kept or dropped on a three-against-three split.
This writes a larger set with its answers derived from the corpus rather than invented, so a
run can be scored the same way twice and two runs can be told apart.

A positive is a membership the corpus teaches, directly or up the chain. A negative has to be
false in the WORLD and not merely absent from the corpus: a box IS a container whatever the
scenes left unsaid, and scoring that as an error taught me the difference. So a negative is
only drawn between two categories that cannot both hold, and never against `thing`, which
everything is.

    python3 teaching/make-probes.py > teaching/probes.tsv
"""
import json, os, sys

HERE = os.path.dirname(os.path.abspath(__file__))
truth = json.load(open(os.path.join(HERE, "isa-truth.json")))
kinds = truth["kinds"]
spoken = truth.get("spoken", {})

# Categories that cannot both hold of one thing. A negative is drawn only across this list,
# which is why it is written out by hand rather than derived: the corpus does not record what
# it never says, and absence is not exclusion.
EXCLUSIVE = [
    "animal", "plant", "material", "quality", "weather", "landform", "tool", "vehicle",
    "building", "clothing", "instrument", "furniture", "shape", "colour", "bird", "fish",
    "mammal", "insect", "reptile", "flower", "tree", "vegetable", "fruit", "gem", "drink",
]


def closure(word, seen=None):
    seen = set() if seen is None else seen
    for up in kinds.get(word) or []:
        if up not in seen:
            seen.add(up)
            closure(up, seen)
    return seen


# Members of these are proper: a river is called the Amazon, never an amazon, and a question
# that puts an article in front of one is a question about a word the corpus never said.
PROPER = {"city", "river", "mountain", "sea", "ocean", "continent", "desert", "country", "lake"}
# Mass and abstract words take no article: a thing is not "a weather" and not "a brown".
MASS = {"weather", "clothing", "furniture", "material", "food", "equipment", "storage",
        "colour", "quality", "state", "art"}
# Abstract on the member side too: brown IS a colour, while a ball is merely equipment and
# is still "a ball". Only these carry bareness down to their members.
ABSTRACT = {"colour", "quality", "state", "art"}
# "a utensil", not "an utensil": the letter is a vowel and the sound is not.
CONSONANT_U = {"utensil", "uniform", "unicorn", "university", "union"}


def named(w):
    """How the word is asked after: bare when it names one particular thing, or none."""
    if (closure(w) & PROPER) or (set(kinds.get(w) or []) & ABSTRACT) or w in MASS:
        return w
    return art(w) + w


def art(w):
    if w in CONSONANT_U or w[:1] not in "aeiou":
        return "a "
    return "an "


def asked(cat):
    """How the category is asked after, on the far side of the question."""
    if cat in MASS:
        return cat
    return art(cat) + cat


rows = []


def add(kind, q, expect, note=""):
    rows.append((kind, q, expect, note))


# Positives: every membership the corpus teaches, direct and inherited, over a spread of
# members rather than a run of birds.
members = sorted(w for w in kinds if closure(w))
seen_cat = {}
for w in members:
    for cat in sorted(closure(w)):
        if cat == "thing":
            continue                      # true of everything, so it separates nothing
        # A ball is a sphere, a sphere is a shape and a shape is a quality, but a ball is
        # not a quality: it HAS one. The chain is sound at each step and false end to end,
        # so the abstract tops are asked about only where the corpus names them directly.
        if cat in ABSTRACT and cat not in (kinds.get(w) or []):
            continue
        if seen_cat.get(cat, 0) >= 3:
            continue                      # cap per category so one branch cannot dominate
        seen_cat[cat] = seen_cat.get(cat, 0) + 1
        add("isa-yes", f"is {named(w)} {asked(cat)}", "yes")

# Negatives: across categories that exclude one another.
# Strided, so the negatives are not all drawn from the front of the alphabet.
neg = 0
stride = max(1, len(members) // 70)
for i, w in enumerate(members[::stride] + members):
    cl = closure(w)
    if not cl:
        continue
    # Rotate the starting point, or every negative comes out asking about an animal.
    ring = EXCLUSIVE[i % len(EXCLUSIVE):] + EXCLUSIVE[:i % len(EXCLUSIVE)]
    for cat in ring:
        if cat in cl or w == cat:
            continue
        if w in closure(cat):
            continue
        # Only when the member sits under some exclusive category itself, so the pair is
        # genuinely opposed rather than merely unrelated.
        if not (cl & set(EXCLUSIVE)):
            continue
        add("isa-no", f"is {named(w)} {asked(cat)}", "no")
        neg += 1
        break
    if neg >= 60:
        break

# What a thing is called: the caption was shown and these words were said over it.
for caption, words in sorted(spoken.get("en", {}).items())[:200:8]:
    if words:
        add("names", f"what is {caption}", "|".join(sorted(words)))

# The ones already known to be wrong, kept so a fix shows up as a change of answer.
add("regression", "is a person an animal", "yes", "taught, answered no since the handoff")
add("regression", "does a person show courage", "yes", "taught, answered no since the handoff")

out = sys.stdout
out.write("# kind\tquestion\texpect\tnote\n")
for kind, q, expect, note in rows:
    out.write(f"{kind}\t{q}\t{expect}\t{note}\n")
sys.stderr.write(f"{len(rows)} probes: "
                 + ", ".join(f"{k}={sum(1 for r in rows if r[0] == k)}"
                             for k in ("isa-yes", "isa-no", "names", "regression")) + "\n")
