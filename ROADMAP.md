# GOLEM Roadmap

Working notes for the path from the first demo (one synthetic channel,
in-RAM memory, six-second window) to the daemon the README describes.
Step labels and status checkboxes live in this file; commits describe the
change itself.

## Principles

Two commitments govern every decision below.

- The glass box. GOLEM has weights, the way a brain has synaptic strengths,
  learned by local rules. The protection is not weightlessness. It is
  legibility: every weight sits on a named edge between interpretable nodes,
  set by a local rule and traceable to the experience that set it, and the
  concept layer never holds a gradient-trained distributed vector.
  Auditability is the line that separates GOLEM from an LLM, and the test
  every other choice answers to.
- Innate engine in code, learned parameters in the graph. Each faculty is a
  fixed procedure on the floor with data that is learned, entrenched, and
  inspectable in memory above it. The grammar engine parses and realizes
  while its closed class lives in memory; the think primitive walks the
  graph while which walk to take is learned. Shrinking the code floor and
  moving the parameters into the weighted graph is the same move repeated.

## Decisions

- Form factor: background daemon plus a `golem` CLI over a local unix socket.
- Action policy: graduated autonomy. Every proposal starts behind
  per-instance confirmation. A specific learned pair can be promoted to
  auto-run after N consistent approvals; any denial, correction, or failed
  run demotes it immediately. Destructive patterns are never promotable.
- Sensors, in build order: shell commands, project file events, system
  vitals, macOS system log.
- Dependencies: small curated pure-Go libraries are allowed (fsnotify,
  modernc.org/sqlite). The cognitive core carries no frameworks and no
  gradient-trained model; neural models are permitted only as sensory
  transducers at the input boundary (a vision captioner, a speech
  recognizer), labelled as such and kept one-way into a channel, so weights
  never reach the audit trail of cognition.
- Language: an innate grammar with a learned, grounded lexicon. Words are
  pointers to amodal concepts, so forms in different languages converge on
  one concept once grounded to the same experience. GOLEM speaks about
  what it has grounded and admits ignorance otherwise. The English
  realizer comes first; the concept layer is language-agnostic from the
  start.
- The concept-frame (the structure the parser fills and the realizer
  reads): illocutionary force (statement, question, command), thematic
  role terms for agent, action, and patient, a temporal qualifier
  (after or before) with its anchor, and the list of words that resolved
  to nothing, which is what the planner turns into admitted ignorance.
  Each role term carries a concept when grounding found one and the bare
  surface phrase when it did not. The shape lives in internal/grammar.
- Senses: every modality reaches the mind through a transducer that emits
  discrete tokens on a named channel, taught ostensively by narrating with
  `tell` while it perceives. Neural transducers are allowed as sense organs
  at the edge under the glass box; agency (acting on the world) rides the
  execution gates and comes last, with untrusted channels barred from
  grounding actions.
- Conversational target: GOLEM runs itself and converses about what it has
  grounded, taking natural-language direction. Its faculties collapse into
  the conscious loop, so only start and stop stay manual. Open-domain
  fluency is not ruled out, since the brain reaches it without an LLM, but
  it is a long road, and the lever is the representation; the absence of an
  LLM was never the barrier. Importing pretrained weights to chase open chat
  is the one move that would cost the glass box, so it stays out.
- Representation: a concept keeps its single named node as the auditable
  handle, and generalization comes from a sparse, interpretable feature
  signature hung underneath it, never from replacing the node with an opaque
  distributed pattern. Similarity is feature overlap; provenance and the
  glass box survive at the concept layer, and only the learned feature layer
  is read by interpretation. Concept cells over feature populations, the
  brain's own arrangement.

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
- [x] Automatic cueing: an observation whose template contains a
      graduated cue (whole-word match) triggers the decision loop on its
      own, behind the same conflict veto and execution gates, with a
      per-cue cooldown against storms. Action echoes and speech never
      cue; the operator keeps golem propose.
- [x] Oxytocin as trust: denials and failed runs lower it globally and it
      recovers slowly; while it is low the graduation bar rises for every
      pair in proportion, never dropping below the configured N.

Done when: at least one boring real pair (for example a log rotation) has
graduated and runs unattended with a full audit trail.

Status: the loop is built and demonstrated end to end (golem demo decide
and golem demo execute): grounding, graduation under a trust-scaled bar,
the ACC conflict veto, the denylist, real execution under three gates,
failed-run demotion, outcome feedback, and automatic cueing from
observations. Execution ships off by default.

## Step 6: Consolidation

Goal: sleep does more than decay.

- [x] Merge near-duplicate fragments into prototype fragments carrying a
      count and exemplar pointers. Three occurrences of one shape collapse
      at sleep: the newest survives as the prototype with the group count,
      the sharpest stays as a linked exemplar, and absorbed edges fold
      onto the prototype with summed weights.
- [x] Hot and cold tiers: working set in RAM, archive in sqlite, promotion
      back to hot on recall. Encodes enter hot, recall answers hot first
      and promotes archive hits, eviction is least-recently-touched under
      a fixed budget, and sleep clears the set because consolidation
      rewrites the archive underneath it. Retrieval practice still writes
      through, and the decision loop stays on the archive path.
- [x] Replay: during sleep, re-fire the day's high-weight fragments
      through the linker to consolidate cross-channel links. The day's
      strongest moments re-fire chronologically; cross-channel pairs
      within a five-minute window bind, and existing stronger edges are
      kept. golem demo consolidate shows one day merging and weaving.

Done when: a week of operation produces a memory smaller than its raw
event stream that still answers the same recall queries.

## Step 7: Observability, then the editor

Goal: see the graph before building the GUI that edits it.

- [x] `golem dump --dot` graph export and `golem chem --watch` live
      chemistry. Dump now carries the link graph on the wire; the dot
      output renders with graphviz as-is.
- [x] Structured event log for every encode, prune, link, and proposal:
      one JSON object per line in events.jsonl, plus replay, merge,
      sleep, approval, denial, and autonomous entries. Size-capped with
      one rotated generation, so the trail cannot quietly eat the disk.
- [ ] GME (the GUI editor) starts only after the storage schema has
      survived a few weeks without changing. The concept layer changed it
      on 2026-06-11 (fragments gained kind, lang, and category; links
      gained a rel column and a wider primary key), which restarts that
      clock. Everything since (thought operations, grammar parameters,
      abstractions, the act layer, and the feature signatures) rides the
      same columns: new kind and rel values and template conventions, no
      schema change, so the clock stands.

