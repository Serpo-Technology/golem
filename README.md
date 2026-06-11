# GOLEM: General Observer Limbic Execution Model

GOLEM replaces the "Next-Token Prediction" of an LLM with **Active Inference**. Instead of guessing the next word, it seeks to minimize "Surprise" (Free Energy) in its environment. It is a concurrent, neuro-symbolic daemon built in Go — a **Self-Correcting, Grounded, and Metacognitive** system.

---

## What Runs Today

The build covers the whole loop the design describes: observation, memory, association, language, and gated action. In detail:

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
- Commands run for real only when three gates all pass: execution is switched on, the command is not on the denylist, and it matches the operator's allowlist. All three ship closed (execution off, allowlist empty), so turning it on takes a deliberate edit to `config.json` and a restart. A graduated, allowed command runs autonomously when its cue is proposed; a run that fails demotes the pair, and every outcome feeds back into memory.
- `golem demo decide` runs the whole loop on throwaway memory: grounding, graduation, the ambiguity veto, and the safety denylist, in about a second.
- A CLI talks to the running daemon over a local socket: `status`, `chem`, `observe`, `tell`, `recall`, `concepts`, `teach`, `propose`, `approve`, `deny`, `habits`, `audit`, `dump`, `stop`, plus `launchd` to print a start-at-login plist.
- Built-in senses, selected in config: file events in watched directories (bucketed to operation, extension, and parent directory), system vitals banded into words (cpu load, memory pressure, disk space), and the macOS unified log filtered to errors and faults. `golem shellhook` prints a zsh hook that feeds the shell channel with commands and their failures.
- Attention is channel-relative: each channel's gate asks for a z-score against that channel's own surprise history, so a chatty channel and a quiet one are judged fairly. Sensors never block the brain; a full queue drops its oldest entry and counts the drop.
- Memory is one graph with three node kinds (episodic moments, language-tagged words, amodal concepts) under typed edges: temporal co-occurrence, directed order, names, and grounds.
- Order is learned by spike timing: a moment binds forward to what followed it, and no edge is written backward. Sleep replay weaves the same directed edges along the day it re-fires, so "what usually happens after make test" is answered by reading edges.
- Concepts are born during sleep from shapes that recurred into prototypes. Each carries a lemma category derived from its grounding role (a command shape is verb-like, a recurring sensory shape noun-like) and grounds out to the episodes that instantiate it. Serotonin paces how many concepts one night may mint.
- Spoken words become word nodes bound to the moments around them. The night's naming pass bonds each word to the concepts its moments ground, at a dopamine-gated rate; bonds that stop recurring decay. Forms from any language converge on one concept once grounded alike, so "testler" recalls what "tests" recalls.
- `golem recall` reads through the lexicon as well as by substring: query words name concepts, concepts ground moments, and paraphrases land together. The reply shows the concept the meaning walked through.
- A workspace holds decaying activation over concepts across ticks and turns, so "do it again" resolves to the warmest verb-like concept. Sleep clears it.
- `golem tell` parses each utterance into a concept-frame with an innate grammar and reads its force. Questions are answered with constructed sentences ("After make test, make deploy usually follows."), unknowns are admitted ("I do not know flibber yet."), high dopamine asks back about new words, and low oxytocin hedges the claim ("may follow"). A command stages a proposal behind the same approve gate as always; speech never executes anything by itself.
- `golem concepts` lists the abstraction layer: each concept, the words that name it per language, its grounding tally, and its current workspace warmth.

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
| **Language (Speech & Parsing)** | **Broca's & Wernicke's Areas** | Builds a sentence from a concept-frame and parses an utterance back into one, with a single innate grammar both ways. | Transformer Decoder / Tokenizer |

---

## Core Methodologies

### 1. Predictive Coding (The Observer)

Unlike LLMs that wait for a prompt, GOLEM is a **General Observer**. It maintains a hierarchy of local predictors across all sensory channels (File I/O, Network, User Input).

- **The Error Signal:** If the input matches the prediction, the data is dropped.
- **Saliency:** Only "Surprises" (prediction errors) are passed to the conscious loop, drastically reducing CPU overhead and focusing "attention" only where it matters.

### 2. Limbic Scaling (The Heartbeat)

GOLEM's behavior is modulated by a simulated **Neurochemical State** (Dopamine, Noradrenaline, Serotonin).

- **Dopamine:** Drives curiosity and the weighting of new "Association Fragments."
- **Noradrenaline:** Controls "sampling speed" — spiking during high-surprise events to increase system alertness.
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

To make GOLEM truly intelligent, it must possess **Metacognition** — the ability to monitor its own uncertainty. This is handled by the **Anterior Cingulate Cortex (ACC)**, which detects "Conflict" between competing pathways in the **Basal Ganglia**.

If two pathways (e.g., "Mop the Floor" vs. "Clear the Logs") are both firing with similar strength, the ACC triggers a "Veto" and forces the system to ask for **Grounding**.

**The Threshold:** If the difference in saliency between the top two choices is less than a specific margin (e.g., 15%), the **Basal Ganglia** inhibits both actions and triggers a "Linguistic Request" to the user.

### 6. The Workspace (Sustained Activation)

A mind holds its active thoughts. The **Workspace** keeps a decaying activation pattern alive across ticks and turns, so the most active concepts stay warm between one utterance and the next.

