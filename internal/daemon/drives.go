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

// A goal is the one intention the loop holds across steps: something it
// wants and keeps acting toward until it is satisfied or stale. The
// first goals are learning goals, adopted when GOLEM asks about a word
// that points at nothing: holding the question open is what makes the
// asking more than a reflex.
type goalState struct {
	kind  string
	topic string
	since time.Time
}

const (
	goalTTL      = 30 * time.Minute
	goalBaseUrge = 0.6
	// goalReward is the dopamine of a question finally answered, a
	// larger spike than routine recall because a held intention resolved.
	goalReward = 0.2
)

// urge decays linearly over the goal's lifetime: a question held too
// long fades rather than becoming an obsession.
func (g *goalState) urge(now time.Time) float64 {
	left := 1 - now.Sub(g.since).Seconds()/goalTTL.Seconds()
	return goalBaseUrge * clamp01(left)
}

// adoptGoal holds a new intention, one at a time: the first open
// question keeps the slot until it resolves or expires.
func (d *Daemon) adoptGoal(kind, topic string) {
	topic = strings.ToLower(strings.TrimSpace(topic))
	if d.goal != nil || topic == "" {
		return
	}
	d.goal = &goalState{kind: kind, topic: topic, since: time.Now()}
	d.event(events.Event{Ev: "goal", Cue: topic, Outcome: "adopted"})
	if err := d.store.Audit(time.Now().Unix(), "goal", topic, "adopted: wants to learn it"); err != nil {
		fmt.Fprintf(d.out, "audit write failed: %v\n", err)
	}
}

// checkGoal settles the held intention: satisfied when the topic now
// resolves to a concept (the question was answered, however it
// happened), abandoned when it has gone stale. Satisfaction is
// rewarding, the way a confirmed association is.
func (d *Daemon) checkGoal() {
	if d.goal == nil {
		return
	}
	if c := d.resolveConcept(d.goal.topic); c != nil {
		d.event(events.Event{Ev: "goal", Cue: d.goal.topic, ID: c.ID, Outcome: "satisfied"})
		if err := d.store.Audit(time.Now().Unix(), "goal", d.goal.topic,
			"satisfied: it names "+c.Content); err != nil {
			fmt.Fprintf(d.out, "audit write failed: %v\n", err)
		}
		d.chem.SpikeDopamine(goalReward)
		d.goal = nil
		return
	}
	if time.Since(d.goal.since) > goalTTL {
		d.event(events.Event{Ev: "goal", Cue: d.goal.topic, Outcome: "abandoned"})
		if err := d.store.Audit(time.Now().Unix(), "goal", d.goal.topic, "abandoned: grown stale"); err != nil {
			fmt.Fprintf(d.out, "audit write failed: %v\n", err)
		}
		d.goal = nil
	}
}

// goalDescription is the held intention's face in status.
func (d *Daemon) goalDescription() string {
	if d.goal == nil {
		return ""
	}
	return fmt.Sprintf("learn what %q means (held %s)",
		d.goal.topic, time.Since(d.goal.since).Round(time.Second))
}
