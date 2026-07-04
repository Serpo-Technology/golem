package daemon

import (
	"fmt"
	"strings"
	"time"

	"golem/internal/events"
)

// Drives give the loop reasons to act beyond chemical reflex. Each is a
// named urge computed from the body state at round start: relevance
// wants the moment's context retrieved, tidiness wants clutter
// consolidated, curiosity wants the nameless named, and a held goal
// wants its topic found. A drive does not act; it biases the act gate,
// and the winning act carries the drive's reason onto the audit trail,
// so every unprompted firing reads as "what, because of which urge."
type drive struct {
	name string
	act  string  // the act this drive pushes
	urge float64 // [0, 1]
	why  string  // the reason, as the trail will read it
}

const (
	driveFloor = 0.05
	driveGain  = 1.0
	// curiosityHush keeps the curiosity drive quiet at baseline reward:
	// only elevated dopamine makes the nameless worth asking about.
	curiosityHush = 0.2
)

// computeDrives reads the urges off the current state. seedNameless
// reports whether the round's warm seed has no name yet, the condition
// curiosity cares about.
func (d *Daemon) computeDrives(now time.Time, seedNameless bool) []drive {
	chem := d.chem.Snapshot()
	var out []drive
	if u := clamp01((chem.Noradrenaline - 0.1) / 0.9); u > driveFloor {
		out = append(out, drive{name: "relevance", act: actRecall, urge: u,
			why: fmt.Sprintf("relevance: alarm %.2f seeks context", chem.Noradrenaline)})
	}
	if d.encodesSinceSleep >= actClutterMin {
		if u := clutterOf(d.encodesSinceSleep); u > driveFloor {
			out = append(out, drive{name: "tidiness", act: actConsolidate, urge: u,
				why: fmt.Sprintf("tidiness: %d fresh moments", d.encodesSinceSleep)})
		}
	}
	if seedNameless {
		if u := clamp01((chem.Dopamine - curiosityHush) / 0.8); u > driveFloor {
			out = append(out, drive{name: "curiosity", act: actSpeak, urge: u,
				why: "curiosity: the warm thing has no name"})
		}
	}
	if d.goal != nil {
		if u := d.goal.urge(now); u > driveFloor {
			out = append(out, drive{name: "goal", act: actRecall, urge: u,
				why: "goal: wants to learn " + d.goal.topic})
		}
	}
	// A held computation is an answer GOLEM owes: the urge starts real and
	// rises as the oldest gap waits, so the compute faculty outbids idle
	// association within a calm minute rather than letting the debt sit.
	if len(d.computeGaps) > 0 && d.cfg.RuntimeEnabled && d.computeRuns < computeRunsPerWake {
		age := now.Sub(d.computeGaps[0].at)
		if u := clamp01(0.5 + age.Seconds()/120); u > driveFloor {
			out = append(out, drive{name: "computation", act: actCompute, urge: u,
				why: "computation: an answer is owed for " + d.computeGaps[0].phrase})
		}
	}
	return out
}

// driveMaps folds the drives into the per-act bias the snapshot carries:
// the strongest urge per act sets the multiplier and supplies the
// reason.
func driveMaps(drives []drive) (bias map[string]float64, why map[string]string) {
	bias = make(map[string]float64, len(drives))
	why = make(map[string]string, len(drives))
	best := map[string]float64{}
	for _, dr := range drives {
		tpl := actOp{name: dr.act}.template()
		if dr.urge <= best[tpl] {
			continue
		}
		best[tpl] = dr.urge
		bias[tpl] = 1 + driveGain*dr.urge
		why[tpl] = dr.why
	}
	return bias, why
}

func clamp01(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}

// A goal is the one intention the loop holds across steps: a target state
// it works toward, a sense of distance to it, and a hold that survives
// across ticks. The first goals are learning goals, adopted when GOLEM
// asks about a word that points at nothing; the target is to understand
// the topic, the distance closes as experience about it accumulates and
// reaches zero when a concept finally answers to it. A goal is released
// when reached, when it goes stale, or when it stops paying, so a held
// intention never becomes an obsession.
type goalState struct {
	kind         string
	topic        string
	since        time.Time
	distance     float64   // current sense of distance to understood, in [0, 1]
	bestDistance float64   // closest it has come, for stall detection
	lastProgress time.Time // when the distance last shortened
}

const (
	goalTTL      = 30 * time.Minute
	goalStallTTL = 10 * time.Minute // no progress for this long: the pursuit has stopped paying
	goalBaseUrge = 0.6
	// goalReward is the dopamine of a question finally answered, a
	// larger spike than routine recall because a held intention resolved.
	goalReward = 0.2
	// goalProgressEps is the shortening that counts as real progress, so a
	// jitter in the distance measure does not read as movement.
	goalProgressEps = 0.05
)

