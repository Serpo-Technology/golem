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
	defaultEpisodeGapSec     = 120
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
// <home>/config.json and is created with defaults on first run. The fields
// are grouped by what they govern: attention, memory formation, sleep,
// senses, action, self-build, and the network the brain is reached over.
type Config struct {
	// Attention and consciousness: what an observation must clear to be
	// noticed, and how a concept held in mind colours what is noticed next.

	// SurpriseThreshold is the absolute floor (in bits) below which an
	// observation can never reach consciousness.
	SurpriseThreshold float64 `json:"surprise_threshold"`
	// ZGate is how many deviations above its own channel's running mean a
	// surprise must sit to reach consciousness once the channel is warmed up.
	ZGate float64 `json:"z_gate"`
	// CalibrationWarmup is the number of observations before the relative
	// ZGate applies; until then the absolute SurpriseThreshold stands alone.
	CalibrationWarmup int `json:"calibration_warmup"`
	// PredictorWindow is the habituation timescale per channel, in
	// observations.
	PredictorWindow float64 `json:"predictor_window"`
	// PredictorWindows overrides the habituation window per channel. A chatty
	// channel needs a longer memory to recognize its own long tail of
	// recurring shapes.
	PredictorWindows map[string]float64 `json:"predictor_windows"`
	// LateralInhibition is how strongly a warmed concept damps the concepts
	// it grounds in, so a category held in mind quiets its members instead of
	// being swamped by them and a name taught over categories can settle on
	// the categories. Gentle by default; 0 turns the damping off and
	// attention is plain warmth.
	LateralInhibition float64 `json:"lateral_inhibition"`

	// Memory formation: the refractory that bounds how fast moments encode,
	// so a burst of surprise cannot flood the store.

	// EncodeBurst and EncodeRefillSec are the per-channel refractory on memory
	// formation: a channel may encode EncodeBurst fragments back to back, then
	// earns one more per EncodeRefillSec. Surprises beyond that are counted and
	// collapsed into a single storm-summary moment.
	EncodeBurst     float64 `json:"encode_burst"`
	EncodeRefillSec float64 `json:"encode_refill_sec"`
	// EncodeBursts overrides the encode burst per channel. A bulk channel like
	// the web delivers a page's worth of lines at once, so it earns a larger
	// burst to take in several sources a cycle, while a trickle channel keeps
	// the small default.
	EncodeBursts map[string]float64 `json:"encode_bursts"`

	// Sleep and consolidation: when the daemon rests to replay, merge, and
	// forget.

	// SleepPressureGate is the serotonin level at which sleep can start.
	SleepPressureGate float64 `json:"sleep_pressure_gate"`
	// InputSilenceSec is how long all input must pause before sleep.
	InputSilenceSec float64 `json:"input_silence_sec"`
	// CalmBeforeSleepSec is how long after the last surprise sleep pressure
	// starts building.
	CalmBeforeSleepSec float64 `json:"calm_before_sleep_sec"`
	// SleepIntervalSec is the minimum gap between consolidation nights.
	SleepIntervalSec float64 `json:"sleep_interval_sec"`
	// NightlyDecay is how much every weight fades each night; PruneFloor is the
	// weight below which a faded fragment is dropped.
	NightlyDecay float64 `json:"nightly_decay"`
	PruneFloor   float64 `json:"prune_floor"`

	// Senses and channels: which built-in sensors the daemon runs, and how the
	// open-world ones are treated. A transducer outside the process can feed
	// any channel through golem observe and needs no entry here.

	// Channels selects which built-in sensors the daemon runs.
	Channels ChannelsConfig `json:"channels"`
	// WebPollSec is how often each watched URL is fetched.
	WebPollSec float64 `json:"web_poll_sec"`
	// UntrustedChannels lists open-world senses whose content may be
	// adversarial: their moments encode at reduced weight and can never cue a
	// habit or anchor the grounding of an action. Omitted, the list is the web
	// channel.
	UntrustedChannels []string `json:"untrusted_channels"`

	// SubliminalEnabled turns on graded attention: a habituated observation on a
	// SubconsciousChannel still encodes below consciousness at low salience rather
	// than being dropped, so learning continues while attention only governs
	// salience. Off by default, so the gate stays binary until a channel opts in.
	SubliminalEnabled bool `json:"subliminal_enabled"`
	// KindLadderEnabled turns on the kind ladder: consolidation derives a defeasible
	// subcategory edge (a kind is-a a strictly broader category that grounds most of its
	// members),
	// and an is-a question climbs it to reach a superordinate up the kind chain even after
	// the superordinate's own thin percept grounding has faded. Read only by the hedged
	// is-a branch, so it softens a "no" to a "probably" and never asserts a firm falsehood.
	// Off by default: held apart from the pointing gate because the is-a measurements cannot
	// see it at all, being read only by the hedged branch.
	KindLadderEnabled bool `json:"kind_ladder_enabled"`
	// OstensiveCoreEnabled decides a category's membership from what its word was
	// POINTED at rather than from the company the word happened to keep. A word said
	// over a percept earns one ostension edge to the freshest thing in view, while its
	// naming company reaches every percept still in the window, so a dense teaching
	// stream spreads the company across the whole room while the ostension stays on the
	// thing meant. Where a word points at nothing the company still decides, so a
	// category taught only in talk is untouched. On by default: set it false for the
	// company reading that came before.
	OstensiveCoreEnabled bool `json:"ostensive_core_enabled"`
	// OstensiveNamingEnabled applies the same pointing test to the nightly naming
	// recount: a word that was shown things keeps its bond only to the ones it was
	// shown, and its remaining shares are read against that company rather than the
	// whole room. Held apart from the membership gate because naming is read by far
	// more than is-a, so it can be turned off alone. Off by default: it carries none of the membership gain, and
	// switched on under a brain grown without it, one surviving pointing is enough to write
	// every other bond of that word down by half a night until it is dropped.
	OstensiveNamingEnabled bool `json:"ostensive_naming_enabled"`
	// RetireDeclinedRungsEnabled gives the coverage test teeth. A class earns its place
	// inside a broader class by what it covers, and the test refuses the rungs that have
	// not earned it, but refusing costs the rung almost nothing: it is left to fade a
	// fiftieth a night from as high as four tenths, so it goes on carrying a firm
	// membership to everything beneath it for some seventy nights after the pass stopped
	// believing it. A rung the pass declines is now written down deliberately, and down
	// rather than away, so one demoted by a gap in the record can climb back. Only the
	// rungs between two classes, which is what the coverage test judges. Off by default. Grown
	// from empty it costs edge precision: across the arms that carry the naming pass, every run
	// with this on scores below every run without it, while what it gains elsewhere stays under
	// the spread between two runs of one config. On a brain grown without it, the whole
	// accumulated backlog is declined at once on the first night.
	RetireDeclinedRungsEnabled bool `json:"retire_declined_rungs_enabled"`
	// UnshownNamesByOwnShareEnabled names a word that was never shown anything by its own
	// company alone. The nightly recount blends the share of the word's company that lands on
	// a concept with the share of that concept's company which is this word, and the second
	// half exists to tell a naming from a passing warmth beside a percept. A word carried
	// wholly in talk has no percept at all, so there is nothing for it to settle, and it does
	// the one thing left to it: it divides by the company of the whole class, handing the word
	// to whichever class is obscurest. A bat kept company with the animals at 0.62 and with a
	// Turkish word for the moon at 0.43 and was named for the moon, four to one, because a
	// hundred words are taught under animal and a handful under the moon; fifteen creatures
	// ended up owning no other name. Off by default.
	UnshownNamesByOwnShareEnabled bool `json:"unshown_names_by_own_share_enabled"`
	// ResolveToOwnConceptEnabled settles which concept a bare noun refers to when its
	// word names several. The word's own abstraction wins over a percept it merely
	// drifted onto, and the perceptless node a telling minted for it loses to the thing
	// once something has shown one. Without this a drifted bond, which the nightly fade
	// spares and recall only strengthens, keeps the word pointing at the wrong thing for
	// good. Off by default: the is-a measurements cannot see it, since it settles resolution
	// rather than membership.
	ResolveToOwnConceptEnabled bool `json:"resolve_to_own_concept_enabled"`
	// SubconsciousChannels are the trusted channels graded attention applies to, so
	// ambient host channels stay fully gated until proven not to pollute. An
	// untrusted channel never learns sub-consciously whatever this lists. A
	// sub-conscious trace is faint on every axis: low weight, low genesis mass that
	// also decays in time, throttled intake, no naming or episodic reach, and repeats
	// within one continuous exposure are de-duplicated (see EpisodeGapSec). So a concept
	// is earned only by recurring across distinct episodes faster than the mass bleeds,
	// not by dwelling on one thing; a continuous ambient stream credits once and fades.
	SubconsciousChannels []string `json:"subconscious_channels"`
	// EpisodeGapSec is the sub-conscious de-duplication window: a habituated percept
	// seen again within this many seconds of its last sighting is the same continuous
	// episode and does not re-credit genesis mass. Larger folds looser recurrence into
	// one exposure (fewer, slower concepts); smaller treats spaced sightings as distinct.
	EpisodeGapSec float64 `json:"episode_gap_sec"`

	// ArchitectPubKey is the architect's Ed25519 public key, base64. The daemon
	// holds only this public half and verifies the architect's signature against
	// it; the private key never touches the daemon. A request that carries
	// authority (setting trust, retuning the source) must bear a signature this
	// key verifies, and every request over the network transport must too. Empty
	// leaves it unset, so no signed authority is accepted and the network
	// transport refuses every request. Set it from the environment with
	// GOLEM_ARCHITECT_PUBKEY, which overrides the file.
	ArchitectPubKey string `json:"architect_pubkey"`

	// Action and execution: how a cue graduates to a habit, and the gates on
	// actually running commands.

	// GraduationApprovals is how many consistent approvals promote a
	// cue-and-command pair from confirm-to-execute to autonomous. A single
	// denial or failed run demotes it.
	GraduationApprovals int `json:"graduation_approvals"`
	// ConflictMargin is the veto threshold: if the top two candidate actions
	// sit within this fraction of each other, GOLEM asks instead of choosing.
	ConflictMargin float64 `json:"conflict_margin"`
	// AutoCueCooldownSec is the minimum gap between automatic firings of the
	// same graduated cue, so a storm of matching observations cannot run one
	// habit over and over.
	AutoCueCooldownSec float64 `json:"auto_cue_cooldown_sec"`
	// ExecuteEnabled is the master switch for actually running commands. Off by
	// default: until it is on, every approved or graduated action is a dry run.
	// Turning it on requires editing this file and restarting; nothing can
	// enable it from a single command.
	ExecuteEnabled bool `json:"execute_enabled"`
	// ExecuteAllowlist is the set of commands cleared to run for real, matched
	// by exact command or whole-word prefix ("go build" clears "go build ./..."
	// but not "google-chrome"). Empty means nothing runs, even with the master
	// switch on. The denylist always overrides this.
	ExecuteAllowlist []string `json:"execute_allowlist"`
	// ExecuteTimeoutSec bounds how long a single command may run.
	ExecuteTimeoutSec float64 `json:"execute_timeout_sec"`
	// ExecuteParallel caps how many committed actions may run at once. Actions
	// whose resource claims collide serialize regardless of the cap; this
	// bounds only the genuinely independent ones.
	ExecuteParallel int `json:"execute_parallel"`

	// Self-build: the gates on GOLEM running and rewriting its own code, each a
	// deliberate opt-in on top of the last.

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

	// Network: how a transducer outside the process reaches the brain.

	// ListenTCP, when set, makes the daemon serve the same wire protocol on a
	// TCP address in addition to the unix socket, so transducers outside the
	// process (a microphone bridge, a captioner, a speaker) reach it across a
	// container boundary. Empty leaves only the socket. Bind it to a container
	// interface ("0.0.0.0:7777") and publish it to host loopback only
	// ("127.0.0.1:7777:7777"), since a reachable brain can be told and asked to
	// act.
	ListenTCP string `json:"listen_tcp"`
}

