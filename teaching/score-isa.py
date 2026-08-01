#!/usr/bin/env python3
"""Score a grown brain's is-a spine against what the corpus actually taught.

Grounding COUNTS are a misleading gauge: a smeared category word grows more
grounding edges, not fewer, while the members it grounds stop being its members.
So this measures precision and recall against ground truth read out of the
teaching scripts themselves.

Ground truth is extracted by DRY-RUNNING each scene through a mock `golem` that
records the calls, the same trick rehearse-spine.sh uses. The scripts define
`teach`, `kindof`, `fact`, `bi` and friends about twenty different ways, so
reading the call stream is the only faithful way to see what was taught:

    observe [--force] vision <caption>      the percept
    tell <word>                             said over that percept
    tell a <x> is a <y>                     an is-a edge, whatever helper wrote it

A kind is taught by SAYING it over the percept, not by asserting it: teach() shows
"a heavy steel hammer" and says "hammer", then "handtool", then "tool" across the
showings. So the words spoken over a caption are its name and its categories
together, and the is-a facts the corpus also states extend them. A member belongs
to category C when C is among those words or reachable from one of them.
Everything else grounded under C kept the word's company without being its member.

    python3 teaching/score-isa.py <brain.db> [<brain.db> ...]
    python3 teaching/score-isa.py --wrong 20 <brain.db>     # list the worst edges

Truth is cached in teaching/isa-truth.json; pass --rebuild after changing scenes.
"""

import argparse
import json
import os
import re
import sqlite3
import subprocess
import sys
import tempfile

REPO = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
TEACHING = os.path.join(REPO, "teaching")
TRUTH = os.path.join(TEACHING, "isa-truth.json")

ARTICLE = re.compile(r"^(?:a|an|the)\s+", re.I)
ISA = re.compile(r"^(?:(?:an|a|the)\s+)?(.+?)\s+(?:is|are)\s+(?:an|a|the)\s+(.+?)\.?$", re.I)
OBSERVE = re.compile(r"^observe\s+(?:--force\s+)?vision\s+(.+)$")
TELL = re.compile(r"^tell\s+(?:--lang\s+(\w+)\s+)?(.+)$")


def head(phrase):
    """The head noun of a caption or phrase, which is what a word names."""
    phrase = ARTICLE.sub("", phrase.strip().lower()).strip(" .")
    parts = phrase.split()
    return parts[-1] if parts else ""


def norm(caption):
    """Match the store's own caption normalisation closely enough to join on it."""
    return re.sub(r"\s+", " ", caption.strip().lower())


def capture(scenes):
    """Dry-run each scene through a recording mock and return the call stream."""
    calls = []
    with tempfile.TemporaryDirectory() as work:
        cap = os.path.join(work, "cap")
        golem = os.path.join(work, "golem")
        with open(golem, "w") as f:
            f.write('#!/bin/sh\nprintf "%s\\n" "$*" >> "$CAPTURE"\nexit 0\n')
        slp = os.path.join(work, "sleep")
        with open(slp, "w") as f:
            f.write("#!/bin/sh\nexit 0\n")
        for p in (golem, slp):
            os.chmod(p, 0o755)
        env = dict(os.environ, PATH=work + os.pathsep + os.environ["PATH"],
                   CAPTURE=cap, GOLEM="golem", GAP="0", BLOCKGAP="0")
        for scene in scenes:
            path = os.path.join(TEACHING, scene + ".zsh")
            if not os.path.exists(path):
                continue
            open(cap, "w").close()
            subprocess.run(["zsh", path], env=env, stdout=subprocess.DEVNULL,
                           stderr=subprocess.DEVNULL, timeout=600)
            with open(cap) as f:
                calls.extend(line.rstrip("\n") for line in f)
    return calls


def build_truth(scenes):
    """Read the call stream into names per caption and a kind graph."""
    spoken = {}           # caption -> the words said over it: its name and its kinds
    kinds = {}            # word -> set of words it was told to be a kind of
    current = None
    for call in capture(scenes):
        m = OBSERVE.match(call)
        if m:
            current = norm(m.group(1))
            spoken.setdefault(current, set())
            continue
        m = TELL.match(call)
        if not m:
            continue
        lang, text = m.group(1) or "en", m.group(2).strip()
        isa = ISA.match(text)
        if isa:
            sub, obj = head(isa.group(1)), head(isa.group(2))
            if sub and obj and sub != obj:
                kinds.setdefault(sub, set()).add(obj)
            continue
        # A short telling right after a showing is a name for what was shown.
        if current and len(text.split()) <= 2 and lang == "en":
            spoken[current].add(head(text))
    return {"spoken": {k: sorted(v) for k, v in spoken.items() if v},
            "kinds": {k: sorted(v) for k, v in kinds.items()}}


