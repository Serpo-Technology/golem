package daemon

import (
	"fmt"
	"strings"

	"golem/internal/config"
	"golem/internal/events"
	"golem/wire"
)

// The voice is pure output, and the brain does not render it. Speaking a
// sentence is emitting a "say" event that carries the words; a transducer
// outside the process reads the event stream and voices them with whatever
// the platform speaks through. This keeps the speaker the host's to choose
// and the brain identical on every machine, the same separation perception
// already has, where a captioner outside the process feeds the senses.
// Emitting never encodes, never warms the workspace, never blocks the loop.
//
// Whether the voice is on and which voice an adapter should prefer are runtime
// state, adjusted over the socket by golem voice and persisted to the config.

// speak emits one sentence as a say event when the voice is on, for an external
// speaker to render. Called only from the brain loop.
func (d *Daemon) speak(text string) {
	if !d.voiceOn || text == "" {
		return
	}
	d.event(events.Event{Ev: "say", Tpl: wire.Clip(text, 240)})
}

// handleVoice is the runtime control surface of the speech effector:
// status, on, off, and use <name>. Mutations persist to the config file
// and land in the event trail. Called only from the brain loop.
func (d *Daemon) handleVoice(req wire.Request) wire.Response {
	state := func() *wire.VoiceInfo {
		return &wire.VoiceInfo{Enabled: d.voiceOn, Name: d.voiceName}
	}
	switch req.Arg {
	case "", "status":
		return wire.Response{OK: true, Voice: state()}
	case "on":
		d.voiceOn = true
		d.cfg.Channels.Voice = true
	case "off":
		d.voiceOn = false
		d.cfg.Channels.Voice = false
	case "use":
		name := strings.TrimSpace(req.Content)
		if len(name) > 64 || strings.ContainsAny(name, "\n\r") {
			return wire.Response{Error: "not a voice name"}
		}
		d.voiceName = name
		d.cfg.Channels.VoiceName = name
	default:
		return wire.Response{Error: "voice takes status, on, off, or use <name>"}
	}
	if err := config.Save(d.home, d.cfg); err != nil {
		fmt.Fprintf(d.out, "voice setting not persisted: %v\n", err)
	}
	d.event(events.Event{Ev: "voice", Tpl: strings.TrimSpace(req.Arg + " " + d.voiceName)})
	if req.Arg == "use" {
		// Hearing the new voice at once beats discovering it on the
		// next reply; muted stays muted.
		d.speak("This is my voice.")
	}
	return wire.Response{OK: true, Voice: state()}
}
