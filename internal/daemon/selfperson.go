package daemon

import (
	"strings"
	"time"
	"unicode"

	"golem/internal/events"
	"golem/internal/grammar"
	"golem/internal/memory"
)

// A conversation participant speaks in its own person. The declarative copula
// carries only the third person ("is"), the right form for every thing-noun,
// so a clause about the self or the addressee is assembled here with the
// pronoun and the agreement English gives the first and second person: "I am
// not a daemon", "you are a daemon", never "an I is not a daemon". English
// only for now; a pro-drop tongue carries the person on the verb and joins
// when its participant surfaces are built.

// participantPerson gives the subject pronoun and the copula forms a
// participant takes from GOLEM's side of the talk: the self says "I am", the
// addressee hears "you are", the two together "we are". ok is false for a
// concept that is no participant.
func (d *Daemon) participantPerson(id uint64) (subject, cop, negCop string, ok bool) {
	switch id {
	case d.selfID:
		return "I", "am", "am not", true
	case d.userID:
		return "you", "are", "are not", true
	case d.bothID:
		return "we", "are", "are not", true
	}
	return "", "", "", false
}

// planIdentity reads a proper name told for a conversation participant: "you
// are golem" names the self, "I am Ada" names the speaker, and the Turkish
// zero-copula "sen golem" carries the same telling without a copula. A bare
// one-word predicate is a name where an articled one ("you are a daemon") is a
// kind, which the told-membership path keeps; a predicate that resolves to a
// grown category or a relation stays a kind too, so a tongue without articles
// still splits. The word must be new to the tongue or spoken capitalized, so
// "i am hungry" does not name the speaker. Naming takes the architect's trust:
// a peer's bare claim falls through and lands as the defeasible told-identity,
// hedged, never as the name.
func (d *Daemon) planIdentity(frame grammar.Frame, raw string, momentID uint64) (string, bool) {
	if frame.Force != grammar.ForceStatement || frame.Negate || frame.Comparative ||
		frame.LocPrep != "" || frame.Wh != "" || len(frame.Quals) > 0 {
		return "", false
	}
	if frame.Action.Phrase != "" && frame.Action.Phrase != "be" {
		return "", false
	}
	member, other := frame.Agent, frame.Patient
	if member.Concept == nil || !d.isParticipant(member.Concept.ID) {
		member, other = other, member
	}
	if member.Concept == nil || !d.isParticipant(member.Concept.ID) || member.Concept.ID == d.bothID {
		return "", false
	}
	name := strings.ToLower(strings.TrimSpace(other.Phrase))
	if name == "" || len(strings.Fields(name)) != 1 {
		return "", false // a name is one bare word; a fuller predicate is a kind or a fact
	}
	lang := frame.Lang
	if lang == "" {
		lang = "en"
	}
	// An articled predicate is a membership, never a name.
	tokens, _ := grammar.Tokenize(raw)
	for _, t := range tokens {
		if d.gram.WordClass(frame.Lang, t) == "det" {
			return "", false
		}
	}
	// A predicate that grounds in something other than the participant is a
	// kind or a thing, unless the word is offered as a proper name outright.
	if c := other.Concept; c != nil && c.ID != member.Concept.ID {
		if d.isCategory(c) || d.isParticipant(c.ID) || c.Category == "relation" {
			return "", false
		}
	}
	// The name must be new to the lexicon or spoken capitalized: a known word
	// read lowercase is a predicate, not a christening. A word that already
	// names this participant is a re-telling and reinforces whatever its case.
	novel := true
	if words, err := d.store.WordNodesByForms([]string{name}); err == nil && len(words) > 0 {
		novel = false
		ids := make([]uint64, 0, len(words))
		for _, w := range words {
			ids = append(ids, w.ID)
		}
		if names, err := d.store.LinksFromRel(ids, memory.RelNames); err == nil {
			for _, e := range names {
				if e.To == member.Concept.ID {
					novel = true
					break
				}
			}
		}
	}
	if !novel && !capitalizedIn(raw, name) {
		return "", false
	}
	// A participant already named keeps its name against a bare lowercase word
	// ("you are right" mid-talk is a judgement, not a rename); renaming asks
	// for the capitalized form.
	existing := d.namingWord(member.Concept.ID, lang)
	if existing == "" && lang != "en" {
		existing = d.namingWord(member.Concept.ID, "en")
	}
	if existing != "" && existing != name && !capitalizedIn(raw, name) {
		return "", false
	}
	if d.momentTrust(momentID) < architectTrust {
		return "", false // identity is the architect's to give; a peer's claim stays a defeasible telling
	}
	w, _, err := d.mintWordNode(name, lang, d.chem.Snapshot(), time.Now())
	if err != nil {
		return "", false
	}
	if err := d.store.SetLinkRel(w.ID, member.Concept.ID, memory.RelNames, opNameWeight); err != nil {
		return "", false
	}
	d.event(events.Event{Ev: "name", From: w.ID, To: member.Concept.ID, Rel: memory.RelNames, Weight: opNameWeight})
	d.ws.excite(member.Concept.ID, exciteSpeech)
	d.noteFocus(member.Concept)
	return d.sayIdentity(member.Concept.ID, name, frame.Lang), true
}

