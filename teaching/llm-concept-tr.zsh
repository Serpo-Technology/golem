#!/usr/bin/env zsh
# The language-model concept, brought into Turkish. The English concept is already
# grounded (restore the llm-concept snapshot first): a model and an llm that predict,
# learn, and read, over words, patterns, rules, texts, and sentences. None of these
# names a percept, so Turkish cannot reach them through a shared sight the way it
# reaches "dog" through "köpek". It reaches them through the relation instead. Each
# fact is told in English to warm its relation, then in Turkish at once: the Turkish
# verb binds to the warm relation, and its object binds to the object of the warm
# proposition, so "desen" becomes another name for "pattern", not a parallel thing.
# The Turkish is past tense, whose stem is regular, so the verbs need no seeding.
#
#   GOLEM=golem zsh llm-concept-tr.zsh
set -u
GOLEM=${GOLEM:-golem}
GAP=${GAP:-2}

log()   { print -r -- "$(date +%H:%M:%S) $*" }
sayw()  { local i; for i in 1 2 3; do $GOLEM tell "$1" >/dev/null 2>&1 && { sleep 0.2; return 0 }; sleep 1; done; return 0 }
saytr() { local i; for i in 1 2 3; do $GOLEM tell --lang tr "$1" >/dev/null 2>&1 && { sleep 0.2; return 0 }; sleep 1; done; return 0 }
nap()   { log "(nap: consolidate)"; $GOLEM sleep >/dev/null 2>&1; sleep "${BLOCKGAP:-8}" }
# The English telling warms the relation; the Turkish telling, at once, binds to it
# before a consolidation can clear the workspace (the bi_rel adjacency).
bi() { log "bind: $1 / $2"; sayw "$1"; saytr "$2"; sleep $GAP }

log "=== the language-model concept in Turkish ==="

# predict / öngör, learn / öğren, read / oku: each relation warmed in English, bound
# in Turkish, and its object bound on it (word/kelime, pattern/desen, text/yazı).
bi "a model predicts a word"   "model kelimeyi öngördü"
bi "a model learns a pattern"  "model deseni öğrendi"
bi "a model reads a text"      "model yazıyı okudu"
nap

# The remaining objects (rule/kural, sentence/cümle) and the llm as the agent, so the
# whole vocabulary the English concept holds is named in Turkish too.
bi "a model learns a rule"     "model kuralı öğrendi"
bi "an llm reads a sentence"   "llm cümleyi okudu"
bi "an llm predicts a word"    "llm kelimeyi öngördü"
bi "an llm learns a pattern"   "llm deseni öğrendi"
nap

log "=== done: the language-model concept is bilingual ==="
