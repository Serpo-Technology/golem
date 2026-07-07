package daemon

import (
	"strings"

	"golem/internal/grammar"
	"golem/internal/memory"
)

// latinComparativeBase maps a Latin comparative adjective to the Latin pole adjective it
// decomposes to, a suppletive override read fail-closed: "maior" is the comparative of
// "magnus", "minor" of "parvus". The base resolves to the shared pole concept, so a Latin
// comparison orders on the same scale an English or Turkish one does, and "maior" never
// mints a relation.
var latinComparativeBase = map[string]string{"maior": "magnus", "minor": "parvus"}

// latinComparison reads a Latin comparison "X maior quam Y" (X is bigger than Y): the
// comparative adjective is the raw pole surface, the noun before it the subject, and the
// noun after "quam" the standard. The interrogative est and its enclitic, and quam
// itself, ground in nothing and are skipped. It returns false when no comparative is
// present.
func (d *Daemon) latinComparison(raw string) (subj *memory.Fragment, relWord string, std *memory.Fragment, ok bool) {
	tokens := strings.Fields(strings.ToLower(strings.TrimSuffix(strings.TrimSpace(raw), "?")))
	compIdx := -1
	for i, tok := range tokens {
		if _, has := latinComparativeBase[tok]; has {
			compIdx, relWord = i, tok
			break
		}
	}
	if compIdx < 0 {
		return nil, "", nil, false
	}
	for i := 0; i < compIdx; i++ {
		if c := d.resolveConcept(tokens[i]); c != nil {
			subj = c
			break
		}
	}
	for i := compIdx + 1; i < len(tokens); i++ {
		if c := d.resolveConcept(tokens[i]); c != nil {
			std = c
			break
		}
	}
	return subj, relWord, std, subj != nil && std != nil
}

// comprehendComparisonLA stores a Latin comparison statement as a canonical ordering on
// the shared scale, decomposing the raw Latin comparative surface into its pole the same
// way the English and Turkish write paths do.
func (d *Daemon) comprehendComparisonLA(raw string, momentID uint64) bool {
	subj, relWord, std, ok := d.latinComparison(raw)
	if !ok {
		return false
	}
	w := comprehendWeight * d.momentTrust(momentID)
	return d.storeComparison(grammar.Frame{Comparative: true, Lang: "la",
		Agent:   grammar.Term{Concept: subj},
		Patient: grammar.Term{Concept: std},
		Action:  grammar.Term{Phrase: relWord}}, momentID, w)
}

// planLatinComparison answers a Latin comparison question ("estne rosa maior quam
// viola") by decomposing the comparative to its pole and walking the canonical orderings
// on its scale, the same reasoning the English comparison question uses, and speaking the
// verdict back in Latin with the original subject and standard (invariant I2).
func (d *Daemon) planLatinComparison(frame grammar.Frame, raw string) (string, bool) {
	if frame.Lang != "la" || frame.Force != grammar.ForceQuestion {
		return "", false
	}
	subj, relWord, std, ok := d.latinComparison(raw)
	if !ok {
		return "", false
	}
	d.noteFocus(subj)
	pole, scaleID, sign, pok := d.comparativeToPole(relWord, "la")
	if !pok {
		return d.withInvite("la", "Nescio."), true // no gradable pole: admit the gap (invariant I4)
	}
	sf, err := d.store.FragmentsByIDs([]uint64{scaleID})
	if err != nil || len(sf) != 1 {
		return d.withInvite("la", "Nescio."), true
	}
	_ = pole
	if subj.ID != std.ID {
		walkFrom, walkTo := subj.ID, std.ID
		if sign == "lt" {
			walkFrom, walkTo = std.ID, subj.ID
		}
		if d.comparesTransitively(sf[0], walkFrom, walkTo) {
			clause := d.conceptName(subj.ID, subj.Content, "la") + " " + relWord + " quam " +
				d.conceptName(std.ID, std.Content, "la")
			return upperFirst(d.gram.Word("la", "yes")) + ", " + clause + ".", true
		}
	}
	return upperFirst(d.gram.Word("la", "no")) + ".", true
}