- **Dialogue memory:** "do it again" and "that one" resolve to whatever is still warm in the buffer.
- **What warms it:** a moment of a known shape, words that name a concept, and every concept an answer walks through. Each homeostasis tick cools the pattern, and sleep clears it: working memory does not survive the night.
- **The broadcast:** sustained, re-broadcast activation is the substrate the Global Workspace theory of consciousness points to, and the seat of a future inner loop where GOLEM perceives its own active concepts.

### 7. Grammar and Speech (Broca & Wernicke)

GOLEM does not store sentences to repeat them. It builds them. Following Levelt's model of speech production, a message starts as a small structure of concepts and roles (who did what to what), and a single innate grammar renders that structure into a sentence.

- **The Realizer (Broca):** the grammar runs as a generator, the way a compiler's code generator emits from a syntax tree. A concept-frame goes in and a grammatical sentence comes out, inflected and ordered for the active language. Ungrammatical strings are never built, so nothing needs correcting after the fact.
- **The Parser (Wernicke):** the same grammar runs backward, turning an utterance into a concept-frame and reading its force (statement, question, command).
- **What to Say:** a message planner above the grammar chooses the content. With nothing grounded for a query, GOLEM says it does not know; when curiosity is high, it asks.
- **One Example is Enough:** the grammar is innate; only the lexicon (which words point to which concepts) is learned, and a word taught in a second language attaches to the same concept as its English synonym.

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
6. **PFC:** "Unexpected!" → Triggers **Noradrenaline Spike**.
7. **Execution:** GOLEM alerts the user and records the moment.

### B. Memory Formation & Retrieval

- **Encoding:** While you work, the **Hippocampus** creates a "Moment Struct" containing: `[FileEdited, CurrentMood(Dopamine), UserTyped, Time]`.
- **Retrieval:** When you say "Show me that error," GOLEM doesn't search for the text "error." It searches for the **Noradrenaline Spike** in its memory — finding the exact moment your system felt "stressed."

### C. The Full Interaction Loop — Conflict Resolution

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

**Importance Scaling** — A memory is "Important" if:

1. It had a **High Limbic Weight** (Happened during high Dopamine/Noradrenaline).
2. It has **High Connectivity** (It is linked to many other fragments).

**The Pruning (Janitor):**

- Every "Night," GOLEM reduces the weight of all fragments by 10%.
- If a fragment's weight ≤ a threshold (e.g., 0.1), it is **permanently deleted**.
- **Result:** Repetitive, boring data fades away. Rare, high-impact events (like a system crash) stay forever.
- Word and concept nodes sit outside the nightly fade. They die by disconnection instead: a word vanishes when its last edge does, and a concept vanishes when nothing grounds it anymore.

---

## Systems Consolidation (The Sleep Cycle)

GOLEM features an automated "Sleep" phase triggered by high Serotonin and low input.

1. **Replay:** The system reactivates the day's "surprising" fragments in order, weaving cross-channel links and directed successor edges the waking window missed.
2. **Synaptic Downscaling:** Global weight reduction of all memories. Weak/noisy fragments are pruned (deleted).
3. **Abstraction:** Recurring shapes merge into prototype fragments, prototypes distil into concept nodes, and the day's words bond to the concepts their moments ground.

---

## Technical Specifications

- **Language:** Go (1.25+)
- **Concurrency:** parallel substrate, serial commitment. Sensory channels run as `goroutines` and committed execution runs off the loop; the `select` loop is the single serial thread that decides and writes memory, the Global Workspace and the action gate at once.
- **Architecture:** Event-driven, non-blocking, neuro-symbolic.
- **Dependencies:** Zero-bloat. Pure-Go libraries only (sqlite via modernc.org, fsnotify); no model weights, no frameworks, no cgo.

---

## Getting Started

```bash
# Clone the mind
git clone https://github.com/user/golem.git

# Run the synthetic demo (it observes, sleeps, and exits on its own)
go run . demo

# Watch concepts, words, and built sentences emerge from one day
go run . demo language

# Run the tests
go test ./...

# Build and start the daemon
go build -o golem .
./golem run

# Talk to it from another terminal
./golem observe shell "run: make test"
./golem status
./golem recall disk
./golem tell "what usually happens after make test"
./golem tell --lang tr "testler temiz"
./golem concepts
./golem stop

# Optional: feed your shell into GOLEM (open a new shell afterward)
./golem shellhook >> ~/.zshrc
```

> **Note:** GOLEM is designed to run in the background of your OS, observing and learning your patterns without the need for constant prompting.

---

## Future Roadmap

### Memory Editor Tool (GME)

The next major phase is the **GOLEM Memory Editor** — a specialized interface for the "Ghost in the Machine," providing GUI-based "Neurosurgery" on your GOLEM model.

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
| **GABA** | Inhibition | Sensory inhibition — focusing the system on one task. |
| **Adenosine** | Sleep Pressure | Triggers the Consolidation/Batching cycle. |
| **Oxytocin** (built) | Alignment / Bond | Measures user trust. Denials and failed runs lower it, which raises the graduation bar everywhere, so a corrected GOLEM graduates habits more slowly and stays in confirm-first mode longer. |
| **Cortisol** | System Stress / Resource Strain | Measures CPU/RAM/Heat levels. High Cortisol triggers "Avoidance" behaviors, like shutting down non-essential sensory channels to save the core. |

