package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"sort"
	"strings"
	"sync/atomic"
	"syscall"
	"time"
)

// Daemon is the long-running form of GOLEM: the same observe/encode/sleep
// loop as the demo, but multi-channel, persistent, and queryable over a
// unix socket. All state mutation happens inside the single Run loop, so
// the brain stays single-threaded no matter how many sensors or CLI
// clients are talking to it.
type Daemon struct {
	home string
	cfg  Config
	out  io.Writer

	store       *Store
	chem        *Neurochemistry
	predictors  map[string]*Predictor
	calibrators map[string]*Calibrator
	stats       map[string]*ChannelStat
	drops       map[string]*atomic.Int64

	obs  chan Observation
	reqs chan ipcReq
	stop chan struct{} // closed to stop sensor goroutines

	started      time.Time
	lastObs      time.Time
	lastSurprise time.Time
	lastSleep    time.Time
	sleeps       int

	// recent holds the just-encoded moments that new fragments bind to.
	// It serves as the hippocampal present.
	recent []recentMoment

	// hot is the RAM working set in front of the sqlite archive: recent
	// encodes and recalls answer from here, and archive hits promote.
	hot *hotTier

	// governors throttle memory formation per channel, and pendingStorm
	// counts the surprises each throttle suppressed until they can be
	// summarized as one moment.
	governors    map[string]*tokenBucket
	pendingStorm map[string]int

	// pending holds the last proposal awaiting the operator's approve or
	// deny.
	pending *Decision

	// gradCues caches the graduated cue templates that automatic cueing
	// matches observations against; lastAutoCue is the per-cue cooldown.
	gradCues       []string
	gradCuesLoaded bool
	lastAutoCue    map[string]time.Time

	// Commands run off the loop, one at a time: running marks a command
	// in flight, and execDone carries its result back into the loop where
	// the bookkeeping finishes single-threaded.
	running    bool
	runningCmd string
	execDone   chan execResult
}

// execResult is a finished command on its way back into the brain loop.
// finish completes the habit, audit, feedback, and reply work that must
// run single-threaded, so the loop calls it, never the exec goroutine.
type execResult struct {
	ran, ok bool
	outcome string
	finish  func(ran, ok bool, outcome string)
}

// tokenBucket is the refractory on encoding: a burst allowance that
// refills with time. Suppressing formation is cheaper and saner than
// letting a storm mint hundreds of near-identical memories.
type tokenBucket struct {
	tokens float64
	max    float64
	refill float64 // tokens per second
	last   time.Time
}

func (b *tokenBucket) advance(now time.Time) {
	// Observation timestamps can lag the bucket's creation; time only
	// ever moves the bucket forward.
	if !now.After(b.last) {
		return
	}
	b.tokens += now.Sub(b.last).Seconds() * b.refill
	if b.tokens > b.max {
		b.tokens = b.max
	}
	b.last = now
}

func (b *tokenBucket) take(now time.Time) bool {
	b.advance(now)
	if b.tokens >= 1 {
		b.tokens--
		return true
	}
	return false
}

// full reports whether the bucket has recovered its whole burst
// allowance, which is what counts as a real lull on the channel.
func (b *tokenBucket) full(now time.Time) bool {
	b.advance(now)
	return b.tokens >= b.max
}

type recentMoment struct {
	id       uint64
	template string
	at       time.Time
}

// Association tuning. Link strength rides on dopamine, repetition of a
// template pair earns a bonus and a reward spike, and recall both rewards
// the chemistry and strengthens the paths it walked.
const (
	linkWindow        = 30 * time.Second
	maxLinkFanout     = 8
	linkBaseWeight    = 0.2
	linkDopamineGain  = 0.4
	assocRepeatBonus  = 0.1
	confirmReward     = 0.08
	recallReward      = 0.05
	linkPracticeDelta = 0.05
	spreadHops        = 2
	spreadDamping     = 0.7
	spreadFloor       = 0.05
	spreadBudget      = 10
)

type ipcReq struct {
	req   Request
	reply chan Response
}

func NewDaemon(home string, cfg Config, out io.Writer) (*Daemon, error) {
	if err := os.MkdirAll(home, 0o755); err != nil {
		return nil, err
	}
	store, err := OpenStore(filepath.Join(home, "memory.db"))
	if err != nil {
		return nil, err
	}
	d := &Daemon{
		home:         home,
		cfg:          cfg,
		out:          out,
		store:        store,
		chem:         NewNeurochemistry(),
		predictors:   make(map[string]*Predictor),
		calibrators:  make(map[string]*Calibrator),
		stats:        make(map[string]*ChannelStat),
		drops:        make(map[string]*atomic.Int64),
		governors:    make(map[string]*tokenBucket),
		pendingStorm: make(map[string]int),
		obs:          make(chan Observation, 256),
		reqs:         make(chan ipcReq),
		stop:         make(chan struct{}),
		execDone:     make(chan execResult, 1),
		hot:          newHotTier(),
	}
	if err := d.loadModels(); err != nil {
		store.Close()
		return nil, err
	}
	return d, nil
}

