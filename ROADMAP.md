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
- Neurochemical palette: a transmitter earns a place by bending a behavior that
  nothing built already covers. Completing the set for biological fidelity is
  not by itself a reason. Dopamine, noradrenaline, and serotonin already run
  reward, salience, and sleep pressure, and oxytocin tracks trust and raises the
  graduation bar under correction. Of the rest sketched for v2: GABA is the
  lateral inhibition of Step 29, the one whose function is due now; cortisol, a
  stress signal read from processor, memory, and heat that sheds non-essential
  sensing to protect the core loop, is a real computer-side homeostasis worth
  adding when GOLEM runs ambient under load; acetylcholine, a precision signal
  that would commit a link hard rather than leave it soft, overlaps the
  learning-rate dopamine already gates and waits until that coarseness bites;
  adenosine duplicates serotonin's sleep pressure and stays out unless mood and
  sleep pressure are ever worth splitting apart.
- Self-extension: GOLEM is meant to evolve and program itself, and that
  capability is the demonstration, the real thing speaking for itself. The
  discipline that keeps it GOLEM through that growth is that meaning stays
  grounded throughout: code grounded in execution, authored memory grounded by
  teaching, every self-change traced to a grounded outcome and auditable in the
  glass box. From the runtime onward the whole of GOLEM is run inside a sealed
  container the operator places it in, the containment external rather than a
  feature GOLEM builds, with the terminal and the memory editor as separate human
  interfaces into it.

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
      abstractions, the act layer, the feature signatures, and the
      sentence constructions) rides the same columns: new kind and rel
      values and template conventions, no schema change, so the clock
      stands.

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

- [x] A construction representation in memory: a form-and-meaning pattern, an
      intent or frame on the meaning side and an ordered template with typed
      slots on the form side, a new node kind beside KindGrammar, weighted
      and inspectable.
- [x] The realizer and the act and drive layers fill a memory construction's
      slots instead of concatenating hardcoded strings. The slot-filling
      engine, the morphology, and the word order stay innate code; only the
      patterns move out.
- [x] More than one construction per intent, so a meaning has alternative
      forms, and the choice among them reads from context and chemistry, the
      way oxytocin already picks "usually follows" over "may follow".
- [x] Seed the first constructions through the trainer along the grammar-seed
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

Status: built and demonstrated (golem demo language). Constructions are a
node kind of their own beside the grammar parameters, seeded through the
same trainer and topped up into an older memory at boot, outside the
nightly fade and the lexical competition. Each carries an intent, an
ordered template with typed slots, a chemistry condition, and a weight;
the engine fills the slots while the morphology and word order stay
innate, so the conjugated verb group of a succession answer is still the
engine's and only the frame around it is learned. The introspection
report, the succession answer, and the definition answer realize through
them, with the innate realizer kept as the floor so a missing or forgotten
construction still speaks. The oxytocin split that chose "usually follows"
over "may follow" is now a choice between an assert and a hedge variant of
one intent. golem teach --say adds a construction, entrenched above the
seeds so it is the one that speaks; forgetting it reverts the line. All
four done-when criteria hold in tests.

## Step 22: Compositional generation

Goal: build a sentence by composing constructions, so GOLEM can say something
it was never given whole. This is the deepest sense of fluid, and the one the
no-weights stance makes hardest.

Even with constructions in memory, Step 21 still selects a whole learned
pattern. Real fluency composes: a clause nests in another, modifiers stack,
two constructions join into an utterance never seen as a unit. Staged late and
treated as research at its far end.

- [x] A construction slot can be filled by another construction, not only a
      term, so patterns nest.
- [x] The planner builds a layered message and the realizer renders it by
      recursive slot-filling, so a novel combination still comes out
      grammatical.
- [x] Selection across the layers stays driven by context and chemistry, the
      same choice Step 21 makes, now at every level.

Flagged: open-ended compositional generation is the deep reach, where the
grounded path is least proven. Treat one level of nesting and coordination as
the milestone and arbitrary generative fluency as open research.

Done when: GOLEM produces a correct sentence built from two or more
constructions it was never given as a single template, and the composition
reads back as the constructions that made it.

Status: built to the milestone and demonstrated (golem demo language). A
slot is filled by a layered message, an intent and a binding whose own
slots may nest further messages; the realizer composes bottom-up, each
layer selecting its construction by the same chemistry and weight a single
construction uses, and only the outermost clause is capitalized so a
nested one reads naturally mid-sentence. The full definition answer is the
first composition: the definition clause and, when experience stands
behind the concept, the count clause nest in a frame neither template
holds alone, and the count slot collapses when there is none. The
composition lands on the trail as the constructions that made it, leaves
first, so it reads back. A missing construction at any layer fails the
whole compose to the innate realizer, so removing one never speaks a
half-built clause. The done-when holds in tests; one level of nesting is
the milestone and arbitrary generative fluency stays open research.

## Step 23: Semantic features

Goal: deepen the feature layer from surface tokens to learned semantic
features, so resemblance can hold between things that share meaning without
sharing a describing word.

Step 20's features are surface-deep (tok:small, tok:brown), so a paraphrase
lands only on shared surface attributes and a true synonym with no shared
token misses. Let features be discovered, not only minted from caption
tokens: an attribute that recurs across many concepts becomes a feature in
its own right, so two things resemble each other by a learned trait instead
of a shared word.

- [x] Features discovered by clustering co-occurring attributes across
      concepts during consolidation, beside the surface tokens already minted.
- [x] A discovered feature is itself an inspectable node, read by
      interpretation (which concepts it fires for), the give already flagged
      for the feature layer.
- [x] Resemblance computed over discovered features, so similarity survives a
      change of words.

Flagged research: feature discovery is concept induction's cousin, hard and
unproven at scale; the surface-token layer stays the floor it builds on.

Done when: two concepts that share no caption token but keep similar
conceptual company resemble each other, and the shared discovered features
read back.

Status: built. Consolidation now mines a second kind of feature beside the
surface tokens: the conceptual company a concept keeps. It sweeps the whole
concept graph, sums the lived neighbors of each concept's grounded episodes
into the concepts they instantiate, normalizes to shares, and keeps a
company concept as a discovered feature only where it recurs across enough
concepts to be a shared trait rather than an incidental neighbor. Each is a
"co:" feature node like any other, visible in dump and in the concept
signature, so two concepts with no describing word in common but the same
neighborhood come to share a feature and resemble each other through it. The
done-when holds in tests, with the no-shared-token case asserted alongside.

## Step 24: A general realization engine

Goal: shrink the slot-filling engine from hand-coded English rules to a
small, language-agnostic combination primitive over declarative, learnable
knowledge, so even the engine becomes fluid and open composition falls out.
This subsumes Step 22's far end.

Today a construction's slots are filled by a procedural engine with English
morphology, agreement, and word order baked in, the innate floor. Classical
symbolic NLP offers a different shape: elementary structures that combine by
one general operation. Borrow it. Tree-adjoining grammar combines elementary
trees by substitution and adjunction; unification grammars (HPSG, LFG, Kay's
functional unification) merge feature structures. In either, the engine is
one small general mechanism and all the linguistic knowledge is declarative
data in memory.

- [x] Replace the procedural slot-filler with a general combination primitive
      (unification, or tree substitution and adjunction), the new and far
      smaller innate floor.
- [x] Move agreement, article choice, and word order out of code into
      declarative feature structures or elementary trees, weighted and
      inspectable beside the constructions.
- [x] Open composition falls out: structures combine by the general operation
      to any depth, so a novel sentence is built, and read back, as the
      structures that made it.
- [x] Keep output reliable: unify or search toward a grammatical result, and
      fail to the simpler engine instead of emitting a broken sentence.

Forced deviation, flagged: the floor shrinks and generalizes but never
vanishes; a unifier or a tree-combiner is the irreducible engine. The gain is
that the English-specific knowledge becomes learnable, inspectable data
instead of code.

