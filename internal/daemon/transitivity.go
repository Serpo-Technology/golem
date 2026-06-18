package daemon

import (
	"golem/internal/grammar"
	"golem/internal/memory"
)

// Transitivity is a property a relation can be TOLD it has, not one baked into a
// construction the way the comparative's ordering is. Told "before is
// transitive", GOLEM records the property on the relation concept and thereafter
// chains it: A before B and B before C entail A before C. The chaining walk
// (comparesTransitively) is general; what makes it fire for an ordinary relation
// is this learned property, so the rule lives in the graph as a fact about the
// relation rather than in the grammar.

// transitiveWeight is the weight of the self-edge that marks a relation
// transitive, matched to the converse edge so a learned relation-property
// persists across the nightly decay the same way a discovered converse does.
const transitiveWeight = 1.0

// markTransitive records that a relation is transitive: a self-edge on the
// relation concept carrying the learned property.
func (d *Daemon) markTransitive(relID uint64) {
	d.store.SetLinkRel(relID, relID, memory.RelTransitive, transitiveWeight)
}

// relationIsTransitive reports whether a relation has been told it is
// transitive, so a query over it chains the told orderings rather than reading
// only the direct fact.
func (d *Daemon) relationIsTransitive(relID uint64) bool {
	edges, err := d.store.LinksFromRel([]uint64{relID}, memory.RelTransitive)
	return err == nil && len(edges) > 0
}

// planProperty learns or reads a property predicated of a relation. "before is
// transitive" marks the relation transitive and confirms; "is before
// transitive" reads the property back. The subject must name a relation GOLEM
// knows; a property predicated of an unknown relation is an honest gap. It
// returns false when the clause predicates no property, so ordinary handling
// stands.
func (d *Daemon) planProperty(frame grammar.Frame) (string, bool) {
	if frame.Property == "" {
		return "", false
	}
	// The relation is the clause's subject: the agent when stated ("outrank is
	// transitive"), the patient when the copula inverted to ask ("is outrank
	// transitive"), so both positions are read.
	subject := frame.Agent
	if subject.Phrase == "" && subject.Concept == nil {
		subject = frame.Patient
	}
	rel := d.propertySubjectRelation(subject)
	if rel == nil {
		// The thing a property is predicated of is not a relation GOLEM knows, so
		// there is nothing to mark: admit the gap rather than invent a rule.
		word := subject.Phrase
		if word == "" && subject.Concept != nil {
			word = subject.Concept.Content
		}
		return d.realizeAs(frame.Lang, grammar.Frame{Force: grammar.ForceStatement,
			Action: grammar.Term{Phrase: "know"}, Negate: true,
			Patient: grammar.Term{Phrase: word}}, d.chem.Snapshot()), true
	}
	d.noteFocus(rel)
	// Only transitivity is a property GOLEM reasons with today; the marker seeds no
	// other, so a set Property is the transitive rule.
	if frame.Force == grammar.ForceQuestion {
		held := d.relationIsTransitive(rel.ID)
		word := "no"
		if held {
			word = "yes"
		}
		return upperFirst(d.gram.Word(frame.Lang, word)) + ".", true
	}
	d.markTransitive(rel.ID)
	return d.confirmProperty(frame.Lang, rel, frame.Property), true
}

// propertySubjectRelation resolves the relation a property clause is about: the
// subject names a relation, reached the same way a role question reaches its
// verb, so "beat" predicated of a property finds the chase-like relation it
// names rather than a noun that shares the surface.
func (d *Daemon) propertySubjectRelation(subject grammar.Term) *memory.Fragment {
	if subject.Concept != nil && subject.Concept.Category == "relation" {
		return subject.Concept
	}
	if subject.Phrase != "" {
		if r := d.resolveRelation(subject.Phrase); r != nil {
			return r
		}
	}
	if subject.Concept != nil {
		return d.resolveRelation(subject.Concept.Content)
	}
	return nil
}

// confirmProperty acknowledges a learned relation-property, the firm shape an
// is-a grounding takes: an acknowledgement and the rule now held, "I see. Before
// is transitive." The clause runs through the copular construction so the tongue
// supplies its own "is", the property standing as a bare predicate.
func (d *Daemon) confirmProperty(lang string, rel *memory.Fragment, property string) string {
	name := d.relationName(rel.Content, lang)
	if name == "" {
		name = rel.Content
	}
	clause := d.realizeAs(lang, grammar.Frame{Force: grammar.ForceStatement,
		Agent:   grammar.Term{Phrase: name},
		Action:  grammar.Term{Phrase: "be"},
		Patient: grammar.Term{Phrase: property}}, d.chem.Snapshot())
	ack := upperFirst(d.gram.Word(lang, "isee"))
	switch {
	case ack != "" && clause != "":
		return ack + ". " + clause
	case clause != "":
		return clause
	}
	return upperFirst(d.gram.Word(lang, "isee")) + "."
}
