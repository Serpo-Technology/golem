// Package daemon is the brain loop: one goroutine that owns every piece
// of mutable state, fed by sensors and the socket, deciding serially,
// acting in parallel under claims, and consolidating in its sleep. The
// other packages are organs; this one is where they meet.
package daemon

import (
	"bufio"
	"crypto/ed25519"
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
	"golem/internal/authkey"
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
	// shadowVerbose makes the declarative shadow report every clause it covers or declines, not
	// only the divergences, so a soak can measure the engine's coverage. Off in normal running.
	shadowVerbose bool

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

	// computeGaps holds computations comprehended but not yet run (the runtime
	// was shut), for the compute faculty to close at idle; computeRuns counts
	// thought-initiated runs since the last sleep, the per-wake budget;
	// triedConjectures marks the experiments this wake has already wondered
	// about, so an idle mind moves on rather than worrying one pair.
	computeGaps      []computeGap
	computeRuns      int
	triedConjectures map[string]bool

	// pendingRetune is the one self-tuning wish the nightly review may hold
	// at a time: formed from the experiment trail, built by the retune act
	// at idle, cleared when the build is handed off.
	pendingRetune *retuneProposal

	// architectPub is the architect's public key, decoded once at boot; nil
	// when none is configured. reqPrincipal carries who the current request
	// was authenticated as, set on the loop just before each handle so the
	// single-threaded handlers can read it. sigMu guards seenSigs, the cache
	// of recently accepted signatures that makes a captured request one-shot.
	architectPub ed25519.PublicKey
	reqPrincipal principal
	sigMu        sync.Mutex
	seenSigs     map[string]int64

	// sourceKeys holds each enrolled peer's public key, so a peer signs its
	// own requests and is heard as exactly that source. The architect enrolls
	// a key (a signed authority action) and the map is decoded from the store
	// at boot; keyMu guards it, since it is read off the loop in principalOf
	// and written on the loop by an enroll.
	sourceKeys map[string]ed25519.PublicKey
	keyMu      sync.RWMutex

	// statementAbandoned marks that the current telling was read as a
	// proposition and yielded none (a misparse: a clausal shape whose roles
	// never bound). Such a telling lays no naming company and no attention
	// binding: an utterance the grammar could not understand must be inert,
	// not grounded into whatever happened to be warm. Reset before each plan.
	statementAbandoned bool

	// restored is the diacritic fold's per-turn record: for each ASCII Turkish
	// surface recovered this turn, the concept it resolved to and the confidence a
	// bare guess of it may be spoken at. It is the memo that folds a span once, the
	// signal the word-form mint reads to skip a lossy KindWord row, and the anti-leak
	// set the naming company drops, so "buyuk" exists only as a lossy view of "büyük"
	// and never caches an alias. Cleared at the start of each turn.
	restored map[string]restoredNote
	// foldEnabled gates the diacritic fold: ON during the main resolve pass so a
	// lossy Turkish token recovers its proper concept, OFF while sniffTurkish runs so
	// language detection stays on exact and morphological evidence and one spuriously
	// folding token can never flip a whole clause to Turkish.
	foldEnabled bool

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
	// summarized as one moment. subGovernors throttle the sub-conscious
	// stream on its own budget so an ambient flood cannot drain the tokens a
	// genuine conscious surprise needs, nor spend them faster than it earns.
	governors    map[string]*tokenBucket
	subGovernors map[string]*tokenBucket
	pendingStorm map[string]int

	// absIdx holds the naming graph prefetched once for a consolidation's abstraction
	// pass, so the per-word resolution reads it from memory instead of a round-trip to
	// the single-writer store for every word. It is set only while that pass runs.
	absIdx *abstractionIndex
	// absIdxOff runs the abstraction pass on the store directly, without the prefetched
	// index, so a test can assert the two paths reach byte-identical results.
	absIdxOff bool
	// episodes tracks, per sub-conscious channel, when each template was last seen,
	// so a percept repeated within one episode is de-duplicated instead of crediting
	// genesis mass by sheer volume. Touched only for Subconscious channels.
	episodes map[string]*episodeTracker

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

	// pendingReferent is a role question parked on an ambiguous pro-form,
	// waiting for the turn that names which referent was meant. Read once and
	// cleared each turn, so an unanswered clarification drops.
	pendingReferent *pendingReferent

	// sawCell, sawColor, and sawAt are the eye's short focus: the colour cell and the
	// raw colour of the thing just seen, and when. A colour told right after ("its
	// colour is crimson") grounds the word to what was measured and moves that cell's
	// prototype toward the raw colour, so colour is learned by being shown and told, not
	// by a command. It lapses with the conversational moment.
	sawCell  string
	sawColor [3]float64
	sawAt    time.Time

	// colourCells are the perceptual colour-sense concepts (percept:col:*), innate
	// scaffolding a colour word names, kept out of the grown-concept listing the way
	// the participants and grammar classes are.
	colourCells map[uint64]bool

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

	// lastTrace holds the justification traces of the turn's assertions, so
	// "why do you say that" reads the proof of the last answer. turnTraced
	// marks that this turn has already displaced the previous set; a turn that
	// asserts nothing traceable drops the set instead, so a stale proof is
	// never spoken over a fresh claim. Cleared at sleep with the discourse.
	lastTrace  []uint64
	turnTraced bool

	// openQuestions are the questions raised this exchange that GOLEM could not
	// answer, held so it can follow up when the answer later arrives. Cleared at
	// sleep. followUp carries a resolution back to the planner: when a told
	// proposition answers an open question, GOLEM speaks the answer on that turn
	// rather than staying silent.
	openQuestions  []openQuestion
	followUp       string
	proposedKinds  map[string]bool     // un-named kinds already offered this session, so each is proposed once
	pendingKind    []uint64            // the members of the kind last proposed and not yet named; the speaker's naming grounds them
	pendingCompute *pendingComputation // a computation GOLEM was asked for but does not know, awaiting an example or a definition to learn it by

	// selfID, userID, and bothID are the conversation's participants as concepts:
	// GOLEM, the one it speaks with, and the two together. The deictic pronouns
	// resolve to them ("you" is GOLEM, "I" the user) so a turn about the speakers
	// is reasoned over like any other, and GOLEM can be asked about itself.
	selfID uint64
	userID uint64
	bothID uint64
	// commandName maps a faculty concept to its canonical name, and
	// commandTriggers the concepts each faculty concerns, so a reworded or
	// translated imperative routes to the faculty its verb names or resembles.
	commandName     map[uint64]string
	commandTriggers map[string][]uint64
	// opConcept maps an operation concept to the operation it names, the grounded
	// substitute for the code engine's English word map: a computation word resolves
	// to its concept, and the concept yields the operation the engine runs.
	opConcept map[uint64]code.OpRef
	// stringOpConcept maps a string-primitive concept (concat, prepend, lastChar,
	// dropLast) to the operation it names, kept apart from opConcept the way the
	// comparisons and extremals are, so a string primitive never reads as an ordinary
	// call head in prose and IsComputationSurface never picks it up. A morphology rule
	// composes over these; the serve path never resolves one from a spoken word.
	stringOpConcept map[uint64]code.OpRef
	// cmpConcept maps a comparison concept to its operator symbol (">", "<", "==").
	// Comparisons are grounded operations a conditional's test names ("above" -> gt),
	// kept apart from opConcept so a comparison word never reads as a call head;
	// cmpDefault is the bare copula's comparison, equality.
	cmpConcept map[uint64]string
	cmpDefault string
	// extremalConcept maps an extremal concept to the comparison symbol its reduction
	// turns on (max -> ">", min -> "<"). Extremals are grounded reductions a property
	// goal names ("greatest" -> max), kept apart from opConcept and cmpConcept so an
	// extremal word never reads as a call head or a conditional relator.
	extremalConcept map[uint64]string
	// propertyConcept maps a property concept to the grounded property it denotes, the
	// conjunction of relations a function must satisfy ("magnitude" -> the output squares to
	// the input squared and is not below zero). A property goal names one ("the abs is the
	// magnitude") and the procedure is discovered by running; kept apart from the other
	// concept maps so a property word never reads as a call head, relator, or extremal.
	propertyConcept map[uint64]code.Property

	// reasonRuleID caches the roots of the durable compositional reasoning-rule subgraphs by
	// their template once found or created, so a reasoning handler runs the graph-stored rule
	// without a scan per query. Set and read only on the single request loop; nil until use.
	reasonRuleID map[string]uint64

	// refinements are the category-refinement concepts a closed-class word marks: a
	// speech-act verb's discourse move (clarify, recall), a relator's direction
	// (after, before), a predicate's relation property (transitive). They are graph
	// concepts the words link to, so the grammar derives a word's colon-refined
	// category from the graph; like the participants, they sit outside the grown
	// concept layer.
	refinements map[uint64]string
	// categoryClasses are the bare grammatical-class concepts a closed-class word is a
	// member of: a determiner, a copula, a negator. Where a refinement adds a subtype to a
	// class kept in the seed, a class concept IS the broad class, so the grammar derives a
	// word's whole category from the link rather than a seeded class string, and membership
	// becomes a teachable graph fact. Like the refinements, they sit outside the grown layer.
	categoryClasses map[uint64]string
	// gradableScaffold holds the comparative machinery seeded as class concepts: the
	// comparison operator, the greater and lesser pole roles, and the rule, regular,
	// irregular, and exception meta concepts. The concept ID maps to its content word.
	// Like the category classes, they sit outside the grown concept layer; the gradable
	// adjectives and the scales they grade along are grown content, taught rather than
	// seeded here.
	gradableScaffold map[uint64]string

	// argConstructions are the argument-structure construction concepts (transitive,
	// intransitive), each carrying its form pole as a slot subgraph in the graph (an ordered slot
	// per argument holding its role, syntactic type, and feature preference, plus the predicate
	// role), so a form-meaning pattern is a graph fact the way a category membership is. The
	// concept ID maps to "<lang>:<name>". argConstructionParts holds the slot and role/syn/prefer
	// vocabulary nodes of those subgraphs. Both sit outside the grown concept layer.
	argConstructions     map[uint64]string
	argConstructionParts map[uint64]bool

	// declRules are the declarative-grammar phrase-rule concepts (the general realization engine's
	// syn rules), each carrying its form pole as a subgraph: an ordered slot per daughter, a head edge,
	// and a flow node per agreement constraint. So a phrase rule is a graph fact the engine reads its
	// daughter order and agreement from, the way the argument-structure constructions are. The concept
	// ID maps to its template ("syn:<lang>:<name>"). declParts holds the slot, flow, and role vocabulary
	// nodes of those subgraphs. Both sit outside the grown concept layer.
	declRules map[uint64]string
	declParts map[uint64]bool

	// answerConstructions are the answer-template concepts (a definition, a seen-count, a succession),
	// each carrying its Compose template as the concept's content and linking to the chemistry condition
	// that selects its variant, so a realization template is a graph fact too. The concept ID maps to its
	// "cxn:<lang>:<intent>:<variant>" parameter key. answerConstructionParts holds the condition vocabulary
	// nodes. Both sit outside the grown concept layer.
	answerConstructions     map[uint64]string
	answerConstructionParts map[uint64]bool

	// deicticByForm resolves a deictic pronoun to the participant it points at,
	// built from the graph: each deixis word is linked to its participant concept,
	// so the mapping is a learnable graph relation, not a hardcoded matcher.
	deicticByForm map[string]uint64

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
	channel  string // the sense it came in on, so ostension grounds only in a perceived percept, not a spoken one
	at       time.Time
	heat     float64 // how warm the concept lending this moment is, for weighting an attention bond
}

