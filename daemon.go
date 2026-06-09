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

	store      *Store
	chem       *Neurochemistry
	predictors map[string]*Predictor
	stats      map[string]*ChannelStat

	obs  chan Observation
	reqs chan ipcReq
	stop chan struct{} // closed to stop sensor goroutines

	started      time.Time
	lastObs      time.Time
	lastSurprise time.Time
	lastSleep    time.Time
	sleeps       int
}

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
		home:       home,
		cfg:        cfg,
		out:        out,
		store:      store,
		chem:       NewNeurochemistry(),
		predictors: make(map[string]*Predictor),
		stats:      make(map[string]*ChannelStat),
		obs:        make(chan Observation, 256),
		reqs:       make(chan ipcReq),
		stop:       make(chan struct{}),
	}
	if err := d.loadModels(); err != nil {
		store.Close()
		return nil, err
	}
	return d, nil
}

// loadModels restores every channel predictor persisted by earlier runs,
// so a restart does not reset what counts as "normal."
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
		p := NewPredictor(d.cfg.PredictorWindow)
		p.Seed(counts)
		d.predictors[ch] = p
	}
	return nil
}

func (d *Daemon) predictor(channel string) *Predictor {
	p, ok := d.predictors[channel]
	if !ok {
		p = NewPredictor(d.cfg.PredictorWindow)
		d.predictors[channel] = p
	}
	return p
}

func (d *Daemon) stat(channel string) *ChannelStat {
	st, ok := d.stats[channel]
	if !ok {
		st = &ChannelStat{}
		d.stats[channel] = st
	}
	return st
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

	homeostasis := time.NewTicker(200 * time.Millisecond)
	defer homeostasis.Stop()

	for {
		select {
		case o := <-d.obs:
			d.process(o)

		case r := <-d.reqs:
			resp, quit := d.handle(r.req)
			r.reply <- resp
			if quit {
				d.shutdown(listener, sock)
				return nil
			}

		case <-homeostasis.C:
			d.chem.Decay()
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
	conn.SetDeadline(time.Now().Add(10 * time.Second))

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
// learn, and encode if it clears the gate.
func (d *Daemon) process(o Observation) ObserveResult {
	d.lastObs = time.Now()
	st := d.stat(o.Channel)
	st.Observed++

	tpl := Normalize(o.Content)
	p := d.predictor(o.Channel)
	bits := p.Surprise(tpl)
	p.Learn(tpl)

	if bits < d.cfg.SurpriseThreshold {
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
	fmt.Fprintf(d.out, "[!] %.2f bits  %-8s %q -> %q  NA=%.2f  #%d\n",
		bits, o.Channel, o.Content, tpl, frag.Limbic.Noradrenaline, id)
	return ObserveResult{Bits: bits, Conscious: true, FragmentID: id}
}

func (d *Daemon) readyToSleep() bool {
	return d.chem.Serotonin() >= d.cfg.SleepPressureGate &&
		time.Since(d.lastObs) >= d.cfg.InputSilence() &&
		time.Since(d.lastSleep) >= d.cfg.SleepInterval()
}

// sleepCycle is one consolidation night: decay and prune in the store,
// flush predictor state, wake with reset sleep pressure.
func (d *Daemon) sleepCycle() {
	pruned, err := d.store.Sleep(d.cfg.NightlyDecay, d.cfg.PruneFloor)
	if err != nil {
		fmt.Fprintf(d.out, "sleep error: %v\n", err)
		return
	}
	d.flushModels()
	d.chem.WakeRested()
	d.sleeps++
	d.lastSleep = time.Now()
	held, _ := d.store.Count()
	fmt.Fprintf(d.out, "zzz night %d: pruned %d faded fragment(s), %d held.\n",
		d.sleeps, pruned, held)
}

func (d *Daemon) flushModels() {
	for ch, p := range d.predictors {
		if err := d.store.SaveModel(ch, p.Export()); err != nil {
			fmt.Fprintf(d.out, "model flush error on %s: %v\n", ch, err)
		}
	}
}

func (d *Daemon) shutdown(listener net.Listener, sock string) {
	close(d.stop)
	listener.Close()

	pruned, err := d.store.Sleep(d.cfg.NightlyDecay, d.cfg.PruneFloor)
	if err == nil {
		d.sleeps++
		fmt.Fprintf(d.out, "zzz final night %d: pruned %d fragment(s).\n", d.sleeps, pruned)
	}
	d.flushModels()
	d.store.Close()
	os.Remove(sock)
	fmt.Fprintln(d.out, "golem asleep.")
}

// handle services one CLI request inside the brain loop. The second return
// value reports whether the daemon should shut down.
func (d *Daemon) handle(req Request) (Response, bool) {
	switch req.Cmd {
	case "status":
		held, _ := d.store.Count()
		return Response{OK: true, Status: &StatusInfo{
			PID:           os.Getpid(),
			UptimeSec:     time.Since(d.started).Seconds(),
			Fragments:     held,
			Channels:      d.stats,
			Sleeps:        d.sleeps,
			LastSleepUnix: d.lastSleep.Unix(),
			Chem:          d.chem.Snapshot(),
		}}, false

	case "chem":
		chem := d.chem.Snapshot()
		return Response{OK: true, Chem: &chem}, false

	case "observe":
		if req.Channel == "" || req.Content == "" {
			return Response{Error: "observe needs a channel and content"}, false
		}
		res := d.process(Observation{Channel: req.Channel, Content: req.Content, At: time.Now()})
		return Response{OK: true, Observe: &res}, false

	case "recall":
		var frags []*Fragment
		var err error
		if req.Query == "" {
			var f *Fragment
			f, err = d.store.RecallByStress()
			if f != nil {
				frags = []*Fragment{f}
			}
		} else {
			frags, err = d.store.Recall(req.Query, 5)
		}
		if err != nil {
			return Response{Error: err.Error()}, false
		}
		infos := make([]*FragmentInfo, 0, len(frags))
		for _, f := range frags {
			infos = append(infos, fragmentInfo(f))
		}
		return Response{OK: true, Fragments: infos}, false

	case "dump":
		frags, err := d.store.Dump()
		if err != nil {
			return Response{Error: err.Error()}, false
		}
		infos := make([]*FragmentInfo, 0, len(frags))
		for _, f := range frags {
			infos = append(infos, fragmentInfo(f))
		}
		return Response{OK: true, Fragments: infos}, false

	case "stop":
		return Response{OK: true}, true

	default:
		return Response{Error: "unknown command: " + req.Cmd}, false
	}
}
