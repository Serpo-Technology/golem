package daemon

import (
	"strings"
	"time"

	"golem/internal/memory"
)

// A gradable comparison is composed, not stored as an atomic per-tongue relation.
// "big" and "small" are pole adjectives that grade along a shared scale ("size"),
// each carrying a greater-or-lesser role and a per-tongue comparative surface. A
// told ordering is Proposed on the scale, canonical greater-over-lesser, so both
// poles and every tongue read the same orderings. These readers turn a pole or a
// scale content into the graph facts the write path, the reasoning walk, the
// numeric path, and the describe/justify readers consume. They reference only
// existing graph data (RelScale, RelMarks pole roles, RelOpposite, the realized
// surfaces), so nothing here decides an ordering's direction; the un-gated pole
// role does (invariant I1).

// scaleOf reads the dimension a gradable pole grades along, the pole's RelScale target.
// It is how a pole surface reaches its scale so an ordering hangs on the one dimension
// both poles share.
func (d *Daemon) scaleOf(pole *memory.Fragment) (uint64, bool) {
	if pole == nil {
		return 0, false
	}
	edges, err := d.store.LinksFromRel([]uint64{pole.ID}, memory.RelScale)
	if err != nil || len(edges) == 0 {
		return 0, false
	}
	return edges[0].To, true
}

// poleRoleID returns the id of a pole role concept ("greater" or "lesser"), the
// scaffolding node a pole is marked with. Empty side or an unseeded scaffold yields 0.
func (d *Daemon) poleRoleID(side string) uint64 {
	for id, content := range d.gradableScaffold {
		if content == side {
			return id
		}
	}
	return 0
}

// poleSign reads which end of its scale a pole names, from its un-gated pole role:
// the greater pole reads "gt", the lesser "lt". This role, never a trust gate, is the
// ground truth an ordering canonicalizes by (invariant I1) and the numeric sign derives
// from. It returns false when the pole carries no role, so the write path refuses rather
// than guess a direction.
func (d *Daemon) poleSign(pole *memory.Fragment) (string, bool) {
	if pole == nil {
		return "", false
	}
	greaterID, lesserID := d.poleRoleID("greater"), d.poleRoleID("lesser")
	edges, err := d.store.LinksFromRel([]uint64{pole.ID}, memory.RelMarks)
	if err != nil {
		return "", false
	}
	for _, e := range edges {
		switch e.To {
		case greaterID:
			return "gt", true
		case lesserID:
			return "lt", true
		}
	}
	return "", false
}

// flipSign inverts a pole sign, so "less" (the grammatical inverse of a greater pole)
// and an lt-pole query both read the other end of the scale.
func flipSign(sign string) string {
	if sign == "lt" {
		return "gt"
	}
	return "lt"
}

// canonicalOrdering orders a told comparison into its greater-over-lesser pair by the
// parsed pole sign: a greater-pole telling keeps subject over standard, a lesser-pole
// telling flips them. One canonical direction lives per scale, so both poles read the
// same orderings and the converse is an argument swap rather than a stored edge.
func canonicalOrdering(sign string, subj, std *memory.Fragment) (greater, lesser *memory.Fragment) {
	if sign == "lt" {
		return std, subj
	}
	return subj, std
}

// scaleDimension returns the scale concept a content names, or nil: a resolvable
// concept with poles hung on it by RelScale. It is how a scale-ordering content
// ("size") is turned back into its dimension Fragment for the moment and evidence reads.
func (d *Daemon) scaleDimension(content string) *memory.Fragment {
	dim := d.resolveConcept(content)
	if dim == nil {
		return nil
	}
	edges, err := d.store.LinksToRel([]uint64{dim.ID}, memory.RelScale)
	if err != nil || len(edges) == 0 {
		return nil
	}
	return dim
}

// isScale reports whether a relation content names a scale dimension: a resolvable
// concept with poles hung on it by RelScale. It is how a gathered fact is known to be a
// comparison after the cutover, the scale membership standing where the atomic relation's
// opposite pole stood.
func (d *Daemon) isScale(relContent string) bool {
	return d.scaleDimension(relContent) != nil
}

// scalePole returns the pole of a scale on the named side ("greater" or "lesser"): the
// scale's incoming RelScale poles filtered to the one whose role matches. It is how a
// describe reader speaks a comparison from the focus's side, the greater pole when the
// focus stood above, the lesser when below.
func (d *Daemon) scalePole(scaleContent, side string) *memory.Fragment {
	dim := d.scaleDimension(scaleContent)
	if dim == nil {
		return nil
	}
	poles, err := d.store.LinksToRel([]uint64{dim.ID}, memory.RelScale)
	if err != nil {
		return nil
	}
	ids := make([]uint64, 0, len(poles))
	for _, e := range poles {
		ids = append(ids, e.From)
	}
	frags, err := d.store.FragmentsByIDs(ids)
	if err != nil {
		return nil
	}
	wantSign := "gt"
	if side == "lesser" {
		wantSign = "lt"
	}
	for _, p := range frags {
		if sign, ok := d.poleSign(p); ok && sign == wantSign {
			return p
		}
	}
	return nil
}

