package daemon

import (
	"sort"
	"strings"
	"time"

	"golem/internal/grammar"
	"golem/internal/limbic"
	"golem/internal/memory"
)

// A built-in faculty is a command GOLEM runs on itself: recall a memory,
// consolidate, report its state. Each is seeded as a concept named by the words
// that ask for it: its canonical verb, its synonyms, and its forms in other
// tongues. So a reworded or translated imperative resolves to the same faculty
// the exact verb does, and cross-tongue access falls out of the shared concept.
// The trigger concepts are what the faculty concerns, associated by edges, so a
// verb naming no synonym can still reach the faculty by resembling what it is
// about.
var builtinCommands = []struct {
	name     string
	names    map[string][]string
	triggers []string
}{
	{"recall",
		map[string][]string{"en": {"recall", "recollect", "remember", "remind"}, "tr": {"hatırla", "anımsa"}},
		[]string{"memory"}},
	{"consolidate",
		map[string][]string{"en": {"consolidate"}},
		[]string{"sleep", "memory"}},
	{"introspect",
		map[string][]string{"en": {"introspect", "reflect"}},
		[]string{"self", "feeling"}},
}

// resemble thresholds for routing an imperative to a faculty by how much its verb
// resembles what the faculty concerns.
const (
	commandAccept = 0.6
	commandAsk    = 0.4
	commandMargin = 0.15
)

// ensureCommands seeds GOLEM's faculties as concepts and loads the routing index.
// Idempotent, like the participants: the concepts and their naming and trigger
// edges are reused when present.
func (d *Daemon) ensureCommands() {
	d.commandName = map[uint64]string{}
	d.commandTriggers = map[string][]uint64{}
	for _, bc := range builtinCommands {
		id := d.ensureCommandConcept("command:"+bc.name, bc.name)
		if id == 0 {
			continue
		}
		d.commandName[id] = bc.name
		for lang, words := range bc.names {
			for _, w := range words {
				d.nameConceptWord(id, w, lang)
			}
		}
		var tids []uint64
		for _, t := range bc.triggers {
			tid := d.ensureConcept("trigger:"+t, t)
			if tid == 0 {
				continue
			}
			d.nameConceptWord(tid, t, "en")
			if err := d.store.LinkRel(id, tid, memory.RelMarks, 1); err != nil {
				continue
			}
			tids = append(tids, tid)
		}
		d.commandTriggers[bc.name] = tids
	}
}

// ensureCommandConcept mints a faculty concept, distinct in category so a synonym
// of a faculty is never mistaken for a noun or a relation in ordinary talk.
func (d *Daemon) ensureCommandConcept(template, content string) uint64 {
	if c, err := d.store.ConceptByTemplate(template); err == nil && c != nil {
		return c.ID
	}
	c := &memory.Fragment{Kind: memory.KindConcept, Channel: "concept", Content: content,
		Template: template, Category: "command", Limbic: d.chem.Snapshot(), Time: time.Now()}
	if _, err := d.store.Encode(c); err != nil {
		return 0
	}
	return c.ID
}

// nameConceptWord binds a word in a tongue to a concept, the same naming edge a
// learner lays down, so the word resolves to the concept.
func (d *Daemon) nameConceptWord(conceptID uint64, word, lang string) {
	w, _, err := d.store.WordNode(word, lang, d.chem.Snapshot(), time.Now())
	if err != nil {
		return
	}
	if err := d.store.SetLinkRel(w.ID, conceptID, memory.RelNames, 0.9); err != nil {
		return
	}
}

