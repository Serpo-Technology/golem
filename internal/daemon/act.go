package daemon

import (
	"fmt"
	"strings"
	"time"

	"golem/internal/events"
	"golem/internal/grammar"
	"golem/internal/limbic"
	"golem/internal/memory"
	"golem/wire"
)

// The faculties are acts: recall, ground, consolidate, speak, propose,
// and introspect join the walk primitive in one operant space, each a
// setting the same arbiter can ignite. A quiet round fans every walk and
// every act out over one snapshot, chemistry and learned worth weigh
// them together, and when an act wins the loop performs the faculty the
// way a CLI verb would, on the loop, single-threaded. The CLI verbs stay
// as handles on the same functions; they are no longer the only way the
// faculties fire.
//
// Acts are quieter than thoughts by innate gain, each cools on its own
// refractory so the mind cannot fidget one faculty over and over, and
// every firing lands on the audit trail with what called for it. The
// operant loop covers acts exactly as it covers thoughts: an act that
// surfaced or changed something reinforces its operation node, a barren
// one wears it, and a taught word ("tidy" for the consolidate act) leans
// selection its way through workspace warmth.
const (
	actRecall      = "recall"
	actGround      = "ground"
	actConsolidate = "consolidate"
	actSpeak       = "speak"
	actPropose     = "propose"
	actIntrospect  = "introspect"

	// relAct is the mental-teach face of the act space: teach --mind
	// <word> act <name> grounds a word to a faculty the way a relation
	// and signal ground one to a walk.
	relAct = "act"
)

var actNames = []string{actRecall, actGround, actConsolidate, actSpeak, actPropose, actIntrospect}

// actOp is one faculty as an operation: the innate floor is the hand
// list of acts, and everything above (worth, naming, selection) is
// learned, the same stance the walk primitive takes.
type actOp struct {
	name string
}

func (op actOp) template() string { return "act(" + op.name + ")" }

// actNameOf parses an act template back to its faculty, refusing shapes
// outside the innate list.
func actNameOf(tpl string) (string, bool) {
	body, ok := strings.CutPrefix(tpl, "act(")
	if !ok || !strings.HasSuffix(body, ")") {
		return "", false
	}
	name := strings.TrimSuffix(body, ")")
	for _, n := range actNames {
		if n == name {
			return name, true
		}
	}
	return "", false
}

// actOps is the act half of the operation space, in a fixed order.
func actOps() []actOp {
	out := make([]actOp, 0, len(actNames))
	for _, n := range actNames {
		out = append(out, actOp{name: n})
	}
	return out
}

// actGains is the innate loudness prior for the acts, tuned under the
// walks: thinking stays the default of a quiet mind, and a faculty fires
// when its signal, the chemistry's appetite, a warm name, or its earned
// worth genuinely calls for it.
var actGains = map[string]float64{
	actRecall:      0.35,
	actGround:      0.40,
	actConsolidate: 0.60,
	actSpeak:       0.35,
	actPropose:     0.40,
	actIntrospect:  0.30,
}

// actAppetite is the chemistry's pull toward each faculty: alarm calls
// for relevant memory, reward chemistry for binding and proposing, sleep
// pressure for tidying, trust for speaking up, and calm for looking
// inward.
func actAppetite(chem limbic.State, name string) float64 {
	a := 1.0
	switch name {
	case actRecall:
		// Sub-linear in alarm, so full-blown stress still prefers walking
		// the stressful grounding itself over a broad retrieval.
		a = 0.4 + 1.2*chem.Noradrenaline
	case actGround, actPropose:
		a = 0.5 + 1.5*chem.Dopamine
	case actConsolidate:
		a = 0.4 + chem.Serotonin
	case actSpeak:
		a = 0.5 + chem.Oxytocin
	case actIntrospect:
		a = 1.0 + 0.6*(0.1-chem.Noradrenaline)
	}
	if a < 0.1 {
		a = 0.1
	}
	return a
}

// Per-act refractories: a faculty that just fired holds its tongue for a
// while, so the inner life stays a murmur rather than a tic.
var actCooldowns = map[string]time.Duration{
	actRecall:      30 * time.Second,
	actGround:      60 * time.Second,
	actConsolidate: 5 * time.Minute,
	actSpeak:       45 * time.Second,
	actPropose:     60 * time.Second,
	actIntrospect:  60 * time.Second,
}