// ChannelsConfig selects the daemon's built-in sensors. Each samples the host
// the daemon runs on; a containerized brain leaves these off and is fed the
// host's senses by an adapter through golem observe instead.
type ChannelsConfig struct {
	// Synthetic runs the demo's burst-and-quiet log generator, useful for
	// soak tests against a live daemon.
	Synthetic bool `json:"synthetic"`
	// Vitals samples cpu load, memory pressure, and disk space into bands and
	// reports band changes (sysctl on macOS, /proc on Linux).
	Vitals bool `json:"vitals"`
	// Syslog follows the system log at error level: the unified log on macOS,
	// the systemd journal on Linux.
	Syslog bool `json:"syslog"`
	// WatchDirs lists directories (not recursive) for file events.
	WatchDirs []string `json:"watch_dirs"`
	// TailFiles lists files whose appended lines arrive as observations on the
	// reading channel, distinct from the file-event sense.
	TailFiles []string `json:"tail_files"`
	// WatchURLs lists pages fetched periodically; their stripped text arrives
	// on the web channel, which is untrusted by default.
	WatchURLs []string `json:"watch_urls"`
	// Screen reports the frontmost application as a symbolic stream on the
	// screen channel (System Events on macOS, the X11 active window on Linux);
	// a headless host or a container has none.
	Screen bool `json:"screen"`
	// Voice emits each spoken reply as a say event for an external speaker to
	// render; the brain runs no text-to-speech of its own.
	Voice bool `json:"voice"`
	// VoiceName names the voice an external speaker should prefer; empty leaves
	// the choice to the adapter. golem voice adjusts both voice fields at
	// runtime.
	VoiceName string `json:"voice_name"`
}

