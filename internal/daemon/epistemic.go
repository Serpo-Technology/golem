package daemon

import (
	"sort"
	"strings"

	"golem/internal/grammar"
)

// epistemicOpener marks the auxiliaries that front a "do you know" question. The
// clause cannot be read from its frame: "do" also opens an imperative, so the
// grammar takes the whole thing for a command, and "know" is a function word, so it
// never reads as a verb. The question is recognized from its tokens instead.
var epistemicOpener = map[string]bool{"do": true, "does": true, "did": true}

// planEpistemic answers a reflexive question about what GOLEM itself knows. GOLEM
// knows a thing when it has grounded it in experience, so a thing with a positive
// grounding tally answers yes and an ungrounded or unrecognized thing answers no.
// Read with "I" instead of "you", the same shape asks about the speaker, whose
// knowledge GOLEM cannot see. The whole "do you know ..." shape is caught here so
// the command misreading never surfaces, even for a thing GOLEM cannot place.
func (d *Daemon) planEpistemic(raw string) (string, bool) {
	tokens := strings.Fields(strings.ToLower(strings.Trim(raw, " ?.!")))
	if len(tokens) < 3 || !epistemicOpener[tokens[0]] || tokens[2] != "know" {
		return "", false
	}
	// "Do you know X" asks of GOLEM; "do I know X" asks of the speaker.
	subject := tokens[1]
	if subject != "you" && subject != "i" {
		return "", false
	}
	thing := strings.Join(tokens[3:], " ")
	if thing == "" {
		return "Know what?", true
	}
	if subject == "i" {
		return "I cannot tell what you know.", true
	}
	// A proposition as the object of know ("do you know that the dog chased the
	// cat") asks whether GOLEM holds that fact, not whether it knows an incidental
	// noun in it. The optional "that" is dropped; a clause that is not a full
	// proposition falls through to the concept check below.
	if line, ok := d.knowsProposition(strings.TrimPrefix(thing, "that ")); ok {
		return line, true
	}
	if c := d.resolveConcept(thing); c != nil {
		if tally, err := d.store.GroundingTally(); err == nil && tally[c.ID] > 0 {
			return "Yes, I know " + thing + ".", true
		}
	}
	return "No, I do not know " + thing + ".", true
}

// knowsProposition answers whether GOLEM holds a proposition named as the object
// of know. The clause is comprehended with word order authoritative, the way a
// polar question and a supposition read theirs, so the roles are the ones asked.
// GOLEM knows the fact when the graph holds it outright or deduces it soundly
// (beliefObserved); a mere resemblance guess is not knowing. It returns false when
// the clause is not a full proposition over a known relation, leaving the object to
// be read as a plain concept.
func (d *Daemon) knowsProposition(clause string) (string, bool) {
	m, ok := d.gram.Comprehend("en", clause, d.resolveConcept, nil)
	if !ok {
		return "", false
	}
	agent, patient := m.Roles["agent"], m.Roles["patient"]
	rel := d.resolveRelation(m.Relation.Phrase)
	if agent.Concept == nil || patient.Concept == nil || rel == nil {
		return "", false
	}
	echo := clause
	if line, spoke := d.gram.Speak("en", grammar.Meaning{
		Relation: grammar.Term{Phrase: d.relationName(rel.Content, "en")},
		Roles: map[string]grammar.Term{
			"agent":   {Phrase: d.conceptName(agent.Concept.ID, agent.Concept.Content, "en"), Definite: true},
			"patient": {Phrase: d.conceptName(patient.Concept.ID, patient.Concept.Content, "en"), Definite: true},
		},
		Past: m.Past,
	}); spoke {
		echo = lowerFirst(strings.TrimSuffix(strings.TrimSpace(line), "."))
	}
	belief, _, _ := d.assessProposition(agent.Concept, rel.Content, patient.Concept)
	if belief == beliefObserved {
		return "Yes, I know that " + echo + ".", true
	}
	return "No, I do not know that " + echo + ".", true
}

// planKnowledgeSummary answers the open "what do you know" with a report of what
// GOLEM holds, read from the graph: the kinds it has abstracted and the actions it
// has learned. It is the counterpart to "what do you know about X", which reads one
// concept; the bare question reads the whole graph. The "about X" form is handled
// before this, so only the open question reaches here.
func (d *Daemon) planKnowledgeSummary(raw string) (string, bool) {
	if strings.ToLower(strings.Trim(strings.TrimSpace(raw), " ?.!")) != "what do you know" {
		return "", false
	}
	return d.knowledgeSummary(), true
}

// knowledgeSummary reads the concept layer into a short self-report: the kinds are
// the abstractions GOLEM has formed, the actions its verbal relations. System
// concepts (the participants, the faculties) are left out, and gradable and
// placing relations are kept out of the action list so it reads as things done.
func (d *Daemon) knowledgeSummary() string {
	concepts, err := d.store.Concepts()
	if err != nil {
		return "I have not learned much yet."
	}
	var kinds, actions []string
	for _, c := range concepts {
		if d.isSystemConcept(c.ID) {
			continue
		}
		switch {
		case strings.HasPrefix(c.Template, "abs:en:"):
			// A kind to report is an English superordinate: an abstraction over
			// several members whose own word heads none of them ("animal" over the
			// dog and the cat, where a basic-level shadow's word heads its own
			// percept). The template lemma names it, free of the inherited-member
			// gloss that can otherwise pollute a category's spoken name; the Turkish
			// twin is left out so the English report names each kind once.
			lemma, _ := abstractionLemma(c.Template)
			if members := d.categoryMembers(c.ID); len(members) > 0 && !d.headsCorePercept(lemma, members) {
				kinds = append(kinds, lemma)
			}
		case c.Category == "relation" && d.verbalRelation(c.ID):
			actions = append(actions, d.conceptName(c.ID, c.Content, "en"))
		}
	}
	kinds = capWords(kinds, 5)
	actions = capWords(actions, 5)

	var parts []string
	if len(kinds) > 0 {
		parts = append(parts, "about kinds like "+grammar.JoinAnd(kinds))
	}
	if len(actions) > 0 {
		parts = append(parts, "how they "+grammar.JoinAnd(actions))
	}
	if len(parts) == 0 {
		return "I have not learned much yet."
	}
	return "I know " + strings.Join(parts, ", and ") + "."
}

// capWords sorts a set of names, drops duplicates, and keeps at most n, so a
// self-report reads as a short, stable list of examples rather than a dump.
func capWords(words []string, n int) []string {
	sort.Strings(words)
	out := words[:0]
	var last string
	for _, w := range words {
		if w == "" || w == last {
			continue
		}
		out = append(out, w)
		last = w
	}
	if len(out) > n {
		out = out[:n]
	}
	return out
}
