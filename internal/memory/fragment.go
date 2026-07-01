// Package memory is the hippocampus and the archive: moments, words,
// concepts, thought operations, and grammar parameters as nodes of one
// typed graph, persisted in sqlite, with a RAM working set in front and
// consolidation rewriting it at sleep.
package memory

import (
	"time"

	"golem/internal/limbic"
)

// A node's kind decides which life it lives in the one shared graph: an
// episodic node is a moment and fades unless practiced, a word node is a
// surface form in some language, a concept node is the amodal hub that
// words name and episodes instantiate, an operation node is one setting
// of the think primitive whose weight is the learned preference for that
// kind of thought, a grammar node is one entrenched parameter of the
// language faculty, a feature node is one interpretable attribute that
// concepts share, the sparse layer similarity reads, and a construction
// node is one sentence pattern the planner fills, the form of an answer
// lifted out of the realizer into memory.
const (
	KindEpisodic     = "episodic"
	KindWord         = "word"
	KindConcept      = "concept"
	KindOperation    = "operation"
	KindGrammar      = "grammar"
	KindFeature      = "feature"
	KindConstruction = "construction"
)

// An edge's relation types the association: temporal is co-occurrence
// (bidirectional), successor is order (directed, earlier to later), names
// points a word at its concept, grounds points a concept at the episodes
// that instantiate it, and feature points a concept at an attribute its
// lived experience exhibits, weighted by how much of it does. A relation
// reified as a concept grounds in the moments that instantiate it like any
// other, and those moments point at what filled each role: agent at what
// acted, patient at what was acted on, so a proposition is a moment with its
// roles named.
const (
	RelTemporal  = "temporal"
	RelSuccessor = "successor"
	RelNames     = "names"
	RelGrounds   = "grounds"
	RelFeature   = "feature"
	RelAgent     = "agent"
	RelPatient   = "patient"
	// RelOpposite links two gradable relations discovered to be converse poles of
	// one scale (bigger and smaller), so an ordering told one way answers the
	// other. It joins relation concepts, not the things they order.
	RelOpposite = "opposite"
	// RelTransitive marks a relation concept as transitive: a self-edge that
	// records a learned property of the relation, so a told ordering chains
	// (A before B and B before C entail A before C). It is set when GOLEM is
	// told the relation is transitive, not baked into any construction, so the
	// chaining is a fact about the relation rather than the grammar.
	RelTransitive = "transitive"
	// RelLocative marks a relation concept as locative: a self-edge that records
	// the learned property that the relation places its figure at its ground
	// ("on", "in", "under"). It is set when GOLEM is told the relation is a place,
	// so a "where" query reads it and the clause renders as a copula with a
	// prepositional complement rather than a verb. Like RelTransitive, the
	// character is a fact about the relation in the graph, not baked into a
	// construction.
	RelLocative = "locative"
	// RelPossessive marks a relation concept as possessive: a self-edge recording
	// that the relation holds one thing as another's ("the tree has a leaf"). It lets
	// a Turkish possession ("ağacın yaprağı var"), which marks possession by case and
	// the existential "var" rather than a verb, render and unify across tongues with
	// the English "have"/"own" the same property names.
	RelPossessive = "possessive"
	// RelPartWhole marks a relation concept as part-whole: a self-edge recording that the
	// relation holds one thing as part of another ("the leaf is part of the tree"). A part is
	// located where its whole is, so the containment walk steps through it: a leaf part of a tree
	// that is in the forest is in the forest. Like the others, the character is a fact about the
	// relation in the graph, not baked into a construction.
	RelPartWhole = "partwhole"
	// RelMarks links a closed-class word to a concept it marks, so a grammatical
	// category the engine reads is derived from the graph rather than a seeded
	// string. A speech-act verb marks the discourse move it carries (clarify,
	// recall), the first colon-class category moved into the graph.
	RelMarks = "marks"
	// RelTrust records how far GOLEM trusts a source: a self-edge on a source node
	// whose weight is the trust, so a telling from that source is weighed by it. It
	// mirrors RelTransitive and RelLocative, a learned property held in the graph
	// rather than a field, so trust is auditable and adjustable like any other weight.
	RelTrust = "trust"
	// RelSource links a told moment to the source that spoke it, so a proposition is
	// auditable back to who said it and its weight can be scaled by that source's
	// trust. A moment with no source edge was the local operator's, trusted.
	RelSource = "source"
	// RelOstends links a word to the percept it was pointed at when it was uttered
	// to name it: the freshly attended episode of an ostensive naming, weighted by
	// how fresh that percept was. It is the salience the uniform naming company
	// throws away. A word's temporal company spreads to whatever lingered warm in
	// the scene, so it cannot tell a name said over one constant percept from a
	// category said over varying members; its ostension does, since only what is
	// freshly pointed at earns the edge. The abstraction pass reads it to bind a
	// diluted cross-tongue name to its referent rather than splitting a phantom.
	RelOstends = "ostends"
)

// Fragment is the atomic unit of GOLEM's memory: one "moment."
//
// This struct is the most important thing in the whole project. It is the
// REPRESENTATION CONTRACT: the membrane every specialized module reads from
// and writes to. The predictor, the hippocampus, and the basal ganglia all
// agree on this shape. Get it right and the parts compose; get it wrong
// and you rewrite everything. Expect to fight with this file.
type Fragment struct {
	ID       uint64       // assigned by the hippocampus on encode
	Channel  string       // the "labeled line" it arrived on (which sense)
	Content  string       // the raw observation that points at the experience
	Template string       // normalized shape of Content; what prediction runs on
	Surprise float64      // information-theoretic surprise (bits) that birthed it
	Limbic   limbic.State // neurochemical snapshot at the instant of encoding
	Time     time.Time
	Weight   float64  // synaptic weight; decays during sleep, drives pruning
	Count    int      // occurrences this fragment stands for; above 1 it is a prototype
	Kind     string   // episodic, word, or concept; empty means episodic
	Lang     string   // language tag on word nodes ("en", "tr"); empty elsewhere
	Category string   // lemma category on concept nodes ("verb", "noun")
	Edited   bool     // hand-touched through the editor, so a shaped memory and a lived one stay distinguishable
	Links    []uint64 // synaptic links to other fragments (the association hook)
}