Flagged research: inducing the declarative grammar from usage, and the search
cost of generation, are the unproven parts. Treat a hand-seeded declarative
grammar run by the general engine as the milestone, and induced structures as
open research.

Done when: agreement and word order are read from memory nodes rather than
code, two structures combine to any depth by the general engine, and editing
a declarative rule changes the grammar without touching the engine.

Status: built to the milestone. The general engine is one operation,
feature-structure unification, over declarative rules read from memory: a
phrase rule names its daughters' surface order (the word order) and the
features that flow between them (the agreement), and a lexical entry gives a
lemma its category and inflected forms. The engine unifies, selects each
form, and linearizes, recursing through a noun phrase nested in a clause so
two structures combine to depth, and it returns the trace of rules and
lexical items that built the sentence so the composition reads back. The
rules are ordinary grammar nodes, seeded through the trainer and editable in
the graph: changing the word-order node and recompiling reorders the clause
with no change to the engine, agreement intact. It fails cleanly on an
unknown lemma or a language it has no grammar for, so the procedural realizer
stays the reliable floor beneath it. A hand-seeded declarative grammar run by
the engine is the milestone; inducing the grammar from usage and the search
cost of open generation stay research. All done-when criteria hold in tests.

## Step 25: Goal-pursuit

Goal: a held, pursued goal, so self-direction reaches past Step 19's reflex
drives into intention that persists across steps.

Step 19 gave cheap drives (relevance, tidiness, curiosity) that bias the
action gate in the moment. A goal is a state the loop holds and works toward
over many steps, re-selecting acts until it is reached or abandoned. This is
the least-charted piece of the whole project.

- [x] A goal representation the loop can hold: a target state, a sense of
      distance to it, and a hold that survives across ticks.
- [x] The action gate weighs acts by how far they move toward the held goal,
      alongside chemistry and learned worth.
- [x] A goal is set by a drive, by a request, or by GOLEM itself, and released
      when reached or when it stops paying.

Flagged research: persistent goal-pursuit is the frontier of the frontier,
where the grounded path is least proven; nothing is promised beyond the first
held goal that visibly steers a few acts.

Done when: GOLEM holds a goal across several loop ticks, its acts visibly bend
toward it, and the goal and its progress read back on the trail.

Status: built to the milestone. The held intention from Step 19 grows a
target and a measured distance: a learning goal aims at understanding its
topic, and the distance closes from one as lived experience about it is
gathered, reaching zero when a concept finally answers to it. Each quiet tick
the gate aims the recall act at the topic, weighted by how far the target
still is; the closing distance is logged as progress on the trail, and the
goal is released when it is reached (with a reward), when it goes stale, or
when it stops making progress, so a pursuit never becomes an obsession. A
goal is set by a request (a question that left a word pointing at nothing) or
by GOLEM itself (curiosity over a warm nameless thing). A held, freely chosen
goal beyond learning stays the frontier; the done-when holds in tests.

## Step 26: Salience by information, not frequency

Goal: weight persistence, the relevance drive, and the inner loop by how
informative a node is rather than how often it has occurred, so a
high-frequency low-information node is cheap to keep and quiet to attend to
while a meaningful concept resists forgetting. This generalizes Step 11's
cross-situational reliability from the lexicon to the rest of the system.

A bug once repeated a companion caption thirty to sixty times and exposed a
flaw that was always there. Raw frequency drives three things, and all three
broke: the junk became the most durable node in memory, it pulled the
relevance drive to its ceiling, and it dominated the inner loop as the
most-connected hub, while a real concept seen thirteen times and never
revisited died. Step 11 fixed exactly this currency for naming, where a
count-based bond once let one word name everything. The same currency is
wrong for forgetting, for the drive, and for the walk.

- [x] Persistence by information: a concept's resistance to decay weights by
      the diversity of its grounding, so a thing seen many times in one fixed
      context is cheap to drop while a thing lived across varied contexts is
      durable.
- [x] Drive salience by information: the relevance drive discounts a node by
      how predictably it recurs, so a high-frequency low-information node
      cannot pull the loop above real knowledge.
- [x] The walk avoids junk hubs: damp a destination by its degree or its
      information, so the inner loop stops circling a node everything connects
      to.
- [x] Entrench what was taught: deliberately narrated or taught content
      carries a durability boost over an incidentally observed companion, so
      meaning outlives ambient noise.
- [x] A true-forget maintenance path: a command to delete a node and its
      edges, since some traces, bug-junk and sensitive data alike, must be
      destroyable rather than only demoted (today this needs database surgery).

Framing: this is cross-situational reliability generalized from the lexicon
to persistence and attention, and the tunable, information-weighted forgetting
from the design notes made concrete. The currency changes from how often to
how informative.

Done when: a node repeated many times in one fixed context is cheap to retain
and never pulls the drive above a varied concept, a taught concept survives
where an incidental companion fades, the inner loop does not circle a
high-degree hub, and a node can be truly deleted by command.

Status: built. Consolidation scores each concept by its information, the
diversity of the contexts its experience spans plus a boost when a firm name
was deliberately taught, and caches it on the node. The score drives three
things that used to run on raw count. Persistence: the experience under an
informative concept is refreshed so it outlives the night, while a node lived
in one fixed context is left to fade however often it recurred, so a taught
or varied concept survives where an incidental companion does not. Salience:
a recurring shape warms its concept in proportion to its information, so a
high-frequency low-information node cannot pull the inner loop above real
knowledge. The walk: a destination is damped by the breadth of edges that
reach it, so the loop stops circling a hub everything connects to while a
focused neighbor keeps its pull. And golem forget deletes a remembered shape
and its edges outright, the concept and its episodes alike, the true-forget
that a demotion never gave. All four done-when criteria hold in tests.

## Step 27: Ground language in attention, not only perception

Goal: let a told word bind to the warmest concept in the workspace, not only
to the percept's concept, so GOLEM can name what is in mind and not only what
is in view. This makes hierarchy teachable and opens naming the abstract and
the internal.

The day-six frontier settled a boundary: a told word grounds in the moment it
is spoken in, and a moment grounds in the percept's concept, which is always
specific. So ostension tops out at one level. A `thing` told over a dog
percept grounds in `dog`, a flat sibling of the categories, never in `animal`
above them, because no percept maps to `animal`. The fix reuses machinery
already built: the workspace holds what is active, and recall already warms a
concept. Bind the told word to the warmest workspace concept, so warming a
category and then naming it grounds the name on the category.

- [x] A told word binds to the warmest concept in the workspace, alongside
      the perceptual window, so what is attended can be the referent.
- [x] Naming a recalled or thought-about concept grounds the word there:
      recall animal, say "thing", and the name grounds in animal, so a
      superordinate over the categories becomes teachable.
- [x] The same wire lets GOLEM name an abstraction, a thought, or what was
      just discussed, since each is warm in the workspace even when nothing
      perceptual matches.
- [x] Cross-situational reliability still cleans up, so a word warmed by
      accident in one scene fades unless the binding recurs, and
      attention-grounding does not become a new junk source.

Framing: this is the brain's own arrangement, naming what is attended and not
only what is sensed, the difference between a word for what is in view and a
word for what is in mind. It needs no new representation, only a change to
what a told word grounds in.

Done when: recalling a category to warm it and then telling a third name
grounds that name in the category, so a `thing` taught that way grounds in
`animal`, `metal`, and `plant` rather than in their specifics, and a word
taught while a concept is only thought about grounds in that concept with
nothing perceptual present.

Status: built. A told word now binds to what is attended as well as to what
is perceived: before the utterance is processed, the warmest concepts in the
workspace above a gate lend their grounded moments as binding targets, so the
word keeps company with what is held in mind. Naming then grounds it there by
the same cross-situational pass that grounds it in a percept, so recalling a
category to warm it and telling a name points the name at the category with
nothing perceptual present, and three such situations distil a superordinate
over the categories through the existing abstraction machinery. It needs no
new representation, only a change to what a told word grounds in, and the
reliability pass still discards a one-off accidental warming, so attention
does not become a new junk source. The done-when holds in tests, with the
no-recurrence cleanup asserted alongside.

