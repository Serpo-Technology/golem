# GOLEM: General Observer Limbic Execution Model

GOLEM replaces the "Next-Token Prediction" of an LLM with **Active Inference**. Instead of guessing the next word, it seeks to minimize "Surprise" (Free Energy) in its environment. It is a concurrent, neuro-symbolic daemon built in Go: a **Self-Correcting, Grounded, and Metacognitive** system.

---

## What Runs Today

The build covers the whole loop the design describes: observation, memory, association, language, thought, self-directed acts, and gated action. In detail:

- A normalizer collapses the variable parts of each line (numbers, hex, UUIDs, timestamps, IPs, paths) so prediction runs on line shapes.
- A forgetting frequency model scores Shannon surprise per shape. It habituates to repeated anomalies, adapts when the channel's normal drifts, and stays bounded no matter how long it runs.
- Surprises spike noradrenaline and are stored as memory fragments that carry a snapshot of the neurochemical state.
- Serotonin acts as sleep pressure: it climbs through calm stretches, and high pressure plus silent input triggers the sleep cycle.
- Consolidation decays memory weights, slower for sharper memories, and prunes what fades. Recall restores a memory's weight.
- Retrieval is by feeling: "show me when things felt wrong" returns the fragment with the highest noradrenaline at encoding.
- The daemon form (`golem run`) accepts observations on any number of named channels and keeps its memory in sqlite under `~/.golem`, so fragments survive restarts and hard kills. Channel models persist too, flushed at each sleep and at shutdown.
- Moments that occur near each other in time become linked. When the same pairing keeps recurring, the bond strengthens and dopamine rises, which raises the binding rate in turn.
- Memory formation is rate limited per channel: an error storm collapses into a few exemplars plus one summary moment that records the storm's size.
- `golem tell` is the linguistic channel: your words always encode, bind to the moments around them, and the reply is what they bring to mind. Words are pointers to experience.
- `golem recall` returns direct matches plus the context cloud reached through the link graph, ranked by activation. Recall strengthens every path it walks; sleep decays the paths nothing walks.
- A cue can propose an action. `golem propose "run the tests"` weighs the commands that cue has been grounded to and proposes the strongest, or asks when two are too close to call. Approvals strengthen the grounding and, after a few consistent ones, graduate the pair to autonomous; a denial demotes it. Destructive commands (a denylist covering `rm -rf`, `shutdown`, force-pushes, and the like) can be proposed but never graduate, and every decision lands in an audit trail.
- Commands run for real only when three gates all pass: execution is switched on, no part of the command is on the denylist, and every stage matches the operator's allowlist. All three ship closed (execution off, allowlist empty), so turning it on takes a deliberate edit to `config.json` and a restart. A graduated, allowed command runs autonomously when its cue is proposed; a run that fails demotes the pair, and every outcome feeds back into memory.
- No shell sits behind execution. A command parses into a pipeline of argv stages with explicit pipe and redirect structure, or it is refused with the reason audited: substitution, subshells, chaining, globbing, and escapes are outside the modeled subset. Stages run wired together with real pipes, each stage authorized on its own, and the pipeline succeeds only when every stage exits zero.
- Independent actions run at once, up to a configured cap, while decision stays serial on the brain loop. Each committed pipeline holds claims on the files it names; actions whose claims collide wait their turn in a bounded queue, so two writers of one file can never race. Every outcome returns through one channel, and stopping settles what is running before the final consolidation.
- `golem demo decide` runs the whole loop on throwaway memory: grounding, graduation, the ambiguity veto, and the safety denylist, in about a second. `golem demo pipeline` shows the action layer: refusals, per-stage authorization, overlapping independent actions, and two writers of one file taking turns.
- A CLI talks to the running daemon over a local socket: `status`, `chem`, `observe`, `tell`, `recall`, `concepts`, `teach`, `propose`, `approve`, `deny`, `habits`, `audit`, `voice`, `dump`, `sleep`, `stop`, plus `launchd` to print a start-at-login plist.
- Built-in senses, selected in config: file events in watched directories (bucketed to operation, extension, and parent directory), system vitals banded into words (cpu load, memory pressure, disk space), the macOS unified log filtered to errors and faults, tailed files whose appended lines arrive as the reading channel, fetched pages stripped to text on the web channel, and the frontmost application as a screen channel. `golem shellhook` prints a zsh hook that feeds the shell channel with commands and their failures.
- Replies can be spoken: with the voice on, every sentence the realizer builds is also rendered through the system text-to-speech, queued off the brain loop. `golem voice` switches speech on and off at runtime and `golem voice use <name>` picks the system voice; both persist to config and land in the event trail. Eyes and ears are external transducers by design, neural weights kept at the edge: pipe a captioner into `golem observe vision "a red door"` and a speech recognizer into `golem tell`, narrate while GOLEM perceives, and the night's naming pass grounds the words across showings.
- Open-world perception is policed: channels marked untrusted (the web by default) encode at half weight, can never fire a graduated habit, and never anchor the grounding of an action, so an adversarial page can be remembered without ever moving the hands.
- Attention is channel-relative: each channel's gate asks for a z-score against that channel's own surprise history, so a chatty channel and a quiet one are judged fairly. Sensors never block the brain; a full queue drops its oldest entry and counts the drop.
- Memory is one graph of typed nodes (episodic moments, language-tagged words, amodal concepts, thought operations, grammar parameters, feature attributes) under typed edges: temporal co-occurrence, directed order, names, grounds, and feature.
- Order is learned by spike timing: a moment binds forward to what followed it, and no edge is written backward. Sleep replay weaves the same directed edges along the day it re-fires, so "what usually happens after make test" is answered by reading edges.
- Concepts are born during sleep from shapes that recurred into prototypes. Each carries a lemma category derived from its grounding role (a command shape is verb-like, a recurring sensory shape noun-like) and grounds out to the episodes that instantiate it. Serotonin paces how many concepts one night may mint.
- Spoken words become word nodes bound to the moments around them. The night's naming pass bonds each word to the concepts its moments ground by cross-situational reliability: the share of the word's lived company that lands on the concept, blended with the reverse share, so a word scattered across many concepts bonds weakly everywhere while a faithful one bonds firmly. Dopamine gates how readily words bond at all, a coincidental bond weakens the moment its word recurs amid other active concepts, and utterances count for nothing: a repeated question can never become a thing. Forms from any language converge on one concept once grounded alike, so "testler" recalls what "tests" recalls.
- `golem recall` reads through the lexicon as well as by substring: query words name concepts, concepts ground moments, and paraphrases land together. The reply shows the concept the meaning walked through.
- A workspace holds decaying activation over concepts across ticks and turns, so "do it again" resolves to the warmest verb-like concept. Sleep clears it.
- When perception goes quiet and sleep pressure is building, GOLEM mind-wanders: the warmest concept associates one step through its lived edges and the result feeds back into the workspace, chaining across ticks until fatigue and decay still it. Each step re-enters perception on a thought channel through the same surprise gate the senses face, so a routine association passes unremembered while a novel one is encoded as an insight GOLEM can later recall having had. Thoughts never reset the quiet timers, never fire habits, and never become concepts.
- Thinking is concurrent under a serial arbiter: every setting of one primitive, walk(relation, signal), computes on an immutable snapshot of what is warm and posts a candidate through one channel; the brain loop weighs the candidates and commits every mutation single-threaded. Rounds yield whole to fresh perception, and sleep invalidates in-flight thought.
- The kinds of thought emerge from the walk's settings rather than from named modules: free association is the temporal walk by lived weight, expectation the successor walk, stressful recollection the grounding walk by noradrenaline, deliberation the grounding walk by saliency. Mood selects among them (an alarmed mind dwells on stressful memory, a rewarded one notices what it could run), a thought surprising enough to be remembered reinforces the operation that produced it, and each operation's learned worth sits on an inspectable node in the graph.
- `golem teach --mind worry grounds stress` gives a kind of thinking a name: the word bonds to a concept grounded in the operation itself, and saying it leans the next quiet stretch toward that operation. `golem demo think` walks the arc from lived day to remembered insight to a worried dwell.
- The faculties themselves are acts in the same operant space: recall, ground, consolidate, speak, propose, and introspect compete with the walks under one arbiter, weighed by the chemistry's appetite, their learned worth, and the warmth of any taught name (`golem teach --mind tidy act consolidate`). A winning act runs the function its CLI verb runs, audits the firing, and a proposal it stages still waits behind approve. Each act cools on its own refractory, so the inner life stays a murmur.
- Drives give the unprompted acts their reasons: relevance (alarm seeks context) pushes recall, tidiness (clutter since the last night) pushes consolidation, curiosity (high dopamine over a nameless warm thing) makes the speak act ask what it is, and every drive signs the audit trail with its urge, so an autonomous firing reads as "what, because of which reason."
- One goal at a time can be held across turns: asking about a word that points at nothing adopts a learning goal, visible in `golem status`, hunted by the recall act on quiet rounds, rewarded with dopamine and cleared when the word finally names something, and dropped once it goes stale.
- Each concept carries a sparse signature of interpretable features mined from its lived experience at sleep: the recurring words of its shape, its channel, how it felt. Similarity is feature overlap, so "full disk emergency", never seen and never told, still recalls the disk-full experience, and the reply names the shared attributes that carried the match. The same relation gives the inner loop resemblance: disconnected kin associate as "X is like Y" while every thought still starts and lands on a named node.
- `golem tell` parses each utterance into a concept-frame with an innate grammar and reads its force. Questions are answered with constructed sentences ("After make test, make deploy usually follows."), unknowns are admitted ("I do not know flibber yet."), high dopamine asks back about new words, and low oxytocin hedges the claim ("may follow"). A command stages a proposal behind the same approve gate as always; speech never executes anything by itself.
- Plain language drives the faculties and holds a thread: "recall the disk" runs the recall act and speaks the moment it found, "remember this as a bell" names whatever is most present, "tidy" consolidates, "what were you thinking" reads the inner loop, and "it", "that", and "the other one" follow the conversation's warm topic across turns, with the answer's subject becoming the next referent. A question that leaves a real gap can earn an appended follow-up question: the planner also decides what to say next.
- English inflects, never bolts on: verbs conjugate from principal parts with agreement computed from person, number, and tense ("what happened after make test" gets "usually followed" back), nouns pluralize on a rule table with the irregulars, articles follow the sound ("an hour", "a user"), and negation takes do-support everywhere, so a denial can never realize as its opposite.
- The interrogative routes the answer: what and which read order or definitions ("which commands follow make test" coordinates the field with plural agreement), when answers with the newest moment's time, where with its channel, how many with the tally, and who and why name gaps GOLEM genuinely has. "After X and before Y" parses as two clauses and the answer satisfies both.
- Turkish is the second grammar on the same frame: detected from its closed class, parsed SOV with the anchor before its relator, and answered in kind with suffix morphology under vowel harmony and voicing assimilation ("Make test'ten sonra genellikle make deploy gelir."). The language of the question picks the language of the answer.
- Abstractions form by the same machinery as concrete concepts: a word that never settles on one sensory shape but keeps stable company with the same small cloud of concepts distils, at sleep, into a concept grounded in that constellation. "Trouble" can come to mean disk alarms plus failed deploys jointly, recall reaches the moments through it, and the chain unfolds concept by concept down to lived experience, so even an abstraction stays auditable.
- `golem concepts` lists the abstraction layer: each concept, the words that name it per language, its grounding tally, and its current workspace warmth.
- The grammar's parameters live in memory: function words and their categories, event-verb forms, principal parts, word order, and Turkish suffix recipes are nodes in the same graph, trained in as the first curriculum on first boot and compiled into a working copy at each wake. The engine procedures stay innate; the data is inspectable in `golem dump`. Grammar never fades with the nightly decay, and the one force on it is competition with the lexicon: a form deliberately taught as a label can, over sustained nights, out-weigh its seeded closed-class entry and start parsing as a content word, then heal back once the usage fades.

