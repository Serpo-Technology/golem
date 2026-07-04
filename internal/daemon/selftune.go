package daemon

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"golem/internal/memory"
	"golem/internal/selfbuild"
)

// Self-tuning is the experiment organ noticing its own record and proposing
// to adjust the constants that govern it. The evidence is the trail the organ
// already writes: a conjecture held one wake and refuted on a later one is a
// betrayal, a rule held again is a reconfirmation, and the balance between
// them says whether the confidence those rules are held at is set right. The
// nightly review reads that record, and when it clears a threshold, forms one
// proposal: spoken, audited, and built at idle through the same pipeline an
// operator's retune takes. The deploy stays the operator's; the mind only
// stages a vetted candidate and says why it wants it.

const (
	// selftuneFile bounds what the mind may retune: the experiment organ's
	// own numeric constants, nothing else. Widening this is the operator's
	// edit, never the mind's.
	selftuneFile = "internal/daemon/experiment.go"
	// selftuneCandidate is the staged binary the autonomous review produces,
	// named apart from the operator's so neither overwrites the other's
	// vetted candidate at the shared stage path.
	selftuneCandidate = "golem-candidate-selftune"
	// selftuneCue is the trail cue the review writes and reads its own
	// proposals under.
	selftuneCue = "selftune"
	// selftuneStepDown and selftuneStepUp size one proposal: a betrayed
	// confidence drops harder than a vindicated one rises, the same
	// asymmetry trust learning keeps.
	selftuneStepDown = 0.1
	selftuneStepUp   = 0.05
	// selftuneFloor and selftuneCeil bound where the confidence may be
	// pushed: never below a hedge worth voicing, never near grounded
	// certainty, because a rule standing on runs stays a belief.
	selftuneFloor = 0.3
	selftuneCeil  = 0.75
	// selftuneReconfirmMin is how many reconfirmations, with no betrayal
	// among them, argue the confidence undersells the rules.
	selftuneReconfirmMin = 4
)

// leashedConsts names the constants no proposal may touch regardless of what
// any signal says: the trust model, the fitness machinery, the self-build
// bound, and this policy's own knobs. The eligible file above already
// excludes them by location; the list holds if that ever widens.
var leashedConsts = map[string]bool{
	"selfBuildBudget":      true,
	"architectTrust":       true,
	"defaultSourceTrust":   true,
	"trustStep":            true,
	"trustAuthorityFloor":  true,
	"trustFloorLearned":    true,
	"trustCeilingLearned":  true,
	"fitGoalWeight":        true,
	"fitPredWeight":        true,
	"fitTrustWeight":       true,
	"fitEpsilon":           true,
	"fitFloorDrop":         true,
	"selftuneStepDown":     true,
	"selftuneStepUp":       true,
	"selftuneFloor":        true,
	"selftuneCeil":         true,
	"selftuneReconfirmMin": true,
}

// retuneProposal is one wish the review has formed and the retune act has
// yet to build: which constant, from what to what, and the evidence that
// argued it.
type retuneProposal struct {
	name   string
	from   float64
	to     float64
	reason string
	at     time.Time
}

// experimentRecord folds the experiment trail forward and counts, inside the
// window, the transitions that bear on the held confidence: a pair refuted
// after it was ever held is a betrayal, a held pair held again is a
// reconfirmation. Stopped tests say nothing and count as nothing. The fold
// runs over the whole trail so a pair's standing carries into the window,
// while only transitions at or after since are counted.
func experimentRecord(rows []memory.AuditEntry, since int64) (betrayals, reconfirms int) {
	held := map[string]bool{}
	for _, r := range rows {
		switch {
		case strings.HasPrefix(r.Outcome, "held"):
			if held[r.Action] && r.At >= since {
				reconfirms++
			}
			held[r.Action] = true
		case strings.HasPrefix(r.Outcome, "refuted"):
			if held[r.Action] && r.At >= since {
				betrayals++
			}
			held[r.Action] = false
		}
	}
	return betrayals, reconfirms
}

// proposedTarget reads the new value out of a "proposed <old> -> <new>: ..."
// trail row, so the review can recognize its own past asks.
func proposedTarget(outcome string) (float64, bool) {
	rest, ok := strings.CutPrefix(outcome, "proposed ")
	if !ok {
		return 0, false
	}
	_, after, ok := strings.Cut(rest, "-> ")
	if !ok {
		return 0, false
	}
	val, _, _ := strings.Cut(after, ":")
	f, err := strconv.ParseFloat(strings.TrimSpace(val), 64)
	return f, err == nil
}

// sayNumber renders a proposal value the way it will be spliced into source.
func sayNumber(v float64) string {
	return strconv.FormatFloat(v, 'g', -1, 64)
}

// lastProposalTarget returns the newest proposal's target and whether a deploy
// verdict has already been spoken after it, so a deployed retune is judged
// once.
func lastProposalTarget(tunes []memory.AuditEntry) (target float64, at int64, spoken, ok bool) {
	for i := range tunes {
		if to, got := proposedTarget(tunes[i].Outcome); got {
			target, at, ok, spoken = to, tunes[i].At, true, false
		}
		if ok && tunes[i].At >= at && strings.HasPrefix(tunes[i].Outcome, "deployed") {
			spoken = true
		}
	}
	return target, at, spoken, ok
}

// tuningWindow is the moment fresh evidence is counted from: the last
// candidate the review actually built or saw deployed. A mere proposal does
// not move it, so a wish dropped before it built (a restart, a busy pipeline)
// lets the same evidence argue again rather than spending it on an ask that
// never ran. Rows before a built or deployed candidate already argued their
// case or ran under the binary a retune replaced.
func tuningWindow(tunes []memory.AuditEntry) int64 {
	var since int64
	for _, t := range tunes {
		if (strings.HasPrefix(t.Outcome, "built") || strings.HasPrefix(t.Outcome, "deployed")) &&
			t.At > since {
			since = t.At
		}
	}
	return since
}

