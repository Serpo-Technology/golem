// Package client is GME's connection to the running GOLEM daemon: a thin wrapper
// over the wire protocol that both the console and the GUI call, so the two
// share one path to the brain and read what it returns the same way. The daemon
// stays the only process that touches the database; this is a client to its
// single brain loop.
package client

import "golem/wire"

// Client talks to a daemon's socket under a home directory.
type Client struct{ Home string }

// New finds the daemon the same way the terminal does, honoring GOLEM_HOME.
func New() (*Client, error) {
	home, err := wire.Home()
	if err != nil {
		return nil, err
	}
	return &Client{Home: home}, nil
}

func (c *Client) call(req wire.Request) (wire.Response, error) {
	return wire.Call(c.Home, req)
}

// Status reads the daemon's vital signs and workspace.
func (c *Client) Status() (*wire.StatusInfo, error) {
	resp, err := c.call(wire.Request{Cmd: "status"})
	return resp.Status, err
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

// Dump reads the whole graph: every fragment and the links among them.
func (c *Client) Dump() ([]*wire.FragmentInfo, []*wire.LinkInfo, error) {
	resp, err := c.call(wire.Request{Cmd: "dump"})
	return resp.Fragments, resp.Links, err
}
