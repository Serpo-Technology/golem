package config

import (
	"os"
	"path/filepath"
	"testing"
)

// A .env in the home directory feeds variables into the environment without
// overriding one already set, and comments and blank lines are skipped.
func TestLoadDotenv(t *testing.T) {
	home := t.TempDir()
	body := "# a comment\n\nexport GOLEM_ARCHITECT_PUBKEY=abc123\nGOLEM_LISTEN_TCP=\"127.0.0.1:9\"\nGOLEM_PRESET=fromfile\n"
	if err := os.WriteFile(filepath.Join(home, ".env"), []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GOLEM_ARCHITECT_PUBKEY", "")
	t.Setenv("GOLEM_LISTEN_TCP", "")
	t.Setenv("GOLEM_PRESET", "fromenv") // already set: the file must not override

	LoadDotenv(home)
	if got := os.Getenv("GOLEM_ARCHITECT_PUBKEY"); got != "abc123" {
		t.Fatalf("the pubkey should come from the file: %q", got)
	}
	if got := os.Getenv("GOLEM_LISTEN_TCP"); got != "127.0.0.1:9" {
		t.Fatalf("a quoted value should be unquoted: %q", got)
	}
	if got := os.Getenv("GOLEM_PRESET"); got != "fromenv" {
		t.Fatalf("a set variable must not be overridden by the file: %q", got)
	}
}

// The environment overlays the loaded config for the deployment fields.
func TestEnvOverlaysConfig(t *testing.T) {
	home := t.TempDir()
	if _, err := Load(home); err != nil { // writes a default config.json
		t.Fatal(err)
	}
	t.Setenv("GOLEM_ARCHITECT_PUBKEY", "env-key")
	t.Setenv("GOLEM_LISTEN_TCP", "127.0.0.1:5555")
	cfg, err := Load(home)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.ArchitectPubKey != "env-key" {
		t.Fatalf("the environment should set the pubkey: %q", cfg.ArchitectPubKey)
	}
	if cfg.ListenTCP != "127.0.0.1:5555" {
		t.Fatalf("the environment should set the bind address: %q", cfg.ListenTCP)
	}
}

// A first boot, before any config.json exists, still applies the environment,
// so a container handed its key and port by env is configured on its first run.
func TestFirstRunAppliesEnv(t *testing.T) {
	home := t.TempDir()
	t.Setenv("GOLEM_ARCHITECT_PUBKEY", "first-run-key")
	t.Setenv("GOLEM_LISTEN_TCP", "127.0.0.1:6")
	cfg, err := Load(home)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.ArchitectPubKey != "first-run-key" || cfg.ListenTCP != "127.0.0.1:6" {
		t.Fatalf("a first-run load should apply env: %q %q", cfg.ArchitectPubKey, cfg.ListenTCP)
	}
}

// Save must not bake an env-only key or bind address into the file: a voice or
// runtime toggle persists its own change while the env fields stay env-only.
func TestSaveDoesNotBakeEnvFields(t *testing.T) {
	home := t.TempDir()
	if _, err := Load(home); err != nil { // writes a default config.json (empty env fields)
		t.Fatal(err)
	}
	t.Setenv("GOLEM_ARCHITECT_PUBKEY", "env-only-key")
	t.Setenv("GOLEM_LISTEN_TCP", "0.0.0.0:7777")
	cfg, _ := Load(home) // the env overlays these two fields
	cfg.Channels.VoiceName = "changed"
	if err := Save(home, cfg); err != nil {
		t.Fatal(err)
	}
	disk, err := loadRaw(home)
	if err != nil {
		t.Fatal(err)
	}
	if disk.ArchitectPubKey != "" || disk.ListenTCP != "" {
		t.Fatalf("env-only fields must not be baked into the file: %q %q", disk.ArchitectPubKey, disk.ListenTCP)
	}
	if disk.Channels.VoiceName != "changed" {
		t.Fatalf("a real setting change should still persist: %q", disk.Channels.VoiceName)
	}
}
