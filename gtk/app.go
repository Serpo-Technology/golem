package main

import (
	"context"
	"encoding/base64"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"golem/wire"
	"gtk/client"
)

// App is the bridge the window calls into: each method is one request to the
// running brain through the shared client, over the signed network transport to
// the container or a local socket beside it. The window reads and shapes the
// live brain and never touches the database itself. The brain's own event
// stream is forwarded to the window as it happens, so its inner life shows live.
type App struct {
	ctx      context.Context
	c        *client.Client
	err      error
	stopFeed context.CancelFunc
}

// NewApp builds the bridge, resolving the connection once.
func NewApp() *App {
	c, err := client.New()
	return &App{c: c, err: err}
}

// startup stores the window context and starts forwarding the brain's events.
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	a.startFeed()
}

// startFeed opens the event subscription and re-emits each event to the window.
// It is safe to call again after a reconnect: the prior feed is cancelled first.
func (a *App) startFeed() {
	if a.err != nil || a.ctx == nil {
		return
	}
	if a.stopFeed != nil {
		a.stopFeed()
	}
	ctx, cancel := context.WithCancel(a.ctx)
	a.stopFeed = cancel
	go a.c.Subscribe(ctx,
		func(ev client.Event) { runtime.EventsEmit(a.ctx, "golem:event", ev) },
		func(connected bool, err error) {
			msg := ""
			if err != nil {
				msg = err.Error()
			}
			runtime.EventsEmit(a.ctx, "golem:stream", map[string]any{"connected": connected, "error": msg})
		})
}

// --- Connection ---

// Connect points the toolkit at a brain and reloads its key, restarts the event
// feed, and reports the resulting connection.
func (a *App) Connect(net, addr, keySource string) (client.Info, error) {
	if a.err != nil {
		return client.Info{}, a.err
	}
	info, err := a.c.Reconfigure(net, addr, keySource)
	if err != nil {
		return info, err
	}
	a.startFeed()
	return info, nil
}

// Keypair is a freshly minted architect keypair for the window to show: the
// public half to register with a brain, the private half for the architect to
// keep. GOLEM never mints one.
type Keypair struct {
	Public  string `json:"public"`
	Private string `json:"private"`
}

// GenerateKeypair mints an architect keypair in the toolkit, the architect's
// own instrument, so the brain never produces the confidential half.
func (a *App) GenerateKeypair() (*Keypair, error) {
	pub, priv, err := client.GenerateKeypair()
	if err != nil {
		return nil, err
	}
	return &Keypair{Public: pub, Private: priv}, nil
}

// ConnInfo reports where the toolkit points and whether a key is loaded.
func (a *App) ConnInfo() (client.Info, error) {
	if a.err != nil {
		return client.Info{}, a.err
	}
	return a.c.Info(), nil
}

// --- Reads ---

func (a *App) Status() (*wire.StatusInfo, error) {
	if a.err != nil {
		return nil, a.err
	}
	return a.c.Status()
}
func (a *App) Chem() (*wire.Chem, error) {
	if a.err != nil {
		return nil, a.err
	}
	return a.c.Chem()
}
func (a *App) Concepts() ([]*wire.ConceptInfo, error) {
	if a.err != nil {
		return nil, a.err
	}
	return a.c.Concepts()
}
func (a *App) Cognition() (*wire.CognitionInfo, error) {
	if a.err != nil {
		return nil, a.err
	}
	return a.c.Cognition()
}
func (a *App) Health() (*wire.Health, error) {
	if a.err != nil {
		return nil, a.err
	}
	return a.c.Health()
}
func (a *App) Audit() ([]*wire.AuditInfo, error) {
	if a.err != nil {
		return nil, a.err
	}
	return a.c.Audit()
}
func (a *App) Habits() ([]*wire.HabitInfo, error) {
	if a.err != nil {
		return nil, a.err
	}
	return a.c.Habits()
}
func (a *App) Search(query string) ([]*wire.FragmentInfo, error) {
	if a.err != nil {
		return nil, a.err
	}
	return a.c.Search(query)
}

