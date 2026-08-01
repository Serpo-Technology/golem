# Teaching GOLEM

A reproducible ostensive training run, in a container or against any reachable
daemon. Each scene directs attention at its taught percept with a forced
observation, so the same lesson teaches identically however many ambient
sensors are live: the forced percept is attended and learned at full salience
even after routine repetition would habituate it, while the unforced companions
and any ambient sensors stay out of a taught concept's company.

## In a container

    docker build -t golem:latest .
    docker run -d --name golem-brain \
      -v "$(pwd)/teaching/always-online.json:/root/.golem/config.json:ro" golem:latest
    docker exec golem-brain env GAP=10 zsh /opt/curriculum.zsh

The image carries the curriculum at `/opt/curriculum.zsh`. Watch progress with
`docker exec golem-brain golem concepts` and `docker exec golem-brain golem status`.

## The teaching config

`always-online.json` is one config that teaches and runs the sensors at once. A
lesson forces its taught percept, so it is learned at full salience however
routine repetition would otherwise habituate it below consciousness, and a
channel listed in `subconscious_channels` (for example `["vitals"]`, never the
teaching channel) is treated as background: its observations withhold salience
and never enter a taught concept's company, so a live monitor can run alongside
the lesson without polluting it. `subliminal_enabled` turns on the graded
attention gate the background channels rely on, and `calibration_warmup: 30`
lets habituation work normally.

The one knob a fresh brain needs is the lowered `surprise_threshold: 0.3`. A
cold predictor finds the everyday captions barely surprising: "a small black
cat" carries only 0.5 bits before anything is known, so under the default 2.0
floor nothing would encode. The lowered floor admits them until the predictor
warms. A brain that is already taught can run the default config for normal
operation; the lowered floor only matters while the predictor is cold.

`kind_ladder_enabled` is off by default and belongs on for a corpus that teaches
three levels, a member under its kind and the kind under a superordinate, the way
`menagerie.zsh` names a salmon a fish and an animal and the curriculum names a dog
a mammal and an animal. With it on, the nightly pass lays a defeasible edge from a
narrower category to a broader one it falls under, and a member that reaches the
broader class only along that edge is answered with a hedge, "probably", where a
grounding would earn a firm yes. That carries the superordinate answer through
after the superordinate's own grounding has thinned. It cannot carry a member's
link to its own kind, so a long grow still needs the rehearsal below.

## Captioning a percept

A caption's head noun is read as that percept's basic name, and a word that names
one of the very percepts it would generalize cannot distil into a category. So
never caption a percept with the class you mean to teach over it: "a small blue
bird" teaches `bird` as a name, the class never forms, and its is-a answer lands
on a sibling. Caption the member by its species and keep the class word for the
telling: show "a small blue jay", name it `jay`, then tell `bird`. Made things
follow the same rule, and the rule runs across the whole corpus rather than one
scene, since `furniture.zsh` teaches `table` and `bed` as kinds, `artifacts.zsh`
shows a coffeetable and a mattress instead. Before adding a scene, take every
word it uses as a kind or a superordinate and check that none of them ends one of
its captions.

## Building on a snapshot

Each lesson builds on the last. Restore the latest snapshot into the
container before teaching, so prior knowledge is never lost.

    docker build -t golem:latest .
    docker run -d --name golem-brain \
      -v "$(pwd)/teaching/always-online.json:/root/.golem/config.json:ro" golem:latest
    CONTAINER=golem-brain ./teaching/snapshot.sh restore ~/.golem/snapshots/<latest>.db
    docker cp teaching/mind-verbs.zsh golem-brain:/opt/
    docker exec golem-brain env GOLEM=golem GAP=1 BLOCKGAP=6 zsh /opt/mind-verbs.zsh
    CONTAINER=golem-brain ./teaching/snapshot.sh save <label>

`snapshot.sh restore` copies a snapshot into the container's `memory.db`, and
`snapshot.sh save` checkpoints it back out to `~/.golem/snapshots`. Teaching
into a fresh container would start from an empty brain and lose everything.

Teach the shown scenes before the told ones. A shown scene mints its members from
percepts, while a told scene names concepts it expects to be there already, so
`relations-web.zsh`, `relations-deep.zsh`, and `scales-deepen.zsh` bind only what
the earlier scenes grounded. Within each group the order is free.

## Rehearsing the spine after a long grow

A grow that teaches each domain once and moves on erodes the categories it taught
first. As later scenes stop mentioning them their naming episodes fade, the nightly
abstraction pass grounds fewer members each night, and a class that still names
itself grounds almost nothing, so its is-a answers turn to no. Lowering
`nightly_decay` does not prevent it: the loss is in which members the category
reselects as its core, not in the weights. Re-showing the percepts restores the
grounding; re-telling the facts does not, since a spoken sentence is an utterance
and utterances stay out of naming company.

`rehearse-spine.sh` is the repair. It replays every percept and every name across
the scenes you name, in one flat pass of two rounds with a nap stride scaled to the
corpus, so the whole spine reconsolidates without its own tail eroding its head.
Run it once, at the end of the whole grow, never between the scenes:

    docker exec golem-brain sh -c 'GOLEM=golem bash /root/teaching/rehearse-spine.sh \
      curriculum world menagerie flora artifacts vehicles ...'

