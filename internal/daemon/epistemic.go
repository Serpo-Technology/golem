package daemon

import (
	"sort"
	"strings"

	"golem/internal/grammar"
	"golem/internal/memory"
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
	if d.knowsThing(thing) {
		return "Yes, I know " + thing + ".", true
	}
	return "No, I do not know " + thing + ".", true
}

// knowsThing reports whether GOLEM knows the thing a phrase names, in either of the
// two ways a thing comes to be known. By acquaintance: the concept has a positive
// grounding tally, a thing GOLEM has perceived. By description: the concept fills a
// role in a told proposition, a thing GOLEM was told a fact about. An abstract word
// like "llm" can never be perceived, so it is known the second way alone, yet it is
// known, the same facts "what do you know about X" reports back. It is the shared bar
// behind "do you know X" in every tongue, so each surface phrases the verdict its own
// way.
func (d *Daemon) knowsThing(phrase string) bool {
	c := d.resolveConcept(phrase)
	if c == nil {
		return false
	}
	if tally, err := d.store.GroundingTally(); err == nil && tally[c.ID] > 0 {
		return true
	}
	return d.participatesInProposition(c.ID)
}

// participatesInProposition reports whether the concept stands as the agent or
// patient of a grounded proposition, the edge a told fact lays and consolidation
// spares from the groundless-concept sweep. This is knowledge by description: even a
// concept that names no percept is known once a fact has been told of it.
func (d *Daemon) participatesInProposition(id uint64) bool {
	for _, role := range []string{memory.RelAgent, memory.RelPatient} {
		if edges, err := d.store.LinksToRel([]uint64{id}, role); err == nil && len(edges) > 0 {
			return true
		}
	}
	return false
}

// trKnowVerb and trKnowParticle recognize the Turkish "do you know" question: the
// verb of knowing ("biliyor"/"bilir", the progressive and aorist of bilmek) before
// a second-person question particle ("biliyor musun", "bilir misin"). The first
// person ("biliyor muyum") asks of the speaker and is left out, as "do I know" is.
var trKnowVerb = map[string]bool{"biliyor": true, "bilir": true}
var trKnowParticle = map[string]bool{"musun": true, "müsün": true, "mısın": true, "misin": true}

// planEpistemicTR answers the Turkish "X biliyor musun" the way planEpistemic
// answers the English "do you know X": a thing GOLEM has grounded it knows, named
// back with the accusative the question carried, otherwise it does not.
func (d *Daemon) planEpistemicTR(raw string) (string, bool) {
	tokens := strings.Fields(strings.ToLower(strings.Trim(raw, " ?.!")))
	if len(tokens) < 3 {
		return "", false
	}
	if !trKnowParticle[tokens[len(tokens)-1]] || !trKnowVerb[tokens[len(tokens)-2]] {
		return "", false
	}
	obj := tokens[:len(tokens)-2]
	if len(obj) == 0 {
		return "Neyi?", true
	}
	// A factual nominalization as the object ("köpeğin kediyi kovaladığını") asks
	// whether GOLEM holds that proposition, not whether it knows a noun in it.
	if line, ok := d.knowsPropositionTR(obj); ok {
		return line, true
	}
	// The object wears the accusative ("kediyi"); resolve it to the base concept
	// through the case stems the Turkish morphology yields before grounding it.
	phrase := strings.Join(obj, " ")
	known := false
	for _, base := range d.gram.CaseStems("tr", phrase) {
		if d.knowsThing(base) {
			known = true
			break
		}
	}
	if known {
		return "Evet, " + phrase + " biliyorum.", true
	}
	return "Hayır, " + phrase + " bilmiyorum.", true
}

// trNominalVerb recovers a verb stem from a Turkish factual nominalization
// ("kovaladığını" -> "kovala"): the -DIK nominalizer with a third-person possessive
// and the accusative fused on, across vowel harmony and the voiced/voiceless start.
// The nominalization is past-factual, the only embedded tense GOLEM is asked about.
func trNominalVerb(w string) (string, bool) {
	for _, suf := range []string{"dığını", "diğini", "duğunu", "düğünü", "tığını", "tiğini", "tuğunu", "tüğünü"} {
		if strings.HasSuffix(w, suf) && len([]rune(w)) > len([]rune(suf))+1 {
			return strings.TrimSuffix(w, suf), true
		}
	}
	return "", false
}

