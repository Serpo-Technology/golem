package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
	"path/filepath"
	"time"
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
}

type Response struct {
	OK        bool            `json:"ok"`
	Error     string          `json:"error,omitempty"`
	Status    *StatusInfo     `json:"status,omitempty"`
	Chem      *LimbicState    `json:"chem,omitempty"`
	Fragments []*FragmentInfo `json:"fragments,omitempty"`
	Observe   *ObserveResult  `json:"observe,omitempty"`
	Decision  *DecisionInfo   `json:"decision,omitempty"`
	Action    *ActionResult   `json:"action,omitempty"`
	Habits    []*HabitInfo    `json:"habits,omitempty"`
	Audit     []*AuditInfo    `json:"audit,omitempty"`
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
	Channels       map[string]*ChannelStat `json:"channels"`
	Sleeps         int                     `json:"sleeps"`
	LastSleepUnix  int64                   `json:"last_sleep_unix"`
	Chem           LimbicState             `json:"chem"`
	PromotionBar   int                     `json:"promotion_bar"`
	ExecuteEnabled bool                    `json:"execute_enabled"`
	AllowlistSize  int                     `json:"allowlist_size"`
	Running        string                  `json:"running,omitempty"`
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
	ID         uint64      `json:"id"`
	Channel    string      `json:"channel"`
	Content    string      `json:"content"`
	Template   string      `json:"template"`
	Surprise   float64     `json:"surprise"`
	Limbic     LimbicState `json:"limbic"`
	AtUnix     int64       `json:"at_unix"`
	Weight     float64     `json:"weight"`
	Activation float64     `json:"activation,omitempty"`
	LinkCount  int         `json:"links,omitempty"`
}

func fragmentInfo(f *Fragment) *FragmentInfo {
	return &FragmentInfo{
		ID: f.ID, Channel: f.Channel, Content: f.Content, Template: f.Template,
		Surprise: f.Surprise, Limbic: f.Limbic, AtUnix: f.Time.Unix(), Weight: f.Weight,
	}
}

type ObserveResult struct {
	Bits       float64 `json:"bits"`
	Conscious  bool    `json:"conscious"`
	FragmentID uint64  `json:"fragment_id,omitempty"`
}

func socketPath(home string) string { return filepath.Join(home, "golem.sock") }

// ipcCall performs one request round trip against the daemon's socket.
func ipcCall(home string, req Request) (Response, error) {
	var resp Response
	conn, err := net.DialTimeout("unix", socketPath(home), 2*time.Second)
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
