package main

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"time"
)

// The decision loop is GOLEM's basal ganglia: given a cue, it gathers the
// actions that cue has been grounded to, picks a winner, and either
// proposes it or, when two actions are too close to call, vetoes and asks.
// Approval and denial reshape the grounding, and a pair approved enough
// times graduates from confirm-to-execute to autonomous.

const (
	actionPrefix    = "run: "
	approveDelta    = 0.08
	denyDelta       = 0.15
	actionFloor     = 0.05
	dopamineFloor   = 0.5 // saliency = link strength scaled into [0.5, 1.5] by dopamine
	clarifyTopCount = 3

	// Trust moves on every judged outcome: corrections cost almost four
	// times what a confirmation earns, so a streak of denials raises the
	// graduation bar quickly and a few good runs do not erase it.
	trustOnConfirm    = 0.04
	trustOnCorrection = 0.15
	trustOnAutonomous = 0.02
)

// Candidate is one command the cue could mean, aggregated across every
// remembered moment that ran it.
type Candidate struct {
	Command  string
	Saliency float64
	Members  []uint64   // action fragments that share this command
	Edges    []LinkEdge // cue-to-action edges that carry the grounding
}

// Decision is the outcome of weighing a cue: a proposal, or a conflict the
// user must resolve, or nothing grounded at all.
type Decision struct {
	Cue          string
	CueTemplate  string
	Proposed     *Candidate
	Alternatives []*Candidate
	Conflict     bool
	Question     string
	Status       string // habit status of the proposed command
}

// promotionBar is how many consistent approvals graduation takes under
// current trust. At baseline oxytocin the configured requirement applies
// as written; falling trust raises the bar in proportion, for every pair
// at once, and high trust never lowers it below the configured floor.
func (d *Daemon) promotionBar() int {
	n := d.cfg.GraduationApprovals
	ox := d.chem.Snapshot().Oxytocin
	if ox >= oxytocinBaseline {
		return n
	}
	return int(math.Ceil(float64(n) * oxytocinBaseline / ox))
}

// commandOf reports the shell command a fragment represents, if it is one.
func commandOf(f *Fragment) (string, bool) {
	if f.Channel != "shell" {
		return "", false
	}
	if !strings.HasPrefix(f.Content, actionPrefix) {
		return "", false
	}
	return strings.TrimSpace(strings.TrimPrefix(f.Content, actionPrefix)), true
}

// isDenylisted reports whether a command is one GOLEM must never run
// without explicit confirmation, regardless of how often it is approved.
func isDenylisted(cmd string) bool {
	low := strings.ToLower(cmd)
	for _, p := range actionDenylist {
		if strings.Contains(low, p) {
			return true
		}
	}
	return false
}

