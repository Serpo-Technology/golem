package main

import (
	"math/rand"
	"time"
)

// Observation is one packet arriving on a labeled sensory line.
type Observation struct {
	Channel string
	Content string
	At      time.Time
}

// LogSensor simulates GOLEM's "eye" on one channel: a syslog that emits "Ping"
// most of the time and, ~10% of the time, something alarming. It runs as its
// own goroutine and pushes onto a channel — this is the concurrency model the
// README calls for. Add more sensors exactly like this one.
func LogSensor(out chan<- Observation, stop <-chan struct{}) {
	anomalies := []string{"Connection Refused", "Disk Full", "Segfault"}
	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-stop:
			return
		case <-ticker.C:
			content := "Ping"
			if rand.Float64() < 0.10 {
				content = anomalies[rand.Intn(len(anomalies))]
			}
			out <- Observation{Channel: "syslog", Content: content, At: time.Now()}
		}
	}
}
