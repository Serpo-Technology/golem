package daemon

import (
	"strings"
	"testing"
	"time"

	"golem/internal/attention"
	"golem/internal/config"
	"golem/internal/limbic"
	"golem/internal/memory"
	"golem/internal/senses"
	"golem/internal/wire"
)

// settleChem drains the alarm a day of encodes leaves behind, the way
// the homeostasis tick does across the calm stretch that gates
// mind-wandering: by the time a round may start, noradrenaline has long
// drifted home.
func settleChem(d *Daemon) {
	for range 200 {
		d.chem.Decay()
	}
}

// thinkingDay lays down a lived sequence A -> B -> C, distils the three
// concepts, and returns them ready to think over.
func thinkingDay(t *testing.T, d *Daemon, s *memory.Store) (a, b, c *memory.Fragment) {
	t.Helper()
	base := time.Now().Add(-4 * time.Hour)
	for i := range 4 {
		at := base.Add(time.Duration(i) * 10 * time.Minute)
		d.recent = nil
		// Spaced so each step neighbors only the next: A binds B, B
		// binds C, and A never binds C directly.
		d.process(senses.Observation{Channel: "shell", Content: "run: make test", At: at}, true, true)
		d.process(senses.Observation{Channel: "shell", Content: "run: make deploy", At: at.Add(20 * time.Second)}, true, true)
		d.process(senses.Observation{Channel: "shell", Content: "run: make push", At: at.Add(45 * time.Second)}, true, true)
	}
	d.recent = nil
	if _, err := s.MergeDuplicates(); err != nil {
		t.Fatal(err)
	}
	calm(d)
	d.growConcepts()
	settleChem(d)
	// The world has been quiet since: rounds may commit.
	d.lastObs = time.Now().Add(-time.Minute)
	d.lastSurprise = time.Now().Add(-time.Minute)
	a, _ = s.ConceptByTemplate(attention.Normalize("run: make test"))
	b, _ = s.ConceptByTemplate(attention.Normalize("run: make deploy"))
	c, _ = s.ConceptByTemplate(attention.Normalize("run: make push"))
	if a == nil || b == nil || c == nil {
		t.Fatal("the day should distil three concepts")
	}
	return a, b, c
}

// With input quiet, a warm concept extends into a chain of associated
// concepts across successive steps: the warmth moves through the lived
// graph instead of dying where it started.
func TestMindWanderingChainsAcrossTicks(t *testing.T) {
	d, s := testDaemon(t)
	a, b, c := thinkingDay(t, d, s)

	d.ws.excite(a.ID, 1.0)
	d.startThinkRound()
	d.settleThinking()
	if w := d.ws.warmest(1); len(w) == 0 || w[0].id != b.ID {
		t.Fatalf("the first step should hand warmth to the successor concept, warmest %+v", w)
	}
	if d.ws.heat(a.ID) >= d.ws.heat(b.ID) {
		t.Fatal("the seed should fatigue below what it warmed")
	}
	d.startThinkRound()
	d.settleThinking()
	if d.ws.heat(c.ID) == 0 {
		t.Fatalf("the second step should extend the chain: heat(c)=%.2f", d.ws.heat(c.ID))
	}
}

// An unreinforced chain damps back to nothing: fatigue and decay always
// win over a finite graph, so rumination is bounded.
func TestChainDampsToNothing(t *testing.T) {
	d, s := testDaemon(t)
	a, _, _ := thinkingDay(t, d, s)

	d.ws.excite(a.ID, 1.0)
	for range 400 {
		d.startThinkRound()
		d.settleThinking()
		d.ws.decay()
		if len(d.ws.warmest(1)) == 0 {
			return
		}
	}
	t.Fatalf("the chain should have damped, workspace still warm: %+v", d.ws.warmest(3))
}

// Thoughts pass the same surprise gate perception uses: a routine
// association habituates and stays transient, so memory does not flood.
func TestRoutineThoughtsStayTransient(t *testing.T) {
	d, s := testDaemon(t)
	a, _, _ := thinkingDay(t, d, s)

	for range 30 {
		settleChem(d)
		d.ws.excite(a.ID, 1.0)
		d.startThinkRound()
		d.settleThinking()
		d.ws.clear()
	}
	frags, _ := s.Dump()
	thoughts := 0
	for _, f := range frags {
		if f.Channel == thoughtChannel {
			thoughts++
		}
	}
	if thoughts > 3 {
		t.Fatalf("a repeated thought should habituate, %d encoded", thoughts)
	}
}

