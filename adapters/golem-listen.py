#!/usr/bin/env python3
"""Give GOLEM ears, or a console. Read lines from stdin, send each to the brain
over TCP, and print the reply.

By default a line is a told utterance; with --observe it is a percept on a named
channel (a captioner's output, say). The brain is platform-agnostic: it takes
text. This adapter is the platform-specific edge that turns a microphone, a
captioner, or a keyboard into that text.

    # type at it, or pipe a speech recognizer in for ears:
    python3 golem-listen.py [--addr 127.0.0.1:7777] [--lang tr]
    whisper-stream | python3 golem-listen.py --lang en

    # feed a captioner as vision percepts:
    captioner | python3 golem-listen.py --observe vision
"""
import argparse
import json
import socket
import sys


def call(addr, req):
    host, port = addr.rsplit(":", 1)
    with socket.create_connection((host, int(port)), timeout=120) as sock:
        sock.sendall((json.dumps(req) + "\n").encode())
        return json.loads(sock.makefile("rb").readline())


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--addr", default="127.0.0.1:7777", help="brain TCP address")
    ap.add_argument("--lang", default="", help="language tag for a told utterance (e.g. tr)")
    ap.add_argument("--observe", default="", help="send each line as an observation on this channel")
    args = ap.parse_args()

    for line in sys.stdin:
        text = line.strip()
        if not text:
            continue
        if args.observe:
            req = {"cmd": "observe", "channel": args.observe, "content": text}
        else:
            req = {"cmd": "tell", "content": text}
            if args.lang:
                req["lang"] = args.lang
        try:
            resp = call(args.addr, req)
        except OSError as err:
            print("error:", err, file=sys.stderr)
            continue
        if resp.get("error"):
            print("error:", resp["error"], file=sys.stderr)
        elif resp.get("say"):
            print(resp["say"], flush=True)


if __name__ == "__main__":
    main()
