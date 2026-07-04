// Package client is the toolkit's connection to a running GOLEM brain: a wrapper
// over the wire protocol that both the console and the window call, so the two
// share one path to the brain and read what it returns the same way. The daemon
// stays the only process that touches the database; this is a client to its
// single brain loop.
//
// The toolkit's brain runs in a container, reached over the signed TCP transport
// rather than the local unix socket, so every request the toolkit sends is
// signed with the architect's key. The client still speaks to a local socket
// when pointed at one, unsigned, for a brain running beside it.
package client

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"golem/wire"
)

// Client talks to a GOLEM brain over the wire, on TCP with a signature or on a
// local socket without one. Its connection can be reconfigured at runtime from
// the toolkit's settings.
type Client struct {
	home string

	mu   sync.RWMutex
	net  string // "tcp" or "unix"
	addr string // host:port for tcp, socket path for unix
	priv string // base64 signing key, empty when none is loaded
}

// conn is the persisted connection setting, so the toolkit reconnects to the
// same brain on the next launch.
type conn struct {
	Net     string `json:"net"`      // "tcp" (default) or "unix"
	Addr    string `json:"addr"`     // host:port or socket path
	KeyFile string `json:"key_file"` // where the signing key is read from
}

// Info reports the current connection to the window: where it points and
// whether a signing key is loaded, without ever exposing the key.
type Info struct {
	Net    string `json:"net"`
	Addr   string `json:"addr"`
	Signed bool   `json:"signed"`
}

const defaultTCP = "127.0.0.1:7777"

// New builds the client from the saved connection, or the default: the
// container's brain on host loopback, signed with the architect key found in
// the environment. A first run with no saved setting and no key still connects
// read-only attempts will report the missing key clearly.
func New() (*Client, error) {
	home, err := wire.Home()
	if err != nil {
		return nil, err
	}
	c := &Client{home: home, net: "tcp", addr: defaultTCP}
	saved, keyFile := c.loadConn()
	c.net, c.addr = saved.Net, saved.Addr
	priv, _ := loadKey(keyFile)
	c.priv = priv
	return c, nil
}

// Reconfigure points the client at a different brain and reloads its key, then
// persists the choice. keySource is a signing key file, or empty to read the
// environment. It returns the resulting connection info.
func (c *Client) Reconfigure(net, addr, keySource string) (Info, error) {
	net = strings.TrimSpace(net)
	if net == "" {
		net = "tcp"
	}
	if net != "tcp" && net != "unix" {
		return Info{}, fmt.Errorf("transport must be tcp or unix")
	}
	addr = strings.TrimSpace(addr)
	if addr == "" {
		if net == "tcp" {
			addr = defaultTCP
		} else {
			addr = wire.SocketPath(c.home)
		}
	}
	priv, err := loadKey(keySource)
	if err != nil {
		return Info{}, err
	}
	c.mu.Lock()
	c.net, c.addr, c.priv = net, addr, priv
	c.mu.Unlock()
	c.saveConn(conn{Net: net, Addr: addr, KeyFile: keySource})
	return c.Info(), nil
}

// Info reports the current connection.
func (c *Client) Info() Info {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return Info{Net: c.net, Addr: c.addr, Signed: c.priv != ""}
}

// call sends one request and reads one reply, signing it first when the brain
// is reached over the network. A network brain with no key loaded is refused
// here with a clear reason rather than a bare transport error from the far end.
func (c *Client) call(req wire.Request) (wire.Response, error) {
	c.mu.RLock()
	net, addr, priv := c.net, c.addr, c.priv
	c.mu.RUnlock()
	if net == "tcp" {
		if priv == "" {
			return wire.Response{}, errors.New("no signing key: set the key in Settings, or GOLEM_PRIVATE_KEY")
		}
		if err := wire.SignRequest(priv, &req); err != nil {
			return wire.Response{}, err
		}
	}
	return wire.CallAddr(net, addr, req)
}

// --- Reads ---

// Status reads the daemon's vital signs and workspace.
func (c *Client) Status() (*wire.StatusInfo, error) {
	resp, err := c.call(wire.Request{Cmd: "status"})
	return resp.Status, err
}