// Clutter is the tidiness signal: fresh encodes since the last
// consolidation, scaled into [0, 1]. Below the minimum there is nothing
// worth an unprompted night.
const (
	actClutterMin   = 24
	actClutterScale = 96.0
)

func clutterOf(encodes int) float64 {
	c := float64(encodes) / actClutterScale
	if c > 1 {
		c = 1
	}
	return c
}

// actPanel lists the acts eligible this round: every faculty not still
// cooling from its last firing. The filter runs on the loop, where the
// refractory map lives.
func (d *Daemon) actPanel(now time.Time) []actOp {
	out := make([]actOp, 0, len(actNames))
	for _, op := range actOps() {
		if last, ok := d.lastAct[op.template()]; ok && now.Sub(last) < actCooldowns[op.name] {
			continue
		}
		out = append(out, op)
	}
	return out
}

// actPriority prices one act's claim on ignition with the same shape the
// walks use: measured signal, innate gain, chemical appetite, learned
// worth, and the warmth of any concept taught to name the act, times
// the bias of whichever drive is pushing it.
func actPriority(sn thoughtSnapshot, op actOp, sig float64) float64 {
	p := sig * actGains[op.name] * actAppetite(sn.chem, op.name) *
		sn.worth(op.template()) * (1 + opHeatGain*sn.opHeat[op.template()])
	if bias, ok := sn.driveBias[op.template()]; ok {
		p *= bias
	}
	return p
}

// actThought is one faculty's offer on the snapshot: how loudly the body
// state calls for it, and the target it would take. Pure computation; the
// performance happens on the loop only if the arbiter ignites it.
func actThought(sn thoughtSnapshot, op actOp) thoughtCandidate {
	none := thoughtCandidate{epoch: sn.epoch, op: op.template()}
	var sig float64
	arg := sn.seedGloss
	switch op.name {
	case actRecall:
		// Something is vividly present: retrieving its grounded context is
		// the relevance reflex, strongest when there is real experience
		// behind the seed.
		for _, dst := range sn.dests[memory.RelGrounds] {
			sig += dst.weight
		}
		if sig > 1 {
			sig = 1
		}
		// A held goal aims the act: the loop hunts the topic it wants to
		// learn, even when nothing warm grounds it.
		if sn.goalTopic != "" {
			arg = sn.goalTopic
			if sig < 0.5 {
				sig = 0.5
			}
		}
	case actGround:
		// Fresh experience accumulates ungrounded between nights; binding
		// the concept layer to it is worth more the more has piled up.
		sig = sn.clutter
	case actConsolidate:
		if sn.canConsolidate {
			sig = sn.clutter
		}
	case actSpeak:
		sig = sn.seedHeat
	case actPropose:
		// A runnable plan with the floor clear: staging it is the act;
		// approval stays with the operator.
		if sn.plan.ok && !sn.plan.conflict && !sn.pendingHeld {
			sig = sn.plan.saliency
			if sig > 1 {
				sig = 1
			}
			arg = sn.plan.command
		}
	case actIntrospect:
		sig = sn.introRecency
	}
	if sig <= 0 {
		return none
	}
	return thoughtCandidate{
		epoch: sn.epoch, op: op.template(), ok: true,
		content: "act: " + op.name,
		arg:     arg,
		why:     sn.driveWhy[op.template()],
		seed:    sn.seed, seedHeat: sn.seedHeat,
		strength: actPriority(sn, op, sig) * sn.seedHeat,
	}
}