// latinGloss speaks a concept's name in Latin, its own name when it has one and
// its plain content otherwise, never a borrowed one. conceptName falls to the
// vector layer for a concept with no Latin name and proposes its nearest named
// neighbour, which for a category is one of its own members (the animal category
// glossed "vacca"), speaking a thing as one of its members. A definition and an
// is-a must name each concept by its own word or its content, not a guess.
func (d *Daemon) latinGloss(c *memory.Fragment) string {
	if d.conceptNamedIn(c.ID, "la") {
		return d.conceptName(c.ID, c.Content, "la")
	}
	return c.Content
}

// latinPatientForm speaks a concept in the accusative for the patient slot. The
// third declension's accusative is no rule from the nominative (caro/carnem), so
// a taught -em name is used whole when one exists; otherwise the nominative gloss
// stands and the generator cases the regular declensions (-a/-us) itself, a third
// declension noun with no taught accusative keeping its nominative as the honest
// limit. Never a borrowed name, so a food noun is not glossed as a neighbour.
func (d *Daemon) latinPatientForm(id uint64, gloss string) string {
	if namers, err := d.store.LinksToRel([]uint64{id}, memory.RelNames); err == nil && len(namers) > 0 {
		ids := make([]uint64, 0, len(namers))
		for _, e := range namers {
			ids = append(ids, e.From)
		}
		if words, err := d.store.FragmentsByIDs(ids); err == nil {
			for _, w := range words {
				if w.Lang == "la" && strings.HasSuffix(w.Content, "em") {
					return w.Content
				}
			}
		}
	}
	return d.latinGloss(&memory.Fragment{ID: id, Content: gloss})
}

// latinRoleForm speaks a concept in the case its role wears: the accusative for
// the patient, the nominative for the agent, so a Latin clause reads and
// round-trips whatever the noun's declension.
func (d *Daemon) latinRoleForm(role string, id uint64, gloss string) string {
	if role == "patient" {
		return d.latinPatientForm(id, gloss)
	}
	return d.latinGloss(&memory.Fragment{ID: id, Content: gloss})
}

// resolveLatinNoun resolves a Latin token to its concept, trying the nominative
// as written and then the case stems the declensions yield, so a cased mention
// (accusative "equum") reaches the same concept the nominative names.
func (d *Daemon) resolveLatinNoun(tok string) *memory.Fragment {
	if c := d.resolveConcept(tok); c != nil {
		return c
	}
	for _, base := range d.gram.CaseStems("la", tok) {
		if c := d.resolveConcept(base); c != nil {
			return c
		}
	}
	return nil
}

// planLatinDefinition answers "quid est X" (what is X) with the kind X grounds
// under, rendered as a Latin copula clause, the same grounded-kind reading the
// English definition makes. The interrogative and copula lead; the rest is the
// noun asked about. It admits Nescio when the noun is unknown or grounds under no
// kind, and returns false when the clause is not a Latin what-is question, so the
// role-question path still reads "quid <verb>".
func (d *Daemon) planLatinDefinition(frame grammar.Frame, raw string) (string, bool) {
	if frame.Lang != "la" {
		return "", false
	}
	toks := strings.Fields(strings.ToLower(strings.TrimSuffix(strings.TrimSpace(raw), "?")))
	if len(toks) < 3 || toks[0] != "quid" || (toks[1] != "est" && toks[1] != "sunt") {
		return "", false
	}
	var c *memory.Fragment
	for _, t := range toks[2:] {
		if c = d.resolveLatinNoun(t); c != nil {
			break
		}
	}
	if c == nil {
		return d.withInvite("la", "Nescio."), true
	}
	d.noteFocus(c)
	if cat := d.groundedKindOf(c.ID); cat != nil {
		return upperFirst(d.latinGloss(c)) + " est " + d.latinGloss(cat) + ".", true
	}
	return d.withInvite("la", "Nescio."), true
}

