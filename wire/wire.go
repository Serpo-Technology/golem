// Package wire is the boundary between the daemon and anything that
// talks to it: the request and response shapes on the unix socket, the
// display forms of memory, and the client side of one round trip.
package wire

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"time"
)

// Chem is the wire form of the limbic chemistry: the four levels a client reads,
// mirroring the daemon's internal state so the wire types carry no dependency on
// it and a separate program can import them. The field names match the internal
// shape, so the JSON is the same on both sides.
type Chem struct {
	Dopamine      float64
	Noradrenaline float64
	Serotonin     float64
	Oxytocin      float64
}

// The CLI talks to the daemon over a unix socket with one JSON request and
// one JSON response per connection. The daemon is the only process that
// touches the database; every command routes through its single brain loop.

type Request struct {
	Cmd     string `json:"cmd"`
	Channel string `json:"channel,omitempty"`
	Content string `json:"content,omitempty"`
	Query   string `json:"query,omitempty"`
	Command string `json:"command,omitempty"`
	Lang    string `json:"lang,omitempty"`  // language tag for tell; empty means English
	Mind    bool   `json:"mind,omitempty"`  // teach: ground the word to a thought operation, not a command
	Say     bool   `json:"say,omitempty"`   // teach: add a construction (Content is the intent, Command the template)
	Force   bool   `json:"force,omitempty"` // observe: a deliberate teaching percept, encoded past the attention gate (ambient senses leave this off)
	Arg     string `json:"arg,omitempty"`   // voice subcommand: status, on, off, use (the name rides in Content)
	// The editor's raw surgery names a node by id and carries the new value:
	// Content for an edit, Weight for a reweight.
	ID     uint64  `json:"id,omitempty"`
	Weight float64 `json:"weight,omitempty"`
	// Media carries the raw bytes a perceptual observe was captioned from (an
	// image, an audio clip), base64 over the wire. The daemon keeps them in the
	// content-addressed media store and references them from the moment; the
	// brain still reasons only over Content, the caption. Mime is the type hint.
	Media []byte `json:"media,omitempty"`
	Mime  string `json:"mime,omitempty"`
}

type Response struct {
	OK        bool            `json:"ok"`
	Error     string          `json:"error,omitempty"`
	Status    *StatusInfo     `json:"status,omitempty"`
	Chem      *Chem           `json:"chem,omitempty"`
	Fragments []*FragmentInfo `json:"fragments,omitempty"`
	Links     []*LinkInfo     `json:"links,omitempty"`
	Observe   *ObserveResult  `json:"observe,omitempty"`
	Decision  *DecisionInfo   `json:"decision,omitempty"`
	Action    *ActionResult   `json:"action,omitempty"`
	Habits    []*HabitInfo    `json:"habits,omitempty"`
	Audit     []*AuditInfo    `json:"audit,omitempty"`
	Concepts  []*ConceptInfo  `json:"concepts,omitempty"`
	Cognition *CognitionInfo  `json:"cognition,omitempty"`
	Say       string          `json:"say,omitempty"` // a sentence GOLEM built in reply
	Voice     *VoiceInfo      `json:"voice,omitempty"`
}

// VoiceInfo is the wire form of the voice state: whether spoken replies are
// emitted as say events for an external speaker, and the voice an adapter
// should prefer (empty leaves the choice to the adapter).
type VoiceInfo struct {
	Enabled bool   `json:"enabled"`
	Name    string `json:"name,omitempty"`
}

// ConceptInfo is the wire form of one concept node: its gloss, its lemma
// category, the words that name it tagged by language, how much
// experience grounds it, how warm it currently sits in the workspace,
// and the sparse feature signature its experience earned.
type ConceptInfo struct {
	ID       uint64   `json:"id"`
	Gloss    string   `json:"gloss"`
	Category string   `json:"category"`
	Names    []string `json:"names,omitempty"`
	Grounded int      `json:"grounded"`
	Heat     float64  `json:"heat,omitempty"`
	Features []string `json:"features,omitempty"`
}

