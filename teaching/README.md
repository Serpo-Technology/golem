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