## Step 28: Information-weighting in the feature layer

Goal: extend Step 26's information-weighting from persistence, the drive, and
the walk into the feature layer, so signature composition and resemblance
ranking stop running on raw count, where a frequent feature swamps a correct
but less common match.

Step 26 changed the currency for what is kept and what is attended, and it
took, since the junk audit is clean. But the feature path was untouched, and
it shows. On day five `a tall green shrub` surfaced the whole green family
above the correct tree, because green had become high-frequency. On day six
`thing`'s signature was built from its highest-count carriers, dog and key,
with tree dropped. The same fix Step 26 applied elsewhere belongs here.

- [x] Signature composition by information: which features a concept carries
      weighs by how much each discriminates, so a frequent feature does not
      crowd out a rarer but defining one, and tree's tokens survive beside
      dog's and key's.
- [x] Resemblance ranking by information: a shared feature counts toward a
      match in proportion to how informative it is, so a frequent feature
      stops outranking a correct two-feature match.
- [x] One currency: the reliability measure Step 11 and Step 26 already use,
      applied to the feature path, so the whole system speaks it.

Done when: a probe sharing a rare defining feature outranks one sharing only
a frequent feature, and a concept's signature keeps a low-count defining token
rather than dropping it for a high-count one.

Status: built. Signature composition runs in two passes now: the first
gathers every concept's candidate features uncapped and counts how many
concepts carry each, and the second weighs every candidate by how much it
discriminates before trimming to the sparse top, so a feature carried by many
concepts is discounted and a rare defining one survives the cap it used to
lose. The discriminative weight is what stays on the feature edge, so
resemblance reads the same currency: a shared rare feature counts for more
than a shared common one, and a correct two-feature match outranks a single
frequent coincidence. The measure is smoothed so a feature shared by every
concept is discounted rather than annihilated, and a single-concept corpus,
with nothing to discriminate against, leaves every share untouched. It is the
cross-situational reliability of Steps 11 and 26 brought to the feature layer,
so the whole system speaks one currency. The done-when holds in tests.

## Step 29: Lateral inhibition along the abstraction hierarchy

Goal: let a warmed concept damp the concepts it subsumes, so attending to a
category presents the category to the workspace instead of flooding it with the
category's members. This unblocks the second level of abstraction through the
constellation machinery already built, and sharpens attention wherever a
general idea would otherwise be swamped by its instances.

Step 27 built attention-grounding, a told word binding to the warmest workspace
concept, and it fires: a name grounds with nothing in view. But two-level
nesting still does not form. Warming a category, by recall or by speech, spreads
activation into its thick, well-grounded members, and they take the attention
slots over the thin category node, so the constellation that hardens is made of
members and the superordinate lands as their flat sibling, the same shape as the
day-six `thing`. The members win the slot the category should hold. The fix is
the brain's own arrangement, where a general concept quiets the particulars it
contains, which is what categorization is.

- [x] A warmed concept damps the concepts it grounds in, gated by relative
      warmth, so a category attended above its members quiets them rather than
      drowning in them, while a member merely perceived is left alone when no
      category above it is attended.
- [x] Attending a category and then telling a name grounds the name on the
      category, so abstraction-by-constellation (Step 16) gathers the categories
      and a `thing` taught over three of them grounds in `animal`, `metal`, and
      `plant` rather than in one member each. (Live: a name taught over the three
      categories grounded in animal/metal/plant, its recall walking to them and
      through them to their members. The warming must seat the category at the
      ceiling: speech warms the named concept, while the CLI recall handler does
      not, so a name is warmed by speaking the category, not by recalling it.)
- [x] The damping is a tunable defaulted gentle, exposed like the rest of the
      chemistry, so it sharpens attention without starving the cases that want a
      general and a specific held together. (config: lateral_inhibition, 0.6.)
- [x] The behavior already verified holds: the inner loop's wandering, recall's
      spread, and the abstractions distilled so far do not regress once
      inhibition is on. (Inhibition is scoped to the naming attention path;
      recall and the wandering loop read raw warmth and are untouched.)

Framing: this is GABA, the brain's inhibitory transmitter, brought to the
concept hierarchy. Lateral inhibition already sharpens vision and focuses
attention; the same arrangement lets a category thought hold still without
dissolving into its examples. It needs no new representation, only that a warm
concept turn down the concepts beneath it.

Fork: the alternative to nested grounding is an explicit subsumption relation.
Across every attempt the hierarchy emerges associatively regardless, a
superordinate keeping company with its categories even while it grounds flat in
their members. So rather than force `thing` to ground in `animal`, a learned
is-a edge could link them while each stays grounded in its own exemplars, which
is arguably truer, since a thing's lived experience is the exemplars and not the
abstract category. Lateral inhibition serves the nested-grounding path and
reuses the constellation mechanism, so it goes first; the subsumption relation
is held in reserve for if a strict tree fights the grain.

Done when: warming a category and teaching a name across three categories
grounds the name in the categories rather than their members, so a `thing`
taught that way is their parent and not their sibling, and the inner loop and
recall behave as before with inhibition at its gentle default.

## Step 30: The seed becomes data, the engine stays code

Goal: lift the grammar and construction seed out of Go literals into a single
bootstrap the existing seeder writes into starter memory on first run, so the
only grammar left in code is the engine, the seed is a script anyone can edit,
and a new language is data rather than a new code path.

Step 14 moved grammar's runtime into memory: the engine compiles its working
copy from grammar nodes, and a form's lived naming mass can out-weigh a seeded
entry, so grammar learns and bends to usage there. But the seed's source stayed
in code. `Curriculum` and `Constructions` are functions returning literals
sourced from the tables in `english.go` and `turkish.go`, and `Builtin`
compiles straight from those tables, which makes them look load-bearing at
runtime when they serve only callers with no store. The learnable home of
grammar is memory while its source is still hardcoded, the seam this closes.

- [x] One bootstrap holds every seed source, the closed class, sortals, event
      verbs, principal parts, word order, suffix recipes, the constructions, and
      the declarative grammar, in the uniform parameter shape they already
      share, embedded in the binary so first-deployment seeding needs no
      external file.
- [x] The existing idempotent seeder writes the bootstrap into starter memory on
      first run and tops up new entries on a later build while every parameter
      the store already holds keeps its learned weight, the path already in use,
      now fed by data.
- [x] The grammar files keep only the engine, parse, compile, morphology, and
      unification, and the package-level fallback and the tests read the
      embedded bootstrap, so nothing reads a hardcoded table.
- [x] A new language or a corrected form is a bootstrap edit, not a code change.

Framing: this is the innate-engine and learned-content split made clean. The
engine is the innate floor and belongs in code, like the walk; the seed is the
first curriculum and belongs in data, like every concept grounded after it.
Today the engine and the seed share the same files; this separates them, so the
code holds mechanism and the data holds content.

Fluid path: a seeded entry is the rigid bootstrap that learned structure
supersedes. Once it is data on one footing with what is learned, it is a first
guess at maximal weight that lived usage out-weighs, and the measure of progress
is how few seeded entries stay load-bearing as the system learns its own.

Done when: deleting a form from the bootstrap removes it from a fresh brain and
only a fresh brain, an existing brain keeps what it learned, and the grammar
package compiles with no grammar content left in it, only the engine.

Status: built. The seed lives in `seed.txt`, embedded and read by `Curriculum`
and `Constructions`, which split it by key prefix; the daemon's existing seeder
writes it into starter memory on first run and tops up later builds while
learned weights stand. The closed class, sortals, event verbs, principal parts,
word order, suffix recipes, the declarative grammar, and the constructions all
left the Go files; only the engine and its types remain, so a form is added or
corrected by editing the bootstrap. The seed reproduces the old tables exactly,
so the suite is unchanged, and a test pins the load and the split.