// CognitionInfo is the wire form of GOLEM's reasoning state: how richly its
// concepts resemble one another, the gate every inference rides, and what it
// currently believes and has come to keep, the inferred layer that lives beside
// the observed graph. It is the view for watching a brain grow rich enough to
// reason: when resemblance crosses the floor, the guesses begin.
type CognitionInfo struct {
	Concepts    int                `json:"concepts"`
	Features    int                `json:"features"`
	Floor       float64            `json:"floor"`       // the resemblance an inference must cross to fire
	AboveFloor  int                `json:"above_floor"` // concept pairs whose resemblance crosses it
	Resemblance []*ResemblancePair `json:"resemblance,omitempty"`
	Universal   []string           `json:"universal,omitempty"` // features carried by ~all concepts, named and passed over
	Held        []*BeliefInfo      `json:"held,omitempty"`      // working-memory beliefs and suppositions
	Promoted    []*BeliefInfo      `json:"promoted,omitempty"`  // long-term beliefs kept across the night
}

// ResemblancePair is two concepts and how strongly they resemble each other by
// shared features, the stronger of the two directions.
type ResemblancePair struct {
	A      string  `json:"a"`
	B      string  `json:"b"`
	Weight float64 `json:"weight"`
}

// BeliefInfo is the wire form of one inferred proposition GOLEM holds: who it is
// about, how firmly, how far from observation, and how it is held.
type BeliefInfo struct {
	Agent   string  `json:"agent"`
	Rel     string  `json:"rel"`
	Patient string  `json:"patient"`
	Conf    float64 `json:"conf"`
	Depth   int     `json:"depth"`
	Status  string  `json:"status"`           // inferred or hypothetical
	Voiced  bool    `json:"voiced,omitempty"` // GOLEM has spoken it
	Kept    bool    `json:"kept,omitempty"`   // promoted to long-term memory
}

// LinkInfo is the wire form of one association edge, set on dump so the
// graph can be exported whole.
type LinkInfo struct {
	From   uint64  `json:"from"`
	To     uint64  `json:"to"`
	Rel    string  `json:"rel,omitempty"`
	Weight float64 `json:"weight"`
}

// DecisionInfo is the wire form of a proposal: a winning command, or a
// conflict to resolve, or nothing grounded.
type DecisionInfo struct {
	Cue          string   `json:"cue"`
	Grounded     bool     `json:"grounded"`
	Proposed     string   `json:"proposed,omitempty"`
	Saliency     float64  `json:"saliency,omitempty"`
	Status       string   `json:"status,omitempty"`
	Denylisted   bool     `json:"denylisted,omitempty"`
	Conflict     bool     `json:"conflict,omitempty"`
	Question     string   `json:"question,omitempty"`
	Alternatives []string `json:"alternatives,omitempty"`
	Autonomous   bool     `json:"autonomous,omitempty"`
	Outcome      string   `json:"outcome,omitempty"`
	Demoted      bool     `json:"demoted,omitempty"`
}

// ActionResult is the wire form of an approve or deny outcome.
type ActionResult struct {
	Command   string `json:"command"`
	Outcome   string `json:"outcome"`
	Status    string `json:"status"`
	Approvals int    `json:"approvals"`
	Denials   int    `json:"denials"`
}

type HabitInfo struct {
	Cue       string `json:"cue"`
	Action    string `json:"action"`
	Status    string `json:"status"`
	Approvals int    `json:"approvals"`
	Denials   int    `json:"denials"`
}

type AuditInfo struct {
	AtUnix  int64  `json:"at_unix"`
	Cue     string `json:"cue"`
	Action  string `json:"action"`
	Outcome string `json:"outcome"`
}

type StatusInfo struct {
	PID            int                     `json:"pid"`
	UptimeSec      float64                 `json:"uptime_sec"`
	Fragments      int                     `json:"fragments"`
	HotFragments   int                     `json:"hot_fragments"`
	Concepts       int                     `json:"concepts"`
	Words          int                     `json:"words"`
	Channels       map[string]*ChannelStat `json:"channels"`
	Sleeps         int                     `json:"sleeps"`
	LastSleepUnix  int64                   `json:"last_sleep_unix"`
	Chem           Chem                    `json:"chem"`
	PromotionBar   int                     `json:"promotion_bar"`
	ExecuteEnabled bool                    `json:"execute_enabled"`
	AllowlistSize  int                     `json:"allowlist_size"`
	VoiceEnabled   bool                    `json:"voice_enabled"`
	VoiceName      string                  `json:"voice_name,omitempty"`
	Running        []string                `json:"running,omitempty"` // in-flight actions, oldest first
	Queued         int                     `json:"queued,omitempty"`  // actions waiting on claims or a slot
	Warm           []*WarmInfo             `json:"warm,omitempty"`    // workspace concepts, hottest first
	Goal           string                  `json:"goal,omitempty"`    // the held intention, when one is open
}

