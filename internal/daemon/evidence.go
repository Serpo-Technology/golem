package daemon

import (
	"strings"

	"golem/internal/grammar"
	"golem/internal/memory"
)

// The evidence tally is how much stands behind a claim: how often the
// proposition was encountered, whether it was seen or told and by whom, and
// whether a denial stands over it. It is the companion to the justification
// trace: where "why do you say that" reads the proof, "how sure are you" reads
// the weight of it. Both are derived live from the graph the answer already
// rests on, so nothing new is stored; the tally aggregates the same moments
// propositionTrust walks, reading their occurrence counts and provenance rather
// than the firmest trust alone. Honesty as calibrated labelling, made speakable.
type evidence struct {
	moments     int  // distinct moments carrying the tuple
	occurrences int  // times the proposition was encountered (summed moment counts)
	seen        int  // occurrences on a perceptual channel
	told        int  // occurrences on the tell channel
	sources     int  // distinct told-sources named on the moments
	architect   bool // at least one untagged (architect's) telling
	denied      bool // a standing denial, direct or categorical
}

// propositionEvidence tallies what stands behind a proposition, over the moments
// whose role edges carry this agent and patient under this relation, the same
// walk propositionTrust makes. Each moment lends its occurrence count to the
// total, split into seen and told by the channel it came in on, and its source
// (RelSource) to the count of who vouched for it; an untagged moment is the
// architect's. A standing denial is read from the deny edges the polar path uses.
func (d *Daemon) propositionEvidence(agentID uint64, relContent string, patientID uint64) evidence {
	var ev evidence
	agEdges, err := d.store.LinksToRel([]uint64{agentID}, memory.RelAgent)
	if err != nil {
		return ev
	}
	sourceSeen := map[uint64]bool{}
	for _, e := range agEdges {
		a, p, rel, ok := d.claimAt(e.From)
		if !ok || a != agentID || p != patientID || rel != relContent {
			continue
		}
		frags, err := d.store.FragmentsByIDs([]uint64{e.From})
		if err != nil || len(frags) != 1 {
			continue
		}
		m := frags[0]
		count := m.Count
		if count < 1 {
			count = 1
		}
		ev.moments++
		ev.occurrences += count
		if m.Channel == "tell" || m.Channel == "thought" {
			ev.told += count
		} else {
			ev.seen += count
		}
		src, err := d.store.LinksFromRel([]uint64{m.ID}, memory.RelSource)
		if err == nil && len(src) > 0 {
			if !sourceSeen[src[0].To] {
				sourceSeen[src[0].To] = true
				ev.sources++
			}
		} else {
			ev.architect = true
		}
	}
	if af, err := d.fragmentsByIDsTiered([]uint64{agentID}); err == nil && len(af) == 1 {
		ev.denied = d.propositionDenied(af[0], relContent, patientID) ||
			d.propositionDeniedCategorically(af[0], relContent, patientID)
	}
	return ev
}

// planEvidence answers "how sure are you": the weight-of-evidence companion to
// "why do you say that". It reads the last claim's tuple off the trace the
// answer left, tallies what stands behind it, and speaks the tally, calibrated
// and honest. The ask phrases are seeded per tongue and matched whole, so the
// reply speaks the tongue the probe arrived in. It returns false on any other
// clause, and defers to the justify probe (routed before it) for the proof.
func (d *Daemon) planEvidence(frame grammar.Frame, raw string) (string, bool) {
	ask := strings.Join(strings.Fields(strings.ToLower(strings.Trim(raw, " ?.!"))), " ")
	if ask == "" {
		return "", false
	}
	lang := ""
	for _, lg := range []string{frame.Lang, "en", "tr", "la"} {
		if lg == "" {
			continue
		}
		if ask == d.gram.Word(lg, "sureask") {
			lang = lg
			break
		}
	}
	if lang == "" {
		return "", false
	}
	// Reading the weight of evidence is not asserting, so the trace stays in
	// mind and the probe can be asked again.
	d.turnTraced = true
	if len(d.lastTrace) == 0 {
		return justifySurface(d, lang, "justifynone"), true
	}
	relID, agentID, patientID, _ := d.traceTuple(d.lastTrace[0])
	if agentID == 0 || patientID == 0 {
		return justifySurface(d, lang, "justifynone"), true
	}
	// An is-a claim carries no relation concept; its weight is the grounding
	// tally the definition already speaks, so it answers with the seen count.
	if relID == 0 {
		if tally, err := d.store.GroundingTally(); err == nil && tally[agentID] > 0 {
			if line := d.countClause(lang, tally[agentID], d.chem.Snapshot()); line != "" {
				return line, true
			}
		}
		return justifySurface(d, lang, "justifysaw"), true
	}
	relContent := ""
	if rf, err := d.store.FragmentsByIDs([]uint64{relID}); err == nil && len(rf) == 1 {
		relContent = rf[0].Content
	}
	ev := d.propositionEvidence(agentID, relContent, patientID)
	return d.speakEvidence(lang, ev), true
}

// speakEvidence renders a tally as an honest line about how much stands behind
// the claim, leading with a confidence adverb the tally earns when the tongue
// seeds one. A denial owns the answer; a proposition seen owns the seen count;
// one told owns the telling and who gave it; a guess with no stored moment
// admits it is a guess.
func (d *Daemon) speakEvidence(lang string, ev evidence) string {
	if ev.denied {
		return justifySurface(d, lang, "surenone")
	}
	if ev.moments == 0 {
		// The last answer was a hedge from resemblance or a category, with no
		// moment of its own: honest about being a guess.
		return justifySurface(d, lang, "sureguess")
	}
	lead := func(key, line string) string {
		if adv := d.gram.Word(lang, key); adv != "" && line != "" {
			return upperFirst(adv) + ", " + lowerFirst(line)
		}
		return line
	}
	// Seen carries the most weight; a firm, repeated seeing reads certain.
	if ev.seen > 0 {
		line := d.countClause(lang, ev.seen, d.chem.Snapshot())
		if ev.seen >= 3 {
			return lead("certain", line)
		}
		return line
	}
	// Told: name who, and how firmly the telling reads. The architect's word
	// and a well-repeated telling read likely; a single peer telling, unsure.
	toldLine := d.toldEvidenceLine(lang, ev)
	switch {
	case ev.architect && ev.told >= 2:
		return lead("certain", toldLine)
	case ev.architect:
		return lead("likely", toldLine)
	case ev.sources >= 2:
		return lead("likely", toldLine)
	default:
		return lead("unsure", toldLine)
	}
}

// toldEvidenceLine speaks who told GOLEM and how often, the told half of the
// tally: the architect named as the addressee, a lone peer by its name, several
// sources by their count. It falls back to a plain "I was told so" when the
// tongue seeds none of the pieces.
func (d *Daemon) toldEvidenceLine(lang string, ev evidence) string {
	if ev.architect {
		you := d.gram.Word(lang, "justifytoldyou")
		if you == "" {
			return justifySurface(d, lang, "justifytoldso")
		}
		if ev.told >= 2 {
			if again := d.gram.Word(lang, "often"); again != "" {
				return upperFirst(you) + ", " + again + "."
			}
		}
		return upperFirst(you) + "."
	}
	if ev.sources >= 2 {
		many := d.gram.Word(lang, "manysources")
		if many == "" {
			return justifySurface(d, lang, "justifytoldso")
		}
		return upperFirst(many) + "."
	}
	return justifySurface(d, lang, "justifytoldso")
}
