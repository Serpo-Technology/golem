package senses

import (
	"os/exec"
	"strings"
	"sync/atomic"
	"time"
)

// ScreenSensor is a first accessibility sense: the frontmost
// application, polled and reported only on change, so attention shifts
// arrive as a symbolic stream ("frontmost app Safari") without a pixel
// in sight. It reads through osascript and System Events, which may ask
// the operator for an automation grant on first use; until granted the
// sense stays quietly dark and keeps trying at a slower pace.
const (
	screenChannel  = "screen"
	screenInterval = 5 * time.Second
	screenBackoff  = 60 * time.Second
)

func ScreenSensor(out chan Observation, stop <-chan struct{}, dropped *atomic.Int64) {
	last := ""
	wait := screenInterval
	timer := time.NewTimer(wait)
	defer timer.Stop()
	for {
		select {
		case <-stop:
			return
		case <-timer.C:
			name, err := frontmostApp()
			if err != nil {
				wait = screenBackoff
			} else {
				wait = screenInterval
				if name != "" && name != last {
					last = name
					emit(out, dropped, Observation{Channel: screenChannel,
						Content: "frontmost app " + name, At: time.Now()})
				}
			}
			timer.Reset(wait)
		}
	}
}

func frontmostApp() (string, error) {
	raw, err := exec.Command("/usr/bin/osascript", "-e",
		`tell application "System Events" to get name of first application process whose frontmost is true`).Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(raw)), nil
}
