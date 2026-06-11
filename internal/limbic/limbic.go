// Package limbic is GOLEM's neurochemistry: the four-signal state
// every memory snapshots and the homeostasis that moves it.
package limbic

// How strongly one surprising bit moves the chemistry. Shared by the demo
// loop and the daemon so the two brains feel events the same way.
const (
	NASpikePerBit           = 0.12
	SleepSuppressOnSurprise = 0.15
)

// State is a snapshot of the chemistry at a single moment.
// It is copied into every Fragment so a memory carries not just *what*
// happened but *how the system felt* when it happened.
type State struct {
	Dopamine      float64 // curiosity / reward weighting
	Noradrenaline float64 // alertness; spikes on surprise
	Serotonin     float64 // stability / sleep pressure
	Oxytocin      float64 // trust in the system's own judgement; corrections lower it
}

// System is GOLEM's limbic machinery: noradrenaline spikes in
// proportion to surprise and decays back toward baseline, which makes
// memories distinguishable by how "stressed" the system was when they
// formed; serotonin acts as sleep pressure, climbing through quiet
// stretches, dropping when something alarming happens, and gating the
// consolidation cycle; and oxytocin is trust, the system's confidence in
// its own judgement, which corrections lower and the graduation bar reads.
type System struct {
	state State
}

// OxytocinBaseline is resting trust. At baseline the configured
// graduation requirement applies as written; below it the bar rises.
const OxytocinBaseline = 0.5

func New() *System {
	return &System{state: State{
		Dopamine:      0.2,
		Noradrenaline: 0.1,
		Serotonin:     0.5,
		Oxytocin:      OxytocinBaseline,
	}}
}

// Snapshot returns the current state by value (so a Fragment keeps its own copy).
func (n *System) Snapshot() State { return n.state }

// Serotonin reports the current sleep pressure, for the sleep gate.
func (n *System) Serotonin() float64 { return n.state.Serotonin }

// Decay is homeostasis: noradrenaline and dopamine drift a little back
// toward baseline every tick. Serotonin is deliberately excluded; it tracks
// sleep pressure and moves only through BuildSleepPressure and SuppressSleep.
// Oxytocin drifts home fifty times slower than alertness does, because a
// correction should sting for minutes of ticks while a surprise fades in
// seconds.
func (n *System) Decay() {
	n.state.Noradrenaline += (0.1 - n.state.Noradrenaline) * 0.10
	n.state.Dopamine += (0.2 - n.state.Dopamine) * 0.10
	n.state.Oxytocin += (OxytocinBaseline - n.state.Oxytocin) * 0.002
}

// SpikeNoradrenaline raises alertness, clamped at 1.0.
func (n *System) SpikeNoradrenaline(amount float64) {
	n.state.Noradrenaline += amount
	if n.state.Noradrenaline > 1 {
		n.state.Noradrenaline = 1
	}
}

// SpikeDopamine is reward: an association confirmed by repetition, or a
// recall that served its keeper. Dopamine is the link learning rate, so a
// memory that keeps proving useful makes the system bind new moments more
// eagerly. Clamped at 1.0; Decay drifts it back toward baseline.
func (n *System) SpikeDopamine(amount float64) {
	n.state.Dopamine += amount
	if n.state.Dopamine > 1 {
		n.state.Dopamine = 1
	}
}

// RaiseTrust is a judgement confirmed: a clean approval or a successful
// autonomous run. Capped at 1.0; trust above baseline never lowers the
// graduation bar below its configured floor.
func (n *System) RaiseTrust(amount float64) {
	n.state.Oxytocin += amount
	if n.state.Oxytocin > 1 {
		n.state.Oxytocin = 1
	}
}

// LowerTrust is a correction: a denial or a failed run. Trust falls fast
// and recovers slowly, and while it is low the graduation bar rises for
// every pair, learned or future. Floored at 0.05 so the bar stays finite.
func (n *System) LowerTrust(amount float64) {
	n.state.Oxytocin -= amount
	if n.state.Oxytocin < 0.05 {
		n.state.Oxytocin = 0.05
	}
}

// BuildSleepPressure climbs serotonin toward 1.0. The main loop calls this
// on ticks when nothing surprising has happened recently, so sustained calm
// is what carries GOLEM toward sleep.
func (n *System) BuildSleepPressure() {
	n.state.Serotonin += (1.0 - n.state.Serotonin) * 0.12
}

// SuppressSleep pushes sleep pressure down, floored at 0.05. Surprise is
// alerting: an alarmed system should not drift into consolidation.
func (n *System) SuppressSleep(amount float64) {
	n.state.Serotonin -= amount
	if n.state.Serotonin < 0.05 {
		n.state.Serotonin = 0.05
	}
}

// WakeRested resets sleep pressure to baseline after a consolidation
// night, so the next night has to be earned by another calm stretch.
func (n *System) WakeRested() {
	n.state.Serotonin = 0.5
}
