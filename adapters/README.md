# Adapters

GOLEM's brain runs in the container and only ever moves text. It does not run a
microphone, a camera, or a speaker. Transduction lives out here, in adapters, so
the brain is identical on every machine and the platform-specific pieces are the
only things you swap.

```
  host (these adapters)                      container (the brain)
   mic  -> speech recognizer -\
   cam  -> captioner ----------\-- tell / observe -->  golem run
   files / metrics ------------/                        text in, reason,
   speaker <- text to speech -\                         text out + events
   screen / notify <-----------\-- reply / event stream <--
            `------------------ TCP: docker -p 127.0.0.1:7777:7777 ------------'
```

## The wire

The daemon serves newline-delimited JSON over the unix socket and, when
`listen_tcp` is set, over TCP. One request per line, one reply per line.

- Tell it something: `{"cmd":"tell","content":"did the dog chase the cat"}` or
  with a language tag `{"cmd":"tell","content":"kopek ne kovaladi","lang":"tr"}`.
  The reply is `{"ok":true,"say":"..."}`.
- Feed a percept: `{"cmd":"observe","channel":"vision","content":"a brown dog"}`.
- Hear it speak: `{"cmd":"subscribe"}` holds the connection open and streams every
  event as a JSON line. The `"ev":"say"` events carry sentences GOLEM speaks,
  including the ones it says on its own, not only replies to a request.

## Running

Publish the brain's TCP port to host loopback only, then point the adapters at it.

```
# seed the runtime folder once: a config and, if you have one, a brain
cp teaching/clean-room-code.json runtime/config.json
# cp <a snapshot>.db runtime/memory.db

docker run -d --name golem-brain -p 127.0.0.1:7777:7777 \
  -v "$PWD/runtime:/root/.golem" golem:latest golem run

# ears + console: type, or pipe a recognizer in
python3 adapters/golem-listen.py --lang tr

# a voice: speak every say event with the host's text-to-speech
python3 adapters/golem-speak.py

# the host's senses: feed this machine's load, logs, and active window in
python3 adapters/golem-senses.py
```

`golem-listen.py` turns stdin (a keyboard, a speech recognizer, a captioner with
`--observe`) into tells and observations. `golem-speak.py` turns the say stream
into speech with whatever the host speaks through (`say`, `espeak`, `spd-say`,
or `--cmd`). `golem-senses.py` samples this machine's vital signs, system log,
and frontmost window and feeds them as observations, the host senses a
containerized brain cannot reach itself; the readings and bands match the
in-binary senses, so the brain sees the same shapes either way. Write your own
in any language: open the TCP socket, send a line, read a line.

## Talking to GOLEM as an LLM peer

An LLM can learn GOLEM by conversing with it on the same channel a person uses: it
watches what GOLEM says and answers, and GOLEM weighs the answers by how far it trusts
that source. GOLEM already asks on its own, so the peer mostly answers its questions.

- Watch the conversation: `golem watch` streams both sides, labelled by who spoke.
- Speak as the peer: `golem tell --source llm "a zebra is an animal"`. Declaring a
  source means GOLEM hears it as that peer, not the operator, at that source's trust.
- Set trust (needs the master key in config): `golem trust llm 0.6`. A newly met
  source starts low, so its claims enter defeasibly and never overwrite what GOLEM
  grounded; the architect raises trust for a source to believe more, lowers it to doubt.

Brief for the LLM (the prompt the architect gives the tasked model): you are teaching
a young learner that only understands simple present-tense statements. Speak only in
short sentences it can parse: "X is a Y", "X is bigger than Y", or subject-verb-object
("a zebra eats grass"). Introduce at most one new noun per turn, and tie it to words
the learner already knows (read its lexicon with `golem concepts`). No abstract prose,
no lists, no clauses. Answer its questions in that register; when it asks "what is X",
say what X is with one known category or property.

## Safety

A reachable brain can be told things and, where a habit is graduated, asked to
act, and this config has execution on behind an allowlist and a denylist. Keep
the published port on `127.0.0.1` so only the host reaches it; do not expose it
to a network.