func Default() Config {
	return Config{
		// Attention.
		SurpriseThreshold: defaultSurpriseThreshold,
		ZGate:             defaultZGate,
		CalibrationWarmup: defaultCalibWarmup,
		PredictorWindow:   defaultPredictorWindow,
		PredictorWindows:  map[string]float64{"syslog": 8192},
		LateralInhibition: defaultLateralInhibition,
		// Memory formation.
		EncodeBurst:     6,
		EncodeRefillSec: 2,
		EncodeBursts:    map[string]float64{"web": 200},
		// Sleep and consolidation.
		SleepPressureGate:  defaultSleepPressureGate,
		InputSilenceSec:    defaultInputSilenceSec,
		CalmBeforeSleepSec: defaultCalmSec,
		SleepIntervalSec:   defaultSleepIntervalSec,
		NightlyDecay:       defaultNightlyDecay,
		PruneFloor:         defaultPruneFloor,
		EpisodeGapSec:      defaultEpisodeGapSec,
		// Senses and channels.
		Channels: ChannelsConfig{
			Vitals:    true,
			Syslog:    true,
			WatchDirs: []string{},
			TailFiles: []string{},
			WatchURLs: []string{},
			VoiceName: "",
		},
		WebPollSec:        defaultWebPollSec,
		UntrustedChannels: []string{"web"},
		// Action and execution.
		GraduationApprovals: defaultGraduation,
		ConflictMargin:      defaultConflictMargin,
		AutoCueCooldownSec:  defaultAutoCueCooldown,
		ExecuteEnabled:      false,
		ExecuteAllowlist:    []string{},
		ExecuteTimeoutSec:   defaultExecuteTimeoutSec,
		ExecuteParallel:     defaultExecuteParallel,
		// A category is read off what its word was shown, not off the company the word
		// happened to keep. Grown from empty over the same forty-five scenes on one engine,
		// this against the company reading and nothing else between the two, the memberships
		// a firm answer asserts fall from 26147 to 1214 while the true ones hold, and their
		// precision goes from a twentieth to nine tenths.
		//
		// It is the only one of the pointing settings that stands on by default, and the
		// measurement is what decides that rather than caution. Held off one at a time over
		// every checkpoint of that grow, this one costs 0.3116 of the closure precision and
		// is worse on all forty-seven; the other four move it by 0.0006 at most, and two of
		// them not at all. Alone it holds 0.9732 against 0.9738 for all five together. It is
		// also the only one safe to switch on under a brain grown without it: the others read
		// a record of pointings that was fading all the while they were off, and take a third
		// of that brain's names down with them over five nights, where this one leaves them
		// where the company reading does.
		OstensiveCoreEnabled: true,
		// A bare word is settled to the concept it is a word FOR. Grown from empty, the one
		// arm that differs from its neighbour by this alone comes out with the same recall to
		// four places and the same count of nameless concepts, and answers "is an oak a tree"
		// yes where the other says no: every arm carrying it answers that question right and
		// every arm without it answers it wrong. It costs a brain grown without it nothing,
		// which is why it stands here beside the core rather than in the teaching configs.
		ResolveToOwnConceptEnabled: true,
		// Self-build.
		RuntimeEnabled: false,
		SourcePath:     "",
		MainPackage:    "./cmd/golem",
		// Network: only the unix socket until an address is set.
		ListenTCP: "",
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

// EpisodeGap is the sub-conscious de-duplication window as a duration. A
// non-positive setting falls back to the default rather than collapsing the
// window to zero, so an opted-in channel is never left without de-duplication.
func (c Config) EpisodeGap() time.Duration {
	if c.EpisodeGapSec <= 0 {
		return defaultEpisodeGapSec * time.Second
	}
	return time.Duration(c.EpisodeGapSec * float64(time.Second))
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
		// The environment overlays even a first-run default, so a container
		// handed its key and port by env is configured on its very first boot.
		applyEnv(&cfg)
		return cfg, nil
	}
	if err != nil {
		return cfg, err
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		return cfg, err
	}
	applyEnv(&cfg)
	return cfg, nil
}

// loadRaw reads the config file without the environment overlay, so Save can
// tell a file setting from a runtime env override.
func loadRaw(home string) (Config, error) {
	cfg := Default()
	data, err := os.ReadFile(filepath.Join(home, "config.json"))
	if err != nil {
		return cfg, err
	}
	err = json.Unmarshal(data, &cfg)
	return cfg, err
}

// applyEnv overlays the environment onto the loaded config for the fields it
// is plausible to set per deployment or to keep out of a copied file: the
// architect's public key and the network bind address. The environment wins
// over the file, so a container can be handed its key and port without editing
// the JSON.
func applyEnv(cfg *Config) {
	if v := strings.TrimSpace(os.Getenv("GOLEM_ARCHITECT_PUBKEY")); v != "" {
		cfg.ArchitectPubKey = v
	}
	if v := strings.TrimSpace(os.Getenv("GOLEM_LISTEN_TCP")); v != "" {
		cfg.ListenTCP = v
	}
}

// envSet reports whether an environment variable carries a real value, the same
// test applyEnv uses to decide an override.
func envSet(key string) bool {
	return strings.TrimSpace(os.Getenv(key)) != ""
}

// LoadDotenv reads a .env file into the process environment before the config
// is loaded, so secrets and per-machine settings live in a file that is never
// committed rather than in the config JSON. It looks in the home directory
// first, then the working directory, and never overrides a variable already
// set in the real environment. A missing file is not an error; a malformed
// line is skipped.
func LoadDotenv(home string) {
	for _, path := range []string{filepath.Join(home, ".env"), ".env"} {
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		for _, line := range strings.Split(string(data), "\n") {
			line = strings.TrimSpace(line)
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			line = strings.TrimPrefix(line, "export ")
			key, val, ok := strings.Cut(line, "=")
			if !ok {
				continue
			}
			key = strings.TrimSpace(key)
			val = strings.Trim(strings.TrimSpace(val), `"'`)
			if key == "" {
				continue
			}
			// An unset or empty variable takes the file's value; a real value
			// already in the environment wins.
			if v, set := os.LookupEnv(key); !set || strings.TrimSpace(v) == "" {
				os.Setenv(key, val)
			}
		}
	}
}

// Save writes the configuration back to <home>/config.json atomically,
// so a kill mid-write cannot leave the file torn. The daemon calls this
// when an operator command (golem voice) changes a setting that should
// survive a restart. A field the environment overrode is a runtime overlay,
// not a file setting: Save keeps the file's own value for it, so a voice or
// runtime toggle never bakes an env-only key or bind address into the file.
func Save(home string, cfg Config) error {
	if envSet("GOLEM_ARCHITECT_PUBKEY") || envSet("GOLEM_LISTEN_TCP") {
		if disk, err := loadRaw(home); err == nil {
			if envSet("GOLEM_ARCHITECT_PUBKEY") {
				cfg.ArchitectPubKey = disk.ArchitectPubKey
			}
			if envSet("GOLEM_LISTEN_TCP") {
				cfg.ListenTCP = disk.ListenTCP
			}
		}
	}
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

// Subconscious reports whether a channel learns below consciousness: a habituated
// observation on it encodes faintly rather than dropping. It requires the feature
// enabled and the channel allow-listed, and never applies to an untrusted channel,
// so open-world perception can never accrue below the attention gate.
func (c Config) Subconscious(channel string) bool {
	if !c.SubliminalEnabled || c.Untrusted(channel) {
		return false
	}
	return c.Ambient(channel)
}

// Ambient reports whether a channel is declared background: an allow-listed
// sub-conscious channel is a monitoring stream, not something attention is
// directed at, so its observations stay context and never become episodic
// figures. Unlike Subconscious this does not require SubliminalEnabled, because
// the harm it guards (a fluctuating band that becomes conscious on a debut or a
// state change entangling into attended concepts' company) comes from conscious
// ambient episodes, not sub-conscious ones. A missing list makes it always false,
// so a channels-off brain is unaffected.
func (c Config) Ambient(channel string) bool {
	if c.Untrusted(channel) {
		return false
	}
	for _, ch := range c.SubconsciousChannels {
		if ch == channel {
			return true
		}
	}
	return false
}
