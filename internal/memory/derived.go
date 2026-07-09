package memory

// Derived facts are the SOUND conclusions GOLEM has worked out and remembered: a
// transitive chain closed, a category membership composed, a symmetric relation
// reversed, a containment carried through. They are held apart from the observed
// graph, like beliefs, so a reasoned fact is never mistaken for something seen.
// The difference from a belief is certainty: a belief is a hedged guess and reads
// "probably", a derived fact is a sound deduction and reads "yes". This store is an
// INDEX of what GOLEM has concluded, for a census, a description, and what it can
// offer; the firm answer to any question is still recomputed live against the
// graph, so a derived row is never the authority for an assertion and can never go
// stale-spoken. What a derived fact was reasoned from is re-walked live, not kept
// here; this store keeps the conclusion and how it was reached.

// DerivedFact is one sound conclusion kept in long-term memory: a directed
// relation between an agent and a patient, and the kind of deduction that reached
// it. There is no confidence field: a sound deduction is certain.
type DerivedFact struct {
	Rel                  string
	Agent, Patient       uint64
	AgentGloss, PatGloss string
	Kind                 string // "transitive" | "categorical" | "symmetric" | "containment"
}

// SaveDerived records a sound conclusion, replacing any earlier one for the same
// relation and arguments, so re-deriving it keeps a single row rather than
// multiplying it.
func (s *Store) SaveDerived(f DerivedFact) error {
	_, err := s.db.Exec(
		`INSERT OR REPLACE INTO derived (rel, agent, patient, agent_gloss, pat_gloss, kind)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		f.Rel, f.Agent, f.Patient, f.AgentGloss, f.PatGloss, f.Kind)
	return err
}

// DerivedFacts reads every sound conclusion kept, the ledger a census re-verifies
// and surfaces.
func (s *Store) DerivedFacts() ([]DerivedFact, error) {
	return s.queryDerived(`SELECT rel, agent, patient, agent_gloss, pat_gloss, kind FROM derived`)
}

// DerivedAbout reads the sound conclusions with a given agent, the ones a
// description of that concept would voice.
func (s *Store) DerivedAbout(agent uint64) ([]DerivedFact, error) {
	return s.queryDerived(
		`SELECT rel, agent, patient, agent_gloss, pat_gloss, kind FROM derived WHERE agent = ?`, agent)
}

func (s *Store) queryDerived(query string, args ...any) ([]DerivedFact, error) {
	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []DerivedFact
	for rows.Next() {
		var f DerivedFact
		if err := rows.Scan(&f.Rel, &f.Agent, &f.Patient, &f.AgentGloss, &f.PatGloss, &f.Kind); err != nil {
			return nil, err
		}
		out = append(out, f)
	}
	return out, rows.Err()
}

// HasDerived reports whether a sound conclusion is already kept, so a night does
// not re-draw one it has already worked out.
func (s *Store) HasDerived(rel string, agent, patient uint64) (bool, error) {
	var n int
	err := s.db.QueryRow(
		`SELECT COUNT(1) FROM derived WHERE rel = ? AND agent = ? AND patient = ?`, rel, agent, patient).Scan(&n)
	return n > 0, err
}

// ForgetDerived drops a sound conclusion, the path a derived fact leaves by when a
// premise it rested on is retracted, or when the graph is later told it directly.
func (s *Store) ForgetDerived(rel string, agent, patient uint64) error {
	_, err := s.db.Exec(`DELETE FROM derived WHERE rel = ? AND agent = ? AND patient = ?`, rel, agent, patient)
	return err
}