[ROADMAP.md](ROADMAP.md) tracks how the build got here and what stays open; the GUI memory editor below is the part still ahead of the code.

---

## Architecture & Biological Mapping

GOLEM maps its components directly to biological structures, replacing LLM internals with deterministic, symbolic logic:

| GOLEM Layer | Bio-Equivalent | Role | LLM-Alternative |
| --- | --- | --- | --- |
| **Ingress / Input Relay** | **Thalamus** | Acts as the "Relay Station." Filters all raw data (IO, text, packets) and routes them to the relevant cortex. Data routing & noise filtering. | Go Channels / Multiplexers |
| **Predictor** | **Prefrontal Cortex (PFC)** | The "Executive." Holds the goal and predicts what should happen next. Only "wakes up" the system if `reality ≠ prediction`. Anticipates next system state. | Hierarchical Temporal Memory (HTM) |
| **Conflict Monitor** | **Anterior Cingulate Cortex (ACC)** | Detects "Conflict" between competing pathways in the Basal Ganglia. Detecting ambiguity/uncertainty. | Saliency Delta Logic |
| **Action Selector / Action Gate** | **Basal Ganglia** | The "Gatekeeper." Chooses which "Thought" or "Action" to execute based on current Neurochemistry. Selecting the "Winner" action. | Winner-Take-All Gating |
| **Association Map / Memory Hub** | **Hippocampus** | The "Librarian." Binds disparate sensory fragments into a single "Moment" or "Context." | Weighted Association Graph / Vector Knowledge Graph (SDRs) |
| **Semantic Hub / Concept Store** | **Anterior Temporal Lobe** | The amodal meaning layer. Words and senses converge on one concept node, so synonyms and cross-language translations share a single pointer. | Word Embeddings / Shared Vector Space |
| **Workspace / Global Broadcast** | **Dorsolateral PFC** | Sustained activation held across ticks. Carries dialogue reference and is the seat of the inner loop. | Attention Context Window |
| **Inner Loop / Thought Operations** | **Default Mode Network** | Mind-wandering when perception goes quiet: one primitive, walk(relation, signal), applied to the warmest concept, with chemistry selecting the kind of thought and remembered insights reinforcing the operations that produce them. | Chain-of-Thought Sampling |
| **Drives & Goals** | **Hypothalamus** | Named urges (relevance, tidiness, curiosity, the held goal) computed from body state. A drive only biases the action gate, and the winning act carries its drive's reason onto the audit trail. | Reward Shaping / Objective Functions |
| **Feature Signatures** | **Cortical Feature Populations** | Sparse interpretable attributes under each concept node, mined from lived experience. Similarity is feature overlap, so novel phrasings land near what they resemble while concept cells keep the provenance. | Embedding Similarity |
| **Language (Speech & Parsing)** | **Broca's & Wernicke's Areas** | Builds a sentence from a concept-frame and parses an utterance back into one, with a single innate grammar both ways over parameters held in memory. | Transformer Decoder / Tokenizer |