## Step 31: Reify relations

Goal: make a relation a first-class concept, so a verb like chase or a stated
is-a becomes a node a proposition binds into, the structure shared across
Paris-France and Tokyo-Japan can be reified, and comprehension has somewhere to
put the link between two bound concepts.

GOLEM forms a concept from a recurring constellation of concepts (Step 16) and
now nests them (Step 29), but it cannot reify the relation itself, the thing
that holds between a dog and the cat it chased, or between a capital and its
country. The roadmap has carried this as relational abstraction with no
mechanism, and the comprehension work needs it first: to bind `chased` in "a
dog chased a cat" there must be a chase-relation to bind it into. A relation is
the substrate a proposition stands on.

- [x] A relation is a concept like any other, grounded, named, weighted, and
      salient by information, living in the one graph under the one glass box,
      but it carries argument slots for the roles it relates, chaser and chased,
      capital and country.
- [x] A relation grounds in the moments that instantiate it, so chase grounds in
      seen or told chasings the way a thing grounds in its sightings, and its
      slots fill from what played the roles there.
- [x] A relation forms from grounded usage, a parsed proposition writing the
      relation and its roles rather than blind co-occurrence guessing them. That
      producer is comprehension, so it rides with Step 32, which already owns
      writing a parsed proposition into the graph.
- [x] The walk crosses a relation, so a step from one concept to another through
      the relation that binds them runs on the primitive already built.

Framing: this is the constellation mechanism raised from things to the structure
between them. A thing is the constellation of its features and company; a
relation is the recurring configuration of the concepts it binds, with the
roles named. The innate part is the slot-bearing node and the walk that crosses
it; the learned part is which relations exist and what they ground in, distilled
from usage like everything else.

Fluid path: relations are not a fixed inventory and the seed needs none. They
form from grounded co-occurrence the way categories do, so the set grows with
experience, a relation barely lived staying weak and forgettable while one lived
often entrenches, the same currency as every other node. Generalizing one
relation across instances by structure-mapping, the analogy that sees
Paris-France and Tokyo-Japan as the same shape, is the deeper reach this leaves
open; reifying and grounding the relation is the floor it stands on.

Done when: chase, taught by narrating chasings, becomes a relation whose slots
are the chaser and the chased, a walk from a dog to what it chased crosses it,
and a second relation learned the same way stands apart from the first.

Status: substrate and crossing built, formation deferred to comprehension. A
relation is a concept that grounds in the moment instantiating it, with agent
and patient edges on that moment naming what filled each role, written by one
store method. The walk crosses a relation: from a role-filler it steps through
the shared moment to the co-argument and back, the role relations added to the
walk's repertoire and gains, dormant until propositions exist. What remains,
turning a narrated chasing into the role-bound instance, is parsing a transitive
utterance into roles and writing it, which is the comprehension of the next
step; built here it would be torn out when the bidirectional engine lands, so it
rides with Step 32. The representation and the crossing hold in tests.

## Step 32: Bidirectional constructions and the comprehension engine

Goal: make a construction a true form-and-meaning pair run by one unification
engine in both directions, so the inventory that speaks also comprehends, an
utterance parses into a meaning structure GOLEM writes into the graph, and the
rigid planner dispatch gives way to meaning handed to the engine.

Today a construction is a surface template keyed by a bare intent label, and the
planner fills its slots through a fixed dispatch over a fixed set of intents,
which is the rigid fill-in-the-blank machine: the surface moved into memory,
what GOLEM can mean did not. The move, drawn from Fluid Construction Grammar, is
to deepen the meaning side from a label into a grounded structure of roles over
concepts and relations, so one engine unifies meaning with form to speak and
form with meaning to understand. Comprehension then carries meaning inward the
way perception does, the capability that gates conversation.

- [x] A construction carries two poles, a form pole of order, function words,
      and slots, and a meaning pole of roles over concepts and the relation
      (Step 31) that binds them, both in memory, both learned.
- [x] One unification engine runs the inventory both ways, meaning to form to
      generate and form to meaning to comprehend, the engine the innate floor
      and the constructions the learned content, the same split as the walk and
      the graph.
- [x] Comprehension binds an utterance's word-concepts into a construction's
      roles by best fit, the feature signatures constraining it so an animate
      concept is the better agent, and writes the resulting proposition into the
      graph, grounding what was said.
- [x] A partial or novel utterance yields a partial meaning and a learning
      signal, not a parse failure, so an unknown relation leaves its slots bound
      and itself marked to be grounded.
- [x] The planner's fixed answer dispatch gives way to building a meaning
      structure and handing it to the engine, so an answer is generated from
      meaning rather than selected from a menu, and the per-language parse fork
      retires into the one engine over each language's seeded constructions.
- [x] New constructions are induced from grounded usage, an item-based pattern
      lifted from repeated form-meaning pairings and generalized at its slot over
      recurrences, so the inventory grows from what is heard.

Framing: this is the system's own principle reaching language at last.
Perception carries the world into the graph; this lets language carry meaning in
too, by the same grounded, weighted, inspectable structure. The engine is
innate, the constructions are learned, and comprehension is generation run
backward over them.

Fluid path: the seeded constructions are the bootstrap and induction is what
makes them fluid. A few argument-structure patterns are seeded as data (Step 30)
so there is something to parse with at all, but each is a form-meaning pair on
one footing with what is learned, and usage-based induction grows new ones and
re-weights the seeds. Induction over open usage is the long arc of the work,
demonstrated in grounded language games and unsolved at large, so the seed
carries weight for a while and the measure of fluidity is how little of it stays
load-bearing.

Done when: "a dog chased a cat," told to a brain that has grounded dog, cat, and
chase, parses into chaser and chased and is written as a proposition the brain
can then be asked about, a sentence never given whole is both said and
understood through the same constructions, and an answer the old planner
hardcoded now comes from a meaning structure run through the engine.

Status: the meaning pole is deepened and generation runs through it. An
argument-structure construction is now a two-poled object, a form pole of
ordered, typed slots and a meaning pole of thematic roles over a relation, with
one inventory of role names shared between the poles so the same construction
can run both ways. The transitive and intransitive constructions are seeded as
data beside the answer templates, written into starter memory as construction
nodes that compete by weight, never as Go literals. A proposition realizes
through them: the meaning's roles pick the construction that binds them, the
slots lay out in the language's word-order parameter so the same construction
speaks SVO and SOV, the noun phrases speak their fillers, and the predicate is
conjugated by the morphology engine, which now carries the regular English
spelling rules so an open-vocabulary verb like chase comes out as chased.

The other direction runs now too, through the same constructions. Comprehension
segments an utterance into the slots' surface spans by the determiners and the
word order, the layout generation reads, resolves each noun phrase to its
concept and the predicate to the relation's lemma (the morphology run backward,
so chased keys on chase), and yields the proposition chase(agent, patient). Best
fit refines what the order proposed: given a feature-fit scorer, an animate
concept stranded in the patient slot is read as the agent, while position stands
when it already agrees.

Comprehension is wired into the daemon and writes the proposition. A told
statement that parses as a proposition mints its relation when the verb is new,
a relation concept under a key that marks it as one and named by the verb's
lemma, and writes the proposition with the one store method: the relation
grounds in the moment of its telling, the moment points at the dog as agent and
the cat as patient. The best fit reads an animacy off lived usage, a concept's
affinity for the agent role measured from the role edges it already carries and
its feature kinship to past agents, so it is empty on the first telling and
sharpens as propositions accumulate. With the proposition in the graph the walk
that crosses a relation reaches the co-argument, the crossing reified relations
left dormant now lit, and the producer that step deferred is supplied. This
satisfies the first clause of the done-when: "a dog chased a cat" is parsed into
chaser and chased and written as a proposition the brain can be asked about.

