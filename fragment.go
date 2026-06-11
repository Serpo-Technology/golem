package main

import "time"

// LimbicState is a snapshot of GOLEM's neurochemistry at a single moment.
// It is copied into every Fragment so a memory carries not just *what*
// happened but *how the system felt* when it happened.
type LimbicState struct {
	Dopamine      float64 // curiosity / reward weighting
	Noradrenaline float64 // alertness; spikes on surprise
	Serotonin     float64 // stability / sleep pressure
	Oxytocin      float64 // trust in the system's own judgement; corrections lower it
}

// Fragment is the atomic unit of GOLEM's memory: one "moment."
//
// This struct is the most important thing in the whole project. It is the
// REPRESENTATION CONTRACT — the membrane every specialized module reads from
// and writes to. The predictor, the hippocampus, the (future) basal ganglia
// all agree on this shape. Get it right and the parts compose; get it wrong
// and you rewrite everything. Expect to fight with this file.
type Fragment struct {
	ID       uint64      // assigned by the hippocampus on encode
	Channel  string      // the "labeled line" it arrived on (which sense)
	Content  string      // the raw observation — a pointer to the experience
	Template string      // normalized shape of Content; what prediction runs on
	Surprise float64     // information-theoretic surprise (bits) that birthed it
	Limbic   LimbicState // neurochemical snapshot at the instant of encoding
	Time     time.Time
	Weight   float64  // synaptic weight; decays during sleep, drives pruning
	Links    []uint64 // synaptic links to other fragments (the association hook)
}
