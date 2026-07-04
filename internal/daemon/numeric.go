package daemon

import (
	"fmt"
	"strconv"
	"time"

	"golem/internal/code"
	"golem/internal/events"
	"golem/internal/grammar"
	"golem/internal/limbic"
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

// numericCheck settles a numeric comparative frame by computing: both sides
// bare numerals, the gradable's taught pole, the difference from the landed
// fact or a run, the verdict by sign. The refuted direction is denied on
// record between the number concepts, the defeasible negative every read
// honours and the nightly trust pass judges a source's claims against. ok is
// false when either side is no numeral or the gradable carries no pole, so
// the told-ordering paths stand untouched.
func (d *Daemon) numericCheck(frame grammar.Frame) (subj, std *memory.Fragment, diff int, holds, ok bool) {
	a, b := frame.Agent.Phrase, frame.Patient.Phrase
	if a == "" && frame.Agent.Concept != nil {
		a = frame.Agent.Concept.Content
	}
	if b == "" && frame.Patient.Concept != nil {
		b = frame.Patient.Concept.Content
	}
	if !numericLiteral(a) || !numericLiteral(b) {
		return nil, nil, 0, false, false
	}
	sym, ok0 := d.numericPole(frame.Action.Phrase)
	if !ok0 || (sym != ">" && sym != "<") {
		return nil, nil, 0, false, false
	}
	diff, _, ok0 = d.numericDifference(a, b)
	if !ok0 {
		return nil, nil, 0, false, false
	}
	holds = (sym == ">" && diff > 0) || (sym == "<" && diff < 0)
	subjID, stdID := d.ensureNumberConcept(a), d.ensureNumberConcept(b)
	if subjID == 0 || stdID == 0 {
		return nil, nil, 0, false, false
	}
	frags, err := d.store.FragmentsByIDs([]uint64{subjID, stdID})
	if err != nil || len(frags) != 2 {
		return nil, nil, 0, false, false
	}
	subj, std = frags[0], frags[1]
	if subj.ID != subjID {
		subj, std = std, subj
	}
	if rel := d.resolveRelation(frame.Action.Phrase); rel != nil {
		from, to := subjID, stdID
		if holds {
			from, to = stdID, subjID
		}
		d.store.SetLinkRel(from, to, denyRel(rel.Content), computedFactWeight)
	}
	return subj, std, diff, holds, true
}

// compareNumbers answers a comparative question over bare numerals from the
// check: firm, GOLEM's own observation outranking any telling.
func (d *Daemon) compareNumbers(frame grammar.Frame) (string, bool) {
	subj, std, _, holds, ok := d.numericCheck(frame)
	if !ok {
		return "", false
	}
	d.noteFocus(subj)
	return d.sayComparison(frame.Lang, subj, frame.Action.Phrase, std, holds), true
}

// magnitudeNumbers answers "how much bigger is 17 than 12" over bare numerals
// with the metric amount the difference computes, where a told ordering could
// only bucket ranks. A subject not standing on the asked side takes the plain
// denial, the shape planMagnitude already answers with.
func (d *Daemon) magnitudeNumbers(frame grammar.Frame) (string, bool) {
	subj, std, diff, holds, ok := d.numericCheck(frame)
	if !ok {
		return "", false
	}
	d.noteFocus(subj)
	if !holds {
		return d.sayComparison(frame.Lang, subj, frame.Action.Phrase, std, false), true
	}
	if diff < 0 {
		diff = -diff
	}
	return d.sayMagnitude(frame.Lang, subj, frame.Action.Phrase, std, strconv.Itoa(diff)), true
}

// countMembers answers a counting question whose topic is a category: the
// tally of the concepts the category grounds, GOLEM looking at its own graph.
// The count is derived fresh at each asking and audited, never landed as a
// fact, since it moves as the brain grows. It declines a topic that is no
// category, so the episode tally answers as before.
func (d *Daemon) countMembers(frame grammar.Frame, topic grammar.Term, chem limbic.State) (string, bool) {
	// The category can sit in any term of the asking ("how many animals do you
	// know" seats the pronoun as the patient and the animals as the subject),
	// and its span can carry other words, so every term's tokens are tried
	// through their lemmas until one names a category.
	var c *memory.Fragment
	for _, term := range []grammar.Term{topic, frame.Agent, frame.Patient} {
		if d.isCategory(term.Concept) {
			c = term.Concept
			break
		}
		if term.Phrase == "" {
			continue
		}
		toks, _ := grammar.Tokenize(term.Phrase)
		for _, tok := range toks {
			lemma := d.gram.Lemma(frame.Lang, tok)
			if cand := d.resolveConceptLang(lemma, frame.Lang); d.isCategory(cand) {
				c = cand
				break
			}
		}
		if c != nil {
			break
		}
	}
	if c == nil {
		return "", false
	}
	edges, err := d.store.LinksFromRel([]uint64{c.ID}, memory.RelGrounds)
	if err != nil {
		return "", false
	}
	targets := make([]uint64, 0, len(edges))
	for _, e := range edges {
		targets = append(targets, e.To)
	}
	frags, err := d.store.FragmentsByIDs(targets)
	if err != nil {
		return "", false
	}
	n := 0
	for _, f := range frags {
		if f.Kind == memory.KindConcept {
			n++
		}
	}
	name := d.conceptName(c.ID, c.Content, frame.Lang)
	d.event(events.Event{Ev: "count", Cue: name, Outcome: strconv.Itoa(n)})
	if err := d.store.Audit(time.Now().Unix(), "count", name, strconv.Itoa(n)+" members"); err != nil {
		fmt.Fprintf(d.out, "audit write failed: %v\n", err)
	}
	d.noteFocus(c)
	spoken := name
	if n != 1 {
		spoken = d.gram.PluralNoun(frame.Lang, name)
	}
	return d.realizeAs(frame.Lang, grammar.Frame{Force: grammar.ForceStatement,
		Action:  grammar.Term{Phrase: "know"},
		Patient: grammar.Term{Phrase: strconv.Itoa(n) + " " + spoken}}, chem), true
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