A partial or novel utterance degrades rather than failing. An unknown verb in
transitive position is no obstacle: it mints a candidate relation and the slots
still bind, so "a dog blorped a cat" leaves a blorp to ground rather than a parse
error. A role whose filler grounds in nothing leaves a partial proposition for
what was understood and holds the gap as a learning goal, the signal
comprehension would otherwise swallow now that it has taken over the told
statement, behind the same curiosity gate the older path used.

An answer now comes from a meaning structure run through the engine, not only
from the fixed menu. A question about a relation's role, "what did the dog
chase" or "who chased the cat," is answered by crossing the proposition to the
co-argument and speaking it back through the very construction comprehension
read forward: the walk reaches the patient the dog chased, a meaning is built
over the relation and the two roles, and the engine generates "the dog chased
the cat," tense settled from the question's verb. This closes the done-when's
last clause and the conversational probe, and establishes the engine-driven
answer pattern.

More of the relational domain has since crossed onto the engine. Both role
questions, "what did the dog chase" and "who chased the cat," and the relation
itself asked, "what did the dog do," now build a meaning over the relation and
its roles and speak it back through the one construction engine, all routed
through a single entry that hands a meaning to the engine. A relation is
first-class in answers too: "what is chase" is answered that chase is an action,
the kind a verb names, rather than a thing. The whole probe runs end to end
through the daemon's own handler in a test, the path the CLI marshals to, with a
chasing told, asked about from both roles, the relation defined, and the activity
recalled, every answer generated from a meaning structure.

The planner's other branches already build meaning structures handed to an
engine: the definition through a layered message and the construction composer,
the succession and the temporal and count answers through a frame and the
realizer, so the rigid intent-label dispatch is already largely gone, and what
is left in those branches is each answer type's own graph logic, the work and
not boilerplate.

The per-language parse fork is retired. There was an English parser and a
separate Turkish one; there is now a single procedure that reads the language
from the seed (the closed classes share no forms, so a hit names the tongue),
takes the word order from the order parameter, and derives the rest from it:
where the verb sits sets whether a relator precedes its anchor or follows it and
which role a span fills first, and a colon-suffixed category carries an
interrogative's type and a relator's direction so a tongue whose wh-word or
relator is a different form ("ne" for what, "sonra" for after) is read from data
and not a code path. A third verb-final language, defined only as seed
parameters, is detected and parsed with no new code, the property a test pins.
The case stripping and a few lexical specials (the do-auxiliary, the copula's
past) remain as the languages' morphology on the floor, the residue a later pass
can seed; the fork itself, the second parse function, is gone. So the answers
come from meaning structures run through engines and the parsing is one engine
over each language's seeded grammar, both directions and both languages on the
one floor.

Constructions are induced from grounded usage now, in the same nightly rhythm as
concept and abstraction genesis. A relation told in enough propositions, its
role slots filled by varied concepts across them, has shown a construction whose
slots generalized over the recurrences, from the specific fillers first heard to
open positions; consolidation writes it as a construction node keyed by the
relation, entrenched by the recurrence and competing with the seed by weight.
The slot generalizes only when varied fillers fill it, so a relation always
heard with the same agent induces nothing yet, and a relation heard with many
agents and patients induces the transitive while one heard with no patient
induces the intransitive. The supersession is real: with the seeded transitive
removed, a brand-new transitive clause still parses, through the construction
grown from usage, the measure of how little of the seed need stay load-bearing.
This is the bootstrap stage of a long arc, honestly: the induced construction
re-derives one relation's argument structure from its own tellings, which is
item-based learning; generalizing across relations into a single abstract
construction, and inducing the novel structures a richer comprehension would
surface, are the reach this leaves open, the open research the framing names.

Step 32 stands built on this footing. A construction is a two-poled
form-and-meaning pair; one engine runs the inventory both ways and across
languages; a told proposition is comprehended into the graph and a partial one
degrades to a learning signal; answers come from meaning structures run through
the engine; the per-language parse fork is folded into the one engine; and the
inventory grows by induction from what is heard. The seed still carries weight,
and the deep generalizations stay open, as the fluid path expects.

## Step 33: Discourse on the workspace field

Goal: hold the thread of a conversation in the workspace's activation field, so
a pronoun binds to what is warm, a follow-up reads as a move on the warm
proposition, and a topic persists across turns, all on the field already there
and not a discourse template.

The probe showed "what is it" resolving to the warm dog, so the workspace is
already minimal discourse memory of the brain's kind, a decaying activation
field and not a symbolic stack. But it holds one warm node, not a structure: no
set of recent referents, no reading of "why" or "and then," no topic continuity
past raw warmth. A conversation is a structure of references and moves, and the
field needs to carry a little more of it.

- [x] The workspace keeps a small decaying set of recently-active concepts with
      the roles they last played, so a turn leaves referents behind, not only
      the hottest node.
- [x] A pronoun or bare reference binds to the best-fit member of that set, most
      active and feature-compatible, so "it" finds the inanimate it should and
      "she" the animate, by the signatures already carried.
- [x] A discourse move reads as a walk over the warm proposition, "why" to its
      grounds, "and then" to its successor, "what about X" shifting the warm
      referent, so connectives run on the primitive already built and not a
      parser of dialogue acts.
- [x] A topic persists as the sustained warm region the inner loop already
      biases toward, so a subject can be followed across turns and its change
      noticed.

Framing: discourse is sustained activation and binding, the same field that gave
anaphora for free, read a little more richly. The innate part is the decay, the
binding, and the walk; the learned part is nothing new, only the concepts and
relations already grounded, now tracked across a few turns. No discourse
representation is added; the workspace is the discourse model.

Fluid path: there is no template to make fluid here because there is none. The
mechanism is the activation field and the walk, both already general, so what
grows is only what is grounded, and a conversation about richer concepts is a
richer conversation with nothing language-specific or hand-listed to maintain.

Done when: across turns "it" and "she" bind to the right warm referents, "why"
answers about the proposition just stated by walking its grounds, the subject of
an exchange can be followed and its change noticed, and none of it reads from a
discourse script.

Status: the referent set and pronoun binding are in. The workspace now records
the thematic role each warm concept last played, so a turn leaves the dog behind
as an agent and the cat as a patient, not just a hottest node. A pronoun binds
by best fit over that set: activation is the base, and the pronoun's animacy
shifts it, so "it" finds the inanimate and "she" the animate after "a dog chased
a cat". The animacy a referent carries is read from the role it last played, an
agent leaning animate and a patient inanimate, a usage-grounded proxy rather
than a hand-listed table or a dedicated feature; with no role to read the
binding falls back to pure warmth, so a bare "it" and "the other one" resolve
the way they did before. The pronouns carry their preference as seed data ("it"
inanimate, "she" and "he" animate), so a tongue's pronoun inventory stays in the
bootstrap. Gender past animacy is not distinguished, there being no grounded
signal for it.

A discourse move now reads as a walk over the warm proposition. "Why" walks to
the antecedent in the learned sequence, the grounded answer where GOLEM models
no cause, so after it learns make deploy follows make test, "why" about the warm
make deploy walks back to make test; with nothing preceding, it admits that gap
in those terms rather than a generic shrug. "And then" walks the other way, to
the successor. "What about X" shifts the conversation onto X and answers about
it. All three run on the succession walk already built, with no parser of
dialogue acts.

A topic is followed across turns and its change noticed. The dominant warm
concept is held on the daemon as the subject of the exchange, sustained on the
decaying field the inner loop already biases toward, so a subject stays current
across turns until something displaces it; when a new concept takes the field
the change is recorded on the trail. The dog held while the talk stays on it,
the cat displacing it when the talk turns. No discourse representation was
added: the workspace is the discourse model, read a little more richly, and the
topic is cleared with working memory at sleep, a stale subject being worse than
none.

