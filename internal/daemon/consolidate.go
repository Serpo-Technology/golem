package daemon

import "golem/internal/memory"

// consolidateReasonTopics bounds how many of the day's concepts a night reasons about, so
// consolidation draws a handful of fresh conclusions rather than sweeping the whole graph.
const consolidateReasonTopics = 8

// consolidateReasoning is the reasoning half of consolidation: over the concepts the day's
// strongest facts concern, GOLEM draws what it was NOT asked -- the guesses those concepts'
// resemblances and categories imply -- into the held layer, so a night reasons about what the
// day taught rather than only replaying and pruning it. The conclusions are held, never written
// to the observed graph, so the pass PROPOSES without asserting: an idle guess speaks "probably"
// and fades unless it recurs, and only a conclusion that engagement re-draws rides the same
// promotion a questioned one does. So the reasoning faculty runs unprompted while GOLEM rests,
// and the glass box holds. It reads the day's strong episodes the way replay does (by stored
// weight, so the pre-sleep quiet has not cooled them) and returns how many concepts it reasoned
// over and how many fresh conclusions it drew, for the night's account.
func (d *Daemon) consolidateReasoning() (topics, drawn int) {
	episodes, err := d.store.StrongSince(d.lastSleep, replayFloor, replayCap)
	if err != nil {
		return 0, 0
	}
	seen := map[uint64]bool{}
	var ids []uint64
	for _, e := range episodes {
		for _, role := range []string{memory.RelAgent, memory.RelPatient} {
			edges, err := d.store.LinksFromRel([]uint64{e.ID}, role)
			if err != nil {
				continue
			}
			for _, edge := range edges {
				if !seen[edge.To] {
					seen[edge.To] = true
					ids = append(ids, edge.To)
				}
			}
		}
		if len(ids) >= consolidateReasonTopics {
			break
		}
	}
	if len(ids) > consolidateReasonTopics {
		ids = ids[:consolidateReasonTopics]
	}
	frags, err := d.fragmentsByIDsTiered(ids)
	if err != nil {
		return 0, 0
	}
	before := d.held.len()
	for _, f := range frags {
		// Reason about learned concepts, not the discourse scaffolding: the speaker and GOLEM
		// itself, the class and construction concepts. The engine excludes these from resemblance
		// everywhere, so a guess about a participant would attribute another's doings to it.
		if f.Kind != memory.KindConcept || d.isSystemConcept(f.ID) {
			continue
		}
		topics++
		d.drawInferenceAbout(f)
	}
	return topics, d.held.len() - before
}
