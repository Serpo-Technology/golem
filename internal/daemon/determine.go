package daemon

import (
	"time"

	"golem/internal/events"
)

// The top of the loop is one continuous determination: what to do now. Each
// quiet tick ranks the autonomous candidates, the active goal, consolidation
// pressure, and idle, by chemistry and carries out the strongest, with idle the
// floor so the absence of a task is a determination to rest rather than a
// stall. A user request does not wait in this ranking; it preempts as its own
// arrival on the loop, the world above the inner life. This is the basal
// ganglia's continuous selection with a resting default, the same
// snapshot-candidate-commit shape the thought round and the execution layer
// already use: the innate part is the arbitration, the learned part which goals
// matter and what they are worth.
const (
	idleRank        = 0.1 // the floor: there is always an answer, even if it is rest
	consolidateRank = 0.9 // sleep under pressure outranks pursuit and idle
	goalInertia     = 0.2 // a goal already in pursuit resists a passing distraction

	// goalPursuitCooldown paces the active mulling of a goal's topic, so a
	// held intention is worked at intervals rather than every tick.
	goalPursuitCooldown = 4 * time.Second
)

// determination is the loop's answer to "what to do now": the chosen mode, the
// urge that won it, and the reason for the trail.
type determination struct {
	mode string
	rank float64
	why  string
}

// determine ranks the autonomous candidates and returns the strongest, idle the
// floor. Consolidation enters under sleep pressure; a held goal enters at its
// urge, lifted by inertia when it is already the pursuit so a passing
// distraction does not unseat it. There is always an answer, so emptiness is a
// determination to rest, not a special case.
func (d *Daemon) determine(now time.Time) determination {
	best := determination{mode: "idle", rank: idleRank, why: "nothing presses; rest and wander"}
	if d.readyToSleep() && consolidateRank > best.rank {
		best = determination{mode: "consolidate", rank: consolidateRank, why: "consolidation pressure"}
	}
	if d.goal != nil {
		rank := d.goal.urge(now)
		if d.lastMode == "pursue" {
			rank += goalInertia
		}
		if rank > best.rank {
			best = determination{mode: "pursue", rank: rank, why: "goal: wants to learn " + d.goal.topic}
		}
	}
	return best
}

// carryOut commits the loop to a determination, the one place a mode is enacted:
// consolidation sleeps; pursuit wanders the warm context toward the goal or, with
// nothing warm, mulls the topic on a cooldown to keep it alive for naming and
// consolidation to ground; idle wanders when calm and rests otherwise. The mode
// is recorded when it changes, so the transitions read off the trail.
func (d *Daemon) carryOut(det determination, now time.Time) {
	if det.mode != d.lastMode {
		d.event(events.Event{Ev: "determine", Outcome: det.mode, Weight: det.rank})
		d.lastMode = det.mode
	}
	switch det.mode {
	case "consolidate":
		d.sleepCycle()
	case "pursue":
		switch {
		case d.thinking == 0 && d.mindWandering():
			d.startThinkRound()
		case now.Sub(d.lastPursuit) >= goalPursuitCooldown && time.Since(d.lastObs) >= d.cfg.InputSilence():
			d.pursueGoal(now)
		}
	case "idle":
		if d.thinking == 0 && d.mindWandering() {
			d.startThinkRound()
		}
	}
}

// pursueGoal mulls the held goal's topic: a recall that gathers and warms the
// experience around it, the active pursuit between wanders, paced so it does not
// fire every tick. The recall keeps the topic warm for naming and the night's
// consolidation to ground it, the way a held question keeps drawing attention.
func (d *Daemon) pursueGoal(now time.Time) {
	if d.goal == nil {
		return
	}
	d.lastPursuit = now
	_, outcome, fruitful := d.actRecallNow(d.goal.topic)
	d.finishAct(actRecall, d.goal.topic, outcome, fruitful, "goal")
}