Step 33 stands built. The workspace carries a decaying set of recent referents
with the roles they played; a pronoun binds over that set by best fit, "it"
finding the inanimate and "she" the animate; a follow-up reads as a walk over
the warm proposition, "why" to the antecedent, "and then" to the successor,
"what about X" shifting the subject; and a topic is followed across turns and
its change noticed. None of it reads from a discourse script, and nothing
language-specific was added, only the field and the walk read more richly. What
stays open is gender past animacy, there being no grounded signal for it, and
the depth of "why" past the learned antecedent, GOLEM modeling no cause.

## Step 34: The goal loop, one determination each moment

Goal: make the top of the loop a single continuous determination, what to do
now, run every tick over a candidate set and arbitrated by chemistry, so a goal
is whatever currently wins selection and the absence of one is not a stall but a
determination whose answer is to rest. This folds drive, request, consolidation,
and idle into one decision and deletes the no-goal special case.

GOLEM has the pieces, the inner loop, the drives, the act gate, but no single
place that decides, each moment, among pursuing a goal, answering a request,
consolidating, and idling. Without it, no goal reads as a frozen state rather
than a choice. The brain runs action selection continuously and falls to the
default-mode network when nothing presses; idling is an output, not a gap.

- [x] Each tick determines an action over the candidates, the active goal, a
      pending request, consolidation pressure, and idle, ranked by chemistry,
      with idle as the floor so there is always an answer.
- [x] An adopted goal carries inertia, persisting against distraction until
      satisfied or abandoned, so the loop pursues rather than re-deciding from
      scratch every tick.
- [x] A salient trigger preempts, a user request above all, so the loop
      interrupts idle or wandering when the world demands it, noradrenaline the
      currency of urgency.
- [x] Idling is a first-class outcome, the default-mode wander already built,
      reached when nothing outranks it rather than as a missing case.

Framing: this is the basal ganglia's job made explicit, continuous selection
with a resting default, the same snapshot-in, candidate-out, loop-commits shape
the thought round and the execution layer already use. The innate part is the
arbitration; the learned part is which goals matter and what they are worth.

Fluid path: nothing here is a fixed script. The candidate set is whatever is warm
or pending, the ranking is chemistry over learned worth, and a new kind of goal
needs no new branch, only a way to enter the candidate set.

Done when: with no task GOLEM rests and wanders rather than stalling, an adopted
goal survives a passing distraction, a user request interrupts within a tick, and
the transition among pursue, answer, consolidate, and rest reads off one
arbitration with no special case for emptiness.

Status: built. The top of the loop is one determination now. Each quiet tick
ranks the autonomous candidates, consolidation under sleep pressure, a held goal
at its urge, and idle, by chemistry and carries out the strongest, with idle the
floor so the absence of a task is a determination to rest rather than a stall;
the old if-ready-sleep-else-if-wandering branch and its implicit empty case are
gone. A held goal carries inertia, lifted in the ranking when it is already the
pursuit so a passing distraction does not unseat it, and it is actively mulled
between wanders, a paced recall keeping its topic warm for naming and the night
to ground. A user request is not polled in this ranking; it preempts as its own
arrival on the loop, the select serving it the moment it lands while the inner
life runs concurrently, so answering one never drops the held goal. Idle is the
default-mode wander reached when nothing outranks it, resting when there is no
warm seed to follow. The arbitration is the basal ganglia's continuous
selection with a resting default, and a new kind of candidate needs only a way
to enter the ranking, no new branch. What stays open is richer worth on the
candidates than the current urges, and active pursuit that does more than recall
the topic.

## Step 35: A runtime for the code GOLEM writes

Goal: give GOLEM the computer's native strength, the ability to write a program
and run it, so computation it would labor at symbolically, arithmetic, search,
transformation, it offloads to executed code the way a person reaches for a tool.
The runtime is the capability GOLEM gains; the sandbox around the whole of it is
the operator's, placed from outside, not a feature GOLEM builds for itself.

GOLEM runs on von Neumann hardware whose strength is programmability, and the
honest way to use that strength is to let GOLEM program the machine rather than
simulate computation in the graph. Executed code is also more auditable than a
weight, because it can be read. Running code GOLEM wrote is the execute gate kept
shut, and it opens only once the whole process is placed inside the sealed
container the operator runs it in, the containment a precondition GOLEM trusts
rather than enforces.

- [x] GOLEM can compose a program and execute it, the result returned as a
      percept it can ground, through an interpreter or the toolchain it carries.
- [x] A program it runs is bounded, a time and resource budget and a recovered
      panic, so a bad program fails to a contained error GOLEM learns from rather
      than wedging its own loop.
- [x] The executed program is an artifact, written to the graph and the audit
      trail, so what GOLEM ran is as readable as what it thought, the glass box
      extended rather than pierced.

Framing: this is the keep-the-computer's-advantages principle cashed in. It is
not brain-emulation, executed code is still von Neumann; it is GOLEM using the
machine's own gift. The runtime and the bounding of a single program are what
GOLEM builds; the outer sandbox around the whole process is the operator's hand,
and which programs are worth writing is learned, grounded in what they do (Step
36).

Fluid path: the runtime is a fixed capability, but its use is open. A program is
another act the goal loop can select, judged by its outcome, so the computations
GOLEM reaches for grow with grounding rather than being enumerated.

Done when: GOLEM writes and runs a small program, grounds its result, and a
runaway or panicking program is contained to an error it can learn from without
disturbing the loop, the whole running inside the sandbox the operator placed it
in.

Status: built. The runtime is one capability behind a small Runner seam, so the
engine that carries it out can change without the caller: an embedded Go
interpreter runs the snippets a tool needs over a curated, computation-only
slice of the standard library, with os, exec, net, and syscall left out of
reach, and a compiling toolchain can stand behind the same seam when GOLEM comes
to rebuild itself. A run is bounded by a time budget and a recovered panic, the
program in its own goroutine so an overrun is contained to a timeout the caller
learns from rather than blocking the loop; a true runaway leaks that goroutine
until the process ends, which the operator's container is the backstop for, the
hard resource bound being the operator's hand and not the runtime's. The program
is an artifact, its source written to the graph and the run to the trail, and
its result is grounded by feeding it back through perception on a code channel,
so what GOLEM ran is as readable as what it thought. Running code GOLEM wrote is
gated shut by default and opens only with runtime_enabled set, the operator's
switch once the whole process sits in the sandbox. A program is reached by golem
compute today; making it an act the goal loop selects, and grounding which
programs are worth writing, is the next step. What stays open is hard resource
bounding in-process (the interpreter cannot kill a runaway) and the toolchain
backend for self-build.

## Step 36: Ground programming as a learned language

Goal: teach GOLEM to code the way it learns any language, through the
bidirectional construction engine, so a programming language is form on one side
and computation on the other, grounded in what the code does when it runs.

Code is language. Step 32 built an engine that learns a language as form-meaning
constructions, and a programming language has a grammar too. The form pole is the
syntax, the meaning pole is the computation (an abstract syntax tree, which is
pseudocode made precise), and the grounding is execution: a loop is grounded in
having repeated, seen by running it in the Step 35 sandbox and watching the
effect. So pseudocode is the meaning representation, literal language is the
teaching surface, source is the executable form, and the run is the percept.

- [x] A programming language is seeded and learned as constructions like any
      other, its syntax the form pole and an executable meaning the other, in
      memory and competing by weight.
- [x] Comprehending code yields its computation; generating from a computation
      yields code, the one engine run both ways, so GOLEM both reads and writes
      programs.
- [x] Code grounds in execution, a construction's meaning tied to the effect of
      running it, so understanding a program is knowing what it does, not just
      its shape.
- [ ] Literal-language and pseudocode teaching reach the same grounded meaning,
      so a loop or a function can be taught in words, in pseudocode, or by
      example, and converge.

Framing: this is the language faculty turned on its most precise language. The
engine is innate, the constructions of the language are learned, and execution
is the grounding that keeps the code understood rather than merely emitted, which
is what makes Step 38's self-programming a grounded act.