## Step 8: The action layer: pipelines and parallel execution

Goal: lift the two execution limits without loosening safety, by taking
the shell out of the loop and letting independent actions run at once.

Two limits stand today, and both trace to one choice. Commands run through
`/bin/sh -c`, and the allowlist matches by prefix, so the shell-control ban
is the only thing stopping a prefix match from authorizing a whole compound
program; and a single in-flight slot refuses any second action. Removing the
shell and generalizing the slot lifts both, guided by how the brain itself
is built: a parallel substrate with serial commitment.

Structured pipelines, no shell:

- [x] Parse a command into a pipeline of argv stages with explicit pipe and
      redirect structure, never handing the string to a shell.
- [x] Run the stages with Go's own piping, one stage's output wired to the
      next stage's input.
- [x] Authorize each stage's program against the allowlist and denylist on
      its own, so a prefix match can no longer clear a compound program.
- [x] Refuse what the safe subset cannot model: command substitution,
      backticks, subshells, globbing, variable expansion. "Parse into the
      safe pipeline or refuse" replaces the shell-control ban.
- [x] Commands stay plain strings everywhere upstream (teach, grounding,
      proposals); only the executor changes.

Parallel action, serial commitment:

- [x] Decision stays serial on the brain loop: the winner-take-all
      selection never races, and only committed execution fans out.
- [x] Bounded concurrency: committed actions run as goroutines up to a cap;
      every outcome still marshals back through one channel, so habit,
      audit, and feedback bookkeeping stay single-threaded.
- [x] Effector exclusion: two actions that touch the same resource (a file,
      a port, a working directory) serialize; independent ones run free.
      This is the brain's "one hand cannot grab two things," generalized
      from limbs to resources.
- [x] Stop settles every in-flight command before the final consolidation.

This is the parallel-processor, serial-arbiter, single-threaded-reabsorption
pattern in miniature, the same shape the later thoughts-as-goroutines work
will apply to cognition itself.

Done when: a grounded pipeline such as "make test | tee log" runs with each
stage authorized and no shell involved, two independent allowlisted actions
run at once while two that touch the same file serialize, and the audit
trail stays coherent through both.

Status: built and demonstrated end to end (golem demo pipeline). The
modeled subset is words with literal quoting, pipes, one input redirect
on the first stage, one output redirect on the last, and a leading ~/
expanded to home; everything else is refused with the reason audited,
and a pipeline succeeds only when every stage exits zero. Claims for
exclusion are the redirect files plus every non-flag argument, string
equality after path cleaning: deliberately conservative, since a false
conflict only serializes. Approved and proposed actions queue behind a
held claim (bounded, FIFO, dropped with an audit line if the daemon
stops first); reflex cueing skips instead of queueing, so a stale
observation can never fire a habit late. Both done-when criteria hold
in tests, with overlap and serialization asserted by wall clock and by
the run table.

## Step 9: The concept layer and grammar

Goal: GOLEM understands and speaks about its own experience, grounded, and
without learned weights.

Words began as whole-sentence blobs matched by substring. This step added
the layer the brain has: an amodal concept that sits between the word and
the experience, so meaning accrues per concept across episodes and across
languages. The existing episodic memory, decision loop, execution,
chemistry, tiers, replay, and observability all keep running; the
language layer is a new region wired into them, and `tell` was extended
rather than replaced, so verbatim recall survives.

Architecture: one graph, three node kinds and typed edges.

- A node gains a kind: episodic (a moment, what exists now), word (a
  surface form, language-tagged), concept (an amodal hub, no language).
- An edge gains a relation: temporal (co-occurrence, existing), successor
  (directed order), names (word -> concept, the pointer), grounds
  (concept -> episodic, what instantiates it).
- Many words name one concept, so "apple" and the Turkish "elma" converge
  on the same node once both ground to the same experience. Cross-lingual
  synonymy falls out with no special machinery.

The organs, each mapped to a brain region:

- [x] Concept layer: concept nodes plus names and grounds edges. Recall
      and grounding read through it, so paraphrases that share content
      concepts land together (the semantic hub, anterior temporal lobe).
- [x] Directed successor edges: asymmetric binding by spike timing, so A
      before B strengthens A -> B more than the reverse. This gives order
      to both "what follows make test" and word order (hippocampal
      sequence; the replay pass already walks the day in order).
- [x] Persistent workspace: a decaying activation pattern held across
      ticks and turns, so dialogue reference ("that one", "do it again")
      resolves to the warmest concept (working memory, dlPFC). It is also
      the Global Workspace substrate and the seat of a future inner loop,
      so it pays off more than once.
- [x] Lemma category: each concept carries a syntactic category, derived
      from its grounding role where possible (a concept in command
      position is verb-like, a recurring sensory object is noun-like),
      with a tiny innate seed for function words.
- [x] Realizer and parser: one innate grammar run both directions.
      Forward renders a concept-frame into a sentence (Broca, the compiler
      back end). Backward parses a tell into a concept-frame and reads its
      force as statement, question, or command (Wernicke, the front end).
      A message planner above the grammar chooses what to say, and is
      where GOLEM admits it does not know, or asks when curious.

Concept genesis: concepts are born during consolidation. That is the same
sleep pass that already merges prototypes, and the place episodic memory
distils into semantic memory (complementary learning systems). The first
content concepts bootstrap from recurring experience with no seeding;
the innate seed turned out to be needed only for the closed class of
function words (core knowledge).

Chemistry keeps all of its existing roles and gains new ones: dopamine
gates how readily a word bonds to a concept, serotonin paces concept
formation at sleep, oxytocin shapes whether speech asserts or hedges, and
a future acetylcholine knob decides whether a naming bond is hard or soft.
New chemicals can be added later and each finds a behavior to bend.

Forced deviations from biology, each flagged:

- Innate grammar, learned lexicon. The grammar ships built in; only which
  words point to which concepts is learned. Forced by no weights and no
  corpus; the brain-faithful cover is nativism (universal grammar, core
  knowledge). This is the load-bearing bet of the step.
- An innate function-word seed, because the grammatical scaffold cannot
  be learned from a handful of examples.
