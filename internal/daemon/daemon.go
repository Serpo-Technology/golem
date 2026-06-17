// Package daemon is the brain loop: one goroutine that owns every piece
// of mutable state, fed by sensors and the socket, deciding serially,
// acting in parallel under claims, and consolidating in its sleep. The
// other packages are organs; this one is where they meet.
package daemon

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"maps"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"golem/internal/attention"
	"golem/internal/code"
	"golem/internal/config"
	"golem/internal/events"
	"golem/internal/grammar"
	"golem/internal/limbic"
	"golem/internal/memory"
	"golem/internal/pipeline"
	"golem/internal/runtime"
	"golem/internal/senses"
	"golem/wire"
)

// Daemon is the long-running form of GOLEM: the same observe/encode/sleep
// loop as the demo, but multi-channel, persistent, and queryable over a
// unix socket. All state mutation happens inside the single Run loop, so
// the brain stays single-threaded no matter how many sensors or CLI
// clients are talking to it.
type Daemon struct {
	home string
	cfg  config.Config
	out  io.Writer

	store       *memory.Store
	chem        *limbic.System
	predictors  map[string]*attention.Predictor
	calibrators map[string]*attention.Calibrator
	stats       map[string]*wire.ChannelStat
	drops       map[string]*atomic.Int64

	obs  chan senses.Observation
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
	hot *memory.HotTier

	// ws is working memory: decaying activation over concepts, the
	// referent pool for "that one" and "do it again."
	ws *workspace

	// held is propositional working memory: the propositions GOLEM holds in
	// mind that are not graph facts, an inference it drew or a supposition it
	// entertains, so a chain can build on a held step. It decays with ws and
	// clears at sleep.
	held *heldStore

	// universalFeats names the features carried by nearly every concept, the ones
	// that discriminate nothing, mapped to how universal each is. The resemblance
	// walk passes them by so a shared channel or a baseline stress cannot make
	// every concept resemble every other. Refreshed at consolidation, loaded at
	// boot.
	universalFeats map[uint64]float64

	// gram is the language faculty's working copy: grammar parameters
	// read out of the graph and compiled for the engine, rebuilt at
	// wake after the nightly competition with the lexicon.
	gram *grammar.Params

	// code is the programming-language faculty: code constructions compiled
	// from the seeded nodes, comprehending a computation from a surface and
	// generating the source to run it.
	code *code.Engine

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

	// Committed actions fan out as goroutines, the parallel substrate
	// under a serial arbiter: decision stays on the loop, running maps
	// each in-flight action to the resource claims it holds, waiting
	// parks actions whose claims collide or that found no free slot,
	// and every outcome marshals back through execDone so habit, audit,
	// and feedback bookkeeping stay single-threaded.
	running   map[uint64]*actionRun
	nextRunID uint64
	waiting   []*queuedAction
	execDone  chan execResult

	// Thinking fans out the way actions do: every setting of the walk
	// primitive computes on a snapshot and posts through one channel,
	// the loop arbitrates and commits. thinking counts the open round's
	// in-flight operations, thinkEpoch stamps rounds so sleep can
	// invalidate stale ones, thinkSeed remembers what the round was
	// about, and ops caches the operation nodes whose weights hold each
	// operation's learned worth.
	thinkDone     chan thoughtCandidate
	thinking      int
	thinkEpoch    uint64
	thinkRound    []thoughtCandidate
	thinkSeed     uint64
	thinkSeedHeat float64
	lastThought   string
	lastThoughtAt time.Time
	// recentThoughts is the refractory ring: thoughts that re-entered
	// perception lately, so the loop cannot ruminate one line into memory.
	recentThoughts []string
	ops            map[string]*memory.Fragment

	// The faculties fire from the same arbiter the walks use. lastAct
	// holds each act's refractory, and encodesSinceSleep is the clutter
	// the tidiness acts read.
	lastAct           map[string]time.Time
	encodesSinceSleep int

	// goal is the one intention held across steps: adopted when GOLEM
	// asks about a word that points at nothing, pursued by the drives,
	// settled by checkGoal.
	goal *goalState

	// goalsSatisfied and goalsAbandoned tally how goals resolve, the goal
	// loop's contribution to the grounded fitness: the share satisfied is one
	// outcome a self-change is judged against.
	goalsSatisfied int
	goalsAbandoned int

	// topic is the subject of the exchange, the dominant warm concept
	// followed across turns; when a new one takes the field the change is
	// noticed. Held on the daemon because a conversation outlives one turn.
	topic uint64

	// lastVolunteer paces unprompted remarks: GOLEM offers a conclusion of its
	// own now and then when the room is quiet, not in a stream.
	lastVolunteer time.Time

	// lastDrewTopic is the topic GOLEM last drew inferences about while idle, so
	// it reasons about a subject once when it takes the field rather than every
	// tick. Cleared at sleep with the topic.
	lastDrewTopic uint64

	// lastRole is the thread a role question leaves behind: the relation it
	// asked about, so an elliptical follow-up can ask the same of a new
	// referent. It is meaning, not text, so the thread holds whatever language
	// each turn arrives in.
	lastRole *roleContext

	// threadProps are the propositions the current exchange has set down, the
	// discourse memory recall leans toward so what was just said wins over what
	// is worn deepest into the graph, without changing the long-term weight.
	threadProps []threadProp

	// topicTrail is the recent subjects the exchange has moved through, newest
	// first and distinct, so a conversation can return to what it was on before
	// the current subject took the field. Cleared at sleep with the topic.
	topicTrail []uint64

	// turnFocus is the concept the current turn is explicitly about, the subject
	// it named rather than what the answer happened to warm. The topic follows it
	// when set, so a subject the speaker raised holds the field over a neighbour
	// the reply lit up. Set while planning a turn, read and cleared by trackTopic.
	turnFocus uint64

	// claims are the propositions GOLEM has asserted this exchange and how firmly,
	// the speaker's-side record of the discourse to threadProps's listener side:
	// what it has committed to, so it can stand by a claim, note having said it,
	// and not quietly contradict itself. Cleared at sleep with the rest.
	claims []claim

	// openQuestions are the questions raised this exchange that GOLEM could not
	// answer, held so it can follow up when the answer later arrives. Cleared at
	// sleep. followUp carries a resolution back to the planner: when a told
	// proposition answers an open question, GOLEM speaks the answer on that turn
	// rather than staying silent.
	openQuestions []openQuestion
	followUp      string
	proposedKinds map[string]bool // un-named kinds already offered this session, so each is proposed once

	// selfID, userID, and bothID are the conversation's participants as concepts:
	// GOLEM, the one it speaks with, and the two together. The deictic pronouns
	// resolve to them ("you" is GOLEM, "I" the user) so a turn about the speakers
	// is reasoned over like any other, and GOLEM can be asked about itself.
	selfID uint64
	userID uint64
	bothID uint64

	// lastMode is the determination the loop last carried out (consolidate,
	// pursue, idle), the inertia a held goal leans on and the transition the
	// trail records. lastPursuit paces the active mulling of a goal's topic.
	lastMode    string
	lastPursuit time.Time

	// runner runs the code GOLEM writes, the runtime.Runner seam: nil falls to
	// the embedded interpreter, and a compiling toolchain stands here when
	// GOLEM comes to rebuild itself.
	runner runtime.Runner

	// voiceOn gates whether spoken replies are emitted as say events for an
	// external speaker; voiceName is the voice an adapter should prefer. Both
	// start from config and follow golem voice at runtime, owned by the brain
	// loop like every other mutable state.
	voiceOn   bool
	voiceName string

	// events is the structured trail (nil when it could not open; the
	// brain never dies for its diary).
	events *events.Log

	// netListeners holds any transport beyond the unix socket, a TCP address
	// for transducers outside the process, closed alongside it at shutdown.
	netListeners []net.Listener

	// subs are the open event-stream subscribers (a speaker outside the
	// process, say), fed each event as it happens. The mutex guards the set,
	// touched by the brain loop on fan-out and by connection goroutines on
	// subscribe and disconnect.
	subsMu sync.Mutex
	subs   map[chan events.Event]struct{}

	// media is the content-addressed store for the raw bytes a perceptual
	// moment was captioned from, kept so a transducer can look again.
	media *memory.MediaStore
}

