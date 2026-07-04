package daemon

import (
	"strconv"

	"golem/internal/code"
	"golem/internal/grammar"
	"golem/internal/memory"
)

// A numeric comparison is a claim GOLEM can settle for itself: "is 17 bigger
// than 12" over bare numerals needs no told ordering, only the difference its
// own runtime computes. What a gradable word means in number is TAUGHT, not
// coded: "bigger is above" marks the relation with the grounded comparison it
// runs on (a marks edge to the cmp concept, the told-property shape), so the
// mapping lives in the graph, erodible and auditable, and an unmarked gradable
// falls through to the told orderings as before. The verdict of a run is
// GOLEM's own observation: it answers firm over any telling, and the false
// direction is denied on record, so a source that claimed it is contradicted
// by the check when the nightly trust pass judges the night.

// cmpMarkFloor is how firmly a relation must be marked with a comparison
// before the mark is believed: a full-trust telling writes above it, a
// low-trust source's cannot set it alone, since a flipped pole would invert
// arithmetic truth.
const cmpMarkFloor = 0.5

// numericLiteral reports whether a phrase is a bare integer numeral, an
// optional minus then digits.
func numericLiteral(s string) bool {
	if s == "" {
		return false
	}
	if s[0] == '-' {
		s = s[1:]
	}
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// planPole learns what a gradable word means in number: told "bigger is
// above" by a fully trusted speaker, the relation the subject names is marked
// with the comparison the predicate names, a marks edge to the grounded cmp
// concept. A speaker below full trust is refused: a flipped pole would invert
// every checked comparison, so the mapping stays with the architect.
func (d *Daemon) planPole(frame grammar.Frame, momentID uint64) (string, bool) {
	if frame.Force != grammar.ForceStatement || frame.Action.Phrase != "be" {
		return "", false
	}
	poleWord := frame.Patient.Phrase
	if poleWord == "" {
		poleWord = frame.LocPrep
	}
	if poleWord == "" {
		return "", false
	}
	sym, ok := d.cmpRelator(poleWord)
	if !ok || sym == "" || sym == d.cmpDefault {
		return "", false
	}
	rel := d.propertySubjectRelation(frame.Agent)
	if rel == nil {
		return "", false
	}
	if d.momentTrust(momentID) < trustAuthorityFloor {
		return "I take what a word means in number from the architect.", true
	}
	cmpID, ok := d.cmpConceptBySymbol(sym)
	if !ok {
		return "", false
	}
	d.store.SetLinkRel(rel.ID, cmpID, memory.RelMarks, deliberateName)
	return d.confirmProperty(frame.Lang, rel, poleWord), true
}

// cmpConceptBySymbol finds the grounded comparison concept carrying a symbol,
// the inverse read of cmpConcept.
func (d *Daemon) cmpConceptBySymbol(sym string) (uint64, bool) {
	for id, s := range d.cmpConcept {
		if s == sym {
			return id, true
		}
	}
	return 0, false
}

// numericPole reads what a gradable word means in number: the comparison
// symbol its relation was marked with, empty when the word was never taught a
// pole, so the caller falls through to the told orderings.
func (d *Daemon) numericPole(gradable string) (string, bool) {
	rel := d.resolveRelation(gradable)
	if rel == nil {
		return "", false
	}
	edges, err := d.store.LinksFromRel([]uint64{rel.ID}, memory.RelMarks)
	if err != nil {
		return "", false
	}
	for _, e := range edges {
		if e.Weight < cmpMarkFloor {
			continue
		}
		if sym, ok := d.cmpConcept[e.To]; ok {
			return sym, true
		}
	}
	return "", false
}

// compareNumbers settles a comparative question over bare numerals by
// computing: the difference of the two, from the landed fact when one stands
// in either direction, from a run otherwise, the sign deciding the taught
// pole. The verdict is firm, GOLEM's own observation outranking any telling,
// and the false direction is denied on record between the number concepts so
// a source that claimed it is contradicted when the trust pass judges the
// night. It declines when either side is no numeral or the gradable carries
// no taught pole, so the told-ordering path stands untouched.
func (d *Daemon) compareNumbers(frame grammar.Frame) (string, bool) {
	a, b := frame.Agent.Phrase, frame.Patient.Phrase
	if a == "" && frame.Agent.Concept != nil {
		a = frame.Agent.Concept.Content
	}
	if b == "" && frame.Patient.Concept != nil {
		b = frame.Patient.Concept.Content
	}
	if !numericLiteral(a) || !numericLiteral(b) {
		return "", false
	}
	sym, ok := d.numericPole(frame.Action.Phrase)
	if !ok || (sym != ">" && sym != "<") {
		return "", false
	}
	diff, ran, ok := d.numericDifference(a, b)
	if !ok {
		return "", false
	}
	_ = ran
	holds := (sym == ">" && diff > 0) || (sym == "<" && diff < 0)
	subjID, stdID := d.ensureNumberConcept(a), d.ensureNumberConcept(b)
	if subjID == 0 || stdID == 0 {
		return "", false
	}
	frags, err := d.store.FragmentsByIDs([]uint64{subjID, stdID})
	if err != nil || len(frags) != 2 {
		return "", false
	}
	subj, std := frags[0], frags[1]
	if subj.ID != subjID {
		subj, std = std, subj
	}
	// The direction the run refuted is denied on record between the numbers,
	// the defeasible negative every read honours and the nightly trust pass
	// judges a source's claims against.
	if rel := d.resolveRelation(frame.Action.Phrase); rel != nil {
		from, to := subjID, stdID
		if holds {
			from, to = stdID, subjID
		}
		d.store.SetLinkRel(from, to, denyRel(rel.Content), computedFactWeight)
	}
	d.noteFocus(subj)
	return d.sayComparison(frame.Lang, subj, frame.Action.Phrase, std, holds), true
}

// numericDifference reads the difference of two numerals: the landed fact in
// either direction, or a fresh run landed with its provenance. The comparison
// rides subtraction so one arithmetic fact settles both poles and both
// directions of asking.
func (d *Daemon) numericDifference(a, b string) (diff int, ran, ok bool) {
	comp := code.Computation{Op: "subtract", Args: []code.Computation{{Lit: a}, {Lit: b}}}
	if val, ok := d.computedFact(comp); ok {
		if n, err := strconv.Atoi(val); err == nil {
			return n, false, true
		}
	}
	// The swapped fact answers with its sign flipped, so one landing serves
	// every later asking of the pair.
	swapped := code.Computation{Op: "subtract", Args: []code.Computation{{Lit: b}, {Lit: a}}}
	if val, ok := d.computedFact(swapped); ok {
		if n, err := strconv.Atoi(val); err == nil {
			return -n, false, true
		}
	}
	val, ran, ok := d.computeValue(comp, nil, 0)
	if !ok {
		return 0, ran, false
	}
	n, err := strconv.Atoi(val)
	if err != nil {
		return 0, ran, false
	}
	return n, ran, true
}