- Role-derived syntactic category, because part of speech cannot be
  induced from a handful of examples (syntactic bootstrapping).
- No phonology; straight to orthographic word forms, since input is text.

Scope and order: GOLEM speaks about what it has grounded, and outside that
it says it does not know and may ask. The English realizer comes first;
the pointer layer is language-agnostic from day one, so Turkish word forms
can attach to concepts before a Turkish realizer exists.

Done when: GOLEM answers "what usually happens after make test" with a
constructed sentence that names the successor command, and a word taught
in a second language recalls the same concept as its English synonym.

Status: built and demonstrated end to end (golem demo language): one
graph with episodic, word, and concept nodes under typed edges; order
woven forward-only while awake and during replay; concepts born at sleep
under a serotonin budget and grounded to their episodes; words bonded to
concepts at a dopamine-gated rate, with English and Turkish forms
converging on one node; the workspace resolving "do it again"; and the
grammar answering after, before, and what-is questions, hedging under low
oxytocin, admitting ignorance, and asking about unknown words when
dopamine is high. Both done-when criteria hold in tests. Naming was
deliberately greedy at first (bystander words bonded too), with the gap
to a concept's firmest name spoken as a tentative pointer ("may name");
cross-situational naming later replaced the greed with reliability
ratios. A spoken command stages a
proposal behind the usual approve gate and never runs by itself; golem
propose remains the lever that can fire a graduated pair autonomously.

## Step 10: Full English and Turkish grammars

Goal: grow the innate grammar from a working skeleton into grammars that
cover GOLEM's own speech in full, English first and then Turkish, one
shared frame read both directions.

Today the realizer is a compositional clause builder over a handful of
verb arms with no morphology underneath, and the parser collapses every
wh-word onto two handlers. It says the shapes the demo walks and degrades
outside them. This step closes that gap. The three correctness leaks found
in review are already fixed (a denial no longer realizes as its opposite,
"do it again" follows the latest mention, and a weakly-bonded name is
spoken tentatively); what remains is coverage.

Build, not buy: there is no pure-Go surface realizer to adopt, the mature
ones (SimpleNLG, OpenCCG) are JVM and one ranks output with n-gram model
weights, and none expose the limbic hedging that is GOLEM's point.
Morphology stays hand-rolled with no external grammar or inflection
dependency.

English, the engine:

- [x] A tense and aspect feature on the frame, parsed and realized, so "X
      followed Y" reads and speaks distinctly from "X follows Y".
- [x] Subject-verb agreement computed from person and number, not a blind
      "-s"; a number feature on terms so a plural subject agrees.
- [x] Noun number on a rule table with the irregulars, so subjects and
      counts inflect.
- [x] Article selection: a or an by the following sound, "the" for the
      definite, none where bare is right.
- [x] General negation: do-support across verbs and parse-side detection
      of not, never, and no, building on the denial fix already in place.
- [x] Wh-question typing: who, what, which, when, where, why, and how carry
      distinct meanings and route to distinct answers, including a real
      "which command follows X" that no longer falls back to a definition.
- [x] Constituent parsing past the single clause: a second temporal clause,
      coordination, and modifier attachment, so extra relators stop folding
      into the anchor span.

Turkish, the second language:

- [x] Word order as a parameter (SOV), not an English SVO assumption baked
      into the builder.
- [x] Agglutinative morphology: case, possessive, and tense suffixes
      stacked on the stem.
- [x] Vowel harmony on the suffixes.
- [x] The pointer layer already lets Turkish words name concepts; this adds
      the realizer that answers in Turkish, chosen by the language the
      question arrived in.

Done when: GOLEM inflects correctly across tense, number, and agreement in
a constructed English answer, never asserts the opposite of a negated
input, and answers a Turkish question in Turkish about the same concept an
English question reaches.

Status: built and demonstrated end to end (golem demo language). English
conjugates from a table of principal parts with agreement computed from
person, number, and tense; nouns pluralize on a rule table with the
irregulars; articles follow the sound; negation takes do-support
everywhere; the interrogatives route to distinct answers (order,
definition, time, channel, count, and honestly admitted gaps for who and
why); and "after X and before Y" parses as two qualifiers the planner
intersects. Turkish detects from its closed class, parses SOV with case
suffixes stripped from anchors, and realizes with suffix morphology
under twofold and fourfold vowel harmony with voicing assimilation,
apostrophes on foreign anchors, and a clock time harmonized by the
spoken form of its last digit. Harmony runs on spelling, the same
no-phonology deviation flagged for English. All three done-when criteria
hold in tests. Constituency covers a second temporal clause and
coordination in qualifiers and answer subjects; relative clauses and
clause embedding beyond the known-gap idiom stay out of scope, and the
Turkish surface is the answer shapes GOLEM speaks rather than open
conversation.

## Step 11: Cross-situational naming

Goal: stop greedy naming from bonding a word to everything it once kept
company with, so the lexicon points where experience actually points.

On real noisy channels the naming layer drifts. A word said once into a
busy stretch ("disk space low" spoken amid a flurry of vitals) bonds to
every concept in the window, so "disk", "low", and "space" each end up
naming half a dozen unrelated concepts; and a question asked often enough
("what usually happens after disk space low") recurs into a concept of its
own. Three fixes, smallest first.

- [x] Keep utterances out of concept genesis. A tell is language about the
      world, so its shape should mint words that name world-concepts and
      never become a concept itself. Exclude the tell channel (and any
      channel that is not lived experience) from ConceptCandidates in
      internal/memory/semantic.go, the same exclusion the planner already
      applies when it answers. A question can then no longer distil into a
      thing.
- [x] Weigh a name by reliability. The weight in growConcepts
      (internal/daemon/concept.go), namesBaseWeight + namesDopamineGain *
      dopamine + namesRepeatBonus * (episodes - 1), rewards how often a word
      and concept co-occurred, so a word present once near six concepts
      earns a real bond to each. Replace it with a cross-situational
      measure: roughly P(concept | word), the share of the word's
      co-occurrences that land on this concept, optionally blended with
      P(word | concept) for the other direction. A promiscuous word splits
      its mass across its targets and bonds weakly to each, while a word
      paired with one concept nearly every time keeps almost all its mass
      there. Extend NamingPairs (internal/memory/semantic.go) to return the
      per-word and per-concept co-occurrence totals the ratio needs; the
      inputs are counts already in the graph, so it stays weight-free. This
      is the textbook cross-situational word-learning result, and it makes
      the tentative-speech hedge fire on real contrast.
