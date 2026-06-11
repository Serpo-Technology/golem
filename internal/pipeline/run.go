package pipeline

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

// RunCommand is the bare execution: no gates, no daemon state, safe to
// call from any goroutine. The caller owns policy. The command is parsed
// into its pipeline and the stages run with no shell anywhere.
func RunCommand(cmd string, timeout time.Duration) (ran bool, ok bool, outcome string) {
	p, err := Parse(cmd)
	if err != nil {
		// Policy parses before committing, so reaching this means the
		// command changed shape on the way in; treat it as a failed run
		// so a habit carrying it demotes.
		return true, false, "failed to parse: " + err.Error()
	}
	return Run(p, timeout)
}

// Run executes the stages wired stdout to stdin with real pipes,
// the way a shell would plumb them, minus the shell. Success means every
// stage exited zero: a pipeline is one action, and an action whose first
// half failed did not succeed, whatever its tail thought.
func Run(p *Pipeline, timeout time.Duration) (ran bool, ok bool, outcome string) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	n := len(p.Stages)
	cmds := make([]*exec.Cmd, n)
	captures := make([]*bytes.Buffer, n)
	for i, argv := range p.Stages {
		cmds[i] = exec.CommandContext(ctx, argv[0], argv[1:]...)
		captures[i] = &bytes.Buffer{}
		cmds[i].Stderr = captures[i]
	}

	var closeAfterStart []*os.File
	if p.InFile != "" {
		in, err := os.Open(p.InFile)
		if err != nil {
			return true, false, fmt.Sprintf("failed: %v", err)
		}
		defer in.Close()
		cmds[0].Stdin = in
	}
	var lastOut bytes.Buffer
	if p.OutFile == "" {
		cmds[n-1].Stdout = &lastOut
	} else {
		flags := os.O_CREATE | os.O_WRONLY
		if p.AppendOut {
			flags |= os.O_APPEND
		} else {
			flags |= os.O_TRUNC
		}
		out, err := os.OpenFile(p.OutFile, flags, 0o644)
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
	if r := []rune(s); len(r) > 120 {
		return string(r[:120]) + "..."
	}
	return s
}

// ClaimsOf derives a command's resource claims. Policy has already
// parsed the command by the time anything commits it, so a parse error
// here just means no claims to hold.
func ClaimsOf(cmd string) []string {
	p, err := Parse(cmd)
	if err != nil {
		return nil
	}
	return p.Claims()
}
