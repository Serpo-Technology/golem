# GOLEM in a container: the operator's sandbox the design calls for. The Go
# toolchain image carries the compiler the self-build pipeline and the code
# runtime lean on, plus zsh for the teaching curriculum. The daemon owns the
# foreground; teach and probe it with `docker exec <name> golem ...`.
FROM golang:1.25

RUN apt-get update \
    && apt-get install -y --no-install-recommends zsh ca-certificates \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /src
COPY . .
RUN go build -o /usr/local/bin/golem ./cmd/golem

# The curriculum travels with the image so the brain can be retaught.
COPY teaching/curriculum.zsh /opt/curriculum.zsh

WORKDIR /root
CMD ["golem", "run"]
