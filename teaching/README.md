# Teaching GOLEM

A reproducible ostensive training run, in a container or against any reachable
daemon. Each scene directs attention at its taught percept with a forced
observation, so the same lesson teaches identically whether the sensors are off
(the quiet room) or on (always-online). Only the companions and any ambient
sensors differ between the two.

## In a container (quiet room)

    docker build -t golem:latest .
    docker run -d --name golem-brain \
      -v "$(pwd)/teaching/clean-room.json:/root/.golem/config.json:ro" golem:latest
    docker exec golem-brain env GAP=10 zsh /opt/curriculum.zsh

The image carries the curriculum at `/opt/curriculum.zsh`. Watch progress with
`docker exec golem-brain golem concepts` and `docker exec golem-brain golem status`.

## Always-online (sensors on)

The quiet room disables habituation (`calibration_warmup: 5000`) and turns every
ambient sensor off, so nothing but the lesson is ever seen. It is not the only
way to teach. With the graded attention gate on, a lesson runs with the sensors
live: the taught percept is forced, so it is attended and learned at full
salience however routine repetition would otherwise habituate it; a channel
listed in `subconscious_channels` is treated as background, so its observations
withhold salience and never enter a taught concept's company. Set
`calibration_warmup: 30`, `subliminal_enabled: true`, and list only the ambient
channels (for example `["vitals"]`, never the teaching channel) in
`subconscious_channels`. Everything else about the lesson is unchanged.

## Building on a snapshot (cumulative milestones)

Each milestone builds on the last. Restore the latest snapshot into the
container before teaching, so prior knowledge is never lost.

    docker build -t golem:latest .
    docker run -d --name golem-brain \
      -v "$(pwd)/teaching/clean-room.json:/root/.golem/config.json:ro" golem:latest
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
- `clean-room.json`, `clean-room-code.json` the quiet-room teaching config.
  Ambient sensors are off so only the lesson is observed, and the absolute
  surprise floor is lowered so a fresh brain's cold predictor still admits the
  everyday captions: on a cold predictor "a small black cat" carries only 0.5
  bits, under the default 2.0 floor, so nothing would encode. `-code` adds the
  TCP listener and the code faculty. A warmed predictor over the normal config
  does not need the lowered floor. Restore the default config for normal
  operation once a brain is taught.
- `snapshot.sh` save, restore, and list the brain's snapshots (its fluid
  weights) under `~/.golem/snapshots`.
