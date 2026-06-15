package daemon

import (
	"encoding/json"
	"net"
	"time"

	"golem/internal/events"
)

// The event trail doubles as an output stream. A transducer outside the
// process subscribes over the transport and receives each event as it
// happens, so a speaker can voice the say events GOLEM emits on its own,
// the idle thought spoken, the self-driven act, not only the reply to a
// request. Feeding a subscriber never blocks the brain loop: a reader that
// falls behind drops events rather than stalling cognition.

func (d *Daemon) addSub(ch chan events.Event) {
	d.subsMu.Lock()
	if d.subs == nil {
		d.subs = map[chan events.Event]struct{}{}
	}
	d.subs[ch] = struct{}{}
	d.subsMu.Unlock()
}

func (d *Daemon) removeSub(ch chan events.Event) {
	d.subsMu.Lock()
	delete(d.subs, ch)
	d.subsMu.Unlock()
}

// fanout offers an event to every subscriber without blocking: a full buffer
// means that reader fell behind, and a dropped event beats a stalled loop.
func (d *Daemon) fanout(ev events.Event) {
	d.subsMu.Lock()
	defer d.subsMu.Unlock()
	for ch := range d.subs {
		select {
		case ch <- ev:
		default:
		}
	}
}

// streamEvents holds a connection open and writes each event to it as a JSON
// line until the client goes away or the daemon stops. The client's only job
// is to keep the connection open; any read error (its close) ends the stream.
func (d *Daemon) streamEvents(conn net.Conn) {
	ch := make(chan events.Event, 256)
	d.addSub(ch)
	defer d.removeSub(ch)

	gone := make(chan struct{})
	go func() {
		buf := make([]byte, 256)
		for {
			if _, err := conn.Read(buf); err != nil {
				close(gone)
				return
			}
		}
	}()

	enc := json.NewEncoder(conn)
	for {
		select {
		case <-d.stop:
			return
		case <-gone:
			return
		case ev := <-ch:
			conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := enc.Encode(ev); err != nil {
				return
			}
		}
	}
}
