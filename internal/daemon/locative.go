package daemon

import (
	"strings"

	"golem/internal/grammar"
	"golem/internal/limbic"
	"golem/internal/memory"
)

// Locativity is a property a relation can carry: that it places its figure at its
// ground ("the leaf is on the tree"). Like transitivity it lives in the graph as
// a fact about the relation, not in a construction. GOLEM does not need to be told
// it separately, though: the locative construction itself teaches it. A relation
// spoken as a copula with a seeded preposition ("is on", "is in", "is under") is
// marked locative when it is comprehended, so the property grows from the same
// sentences that lay the spatial facts, and thereafter a "where" query reads it
// and the clause renders as a copula with a prepositional complement rather than
// a verb.

// locativeWeight is the weight of the self-edge that marks a relation locative,
// matched to the transitive marker so a learned relation-property persists across
// the nightly decay the same way.
const locativeWeight = 1.0

// markLocative records that a relation is locative: a self-edge on the relation
// concept carrying the learned property.
func (d *Daemon) markLocative(relID uint64) {
	d.store.SetLinkRel(relID, relID, memory.RelLocative, locativeWeight)
}

// relationIsLocative reports whether a relation has been marked locative, so a
// query over it reads its figure's place and the clause renders with a
// preposition rather than as a verb.
func (d *Daemon) relationIsLocative(relID uint64) bool {
	edges, err := d.store.LinksFromRel([]uint64{relID}, memory.RelLocative)
	return err == nil && len(edges) > 0
}

// factIsLocative reports whether a gathered fact's relation is locative, the test
// the answer paths use to render a clause with its preposition rather than a verb.
func (d *Daemon) factIsLocative(relContent string) bool {
	rel := d.resolveRelation(relContent)
	return rel != nil && d.relationIsLocative(rel.ID)
}

// locativeClause renders "FIGURE is PREP GROUND" ("the leaf is on the tree")
// through the copula construction, the preposition and ground riding as the
// complement the way an is-a answer's category does. The figure and ground are
// already glossed; the figure is "it" when the subject is carried from a clause
// just spoken. It returns false when the engine cannot speak the clause.
func (d *Daemon) locativeClause(lang, figure, prep, ground string, given, negate bool, chem limbic.State) (string, bool) {
	subject := definiteForm(lang, figure)
	if given {
		if pron := d.gram.Pronoun(lang, "agent"); pron != "" {
			subject = pron
		}
	}
	comp := prep + " " + definiteForm(lang, ground)
	line := d.realizeAs(lang, grammar.Frame{
		Force:   grammar.ForceStatement,
		Action:  grammar.Term{Phrase: "be"},
		Agent:   grammar.Term{Phrase: subject},
		Patient: grammar.Term{Phrase: comp},
		Negate:  negate,
	}, chem)
	if line == "" {
		return "", false
	}
	return line, true
}

// definiteForm marks a gloss definite for a tongue that uses an article ("the
// tree"); a caseless tongue (Turkish) leaves the bare noun, its definiteness read
// from context until the locative case lands.
func definiteForm(lang, gloss string) string {
	if lang == "tr" || gloss == "" {
		return gloss
	}
	return "the " + gloss
}

// planLocativePolar answers "is X on Y" from the placed proposition. A copula
// question carrying a locative preposition is a place question, not a category
// one: subject-auxiliary inversion seats the figure in the patient slot and the
// ground in the agent slot, and the preposition names the relation. It reuses
// answerPolar, so the affirmed clause is the same copula the figure's "where"
// reads ("Yes, the leaf is on the tree"). It returns false when the clause is not
// a locative copula question, so ordinary copula handling stands.
func (d *Daemon) planLocativePolar(frame grammar.Frame) (string, bool) {
	// A wh-question carrying a preposition ("what is on the tree") asks for a role,
	// not a yes/no, so it is left to the role-question path; only a bare yes/no
	// copula question is a locative polar.
	if frame.Force != grammar.ForceQuestion || frame.LocPrep == "" || frame.Wh != "" {
		return "", false
	}
	figure, ground := frame.Patient.Concept, frame.Agent.Concept
	if figure == nil {
		figure, ground = frame.Agent.Concept, frame.Patient.Concept
	}
	if figure == nil || ground == nil {
		return d.realizeAs(frame.Lang, grammar.Frame{Force: grammar.ForceStatement,
			Action: grammar.Term{Phrase: "know"}, Negate: true}, d.chem.Snapshot()), true
	}
	d.noteFocus(figure)
	rel := d.resolveRelation(frame.LocPrep)
	if rel == nil {
		// The preposition has never placed anything, so the proposition cannot hold.
		return upperFirst(d.gram.Word(frame.Lang, "no")) + ".", true
	}
	belief, _, supposed := d.assessProposition(figure, rel.Content, ground)
	return d.answerPolar(frame.Lang, figure, rel, ground, false, belief, supposed), true
}

// planWhere answers "where is X" from the figure's locative relations: it walks
// the relations marked locative that X stands in as figure and speaks the place,
// "The leaf is on the tree." It returns false when X holds no place, so an event
// "where did X happen" falls through to the occurrence answer rather than a
// spatial one it has nothing for.
func (d *Daemon) planWhere(frame grammar.Frame, chem limbic.State) (string, bool) {
	figure := frame.Agent
	if figure.Concept == nil && figure.Phrase == "" {
		figure = frame.Patient
	}
	fc := figure.Concept
	if fc == nil && figure.Phrase != "" {
		fc = d.resolveConcept(figure.Phrase)
	}
	if fc == nil {
		return "", false
	}
	d.noteFocus(fc)
	figName := d.conceptName(fc.ID, fc.Content, frame.Lang)
	var clauses []string
	for _, f := range d.gatherFacts(fc, "agent", "") {
		if !d.factIsLocative(f.rel) {
			continue
		}
		ground := d.conceptName(f.coID, f.coGloss, frame.Lang)
		prep := d.relationName(f.rel, frame.Lang)
		if clause, ok := d.locativeClause(frame.Lang, figName, prep, ground, len(clauses) > 0, false, chem); ok {
			clauses = append(clauses, clause)
		}
	}
	if len(clauses) == 0 {
		return "", false
	}
	return strings.Join(clauses, " "), true
}