// decide weighs the actions grounded to a cue and returns a proposal or a
// conflict. It reads memory only; nothing is executed or recorded here.
func (d *Daemon) decide(cue string) (Decision, error) {
	dec := Decision{Cue: cue, CueTemplate: Normalize(cue)}

	anchors, err := d.store.Recall(cue, 5)
	if err != nil || len(anchors) == 0 {
		return dec, err
	}
	ids := make([]uint64, 0, len(anchors))
	for _, a := range anchors {
		ids = append(ids, a.ID)
	}
	// Grounding is co-occurrence. The successor edge a cue also earns by
	// preceding its command would double-count the same evidence here.
	edges, err := d.store.LinksFromRel(ids, RelTemporal)
	if err != nil {
		return dec, err
	}
	neighborIDs := make([]uint64, 0, len(edges))
	for _, e := range edges {
		neighborIDs = append(neighborIDs, e.To)
	}
	neighbors, err := d.store.FragmentsByIDs(neighborIDs)
	if err != nil {
		return dec, err
	}
	byID := make(map[uint64]*Fragment, len(neighbors))
	for _, f := range neighbors {
		byID[f.ID] = f
	}

	// Aggregate by command: repeated groundings of the same command
	// reinforce one candidate rather than scattering into many.
	cands := map[string]*Candidate{}
	for _, e := range edges {
		f := byID[e.To]
		if f == nil {
			continue
		}
		cmd, ok := commandOf(f)
		if !ok {
			continue
		}
		c := cands[cmd]
		if c == nil {
			c = &Candidate{Command: cmd}
			cands[cmd] = c
		}
		c.Saliency += e.Weight * f.Weight
		c.Members = append(c.Members, f.ID)
		c.Edges = append(c.Edges, e)
	}
	if len(cands) == 0 {
		return dec, nil
	}

	// Dopamine sets the overall drive; it scales every candidate equally,
	// so it changes whether GOLEM acts, not which action wins.
	drive := dopamineFloor + d.chem.Snapshot().Dopamine
	ranked := make([]*Candidate, 0, len(cands))
	for _, c := range cands {
		c.Saliency *= drive
		ranked = append(ranked, c)
	}
	sort.Slice(ranked, func(i, j int) bool {
		if ranked[i].Saliency != ranked[j].Saliency {
			return ranked[i].Saliency > ranked[j].Saliency
		}
		return ranked[i].Command < ranked[j].Command
	})

	top := ranked[0]
	if top.Saliency < actionFloor {
		return dec, nil
	}
	dec.Proposed = top
	dec.Alternatives = ranked[1:]

	// The ACC veto: if the top two are within the margin, the winner is
	// not clear enough to act on, so ask instead of guessing.
	if len(ranked) >= 2 {
		margin := (top.Saliency - ranked[1].Saliency) / top.Saliency
		if margin < d.cfg.ConflictMargin {
			dec.Conflict = true
			dec.Question = conflictQuestion(cue, ranked)
			return dec, nil
		}
	}

	h, err := d.store.LoadHabit(dec.CueTemplate, top.Command)
	if err != nil {
		return dec, err
	}
	dec.Status = h.Status
	return dec, nil
}

func conflictQuestion(cue string, ranked []*Candidate) string {
	n := min(clarifyTopCount, len(ranked))
	cmds := make([]string, n)
	for i := range n {
		cmds[i] = ranked[i].Command
	}
	return fmt.Sprintf("%q is ambiguous between: %s. Which did you mean?",
		cue, strings.Join(cmds, " | "))
}

// approveStart is the bookkeeping that precedes an approved run: the
// grounding strengthens and reward fires whether or not the command turns
// out to be cleared to execute. A vetoed proposal cannot be approved; the
// operator must clarify first, or the ACC veto would be advisory only.
func (d *Daemon) approveStart(dec Decision) error {
	if dec.Conflict {
		return fmt.Errorf("proposal is ambiguous; clarify and re-propose before approving")
	}
	if dec.Proposed == nil {
		return fmt.Errorf("nothing to approve")
	}
	if err := d.store.NudgeLinks(dec.Proposed.Edges, approveDelta); err != nil {
		return err
	}
	d.chem.SpikeDopamine(recallReward)
	return nil
}

// approveFinish is the bookkeeping after an approved run came back: the
// habit judgement, trust, the audit row, and the outcome fed back as
// memory. It must run on the brain loop.
func (d *Daemon) approveFinish(dec Decision, ran, ok bool, outcome string, at time.Time) (Habit, error) {
	c := dec.Proposed
	h, err := d.store.LoadHabit(dec.CueTemplate, c.Command)
	if err != nil {
		return Habit{}, err
	}
	switch {
	case ran && !ok:
		// A command that errors is a correction. Demote it and reset the
		// approval count, so re-graduation takes fresh consistent
		// approvals rather than a single success after a failure.
		h.Denials++
		h.Approvals = 0
		h.Status = HabitDemoted
		d.chem.LowerTrust(trustOnCorrection)
	case isDenylisted(c.Command):
		h.Approvals++
		h.Status = HabitProposing // never promotable
		d.chem.RaiseTrust(trustOnConfirm)
	case h.Approvals+1 >= d.promotionBar():
		h.Approvals++
		h.Status = HabitGraduated
		d.chem.RaiseTrust(trustOnConfirm)
	default:
		h.Approvals++
		h.Status = HabitProposing
		d.chem.RaiseTrust(trustOnConfirm)
	}
	if err := d.store.SaveHabit(h); err != nil {
		return Habit{}, err
	}
	d.habitsChanged()
	d.event(Event{Ev: "approval", Cue: dec.CueTemplate, Action: c.Command, Outcome: outcome})
	if err := d.store.Audit(at.Unix(), dec.CueTemplate, c.Command, "approved: "+outcome); err != nil {
		fmt.Fprintf(d.out, "audit write failed: %v\n", err)
	}
	if ran {
		d.feedback(c.Command, outcome, at)
	}
	return h, nil
}

