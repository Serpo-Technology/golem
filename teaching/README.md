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

## Against an existing daemon

    GOLEM=golem GAP=15 zsh teaching/curriculum.zsh

## Files

- `curriculum.zsh` the foundation lesson: vocabulary, the animal, metal, and
  plant categories, the green adjective, two Turkish words, the door-bell
  sequence, the thing superordinate, and a few chasings for the comprehension
  engine. Each scene forces its taught percept (`fobs`) and leaves the
  combinatorial companions ambient (`obs`). Parameterized by `GOLEM` and `GAP`.
- `world.zsh`, `world-bilingual.zsh`, `world-breadth.zsh`, `world-foods.zsh`,
  `world-countries.zsh` broaden the world (more animals, plants, foods, and
  countries, bilingual). They force the taught percept the same way and build on
  the curriculum: restore its snapshot first.
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
