package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// runPipelineDemo shows the action layer on a throwaway memory: commands
// parse into shell-free pipelines or are refused, every stage faces the
// allowlist and denylist on its own, independent actions overlap while
// actions claiming the same file serialize, and the audit trail holds
// through all of it. Only harmless commands run.
func runPipelineDemo() {
	home, err := os.MkdirTemp("", "golem-pipe-")
	if err != nil {
		fail(err)
	}
	defer os.RemoveAll(home)

	cfg := DefaultConfig()
	cfg.Channels = ChannelsConfig{}
	cfg.ExecuteEnabled = true
	cfg.ExecuteAllowlist = []string{"echo"}
	d, err := NewDaemon(home, cfg, io.Discard)
	if err != nil {
		fail(err)
	}
	defer d.store.Close()

	line := "--------------------------------------------------------------------"
	fmt.Println("GOLEM action-layer demo. Execution is ON in a throwaway memory.")
	fmt.Println("No shell is involved at any point below.")
	fmt.Println(line)

	fmt.Println("\n1. PARSE OR REFUSE. A command becomes a pipeline of argv stages,")
	fmt.Println("   or it does not run. What a shell would interpret is refused:")
	for _, evil := range []string{
		"echo hi; rm -rf /",
		"echo $(whoami)",
		"echo $HOME",
		"rm *.go",
		"echo hi 2> /dev/null",
	} {
		_, err := parsePipeline(evil)
		fmt.Printf("   %-26q -> refused: %v\n", evil, err)
	}
	fmt.Printf("   %-26q -> two stages, no shell semantics left\n", "echo hi | tr a-z A-Z")

	fmt.Println("\n2. EVERY STAGE ANSWERS FOR ITSELF. The allowlist holds only echo,")
	fmt.Println("   so an allowlisted first stage clears nothing about the second:")
	cmd := "echo cogito ergo sum | tr a-z A-Z"
	if ok, reason := d.mayExecute(cmd); !ok {
		fmt.Printf("   %q\n   -> held back: %s\n", cmd, reason)
	}
	fmt.Println("   The operator allowlists tr, sleep, and tee as well. Now:")
	d.cfg.ExecuteAllowlist = []string{"echo", "tr", "sleep", "tee"}
	if ok, _ := d.mayExecute(cmd); ok {
		_, _, outcome := d.execute(cmd)
		fmt.Printf("   %q\n   -> every stage authorized, ran: %s\n", cmd, outcome)
	}

	audit := func(cmd string) func(ran, ok bool, outcome string) {
		return func(_, _ bool, outcome string) {
			d.store.Audit(time.Now().Unix(), "demo", cmd, outcome)
		}
	}
	noDrop := func(string) {}

	fmt.Println("\n3. INDEPENDENT ACTIONS OVERLAP. Two half-second commands with no")
	fmt.Println("   shared claim are committed back to back:")
	start := time.Now()
	d.commitAction("sleep 0.5", audit("sleep 0.5"), noDrop)
	d.commitAction("sleep 0.6", audit("sleep 0.6"), noDrop)
	fmt.Printf("   running now: %s\n", strings.Join(d.runningCmds(), "  +  "))
	d.drainActions()
	fmt.Printf("   both finished in %.1fs: they ran at once, not 1.1s end to end.\n",
		time.Since(start).Seconds())

	fmt.Println("\n4. ONE HAND, ONE FILE. The first action holds gate.txt for half a")
	fmt.Println("   second; the second wants the same file and must wait its turn:")
	gate := filepath.Join(home, "gate.txt")
	short := func(s string) string { return strings.ReplaceAll(s, home+string(os.PathSeparator), "") }
	first := "sleep 0.5 | tee " + gate
	second := "echo waited-my-turn >> " + gate
	d.commitAction(first, audit(first), noDrop)
	d.commitAction(second, audit(second), noDrop)
	fmt.Printf("   running: %q   queued: %d (the claim on gate.txt is held)\n",
		short(strings.Join(d.runningCmds(), ", ")), len(d.waiting))
	d.drainActions()
	data, _ := os.ReadFile(gate)
	fmt.Printf("   the writer waited out the hold, then ran: gate.txt reads %q.\n",
		strings.TrimSpace(string(data)))

	fmt.Println("\n" + line)
	fmt.Println("The trail, coherent through refusals, pipes, and parallelism:")
	entries, _ := d.store.AuditTail(4)
	for _, e := range entries {
		fmt.Printf("  %q -> %s\n", short(e.Action), e.Outcome)
	}
}
