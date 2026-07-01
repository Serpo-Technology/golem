#!/usr/bin/env python3
"""Give GOLEM eyes. Turn a real image into a percept: a vision model describes it in
the plain register GOLEM grounds in, and the image is observed on the vision channel
with its raw bytes kept, so the brain reasons over the caption while the picture it was
distilled from stays on record.

The neural perception lives out here, in the transducer, so the brain stays a glass
box: it never sees pixels, only the caption and the features mined from it, and it can
look again at the stored image. Point this at image files (a folder, a camera frame, a
screenshot) and each becomes something GOLEM has seen.

    export ANTHROPIC_API_KEY=...            # the vision model this transducer runs
    python3 golem-see.py [--addr 127.0.0.1:7777] cat.jpg ball.png ...

The caption backend is swappable: replace caption() to run a local captioner, or feed
captions from a vision-capable model you task by hand. The daemon side is the same
either way, `observe --media`, so any front-end that speaks GOLEM's register works.
"""
import argparse
import base64
import json
import mimetypes
import os
import socket
import sys
import urllib.request

# The register the learner grounds in: a short caption, one or two plain adjectives and
# a head noun, the way its lessons are captioned ("a small black cat", "a round red
# ball"). Not a description, not a sentence, just what the thing is, so the percept
# crystallizes into a concept and resembles the ones already learned.
REGISTER = (
    "You are the eyes of a young learner that understands only short, plain captions. "
    "Describe the main thing in the image as a lowercase noun phrase of one or two "
    "simple adjectives and a head noun, like 'a small black cat' or 'a round red ball'. "
    "One thing only, no sentence, no punctuation, no explanation. Reply with the caption "
    "and nothing else."
)

CLAUDE_MODEL = "claude-sonnet-5"


def caption(image_bytes, mime):
    """Describe an image in the learner's register. Uses the Anthropic vision API; swap
    this out for a local model or a hand-tasked one without touching the rest."""
    key = os.environ.get("ANTHROPIC_API_KEY")
    if not key:
        raise SystemExit("set ANTHROPIC_API_KEY, or replace caption() with another backend")
    body = json.dumps({
        "model": CLAUDE_MODEL,
        "max_tokens": 32,
        "system": REGISTER,
        "messages": [{
            "role": "user",
            "content": [
                {"type": "image", "source": {
                    "type": "base64", "media_type": mime,
                    "data": base64.b64encode(image_bytes).decode()}},
                {"type": "text", "text": "What is this?"},
            ],
        }],
    }).encode()
    req = urllib.request.Request(
        "https://api.anthropic.com/v1/messages", data=body,
        headers={"x-api-key": key, "anthropic-version": "2023-06-01",
                 "content-type": "application/json"})
    with urllib.request.urlopen(req, timeout=60) as r:
        out = json.load(r)
    return out["content"][0]["text"].strip().strip(".").lower()


def observe(addr, channel, text, image_bytes, mime):
    """Feed the caption with its raw bytes: the brain reasons over the caption, the
    picture is kept content-addressed and referenced from the moment."""
    req = {"cmd": "observe", "channel": channel, "content": text, "force": True,
           "media": base64.b64encode(image_bytes).decode(), "mime": mime}
    host, port = addr.rsplit(":", 1)
    with socket.create_connection((host, int(port)), timeout=120) as sock:
        sock.sendall((json.dumps(req) + "\n").encode())
        return json.loads(sock.makefile("rb").readline())


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--addr", default="127.0.0.1:7777", help="brain TCP address")
    ap.add_argument("--channel", default="vision", help="the sense to observe on")
    ap.add_argument("images", nargs="+", help="image files to show the brain")
    args = ap.parse_args()

    for path in args.images:
        try:
            data = open(path, "rb").read()
        except OSError as err:
            print("error:", err, file=sys.stderr)
            continue
        mime = mimetypes.guess_type(path)[0] or "image/png"
        try:
            cap = caption(data, mime)
        except Exception as err:
            print("caption error:", err, file=sys.stderr)
            continue
        if not cap:
            print("no caption for", path, file=sys.stderr)
            continue
        try:
            observe(args.addr, args.channel, cap, data, mime)
        except OSError as err:
            print("observe error:", err, file=sys.stderr)
            continue
        print(f"{path}: saw {cap!r}", flush=True)


if __name__ == "__main__":
    main()