// episodeTracker remembers, per sub-conscious channel, when each template was last
// seen, so a percept repeated within one episode can be recognized and not re-credit
// genesis mass. It holds two generations of a template->last-seen map: a read checks
// both, a write lands in cur, and when cur fills it rotates over prev and starts
// fresh, discarding the older generation in O(1). Memory is bounded at ~2*episodeCap
// per channel, and a template that keeps recurring is never lost because every
// occurrence rewrites it into cur.
type episodeTracker struct {
	cur, prev map[string]time.Time
}

// episodeCap bounds one generation of a channel's episode tracker. A channel with
// more distinct live templates than this rotates, so a flood of ever-changing
// templates cannot grow the map without bound.
const episodeCap = 4096

// seen records this occurrence of tpl at time at and reports whether it falls inside
// the same episode as the previous one (the gap since last-seen is within window).
// It is a SLIDING window: every occurrence rewrites last-seen, so a continuous stream
// never gaps out and stays one episode. An out-of-order timestamp is treated as a
// fresh episode, which only ever over-encodes, never drops wrongly.
func (e *episodeTracker) seen(tpl string, at time.Time, window time.Duration) bool {
	last, known := e.cur[tpl]
	inCur := known
	if !known {
		last, known = e.prev[tpl]
	}
	same := known && !at.Before(last) && at.Sub(last) <= window
	if e.cur == nil {
		e.cur = make(map[string]time.Time)
	}
	if !inCur && len(e.cur) >= episodeCap {
		e.prev = e.cur
		e.cur = make(map[string]time.Time)
	}
	e.cur[tpl] = at
	return same
}

// untrustedWeight is the starting weight of a moment from an untrusted
// channel: half a memory, gone in half the nights.
const untrustedWeight = 0.5

// A subliminal moment, encoded below consciousness because it has become routine,
// is faint on both axes: a low weight so it fades in a few nights unless something
// reinforces it, and a low genesis mass so ambient repetition alone needs far more
// of it than a handful of attended showings to earn a concept.
const (
	subliminalWeight = 0.35
	subliminalMass   = 0.3
)

// untrustedNameScale is how lightly a word read on an untrusted channel bonds,
// so reading grows the vocabulary only by recurrence: a word met across many of
// the watched sources entrenches, while a word from one page fades.
const untrustedNameScale = 0.5

// Association tuning. Link strength rides on dopamine, repetition of a
// template pair earns a bonus and a reward spike, and recall both rewards
// the chemistry and strengthens the paths it walked.
const (
	linkWindow    = 30 * time.Second
	maxLinkFanout = 8
	// linkWeight is the strength of a fresh bond between a word and the moment it
	// kept company with. It is a constant: naming reads the RATIO of a word's
	// company across situations, so the rate it bonds is fixed, not modulated by
	// how rewarded GOLEM felt at that instant. Dopamine drives behaviour (what to
	// pursue, whether curiosity fires), not how firmly a word binds.
	linkWeight        = 0.5
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
	pr    principal
}

// principal is who a request was authenticated as by the time it reaches the
// loop: whether a valid architect signature was verified, whether it arrived
// on the local unix socket, and a short fingerprint of the signature for the
// trail. Authority (setting trust, retuning the source) takes architectSigned;
// the local operator is trusted for telling by locality alone.
type principal struct {
	architectSigned bool
	peerSource      string // the source a peer proved with its own key, empty otherwise
	local           bool
	sigFingerprint  string
}

// sigWindow bounds how far a signature's timestamp may sit from now, absorbing
// clock skew while keeping a captured request replayable only within it.
const sigWindow = 2 * time.Minute

// verifyArchitect checks a request's signature. It returns whether a valid
// architect signature was present, and an error when a signature was presented
// but does not hold up: no key configured, a bad signature, a stale timestamp,
// or a signature already spent. An unsigned request returns (false, nil), which
// the caller allows only on the local socket. The seen-signature cache makes a
// captured request one-shot; it is swept of expired entries on each pass and
// needs no persistence, since a signature older than the window fails the
// timestamp check anyway.
// signer names whose key verified a request's signature: the architect, or a
// named peer that proved its own identity, or neither (unsigned).
type signer struct {
	architect bool
	peer      string
}

// authenticate verifies a request's signature against the architect's key or a
// registered peer's key, and enforces freshness and the one-shot cache once the
// signature is known genuine. It returns who signed (a zero signer for an
// unsigned request) and an error only when a presented signature does not hold
// up: malformed, verified by no known key, stale, or already used.
func (d *Daemon) authenticate(req wire.Request) (signer, error) {
	if req.Sig == "" {
		return signer{}, nil
	}
	msg := wire.CanonicalBytes(req)
	if len(msg) == 0 {
		return signer{}, errors.New("the request cannot be canonicalized")
	}
	// The cache key is the signature's canonical form, so a whitespace- or
	// padding-permuted copy of an accepted signature is the same key and is
	// caught as a replay rather than slipping through as a new string.
	sigKey, ok := authkey.NormalizeSig(req.Sig)
	if !ok {
		return signer{}, errors.New("the signature is malformed")
	}
	// Whose signature is it? The architect's key, or the key enrolled for the
	// source it declares. A signature that verifies under neither is refused,
	// and a declared source can be proved only by its own key, so a peer can
	// neither borrow another source's name nor claim the architect.
	var who signer
	switch {
	case d.architectPub != nil && authkey.Verify(d.architectPub, msg, req.Sig):
		who = signer{architect: true}
	case req.Source != "" && d.verifyPeer(req.Source, msg, req.Sig):
		who = signer{peer: req.Source}
	default:
		return signer{}, errors.New("the signature does not verify")
	}
	now := time.Now().UnixNano()
	skew := now - req.Ts
	if skew < 0 {
		skew = -skew
	}
	if skew > int64(sigWindow) {
		return signer{}, errors.New("the signature is stale")
	}
	d.sigMu.Lock()
	defer d.sigMu.Unlock()
	if d.seenSigs == nil {
		d.seenSigs = make(map[string]int64)
	}
	for k, exp := range d.seenSigs {
		if exp < now {
			delete(d.seenSigs, k)
		}
	}
	if _, seen := d.seenSigs[sigKey]; seen {
		return signer{}, errors.New("the signature was already used")
	}
	// The entry must outlive the whole interval the signature stays fresh,
	// which ends at Ts + sigWindow, so a forward-skewed signature cannot be
	// replayed in the gap between the cache expiring and the freshness deadline.
	d.seenSigs[sigKey] = req.Ts + int64(sigWindow)
	return who, nil
}

// verifyPeer reports whether a message carries a valid signature from the key
// enrolled for a source. It reads the key map under the lock, since an enroll
// may be writing it on the loop.
func (d *Daemon) verifyPeer(source string, msg []byte, sigB64 string) bool {
	d.keyMu.RLock()
	pub, ok := d.sourceKeys[source]
	d.keyMu.RUnlock()
	return ok && authkey.Verify(pub, msg, sigB64)
}

// verifyArchitect reports whether the architect signed a request, the narrow
// question the authority gates and the replay tests ask. A peer signature is
// genuine but not the architect's, so it returns false with no error.
func (d *Daemon) verifyArchitect(req wire.Request) (bool, error) {
	who, err := d.authenticate(req)
	if err != nil {
		return false, err
	}
	return who.architect, nil
}

