// Command gtk is the console half of the Golem Toolkit. It talks to the running
// brain over the same connection the window does and renders what the graph
// holds, so reading and shaping memory stays distinct from conversing with
// GOLEM. It shares its client with the window; the daemon stays the only
// process that writes the database.
package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"golem/wire"
	"gtk/client"
)

const usage = `gtk is a console for GOLEM's memory, talking to the running daemon.

Keys, the architect's own (the brain never mints one):
  gtk keygen              make an architect keypair: register the public key
                          with the brain, keep the private key to sign with

Read:
  gtk status              the daemon's vital signs and workspace
  gtk search <words>      the moments a cue brings to mind
  gtk concepts            the concepts memory has distilled
  gtk cognition           resemblance, beliefs: is the brain rich enough to reason
  gtk audit [n]           the last n entries of the trail (default 20)
  gtk dump                every fragment the graph holds

Author, grounded through teaching:
  gtk observe <channel> <text>   file a grounded moment
  gtk teach <cue> <command>      ground a cue to a command

Surgery, raw and audited:
  gtk edit <id> <text>           splice a fragment's content
  gtk reweight <id> <weight>     set a fragment's weight (0 to 1)
  gtk delete <id>                remove a fragment by id
  gtk mark <id>                  mark a fragment hand-edited
`

func main() {
	args := os.Args[1:]
	if len(args) == 0 {
		fmt.Print(usage)
		return
	}
	// Key generation needs no brain: it is the architect minting their own
	// keypair, done here rather than in GOLEM, which only ever holds the
	// public half.
	if args[0] == "keygen" {
		runKeygen()
		return
	}
	c, err := client.New()
	if err != nil {
		fail(err)
	}

	switch args[0] {
	case "status":
		st, err := c.Status()
		check(c, err)
		printStatus(st)

	case "search", "recall":
		if len(args) < 2 {
			fail(fmt.Errorf("usage: gtk search <words>"))
		}
		frags, err := c.Search(strings.Join(args[1:], " "))
		check(c, err)
		if len(frags) == 0 {
			fmt.Println("nothing comes to mind.")
			return
		}
		printFragments(frags)

	case "concepts":
		cs, err := c.Concepts()
		check(c, err)
		printConcepts(cs)

	case "cognition", "mind":
		info, err := c.Cognition()
		check(c, err)
		printCognition(info)

	case "audit":
		n := 20
		if len(args) >= 2 {
			if v, err := strconv.Atoi(args[1]); err == nil && v > 0 {
				n = v
			}
		}
		entries, err := c.Audit()
		check(c, err)
		printAudit(recent(entries, n))

	case "dump":
		frags, _, err := c.Dump()
		check(c, err)
		if len(frags) == 0 {
			fmt.Println("memory is empty.")
			return
		}
		printFragments(frags)

	case "observe":
		if len(args) < 3 {
			fail(fmt.Errorf("usage: gtk observe <channel> <text>"))
		}
		_, err := c.Observe(args[1], strings.Join(args[2:], " "), nil, "", false)
		check(c, err)
		fmt.Println("observed.")

	case "teach":
		if len(args) < 3 {
			fail(fmt.Errorf("usage: gtk teach <cue> <command>"))
		}
		check(c, c.Teach(args[1], strings.Join(args[2:], " "), "", false, false, false))
		fmt.Printf("grounded %q -> %q\n", args[1], strings.Join(args[2:], " "))

	case "edit":
		if len(args) < 3 {
			fail(fmt.Errorf("usage: gtk edit <id> <text>"))
		}
		check(c, c.Edit(mustID(args[1]), strings.Join(args[2:], " ")))
		fmt.Printf("edited #%s\n", args[1])

	case "reweight":
		if len(args) < 3 {
			fail(fmt.Errorf("usage: gtk reweight <id> <weight>"))
		}
		w, err := strconv.ParseFloat(args[2], 64)
		if err != nil {
			fail(fmt.Errorf("a weight is a number between 0 and 1"))
		}
		check(c, c.Reweight(mustID(args[1]), w))
		fmt.Printf("reweighted #%s to %.2f\n", args[1], w)

	case "delete":
		if len(args) < 2 {
			fail(fmt.Errorf("usage: gtk delete <id>"))
		}
		check(c, c.Delete(mustID(args[1])))
		fmt.Printf("deleted #%s\n", args[1])

	case "mark":
		if len(args) < 2 {
			fail(fmt.Errorf("usage: gtk mark <id>"))
		}
		check(c, c.Mark(mustID(args[1])))
		fmt.Printf("marked #%s edited\n", args[1])

	default:
		fmt.Print(usage)
		os.Exit(2)
	}
}