// WarmInfo is one workspace entry: a concept gloss and its activation.
type WarmInfo struct {
	Gloss string  `json:"gloss"`
	Heat  float64 `json:"heat"`
}

type ChannelStat struct {
	Observed   int   `json:"observed"`
	Conscious  int   `json:"conscious"`
	Suppressed int   `json:"suppressed"`
	Dropped    int64 `json:"dropped"`
}

// FragmentInfo is the wire form of a Fragment. Activation is set on recall
// results: 1.0 marks a direct match, lower values mark moments reached
// through the link graph. LinkCount is the fragment's degree, set on dump.
type FragmentInfo struct {
	ID         uint64  `json:"id"`
	Channel    string  `json:"channel"`
	Content    string  `json:"content"`
	Template   string  `json:"template"`
	Surprise   float64 `json:"surprise"`
	Limbic     Chem    `json:"limbic"`
	AtUnix     int64   `json:"at_unix"`
	Weight     float64 `json:"weight"`
	Count      int     `json:"count,omitempty"` // above 1 the fragment is a prototype
	Kind       string  `json:"kind,omitempty"`  // word and concept nodes; empty means episodic
	Lang       string  `json:"lang,omitempty"`
	Category   string  `json:"category,omitempty"`
	Activation float64 `json:"activation,omitempty"`
	LinkCount  int     `json:"links,omitempty"`
	Edited     bool    `json:"edited,omitempty"` // hand-touched through the editor
}

type ObserveResult struct {
	Bits       float64 `json:"bits"`
	Conscious  bool    `json:"conscious"`
	FragmentID uint64  `json:"fragment_id,omitempty"`
}

// Home resolves the daemon's state directory the same way the daemon does:
// GOLEM_HOME when set, otherwise ~/.golem. A client uses it to find the socket
// without depending on the daemon's configuration package.
func Home() (string, error) {
	if h := os.Getenv("GOLEM_HOME"); h != "" {
		return h, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".golem"), nil
}

// SocketPath is where a home directory keeps its daemon socket.
func SocketPath(home string) string { return filepath.Join(home, "golem.sock") }

// Call performs one request round trip against the daemon's socket.
func Call(home string, req Request) (Response, error) {
	return CallAddr("unix", SocketPath(home), req)
}

// CallAddr performs one request round trip against any transport the daemon
// serves, the unix socket or a TCP address, so a process outside the daemon
// (a sense bridge, a speaker) speaks the same protocol over the network. The
// wire is one JSON request and one JSON reply, newline-delimited.
func CallAddr(network, addr string, req Request) (Response, error) {
	var resp Response
	conn, err := net.DialTimeout(network, addr, 2*time.Second)
	if err != nil {
		return resp, err
	}
	defer conn.Close()
	// Generous, because an approve or propose may run a command for as
	// long as the daemon's execution timeout allows.
	conn.SetDeadline(time.Now().Add(120 * time.Second))

	payload, err := json.Marshal(req)
	if err != nil {
		return resp, err
	}
	if _, err := conn.Write(append(payload, '\n')); err != nil {
		return resp, err
	}
	line, err := bufio.NewReader(conn).ReadBytes('\n')
	if err != nil {
		return resp, err
	}
	if err := json.Unmarshal(line, &resp); err != nil {
		return resp, err
	}
	if !resp.OK && resp.Error != "" {
		return resp, fmt.Errorf("%s", resp.Error)
	}
	return resp, nil
}

// Clip keeps display lines readable and log files honest; stored
// content is never affected.
func Clip(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "..."
}
