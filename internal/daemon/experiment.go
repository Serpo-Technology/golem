package daemon

import (
	"fmt"
	"time"

	"golem/internal/code"
	"golem/internal/events"
	"golem/internal/grammar"
)

// An experiment is thought reaching for the machine on its own account: with
// no computation owed, a curious idle mind picks a conjecture about its own
// learned functions (are these two each other's undoing?), composes the
// nested programs that test it, runs them over probe inputs, and holds what
// it finds as a belief. The epistemics are honest induction: each run grounds
// its own case firmly, and the rule over all cases is held as a probable
// belief that entrenches the way any belief does, by being drawn again. A
// conjecture the runs refute is put down on record for the wake, so the mind
// moves on rather than worrying one pair.

const (
	// experimentConf is the confidence a passed conjecture is held at: a rule
	// standing on a handful of cases is a belief, never a grounded fact.
	experimentConf = 0.6
	// experimentRel is the relation an inverse conjecture is held under.
	experimentRel = "inverse"
)

// experimentInputs are the probe values a conjecture is tried over: small
// even numbers, so an integer-halving pair is not refuted by truncation
// before the rule has a chance to show itself. Widening the probe set is the
// natural refinement when experiments grow past inverses.
var experimentInputs = []string{"2", "4", "6"}

// unaryFunctions lists the learned single-parameter integer functions, the
// inventory conjectures range over. The binary primitives and the list
// functions stay out: an inverse conjecture is about one value returned to
// itself.
func (d *Daemon) unaryFunctions() []code.Op {
	if d.code == nil {
		return nil
	}
	var out []code.Op
	for _, op := range d.code.Ops() {
		if len(op.Params) != 1 || op.Body == nil {
			continue
		}
		if len(op.ParamTypes) > 0 && op.ParamTypes[0] != "" && op.ParamTypes[0] != "int" {
			continue
		}
		out = append(out, op)
	}
	return out
}

// nextConjecture picks the first ordered pair of learned functions this wake
// has not yet tried: f undoing g. ok is false when nothing is left to wonder
// about, so the act stays quiet.
func (d *Daemon) nextConjecture() (f, g code.Op, ok bool) {
	fns := d.unaryFunctions()
	for _, a := range fns {
		for _, b := range fns {
			if a.Name == b.Name {
				continue
			}
			if d.triedConjectures[a.Name+"|"+b.Name] {
				continue
			}
			return a, b, true
		}
	}
	return code.Op{}, code.Op{}, false
}

// hasConjecture reports whether an untried conjecture stands, the cheap check
// the think snapshot reads.
func (d *Daemon) hasConjecture() bool {
	_, _, ok := d.nextConjecture()
	return ok
}

// runExperiment tests one conjecture by running: for each probe input, the
// composed program f(g(x)), the pair holding when every output returns its
// input. Each run is the ordinary grounded run, budgeted and auditable, its
// program an artifact like any other.
func (d *Daemon) runExperiment(f, g code.Op) (holds bool, tried int) {
	for _, x := range experimentInputs {
		if d.computeRuns >= computeRunsPerWake {
			return false, tried
		}
		comp := code.Computation{Op: f.Name, Args: []code.Computation{
			{Op: g.Name, Args: []code.Computation{{Lit: x}}}}}
		src, ok := d.generateCode(comp, nil, 0)
		if !ok {
			return false, tried
		}
		res, _ := d.runProgram(src)
		d.computeRuns++
		tried++
		if res.Err != "" || res.Value != x {
			return false, tried
		}
	}
	return tried == len(experimentInputs), tried
}

// actExperimentNow is the compute faculty's idle mode: it takes the next
// untried conjecture, tests it by running, and holds a passed rule as a
// belief, spoken in the property register with the hedge finite evidence
// earns. A refuted pair is noted for the wake and the outcome audited either
// way, so the trail reads what was wondered and what the runs said.
func (d *Daemon) actExperimentNow() (string, bool) {
	f, g, ok := d.nextConjecture()
	if !ok {
		return "nothing to wonder about", false
	}
	if d.triedConjectures == nil {
		d.triedConjectures = map[string]bool{}
	}
	d.triedConjectures[f.Name+"|"+g.Name] = true
	holds, tried := d.runExperiment(f, g)
	what := experimentRel + "(" + f.Name + ", " + g.Name + ")"
	if !holds {
		outcome := fmt.Sprintf("refuted after %d run(s)", tried)
		d.event(events.Event{Ev: "experiment", Cue: what, Outcome: outcome})
		if err := d.store.Audit(time.Now().Unix(), "experiment", what, outcome); err != nil {
			fmt.Fprintf(d.out, "audit write failed: %v\n", err)
		}
		return what + " " + outcome, true
	}
	fc, err1 := d.store.ConceptByTemplate("op:" + f.Name)
	gc, err2 := d.store.ConceptByTemplate("op:" + g.Name)
	if err1 != nil || err2 != nil || fc == nil || gc == nil {
		return "the functions have no concepts to hold the rule on", false
	}
	d.held.add(heldProp{rel: experimentRel, agent: fc.ID, patient: gc.ID,
		agentGloss: f.Name, patientGloss: g.Name,
		status: heldInferred, conf: experimentConf, voiced: true})
	outcome := fmt.Sprintf("held over %d runs", tried)
	d.event(events.Event{Ev: "experiment", Cue: what, Outcome: outcome})
	if err := d.store.Audit(time.Now().Unix(), "experiment", what, outcome); err != nil {
		fmt.Fprintf(d.out, "audit write failed: %v\n", err)
	}
	clause := d.realizeAs("en", grammar.Frame{Force: grammar.ForceStatement,
		Agent:  grammar.Term{Phrase: "the " + f.Name},
		Action: grammar.Term{Phrase: "be"},
		Patient: grammar.Term{
			Phrase: "the " + experimentRel + " of the " + g.Name}}, d.chem.Snapshot())
	line := upperFirst(d.gram.Word("en", "probably")) + ", " + lowerFirst(clause)
	d.sayLine(line)
	d.chem.SpikeDopamine(goalReward)
	return line, true
}