// mustID reads a fragment id from the command line, failing cleanly when it is
// not a number, so surgery never lands on a misread target.
func mustID(s string) uint64 {
	id, err := strconv.ParseUint(s, 10, 64)
	if err != nil {
		fail(fmt.Errorf("a fragment id is a number, not %q", s))
	}
	return id
}

// check exits with a friendly message when the daemon is not listening, the same
// contract the terminal client keeps.
func check(c *client.Client, err error) {
	if err == nil {
		return
	}
	fail(fmt.Errorf("%v (connected to %s)", err, c.Info().Addr))
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "gtk:", err)
	os.Exit(1)
}

// runKeygen mints an architect keypair and prints both halves: the public key
// to register with a brain (its config's architect_pubkey, or
// GOLEM_ARCHITECT_PUBKEY), the private key to keep and sign with
// (GOLEM_PRIVATE_KEY, or a file for GOLEM_PRIVATE_KEY_FILE). GOLEM never sees
// the private half; the architect mints it here.
func runKeygen() {
	pub, priv, err := client.GenerateKeypair()
	if err != nil {
		fail(err)
	}
	fmt.Println("# architect keypair. keep the private key secret; the brain holds only the public key.")
	fmt.Printf("architect_pubkey (config / GOLEM_ARCHITECT_PUBKEY):\n%s\n\n", pub)
	fmt.Printf("private key (GOLEM_PRIVATE_KEY, or a file for GOLEM_PRIVATE_KEY_FILE):\n%s\n", priv)
}

// recent keeps the n most recent entries of the trail. The daemon returns it
// newest first, so the most recent are the head.
func recent(entries []*wire.AuditInfo, n int) []*wire.AuditInfo {
	if n <= 0 || len(entries) <= n {
		return entries
	}
	return entries[:n]
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
		if f.Edited {
			line += "  *edited"
		}
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

func printCognition(info *wire.CognitionInfo) {
	if info == nil {
		fmt.Println("no cognition.")
		return
	}
	fmt.Printf("%d concepts, %d features\n", info.Concepts, info.Features)
	fmt.Printf("resemblance: %d pair(s) cross the floor %.2f, where a guess can fire\n",
		info.AboveFloor, info.Floor)
	for _, p := range info.Resemblance {
		mark := "  "
		if p.Weight >= info.Floor {
			mark = "* " // a pair the brain can reason across
		}
		fmt.Printf("  %s%-16s ~ %-16s %.2f\n", mark, p.A, p.B, p.Weight)
	}
	if len(info.Resemblance) == 0 {
		fmt.Println("  no concept shares a feature with another yet.")
	}
	if len(info.Universal) > 0 {
		fmt.Printf("universal features, named and passed over by resemblance: %s\n",
			strings.Join(info.Universal, ", "))
	}
	if len(info.Held) > 0 {
		fmt.Printf("believes now (working memory), %d:\n", len(info.Held))
		for _, b := range info.Held {
			printBelief(b)
		}
	}
	if len(info.Promoted) > 0 {
		fmt.Printf("kept (long-term), %d:\n", len(info.Promoted))
		for _, b := range info.Promoted {
			printBelief(b)
		}
	}
	if len(info.Held) == 0 && len(info.Promoted) == 0 {
		fmt.Println("no beliefs held; they form as resemblance crosses the floor.")
	}
}

func printBelief(b *wire.BeliefInfo) {
	tags := ""
	if b.Status == "hypothetical" {
		tags += " supposed"
	}
	if b.Kept {
		tags += " kept"
	}
	if b.Depth > 0 {
		tags += fmt.Sprintf(" depth %d", b.Depth)
	}
	fmt.Printf("  %s %s %s  (%.2f)%s\n", b.Agent, b.Rel, b.Patient, b.Conf, tags)
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
