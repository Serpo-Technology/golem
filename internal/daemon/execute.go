package daemon

import (
	"fmt"
	"time"

	"golem/internal/config"
	"golem/internal/pipeline"
	"golem/internal/senses"
)

// Execution is the only place GOLEM can affect anything outside its own
// memory, so it is deliberately hard to reach. A command runs for real
// only when three independent gates all pass:
//
//  1. the master switch (cfg.ExecuteEnabled) is on,
//  2. no part of the command is on the destructive denylist, and
//  3. every stage of the command matches the operator's allowlist.
//
// Anything short of all three is a dry run. The trust model is narrow:
// only commands the operator themselves grounded (the shell channel is fed
// by their own commands or by golem teach) and then explicitly allowlisted
// can run, and the denylist still vetoes the unrecoverable shapes on top.
// Turning execution on means editing config.json and restarting; there is
// no command that enables it.
//
// No shell sits behind any of it. A command is parsed into a pipeline of
// argv stages or refused (pipeline.go), each stage is authorized on its
// own, and the stages run wired together with Go's piping, so a prefix
// match on one program can never clear a compound program.

// mayExecute reports whether a command is cleared to run, and if not, why,
// so the reason can be recorded in the audit trail.
func (d *Daemon) mayExecute(cmd string) (bool, string) {
	if !d.cfg.ExecuteEnabled {
		return false, "execution disabled"
	}
	p, err := pipeline.Parse(cmd)
	if err != nil {
		return false, err.Error()
	}
	// The raw string is checked too, so a denylisted shape in a redirect
	// target ("> /dev/disk") is caught alongside the per-stage forms.
	if config.Denylisted(cmd) {
		return false, "denylisted"
	}
	for _, argv := range p.Stages {
		stage := pipeline.StageString(argv)
		if config.Denylisted(stage) {
			return false, "denylisted"
		}
		if !config.Allowlisted(stage, d.cfg.ExecuteAllowlist) {
			if len(p.Stages) == 1 {
				return false, "not allowlisted"
			}
			return false, "not allowlisted: " + stage
		}
	}
	return true, ""
}

// execTimeout resolves the configured per-command bound.
func (d *Daemon) execTimeout() time.Duration {
	return d.cfg.ExecTimeout()
}

// dryRun describes a command that policy held back, in the form the audit
// trail records.
func dryRun(reason, cmd string) string {
	return fmt.Sprintf("[dry-run: %s] %s", reason, cmd)
}

// execute runs a command if policy allows, or describes the dry run if not.
// It reports whether the command actually ran and whether it succeeded
// (exit zero); a dry run reports ran=false, ok=true. This synchronous form
// serves the demos and tests; the daemon's request path runs the same
// runCommand off the brain loop.
func (d *Daemon) execute(cmd string) (ran bool, ok bool, outcome string) {
	allowed, reason := d.mayExecute(cmd)
	if !allowed {
		return false, true, dryRun(reason, cmd)
	}
	return pipeline.RunCommand(cmd, d.execTimeout())
}

// feedback records a completed action back into memory so the loop closes:
// a routine success habituates and fades, while a failure stays surprising
// and becomes a moment GOLEM can recall. The outcome is reported on its
// own channel and never parses back into a runnable command.
func (d *Daemon) feedback(cmd, outcome string, at time.Time) {
	d.process(senses.Observation{
		Channel: "action",
		Content: fmt.Sprintf("ran %s -> %s", cmd, outcome),
		At:      at,
	}, false, true)
}
