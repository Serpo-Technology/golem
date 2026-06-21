package daemon

import "strings"

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
	if c := d.resolveConcept(thing); c != nil {
		if tally, err := d.store.GroundingTally(); err == nil && tally[c.ID] > 0 {
			return "Yes, I know " + thing + ".", true
		}
	}
	return "No, I do not know " + thing + ".", true
}
