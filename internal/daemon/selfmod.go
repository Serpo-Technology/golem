package daemon

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"golem/internal/code"
	"golem/internal/events"
	"golem/internal/selfbuild"
	"golem/wire"
)

// Self-modification is the keystone of the self-extending GOLEM, and it earns
// its place only on the steps under it: a runtime to run code, a fitness to
// judge a change, and the source to change. GOLEM proposes a change to its own
// source and runs it through the build-and-deploy pipeline already used by hand,
// never a live patch: compile, test, fitness gate, then a candidate staged for
// deploy, with the prior binary the floor and every step on the trail. The live
// swap is the operator's, behind say-so; the pipeline stops at a vetted
// candidate sitting off to the side.

// selfBuildBudget bounds a self-build generously: compiling and testing a whole
// tree is not a snippet, and the pipeline runs off the loop.
const selfBuildBudget = 5 * time.Minute

// selfModShut is the refusal both entries give while the gates are down: it
// takes the runtime open and a source path set, two opt-ins, to build.
const selfModShut = "self-modification is closed; set runtime_enabled and source_path inside the sandbox"

// selfChangeGrounds checks the gates and prepares the staging ground, handing
// back the paths a build needs. The refusal carries the stage it died at, so
// the in-line and the off-loop entries decline the same way.
func (d *Daemon) selfChangeGrounds() (src, stage, main string, refuse selfbuild.Result) {
	if !d.cfg.RuntimeEnabled || d.cfg.SourcePath == "" {
		return "", "", "", selfbuild.Result{Stage: "gate", Err: selfModShut}
	}
	stage = filepath.Join(d.home, "stage")
	if err := os.MkdirAll(stage, 0o755); err != nil {
		return "", "", "", selfbuild.Result{Stage: "stage", Err: err.Error()}
	}
	main = d.cfg.MainPackage
	if main == "" {
		main = "./cmd/golem"
	}
	return d.cfg.SourcePath, stage, main, selfbuild.Result{}
}

// settleSelfChange absorbs a finished build single-threaded: the trail event,
// the audit, and the fitness gate all touch daemon state, so the loop runs
// them whether the build ran in line or off to the side. The candidate built
// and tested clean before fitness judges whether it improves a grounded
// outcome; a change that does not is reverted by discarding the candidate, so
// a valid-but-pointless change never reaches deploy.
func (d *Daemon) settleSelfChange(ch selfbuild.Change, res selfbuild.Result,
	fitness func() fitnessVerdict) selfbuild.Result {
	d.event(events.Event{Ev: "selfmod", Tpl: ch.File, Outcome: res.Stage})
	d.auditSelfmod(ch, res.Stage, res.Err)
	if !res.Accepted {
		return res // a compile or test failure: the prior binary stands
	}
	v := fitness()
	if !keepOrRevert(v, func() { os.Remove(res.Candidate) }) {
		d.auditSelfmod(ch, "fitness", v.reason)
		return selfbuild.Result{Stage: "fitness", Err: v.reason}
	}
	d.auditSelfmod(ch, "staged", "candidate awaiting operator deploy: "+v.reason)
	return res
}

// proposeSelfChange runs a proposed change to GOLEM's own source through the
// pipeline and the fitness gate, in line: the caller waits out the build. It
// is gated twice, the runtime open and a source path set, and never touches
// the original tree or the live binary: it builds and tests on a copy, and on
// a passing fitness verdict leaves a candidate staged for the operator to
// deploy. A change that fails to compile, to test, or to improve is rejected
// and the prior stands, the whole event on the trail.
func (d *Daemon) proposeSelfChange(ch selfbuild.Change, fitness func() fitnessVerdict) selfbuild.Result {
	src, stage, main, refuse := d.selfChangeGrounds()
	if refuse.Err != "" {
		return refuse
	}
	ctx, cancel := context.WithTimeout(context.Background(), selfBuildBudget)
	defer cancel()
	res := selfbuild.Build(ctx, src, stage, main, operatorCandidate, ch)
	return d.settleSelfChange(ch, res, fitness)
}

// operatorCandidate is the staged binary an operator's retune produces; the
// autonomous review stages under its own name so the two never overwrite each
// other's vetted candidate at the shared stage path.
const operatorCandidate = "golem-candidate"

// beginSelfChange runs the same pipeline off the brain loop. The gates and the
// staging ground are checked here for an immediate refusal; the copy, build,
// and test run in a goroutine that touches no daemon state; and the outcome
// marshals back through execDone, where the loop settles it single-threaded
// and hands the verdict to done. The build holds a run slot while it works,
// and only one self-build runs at a time: the toolchain is too heavy to race
// against itself. binName sets which staged candidate the build produces, so
// distinct producers keep distinct candidates. The build context cancels when
// the daemon stops, so a shutdown is not held for the length of a compile.
func (d *Daemon) beginSelfChange(ch selfbuild.Change, binName string,
	fitness func() fitnessVerdict, done func(selfbuild.Result)) (refusal string) {
	src, stage, main, refuse := d.selfChangeGrounds()
	if refuse.Err != "" {
		return refuse.Err
	}
	if d.selfBuildRunning() {
		return "a self-change is already building; its outcome lands on the trail first"
	}
	d.nextRunID++
	id := d.nextRunID
	d.running[id] = &actionRun{cmd: "selfbuild " + ch.File}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), selfBuildBudget)
		defer cancel()
		go func() {
			select {
			case <-d.stop:
				cancel()
			case <-ctx.Done():
			}
		}()
		res := selfbuild.Build(ctx, src, stage, main, binName, ch)
		d.execDone <- execResult{id: id, ran: true, ok: res.Accepted, outcome: res.Stage,
			finish: func(bool, bool, string) { done(d.settleSelfChange(ch, res, fitness)) }}
	}()
	return ""
}

