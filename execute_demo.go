package main

import (
	"fmt"
	"io"
	"os"
	"time"
)

// runExecutionDemo shows real execution under its three gates, using
// harmless commands in a throwaway memory: a success that graduates and
// then runs autonomously, a failure that demotes, and a denylisted command
// that stays a dry run even though it is allowlisted.
func runExecutionDemo() {
	home, err := os.MkdirTemp("", "golem-exec-")
	if err != nil {
		fail(err)
	}
	defer os.RemoveAll(home)

	cfg := DefaultConfig()
	cfg.Channels = ChannelsConfig{}
	cfg.ExecuteEnabled = true
	cfg.ExecuteAllowlist = []string{"echo", "false", "rm -rf"} // rm -rf is allowlisted on purpose
	d, err := NewDaemon(home, cfg, io.Discard)
	if err != nil {
		fail(err)
	}
	defer d.store.Close()

	base := time.Now().Add(-100 * time.Hour)
	line := "--------------------------------------------------------------------"
	fmt.Println("GOLEM real-execution demo. Execution is ON in a throwaway memory,")
	fmt.Println("with echo, false, and rm -rf allowlisted. Watch the gates.")
	fmt.Println(line)

	// A succeeding command graduates, then runs autonomously.
	fmt.Println("\n1. A real, allowed command. Teaching \"greet\" -> echo, then approving.")
	for i := 0; i < 4; i++ {
		d.teach("greet the world", "echo hello-from-golem", base.Add(time.Duration(i)*time.Hour))
	}
	for i := 0; i < cfg.GraduationApprovals; i++ {
		dec, _ := d.decide("greet the world")
		_, outcome, _ := d.approve(dec, base.Add(time.Duration(10+i)*time.Hour))
		fmt.Printf("   approve #%d -> %s\n", i+1, outcome)
	}
	fmt.Println("   Now graduated. Proposing it runs it autonomously, no approval:")
	resp, _ := d.handle(Request{Cmd: "propose", Content: "greet the world"})
	fmt.Printf("   propose -> autonomous run: %s\n", resp.Decision.Outcome)

	// A failing command demotes instead of graduating.
	fmt.Println("\n2. A command that fails. Teaching \"do the thing\" -> false, then approving.")
	for i := 0; i < 4; i++ {
		d.teach("do the thing", "false", base.Add(time.Duration(30+i)*time.Hour))
	}
	dec, _ := d.decide("do the thing")
	h, outcome, _ := d.approve(dec, base.Add(50*time.Hour))
	fmt.Printf("   approve -> %s\n   status is now %q: a failed run demotes, never graduates.\n", outcome, h.Status)

	// A denylisted command stays a dry run even though allowlisted.
	fmt.Println("\n3. A denylisted command, allowlisted anyway. Teaching \"wipe\" -> rm -rf.")
	for i := 0; i < 4; i++ {
		d.teach("wipe the scratch dir", "rm -rf ./scratch", base.Add(time.Duration(60+i)*time.Hour))
	}
	dec, _ = d.decide("wipe the scratch dir")
	_, outcome, _ = d.approve(dec, base.Add(80*time.Hour))
	fmt.Printf("   approve -> %s\n   the denylist overrides the allowlist; nothing ran.\n", outcome)

	// The graduated pair fires from an observation, no human in the loop.
	fmt.Println("\n4. AUTOMATIC CUEING. The graduated pair from act 1 again, but this")
	fmt.Println("   time nobody types the cue: an observation contains it.")
	fmt.Println("   syslog says: \"maintenance window: greet the world before backup\"")
	d.process(Observation{Channel: "syslog",
		Content: "maintenance window: greet the world before backup", At: time.Now()}, false, true)
	if entries, _ := d.store.AuditTail(1); len(entries) == 1 {
		fmt.Printf("   observed -> %s\n", entries[0].Outcome)
	}
	fmt.Println("   the cue came from the senses; no proposal, no approval, full audit.")

	fmt.Println("\n" + line)
	fmt.Println("Decision trail:")
	entries, _ := d.store.AuditTail(8)
	for i := len(entries) - 1; i >= 0; i-- {
		fmt.Printf("  %q -> %q  %s\n", entries[i].Cue, entries[i].Action, entries[i].Outcome)
	}
}