// loadModels restores every channel predictor and calibrator persisted by
// earlier runs, so a restart does not reset what counts as "normal."
func (d *Daemon) loadModels() error {
	channels, err := d.store.ModelChannels()
	if err != nil {
		return err
	}
	for _, ch := range channels {
		counts, err := d.store.LoadModel(ch)
		if err != nil {
			return err
		}
		p := NewPredictor(d.windowFor(ch))
		p.Seed(counts)
		d.predictors[ch] = p
	}
	calibs, err := d.store.LoadCalibs()
	if err != nil {
		return err
	}
	for ch, cs := range calibs {
		c := &Calibrator{}
		c.Seed(cs.N, cs.Mean, cs.Vari)
		d.calibrators[ch] = c
	}
	return nil
}

// windowFor resolves a channel's habituation window. Both the lazy
// creation path and model reload must use it, or a restart would rebuild
// a long-window model under a short window and compact it to nothing.
func (d *Daemon) windowFor(channel string) float64 {
	if w, ok := d.cfg.PredictorWindows[channel]; ok {
		return w
	}
	return d.cfg.PredictorWindow
}

func (d *Daemon) predictor(channel string) *Predictor {
	p, ok := d.predictors[channel]
	if !ok {
		p = NewPredictor(d.windowFor(channel))
		d.predictors[channel] = p
	}
	return p
}

func (d *Daemon) governor(channel string) *tokenBucket {
	b, ok := d.governors[channel]
	if !ok {
		burst := d.cfg.EncodeBurst
		if burst < 1 {
			burst = 1
		}
		refillSec := d.cfg.EncodeRefillSec
		if refillSec <= 0 {
			refillSec = 2
		}
		b = &tokenBucket{
			tokens: burst,
			max:    burst,
			refill: 1 / refillSec,
			last:   time.Now(),
		}
		d.governors[channel] = b
	}
	return b
}

func (d *Daemon) calibrator(channel string) *Calibrator {
	c, ok := d.calibrators[channel]
	if !ok {
		c = &Calibrator{}
		d.calibrators[channel] = c
	}
	return c
}

func (d *Daemon) stat(channel string) *ChannelStat {
	st, ok := d.stats[channel]
	if !ok {
		st = &ChannelStat{}
		d.stats[channel] = st
	}
	return st
}

// dropCounter returns the shared drop counter for a channel's sensor. All
// counters are created before sensor goroutines start, so the map is never
// written concurrently; the values are atomics precisely because sensors
// increment them outside the brain loop.
func (d *Daemon) dropCounter(channel string) *atomic.Int64 {
	c, ok := d.drops[channel]
	if !ok {
		c = new(atomic.Int64)
		d.drops[channel] = c
	}
	return c
}