// handleRetune is the operator's conversational entry to the retune pipeline:
// golem retune <file> <constant> <value>. The gates and the constant reading
// stay on the loop, so a shut gate, an absent constant, or a build already
// running is refused in the reply; the build and test run beside the mind,
// which keeps sensing and answering, and the verdict is spoken and audited
// when it lands. The fitness for an operator-asked change is the operator's
// judgment at deploy time, so a candidate that built and tested clean is kept
// staged.
func (d *Daemon) handleRetune(req wire.Request) wire.Response {
	// Only the architect's signature authorizes a retune, the same authority
	// the trust verb demands: a change to the source is the architect's ask,
	// and over an open transport no peer may stage one by asking nicely.
	if !d.reqPrincipal.architectSigned {
		return wire.Response{Error: "retuning the source needs the architect's signature"}
	}
	file := strings.TrimSpace(req.Query)
	name := strings.TrimSpace(req.Arg)
	value := strings.TrimSpace(req.Content)
	if file == "" || name == "" || value == "" {
		return wire.Response{Error: "retune needs a file, a constant, and a value"}
	}
	why := "the architect asked"
	if d.reqPrincipal.sigFingerprint != "" {
		why += " (signed " + d.reqPrincipal.sigFingerprint + ")"
	}
	ch, refuse := d.retuneChange(file, name, value, why)
	if refuse.Err != "" {
		return wire.Response{OK: true, Say: "retune " + refuse.Stage + ": " + refuse.Err}
	}
	fitness := func() fitnessVerdict {
		return fitnessVerdict{improved: true,
			reason: "the architect asked; the deploy stays the architect's"}
	}
	asked := "the retune of " + name + " to " + value
	if busy := d.beginSelfChange(ch, operatorCandidate, fitness, func(res selfbuild.Result) {
		if res.Err != "" {
			d.sayLine(asked + " died at " + res.Stage + ": " + res.Err)
			return
		}
		d.sayLine(asked + " built and tested clean. The candidate waits at " +
			res.Candidate + " for the architect to deploy.")
	}); busy != "" {
		return wire.Response{OK: true, Say: "retune: " + busy}
	}
	return wire.Response{OK: true, Say: "I am building and testing a candidate with " + name +
		" retuned to " + value + ". I stay awake while it runs and will say the outcome."}
}

// retuneChange reads a tuning constant out of the source and shapes the edit:
// an AST-located splice of that one declaration, so the change comes from
// understanding what is there rather than a transform handed in. A shut gate
// or a constant that is not there is declined at the reading, before any
// build.
func (d *Daemon) retuneChange(file, name, newValue, why string) (selfbuild.Change, selfbuild.Result) {
	if !d.cfg.RuntimeEnabled || d.cfg.SourcePath == "" {
		return selfbuild.Change{}, selfbuild.Result{Stage: "gate", Err: selfModShut}
	}
	// The file is a path inside the source tree, nothing else: a name that
	// climbs out is refused at the reading, so no ask can splice a file the
	// sandbox does not own.
	root := filepath.Clean(d.cfg.SourcePath)
	full := filepath.Join(d.cfg.SourcePath, file)
	if full != root && !strings.HasPrefix(full, root+string(filepath.Separator)) {
		return selfbuild.Change{}, selfbuild.Result{Stage: "read",
			Err: "the file must live inside the source tree"}
	}
	src, err := os.ReadFile(full)
	if err != nil {
		return selfbuild.Change{}, selfbuild.Result{Stage: "read", Err: err.Error()}
	}
	old, err := code.ConstValue(string(src), name)
	if err != nil {
		return selfbuild.Change{}, selfbuild.Result{Stage: "read", Err: err.Error()}
	}
	return selfbuild.Change{
		File:  file,
		Why:   fmt.Sprintf("%s: retune %s from %s to %s", why, name, old, newValue),
		Apply: func(s string) (string, error) { return code.SetConst(s, name, newValue) },
	}, selfbuild.Result{}
}

// proposeRetune is the in-line twin: the same reading and the same pipeline,
// with the caller waiting out the build.
func (d *Daemon) proposeRetune(file, name, newValue, why string, fitness func() fitnessVerdict) selfbuild.Result {
	ch, refuse := d.retuneChange(file, name, newValue, why)
	if refuse.Err != "" {
		return refuse
	}
	return d.proposeSelfChange(ch, fitness)
}

// selfBuildRunning reports whether a self-change currently holds the build
// slot, so a second one waits its turn.
func (d *Daemon) selfBuildRunning() bool {
	for _, run := range d.running {
		if strings.HasPrefix(run.cmd, "selfbuild ") {
			return true
		}
	}
	return false
}

// auditSelfmod records one stage of a self-change on the trail, so the system's
// growth is as readable as its thought, change by change.
func (d *Daemon) auditSelfmod(ch selfbuild.Change, stage, detail string) {
	note := ch.Why + " -> " + stage
	if detail != "" {
		note += ": " + detail
	}
	if err := d.store.Audit(time.Now().Unix(), "selfmod", ch.File, note); err != nil {
		return
	}
}
