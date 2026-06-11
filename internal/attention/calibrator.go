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

// Gate decides whether a surprise reaches consciousness on this channel,
// judging it against the statistics as they stood before this observation,
// then folding the observation in. During warmup only the absolute floor
// applies.
func (c *Calibrator) Gate(bits, absFloor, zGate float64, warmup int) bool {
	mean, sd, n := c.mean, math.Sqrt(c.vari), c.n
	c.observe(bits)
	if bits < absFloor {
		return false
	}
	if n < warmup {
		return true
	}
	// Floor the deviation so a near-constant channel does not alarm on
	// noise-level wobble.
	if sd < 0.25 {
		sd = 0.25
	}
	return bits-mean >= zGate*sd
}
