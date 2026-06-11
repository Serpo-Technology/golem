package main

import (
	"fmt"
	"io"
	"os"
	"time"
)

// runConsolidationDemo feeds one synthetic day into a throwaway memory
// and sleeps on it: repeats of one shape merge into a prototype, and an
// incident scattered across three senses gets woven together by replay,
// so the morning recall walks from one sense to what the others saw.
func runConsolidationDemo() {
	home, err := os.MkdirTemp("", "golem-sleep-")
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

	line := "--------------------------------------------------------------------"
	day := time.Now().Add(-8 * time.Hour)
	feed := func(channel, content string, at time.Time) {
		d.process(Observation{Channel: channel, Content: content, At: at}, true, true)
	}

	fmt.Println("GOLEM consolidation demo. One synthetic day, then one night of")
	fmt.Println("sleep, in a throwaway memory.")
	fmt.Println(line)

	fmt.Println("\n1. REPEATS. The same disk-full shape arrives six times across the")
	fmt.Println("   day, hours apart, each with a different volume number.")
	for i := 0; i < 6; i++ {
		feed("syslog", fmt.Sprintf("Disk full on /dev/disk1s%d", i+1), day.Add(time.Duration(i)*70*time.Minute))
	}

	fmt.Println("\n2. AN INCIDENT. Three senses speak within four minutes, too far")
	fmt.Println("   apart for the live 30-second binding window:")
	incident := day.Add(5 * time.Hour)
	feed("syslog", "Disk full on /dev/disk1s7", incident)
	feed("vitals", "memory pressure went high", incident.Add(2*time.Minute))
	feed("shell", "command failed with exit status 1", incident.Add(4*time.Minute))
	fmt.Println("   syslog: Disk full on /dev/disk1s7")
	fmt.Println("   vitals: memory pressure went high      (2 minutes later)")
	fmt.Println("   shell:  command failed with exit status 1  (4 minutes later)")

	held, _ := d.store.Count()
	fmt.Printf("\n   Held before sleep: %d fragments.\n", held)
	fmt.Println("   recall \"memory pressure\" before sleep:")
	printDemoRecall(d)

	fmt.Println("\n3. SLEEP. Replay weaves the day's strong cross-channel moments,")
	fmt.Println("   decay prunes, and recurring shapes merge into prototypes.")
	d.sleepCycle()

	held, _ = d.store.Count()
	fmt.Printf("\n4. THE MORNING AFTER. Held: %d fragments.\n", held)
	frags, _ := d.store.Dump()
	for _, f := range frags {
		if f.Count > 1 {
			fmt.Printf("   prototype #%d %q stands for %d occurrences\n", f.ID, f.Content, f.Count)
		}
	}
	fmt.Println("   recall \"memory pressure\" after sleep:")
	printDemoRecall(d)
	fmt.Println("\n   The vitals moment now brings the disk and the failed command to")
	fmt.Println("   mind: replay bound what the day kept minutes apart.")
}

func printDemoRecall(d *Daemon) {
	resp, _, _ := d.handle(Request{Cmd: "recall", Query: "memory pressure"}, nil)
	if len(resp.Fragments) == 0 {
		fmt.Println("     nothing comes to mind.")
		return
	}
	for _, f := range resp.Fragments {
		marker := "direct"
		if f.Activation < 1 {
			marker = fmt.Sprintf("assoc %.2f", f.Activation)
		}
		count := ""
		if f.Count > 1 {
			count = fmt.Sprintf("  x%d", f.Count)
		}
		fmt.Printf("     [%s] %-8s %q%s\n", marker, f.Channel, f.Content, count)
	}
}
