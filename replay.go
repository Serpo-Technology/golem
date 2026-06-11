package main

import (
	"fmt"
	"sort"
	"time"
)

// Replay is the dreaming half of consolidation: the day's strongest
// moments re-fire through the linker in order, so senses that spoke
// minutes apart can bind even though the live window had long closed.
// Only cross-channel pairs are woven; same-channel adjacency was already
// captured while awake.
const (
	replayWindow = 5 * time.Minute
	replayFloor  = 0.8
	replayCap    = 32
	replayWeight = 0.3
)

// replay walks the day's high-weight fragments chronologically and links
// cross-channel pairs that fell inside the replay window. Link keeps the
// strongest weight it has seen, so replay never weakens what waking
// built. It runs before nightly decay, on the day as it was lived.
func (d *Daemon) replay(since time.Time) (moments, woven int) {
	frags, err := d.store.StrongSince(since, replayFloor, replayCap)
	if err != nil {
		fmt.Fprintf(d.out, "replay error: %v\n", err)
		return 0, 0
	}
	sort.Slice(frags, func(i, j int) bool { return frags[i].Time.Before(frags[j].Time) })
	for i, f := range frags {
		fan := 0
		for j := i - 1; j >= 0 && fan < maxLinkFanout; j-- {
			g := frags[j]
			if f.Time.Sub(g.Time) > replayWindow {
				break
			}
			if g.Channel == f.Channel {
				continue
			}
			if err := d.store.Link(f.ID, g.ID, replayWeight); err != nil {
				fmt.Fprintf(d.out, "replay link error: %v\n", err)
				continue
			}
			d.store.Link(g.ID, f.ID, replayWeight)
			woven++
			fan++
		}
	}
	return len(frags), woven
}
