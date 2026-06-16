#!/usr/bin/env python3
"""Embed each captioned percept and feed it to the brain with its coordinate.

A percept arrives as a caption (a captioner's text, a recognizer's transcript).
This adapter places that caption in a vector space and sends `observe <channel>
<caption>` with the embedding alongside, so the brain stores an associative
coordinate beside the moment. The brain computes no embedding of its own; this
is the embedder transducer, beside the captioner, the same separation eyes and
the voice already keep. At consolidation the brain rolls each percept's
coordinate up into the concept it grounds, so resemblance becomes nearness in a
space rather than overlap of discrete tokens.

Input: lines on stdin, each a caption, or "channel<TAB>caption" to choose a
channel (default: vision). Pipe a captioner in:

    captioner | python3 golem-embed.py [--addr 127.0.0.1:7777] [--channel vision]

Backends, auto-selected: sentence-transformers (real semantics) if installed,
else a dependency-free hashing embedder (surface similarity only, for testing
the pipe). One brain must keep ONE backend: widths and spaces differ between
them, and a mismatch reads as unrelated.
"""
import argparse
import hashlib
import json
import math
import socket
import sys


def load_embedder(prefer):
    if prefer in ("auto", "st"):
        try:
            from sentence_transformers import SentenceTransformer
            model = SentenceTransformer("all-MiniLM-L6-v2")

            def embed(text):
                return [float(x) for x in model.encode(text, normalize_embeddings=True)]

            return embed, "sentence-transformers/all-MiniLM-L6-v2 (384d, semantic)"
        except Exception as err:
            if prefer == "st":
                raise
            print("sentence-transformers unavailable (%s); using the hash fallback" % err,
                  file=sys.stderr)

    # Dependency-free fallback: hashed character trigrams into a fixed width,
    # L2-normalized. Surface similarity only, no semantics, enough to exercise
    # the pipe and to cluster captions that share spelling.
    dim = 256

    def embed(text):
        vec = [0.0] * dim
        t = "  " + text.lower() + "  "
        for i in range(len(t) - 2):
            h = int(hashlib.md5(t[i:i + 3].encode()).hexdigest(), 16)
            vec[h % dim] += 1.0 if (h >> 8) & 1 else -1.0
        norm = math.sqrt(sum(x * x for x in vec)) or 1.0
        return [x / norm for x in vec]

    return embed, "hash-trigram (%dd, surface-only)" % dim


def observe(addr, channel, caption, embedding):
    host, port = addr.rsplit(":", 1)
    req = {"cmd": "observe", "channel": channel, "content": caption, "embedding": embedding}
    with socket.create_connection((host, int(port)), timeout=10) as s:
        s.sendall((json.dumps(req) + "\n").encode())
        s.makefile("rb").readline()


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--addr", default="127.0.0.1:7777", help="brain TCP address")
    ap.add_argument("--channel", default="vision", help="default channel for a bare caption")
    ap.add_argument("--backend", default="auto", choices=["auto", "st", "hash"])
    args = ap.parse_args()
    embed, name = load_embedder(args.backend)
    print("embedder: %s -> %s" % (name, args.addr), file=sys.stderr)
    for line in sys.stdin:
        line = line.rstrip("\n")
        if not line.strip():
            continue
        channel, caption = args.channel, line
        if "\t" in line:
            channel, caption = line.split("\t", 1)
        try:
            observe(args.addr, channel, caption, embed(caption))
            print("  embedded + observed [%s] %s" % (channel, caption), file=sys.stderr)
        except OSError as err:
            print("  send failed: %s" % err, file=sys.stderr)


if __name__ == "__main__":
    main()
