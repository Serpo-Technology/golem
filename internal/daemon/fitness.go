package daemon

import "time"

// Fitness is GOLEM's grounded answer to "is this change an improvement," the
// gate that turns self-modification from a hazard into a direction. Any change
// is a hypothesis, this makes me better at something, and it is kept only if
// grounded outcomes agree after it lands. The measure is multi-factor on
// purpose: a single currency is gameable, approval alone breeds flattery and
// surprise-reduction alone seeks boredom, so improvement must show across goal
// progress, world prediction, and trust together, none of them allowed to
// collapse. This is the operation layer's reinforce-or-wear loop lifted to
// larger units, and it couples to the goal loop, which generates the outcomes
// it reads.
const (
	fitGoalWeight  = 0.4  // how much advancing goals counts
	fitPredWeight  = 0.3  // how much predicting the world counts
	fitTrustWeight = 0.3  // how much standing trust counts
	fitEpsilon     = 0.02 // a change must clear this to count as improvement
	fitFloorDrop   = 0.2  // a factor dropping this far fails a change even if the blend rose
)

// fitnessSnapshot is the grounded state a change is judged against: how well
// goals are being met, how well the world is predicted, and how trusted GOLEM
// stands, each in [0, 1].
type fitnessSnapshot struct {
	goalProgress float64
	prediction   float64
	trust        float64
	at           time.Time
}

// score blends the factors into one currency. The blend is what improvement is
// read against, so no single factor decides on its own.
func (s fitnessSnapshot) score() float64 {
	return fitGoalWeight*s.goalProgress + fitPredWeight*s.prediction + fitTrustWeight*s.trust
}

// fitnessVerdict is the judgement on a change: whether it improved the grounded
// blend, by how much, and which outcome moved it, so the decision is legible.
type fitnessVerdict struct {
	improved bool
	delta    float64
	reason   string
}

// fitnessNow reads the grounded outcomes off the live state: goal progress from
// the rate goals are satisfied rather than abandoned, world prediction from the
// calm of low alarm (surprise spikes noradrenaline, so its absence is
// predictability), and trust from oxytocin. The factors are coarse and sharpen
// with experience; the mechanism that reads and blends them is what is fixed.
func (d *Daemon) fitnessNow() fitnessSnapshot {
	chem := d.chem.Snapshot()
	return fitnessSnapshot{
		goalProgress: d.goalProgressRate(),
		prediction:   clamp01(1 - chem.Noradrenaline),
		trust:        clamp01(chem.Oxytocin),
		at:           time.Now(),
	}
}

// goalProgressRate is the share of resolved goals that were satisfied rather
// than abandoned, the goal loop's contribution to fitness. With nothing
// resolved yet it is neutral, so a fresh system neither rewards nor penalizes.
func (d *Daemon) goalProgressRate() float64 {
	total := d.goalsSatisfied + d.goalsAbandoned
	if total == 0 {
		return 0.5
	}
	return float64(d.goalsSatisfied) / float64(total)
}

// judgeFitness compares the grounded state before a change with the state after
// it has landed and run. A change is kept only when the blend improved past the
// epsilon and no single factor collapsed, so a degenerate optimum that buys one
// outcome by wrecking another is caught. The reason names the outcome that moved
// most, the legible trace of why a change survived.
func judgeFitness(before, after fitnessSnapshot) fitnessVerdict {
	delta := after.score() - before.score()
	switch {
	case after.goalProgress < before.goalProgress-fitFloorDrop:
		return fitnessVerdict{delta: delta, reason: "goal progress collapsed"}
	case after.prediction < before.prediction-fitFloorDrop:
		return fitnessVerdict{delta: delta, reason: "world prediction collapsed"}
	case after.trust < before.trust-fitFloorDrop:
		return fitnessVerdict{delta: delta, reason: "trust collapsed"}
	}
	if delta <= fitEpsilon {
		return fitnessVerdict{delta: delta, reason: "no grounded improvement"}
	}
	return fitnessVerdict{improved: true, delta: delta, reason: "improved " + dominantFactor(before, after)}
}

// keepOrRevert applies a verdict: a change that improved a grounded outcome
// stands, and one that did not is undone by the revert it was issued with, the
// reinforce-or-wear loop the operation weights already run, now over the larger
// unit of a change. It returns whether the change was kept.
func keepOrRevert(v fitnessVerdict, revert func()) bool {
	if v.improved {
		return true
	}
	if revert != nil {
		revert()
	}
	return false
}

// dominantFactor names the outcome that rose the most across a change, so the
// kept change records which grounded improvement earned it.
func dominantFactor(before, after fitnessSnapshot) string {
	g := after.goalProgress - before.goalProgress
	p := after.prediction - before.prediction
	t := after.trust - before.trust
	switch {
	case g >= p && g >= t:
		return "goal progress"
	case p >= g && p >= t:
		return "world prediction"
	default:
		return "trust"
	}
}
