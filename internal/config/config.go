// Package config is everything a running GOLEM can be tuned with, the
// destructive-pattern denylist included.
package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Compiled-in defaults. The config file starts from these and any field
// missing from the file keeps its default.
const (
	defaultSurpriseThreshold = 2.0
	defaultPredictorWindow   = 256
	defaultSleepPressureGate = 0.8
	defaultInputSilenceSec   = 1.0
	defaultCalmSec           = 1.5
	defaultSleepIntervalSec  = 600
	defaultNightlyDecay      = 0.10
	defaultPruneFloor        = 0.10
	defaultZGate             = 2.5
	defaultCalibWarmup       = 30
	defaultGraduation        = 3
	defaultConflictMargin    = 0.15
	defaultLateralInhibition = 0.6
	defaultExecuteTimeoutSec = 30
	defaultExecuteParallel   = 4
	defaultAutoCueCooldown   = 60
	defaultWebPollSec        = 900
)

// actionDenylist is the set of command substrings GOLEM will always
// propose behind explicit confirmation and never graduate to autonomy, no
// matter how often they are approved. The shapes that are unrecoverable
// when wrong.
var denylist = []string{
	"rm -rf", "rm -fr", "mkfs", "dd if=", "dd of=",
	"shutdown", "reboot", "halt", ":(){", "sudo rm", "> /dev/",
	"git push --force", "git push -f", "drop table", "drop database",
}

// Config is everything a running GOLEM can be tuned with. It lives at
// <home>/config.json and is created with defaults on first run.
type Config struct {
	// SurpriseThreshold is the absolute floor (in bits) below which an
	// observation can never reach consciousness.
	SurpriseThreshold float64 `json:"surprise_threshold"`
	// PredictorWindow is the habituation timescale per channel, in
	// observations.
	PredictorWindow float64 `json:"predictor_window"`
	// SleepPressureGate is the serotonin level at which sleep can start.
	SleepPressureGate float64 `json:"sleep_pressure_gate"`
	// InputSilenceSec is how long all input must pause before sleep.
	InputSilenceSec float64 `json:"input_silence_sec"`
	// CalmBeforeSleepSec is how long after the last surprise sleep
	// pressure starts building.
	CalmBeforeSleepSec float64 `json:"calm_before_sleep_sec"`
	// SleepIntervalSec is the minimum gap between consolidation nights.
	SleepIntervalSec float64 `json:"sleep_interval_sec"`
	// NightlyDecay and PruneFloor govern consolidation.
	NightlyDecay float64 `json:"nightly_decay"`
	PruneFloor   float64 `json:"prune_floor"`
	// ZGate is how many deviations above its own channel's running mean a
	// surprise must sit to reach consciousness once the channel is warmed
	// up. CalibrationWarmup is the number of observations before the
	// relative gate applies.
	ZGate             float64 `json:"z_gate"`
	CalibrationWarmup int     `json:"calibration_warmup"`
	// EncodeBurst and EncodeRefillSec are the per-channel refractory on
	// memory formation: a channel may encode EncodeBurst fragments back to
	// back, then earns one more per EncodeRefillSec. Surprises beyond that
	// are counted and collapsed into a single storm-summary moment.
	EncodeBurst     float64 `json:"encode_burst"`
	EncodeRefillSec float64 `json:"encode_refill_sec"`
	// PredictorWindows overrides the habituation window per channel. A
	// chatty channel needs a longer memory to recognize its own long tail
	// of recurring shapes.
	PredictorWindows map[string]float64 `json:"predictor_windows"`
	// Channels selects which built-in sensors the daemon runs. External
	// senses (the shell hook, scripts) need no entry here; anything can
	// feed any channel through golem observe.
	Channels ChannelsConfig `json:"channels"`
	// GraduationApprovals is how many consistent approvals promote a
	// cue-and-command pair from confirm-to-execute to autonomous. A single
	// denial or failed run demotes it.
	GraduationApprovals int `json:"graduation_approvals"`
	// ConflictMargin is the ACC veto threshold: if the top two candidate
	// actions sit within this fraction of each other, GOLEM asks instead
	// of choosing.
	ConflictMargin float64 `json:"conflict_margin"`
	// LateralInhibition is how strongly a warmed concept damps the
	// concepts it grounds in, so a category held in mind quiets its
	// members instead of being swamped by them and a name taught over
	// categories can settle on the categories. Gentle by default; 0
	// turns the damping off and attention is plain warmth.
	LateralInhibition float64 `json:"lateral_inhibition"`
	// ExecuteEnabled is the master switch for actually running commands.
	// Off by default: until it is on, every approved or graduated action
	// is a dry run. Turning it on requires editing this file and
	// restarting; nothing can enable it from a single command.
	ExecuteEnabled bool `json:"execute_enabled"`
	// ExecuteAllowlist is the set of commands cleared to run for real,
	// matched by exact command or whole-word prefix ("go build" clears
	// "go build ./..." but not "google-chrome"). Empty means nothing runs,
	// even with the master switch on. The denylist always overrides this.
	ExecuteAllowlist []string `json:"execute_allowlist"`
	// ExecuteTimeoutSec bounds how long a single command may run.
	ExecuteTimeoutSec float64 `json:"execute_timeout_sec"`
	// ExecuteParallel caps how many committed actions may run at once.
	// Actions whose resource claims collide serialize regardless of the
	// cap; this bounds only the genuinely independent ones.
	ExecuteParallel int `json:"execute_parallel"`
	// RuntimeEnabled is the gate on running code GOLEM wrote. Off by default,
	// like ExecuteEnabled: the runtime opens only once the operator has placed
	// the whole process inside the sealed container, the containment a
	// precondition GOLEM trusts rather than enforces. Editing this file and
	// restarting is the only way to open it.
	RuntimeEnabled bool `json:"runtime_enabled"`
	// SourcePath points GOLEM at its own source tree for self-modification.
	// Empty keeps self-modification off; setting it is a second opt-in on top
	// of runtime_enabled, the operator deciding GOLEM may read and rebuild its
	// own source. The pipeline never edits this tree and never swaps the live
	// binary; it works on a copy and stages a candidate.
	SourcePath string `json:"source_path"`
	// MainPackage is the package the self-build compiles to the candidate
	// binary, the brain's entry point.
	MainPackage string `json:"main_package"`
	// AutoCueCooldownSec is the minimum gap between automatic firings of
	// the same graduated cue, so a storm of matching observations cannot
	// run one habit over and over.
	AutoCueCooldownSec float64 `json:"auto_cue_cooldown_sec"`
	// WebPollSec is how often each watched URL is fetched.
	WebPollSec float64 `json:"web_poll_sec"`
	// UntrustedChannels lists open-world senses whose content may be
	// adversarial: their moments encode at reduced weight and can never
	// cue a habit or anchor the grounding of an action. Omitted, the
	// list is the web channel.
	UntrustedChannels []string `json:"untrusted_channels"`
}

