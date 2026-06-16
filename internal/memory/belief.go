package memory

// Beliefs are the inferred propositions GOLEM has concluded often enough to keep:
// a layer of long-term memory held apart from the observed graph, so a guess
// that earned its place is remembered without ever being mistaken for something
// seen. The separation is the honesty: an observed proposition lives in the
// fragment graph and reads "yes", a believed one lives here and reads "probably".
// What a belief was drawn from stays in working memory; this store keeps the
// conclusion itself across the night.

// Belief is one inferred proposition kept in long-term memory: a directed
// relation between an agent and a patient, the confidence it is held at, and how
// far it stood from observation when it was drawn.
type Belief struct {
	Rel                  string
	Agent, Patient       uint64
	AgentGloss, PatGloss string
	Conf                 float64
	Depth                int
}

// SaveBelief records an inferred proposition in long-term memory, replacing any
// earlier confidence for the same relation and arguments, so re-promoting a
// belief keeps the latest reading rather than multiplying it.
func (s *Store) SaveBelief(b Belief) error {
	_, err := s.db.Exec(
		`INSERT OR REPLACE INTO beliefs (rel, agent, patient, agent_gloss, pat_gloss, conf, depth)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		b.Rel, b.Agent, b.Patient, b.AgentGloss, b.PatGloss, b.Conf, b.Depth)
	return err
}

// Beliefs reads every inferred proposition kept in long-term memory, the set
// loaded back into working memory after a night so a believed conclusion
// survives the sleep that clears the rest.
func (s *Store) Beliefs() ([]Belief, error) {
	rows, err := s.db.Query(`SELECT rel, agent, patient, agent_gloss, pat_gloss, conf, depth FROM beliefs`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Belief
	for rows.Next() {
		var b Belief
		if err := rows.Scan(&b.Rel, &b.Agent, &b.Patient, &b.AgentGloss, &b.PatGloss, &b.Conf, &b.Depth); err != nil {
			return nil, err
		}
		out = append(out, b)
	}
	return out, rows.Err()
}

// ForgetBelief drops an inferred proposition from long-term memory, the path a
// belief leaves by when the observation it stood in for is finally seen, or when
// it is overturned.
func (s *Store) ForgetBelief(rel string, agent, patient uint64) error {
	_, err := s.db.Exec(`DELETE FROM beliefs WHERE rel = ? AND agent = ? AND patient = ?`, rel, agent, patient)
	return err
}