// A novel combination clears the gate, is encoded as an insight, and a
// later recall shows GOLEM remembers having thought it.
func TestInsightIsRememberedThought(t *testing.T) {
	d, s := testDaemon(t)
	a, _, _ := thinkingDay(t, d, s)

	// The thought channel learns its routine shapes first.
	for i := range 6 {
		d.process(senses.Observation{Channel: thoughtChannel,
			Content: "thought: filler shape " + string(rune('a'+i)), At: time.Now()}, true, true)
	}
	d.ws.excite(a.ID, 1.0)
	d.startThinkRound()
	d.settleThinking()

	frags, _ := s.Dump()
	var insight *memory.Fragment
	for _, f := range frags {
		if f.Channel == thoughtChannel && strings.Contains(f.Content, "make") {
			insight = f
		}
	}
	if insight == nil {
		t.Fatal("a novel association should encode as an insight")
	}
	resp, _, _ := d.handle(wire.Request{Cmd: "recall", Query: "thought"}, nil)
	var remembered bool
	for _, f := range resp.Fragments {
		remembered = remembered || f.ID == insight.ID
	}
	if !remembered {
		t.Fatal("recall should surface the remembered thought")
	}
}

// The mind's own activity is not the world's: a thought never fires a
// graduated habit, however plainly it contains the cue.
func TestThoughtsNeverCueHabits(t *testing.T) {
	d, s := testDaemon(t)
	d.cfg.ExecuteEnabled = true
	d.cfg.ExecuteAllowlist = []string{"echo"}
	ground(t, d, "make test", "echo fired", 4)
	if err := s.SaveHabit(memory.Habit{Cue: attention.Normalize("make test"), Action: "echo fired",
		Approvals: 3, Status: memory.HabitGraduated}); err != nil {
		t.Fatal(err)
	}
	d.habitsChanged()

	d.process(senses.Observation{Channel: thoughtChannel,
		Content: "thought: after make test, make deploy", At: time.Now()}, false, true)
	d.settleExecution()

	entries, _ := s.AuditTail(5)
	for _, e := range entries {
		if strings.HasPrefix(e.Outcome, "autonomous") {
			t.Fatalf("a thought must never cue a habit: %+v", e)
		}
	}
}

// Thoughts leave the quiet timers alone: mind-wandering neither throttles
// itself nor holds off sleep's input-silence test.
func TestThoughtsDoNotResetQuietTimers(t *testing.T) {
	d, _ := testDaemon(t)
	obs := time.Now().Add(-time.Minute)
	sur := time.Now().Add(-2 * time.Minute)
	d.lastObs, d.lastSurprise = obs, sur

	d.process(senses.Observation{Channel: thoughtChannel,
		Content: "thought: anything at all", At: time.Now()}, false, true)
	if !d.lastObs.Equal(obs) || !d.lastSurprise.Equal(sur) {
		t.Fatalf("thoughts must not touch the quiet timers: obs %v sur %v", d.lastObs, d.lastSurprise)
	}
}

// The default-mode condition: quiet, calm, not yet asleep, something
// warm to think from.
func TestMindWanderingCondition(t *testing.T) {
	d, s := testDaemon(t)
	a, _, _ := thinkingDay(t, d, s)

	d.lastObs = time.Now().Add(-10 * time.Second)
	d.lastSurprise = time.Now().Add(-10 * time.Second)
	d.lastSleep = time.Now()
	if d.mindWandering() {
		t.Fatal("an empty workspace has nothing to wander over")
	}
	d.ws.excite(a.ID, 0.8)
	if !d.mindWandering() {
		t.Fatal("quiet, calm, and warm should wander")
	}
	d.lastObs = time.Now()
	if d.mindWandering() {
		t.Fatal("fresh input should hold thinking off")
	}
}

// One round in flight at a time: launching again while specialists are
// out is a no-op, so the inner life stays bounded.
func TestThinkRoundsAreSingleFlight(t *testing.T) {
	d, s := testDaemon(t)
	a, _, _ := thinkingDay(t, d, s)
	d.ws.excite(a.ID, 1.0)

	d.startThinkRound()
	inFlight := d.thinking
	if inFlight == 0 {
		t.Fatal("the round should have launched specialists")
	}
	d.startThinkRound()
	if d.thinking != inFlight {
		t.Fatalf("a second launch must not stack: %d then %d", inFlight, d.thinking)
	}
	d.settleThinking()
}

