package daemon

import (
	"strings"

	"golem/internal/grammar"
)

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