---

## Core Methodologies

### 1. Predictive Coding (The Observer)

Unlike LLMs that wait for a prompt, GOLEM is a **General Observer**. It maintains a hierarchy of local predictors across all sensory channels (File I/O, Network, User Input).

- **The Error Signal:** If the input matches the prediction, the data is dropped.
- **Saliency:** Only "Surprises" (prediction errors) are passed to the conscious loop, drastically reducing CPU overhead and focusing "attention" only where it matters.

### 2. Limbic Scaling (The Heartbeat)

GOLEM's behavior is modulated by a simulated **Neurochemical State** (Dopamine, Noradrenaline, Serotonin).

- **Dopamine:** Drives curiosity and the weighting of new "Association Fragments."
- **Noradrenaline:** Controls "sampling speed"; it spikes during high-surprise events to increase system alertness.
- **Serotonin:** Regulates stability and triggers the "Sleep/Janitor" cycle.

### 3. Multimodal Association (The Memory)

Memory in GOLEM is not a flat "data blob." It mimics the human Hippocampus-Neocortex relationship.

- **Labeled Lines:** Data is categorized by the physical "channel" it arrived through (Visual, Auditory, SystemState).
- **Synaptic Links:** Fragments are bound together by timestamps and neuro-state, creating a "Context Cloud" rather than a simple database entry.

