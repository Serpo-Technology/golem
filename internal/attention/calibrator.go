package attention

import "math"

// Calibrator learns what "unusual" means for one channel. Raw surprise
// bits are not comparable across channels: a 4-bit event is routine on a
// channel with diverse templates and an alarm on a monotonous one. Each
// channel therefore tracks an exponentially weighted mean and deviation of
// its own surprise stream, and the gate asks for a z-score against that
// history on top of the absolute floor.
type Calibrator struct {
	n    int
	mean float64
	vari float64
}

// calibAlpha is the forgetting rate of the calibration statistics, so a
// channel that turns chaotic eventually makes chaos the baseline.
const calibAlpha = 0.02

func (c *Calibrator) observe(bits float64) {
	// Early samples use a faster effective rate so the estimate converges
	// before the slow exponential takes over.
	a := calibAlpha
	if quick := 1.0 / float64(c.n+1); quick > a {
		a = quick
	}
	d := bits - c.mean
	c.mean += a * d
	c.vari += a * (d*d - c.vari)
	c.n++
}

// Export returns the calibration statistics for persistence.
func (c *Calibrator) Export() (n int, mean, vari float64) {
	return c.n, c.mean, c.vari
}

// Seed restores persisted calibration statistics, so a restarted daemon
// does not relive the warmup flood on channels it already knows.
func (c *Calibrator) Seed(n int, mean, vari float64) {
	c.n, c.mean, c.vari = n, mean, vari
}

// Assess folds the observation into the channel's statistics and reports two
// things the graded gate needs kept apart: whether the surprise reaches
// consciousness (clears the z-threshold) and whether it clears the absolute noise
// floor at all. Below the floor is pure noise and reaches neither; above the floor
// but below the threshold is a habituated stimulus, which reaches no consciousness
// yet is real enough to learn from below it. During warmup only the floor applies.
func (c *Calibrator) Assess(bits, absFloor, zGate float64, warmup int) (conscious, aboveFloor bool) {
	mean, sd, n := c.mean, math.Sqrt(c.vari), c.n
	c.observe(bits)
	if bits < absFloor {
		return false, false
	}
	if n < warmup {
		return true, true
	}
	// Floor the deviation so a near-constant channel does not alarm on
	// noise-level wobble.
	if sd < 0.25 {
		sd = 0.25
	}
	return bits-mean >= zGate*sd, true
}

// Gate reports only whether a surprise reaches consciousness, the reading Assess
// subsumes; kept for callers that do not distinguish habituation from noise.
func (c *Calibrator) Gate(bits, absFloor, zGate float64, warmup int) bool {
	conscious, _ := c.Assess(bits, absFloor, zGate, warmup)
	return conscious
}