// Graph is the whole memory graph for the window: every fragment and the links
// among them, the same glass box the audit trail records.
type Graph struct {
	Fragments []*wire.FragmentInfo `json:"fragments"`
	Links     []*wire.LinkInfo     `json:"links"`
}

// Dump reads the whole graph for inspection.
func (a *App) Dump() (*Graph, error) {
	if a.err != nil {
		return nil, a.err
	}
	frags, links, err := a.c.Dump()
	if err != nil {
		return nil, err
	}
	return &Graph{Fragments: frags, Links: links}, nil
}

// --- Conversation and perception ---

func (a *App) Tell(content, lang string) (*wire.Response, error) {
	if a.err != nil {
		return nil, a.err
	}
	return a.c.Tell(content, lang)
}

// See shows GOLEM an image from the window: the bytes arrive base64 encoded and
// GOLEM answers in its own words with what it recognized.
func (a *App) See(b64, mime, lang string) (*wire.Response, error) {
	if a.err != nil {
		return nil, a.err
	}
	data, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		return nil, err
	}
	return a.c.See(data, mime, lang)
}

func (a *App) Observe(channel, content string) (*wire.Response, error) {
	if a.err != nil {
		return nil, a.err
	}
	return a.c.Observe(channel, content, nil, "", false)
}

// --- Teaching, proposals, identity ---

func (a *App) Teach(cue, command, lang string, say, inflect, mind bool) error {
	if a.err != nil {
		return a.err
	}
	return a.c.Teach(cue, command, lang, say, inflect, mind)
}
func (a *App) Propose(content string) (*wire.DecisionInfo, error) {
	if a.err != nil {
		return nil, a.err
	}
	return a.c.Propose(content)
}
func (a *App) Approve() (*wire.ActionResult, error) {
	if a.err != nil {
		return nil, a.err
	}
	return a.c.Approve()
}
func (a *App) Deny() (*wire.ActionResult, error) {
	if a.err != nil {
		return nil, a.err
	}
	return a.c.Deny()
}
func (a *App) Trust(source string, level float64, set bool) (string, error) {
	if a.err != nil {
		return "", a.err
	}
	return a.c.Trust(source, level, set)
}
func (a *App) Enroll(source, pubkey string) (string, error) {
	if a.err != nil {
		return "", a.err
	}
	return a.c.Enroll(source, pubkey)
}
func (a *App) Name(query, word, lang string) (string, error) {
	if a.err != nil {
		return "", a.err
	}
	return a.c.Name(query, word, lang)
}

// --- Controls ---

func (a *App) Retune(file, constant, value string) (string, error) {
	if a.err != nil {
		return "", a.err
	}
	return a.c.Retune(file, constant, value)
}
func (a *App) Runtime(arg string) (string, error) {
	if a.err != nil {
		return "", a.err
	}
	return a.c.Runtime(arg)
}

// VoiceState carries both fields the window shows after a voice change.
type VoiceState struct {
	Voice *wire.VoiceInfo `json:"voice"`
	Say   string          `json:"say"`
}

func (a *App) Voice(arg, name string) (*VoiceState, error) {
	if a.err != nil {
		return nil, a.err
	}
	v, say, err := a.c.Voice(arg, name)
	if err != nil {
		return nil, err
	}
	return &VoiceState{Voice: v, Say: say}, nil
}
func (a *App) Sleep() error {
	if a.err != nil {
		return a.err
	}
	return a.c.Sleep()
}
func (a *App) Forget(query string) (string, error) {
	if a.err != nil {
		return "", a.err
	}
	return a.c.Forget(query)
}

// --- Memory surgery ---

func (a *App) Edit(id uint64, content string) error {
	if a.err != nil {
		return a.err
	}
	return a.c.Edit(id, content)
}
func (a *App) Reweight(id uint64, weight float64) error {
	if a.err != nil {
		return a.err
	}
	return a.c.Reweight(id, weight)
}
func (a *App) Delete(id uint64) error {
	if a.err != nil {
		return a.err
	}
	return a.c.Delete(id)
}
func (a *App) Mark(id uint64) error {
	if a.err != nil {
		return a.err
	}
	return a.c.Mark(id)
}