type ChannelsConfig struct {
	// Synthetic runs the demo's burst-and-quiet log generator, useful for
	// soak tests against a live daemon.
	Synthetic bool `json:"synthetic"`
	// Vitals samples cpu load, memory pressure, and disk space into bands
	// and reports band changes.
	Vitals bool `json:"vitals"`
	// Syslog follows the macOS unified log at error level.
	Syslog bool `json:"syslog"`
	// WatchDirs lists directories (not recursive) for file events.
	WatchDirs []string `json:"watch_dirs"`
	// TailFiles lists files whose appended lines arrive as observations
	// on the reading channel, distinct from the file-event sense.
	TailFiles []string `json:"tail_files"`
	// WatchURLs lists pages fetched periodically; their stripped text
	// arrives on the web channel, which is untrusted by default.
	WatchURLs []string `json:"watch_urls"`
	// Screen reports the frontmost application as a symbolic stream on
	// the screen channel.
	Screen bool `json:"screen"`
	// Voice renders spoken replies through the system text-to-speech.
	Voice bool `json:"voice"`
	// VoiceName picks which system voice speaks; empty means the system
	// default. golem voice adjusts both voice fields at runtime.
	VoiceName string `json:"voice_name"`
}

func Default() Config {
	return Config{
		SurpriseThreshold:   defaultSurpriseThreshold,
		PredictorWindow:     defaultPredictorWindow,
		SleepPressureGate:   defaultSleepPressureGate,
		InputSilenceSec:     defaultInputSilenceSec,
		CalmBeforeSleepSec:  defaultCalmSec,
		SleepIntervalSec:    defaultSleepIntervalSec,
		NightlyDecay:        defaultNightlyDecay,
		PruneFloor:          defaultPruneFloor,
		ZGate:               defaultZGate,
		CalibrationWarmup:   defaultCalibWarmup,
		EncodeBurst:         6,
		EncodeRefillSec:     2,
		PredictorWindows:    map[string]float64{"syslog": 8192},
		GraduationApprovals: defaultGraduation,
		ConflictMargin:      defaultConflictMargin,
		LateralInhibition:   defaultLateralInhibition,
		ExecuteEnabled:      false,
		RuntimeEnabled:      false,
		SourcePath:          "",
		MainPackage:         "./cmd/golem",
		ExecuteAllowlist:    []string{},
		ExecuteTimeoutSec:   defaultExecuteTimeoutSec,
		ExecuteParallel:     defaultExecuteParallel,
		AutoCueCooldownSec:  defaultAutoCueCooldown,
		WebPollSec:          defaultWebPollSec,
		UntrustedChannels:   []string{"web"},
		Channels: ChannelsConfig{
			Vitals:    true,
			Syslog:    true,
			WatchDirs: []string{},
			TailFiles: []string{},
			WatchURLs: []string{},
			VoiceName: "Samantha",
		},
	}
}

