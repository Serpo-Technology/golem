package daemon

import (
	"strings"
	"time"

	"golem/internal/attention"
	"golem/internal/events"
	"golem/internal/grammar"
	"golem/internal/memory"
)

// A told cause connects two propositions: "the dog chased the cat because the
// cat took the bone" teaches both clauses and lays a cause edge from the
// chase's moment to the taking's, so a later "why" walks from the fact to the
// reason it was given. The edge is a telling like any other, defeasible and
// decaying; it is what GOLEM was told explains the fact, never a causal model
// of its own.

// planBecause reads a compound telling "X because Y": both clauses comprehend
// as propositions and the cause edge is laid between their moments. The split
// word is the tongue's seeded cause conjunction, read from the graph. It
// declines when the main clause does not comprehend, leaving the whole
// compound to the ordinary paths; a main clause understood without its cause
// still teaches and is acknowledged.
func (d *Daemon) planBecause(frame grammar.Frame, raw string, momentID uint64) (string, bool) {
	if frame.Force == grammar.ForceQuestion || frame.Force == grammar.ForceCommand {
		return "", false
	}
	tokens, _ := grammar.Tokenize(raw)
	split := -1
	for i, t := range tokens {
		if d.gram.CauseWord(frame.Lang, t) {
			split = i
			break
		}
	}
	if split <= 0 || split >= len(tokens)-1 {
		return "", false
	}
	main := strings.Join(tokens[:split], " ")
	cause := strings.Join(tokens[split+1:], " ")
	mainFrame := d.gram.ParseHint(main, frame.Lang, d.resolveIn(frame.Lang))
	if mainFrame.Lang == "" {
		mainFrame.Lang = frame.Lang
	}
	if mainFrame.Force != grammar.ForceStatement || !d.comprehendStatement(mainFrame, main, momentID) {
		return "", false
	}
	// The cause clause grounds in a moment of its own, so its proposition has a
	// fragment to ride and the cause edge two distinct ends. The moment carries
	// its normalized template like any heard moment: without one, the merge
	// pass keys every cause moment as one duplicate shape and folds them onto a
	// single prototype, re-targeting their cause edges wholesale.
	causeMoment := &memory.Fragment{Kind: memory.KindEpisodic, Channel: "tell", Content: cause,
		Template: attention.Normalize(cause), Limbic: d.chem.Snapshot(), Time: time.Now()}
	if _, err := d.store.Encode(causeMoment); err != nil {
		return d.ackStatement(frame.Lang), true
	}
	causeFrame := d.gram.ParseHint(cause, frame.Lang, d.resolveIn(frame.Lang))
	if causeFrame.Lang == "" {
		causeFrame.Lang = frame.Lang
	}
	if causeFrame.Force == grammar.ForceStatement && d.comprehendStatement(causeFrame, cause, causeMoment.ID) {
		if err := d.store.LinkRel(momentID, causeMoment.ID, memory.RelCause, comprehendWeight); err == nil {
			d.event(events.Event{Ev: "link", From: momentID, To: causeMoment.ID,
				Rel: memory.RelCause, Weight: comprehendWeight})
		}
	}
	// A telling that answered an open question follows up with the answer it
	// can now give, the same precedence the plain statement path keeps.
	if d.followUp != "" {
		return d.followUp, true
	}
	return d.ackStatement(frame.Lang), true
}

// planWhyCause answers a why from a told cause: the asked clause resolves to
// its proposition, the cause edge walks to the proposition it was told
// explains it, and that clause is spoken behind the tongue's because. The
// clause is read from the tokens the way the role questions read theirs,
// since a taught verb is no seeded event verb for the frame parser. It
// declines when the clause resolves to no proposition or no cause was told,
// so the succession walk and its admission stand.
func (d *Daemon) planWhyCause(frame grammar.Frame, raw string) (string, bool) {
	rel, knowns := d.multiRoleQuestionParts(frame.Lang, raw)
	if rel == nil || len(knowns) < 2 {
		return "", false
	}
	moment, ok := d.propositionMoment(rel, knowns[0].ID, knowns[1].ID)
	if !ok {
		return "", false
	}
	edges, err := d.store.LinksFromRel([]uint64{moment}, memory.RelCause)
	if err != nil || len(edges) == 0 {
		return "", false
	}
	agentID, patientID, relContent, ok := d.claimAt(edges[0].To)
	if !ok {
		return "", false
	}
	pair, err := d.store.FragmentsByIDs([]uint64{agentID, patientID})
	if err != nil || len(pair) != 2 {
		return "", false
	}
	gloss := map[uint64]string{}
	for _, f := range pair {
		gloss[f.ID] = d.conceptName(f.ID, f.Content, frame.Lang)
	}
	line, ok := d.gram.Speak(frame.Lang, grammar.Meaning{
		Relation: grammar.Term{Phrase: d.relationName(relContent, frame.Lang)},
		Lang:     frame.Lang,
		Past:     true, // the told cause recounts what had happened
		Roles: map[string]grammar.Term{
			"agent":   {Phrase: gloss[agentID], Definite: true},
			"patient": {Phrase: gloss[patientID], Definite: true},
		},
	})
	if !ok {
		return "", false
	}
	d.noteFocus(knowns[0]) // the asked-about subject stays the topic
	because := d.gram.Word(frame.Lang, "because")
	if because == "" {
		return line, true
	}
	return upperFirst(because) + " " + lowerFirst(line), true
}

// propositionMoment finds the moment that grounds a proposition: the newest
// grounding of the relation whose agent and patient edges point at the given
// pair. It is how a told cause is looked up from the clause it explains.
func (d *Daemon) propositionMoment(rel *memory.Fragment, agentID, patientID uint64) (uint64, bool) {
	grounds, err := d.store.LinksFromRel([]uint64{rel.ID}, memory.RelGrounds)
	if err != nil {
		return 0, false
	}
	var found uint64
	for _, g := range grounds {
		edges, err := d.store.LinksFrom([]uint64{g.To})
		if err != nil {
			continue
		}
		var agentOK, patientOK bool
		for _, e := range edges {
			if e.Rel == memory.RelAgent && e.To == agentID {
				agentOK = true
			}
			if e.Rel == memory.RelPatient && e.To == patientID {
				patientOK = true
			}
		}
		if agentOK && patientOK && g.To > found {
			found = g.To // the newest telling of the pair carries the freshest cause
		}
	}
	return found, found != 0
}