// planLatinIsA answers "estne X Y" (is X a Y) as a membership check, the Latin
// counterpart of the English is-a question. It is distinct from the comparison
// "estne X maior quam Y", so a comparative or a quam declines it and leaves the
// comparison path to answer. Grounded or told membership answers Ita; anything
// else Minime. It returns false when the clause is not a two-noun copular
// question.
func (d *Daemon) planLatinIsA(frame grammar.Frame, raw string) (string, bool) {
	if frame.Lang != "la" {
		return "", false
	}
	toks := strings.Fields(strings.ToLower(strings.TrimSuffix(strings.TrimSpace(raw), "?")))
	// The interrogative enclitic -ne on the copula is the question marker ("estne
	// X Y", is X a Y); a bare "est" is a statement that teaches, left to the
	// comprehension path, so only the enclitic form is answered here. The frame
	// parser does not always read the enclitic as question force, so the marker is
	// read from the word rather than frame.Force.
	if len(toks) < 3 || toks[0] != "estne" {
		return "", false
	}
	var nouns []*memory.Fragment
	for _, t := range toks[1:] {
		if t == "quam" || latinComparativeBase[t] != "" {
			return "", false // a comparison, not a membership; leave it to planLatinComparison
		}
		if c := d.resolveLatinNoun(t); c != nil {
			nouns = append(nouns, c)
		}
	}
	if len(nouns) < 2 {
		return "", false
	}
	member, cat := nouns[0], nouns[1]
	yes := member.ID == cat.ID || d.isA(member.ID, cat.ID) || d.toldIsA(member.ID, cat.ID)
	d.noteFocus(member)
	memberName := d.latinGloss(member)
	catName := d.latinGloss(cat)
	if yes {
		return upperFirst(d.gram.Word("la", "yes")) + ", " + memberName + " est " + catName + ".", true
	}
	return upperFirst(d.gram.Word("la", "no")) + ", " + memberName + " non est " + catName + ".", true
}

// latinPrep maps a Latin spatial preposition to the English preposition whose locative
// relation it shares, so "in mensa" joins the same place relation as "in the table".
var latinPrep = map[string]string{"in": "in", "sub": "under", "super": "on"}

// latinPrepFor reverses latinPrep for generation: a relation's preposition to its Latin
// form.
var latinPrepFor = map[string]string{"in": "in", "under": "sub", "on": "super"}

// comprehendLocativeLA reads a Latin locative clause: a preposition governs the ground
// and the bare noun is the figure ("rosa in mensa est", the rose is on the table), the
// copula optional. The preposition resolves to the shared locative relation, so the
// Latin and English place clauses are one relation. It returns false when the clause
// carries no spatial preposition, so ordinary handling stands.
func (d *Daemon) comprehendLocativeLA(raw string, momentID uint64) bool {
	tokens := strings.Fields(strings.ToLower(strings.TrimSpace(raw)))
	prepIdx, prep := -1, ""
	for i, tok := range tokens {
		if p, ok := latinPrep[tok]; ok {
			prepIdx, prep = i, p
			break
		}
	}
	if prepIdx < 0 || prepIdx+1 >= len(tokens) {
		return false
	}
	ground := d.resolveConcept(tokens[prepIdx+1])
	if ground == nil {
		return false
	}
	var figure *memory.Fragment
	for i, tok := range tokens {
		if i == prepIdx || i == prepIdx+1 || tok == "est" {
			continue
		}
		if c := d.resolveConcept(tok); c != nil {
			figure = c
			break
		}
	}
	if figure == nil || figure.ID == ground.ID {
		return false
	}
	return d.storeLocative(momentID, figure, ground, d.prepositionRelation(prep))
}