// urge decays linearly over the goal's lifetime and scales with how far
// the target still is, so a question held too long fades rather than
// becoming an obsession and a nearly-met goal pursues gently.
func (g *goalState) urge(now time.Time) float64 {
	left := 1 - now.Sub(g.since).Seconds()/goalTTL.Seconds()
	return goalBaseUrge * clamp01(left) * g.distance
}

// goalDistance measures how far a topic is from understood, in [0, 1]:
// zero once a concept answers to it, and shortening from one as lived
// experience about it is gathered even before it is named. It is the
// sense of distance the pursuit closes.
func (d *Daemon) goalDistance(topic string) float64 {
	if topic == "" {
		return 1
	}
	if d.resolveConcept(topic) != nil {
		return 0
	}
	found, err := d.store.Recall(topic, 5)
	if err != nil {
		return 1
	}
	n := len(found)
	if n > 3 {
		n = 3
	}
	return 1 - 0.2*float64(n)
}

// adoptGoal holds a new intention, one at a time: the first open
// question keeps the slot until it resolves, goes stale, or stops paying.
func (d *Daemon) adoptGoal(kind, topic string) {
	topic = strings.ToLower(strings.TrimSpace(topic))
	if d.goal != nil || topic == "" {
		return
	}
	now := time.Now()
	dist := d.goalDistance(topic)
	d.goal = &goalState{kind: kind, topic: topic, since: now, distance: dist, bestDistance: dist, lastProgress: now}
	d.event(events.Event{Ev: "goal", Cue: topic, Outcome: "adopted"})
	if err := d.store.Audit(now.Unix(), "goal", topic, "adopted: wants to learn it"); err != nil {
		fmt.Fprintf(d.out, "audit write failed: %v\n", err)
	}
}

// checkGoal settles the held intention each tick: satisfied when the topic
// now resolves to a concept (the question is answered, however that
// happened), with the closing distance logged as progress on the trail,
// and released when it goes stale or when it has stopped making progress.
// Satisfaction is rewarding, the way a confirmed association is.
func (d *Daemon) checkGoal() {
	if d.goal == nil {
		return
	}
	now := time.Now()
	d.goal.distance = d.goalDistance(d.goal.topic)
	if d.goal.distance <= 0 {
		names := ""
		if c := d.resolveConcept(d.goal.topic); c != nil {
			names = c.Content
		}
		d.event(events.Event{Ev: "goal", Cue: d.goal.topic, Outcome: "satisfied"})
		if err := d.store.Audit(now.Unix(), "goal", d.goal.topic, "satisfied: it names "+names); err != nil {
			fmt.Fprintf(d.out, "audit write failed: %v\n", err)
		}
		d.chem.SpikeDopamine(goalReward)
		d.goalsSatisfied++
		d.goal = nil
		return
	}
	if d.goal.distance < d.goal.bestDistance-goalProgressEps {
		d.goal.bestDistance = d.goal.distance
		d.goal.lastProgress = now
		d.event(events.Event{Ev: "goal", Cue: d.goal.topic, Outcome: "progress", Weight: d.goal.distance})
		if err := d.store.Audit(now.Unix(), "goal", d.goal.topic,
			fmt.Sprintf("progress: closing in, distance %.2f", d.goal.distance)); err != nil {
			fmt.Fprintf(d.out, "audit write failed: %v\n", err)
		}
	}
	switch {
	case now.Sub(d.goal.since) > goalTTL:
		d.abandonGoal("grown stale")
	case now.Sub(d.goal.lastProgress) > goalStallTTL:
		d.abandonGoal("stopped paying")
	}
}

// abandonGoal releases the held intention and records why, so a dropped
// goal is as legible as a pursued one.
func (d *Daemon) abandonGoal(why string) {
	d.event(events.Event{Ev: "goal", Cue: d.goal.topic, Outcome: "abandoned"})
	if err := d.store.Audit(time.Now().Unix(), "goal", d.goal.topic, "abandoned: "+why); err != nil {
		fmt.Fprintf(d.out, "audit write failed: %v\n", err)
	}
	d.goalsAbandoned++
	d.goal = nil
}

// goalDescription is the held intention's face in status: its topic and
// how far it still has to go.
func (d *Daemon) goalDescription() string {
	if d.goal == nil {
		return ""
	}
	return fmt.Sprintf("learn what %q means (distance %.2f, held %s)",
		d.goal.topic, d.goal.distance, time.Since(d.goal.since).Round(time.Second))
}
