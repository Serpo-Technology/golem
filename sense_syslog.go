package main

import (
	"bufio"
	"os/exec"
	"strings"
	"sync/atomic"
	"time"
)

// SyslogSensor follows the macOS unified log at error level and feeds each
// line in as an observation. This is the channel that leans hardest on the
// normalizer: real log lines are saturated with PIDs, addresses, and
// timestamps. The subprocess is restarted with backoff if it dies, and
// content is capped so a pathological line cannot bloat memory.
func SyslogSensor(out chan Observation, stop <-chan struct{}, dropped *atomic.Int64) {
	for {
		runLogStream(out, stop, dropped)
		select {
		case <-stop:
			return
		case <-time.After(5 * time.Second):
		}
	}
}

func runLogStream(out chan Observation, stop <-chan struct{}, dropped *atomic.Int64) {
	// The predicate keeps the stream to genuine errors and faults; the
	// --level flag does not reliably narrow a stream this way.
	cmd := exec.Command("/usr/bin/log", "stream", "--style", "compact",
		"--predicate", "messageType == error OR messageType == fault")
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return
	}
	if err := cmd.Start(); err != nil {
		return
	}
	procDone := make(chan struct{})
	go func() {
		select {
		case <-stop:
			cmd.Process.Kill()
		case <-procDone:
		}
	}()

	scanner := bufio.NewScanner(stdout)
	scanner.Buffer(make([]byte, 64*1024), 64*1024)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "Filtering the log data") ||
			strings.HasPrefix(line, "Timestamp") {
			continue
		}
		if len(line) > 240 {
			line = line[:240]
		}
		emit(out, dropped, Observation{Channel: "syslog", Content: line, At: time.Now()})
	}
	// A scan error (an overlong line past the buffer, a broken pipe)
	// ends the sense; report it as an observation rather than going
	// silently dark, so the failure itself can reach consciousness.
	if err := scanner.Err(); err != nil {
		emit(out, dropped, Observation{Channel: "syslog",
			Content: "syslog sense failed: " + err.Error(), At: time.Now()})
	}
	cmd.Wait()
	close(procDone)
}