### 4. Language as a "Pointer" (The Grounding)

In an LLM, the word "Apple" is a 1536-dimensional vector of probabilities. In GOLEM, the word "Apple" is a **Pointer** to a sensory experience.

- **How it Learns:** If you type "Cleanup" and then run `rm -rf ./tmp`, GOLEM's **Basal Ganglia** logic notices the temporal proximity. It creates a **Synaptic Link** (a Go pointer) in the **Hippocampus** between the string "Cleanup" and the execution of that command.
- **Why it's Better:** It doesn't need 1 trillion tokens to know what "Cleanup" means. It needs **one** grounded example. Language becomes a "Remote Control" for the system's internal functions.
- **The Concept Layer:** "Cleanup," the Turkish "Temizle," and the act of clearing a directory are three handles on one **amodal concept** node. Word fragments *name* the concept; the concept *grounds* out to the experiences that instantiate it. Because many words can name one concept, synonyms within a language and translations across languages converge on the same node the moment they are grounded to the same experience, with no shared vector space and no special cross-lingual machinery. This is the semantic hub the brain keeps in its anterior temporal lobe.

### 5. Metacognition: The Conflict Resolver (ACC Logic)

To make GOLEM truly intelligent, it must possess **Metacognition**, which is the ability to monitor its own uncertainty. This is handled by the **Anterior Cingulate Cortex (ACC)**, which detects "Conflict" between competing pathways in the **Basal Ganglia**.

