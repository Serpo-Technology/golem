package daemon

import (
	"context"
	"os"
	"path/filepath"
	"time"

	"golem/internal/events"
	"golem/internal/selfbuild"
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

// proposeSelfChange runs a proposed change to GOLEM's own source through the
// pipeline and the fitness gate. It is gated twice, the runtime open and a
// source path set, and never touches the original tree or the live binary: it
// builds and tests on a copy, and on a passing fitness verdict leaves a
// candidate staged for the operator to deploy. A change that fails to compile,
// to test, or to improve is rejected and the prior stands, the whole event on
// the trail.
func (d *Daemon) proposeSelfChange(ch selfbuild.Change, fitness func() fitnessVerdict) selfbuild.Result {
	if !d.cfg.RuntimeEnabled || d.cfg.SourcePath == "" {
		return selfbuild.Result{Stage: "gate",
			Err: "self-modification is closed; set runtime_enabled and source_path inside the sandbox"}
	}
	stage := filepath.Join(d.home, "stage")
	if err := os.MkdirAll(stage, 0o755); err != nil {
		return selfbuild.Result{Stage: "stage", Err: err.Error()}
	}
	main := d.cfg.MainPackage
	if main == "" {
		main = "./cmd/golem"
	}
	ctx, cancel := context.WithTimeout(context.Background(), selfBuildBudget)
	defer cancel()

	res := selfbuild.Build(ctx, d.cfg.SourcePath, stage, main, "golem-candidate", ch)
	d.event(events.Event{Ev: "selfmod", Tpl: ch.File, Outcome: res.Stage})
	d.auditSelfmod(ch, res.Stage, res.Err)
	if !res.Accepted {
		return res // a compile or test failure: the prior binary stands
	}

	// The candidate built and tested clean; fitness judges whether it improves
	// a grounded outcome. A change that does not is reverted by discarding the
	// candidate, so a valid-but-pointless change never reaches deploy.
	v := fitness()
	if !keepOrRevert(v, func() { os.Remove(res.Candidate) }) {
		d.auditSelfmod(ch, "fitness", v.reason)
		return selfbuild.Result{Stage: "fitness", Err: v.reason}
	}
	d.auditSelfmod(ch, "staged", "candidate awaiting operator deploy: "+v.reason)
	return res
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