- [x] Weaken a bond on negative evidence. A reliability weight computed at
      consolidation still only grows or decays uniformly. Add the
      anti-Hebbian half: when a word recurs and its concept is not active in
      the window, weaken that word-to-concept bond, since the pairing was a
      coincidence. This is heterosynaptic depression, the negative side of
      the same cross-situational learning, and the most invasive of the
      three, so it lands last.

Forced deviation, flagged: reliability is computed from co-occurrence counts
at consolidation rather than learned online by a model, the same
counts-over-weights stance the rest of memory takes.

Done when: on the live syslog and vitals channels a word that kept company
with one shape names that concept firmly while a word scattered across many
names each weakly or not at all, "what is disk" no longer answers with a
question that became a concept, and a bond formed by a one-off coincidence
fades as the word recurs elsewhere.

Status: built, all three fixes. Genesis, reground, and the naming ratio
all exclude the tell channel, and sleep drops any leftover grounds edge
into an utterance, so question-concepts already in a live memory die of
disconnection on the next night. A bond's weight is the smoothed share
of the word's lived company that lands on the concept, blended with the
share of the concept's company that is the word, scaled by dopamine and
set at each consolidation in both directions. Negative evidence acts
between nights too: a recurring word whose window holds active concepts
that exclude a bonded one loses a step of that bond, while a window
with no active concepts is treated as no evidence at all. The
tentative-speech hedge now fires on real reliability contrast.

## Step 12: The inner loop, then thoughts as goroutines

Goal: close the self-referential loop so GOLEM perceives its own active
thoughts, then make thinking concurrent. This is the frontier the rest of
the system was built toward, the parallel-substrate and serial-commitment
pattern the action layer proved, now applied to cognition.

Today every cognitive act (encode, spread, decide, plan, recall, sleep)
runs synchronously on the brain loop, and workspace activation is seeded by
perception and decays without ever feeding back. GOLEM thinks one thought
at a time, in lockstep with perception, and never about itself. The reframe
that makes this tractable: the inner loop is replay run while awake. The
replay engine (internal/daemon/replay.go) already walks remembered moments
and weaves links during sleep; the inner loop is that same engine gated by
quiet wakefulness, with its product fed back into the workspace instead of
only strengthening edges. Two phases, the first inline and low-risk, the
second its concurrency refactor.

Phase one, the inner loop (inline, the self-reference):

- [x] Think one step on idle ticks. When input is quiet and sleep pressure
      is building but sleep has not yet triggered (the brain's default-mode
      condition, mind-wandering when not task-engaged), take the warmest
      concepts in the workspace (internal/daemon/workspace.go), spread one
      hop through the link graph, and feed the result back into the
      workspace. Activation chains across ticks instead of dying when a
      function returns; the chaining is thinking and the feedback is the
      self-reference. Bound it to one step per tick, and let workspace decay
      carry an unreinforced chain back to nothing.
- [x] Remember only surprising thoughts. Re-enter each thought as an
      internal observation on a thought channel through the same surprise
      gate perception uses, so a novel internal combination is encoded (an
      insight) and a routine spread stays transient. Memory does not flood,
      and GOLEM now perceives its own activity, which is the start of a
      self-model.
- [x] Keep it undirected at first. Free association along learned edge
      weights, since GOLEM has no explicit goal yet. Coherence comes from
      the spread following the strong paths, which are the regularities
      GOLEM actually lived, which is exactly why the naming graph must be
      cleaned (Step 11) before the loop runs over it.

Phase two, thoughts as goroutines (the parallel specialists):

- [x] Move the specialists off the loop. Run the cognitive processes as
      goroutines, a spreader, a predictor, a decider, a recaller, each
      reading a read-only snapshot of the workspace and posting a candidate
      back. The loop arbitrates which candidate ignites and broadcasts, the
      winner-take-all gate the basal ganglia already is here.
- [x] Preserve the single-threaded-memory invariant by construction.
      Specialists compute on snapshots and never touch the store; they post
      through one channel and the loop applies every mutation
      single-threaded, the snapshot-in, candidate-out, loop-commits pattern
      the execution layer already uses (the execDone path in
      internal/daemon/daemon.go). That is what keeps concurrent thought from
      racing the graph.
- [x] Gate and bound the thinking. Specialists run when perception is quiet
      and yield under load, capped in flight, so the inner life never
      starves sensing or floods the loop.

Forced deviations, flagged: the specialists are hand-coded modules rather
than emergent ones, the same innate-machinery, learned-content stance the
grammar takes; and concurrent thought is nondeterministic, so tests assert
invariants (memory never races, every chain damps, a surprising thought is
encoded) rather than exact transcripts.

Done when, phase one: with input quiet, GOLEM extends a warm concept into a
short chain of associated concepts across successive ticks, encodes a
thought only when it is surprising, and a later recall shows it remembers
having thought it. Done when, phase two: the same chains are produced by
specialist goroutines arbitrated on the loop, the store shows no race under
concurrent thinking, and thinking yields to perception under load.

Status: built through both phases and demonstrated (golem demo think).
The default-mode condition gates rounds: calm, input-silent, sleep not
yet fired, something warm. Association walks a concept's grounded
episodes along temporal and successor edges to the concepts of what
they kept company with; each committed step hands warmth on at a
discount and fatigues the seed, so chains move and then damp. A step
re-enters perception on the thought channel through the standard
surprise gate; thoughts never reset the quiet timers, never cue
habits, and never distil into concepts. Four specialists (spreader,
predictor, recaller, decider) compute concurrently on an immutable
loop-built snapshot and post candidates through one channel; the loop
arbitrates winner-take-all and commits single-threaded. Rounds are
single-flight, epoch-stamped so sleep invalidates in-flight thought,
and discarded whole when perception turns busy. The decider only ever
thinks a command runnable. Both done-when groups hold in tests, with
the race detector refereeing thinking against live perception.

## Step 13: Emergent thought-types

Goal: stop naming the kinds of thought in code. Hand-code the ability to
think, and let which kind of thought GOLEM spawns emerge from its own
graph, chemistry, and concepts.