// principalOf authenticates a request arriving on a transport and returns who
// it speaks as, or an error to refuse it. A bad signature is always refused; an
// unsigned request is refused on the network transport and allowed on the local
// socket as the operator. A peer that signed with its enrolled key is carried
// as that source.
func (d *Daemon) principalOf(req wire.Request, transport string) (principal, error) {
	who, err := d.authenticate(req)
	if err != nil {
		return principal{}, err
	}
	authenticated := who.architect || who.peer != ""
	if transport == "tcp" && !authenticated {
		return principal{}, errors.New("this brain accepts only signed requests over the network")
	}
	pr := principal{architectSigned: who.architect, peerSource: who.peer, local: transport == "unix"}
	if authenticated && len(req.Sig) >= 12 {
		pr.sigFingerprint = req.Sig[:12]
	}
	return pr, nil
}

// loadSourceKeys decodes every enrolled peer key from the store into the map
// principalOf checks, so a peer's identity survives a restart.
func (d *Daemon) loadSourceKeys() error {
	raw, err := d.store.SourceKeys()
	if err != nil {
		return err
	}
	keys := make(map[string]ed25519.PublicKey, len(raw))
	for name, pubB64 := range raw {
		pub, err := authkey.DecodePublic(pubB64)
		if err != nil || pub == nil {
			fmt.Fprintf(d.out, "skipping malformed enrolled key for %q: %v\n", name, err)
			continue
		}
		keys[name] = pub
	}
	d.keyMu.Lock()
	d.sourceKeys = keys
	d.keyMu.Unlock()
	return nil
}

