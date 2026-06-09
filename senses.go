package main

import (
	"fmt"
	"math/rand"
	"time"
)

// Observation is one packet arriving on a labeled sensory line.
type Observation struct {
	Channel string
	Content string
	At      time.Time
}

// LogSensor simulates GOLEM's "eye" on one channel: a syslog whose lines
// carry changing numbers the way real logs do. Most lines are routine pings
// whose seq and latency differ every time; some are alarming. Without the
// normalizer every one of these lines would be unique and therefore
// "surprising"; with it, only a new SHAPE wakes the system. It runs as its
// own goroutine and pushes onto a channel. Add more sensors exactly like
// this one.
func LogSensor(out chan<- Observation, stop <-chan struct{}) {
	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()
	seq := 4731
	for {
		select {
		case <-stop:
			return
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
			out <- Observation{Channel: "syslog", Content: content, At: time.Now()}
		}
	}
}
