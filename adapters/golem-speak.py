#!/usr/bin/env python3
"""Voice what GOLEM says. Subscribe to the brain's event stream over TCP and
render each "say" event through whatever the host speaks with.

The brain is platform-agnostic: it only emits the sentence as a "say" event.
This adapter is the platform-specific edge. The text-to-speech command is
auto-detected (say on macOS, espeak / spd-say on Linux) or given with --cmd,
so the same brain speaks on any machine by swapping the adapter, not the brain.

    python3 golem-speak.py [--addr 127.0.0.1:7777] [--cmd "espeak"]
"""
import argparse
import json
import os
import shutil
import socket
import subprocess
import sys


def detect_tts():
    for cmd in (["say"], ["espeak"], ["spd-say", "-w"], ["festival", "--tts"]):
        if shutil.which(cmd[0]):
            return cmd
    return None


def sign(req):
    """Sign a request through the golem CLI (reads GOLEM_PRIVATE_KEY). Even a
    subscription is a request over the network transport and must be signed;
    GOLEM_BIN points at the binary when it is not on PATH."""
    binary = os.environ.get("GOLEM_BIN", "golem")
    out = subprocess.run([binary, "sign"], input=json.dumps(req).encode(),
                         capture_output=True)
    if out.returncode != 0:
        raise OSError(out.stderr.decode().strip() or "golem sign failed")
    return out.stdout.strip()


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--addr", default="127.0.0.1:7777", help="brain TCP address")
    ap.add_argument("--cmd", default="", help="text-to-speech command; auto-detected when empty")
    args = ap.parse_args()

    tts = args.cmd.split() if args.cmd else detect_tts()
    if not tts:
        sys.exit("no text-to-speech command found; install one or pass --cmd")

    try:
        signed_subscribe = sign({"cmd": "subscribe"}) + b"\n"
    except OSError as err:
        sys.exit("cannot sign the subscription: " + str(err) +
                 "\n(set GOLEM_PRIVATE_KEY and put golem on PATH, or GOLEM_BIN)")

    host, port = args.addr.rsplit(":", 1)
    with socket.create_connection((host, int(port))) as sock:
        sock.sendall(signed_subscribe)
        for line in sock.makefile("rb"):
            try:
                ev = json.loads(line)
            except json.JSONDecodeError:
                continue
            if ev.get("ev") == "say" and ev.get("tpl"):
                text = ev["tpl"]
                print("say:", text, flush=True)
                subprocess.run(tts + [text])


if __name__ == "__main__":
    main()
