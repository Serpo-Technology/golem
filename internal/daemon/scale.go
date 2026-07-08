package daemon

import (
	"sort"
	"strconv"
	"strings"
	"time"

	"golem/internal/code"
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
	// An irregular surface (good -> better, maior -> magnus) is owned by a reverse
	// RelComparativeForm lookup that fires before any stemming, fail-closed, so a suppletive
	// form resolves to its pole rather than feeding the stemmer junk. It is one lookup for
	// every tongue, and it is what dissolves the Latin latinComparativeBase map into graph
	// overrides. An analytic "more X" / "less X" surface is left to the prefix handling
	// below (it is the productive inversion, not a suppletive form), so the two do not race.
	if !strings.HasPrefix(s, "more ") && !strings.HasPrefix(s, "less ") {
		if pole, ok := d.poleByComparativeForm(s, lang); ok {
			if sid, sok := d.scaleOf(pole); sok {
				if sg, gok := d.poleSign(pole); gok {
					return pole, sid, sg, true
				}
			}
		}
	}
	flip := false
	var bases []string
	switch lang {
	case "tr":
		s = strings.TrimSpace(strings.TrimPrefix(s, "daha "))
		bases = []string{s}
	case "la":
		// Latin has no productive rule; a comparative is either a suppletive override (caught
		// by the reverse lookup above) or the bare adjective, so the surface stands as its own
		// base with no Go override map.
		bases = []string{s}
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
		// Resolve the base in the comparison's own tongue, so a Turkish comparative
		// surface written in ASCII ("daha buyuk") reaches the diacritic fold gate and
		// decomposes to the existing "büyük" pole rather than missing.
		p := d.resolveConceptLang(base, lang)
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

// poleByComparativeForm resolves the pole whose irregular (meta:irregular) comparative
// surface in a tongue equals the surface, the reverse of the deposited override edge. It
// is the fail-closed lookup that lets "better" reach good and "maior" reach magnus without
// the stemmer ever producing junk, and it reads only meta:irregular forms so a regular
// derived surface never shadows the stemming path.
func (d *Daemon) poleByComparativeForm(surface, tongue string) (*memory.Fragment, bool) {
	irr := d.scaffoldID("meta:irregular")
	if irr == 0 {
		return nil, false
	}
	for _, pole := range d.allPoles() {
		edges, err := d.store.LinksFromRel([]uint64{pole.ID}, memory.RelComparativeForm)
		if err != nil {
			continue
		}
		for _, e := range edges {
			frags, err := d.store.FragmentsByIDs([]uint64{e.To})
			if err != nil || len(frags) != 1 {
				continue
			}
			if frags[0].Lang == tongue && frags[0].Content == surface && d.marks(e.To, irr) {
				return pole, true
			}
		}
	}
	return nil, false
}

// isLatinComparativeWord reports whether a Latin token is a comparative surface, read from
// the graph overrides rather than a Go map. It keys on the meta:irregular reverse lookup, not
// the general pole resolver: comparativeToPole also resolves a bare positive adjective (magnus,
// parvus) by its own name, so keying on it would read a positive as a comparison and wrongly
// bail a Latin is-a or definition sentence that merely mentions one. Only a suppletive override
// surface (maior, minor) decomposes through poleByComparativeForm, which is the comparison.
func (d *Daemon) isLatinComparativeWord(tok string) bool {
	_, ok := d.poleByComparativeForm(strings.ToLower(strings.TrimSpace(tok)), "la")
	return ok
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

// assertRealizedForm HAND-STORES a pole's comparative surface for a tongue as a durable,
// prune-spared edge tagged meta:irregular, the override path: it pre-empts the rule in both
// directions, so the reverse lookup owns the surface's token before any stemming. cmpform
// and the scale-block test helper deposit good/better, daha büyük, and maior/minor this way
// before the first comparison fact. A rule-derived surface takes the separate meta:regular
// deposit in deriveComparativeForm.
func (d *Daemon) assertRealizedForm(pole *memory.Fragment, feature, tongue, surface string) {
	if id, ok := d.depositRealizedForm(pole, tongue, surface); ok {
		d.tagRealizedForm(id, "meta:irregular")
	}
}

// depositRealizedForm mints the surface WordNode and sets the durable RelComparativeForm
// edge, returning the word id so a caller can tag the deposited form with its provenance.
// It is the one write behind both the hand-store (cmpform) and the offline derivation.
func (d *Daemon) depositRealizedForm(pole *memory.Fragment, tongue, surface string) (uint64, bool) {
	if pole == nil || surface == "" {
		return 0, false
	}
	if tongue == "" {
		tongue = "en"
	}
	w, _, err := d.store.WordNode(surface, tongue, d.chem.Snapshot(), time.Now())
	if err != nil || w == nil {
		return 0, false
	}
	d.store.SetLinkRel(pole.ID, w.ID, memory.RelComparativeForm, deliberateName)
	return w.ID, true
}

// scaffoldID reads a seeded scaffold concept's id by its template, 0 when the concept is
// not seeded. It is how the derivation reaches the meta and tongue tags a rule is keyed on.
func (d *Daemon) scaffoldID(template string) uint64 {
	c, err := d.store.ConceptByTemplate(template)
	if err != nil || c == nil {
		return 0
	}
	return c.ID
}

// marks reports whether a node carries a RelMarks edge to tag, the membership test the
// keyed rule selection and the provenance read turn on.
func (d *Daemon) marks(nodeID, tag uint64) bool {
	if nodeID == 0 || tag == 0 {
		return false
	}
	edges, err := d.store.LinksFromRel([]uint64{nodeID}, memory.RelMarks)
	if err != nil {
		return false
	}
	for _, e := range edges {
		if e.To == tag {
			return true
		}
	}
	return false
}

// tagComparativeRule marks a rule construction as the regular comparative rule for a
// tongue: RelMarks edges to cmp:comparative, meta:regular, and the tongue tag, so the
// keyed selection walk finds it deterministically. It mirrors the pole-role tagging the
// scale block writes.
func (d *Daemon) tagComparativeRule(ruleNodeID uint64, tongue string) bool {
	comp := d.scaffoldID("cmp:comparative")
	reg := d.scaffoldID("meta:regular")
	lang := d.scaffoldID("lang:" + tongue)
	if ruleNodeID == 0 || comp == 0 || reg == 0 || lang == 0 {
		return false
	}
	d.store.SetLinkRel(ruleNodeID, comp, memory.RelMarks, deliberateName)
	d.store.SetLinkRel(ruleNodeID, reg, memory.RelMarks, deliberateName)
	d.store.SetLinkRel(ruleNodeID, lang, memory.RelMarks, deliberateName)
	return true
}

// comparativeRuleName selects the regular comparative rule for a tongue by a keyed graph
// walk: the code construction carrying RelMarks to cmp:comparative AND meta:regular AND the
// tongue tag. It returns the operation name (its "code:" template stripped) so the
// derivation builds a call to it. Deterministic and fail-closed: no matching rule yields
// false and the caller admits the gap.
func (d *Daemon) comparativeRuleName(tongue string) (string, bool) {
	comp := d.scaffoldID("cmp:comparative")
	reg := d.scaffoldID("meta:regular")
	lang := d.scaffoldID("lang:" + tongue)
	if comp == 0 || reg == 0 || lang == 0 {
		return "", false
	}
	marked, err := d.store.LinksToRel([]uint64{comp}, memory.RelMarks)
	if err != nil {
		return "", false
	}
	for _, e := range marked {
		node := e.From
		if !d.marks(node, reg) || !d.marks(node, lang) {
			continue
		}
		frags, err := d.store.FragmentsByIDs([]uint64{node})
		if err != nil || len(frags) != 1 {
			continue
		}
		if name, ok := strings.CutPrefix(frags[0].Template, "code:"); ok {
			return name, true
		}
	}
	return "", false
}

// poleBaseWord reads a pole's canonical lemma in a tongue: its firmest naming word,
// ties broken by the lowest fragment id, a total order so the derived surface is
// byte-identical across re-runs whatever the DB row order.
func (d *Daemon) poleBaseWord(pole *memory.Fragment, tongue string) (string, bool) {
	edges, err := d.store.LinksToRel([]uint64{pole.ID}, memory.RelNames)
	if err != nil || len(edges) == 0 {
		return "", false
	}
	ids := make([]uint64, 0, len(edges))
	for _, e := range edges {
		ids = append(ids, e.From)
	}
	words, err := d.store.FragmentsByIDs(ids)
	if err != nil {
		return "", false
	}
	byID := make(map[uint64]*memory.Fragment, len(words))
	for _, w := range words {
		byID[w.ID] = w
	}
	var cands []memory.LinkEdge
	for _, e := range edges {
		if w, ok := byID[e.From]; ok && w.Lang == tongue {
			cands = append(cands, e)
		}
	}
	if len(cands) == 0 {
		return "", false
	}
	sort.Slice(cands, func(i, j int) bool {
		if cands[i].Weight != cands[j].Weight {
			return cands[i].Weight > cands[j].Weight // firmest first
		}
		return cands[i].From < cands[j].From // then lowest id
	})
	return byID[cands[0].From].Content, true
}

// allPoles enumerates every gradable pole: a concept marked with a greater or lesser pole
// role. It is how the batched derivation sweeps the poles that carry a scale and a role.
func (d *Daemon) allPoles() []*memory.Fragment {
	greater, lesser := d.poleRoleID("greater"), d.poleRoleID("lesser")
	seen := map[uint64]bool{}
	var ids []uint64
	for _, role := range []uint64{greater, lesser} {
		if role == 0 {
			continue
		}
		edges, err := d.store.LinksToRel([]uint64{role}, memory.RelMarks)
		if err != nil {
			continue
		}
		for _, e := range edges {
			if !seen[e.From] {
				seen[e.From] = true
				ids = append(ids, e.From)
			}
		}
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	frags, err := d.store.FragmentsByIDs(ids)
	if err != nil {
		return nil
	}
	// FragmentsByIDs may reorder; restore the id order so the sweep is deterministic.
	byID := make(map[uint64]*memory.Fragment, len(frags))
	for _, f := range frags {
		byID[f.ID] = f
	}
	out := make([]*memory.Fragment, 0, len(ids))
	for _, id := range ids {
		if f, ok := byID[id]; ok {
			out = append(out, f)
		}
	}
	return out
}

// deriveComparativeForm runs the tongue's comparative morphology rule on a pole's base
// word OFFLINE and freezes the output into the durable RelComparativeForm edge the serve
// path reads, so the serve path never runs code. It is fail-closed at every gate: it skips
// a pole that already carries a form (idempotent, and an irregular hand-store pre-empts the
// rule); routes an analytic-marked pole to "more <base>" without running the rule; selects
// the rule by the keyed meta walk; runs it through the offline runner; and deposits only if
// the derived surface decomposes back to the same pole (the per-pole round-trip gate that
// keeps a guessed form off deliberateName). It returns the deposited surface, false when it
// admitted the gap.
func (d *Daemon) deriveComparativeForm(pole *memory.Fragment, tongue, feature string) (string, bool) {
	if pole == nil || !d.cfg.RuntimeEnabled {
		return "", false
	}
	if tongue == "" {
		tongue = "en"
	}
	// A pole already carrying a form (a re-run, or an irregular hand-stored first) is left
	// untouched, so the rule can never overwrite "better" with "gooder".
	if s := d.realizeComparative(pole, tongue, feature); s != "" {
		return s, false
	}
	base, ok := d.poleBaseWord(pole, tongue)
	if !ok {
		return "", false
	}
	// An analytic-marked pole is realized "more <base>" without running the synthetic rule,
	// the driver-level branch the rule body (whose only input is the stem) cannot make.
	if d.marks(pole.ID, d.scaffoldID("cmp:analytic")) {
		surface := "more " + base
		if p, _, _, ok := d.comparativeToPole(surface, tongue); !ok || p == nil || p.ID != pole.ID {
			return "", false
		}
		if id, ok := d.depositRealizedForm(pole, tongue, surface); ok {
			d.tagRealizedForm(id, "cmp:analytic")
			return surface, true
		}
		return "", false
	}
	ruleName, ok := d.comparativeRuleName(tongue)
	if !ok {
		return "", false
	}
	comp := code.Computation{Op: ruleName, Args: []code.Computation{{Lit: strconv.Quote(base)}}}
	src, ok := d.generateCode(comp, nil, 0)
	if !ok {
		return "", false
	}
	surface, ok := d.evalCode(src)
	if !ok || surface == "" {
		return "", false
	}
	// The per-pole round-trip gate: deposit only if the derived surface decomposes back to
	// this same pole, so a non-decomposable form (a pole outside the held-out orthographic
	// classes) is refused before it is frozen rather than served forever.
	if p, _, _, ok := d.comparativeToPole(surface, tongue); !ok || p == nil || p.ID != pole.ID {
		return "", false
	}
	id, ok := d.depositRealizedForm(pole, tongue, surface)
	if !ok {
		return "", false
	}
	// Provenance: the deposited form links to its rule construction and meta:regular, so a
	// "why did you say bigger" trace walks base -> rule -> form.
	d.tagRealizedForm(id, "meta:regular")
	if ruleNode, err := d.store.FragmentByTemplate("code:" + ruleName); err == nil && ruleNode != nil {
		d.store.SetLinkRel(id, ruleNode.ID, memory.RelMarks, deliberateName)
	}
	return surface, true
}

// tagRealizedForm marks a deposited comparative surface with a meta tag (meta:regular for a
// derived form, cmp:analytic for an analytic one), the provenance a justification trace
// reads.
func (d *Daemon) tagRealizedForm(formID uint64, template string) {
	if tag := d.scaffoldID(template); tag != 0 {
		d.store.SetLinkRel(formID, tag, memory.RelMarks, deliberateName)
	}
}
