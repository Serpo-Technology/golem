# Golem Toolkit

The Golem Toolkit is the primary way to work a GOLEM brain: conversing with it,
feeding its senses, watching its inner life, and shaping its memory, all over one
connection. The brain it works usually runs in a container, reached over the
signed network transport, so every request the toolkit sends is signed with the
architect's key. It can also talk to a brain beside it on the unix socket,
unsigned, when pointed at one.

It ships in two faces of one client:

- **`gtk`, the console** (`cmd/gtk`): a CLI over the same connection, for reading
  and shaping the live graph from a terminal.
- **The window**: a Wails desktop app (Go backend, Svelte frontend) whose panels
  are one connection to the brain: Chat, Senses, Memory, Trail, Mind, Identity,
  Controls, and Settings.

## Generating the keypair

The brain authenticates the architect by an Ed25519 signature. You hold the
private key; the brain holds only the matching public key. Make a keypair once
with the `golem` binary:

```bash
golem keygen
```

It prints two things:

- the **public key**, to register with the brain, as `architect_pubkey` in its
  config or in the `GOLEM_ARCHITECT_PUBKEY` environment variable;
- the **private key**, to keep on your machine and give the toolkit through
  `GOLEM_PRIVATE_KEY`, a file named by `GOLEM_PRIVATE_KEY_FILE`, or the Settings
  panel. The brain never sees it.

Keep the private key secret and out of version control (a `.env` and `*.key` are
already git-ignored). To rotate it, run `keygen` again and re-register the new
public key.

## Running the brain in a container

For the toolkit on your machine to reach a brain in a container, the brain must
listen on all interfaces inside the container and publish its port to host
loopback only. A container-loopback bind is unreachable through Docker's port
forward, so `listen_tcp` inside the container is `0.0.0.0:7777`, and Docker
publishes it to `127.0.0.1`. The signature requirement is what keeps that port
safe: an unsigned or wrongly-signed request is refused.

```bash
docker run -d --name golem-brain \
  -p 127.0.0.1:7777:7777 \
  -e GOLEM_ARCHITECT_PUBKEY="$(cat architect.pub)" \
  -v "$PWD/runtime:/root/.golem" \
  golem:latest golem run
```

The startup log notes that the port is reachable off-host and every request must
be signed; with no public key set it warns that the transport will refuse
everything.

## Building and connecting the toolkit

The console:

```bash
go -C gtk build -o gtk ./cmd/gtk
GOLEM_PRIVATE_KEY="$(cat architect.key)" gtk/gtk status
```

The window (install the Wails CLI once):

```bash
go install github.com/wailsapp/wails/v2/cmd/wails@latest
cd gtk
wails dev          # live-reload during development
wails build        # a bundled app in build/bin/GolemToolkit.app
```

Give the window the private key by launching it with `GOLEM_PRIVATE_KEY` set, or
enter a key file path in the **Settings** panel. Point Settings at the brain's
address (`tcp` and `127.0.0.1:7777` for the container above) and connect. The
connection is saved to `~/.golem/gtk.json`, so the next launch reconnects to it.

## The panels

- **Chat** conversing with the brain: type or speak (the browser's speech
  recognition, when the window's webview offers it), and hear replies and the
  brain's own spontaneous speech aloud (the browser's speech synthesis).
- **Senses** filing a grounded moment on a channel, and showing the brain an
  image, which it measures itself and answers for in its own words.
- **Memory** searching by cue with in-place edit, mark, and delete; the distilled
  concepts; and the interactive graph of the live store.
- **Trail** the decision trail beside a live feed of the brain's own events as it
  hears, thinks, speaks, and acts.
- **Mind** the reasoning state: resemblance and beliefs, and the naming-graph
  health.
- **Identity** setting how far the brain trusts a source, and enrolling or
  revoking a peer's public key so a peer can prove its own identity.
- **Controls** the code runtime, the voice, on-demand consolidation, forgetting,
  and retuning a tuning constant (the build runs off the brain loop and the
  deploy stays yours).
- **Settings** the connection itself: transport, address, and signing key.

## Module layout

The toolkit is its own Go module (`gtk`), so the daemon's dependency tree stays
small and the webview stack lives only here. It shares the daemon's `wire`
package for the protocol and signing, and defines its own view of the event
stream so it needs none of the daemon's internals. The built frontend
(`frontend/dist`) and the generated Wails bindings (`frontend/wailsjs`) are
git-ignored and produced by the build.