// capitalizedIn reports whether a word was spoken capitalized in the raw
// utterance, the typographic mark of a proper name that lets a known word
// still be given as one ("I am Rose").
func capitalizedIn(raw, name string) bool {
	for _, f := range strings.Fields(raw) {
		f = strings.Trim(f, " ?.!,")
		if strings.EqualFold(f, name) {
			r := []rune(f)
			return len(r) > 0 && unicode.IsUpper(r[0])
		}
	}
	return false
}

// planWhoIdentity answers "who are you" and "who am I" from the name a
// participant carries: the deictic resolves to the participant and the firmest
// naming word is spoken from GOLEM's side, the English name crossing tongues
// when the asked one has none of its own. It declines when no name has been
// given, so the gap admission stands and its invitation is how the name gets
// taught.
func (d *Daemon) planWhoIdentity(frame grammar.Frame) (string, bool) {
	member := frame.Patient.Concept
	if member == nil || !d.isParticipant(member.ID) {
		member = frame.Agent.Concept
	}
	if member == nil || !d.isParticipant(member.ID) {
		return "", false
	}
	name := d.namingWord(member.ID, frame.Lang)
	if name == "" && frame.Lang != "en" {
		name = d.namingWord(member.ID, "en") // a proper name crosses tongues
	}
	if name == "" {
		return "", false
	}
	d.noteFocus(d.mustFragment(member.ID))
	return d.sayIdentity(member.ID, name, frame.Lang), true
}

// mustFragment fetches a fragment by id for a focus note, nil when it cannot
// be read, which noteFocus already tolerates.
func (d *Daemon) mustFragment(id uint64) *memory.Fragment {
	frags, err := d.store.FragmentsByIDs([]uint64{id})
	if err != nil || len(frags) != 1 {
		return nil
	}
	return frags[0]
}

// sayIdentity speaks a participant's identity from GOLEM's side of the talk:
// the English person forms carry the copula ("I am golem"), and a tongue whose
// participant surface stands without one joins subject to name directly ("Ben
// golem"). It is the confirmation a naming earns and the answer a who-question
// reads back.
func (d *Daemon) sayIdentity(memberID uint64, name, lang string) string {
	if lang == "" || lang == "en" {
		if subject, cop, _, ok := d.participantPerson(memberID); ok {
			return upperFirst(subject) + " " + cop + " " + name + "."
		}
	}
	if subject := d.participantName(memberID, lang); subject != "" {
		return upperFirst(subject) + " " + name + "."
	}
	return upperFirst(name) + "."
}

// sayParticipantIsA speaks an identity clause about a participant: the
// pronoun, the agreeing copula, and the predicate with its article. yes
// affirms, and hedged marks nothing here beyond what the caller already said with its
// lead word, whether a telling or a climbed kind edge stands behind it; the clause
// itself is the same either way.
func (d *Daemon) sayParticipantIsA(lang string, memberID uint64, cat *memory.Fragment, yes, hedged bool) string {
	subject, cop, negCop, ok := d.participantPerson(memberID)
	if !ok {
		return ""
	}
	catName := grammar.Indefinite(d.conceptName(cat.ID, cat.Content, lang))
	if yes {
		return subject + " " + cop + " " + catName + "."
	}
	return subject + " " + negCop + " " + catName + "."
}