If two pathways (e.g., "Mop the Floor" vs. "Clear the Logs") are both firing with similar strength, the ACC triggers a "Veto" and forces the system to ask for **Grounding**.

**The Threshold:** If the difference in saliency between the top two choices is less than a specific margin (e.g., 15%), the **Basal Ganglia** inhibits both actions and triggers a "Linguistic Request" to the user.

### 6. The Workspace (Sustained Activation)

A mind holds its active thoughts. The **Workspace** keeps a decaying activation pattern alive across ticks and turns, so the most active concepts stay warm between one utterance and the next.

- **Dialogue memory:** "do it again" and "that one" resolve to whatever is still warm in the buffer.
- **What warms it:** a moment of a known shape, words that name a concept, every concept an answer walks through, and the associations each committed thought hands its warmth to. Each homeostasis tick cools the pattern, and sleep clears it: working memory does not survive the night.
- **The broadcast:** sustained, re-broadcast activation is the substrate the Global Workspace theory of consciousness points to, and the seat of the inner loop: when perception goes quiet, the warmest concept seeds the next thought, and the thought's product feeds back into the same buffer.

### 7. Grammar and Speech (Broca & Wernicke)

GOLEM does not store sentences to repeat them. It builds them. Following Levelt's model of speech production, a message starts as a small structure of concepts and roles (who did what to what), and a single innate grammar renders that structure into a sentence.

- **The Realizer (Broca):** the grammar runs as a generator, the way a compiler's code generator emits from a syntax tree. A concept-frame goes in and a grammatical sentence comes out, inflected and ordered for the active language. Ungrammatical strings are never built, so nothing needs correcting after the fact.
- **The Parser (Wernicke):** the same grammar runs backward, turning an utterance into a concept-frame and reading its force (statement, question, command).
- **What to Say:** a message planner above the grammar chooses the content. With nothing grounded for a query, GOLEM says it does not know; when curiosity is high, it asks.
- **One Example is Enough:** the engine is innate and its parameters are entrenched entries in memory; the lexicon (which words point to which concepts) is learned, and a word taught in a second language attaches to the same concept as its English synonym.