Context: the inner loop runs four hand-named specialists in
internal/daemon/think.go, but all four are one primitive at four settings.
Spread walks association edges by weight, predict walks successor edges by
weight, recall walks grounding edges by noradrenaline, deliberate walks
grounding-to-command edges by saliency: every one is "walk relation R from
the seed, rank the destinations by signal S." Collapsing them turns a fixed
name set into thought-types drawn from the graph's own structure, the same
innate-floor, emergent-above-it shape language and action already have.
Thought has to bottom out in an executable operation, so the primitive is
the floor and everything above it is open.

- [x] Collapse the specialists into one primitive, walk(R, S): traverse
      relation R from the seed and rank destinations by signal S. R ranges
      over the edge relations the graph holds (temporal, successor, names,
      grounds), S over the signals already tracked (weight, noradrenaline,
      recency, dopamine-saliency). Behaviour-preserving: the four current
      thoughts become four (R, S) settings and the think demo still forms
      its insight. The thought-type set is now the cross-product, so a new
      relation or signal yields a new kind of thought with no new code.
- [x] Let state select the primitive instead of running a fixed panel.
      Chemistry and the warm seed choose which (R, S) fires: high
      noradrenaline toward grounding-by-stress, high dopamine toward
      deliberation, calm toward free association. The kind of thought now
      emerges from mood and what is warm.
- [x] Ground thought-types by their fruit. An open operation space is
      mostly noise, so turn the operant loop inward: a thought that lands
      somewhere surprising or rewarded reinforces the (R, S) that produced
      it, the way dopamine reinforces a useful action. GOLEM learns which
      of its own operations are worth running.
- [x] Name operations with concepts (the mental-teach). As teach grounds a
      cue to a command, ground a concept to a (R, S) operation, so a word
      can name a kind of thinking ("worry" for grounding-by-stress,
      "expect" for successor), and the names become learnable and
      composable.

Hard parts, flagged: composition (a plan is predict-then-deliberate, a
chain of primitives, harder to learn than a single operation) and the
selection pressure above, without which open thinking is noise. Forced
deviation: the primitive and the signals are the innate floor, the same
innate-machinery, learned-content stance as the grammar and the action
layer.

Horizon: GOLEM already perceives its thoughts on the thought channel. Once
thought-types are concepts, it can learn words for its own operations by
watching which (R, S) produced which thought-moment, a vocabulary of its
own mind and the start of real metacognition.

Done when: the four named specialists are gone, replaced by walk(R, S)
selected by state; the think demo still forms an insight; a thought-type
that repeatedly bears fruit is preferred over one that does not; and a
concept can be taught to name an operation.

Status: built and demonstrated (golem demo think). The named specialists
are gone; every setting of walk(R, S) computes each round on one
snapshot and the arbiter weighs the candidates by the chemistry's
appetite for the signal (alarm leans stress and recency, reward
chemistry leans saliency, calm leans lived weight), by an innate
loudness prior per pairing, and by learned worth. An encoded insight
reinforces the operation that produced it and takes a grounds edge from
the operation node, so the trail shows which operation produced which
thought; a routine thought wears its operation a little; the worth lives
as the operation node's weight, bounded on both sides and visible in
golem dump. golem teach --mind worry grounds stress grounds a concept to
an operation and bonds the word to it, and the policy reads the
concept's workspace warmth as a lean toward its operation, so saying the
word steers idle thought. All four done-when criteria hold in tests,
with the race detector still refereeing rounds against live perception.

## Step 14: Grammar into memory

Goal: move the grammar's parameters out of code and into the weighted,
auditable graph, so the grammar is inspectable, learnable, and entrenched
rather than frozen.

Today the engine and its data both live in code: internal/grammar holds the
parser and realizer procedures and also the closed-class category table, the
morphology rules, the word-order setting, and the verbs' principal parts.
The engine is the innate floor and stays. The data should be learned
parameters in memory, following the principle above.

- [x] Represent grammar parameters as memory nodes and edges: a function
      word and its category, a suffix and its rule, the word-order setting
      for a language, a verb and its principal parts. Each is a fragment or
      edge in the same store as everything else, readable by dump and recall.
- [x] Read the engine from memory instead of from constants: the parser
      looks up a token's category in the graph, the realizer reads
      morphology and order from the graph. Cache into the hot tier so
      per-parse lookups stay cheap.
- [x] Entrench against forgetting: seed grammar nodes at maximal weight with
      a forgetting floor far below normal, reusing the semantic-node
      forgetting law (words and concepts already die only by disconnection,
      not by nightly decay). Native grammar should be the hardest thing to
      lose.
- [x] Resolve conflict by weighted competition: when learned usage disagrees
      with a seeded parameter, the stronger wins, the same mechanism naming
      already uses, so grammar bends to real usage without a special case.
- [x] Seed the parameters through the trainer, not by hand-writing the
      database: the grammar seed is the first curriculum, written into the
      starter memory, so the seeding path is the one all bootstrapping uses.

Forced deviation, flagged: the seed is strong innate priors in a learnable
substrate, which shifts the stance from frozen Universal Grammar toward
entrenched, usage-based grammar; the engine stays the innate floor.

Done when: a function word's category and a verb's principal parts can be
read from golem dump, the realizer and parser run from those memory entries
with no behaviour change, grammar survives a sleep that would prune ordinary
memory, and a sustained contrary usage can re-weight a seeded parameter.

Status: built. First boot trains the curriculum into starter memory
through the standard encode path: every closed-class form with its
category, every event-verb form with its predicate and tense, the
verbs' principal parts and Turkish form sets, the word-order setting
per language, and the Turkish suffix recipes (DAn, (y)I, DIr, DA) that
an innate interpreter realizes under harmony and assimilation. The
engine compiles those nodes into a RAM working copy at boot and again
at each wake, so per-parse lookups stay map hits. Grammar nodes are a
kind of their own that nightly decay never touches; the only force on
them is the competition: a form deliberately labeled (tell --about
mints closed-class words too) accrues naming mass, the seed erodes a
step per contested night and heals when the usage fades, and the
compile drops a parameter the lexicon has out-weighed, so the form
parses as content from then on. English plural irregulars, article
sound rules, and number words stay in the engine as its innate floor.
All four done-when criteria hold in tests.