// performAct fires one faculty from the loop, the same function the
// matching CLI verb or spoken intent calls, and leaves the firing on
// the audit trail: which act, what it took as its target, and what came
// of it. Fruit feeds the operant loop exactly as an insight feeds a
// walk.
func (d *Daemon) performAct(name string, winner thoughtCandidate) {
	var outcome string
	var fruitful bool
	switch name {
	case actRecall:
		_, outcome, fruitful = d.actRecallNow(winner.arg)
	case actGround:
		outcome, fruitful = d.actGroundNow()
	case actConsolidate:
		outcome, fruitful = d.actConsolidateNow()
	case actSpeak:
		line := d.remarkOn(winner.seed)
		if line == "" {
			outcome = "nothing to say"
			break
		}
		d.sayLine(line)
		outcome = line
	case actPropose:
		di, line := d.proposeFromSeed(winner.seed)
		if di == nil {
			outcome = "nothing to propose"
			break
		}
		d.sayLine(line)
		fruitful = true
		outcome = "staged " + di.Proposed
	case actIntrospect:
		line := d.introspectLine()
		d.sayLine(line)
		fruitful = d.lastThought != ""
		outcome = line
	}
	trigger := "unprompted"
	if winner.why != "" {
		// The drive that called for the act rides onto the trail with it.
		trigger += " (" + winner.why + ")"
	}
	d.finishAct(name, winner.arg, outcome, fruitful, trigger)
}

// finishAct is every firing's shared bookkeeping, however the act was
// triggered: the refractory stamp, the operant fruit, the event line,
// and the audit row that names the trigger.
func (d *Daemon) finishAct(name, target, outcome string, fruitful bool, trigger string) {
	tpl := actOp{name: name}.template()
	if d.lastAct == nil {
		d.lastAct = make(map[string]time.Time)
	}
	d.lastAct[tpl] = time.Now()
	d.noteOpFruit(tpl, fruitful, 0)
	d.event(events.Event{Ev: "act", Tpl: tpl, Cue: target, Outcome: trigger + ": " + outcome})
	if err := d.store.Audit(time.Now().Unix(), "act:"+name, target, trigger+": "+outcome); err != nil {
		fmt.Fprintf(d.out, "audit write failed: %v\n", err)
	}
	fmt.Fprintf(d.out, "      * act %s(%s): %s\n", name, target, outcome)
}

// actRecallNow is the recall faculty's body: the same seed path the
// recall verb walks, with reward and retrieval practice when memory
// serves. Utterances are excluded: the faculty retrieves what was
// lived, and an asking that surfaced its own words would chase its
// tail. The recall verb keeps the unfiltered view.
func (d *Daemon) actRecallNow(query string) (seeds []*memory.Fragment, outcome string, fruitful bool) {
	found, _, err := d.recallSeeds(query)
	if err != nil {
		return nil, "recall failed: " + err.Error(), false
	}
	for _, f := range found {
		if f.Channel != utteranceChannel {
			seeds = append(seeds, f)
		}
	}
	if len(seeds) > 0 {
		d.chem.SpikeDopamine(recallReward)
		if _, err := d.spread(seeds); err != nil {
			fmt.Fprintf(d.out, "act recall spread error: %v\n", err)
		}
	}
	return seeds, fmt.Sprintf("found %d moment(s)", len(seeds)), len(seeds) > 0
}

// actGroundNow binds the concept layer to whatever fresh experience
// matches it, without waiting for the night.
func (d *Daemon) actGroundNow() (outcome string, fruitful bool) {
	n, err := d.store.Reground(groundsWeight)
	if err != nil {
		return "grounding failed: " + err.Error(), false
	}
	return fmt.Sprintf("wove %d grounding(s)", n), n > 0
}

// actConsolidateNow runs the same night the sleep verb runs.
func (d *Daemon) actConsolidateNow() (outcome string, fruitful bool) {
	pruned, merged, born := d.sleepCycle()
	return fmt.Sprintf("pruned %d, merged %d, %d concept(s) born", pruned, merged, born),
		pruned+merged+born > 0
}

// sayLine carries an unprompted sentence to whoever is listening: the
// daemon log always, the voice when it is on.
func (d *Daemon) sayLine(line string) {
	if line == "" {
		return
	}
	fmt.Fprintf(d.out, "golem: %s\n", line)
	d.speak(line)
}

