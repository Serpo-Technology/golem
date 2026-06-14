// Command gme is a separate console for GOLEM's memory. It talks to the running
// daemon over the same unix socket the terminal does and renders what the graph
// holds, so reading and shaping memory stays distinct from conversing with
// GOLEM. The daemon remains the only process that touches the database; gme is a
// client to its single brain loop.
package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"golem/internal/config"
	"golem/internal/wire"
)

const usage = `gme is a console for GOLEM's memory, talking to the running daemon.

Usage:
  gme status              the daemon's vital signs and workspace
  gme search <words>      the moments a cue brings to mind
  gme concepts            the concepts memory has distilled
  gme audit [n]           the last n entries of the trail (default 20)
  gme dump                every fragment the graph holds
`

func main() {
	args := os.Args[1:]
	if len(args) == 0 {
		fmt.Print(usage)
		return
	}
	home, err := config.Home()
	if err != nil {
		fail(err)
	}

	switch args[0] {
	case "status":
		resp := call(home, wire.Request{Cmd: "status"})
		printStatus(resp.Status)

	case "search", "recall":
		if len(args) < 2 {
			fail(fmt.Errorf("usage: gme search <words>"))
		}
		resp := call(home, wire.Request{Cmd: "recall", Query: strings.Join(args[1:], " ")})
		if len(resp.Fragments) == 0 {
			fmt.Println("nothing comes to mind.")
			return
		}
		printFragments(resp.Fragments)

	case "concepts":
		resp := call(home, wire.Request{Cmd: "concepts"})
		printConcepts(resp.Concepts)

	case "audit":
		n := 20
		if len(args) >= 2 {
			if v, err := strconv.Atoi(args[1]); err == nil && v > 0 {
				n = v
			}
		}
		resp := call(home, wire.Request{Cmd: "audit"})
		printAudit(lastN(resp.Audit, n))

	case "dump":
		resp := call(home, wire.Request{Cmd: "dump"})
		if len(resp.Fragments) == 0 {
			fmt.Println("memory is empty.")
			return
		}
		printFragments(resp.Fragments)

	default:
		fmt.Print(usage)
		os.Exit(2)
	}
}

// call performs one round trip and exits with a friendly message when the
// daemon is not listening, the same contract the terminal client keeps.
func call(home string, req wire.Request) wire.Response {
	resp, err := wire.Call(home, req)
	if err != nil {
		if _, statErr := os.Stat(wire.SocketPath(home)); os.IsNotExist(statErr) {
			fail(fmt.Errorf("the daemon is not running; start it with: golem run"))
		}
		fail(err)
	}
	return resp
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "gme:", err)
	os.Exit(1)
}

// lastN keeps the most recent n entries of a chronological trail, the tail being
// the latest, so a long history renders to a readable window.
func lastN(entries []*wire.AuditInfo, n int) []*wire.AuditInfo {
	if n <= 0 || len(entries) <= n {
		return entries
	}
	return entries[len(entries)-n:]
}

func printStatus(st *wire.StatusInfo) {
	if st == nil {
		fmt.Println("no status.")
		return
	}
	fmt.Printf("golem daemon, pid %d, up %s\n", st.PID,
		time.Duration(st.UptimeSec*float64(time.Second)).Round(time.Second))
	fmt.Printf("  fragments %d (%d hot), concepts %d, words %d, nights %d\n",
		st.Fragments, st.HotFragments, st.Concepts, st.Words, st.Sleeps)
	if len(st.Warm) > 0 {
		parts := make([]string, 0, len(st.Warm))
		for _, w := range st.Warm {
			parts = append(parts, fmt.Sprintf("%q %.2f", w.Gloss, w.Heat))
		}
		fmt.Printf("  workspace: %s\n", strings.Join(parts, ", "))
	}
	if st.Goal != "" {
		fmt.Printf("  goal: %s\n", st.Goal)
	}
}

func printFragments(frags []*wire.FragmentInfo) {
	for _, f := range frags {
		kind := f.Kind
		if kind == "" {
			kind = "episodic"
		}
		line := fmt.Sprintf("#%-5d [%s/%s] %s", f.ID, kind, f.Channel, wire.Clip(f.Content, 80))
		if f.Activation > 0 && f.Activation < 1 {
			line += fmt.Sprintf("  (%.2f)", f.Activation)
		}
		fmt.Println(line)
	}
}

func printConcepts(cs []*wire.ConceptInfo) {
	if len(cs) == 0 {
		fmt.Println("no concepts born yet; they distil from recurring shapes during sleep.")
		return
	}
	for _, c := range cs {
		names := ""
		if len(c.Names) > 0 {
			names = " {" + strings.Join(c.Names, ", ") + "}"
		}
		fmt.Printf("#%-5d %s%s  grounded %d\n", c.ID, c.Gloss, names, c.Grounded)
	}
}

func printAudit(entries []*wire.AuditInfo) {
	if len(entries) == 0 {
		fmt.Println("the trail is empty.")
		return
	}
	for _, e := range entries {
		fmt.Printf("  %s  %q -> %q  %s\n",
			time.Unix(e.AtUnix, 0).Format("Jan 02 15:04:05"), e.Cue, e.Action, e.Outcome)
	}
}
