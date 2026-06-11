package daemon

import (
	"os/exec"
	"strings"
	"testing"
	"time"

	"golem/internal/config"
	"golem/internal/wire"
)

// golem voice flips the runtime switch, picks the speaking voice, and
// persists both to the config file so a restart agrees with the menu.
func TestVoiceControl(t *testing.T) {
	d, _ := testDaemon(t)
	d.home = t.TempDir()

	spoken := make(chan string, 4)
	orig := sayCommand
	sayCommand = func(text, voice string) *exec.Cmd {
		spoken <- voice + "|" + text
		return exec.Command("/usr/bin/true")
	}
	defer func() { sayCommand = orig }()

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

	if resp = d.handleVoice(wire.Request{Arg: "use", Content: "Yelda"}); !resp.OK || resp.Voice.Name != "Yelda" {
		t.Fatalf("voice use failed: %+v", resp)
	}
	select {
	case got := <-spoken:
		if !strings.HasPrefix(got, "Yelda|") {
			t.Fatalf("the sample should speak in the chosen voice, got %q", got)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("choosing a voice while on should speak a sample")
	}
	if cfg, _ := config.Load(d.home); cfg.Channels.VoiceName != "Yelda" {
		t.Fatal("voice name did not persist")
	}

	// An empty name returns to the system default; a malformed one is
	// refused outright.
	if resp = d.handleVoice(wire.Request{Arg: "use", Content: ""}); !resp.OK || resp.Voice.Name != "" {
		t.Fatalf("clearing the voice name failed: %+v", resp)
	}
	if resp = d.handleVoice(wire.Request{Arg: "use", Content: "a\nb"}); resp.Error == "" {
		t.Fatal("a control character in a voice name must be refused")
	}

	if resp = d.handleVoice(wire.Request{Arg: "off"}); !resp.OK || resp.Voice.Enabled {
		t.Fatalf("voice off failed: %+v", resp)
	}
	if cfg, _ := config.Load(d.home); cfg.Channels.Voice {
		t.Fatal("voice off did not persist")
	}
	if resp = d.handleVoice(wire.Request{Arg: "louder"}); resp.Error == "" {
		t.Fatal("an unknown subcommand must be refused")
	}
}
