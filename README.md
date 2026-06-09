# GOLEM: General Observer Limbic Execution Model

GOLEM replaces the "Next-Token Prediction" of an LLM with **Active Inference**. Instead of guessing the next word, it seeks to minimize "Surprise" (Free Energy) in its environment. It is a concurrent, neuro-symbolic daemon built in Go — a **Self-Correcting, Grounded, and Metacognitive** system.

---

## What Runs Today

The current build is the observation half of the design, on one synthetic log channel:

- A normalizer collapses the variable parts of each line (numbers, hex, UUIDs, timestamps, IPs, paths) so prediction runs on line shapes.
- A forgetting frequency model scores Shannon surprise per shape. It habituates to repeated anomalies, adapts when the channel's normal drifts, and stays bounded no matter how long it runs.
- Surprises spike noradrenaline and are stored as memory fragments that carry a snapshot of the neurochemical state.
- Serotonin acts as sleep pressure: it climbs through calm stretches, and high pressure plus silent input triggers the sleep cycle.
- Consolidation decays memory weights, slower for sharper memories, and prunes what fades. Recall restores a memory's weight.
- Retrieval is by feeling: "show me when things felt wrong" returns the fragment with the highest noradrenaline at encoding.
- The daemon form (`golem run`) accepts observations on any number of named channels and keeps its memory in sqlite under `~/.golem`, so fragments survive restarts and hard kills. Channel models persist too, flushed at each sleep and at shutdown.
- A CLI talks to the running daemon over a local socket: `status`, `chem`, `observe`, `recall`, `dump`, `stop`, plus `launchd` to print a start-at-login plist.
- Built-in senses, selected in config: file events in watched directories (bucketed to operation, extension, and parent directory), system vitals banded into words (cpu load, memory pressure, disk space), and the macOS unified log filtered to errors and faults. `golem shellhook` prints a zsh hook that feeds the shell channel with commands and their failures.
- Attention is channel-relative: each channel's gate asks for a z-score against that channel's own surprise history, so a chatty channel and a quiet one are judged fairly. Sensors never block the brain; a full queue drops its oldest entry and counts the drop.

Everything below describes the design target. [ROADMAP.md](ROADMAP.md) tracks the path from here to there.

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

### 5. Metacognition: The Conflict Resolver (ACC Logic)

To make GOLEM truly intelligent, it must possess **Metacognition** — the ability to monitor its own uncertainty. This is handled by the **Anterior Cingulate Cortex (ACC)**, which detects "Conflict" between competing pathways in the **Basal Ganglia**.

If two pathways (e.g., "Mop the Floor" vs. "Clear the Logs") are both firing with similar strength, the ACC triggers a "Veto" and forces the system to ask for **Grounding**.

**The Threshold:** If the difference in saliency between the top two choices is less than a specific margin (e.g., 15%), the **Basal Ganglia** inhibits both actions and triggers a "Linguistic Request" to the user.

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

---

## Systems Consolidation (The Sleep Cycle)

GOLEM features an automated "Sleep" phase triggered by high Serotonin and low input.

1. **Replay:** The system reactivates the day's "surprising" fragments.
2. **Synaptic Downscaling:** Global weight reduction of all memories. Weak/noisy fragments are pruned (deleted).
3. **Abstraction:** Multiple specific fragments are compressed into "General Rules."

---

## Technical Specifications

- **Language:** Go (1.25+)
- **Concurrency:** Heavy use of `goroutines` for sensory channels and `select` for the Global Workspace (Consciousness).
- **Architecture:** Event-driven, non-blocking, neuro-symbolic.
- **Dependencies:** Zero-bloat. Pure-Go libraries only (sqlite via modernc.org, fsnotify); no model weights, no frameworks, no cgo.

---

## Getting Started

```bash
# Clone the mind
git clone https://github.com/user/golem.git

# Run the synthetic demo (it observes, sleeps, and exits on its own)
go run . demo

# Run the tests
go test ./...

# Build and start the daemon
go build -o golem .
./golem run

# Talk to it from another terminal
./golem observe shell "run: make test"
./golem status
./golem recall disk
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
| **Oxytocin** | Alignment / Bond | Measures "User Trust." If the user frequently corrects GOLEM, Oxytocin drops, causing the system to become "Hesitant" — asking for permission before executing commands. |
| **Cortisol** | System Stress / Resource Strain | Measures CPU/RAM/Heat levels. High Cortisol triggers "Avoidance" behaviors, like shutting down non-essential sensory channels to save the core. |