// approve accepts a proposed candidate synchronously: strengthen, run
// under the current execution policy, record. The daemon's request path
// runs the same pieces with the command moved off the loop; this
// whole-in-one form serves the demos and tests.
func (d *Daemon) approve(dec Decision, at time.Time) (Habit, string, error) {
	if err := d.approveStart(dec); err != nil {
		return Habit{}, "", err
	}
	ran, ok, outcome := d.execute(dec.Proposed.Command)
	h, err := d.approveFinish(dec, ran, ok, outcome, at)
	if err != nil {
		return Habit{}, "", err
	}
	return h, outcome, nil
}

// autorunFinish records a completed autonomous run: demotion on failure,
// trust, the audit row, and the outcome fed back as memory. It must run
// on the brain loop. The trigger names who supplied the cue in the audit
// trail: "" for the operator's propose, "observed" for automatic cueing.
func (d *Daemon) autorunFinish(dec Decision, trigger string, ran, ok bool, outcome string, at time.Time) (demoted bool) {
	c := dec.Proposed
	verb := "autonomous"
	if trigger != "" {
		verb += " (" + trigger + ")"
	}
	if ran && !ok {
		d.chem.LowerTrust(trustOnCorrection)
		if h, err := d.store.LoadHabit(dec.CueTemplate, c.Command); err == nil {
			h.Denials++
			h.Approvals = 0
			h.Status = HabitDemoted
			if err := d.store.SaveHabit(h); err != nil {
				fmt.Fprintf(d.out, "demotion not persisted: %v\n", err)
			} else {
				demoted = true
				verb += " failed, demoted"
			}
		}
	} else if ran {
		d.chem.RaiseTrust(trustOnAutonomous)
	}
	d.habitsChanged()
	d.event(Event{Ev: "autonomous", Cue: dec.CueTemplate, Action: c.Command, Outcome: verb + ": " + outcome})
	if err := d.store.Audit(at.Unix(), dec.CueTemplate, c.Command, verb+": "+outcome); err != nil {
		fmt.Fprintf(d.out, "audit write failed: %v\n", err)
	}
	if ran {
		d.feedback(c.Command, outcome, at)
	}
	return demoted
}

// deny rejects a proposed candidate: it weakens the grounding and demotes
// the pair, so a wrong proposal does not recur.
func (d *Daemon) deny(dec Decision, at time.Time) (Habit, error) {
	if dec.Conflict {
		return Habit{}, fmt.Errorf("proposal is ambiguous; clarify and re-propose before denying")
	}
	c := dec.Proposed
	if c == nil {
		return Habit{}, fmt.Errorf("nothing to deny")
	}
	if err := d.store.NudgeLinks(c.Edges, -denyDelta); err != nil {
		return Habit{}, err
	}
	d.chem.LowerTrust(trustOnCorrection)
	h, err := d.store.LoadHabit(dec.CueTemplate, c.Command)
	if err != nil {
		return Habit{}, err
	}
	h.Denials++
	h.Status = HabitDemoted
	if err := d.store.SaveHabit(h); err != nil {
		return Habit{}, err
	}
	d.habitsChanged()
	d.event(Event{Ev: "denial", Cue: dec.CueTemplate, Action: c.Command})
	if err := d.store.Audit(at.Unix(), dec.CueTemplate, c.Command, "denied"); err != nil {
		fmt.Fprintf(d.out, "audit write failed: %v\n", err)
	}
	return h, nil
}

// teach grounds a cue to a command directly, the manual-grounding path:
// both are force-encoded in one episode so the action binds to the cue
// even on a cold channel. Repetition strengthens the bond. The lesson is
// its own episode and leaves the live binding window untouched, so manual
// teaching cannot accidentally bind to whatever a sensor just saw.
func (d *Daemon) teach(cue, command string, at time.Time) {
	saved := d.recent
	d.recent = nil
	d.process(Observation{Channel: "tell", Content: cue, At: at}, true, true)
	d.process(Observation{Channel: "shell", Content: actionPrefix + command, At: at.Add(2 * time.Second)}, true, true)
	d.recent = saved
}
