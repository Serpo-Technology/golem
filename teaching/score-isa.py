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

# The bar the abstraction pass writes a grounding at, absPwcFloor in
# internal/daemon/concept.go. A rung that has faded under it is one the pass declined to
# refresh, and standingIsA will not climb it to reach a firm yes. Keep the two in step.
#
# COMPARE BRAINS AT THE SAME POINT IN THE NAP CYCLE. The engine admits a rung on a strict
# less-than, and the pass writes most of them at exactly the bar: on the deliverable 1087
# of 1598 sit on 0.1 to the digit. So a freshly written brain keeps all of them and a brain
# one nap older has lost every one, which moves this figure further than any change to the
# engine will. Scoring a saved snapshot against a napped one measures the nap.
ABS_PWC_FLOOR = 0.10

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
            r = subprocess.run(["zsh", path], env=env, stdout=subprocess.DEVNULL,
                               stderr=subprocess.DEVNULL, timeout=600)
            if r.returncode != 0:
                raise SystemExit(f"scene {scene} exited {r.returncode}; truth would be "
                                 f"silently short, which reads as higher precision")
            with open(cap) as f:
                calls.extend(line.rstrip("\n") for line in f)
    return calls


def build_truth(scenes):
    """Read the call stream into names per caption, per tongue, and a kind graph."""
    spoken = {}           # lang -> caption -> the words said over it: its name and its kinds
    kinds = {}            # word -> set of words it was told to be a kind of
    current = None
    for call in capture(scenes):
        m = OBSERVE.match(call)
        if m:
            current = norm(m.group(1))
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
        # A short telling right after a showing is a name for what was shown. Every
        # tongue names, so the truth is kept per tongue: judging a Turkish category
        # against English tellings counts a correct naming as a wrong edge.
        if current and len(text.split()) <= 2:
            spoken.setdefault(lang, {}).setdefault(current, set()).add(head(text))
    return {"spoken": {lg: {k: sorted(v) for k, v in caps.items() if v}
                       for lg, caps in spoken.items()},
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
    """lang -> caption -> every category it legitimately belongs to, its own name included.

    The kind graph is stated in English tellings, so only the English side climbs it. A
    tongue that merely names its percepts is judged on those namings alone, which is what
    it was actually taught."""
    kinds = truth["kinds"]
    out = {}
    for lang, caps in truth["spoken"].items():
        acc = {}
        for caption, ns in caps.items():
            reach = set()
            for n in ns:
                reach |= closure(n, kinds) if lang == "en" else {n}
            acc[caption] = reach
        out[lang] = acc
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
    """Score a brain two ways: per edge, and over the closure the engine climbs.

    The edge number is a diagnostic. The CLOSURE number is the one that governs an
    answer: planIsA asks isA, isA asks categoriesOf, and categoriesOf climbs the
    grounds edges TRANSITIVELY, so one wrong rung high in the graph hands every
    member below it a firm membership. Counting edges charges that rung once; the
    brain pays for it on every member it reaches. Score both, and judge on the second.
    """
    con = sqlite3.connect("file:" + db + "?mode=ro", uri=True)
    # A concept does not ground itself. A self loop is no membership, and counting one
    # as correct flatters every brain that carries them.
    rows = con.execute("""
        SELECT l.from_id, l.to_id, a.template, b.template, l.weight FROM links l
        JOIN fragments a ON a.id = l.from_id AND a.template LIKE 'abs:%'
        JOIN fragments b ON b.id = l.to_id
        WHERE l.rel = 'grounds' AND l.from_id <> l.to_id""").fetchall()
    kinds = truth["kinds"]

    def reachOf(member):
        """What the corpus says this member belongs to, judged in its own tongue."""
        if member.startswith("abs:"):
            parts = member.split(":", 2)
            if len(parts) != 3:
                return None
            sub = parts[2]
            if parts[1] != "en":
                return {sub}          # a tongue that names, judged on its namings
            return closure(sub, kinds) if sub in kinds or sub in anc_words(anc) else None
        return anc.get("en", {}).get(norm(member))

    def judge(lemma, lang, member):
        """None when the corpus says nothing about this member, else True or False."""
        if lang != "en":
            caps = anc.get(lang, {})
            if member.startswith("abs:"):
                return lemma == member.split(":", 2)[-1] or None
            r = caps.get(norm(member))
            return None if r is None else lemma in r
        r = reachOf(member)
        return None if not r else lemma in r

    ok = bad = unjudged = 0
    wrong = []
    for _, _, cat, member, _ in rows:
        parts = cat.split(":", 2)
        if len(parts) != 3:
            continue
        v = judge(parts[2], parts[1], member)
        if v is None:
            unjudged += 1
        elif v:
            ok += 1
        else:
            bad += 1
            wrong.append((parts[2], member))

    # The climb: every abstraction reachable by following grounds edges upward from the
    # member, deduped by first sight, start excluded. Each reached category is a firm
    # membership the brain asserts.
    #
    # Run it twice, because the engine no longer climbs the whole graph to reach a firm
    # yes. planIsA asks standingIsA, which skips any rung that has faded under the bar the
    # abstraction pass writes at, while inheritance, denial and the nightly derivations go
    # on reading every edge. The barred figure is the one an answer is judged on; the full
    # figure is what the rest of the engine still sees, and stays reported so a brain grown
    # before the bar existed remains comparable.
    tpl = {}
    for f, t, ct, mt, _ in rows:
        tpl[f], tpl[t] = ct, mt
    members = {t for _, t, _, _, _ in rows if not tpl[t].startswith("abs:")}

    def climb(minWeight):
        parents = {}
        for f, t, _, _, w in rows:
            if w >= minWeight:
                parents.setdefault(t, []).append(f)
        cok = cbad = 0
        worstRung = {}
        for m in members:
            r = reachOf(tpl[m])
            if not r:
                continue
            seen, stack = {m}, [m]
            while stack:
                cur = stack.pop()
                for p in parents.get(cur, ()):
                    if p in seen:
                        continue
                    seen.add(p)
                    stack.append(p)
                    pt = tpl[p]
                    parts = pt.split(":", 2)
                    if len(parts) != 3 or parts[1] != "en":
                        continue
                    if parts[2] in r:
                        cok += 1
                    else:
                        cbad += 1
                        worstRung[pt] = worstRung.get(pt, 0) + 1
        return cok, cbad, worstRung

    cok, cbad, worstRung = climb(ABS_PWC_FLOOR)
    fok, fbad, _ = climb(0.0)

    # Recall over every membership the CORPUS teaches, never filtered to the categories
    # this brain happens to hold: a brain that forgets a broad word must not score as
    # having improved.
    laid = set((c.split(":", 2)[2], norm(m)) for _, _, c, m, _ in rows
               if not m.startswith("abs:") and c.startswith("abs:en:"))
    gold = {(c, caption) for caption, cats in anc.get("en", {}).items() for c in cats}
    found = len(gold & laid)
    nameless = con.execute("""
        SELECT COUNT(*) FROM fragments c
        WHERE c.kind = 'concept' AND c.template NOT LIKE 'abs:%'
          AND c.template NOT LIKE 'noun:%'
          AND EXISTS (SELECT 1 FROM links g WHERE g.from_id = c.id AND g.rel = 'grounds')
          AND NOT EXISTS (SELECT 1 FROM links n WHERE n.to_id = c.id
                          AND n.rel = 'names' AND n.weight >= 0.1)""").fetchone()[0]
    con.close()
    judged = ok + bad
    cj = cok + cbad
    return {"edges": len(rows), "judged": judged, "unjudged": unjudged,
            "correct": ok, "wrong": bad,
            "precision": ok / judged if judged else 0.0,
            "closureJudged": cj, "closureCorrect": cok,
            "closure": cok / cj if cj else 0.0,
            "fullJudged": fok + fbad, "fullCorrect": fok,
            "full": fok / (fok + fbad) if fok + fbad else 0.0,
            "recall": found / len(gold) if gold else 0.0,
            "gold": len(gold), "nameless": nameless,
            "worstRung": sorted(worstRung.items(), key=lambda kv: -kv[1])[:5],
            "worst": wrong[:worst]}


def anc_words(anc):
    out = set()
    for caps in anc.values():
        for cats in caps.values():
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
    print(f"truth: {sum(len(c) for c in truth['spoken'].values())} captions, {len(truth['kinds'])} words with kinds "
          f"({os.path.relpath(TRUTH, REPO)})")
    for db in args.brains:
        r = score(db, truth, anc, args.wrong)
        print(f"\n{os.path.basename(db)}")
        print(f"  grounding edges {r['edges']:5}   judged {r['judged']:5}   unjudged {r['unjudged']:4}")
        print(f"  edge precision  {r['precision']:.4f}  ({r['correct']} correct, {r['wrong']} wrong)")
        print(f"  CLOSURE prec    {r['closure']:.4f}  ({r['closureCorrect']} of {r['closureJudged']} "
              f"memberships a firm answer actually asserts)")
        print(f"  closure, unbarred {r['full']:.4f}  ({r['fullCorrect']} of {r['fullJudged']} "
              f"over every rung, which inheritance and the derivations still read)")
        print(f"  recall          {r['recall']:.4f}  ({r['gold']} memberships the corpus teaches)")
        print(f"  nameless concepts {r['nameless']}")
        for rung, n in r.get("worstRung", []):
            print(f"    worst rung: {rung} hands out {n} wrong memberships")
        for lemma, member in r["worst"]:
            print(f"    wrong: abs:en:{lemma} grounds {member}")


if __name__ == "__main__":
    sys.exit(main())
