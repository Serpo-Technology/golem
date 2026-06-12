package daemon

import (
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"time"

	"golem/internal/attention"
	"golem/internal/config"
	"golem/internal/memory"
	"golem/internal/senses"
)

// DemoThink shows the inner loop on a throwaway memory: a lived day
// distils into concepts, quiet falls, and GOLEM wanders the graph in
// rounds of one primitive, walk(relation, signal), with mood selecting
// the kind of thought, fruit teaching it which operations are worth
// running, and a taught word leaning the mind toward the operation it
// names.
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
	fmt.Println("   every setting of walk(relation, signal) computing at once and")
	fmt.Println("   the calm chemistry picking free association to ignite.")
	d.process(senses.Observation{Channel: "shell", Content: "run: make test", At: time.Now()}, true, true)
	d.recent = nil
	settleAlarm(d)
	d.lastObs = time.Now().Add(-time.Minute)
	d.lastSurprise = time.Now().Add(-time.Minute)

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
	if encoded == 0 {
		fmt.Println("   (every thought this stretch was routine; nothing was kept)")
	}

	fmt.Println("\n5. FRUIT. A remembered thought reinforced the operation that")
	fmt.Println("   produced it; a routine one wore its operation a little. The")
	fmt.Println("   learned worth lives on the operation nodes, in the open:")
	printOps(d)

	fmt.Println("\n6. NAMING A KIND OF THOUGHT. The mental-teach grounds a concept")
	fmt.Println("   to walk(grounds, stress) and bonds the word \"worry\" to it, so")
	fmt.Println("   a word now means a way of thinking. Saying it warms the")
	fmt.Println("   concept, and an alarmed moment leans the same way:")
	if err := d.teachThought("worry", "grounds", "stress", "en"); err != nil {
		demoFail(err)
	}
	fmt.Println("      taught: \"worry\" -> walk(grounds,stress)")
	d.chem.SpikeNoradrenaline(0.6)
	tpl := attention.Normalize("Disk full on /dev/disk1s1")
	if disk, err := d.store.ConceptByTemplate(tpl); err == nil && disk != nil {
		d.ws.excite(disk.ID, 1.0)
	}
	d.lastObs = time.Now().Add(-time.Minute)
	d.lastSurprise = time.Now().Add(-time.Minute)
	d.startThinkRound()
	d.settleThinking()
	if d.lastThought != "" {
		fmt.Printf("      the worried mind dwells: %q\n", d.lastThought)
	}

	fmt.Println("\n7. THE FACULTIES ARE ACTS. Recall, ground, consolidate, speak,")
	fmt.Println("   propose, and introspect sit in the same operant space as the")
	fmt.Println("   walks. A day's clutter has piled up, \"tidy\" was taught to name")
	fmt.Println("   the consolidate act, and saying it steers the quiet loop to fire")
	fmt.Println("   the same consolidation the sleep verb runs:")
	if err := d.teachThought("tidy", "act", "consolidate", "en"); err != nil {
		demoFail(err)
	}
	fmt.Println("      taught: \"tidy\" -> act(consolidate)")
	settleAlarm(d)
	// A calm stretch builds moderate sleep pressure: messy and drowsy,
	// but not yet over the sleep gate. That gap is the act's niche.
	for range 8 {
		d.chem.BuildSleepPressure()
	}
	d.encodesSinceSleep = 40
	d.lastSleep = time.Now().Add(-time.Hour) // the nap was hours ago; clutter is real
	nights := d.sleeps
	d.lastObs = time.Now().Add(-time.Minute)
	d.lastSurprise = time.Now().Add(-time.Minute)
	d.startThinkRound()
	d.settleThinking()
	if d.sleeps > nights {
		fmt.Println("      the loop consolidated unprompted; the firing is on the trail:")
		if entries, err := d.store.AuditTail(3); err == nil {
			for _, e := range entries {
				if strings.HasPrefix(e.Cue, "act:") {
					fmt.Printf("      audit: %s -> %s\n", e.Cue, e.Outcome)
				}
			}
		}
	}

	fmt.Println("\n" + line)
	fmt.Println("The loop closed: perception warmed a concept, one primitive walked")
	fmt.Println("the lived graph with mood picking the setting, the surprising step")
	fmt.Println("became memory and fed its operation, a taught word now names a kind")
	fmt.Println("of thinking, and the faculties fire from the same arbiter as acts.")
}

// settleAlarm drains the day's noradrenaline the way the homeostasis
// tick does across a real calm stretch.
func settleAlarm(d *Daemon) {
	for range 200 {
		d.chem.Decay()
	}
}

// printOps lists the operation nodes the run has minted, worth first.
func printOps(d *Daemon) {
	ops, err := d.store.NodesByKind(memory.KindOperation)
	if err != nil || len(ops) == 0 {
		fmt.Println("   (no operation has committed a thought yet)")
		return
	}
	sort.Slice(ops, func(i, j int) bool { return ops[i].Weight > ops[j].Weight })
	for _, op := range ops {
		fmt.Printf("   %-28s worth %.2f\n", op.Template, op.Weight)
	}
}

// calmPressure builds the sleep pressure a real quiet evening would.
func calmPressure(d *Daemon) {
	for range 20 {
		d.chem.BuildSleepPressure()
	}
}
