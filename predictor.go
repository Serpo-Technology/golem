package main

import (
	"maps"
	"math"
)

// Predictor is the PFC stand-in for one channel. It keeps a decaying
// frequency model of the templates it has seen and scores each new one by
// genuine Shannon surprise:
//
//	surprise = -log2( P(template) )
//
// Expected shapes (high P) carry almost no bits; rare shapes carry many.
// Counts halve whenever the window fills, so the model remembers roughly
// the last `window` observations. That one mechanism bounds memory, puts
// habituation on a chosen timescale, and lets an old normal become
// surprising again after the world drifts. Crude but principled and
// genuinely cheap. It is, however, the WRONG brain long-term (see the
// note at the bottom of this file).
type Predictor struct {
	counts map[string]float64
	total  float64
	window float64
}

// NewPredictor sizes the model's memory: the habituation timescale is
// roughly `window` observations on this channel.
func NewPredictor(window float64) *Predictor {
	if window < 16 {
		window = 16
	}
	return &Predictor{counts: make(map[string]float64), window: window}
}

// Surprise returns the information content of a template BEFORE learning it.
//
// Note the cold-start semantics: with no prior, there is no expectation, and
// you cannot be surprised by something you had no expectation about. So the
// first observation is, by definition, unremarkable.
func (p *Predictor) Surprise(tpl string) float64 {
	if p.total == 0 {
		return 0.5 // no prior yet
	}
	// Laplace smoothing: unseen templates get high-but-finite surprise.
	prob := (p.counts[tpl] + 1) / (p.total + 1)
	bits := -math.Log2(prob)
	if bits <= 0 {
		return 0 // a certainty is zero bits, never negative zero
	}
	return bits
}

// Learn folds reality into the model. Because the model moves toward what it
// sees, the SAME surprise will not fire twice: repeat an anomaly and it
// habituates. Watch for the declining bit counts when you run the program.
func (p *Predictor) Learn(tpl string) {
	p.counts[tpl]++
	p.total++
	if p.total > p.window {
		p.compact()
	}
}

// Export returns a copy of the model's counts, for persistence.
func (p *Predictor) Export() map[string]float64 {
	out := make(map[string]float64, len(p.counts))
	maps.Copy(out, p.counts)
	return out
}

// Seed restores a persisted model. Total mass is recomputed from the
// counts, which the model keeps as an invariant anyway.
func (p *Predictor) Seed(counts map[string]float64) {
	p.counts = make(map[string]float64, len(counts))
	p.total = 0
	for tpl, c := range counts {
		p.counts[tpl] = c
		p.total += c
	}
}

// compact halves every count so that recent observations dominate the model,
// then evicts whatever has faded below notice. A template unseen for a few
// windows' worth of input decays to nothing and is forgotten, which is what
// keeps the vocabulary bounded no matter how long the channel runs.
func (p *Predictor) compact() {
	p.total = 0
	for tpl, c := range p.counts {
		c /= 2
		if c < 0.25 {
			delete(p.counts, tpl)
			continue
		}
		p.counts[tpl] = c
		p.total += c
	}
}

// ----------------------------------------------------------------------------
// WHERE THE HARD PROBLEM LIVES:
// This produces ONE comparable number per channel. The instant you add a
// second sensory channel, you must make these numbers comparable ACROSS
// channels with different alphabets and rates. That cross-channel saliency
// problem — not the threshold, not the gating — is the real work. Replace this
// frequency model with something better only when it starts lying to you.
// ----------------------------------------------------------------------------