// comparativeToPole decomposes a comparative surface into the gradable pole it names,
// its scale, and the sign of the ordering it asserts. It is the write and reasoning
// path's one seam onto the surface, replacing the atomic relation lookup: the grammar
// hands over the raw token ("bigger", "büyük", "less clever", "maior") and this reads a
// pole out of the graph rather than minting a relation. It is candidate-based and
// fail-closed: a stemmed base is accepted only if it resolves to a pole carrying both a
// scale and a role, so "water" or "her" decompose to nothing (ok=false) and the caller
// admits the gap (invariant I4). "less" flips the sign; the Turkish "daha" particle and
// the Latin suppletive overrides are read here too.
func (d *Daemon) comparativeToPole(surface, lang string) (pole *memory.Fragment, scaleID uint64, sign string, ok bool) {
	s := strings.ToLower(strings.TrimSpace(surface))
	if s == "" {
		return nil, 0, "", false
	}
	if lang == "" {
		lang = "en"
	}
	flip := false
	var bases []string
	switch lang {
	case "tr":
		s = strings.TrimSpace(strings.TrimPrefix(s, "daha "))
		bases = []string{s}
	case "la":
		if base, has := latinComparativeBase[s]; has {
			bases = []string{base}
		} else {
			bases = []string{s}
		}
	default: // en and any tongue whose comparative is analytic or -er suffixed
		switch {
		case strings.HasPrefix(s, "more "):
			bases = []string{strings.TrimSpace(strings.TrimPrefix(s, "more "))}
		case strings.HasPrefix(s, "less "):
			bases = []string{strings.TrimSpace(strings.TrimPrefix(s, "less "))}
			flip = true
		default:
			bases = stemComparativeEN(s)
		}
	}
	for _, base := range bases {
		p := d.resolveConcept(base)
		if p == nil {
			continue
		}
		sid, sok := d.scaleOf(p)
		if !sok {
			continue
		}
		sg, gok := d.poleSign(p)
		if !gok {
			continue
		}
		if flip {
			sg = flipSign(sg)
		}
		return p, sid, sg, true
	}
	return nil, 0, "", false
}

// stemComparativeEN reverse-stems an English "-er" comparative into the candidate bases
// its pole adjective might be spelled as, undoing the orthographic sub-rules the forward
// rule applies: consonant doubling (bigger -> big), the silent e (larger -> large), and y
// to i (heavier -> heavy). The candidates are tried in turn and the first that resolves to
// a gradable pole wins, so the stemming stays fail-closed rather than committing to one guess.
func stemComparativeEN(s string) []string {
	if !strings.HasSuffix(s, "er") || len(s) < 4 {
		return []string{s}
	}
	core := s[:len(s)-2]
	cands := []string{core}
	if n := len(core); n >= 2 && core[n-1] == core[n-2] { // undo consonant doubling
		cands = append(cands, core[:n-1])
	}
	if strings.HasSuffix(core, "i") { // undo y -> i
		cands = append(cands, core[:len(core)-1]+"y")
	}
	cands = append(cands, core+"e") // undo the dropped silent e
	return cands
}

// isGradablePoleSurface reports whether a lemma resolves to a gradable pole adjective:
// a concept carrying a scale and a pole role. It is the refusal predicate that keeps a
// comparative surface from ever minting a relation, the belt-and-braces half of the
// structural guard against a comparative frame reaching relationConcept.
func (d *Daemon) isGradablePoleSurface(lemma, lang string) bool {
	if _, _, _, ok := d.comparativeToPole(lemma, lang); ok {
		return true
	}
	c := d.resolveConcept(lemma)
	if c == nil {
		return false
	}
	if _, ok := d.scaleOf(c); !ok {
		return false
	}
	_, ok := d.poleSign(c)
	return ok
}

// realizeComparative returns a pole's comparative surface in a tongue, read from the
// durable RelComparativeForm edge the scale block deposited, so the serve path speaks the
// stored word without running morphology. It returns "" on a miss, so a caller routes the
// admit-gap rather than dropping the word (invariant I4).
func (d *Daemon) realizeComparative(pole *memory.Fragment, tongue, feature string) string {
	if pole == nil {
		return ""
	}
	if tongue == "" {
		tongue = "en"
	}
	edges, err := d.store.LinksFromRel([]uint64{pole.ID}, memory.RelComparativeForm)
	if err != nil {
		return ""
	}
	ids := make([]uint64, 0, len(edges))
	for _, e := range edges {
		ids = append(ids, e.To)
	}
	forms, err := d.store.FragmentsByIDs(ids)
	if err != nil {
		return ""
	}
	for _, f := range forms {
		if f.Lang == tongue {
			return f.Content
		}
	}
	return ""
}

// assertRealizedForm deposits a pole's comparative surface for a tongue as a durable,
// prune-spared edge, so realizeComparative reads it at serve time. The scale block and
// the tests call it to ground "bigger"/"daha büyük"/"maior" onto the poles before the
// first comparison fact.
func (d *Daemon) assertRealizedForm(pole *memory.Fragment, feature, tongue, surface string) {
	if pole == nil || surface == "" {
		return
	}
	w, _, err := d.store.WordNode(surface, tongue, d.chem.Snapshot(), time.Now())
	if err != nil || w == nil {
		return
	}
	d.store.SetLinkRel(pole.ID, w.ID, memory.RelComparativeForm, deliberateName)
}
