// Package wire is the boundary between the daemon and anything that
// talks to it: the request and response shapes on the unix socket, the
// display forms of memory, and the client side of one round trip.
package wire

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
	"path/filepath"
	"time"

	"golem/internal/limbic"
	"golem/internal/memory"
)

// The CLI talks to the daemon over a unix socket with one JSON request and
// one JSON response per connection. The daemon is the only process that
// touches the database; every command routes through its single brain loop.

type Request struct {
	Cmd     string `json:"cmd"`
	Channel string `json:"channel,omitempty"`
	Content string `json:"content,omitempty"`
	Query   string `json:"query,omitempty"`
	Command string `json:"command,omitempty"`
	Lang    string `json:"lang,omitempty"` // language tag for tell; empty means English
	Mind    bool   `json:"mind,omitempty"` // teach: ground the word to a thought operation, not a command
}

type Response struct {
	OK        bool            `json:"ok"`
	Error     string          `json:"error,omitempty"`
	Status    *StatusInfo     `json:"status,omitempty"`
	Chem      *limbic.State   `json:"chem,omitempty"`
	Fragments []*FragmentInfo `json:"fragments,omitempty"`
	Links     []*LinkInfo     `json:"links,omitempty"`
	Observe   *ObserveResult  `json:"observe,omitempty"`
	Decision  *DecisionInfo   `json:"decision,omitempty"`
	Action    *ActionResult   `json:"action,omitempty"`
	Habits    []*HabitInfo    `json:"habits,omitempty"`
	Audit     []*AuditInfo    `json:"audit,omitempty"`
	Concepts  []*ConceptInfo  `json:"concepts,omitempty"`
	Say       string          `json:"say,omitempty"` // a sentence GOLEM built in reply
}

// ConceptInfo is the wire form of one concept node: its gloss, its lemma
// category, the words that name it tagged by language, how much
// experience grounds it, and how warm it currently sits in the
// workspace.
type ConceptInfo struct {
	ID       uint64   `json:"id"`
	Gloss    string   `json:"gloss"`
	Category string   `json:"category"`
	Names    []string `json:"names,omitempty"`
	Grounded int      `json:"grounded"`
	Heat     float64  `json:"heat,omitempty"`
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
	Chem           limbic.State            `json:"chem"`
	PromotionBar   int                     `json:"promotion_bar"`
	ExecuteEnabled bool                    `json:"execute_enabled"`
	AllowlistSize  int                     `json:"allowlist_size"`
	Running        []string                `json:"running,omitempty"` // in-flight actions, oldest first
	Queued         int                     `json:"queued,omitempty"`  // actions waiting on claims or a slot
	Warm           []*WarmInfo             `json:"warm,omitempty"`    // workspace concepts, hottest first
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
	ID         uint64       `json:"id"`
	Channel    string       `json:"channel"`
	Content    string       `json:"content"`
	Template   string       `json:"template"`
	Surprise   float64      `json:"surprise"`
	Limbic     limbic.State `json:"limbic"`
	AtUnix     int64        `json:"at_unix"`
	Weight     float64      `json:"weight"`
	Count      int          `json:"count,omitempty"` // above 1 the fragment is a prototype
	Kind       string       `json:"kind,omitempty"`  // word and concept nodes; empty means episodic
	Lang       string       `json:"lang,omitempty"`
	Category   string       `json:"category,omitempty"`
	Activation float64      `json:"activation,omitempty"`
	LinkCount  int          `json:"links,omitempty"`
}

func NewFragmentInfo(f *memory.Fragment) *FragmentInfo {
	info := &FragmentInfo{
		ID: f.ID, Channel: f.Channel, Content: f.Content, Template: f.Template,
		Surprise: f.Surprise, Limbic: f.Limbic, AtUnix: f.Time.Unix(), Weight: f.Weight,
		Lang: f.Lang, Category: f.Category,
	}
	if f.Count > 1 {
		info.Count = f.Count
	}
	if f.Kind != "" && f.Kind != memory.KindEpisodic {
		info.Kind = f.Kind
	}
	return info
}

type ObserveResult struct {
	Bits       float64 `json:"bits"`
	Conscious  bool    `json:"conscious"`
	FragmentID uint64  `json:"fragment_id,omitempty"`
}

// SocketPath is where a home directory keeps its daemon socket.
func SocketPath(home string) string { return filepath.Join(home, "golem.sock") }

// Call performs one request round trip against the daemon's socket.
func Call(home string, req Request) (Response, error) {
	var resp Response
	conn, err := net.DialTimeout("unix", SocketPath(home), 2*time.Second)
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
