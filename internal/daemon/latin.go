package daemon

import (
	"strings"

	"golem/internal/grammar"
	"golem/internal/memory"
)

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
	gap := func() (string, bool) { return "Nescio.", true } // I do not know
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
			knownRole: {Phrase: d.conceptName(known.Concept.ID, known.Concept.Content, "la")},
			askedRole: {Phrase: d.conceptName(f.coID, f.coGloss, "la")},
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