// actionRun is one command in flight and the claims it holds.
type actionRun struct {
	cmd    string
	claims []string
}

// queuedAction is a committed command waiting its turn: start launches
// it once its claims come free, drop resolves it without running when
// the daemon stops first.
type queuedAction struct {
	cmd    string
	claims []string
	start  func()
	drop   func(reason string)
}

// waitingCap bounds the action queue. A queue this deep means something
// is wrong upstream, and refusing loudly beats hoarding stale intents.
const waitingCap = 16

// event stamps and appends one line to the structured trail. A write
// error closes the trail rather than spamming the daemon log.
func (d *Daemon) event(ev events.Event) {
	ev.At = time.Now().UnixMilli()
	// Offer it to any live subscriber first, so an external speaker hears it
	// even on a brain whose diary could not open.
	d.fanout(ev)
	if d.events == nil {
		return
	}
	if err := d.events.Append(ev); err != nil {
		fmt.Fprintf(d.out, "event log failed, closing it: %v\n", err)
		d.events.Close()
		d.events = nil
	}
}

// execResult is a finished command on its way back into the brain loop.
// finish completes the habit, audit, feedback, and reply work that must
// run single-threaded, so the loop calls it, never the exec goroutine.
type execResult struct {
	id      uint64
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
	// senses.Observation timestamps can lag the bucket's creation; time only
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
	heat     float64 // how warm the concept lending this moment is, for weighting an attention bond
}

// untrustedWeight is the starting weight of a moment from an untrusted
// channel: half a memory, gone in half the nights.
const untrustedWeight = 0.5

// untrustedNameScale is how lightly a word read on an untrusted channel bonds,
// so reading grows the vocabulary only by recurrence: a word met across many of
// the watched sources entrenches, while a word from one page fades.
const untrustedNameScale = 0.5

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

// Successor tuning. Binding is asymmetric by spike timing: the moment
// that fired first strengthens its edge toward what followed, and no
// reverse edge is written at all. Repetition of the same ordered template
// pair earns a growing bonus, the directional twin of assocRepeatBonus.
const (
	successorBaseWeight  = 0.25
	successorRepeatBonus = 0.1
	successorMaxWeight   = 0.95
)

type ipcReq struct {
	req   wire.Request
	reply chan wire.Response
}

