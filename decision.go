package main

import (
	"fmt"
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
	edges, err := d.store.LinksFrom(ids)
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
	n := clarifyTopCount
	if n > len(ranked) {
		n = len(ranked)
	}
	cmds := make([]string, n)
	for i := 0; i < n; i++ {
		cmds[i] = ranked[i].Command
	}
	return fmt.Sprintf("%q is ambiguous between: %s. Which did you mean?",
		cue, strings.Join(cmds, " | "))
}

// approve accepts a proposed candidate: it strengthens the grounding,
// counts toward graduation unless the command is denylisted, runs the
// action under the current execution policy, and records the outcome. A
// vetoed proposal cannot be approved; the operator must clarify first, or
// the ACC veto would be advisory only.
func (d *Daemon) approve(dec Decision, at time.Time) (Habit, string, error) {
	if dec.Conflict {
		return Habit{}, "", fmt.Errorf("proposal is ambiguous; clarify and re-propose before approving")
	}
	c := dec.Proposed
	if c == nil {
		return Habit{}, "", fmt.Errorf("nothing to approve")
	}
	if err := d.store.NudgeLinks(c.Edges, approveDelta); err != nil {
		return Habit{}, "", err
	}
	d.chem.SpikeDopamine(recallReward)

	h, err := d.store.LoadHabit(dec.CueTemplate, c.Command)
	if err != nil {
		return Habit{}, "", err
	}
	h.Approvals++
	if isDenylisted(c.Command) {
		h.Status = HabitProposing // never promotable
	} else if h.Approvals >= d.cfg.GraduationApprovals {
		h.Status = HabitGraduated
	} else {
		h.Status = HabitProposing
	}
	if err := d.store.SaveHabit(h); err != nil {
		return Habit{}, "", err
	}

	outcome := d.execute(c.Command)
	if err := d.store.Audit(at.Unix(), dec.CueTemplate, c.Command, "approved: "+outcome); err != nil {
		fmt.Fprintf(d.out, "audit write failed: %v\n", err)
	}
	return h, outcome, nil
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
	h, err := d.store.LoadHabit(dec.CueTemplate, c.Command)
	if err != nil {
		return Habit{}, err
	}
	h.Denials++
	h.Status = HabitDemoted
	if err := d.store.SaveHabit(h); err != nil {
		return Habit{}, err
	}
	if err := d.store.Audit(at.Unix(), dec.CueTemplate, c.Command, "denied"); err != nil {
		fmt.Fprintf(d.out, "audit write failed: %v\n", err)
	}
	return h, nil
}

// execute carries out an approved command under the current policy.
// Execution is off by default: GOLEM shows the command it would run and
// records the intent, but does not touch the system until the operator
// turns autonomy on for specific commands. The dangerous step is opt-in.
func (d *Daemon) execute(cmd string) string {
	return "[dry-run] would run: " + cmd
}

// teach grounds a cue to a command directly, the manual-grounding path:
// both are force-encoded in one episode so the action binds to the cue
// even on a cold channel. Repetition strengthens the bond.
func (d *Daemon) teach(cue, command string, at time.Time) {
	d.recent = nil // each lesson is its own episode
	d.process(Observation{Channel: "tell", Content: cue, At: at}, true, true)
	d.process(Observation{Channel: "shell", Content: actionPrefix + command, At: at.Add(2 * time.Second)}, true, true)
}