// knowsPropositionTR answers the Turkish "<subject> <object> <nominalized verb>
// biliyor musun": the genitive subject, accusative object, and -DIK verb name a
// proposition, settled against the graph the way the English "that" clause is, and
// the nominalization is echoed back. It returns false when the object is not such a
// clause, leaving it to be read as a plain concept.
func (d *Daemon) knowsPropositionTR(tokens []string) (string, bool) {
	if len(tokens) < 3 {
		return "", false
	}
	stem, ok := trNominalVerb(tokens[len(tokens)-1])
	if !ok {
		return "", false
	}
	rel := d.resolveRelation(stem)
	if rel == nil {
		return "", false
	}
	subject := d.resolveGenitiveTR(tokens[0])
	patient := d.resolveAccusativeTR(strings.Join(tokens[1:len(tokens)-1], " "))
	if subject == nil || patient == nil {
		return "", false
	}
	echo := strings.Join(tokens, " ")
	if belief, _, _ := d.assessProposition(subject, rel.Content, patient); belief == beliefObserved {
		return "Evet, " + echo + " biliyorum.", true
	}
	return "Hayır, " + echo + " bilmiyorum.", true
}

// resolveGenitiveTR resolves a genitive-cased Turkish subject to its concept,
// trying the stems the genitive yields ("köpeğin" -> "köpek").
func (d *Daemon) resolveGenitiveTR(w string) *memory.Fragment {
	if cands, ok := d.gram.GenitiveGround("tr", w); ok {
		for _, c := range cands {
			if r := d.resolveConcept(c); r != nil {
				return r
			}
		}
	}
	return d.resolveConcept(w)
}

// resolveAccusativeTR resolves an accusative-cased Turkish object to its concept,
// trying the case stems ("kediyi" -> "kedi").
func (d *Daemon) resolveAccusativeTR(phrase string) *memory.Fragment {
	for _, base := range d.gram.CaseStems("tr", phrase) {
		if r := d.resolveConcept(base); r != nil {
			return r
		}
	}
	return d.resolveConcept(phrase)
}

// planEpistemicLA answers the Latin "scisne X", the enclitic -ne on the
// second-person scis (you know) asking whether GOLEM knows X, settled against the
// same grounding the other tongues read.
func (d *Daemon) planEpistemicLA(raw string) (string, bool) {
	tokens := strings.Fields(strings.ToLower(strings.Trim(raw, " ?.!")))
	if len(tokens) < 2 || tokens[0] != "scisne" {
		return "", false
	}
	// "scisne quod <clause>": a proposition as the object of know, the clause read
	// by the same case-driven comprehension a Latin statement is.
	if rest := tokens[1:]; len(rest) >= 4 && rest[0] == "quod" {
		if line, ok := d.knowsPropositionLA(strings.Join(rest[1:], " ")); ok {
			return line, true
		}
	}
	obj := strings.Join(tokens[1:], " ")
	known := d.knowsThing(obj)
	if !known {
		// The object of scio is accusative ("scisne puellam"); resolve its
		// nominative when the cased form names nothing directly.
		if base := strings.TrimSuffix(obj, "m"); base != obj {
			known = d.knowsThing(base)
		}
	}
	if known {
		return "Ita, " + obj + " scio.", true
	}
	return "Minime, " + obj + " nescio.", true
}

// knowsPropositionLA answers the Latin "scisne quod <clause>": the clause is
// comprehended by case the way a Latin statement is, and GOLEM knows the fact when
// the graph holds it (beliefObserved), not when it merely guesses. It returns false
// when the clause is not a full proposition over a known relation.
func (d *Daemon) knowsPropositionLA(clause string) (string, bool) {
	m, ok := d.gram.Comprehend("la", clause, d.resolveConcept, nil)
	if !ok {
		return "", false
	}
	agent, patient := m.Roles["agent"], m.Roles["patient"]
	rel := d.resolveRelation(m.Relation.Phrase)
	if agent.Concept == nil || patient.Concept == nil || rel == nil {
		return "", false
	}
	if belief, _, _ := d.assessProposition(agent.Concept, rel.Content, patient.Concept); belief == beliefObserved {
		return "Ita, scio quod " + clause + ".", true
	}
	return "Minime, nescio quod " + clause + ".", true
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
	switch strings.ToLower(strings.Trim(strings.TrimSpace(raw), " ?.!")) {
	case "what do you know":
		return d.knowledgeSummary("en"), true
	case "ne biliyorsun", "neler biliyorsun":
		return d.knowledgeSummary("tr"), true
	case "quid scis":
		return d.knowledgeSummary("la"), true
	}
	return "", false
}

