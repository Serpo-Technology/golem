#!/usr/bin/env python3
"""Ask a brain the whole question set and score what it says.

    python3 teaching/run-probes.py <image> <brain.db> <config.json> [probes.tsv]

A firm answer and a hedge are counted apart rather than blended. A brain that hedges
everything is not the same as one that answers, and a single accuracy number hides which
of the two you have.
"""
import json, os, re, subprocess, sys, tempfile, time

HERE = os.path.dirname(os.path.abspath(__file__))
args = [a for a in sys.argv[1:] if not a.startswith("--")]
IMAGE, DB, CFG = args[0], args[1], args[2]
PROBES = args[3] if len(args) > 3 else os.path.join(HERE, "probes.tsv")
NAME = "probeset-%d" % os.getpid()


def sh(*a, **k):
    return subprocess.run(a, capture_output=True, text=True, **k)


def probeConfig():
    """The given config with sleeping held off, on BOTH the gates that start a night.

    A run of two hundred questions takes about as long as the gap between nights, so the
    daemon naps in the middle of it and every question asked while it sleeps comes back
    with nothing. That is the harness answering, not the brain: the same questions asked
    one at a time all answer.

    The interval is not the only way a night starts. Sleep is also pressure driven, so a
    long run builds enough of it to nap however far out the interval is pushed. A run
    reporting 25 firm answers and 178 unknowns is exactly that: the names group answered,
    the daemon went under, and everything after came back empty. It looks like catastrophic
    forgetting and it is the instrument. Hold both gates and what the brain does not know
    is told apart from what it was too busy to say.
    """
    with open(CFG) as f:
        cfg = json.load(f)
    cfg["sleep_interval_sec"] = 999999
    # Serotonin runs 0 to 1, so a gate above 1 is never reached, and a calm window longer
    # than the run means the pressure never starts building in the first place.
    cfg["sleep_pressure_gate"] = 99.0
    cfg["calm_before_sleep_sec"] = 999999
    cfg["input_silence_sec"] = 999999
    fd, path = tempfile.mkstemp(suffix=".json")
    with os.fdopen(fd, "w") as f:
        json.dump(cfg, f)
    return path


def up():
    sh("docker", "rm", "-f", NAME)
    sh("docker", "run", "-d", "--name", NAME, "-v", f"{probeConfig()}:/root/.golem/config.json:ro",
       IMAGE, "sleep", "infinity")
    sh("docker", "cp", os.path.abspath(DB), f"{NAME}:/root/.golem/memory.db")
    sh("docker", "exec", "-d", NAME, "golem", "run")
    for _ in range(60):
        if sh("docker", "exec", NAME, "golem", "status").returncode == 0:
            return True
        time.sleep(1)
    return False


def ask(q):
    r = sh("docker", "exec", NAME, "golem", "tell", q)
    for line in (r.stdout + r.stderr).splitlines():
        if "golem>" in line:
            return line.split("golem>", 1)[1].strip()
    return ""


SUBJ = re.compile(r"^(?:yes|no|probably|perhaps|maybe)[,.]?\s+(?:an?\s+|the\s+)?([a-z]+)", re.I)


def subject_of(text):
    """The noun the answer is actually about, which is not always the one asked about."""
    m = SUBJ.match(text.strip())
    return m.group(1).lower() if m else ""


def asked_about(q):
    """The noun the question is about: 'is a bat a mammal' -> bat."""
    parts = q.split()
    if parts and parts[0] == "is":
        parts = parts[1:]
    if parts and parts[0] in ("a", "an", "the"):
        parts = parts[1:]
    return parts[0].lower() if parts else ""


def verdict(answer):
    """Firm yes, hedged yes, firm no, hedged no, an owned ignorance, or no answer at all."""
    a = answer.lower()
    # Saying so is not silence. A brain that has never been shown the thing and says as much
    # is the honesty rail working, and counting it beside a wrong answer hides which of the
    # two you are looking at.
    if a.startswith("i do not know") or a.startswith("i don't know"):
        return "unknown"
    if a.startswith("yes"):
        return "yes"
    if a.startswith("probably") or a.startswith("perhaps") or a.startswith("maybe"):
        return "probably-no" if " not " in a[:40] else "probably"
    if a.startswith("no"):
        return "no"
    return "none"


def main():
    if not up():
        print("daemon did not come up", file=sys.stderr)
        return 2
    rows = [l.rstrip("\n").split("\t") for l in open(PROBES)
            if l.strip() and not l.startswith("#")]
    tally, wrong = {}, []
    for kind, q, expect, *rest in rows:
        ans = ask(q)
        v = verdict(ans)
        t = tally.setdefault(kind, {"firm": 0, "hedged": 0, "wrong": 0, "silent": 0, "n": 0, "misread": 0, "unknown": 0})
        t["n"] += 1
        if kind == "names":
            ok = any(w and w in ans.lower() for w in expect.split("|"))
            t["firm" if ok else "wrong"] += 1
            if not ok:
                wrong.append((q, expect, ans))
        elif expect == "yes":
            if v == "yes":
                t["firm"] += 1
            elif v == "probably":
                t["hedged"] += 1
            elif v == "unknown":
                t["unknown"] += 1
            elif v == "none":
                t["silent"] += 1
            else:
                t["wrong"] += 1
                misread = subject_of(ans) and subject_of(ans) != asked_about(q)
                t["misread"] = t.get("misread", 0) + (1 if misread else 0)
                wrong.append((q, expect, ans))
        else:
            if v in ("no", "probably-no"):
                t["firm" if v == "no" else "hedged"] += 1
            elif v == "unknown":
                t["unknown"] += 1
            elif v == "none":
                t["silent"] += 1
            else:
                t["wrong"] += 1
                wrong.append((q, expect, ans))
    sh("docker", "rm", "-f", NAME)

    print(f"{os.path.basename(DB)}  under {os.path.basename(CFG)}")
    print(f"{'group':12} {'n':>4} {'firm':>6} {'hedged':>7} {'wrong':>6} {'misread':>8} {'unknown':>8} {'silent':>7}  {'firm+hedged':>12}")
    tot = {"firm": 0, "hedged": 0, "wrong": 0, "silent": 0, "n": 0, "misread": 0, "unknown": 0}
    for kind in ("isa-yes", "isa-no", "names", "regression"):
        t = tally.get(kind)
        if not t:
            continue
        for k in tot:
            tot[k] += t[k]
        acc = (t["firm"] + t["hedged"]) / t["n"] if t["n"] else 0
        print(f"{kind:12} {t['n']:4} {t['firm']:6} {t['hedged']:7} {t['wrong']:6} {t.get('misread', 0):8} {t.get('unknown', 0):8} {t['silent']:7}  {acc:12.3f}")
    acc = (tot["firm"] + tot["hedged"]) / tot["n"] if tot["n"] else 0
    print(f"{'ALL':12} {tot['n']:4} {tot['firm']:6} {tot['hedged']:7} {tot['wrong']:6} {tot['misread']:8} {tot['unknown']:8} {tot['silent']:7}  {acc:12.3f}")
    if tot["wrong"]:
        print(f"\nof {tot['wrong']} wrong answers, {tot['misread']} are about a different word "
              f"than the one asked about, so they are a failure to settle the word rather than "
              f"a failure to reason.")
    if "--wrong" in sys.argv:
        print("\nwrong answers:")
        for q, e, a in wrong[:40]:
            print(f"  {q}\n    expected {e}, got {a!r}")
    return 0


sys.exit(main())