// New opens a daemon over the given home directory: store, models,
// chemistry, and the event trail, ready for Run.
func New(home string, cfg config.Config, out io.Writer) (*Daemon, error) {
	if err := os.MkdirAll(home, 0o755); err != nil {
		return nil, err
	}
	store, err := memory.OpenStore(filepath.Join(home, "memory.db"))
	if err != nil {
		return nil, err
	}
	mediaStore, err := memory.OpenMediaStore(home)
	if err != nil {
		store.Close()
		return nil, err
	}
	d := &Daemon{
		home:         home,
		cfg:          cfg,
		out:          out,
		store:        store,
		media:        mediaStore,
		chem:         limbic.New(),
		predictors:   make(map[string]*attention.Predictor),
		calibrators:  make(map[string]*attention.Calibrator),
		stats:        make(map[string]*wire.ChannelStat),
		drops:        make(map[string]*atomic.Int64),
		governors:    make(map[string]*tokenBucket),
		pendingStorm: make(map[string]int),
		obs:          make(chan senses.Observation, 256),
		reqs:         make(chan ipcReq),
		stop:         make(chan struct{}),
		running:      make(map[uint64]*actionRun),
		execDone:     make(chan execResult, 16),
		thinkDone:    make(chan thoughtCandidate, 8),
		hot:          memory.NewHotTier(),
		ws:           newWorkspace(),
		held:         newHeldStore(),
		voiceOn:      cfg.Channels.Voice,
		voiceName:    cfg.Channels.VoiceName,
	}
	if err := d.loadModels(); err != nil {
		store.Close()
		return nil, err
	}
	if err := d.loadGrammar(); err != nil {
		store.Close()
		return nil, err
	}
	// Conclusions a past run kept return to mind, so a believed proposition is
	// remembered across a restart the way the graph remembers what was seen.
	d.loadBeliefs()
	// The features a past run named universal are passed over from the first
	// resemblance, not from the first night.
	d.loadUniversalFeats()
	// The conversation's participants are in mind from the first turn, so "you"
	// and "I" resolve to someone and GOLEM can be asked about itself.
	d.ensureParticipants()
	if events, err := events.Open(home); err == nil {
		d.events = events
	} else {
		fmt.Fprintf(out, "event log unavailable: %v\n", err)
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
		p := attention.NewPredictor(d.windowFor(ch))
		p.Seed(counts)
		d.predictors[ch] = p
	}
	calibs, err := d.store.LoadCalibs()
	if err != nil {
		return err
	}
	for ch, cs := range calibs {
		c := &attention.Calibrator{}
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

func (d *Daemon) predictor(channel string) *attention.Predictor {
	p, ok := d.predictors[channel]
	if !ok {
		p = attention.NewPredictor(d.windowFor(channel))
		d.predictors[channel] = p
	}
	return p
}

// burstFor resolves a channel's encode burst, the per-channel override before
// the global default, so a bulk channel like the web takes in more per cycle.
func (d *Daemon) burstFor(channel string) float64 {
	if b, ok := d.cfg.EncodeBursts[channel]; ok {
		return b
	}
	return d.cfg.EncodeBurst
}

func (d *Daemon) governor(channel string) *tokenBucket {
	b, ok := d.governors[channel]
	if !ok {
		burst := d.burstFor(channel)
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

func (d *Daemon) calibrator(channel string) *attention.Calibrator {
	c, ok := d.calibrators[channel]
	if !ok {
		c = &attention.Calibrator{}
		d.calibrators[channel] = c
	}
	return c
}

func (d *Daemon) stat(channel string) *wire.ChannelStat {
	st, ok := d.stats[channel]
	if !ok {
		st = &wire.ChannelStat{}
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

// ensureParticipants makes sure the conversation's participants exist as concepts
// and caches their ids: GOLEM itself, the one it speaks with, and the two
// together. They are persistent and discourse-grounded rather than perceptual, the
// referents the deictic pronouns ("you", "I", "we") point at.
func (d *Daemon) ensureParticipants() {
	d.selfID = d.ensureConcept("deixis:self", "GOLEM")
	d.userID = d.ensureConcept("deixis:user", "the speaker")
	d.bothID = d.ensureConcept("deixis:both", "us")
}

// ensureConcept returns the id of the concept with a template, creating it if
// absent, so a singleton concept is stable across restarts.
func (d *Daemon) ensureConcept(template, content string) uint64 {
	if c, err := d.store.ConceptByTemplate(template); err == nil && c != nil {
		return c.ID
	}
	c := &memory.Fragment{Kind: memory.KindConcept, Channel: "concept", Content: content,
		Template: template, Category: "noun", Limbic: d.chem.Snapshot(), Time: time.Now()}
	if _, err := d.store.Encode(c); err != nil {
		fmt.Fprintf(d.out, "participant concept error: %v\n", err)
		return 0
	}
	return c.ID
}

// Run blocks until the daemon is stopped by a signal or a stop command.
// Shutdown always runs a final consolidation and flushes predictor state.
func (d *Daemon) Run() error {
	sock := wire.SocketPath(d.home)
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

	// A TCP transport, when configured, serves the same protocol across a
	// container boundary so external senses and a speaker can reach the brain.
	if addr := strings.TrimSpace(d.cfg.ListenTCP); addr != "" {
		tcp, err := net.Listen("tcp", addr)
		if err != nil {
			listener.Close()
			os.Remove(sock)
			d.store.Close()
			return err
		}
		d.netListeners = append(d.netListeners, tcp)
		go d.acceptLoop(tcp)
		fmt.Fprintf(d.out, "golem also listening on tcp %s\n", tcp.Addr())
	}

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
			res := d.process(o, false, true)
			// Reading grows the vocabulary: an untrusted text channel mints the
			// words it took in, lightly, so the lexicon learns from the page
			// without the page gaining the authority to act.
			if res.FragmentID != 0 && d.cfg.Untrusted(o.Channel) {
				d.readVocabulary(o.Content, res.FragmentID)
			}

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
			d.finishRun(res)

		case cand := <-d.thinkDone:
			d.absorbThought(cand)

		case <-homeostasis.C:
			now := time.Now()
			d.chem.Decay()
			d.ws.decay()
			d.held.decay()
			d.flushStorms(now)
			if time.Since(d.lastSurprise) > d.cfg.CalmBeforeSleep() {
				d.chem.BuildSleepPressure()
			}
			// One determination each moment, ranked by chemistry with idle
			// the floor: pursue a goal, consolidate, or rest and wander. One
			// think round at a time keeps the inner life from crowding sensing.
			d.carryOut(d.determine(now), now)

		case <-sigc:
			fmt.Fprintln(d.out, "signal received; sleeping once, then shutting down.")
			d.shutdown(listener, sock)
			return nil
		}
	}
}

// parallelCap is how many committed actions may fan out at once.
func (d *Daemon) parallelCap() int {
	if d.cfg.ExecuteParallel < 1 {
		return 1
	}
	return d.cfg.ExecuteParallel
}

// claimConflict reports the running command holding one of the given
// claims, if any does. This is effector exclusion: one hand cannot grab
// two things, so two actions touching the same resource never overlap.
func (d *Daemon) claimConflict(claims []string) (string, bool) {
	for _, run := range d.running {
		for _, want := range claims {
			if slices.Contains(run.claims, want) {
				return run.cmd, true
			}
		}
	}
	return "", false
}

// canStart reports whether a committed command could fan out right now:
// a slot free under the cap and no claim held by a running action.
func (d *Daemon) canStart(claims []string) bool {
	if len(d.running) >= d.parallelCap() {
		return false
	}
	_, conflict := d.claimConflict(claims)
	return !conflict
}

// commitAction is the serial arbiter's hand-off to the parallel
// substrate: the decision already chose this command, and the only
// question left is when it may run. It starts now when its claims are
// free under the cap, parks in arrival order when not, and errors only
// when the wait queue itself is full. finish resolves a completed run;
// drop resolves a parked action that the daemon stops before starting.
func (d *Daemon) commitAction(cmd string, finish func(ran, ok bool, outcome string), drop func(reason string)) (queued bool, err error) {
	claims := pipeline.ClaimsOf(cmd)
	if d.canStart(claims) {
		d.startExec(cmd, claims, finish)
		return false, nil
	}
	if len(d.waiting) >= waitingCap {
		return false, errors.New("the action queue is full; try again when something finishes")
	}
	d.waiting = append(d.waiting, &queuedAction{
		cmd:    cmd,
		claims: claims,
		start:  func() { d.startExec(cmd, claims, finish) },
		drop:   drop,
	})
	return true, nil
}

// startExec runs one command off the brain loop, holding its claims in
// the run table until the outcome marshals back. The goroutine touches
// no daemon state: it runs the command and posts the result to execDone,
// where the loop calls finish single-threaded. While commands run the
// loop keeps sensing and answering.
func (d *Daemon) startExec(cmd string, claims []string, finish func(ran, ok bool, outcome string)) {
	d.nextRunID++
	id := d.nextRunID
	d.running[id] = &actionRun{cmd: cmd, claims: claims}
	timeout := d.execTimeout()
	go func() {
		ran, ok, outcome := pipeline.RunCommand(cmd, timeout)
		d.execDone <- execResult{id: id, ran: ran, ok: ok, outcome: outcome, finish: finish}
	}()
}

// finishRun reabsorbs one completed action: release its claims, finish
// its bookkeeping single-threaded, and start whatever the freed claims
// unblocked.
func (d *Daemon) finishRun(res execResult) {
	delete(d.running, res.id)
	res.finish(res.ran, res.ok, res.outcome)
	d.pumpQueue()
}

// pumpQueue starts every waiting action whose claims have come free, in
// arrival order. An action whose claims are still held stays parked
// without blocking a free sibling behind it.
func (d *Daemon) pumpQueue() {
	kept := d.waiting[:0]
	for _, q := range d.waiting {
		if d.canStart(q.claims) {
			q.start()
			continue
		}
		kept = append(kept, q)
	}
	d.waiting = kept
}

// settleExecution drains every in-flight command the way the Run loop
// would and resolves the wait queue without running it: an action that
// never started must not start while the daemon is going down. Only
// loopless drivers call it: shutdown, demos, and tests.
func (d *Daemon) settleExecution() {
	for _, q := range d.waiting {
		q.drop("daemon stopping")
	}
	d.waiting = nil
	for len(d.running) > 0 {
		res := <-d.execDone
		delete(d.running, res.id)
		res.finish(res.ran, res.ok, res.outcome)
	}
}

// drainActions completes every running and queued action the way the
// Run loop would, queue included. It serves drivers that want the work
// done rather than abandoned: demos and tests.
func (d *Daemon) drainActions() {
	for len(d.running) > 0 {
		d.finishRun(<-d.execDone)
	}
}

// handleNow services one request synchronously, completing any execution
// it starts (or parks) before returning. It is the entry point for demos
// and tests that drive the daemon without its Run loop.
func (d *Daemon) handleNow(req wire.Request) (wire.Response, bool) {
	reply := make(chan wire.Response, 1)
	resp, deferred, quit := d.handle(req, reply)
	if !deferred {
		return resp, quit
	}
	for {
		select {
		case resp := <-reply:
			return resp, quit
		case res := <-d.execDone:
			d.finishRun(res)

		case cand := <-d.thinkDone:
			d.absorbThought(cand)
		}
	}
}

// startSensors launches the built-in senses the config asks for. Every
// sensor is a goroutine that emits into the shared workspace queue and
// stops when d.stop closes. Drop counters are created here, before the
// goroutines exist, so the map is never written concurrently.
func (d *Daemon) startSensors() {
	var active []string
	ch := d.cfg.Channels
	if ch.Synthetic {
		go senses.LogSensor("synthetic", d.obs, d.stop, d.dropCounter("synthetic"))
		active = append(active, "synthetic")
	}
	if len(ch.WatchDirs) > 0 {
		go senses.FileSensor(ch.WatchDirs, d.obs, d.stop, d.dropCounter("files"))
		active = append(active, fmt.Sprintf("files(%d dirs)", len(ch.WatchDirs)))
	}
	if ch.Vitals {
		go senses.VitalsSensor(d.obs, d.stop, d.dropCounter("vitals"))
		active = append(active, "vitals")
	}
	if ch.Syslog {
		go senses.SyslogSensor(d.obs, d.stop, d.dropCounter("syslog"))
		active = append(active, "syslog")
	}
	if len(ch.TailFiles) > 0 {
		senses.TailSensor(ch.TailFiles, d.obs, d.stop, d.dropCounter("reading"))
		active = append(active, fmt.Sprintf("reading(%d files)", len(ch.TailFiles)))
	}
	if len(ch.WatchURLs) > 0 {
		go senses.WebSensor(ch.WatchURLs, d.cfg.WebPoll(), d.obs, d.stop, d.dropCounter("web"))
		active = append(active, fmt.Sprintf("web(%d urls, untrusted)", len(ch.WatchURLs)))
	}
	if ch.Screen {
		go senses.ScreenSensor(d.obs, d.stop, d.dropCounter("screen"))
		active = append(active, "screen")
	}
	if ch.Voice {
		active = append(active, "voice(out)")
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
	// The deadline must outlast a command that waits out a conflicting
	// run before taking its own full time, or the client would give up
	// on a request that is still queued or executing.
	conn.SetDeadline(time.Now().Add(2*d.cfg.ExecTimeout() + 15*time.Second))

	line, err := bufio.NewReader(conn).ReadBytes('\n')
	if err != nil {
		return
	}
	var req wire.Request
	resp := wire.Response{}
	if err := json.Unmarshal(line, &req); err != nil {
		resp.Error = "bad request: " + err.Error()
	} else if req.Cmd == "subscribe" {
		// A subscription holds the connection open and streams events as they
		// happen, the output channel a speaker reads; it never goes through the
		// request queue and outlasts the request deadline.
		conn.SetDeadline(time.Time{})
		d.streamEvents(conn)
		return
	} else {
		reply := make(chan wire.Response, 1)
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
func (d *Daemon) process(o senses.Observation, forced, bind bool) wire.ObserveResult {
	// The quiet timers track the world: a thought is the mind's own
	// activity, and mind-wandering must neither hold off sleep's
	// input-silence test nor throttle itself.
	if o.Channel != thoughtChannel {
		d.lastObs = time.Now()
	}
	st := d.stat(o.Channel)
	st.Observed++

	tpl := attention.Normalize(o.Content)
	p := d.predictor(o.Channel)
	bits := p.Surprise(tpl)
	p.Learn(tpl)

	// The reflex arc runs before the gate: a graduated habit must fire on
	// its cue even when the cue has become too routine to reach
	// consciousness. Speech is excluded (the operator has golem propose),
	// so are action echoes: the feedback channel and shell lines that
	// are themselves commands, or an outcome could re-cue its own habit,
	// and so is every untrusted channel, because a fetched page that
	// happens to contain a cue must never move the hands.
	if !forced && o.Channel != "action" && o.Channel != thoughtChannel &&
		!d.cfg.Untrusted(o.Channel) &&
		!strings.HasPrefix(o.Content, senses.ActionPrefix) {
		d.autoCue(o, tpl)
	}

	// The gate is channel-relative: the same bit count that is routine on
	// a diverse channel is an alarm on a monotonous one.
	if !forced &&
		!d.calibrator(o.Channel).Gate(bits, d.cfg.SurpriseThreshold, d.cfg.ZGate, d.cfg.CalibrationWarmup) {
		return wire.ObserveResult{Bits: bits}
	}

	// The refractory: past the burst allowance, a storm of surprises is
	// counted instead of encoded, and later collapsed into one summary
	// moment. The predictor and calibrator have already learned the line.
	if !forced && !d.governor(o.Channel).take(o.At) {
		st.Suppressed++
		d.pendingStorm[o.Channel]++
		return wire.ObserveResult{Bits: bits}
	}

	if o.Channel != thoughtChannel {
		d.lastSurprise = time.Now()
	}
	d.chem.SpikeNoradrenaline(bits * limbic.NASpikePerBit)
	d.chem.SuppressSleep(limbic.SleepSuppressOnSurprise)

	frag := &memory.Fragment{
		Channel:  o.Channel,
		Content:  o.Content,
		Template: tpl,
		Surprise: bits,
		Limbic:   d.chem.Snapshot(),
		Time:     o.At,
	}
	if d.cfg.Untrusted(o.Channel) {
		// Open-world perception is remembered lightly: it fades in
		// fewer nights and never carries the authority of lived
		// experience.
		frag.Weight = untrustedWeight
	}
	id, err := d.store.Encode(frag)
	if err != nil {
		fmt.Fprintf(d.out, "memory error: %v\n", err)
		return wire.ObserveResult{Bits: bits}
	}
	st.Conscious++
	d.encodesSinceSleep++
	d.hot.Put(frag)
	// A moment whose shape has a concept warms that concept: experience
	// keeps working memory pointed at what is happening. The warming scales
	// with the concept's information, so a high-frequency low-information
	// node cannot pull the inner loop above real knowledge.
	if c, err := d.store.ConceptByTemplate(tpl); err == nil && c != nil {
		d.ws.excite(c.ID, exciteEncode*infoScale(c.Surprise))
	}
	d.event(events.Event{Ev: "encode", Channel: o.Channel, ID: id, Bits: bits, Tpl: wire.Clip(tpl, 120)})
	fmt.Fprintf(d.out, "[!] %.2f bits  %-8s %q -> %q  NA=%.2f  #%d\n",
		bits, o.Channel, wire.Clip(o.Content, 140), wire.Clip(tpl, 140), frag.Limbic.Noradrenaline, id)

	if bind {
		if linked := d.linkMoment(frag); len(linked) > 0 {
			fmt.Fprintf(d.out, "      ~ #%d linked to %s\n", id, fmtIDs(linked))
		}
		d.recent = append(d.recent, recentMoment{id: frag.ID, template: frag.Template, at: frag.Time})
		if len(d.recent) > 32 {
			d.recent = d.recent[len(d.recent)-32:]
		}
	}
	return wire.ObserveResult{Bits: bits, Conscious: true, FragmentID: id}
}

// linkMoment binds a new fragment to the recent past: every fragment
// encoded within linkWindow gets a bidirectional edge, capped at the most
// recent maxLinkFanout moments.
func (d *Daemon) linkMoment(f *memory.Fragment) []uint64 {
	return d.bindTo(f, d.recentWithin(f.Time, f.ID))
}

// recentWithin returns the moments of the live binding window as of a
// time, newest first, capped at the link fanout. The excluded id lets a
// caller leave a fragment already in the window out of its own targets.
func (d *Daemon) recentWithin(at time.Time, exclude uint64) []recentMoment {
	var targets []recentMoment
	for i := len(d.recent) - 1; i >= 0 && len(targets) < maxLinkFanout; i-- {
		r := d.recent[i]
		if at.Sub(r.at) > linkWindow {
			break
		}
		if r.id == exclude {
			continue
		}
		targets = append(targets, r)
	}
	return targets
}

// recentNaming is recentWithin over the tighter naming window: a told word
// grounds only in what was perceived in the last breath, not across the whole
// adjacency window. Episode adjacency still spans linkWindow, so "the door opens,
// then the bell rings" keeps its succession, but the word does not bind to the
// scene before, which is what let a name taught now catch what lingered.
func (d *Daemon) recentNaming(at time.Time, exclude uint64) []recentMoment {
	var targets []recentMoment
	for i := len(d.recent) - 1; i >= 0 && len(targets) < maxLinkFanout; i-- {
		r := d.recent[i]
		if at.Sub(r.at) > attentionRecency {
			break
		}
		if r.id == exclude {
			continue
		}
		targets = append(targets, r)
	}
	return targets
}

// bindTo writes the bidirectional edges between a fragment and its
// targets. Edge strength rides on dopamine, and a template pair that keeps
// co-occurring earns a growing bonus. Confirmation pays one reward spike
// per moment no matter how many pairs confirmed, so a busy stretch cannot
// pump dopamine through sheer fanout. A pair of identical templates still
// binds as episode adjacency but never counts as confirmation, because an
// event repeating itself says nothing about two different things belonging
// together.
//
// Order is bound alongside co-occurrence: each target fired before f, so
// a directed successor edge runs from the target toward f and nothing
// runs back. That timing asymmetry is what lets "what follows X" have a
// different answer than "what precedes X."
func (d *Daemon) bindTo(f *memory.Fragment, targets []recentMoment) []uint64 {
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
		d.event(events.Event{Ev: "link", From: f.ID, To: r.id, Weight: w})
		linked = append(linked, r.id)
		d.bindSuccessor(r, f)
	}
	if confirmed {
		d.chem.SpikeDopamine(confirmReward)
	}
	return linked
}

// bindSuccessor writes the directed order edge from an earlier moment to
// the one that just followed it. Identical templates are skipped: an
// event following itself is repetition, and repetition is the merge
// pass's business.
func (d *Daemon) bindSuccessor(earlier recentMoment, f *memory.Fragment) {
	if earlier.template == f.Template {
		return
	}
	count, err := d.store.BumpSucc(earlier.template, f.Template)
	if err != nil {
		fmt.Fprintf(d.out, "succ error: %v\n", err)
		return
	}
	w := successorBaseWeight + float64(count-1)*successorRepeatBonus
	if w > successorMaxWeight {
		w = successorMaxWeight
	}
	if err := d.store.LinkRel(earlier.id, f.ID, memory.RelSuccessor, w); err != nil {
		fmt.Fprintf(d.out, "successor link error: %v\n", err)
		return
	}
	d.event(events.Event{Ev: "link", From: earlier.id, To: f.ID, Rel: memory.RelSuccessor, Weight: w})
}

// runningCmds lists the in-flight commands oldest first, for status.
func (d *Daemon) runningCmds() []string {
	ids := make([]uint64, 0, len(d.running))
	for id := range d.running {
		ids = append(ids, id)
	}
	slices.Sort(ids)
	out := make([]string, 0, len(ids))
	for _, id := range ids {
		out = append(out, d.running[id].cmd)
	}
	return out
}

// dedupeInfos keeps the first appearance of each fragment in a reply, so
// a node reached as seed, mediator, and cloud member shows up once, in
// its strongest role.
func dedupeInfos(infos []*wire.FragmentInfo) []*wire.FragmentInfo {
	seen := make(map[uint64]bool, len(infos))
	out := infos[:0]
	for _, info := range infos {
		if seen[info.ID] {
			continue
		}
		seen[info.ID] = true
		out = append(out, info)
	}
	return out
}

func fmtIDs(ids []uint64) string {
	parts := make([]string, len(ids))
	for i, id := range ids {
		parts[i] = fmt.Sprintf("#%d", id)
	}
	return strings.Join(parts, ", ")
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
		d.process(senses.Observation{
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
func (d *Daemon) spread(seeds []*memory.Fragment) ([]*wire.FragmentInfo, error) {
	act := map[uint64]float64{}
	seedSet := map[uint64]bool{}
	frontier := make([]uint64, 0, len(seeds))
	for _, s := range seeds {
		act[s.ID] = 1.0
		seedSet[s.ID] = true
		frontier = append(frontier, s.ID)
	}

	var traversed []memory.LinkEdge
	for hop := 0; hop < spreadHops && len(frontier) > 0; hop++ {
		edges, err := d.store.LinksFrom(frontier)
		if err != nil {
			return nil, err
		}
		// Relax against the activations as they stood when the hop began,
		// so the outcome cannot depend on the order edges come back from
		// the store.
		base := make(map[uint64]float64, len(act))
		maps.Copy(base, act)
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
	out := make([]*wire.FragmentInfo, 0, len(frags))
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
func fairTruncate(frags []*memory.Fragment, budget int) []*memory.Fragment {
	queues := map[string][]*memory.Fragment{}
	var order []string
	for _, f := range frags {
		if _, ok := queues[f.Channel]; !ok {
			order = append(order, f.Channel)
		}
		queues[f.Channel] = append(queues[f.Channel], f)
	}
	picked := make([]*memory.Fragment, 0, budget)
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

// sleepCycle is one consolidation night: replay the day to weave
// cross-channel links and order, decay and prune in the store, merge
// recurring shapes into prototypes, distil prototypes into concepts and
// bond words to them, flush predictor state, wake with reset sleep
// pressure. The counts come back so the consolidate act can judge its
// own fruit.
func (d *Daemon) sleepCycle() (pruned, merged, born int) {
	_, woven := d.replay(d.lastSleep)
	pruned, err := d.store.Sleep(d.cfg.NightlyDecay, d.cfg.PruneFloor)
	if err != nil {
		fmt.Fprintf(d.out, "sleep error: %v\n", err)
		return 0, 0, 0
	}
	merged, err = d.store.MergeDuplicates()
	if err != nil {
		fmt.Fprintf(d.out, "merge error: %v\n", err)
	}
	born, named := d.growConcepts()
	// Words whose company is a stable cloud rather than one shape
	// distil into abstractions grounded in their constellations.
	born += d.growAbstractions()
	// Each concept's sparse signature rewrites from its current
	// grounding, so resemblance stays current and auditable.
	d.growFeatures()
	// The embedder learns each concept's place in a continuous space from its
	// own feature company, so the resemblance walk gains geometric neighbours.
	d.growEmbeddings()
	// A relation told often enough, its slots filled by varied concepts,
	// induces an argument-structure construction grown from that usage.
	born += d.growConstructions()
	// Usage and seed settle their nightly contest, and the working
	// grammar recompiles from the surviving weights.
	d.grammarCompetition()
	// Consolidation rewrote weights, pruned, and merged underneath any
	// cached copy; the working set rebuilds from what the morning uses.
	d.hot.Clear()
	d.ws.clear()
	// The night keeps the conclusions GOLEM has re-drawn often enough and clears
	// the rest of working memory, so a believed proposition survives where a
	// passing thought does not.
	d.promoteBeliefs()
	d.topic = 0
	d.lastDrewTopic = 0
	d.lastRole = nil
	d.threadProps = nil
	d.topicTrail = nil
	d.turnFocus = 0
	d.claims = nil
	d.openQuestions = nil
	d.followUp = ""
	d.proposedKinds = nil
	// The night may have pruned a participant that nothing grounded it through;
	// re-establish the trio so the cached ids point at live concepts for the new
	// day. One spoken of in a stored fact kept its grounding and its id with it.
	d.ensureParticipants()
	d.flushModels()
	if err := d.store.Checkpoint(); err != nil {
		fmt.Fprintf(d.out, "checkpoint error: %v\n", err)
	}
	d.chem.WakeRested()
	// Consolidation rewrote the graph a thought round may be reading;
	// candidates from before this moment are stale on arrival.
	d.thinkEpoch++
	d.sleeps++
	d.lastSleep = time.Now()
	d.encodesSinceSleep = 0
	held, _ := d.store.Count()
	d.event(events.Event{Ev: "replay", Count: woven})
	d.event(events.Event{Ev: "prune", Count: pruned})
	d.event(events.Event{Ev: "merge", Count: merged})
	d.event(events.Event{Ev: "sleep", Count: held})
	fmt.Fprintf(d.out, "zzz night %d: wove %d replay pair(s), pruned %d faded fragment(s), merged %d into prototypes, %d concept(s) born, %d word(s) bonded, %d held.\n",
		d.sleeps, woven, pruned, merged, born, named, held)
	return pruned, merged, born
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
	for _, l := range d.netListeners {
		l.Close()
	}

	// Every in-flight command finishes and its outcome is recorded before
	// the final consolidation, so stopping cannot lose a result; queued
	// actions that never started are dropped with an audit line instead
	// of starting on the way down. The wait is bounded by the execution
	// timeout, which every running command shares.
	d.settleExecution()

	d.replay(d.lastSleep)
	pruned, err := d.store.Sleep(d.cfg.NightlyDecay, d.cfg.PruneFloor)
	if err == nil {
		d.sleeps++
		fmt.Fprintf(d.out, "zzz final night %d: pruned %d fragment(s).\n", d.sleeps, pruned)
	}
	if _, err := d.store.MergeDuplicates(); err != nil {
		fmt.Fprintf(d.out, "merge error: %v\n", err)
	}
	d.growConcepts()
	d.growAbstractions()
	d.growFeatures()
	d.growEmbeddings()
	d.growConstructions()
	d.grammarCompetition()
	d.flushModels()
	d.store.Checkpoint()
	d.event(events.Event{Ev: "prune", Count: pruned})
	d.event(events.Event{Ev: "sleep"})
	d.events.Close()
	d.store.Close()
	os.Remove(sock)
	fmt.Fprintln(d.out, "golem asleep.")
}

// tellAbout binds words to remembered moments found by query instead of to
// the present, which lets anyone annotate events after the fact. The
// annotation is stored only when something matches; words about nothing
// teach nothing.
func (d *Daemon) tellAbout(req wire.Request) wire.Response {
	anchors, err := d.store.Recall(req.Query, 3)
	if err != nil {
		return wire.Response{Error: err.Error()}
	}
	if len(anchors) == 0 {
		return wire.Response{Error: fmt.Sprintf("no remembered moment matches %q; the words were not stored", req.Query)}
	}

	res := d.process(senses.Observation{Channel: "tell", Content: req.Content, At: time.Now()}, true, false)
	if res.FragmentID == 0 {
		return wire.Response{Error: "the words could not be stored; check the daemon log"}
	}
	f := &memory.Fragment{ID: res.FragmentID, Template: attention.Normalize(req.Content), Time: time.Now()}
	targets := make([]recentMoment, 0, len(anchors))
	for _, a := range anchors {
		targets = append(targets, recentMoment{id: a.ID, template: a.Template, at: a.Time})
	}
	if linked := d.bindTo(f, targets); len(linked) > 0 {
		fmt.Fprintf(d.out, "      ~ #%d annotates %s\n", f.ID, fmtIDs(linked))
	}
	// Annotation words enter the lexicon bound to the annotated moments,
	// so a label given after the fact can still come to name them. The
	// labeling is deliberate, so even closed-class forms mint: this is
	// the usage that can come to outweigh a seeded grammar parameter. The
	// moments span the adjacency window, but a name grounds only in the
	// freshest of them, so a caption does not bind to the scene before.
	naming := make([]recentMoment, 0, len(targets))
	for _, t := range targets {
		if f.Time.Sub(t.at) <= attentionRecency {
			naming = append(naming, t)
		}
	}
	words := d.mintWords(req.Content, req.Lang, f.ID, naming, true)
	d.warmConcepts(words, exciteSpeech)

	resp := wire.Response{OK: true, Observe: &res}
	if seeds, err := d.store.FragmentsByIDs([]uint64{f.ID}); err == nil && len(seeds) == 1 {
		if cloud, err := d.spread(seeds); err == nil {
			resp.Fragments = cloud
		}
	}
	return resp
}

// forget is the true-forget maintenance path: it deletes a remembered
// shape and everything hanging off it, the concept, its episodes, and
// their edges, since some traces (a bug's junk, sensitive data) must be
// destroyable rather than only demoted. The strongest match to the query
// names the shape; its whole template goes, and the cached working set is
// dropped so nothing keeps a stale copy.
func (d *Daemon) forget(query string) (int, string, error) {
	tpl, what := "", ""
	// Prefer the concept or abstraction the query names over an incidental
	// episodic mention of it. A told word, or a question carrying the word, is
	// a moment about the concept and not the concept itself; resolving to one
	// of those first deleted the mention and left the concept orphaned, so the
	// concept comes first and an episodic shape is the fallback for a moment
	// that never distilled into one.
	if c := d.resolveConcept(query); c != nil {
		tpl, what = c.Template, c.Content
	} else if hits, err := d.store.Recall(query, 1); err != nil {
		return 0, "", err
	} else if len(hits) > 0 {
		tpl, what = hits[0].Template, hits[0].Content
	} else {
		return 0, "", nil
	}
	n, err := d.store.Forget(tpl)
	if err != nil {
		return 0, "", err
	}
	d.hot.Clear()
	d.ws.clear()
	d.held.clear()
	d.event(events.Event{Ev: "forget", Tpl: wire.Clip(tpl, 120), Count: n})
	if err := d.store.Audit(time.Now().Unix(), "forget", what, fmt.Sprintf("deleted %d node(s)", n)); err != nil {
		fmt.Fprintf(d.out, "audit write failed: %v\n", err)
	}
	return n, what, nil
}

// handle services one CLI request inside the brain loop. A deferred reply
// means an execution was started: the reply channel is answered when the
// command's result marshals back through execDone, and the loop must not
// reply itself. The quit return reports whether the daemon should shut
// down.
func (d *Daemon) handle(req wire.Request, reply chan wire.Response) (resp wire.Response, deferred, quit bool) {
	switch req.Cmd {
	case "status":
		held, _ := d.store.Count()
		concepts, _ := d.store.CountKind(memory.KindConcept)
		words, _ := d.store.CountKind(memory.KindWord)
		for name, ctr := range d.drops {
			d.stat(name).Dropped = ctr.Load()
		}
		return wire.Response{OK: true, Status: &wire.StatusInfo{
			PID:            os.Getpid(),
			UptimeSec:      time.Since(d.started).Seconds(),
			Fragments:      held,
			HotFragments:   d.hot.Len(),
			Concepts:       concepts,
			Words:          words,
			Channels:       d.stats,
			Sleeps:         d.sleeps,
			LastSleepUnix:  d.lastSleep.Unix(),
			Chem:           chemWire(d.chem.Snapshot()),
			PromotionBar:   d.promotionBar(),
			ExecuteEnabled: d.cfg.ExecuteEnabled,
			AllowlistSize:  len(d.cfg.ExecuteAllowlist),
			VoiceEnabled:   d.voiceOn,
			VoiceName:      d.voiceName,
			Running:        d.runningCmds(),
			Queued:         len(d.waiting),
			Warm:           d.warmInfos(3),
			Goal:           d.goalDescription(),
		}}, false, false

	case "concepts":
		infos, err := d.conceptInfos()
		if err != nil {
			return wire.Response{Error: err.Error()}, false, false
		}
		return wire.Response{OK: true, Concepts: infos}, false, false

	case "cognition":
		info, err := d.cognitionInfo()
		if err != nil {
			return wire.Response{Error: err.Error()}, false, false
		}
		return wire.Response{OK: true, Cognition: info}, false, false

	case "chem":
		chem := chemWire(d.chem.Snapshot())
		return wire.Response{OK: true, Chem: &chem}, false, false

	case "observe":
		if req.Channel == "" || req.Content == "" {
			return wire.Response{Error: "observe needs a channel and content"}, false, false
		}
		res := d.process(senses.Observation{Channel: req.Channel, Content: req.Content, At: time.Now()}, false, true)
		// A captioned percept keeps the raw bytes it was distilled from in the
		// content-addressed store and references them from the moment, so a
		// transducer can look again; the brain still reasons only over Content.
		if len(req.Media) > 0 && d.media != nil && res.FragmentID != 0 {
			if hash, err := d.media.Put(req.Media); err != nil {
				fmt.Fprintf(d.out, "media store error: %v\n", err)
			} else if err := d.store.SetMedia(res.FragmentID, hash, req.Mime); err != nil {
				fmt.Fprintf(d.out, "media reference error: %v\n", err)
			}
		}
		return wire.Response{OK: true, Observe: &res}, false, false

	case "runcode":
		if req.Content == "" {
			return wire.Response{Error: "runcode needs a program"}, false, false
		}
		// A computation stated as language or as an expression is comprehended
		// to its meaning and the source generated from it, the code engine run
		// both ways before the runtime grounds the result; anything richer is
		// run as the program it already is.
		src := req.Content
		if d.code != nil {
			if comp, ok := d.code.Comprehend(src); ok {
				if generated, ok := d.code.ToCode(comp); ok {
					src = generated
				}
			}
		}
		return wire.Response{OK: true, Say: runSummary(d.runProgram(src))}, false, false

	case "tell":
		if req.Content == "" {
			return wire.Response{Error: "tell needs words"}, false, false
		}
		if req.Query != "" {
			return d.tellAbout(req), false, false
		}
		// A function taught in language is learned as a construction, not stored
		// as a percept: "the double of a number is the product of the number and
		// 2" mints the operation GOLEM can then call and write. Only a genuine
		// definition matches, so an ordinary utterance falls through to grounding.
		if d.code != nil {
			if op, ok := d.code.ComprehendDefinition(req.Content); ok {
				line, err := d.learnFunction(op)
				if err != nil {
					return wire.Response{Error: err.Error()}, false, false
				}
				return wire.Response{OK: true, Say: line}, false, false
			}
			// A function can also be shown rather than told: "the double of 3 is
			// 6 and the double of 5 is 10" has its rule worked out from the pairs
			// and kept the same way a told one is.
			if name, ins, outs, ok := d.code.ComprehendExamples(req.Content); ok {
				op, found := d.induceFunction(name, ins, outs)
				if !found {
					return wire.Response{OK: true, Say: "I could not find a rule that fits those examples."}, false, false
				}
				line, err := d.learnFunction(op)
				if err != nil {
					return wire.Response{Error: err.Error()}, false, false
				}
				return wire.Response{OK: true, Say: line}, false, false
			}
		}
		// What is warm before the utterance is captured now, to ground the words
		// in once the planner has read the clause.
		attn := d.attentionAnchors()
		res := d.process(senses.Observation{Channel: "tell", Content: req.Content, At: time.Now()}, true, true)
		if res.FragmentID == 0 {
			return wire.Response{Error: "the words could not be stored; check the daemon log"}, false, false
		}
		resp := wire.Response{OK: true, Observe: &res}
		// The grammar reads the utterance's force and the planner
		// decides the reply: an answer, an admission, or a staged
		// proposal. Verbatim memory above is untouched by any of it.
		// The answer language follows the question's: detected from
		// the closed class, with the spoken --lang tag as fallback.
		frame := d.gram.Parse(req.Content, d.resolveConcept)
		if frame.Lang == "" && req.Lang != "" {
			frame.Lang = req.Lang
		}
		if frame.Lang == "" {
			frame.Lang = "en" // English is the default tongue when none was detected or tagged
		}
		resp.Say, resp.Decision = d.plan(frame, req.Content, res.FragmentID)
		// The words enter the lexicon only after the clause is read, so an
		// inflection keys on the lemma comprehension has by now established: a
		// verb's surface ("kovaladı") reinforces the relation word ("kovala")
		// instead of minting a node before its lemma is a concept. They keep
		// company with the recent moments the utterance bound to.
		words := d.mintWords(req.Content, req.Lang, res.FragmentID, d.recentNaming(time.Now(), res.FragmentID), false)
		d.warmConcepts(words, exciteSpeech)
		// Grounding the words in what was merely held in mind waits on the force:
		// only a declarative naming grounds a word in the attended concept. A
		// question or a discourse move ("what else") names nothing, so its words do
		// not bind to whatever was warm, and a proposition's words already bound to
		// their roles, so an argument is not also named after its co-arguments.
		if frame.Force == grammar.ForceStatement && !d.momentBoundRoles(res.FragmentID) {
			d.bindAttention(words, attn)
		}
		// A reply that was built is also spoken when the voice is on.
		d.speak(resp.Say)
		// The exchange may have named what a held goal was hunting.
		d.checkGoal()
		// The turn leaves a subject behind: follow it, and notice when a new
		// one has taken the field.
		d.trackTopic()
		// What do these words point at? The new fragment just bound to the
		// recent past, so a spread from it is GOLEM's side of the exchange.
		if seeds, err := d.store.FragmentsByIDs([]uint64{res.FragmentID}); err == nil && len(seeds) == 1 {
			if cloud, err := d.spread(seeds); err == nil {
				resp.Fragments = cloud
			}
		}
		return resp, false, false

	case "name":
		// Deliberate naming: assert that a word names a grounded concept,
		// directly, where cross-situational naming would have to infer the
		// pointer from lived company and the attention path can land it on a
		// warm neighbour. The concept must already be grounded; this only adds
		// the label, the language-as-pointer move made in one operation.
		if req.Content == "" || req.Query == "" {
			return wire.Response{Error: "name needs a word and a concept"}, false, false
		}
		c := d.resolveConcept(req.Query)
		if c == nil {
			return wire.Response{Error: fmt.Sprintf("no concept matches %q to name", req.Query)}, false, false
		}
		lang := req.Lang
		if lang == "" {
			lang = "en"
		}
		w, _, err := d.store.WordNode(req.Content, lang, d.chem.Snapshot(), time.Now())
		if err != nil {
			return wire.Response{Error: err.Error()}, false, false
		}
		if err := d.store.SetLinkRel(w.ID, c.ID, memory.RelNames, deliberateName); err != nil {
			return wire.Response{Error: err.Error()}, false, false
		}
		if err := d.store.Audit(time.Now().Unix(), "name", req.Content, c.Content); err != nil {
			fmt.Fprintf(d.out, "audit write failed: %v\n", err)
		}
		return wire.Response{OK: true, Say: fmt.Sprintf("%q now names %s.", req.Content, c.Content)}, false, false

	case "recall":
		var seeds []*memory.Fragment
		var mediators []*wire.FragmentInfo
		var err error
		if req.Query == "" {
			var f *memory.Fragment
			f, err = d.store.RecallByStress()
			if f != nil {
				seeds = []*memory.Fragment{f}
				d.hot.Put(f)
			}
		} else {
			seeds, mediators, err = d.recallSeeds(req.Query)
		}
		if err != nil {
			return wire.Response{Error: err.Error()}, false, false
		}
		infos := make([]*wire.FragmentInfo, 0, len(seeds)+len(mediators))
		for _, f := range seeds {
			info := fragmentInfo(f)
			info.Activation = 1.0
			infos = append(infos, info)
		}
		infos = append(infos, mediators...)
		if len(seeds) > 0 {
			// Memory served its keeper; that is what reward means here.
			d.chem.SpikeDopamine(recallReward)
			if cloud, err := d.spread(seeds); err == nil {
				infos = append(infos, cloud...)
			}
		}
		return wire.Response{OK: true, Fragments: dedupeInfos(infos)}, false, false

	case "dump":
		frags, err := d.store.Dump()
		if err != nil {
			return wire.Response{Error: err.Error()}, false, false
		}
		degrees, _ := d.store.LinkCounts()
		infos := make([]*wire.FragmentInfo, 0, len(frags))
		for _, f := range frags {
			info := fragmentInfo(f)
			info.LinkCount = degrees[f.ID]
			infos = append(infos, info)
		}
		edges, _ := d.store.AllLinks()
		links := make([]*wire.LinkInfo, 0, len(edges))
		for _, e := range edges {
			links = append(links, &wire.LinkInfo{From: e.From, To: e.To, Rel: e.Rel, Weight: e.Weight})
		}
		return wire.Response{OK: true, Fragments: infos, Links: links}, false, false

	case "teach":
		if req.Say {
			if err := d.teachConstruction(req.Content, req.Command, req.Lang); err != nil {
				return wire.Response{Error: err.Error()}, false, false
			}
			return wire.Response{OK: true}, false, false
		}
		if req.Content == "" || req.Command == "" {
			return wire.Response{Error: "teach needs a cue and a command"}, false, false
		}
		if req.Mind {
			parts := strings.Fields(req.Command)
			if len(parts) != 2 {
				return wire.Response{Error: "teach --mind needs a relation and a signal"}, false, false
			}
			if err := d.teachThought(req.Content, parts[0], parts[1], req.Lang); err != nil {
				return wire.Response{Error: err.Error()}, false, false
			}
			return wire.Response{OK: true}, false, false
		}
		d.teach(req.Content, req.Command, time.Now())
		return wire.Response{OK: true}, false, false

	case "propose":
		if req.Content == "" {
			return wire.Response{Error: "propose needs a cue"}, false, false
		}
		dec, err := d.decide(req.Content)
		if err != nil {
			return wire.Response{Error: err.Error()}, false, false
		}
		di := decisionInfo(dec)
		// A graduated, allowed command is the payoff of graduation: it runs
		// now, with no separate approval, and there is nothing left pending.
		// The run happens off the loop (parked first if its claims are
		// held); the reply waits for its outcome either way.
		if dec.Proposed != nil && !dec.Conflict && dec.Status == memory.HabitGraduated {
			if allowed, _ := d.mayExecute(dec.Proposed.Command); allowed {
				cmd := dec.Proposed.Command
				finish := func(ran, ok bool, outcome string) {
					demoted := d.autorunFinish(dec, "", ran, ok, outcome, time.Now())
					di.Autonomous = true
					di.Outcome = outcome
					di.Demoted = demoted
					reply <- wire.Response{OK: true, Decision: di}
				}
				drop := func(reason string) {
					if err := d.store.Audit(time.Now().Unix(), dec.CueTemplate, cmd, "queued action dropped: "+reason); err != nil {
						fmt.Fprintf(d.out, "audit write failed: %v\n", err)
					}
					reply <- wire.Response{Error: "the queued action was dropped: " + reason}
				}
				if _, err := d.commitAction(cmd, finish, drop); err != nil {
					return wire.Response{Error: err.Error()}, false, false
				}
				d.pending = nil
				return wire.Response{}, true, false
			}
		}
		outcome := "proposed"
		switch {
		case dec.Proposed == nil:
			outcome = "ungrounded"
		case dec.Conflict:
			outcome = "conflict"
		}
		d.event(events.Event{Ev: "proposal", Cue: dec.CueTemplate, Action: di.Proposed, Outcome: outcome})
		d.pending = &dec
		return wire.Response{OK: true, Decision: di}, false, false

	case "approve":
		if d.pending == nil || d.pending.Proposed == nil {
			return wire.Response{Error: "no proposal is pending"}, false, false
		}
		if d.pending.Conflict {
			return wire.Response{Error: "the pending proposal is ambiguous; clarify and re-propose first"}, false, false
		}
		dec := *d.pending
		cmd := dec.Proposed.Command
		allowed, reason := d.mayExecute(cmd)
		if !allowed {
			if err := d.approveStart(dec); err != nil {
				return wire.Response{Error: err.Error()}, false, false
			}
			d.pending = nil
			outcome := dryRun(reason, cmd)
			h, err := d.approveFinish(dec, false, true, outcome, time.Now())
			if err != nil {
				return wire.Response{Error: err.Error()}, false, false
			}
			return wire.Response{OK: true, Action: &wire.ActionResult{
				Command: cmd, Outcome: outcome,
				Status: h.Status, Approvals: h.Approvals, Denials: h.Denials,
			}}, false, false
		}
		finish := func(ran, ok bool, outcome string) {
			h, err := d.approveFinish(dec, ran, ok, outcome, time.Now())
			if err != nil {
				reply <- wire.Response{Error: err.Error()}
				return
			}
			reply <- wire.Response{OK: true, Action: &wire.ActionResult{
				Command: cmd, Outcome: outcome,
				Status: h.Status, Approvals: h.Approvals, Denials: h.Denials,
			}}
		}
		drop := func(reason string) {
			if err := d.store.Audit(time.Now().Unix(), dec.CueTemplate, cmd, "queued action dropped: "+reason); err != nil {
				fmt.Fprintf(d.out, "audit write failed: %v\n", err)
			}
			reply <- wire.Response{Error: "the queued action was dropped: " + reason}
		}
		// A full queue refuses before anything changes, so the proposal
		// stays pending and the grounding is untouched for the retry.
		if !d.canStart(pipeline.ClaimsOf(cmd)) && len(d.waiting) >= waitingCap {
			return wire.Response{Error: "the action queue is full; approve again when something finishes"}, false, false
		}
		// The grounding strengthens at commitment, whether the run starts
		// now or waits its turn in the queue.
		if err := d.approveStart(dec); err != nil {
			return wire.Response{Error: err.Error()}, false, false
		}
		if _, err := d.commitAction(cmd, finish, drop); err != nil {
			return wire.Response{Error: err.Error()}, false, false
		}
		d.pending = nil
		return wire.Response{}, true, false

	case "deny":
		if d.pending == nil || d.pending.Proposed == nil {
			return wire.Response{Error: "no proposal is pending"}, false, false
		}
		if d.pending.Conflict {
			return wire.Response{Error: "the pending proposal is ambiguous; clarify and re-propose first"}, false, false
		}
		dec := *d.pending
		h, err := d.deny(dec, time.Now())
		if err != nil {
			return wire.Response{Error: err.Error()}, false, false
		}
		d.pending = nil
		return wire.Response{OK: true, Action: &wire.ActionResult{
			Command: dec.Proposed.Command, Outcome: "denied",
			Status: h.Status, Approvals: h.Approvals, Denials: h.Denials,
		}}, false, false

	case "habits":
		habits, err := d.store.AllHabits()
		if err != nil {
			return wire.Response{Error: err.Error()}, false, false
		}
		out := make([]*wire.HabitInfo, 0, len(habits))
		for _, h := range habits {
			out = append(out, &wire.HabitInfo{Cue: h.Cue, Action: h.Action,
				Status: h.Status, Approvals: h.Approvals, Denials: h.Denials})
		}
		return wire.Response{OK: true, Habits: out}, false, false

	case "audit":
		entries, err := d.store.AuditTail(20)
		if err != nil {
			return wire.Response{Error: err.Error()}, false, false
		}
		out := make([]*wire.AuditInfo, 0, len(entries))
		for _, e := range entries {
			out = append(out, &wire.AuditInfo{AtUnix: e.At, Cue: e.Cue, Action: e.Action, Outcome: e.Outcome})
		}
		return wire.Response{OK: true, Audit: out}, false, false

	case "voice":
		return d.handleVoice(req), false, false

	case "sleep":
		// Consolidate once on demand and stay running, the same cycle the
		// homeostasis tick fires when the daemon falls quiet. It runs on the
		// loop, so memory is never touched concurrently.
		d.sleepCycle()
		return wire.Response{OK: true}, false, false

	case "forget":
		if req.Query == "" {
			return wire.Response{Error: "forget needs something to forget"}, false, false
		}
		n, what, err := d.forget(req.Query)
		if err != nil {
			return wire.Response{Error: err.Error()}, false, false
		}
		if n == 0 {
			return wire.Response{Error: fmt.Sprintf("nothing matches %q to forget", req.Query)}, false, false
		}
		return wire.Response{OK: true, Say: fmt.Sprintf("forgot %q (%d node(s) gone).", what, n)}, false, false

	case "edit":
		if req.ID == 0 {
			return wire.Response{Error: "edit needs a fragment id"}, false, false
		}
		if err := d.editContent(req.ID, req.Content); err != nil {
			return wire.Response{Error: err.Error()}, false, false
		}
		return wire.Response{OK: true, Say: fmt.Sprintf("edited #%d.", req.ID)}, false, false

	case "reweight":
		if req.ID == 0 {
			return wire.Response{Error: "reweight needs a fragment id"}, false, false
		}
		if err := d.reweight(req.ID, req.Weight); err != nil {
			return wire.Response{Error: err.Error()}, false, false
		}
		return wire.Response{OK: true, Say: fmt.Sprintf("reweighted #%d to %.2f.", req.ID, req.Weight)}, false, false

	case "delete":
		if req.ID == 0 {
			return wire.Response{Error: "delete needs a fragment id"}, false, false
		}
		n, err := d.deleteNode(req.ID)
		if err != nil {
			return wire.Response{Error: err.Error()}, false, false
		}
		if n == 0 {
			return wire.Response{Error: fmt.Sprintf("no fragment #%d to delete", req.ID)}, false, false
		}
		return wire.Response{OK: true, Say: fmt.Sprintf("deleted #%d.", req.ID)}, false, false

	case "mark":
		if req.ID == 0 {
			return wire.Response{Error: "mark needs a fragment id"}, false, false
		}
		if err := d.markNode(req.ID); err != nil {
			return wire.Response{Error: err.Error()}, false, false
		}
		return wire.Response{OK: true, Say: fmt.Sprintf("marked #%d edited.", req.ID)}, false, false

	case "stop":
		return wire.Response{OK: true}, false, true

	default:
		return wire.Response{Error: "unknown command: " + req.Cmd}, false, false
	}
}

func decisionInfo(dec Decision) *wire.DecisionInfo {
	di := &wire.DecisionInfo{Cue: dec.Cue}
	if dec.Proposed == nil {
		return di
	}
	di.Grounded = true
	di.Proposed = dec.Proposed.Command
	di.Saliency = dec.Proposed.Saliency
	di.Status = dec.Status
	di.Denylisted = config.Denylisted(dec.Proposed.Command)
	di.Conflict = dec.Conflict
	di.Question = dec.Question
	for _, a := range dec.Alternatives {
		di.Alternatives = append(di.Alternatives, a.Command)
	}
	return di
}
