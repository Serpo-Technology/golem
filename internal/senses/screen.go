package senses

import (
	"errors"
	"os/exec"
	"runtime"
	"strings"
	"sync/atomic"
	"time"
)

// ScreenSensor is a first accessibility sense: the frontmost application,
// polled and reported only on change, so attention shifts arrive as a symbolic
// stream ("frontmost app Safari") without a pixel in sight. It reads the active
// window per platform, System Events on macOS and the X11 active window on
// Linux, either of which may need a grant or a running display; until then the
// sense stays quietly dark and keeps trying at a slower pace. A headless host or
// a container has no frontmost window, which is the usual Linux case, so the
// sense simply produces nothing there.
const (
	screenChannel  = "screen"
	screenInterval = 5 * time.Second
	screenBackoff  = 60 * time.Second
)

var errNoScreen = errors.New("frontmost-app sense not supported on this platform")

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
	switch runtime.GOOS {
	case "darwin":
		raw, err := exec.Command("/usr/bin/osascript", "-e",
			`tell application "System Events" to get name of first application process whose frontmost is true`).Output()
		if err != nil {
			return "", err
		}
		return strings.TrimSpace(string(raw)), nil
	case "linux":
		// Best effort on an X11 desktop; absent a display or xdotool the call
		// errors and the sense stays dark.
		raw, err := exec.Command("xdotool", "getactivewindow", "getwindowname").Output()
		if err != nil {
			return "", err
		}
		return strings.TrimSpace(string(raw)), nil
	}
	return "", errNoScreen
}