// Run blocks until the daemon is stopped by a signal or a stop command.
// Shutdown always runs a final consolidation and flushes predictor state.
func (d *Daemon) Run() error {
	sock := socketPath(d.home)
	if err := claimSocket(sock); err != nil {
		d.store.Close()
		return err
	}
	listener, err := net.Listen("unix", sock)
	if err != nil {
		d.store.Close()
		return err
	}
	go d.acceptLoop(listener)

	sigc := make(chan os.Signal, 1)
	signal.Notify(sigc, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(sigc)

	now := time.Now()
	d.started, d.lastObs, d.lastSurprise, d.lastSleep = now, now, now, now

	held, _ := d.store.Count()
	fmt.Fprintf(d.out, "golem awake. home=%s, %d fragment(s) held, %d channel model(s) loaded.\n",
		d.home, held, len(d.predictors))
	d.startSensors()

	homeostasis := time.NewTicker(200 * time.Millisecond)
	defer homeostasis.Stop()

	for {
		select {
		case o := <-d.obs:
			d.process(o, false, true)

		case r := <-d.reqs:
			resp, deferred, quit := d.handle(r.req, r.reply)
			if !deferred {
				r.reply <- resp
			}
			if quit {
				d.shutdown(listener, sock)
				return nil
			}

		case res := <-d.execDone:
			d.running = false
			d.runningCmd = ""
			res.finish(res.ran, res.ok, res.outcome)

		case <-homeostasis.C:
			d.chem.Decay()
			d.flushStorms(time.Now())
			if time.Since(d.lastSurprise) > d.cfg.CalmBeforeSleep() {
				d.chem.BuildSleepPressure()
			}
			if d.readyToSleep() {
				d.sleepCycle()
			}

		case <-sigc:
			fmt.Fprintln(d.out, "signal received; sleeping once, then shutting down.")
			d.shutdown(listener, sock)
			return nil
		}
	}
}

// startExec runs one command off the brain loop. The goroutine touches no
// daemon state: it runs the command and posts the result to execDone,
// where the loop calls finish single-threaded. While the command runs the
// loop keeps sensing and answering; one command runs at a time, so callers
// must check d.running first.
func (d *Daemon) startExec(cmd string, finish func(ran, ok bool, outcome string)) {
	d.running = true
	d.runningCmd = cmd
	timeout := d.execTimeout()
	go func() {
		ran, ok, outcome := runCommand(cmd, timeout)
		d.execDone <- execResult{ran: ran, ok: ok, outcome: outcome, finish: finish}
	}()
}

// settleExecution waits for the in-flight command, if any, and completes
// it the way the Run loop would. Only loopless drivers need it: demos,
// tests, and shutdown, where the loop has already stopped selecting.
func (d *Daemon) settleExecution() {
	if !d.running {
		return
	}
	res := <-d.execDone
	d.running = false
	d.runningCmd = ""
	res.finish(res.ran, res.ok, res.outcome)
}

// handleNow services one request synchronously, completing any execution
// it starts before returning. It is the entry point for demos and tests
// that drive the daemon without its Run loop.
func (d *Daemon) handleNow(req Request) (Response, bool) {
	reply := make(chan Response, 1)
	resp, deferred, quit := d.handle(req, reply)
	if !deferred {
		return resp, quit
	}
	d.settleExecution()
	return <-reply, quit
}

// startSensors launches the built-in senses the config asks for. Every
// sensor is a goroutine that emits into the shared workspace queue and
// stops when d.stop closes. Drop counters are created here, before the
// goroutines exist, so the map is never written concurrently.
func (d *Daemon) startSensors() {
	var active []string
	ch := d.cfg.Channels
	if ch.Synthetic {
		go LogSensor("synthetic", d.obs, d.stop, d.dropCounter("synthetic"))
		active = append(active, "synthetic")
	}
	if len(ch.WatchDirs) > 0 {
		go FileSensor(ch.WatchDirs, d.obs, d.stop, d.dropCounter("files"))
		active = append(active, fmt.Sprintf("files(%d dirs)", len(ch.WatchDirs)))
	}
	if ch.Vitals {
		go VitalsSensor(d.obs, d.stop, d.dropCounter("vitals"))
		active = append(active, "vitals")
	}
	if ch.Syslog {
		go SyslogSensor(d.obs, d.stop, d.dropCounter("syslog"))
		active = append(active, "syslog")
	}
	if len(active) == 0 {
		fmt.Fprintln(d.out, "senses: none built in; feed channels with golem observe or the shell hook.")
		return
	}
	fmt.Fprintf(d.out, "senses: %s\n", strings.Join(active, ", "))
}

// claimSocket removes a stale socket file, but refuses to evict a live
// daemon.
func claimSocket(sock string) error {
	if _, err := os.Stat(sock); err != nil {
		return nil
	}
	conn, err := net.DialTimeout("unix", sock, 500*time.Millisecond)
	if err == nil {
		conn.Close()
		return errors.New("another golem daemon is already running on " + sock)
	}
	return os.Remove(sock)
}

func (d *Daemon) acceptLoop(l net.Listener) {
	for {
		conn, err := l.Accept()
		if err != nil {
			return // listener closed during shutdown
		}
		go d.serveConn(conn)
	}
}

func (d *Daemon) serveConn(conn net.Conn) {
	defer conn.Close()
	// The deadline must outlast a command the loop may run synchronously,
	// or the client would give up on a request that is still executing.
	execBudget := d.cfg.ExecuteTimeoutSec
	if execBudget <= 0 {
		execBudget = defaultExecuteTimeoutSec
	}
	conn.SetDeadline(time.Now().Add(time.Duration(execBudget+15) * time.Second))

	line, err := bufio.NewReader(conn).ReadBytes('\n')
	if err != nil {
		return
	}
	var req Request
	resp := Response{}
	if err := json.Unmarshal(line, &req); err != nil {
		resp.Error = "bad request: " + err.Error()
	} else {
		reply := make(chan Response, 1)
		select {
		case d.reqs <- ipcReq{req: req, reply: reply}:
			resp = <-reply
		case <-time.After(5 * time.Second):
			resp.Error = "daemon busy"
		}
	}
	payload, _ := json.Marshal(resp)
	conn.Write(append(payload, '\n'))
}

// process runs one observation through the full loop: normalize, score,
// learn, and encode if it clears the gate. Forced observations (the
// linguistic channel) skip the gate; what the user says is significant by
// definition. When bind is false the fragment neither links to the recent
// past nor enters it, which is what retroactive annotation needs.
func (d *Daemon) process(o Observation, forced, bind bool) ObserveResult {
	d.lastObs = time.Now()
	st := d.stat(o.Channel)
	st.Observed++

	tpl := Normalize(o.Content)
	p := d.predictor(o.Channel)
	bits := p.Surprise(tpl)
	p.Learn(tpl)

	// The reflex arc runs before the gate: a graduated habit must fire on
	// its cue even when the cue has become too routine to reach
	// consciousness. Speech is excluded (the operator has golem propose),
	// and so are action echoes: the feedback channel and shell lines that
	// are themselves commands, or an outcome could re-cue its own habit.
	if !forced && o.Channel != "action" && !strings.HasPrefix(o.Content, actionPrefix) {
		d.autoCue(o, tpl)
	}

	// The gate is channel-relative: the same bit count that is routine on
	// a diverse channel is an alarm on a monotonous one.
	if !forced &&
		!d.calibrator(o.Channel).Gate(bits, d.cfg.SurpriseThreshold, d.cfg.ZGate, d.cfg.CalibrationWarmup) {
		return ObserveResult{Bits: bits}
	}

	// The refractory: past the burst allowance, a storm of surprises is
	// counted instead of encoded, and later collapsed into one summary
	// moment. The predictor and calibrator have already learned the line.
	if !forced && !d.governor(o.Channel).take(o.At) {
		st.Suppressed++
		d.pendingStorm[o.Channel]++
		return ObserveResult{Bits: bits}
	}

	d.lastSurprise = time.Now()
	d.chem.SpikeNoradrenaline(bits * naSpikePerBit)
	d.chem.SuppressSleep(sleepSuppressOnSurprise)

	frag := &Fragment{
		Channel:  o.Channel,
		Content:  o.Content,
		Template: tpl,
		Surprise: bits,
		Limbic:   d.chem.Snapshot(),
		Time:     o.At,
	}
	id, err := d.store.Encode(frag)
	if err != nil {
		fmt.Fprintf(d.out, "memory error: %v\n", err)
		return ObserveResult{Bits: bits}
	}
	st.Conscious++
	d.hot.put(frag)
	fmt.Fprintf(d.out, "[!] %.2f bits  %-8s %q -> %q  NA=%.2f  #%d\n",
		bits, o.Channel, clip(o.Content, 140), clip(tpl, 140), frag.Limbic.Noradrenaline, id)

	if bind {
		if linked := d.linkMoment(frag); len(linked) > 0 {
			fmt.Fprintf(d.out, "      ~ #%d linked to %s\n", id, fmtIDs(linked))
		}
		d.recent = append(d.recent, recentMoment{id: frag.ID, template: frag.Template, at: frag.Time})
		if len(d.recent) > 32 {
			d.recent = d.recent[len(d.recent)-32:]
		}
	}
	return ObserveResult{Bits: bits, Conscious: true, FragmentID: id}
}

// linkMoment binds a new fragment to the recent past: every fragment
// encoded within linkWindow gets a bidirectional edge, capped at the most
// recent maxLinkFanout moments.
func (d *Daemon) linkMoment(f *Fragment) []uint64 {
	var targets []recentMoment
	for i := len(d.recent) - 1; i >= 0 && len(targets) < maxLinkFanout; i-- {
		r := d.recent[i]
		if f.Time.Sub(r.at) > linkWindow {
			break
		}
		targets = append(targets, r)
	}
	return d.bindTo(f, targets)
}

// bindTo writes the bidirectional edges between a fragment and its
// targets. Edge strength rides on dopamine, and a template pair that keeps
// co-occurring earns a growing bonus. Confirmation pays one reward spike
// per moment no matter how many pairs confirmed, so a busy stretch cannot
// pump dopamine through sheer fanout. A pair of identical templates still
// binds as episode adjacency but never counts as confirmation, because an
// event repeating itself says nothing about two different things belonging
// together.
func (d *Daemon) bindTo(f *Fragment, targets []recentMoment) []uint64 {
	var linked []uint64
	confirmed := false
	da := d.chem.Snapshot().Dopamine
	for _, r := range targets {
		w := linkBaseWeight + linkDopamineGain*da
		if r.template != f.Template {
			count, err := d.store.BumpAssoc(f.Template, r.template)
			if err != nil {
				fmt.Fprintf(d.out, "assoc error: %v\n", err)
				continue
			}
			if count > 1 {
				confirmed = true
				bonus := float64(count-1) * assocRepeatBonus
				if bonus > 0.5 {
					bonus = 0.5
				}
				w += bonus
			}
		}
		if w > 0.95 {
			w = 0.95
		}
		if err := d.store.Link(f.ID, r.id, w); err != nil {
			fmt.Fprintf(d.out, "link error: %v\n", err)
			continue
		}
		d.store.Link(r.id, f.ID, w)
		linked = append(linked, r.id)
	}
	if confirmed {
		d.chem.SpikeDopamine(confirmReward)
	}
	return linked
}

func fmtIDs(ids []uint64) string {
	parts := make([]string, len(ids))
	for i, id := range ids {
		parts[i] = fmt.Sprintf("#%d", id)
	}
	return strings.Join(parts, ", ")
}

// clip keeps diary lines readable and the log file honest; stored content
// is not affected.
func clip(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "..."
}

// flushStorms turns each channel's suppressed surprises into one summary
// moment once the storm has eased, where eased means the channel's whole
// burst allowance has recovered. Anything sooner would fragment one storm
// into a stream of partial summaries. The summary is forced: a storm is
// always worth one memory, and it binds to the storm's own exemplars
// still sitting in the recent window.
func (d *Daemon) flushStorms(now time.Time) {
	for ch, n := range d.pendingStorm {
		if n == 0 || !d.governor(ch).full(now) {
			continue
		}
		d.pendingStorm[ch] = 0
		d.process(Observation{
			Channel: ch,
			Content: fmt.Sprintf("%s storm: %d surprising lines suppressed", ch, n),
			At:      now,
		}, true, true)
	}
}

// spread walks the link graph outward from seed fragments, decaying
// activation by edge weight and per-hop damping, and returns the context
// cloud. The cloud holds associated moments ranked by how much activation
// reached them, with the budget shared across channels so one loud channel
// cannot monopolize the reply. Edges that carried activation are
// strengthened a little, because recall is practice.
func (d *Daemon) spread(seeds []*Fragment) ([]*FragmentInfo, error) {
	act := map[uint64]float64{}
	seedSet := map[uint64]bool{}
	frontier := make([]uint64, 0, len(seeds))
	for _, s := range seeds {
		act[s.ID] = 1.0
		seedSet[s.ID] = true
		frontier = append(frontier, s.ID)
	}

	var traversed []LinkEdge
	for hop := 0; hop < spreadHops && len(frontier) > 0; hop++ {
		edges, err := d.store.LinksFrom(frontier)
		if err != nil {
			return nil, err
		}
		// Relax against the activations as they stood when the hop began,
		// so the outcome cannot depend on the order edges come back from
		// the store.
		base := make(map[uint64]float64, len(act))
		for id, a := range act {
			base[id] = a
		}
		var next []uint64
		for _, e := range edges {
			a := base[e.From] * e.Weight * spreadDamping
			if a < spreadFloor || a <= act[e.To] {
				continue
			}
			if _, seen := act[e.To]; !seen {
				next = append(next, e.To)
			}
			act[e.To] = a
			traversed = append(traversed, e)
		}
		frontier = next
	}
	if len(traversed) > 0 {
		d.store.StrengthenLinks(traversed, linkPracticeDelta)
	}

	ids := make([]uint64, 0, len(act))
	for id := range act {
		if !seedSet[id] {
			ids = append(ids, id)
		}
	}
	frags, err := d.fragmentsByIDsTiered(ids)
	if err != nil {
		return nil, err
	}
	byActivation := func(i, j int) bool {
		if act[frags[i].ID] != act[frags[j].ID] {
			return act[frags[i].ID] > act[frags[j].ID]
		}
		return frags[i].ID > frags[j].ID
	}
	sort.Slice(frags, byActivation)
	if len(frags) > spreadBudget {
		frags = fairTruncate(frags, spreadBudget)
		sort.Slice(frags, byActivation)
	}
	out := make([]*FragmentInfo, 0, len(frags))
	for _, f := range frags {
		info := fragmentInfo(f)
		info.Activation = act[f.ID]
		out = append(out, info)
	}
	return out, nil
}

// fairTruncate keeps the budget's strongest moments while sharing slots
// round-robin across channels. A burst on one channel therefore cannot
// crowd a quieter channel's contribution out of the reply. The input must
// already be sorted strongest first.
func fairTruncate(frags []*Fragment, budget int) []*Fragment {
	queues := map[string][]*Fragment{}
	var order []string
	for _, f := range frags {
		if _, ok := queues[f.Channel]; !ok {
			order = append(order, f.Channel)
		}
		queues[f.Channel] = append(queues[f.Channel], f)
	}
	picked := make([]*Fragment, 0, budget)
	for len(picked) < budget {
		took := false
		for _, ch := range order {
			if len(queues[ch]) == 0 {
				continue
			}
			picked = append(picked, queues[ch][0])
			queues[ch] = queues[ch][1:]
			took = true
			if len(picked) == budget {
				break
			}
		}
		if !took {
			break
		}
	}
	return picked
}

func (d *Daemon) readyToSleep() bool {
	return d.chem.Serotonin() >= d.cfg.SleepPressureGate &&
		time.Since(d.lastObs) >= d.cfg.InputSilence() &&
		time.Since(d.lastSleep) >= d.cfg.SleepInterval()
}

// sleepCycle is one consolidation night: decay and prune in the store,
// merge recurring shapes into prototypes, flush predictor state, wake
// with reset sleep pressure.
func (d *Daemon) sleepCycle() {
	pruned, err := d.store.Sleep(d.cfg.NightlyDecay, d.cfg.PruneFloor)
	if err != nil {
		fmt.Fprintf(d.out, "sleep error: %v\n", err)
		return
	}
	merged, err := d.store.MergeDuplicates()
	if err != nil {
		fmt.Fprintf(d.out, "merge error: %v\n", err)
	}
	// Consolidation rewrote weights, pruned, and merged underneath any
	// cached copy; the working set rebuilds from what the morning uses.
	d.hot.clear()
	d.flushModels()
	if err := d.store.Checkpoint(); err != nil {
		fmt.Fprintf(d.out, "checkpoint error: %v\n", err)
	}
	d.chem.WakeRested()
	d.sleeps++
	d.lastSleep = time.Now()
	held, _ := d.store.Count()
	fmt.Fprintf(d.out, "zzz night %d: pruned %d faded fragment(s), merged %d into prototypes, %d held.\n",
		d.sleeps, pruned, merged, held)
}

func (d *Daemon) flushModels() {
	for ch, p := range d.predictors {
		if err := d.store.SaveModel(ch, p.Export()); err != nil {
			fmt.Fprintf(d.out, "model flush error on %s: %v\n", ch, err)
		}
	}
	for ch, c := range d.calibrators {
		n, mean, vari := c.Export()
		if err := d.store.SaveCalib(ch, n, mean, vari); err != nil {
			fmt.Fprintf(d.out, "calibration flush error on %s: %v\n", ch, err)
		}
	}
}

func (d *Daemon) shutdown(listener net.Listener, sock string) {
	close(d.stop)
	listener.Close()

	// An in-flight command finishes and its outcome is recorded before
	// the final consolidation, so stopping cannot lose a result. The wait
	// is bounded by the execution timeout.
	d.settleExecution()

	pruned, err := d.store.Sleep(d.cfg.NightlyDecay, d.cfg.PruneFloor)
	if err == nil {
		d.sleeps++
		fmt.Fprintf(d.out, "zzz final night %d: pruned %d fragment(s).\n", d.sleeps, pruned)
	}
	if _, err := d.store.MergeDuplicates(); err != nil {
		fmt.Fprintf(d.out, "merge error: %v\n", err)
	}
	d.flushModels()
	d.store.Checkpoint()
	d.store.Close()
	os.Remove(sock)
	fmt.Fprintln(d.out, "golem asleep.")
}

// tellAbout binds words to remembered moments found by query instead of to
// the present, which lets anyone annotate events after the fact. The
// annotation is stored only when something matches; words about nothing
// teach nothing.
func (d *Daemon) tellAbout(req Request) Response {
	anchors, err := d.store.Recall(req.Query, 3)
	if err != nil {
		return Response{Error: err.Error()}
	}
	if len(anchors) == 0 {
		return Response{Error: fmt.Sprintf("no remembered moment matches %q; the words were not stored", req.Query)}
	}

	res := d.process(Observation{Channel: "tell", Content: req.Content, At: time.Now()}, true, false)
	if res.FragmentID == 0 {
		return Response{Error: "the words could not be stored; check the daemon log"}
	}
	f := &Fragment{ID: res.FragmentID, Template: Normalize(req.Content), Time: time.Now()}
	targets := make([]recentMoment, 0, len(anchors))
	for _, a := range anchors {
		targets = append(targets, recentMoment{id: a.ID, template: a.Template, at: a.Time})
	}
	if linked := d.bindTo(f, targets); len(linked) > 0 {
		fmt.Fprintf(d.out, "      ~ #%d annotates %s\n", f.ID, fmtIDs(linked))
	}

	resp := Response{OK: true, Observe: &res}
	if seeds, err := d.store.FragmentsByIDs([]uint64{f.ID}); err == nil && len(seeds) == 1 {
		if cloud, err := d.spread(seeds); err == nil {
			resp.Fragments = cloud
		}
	}
	return resp
}

// handle services one CLI request inside the brain loop. A deferred reply
// means an execution was started: the reply channel is answered when the
// command's result marshals back through execDone, and the loop must not
// reply itself. The quit return reports whether the daemon should shut
// down.
func (d *Daemon) handle(req Request, reply chan Response) (resp Response, deferred, quit bool) {
	switch req.Cmd {
	case "status":
		held, _ := d.store.Count()
		for name, ctr := range d.drops {
			d.stat(name).Dropped = ctr.Load()
		}
		return Response{OK: true, Status: &StatusInfo{
			PID:            os.Getpid(),
			UptimeSec:      time.Since(d.started).Seconds(),
			Fragments:      held,
			HotFragments:   d.hot.len(),
			Channels:       d.stats,
			Sleeps:         d.sleeps,
			LastSleepUnix:  d.lastSleep.Unix(),
			Chem:           d.chem.Snapshot(),
			PromotionBar:   d.promotionBar(),
			ExecuteEnabled: d.cfg.ExecuteEnabled,
			AllowlistSize:  len(d.cfg.ExecuteAllowlist),
			Running:        d.runningCmd,
		}}, false, false

	case "chem":
		chem := d.chem.Snapshot()
		return Response{OK: true, Chem: &chem}, false, false

	case "observe":
		if req.Channel == "" || req.Content == "" {
			return Response{Error: "observe needs a channel and content"}, false, false
		}
		res := d.process(Observation{Channel: req.Channel, Content: req.Content, At: time.Now()}, false, true)
		return Response{OK: true, Observe: &res}, false, false

	case "tell":
		if req.Content == "" {
			return Response{Error: "tell needs words"}, false, false
		}
		if req.Query != "" {
			return d.tellAbout(req), false, false
		}
		res := d.process(Observation{Channel: "tell", Content: req.Content, At: time.Now()}, true, true)
		if res.FragmentID == 0 {
			return Response{Error: "the words could not be stored; check the daemon log"}, false, false
		}
		resp := Response{OK: true, Observe: &res}
		// What do these words point at? The new fragment just bound to the
		// recent past, so a spread from it is GOLEM's side of the exchange.
		if res.FragmentID != 0 {
			if seeds, err := d.store.FragmentsByIDs([]uint64{res.FragmentID}); err == nil && len(seeds) == 1 {
				if cloud, err := d.spread(seeds); err == nil {
					resp.Fragments = cloud
				}
			}
		}
		return resp, false, false

	case "recall":
		var seeds []*Fragment
		var err error
		if req.Query == "" {
			var f *Fragment
			f, err = d.store.RecallByStress()
			if f != nil {
				seeds = []*Fragment{f}
				d.hot.put(f)
			}
		} else {
			seeds, err = d.recallTiered(req.Query, 3)
		}
		if err != nil {
			return Response{Error: err.Error()}, false, false
		}
		infos := make([]*FragmentInfo, 0, len(seeds))
		for _, f := range seeds {
			info := fragmentInfo(f)
			info.Activation = 1.0
			infos = append(infos, info)
		}
		if len(seeds) > 0 {
			// Memory served its keeper; that is what reward means here.
			d.chem.SpikeDopamine(recallReward)
			if cloud, err := d.spread(seeds); err == nil {
				infos = append(infos, cloud...)
			}
		}
		return Response{OK: true, Fragments: infos}, false, false

	case "dump":
		frags, err := d.store.Dump()
		if err != nil {
			return Response{Error: err.Error()}, false, false
		}
		degrees, _ := d.store.LinkCounts()
		infos := make([]*FragmentInfo, 0, len(frags))
		for _, f := range frags {
			info := fragmentInfo(f)
			info.LinkCount = degrees[f.ID]
			infos = append(infos, info)
		}
		return Response{OK: true, Fragments: infos}, false, false

	case "teach":
		if req.Content == "" || req.Command == "" {
			return Response{Error: "teach needs a cue and a command"}, false, false
		}
		d.teach(req.Content, req.Command, time.Now())
		return Response{OK: true}, false, false

	case "propose":
		if req.Content == "" {
			return Response{Error: "propose needs a cue"}, false, false
		}
		dec, err := d.decide(req.Content)
		if err != nil {
			return Response{Error: err.Error()}, false, false
		}
		di := decisionInfo(dec)
		// A graduated, allowed command is the payoff of graduation: it runs
		// now, with no separate approval, and there is nothing left pending.
		// The run happens off the loop; the reply waits for its outcome.
		if dec.Proposed != nil && !dec.Conflict && dec.Status == HabitGraduated {
			if allowed, _ := d.mayExecute(dec.Proposed.Command); allowed {
				if d.running {
					return Response{Error: "an action is already running (" + d.runningCmd + "); propose again when it finishes"}, false, false
				}
				d.pending = nil
				d.startExec(dec.Proposed.Command, func(ran, ok bool, outcome string) {
					demoted := d.autorunFinish(dec, "", ran, ok, outcome, time.Now())
					di.Autonomous = true
					di.Outcome = outcome
					di.Demoted = demoted
					reply <- Response{OK: true, Decision: di}
				})
				return Response{}, true, false
			}
		}
		d.pending = &dec
		return Response{OK: true, Decision: di}, false, false

	case "approve":
		if d.pending == nil || d.pending.Proposed == nil {
			return Response{Error: "no proposal is pending"}, false, false
		}
		if d.pending.Conflict {
			return Response{Error: "the pending proposal is ambiguous; clarify and re-propose first"}, false, false
		}
		dec := *d.pending
		cmd := dec.Proposed.Command
		allowed, reason := d.mayExecute(cmd)
		if allowed && d.running {
			// The proposal stays pending, so the operator can just retry.
			return Response{Error: "an action is already running (" + d.runningCmd + "); approve again when it finishes"}, false, false
		}
		if err := d.approveStart(dec); err != nil {
			return Response{Error: err.Error()}, false, false
		}
		d.pending = nil
		if !allowed {
			outcome := dryRun(reason, cmd)
			h, err := d.approveFinish(dec, false, true, outcome, time.Now())
			if err != nil {
				return Response{Error: err.Error()}, false, false
			}
			return Response{OK: true, Action: &ActionResult{
				Command: cmd, Outcome: outcome,
				Status: h.Status, Approvals: h.Approvals, Denials: h.Denials,
			}}, false, false
		}
		d.startExec(cmd, func(ran, ok bool, outcome string) {
			h, err := d.approveFinish(dec, ran, ok, outcome, time.Now())
			if err != nil {
				reply <- Response{Error: err.Error()}
				return
			}
			reply <- Response{OK: true, Action: &ActionResult{
				Command: cmd, Outcome: outcome,
				Status: h.Status, Approvals: h.Approvals, Denials: h.Denials,
			}}
		})
		return Response{}, true, false

	case "deny":
		if d.pending == nil || d.pending.Proposed == nil {
			return Response{Error: "no proposal is pending"}, false, false
		}
		if d.pending.Conflict {
			return Response{Error: "the pending proposal is ambiguous; clarify and re-propose first"}, false, false
		}
		dec := *d.pending
		h, err := d.deny(dec, time.Now())
		if err != nil {
			return Response{Error: err.Error()}, false, false
		}
		d.pending = nil
		return Response{OK: true, Action: &ActionResult{
			Command: dec.Proposed.Command, Outcome: "denied",
			Status: h.Status, Approvals: h.Approvals, Denials: h.Denials,
		}}, false, false

	case "habits":
		habits, err := d.store.AllHabits()
		if err != nil {
			return Response{Error: err.Error()}, false, false
		}
		out := make([]*HabitInfo, 0, len(habits))
		for _, h := range habits {
			out = append(out, &HabitInfo{Cue: h.Cue, Action: h.Action,
				Status: h.Status, Approvals: h.Approvals, Denials: h.Denials})
		}
		return Response{OK: true, Habits: out}, false, false

	case "audit":
		entries, err := d.store.AuditTail(20)
		if err != nil {
			return Response{Error: err.Error()}, false, false
		}
		out := make([]*AuditInfo, 0, len(entries))
		for _, e := range entries {
			out = append(out, &AuditInfo{AtUnix: e.At, Cue: e.Cue, Action: e.Action, Outcome: e.Outcome})
		}
		return Response{OK: true, Audit: out}, false, false

	case "stop":
		return Response{OK: true}, false, true

	default:
		return Response{Error: "unknown command: " + req.Cmd}, false, false
	}
}

func decisionInfo(dec Decision) *DecisionInfo {
	di := &DecisionInfo{Cue: dec.Cue}
	if dec.Proposed == nil {
		return di
	}
	di.Grounded = true
	di.Proposed = dec.Proposed.Command
	di.Saliency = dec.Proposed.Saliency
	di.Status = dec.Status
	di.Denylisted = isDenylisted(dec.Proposed.Command)
	di.Conflict = dec.Conflict
	di.Question = dec.Question
	for _, a := range dec.Alternatives {
		di.Alternatives = append(di.Alternatives, a.Command)
	}
	return di
}