Fluid path: no fixed code templates. Programming constructions are induced and
taught like the rest, grounded in execution, so GOLEM's command of a language
grows from what it has run, and a second language is more seeded constructions
over the one engine.

Done when: GOLEM, told what a small program should do, writes it, runs it in the
sandbox, and grounds the result; shown a program, it says what it computes; and
the same meaning taught in words and in pseudocode lands on one grounded concept.

Status: built as an arithmetic bootstrap. A programming language is seeded into
starter memory as construction nodes beside the rest of the grammar, each
operation a form-meaning pair with two surfaces: the operator symbol that runs it
and the word it is taught by. The nodes carry weight in the same store the
grammar competition reads, so a learned operation stands or falls by the force
the rest of the inventory does. One engine runs the language both ways:
comprehending a surface yields the computation, so the literal "the sum of 6 and
7" and the code "6 + 7" both land on the one tree add(6, 7), and generating from
a computation yields either surface back, so GOLEM reads and writes the language
from a single inventory. The computation grounds in execution: GOLEM comprehends
a stated computation, generates the source, runs it in the Step 35 runtime, and
grounds the result by feeding it back through perception, so what a construction
means is the 13 it produces when it runs. The first language is arithmetic, the
floor the fluid path grows from. What stays open is the breadth: richer
constructs such as loops and functions, a teaching surface that induces a
construction from worked examples, and comprehending arbitrary Go source. That
last reach is what would let Step 38 propose a change by reading its own code;
today a change is a transform it is handed.

## Step 37: A grounded fitness for self-change

Goal: give GOLEM a grounded measure of whether a change to itself is an
improvement, so self-modification converges rather than drifts. Any change is a
hypothesis, this makes me better at something, kept only if grounded outcomes
agree.

A system that can rewrite itself needs an answer to is this better, or it walks
randomly until it breaks. Passing tests says a change is valid, not that it
helps. The measure has to be grounded in outcomes, goal progress, reduced
prediction error on the world, user trust, and multi-factor, because any single
currency is gameable: approval alone breeds flattery, surprise-reduction alone
seeks boredom. This is operant learning lifted from the operation layer, where a
fruitful thought already wears its operation's weight, up to architecture.

- [x] A change is evaluated against grounded outcomes after it lands, goal
      progress, world prediction, and trust blended, so improvement is measured,
      not assumed.
- [x] The currency is multi-factor and anchored in real outcomes, so no single
      signal can be gamed into a degenerate optimum.
- [x] A change that fails to improve is reverted, the same reinforce-or-wear loop
      the operation weights already run, now over larger units.
- [x] The reason a change survived is legible, which outcome it improved, so
      self-improvement is auditable in the glass box rather than opaque.

Framing: this is reinforcement learning's selection pressure, grounded and
glass-box, the gate that turns self-modification from a hazard into a direction.
It couples to the goal loop, which generates the outcomes the fitness reads, so
Step 34 and this are two ends of one mechanism.

Fluid path: the fitness is a measure, not a goal hand-set; what counts as better
is read from grounded outcomes that themselves grow with experience, so the
standard GOLEM holds itself to sharpens as it lives rather than being fixed.

Done when: a beneficial self-change is kept and a harmful one reverted on
grounded evidence, the decision traces to the outcome it moved, and no
single-signal exploit survives the blend.

Status: built. A change is judged against a grounded snapshot taken before and
after it lands: goal progress from the share of goals satisfied rather than
abandoned, world prediction from the calm of low alarm, and trust from oxytocin,
blended into one currency. A change is kept only when the blend rose past an
epsilon and no single factor collapsed, so a degenerate optimum that buys one
outcome by wrecking another is caught, the no-single-signal-exploit guard the
multi-factor measure exists for. The verdict drives a keep-or-revert loop, the
reinforce-or-wear the operation weights already run lifted to the larger unit of
a change, with the revert supplied by the caller, the deploy pipeline of the
next step. The reason a change survived names the outcome that moved it, so the
decision is legible on the trail. The factors are coarse, the alarm proxy for
prediction the roughest, and they sharpen with experience; the mechanism that
reads and blends them is the fixed part. This couples to the goal loop, which
generates the outcomes the fitness reads, so the two are ends of one mechanism.

## Step 38: Self-source and self-modification

Goal: give GOLEM its own source and let it improve itself through the tested,
fitness-gated deploy pipeline, never a live patch. This is the self-referential
loop the project aimed at from the start, made safe by grounding and the gate.

With code grounded (Step 36), a runtime to run it (Step 35), and a fitness to
judge a change (Step 37), GOLEM can read its own source and propose changes to
it. The danger is a system editing its running engine; the discipline is the
build-and-deploy pipeline already used by hand, propose, compile, test, judge by
fitness, swap, with backups behind it, so a bad change is caught or reverted and
never bricks the live mind.

- [ ] GOLEM can read and reason about its own source as grounded code, so a
      proposed change comes from understanding, not guesswork.
- [x] A self-change runs the full pipeline, compile, test, fitness gate, then
      swap, and never hot-patches the running engine.
- [x] A change that fails compile, test, or fitness is rejected and the prior
      binary stands, the backup the floor under every swap.
- [x] Every self-modification is on the audit trail, what changed, why proposed,
      which outcome it improved, so the system's growth is readable.

Framing: this is the keystone of the self-extending GOLEM, and it earns its place
only on the three steps under it. The pipeline is innate guardrail; what GOLEM
becomes is learned and grounded, traceable change by change in the glass box,
which is what makes the self-extension a demonstration rather than a black box.

Fluid path: the architecture itself becomes learned content under the same
currency as everything else, a faculty kept because it improved grounded
outcomes, dropped when it did not, so GOLEM's shape grows from its experience
rather than from a fixed design.

Done when: GOLEM proposes a change to its own source that improves a grounded
outcome, the change survives the pipeline and the fitness gate and swaps in, a
regressing change is caught and reverted, and the whole event reads back off the
trail.

Status: the pipeline and the gate are built; the grounded reasoning a change
should be proposed from arrives with the next step, so a change is run through
the pipeline rather than yet originated by understanding. A proposed change to
GOLEM's own source runs the build-and-deploy pipeline on an isolated copy: it
copies the tree, applies the change there, compiles the main package to a
candidate binary off to the side, and runs the tests, never editing the original
or hot-patching the running engine. A clean candidate is judged by the grounded
fitness; a change that does not improve is reverted by discarding the candidate,
and one that fails to compile or test is rejected at the stage it died at, the
prior standing. The pipeline stops at a vetted candidate staged under home; the
live swap is the operator's, behind say-so, never automatic, the discipline that
keeps a bad change from bricking the live mind. Every stage is on the trail, what
changed and why and which outcome it earned. The capability is twice-gated: the
runtime open and a source path set, two opt-ins. The pipeline is verified on
representative isolated modules rather than re-run on the whole tree: because the
gate runs the full suite on the copy, a test that itself self-builds the live
tree recurses, which is exactly why the self-build tests work on isolated modules
and the live repo and binary are never touched. What stays open is the grounded
proposal of changes (the next step), the real deploy-and-measure of a candidate's
fitness rather than a supplied verdict, and the automated swap, which stays the
operator's by design.

## Step 39: The memory editor, a grounded console

Goal: build a memory editor, a separate program for filling and inspecting
GOLEM's memory, where authoring goes through teaching so what is added is
grounded, with a raw-edit mode for surgery and an optional honest mark on what
was touched.

A glass box deserves an instrument to read and shape it; graph surgery has
already been done by hand with raw SQL. The editor is that instrument, a separate
program from GOLEM and from the terminal that talks to it, so editing memory and
conversing with GOLEM stay distinct. The discipline that keeps it from
re-introducing floating symbols: authoring emits teaching events through GOLEM's
real grounding pipeline, so a filled-in concept is grounded exactly as a narrated
one, only faster and visual.

- [ ] Filling memory in the editor emits teaching events through the grounding
      pipeline, so what is authored is grounded, never a bare declared symbol.