It re-shows percepts, so it also grows the brain by several megabytes.

The pass paces itself, and the pacing is the point. A told word binds to whatever
percept is still inside the attention window, so scenes replayed back to back hand
every category word the whole run of things shown before it, and the pass corrupts
the naming company it exists to rebuild. A percept that differs from the one before
it therefore waits the window out first, which costs a few seconds per change of
percept and some hours over a broad corpus. `PAUSE` sets that wait.

Check what a category's strongest company IS afterwards, not how much of it there
is. Grounding counts rise either way; a smeared word shows up as a class whose
firmest company is something unrelated to it.

## Scoring the is-a spine

    python3 teaching/score-isa.py ~/.golem/snapshots/<brain>.db

Counting grounding edges says a category is healthy when it has merely grown
greedy, so this scores a brain against what the scenes actually taught. It reads
the ground truth by dry-running every scene through a recording mock, which is
the only way to see past the twenty different `teach` helpers, and reports:

- precision, the share of grounded members that belong where they are grounded
- recall, the share of taught memberships the brain actually laid
- nameless concepts, percepts nothing names any more

Judge a change on precision AND recall together. Shrinking every category back to
a handful of members scores well on precision alone while losing most of what the
brain knew. Pass `--wrong 20` to see the worst edges, and `--rebuild` after
editing scenes, which refreshes the cached `isa-truth.json`.

## Against an existing daemon

    GOLEM=golem GAP=15 zsh teaching/curriculum.zsh

## Files

- `curriculum.zsh` the foundation lesson: vocabulary, the animal, metal, and
  plant categories with the mammal kind named between the animals and the
  superordinate, the green adjective, two Turkish words, the door-bell
  sequence, the thing superordinate, and a few chasings for the comprehension
  engine. The plants are shown by species (a sunflower, an elm, a boulder) so the
  class words stay free to abstract. Each scene forces its taught percept (`fobs`)
  and leaves the combinatorial companions ambient (`obs`). Parameterized by
  `GOLEM` and `GAP`.
- `world.zsh`, `world-bilingual.zsh`, `world-breadth.zsh`, `world-foods.zsh`
  broaden the world (more animals, plants, and foods, bilingual). They force the
  taught percept the same way and build on the curriculum: restore its snapshot
  first. `world-countries.zsh` adds a small world of nations by telling, each with
  a currency, a capital, and a language, so their frames run parallel for the
  analogy modes to map across.
- The domain scenes shown on percepts: `menagerie.zsh`, `more-creatures.zsh`,
  `sealife.zsh`, `flora.zsh`, `artifacts.zsh`, `furniture.zsh`,
  `kitchenware.zsh`, `containers.zsh`, `clothing.zsh`, `vehicles.zsh`,
  `tools-instruments.zsh`, `instruments-music.zsh`, `sports-gear.zsh`,
  `toys.zsh`, `oldweapons.zsh`, `machines.zsh`, `buildings.zsh`,
  `materials-minerals.zsh`, `weather-sky.zsh`, `celestial.zsh`, `people-kin.zsh`.
  Each shows every member on its own percept, names it, places it under a kind and
  under a superordinate, and ties it into the world by a relation or two.
  Parameterized by `GOLEM`, `GAP`, and `BLOCKGAP`.
- The domain scenes taught by telling, which bind concepts the shown scenes have
  already grounded: `world-atlas.zsh`, `geography.zsh`, `nature-body.zsh`,
  `food-drink.zsh`, `more-things.zsh`, `society.zsh`, `qualities-shapes.zsh`,
  `qualities-abstract.zsh`, `actions-motion.zsh`, `scales-deepen.zsh`,
  `abstractions-plus.zsh`, `relations-web.zsh`, `relations-web-2.zsh`,
  `relations-deep.zsh`. The relation scenes mint few concepts and many edges, so
  they grow the graph dense where the vocabulary scenes grow it wide.
- `mind-verbs.zsh`, `mind-concept.zsh`, `llm-concept.zsh`, `llm-concept-tr.zsh`,
  `abstract-domains.zsh`, `abstract-domains-2.zsh` the abstract layer:
  experiential verbs, perceptless nouns, and the abstract domains, taught by
  telling rather than by percept. Each builds on an existing world; restore the
  latest snapshot first.
- `always-online.json` the teaching config: the graded attention gate on with
  the vitals sensor treated as background, the TCP listener and code faculty
  open, and the absolute surprise floor lowered so a fresh brain's cold
  predictor still admits the everyday captions. A warmed brain can run the
  default config once it is taught.
- `snapshot.sh` save, restore, and list the brain's snapshots (its fluid
  weights) under `~/.golem/snapshots`.
- `rehearse-spine.sh` the repair pass described above: it dry-runs the scenes you
  name to collect every percept and name they teach, then replays them in one flat
  two-round pass so a long grow's earliest categories are grounded again.