## Step 15: Senses and embodiment

Goal: give GOLEM more bodies. Each new sense is a transducer that emits
discrete tokens on a named channel; the mind grounds them with the
machinery it already has, taught ostensively by narration. Ordered easiest
and safest first.

- [x] Voice out. Render the realizer's sentences to speech (pipe to the
      system text-to-speech). Pure output, the articulatory motor system, no
      change to cognition. The cheapest win and the one that makes GOLEM
      audibly present.
- [x] File-content channel. A sensor that reads or tails files it is given
      and emits their lines as observations, distinct from the existing
      file-event sensor. Already symbolic, no weights.
- [x] Web-text channel. A fetch of a URL that returns the page's stripped
      text as observations on a web channel. Already symbolic. Mark the
      channel untrusted: low weight, barred from grounding actions, so an
      adversarial page cannot become a habit.
- [x] Screen and accessibility channel. Emit UI events from the
      accessibility tree as a symbolic stream, so GOLEM perceives what is on
      screen without pixels.
- [x] Eyes: a vision transducer. A captioner that turns camera or screen
      frames into scene tokens ("a person", "a red car", "a door opens") on
      a vision channel. The first neural sense organ; weights at the edge,
      flagged. Segment the continuous stream on surprise, since a scene
      change is a prediction error worth a moment.
- [x] Ears: a speech transducer. Speech-to-text that turns spoken words into
      tokens on the tell channel, so you can teach GOLEM out loud. The second
      neural organ, paired with voice out to close a spoken loop.
- [x] Agency: acting on the open world. Browsing and other actions as a
      perception-action loop behind the three execution gates and the
      denylist. Last and most bounded, because it is the largest safety
      surface; untrusted perception from the web still cannot ground an
      action.

Ostensive teaching, flagged: a word taught against a rich channel binds by
cross-situational reference, so a thing cannot be named in one showing;
narration across many situations is what fixes meaning, and Step 11 is what
makes it converge. Forced deviation: the vision and speech transducers carry
model weights, permitted as sense organs at the boundary under the glass box.

Done when: GOLEM speaks its answers aloud; a file's contents and a fetched
page ground as experience on their channels; and a thing shown on a neural
channel and named across several showings becomes a concept reachable by its
word.

Status: built. Voice out queues the realizer's sentences to the system
text-to-speech, one at a time, off the loop. Whether it speaks and in
which system voice are runtime state under golem voice, persisted to
config and recorded in the event trail. The
reading channel tails configured files from their current end and emits
appended lines named by file; the web channel fetches configured pages
on a poll, strips them to text lines with a hand-rolled reducer, and
ships untrusted; the screen channel reports frontmost-application
changes through System Events. Untrusted is now a policy with teeth: an
untrusted moment encodes at half weight, can never fire the reflex arc,
and never anchors the grounding of an action, all asserted in tests.
The neural organs stay outside the binary by design (weights at the
edge, one-way into a channel): a captioner pipes into golem observe
vision and a speech recognizer into golem tell, where the surprise gate
already segments the stream and cross-situational naming grounds the
narration; the spoken loop closes against voice out. Agency rides the
existing gates: an allowlisted fetch can act on the web while the web
sense perceives it, and the trust policy keeps what comes back from
ever becoming a reflex.

## Step 16: Abstraction by constellation

Goal: let abstract concepts form by the same machinery as concrete ones, by
distilling a recurring constellation of concepts instead of only a recurring
sensory shape.

A concept is born today from a sensory shape that recurred. That can never
mint "inflation" or "capital", because an abstraction has no recurring
sensory shape; it is grounded instead in the stable cloud of concepts a word
keeps company with. Extend genesis to that case; the cloud still bottoms out
in concrete primitives, so the grounding chain stays intact.

- [x] Detect a stable conceptual constellation: a word or context whose
      co-activated concepts recur across situations with the same core, the
      semantic analogue of a recurring template. Reuse the cross-situational
      reliability from Step 11 to find the consistent core and discount the
      drift.
- [x] Mint an abstract concept grounded in that constellation: a concept
      node whose grounding edges point at the concepts of its constellation,
      the way a concrete concept's point at episodes, so recall and the
      inner loop reach it the same way.
- [x] Keep the chain auditable: every abstract concept traces, through its
      constellation, to the concrete primitives underneath, so "inflation"
      can be unfolded to the experiences it stands on.

Dependency, flagged: the full payoff needs deeper language comprehension
than the answer-shaped realizer has today, because receiving an abstraction
and parsing the company it keeps is the larger half of the work (tracked as
a known limitation). The genesis mechanism is buildable now and grows more
powerful as comprehension deepens.

Out of scope here: relational abstraction (capital-of, cause-of) needs
reifying a relation into a concept by structure-mapping, a further mechanism
left to its own step (tracked as a known limitation).

Done when: a word used across many situations with a consistent conceptual
neighbourhood, and never tied to a single sensory shape, distils into a
concept that recall reaches through its constellation, and that concept
unfolds to the concrete concepts it was built from.

Status: built. Genesis runs right after the naming pass on the same
cross-situational shares: a word spoken across enough situations whose
company holds two or more concepts above the core share, none of them
dominant, distils into an abstract concept that speaks as the word,
grounds each core concept at its reliability share, and is named by the
word at a firm bond. The shares rewrite each night while the
constellation stands, so the grounding stays current and auditable down
to the episodes underneath; a dissolved constellation stops being
refreshed and the concept dies of disconnection as its edges fade.
Recall reads straight through it (word to abstraction to constellation
to moments), the inner loop walks its grounds onto warm concepts, and
window-based naming depression exempts it, since an abstraction's
referent is never directly in a sensory window. The done-when holds in
tests, with the dominance and recurrence refusals asserted alongside.

## Step 17: Faculties become acts

Goal: collapse the CLI into the conscious loop. GOLEM's own faculties
(recall, ground, consolidate, introspect, speak, propose) become selectable
acts in the same operant space as the walk(R, S) thought primitive, so the
loop drives them itself and a request can trigger them. Only start and stop
stay manual.

The gate already selects external commands, chemistry and the workspace
already bias it, and outcome already reinforces it; Step 13 already turned
thinking into a selectable operation with learned worth and a teachable
name. This widens that one move to every faculty. GOLEM already drives
sleep, encoding, naming, and autocue proposals on its own, so the pattern
exists and this extends it.