// Chem reads the four-plus-one chemical levels.
func (c *Client) Chem() (*wire.Chem, error) {
	resp, err := c.call(wire.Request{Cmd: "chem"})
	return resp.Chem, err
}

// Search recalls the moments a cue brings to mind.
func (c *Client) Search(query string) ([]*wire.FragmentInfo, error) {
	resp, err := c.call(wire.Request{Cmd: "recall", Query: query})
	return resp.Fragments, err
}

// Concepts lists the concepts memory has distilled.
func (c *Client) Concepts() ([]*wire.ConceptInfo, error) {
	resp, err := c.call(wire.Request{Cmd: "concepts"})
	return resp.Concepts, err
}

// Audit reads the decision and edit trail.
func (c *Client) Audit() ([]*wire.AuditInfo, error) {
	resp, err := c.call(wire.Request{Cmd: "audit"})
	return resp.Audit, err
}

// Cognition reads GOLEM's reasoning state: resemblance, beliefs, and what it has
// kept, the view for watching a brain grow rich enough to reason.
func (c *Client) Cognition() (*wire.CognitionInfo, error) {
	resp, err := c.call(wire.Request{Cmd: "cognition"})
	return resp.Cognition, err
}

// Health audits the naming graph for the two shapes of consolidation pollution.
func (c *Client) Health() (*wire.Health, error) {
	resp, err := c.call(wire.Request{Cmd: "health"})
	return resp.Health, err
}

// Habits lists the cue-to-command habits and their graduation state.
func (c *Client) Habits() ([]*wire.HabitInfo, error) {
	resp, err := c.call(wire.Request{Cmd: "habits"})
	return resp.Habits, err
}

// Dump reads the whole graph: every fragment and the links among them.
func (c *Client) Dump() ([]*wire.FragmentInfo, []*wire.LinkInfo, error) {
	resp, err := c.call(wire.Request{Cmd: "dump"})
	return resp.Fragments, resp.Links, err
}

// --- Conversation and perception ---

// Tell speaks to GOLEM: the words become a memory, and the reply is what it
// says back, with any proposal it staged and the moments it brought to mind.
func (c *Client) Tell(content, lang string) (*wire.Response, error) {
	resp, err := c.call(wire.Request{Cmd: "tell", Content: content, Lang: lang})
	return &resp, err
}

// See shows GOLEM an image: it measures the pixels and answers in its own words,
// with the moments the sighting recognized.
func (c *Client) See(media []byte, mime, lang string) (*wire.Response, error) {
	resp, err := c.call(wire.Request{Cmd: "see", Media: media, Mime: mime, Lang: lang})
	return &resp, err
}

// Observe files a grounded fragment on a channel through the perception
// pipeline, optionally carrying the raw bytes a percept was captioned from.
func (c *Client) Observe(channel, content string, media []byte, mime string, force bool) (*wire.Response, error) {
	resp, err := c.call(wire.Request{Cmd: "observe", Channel: channel, Content: content,
		Media: media, Mime: mime, Force: force})
	return &resp, err
}

// --- Teaching, proposals, and identity ---

// Teach grounds a cue to a command, a construction, an inflection, or a thought
// operation, matching golem teach and its flags.
func (c *Client) Teach(cue, command, lang string, say, inflect, mind bool) error {
	_, err := c.call(wire.Request{Cmd: "teach", Content: cue, Command: command,
		Lang: lang, Say: say, Inflect: inflect, Mind: mind})
	return err
}

// Propose stages a command from a cue, behind the same approve and deny the
// autonomous path uses.
func (c *Client) Propose(content string) (*wire.DecisionInfo, error) {
	resp, err := c.call(wire.Request{Cmd: "propose", Content: content})
	return resp.Decision, err
}

// Approve runs the staged proposal; Deny rejects it.
func (c *Client) Approve() (*wire.ActionResult, error) {
	resp, err := c.call(wire.Request{Cmd: "approve"})
	return resp.Action, err
}
func (c *Client) Deny() (*wire.ActionResult, error) {
	resp, err := c.call(wire.Request{Cmd: "deny"})
	return resp.Action, err
}

