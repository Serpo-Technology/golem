package main

// How strongly one surprising bit moves the chemistry. Shared by the demo
// loop and the daemon so the two brains feel events the same way.
const (
	naSpikePerBit           = 0.12
	sleepSuppressOnSurprise = 0.15
)

// Neurochemistry is GOLEM's limbic system: noradrenaline spikes in
// proportion to surprise and decays back toward baseline, which makes
// memories distinguishable by how "stressed" the system was when they
// formed; serotonin acts as sleep pressure, climbing through quiet
// stretches, dropping when something alarming happens, and gating the
// consolidation cycle; and oxytocin is trust, the system's confidence in
// its own judgement, which corrections lower and the graduation bar reads.
type Neurochemistry struct {
	state LimbicState
}

// oxytocinBaseline is resting trust. At baseline the configured
// graduation requirement applies as written; below it the bar rises.
const oxytocinBaseline = 0.5

func NewNeurochemistry() *Neurochemistry {
	return &Neurochemistry{state: LimbicState{
		Dopamine:      0.2,
		Noradrenaline: 0.1,
		Serotonin:     0.5,
		Oxytocin:      oxytocinBaseline,
	}}
}

// Snapshot returns the current state by value (so a Fragment keeps its own copy).
func (n *Neurochemistry) Snapshot() LimbicState { return n.state }

// Serotonin reports the current sleep pressure, for the sleep gate.
func (n *Neurochemistry) Serotonin() float64 { return n.state.Serotonin }

// Decay is homeostasis: noradrenaline and dopamine drift a little back
// toward baseline every tick. Serotonin is deliberately excluded; it tracks
// sleep pressure and moves only through BuildSleepPressure and SuppressSleep.
// Oxytocin drifts home fifty times slower than alertness does, because a
// correction should sting for minutes of ticks while a surprise fades in
// seconds.
func (n *Neurochemistry) Decay() {
	n.state.Noradrenaline += (0.1 - n.state.Noradrenaline) * 0.10
	n.state.Dopamine += (0.2 - n.state.Dopamine) * 0.10
	n.state.Oxytocin += (oxytocinBaseline - n.state.Oxytocin) * 0.002
}

// SpikeNoradrenaline raises alertness, clamped at 1.0.
func (n *Neurochemistry) SpikeNoradrenaline(amount float64) {
	n.state.Noradrenaline += amount
	if n.state.Noradrenaline > 1 {
		n.state.Noradrenaline = 1
	}
}

// SpikeDopamine is reward: an association confirmed by repetition, or a
// recall that served its keeper. Dopamine is the link learning rate, so a
// memory that keeps proving useful makes the system bind new moments more
// eagerly. Clamped at 1.0; Decay drifts it back toward baseline.
func (n *Neurochemistry) SpikeDopamine(amount float64) {
	n.state.Dopamine += amount
	if n.state.Dopamine > 1 {
		n.state.Dopamine = 1
	}
}

// RaiseTrust is a judgement confirmed: a clean approval or a successful
// autonomous run. Capped at 1.0; trust above baseline never lowers the
// graduation bar below its configured floor.
func (n *Neurochemistry) RaiseTrust(amount float64) {
	n.state.Oxytocin += amount
	if n.state.Oxytocin > 1 {
		n.state.Oxytocin = 1
	}
}

// LowerTrust is a correction: a denial or a failed run. Trust falls fast
// and recovers slowly, and while it is low the graduation bar rises for
// every pair, learned or future. Floored at 0.05 so the bar stays finite.
func (n *Neurochemistry) LowerTrust(amount float64) {
	n.state.Oxytocin -= amount
	if n.state.Oxytocin < 0.05 {
		n.state.Oxytocin = 0.05
	}
}

// BuildSleepPressure climbs serotonin toward 1.0. The main loop calls this
// on ticks when nothing surprising has happened recently, so sustained calm
// is what carries GOLEM toward sleep.
func (n *Neurochemistry) BuildSleepPressure() {
	n.state.Serotonin += (1.0 - n.state.Serotonin) * 0.12
}

// SuppressSleep pushes sleep pressure down, floored at 0.05. Surprise is
// alerting: an alarmed system should not drift into consolidation.
func (n *Neurochemistry) SuppressSleep(amount float64) {
	n.state.Serotonin -= amount
	if n.state.Serotonin < 0.05 {
		n.state.Serotonin = 0.05
	}
}

// WakeRested resets sleep pressure to baseline after a consolidation
// night, so the next night has to be earned by another calm stretch.
func (n *Neurochemistry) WakeRested() {
	n.state.Serotonin = 0.5
}