### 8. Action: Parallel Substrate, Serial Commitment

The brain is parallel underneath and serial only where it commits: billions of neurons fire at once, and what is serial is the basal-ganglia gate that picks one action and the single thread of attention. GOLEM mirrors this. Senses and committed actions run as goroutines; the `select` loop is the one serial thread, the global workspace and the action-selection gate at the same time.

- **Decision is serial:** the winner-take-all selection lives on the loop and never races, so two conflicting choices cannot fire together.
- **Execution is parallel, bounded:** committed actions fan out as goroutines up to a cap, held apart by **effector exclusion**. Two actions touching the same resource serialize while independent ones run free. A machine has many effectors a body does not, and this uses them without a second thread ever deciding or writing memory.
- **No shell in the loop:** an action is a structured pipeline of argument stages wired with explicit pipes, each stage authorized on its own, so a command runs without ever handing a string to `/bin/sh`.

---

## Typical Examples of Operation

### A. Data Processing (The "Surprise" Filter)

1. **Observation:** A log file updates with a standard "Ping" every 5 seconds.
2. **PFC (Predictor):** "I expect a Ping."
3. **Thalamus (Relay):** Matches the input to the prediction.
4. **Result:** Data is discarded. **CPU usage: ~0%.**
5. **Observation:** A log file updates with "Connection Refused."
6. **PFC:** "Unexpected!" This triggers a **Noradrenaline Spike**.
7. **Execution:** GOLEM alerts the user and records the moment.

### B. Memory Formation & Retrieval

- **Encoding:** While you work, the **Hippocampus** creates a "Moment Struct" containing: `[FileEdited, CurrentMood(Dopamine), UserTyped, Time]`.
- **Retrieval:** When you say "Show me that error," GOLEM doesn't search for the text "error." It searches for the **Noradrenaline Spike** in its memory and finds the exact moment your system felt "stressed."

### C. The Full Interaction Loop: Conflict Resolution

**User Input:** *"Clean it up."*

1. **Observation:** GOLEM sees high log entropy AND a dirty floor sensor.
2. **Conflict:** The **ACC** detects that `Log_Rotate` (Saliency: 0.82) and `Mop_Floor` (Saliency: 0.79) are too close (delta < 0.15).
3. **Inquiry:** GOLEM halts execution.
   > *GOLEM: "Conflict detected. Both the system logs and the physical environment require maintenance. Which 'Clean up' are you referring to?"*
4. **Grounding:** User says "The logs."
5. **Strengthening:** GOLEM executes the log rotation and **strengthens the link** between the current terminal state and that command, while **weakening** the floor link for this specific context.

---

## Memory Life-Cycle: Importance & Pruning

GOLEM uses **Synaptic Scaling** during its "Sleep Cycle" to decide what stays.

**Importance Scaling.** A memory is "Important" if:

1. It had a **High Limbic Weight** (Happened during high Dopamine/Noradrenaline).
2. It has **High Connectivity** (It is linked to many other fragments).

**The Pruning (Janitor):**

- Every "Night," GOLEM reduces the weight of all fragments by 10%.
- If a fragment's weight ≤ a threshold (e.g., 0.1), it is **permanently deleted**.
- **Result:** Repetitive, boring data fades away. Rare, high-impact events (like a system crash) stay forever.
- Word, concept, and feature nodes sit outside the nightly fade. They die by disconnection instead: a word vanishes when its last edge does, a concept when nothing grounds it anymore, and a feature when its last bearer goes.
- Thought-operation and grammar nodes never fade at all. An operation's weight is its learned worth, moved only by the fruit of its own thoughts and acts, and a grammar parameter's weight is its entrenchment, moved only by competition with the lexicon.

---