// routeBuiltinCommand routes an imperative to a faculty when its verb is not the
// exact command word. A verb that names a faculty, in any tongue, runs it. A verb
// that names none but resembles what a faculty concerns proposes it, asked as a
// question when the resemblance is only moderate or two faculties sit close. It
// returns the spoken line and whether it handled the turn; an unhandled turn falls
// through to the grounded-command path.
func (d *Daemon) routeBuiltinCommand(frame grammar.Frame, raw string, chem limbic.State) (string, bool) {
	// A faculty verb the grammar mapped to an intent fills Action; an imperative
	// opener it parsed but did not map leaves Action empty, so the opening word of
	// the utterance is the verb to resolve.
	verb := frame.Action.Phrase
	if verb == "" {
		fields := strings.Fields(raw)
		if len(fields) == 0 {
			return "", false
		}
		verb = strings.ToLower(fields[0])
	}
	c := d.resolveConcept(verb)
	if c == nil {
		return "", false
	}
	if name, ok := d.commandName[c.ID]; ok {
		return d.runBuiltinCommand(name, frame, chem), true
	}
	type scored struct {
		name  string
		score float64
	}
	var ranked []scored
	for name, triggers := range d.commandTriggers {
		best := 0.0
		for _, t := range triggers {
			if r := d.conceptResemblance(c.ID, t); r > best {
				best = r
			}
		}
		ranked = append(ranked, scored{name, best})
	}
	sort.Slice(ranked, func(i, j int) bool {
		if ranked[i].score != ranked[j].score {
			return ranked[i].score > ranked[j].score
		}
		return ranked[i].name < ranked[j].name
	})
	if len(ranked) == 0 || ranked[0].score < commandAsk {
		return "", false // nothing near enough; the grounded-command path may know it
	}
	top := ranked[0]
	margin := top.score
	if len(ranked) >= 2 {
		margin = top.score - ranked[1].score
	}
	if top.score < commandAccept || margin < commandMargin {
		alts := top.name
		if len(ranked) >= 2 && ranked[1].score >= commandAsk {
			alts = top.name + " or " + ranked[1].name
		}
		return "Do you mean to " + alts + "?", true
	}
	return d.runBuiltinCommand(top.name, frame, chem), true
}

// routeFacultyFallback routes a clause that failed to comprehend when its opening
// word names a faculty. It runs only after ordinary statement handling gave up, so
// it cannot steal a clause that taught; and it matches a faculty by name alone, not
// by resemblance, so a faculty's trigger word standing as a subject ("memory
// fades") is never mistaken for a command. This is the path that lets a synonym
// route without being seeded as an imperative opener.
func (d *Daemon) routeFacultyFallback(frame grammar.Frame, raw string) (string, bool) {
	fields := strings.Fields(raw)
	if len(fields) == 0 {
		return "", false
	}
	c := d.resolveConcept(strings.ToLower(fields[0]))
	if c == nil {
		return "", false
	}
	name, ok := d.commandName[c.ID]
	if !ok {
		return "", false
	}
	return d.runBuiltinCommand(name, frame, d.chem.Snapshot()), true
}

// runBuiltinCommand dispatches a resolved faculty. recall reads its topic from the
// clause and asks when none is given, the arity check the closed command set
// affords; the others need no argument.
func (d *Daemon) runBuiltinCommand(name string, frame grammar.Frame, chem limbic.State) string {
	switch name {
	case "recall":
		return d.planRecall(frame, chem)
	case "consolidate":
		outcome, fruitful := d.actConsolidateNow()
		d.finishAct(actConsolidate, "", outcome, fruitful, "asked")
		return "I consolidated: " + outcome + "."
	case "introspect":
		line := d.introspectLine()
		d.finishAct(actIntrospect, "", line, d.lastThought != "", "asked")
		return line
	}
	return ""
}

// conceptResemblance is the cosine of two concepts' embeddings, 1 when they are
// the same concept and 0 when either is unplaced, the graded signal the faculty
// router leans on past the exact synonym match.
func (d *Daemon) conceptResemblance(a, b uint64) float64 {
	if a == b {
		return 1
	}
	va, oka := d.store.Vector(a)
	vb, okb := d.store.Vector(b)
	if !oka || !okb {
		return 0
	}
	return memory.Cosine(va, vb)
}