// Trust sets or reads how far GOLEM trusts a source (needs the architect key).
func (c *Client) Trust(source string, level float64, set bool) (string, error) {
	req := wire.Request{Cmd: "trust", Query: source}
	if set {
		req.Command = "set"
		req.Weight = level
	}
	resp, err := c.call(req)
	return resp.Say, err
}

// Enroll registers or revokes a peer's public key so it can prove its identity
// (needs the architect key). An empty key revokes.
func (c *Client) Enroll(source, pubkey string) (string, error) {
	resp, err := c.call(wire.Request{Cmd: "enroll", Query: source, Content: pubkey})
	return resp.Say, err
}

// Name teaches a word for a concept resolved from a phrase.
func (c *Client) Name(query, word, lang string) (string, error) {
	resp, err := c.call(wire.Request{Cmd: "name", Query: query, Content: word, Lang: lang})
	return resp.Say, err
}

// --- Controls ---

// Retune builds and tests a candidate with one tuning constant changed (needs
// the architect key); the build runs off the brain loop and speaks its verdict.
func (c *Client) Retune(file, constant, value string) (string, error) {
	resp, err := c.call(wire.Request{Cmd: "retune", Query: file, Arg: constant, Content: value})
	return resp.Say, err
}

// Runtime toggles the code runtime; Voice toggles the say stream and its voice.
func (c *Client) Runtime(arg string) (string, error) {
	resp, err := c.call(wire.Request{Cmd: "runtime", Arg: arg})
	return resp.Say, err
}
func (c *Client) Voice(arg, name string) (*wire.VoiceInfo, string, error) {
	resp, err := c.call(wire.Request{Cmd: "voice", Arg: arg, Content: name})
	return resp.Voice, resp.Say, err
}

// Sleep consolidates once on demand; Forget removes the nodes a phrase names.
func (c *Client) Sleep() error {
	_, err := c.call(wire.Request{Cmd: "sleep"})
	return err
}
func (c *Client) Forget(query string) (string, error) {
	resp, err := c.call(wire.Request{Cmd: "forget", Query: query})
	return resp.Say, err
}

// --- Memory surgery ---

// Edit splices a fragment's content by id, the daemon marking it edited.
func (c *Client) Edit(id uint64, content string) error {
	_, err := c.call(wire.Request{Cmd: "edit", ID: id, Content: content})
	return err
}

// Reweight sets a fragment's synaptic weight by id.
func (c *Client) Reweight(id uint64, weight float64) error {
	_, err := c.call(wire.Request{Cmd: "reweight", ID: id, Weight: weight})
	return err
}

// Delete removes a fragment by id and the edges touching it.
func (c *Client) Delete(id uint64) error {
	_, err := c.call(wire.Request{Cmd: "delete", ID: id})
	return err
}

// Mark records that a fragment was hand-touched, without otherwise changing it.
func (c *Client) Mark(id uint64) error {
	_, err := c.call(wire.Request{Cmd: "mark", ID: id})
	return err
}

// --- Event stream ---

// Event is the toolkit's view of one line of the daemon's event stream, the
// shape the brain emits as it perceives, thinks, speaks, and acts. It mirrors
// the daemon's own event, decoupled so the toolkit needs none of its internals.
type Event struct {
	At      int64   `json:"at"`
	Ev      string  `json:"ev"`
	Channel string  `json:"channel,omitempty"`
	ID      uint64  `json:"id,omitempty"`
	From    uint64  `json:"from,omitempty"`
	To      uint64  `json:"to,omitempty"`
	Rel     string  `json:"rel,omitempty"`
	Bits    float64 `json:"bits,omitempty"`
	Weight  float64 `json:"weight,omitempty"`
	Count   int     `json:"count,omitempty"`
	Cue     string  `json:"cue,omitempty"`
	Action  string  `json:"action,omitempty"`
	Outcome string  `json:"outcome,omitempty"`
	Tpl     string  `json:"tpl,omitempty"`
}