// remarkOn builds a grounded comment about a concept: what kind of thing
// it is and how much experience stands behind it, the same definitional
// shape a what-is answer takes. A nameless concept under high dopamine
// is asked about instead: the curiosity drive speaking.
func (d *Daemon) remarkOn(id uint64) string {
	frags, err := d.store.FragmentsByIDs([]uint64{id})
	if err != nil || len(frags) != 1 || frags[0].Kind != memory.KindConcept {
		return ""
	}
	c := frags[0]
	chem := d.chem.Snapshot()
	if names, err := d.store.LinksToRel([]uint64{c.ID}, memory.RelNames); err == nil &&
		len(names) == 0 && chem.Dopamine >= curiosityGate {
		return d.realizeAs("en", grammar.Frame{Force: grammar.ForceQuestion,
			Action:  grammar.Term{Phrase: "be"},
			Patient: grammar.Term{Phrase: wire.Clip(c.Content, spokenPhraseCap)}}, chem)
	}
	kind := "thing"
	if c.Category == "verb" {
		kind = "action"
	}
	parts := []string{d.realizeAs("en", grammar.Frame{Force: grammar.ForceStatement,
		Agent:  grammar.Term{Phrase: c.Content},
		Action: grammar.Term{Phrase: "be"}, Patient: grammar.Term{Phrase: grammar.Indefinite(kind)}}, chem)}
	if tally, err := d.store.GroundingTally(); err == nil && tally[c.ID] > 0 {
		parts = append(parts, d.realizeAs("en", grammar.Frame{Force: grammar.ForceStatement,
			Action:  grammar.Term{Phrase: "see"},
			Patient: grammar.Term{Phrase: "it"}, Times: tally[c.ID]}, chem))
	}
	return strings.Join(parts, " ")
}

// proposeFromSeed stages a proposal from a verb-like concept's grounded
// commands, behind the same pending slot and approval the propose verb
// uses. Nothing runs here; staging is the whole act.
func (d *Daemon) proposeFromSeed(id uint64) (*wire.DecisionInfo, string) {
	frags, err := d.store.FragmentsByIDs([]uint64{id})
	if err != nil || len(frags) != 1 || frags[0].Kind != memory.KindConcept {
		return nil, ""
	}
	dec, err := d.conceptDecision(frags[0])
	if err != nil || dec.Proposed == nil || dec.Conflict || d.pending != nil {
		return nil, ""
	}
	di := d.stageProposal(dec)
	line := d.realizeAs("en", grammar.Frame{Force: grammar.ForceStatement,
		Action:  grammar.Term{Phrase: "propose"},
		Patient: grammar.Term{Phrase: dec.Proposed.Command}}, d.chem.Snapshot())
	return di, line
}

// introspectLine reports the inner loop's own latest product, the same
// answer "what were you thinking" earns. The report and its idle form are
// memory constructions; the innate line is the fallback when none is
// seeded, so the engine still speaks if the construction is forgotten.
func (d *Daemon) introspectLine() string {
	if d.lastThought == "" {
		if line, ok := d.construct("en", "introspect_idle", nil); ok {
			return line
		}
		return "I have not been thinking about anything."
	}
	thought := strings.TrimPrefix(d.lastThought, "thought: ")
	if line, ok := d.construct("en", "introspect", map[string]string{"thought": thought}); ok {
		return line
	}
	return "I was thinking: " + thought
}

// construct realizes an intent through a memory construction, picking the
// variant the chemistry calls for. It returns false when no construction
// is seeded for the language, so every caller can fall back to the innate
// realizer and a tongue with no construction still speaks.
func (d *Daemon) construct(lang, intent string, slots map[string]string) (string, bool) {
	if lang == "" {
		lang = "en"
	}
	return d.gram.Construct(lang, intent, slots, d.chem.Snapshot())
}

// compose renders a layered message by recursive slot-filling, a sentence
// built from constructions that were each seeded on their own. The
// composition that built it lands on the trail, so the utterance reads
// back as the constructions that made it. It returns false when any layer
// has no construction, so the caller falls back to the innate realizer.
func (d *Daemon) compose(lang string, m grammar.Message) (string, bool) {
	if lang == "" {
		lang = "en"
	}
	line, trace, ok := d.gram.Compose(lang, m, d.chem.Snapshot())
	if ok && len(trace) > 0 {
		d.event(events.Event{Ev: "compose", Tpl: strings.Join(trace, " + ")})
	}
	return line, ok
}
