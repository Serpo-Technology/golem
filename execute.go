package main

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// Execution is the only place GOLEM can affect anything outside its own
// memory, so it is deliberately hard to reach. A command runs for real
// only when three independent gates all pass:
//
//  1. the master switch (cfg.ExecuteEnabled) is on,
//  2. the command is not on the destructive denylist, and
//  3. the command matches the operator's allowlist.
//
// Anything short of all three is a dry run. The trust model is narrow:
// only commands the operator themselves grounded (the shell channel is fed
// by their own commands or by golem teach) and then explicitly allowlisted
// can run, and the denylist still vetoes the unrecoverable shapes on top.
// Turning execution on means editing config.json and restarting; there is
// no command that enables it.

// shellControl is the set of characters that turn a command into a
// compound shell program: chaining, pipes, redirects, substitution. An
// allowlist entry matches a command by prefix, so without this gate
// allowlisting "echo" would clear "echo hi; <anything>", which sh -c would
// then run in full. Only simple commands (a program and its arguments) may
// execute; the allowlist constrains the whole command, not just its head.
const shellControl = ";|&<>`$()\n\\"

func hasShellControl(cmd string) bool {
	return strings.ContainsAny(cmd, shellControl)
}

// mayExecute reports whether a command is cleared to run, and if not, why,
// so the reason can be recorded in the audit trail.
func (d *Daemon) mayExecute(cmd string) (bool, string) {
	if !d.cfg.ExecuteEnabled {
		return false, "execution disabled"
	}
	if hasShellControl(cmd) {
		return false, "shell control characters"
	}
	if isDenylisted(cmd) {
		return false, "denylisted"
	}
	if !allowlisted(cmd, d.cfg.ExecuteAllowlist) {
		return false, "not allowlisted"
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
// call from any goroutine. The caller owns policy.
func runCommand(cmd string, timeout time.Duration) (ran bool, ok bool, outcome string) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	out, err := exec.CommandContext(ctx, "/bin/sh", "-c", cmd).CombinedOutput()
	summary := firstLine(string(out))
	// err drives the verdict: a command that returned cleanly succeeded
	// even if the deadline expired a moment later.
	if err == nil {
		if summary == "" {
			return true, true, "ok"
		}
		return true, true, "ok: " + summary
	}
	if ctx.Err() == context.DeadlineExceeded {
		return true, false, "timed out after " + timeout.String()
	}
	if summary == "" {
		return true, false, fmt.Sprintf("failed: %v", err)
	}
	return true, false, fmt.Sprintf("failed (%v): %s", err, summary)
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
