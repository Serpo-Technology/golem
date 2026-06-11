package main

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// EventLog is the structured trail of everything the brain does: one JSON
// object per line in <home>/events.jsonl. It is append-only and rotates
// once past a size cap, keeping one older generation, so a chatty week
// cannot quietly eat the disk the way the unchecked WAL once did.
type EventLog struct {
	path       string
	f          *os.File
	appends    int
	maxBytes   int64
	checkEvery int
}

// Event is one line of the trail. Every field is optional except the
// kind; the flat shape keeps the file greppable.
type Event struct {
	At      int64   `json:"at"` // unix milliseconds
	Ev      string  `json:"ev"` // encode, link, replay, prune, merge, sleep, proposal, approval, denial, autonomous
	Channel string  `json:"channel,omitempty"`
	ID      uint64  `json:"id,omitempty"`
	From    uint64  `json:"from,omitempty"`
	To      uint64  `json:"to,omitempty"`
	Rel     string  `json:"rel,omitempty"` // on link events: the edge relation; empty means temporal
	Bits    float64 `json:"bits,omitempty"`
	Weight  float64 `json:"weight,omitempty"`
	Count   int     `json:"count,omitempty"`
	Cue     string  `json:"cue,omitempty"`
	Action  string  `json:"action,omitempty"`
	Outcome string  `json:"outcome,omitempty"`
	Tpl     string  `json:"tpl,omitempty"`
}

func OpenEventLog(home string) (*EventLog, error) {
	path := filepath.Join(home, "events.jsonl")
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return nil, err
	}
	return &EventLog{path: path, f: f, maxBytes: 4 << 20, checkEvery: 256}, nil
}

func (l *EventLog) Close() error {
	if l == nil || l.f == nil {
		return nil
	}
	return l.f.Close()
}

// Append writes one event line. The size check runs every checkEvery
// appends, so rotation costs a stat only occasionally.
func (l *EventLog) Append(ev Event) error {
	if l == nil || l.f == nil {
		return nil
	}
	b, err := json.Marshal(ev)
	if err != nil {
		return err
	}
	if _, err := l.f.Write(append(b, '\n')); err != nil {
		return err
	}
	l.appends++
	if l.appends >= l.checkEvery {
		l.appends = 0
		return l.rotateIfBig()
	}
	return nil
}

func (l *EventLog) rotateIfBig() error {
	st, err := l.f.Stat()
	if err != nil || st.Size() < l.maxBytes {
		return err
	}
	if err := l.f.Close(); err != nil {
		return err
	}
	if err := os.Rename(l.path, l.path+".1"); err != nil {
		return err
	}
	f, err := os.OpenFile(l.path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		l.f = nil
		return err
	}
	l.f = f
	return nil
}
