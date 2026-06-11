package main

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
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
	p, err := parsePipeline(cmd)
	if err != nil {
		return false, err.Error()
	}
	// The raw string is checked too, so a denylisted shape in a redirect
	// target ("> /dev/disk") is caught alongside the per-stage forms.
	if isDenylisted(cmd) {
		return false, "denylisted"
	}
	for _, argv := range p.stages {
		stage := stageString(argv)
		if isDenylisted(stage) {
			return false, "denylisted"
		}
		if !allowlisted(stage, d.cfg.ExecuteAllowlist) {
			if len(p.stages) == 1 {
				return false, "not allowlisted"
			}
			return false, "not allowlisted: " + stage
		}
	}
	return true, ""
}

// allowlisted matches a command against the operator's allowlist by exact
// command or whole-word prefix, so an entry cannot match a longer
// unrelated command ("go build" clears "go build ./..." but not
// "google-chrome").
func allowlisted(cmd string, allow []string) bool {
	c := strings.TrimSpace(strings.ToLower(cmd))
	for _, p := range allow {
		p = strings.TrimSpace(strings.ToLower(p))
		if p == "" {
			continue
		}
		if c == p || strings.HasPrefix(c, p+" ") {
			return true
		}
	}
	return false
}

// execTimeout resolves the configured per-command bound.
func (d *Daemon) execTimeout() time.Duration {
	t := time.Duration(d.cfg.ExecuteTimeoutSec * float64(time.Second))
	if t <= 0 {
		t = defaultExecuteTimeoutSec * time.Second
	}
	return t
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
	return runCommand(cmd, d.execTimeout())
}

// runCommand is the bare execution: no gates, no daemon state, safe to
// call from any goroutine. The caller owns policy. The command is parsed
// into its pipeline and the stages run with no shell anywhere.
func runCommand(cmd string, timeout time.Duration) (ran bool, ok bool, outcome string) {
	p, err := parsePipeline(cmd)
	if err != nil {
		// Policy parses before committing, so reaching this means the
		// command changed shape on the way in; treat it as a failed run
		// so a habit carrying it demotes.
		return true, false, "failed to parse: " + err.Error()
	}
	return runPipeline(p, timeout)
}

// runPipeline executes the stages wired stdout to stdin with real pipes,
// the way a shell would plumb them, minus the shell. Success means every
// stage exited zero: a pipeline is one action, and an action whose first
// half failed did not succeed, whatever its tail thought.
func runPipeline(p *pipeline, timeout time.Duration) (ran bool, ok bool, outcome string) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	n := len(p.stages)
	cmds := make([]*exec.Cmd, n)
	captures := make([]*bytes.Buffer, n)
	for i, argv := range p.stages {
		cmds[i] = exec.CommandContext(ctx, argv[0], argv[1:]...)
		captures[i] = &bytes.Buffer{}
		cmds[i].Stderr = captures[i]
	}

	var closeAfterStart []*os.File
	if p.inFile != "" {
		in, err := os.Open(p.inFile)
		if err != nil {
			return true, false, fmt.Sprintf("failed: %v", err)
		}
		defer in.Close()
		cmds[0].Stdin = in
	}
	var lastOut bytes.Buffer
	if p.outFile == "" {
		cmds[n-1].Stdout = &lastOut
	} else {
		flags := os.O_CREATE | os.O_WRONLY
		if p.appendOut {
			flags |= os.O_APPEND
		} else {
			flags |= os.O_TRUNC
		}
		out, err := os.OpenFile(p.outFile, flags, 0o644)
		if err != nil {
			return true, false, fmt.Sprintf("failed: %v", err)
		}
		defer out.Close()
		cmds[n-1].Stdout = out
	}
	for i := 0; i < n-1; i++ {
		r, w, err := os.Pipe()
		if err != nil {
			return true, false, fmt.Sprintf("failed: %v", err)
		}
		cmds[i].Stdout = w
		cmds[i+1].Stdin = r
		closeAfterStart = append(closeAfterStart, r, w)
	}

	started := 0
	var startErr error
	for i, c := range cmds {
		if err := c.Start(); err != nil {
			startErr = fmt.Errorf("stage %d (%s) failed: %v", i+1, c.Args[0], err)
			break
		}
		started++
	}
	// The parent's pipe copies must close once the children hold them, or
	// no stage would ever see EOF from its upstream neighbor.
	for _, f := range closeAfterStart {
		f.Close()
	}

	type verdict struct {
		err  error
		name string
	}
	verdicts := make([]verdict, 0, started)
	for i := 0; i < started; i++ {
		verdicts = append(verdicts, verdict{err: cmds[i].Wait(), name: cmds[i].Args[0]})
	}
	if startErr != nil {
		return true, false, startErr.Error()
	}

	var stderr strings.Builder
	for _, c := range captures {
		stderr.WriteString(c.String())
	}
	summary := firstLine(lastOut.String() + stderr.String())

	for i, v := range verdicts {
		if v.err == nil {
			continue
		}
		if ctx.Err() == context.DeadlineExceeded {
			return true, false, "timed out after " + timeout.String()
		}
		if n == 1 {
			if summary == "" {
				return true, false, fmt.Sprintf("failed: %v", v.err)
			}
			return true, false, fmt.Sprintf("failed (%v): %s", v.err, summary)
		}
		if summary == "" {
			return true, false, fmt.Sprintf("failed at stage %d (%s): %v", i+1, v.name, v.err)
		}
		return true, false, fmt.Sprintf("failed at stage %d (%s, %v): %s", i+1, v.name, v.err, summary)
	}
	if summary == "" {
		return true, true, "ok"
	}
	return true, true, "ok: " + summary
}

func firstLine(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		s = s[:i]
	}
	return clip(s, 120)
}

// feedback records a completed action back into memory so the loop closes:
// a routine success habituates and fades, while a failure stays surprising
// and becomes a moment GOLEM can recall. The outcome is reported on its
// own channel and never parses back into a runnable command.
func (d *Daemon) feedback(cmd, outcome string, at time.Time) {
	d.process(Observation{
		Channel: "action",
		Content: fmt.Sprintf("ran %s -> %s", cmd, outcome),
		At:      at,
	}, false, true)
}