def closure(word, kinds, seen=None):
    """Every category a word reaches by taught is-a edges, itself included."""
    seen = seen if seen is not None else set()
    if word in seen:
        return seen
    seen.add(word)
    for up in kinds.get(word, ()):
        closure(up, kinds, seen)
    return seen


def ancestors(truth):
    """caption -> every category it legitimately belongs to, its own name included."""
    spoken, kinds = truth["spoken"], truth["kinds"]
    out = {}
    for caption, ns in spoken.items():
        acc = set()
        for n in ns:
            acc |= closure(n, kinds)
        out[caption] = acc
    return out


def load_truth(scenes, rebuild):
    if not rebuild and os.path.exists(TRUTH):
        with open(TRUTH) as f:
            return json.load(f)
    truth = build_truth(scenes)
    with open(TRUTH, "w") as f:
        json.dump(truth, f, indent=1, sort_keys=True)
    return truth


def score(db, truth, anc, worst):
    con = sqlite3.connect("file:" + db + "?mode=ro", uri=True)
    edges = con.execute("""
        SELECT a.template, b.template FROM links l
        JOIN fragments a ON a.id = l.from_id AND a.template LIKE 'abs:%'
        JOIN fragments b ON b.id = l.to_id
        WHERE l.rel = 'grounds'""").fetchall()
    kinds = truth["kinds"]
    ok = bad = unjudged = 0
    wrong = []
    for cat, member in edges:
        parts = cat.split(":", 2)
        if len(parts) != 3:
            continue
        lemma = parts[2]
        if member.startswith("abs:"):
            sub = member.split(":", 2)[-1]
            reach = closure(sub, kinds) if sub in kinds or sub in anc_words(anc) else None
        else:
            reach = anc.get(norm(member))
        if not reach:
            unjudged += 1
            continue
        if lemma in reach:
            ok += 1
        else:
            bad += 1
            wrong.append((lemma, member))
    # Recall: of the memberships this brain could express, how many did it lay?
    have = set(r[0].split(":", 2)[2] for r in con.execute(
        "SELECT template FROM fragments WHERE template LIKE 'abs:en:%'"))
    laid = set((c.split(":", 2)[2], norm(m)) for c, m in edges if not m.startswith("abs:"))
    gold = set()
    for caption, cats in anc.items():
        for c in cats:
            if c in have:
                gold.add((c, caption))
    found = len(gold & laid)
    # A percept concept nothing names is a concept the brain can no longer talk about.
    nameless = con.execute("""
        SELECT COUNT(*) FROM fragments c
        WHERE c.kind = 'concept' AND c.template NOT LIKE 'abs:%'
          AND c.template NOT LIKE 'noun:%'
          AND EXISTS (SELECT 1 FROM links g WHERE g.from_id = c.id AND g.rel = 'grounds')
          AND NOT EXISTS (SELECT 1 FROM links n WHERE n.to_id = c.id
                          AND n.rel = 'names' AND n.weight >= 0.1)""").fetchone()[0]
    con.close()
    judged = ok + bad
    return {"edges": len(edges), "judged": judged, "unjudged": unjudged,
            "correct": ok, "wrong": bad,
            "precision": ok / judged if judged else 0.0,
            "recall": found / len(gold) if gold else 0.0,
            "gold": len(gold), "nameless": nameless,
            "worst": wrong[:worst]}


def anc_words(anc):
    out = set()
    for cats in anc.values():
        out |= cats
    return out


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("brains", nargs="+")
    ap.add_argument("--rebuild", action="store_true", help="re-read the scenes")
    ap.add_argument("--wrong", type=int, default=0, help="list N wrong edges")
    ap.add_argument("--scenes", default="", help="comma separated, default every scene")
    args = ap.parse_args()

    scenes = ([s for s in args.scenes.split(",") if s] or
              sorted(f[:-4] for f in os.listdir(TEACHING) if f.endswith(".zsh")))
    truth = load_truth(scenes, args.rebuild)
    anc = ancestors(truth)
    print(f"truth: {len(truth['spoken'])} captions, {len(truth['kinds'])} words with kinds "
          f"({os.path.relpath(TRUTH, REPO)})")
    for db in args.brains:
        r = score(db, truth, anc, args.wrong)
        print(f"\n{os.path.basename(db)}")
        print(f"  grounding edges {r['edges']:5}   judged {r['judged']:5}   unjudged {r['unjudged']:4}")
        print(f"  precision       {r['precision']:.4f}  ({r['correct']} correct, {r['wrong']} wrong)")
        print(f"  recall          {r['recall']:.4f}  ({r['gold']} memberships the brain could express)")
        print(f"  nameless concepts {r['nameless']}")
        for lemma, member in r["worst"]:
            print(f"    wrong: abs:en:{lemma} grounds {member}")


if __name__ == "__main__":
    sys.exit(main())
