package main

import (
	"context"

	"gme/client"
	"golem/wire"
)

// App is the bridge the window calls into: each method is one read against the
// running daemon through the shared client, so the frontend reads the live graph
// over the socket and never touches the database itself.
type App struct {
	ctx context.Context
	c   *client.Client
	err error
}

// NewApp builds the bridge, resolving the daemon's home once.
func NewApp() *App {
	c, err := client.New()
	return &App{c: c, err: err}
}

func (a *App) startup(ctx context.Context) { a.ctx = ctx }

// Status reads the daemon's vital signs and workspace.
func (a *App) Status() (*wire.StatusInfo, error) {
	if a.err != nil {
		return nil, a.err
	}
	return a.c.Status()
}

// Search recalls the moments a cue brings to mind.
func (a *App) Search(query string) ([]*wire.FragmentInfo, error) {
	if a.err != nil {
		return nil, a.err
	}
	return a.c.Search(query)
}

// Concepts lists the concepts memory has distilled.
func (a *App) Concepts() ([]*wire.ConceptInfo, error) {
	if a.err != nil {
		return nil, a.err
	}
	return a.c.Concepts()
}

// Audit reads the decision and edit trail.
func (a *App) Audit() ([]*wire.AuditInfo, error) {
	if a.err != nil {
		return nil, a.err
	}
	return a.c.Audit()
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

// Observe files a grounded fragment through the daemon's perception pipeline, so
// what is authored in the window is grounded exactly as a lived moment.
func (a *App) Observe(channel, content string) error {
	if a.err != nil {
		return a.err
	}
	return a.c.Observe(channel, content)
}

// Teach grounds a cue to a command through the daemon's teaching path.
func (a *App) Teach(cue, command string) error {
	if a.err != nil {
		return a.err
	}
	return a.c.Teach(cue, command)
}

// Edit splices a fragment's content by id, the daemon marking it edited.
func (a *App) Edit(id uint64, content string) error {
	if a.err != nil {
		return a.err
	}
	return a.c.Edit(id, content)
}

// Reweight sets a fragment's synaptic weight by id.
func (a *App) Reweight(id uint64, weight float64) error {
	if a.err != nil {
		return a.err
	}
	return a.c.Reweight(id, weight)
}

// Delete removes a fragment by id and the edges touching it.
func (a *App) Delete(id uint64) error {
	if a.err != nil {
		return a.err
	}
	return a.c.Delete(id)
}

// Mark records that a fragment was hand-touched, without otherwise changing it.
func (a *App) Mark(id uint64) error {
	if a.err != nil {
		return a.err
	}
	return a.c.Mark(id)
}
