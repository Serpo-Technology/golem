package main

// Neurochemistry is GOLEM's limbic system. In this slice it does one real
// thing: noradrenaline spikes in proportion to surprise and then decays back
// toward a baseline. That single dynamic is enough to make memories
// distinguishable by how "stressed" the system was when it formed them.
type Neurochemistry struct {
	state LimbicState
}

func NewNeurochemistry() *Neurochemistry {
	return &Neurochemistry{state: LimbicState{
		Dopamine:      0.2,
		Noradrenaline: 0.1,
		Serotonin:     0.5,
	}}
}

// Snapshot returns the current state by value (so a Fragment keeps its own copy).
func (n *Neurochemistry) Snapshot() LimbicState { return n.state }

// Decay is homeostasis: every tick, each chemical drifts a little back toward
// its baseline. This is why a noradrenaline spike fades if nothing else happens.
func (n *Neurochemistry) Decay() {
	n.state.Noradrenaline += (0.1 - n.state.Noradrenaline) * 0.10
	n.state.Dopamine += (0.2 - n.state.Dopamine) * 0.10
	n.state.Serotonin += (0.5 - n.state.Serotonin) * 0.05
}

// SpikeNoradrenaline raises alertness, clamped at 1.0.
func (n *Neurochemistry) SpikeNoradrenaline(amount float64) {
	n.state.Noradrenaline += amount
	if n.state.Noradrenaline > 1 {
		n.state.Noradrenaline = 1
	}
}
