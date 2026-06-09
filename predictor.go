package main

import "math"

// Predictor is the PFC stand-in. It keeps a frequency model of what it has
// seen and scores each new observation by genuine Shannon surprise:
//
//	surprise = -log2( P(observation) )
//
// Expected things (high P) carry almost no bits; rare things carry many.
// This is crude but principled and genuinely cheap — exactly the profile
// GOLEM wants. It is, however, the WRONG brain long-term (see note below).
type Predictor struct {
	counts map[string]int
	total  int
}

func NewPredictor() *Predictor {
	return &Predictor{counts: make(map[string]int)}
}

// Surprise returns the information content of an observation BEFORE learning it.
//
// Note the cold-start semantics: with no prior, there is no expectation, and
// you cannot be surprised by something you had no expectation about. So the
// first observation is, by definition, unremarkable.
func (p *Predictor) Surprise(obs string) float64 {
	if p.total == 0 {
		return 0.5 // no prior yet
	}
	c := p.counts[obs]
	// Laplace smoothing: unseen tokens get high-but-finite surprise.
	prob := float64(c+1) / float64(p.total+1)
	return -math.Log2(prob)
}

// Learn folds reality into the model — this is the "coding" in predictive
// coding. Because the model moves toward what it sees, the SAME surprise will
// not fire twice: repeat an anomaly and it habituates. That habituation is
// the core thesis in miniature; watch for it when you run the program.
func (p *Predictor) Learn(obs string) {
	p.counts[obs]++
	p.total++
}

// ----------------------------------------------------------------------------
// WHERE THE HARD PROBLEM LIVES:
// This produces ONE comparable number per channel. The instant you add a
// second sensory channel, you must make these numbers comparable ACROSS
// channels with different alphabets and rates. That cross-channel saliency
// problem — not the threshold, not the gating — is the real work. Replace this
// frequency model with something better only when it starts lying to you.
// ----------------------------------------------------------------------------
