package daemon

import (
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

// sayParticipantIsA speaks an identity clause about a participant: the
// pronoun, the agreeing copula, and the predicate with its article. yes
// affirms, and told marks nothing here beyond what the caller already said
// with its lead word; the clause itself is the same either way.
func (d *Daemon) sayParticipantIsA(lang string, memberID uint64, cat *memory.Fragment, yes, told bool) string {
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
