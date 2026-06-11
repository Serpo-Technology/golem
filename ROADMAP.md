# GOLEM Roadmap

Working notes for taking GOLEM from the current demo (one synthetic channel,
in-RAM memory, six-second window) to the daemon the README describes. Step
labels and status checkboxes live in this file; commits describe the change
itself.

## Decisions

- Form factor: background daemon plus a `golem` CLI over a local unix socket.
- Action policy: graduated autonomy. Every proposal starts behind
  per-instance confirmation. A specific learned pair can be promoted to
  auto-run after N consistent approvals; any denial, correction, or failed
  run demotes it immediately. Destructive patterns are never promotable.
- Sensors, in build order: shell commands, project file events, system
  vitals, macOS system log.
- Dependencies: small curated pure-Go libraries are allowed (fsnotify,
  modernc.org/sqlite). No model weights, no frameworks.

## Step 1: Core hardening

Goal: the loop stays honest on real-shaped data.

- [x] Decaying-count predictor: exponential forgetting with a bounded
      vocabulary and eviction. Habituation timescale becomes a tunable
      constant.
- [x] Normalizer in front of the predictor: collapse digits, hex, UUIDs,
      timestamps, and paths into placeholder tokens; predict over the
      resulting templates. Fragments keep the raw content.
- [x] Sleep dynamics: replace the additive weight bonus with
      surprise-slowed decay so pruning can actually fire. Recall refreshes
      a fragment's weight.
- [x] Sleep trigger from quiescence and serotonin level instead of a fixed
      timer.
- [x] Tests: habituation curve, pruning regression (the immortal-fragment
      case), normalizer corpus.
- [x] README: fix the run command (`go run .`), align the Go version with
      go.mod, and separate current behavior from design target.

Done when: a multi-hour synthetic soak with mixed templates keeps predictor
size and fragment count bounded while CPU stays near idle.

## Step 2: Daemon and persistent memory

Goal: GOLEM survives restarts and can be questioned.

- [x] Lifecycle: long-running main loop; SIGINT/SIGTERM runs a final sleep
      cycle and a clean flush.
- [x] Storage: sqlite (modernc.org/sqlite) for fragments and links. Append
      on encode, prune and compact during sleep.
- [x] Config file: watched channels and paths, thresholds, autonomy
      settings.
- [x] `golem` CLI over a unix socket: `status`, `chem`, `recall <query>`,
      `dump`.
- [x] launchd agent for start-at-login once the daemon is stable
      (`golem launchd` prints the plist; loading it is a manual step).

Done when: kill -9 mid-run loses at most the last few seconds of memory,
and `golem recall` answers from a freshly started process.

## Step 3: Real senses and fair attention

Goal: several channels share one workspace without starving or flooding it.

- [x] Shell sensor: zsh preexec/precmd hook reports each command and its
      exit status to the daemon socket (`golem shellhook` prints the
      snippet; appending it to ~/.zshrc is a manual step).
- [x] File sensor: fsnotify on configured directories.
- [x] Vitals sensor: CPU, memory pressure, disk, sampled slowly. Later the
      Cortisol input.
- [x] System log sensor: filtered `log stream` (errors and faults only).
- [x] Per-channel calibration: running mean and spread of surprise per
      channel, gate on channel-relative unusualness, keep raw bits in the
      fragment.
- [x] Backpressure: bounded queues, drop-oldest with a counter. The drop
      counter is itself a stress signal.

Done when: shell, files, and vitals run side by side for a day with
sensible relative alert rates.

Soak notes (June 10): one day, four channels. Balance held: no channel
starved, syslog admitted 0.74% to consciousness, 129 nights slept. But
syslog storms overflowed the workspace (5.9% of the channel dropped),
the store reached 18M with the WAL never checkpointing, and the raw
fragments that survived were mostly routine error noise. In response,
memory formation gained a per-channel refractory that collapses storms
into summary moments, syslog got a longer habituation window, and sleep
now checkpoints the WAL. The shell channel was nearly dark because the
hook only loads in terminals opened after installation.

## Step 4: Association and language

Goal: the Links field comes alive and words become pointers.

- [x] Co-occurrence linking: fragments encoded within a short window get
      bidirectional weighted links.
- [x] Linguistic channel: `golem tell "..."` and a small REPL. Words are
      fragments like everything else.