- [x] One action space spanning external commands and internal faculties,
      each a primitive the basal-ganglia gate can select.
- [x] Selection by the machinery already in place: chemistry appetite,
      workspace warmth, and the operation's learned worth, the way
      walk(R, S) is chosen now.
- [x] A concept can name any act (the mental-teach generalized), so a word
      steers a faculty.
- [x] The CLI verbs become handles on those acts, not the only way they
      fire; the loop fires them when its state calls for it.

Forced deviation, flagged: the act primitives are a hand-listed innate
floor, the same innate-machinery, learned-selection stance the grammar and
the think primitive take.

Done when: GOLEM recalls, grounds, or consolidates unprompted when its state
calls for it, the same act a CLI verb triggers, and a taught word can invoke
any of them.

Status: built and demonstrated (golem demo think). Six acts (recall,
ground, consolidate, speak, propose, introspect) compute on the same
snapshot as the walks and the same arbiter ignites one winner per quiet
round: signal times innate gain, times the chemistry's appetite (alarm
for recall, reward for grounding and proposing, sleep pressure for
tidying, trust for speaking, calm for introspection), times learned
worth, times the warmth of any taught name. A winning act runs the
function its CLI verb runs (recall shares the verb's seed path,
consolidate is the sleep verb's cycle, propose stages behind the same
pending slot and never executes), audits the firing, and feeds the
operant loop, so a fruitful faculty is preferred the way a fruitful
thought-type is. Each act cools on its own refractory; teach --mind
<word> act <name> names one. All three done-when criteria hold in
tests.

## Step 18: Natural-language control and conversation

Goal: plain language becomes both control and conversation. A request
resolves to an intent over Step 17's action space, and the planner holds a
multi-turn exchange instead of answering one shot.