## Systems Consolidation (The Sleep Cycle)

GOLEM features an automated "Sleep" phase triggered by high Serotonin and low input.

1. **Replay:** The system reactivates the day's "surprising" fragments in order, weaving cross-channel links and directed successor edges the waking window missed.
2. **Synaptic Downscaling:** Global weight reduction of all memories. Weak/noisy fragments are pruned (deleted).
3. **Abstraction:** Recurring shapes merge into prototype fragments, prototypes distil into concept nodes, the day's words bond to the concepts their moments ground, and a word whose company is a stable cloud of concepts distils into an abstraction grounded in that constellation.
4. **Signatures:** every concept's sparse feature signature rewrites from its current grounding, so resemblance stays current and auditable down to the moments underneath.
5. **The Grammar's Night:** seeded parameters and lived usage settle their competition, and the working grammar recompiles from the surviving weights.

---

## Technical Specifications

- **Language:** Go (1.25+)
- **Concurrency:** parallel substrate, serial commitment. Sensory channels, committed execution, and the thought operations all run as `goroutines` off the loop; the `select` loop is the single serial thread that decides and writes memory, the Global Workspace and the action gate at once.
- **Architecture:** Event-driven, non-blocking, neuro-symbolic.
- **Dependencies:** Zero-bloat. Pure-Go libraries only (sqlite via modernc.org, fsnotify); no model weights, no frameworks, no cgo.

---

## Repository Layout

One binary, one brain-loop package, and the organs it wires together.
Packages live under `internal/` because GOLEM is a daemon, not a library;
the bio-mapping column says which part of the architecture table each
package carries.

```
cmd/golem/          the CLI: dispatch, printing, the DOT export, and the
                    in-RAM synthetic demo
internal/
  limbic/           the neurochemical state and its homeostasis (the limbic
                    system: dopamine, noradrenaline, serotonin, oxytocin)
  attention/        normalizer, surprise predictor, per-channel calibrator
                    (thalamus and prefrontal cortex: what reaches awareness)
  senses/           the observation packet and the built-in sensors: files,
                    file tails, vitals, syslog, web pages, the frontmost
                    app, the synthetic channel, and the shell contract
  memory/           moments, words, concepts, thought operations, grammar
                    parameters, and feature attributes as one typed graph
                    in sqlite, the in-RAM hippocampus, the hot working
                    set, and consolidation (hippocampus and cortex)
  grammar/          the innate language engine and its curriculum: one
                    concept-frame, English and Turkish parsers and
                    realizers, hand-rolled morphology, and the parameters
                    compiled from memory (Broca and Wernicke)
  pipeline/         shell-free execution: parse commands into argv stages or
                    refuse, run them over real pipes, derive resource claims
  events/           the structured trail, one JSON object per line, rotated
  wire/             the socket protocol: requests, responses, display forms,
                    and the client side of one round trip
  config/           tuning, the config file, channel trust, and the
                    destructive-pattern denylist with its matchers
  daemon/           the brain loop that owns all mutable state: encoding,
                    association, sleep, the decision loop and its gates, the
                    parallel action arbiter, the message planner, the
                    workspace, the inner loop with its walk operations and
                    acts, the drives and the held goal, the feature passes,
                    the voice, and the demos that drive it all on throwaway
                    homes (basal ganglia, ACC, hypothalamus, and the global
                    workspace)
```

---

## Getting Started

```bash
# Clone the mind
git clone https://github.com/user/golem.git

# Run the synthetic demo (it observes, sleeps, and exits on its own)
go run ./cmd/golem demo

# Watch concepts, words, and built sentences emerge from one day
go run ./cmd/golem demo language

# Watch shell-free pipelines and parallel actions under the gates
go run ./cmd/golem demo pipeline

# Watch quiet mind-wandering keep one surprising thought
go run ./cmd/golem demo think

# Run the tests
go test ./...

# Build and start the daemon
go build -o golem ./cmd/golem
./golem run

# Talk to it from another terminal
./golem observe shell "run: make test"
./golem status
./golem recall disk
./golem tell "what usually happens after make test"
./golem tell "when did it happen"
./golem tell "recall the disk"
./golem tell --lang tr "testler temiz"
./golem concepts
./golem teach --mind worry grounds stress
./golem teach --mind tidy act consolidate
./golem stop

# Optional: feed your shell into GOLEM (open a new shell afterward)
./golem shellhook >> ~/.zshrc
```