// planLatinWhere answers "ubi est X" (where is X) by reading the figure's place from
// the locative relations and speaking it back as a Latin clause. It returns false when
// the clause asks no "ubi".
func (d *Daemon) planLatinWhere(raw string) (string, bool) {
	tokens := strings.Fields(strings.ToLower(strings.TrimSuffix(strings.TrimSpace(raw), "?")))
	hasUbi := false
	var figure *memory.Fragment
	for _, tok := range tokens {
		if tok == "ubi" {
			hasUbi = true
			continue
		}
		if tok == "est" {
			continue
		}
		if c := d.resolveConcept(tok); c != nil && figure == nil {
			figure = c
		}
	}
	if !hasUbi || figure == nil {
		return "", false
	}
	d.noteFocus(figure)
	figName := d.conceptName(figure.ID, figure.Content, "la")
	var clauses []string
	for _, f := range d.gatherFacts(figure, "agent", "") {
		if !d.factIsLocative(f.rel) {
			continue
		}
		ground := d.conceptName(f.coID, f.coGloss, "la")
		if cl, ok := d.locativeClause("la", figName, f.rel, ground, false, false, d.chem.Snapshot()); ok {
			clauses = append(clauses, cl)
		}
	}
	if len(clauses) == 0 {
		return "Nescio.", true
	}
	return strings.Join(clauses, " "), true
}

// latinWhClause reads a Latin role question: the interrogative names the case of the
// asked role (quis the nominative agent, quem and quid the accusative patient), and
// the rest of the clause carries the known argument and the verb. It returns the bare
// clause, the role asked after, and false when the utterance carries no interrogative.
func latinWhClause(raw string) (clause, askedRole string, ok bool) {
	s := strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(raw), "?"))
	var rest []string
	for _, tok := range strings.Fields(s) {
		switch strings.ToLower(tok) {
		case "quis":
			askedRole = "agent"
		case "quem", "quid":
			askedRole = "patient"
		default:
			rest = append(rest, tok)
		}
	}
	if askedRole == "" || len(rest) < 2 {
		return "", "", false
	}
	return strings.Join(rest, " "), askedRole, true
}

// planLatinWh answers a Latin role question. The interrogative is stripped, the rest
// comprehended to find the known argument and the relation, and the asked role's
// fillers read from the proposition graph the same way the English "what did X chase"
// walk does, then spoken back as a Latin clause. It returns false when the clause
// carries no Latin interrogative, so other handling stands.
func (d *Daemon) planLatinWh(frame grammar.Frame, raw string) (string, bool) {
	if frame.Lang != "la" {
		return "", false
	}
	if line, ok := d.planLatinWhere(raw); ok {
		return line, true
	}
	clause, askedRole, ok := latinWhClause(raw)
	if !ok {
		return "", false
	}
	gap := func() (string, bool) { return d.withInvite("la", "Nescio."), true } // I do not know
	m, ok := d.gram.Comprehend("la", clause, d.resolveConcept, nil)
	if !ok {
		return gap()
	}
	knownRole := "agent"
	if askedRole == "agent" {
		knownRole = "patient"
	}
	known := m.Roles[knownRole]
	rel := d.resolveRelation(m.Relation.Phrase)
	if known.Concept == nil || rel == nil {
		return gap()
	}
	d.noteFocus(known.Concept)
	var clauses []string
	for _, f := range d.gatherFacts(known.Concept, knownRole, rel.Content) {
		roles := map[string]grammar.Term{
			knownRole: {Phrase: d.latinRoleForm(knownRole, known.Concept.ID, known.Concept.Content)},
			askedRole: {Phrase: d.latinRoleForm(askedRole, f.coID, f.coGloss)},
		}
		if line, ok := d.gram.SpeakDecl("la", grammar.Meaning{
			Relation: grammar.Term{Phrase: m.Relation.Phrase}, Roles: roles,
		}); ok {
			clauses = append(clauses, line)
		}
	}
	if len(clauses) == 0 {
		return gap()
	}
	return strings.Join(clauses, " "), true
}