Comprehension is answer-shaped today: a few frames map to answers. This
widens it to intents ("recall the dog" fires recall, "remember this as a
bell" grounds, "what were you thinking" introspects), and turns the planner
into a turn-taker. It stays bounded by grounding, interpreting requests about
its own world and faculties, not open-domain prose.

- [x] Parse a wider range of input into intents, mapped onto the action
      space, and admit incomprehension instead of bluffing when nothing
      parses.
- [x] The workspace carries the topic across turns; reference like "it",
      "that", and "the other one" resolves against it.
- [x] Decide what to say next, including asking a follow-up, not only
      answering the immediate question.

Done when: a plain request drives the matching internal act, and GOLEM
sustains a multi-turn exchange about a grounded topic, holding the thread.

Status: built. Faculty verbs in the closed class open intents over the
act space: "recall the disk" runs the recall act and speaks the moment
it found, "remember this as a bell" names the warm referent (the ground
act), "tidy" runs the consolidation the sleep verb runs, and "what were
you thinking" reads the inner loop, each landing on the audit trail as
an asked act. Anything that maps to nothing is admitted.
The thread lives in the workspace: a spoken answer's subject becomes
the warmest referent, resolving "it" or "the other one" refreshes the
referent it picked, and a pro-form question or "do it again" follows
the topic across turns. When a question leaves a word pointing at
nothing the answer did not already speak, high dopamine appends the
follow-up question, so the planner decides what to say next rather
than only answering. A curriculum top-up at boot seeds newly innate
forms into an older memory without touching learned weights. Both
done-when criteria hold in tests.

## Step 19: Drives and goals

Goal: give the loop reasons to act beyond chemical reflex, so it can direct
itself with intent and not only react.

Today the drives are chemical: surprise encodes, pressure sleeps, dopamine
asks. Genuine self-direction needs a goal it can hold and pursue. This is
the deepest and least-charted piece, so it is staged from cheap drives
toward real goal-pursuit, with the far end treated as open research.

- [x] Cheap drives first: a relevance drive that recalls what fits the
      moment, a tidiness drive that consolidates clutter, building on the
      curiosity drive already present.
- [x] Drives bias the action gate, so GOLEM acts for a reason, and the
      reason is on the trail.
- [x] A goal representation the loop can hold across steps and act toward.

Flagged: the first two are reachable; a held, pursued goal is the frontier
of the frontier and stays open research.

Done when: GOLEM takes an unprompted internal act because a drive called for
it, and the drive is readable on the audit trail.

Status: built. Drives are named urges computed at each quiet round:
relevance (alarm above baseline) pushes the recall act, tidiness (fresh
encodes since the last night) pushes consolidate, curiosity (elevated
dopamine over a warm nameless concept) pushes the speak act, which then
asks what the thing is instead of remarking. A drive never acts; it
multiplies its act's priority in the same arbitration everything else
competes in, and the winning act carries the drive's reason onto the
audit trail ("unprompted (tidiness: 80 fresh moments): ..."). The goal
is one held intention: asking about a word that points at nothing
adopts a learning goal (one slot, on the trail, visible in status),
the goal drive aims the recall act at its topic on the rounds that
follow, satisfaction (the word finally names a concept, however that
happened) rewards with dopamine and clears it, and a stale goal is
dropped before it can become an obsession. Pursuit beyond learning
goals stays open research. The done-when holds in tests.

## Step 20: Sparse feature signatures

Goal: let concepts generalize, so a paraphrase or a novel phrasing lands
near what it resembles, by hanging a sparse, interpretable feature signature
under each concept. Augment the localist node, never replace it.

One node per concept is maximally auditable but blind to paraphrase: every
synonym needs an explicit link. The brain puts concept cells over
distributed feature populations; mirror that. Each named concept keeps its
node and gains a sparse set of interpretable features, and similarity becomes
feature overlap.

- [x] A sparse feature layer under the concept nodes; each concept carries a
      signature of interpretable features.
- [x] Features earn meaning from clustered experience, the one place reading
      is interpretive, where a named node's meaning is simply declared.
- [x] Similarity by feature overlap, so an input lands near the concept it
      resembles with no shared substring, and the shared features explain why.
- [x] Concept-node provenance preserved: a thought still originates from a
      named node, so everything the inner loop and the trail already show
      stays exactly as readable.

Forced deviation, flagged: the feature layer is learned and read by
interpretation, softer than a named node's declared meaning. The trade is
deliberate and confined to the feature layer, so the concept layer stays
crisp.

Boundary, flagged: this is the lever toward open-domain, but breadth still
has to be lived or told, and human-range fluency remains a long road, not a
milestone.

Done when: a word never seen, sharing features with a known concept, recalls
that concept by overlap, and the shared features can be read off to explain
the match.

Status: built. Consolidation rewrites each concept's signature whole
from its current grounding (an abstraction's through its constellation,
so the chain still bottoms out in moments): the recurring open-class
words of its shape, the channels it lives on, and the felt chemistry,
each a shared feature node under a weighted feature edge, floored,
capped, and free of normalizer placeholders. Feature nodes are kind
"feature" in the same store, visible in dump and in golem concepts as
the signature line, and they die by disconnection when their last
bearer goes. Recall reads the layer after the literal and lexical
paths: a phrasing never seen that shares enough attributes lands on
the resembled concept's experience, with the matched feature nodes
returned as mediators, so the reply shows exactly which shared
attributes carried the match. The same relation extends the think
primitive (walk(feature, S), resemblance scored in [0, 1] against the
seed's whole signature), so disconnected kin associate as "X is like
Y" while every thought still starts and lands on named nodes. The
done-when holds in tests.

## Step 21: Constructions into memory

Goal: move the sentence patterns out of code and into the weighted, auditable
graph, so what GOLEM says stops being a fixed template and becomes a learned,
varied construction. The same move Step 14 made for grammar parameters,
raised from words to whole patterns.

Grammar parameters already live in memory (KindGrammar nodes, read in
internal/daemon/grammar.go), but the sentence shapes are still hardcoded: the
realizer assembles from fixed pieces (internal/grammar/english.go), and the
act and drive layers concatenate whole templates (the "I was thinking" line
in internal/daemon/act.go, the drive's "satisfied: it names" line). Those are
the rigid parts. Structure first, then curriculum, the order Step 14 followed.

- [ ] A construction representation in memory: a form-and-meaning pattern, an
      intent or frame on the meaning side and an ordered template with typed
      slots on the form side, a new node kind beside KindGrammar, weighted
      and inspectable.
- [ ] The realizer and the act and drive layers fill a memory construction's
      slots instead of concatenating hardcoded strings. The slot-filling
      engine, the morphology, and the word order stay innate code; only the
      patterns move out.
- [ ] More than one construction per intent, so a meaning has alternative
      forms, and the choice among them reads from context and chemistry, the
      way oxytocin already picks "usually follows" over "may follow".
- [ ] Seed the first constructions through the trainer along the grammar-seed
      path, so the starter memory speaks from day one, then learn new ones
      from usage by pairing a form heard or shown with the intent it carried.

Forced deviation, flagged: a learned construction can be wrong until it is
well-trained, where a hardcoded template was always grammatical. Reliability
is traded for fluidity, earned back over many examples, with the innate
slot-filling engine the floor that keeps even a thin set of constructions
grammatical.

Done when: the "I was thinking" line and the successor and definition answers
are built from constructions read out of memory, golem dump shows them as
nodes, a second construction for one intent can be taught and then chosen,
and removing a construction changes how GOLEM says that thing without breaking
the engine.

## Step 22: Compositional generation

Goal: build a sentence by composing constructions, so GOLEM can say something
it was never given whole. This is the deepest sense of fluid, and the one the
no-weights stance makes hardest.

Even with constructions in memory, Step 21 still selects a whole learned
pattern. Real fluency composes: a clause nests in another, modifiers stack,
two constructions join into an utterance never seen as a unit. Staged late and
treated as research at its far end.

- [ ] A construction slot can be filled by another construction, not only a
      term, so patterns nest.
- [ ] The planner builds a layered message and the realizer renders it by
      recursive slot-filling, so a novel combination still comes out
      grammatical.
- [ ] Selection across the layers stays driven by context and chemistry, the
      same choice Step 21 makes, now at every level.

Flagged: open-ended compositional generation is the deep reach, where the
grounded path is least proven. Treat one level of nesting and coordination as
the milestone and arbitrary generative fluency as open research.

Done when: GOLEM produces a correct sentence built from two or more
constructions it was never given as a single template, and the composition
reads back as the constructions that made it.

## Known limitations

- Commands execute within the modeled pipeline subset. Substitution,
  subshells, chaining, globbing, escapes, and descriptor redirects are
  refused by design; a job that needs them must live in a script the
  allowlist names. Pipeline success is every-stage-zero, stricter than a
  shell's last-stage-wins, so a head that exits nonzero when its reader
  stops early (the head-closes-the-pipe case) reads as a failed run.
- Effector exclusion matches claims by cleaned string equality. Two paths
  naming one file differently can slip past it, and two unrelated commands
  sharing an argument word serialize when they could overlap; the
  conservative direction is the safe one, but it is a heuristic.
- Relational abstraction has no mechanism. GOLEM can form a concept from a
  recurring constellation of concepts (Step 16), but it cannot yet reify a
  relation itself: "capital of" or "causes", the shared structure across
  Paris-France and Tokyo-Japan, needs the analogy and structure-mapping the
  graph does not do.
- Language comprehension is answer-shaped. The realizer and parser cover the
  sentence shapes GOLEM speaks and the questions it answers; open
  comprehension is beyond them. Receiving a stated abstraction, following a
  definition, or parsing arbitrary prose is well past today's grammar, and
  it is the larger half of the abstraction work.
- Untrusted perception is a standing hazard for agency. Web and other
  open-world channels carry adversarial content. The gates and denylist stop
  it from triggering execution, but the hygiene that keeps it from polluting
  memory or grounding an action (marking channels untrusted, weighting them
  low, barring them from action grounding) is a policy to get right as the
  agency work lands.
- Feature signatures are surface-deep. The sparse layer generalizes by
  recurring tokens, channels, and felt chemistry, so a paraphrase lands
  only when it shares surface attributes with the lived shape; a true
  synonym with no shared token still needs a naming bond or a lived
  bridge. Breadth has to be lived or told, and the answer-shaped
  comprehension ceiling stands above the representation either way.
- Language generation is templated. The sentence shapes are hardcoded in the
  realizer and the act handlers, so GOLEM says a thing one fixed way and can
  neither vary the form nor combine two into one. Step 21 moves the patterns
  into learnable memory and Step 22 lets them compose.

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
