package daemon

import (
	"os/exec"

	"golem/internal/events"
	"golem/internal/wire"
)

// The voice is the first effector after the hands: the realizer's
// sentences rendered to speech through the system text-to-speech. Pure
// output, no change to cognition: speaking never encodes, never warms
// the workspace, and never blocks the brain loop. Sentences queue and
// speak one at a time; when the mouth is busy past its small backlog, a
// reply is simply not voiced, because speech that arrives a minute late
// is worse than silence.

// sayCommand builds the speech subprocess; tests stub it.
var sayCommand = func(text string) *exec.Cmd {
	return exec.Command("/usr/bin/say", "--", text)
}

// speak queues one sentence for the voice when the voice is configured
// on. Called only from the brain loop.
func (d *Daemon) speak(text string) {
	if !d.cfg.Channels.Voice || text == "" {
		return
	}
	if d.voice == nil {
		d.voice = make(chan string, 8)
		go d.voiceLoop()
	}
	select {
	case d.voice <- text:
		d.event(events.Event{Ev: "say", Tpl: wire.Clip(text, 120)})
	default:
	}
}

// voiceLoop speaks the queue one sentence at a time until the daemon
// stops.
func (d *Daemon) voiceLoop() {
	for {
		select {
		case <-d.stop:
			return
		case text := <-d.voice:
			if err := sayCommand(text).Run(); err != nil {
				// A machine without the speech command stays mute
				// without complaint; the reply was still written.
				return
			}
		}
	}
}
