# GOLEM: General Observer Limbic Execution Model

GOLEM replaces the "Next-Token Prediction" of an LLM with **Active Inference**. Instead of guessing the next word, it seeks to minimize "Surprise" (Free Energy) in its environment. It is a concurrent, neuro-symbolic daemon built in Go: a **Self-Correcting, Grounded, and Metacognitive** system.

---

## Design Principles

Two commitments govern the whole system.

- **The glass box.** GOLEM has weights, the way a brain has synaptic strengths, learned by local rules; the protection is legibility: every weight sits on a named edge between interpretable nodes, set by a local rule and traceable to the experience that set it, and the concept layer never holds a gradient-trained distributed vector. Auditability is the line that separates GOLEM from an LLM, and the test every other choice answers to. Pretrained weights are allowed only as sensory transducers at the edge (a speech recognizer, for instance), labelled as such and kept one-way into a channel, so weights never reach the audit trail of cognition. Vision GOLEM does for itself: `golem see` decodes an image and measures a percept's colour, shape, and texture from the pixels by plain rule, so even the eye carries no borrowed weights. GOLEM does compute one continuous space, and it stays inside the line: the space is factored in-process out of the concept-by-trait matrix its own grounding already holds, by classical matrix factorization rather than by gradient, so it is reproducible and every coordinate traces to the traits that placed it. It only ever proposes. A guess read off the geometry is disposed of against the graph before it can be spoken, and it is never spoken firm.
- **Innate engine in code, learned parameters in the graph.** Each faculty is a fixed procedure on the floor with data that is learned, entrenched, and inspectable in memory above it. The grammar engine parses and realizes while its closed class lives in memory; the think primitive walks the graph while which walk to take is learned; the code engine runs while the operations it knows are seeded constructions. Shrinking the code floor and moving the parameters into the weighted graph is the same move repeated.

Self-extension rides the same discipline. GOLEM is meant to evolve and program itself, and that capability is the demonstration. What keeps it GOLEM through that growth is that meaning stays grounded throughout: code grounded in execution, authored memory grounded by teaching, every self-change traced to a grounded outcome and auditable. From the runtime onward the whole process is meant to run inside a sealed container the operator places it in: the containment comes from outside, and the terminal and the Golem Toolkit are the separate human interfaces into it.

---

## What Runs Today

The build covers the whole loop the design describes, and the self-extension above it: observation, memory, association, language understood and spoken, thought, self-directed acts, gated action, a held goal, and writing and running code it can fold back into itself under a grounded fitness. In detail:

- A normalizer collapses the variable parts of each line (numbers, hex, UUIDs, timestamps, IPs, paths) so prediction runs on line shapes.
- A forgetting frequency model scores Shannon surprise per shape. It habituates to repeated anomalies, adapts when the channel's normal drifts, and stays bounded no matter how long it runs.
- Surprises spike noradrenaline and are stored as memory fragments that carry a snapshot of the neurochemical state.
- Sleep runs on two processes. Adenosine is the need: it accumulates with every encoded moment and is discharged only by a consolidation, so what tires GOLEM is taking things in. Serotonin is the permission: it climbs through calm stretches and drops when something alarming happens. A night begins only when the need is past its gate and the calm window, the input silence, and the minimum interval all hold, so a busy stretch cannot nap its way through the questions it was asked.
- Consolidation decays memory weights, slower for sharper memories, and prunes what fades. Recall restores a memory's weight.
- Retrieval is by feeling: "show me when things felt wrong" returns the fragment with the highest noradrenaline at encoding.
- The daemon form (`golem run`) accepts observations on any number of named channels and keeps its memory in sqlite under `~/.golem`, so fragments survive restarts and hard kills. Channel models persist too, flushed at each sleep and at shutdown.
- Moments that occur near each other in time become linked. When the same pairing keeps recurring, the bond strengthens and dopamine rises, which raises the binding rate in turn.
- Memory formation is rate limited per channel: an error storm collapses into a few exemplars plus one summary moment that records the storm's size.
- `golem tell` is the linguistic channel: your words always encode, bind to the moments around them, and the reply is what they bring to mind. Words are pointers to experience. A told word binds to what is attended as well as what is in view: warm whatever is most present (recall a category, dwell on a concept) and the name grounds there even with nothing perceptual matching, so a superordinate over the categories becomes teachable.
- `golem recall` returns direct matches plus the context cloud reached through the link graph, ranked by activation. Recall strengthens every path it walks; sleep decays the paths nothing walks.
- A cue can propose an action. `golem propose "run the tests"` weighs the commands that cue has been grounded to and proposes the strongest, or asks when two are too close to call. Approvals strengthen the grounding and, after a few consistent ones, graduate the pair to autonomous; a denial demotes it. Destructive commands (a denylist covering `rm -rf`, `shutdown`, force-pushes, and the like) can be proposed but never graduate, and every decision lands in an audit trail.
- Commands run for real only when three gates all pass: execution is switched on, no part of the command is on the denylist, and every stage matches the operator's allowlist. All three ship closed (execution off, allowlist empty), so turning it on takes a deliberate edit to `config.json` and a restart. A graduated, allowed command runs autonomously when its cue is proposed; a run that fails demotes the pair, and every outcome feeds back into memory.
- No shell sits behind execution. A command parses into a pipeline of argv stages with explicit pipe and redirect structure, or it is refused with the reason audited: substitution, subshells, chaining, globbing, and escapes are outside the modeled subset. Stages run wired together with real pipes, each stage authorized on its own, and the pipeline succeeds only when every stage exits zero.
- Independent actions run at once, up to a configured cap, while decision stays serial on the brain loop. Each committed pipeline holds claims on the files it names; actions whose claims collide wait their turn in a bounded queue, so two writers of one file can never race. Every outcome returns through one channel, and stopping settles what is running before the final consolidation.
- `golem demo decide` runs the whole loop on throwaway memory: grounding, graduation, the ambiguity veto, and the safety denylist, in about a second. `golem demo pipeline` shows the action layer: refusals, per-stage authorization, overlapping independent actions, and two writers of one file taking turns.
- A CLI talks to the running daemon over a local socket, or over a signed TCP transport to a brain in a container: `status`, `chem`, `observe`, `see`, `tell`, `watch`, `trust`, `enroll`, `recall`, `concepts`, `health`, `name`, `scale`, `cmpform`, `cmprule`, `cmpderive`, `compute`, `teach`, `propose`, `approve`, `deny`, `habits`, `audit`, `voice`, `runtime`, `retune`, `dump`, `forget`, `sleep`, `stop`, plus `sign` to sign a request, `shellhook` to feed the shell channel, and `launchd` to print a start-at-login plist. `watch` streams the talking channel, what GOLEM says and what each source says to it; `health` audits the naming graph for concepts split across nodes and words that name more than one; `name`, `scale`, `cmpform`, `cmprule`, and `cmpderive` author the lexicon and the gradable scales by hand. The authority verbs (`trust`, `enroll`, `retune`) take the architect's signature, and every request over the network transport must be signed.
- Built-in senses, selected in config, sample the host the daemon runs on, per platform: file events in watched directories (bucketed to operation, extension, and parent directory), system vitals banded into words (cpu load, memory pressure, disk space; sysctl on macOS, /proc on Linux), the system log filtered to errors and faults (the unified log on macOS, the systemd journal on Linux), tailed files whose appended lines arrive as the reading channel, fetched pages stripped to text on the web channel, and the frontmost application as a screen channel. `golem shellhook` prints a zsh hook that feeds the shell channel with commands and their failures.
- Ears and the voice are external transducers by design, neural weights kept at the edge; the eye is not. `golem see <image>` decodes the pixels in the engine, measures a percept's colour, shape, and texture by plain rule, renders it in GOLEM's own words ("a round red form"), and recognizes it by resemblance, with no captioner in the loop; a colour told right after ("its colour is crimson") grounds the word to what was measured, and a colour category's boundary is learned from such tellings rather than fixed. Other perception still comes in as text: pipe a speech recognizer into `golem tell`, or a captioner into `golem observe vision "a red door"`, narrate while GOLEM perceives, and the night's naming pass grounds the words across showings. Speech goes out as text: with the voice on, every sentence the realizer builds is emitted as a say event for an external speaker to render, so the brain runs no text-to-speech itself and the speaker is the host's to choose. `golem voice` switches the say stream on and off and `golem voice use <name>` names the voice an adapter should prefer; both persist to config. Set `listen_tcp` to serve the wire protocol over TCP as well, so a transducer outside the process reaches a containerized brain from the host (`tell`/`observe` in, replies and a `subscribe` event stream out); `adapters/` holds a reference set.
- Open-world perception is policed: channels marked untrusted (the web by default) encode at half weight, can never fire a graduated habit, and never anchor the grounding of an action, so an adversarial page can be remembered without ever moving the hands.
- A telling is weighed by who says it. A moment can carry its source, and a source holds a trust GOLEM sets by talking ("trust the llm", "I doubt the llm", the same across English, Turkish, and Latin) and never on a peer's own word; a low-trust claim that contradicts what GOLEM has grounded is written but doubted, kept on record so the source's standing can be judged, and a nightly pass earns or erodes each source's trust by how its claims have held up. Told knowledge enters defeasibly: a told membership ("a zebra is an animal") reads a hedged "probably" and is kept off the grounding the categories are built from, so it never overturns what GOLEM has seen for itself, while a grounded truth still answers firmly. This is how GOLEM can learn by conversing with an LLM peer without taking its word as ground.
- Attention is channel-relative: each channel's gate asks for a z-score against that channel's own surprise history, so a chatty channel and a quiet one are judged fairly. Sensors never block the brain; a full queue drops its oldest entry and counts the drop.
- Memory is one graph of typed nodes (episodic moments, language-tagged words, amodal concepts, thought operations, grammar parameters, sentence constructions, feature attributes) under typed edges: temporal co-occurrence, directed order, names, grounds, feature, and the derived relations consolidation lays over them, among them the defeasible kind edge an is-a question climbs.
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
- One goal at a time can be held and pursued across ticks: asking about a word that points at nothing adopts a learning goal with a target and a measured distance, visible in `golem status`. The recall act aims at its topic on quiet rounds, the distance closes as experience about it is gathered and the progress lands on the audit trail, and the goal is released when it is reached (with a dopamine reward), when it goes stale, or when it stops making progress, so a pursuit never becomes an obsession.
- Each concept carries a sparse signature of interpretable features mined from its lived experience at sleep: the recurring words of its shape, its channel, how it felt, and the conceptual company it keeps. Each feature is weighed by how much it discriminates, so a token shared by many concepts is discounted and a rare defining one survives the sparse cap and carries more weight in a match. Similarity is feature overlap, so "full disk emergency", never seen and never told, still recalls the disk-full experience, and the reply names the shared attributes that carried the match. Company is a discovered feature in its own right: two concepts with no word in common but the same neighborhood come to share a "co:" attribute and resemble each other through it. The same relation gives the inner loop resemblance: disconnected kin associate as "X is like Y" while every thought still starts and lands on a named node.
- Resemblance also reads a learned space. At sleep GOLEM factors the concept-by-trait matrix its own grounding already holds, the features a concept carries together with the categories it belongs to, and keeps the leading directions as that concept's coordinates. This is classical latent-semantic factoring over GOLEM's own graph, run in-process with no outside model and no gradient, so two concepts of one kind land near each other even when they share no feature token. The space is blended into the feature overlap underneath it and never replaces it, and it only proposes: what the geometry suggests is checked against the graph before anything is spoken.
- `golem tell` parses each utterance into a concept-frame with an innate grammar and reads its force. Questions are answered with constructed sentences ("After make test, make deploy usually follows."), unknowns are admitted with an invitation ("I do not know flibber yet. Tell me, if you know."), high dopamine asks back about new words, and low oxytocin hedges the claim ("may follow"). A command stages a proposal behind the same approve gate as always; speech never executes anything by itself.
- Plain language drives the faculties and holds a thread: "recall the disk" runs the recall act and speaks the moment it found, "remember this as a bell" names whatever is most present, "tidy" consolidates, "what were you thinking" reads the inner loop, and "it", "that", and "the other one" follow the conversation's warm topic across turns, with the answer's subject becoming the next referent. A question that leaves a real gap can earn an appended follow-up question: the planner also decides what to say next.
- English inflects, never bolts on: verbs conjugate from principal parts with agreement computed from person, number, and tense ("what happened after make test" gets "usually followed" back), nouns pluralize on a rule table with the irregulars, articles follow the sound ("an hour", "a user"), and negation takes do-support everywhere, so a denial can never realize as its opposite.
- The interrogative routes the answer: what and which read order or definitions ("which commands follow make test" coordinates the field with plural agreement), when answers with the newest moment's time, where with its channel, how many with the tally, and who and why name gaps GOLEM genuinely has. "After X and before Y" parses as two clauses and the answer satisfies both.
- Turkish is the second grammar on the same frame: detected from its closed class, parsed SOV with the anchor before its relator, and answered in kind with suffix morphology under vowel harmony and voicing assimilation ("Make test'ten sonra genellikle make deploy gelir."). Latin is the third, read from its case endings and its enclitic question marker. The language of the question picks the language of the answer, and the concepts underneath are the same ones in every tongue.
- Abstractions form by the same machinery as concrete concepts: a word that never settles on one sensory shape but keeps stable company with the same small cloud of concepts distils, at sleep, into a concept grounded in that constellation. "Trouble" can come to mean disk alarms plus failed deploys jointly, recall reaches the moments through it, and the chain unfolds concept by concept down to lived experience, so even an abstraction stays auditable.
- A category can be reached through the kind above it. Consolidation derives a kind edge wherever a narrower category's grounded members are mostly grounded by a broader one, and an is-a question climbs that chain when the direct grounding no longer carries it, so "is a dog an animal" is answered by way of mammal once animal's own percept grounding has thinned. The climb is hedged by construction: the derived edge stays out of the grounded hierarchy, out of the categories a member inherits from, and out of the firm derivations, so it can only soften an answer and never assert a membership. `kind_ladder_enabled` governs it.
- An inference is stored data the one evaluator runs. A reasoning rule is a small graph of composable primitives, a reach that walks from a concept over a relation in a role and a result that reads what the reach landed on, and every parameter of it is a graph leaf. Transitive comparison, category inheritance, part-whole containment, a told symmetry, a multi-hop nested query, and the analogical proportion are each one such composition run against the live graph, so teaching GOLEM a new inference is storing a new composition, and the procedural walk stays as the floor beneath each one.
- A proportion is completed three ways, weakest last. The symbolic mode finds the relation that holds from a to b and applies it to c. Where no told relation names the regularity, the learned space can carry it: king is to queen as man is to woman reads as an offset the pair shares. Where neither carries, a holographic mode maps one entity's frame onto another's by binding algebra alone, so the dollar of America answers the peso of Mexico with no edge walked. The last two are pure proposals the graph has no way to dispose on, so they are hedged in the answer and never spoken firm.
- Comparison is composed out of poles and scales. "Big" and "small" grade along one shared scale, each carrying a greater-or-lesser role and its own comparative surface per tongue, and a told ordering is kept once on the scale, canonical greater-over-lesser, so both poles and all three tongues read the same fact. GOLEM computes the converse for itself: told the elephant is bigger than the dog, it answers that the dog is smaller. Over bare numerals it needs no telling at all, since the pole fixes the direction and its own runtime settles the difference.
- GOLEM works out the day's sound conclusions while it rests. Consolidation closes the transitive chains, category compositions, symmetric reversals, and containments the day's facts newly enable, and remembers them, so what is known grows overnight without being asked. A derived fact is spoken firm, since it follows from observed facts by a sound rule, and it is held apart from the observed graph and labelled as reasoned so it can never be mistaken for something seen. The store is an index and the graph stays the authority: a derived fact is re-verified live every time it is surfaced, so a premise retracted since means the deduction quietly stops being spoken.
- `golem concepts` lists the abstraction layer: each concept, the words that name it per language, its grounding tally, and its current workspace warmth.
- The grammar's parameters live in memory: function words and their categories, event-verb forms, principal parts, word order, and Turkish suffix recipes are nodes in the same graph, trained in as the first curriculum on first boot and compiled into a working copy at each wake. The engine procedures stay innate; the data is inspectable in `golem dump`. Grammar never fades with the nightly decay, and the one force on it is competition with the lexicon: a form deliberately taught as a label can, over sustained nights, out-weigh its seeded closed-class entry and start parsing as a content word, then heal back once the usage fades.
- The sentence patterns live in memory too, as constructions: an intent paired with an ordered template of literal text and typed slots, weighted and inspectable beside the grammar parameters. The introspection report, the succession answer, and the definition answer are filled from them rather than concatenated from hardcoded strings, while the engine keeps the morphology and word order, so the conjugated verb of "usually follows" is still computed, not stored. A meaning can have alternatives the chemistry chooses between (the oxytocin split between "usually follows" and "may follow" is now a choice between two construction variants), `golem teach --say <intent> "<template>"` adds a new way to say a thing, and removing one falls back to the innate realizer without breaking it.
- Constructions compose: a slot can be filled by another construction, so the full definition answer is built by nesting the definition clause and the count clause into a frame no single template holds ("Make test is an action. I have seen it 4 times."). The realizer renders bottom-up, each layer choosing its construction the same way, and the composition lands on the trail as the constructions that made it, so a novel sentence reads back as its parts.
- A general realization engine runs alongside the procedural one: a single operation, feature-structure unification, over a declarative grammar held in memory. A phrase rule names its daughters' surface order and the features that flow between them, a lexical entry gives a lemma its forms, and the engine unifies, selects, and linearizes, recursing through nested phrases so two structures combine to depth. Agreement and word order are editable nodes in `golem dump`: change the word-order rule and recompile, and the clause reorders with no change to the engine. It fails cleanly to the procedural realizer, the reliable floor beneath it.
- Salience is information, not frequency. Consolidation scores each concept by the diversity of the contexts its experience spans, with a durability boost when a firm name was deliberately taught. An informative concept's experience is refreshed so it outlives the night while a node lived in one fixed context fades however often it recurred; a recurring shape warms its concept in proportion to its information, so noise cannot pull the inner loop above real knowledge; and the walk damps a destination by the breadth of edges that reach it, so the loop stops circling a hub everything connects to. `golem forget <what>` is the true-forget: it deletes a remembered shape and its edges outright, the concept and its episodes alike, since some traces must be destroyable rather than only demoted.
- Language is understood and written back into the graph by one engine. A told clause parses through the same construction engine the realizer runs forward, into a proposition over a relation and its roles, and is stored. A relation, a verb like "chase" or a stated relation like "after", is a first-class concept a proposition binds into and the inner loop crosses; a relation told in enough propositions with varied fillers grows its own argument-structure construction at sleep, competing by weight with the seed. A question about a role is answered by building a meaning structure and generating it through that same engine, so the answer comes from meaning rather than a fixed menu.
- A conversation holds its thread: the workspace is the discourse model. A pronoun binds to the recent referent whose animacy fits (read from the role it last played), the topic is followed across turns and its change noticed, and "why", "and then", and "what about" walk the warm proposition. "Why" reads a told cause first ("a lion chases a zebra because the lion wants meat" makes "why did the lion chase the zebra" answerable, across tongues), gives the learned predecessor for an event sequence, and admits the gap for a fact told no reason. The discourse moves are read from grounded speech-act verbs rather than fixed phrases, so describe, clarify, recall, and the question of what GOLEM itself knows ride the verb that carries them and work the same in English, Turkish, and Latin. Asked what a thing is or to describe it, GOLEM opens with the category it grounds under ("A zebra is an animal.") and elaborates it across the relations it stands in, ordered by salience and grouped so like is spoken with like, the opposite poles of a scale joined as a contrast, the answer shading from what it observed to a hedged guess and stopping where grounding thins; a focused question stays terse.
- A conversation opens, holds, and repairs itself. A greeting is met in its own tongue and a farewell returns one; a plain telling is acknowledged ("I see.", "Anladım."); "who are you" answers the name GOLEM was given by talk, and a bare "you are golem" from the architect is how it is given. A claim GOLEM made is stood by on a re-ask ("As I said, yes..."), in whichever tongue the re-ask arrives; a correction ("no, the tiger chased the pig") retracts the old fact from every answer path and teaches the new one. A pro-form two referents fit equally is asked about ("Do you mean the fox or the dog?") and the next turn naming one settles the parked question; a clause the grammar cannot frame is admitted ("I did not understand that.") with the missing word asked after; and a question GOLEM could not answer is followed up the moment a telling fills it ("Now I know the goat ate the grass.").
- An answer can be asked for its proof. Every claim GOLEM affirms leaves a justification trace in the graph, a moment pointing at what the assertion stood on when it was spoken, deliberately shaped so the trace of a guess can never read back as an observed fact. "Why do you say that" or "how do you know" (in any of its tongues) reads the trace and speaks the support as it stands now: the teller ("You told me.", "Sen söyledin."), the seeing ("I saw it."), the likeness a hedge rode in on ("The wolf is like a dog. The dog chased the cat. You told me."), the tally behind a membership ("I have seen it 7 times."). A deduction composed of several facts is owned as reasoning ("I reasoned so."), and a proof that has decayed or been overturned since is admitted ("I no longer remember why."). Traces are ordinary memory: they fade at night, fold when a claim is re-asserted, and never keep their evidence alive; a correction cuts them with the claims, so a guess that leaned on a corrected fact retracts along with it.
- An answer can also be asked how much stands behind it. Where "why do you say that" reads the proof, "how sure are you" reads its weight: a tally computed live over the moments that carry the proposition, how often it was met, whether seen or told and by whom, and whether a denial stands. GOLEM speaks it calibrated, leading with the confidence the tally earns: a thing seen answers with its count ("I have seen it 7 times."), a thing told names who and how often ("I am certain, you told me, more than once."), a guess owns being a guess, and a denial owns the answer. Nothing is stored for this; the weight is read from the same memory the answer rests on.
- The top of the loop is one continuous determination. Each moment GOLEM weighs what to do, consolidate, pursue the held goal, or idle, by a basal-ganglia-style arbitration with idle as the floor, so self-direction is a standing decision the loop makes each moment, even before anything triggers it.
- GOLEM can write and run a program. `golem compute "the sum of 6 and 7"` comprehends the computation, generates the source, runs it under a time budget in an embedded Go interpreter over a curated, computation-only slice of the standard library (os, exec, net, and syscall left out of reach), and grounds the result the way it grounds a percept. A runaway or panicking program is contained to an error it can learn from without disturbing the loop. Running code is gated shut by default and opens only with `runtime_enabled` set, the operator's switch once the whole process sits inside the sandbox.
- Code is a language GOLEM learns. A few arithmetic operations are seeded as constructions like the rest of the grammar, each a form-meaning pair with two surfaces, the operator that runs it and the word it is taught by, read both ways by one engine: the literal "the sum of 6 and 7" and the real Go expression "6 * (7 + 1)", read by the language's own parser, land on the one computation and are grounded by running. The first language is arithmetic, the bootstrap the principle is fixed on; comprehending arbitrary source is the open arc.
- GOLEM has a grounded measure of whether a change to itself is an improvement: goal progress (the share of goals satisfied), world-prediction calm (low alarm), and trust (oxytocin), blended into one currency, with a change kept only when the blend rises and no single factor collapses, so a degenerate optimum that buys one outcome by wrecking another is caught.
- And it can change its own source, under that measure and never live. Given its own source path, GOLEM reads a tuning constant out of its code, proposes a new value, and runs the change through the build-and-deploy pipeline used by hand, compile, test, fitness gate, on an isolated copy, staging a vetted candidate off to the side. It never edits the live tree and never swaps the running binary; the deploy is the operator's. Every stage lands on the audit trail, and the capability is twice gated (the runtime open and a source path set), shipping closed.