// A round that completes into a busy moment is discarded whole: thinking
// yields to perception, leaving the workspace and memory untouched.
func TestThinkingYieldsToPerception(t *testing.T) {
	d, s := testDaemon(t)
	a, b, _ := thinkingDay(t, d, s)
	d.ws.excite(a.ID, 1.0)

	d.startThinkRound()
	d.lastObs = time.Now() // the world spoke while the specialists were out
	d.settleThinking()

	if d.ws.heat(b.ID) != 0 {
		t.Fatal("a yielded round must not warm the workspace")
	}
	if d.ws.heat(a.ID) != 1.0 {
		t.Fatal("a yielded round must not fatigue the seed")
	}
	frags, _ := s.Dump()
	for _, f := range frags {
		if f.Channel == thoughtChannel {
			t.Fatalf("a yielded round must not encode: %q", f.Content)
		}
	}
}

// Sleep invalidates in-flight thought: candidates stamped with an older
// epoch are stale on arrival and the round commits nothing.
func TestSleepDiscardsStaleThoughts(t *testing.T) {
	d, s := testDaemon(t)
	a, b, _ := thinkingDay(t, d, s)
	d.ws.excite(a.ID, 1.0)

	d.startThinkRound()
	d.thinkEpoch++ // consolidation rewrote the graph under the round
	d.settleThinking()

	if d.ws.heat(b.ID) != 0 {
		t.Fatal("stale candidates must not warm the workspace")
	}
	frags, _ := s.Dump()
	for _, f := range frags {
		if f.Channel == thoughtChannel {
			t.Fatalf("stale candidates must not encode: %q", f.Content)
		}
	}
}

// The decider only ever thinks that something is runnable: with a warm
// actionable concept and execution wide open, deliberation surfaces as
// a thought and nothing runs.
func TestDeciderThinksWithoutActing(t *testing.T) {
	d, s := testDaemon(t)
	d.cfg.ExecuteEnabled = true
	d.cfg.ExecuteAllowlist = []string{"echo"}

	// A lone command shape with no neighbors: the spreader and the
	// predictor have nothing, so deliberation gets its turn.
	anchors := feedShape(t, s, "shell", "run: echo hello", 4, time.Now().Add(-2*time.Hour))
	_ = anchors
	if _, err := s.MergeDuplicates(); err != nil {
		t.Fatal(err)
	}
	calm(d)
	d.growConcepts()
	c, _ := s.ConceptByTemplate(attention.Normalize("run: echo hello"))
	if c == nil || c.Category != "verb" {
		t.Fatalf("the command shape should be a verb concept: %+v", c)
	}
	// The thought channel has seen enough shapes that a new one clears
	// the gate. The fillers' alarm settles the way a real calm stretch
	// would, so the relevance drive is not what speaks here.
	for i := range 6 {
		d.process(senses.Observation{Channel: thoughtChannel,
			Content: "thought: filler shape " + string(rune('a'+i)), At: time.Now()}, true, true)
	}
	settleChem(d)
	d.lastObs = time.Now().Add(-time.Minute)
	d.lastSurprise = time.Now().Add(-time.Minute)
	d.ws.excite(c.ID, 1.0)

	d.startThinkRound()
	d.settleThinking()

	frags, _ := s.Dump()
	var deliberated bool
	for _, f := range frags {
		if f.Channel == thoughtChannel && strings.Contains(f.Content, "could run") {
			deliberated = true
		}
	}
	if !deliberated {
		t.Fatal("the decider should think the command runnable")
	}
	entries, _ := s.AuditTail(5)
	if len(entries) != 0 {
		t.Fatalf("deliberation must never act: %+v", entries)
	}
	if len(d.running) != 0 {
		t.Fatal("nothing may be executing")
	}
}

