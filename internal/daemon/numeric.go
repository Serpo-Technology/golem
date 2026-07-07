package daemon

import (
	"strconv"

	"golem/internal/code"
	"golem/internal/grammar"
	"golem/internal/memory"
)

// A numeric comparison is a claim GOLEM can settle for itself: "is 17 bigger
// than 12" over bare numerals needs no told ordering, only the difference its
// own runtime computes. What a gradable word means in number is read from the
// pole it decomposes to: the greater pole gives ">", the lesser "<", the same
// un-gated pole role a told ordering canonicalizes by. So the numeric sign and
// the ordering direction share one substrate without a trust gate, and a surface
// that names no gradable pole falls through to the told orderings. The verdict of
// a run is GOLEM's own observation: it answers firm over any telling, and the
// false direction is denied on record, so a source that claimed it is contradicted
// by the check when the nightly trust pass judges the night.

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

// numericPole reads what a gradable word means in number: the comparison symbol of the
// pole its surface decomposes to, the greater pole giving ">" and the lesser "<". The sign
// comes from the un-gated pole role, never a trust gate (invariant I1), so a told-ordering
// answer never depends on a numeric-mark firmness. It is empty when the surface names no
// gradable pole, so the caller falls through to the told orderings.
func (d *Daemon) numericPole(gradable, lang string) (string, bool) {
	_, _, sign, ok := d.comparativeToPole(gradable, lang)
	if !ok {
		return "", false
	}
	switch sign {
	case "gt":
		return ">", true
	case "lt":
		return "<", true
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
	pole, scaleID, sign, ok0 := d.comparativeToPole(frame.Action.Phrase, frame.Lang)
	if !ok0 || (sign != "gt" && sign != "lt") {
		return nil, nil, 0, false, false
	}
	sym := ">"
	if sign == "lt" {
		sym = "<"
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
	// The refuted ordering is denied between the number concepts, keyed on the scale as a
	// canonical greater-over-lesser edge. The asked comparison claims askedGreater exceeds
	// askedLesser (a lesser pole swaps them first); when it holds, the false converse
	// (askedLesser over askedGreater) is denied, and when it does not, the asked ordering
	// itself is. Either way one directed deny records what the run refuted.
	if sf, err := d.store.FragmentsByIDs([]uint64{scaleID}); err == nil && len(sf) == 1 {
		askedGreaterID, askedLesserID := subjID, stdID
		if sign == "lt" {
			askedGreaterID, askedLesserID = stdID, subjID
		}
		from, to := askedGreaterID, askedLesserID
		if holds {
			from, to = askedLesserID, askedGreaterID
		}
		d.store.SetLinkRel(from, to, denyRel(sf[0].Content), computedFactWeight)
	}
	_ = pole
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
	pole, _, sign, pok := d.comparativeToPole(frame.Action.Phrase, frame.Lang)
	if !pok {
		return "", false
	}
	surface := d.realizeComparative(pole, frame.Lang, "")
	if surface == "" {
		return "", false
	}
	// A "less <greater-adj>" question speaks the greater surface with subject and standard
	// swapped, the same analytic-less relaxation the graph path applies; a genuine lesser
	// pole keeps the original subject (invariant I2).
	saySubj, sayStd := subj, std
	if own, ok := d.poleSign(pole); ok && own != sign {
		saySubj, sayStd = std, subj
	}
	return d.sayComparison(frame.Lang, saySubj, surface, sayStd, holds), true
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
	pole, _, sign, pok := d.comparativeToPole(frame.Action.Phrase, frame.Lang)
	if !pok {
		return "", false
	}
	surface := d.realizeComparative(pole, frame.Lang, "")
	if surface == "" {
		return "", false
	}
	saySubj, sayStd := subj, std
	if own, ok := d.poleSign(pole); ok && own != sign {
		saySubj, sayStd = std, subj
	}
	if !holds {
		return d.sayComparison(frame.Lang, saySubj, surface, sayStd, false), true
	}
	if diff < 0 {
		diff = -diff
	}
	return d.sayMagnitude(frame.Lang, saySubj, surface, sayStd, strconv.Itoa(diff)), true
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
