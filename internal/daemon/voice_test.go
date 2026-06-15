package daemon

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"golem/internal/config"
	"golem/internal/events"
	"golem/wire"
)

// golem voice flips the runtime switch, picks the preferred voice, and
// persists both to the config so a restart agrees with the menu. Speaking is
// emit-only now: a sample is a say event for an external speaker to render,
// not a subprocess the brain runs.
func TestVoiceControl(t *testing.T) {
	d, _ := testDaemon(t)
	d.home = t.TempDir()
	log, err := events.Open(d.home)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { log.Close() })
	d.events = log
	saidSamples := func() int {
		raw, _ := os.ReadFile(filepath.Join(d.home, "events.jsonl"))
		return strings.Count(string(raw), `"ev":"say"`)
	}

	resp := d.handleVoice(wire.Request{Arg: "status"})
	if !resp.OK || resp.Voice == nil || resp.Voice.Enabled {
		t.Fatalf("voice should start off: %+v", resp)
	}

	if resp = d.handleVoice(wire.Request{Arg: "on"}); !resp.OK || !resp.Voice.Enabled {
		t.Fatalf("voice on failed: %+v", resp)
	}
	if cfg, err := config.Load(d.home); err != nil || !cfg.Channels.Voice {
		t.Fatalf("voice on did not persist: %v %+v", err, cfg.Channels)
	}

	// Choosing a voice while on emits one say event (the sample), not a process.
	if resp = d.handleVoice(wire.Request{Arg: "use", Content: "Yelda"}); !resp.OK || resp.Voice.Name != "Yelda" {
		t.Fatalf("voice use failed: %+v", resp)
	}
	if saidSamples() != 1 {
		t.Fatalf("choosing a voice while on should emit one say event, got %d", saidSamples())
	}
	if cfg, _ := config.Load(d.home); cfg.Channels.VoiceName != "Yelda" {
		t.Fatal("voice name did not persist")
	}

	// An empty name returns to the default; a malformed one is refused.
	if resp = d.handleVoice(wire.Request{Arg: "use", Content: ""}); !resp.OK || resp.Voice.Name != "" {
		t.Fatalf("clearing the voice name failed: %+v", resp)
	}
	if resp = d.handleVoice(wire.Request{Arg: "use", Content: "a\nb"}); resp.Error == "" {
		t.Fatal("a control character in a voice name must be refused")
	}

	// Off, speaking emits nothing.
	if resp = d.handleVoice(wire.Request{Arg: "off"}); !resp.OK || resp.Voice.Enabled {
		t.Fatalf("voice off failed: %+v", resp)
	}
	before := saidSamples()
	d.speak("this should be silent")
	if saidSamples() != before {
		t.Fatal("a muted voice must not emit a say event")
	}
	if cfg, _ := config.Load(d.home); cfg.Channels.Voice {
		t.Fatal("voice off did not persist")
	}
	if resp = d.handleVoice(wire.Request{Arg: "louder"}); resp.Error == "" {
		t.Fatal("an unknown subcommand must be refused")
	}
}