func (c Config) InputSilence() time.Duration {
	return time.Duration(c.InputSilenceSec * float64(time.Second))
}
func (c Config) CalmBeforeSleep() time.Duration {
	return time.Duration(c.CalmBeforeSleepSec * float64(time.Second))
}
func (c Config) SleepInterval() time.Duration {
	return time.Duration(c.SleepIntervalSec * float64(time.Second))
}
func (c Config) AutoCueCooldown() time.Duration {
	if c.AutoCueCooldownSec <= 0 {
		return defaultAutoCueCooldown * time.Second
	}
	return time.Duration(c.AutoCueCooldownSec * float64(time.Second))
}

// Home resolves the state directory: $GOLEM_HOME if set, otherwise
// ~/.golem. Memory, config, socket, and logs all live here.
func Home() (string, error) {
	if h := os.Getenv("GOLEM_HOME"); h != "" {
		return h, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".golem"), nil
}

// Load reads <home>/config.json, creating it with defaults on first
// run so there is a file to edit.
func Load(home string) (Config, error) {
	cfg := Default()
	path := filepath.Join(home, "config.json")
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		if mkErr := os.MkdirAll(home, 0o755); mkErr != nil {
			return cfg, mkErr
		}
		pretty, _ := json.MarshalIndent(cfg, "", "  ")
		if wrErr := os.WriteFile(path, append(pretty, '\n'), 0o644); wrErr != nil {
			return cfg, wrErr
		}
		return cfg, nil
	}
	if err != nil {
		return cfg, err
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		return cfg, err
	}
	return cfg, nil
}

// Save writes the configuration back to <home>/config.json atomically,
// so a kill mid-write cannot leave the file torn. The daemon calls this
// when an operator command (golem voice) changes a setting that should
// survive a restart.
func Save(home string, cfg Config) error {
	pretty, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	path := filepath.Join(home, "config.json")
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, append(pretty, '\n'), 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// Denylisted reports whether a command contains one of the shapes GOLEM
// must never run without explicit confirmation, regardless of how often
// it is approved.
func Denylisted(cmd string) bool {
	low := strings.ToLower(cmd)
	for _, p := range denylist {
		if strings.Contains(low, p) {
			return true
		}
	}
	return false
}

// Allowlisted matches a command against the operator's allowlist by
// exact command or whole-word prefix, so an entry cannot match a longer
// unrelated command ("go build" clears "go build ./..." but not
// "google-chrome").
func Allowlisted(cmd string, allow []string) bool {
	c := strings.TrimSpace(strings.ToLower(cmd))
	for _, p := range allow {
		p = strings.TrimSpace(strings.ToLower(p))
		if p == "" {
			continue
		}
		if c == p || strings.HasPrefix(c, p+" ") {
			return true
		}
	}
	return false
}

// ExecTimeout resolves the configured per-command bound.
func (c Config) ExecTimeout() time.Duration {
	t := time.Duration(c.ExecuteTimeoutSec * float64(time.Second))
	if t <= 0 {
		t = defaultExecuteTimeoutSec * time.Second
	}
	return t
}

// WebPoll resolves the fetch interval for watched pages.
func (c Config) WebPoll() time.Duration {
	t := time.Duration(c.WebPollSec * float64(time.Second))
	if t <= 0 {
		t = defaultWebPollSec * time.Second
	}
	return t
}

// Untrusted reports whether a channel carries open-world perception
// that may be adversarial. An untrusted moment encodes at reduced
// weight and is barred from cueing habits and from anchoring the
// grounding of an action; a missing list defaults to the web channel.
func (c Config) Untrusted(channel string) bool {
	list := c.UntrustedChannels
	if list == nil {
		list = []string{"web"}
	}
	for _, ch := range list {
		if ch == channel {
			return true
		}
	}
	return false
}
