package daemon

import (
	"fmt"
	"sort"
	"time"

	"golem/internal/memory"
)

// Replay is the dreaming half of consolidation: the day's strongest
// moments re-fire through the linker in order, so senses that spoke
// minutes apart can bind even though the live window had long closed.
// Only cross-channel pairs are woven into co-occurrence; same-channel
// adjacency was already captured while awake. Order is woven for every
// pair, because re-firing the day in sequence is exactly when the
// hippocampus consolidates what followed what.
const (
	replayWindow          = 5 * time.Minute
	replayFloor           = 0.8
	replayCap             = 32
	replayWeight          = 0.3
	replaySuccessorWeight = 0.25
)

// replay walks the day's high-weight fragments chronologically, links
// cross-channel pairs that fell inside the replay window, and lays a
// directed successor edge along each ordered pair with distinct shapes.
// Link keeps the strongest weight it has seen, so replay never weakens
// what waking built. It runs before nightly decay, on the day as it was
// lived.
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
			if g.Template != f.Template {
				if err := d.store.LinkRel(g.ID, f.ID, memory.RelSuccessor, replaySuccessorWeight); err != nil {
					fmt.Fprintf(d.out, "replay successor error: %v\n", err)
				}
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
