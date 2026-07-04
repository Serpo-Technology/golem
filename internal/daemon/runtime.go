package daemon

import (
	"context"
	"fmt"
	"time"

	"golem/internal/attention"
	"golem/internal/events"
	"golem/internal/memory"
	"golem/internal/runtime"
	"golem/internal/senses"
	"golem/wire"
)

// The code runtime is GOLEM reaching for the machine's own strength: it writes
// a program, runs it under a budget, and grounds the result the way it grounds
// a percept. The runner is held behind the runtime.Runner seam, an interpreter
// today and a compiling toolchain when GOLEM comes to rebuild itself, so the
// caller does not change. Running code GOLEM wrote is gated shut by default and
// opens only inside the operator's container; the bounding of a single program
// is the runtime's, the sandbox around the whole process the operator's.

// runtimeBudget bounds a single program when no execute timeout is configured.
const runtimeBudget = 5 * time.Second

// runProgram runs a program GOLEM composed, grounds its result, and leaves the
// program as a readable artifact in the graph and on the trail, returning the
// artifact's id so a fact grounded on the run can carry its provenance. It
// refuses while the gate is shut, so the capability exists but waits for the
// container.
func (d *Daemon) runProgram(source string) (runtime.Result, uint64) {
	if !d.cfg.RuntimeEnabled {
		return runtime.Result{Err: "the code runtime is closed; the operator opens it inside the sandbox"}, 0
	}
	runner := d.runner
	if runner == nil {
		runner = runtime.Interp{}
	}
	budget := time.Duration(d.cfg.ExecuteTimeoutSec * float64(time.Second))
	if budget <= 0 {
		budget = runtimeBudget
	}
	ctx, cancel := context.WithTimeout(context.Background(), budget)
	defer cancel()
	res := runner.Run(ctx, source)

	now := time.Now()
	// The program is an artifact, as readable as a thought: its source held in
	// the graph and the run recorded on the trail.
	artifact := &memory.Fragment{Kind: memory.KindEpisodic, Channel: "code",
		Content: source, Template: attention.Normalize(source), Time: now}
	if _, err := d.store.Encode(artifact); err != nil {
		fmt.Fprintf(d.out, "runtime artifact error: %v\n", err)
	}
	d.event(events.Event{Ev: "run", Outcome: runSummary(res)})
	if err := d.store.Audit(now.Unix(), "run", wire.Clip(source, 120), runSummary(res)); err != nil {
		fmt.Fprintf(d.out, "audit write failed: %v\n", err)
	}

	// The result is a percept it can ground: feed it back through perception so
	// what a program found enters memory the way the world does.
	if percept := runPercept(res); percept != "" {
		d.process(senses.Observation{Channel: "code", Content: percept, At: now}, true, true)
	}
	return res, artifact.ID
}

// runSummary renders a run's outcome for the trail and the reply: the error
// when it failed, the value with its output, or the output alone.
func runSummary(res runtime.Result) string {
	switch {
	case res.Err != "":
		return "error: " + res.Err
	case res.Value != "" && res.Output != "":
		return res.Output + " => " + res.Value
	case res.Value != "":
		return res.Value
	case res.Output != "":
		return res.Output
	default:
		return "no result"
	}
}

// runPercept is the result a run grounds: the value it evaluated to, or what it
// printed, the thing worth remembering about what the program found.
func runPercept(res runtime.Result) string {
	if res.Value != "" {
		return res.Value
	}
	return res.Output
}