// reviewExperimentTuning is the nightly look at the organ's own record. It
// speaks a deployed proposal's verdict once, windows the evidence at the last
// candidate it built or saw deployed, and otherwise weighs betrayals against
// reconfirmations to form at most one new proposal a night. A wish is only
// formed when the sandbox is open, so the mind never promises a build it
// cannot make; the build itself stays the retune act's business.
func (d *Daemon) reviewExperimentTuning() {
	if d.pendingRetune != nil {
		return
	}
	rows, err := d.store.AuditByCue("experiment")
	if err != nil || len(rows) == 0 {
		return
	}
	tunes, err := d.store.AuditByCue(selftuneCue)
	if err != nil {
		return
	}
	// A running value that matches a past ask, not yet spoken to, is a deploy
	// the operator made: the review marks it and moves the window forward. It
	// does not claim a count since the retune, because the deploy moment is
	// the operator's and is nowhere on the trail.
	if to, _, spoken, ok := lastProposalTarget(tunes); ok && !spoken &&
		math.Abs(to-experimentConf) < 1e-9 {
		d.auditSelftune("experimentConf", "deployed: now running at "+sayNumber(to))
		d.sayLine(fmt.Sprintf("my retune of experimentConf to %s is deployed.", sayNumber(to)))
		return
	}
	// Forming a wish it cannot build would be an empty promise: the review
	// speaks only what the sandbox lets it act on.
	if !d.cfg.RuntimeEnabled || d.cfg.SourcePath == "" {
		return
	}
	b, r := experimentRecord(rows, tuningWindow(tunes))
	from := experimentConf
	switch {
	case b >= 1 && r < 4*b:
		to := math.Max(selftuneFloor, round2(from-selftuneStepDown))
		if to >= from {
			return
		}
		d.holdRetuneProposal("experimentConf", from, to, fmt.Sprintf(
			"%d held rule(s) refuted on a later wake against %d reconfirmation(s)", b, r))
	case b == 0 && r >= selftuneReconfirmMin:
		to := math.Min(selftuneCeil, round2(from+selftuneStepUp))
		if to <= from {
			return
		}
		d.holdRetuneProposal("experimentConf", from, to, fmt.Sprintf(
			"%d reconfirmation(s) and no betrayal", r))
	}
}

func round2(v float64) float64 { return math.Round(v*100) / 100 }

// holdRetuneProposal takes one formed wish into pending, onto the trail, and
// onto the voice. The leash is checked here even though the eligible file
// already excludes every leashed name: a proposal that names one is refused
// on record, never held.
func (d *Daemon) holdRetuneProposal(name string, from, to float64, reason string) {
	if leashedConsts[name] {
		d.auditSelftune(name, "leashed: the constant is not the mind's to retune")
		return
	}
	d.pendingRetune = &retuneProposal{name: name, from: from, to: to,
		reason: reason, at: time.Now()}
	d.auditSelftune(name, fmt.Sprintf("proposed %s -> %s: %s",
		sayNumber(from), sayNumber(to), reason))
	d.sayLine(fmt.Sprintf(
		"I think %s should be %s: %s. I will build and test that when I am idle; the deploy stays the architect's.",
		name, sayNumber(to), reason))
}

// actRetuneNow is the retune act's body: it takes the pending proposal
// through the same read, build, and test pipeline an operator's retune
// takes, off the loop, with the evidence as the fitness reason. A busy
// pipeline leaves the wish pending for the next firing; a proposal that
// cannot even be read is put down on record.
func (d *Daemon) actRetuneNow() (string, bool) {
	p := d.pendingRetune
	if p == nil {
		return "no retune in mind", false
	}
	value := sayNumber(p.to)
	ch, refuse := d.retuneChange(selftuneFile, p.name, value, p.reason)
	if refuse.Err != "" {
		if refuse.Stage == "gate" {
			return "the sandbox is closed", false
		}
		d.pendingRetune = nil
		d.auditSelftune(p.name, refuse.Stage+": "+refuse.Err)
		return "my retune could not be read: " + refuse.Err, false
	}
	name, reason := p.name, p.reason
	busy := d.beginSelfChange(ch, selftuneCandidate, func() fitnessVerdict {
		return fitnessVerdict{improved: true,
			reason: reason + "; the deploy stays the architect's"}
	}, func(res selfbuild.Result) {
		if res.Err != "" {
			d.auditSelftune(name, "build "+res.Stage+": "+res.Err)
			d.sayLine("my retune of " + name + " died at " + res.Stage + ": " + res.Err)
			return
		}
		// The candidate exists: mark it built so the evidence that argued it
		// is spent and only fresh betrayals argue the next one.
		d.auditSelftune(name, "built "+value+": candidate at "+res.Candidate)
		d.sayLine("I built and tested my retune of " + name + " to " + value +
			". The candidate waits at " + res.Candidate + " for the architect to deploy.")
	})
	if busy != "" {
		return busy, false
	}
	d.pendingRetune = nil
	return "building my retune of " + name + " to " + value, true
}

// auditSelftune records one step of the self-tuning review on the trail.
func (d *Daemon) auditSelftune(name, outcome string) {
	if err := d.store.Audit(time.Now().Unix(), selftuneCue, name, outcome); err != nil {
		fmt.Fprintf(d.out, "audit write failed: %v\n", err)
	}
}