- [ ] A separate edit mode allows raw surgery, splicing and deletion, with every
      direct write logged to the audit trail, so a raw edit is never silent.
- [ ] A fragment can optionally be marked as edited through the editor, on the
      fragment itself, so GOLEM can know which memories were hand-touched rather
      than lived and weigh them accordingly; using the mark is optional, but the
      truth is available.
- [ ] The editor reads and renders the live graph for inspection, the same glass
      box the audit trail records, so shaping memory and watching it are one tool.

Framing: the editor is a console beside the terminal, not a replacement for it,
and it serves grounding rather than bypassing it. Teaching is the default path
and grounds; raw edit is the scalpel and is audited; the provenance mark keeps
GOLEM honest with itself about where a memory came from.

Fluid path: the editor adds no knowledge of its own; it is an interface to the
grounding GOLEM already does, so everything it puts in is earned the way lived
experience is, and its reach grows with the faculties it fronts rather than
encoding any content itself.

Done when: a concept authored in the editor is grounded indistinguishably from
one taught at the terminal, a raw edit lands on the audit trail, a fragment
marked as editor-touched reads back as such, and GOLEM can tell a hand-touched
memory from a lived one when the mark is used.

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
- Relational abstraction reifies but does not structure-map. A relation is now
  a first-class concept that grounds in the moments instantiating it (Step 31),
  minted and named from comprehension when a proposition is told (Step 32), so
  "chase" or a stated relation is a node a proposition binds into and the walk
  crosses. What is still missing is the analogy across instances: seeing
  Paris-France and Tokyo-Japan, or two separate chasings, as the same shape, the
  structure-mapping the graph does not do. Reifying and grounding the relation is
  the floor; generalizing one relation across its instances by structure is the
  deeper reach left open.
- Comprehension reads canonical argument-structure clauses; open prose is beyond
  it. The construction engine runs the constructions backward now: a told
  transitive or intransitive clause parses into a proposition over a relation and
  its roles and is written into the graph (Step 32). It goes further than before,
  though a ceiling remains.
  Comprehension covers the seeded argument structures, a subject and a verb and
  an object, with short noun phrases segmented by their determiners; richer noun
  phrases, argument structures beyond agent and patient (ditransitives, obliques,
  modifiers), and arbitrary prose still fall through to a partial meaning. Two
  supporting parts are best-effort: the role binding favors an animate agent by a
  fit learned from prior usage, so it is empty on the first tellings and word
  order alone binds them, and the lemmatizer that keys a relation across its
  inflections is regular-only, so an irregular verb keys on its surface form.
  Receiving a stated abstraction or following a multi-clause definition is past
  today's grammar.
- Untrusted perception is a standing hazard for agency. Web and other
  open-world channels carry adversarial content. The gates and denylist stop
  it from triggering execution, but the hygiene that keeps it from polluting
  memory or grounding an action (marking channels untrusted, weighting them
  low, barring them from action grounding) is a policy to get right as the
  agency work lands.
- Feature resemblance is grounded, not lexical. The sparse layer now
  generalizes by recurring tokens, channels, felt chemistry, and the
  conceptual company a concept keeps (Step 23), so two things can resemble
  each other by a shared neighborhood with no word in common. But
  resemblance still bottoms out in what was lived near: a true synonym that
  never kept the same company nor shared a surface attribute still needs a
  naming bond or a lived bridge. Breadth has to be lived or told, and the
  comprehension ceiling stands above the representation either way.
- Construction induction is item-based bootstrap. The sentence patterns live in
  memory as constructions the engine fills, composes, and now induces: a relation
  told in enough propositions with varied role-fillers grows its
  argument-structure construction during consolidation, on one footing with the
  seed and competing by weight (Step 32). But the induction re-derives one
  relation's argument structure from its own tellings; it does not yet generalize
  across relations into a single abstract construction, nor induce a structure
  the seed lacks, since comprehension only surfaces the transitive and
  intransitive meanings to learn from. A taught construction can still be
  ungrammatical until it carries its own punctuation and well-formed slots; the
  engine keeps spacing and capitalization right but does not repair a malformed
  template. Breadth of expression still has to be lived, told, or induced from
  heard usage, slowly.
- The bidirectional engine speaks the relational answers; the older answers
  still go through the procedural realizer. A question about a relation's role is
  answered now by building a meaning structure and generating it through the
  construction engine the comprehension reads backward (Step 32), so that answer
  comes from meaning rather than a fixed menu. The other answer types
  (definitions, succession, counts) still build a frame or a layered message
  handed to the procedural realizer or the construction composer, which are
  meaning-to-engine paths but not the bidirectional one, and routing them across
  would regress the taught-construction and copula handling they rely on. The
  generation side also carries English morphology, so a relation comprehended in
  Turkish is written into the graph but its relational answer is not yet
  generated with Turkish inflection; the seeded Turkish realizer covers the older
  answer types it speaks.
- One parser reads every language, with small morphology left on the floor. The
  English and Turkish parsers are a single procedure now that reads the language
  from the seeded closed class, takes the word order from the order parameter,
  and derives a relator's side and the role order from it, with an
  interrogative's type and a relator's direction carried in the seed, so a third
  verb-final language parses from its bootstrap alone (Step 32). What stays in
  code rather than data is small and per-language: the case-suffix stripping a
  verb-final language needs, a couple of lexical specials (the do-auxiliary, the
  copula's past), and English as the default tongue when no marker is detected.
  These are the language's morphology and a default on the floor; the second
  parse function is gone, and folding the residue into the seed is a later tidy.
- Salience weights by information, but the measure is coarse. Step 26 changed
  persistence, the relevance drive, and the walk from raw count to
  information: grounding diversity, a boost for a deliberately taught name,
  and edge-breadth hub-damping. The information score is a simple
  channel-diversity-plus-teaching heuristic, and the walk damps by the
  breadth of edges seen from the current seed, so a junk hub that has merged
  into a few episodes can still slip the damping. A finer measure, the
  entropy of a node's whole company, is the next turn.
- Two-level abstraction needs the category seated above its members and warmed
  at the ceiling. Step 29's lateral inhibition damps a warmed category's members
  so the category holds the attention slot, and a name then grounds in the
  category: a superordinate taught over animal, metal, and plant grounds in the
  three categories, not in one member each. The remaining sharp edge is the
  warming itself. Inhibition only quiets a category's own members, so a stray
  concept warmed by something else can still take a slot, and the CLI recall
  handler does not warm the workspace at all; a category is seated reliably only
  by speech (warmConcepts excites the named concept to the ceiling). A direct
  excite-by-id verb would make the seating dependable from any path.
- Discourse is the warm field read by animacy and antecedent. The workspace is
  the discourse model now: a pronoun binds to the recent referent that fits its
  animacy, a topic is followed across turns and its change noticed, and a
  follow-up walks the warm proposition (Step 33). The animacy a referent carries
  is read from the role it last played, an agent leaning animate, a usage proxy
  rather than a known feature, and gender past animacy ("she" against "he") is
  not distinguished, there being no grounded signal for it. "Why" walks to the
  learned antecedent of the warm proposition, GOLEM's grounded answer where it
  models no cause, so it gives a correlational predecessor or admits the gap, not
  an explanation. The set of moves is small (why, and-then, what-about); a wider
  range of connectives is more of the same walk, lived or told.

## Open questions

- A routine command is predicted away and leaves no fragment, so a tell
  can only bind to surprising moments. Whether language should focus
  attention (force-encode what follows speech on user-driven channels)
  belongs with the decision-loop work.

- Template churn: how to merge templates when the normalizer learns a
  better split for lines it already knows.
- Cross-channel calibration windows: tuned per channel class or shared.
- GABA's reach: Step 29 brings inhibition to the concept hierarchy. Whether the
  same signal should also gate sensor sampling rates, damping a noisy channel to
  hold focus on one task, is the sensory half and stays open.