// Concurrent thinking against live perception: the real loop wanders,
// commits or yields, and the store never races. The assertions are the
// invariants; the race detector is the referee.
func TestThinkingUnderLoad(t *testing.T) {
	home := t.TempDir()
	s, err := memory.OpenStore(home + "/memory.db")
	if err != nil {
		t.Fatal(err)
	}
	// A lived day already distilled, so the daemon wakes with something
	// to think about.
	base := time.Now().Add(-2 * time.Hour)
	var proto *memory.Fragment
	for i := range 4 {
		f := &memory.Fragment{Channel: "syslog", Content: "Disk full on /dev/disk1s9",
			Template: attention.Normalize("Disk full on /dev/disk1s9"), Surprise: 3,
			Time: base.Add(time.Duration(i) * time.Minute)}
		if _, err := s.Encode(f); err != nil {
			t.Fatal(err)
		}
		proto = f
	}
	c := &memory.Fragment{Kind: memory.KindConcept, Channel: "concept", Content: "disk full",
		Template: proto.Template, Category: "noun", Time: time.Now()}
	if _, err := s.Encode(c); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Reground(0.8); err != nil {
		t.Fatal(err)
	}
	s.Close()

	cfg := config.Default()
	cfg.Channels = config.ChannelsConfig{}
	cfg.SleepIntervalSec = 3600
	cfg.CalmBeforeSleepSec = 0.05
	cfg.InputSilenceSec = 0.05
	home, done := startTestDaemonWith(t, home, cfg)

	// Perception and mind-wandering interleave for a stretch.
	for i := range 10 {
		if _, err := wire.Call(home, wire.Request{Cmd: "observe", Channel: "syslog",
			Content: "Disk full on /dev/disk1s9"}); err != nil {
			t.Fatal(err)
		}
		if i%3 == 0 {
			time.Sleep(120 * time.Millisecond) // long enough to wander
		}
	}
	time.Sleep(300 * time.Millisecond)
	if _, err := wire.Call(home, wire.Request{Cmd: "status"}); err != nil {
		t.Fatal(err)
	}
	if _, err := wire.Call(home, wire.Request{Cmd: "stop"}); err != nil {
		t.Fatal(err)
	}
	<-done
}

// newestThought returns the latest moment encoded on the thought
// channel, nil when none has been.
func newestThought(t *testing.T, s *memory.Store) *memory.Fragment {
	t.Helper()
	frags, err := s.Dump()
	if err != nil {
		t.Fatal(err)
	}
	var newest *memory.Fragment
	for _, f := range frags {
		if f.Channel == thoughtChannel {
			newest = f
		}
	}
	return newest
}

// The kind of thought emerges from mood: the same warm seed that
// associates forward when calm dwells on its most stressful grounding
// when noradrenaline is high. No specialist is chosen anywhere; the
// chemistry's appetite reweighs the one primitive's settings.
func TestAlarmShiftsThoughtTowardStress(t *testing.T) {
	d, s := testDaemon(t)
	a, _, _ := thinkingDay(t, d, s)

	// The thought channel knows enough shapes that a fresh one encodes.
	for i := range 6 {
		d.process(senses.Observation{Channel: thoughtChannel,
			Content: "thought: filler shape " + string(rune('a'+i)), At: time.Now()}, true, true)
	}
	settleChem(d)
	d.chem.SpikeNoradrenaline(0.8)
	d.ws.excite(a.ID, 1.0)
	d.startThinkRound()
	d.settleThinking()

	thought := newestThought(t, s)
	if thought == nil || !strings.Contains(thought.Content, "that time:") {
		t.Fatalf("an alarmed mind should dwell on a stressful grounding, got %+v", thought)
	}
}

// Fruit grounds the thought-types: an operation whose thought was
// surprising enough to remember gains worth and a provenance edge to the
// insight, while a routine thought wears its operation down.
func TestFruitReinforcesOperation(t *testing.T) {
	d, s := testDaemon(t)
	a, _, _ := thinkingDay(t, d, s)

	// The thought channel knows enough shapes that a fresh one encodes.
	for i := range 6 {
		d.process(senses.Observation{Channel: thoughtChannel,
			Content: "thought: filler shape " + string(rune('a'+i)), At: time.Now()}, true, true)
	}
	settleChem(d)
	d.ws.excite(a.ID, 1.0)
	d.startThinkRound()
	d.settleThinking()

	insight := newestThought(t, s)
	if insight == nil {
		t.Fatal("the first novel association should encode")
	}
	// The trail leads from the insight back to whichever operation
	// produced it: the provenance edge is how the fruit is attributed.
	into, err := s.LinksToRel([]uint64{insight.ID}, memory.RelGrounds)
	if err != nil || len(into) != 1 {
		t.Fatalf("the insight should be grounded by exactly its operation: %v %v", into, err)
	}
	producers, err := s.FragmentsByIDs([]uint64{into[0].From})
	if err != nil || len(producers) != 1 || producers[0].Kind != memory.KindOperation {
		t.Fatalf("the insight's grounding should come from an operation node: %+v", producers)
	}
	op := d.opNode(producers[0].Template)
	if op == nil {
		t.Fatal("the fruitful operation should have a node")
	}
	if op.Weight <= 1.0 {
		t.Fatalf("fruit should raise the operation's worth, got %.2f", op.Weight)
	}

	// Thinking the same thought again soon habituates, and once the gate
	// stops keeping it, the operation wears instead of growing.
	peak := op.Weight
	for range 4 {
		settleChem(d)
		d.ws.clear()
		d.ws.excite(a.ID, 1.0)
		d.startThinkRound()
		d.settleThinking()
		if op.Weight > peak {
			peak = op.Weight
		}
	}
	if op.Weight >= peak {
		t.Fatalf("a routine thought should wear its operation: peak %.2f, now %.2f", peak, op.Weight)
	}
}