// setSourceKey enrolls or rotates a peer's public key (an empty key revokes it),
// updating both the store and the in-memory map. It mints the source node so a
// trust level can be set for it too. It runs on the loop, from an enroll.
func (d *Daemon) setSourceKey(name, pubB64 string) error {
	if _, err := d.ensureSource(name); err != nil {
		return err
	}
	if strings.TrimSpace(pubB64) == "" {
		if err := d.store.DeleteSourceKey(name); err != nil {
			return err
		}
		d.keyMu.Lock()
		delete(d.sourceKeys, name)
		d.keyMu.Unlock()
		return nil
	}
	pub, err := authkey.DecodePublic(pubB64)
	if err != nil || pub == nil {
		return fmt.Errorf("the public key is not valid: %v", err)
	}
	if err := d.store.SetSourceKey(name, pubB64); err != nil {
		return err
	}
	d.keyMu.Lock()
	if d.sourceKeys == nil {
		d.sourceKeys = map[string]ed25519.PublicKey{}
	}
	d.sourceKeys[name] = pub
	d.keyMu.Unlock()
	return nil
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
		subGovernors: make(map[string]*tokenBucket),
		pendingStorm: make(map[string]int),
		episodes:     make(map[string]*episodeTracker),
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
		seenSigs:     make(map[string]int64),
	}
	// The architect's public key is decoded once; a malformed key is a
	// misconfiguration worth refusing at boot rather than failing every
	// signature later.
	pub, err := authkey.DecodePublic(cfg.ArchitectPubKey)
	if err != nil {
		store.Close()
		return nil, fmt.Errorf("architect_pubkey: %w", err)
	}
	d.architectPub = pub
	if err := d.loadSourceKeys(); err != nil {
		store.Close()
		return nil, err
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

func (d *Daemon) newBucket(channel string) *tokenBucket {
	burst := d.burstFor(channel)
	if burst < 1 {
		burst = 1
	}
	refillSec := d.cfg.EncodeRefillSec
	if refillSec <= 0 {
		refillSec = 2
	}
	return &tokenBucket{
		tokens: burst,
		max:    burst,
		refill: 1 / refillSec,
		last:   time.Now(),
	}
}

func (d *Daemon) governor(channel string) *tokenBucket {
	b, ok := d.governors[channel]
	if !ok {
		b = d.newBucket(channel)
		d.governors[channel] = b
	}
	return b
}

// subGovernor throttles the sub-conscious stream on a budget separate from the
// conscious governor, so a fast ambient channel cannot spend faster than it
// refills, and cannot steal the conscious channel's tokens.
func (d *Daemon) subGovernor(channel string) *tokenBucket {
	b, ok := d.subGovernors[channel]
	if !ok {
		b = d.newBucket(channel)
		d.subGovernors[channel] = b
	}
	return b
}

func (d *Daemon) episodeTrackerFor(channel string) *episodeTracker {
	t, ok := d.episodes[channel]
	if !ok {
		t = &episodeTracker{}
		d.episodes[channel] = t
	}
	return t
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

// categoryParticipant marks the conversation participants, discourse scaffolding
// the nightly prune spares: a participant grounds in no percept and may fill no
// role for nights on end, yet what hangs off it (its given name) must not die of
// its disconnection.
const categoryParticipant = "participant"

// ensureParticipants makes sure the conversation's participants exist as concepts
// and caches their ids: GOLEM itself, the one it speaks with, and the two
// together. They are persistent and discourse-grounded rather than perceptual, the
// referents the deictic pronouns ("you", "I", "we") point at.
func (d *Daemon) ensureParticipants() {
	d.selfID = d.ensureParticipant("deixis:self", "GOLEM")
	d.userID = d.ensureParticipant("deixis:user", "the speaker")
	d.bothID = d.ensureParticipant("deixis:both", "us")
	d.ensureDeicticForms()
	d.ensureCommands()
	d.ensureOperations()
}

// ensureParticipant is ensureConcept with the spared participant category, so
// the trio survives the nightly grounding test with stable ids; an existing
// node from an earlier brain is re-classed the same way at boot.
func (d *Daemon) ensureParticipant(template, content string) uint64 {
	if c, err := d.store.ConceptByTemplate(template); err == nil && c != nil {
		if c.Category != categoryParticipant {
			if err := d.store.UpdateCategory(c.ID, categoryParticipant); err != nil {
				fmt.Fprintf(d.out, "participant category error: %v\n", err)
			}
		}
		return c.ID
	}
	c := &memory.Fragment{Kind: memory.KindConcept, Channel: "concept", Content: content,
		Template: template, Category: categoryParticipant, Limbic: d.chem.Snapshot(), Time: time.Now()}
	if _, err := d.store.Encode(c); err != nil {
		fmt.Fprintf(d.out, "participant concept error: %v\n", err)
		return 0
	}
	return c.ID
}

// ensureDeicticForms links each seeded deictic pronoun to the participant it
// points at, so the word-to-participant mapping is a graph relation a learner can
// extend rather than a matcher in code, then loads the resolution index from
// those links. The form lists are the innate seed, the way the participants
// themselves are; the graph is where the mapping lives. Idempotent.
func (d *Daemon) ensureDeicticForms() {
	links := []struct {
		participant uint64
		forms       []string
	}{
		{d.userID, []string{"i", "me", "ben", "beni", "ego"}},
		{d.selfID, []string{"you", "sen", "seni", "tu"}},
		{d.bothID, []string{"we", "us", "biz", "bizi", "nos"}},
	}
	gnodes, err := d.store.NodesByKind(memory.KindGrammar)
	if err != nil {
		return
	}
	idByForm := make(map[string]uint64, len(gnodes))
	for _, n := range gnodes {
		if n.Category == "deixis" {
			idByForm[strings.ToLower(n.Content)] = n.ID
		}
	}
	for _, l := range links {
		if l.participant == 0 {
			continue
		}
		for _, f := range l.forms {
			if id, ok := idByForm[f]; ok {
				if err := d.store.LinkRel(id, l.participant, memory.RelMarks, 1); err != nil {
					fmt.Fprintf(d.out, "deictic link error: %v\n", err)
				}
			}
		}
	}
	d.loadDeicticForms()
}

// loadDeicticForms builds the deictic resolution index from the graph: each
// deixis word linked to a participant resolves to it. Reading the links rather
// than a fixed table is what lets a taught link take effect.
func (d *Daemon) loadDeicticForms() {
	d.deicticByForm = map[string]uint64{}
	edges, err := d.store.LinksToRel([]uint64{d.selfID, d.userID, d.bothID}, memory.RelMarks)
	if err != nil || len(edges) == 0 {
		return
	}
	froms := make([]uint64, 0, len(edges))
	for _, e := range edges {
		froms = append(froms, e.From)
	}
	words, err := d.store.FragmentsByIDs(froms)
	if err != nil {
		return
	}
	contentByID := make(map[uint64]string, len(words))
	for _, w := range words {
		contentByID[w.ID] = w.Content
	}
	for _, e := range edges {
		if form := contentByID[e.From]; form != "" {
			d.deicticByForm[strings.ToLower(form)] = e.To
		}
	}
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

// ensureCategoryRefinements makes the refinement a closed-class word carries into
// a graph concept and links each word to it, so the colon-class taxonomy lives in
// the graph rather than as a colon-suffixed category string in the seed: a
// speech-act verb's discourse move, a relator's direction, a predicate's relation
// property. The grammar compile then derives each word's refined category from
// these links. Idempotent: the concepts and edges are reused if present, so any
// brain self-heals at boot with no migration. English relators name their own
// direction, so only the forms that need translating are linked.
func (d *Daemon) ensureCategoryRefinements() error {
	d.refinements = map[uint64]string{}
	refs := []struct {
		template, name string
		words          []string
	}{
		{"speech:clarify", "clarify", []string{"mean", "means", "meant", "explain", "explains", "explained", "elaborate", "elaborates", "elaborated", "açıkla", "explica", "significas"}},
		// Trust moves: talking about whom to believe. A trusting verb raises a source's
		// trust, a doubting one lowers it, so "trust the llm" and "I doubt the llm" reach
		// the trust the graph holds through language rather than an out-of-band command.
		// Believing a source is trusting it; distrusting is doubting it.
		{"speech:trust", "trust", []string{"trust", "trusts", "trusted", "believe", "believes", "believed", "güven", "güvenirim", "inan", "inanırım", "fido", "confido", "credo"}},
		{"speech:doubt", "doubt", []string{"doubt", "doubts", "doubted", "distrust", "şüphelen", "şüpheleniyorum", "dubito"}},
		// Social moves: a greeting is met with a greeting, a farewell with a farewell.
		// The words route as grounded speech acts the way the trust verbs do, so the
		// opening and closing of an exchange are moves in the graph, not silence.
		{"speech:greet", "greet", []string{"hello", "hi", "merhaba", "selam", "salve", "salvete"}},
		{"speech:farewell", "farewell", []string{"goodbye", "bye", "hoşçakal", "vale", "valete"}},
		{"speech:recall", "recall", []string{"say", "says", "said", "tell", "tells", "told", "söyle", "söyledim", "söyledin", "dedim", "dedin", "dic", "dixi", "dixisti"}},
		{"speech:describe", "describe", []string{"describe", "describes", "described", "anlat", "narra"}},
		{"speech:know", "know", []string{"know", "knows", "known", "biliyor", "bilir", "biliyorsun", "bilirsin", "scis", "scisne"}},
		{"rel:after", "after", []string{"sonra"}},
		{"rel:before", "before", []string{"önce"}},
		{"property:transitive", "transitive", []string{"transitive"}},
		{"property:symmetric", "symmetric", []string{"symmetric"}},
		// Locative prepositions: the closed class whose use as a copula's complement
		// ("the leaf is on the tree") marks its relation locative. The refinement rides
		// on the function-word category so the parser still reads "fn"; only the
		// locative-marking path reads the spec. Linked here so an existing brain gains
		// it at boot rather than needing a re-seed.
		{"fn:locative", "locative", []string{"on", "in", "under"}},
		// Topic preposition: the object of Latin "de" ("quid de cane scis") is what the clause
		// is about, so it fills the patient. Rides the function-word category like the locative.
		{"fn:topic", "topic", []string{"de"}},
		// Cause conjunction: the split point of a compound telling whose second
		// clause explains the first ("the dog ran because the cat came"). Rides the
		// function-word category like the locative, so the parser still flushes it.
		{"fn:cause", "cause", []string{"because", "çünkü", "quia"}},
		// Interrogative type: English wh words name their own type, so only the
		// forms that need translating link; "what:be" carries the copula nedir folds
		// in ("what is"), read back by whInfo's type-then-copula split.
		{"wh:what", "what", []string{"ne", "neyi", "quid"}},
		{"wh:who", "who", []string{"kim", "kimsin", "kimim"}},
		{"wh:where", "where", []string{"nerede"}},
		{"wh:why", "why", []string{"neden", "niye"}},
		{"wh:how", "how", []string{"nasıl"}},
		{"wh:which", "which", []string{"hangi"}},
		{"wh:what:be", "what:be", []string{"nedir"}},
		// Pronoun animacy: the bias a pro-form's binding favors. The bare pro-forms
		// (one, that, this, them) carry none and link to nothing.
		{"pro:animate", "animate", []string{"he", "she"}},
		{"pro:inanimate", "inanimate", []string{"it"}},
	}
	// The closed-class words are grammar nodes; map each by its surface so a word is
	// linked to its refinement whatever tongue seeds it.
	gnodes, err := d.store.NodesByKind(memory.KindGrammar)
	if err != nil {
		return err
	}
	idByWord := make(map[string]uint64, len(gnodes))
	for _, n := range gnodes {
		if strings.HasPrefix(n.Template, "fw:") {
			idByWord[strings.ToLower(n.Content)] = n.ID
		}
	}
	for _, r := range refs {
		// A refinement concept carries the spared "class" category, not a bare noun's:
		// it is grammar scaffolding the nightly prune must keep, or the colon-class
		// taxonomy decays on the first sleep (a pruned "wh:what" reverts Turkish "ne" to
		// a bare interrogative, so a wh-object question stops routing to its planner).
		id := d.ensureClassConcept(r.template, r.name)
		if id == 0 {
			continue
		}
		d.refinements[id] = r.name
		for _, w := range r.words {
			fwID, ok := idByWord[w]
			if !ok {
				continue
			}
			if err := d.store.LinkRel(fwID, id, memory.RelMarks, 1); err != nil {
				return err
			}
		}
	}
	return nil
}

// ensureCategoryClasses makes a bare grammatical class into a graph concept and links
// each member word to it, so the broad class a closed-class word belongs to lives in the
// graph rather than as a category string in the seed. The grammar compile then derives the
// word's category from the link. Idempotent: the concept and edges are reused if present, so
// a brain self-heals at boot. Called before the compile (in loadGrammar), so a member word
// has its category by the time anything reads it.
func (d *Daemon) ensureCategoryClasses() error {
	d.categoryClasses = map[uint64]string{}
	// The broad grammatical classes whose membership is a graph link rather than a seed
	// string: the bare classes (a determiner, a copula, ...) and the bases of the refined
	// classes (a speech verb, a relator, ...), whose colon refinement composes on top
	// through the derive order. Each closed-class word is linked to the class its seed
	// category names, so membership comes from the seed (the floor) and a taught word joins
	// a class by the same link. The deictic pronouns are left to the participant resolution
	// they already ride.
	broad := map[string]bool{
		"det": true, "cop": true, "neg": true, "imp": true, "adv": true, "cnt": true,
		"than": true, "again": true, "oth": true, "time": true, "exist": true, "fn": true,
		"speech": true, "rel": true, "wh": true, "pro": true, "property": true,
	}
	gnodes, err := d.store.NodesByKind(memory.KindGrammar)
	if err != nil {
		return err
	}
	classID := make(map[string]uint64)
	for _, n := range gnodes {
		if !strings.HasPrefix(n.Template, "fw:") || !broad[n.Category] {
			continue
		}
		id, ok := classID[n.Category]
		if !ok {
			id = d.ensureClassConcept("class:"+n.Category, n.Category)
			if id == 0 {
				continue
			}
			classID[n.Category] = id
			d.categoryClasses[id] = n.Category
		}
		if err := d.store.LinkRel(n.ID, id, memory.RelMarks, 1); err != nil {
			return err
		}
	}
	return nil
}

// ensureComparativeScaffold seeds the machinery of gradable comparison as graph
// concepts, so comparison degree, pole direction, and the rule-versus-override
// distinction are graph data rather than Go flags. The comparative operator denotes
// greater-on-scale toward a pole. The greater and lesser pole roles are the un-gated
// ground truth for which end of a scale a pole names; an ordering reads them to orient
// itself and the numeric compare path derives its sign from them. The rule, regular,
// irregular, and exception concepts are the meta layer a realization rule and its
// lexical overrides are tagged with. All are class concepts, so they are prune-spared,
// kept out of cognition, and collision-safe, seeded idempotently at boot beside the
// category classes. The gradable adjectives and the scales they grade along are grown
// content and are taught, so they are not seeded here.
func (d *Daemon) ensureComparativeScaffold() error {
	d.gradableScaffold = map[uint64]string{}
	for _, s := range []struct{ template, content string }{
		{"cmp:comparative", "comparative"},
		{"cmp:analytic", "analytic"},
		{"pole:greater", "greater"},
		{"pole:lesser", "lesser"},
		{"meta:rule", "rule"},
		{"meta:regular", "regular"},
		{"meta:irregular", "irregular"},
		{"meta:exception", "exception"},
		// The tongue a morphology rule realizes is a graph concept too, so "the tongue is
		// graph data" holds. These are un-named class concepts: prune-spared, hidden by
		// isSystemConcept, and collision-free because they carry no RelNames surface a grown
		// noun could clash with. A rule is tagged with its tongue so the keyed selection walk
		// finds the right rule per language.
		{"lang:en", "en"},
		{"lang:tr", "tr"},
		{"lang:la", "la"},
	} {
		id := d.ensureClassConcept(s.template, s.content)
		if id == 0 {
			continue
		}
		d.gradableScaffold[id] = s.content
	}
	return nil
}

// ensureClassConcept is ensureConcept with the distinct "class" category, so a piece of
// grammar scaffolding (a bare grammatical class or a closed-class refinement) lives as a
// graph concept that cannot collide with a grown noun in the naming or hygiene views, is
// excluded as a system concept, and is spared by the nightly prune.
func (d *Daemon) ensureClassConcept(template, content string) uint64 {
	if c, err := d.store.ConceptByTemplate(template); err == nil && c != nil {
		return c.ID
	}
	c := &memory.Fragment{Kind: memory.KindConcept, Channel: "concept", Content: content,
		Template: template, Category: "class", Limbic: d.chem.Snapshot(), Time: time.Now()}
	if _, err := d.store.Encode(c); err != nil {
		fmt.Fprintf(d.out, "class concept error: %v\n", err)
		return 0
	}
	return c.ID
}

// isSystemConcept reports whether a concept is innate scaffolding rather than part
// of the grown layer: a conversation participant, a category refinement, or a bare
// grammatical class. Such a concept is kept out of the concept listing the way the
// participants are.
func (d *Daemon) isSystemConcept(id uint64) bool {
	if d.isParticipant(id) {
		return true
	}
	if d.colourCells[id] {
		return true
	}
	if _, ok := d.categoryClasses[id]; ok {
		return true
	}
	if _, ok := d.gradableScaffold[id]; ok {
		return true
	}
	if _, ok := d.argConstructions[id]; ok {
		return true
	}
	if d.argConstructionParts[id] {
		return true
	}
	if _, ok := d.declRules[id]; ok {
		return true
	}
	if d.declParts[id] {
		return true
	}
	if _, ok := d.answerConstructions[id]; ok {
		return true
	}
	if d.answerConstructionParts[id] {
		return true
	}
	if _, ok := d.commandName[id]; ok {
		return true
	}
	if _, ok := d.opConcept[id]; ok {
		return true
	}
	if _, ok := d.stringOpConcept[id]; ok {
		return true
	}
	if _, ok := d.cmpConcept[id]; ok {
		return true
	}
	if _, ok := d.extremalConcept[id]; ok {
		return true
	}
	if _, ok := d.propertyConcept[id]; ok {
		return true
	}
	for _, triggers := range d.commandTriggers {
		for _, t := range triggers {
			if t == id {
				return true
			}
		}
	}
	_, ok := d.refinements[id]
	return ok
}

// scaffoldingConcept reports whether a concept is grammar living in the graph
// rather than a thing talk can be about: an answer or argument construction,
// a declarative rule or its parts, or a bare grammatical class. Such a node
// shares the graph's namespace but must never resolve as the referent of an
// ordinary word, bind a proposition's role, or be spoken as a noun phrase.
// Participants, operations, and colour cells are system concepts too, but
// words legitimately resolve to them (deixis, "+", "red"), so they stay out
// of this seal.
func (d *Daemon) scaffoldingConcept(id uint64) bool {
	if _, ok := d.categoryClasses[id]; ok {
		return true
	}
	if _, ok := d.argConstructions[id]; ok {
		return true
	}
	if d.argConstructionParts[id] {
		return true
	}
	if _, ok := d.declRules[id]; ok {
		return true
	}
	if d.declParts[id] {
		return true
	}
	if _, ok := d.answerConstructions[id]; ok {
		return true
	}
	return d.answerConstructionParts[id]
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
	go d.acceptLoop(listener, "unix")

	// A TCP transport, when configured, serves the same protocol across a
	// container boundary so external senses and a speaker can reach the brain.
	// Every request over it must be signed, so the port is a channel for the
	// architect's own signed transducers, not an open door.
	if addr := strings.TrimSpace(d.cfg.ListenTCP); addr != "" {
		tcp, err := net.Listen("tcp", addr)
		if err != nil {
			listener.Close()
			os.Remove(sock)
			d.store.Close()
			return err
		}
		d.netListeners = append(d.netListeners, tcp)
		go d.acceptLoop(tcp, "tcp")
		fmt.Fprintf(d.out, "golem also listening on tcp %s\n", tcp.Addr())
		if !loopbackAddr(addr) {
			fmt.Fprintf(d.out, "note: %s is not loopback; the port is reachable off-host and every request must be signed.\n", addr)
		}
		if d.architectPub == nil {
			fmt.Fprintf(d.out, "warning: no architect_pubkey is set, so the tcp transport will refuse every request.\n")
		}
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
			// The request's authenticated identity is set on the loop, single
			// threaded, so every handler reads a consistent principal.
			d.reqPrincipal = r.pr
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
	// A loopless driver speaks as the local operator unless a test set the
	// principal itself (an authority path exercises architectSigned directly).
	if !d.reqPrincipal.local && !d.reqPrincipal.architectSigned {
		d.reqPrincipal = principal{local: true}
	}
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

func (d *Daemon) acceptLoop(l net.Listener, transport string) {
	for {
		conn, err := l.Accept()
		if err != nil {
			return // listener closed during shutdown
		}
		go d.serveConn(conn, transport)
	}
}

func (d *Daemon) serveConn(conn net.Conn, transport string) {
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
		writeResponse(conn, resp)
		return
	}
	// Every request is authenticated before it does anything: a bad signature
	// is refused, and the network transport refuses an unsigned request. This
	// is the one choke point that sees both the transport and the parsed
	// request, so the gate lives here.
	pr, err := d.principalOf(req, transport)
	if err != nil {
		resp.Error = err.Error()
		writeResponse(conn, resp)
		return
	}
	if req.Cmd == "subscribe" {
		// A subscription holds the connection open and streams events as they
		// happen, the output channel a speaker reads; it never goes through the
		// request queue and outlasts the request deadline.
		conn.SetDeadline(time.Time{})
		d.streamEvents(conn)
		return
	}
	reply := make(chan wire.Response, 1)
	select {
	case d.reqs <- ipcReq{req: req, reply: reply, pr: pr}:
		resp = <-reply
	case <-time.After(5 * time.Second):
		resp.Error = "daemon busy"
	}
	writeResponse(conn, resp)
}

// writeResponse marshals one reply and writes it newline-delimited, the single
// form every connection ends with.
func writeResponse(conn net.Conn, resp wire.Response) {
	payload, _ := json.Marshal(resp)
	conn.Write(append(payload, '\n'))
}

// loopbackAddr reports whether a bind address stays on the host's loopback, so
// the daemon can note when a port is reachable off-host.
func loopbackAddr(addr string) bool {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		host = addr
	}
	if host == "" || host == "localhost" {
		return true
	}
	if ip := net.ParseIP(host); ip != nil {
		return ip.IsLoopback()
	}
	return false
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

	// The gate is channel-relative and GRADED: the same bit count that is routine on
	// a diverse channel is an alarm on a monotonous one. Below the absolute floor is
	// pure noise and drops. Above the floor but below the channel's z-threshold has
	// become routine (habituated): it reaches no consciousness, and is dropped as
	// before UNLESS the channel learns sub-consciously, in which case it still
	// encodes -- a faint, quiet trace that can merge into a concept and be named --
	// while the salience it would have spiked is withheld. Attention gates
	// consciousness, not whether an observation encodes.
	conscious := true
	if !forced {
		var aboveFloor bool
		conscious, aboveFloor = d.calibrator(o.Channel).Assess(
			bits, d.cfg.SurpriseThreshold, d.cfg.ZGate, d.cfg.CalibrationWarmup)
		if !aboveFloor {
			return wire.ObserveResult{Bits: bits}
		}
	}
	subliminal := !forced && !conscious
	if subliminal && !d.cfg.Subconscious(o.Channel) {
		return wire.ObserveResult{Bits: bits}
	}
	// A declared-ambient channel is a background monitoring stream: even when one of its
	// observations clears the gate (a band's novel debut, a state change), it is context,
	// not a figure attention is on. So it withholds salience, never warms the workspace, and
	// never enters the episodic recent window, so it cannot become the temporal company that
	// would entangle it into an attended concept's signature. A forced observation is attention
	// the operator directed, so it is never background (this also keeps a teaching channel that
	// was mislabeled ambient from silently losing its naming bind). An empty ambient list (every
	// existing brain) makes this false, leaving the attended path byte-identical.
	ambient := !forced && d.cfg.Ambient(o.Channel)

	// Cross-episode de-duplication, on a sub-conscious channel only. A percept already
	// taken in during this episode (no gap longer than the episode window since it was
	// last seen) has been learned; a repeat within the same continuous exposure must not
	// re-credit genesis mass, or staring at one thing would mint a concept for it by sheer
	// volume. Every occurrence, attended or not, slides the window; a below-consciousness
	// repeat inside it is dropped, so a concept is earned by recurring across SEPARATE
	// episodes, not by dwelling in one. The whole block is gated on Subconscious, so a
	// flag-off or un-opted channel never allocates the tracker and stays byte-identical.
	if d.cfg.Subconscious(o.Channel) {
		sameEpisode := d.episodeTrackerFor(o.Channel).seen(tpl, o.At, d.cfg.EpisodeGap())
		if (subliminal || ambient) && sameEpisode {
			st.Deduped++
			return wire.ObserveResult{Bits: bits}
		}
	}

	// The refractory: past the burst allowance, a CONSCIOUS storm of surprises is
	// counted instead of encoded, and later collapsed into one summary moment. The
	// predictor and calibrator have already learned the line. A background ambient
	// stream is not counted here: its surprises never summarize into a conscious storm
	// moment, which would bind and re-introduce the entanglement the ambient path avoids.
	if !forced && conscious && !ambient && !d.governor(o.Channel).take(o.At) {
		st.Suppressed++
		d.pendingStorm[o.Channel]++
		return wire.ObserveResult{Bits: bits}
	}
	// The background stream (sub-conscious traces and ambient bands alike) has its own
	// refractory on its own budget. A faint trace is cheaply lost, so past the allowance an
	// ambient flood is dropped rather than encoded: this keeps a never-silent channel from
	// accruing genesis mass faster than the budget refills, without spending the conscious
	// tokens or summarizing into a storm moment it never earned.
	if (subliminal || ambient) && !d.subGovernor(o.Channel).take(o.At) {
		st.Suppressed++
		return wire.ObserveResult{Bits: bits}
	}

	// The salience of a surprise is what reaches consciousness: the alerting spike,
	// the pressure to consolidate it phasically, the mark on time. A sub-conscious
	// trace withholds all of it; it is learned, not noticed. A background ambient
	// observation withholds it too, however surprising the band change reads.
	if !subliminal && !ambient {
		if o.Channel != thoughtChannel {
			d.lastSurprise = time.Now()
		}
		d.chem.SpikeNoradrenaline(bits * limbic.NASpikePerBit)
		d.chem.SuppressSleep(limbic.SleepSuppressOnSurprise)
	}

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
	if subliminal || ambient {
		// A below-consciousness trace is faint on both axes: a low weight so it fades
		// in a few nights unless reinforced, and a low genesis mass so routine
		// repetition earns a concept only by weight of numbers, not a few showings.
		// Its whole mass is decayable submass, so it also fades in TIME: unless the
		// percept recurs across episodes faster than the nightly bleed, its mass never
		// reaches the genesis threshold. A background ambient band is faint the same way,
		// so a fluctuating monitor mints a concept only from genuine sustained recurrence.
		frag.Weight = subliminalWeight
		frag.Mass = subliminalMass
		frag.Submass = subliminalMass
	}
	id, err := d.store.Encode(frag)
	if err != nil {
		fmt.Fprintf(d.out, "memory error: %v\n", err)
		return wire.ObserveResult{Bits: bits}
	}
	if subliminal || ambient {
		st.Subliminal++
	} else {
		st.Conscious++
	}
	d.encodesSinceSleep++
	d.chem.BuildSleepNeed(limbic.SleepNeedPerEncode) // what was taken in must be slept on
	if !subliminal && !ambient {
		d.hot.Put(frag)
		// A moment whose shape has a concept warms that concept: experience
		// keeps working memory pointed at what is happening. The warming scales
		// with the concept's information, so a high-frequency low-information
		// node cannot pull the inner loop above real knowledge. A sub-conscious
		// trace never warms the workspace: it is below attention by definition,
		// and a background ambient trace stays out for the same reason.
		if c, err := d.store.ConceptByTemplate(tpl); err == nil && c != nil {
			d.ws.excite(c.ID, exciteEncode*infoScale(c.Surprise))
		}
		d.event(events.Event{Ev: "encode", Channel: o.Channel, ID: id, Bits: bits, Tpl: wire.Clip(tpl, 120)})
		fmt.Fprintf(d.out, "[!] %.2f bits  %-8s %q -> %q  NA=%.2f  #%d\n",
			bits, o.Channel, wire.Clip(o.Content, 140), wire.Clip(tpl, 140), frag.Limbic.Noradrenaline, id)
	}

	// Episodic adjacency and the naming window are attention-level: only attended
	// moments link to the recent past, enter it, and can be what a spoken name
	// points at. A sub-conscious trace stays out, so ambient repetition can neither
	// crowd conscious moments out of the recent window nor capture an ostension; a
	// background ambient observation stays out too, so a monitoring band can never
	// become the temporal company that leaks into an attended concept's signature.
	if bind && !subliminal && !ambient {
		if linked := d.linkMoment(frag); len(linked) > 0 {
			fmt.Fprintf(d.out, "      ~ #%d linked to %s\n", id, fmtIDs(linked))
		}
		d.recent = append(d.recent, recentMoment{id: frag.ID, template: frag.Template, channel: frag.Channel, at: frag.Time})
		if len(d.recent) > 32 {
			d.recent = d.recent[len(d.recent)-32:]
		}
	}
	return wire.ObserveResult{Bits: bits, Conscious: !subliminal && !ambient, FragmentID: id}
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
	for _, r := range targets {
		w := linkWeight
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

// readyToSleep is the two-process sleep gate: NEED that enough has been taken in
// to be worth consolidating (adenosine past the gate), and PERMISSION that it is
// a safe, quiet moment to do it (no recent surprise, input silent, the minimum
// interval elapsed). Activity drives the need; quiet grants the permission.
func (d *Daemon) readyToSleep() bool {
	return d.chem.Adenosine() >= d.cfg.SleepPressureGate &&
		time.Since(d.lastSurprise) >= d.cfg.CalmBeforeSleep() &&
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
	// A night restores the compute faculty's run budget and lets it wonder
	// afresh, the two-process counterpart of the adenosine its runs charged;
	// a passed conjecture re-drawn across wakes is what entrenches into the
	// belief store.
	d.computeRuns = 0
	d.triedConjectures = nil
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
	// The frames rich enough to map from are re-bound into holographic records, so a
	// structural analogy can read the fourth term off the shape of one concept's roles
	// carried onto another's.
	d.growHolographic()
	// A source's trust follows its record: a claim GOLEM has since grounded otherwise
	// counts against it, an unchallenged night restores a little, so a voice earns or
	// loses standing by how its tellings hold up rather than by decree alone.
	d.growSourceTrust()
	// The experiment organ reads its own record the same way: rules that
	// betrayed or kept faith argue for retuning the confidence they are held
	// at, and the review may form one proposal for the idle mind to build.
	d.reviewExperimentTuning()
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
	d.recent = nil // a consolidation closes the scene: the next moment keeps company with the new day, not the one just slept on
	// The night keeps the conclusions GOLEM has re-drawn often enough and clears
	// the rest of working memory, so a believed proposition survives where a
	// passing thought does not.
	d.promoteBeliefs()
	// With the day's passing thoughts cleared, consolidation reasons afresh over what the day
	// taught: GOLEM draws the guesses the day's concepts imply but nobody asked, held in mind for
	// the coming day to volunteer, use, and (if they recur) remember. Held, never asserted, so the
	// idle reasoning stays honest.
	reasonedOver, drewFresh, workedOut := d.consolidateReasoning()
	// Drop any remembered conclusion the day's retractions or merges have unseated, so the derived
	// ledger stays a pure function of the settled graph.
	d.reverifyDerived()
	d.topic = 0
	d.lastDrewTopic = 0
	d.lastRole = nil
	d.pendingReferent = nil
	d.threadProps = nil
	d.topicTrail = nil
	d.turnFocus = 0
	d.claims = nil
	d.lastTrace = nil
	d.openQuestions = nil
	d.followUp = ""
	d.proposedKinds = nil
	d.pendingKind = nil
	d.pendingCompute = nil
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
	fmt.Fprintf(d.out, "zzz night %d: wove %d replay pair(s), pruned %d faded fragment(s), merged %d into prototypes, %d concept(s) born, %d word(s) bonded, reasoned over %d concept(s) drawing %d fresh conclusion(s) and working out %d, %d held.\n",
		d.sleeps, woven, pruned, merged, born, named, reasonedOver, drewFresh, workedOut, held)
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
	d.growHolographic()
	d.growSourceTrust()
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

	case "health":
		report, err := d.hygieneReport()
		if err != nil {
			return wire.Response{Error: err.Error()}, false, false
		}
		return wire.Response{OK: true, Health: report}, false, false

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
		res := d.process(senses.Observation{Channel: req.Channel, Content: req.Content, At: time.Now()}, req.Force, true)
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

	case "see":
		// GOLEM's own eye: it reads the image's pixels and works out the interpretable
		// attributes its captions are made of, so the percept is described in its own
		// words rather than handed a caption from outside. The raw bytes are kept beside
		// the percept, and it says what known thing the attributes it made out resemble,
		// its own recognition off its own resemblance walk.
		if len(req.Media) == 0 {
			return wire.Response{Error: "see needs an image"}, false, false
		}
		sight, ok := d.measure(req.Media)
		if !ok {
			return wire.Response{Error: "the image could not be made out"}, false, false
		}
		// Hold what was just seen in focus, so a colour told right after ("its colour is
		// crimson") grounds to the cell the eye measured and moves its prototype toward
		// the raw colour, learned by being shown and told rather than by a command.
		d.sawCell, d.sawColor, d.sawAt = sight.cell, sight.color, time.Now()
		caption, mime := d.render(sight, req.Lang), sight.mime
		res := d.process(senses.Observation{Channel: "vision", Content: caption, At: time.Now()}, true, true)
		if res.FragmentID == 0 {
			return wire.Response{Error: "the percept could not be stored; check the daemon log"}, false, false
		}
		if d.media != nil {
			if hash, err := d.media.Put(req.Media); err != nil {
				fmt.Fprintf(d.out, "media store error: %v\n", err)
			} else if err := d.store.SetMedia(res.FragmentID, hash, mime); err != nil {
				fmt.Fprintf(d.out, "media reference error: %v\n", err)
			}
		}
		say := "I see " + caption + "."
		if resembled := d.recognize(caption); len(resembled) > 0 {
			say += " It looks like " + resembled[0].Content + "."
		}
		d.speak(say)
		resp := wire.Response{OK: true, Say: say, Observe: &res}
		if seeds, err := d.store.FragmentsByIDs([]uint64{res.FragmentID}); err == nil && len(seeds) == 1 {
			if cloud, err := d.spread(seeds); err == nil {
				resp.Fragments = cloud
			}
		}
		return resp, false, false

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
			if comp, g, root, ok := d.comprehendCall("en", src); ok {
				// A computation that does not type-check (a list operation on a
				// number, say) is a gap to admit, not source to run to a broken program.
				if !d.typeChecks(comp, g, root) {
					return wire.Response{OK: true, Say: "I could not work that out."}, false, false
				}
				// Generate by walking the graph comprehension built from the grounded
				// grammar; a surface the build does not cover falls back to materializing
				// the computation, with the typed render the cross-check and last resort.
				if out, ok := d.generateCode(comp, g, root); ok {
					src = out
				}
				res, _ := d.runProgram(src)
				return wire.Response{OK: true, Say: runSummary(res)}, false, false
			}
			// It did not comprehend. A computation that names an operation GOLEM
			// has not learned, or is otherwise ill-formed, has its gap admitted
			// here; only a genuine raw program falls through to the compiler.
			if name, ok := d.code.UnknownOp(src); ok {
				// An unknown operation is an invitation to be taught, not only a gap:
				// for a plain call GOLEM holds what was asked and asks to be shown, so
				// the example that follows learns it. A richer surface still admits the gap.
				if say, asked := d.requestComputation(src, name); asked {
					return wire.Response{OK: true, Say: say}, false, false
				}
				return wire.Response{OK: true, Say: "I do not know how to compute the " + name + "."}, false, false
			}
			if d.code.IsComputationSurface(src) {
				return wire.Response{OK: true, Say: "I could not work that out."}, false, false
			}
		}
		rawRes, _ := d.runProgram(src)
		return wire.Response{OK: true, Say: runSummary(rawRes)}, false, false

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
			// A computation GOLEM asked to be shown takes the turn first: an example
			// pair of the awaited operation accumulates toward learning it, so a rule
			// can be gathered one example at a time across the exchange.
			if say, handled := d.advancePendingCompute(req.Content); handled {
				return wire.Response{OK: true, Say: say}, false, false
			}
			if op, ok := d.code.ComprehendDefinition(req.Content); ok {
				line, err := d.learnFunction(op)
				if err != nil {
					return wire.Response{Error: err.Error()}, false, false
				}
				return wire.Response{OK: true, Say: line + d.closeComputeLoop(op.Name)}, false, false
			}
			// A turn shaped like a definition whose body is a computation, which the
			// definition path declined on a soundness check, is a failed attempt rather
			// than a world fact: admit the gap by name instead of grounding "the larger
			// of a number ... is ..." as a statement about a thing named larger.
			if name, ok := d.code.IsDefinitionAttempt(req.Content); ok {
				return wire.Response{OK: true, Say: "I could not work out the " + name + "."}, false, false
			}
			// A function stated as the inverse of a known operation is discovered by
			// synthesis: the body whose output the operation returns to the input.
			if say, ok := d.comprehendInverse(req.Content); ok {
				return wire.Response{OK: true, Say: say}, false, false
			}
			// A list function stated by an extremal property is discovered the same
			// way: the running-fold whose output is the list's maximum or minimum.
			if say, ok := d.comprehendExtremal(req.Content); ok {
				return wire.Response{OK: true, Say: say}, false, false
			}
			// A function stated by a property is discovered too: "the abs is the
			// magnitude" works out the procedure whose output meets the property.
			if say, ok := d.comprehendProperty(req.Content); ok {
				return wire.Response{OK: true, Say: say}, false, false
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
				return wire.Response{OK: true, Say: line + d.closeComputeLoop(op.Name)}, false, false
			}
			// A function over a list can also be shown by example: from lists and their
			// results the faculty assembles and grounds a loop, the structure emerging
			// from the search rather than a template.
			if name, lists, outs, ok := d.code.ComprehendListExamples(req.Content); ok {
				op, found := d.synthesizeFold(name, lists, outs)
				if !found {
					return wire.Response{OK: true, Say: "I could not find a rule that fits those examples."}, false, false
				}
				line, err := d.learnFunction(op)
				if err != nil {
					return wire.Response{Error: err.Error()}, false, false
				}
				return wire.Response{OK: true, Say: line}, false, false
			}
			// A function whose result is a list is shown the same way, list in and list
			// out, and assembled as a map or a filter loop, grounded by running.
			if name, ins, outs, ok := d.code.ComprehendListReturnExamples(req.Content); ok {
				op, found := d.synthesizeListReturn(name, ins, outs)
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
		// Who is speaking sets how far this telling is trusted. The exchange is
		// observable: a `heard` event carries the source and the words, so a watcher
		// sees both sides of the conversation, GOLEM's `say` and what was said to it.
		speaker, speakerTrust := d.resolveSpeaker(req)
		d.event(events.Event{Ev: "heard", Cue: speaker, Tpl: wire.Clip(req.Content, 240)})
		// What is warm before the utterance is captured now, to ground the words
		// in once the planner has read the clause.
		attn := d.attentionAnchors()
		res := d.process(senses.Observation{Channel: "tell", Content: req.Content, At: time.Now()}, true, true)
		if res.FragmentID == 0 {
			return wire.Response{Error: "the words could not be stored; check the daemon log"}, false, false
		}
		// Tag the moment with its source, so a proposition comprehension draws from it
		// is weighed by that source's trust and stays auditable back to who said it. The
		// architect (master key or an untagged local telling) is left untagged, read at
		// full trust, so every existing telling is unchanged.
		if speaker != architectSource {
			d.tagMomentSource(res.FragmentID, speaker)
		}
		resp := wire.Response{OK: true, Observe: &res}
		// The grammar reads the utterance's force and the planner
		// decides the reply: an answer, an admission, or a staged
		// proposal. Verbatim memory above is untouched by any of it.
		// The answer language follows the question's: detected from
		// the closed class, with the spoken --lang tag as fallback.
		// Detection reads the tongue from the closed class; the spoken --lang tag is
		// the fallback, applied during the parse so a tongue's constructions are read
		// even when the clause carries no function word to detect it by.
		// The diacritic fold runs through this turn's resolves: a fresh per-turn record
		// so a restoration never leaks across turns, and the fold enabled for the main
		// pass (sniffTurkish toggles it off around its own probe below).
		d.restored = map[string]restoredNote{}
		d.foldEnabled = true
		frame := d.gram.ParseHint(req.Content, req.Lang, d.resolveIn(req.Lang))
		if req.Lang == "" && d.gram.Detect(req.Content) == "" && d.sniffTurkish(req.Content) {
			// An untagged clause no closed-class token places: the lexicon GOLEM
			// has grown gets a vote before English is assumed, so a bare Turkish
			// telling answers in its own tongue. A clause detection already
			// placed is never second-guessed.
			frame = d.gram.ParseHint(req.Content, "tr", d.resolveIn("tr"))
			if frame.Lang == "" {
				frame.Lang = "tr"
			}
		}
		if frame.Lang == "" {
			frame.Lang = "en" // English is the default tongue when none was detected or tagged
		}
		// The question-side construction traversal drives the shapes it reads completely: the
		// copular is-a, the comparison, and its degree question carry every field their planners
		// read, so the walk's frame is preferred and the coded parse becomes the cross-check (a
		// disagreement is logged) and the fallback for the shapes the walk declines. The walk is a
		// recognition reading, so the residual the coded span walk alone gathers (the default tongue
		// and the unresolved words a curiosity follow-up asks after) is carried across. A wh
		// role-question keeps the coded frame: its roles and that residual are read off the tokens,
		// so its cutover waits on the planner reading the meaning graph rather than the frame.
		if walked, wok := d.gram.ParseByTraversal(frame.Lang, req.Content, d.resolveIn(frame.Lang)); wok {
			if !framesAgree(walked, frame) {
				fmt.Fprintf(d.out, "parse traversal mismatch: %q\n", req.Content)
			} else if walked.Wh == "" {
				walked.Lang, walked.Unknown = frame.Lang, frame.Unknown
				frame = walked
			}
		}
		d.statementAbandoned = false
		resp.Say, resp.Decision = d.plan(frame, req.Content, res.FragmentID)
		// The words enter the lexicon only after the clause is read, so an
		// inflection keys on the lemma comprehension has by now established: a
		// verb's surface ("kovaladı") reinforces the relation word ("kovala")
		// instead of minting a node before its lemma is a concept. They keep
		// company with the recent moments the utterance bound to. A peer's words
		// bond at its trust, so a low-trust source's vocabulary entrenches weakly
		// and its naming never firms up on one telling; the architect's is full.
		// Only a declarative telling names. A yes/no question parses with statement force
		// (its clause is checked against the graph by planPolar, not written), so force
		// alone would read it as a naming; the polar shape is detected the same way planPolar
		// does and excluded. Without this an untagged question misread in another tongue
		// ("kedi fareyi izler mi" taken as English) would lay its words as company of a
		// percept that merely lingered warm, a spurious name the night then entrenches.
		_, _, polar := polarClause(frame.Lang, req.Content)
		declarative := frame.Force == grammar.ForceStatement && !polar
		// A declarative lays the naming company consolidation later reads; a question or a
		// command lays none, so its words keep no company with what was warm. The word node
		// may still be created, but inert, with nothing for naming to read.
		// A telling comprehension read as a proposition and then abandoned (a
		// misparse) names nothing either: its words must stay inert rather than
		// keep company with whatever lingered warm.
		var namingAnchors []recentMoment
		if declarative && !d.statementAbandoned {
			namingAnchors = d.recentNaming(time.Now(), res.FragmentID)
		}
		mintLang := req.Lang
		if mintLang == "" && frame.Lang == "tr" {
			mintLang = "tr" // a sniffed Turkish clause mints its words in its own tongue, not as English shadows
		}
		// A comparative surface ("buyuk" of "büyük") is a gradable pole named through the
		// scale block, not a lexeme of its own. Its pole resolves late, in storeComparison,
		// after this mint, so a lossy ASCII spelling would be minted before the fold could
		// mark it restored. Pre-resolve the comparative adjective here, so the fold records
		// the lossy form and the word mint below skips it: one word, one node.
		if frame.Comparative || frame.Magnitude {
			d.comparativeToPole(frame.Action.Phrase, mintLang)
		}
		words := d.mintWordsScaled(req.Content, mintLang, res.FragmentID, namingAnchors, false, speakerTrust)
		d.warmConcepts(words, exciteSpeech)
		// Grounding the words in what was merely held in mind waits on the force:
		// only a declarative naming grounds a word in the attended concept. A
		// question or a discourse move ("what else") names nothing, so its words do
		// not bind to whatever was warm, and a proposition's words already bound to
		// their roles, so an argument is not also named after its co-arguments. A
		// comparison names nothing either: "the horse is bigger than the dog" states
		// an ordering, so its words must not ground in the standard that was warm
		// (else the dog's word keeps company with the horse and they blur).
		if declarative && !frame.Comparative && !d.momentBoundRoles(res.FragmentID) && !d.statementAbandoned {
			// A diacritic-restored surface is dropped from the naming company here too:
			// the word-form mint already skipped it, so it is absent from words, but the
			// naming sites are kept explicitly clear of it so no RelNames alias from a
			// "buyuk" row to "büyük" is ever born and a bare token stays doubted each turn.
			named := d.dropRestoredWords(words)
			d.bindAttention(named, attn)
			// Record what the naming pointed at, the freshest percept, as a salience
			// the abstraction pass reads apart from the uniform company: a name said
			// over one constant percept ostends it alone, a category said over varying
			// members spreads its ostension, so a cross-tongue name diluted by a
			// lingering scene-mate binds to its referent rather than splitting a phantom.
			d.recordOstension(named, time.Now(), res.FragmentID)
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

	case "trust":
		// The architect sets whom GOLEM trusts, so a source it should believe is
		// vouched for and one it should doubt is held low. Only the architect's
		// signature authorizes it, so a peer cannot raise its own trust.
		if !d.reqPrincipal.architectSigned {
			return wire.Response{Error: "setting trust needs the architect's signature"}, false, false
		}
		name := req.Query
		if name == "" {
			return wire.Response{Error: "trust needs a source name"}, false, false
		}
		if name == architectSource {
			return wire.Response{OK: true, Say: "The architect is trusted 1.00 by the master key."}, false, false
		}
		if req.Command == "set" {
			if err := d.setSourceTrust(name, req.Weight); err != nil {
				return wire.Response{Error: err.Error()}, false, false
			}
			note := fmt.Sprintf("%.2f", req.Weight)
			if d.reqPrincipal.sigFingerprint != "" {
				note += " (signed " + d.reqPrincipal.sigFingerprint + ")"
			}
			if err := d.store.Audit(time.Now().Unix(), "trust", name, note); err != nil {
				fmt.Fprintf(d.out, "audit write failed: %v\n", err)
			}
			return wire.Response{OK: true, Say: fmt.Sprintf("%q is now trusted %.2f.", name, req.Weight)}, false, false
		}
		src, err := d.ensureSource(name)
		if err != nil {
			return wire.Response{Error: err.Error()}, false, false
		}
		return wire.Response{OK: true, Say: fmt.Sprintf("%q is trusted %.2f.", name, d.sourceTrust(src.ID))}, false, false

	case "enroll":
		// The architect binds a peer's public key to its source name, so the
		// peer can sign its own tellings and be heard as exactly that source.
		// Only the architect's signature authorizes it, so no peer can enroll
		// itself. An empty key revokes.
		if !d.reqPrincipal.architectSigned {
			return wire.Response{Error: "enrolling a source key needs the architect's signature"}, false, false
		}
		name := strings.TrimSpace(req.Query)
		if name == "" || name == architectSource {
			return wire.Response{Error: "enroll needs a peer source name"}, false, false
		}
		pubB64 := strings.TrimSpace(req.Content)
		if err := d.setSourceKey(name, pubB64); err != nil {
			return wire.Response{Error: err.Error()}, false, false
		}
		note := "enrolled"
		if pubB64 == "" {
			note = "revoked"
		}
		if d.reqPrincipal.sigFingerprint != "" {
			note += " (signed " + d.reqPrincipal.sigFingerprint + ")"
		}
		if err := d.store.Audit(time.Now().Unix(), "enroll", name, note); err != nil {
			fmt.Fprintf(d.out, "audit write failed: %v\n", err)
		}
		if pubB64 == "" {
			return wire.Response{OK: true, Say: fmt.Sprintf("%q can no longer prove its identity; its key is revoked.", name)}, false, false
		}
		return wire.Response{OK: true, Say: fmt.Sprintf("%q can now prove its identity with its own key.", name)}, false, false

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
		w, _, err := d.mintWordNode(req.Content, lang, d.chem.Snapshot(), time.Now())
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

	case "scale":
		// Teach a gradable scale: its dimension and the greater and lesser pole
		// adjectives, with the scale membership, the pole roles, and the antonymy
		// that let a comparison compose onto it. The poles and scale are grown
		// content (resolvable, counted), minted once by template so a re-teach is
		// idempotent; the comparative surfaces are taught apart by cmpform, and a
		// second tongue's words are added by name onto these same concepts.
		if req.Content == "" || req.Query == "" || req.Command == "" {
			return wire.Response{Error: "scale needs a dimension, a greater adjective, and a lesser adjective"}, false, false
		}
		lang := req.Lang
		if lang == "" {
			lang = "en"
		}
		dim := d.ensureClassConcept("scale:"+req.Content, req.Content)
		greater := d.ensureClassConcept("pole:"+req.Content+":greater", req.Query)
		lesser := d.ensureClassConcept("pole:"+req.Content+":lesser", req.Command)
		greaterRole, lesserRole := d.poleRoleID("greater"), d.poleRoleID("lesser")
		if dim == 0 || greater == 0 || lesser == 0 || greaterRole == 0 || lesserRole == 0 {
			return wire.Response{Error: "could not build the scale block"}, false, false
		}
		d.nameConceptWord(dim, req.Content, lang)
		d.nameConceptWord(greater, req.Query, lang)
		d.nameConceptWord(lesser, req.Command, lang)
		d.store.SetLinkRel(greater, dim, memory.RelScale, deliberateName)
		d.store.SetLinkRel(lesser, dim, memory.RelScale, deliberateName)
		d.store.SetLinkRel(greater, greaterRole, memory.RelMarks, deliberateName)
		d.store.SetLinkRel(lesser, lesserRole, memory.RelMarks, deliberateName)
		d.store.SetLinkRel(greater, lesser, memory.RelOpposite, deliberateName)
		d.store.SetLinkRel(lesser, greater, memory.RelOpposite, deliberateName)
		if err := d.store.Audit(time.Now().Unix(), "scale", req.Content, req.Query+" over "+req.Command); err != nil {
			fmt.Fprintf(d.out, "audit write failed: %v\n", err)
		}
		return wire.Response{OK: true, Say: fmt.Sprintf("%s grades %s over %s.", req.Content, req.Query, req.Command)}, false, false

	case "cmpform":
		// Teach a pole adjective's comparative surface in a tongue ("big" -> "bigger",
		// "daha buyuk", "maior"), the durable form the serve path speaks. The pole is
		// resolved by the adjective word, so a second tongue's form rides the pole once
		// that tongue's name for it exists.
		if req.Content == "" || req.Query == "" {
			return wire.Response{Error: "cmpform needs an adjective and its comparative form"}, false, false
		}
		lang := req.Lang
		if lang == "" {
			lang = "en"
		}
		pole := d.resolveConcept(req.Content)
		if pole == nil {
			return wire.Response{Error: fmt.Sprintf("no gradable adjective matches %q", req.Content)}, false, false
		}
		if _, ok := d.scaleOf(pole); !ok {
			return wire.Response{Error: fmt.Sprintf("%q is not a scale pole; teach its scale first", req.Content)}, false, false
		}
		// A hand-stored surface is an irregular override: assertRealizedForm tags it
		// meta:irregular so the reverse lookup owns its token before any stemming.
		d.assertRealizedForm(pole, "", lang, req.Query)
		if err := d.store.Audit(time.Now().Unix(), "cmpform", req.Content, req.Query); err != nil {
			fmt.Fprintf(d.out, "audit write failed: %v\n", err)
		}
		return wire.Response{OK: true, Say: fmt.Sprintf("the comparative of %s is %q.", pole.Content, req.Query)}, false, false

	case "cmprule":
		// Teach and tag the regular comparative morphology rule for a tongue: ensure the
		// rule function exists in the code faculty, run the pure-op determinism guard, and
		// RelMarks it (cmp:comparative + meta:regular + tongue) so the keyed selection walk
		// finds it. The rule composition is Go; rule / regular and the tongue are graph data.
		lang := req.Lang
		if lang == "" {
			lang = "en"
		}
		op, ok := comparativeRule(lang)
		if !ok {
			return wire.Response{Error: fmt.Sprintf("no productive comparative rule for %q", lang)}, false, false
		}
		if op.Body == nil || !isPureMorph(*op.Body) {
			return wire.Response{Error: "a comparative rule must compose only the pure string primitives"}, false, false
		}
		if _, err := d.learnFunction(op); err != nil {
			return wire.Response{Error: err.Error()}, false, false
		}
		node, err := d.store.FragmentByTemplate("code:" + op.Name)
		if err != nil || node == nil || !d.tagComparativeRule(node.ID, lang) {
			return wire.Response{Error: "could not tag the comparative rule"}, false, false
		}
		if err := d.store.Audit(time.Now().Unix(), "cmprule", op.Name, lang); err != nil {
			fmt.Fprintf(d.out, "audit write failed: %v\n", err)
		}
		return wire.Response{OK: true, Say: fmt.Sprintf("the %s comparative rule is %s.", lang, op.Name)}, false, false

	case "cmpderive":
		// Run the tagged rule over every regular pole lacking a form and freeze the derived
		// surface into the durable edge, the final grow sweep. Requires the runtime;
		// idempotent (a pole already carrying a form is skipped) and re-runnable.
		if !d.cfg.RuntimeEnabled {
			return wire.Response{Error: "cmpderive needs the runtime open"}, false, false
		}
		tongues := []string{"en", "tr"}
		if req.Lang != "" {
			tongues = []string{req.Lang}
		}
		derived := 0
		for _, pole := range d.allPoles() {
			for _, tongue := range tongues {
				if _, ok := d.deriveComparativeForm(pole, tongue, ""); ok {
					derived++
				}
			}
		}
		if err := d.store.Audit(time.Now().Unix(), "cmpderive", fmt.Sprintf("%d", derived), strings.Join(tongues, ",")); err != nil {
			fmt.Fprintf(d.out, "audit write failed: %v\n", err)
		}
		return wire.Response{OK: true, Say: fmt.Sprintf("derived %d comparative forms.", derived)}, false, false

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
		infos = dedupeInfos(infos)
		// A recalled percept shows what was actually seen, not only how it was described.
		d.attachMedia(infos)
		return wire.Response{OK: true, Fragments: infos}, false, false

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
		if req.Inflect {
			pairs, err := parseInflectPairs(strings.Fields(req.Command))
			if err != nil {
				return wire.Response{Error: err.Error()}, false, false
			}
			if err := d.teachInflection(req.Lang, req.Content, pairs); err != nil {
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

	case "runtime":
		return d.handleRuntime(req), false, false

	case "retune":
		return d.handleRetune(req), false, false

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
