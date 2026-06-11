package daemon

import (
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"golem/internal/config"
	"golem/internal/senses"
	"golem/internal/wire"
)

// DemoThink shows the inner loop on a throwaway memory: a lived day
// distils into concepts, quiet falls, and GOLEM's specialists wander
// the graph in rounds, chaining warmth across ticks, letting routine
// thoughts pass unremembered, and encoding the novel one as an insight
// it can later recall having had.
func DemoThink() {
	home, err := os.MkdirTemp("", "golem-think-")
	if err != nil {
		demoFail(err)
	}
	defer os.RemoveAll(home)

	cfg := config.Default()
	cfg.Channels = config.ChannelsConfig{}
	d, err := New(home, cfg, io.Discard)
	if err != nil {
		demoFail(err)
	}
	defer d.store.Close()

	line := "--------------------------------------------------------------------"
	day := time.Now().Add(-6 * time.Hour)
	fmt.Println("GOLEM inner-loop demo. One lived day, one night, then a quiet")
	fmt.Println("stretch of mind-wandering. Memory is throwaway; nothing runs.")
	fmt.Println(line)

	fmt.Println("\n1. A DAY WITH A SHAPE. The same sequence four times, spaced so")
	fmt.Println("   each step neighbors only the next, and every push was chased")
	fmt.Println("   by a disk alarm:")
	fmt.Println("      make test  ->  make deploy  ->  make push  ->  disk full")
	for i := range 4 {
		at := day.Add(time.Duration(i) * 40 * time.Minute)
		d.recent = nil
		d.process(senses.Observation{Channel: "shell", Content: "run: make test", At: at}, true, true)
		d.process(senses.Observation{Channel: "shell", Content: "run: make deploy", At: at.Add(20 * time.Second)}, true, true)
		d.process(senses.Observation{Channel: "shell", Content: "run: make push", At: at.Add(45 * time.Second)}, true, true)
		d.chem.SpikeNoradrenaline(0.5) // the alarm lands on a stressed moment
		d.process(senses.Observation{Channel: "syslog",
			Content: fmt.Sprintf("Disk full on /dev/disk1s%d", i+1), At: at.Add(60 * time.Second)}, true, true)
	}
	d.recent = nil

	fmt.Println("\n2. SLEEP. The shapes distil into concepts the inner loop can")
	fmt.Println("   wander over.")
	calmPressure(d)
	d.sleepCycle()
	concepts, _ := d.store.Concepts()
	for _, c := range concepts {
		fmt.Printf("   concept [%-4s] %q\n", c.Category, c.Content)
	}

	fmt.Println("\n3. QUIET FALLS. One last test run warms its concept, then nothing")
	fmt.Println("   happens, and the default-mode condition holds: input silent,")
	fmt.Println("   sleep pressure building, something still warm. GOLEM wanders,")
	fmt.Println("   one specialist round per idle tick.")
	d.process(senses.Observation{Channel: "shell", Content: "run: make test", At: time.Now()}, true, true)
	d.recent = nil
	d.lastObs = time.Now().Add(-time.Minute)
	d.lastSurprise = time.Now().Add(-time.Minute)

	thoughtsBefore := countThoughts(d)
	for tick := 1; tick <= 8; tick++ {
		d.startThinkRound()
		d.settleThinking()
		d.ws.decay()
		warm := d.ws.warmest(2)
		if len(warm) == 0 {
			fmt.Printf("   tick %d: the chain has damped to nothing; the mind is still.\n", tick)
			break
		}
		parts := make([]string, 0, len(warm))
		for _, w := range warm {
			if frags, err := d.store.FragmentsByIDs([]uint64{w.id}); err == nil && len(frags) == 1 {
				parts = append(parts, fmt.Sprintf("%s %.2f", frags[0].Content, w.heat))
			}
		}
		fmt.Printf("   tick %d: warm: %s\n", tick, strings.Join(parts, ", "))
	}

	fmt.Println("\n4. WHAT THE GATE KEPT. Every round re-entered perception on the")
	fmt.Println("   thought channel through the same surprise gate the senses face;")
	fmt.Println("   routine associations passed unremembered, and what was novel")
	fmt.Println("   was encoded:")
	frags, _ := d.store.Dump()
	encoded := 0
	for _, f := range frags {
		if f.Channel == thoughtChannel {
			encoded++
			fmt.Printf("   insight #%d  %q  (%.2f bits)\n", f.ID, f.Content, f.Surprise)
		}
	}
	if encoded-thoughtsBefore == 0 {
		fmt.Println("   (every thought this stretch was routine; nothing was kept)")
	}

	fmt.Println("\n5. REMEMBERING HAVING THOUGHT. Recall reaches the insight like any")
	fmt.Println("   other moment:")
	resp, _, _ := d.handle(wire.Request{Cmd: "recall", Query: "thought"}, nil)
	shown := 0
	for _, f := range resp.Fragments {
		if f.Channel == thoughtChannel && shown < 2 {
			fmt.Printf("   [moment] %q\n", f.Content)
			shown++
		}
	}

	fmt.Println("\n" + line)
	fmt.Println("The loop closed: perception warmed a concept, thought chained it")
	fmt.Println("through lived edges, the surprising step became memory, and memory")
	fmt.Println("now holds a moment whose content is GOLEM's own thinking.")
}

// calmPressure builds the sleep pressure a real quiet evening would.
func calmPressure(d *Daemon) {
	for range 20 {
		d.chem.BuildSleepPressure()
	}
}

func countThoughts(d *Daemon) int {
	frags, _ := d.store.Dump()
	n := 0
	for _, f := range frags {
		if f.Channel == thoughtChannel {
			n++
		}
	}
	return n
}
