package main

import (
	"encoding/json"
	"os"
	"path/filepath"
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
)

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
}

func DefaultConfig() Config {
	return Config{
		SurpriseThreshold:  defaultSurpriseThreshold,
		PredictorWindow:    defaultPredictorWindow,
		SleepPressureGate:  defaultSleepPressureGate,
		InputSilenceSec:    defaultInputSilenceSec,
		CalmBeforeSleepSec: defaultCalmSec,
		SleepIntervalSec:   defaultSleepIntervalSec,
		NightlyDecay:       defaultNightlyDecay,
		PruneFloor:         defaultPruneFloor,
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

// GolemHome resolves the state directory: $GOLEM_HOME if set, otherwise
// ~/.golem. Memory, config, socket, and logs all live here.
func GolemHome() (string, error) {
	if h := os.Getenv("GOLEM_HOME"); h != "" {
		return h, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".golem"), nil
}

// LoadConfig reads <home>/config.json, creating it with defaults on first
// run so there is a file to edit.
func LoadConfig(home string) (Config, error) {
	cfg := DefaultConfig()
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
