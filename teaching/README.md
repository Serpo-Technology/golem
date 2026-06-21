# Teaching GOLEM

A reproducible ostensive training run, in a container or against any reachable
daemon.

## In a container

    docker build -t golem:latest .
    docker run -d --name golem-brain \
      -v "$(pwd)/teaching/clean-room.json:/root/.golem/config.json:ro" golem:latest
    docker exec golem-brain env GAP=10 zsh /opt/curriculum.zsh

The image carries the curriculum at `/opt/curriculum.zsh`. Watch progress with
`docker exec golem-brain golem concepts` and `docker exec golem-brain golem status`.

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

- `curriculum.zsh` the lesson: vocabulary, the animal, metal, and plant
  categories, the green adjective, two Turkish words, the door-bell sequence,
  the thing superordinate, and a few chasings for the comprehension engine.
  Parameterized by `GOLEM` and `GAP`.
- `clean-room.json` the teaching config. Ambient sensors are off so only the
  lesson is observed, and the absolute surprise floor is lowered so a fresh
  brain's cold predictor still admits the everyday captions: on a cold
  predictor "a small black cat" carries only 0.5 bits, under the default 2.0
  floor, so nothing would encode. A warmed predictor over the normal config
  does not need this. Restore the default config for normal operation once a
  brain is taught.
- `mind-verbs.zsh` the experiential verbs of mind (want, love, watch),
  bilingual, taught as relations between an experiencer and a thing. Built on
  an existing world: restore the latest snapshot first.
- `snapshot.sh` save, restore, and list the brain's snapshots (its fluid
  weights) under `~/.golem/snapshots`.
