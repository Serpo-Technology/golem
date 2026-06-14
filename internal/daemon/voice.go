package daemon

import (
	"fmt"
	"os/exec"
	"strings"

	"golem/internal/config"
	"golem/internal/events"
	"golem/wire"
)

// The voice is the first effector after the hands: the realizer's
// sentences rendered to speech through the system text-to-speech. Pure
// output, no change to cognition: speaking never encodes, never warms
// the workspace, and never blocks the brain loop. Sentences queue and
// speak one at a time; when the mouth is busy past its small backlog, a
// reply is simply not voiced, because speech that arrives a minute late
// is worse than silence.
//
// Whether the voice is on and which system voice speaks are runtime
// state, adjusted over the socket by golem voice and persisted to the
// config file so a restart keeps the choice.

// sayCommand builds the speech subprocess; tests stub it. An empty
// voice name leaves the system default in charge.
var sayCommand = func(text, voice string) *exec.Cmd {
	args := []string{}
	if voice != "" {
		args = append(args, "-v", voice)
	}
	args = append(args, "--", text)
	return exec.Command("/usr/bin/say", args...)
}

// utterance is one queued sentence with the voice chosen at queueing
// time, so the speaking goroutine never reads loop-owned state.
type utterance struct {
	text  string
	voice string
}

// speak queues one sentence for the voice when the voice is on. Called
// only from the brain loop.
func (d *Daemon) speak(text string) {
	if !d.voiceOn || text == "" {
		return
	}
	if d.voice == nil {
		d.voice = make(chan utterance, 8)
		// The say builder is captured here, on the loop, so the
		// speaking goroutine reads no shared state afterwards.
		go d.voiceLoop(sayCommand)
	}
	select {
	case d.voice <- utterance{text: text, voice: d.voiceName}:
		d.event(events.Event{Ev: "say", Tpl: wire.Clip(text, 120)})
	default:
	}
}

// voiceLoop speaks the queue one sentence at a time until the daemon
// stops.
func (d *Daemon) voiceLoop(say func(text, voice string) *exec.Cmd) {
	for {
		select {
		case <-d.stop:
			return
		case u := <-d.voice:
			if err := say(u.text, u.voice).Run(); err != nil {
				// A machine without the speech command stays mute
				// without complaint; the reply was still written.
				return
			}
		}
	}
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
