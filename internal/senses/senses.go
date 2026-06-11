// Package senses holds the sensory channels: observations as the one
// packet every sense emits, the built-in sensors, and the contract the
// shell hook speaks.
package senses

import (
	"fmt"
	"math/rand"
	"sync/atomic"
	"time"
)

// ActionPrefix marks a shell-channel line as a command the operator
// ran: the contract between the zsh hook and everything that reads the
// shell sense.
const ActionPrefix = "run: "

// Observation is one packet arriving on a labeled sensory line.
type Observation struct {
	Channel string
	Content string
	At      time.Time
}

// emit delivers an observation without ever blocking a sensor. When the
// workspace queue is full, the oldest queued observation is dropped and
// counted: fresh reality beats stale backlog, and the drop counter is
// itself a stress signal the system can read later.
func emit(out chan Observation, dropped *atomic.Int64, o Observation) {
	select {
	case out <- o:
		return
	default:
	}
	select {
	case <-out:
		dropped.Add(1)
	default:
	}
	select {
	case out <- o:
	default:
		dropped.Add(1)
	}
}

// LogSensor simulates GOLEM's "eye" on one channel: a syslog whose lines
// carry changing numbers the way real logs do. Most lines are routine pings
// whose seq and latency differ every time; some are alarming. Without the
// normalizer every one of these lines would be unique and therefore
// "surprising"; with it, only a new SHAPE wakes the system. It runs as its
// own goroutine and pushes onto a channel. Add more sensors exactly like
// this one.
//
// The simulated log emits a burst and then goes quiet, because quiet is
// what lets sleep pressure build in the main loop.
func LogSensor(channel string, out chan Observation, stop <-chan struct{}, dropped *atomic.Int64) {
	burst := time.NewTimer(4500 * time.Millisecond)
	defer burst.Stop()
	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()
	seq := 4731
	for {
		select {
		case <-stop:
			return
		case <-burst.C:
			return // the log goes quiet
		case <-ticker.C:
			seq++
			content := fmt.Sprintf("Ping seq=%d latency=%dms", seq, 9+rand.Intn(8))
			if rand.Float64() < 0.15 {
				switch rand.Intn(3) {
				case 0:
					content = fmt.Sprintf("Connection refused peer=10.0.0.%d:%d", 2+rand.Intn(250), 30000+rand.Intn(30000))
				case 1:
					content = fmt.Sprintf("Disk full on /dev/disk1s%d", 1+rand.Intn(9))
				case 2:
					content = fmt.Sprintf("Segfault at 0x%x in worker %d", 0x7f0000000000+rand.Int63n(1<<32), rand.Intn(8))
				}
			}
			emit(out, dropped, Observation{Channel: channel, Content: content, At: time.Now()})
		}
	}
}
