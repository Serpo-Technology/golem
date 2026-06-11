// Package memory is the hippocampus and the archive: moments, words,
// and concepts as nodes of one typed graph, persisted in sqlite, with
// a RAM working set in front and consolidation rewriting it at sleep.
package memory

import (
	"time"

	"golem/internal/limbic"
)

// A node's kind decides which life it lives in the one shared graph: an
// episodic node is a moment and fades unless practiced, a word node is a
// surface form in some language, and a concept node is the amodal hub
// that words name and episodes instantiate.
const (
	KindEpisodic = "episodic"
	KindWord     = "word"
	KindConcept  = "concept"
)

// An edge's relation types the association: temporal is co-occurrence
// (bidirectional), successor is order (directed, earlier to later), names
// points a word at its concept, and grounds points a concept at the
// episodes that instantiate it.
const (
	RelTemporal  = "temporal"
	RelSuccessor = "successor"
	RelNames     = "names"
	RelGrounds   = "grounds"
)

// Fragment is the atomic unit of GOLEM's memory: one "moment."
//
// This struct is the most important thing in the whole project. It is the
// REPRESENTATION CONTRACT — the membrane every specialized module reads from
// and writes to. The predictor, the hippocampus, the (future) basal ganglia
// all agree on this shape. Get it right and the parts compose; get it wrong
// and you rewrite everything. Expect to fight with this file.
type Fragment struct {
	ID       uint64       // assigned by the hippocampus on encode
	Channel  string       // the "labeled line" it arrived on (which sense)
	Content  string       // the raw observation — a pointer to the experience
	Template string       // normalized shape of Content; what prediction runs on
	Surprise float64      // information-theoretic surprise (bits) that birthed it
	Limbic   limbic.State // neurochemical snapshot at the instant of encoding
	Time     time.Time
	Weight   float64  // synaptic weight; decays during sleep, drives pruning
	Count    int      // occurrences this fragment stands for; above 1 it is a prototype
	Kind     string   // episodic, word, or concept; empty means episodic
	Lang     string   // language tag on word nodes ("en", "tr"); empty elsewhere
	Category string   // lemma category on concept nodes ("verb", "noun")
	Links    []uint64 // synaptic links to other fragments (the association hook)
}