// Subscribe holds a signed connection open and calls onEvent for every event
// the brain emits, so the window shows the inner life live: what it says, what
// it hears, what it thinks and does. It reconnects on a dropped connection with
// a short backoff, and returns when the context is cancelled. onStatus reports
// connect and disconnect so the window can show a live indicator.
func (c *Client) Subscribe(ctx context.Context, onEvent func(Event), onStatus func(connected bool, err error)) {
	for ctx.Err() == nil {
		err := c.streamOnce(ctx, onEvent, onStatus)
		if ctx.Err() != nil {
			return
		}
		if onStatus != nil {
			onStatus(false, err)
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(2 * time.Second):
		}
	}
}

// streamOnce opens one subscription and reads until it drops or the context ends.
func (c *Client) streamOnce(ctx context.Context, onEvent func(Event), onStatus func(bool, error)) error {
	c.mu.RLock()
	net, addr, priv := c.net, c.addr, c.priv
	c.mu.RUnlock()

	req := wire.Request{Cmd: "subscribe"}
	if net == "tcp" {
		if priv == "" {
			return errors.New("no signing key for the event stream")
		}
		if err := wire.SignRequest(priv, &req); err != nil {
			return err
		}
	}
	payload, err := json.Marshal(req)
	if err != nil {
		return err
	}
	nc, err := net2Dial(net, addr)
	if err != nil {
		return err
	}
	defer nc.Close()
	if _, err := nc.Write(append(payload, '\n')); err != nil {
		return err
	}
	if onStatus != nil {
		onStatus(true, nil)
	}
	// Close the connection when the context ends, so the blocked read returns.
	done := make(chan struct{})
	defer close(done)
	go func() {
		select {
		case <-ctx.Done():
			nc.Close()
		case <-done:
		}
	}()
	sc := bufio.NewScanner(nc)
	sc.Buffer(make([]byte, 0, 64*1024), 1<<20)
	for sc.Scan() {
		var ev Event
		if json.Unmarshal(sc.Bytes(), &ev) != nil {
			continue
		}
		onEvent(ev)
	}
	return sc.Err()
}

// net2Dial dials the transport with a short timeout, the same shapes CallAddr uses.
func net2Dial(network, addr string) (net.Conn, error) {
	return net.DialTimeout(network, addr, 3*time.Second)
}

// --- Connection persistence and key loading ---

func (c *Client) connPath() string { return filepath.Join(c.home, "gtk.json") }

// loadConn reads the saved connection, or the default (the container on
// loopback). It returns the setting and where the key should be read from.
func (c *Client) loadConn() (conn, string) {
	def := conn{Net: "tcp", Addr: defaultTCP}
	data, err := os.ReadFile(c.connPath())
	if err != nil {
		return def, ""
	}
	var cn conn
	if json.Unmarshal(data, &cn) != nil || cn.Net == "" {
		return def, ""
	}
	if cn.Addr == "" {
		cn.Addr = defaultTCP
	}
	return cn, cn.KeyFile
}

// saveConn persists the connection so the next launch reconnects to it.
func (c *Client) saveConn(cn conn) {
	if err := os.MkdirAll(c.home, 0o755); err != nil {
		return
	}
	data, err := json.MarshalIndent(cn, "", "  ")
	if err != nil {
		return
	}
	os.WriteFile(c.connPath(), append(data, '\n'), 0o644)
}

// loadKey reads the signing key from a source: a file path, or the environment
// when the source is empty. An empty result means no key, which the network
// path reports clearly rather than sending an unsigned request.
func loadKey(source string) (string, error) {
	source = strings.TrimSpace(source)
	if source == "" {
		if k := strings.TrimSpace(os.Getenv("GOLEM_PRIVATE_KEY")); k != "" {
			return k, nil
		}
		if f := strings.TrimSpace(os.Getenv("GOLEM_PRIVATE_KEY_FILE")); f != "" {
			source = f
		}
	}
	if source == "" {
		return "", nil
	}
	if _, err := os.Stat(source); err == nil {
		b, err := os.ReadFile(source)
		if err != nil {
			return "", err
		}
		return strings.TrimSpace(string(b)), nil
	}
	// Not a file: treat it as the key itself, so a pasted key works too.
	return source, nil
}