- [x] Spreading-activation recall: seed by content or limbic key, walk
      links by weight. `golem recall disk` returns the moment plus its
      context cloud.
- [x] Retrieval strengthens weights, the counterpart to sleep decay. Both
      fragment weights and the traversed links gain from practice.
- [x] Dopamine becomes real: rises when an association is confirmed by
      repetition or recall, and scales the link learning rate.
- [x] Retroactive annotation: `golem tell --about <query> <words>` binds
      words to remembered moments found by query, so labels can arrive
      after the binding window has closed.

Done when: "show me when things felt wrong" and "what usually happens
after make test" both answer by walking links.

## Step 5: Decision loop

Goal: from memory to proposals, with a human gate.

- [x] Candidate actions surface from linked action fragments of the
      current context; saliency is link weight modulated by chemistry.
- [x] Winner-take-all gate with the ACC margin veto: top two within the
      margin means GOLEM asks instead of acting.
- [x] Proposal flow: every action starts confirm-to-execute. Approvals
      strengthen the link, denials weaken it. Every proposal and outcome
      lands in an audit log.
- [x] Graduation: a specific phrase-and-command pair is promoted to
      auto-run after N consistent approvals. Any denial, correction, or
      failed run demotes it. The destructive-pattern denylist is never
      promotable.
- [x] Manual grounding: golem teach (used by the seeding demo) binds a cue
      to a command directly, so the loop can be exercised without waiting
      a week for organic shell data.
- [x] Real execution behind autonomy: graduated pairs actually run, gated
      by an allowlist and an explicit opt-in. Three gates, all required:
      the master switch, the denylist, the allowlist. A failed run demotes;
      outcomes feed back as memory.
- [ ] Automatic cueing: an observation that matches a graduated cue
      triggers a proposal on its own, so a graduated pair runs with no
      human supplying the cue. Today the cue still comes from golem
      propose.
- [ ] Oxytocin as trust: frequent corrections lower it globally, which
      raises the promotion bar N everywhere.

Done when: at least one boring real pair (for example a log rotation) has
graduated and runs unattended with a full audit trail.

Status: the loop is built and demonstrated end to end (golem demo decide
and golem demo execute): grounding, graduation, the ACC conflict veto, the
denylist, real execution under three gates, failed-run demotion, and
outcome feedback. Execution ships off by default. The remaining gap to
"unattended" is automatic cueing: a graduated pair still needs a human to
type the cue via golem propose.

## Step 6: Consolidation

Goal: sleep does more than decay.

- [ ] Merge near-duplicate fragments into prototype fragments carrying a
      count and exemplar pointers.
- [ ] Hot and cold tiers: working set in RAM, archive in sqlite, promotion
      back to hot on recall.
- [ ] Replay: during sleep, re-fire the day's high-weight fragments
      through the linker to consolidate cross-channel links.

Done when: a week of operation produces a memory smaller than its raw
event stream that still answers the same recall queries.

## Step 7: Observability, then the editor

Goal: see the graph before building the GUI that edits it.

- [ ] `golem dump --dot` graph export and `golem chem --watch` live
      chemistry.
- [ ] Structured event log for every encode, prune, link, and proposal.
- [ ] GME (the GUI editor) starts only after the storage schema has
      survived a few weeks without changing.

## Known limitations

- A command runs synchronously on the single brain loop, so while it runs
  the loop is blocked: sensors back up on their bounded queues and other
  CLI calls get "daemon busy". Connection deadlines are sized to outlast
  the execution timeout so the triggering client still gets its result.
  Moving execution off the loop (run in a goroutine, marshal the outcome
  back) is the proper fix and is future work.
- Only simple commands execute. Anything with shell control characters
  (chaining, pipes, redirects, substitution) is refused, because an
  allowlist entry matches a command by prefix and a shell would otherwise
  run the whole compound program. Pipelines that should run autonomously
  would need to be wrapped in a script the allowlist names.

## Open questions

- A routine command is predicted away and leaves no fragment, so a tell
  can only bind to surprising moments. Whether language should focus
  attention (force-encode what follows speech on user-driven channels)
  belongs with the decision-loop work.

- Template churn: how to merge templates when the normalizer learns a
  better split for lines it already knows.
- Cross-channel calibration windows: tuned per channel class or shared.
- Whether serotonin should also gate sensor sampling rates (the GABA idea
  from the v2 table).