// The refractory ring guards against rumination: a thought re-entered
// within the recent window is recognized as a repeat, and ages out once
// enough distinct thoughts have passed.
func TestThoughtRefractory(t *testing.T) {
	d, _ := testDaemon(t)
	if d.thoughtRuminates("thought: a small brown dog") {
		t.Fatal("a thought never thought is not rumination")
	}
	d.rememberThought("thought: a small brown dog")
	if !d.thoughtRuminates("thought: a small brown dog") {
		t.Fatal("a just-remembered thought is rumination")
	}
	// Push it out of the bounded ring with distinct thoughts.
	for i := range thoughtRefractory {
		d.rememberThought("thought: filler " + string(rune('a'+i)))
	}
	if d.thoughtRuminates("thought: a small brown dog") {
		t.Fatal("a thought aged out of the ring is no longer rumination")
	}
}

// A thought-type that repeatedly bears fruit is preferred over one that
// does not: learned worth multiplies into the arbitration priority.
func TestFruitfulOperationIsPreferred(t *testing.T) {
	sn := thoughtSnapshot{
		chem:    limbic.New().Snapshot(),
		opWorth: map[string]float64{},
		opHeat:  map[string]float64{},
	}
	succ := thoughtOp{rel: memory.RelSuccessor, sig: sigWeight}
	temp := thoughtOp{rel: memory.RelTemporal, sig: sigWeight}
	if opPriority(sn, succ, 1.0) <= opPriority(sn, temp, 1.0) {
		t.Fatal("at equal worth, order keeps its innate edge")
	}
	sn.opWorth[temp.template()] = 1.6 // temporal bore fruit, repeatedly
	sn.opWorth[succ.template()] = 0.6 // successor kept thinking nothing worth keeping
	if opPriority(sn, temp, 1.0) <= opPriority(sn, succ, 1.0) {
		t.Fatal("a fruitful operation should outrank a barren one")
	}
}

// The mental-teach: a word comes to name a thought operation through a
// concept grounded to it, and saying the word leans the policy toward
// that operation.
func TestMentalTeachNamesOperation(t *testing.T) {
	d, s := testDaemon(t)

	if err := d.teachThought("worry", "grounds", "stress", ""); err != nil {
		t.Fatal(err)
	}
	if err := d.teachThought("worry", "nonsense", "stress", ""); err == nil {
		t.Fatal("an unknown relation must be refused")
	}

	c := d.resolveConcept("worry")
	if c == nil {
		t.Fatal("the word should name a concept")
	}
	if c.Content != "walk grounds by stress" {
		t.Fatalf("the concept should gloss the operation, got %q", c.Content)
	}
	grounds, err := s.LinksFromRel([]uint64{c.ID}, memory.RelGrounds)
	if err != nil || len(grounds) != 1 {
		t.Fatalf("the concept should ground exactly the operation: %v %v", grounds, err)
	}
	ops, err := s.FragmentsByIDs([]uint64{grounds[0].To})
	if err != nil || len(ops) != 1 || ops[0].Kind != memory.KindOperation {
		t.Fatalf("the grounding should reach an operation node: %+v", ops)
	}

	// Teaching spoke about the operation, so its concept is warm, and the
	// policy reads that warmth as a lean toward the operation.
	_, heat := d.opPolicy()
	if heat["walk(grounds,stress)"] == 0 {
		t.Fatal("the taught operation should be warm in the policy")
	}
	sn := thoughtSnapshot{chem: d.chem.Snapshot(), opWorth: map[string]float64{},
		opHeat: heat}
	cold := thoughtSnapshot{chem: d.chem.Snapshot(), opWorth: map[string]float64{},
		opHeat: map[string]float64{}}
	op := thoughtOp{rel: memory.RelGrounds, sig: sigStress}
	if opPriority(sn, op, 0.5) <= opPriority(cold, op, 0.5) {
		t.Fatal("a warm name should lean thought toward its operation")
	}

	// Operation nodes and their naming survive a sleep that would prune
	// ordinary memory: the mind's own faculties are not nightly news.
	if _, err := s.Sleep(0.99, 0.99); err != nil {
		t.Fatal(err)
	}
	if n := d.opNode("walk(grounds,stress)"); n == nil {
		t.Fatal("the operation node should survive sleep")
	}
	if c := d.resolveConcept("worry"); c == nil {
		t.Fatal("the operation's name should survive sleep")
	}
}