// knowledgeSummary reads the concept layer into a short self-report: the kinds are
// the abstractions GOLEM has formed, the actions its verbal relations. System
// concepts (the participants, the faculties) are left out, and gradable and
// placing relations are kept out of the action list so it reads as things done.
func (d *Daemon) knowledgeSummary(lang string) string {
	concepts, err := d.store.Concepts()
	if err != nil {
		return notLearnedMuch(lang)
	}
	prefix := "abs:" + lang + ":"
	var kinds, actions []string
	for _, c := range concepts {
		if d.isSystemConcept(c.ID) {
			continue
		}
		switch {
		case strings.HasPrefix(c.Template, prefix):
			// A kind to report is a superordinate in this tongue: an abstraction over
			// several members whose own word heads none of them ("animal" over the
			// dog and the cat, where a basic-level shadow's word heads its own
			// percept). The template lemma names it, free of the inherited-member
			// gloss that can otherwise pollute a category's spoken name; only the
			// tongue asked is read, so each kind is named once in its own language.
			// A reported kind generalizes over several members (three or more), which
			// keeps a real superordinate ("hayvan" over the animals) and leaves out a
			// two-member collision an abstraction sometimes forms by accident.
			lemma, _ := abstractionLemma(c.Template)
			if members := d.categoryMembers(c.ID); distinctIDs(members) >= 3 && !d.headsCorePercept(lemma, members) {
				kinds = append(kinds, lemma)
			}
		case c.Category == "relation" && d.verbalRelation(c.ID):
			actions = append(actions, d.relationName(c.Content, lang))
		}
	}
	return summaryLine(lang, capWords(kinds, 5), capWords(actions, 5))
}

// summaryLine phrases the kinds and actions GOLEM holds in each tongue. English
// names both, the kinds and how the things in them act; Turkish and Latin name the
// kinds, the verb-list phrasing their grammars want left for later.
func summaryLine(lang string, kinds, actions []string) string {
	switch lang {
	case "tr":
		if len(kinds) == 0 {
			return notLearnedMuch("tr")
		}
		return upperFirst(joinAndLang("tr", kinds) + " gibi türleri biliyorum.")
	case "la":
		if len(kinds) == 0 {
			return notLearnedMuch("la")
		}
		return upperFirst(joinAndLang("la", kinds) + " novi.")
	}
	var parts []string
	if len(kinds) > 0 {
		parts = append(parts, "about kinds like "+grammar.JoinAnd(kinds))
	}
	if len(actions) > 0 {
		parts = append(parts, "how they "+grammar.JoinAnd(actions))
	}
	if len(parts) == 0 {
		return notLearnedMuch("en")
	}
	return "I know " + strings.Join(parts, ", and ") + "."
}

// notLearnedMuch is the honest gap a thin brain gives, spoken in the asked tongue.
func notLearnedMuch(lang string) string {
	switch lang {
	case "tr":
		return "Henüz pek bir şey bilmiyorum."
	case "la":
		return "Parum novi."
	}
	return "I have not learned much yet."
}

// joinAndLang coordinates a list with the tongue's conjunction, "and", "ve", "et".
func joinAndLang(lang string, items []string) string {
	and := "and"
	switch lang {
	case "tr":
		and = "ve"
	case "la":
		and = "et"
	}
	switch len(items) {
	case 0:
		return ""
	case 1:
		return items[0]
	case 2:
		return items[0] + " " + and + " " + items[1]
	default:
		return strings.Join(items[:len(items)-1], ", ") + " " + and + " " + items[len(items)-1]
	}
}

// distinctIDs counts the unique ids in a slice, so a category is weighed by how
// many different things it generalizes, not how many grounding edges it carries.
func distinctIDs(ids []uint64) int {
	seen := make(map[uint64]bool, len(ids))
	for _, id := range ids {
		seen[id] = true
	}
	return len(seen)
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
