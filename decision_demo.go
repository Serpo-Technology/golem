package main

import (
	"fmt"
	"io"
	"os"
	"time"
)

// runDecisionDemo grounds a few cue-and-command pairs by synthetic
// teaching, then walks the decision loop end to end: a proposal, a
// graduation, an ambiguity the ACC vetoes, and a denylisted command that
// can never graduate. It uses a throwaway memory and exits on its own.
func runDecisionDemo() {
	home, err := os.MkdirTemp("", "golem-decide-")
	if err != nil {
		fail(err)
	}
	defer os.RemoveAll(home)

	cfg := DefaultConfig()
	cfg.Channels = ChannelsConfig{}
	d, err := NewDaemon(home, cfg, io.Discard)
	if err != nil {
		fail(err)
	}
	defer d.store.Close()

	base := time.Now().Add(-100 * time.Hour)
	line := "--------------------------------------------------------------------"

	fmt.Println("GOLEM decision loop demo. Memory is throwaway; nothing on your")
	fmt.Println("system is touched. Commands are shown, never executed.")
	fmt.Printf("Graduation needs %d consistent approvals.\n", cfg.GraduationApprovals)
	fmt.Println(line)

	// Act 1: ground a cue to a command, then propose.
	fmt.Println("\n1. GROUNDING. Teaching \"run the tests\" -> \"go test ./...\" four times,")
	fmt.Println("   the way the shell hook would see you say it and then run it.")
	for i := 0; i < 4; i++ {
		d.teach("run the tests", "go test ./...", base.Add(time.Duration(i)*time.Hour))
	}
	dec, _ := d.decide("run the tests")
	fmt.Print("   You say \"run the tests\" again -> ")
	showProposal(dec)

	// Act 2: approve until it graduates.
	fmt.Println("\n2. GRADUATION. Approving the proposal, again and again.")
	for i := 0; i < cfg.GraduationApprovals; i++ {
		dec, _ = d.decide("run the tests")
		h, outcome, err := d.approve(dec, base.Add(time.Duration(10+i)*time.Hour))
		if err != nil {
			fail(err)
		}
		fmt.Printf("   approve #%d -> %s  [now %s, approvals %d]\n", i+1, outcome, h.Status, h.Approvals)
	}
	dec, _ = d.decide("run the tests")
	fmt.Print("   You say \"run the tests\" once more -> ")
	showProposal(dec)

	// Act 3: an ambiguous cue the ACC vetoes.
	fmt.Println("\n3. CONFLICT. Teaching \"clean up the project\" to mean two different")
	fmt.Println("   things, equally often, so neither clearly wins.")
	for i := 0; i < 2; i++ {
		d.teach("clean up the project", "git clean -fd", base.Add(time.Duration(30+i)*time.Hour))
		d.teach("clean up the project", "docker system prune -f", base.Add(time.Duration(40+i)*time.Hour))
	}
	dec, _ = d.decide("clean up the project")
	fmt.Print("   You say \"clean up the project\" -> ")
	showProposal(dec)
	fmt.Println("   You clarify: you meant git. Teaching that once more.")
	d.teach("clean up the project", "git clean -fd", base.Add(50*time.Hour))
	dec, _ = d.decide("clean up the project")
	fmt.Print("   You say \"clean up the project\" again -> ")
	showProposal(dec)

	// Act 4: a denylisted command can be proposed but never graduate.
	fmt.Println("\n4. SAFETY. Teaching \"wipe the temp dir\" -> \"rm -rf ./tmp\" and")
	fmt.Printf("   approving it %d times anyway.\n", cfg.GraduationApprovals+1)
	for i := 0; i < 5; i++ {
		d.teach("wipe the temp dir", "rm -rf ./tmp", base.Add(time.Duration(60+i)*time.Hour))
	}
	var lastStatus string
	for i := 0; i < cfg.GraduationApprovals+1; i++ {
		dec, _ = d.decide("wipe the temp dir")
		h, _, err := d.approve(dec, base.Add(time.Duration(70+i)*time.Hour))
		if err != nil {
			fail(err)
		}
		lastStatus = h.Status
	}
	dec, _ = d.decide("wipe the temp dir")
	fmt.Printf("   After %d approvals it is still %q: a denylisted command\n",
		cfg.GraduationApprovals+1, lastStatus)
	fmt.Print("   never runs unattended -> ")
	showProposal(dec)

	// The durable records.
	fmt.Println("\n" + line)
	fmt.Println("Grounded habits:")
	habits, _ := d.store.AllHabits()
	for _, h := range habits {
		fmt.Printf("  [%-9s] %q -> %q  (approvals %d, denials %d)\n",
			h.Status, h.Cue, h.Action, h.Approvals, h.Denials)
	}
	fmt.Println("\nDecision trail (most recent first):")
	entries, _ := d.store.AuditTail(6)
	for _, e := range entries {
		fmt.Printf("  %q -> %q  %s\n", e.Cue, e.Action, e.Outcome)
	}
}

func showProposal(dec Decision) {
	switch {
	case dec.Proposed == nil:
		fmt.Println("nothing grounded yet.")
	case dec.Conflict:
		fmt.Printf("CONFLICT.\n   %s\n", dec.Question)
	default:
		mark := ""
		if isDenylisted(dec.Proposed.Command) {
			mark = "  (denylisted)"
		}
		fmt.Printf("proposes %q  [saliency %.2f, %s]%s\n",
			dec.Proposed.Command, dec.Proposed.Saliency, dec.Status, mark)
	}
}