> **Note:** GOLEM is designed to run in the background of your OS, observing and learning your patterns without the need for constant prompting.

### Configuration

The daemon reads `~/.golem/config.json` and writes one full of defaults on first run. [config.example.json](config.example.json) in the repository shows every field at its default, and a test pins it to the code, so the example cannot drift. The fields most worth knowing:

- `channels` selects the built-in senses: `vitals` and `syslog` ship on; add `watch_dirs` for file events, `tail_files` for the reading channel, `watch_urls` for the web channel, `screen` for the frontmost app, and `voice` to speak replies (`voice_name` picks the system voice; `golem voice` adjusts both without a restart).
- `execute_enabled` and `execute_allowlist` are two of the three execution gates and both ship closed: nothing runs for real until you switch execution on and allowlist commands by hand.
- `untrusted_channels` (the web by default) marks senses whose moments encode lightly and can never cue a habit or ground an action.
- `surprise_threshold`, `z_gate`, and `predictor_windows` tune what reaches consciousness per channel; the sleep settings pace consolidation.

---

## Future Roadmap

### Memory Editor Tool (GME)

The **GOLEM Memory Editor** is the part still ahead of the code. It is a specialized interface for the "Ghost in the Machine," providing GUI-based "Neurosurgery" on your GOLEM model.

**Features:**

- **Fragment Inspection / Synaptic Visualization:** A graph-based GUI to see how GOLEM has linked your files, commands, and conversations. Click any memory node to see exactly what the system saw (the "Blob") and what it felt (the "Neurochemistry") when it was created.
- **Manual Grounding / Manual Weighting:** Drag a "Linguistic Fragment" (a word) onto a "System Action" (a script) to manually teach GOLEM a new skill without training. Manually strengthen or weaken "Synaptic Links" between fragments to prioritize specific knowledge.
  - *Example:* Drag a line between the word "Safety" and the command "Backup" to manually program a new instinct.
- **The "Grounding" Update:** Manually edit a memory fragment and send a **Synaptic Reset** signal to GOLEM. GOLEM receives the notification, triggers a "Causal Update," and immediately re-maps all associated memories (e.g., updating a project path).
- **Synaptic Reset:** If GOLEM is stuck in a bad habit (a "Fixed Action Pattern"), use the editor to prune that specific branch of the graph.
- **Neuro-Injection:** A developer tool to manually spike Dopamine or Noradrenaline to test GOLEM's reaction to stress or novelty.

### Neurochemical Expansion (GOLEM v2)

While the initial system uses Dopamine, Noradrenaline, and Serotonin, the **GOLEM v2** roadmap expands the neurochemical palette:

| Neurochemical | Biological Role | GOLEM Behavior |
| --- | --- | --- |
| **Acetylcholine** | Focus / Precision | Controls "Learning Rate." High Acetylcholine creates "Hard" symbolic links immediately; low creates "Soft" probabilistic links. |
| **GABA** | Inhibition | Sensory inhibition that focuses the system on one task. |
| **Adenosine** | Sleep Pressure | Triggers the Consolidation/Batching cycle. |
| **Oxytocin** (built) | Alignment / Bond | Measures user trust. Denials and failed runs lower it, which raises the graduation bar everywhere, so a corrected GOLEM graduates habits more slowly and stays in confirm-first mode longer. |
| **Cortisol** | System Stress / Resource Strain | Measures CPU/RAM/Heat levels. High Cortisol triggers "Avoidance" behaviors, like shutting down non-essential sensory channels to save the core. |