The [Golem Toolkit (GTK)](#the-golem-toolkit-gtk) is the instrument for working this graph by hand: conversing, feeding senses, watching the inner life, and shaping memory over one connection. The [known limitations](#known-limitations) near the end mark where each faculty still falls short.

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
| **Feature Signatures** | **Cortical Feature Populations** | Sparse interpretable attributes under each concept node, mined from lived experience. Similarity is feature overlap, blended with nearness in a space factored in-process from the same grounding, so novel phrasings land near what they resemble while concept cells keep the provenance. | Embedding Similarity |
| **Reasoning Rules** | **Prefrontal Rule Representation** | An inference held as a small graph of composable primitives and run by one evaluator against the live store. Transitivity, inheritance, part-whole, symmetry, multi-hop, and analogy are stored compositions, so a new inference is new data. | Chain-of-Thought Prompting |
| **Language (Speech & Parsing)** | **Broca's & Wernicke's Areas** | Builds a sentence from a concept-frame and parses an utterance back into one, with a single innate grammar both ways over parameters and sentence constructions held in memory, composed by recursive slot-filling. | Transformer Decoder / Tokenizer |

---

## Core Methodologies

### 1. Predictive Coding (The Observer)

Unlike LLMs that wait for a prompt, GOLEM is a **General Observer**. It maintains a hierarchy of local predictors across all sensory channels (File I/O, Network, User Input).

- **The Error Signal:** If the input matches the prediction, the data is dropped.
- **Saliency:** Only "Surprises" (prediction errors) are passed to the conscious loop, drastically reducing CPU overhead and focusing "attention" only where it matters.

### 2. Limbic Scaling (The Heartbeat)

GOLEM's behavior is modulated by a simulated **Neurochemical State** of five signals. A transmitter earns its place by bending a behavior nothing already covers.

- **Dopamine:** Drives curiosity and the weighting of new "Association Fragments."
- **Noradrenaline:** Controls "sampling speed"; it spikes during high-surprise events to increase system alertness.
- **Serotonin:** Reads calm. It climbs through quiet stretches and drops on alarm, colours a memory's mood, and grants the permission half of the sleep gate.
- **Adenosine:** Reads sleep need. It accumulates with every encoded moment and is discharged only by a consolidation, so activity carries GOLEM toward a night and quiet alone never does.
- **Oxytocin:** Trust in the system's own judgement. Corrections lower it, the graduation bar reads it, and a low reading hedges a claim GOLEM speaks.

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

GOLEM features an automated "Sleep" phase on a two-process gate: adenosine says enough has been taken in to be worth consolidating, and serotonin together with a quiet input window says it is a safe moment to do it. Both must hold, along with a minimum interval since the last night.

1. **Replay:** The system reactivates the day's "surprising" fragments in order, weaving cross-channel links and directed successor edges the waking window missed.
2. **Synaptic Downscaling:** Global weight reduction of all memories. Weak/noisy fragments are pruned (deleted).
3. **Abstraction:** Recurring shapes merge into prototype fragments, prototypes distil into concept nodes, the day's words bond to the concepts their moments ground, and a word whose company is a stable cloud of concepts distils into an abstraction grounded in that constellation.
4. **The Kind Ladder:** a category holding a settled grounded core is wired to any strictly broader category that grounds most of its members, so the levels above the members are derived while the cores are freshest. The edge is defeasible and read only by the hedged is-a answer, and it is spared the nightly fade, so it outlives the grounding that justified it. `kind_ladder_enabled` governs it.
5. **Signatures:** every concept's sparse feature signature rewrites from its current grounding (its recurring tokens, channels, felt chemistry, and the conceptual company it keeps), so resemblance stays current and auditable down to the moments underneath.
6. **Weighing by information:** each concept is scored by the diversity of the contexts its experience spans, with a boost for a deliberately taught name; the experience under an informative concept is refreshed so it outlives the decay while a node lived in one fixed context is left to fade.
7. **Sound Derivation:** the transitive chains, category compositions, symmetric reversals, and containments the day's facts newly enable are worked out and remembered, held apart from the observed graph and labelled as reasoned. The record is an index and the graph stays the authority, so each one is re-verified live whenever it is spoken.
8. **The Learned Space:** the concept-by-trait matrix is refactored and each concept's coordinates are rewritten, so the space that proposes a resemblance tracks the grounding as it stands that night.
9. **The Grammar's Night:** seeded parameters and lived usage settle their competition, and the working grammar recompiles from the surviving weights.

---

## Technical Specifications

- **Language:** Go (1.25+)
- **Concurrency:** parallel substrate, serial commitment. Sensory channels, committed execution, and the thought operations all run as `goroutines` off the loop; the `select` loop is the single serial thread that decides and writes memory, the Global Workspace and the action gate at once.
- **Architecture:** Event-driven, non-blocking, neuro-symbolic.
- **Dependencies:** Zero-bloat. The daemon links five pure-Go libraries (sqlite via `modernc.org`, `fsnotify`, the `yaegi` interpreter that backs the code runtime, `gonum` for the one matrix factorization behind the learned space, and `golang.org/x/net/html` to strip a fetched page to text); no model weights, no frameworks, no cgo. The Golem Toolkit's window is a separate Go module and the only part that pulls in a webview (Wails with a Svelte frontend).

---

## Repository Layout

One binary, one brain-loop package, and the organs it wires together.
Packages live under `internal/` because GOLEM is a daemon, not a library;
the bio-mapping column says which part of the architecture table each
package carries.

```
cmd/golem/          the CLI: dispatch, printing, the DOT export, the
                    compute verb, and the in-RAM synthetic demos
wire/               the public socket protocol: requests, responses, the
                    display forms of memory, and the client side of one
                    round trip (imported by the CLI and the editor alike)
internal/
  limbic/           the neurochemical state and its homeostasis (the limbic
                    system: dopamine, noradrenaline, serotonin, oxytocin)
  attention/        normalizer, surprise predictor, per-channel calibrator
                    (thalamus and prefrontal cortex: what reaches awareness)
  senses/           the observation packet and the built-in sensors: files,
                    file tails, vitals, syslog, web pages, the frontmost
                    app, the synthetic channel, and the shell contract
  memory/           moments, words, concepts, thought operations, grammar
                    parameters, sentence constructions, and feature
                    attributes as one typed graph in sqlite, the in-RAM
                    hippocampus, the hot working set, and consolidation
                    (hippocampus and cortex)
  grammar/          the innate language engine and its curriculum: one
                    concept-frame, English and Turkish parsers and
                    realizers, hand-rolled morphology, the parameters
                    compiled from memory, the constructions filled and
                    composed into sentences, and the general unification
                    engine over a declarative grammar (Broca and Wernicke)
  code/             the programming-language faculty: arithmetic operations
                    as bidirectional constructions, comprehending a
                    computation from a surface and generating source from
                    one, and reading real Go declarations
  runtime/          the bounded code runtime behind a Runner seam: an
                    embedded interpreter over a computation-only slice of
                    the standard library
  selfbuild/        the isolated build-test-stage pipeline for a self-change,
                    run on a copy, never the live tree or binary
  pipeline/         shell-free execution: parse commands into argv stages or
                    refuse, run them over real pipes, derive resource claims
  vsa/              the holographic substrate behind the weakest analogy mode:
                    binding, bundling, and cleanup over a seeded codebook
  authkey/          Ed25519 proof of who is speaking: the architect signs, the
                    daemon holds only the public half and verifies
  events/           the structured trail, one JSON object per line, rotated
  config/           tuning, the config file, channel trust, and the
                    destructive-pattern denylist with its matchers
  daemon/           the brain loop that owns all mutable state: encoding,
                    association, sleep, the continuous determination at the
                    top of the loop, the decision gates, the parallel action
                    arbiter, the message planner, the workspace and
                    discourse, the inner loop with its walks and acts, the
                    drives and the held goal, the feature passes, the code
                    runtime, the grounded fitness and self-modification, the
                    audited raw edits, the voice, and the demos (basal
                    ganglia, ACC, hypothalamus, and the global workspace)
gtk/                the Golem Toolkit, a separate Go module: a shared signed
                    wire client, the gtk console, and the desktop window (Wails
                    with a Svelte frontend)
adapters/           the transducers that stay outside the brain: reference
                    speech, voice, and host-sense bridges that reach a
                    containerized daemon over the signed transport
                    ([adapters/README.md](adapters/README.md))
docs/               design notes too long for this file
Dockerfile          the operator's sandbox: the Go toolchain the code runtime
                    and the self-build pipeline need, with the curriculum
                    carried along
teaching/           the ostensive curriculum: the scene scripts each lesson is
                    taught by, the one config that teaches and observes at
                    once, the snapshot helper, and the spine rehearsal that
                    re-shows every percept a long run has let thin
                    ([teaching/README.md](teaching/README.md))
```

---

## Getting Started

```bash
# Clone the mind
git clone https://github.com/Serpo-Technology/golem.git
cd golem

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
./golem teach --say introspect "my mind was on {thought}"

# Write and run a program (only with runtime_enabled set; see Configuration)
./golem compute "the sum of 6 and 7"
./golem stop

# Optional: feed your shell into GOLEM (open a new shell afterward)
./golem shellhook >> ~/.zshrc
```

The runtime and self-modification gates assume the whole process sits inside a
sandbox the operator placed it in, so the container is the intended way to run
it. The image carries the Go toolchain the code runtime and the self-build
pipeline need, and the curriculum travels with it:

```bash
docker build -t golem:latest .
docker run -d --name golem-brain golem:latest
docker exec golem-brain golem tell "what do you know"
```

[teaching/README.md](teaching/README.md) covers teaching a brain from empty in a
container, the config that teaches and observes at once, and the probes that
score the result.

> **Note:** GOLEM is designed to run in the background of your OS, observing and learning your patterns without the need for constant prompting.

### Running the Memory Editor

The Golem Toolkit is a separate program that talks to a running brain over its wire protocol. See [The Golem Toolkit (GTK)](#the-golem-toolkit-gtk) below for what it does, and [gtk/README.md](gtk/README.md) for connecting it to a container.

```bash
# The console: build it and read or shape the live graph
go -C gtk build -o gtk ./cmd/gtk
gtk/gtk status
gtk/gtk search disk
gtk/gtk audit

# The window: a Wails desktop app (install the Wails CLI once)
go install github.com/wailsapp/wails/v2/cmd/wails@latest
cd gtk && wails build && open build/bin/GolemToolkit.app
```

### Configuration

The daemon reads `~/.golem/config.json` and writes one full of defaults on first run. [config.example.json](config.example.json) in the repository shows every field at its default, and a test pins it to the code, so the example cannot drift. The fields most worth knowing:

- `channels` selects the built-in senses: `vitals` and `syslog` ship on; add `watch_dirs` for file events, `tail_files` for the reading channel, `watch_urls` for the web channel, `screen` for the frontmost app, and `voice` to emit each reply as a say event for an external speaker (`voice_name` names the voice an adapter should prefer; `golem voice` adjusts both without a restart). The vitals, syslog, and screen sensors read the host the daemon runs on, so a containerized brain leaves them off and an adapter feeds it the host's senses through `golem observe`.
- `listen_tcp` (off by default) serves the wire protocol over TCP alongside the unix socket, so a transducer outside the process reaches the brain across a container boundary. Every request over TCP must carry the architect's Ed25519 signature: set `architect_pubkey` (or `GOLEM_ARCHITECT_PUBKEY`) to the public key from `gtk keygen`, and sign requests with the private key through `golem sign`; without a key set the transport refuses every request. Bind it to loopback and publish only to host loopback anyway, since a reachable brain can be told and asked to act.
- `architect_pubkey` is the architect's Ed25519 public key, base64 (from `gtk keygen`; `GOLEM_ARCHITECT_PUBKEY` overrides the file). The daemon holds only the public half and verifies the architect's signature against it; setting trust and retuning the source take a valid signature, as does every request over TCP. The private key stays on the architect's machine and signs through `golem sign` or the CLI's own `trust` and `retune`.
- `execute_enabled` and `execute_allowlist` are two of the three execution gates and both ship closed: nothing runs for real until you switch execution on and allowlist commands by hand.
- `runtime_enabled` gates running code GOLEM writes (the `compute` verb and self-modification) and ships off; `source_path` points GOLEM at its own source for self-modification and is empty by default, with `main_package` naming the package the self-build pipeline compiles. Opening either assumes the operator has placed the whole process inside a sealed container.
- `untrusted_channels` (the web by default) marks senses whose moments encode lightly and can never cue a habit or ground an action.
- `kind_ladder_enabled` (off by default) lets consolidation derive a kind edge from a narrower category to a strictly broader one that grounds most of its members, and lets an is-a question climb it. A member then reaches a superordinate through the kind above it after the superordinate's own percept grounding has thinned, and is answered "probably" there. The edge is read only by that hedged branch and never by the grounded hierarchy, so a wrong one softens an answer without asserting a membership. It earns its place on a curriculum that teaches three levels, a member under its kind and the kind under a superordinate.
- `ostensive_core_enabled` (on by default) decides what a category is a class OF by what its word was pointed at rather than by the company the word kept. A naming earns one edge, to the freshest percept in view, while the company reaches everything still inside the window, so a dense stretch of teaching spreads a word over the whole room and the shares can no longer tell a member from a scene-mate. Where a word points at nothing the company still decides, so a category carried wholly in talk is untouched. It also holds a pointing out of the nightly fade, since a reading that fades turns itself off on a timer, and it says so on the event stream when a word it used to read that way arrives with no pointing left.
- `ostensive_naming_enabled` (off by default) applies the same test to the nightly naming recount: a word that was shown things keeps its bond only to the ones it was shown, judged by kind rather than by identity so a second exemplar cannot evict the first, and its remaining shares are read against that company rather than the whole room.
- `resolve_to_own_concept_enabled` (on by default) settles which concept a bare word refers to. A naming bond is spared the nightly fade and only ever climbs, so a bond a word drifted onto stands at whatever it reached for good; the word's own class now outranks it, and a placeholder minted by a passing telling gives way once something of the kind has actually been shown. It applies only to a phrase of one content word, since a described thing is not about its modifiers.
- `settle_tongue_first_enabled` (off by default) settles the tongue of an untagged clause before its words are looked up rather than after. A word resolves through its tongue, and resolving with none pools every node of the same form, so an English `gem` can draw the votes of a Turkish stem of `gemi`, a ship, and "what is a gem" answers that a gem is a vehicle. The same clause tagged with its language answers about a gem.
- `retire_declined_rungs_enabled` (off by default) gives the coverage test teeth. A class earns its place inside a broader class by what it covers, and a rung that has not earned it is refused; but a refused rung was only left out of the refresh, and it fades so slowly that it goes on handing a firm membership to everything it holds for some seventy nights after the pass stopped believing it. A refused rung is now written down the same night, and down rather than out, so a night whose evidence came in thin demotes it instead of erasing it. Only the rungs between two classes, which is what the coverage test judges.
- `subliminal_enabled` (off by default) makes the attention gate graded instead of binary, and `subconscious_channels` lists the trusted channels it applies to. A habituated observation on such a channel still encodes, below consciousness and faint on every axis: low weight, throttled intake, no naming or episodic reach, and repeats inside one continuous exposure deduplicated. So a live monitor can run beside a lesson without entering the taught concept's company, and a concept is earned by recurring across distinct episodes rather than by being dwelt on.
- Five flags settle which thing a word refers to, each the fix for a specific misnaming that was measured, and all off by default: `unshown_names_by_own_share_enabled` (a word never shown anything is named by its own share rather than by the class's, so a rare class cannot win a word four to one on scarcity alone), `own_name_outranks_its_kind_enabled` (a thing keeps the name its caption headed, so a desk is not answered as a table), `bound_word_is_not_rehomed_enabled` (a word already heading a percept is not moved onto another thing when its bond thins), `definition_speaks_nearest_kind_enabled` (a definition leads with the narrowest category the concept belongs to, in the asked tongue, held to the same bar an is-a answer climbs), and `verb_stem_names_relation_enabled` (a verb reduction counts only when its base names a relation, so the garments stay out of cloth).
- `surprise_threshold`, `z_gate`, and `predictor_windows` tune what reaches consciousness per channel; the sleep settings pace consolidation, and `sleep_pressure_gate` is the adenosine level a night needs before the calm window and the interval are even consulted.

---

## The Golem Toolkit (GTK)

A glass box deserves an instrument to work it. The Golem Toolkit is that instrument: a separate program from the daemon and the terminal that talks to a running brain over its wire protocol. The brain it works usually runs in a container, reached over the signed network transport, so every request the toolkit sends is signed with the architect's key; it also speaks to a brain beside it on the unix socket, unsigned, when pointed at one. It is two faces of one client, and the daemon stays the only process that writes the database. Its own directory is `gtk/`, and it lives in its own Go module so the daemon's dependency tree stays small; the webview stack lives only here. See [gtk/README.md](gtk/README.md) for how to build and connect it.

- **`gtk`, the console.** A CLI that reads the live graph (`status`, `search`, `concepts`, `cognition`, `audit`, `dump`), authors through grounding (`observe`, `teach`), and performs audited raw surgery (`edit`, `reweight`, `delete`, `mark`).
- **The window.** A desktop app (Go + Wails, with a Svelte frontend) whose panels are one connection to the brain: a chat that speaks and listens through the browser's own voice, senses that observe and show images to see, the memory search, concepts, and interactive graph the editor had, the decision trail beside a live feed of the brain's own events, its cognition and health, identity for setting trust and enrolling a peer's key, controls for the runtime, voice, consolidation, and retuning, and the connection settings.

Two disciplines keep it honest:

- **Authoring goes through teaching.** Filling memory emits the same teaching and perception events the terminal does, so an authored concept is grounded exactly as a narrated one, never a bare declared symbol.
- **Raw surgery is audited and marked.** A direct splice, reweight, deletion, or hand-touch runs on the brain loop like every other state change, lands on the audit trail, and optionally marks the fragment as editor-touched, so GOLEM can tell a shaped memory from a lived one. A per-fragment provenance flag carries that mark.

---

## Known Limitations

- **Execution is a modeled pipeline, not a shell.** Substitution, subshells, chaining, globbing, escapes, and descriptor redirects are refused by design; a job that needs them lives in a script the allowlist names. Pipeline success is every-stage-zero, stricter than a shell's last-stage-wins, so a head that exits nonzero when its reader stops early reads as a failed run.
- **Effector exclusion is a string-equality heuristic.** Two paths naming one file differently can slip past it, and two unrelated commands sharing an argument word serialize when they could overlap. It errs conservative, but it is a heuristic.
- **Comprehension reads canonical clauses, not open prose.** A told transitive or intransitive clause parses into a proposition over a relation and its roles, with short noun phrases segmented by their determiners; richer noun phrases, argument structures beyond agent and patient (ditransitives, obliques, modifiers), and arbitrary prose fall through to a partial meaning. Role binding favors an animate agent by a learned fit, so it leans on word order alone on the first tellings, and the lemmatizer that keys a relation across its inflections is regular-only.
- **Analogy completes a proportion without abstracting a schema.** A proportion a:b::c:? is answered three ways: the named relation applied to the fourth term, an offset the pair shares in the learned space, or a frame mapped onto another by binding algebra. The last two are proposals the graph has no fact to check them against, so they stay hedged and are never spoken firm. What is still not drawn is the generalization above them, seeing many separate chasings as one shape and keeping that shape as something GOLEM can hold, name, and reason with.
- **The bidirectional engine speaks only the relational answers.** Definitions, succession, and counts still go through the procedural realizer or the construction composer.
- **Code is reasoned and run, not yet read at large.** The code faculty assembles a procedure by composing the Go floor and grounding it by running: it synthesizes functions, conditionals, recursion, and list folds and transforms from examples or a told definition, learns an unknown computation through dialogue, and each operation it knows is a grounded concept resolved cross-tongue. It can also discover a function from a stated property and confirm it by running, as with the inverse of an operation or the extremal of a list. The code it runs is generated by the same graph-walking machinery that speaks language, with the typed renderer kept as a cross-check. The seeded floor is still arithmetic, so comprehending arbitrary Go read from source (whole programs, library idioms) is the open arc, the reach that would let self-modification propose a change from reading its own logic rather than retuning a constant.
- **Self-modification retunes constants and never deploys itself.** A self-change today reads a numeric constant from GOLEM's own source and proposes a new value; richer edits over more of the syntax tree grow from there. The pipeline builds and tests on an isolated copy and stages a candidate, and the live swap is always the operator's, by design.
- **Resemblance is grounded, not lexical.** Two things resemble each other by shared tokens, channel, felt chemistry, or shared neighborhood; a true synonym that never kept the same company nor shared a surface attribute still needs a naming bond or a lived bridge.
- **Grounded categories thin as teaching moves on.** A category is re-grounded each night from the percepts its word still keeps company with, so a class taught early in a long curriculum grounds fewer members every night as later lessons stop mentioning it, until a word that still names itself grounds almost nothing and its is-a answer flips to no. Lowering `nightly_decay` does not prevent it: the loss is in which members the core reselects, not in the weights. Re-showing the percepts restores the grounding while re-telling the facts does not, since utterances are kept out of naming company, so one flat replay of every percept and every name at the end of a long run is the working repair (`teaching/rehearse-spine.sh`).
- **The kind ladder reaches the superordinate, not the kind.** The derived edge carries a member past its kind to the broader category above it, so a thinned superordinate still answers. A member that has lost the grounding on the kind directly above it has nothing to climb from, so kind-level thinning is still the ceiling on how coherently a broadly taught brain answers is-a.
- **The naming pass reads a caption's head noun as a name.** A percept narrated as "a small blue bird" bonds "bird" to that one percept as its basic-level name, so the word never distils into the category over the birds and its is-a lands on a sibling instead. A member has to be captioned by its species, leaving the class word to the telling that groups them.
- **Salience weighs information coarsely.** The score is a channel-diversity-plus-teaching heuristic; a finer measure, the entropy of a node's whole company, is the next turn.
- **Untrusted perception is a standing hazard.** Open-world channels (the web by default) carry adversarial content. The gates and denylist stop it from triggering execution, and untrusted channels encode lightly and can never ground an action, but keeping it from polluting memory is an ongoing policy.
- **The graph view defaults to the semantic core.** The full store is hundreds of nodes and thousands of temporal edges, a hairball; the window draws concepts and words by default and reveals the episodic mass on demand, capped so a large graph stays responsive.

## Open Directions

A few things the current build does not do, kept honest.

- **Comprehending arbitrary source**, which is what would let self-modification propose changes from understanding its own code rather than retuning a known constant.
- **Abstracting a relational schema.** The proportion is completed; the shape behind a run of instances is not yet lifted out as an object in its own right.
- **Grounding that survives a moving curriculum.** A category taught early should keep its members as later lessons crowd it out, without a flat replay pass at the end of the run. The derived kind ladder softens the answer above the kind; holding the kind's own grounding is the reach left open.
- **A wider neurochemical palette.** Dopamine, noradrenaline, serotonin, adenosine, and oxytocin run reward, salience, calm, sleep need, and trust. A transmitter earns a place only by bending a behavior nothing already covers: a cortisol-like stress signal read from CPU, memory, and heat that sheds non-essential sensing to protect the core loop is the next worth adding when GOLEM runs ambient under load. Acetylcholine, a precision and learning-rate signal, overlaps what dopamine and the attention gate already do and stays out until that coarseness bites.


---

## License

GOLEM is licensed under the GNU Affero General Public License, version 3. The
full text is in [LICENSE](LICENSE).

The network clause is the reason for that choice. GOLEM is a daemon you reach
over a socket, and the AGPL carries the source obligation across that boundary:
anyone you let talk to a modified GOLEM over a network is entitled to the source
of the thing they are talking to. A glass box would stop being one if putting it
behind a port were enough to close it.

Copyright (C) 2026 Serpo Technology Inc.
