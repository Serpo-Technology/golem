// The golem command: the daemon in the foreground, a CLI over its
// socket, and the demos that exercise the whole brain on throwaway
// memory.
package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"mime"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"golem/internal/config"
	"golem/internal/daemon"
	"golem/internal/events"
	"golem/internal/memory"
	"golem/wire"
)

const usageText = `GOLEM is a daemon that watches event channels, remembers surprises, and
forgets what stops mattering.

Usage:
  golem run                       start the daemon (foreground)
  golem demo                      run the synthetic one-channel demo and exit
  golem demo decide               walk the decision loop on throwaway memory
  golem demo execute              real execution under its gates, throwaway
  golem demo consolidate          one day, one night: merge and replay
  golem demo language             concepts, words, and built sentences
  golem demo pipeline             shell-free pipelines and parallel actions
  golem demo think                quiet mind-wandering and a kept insight
  golem status                    show daemon status
  golem chem                      show current neurochemistry
  golem chem --watch              print it once a second until interrupted
  golem observe <channel> <text>  feed one observation to the daemon
  golem observe --force ...        encode it past the attention gate (teaching)
  golem tell <words>              speak to GOLEM; the words become a memory
                                  bound to the moments around them, a
                                  question is answered from what is
                                  grounded, and a command becomes a proposal.
                                  Plain language also drives the faculties:
                                  "recall the disk" retrieves, "remember
                                  this as a bell" names what is present,
                                  "tidy" consolidates, "what were you
                                  thinking" introspects, and "it", "that",
                                  and "the other one" follow the
                                  conversation's warm topic across turns
  golem tell                      the same, as an interactive session
  golem tell --lang <tag> <words> tag the words' language ("tr", "de");
                                  words in any language converge on the
                                  same concepts once grounded alike
  golem tell --about <query> <words>
                                  bind the words to remembered moments that
                                  match the query instead of to the present
  golem tell --source <name> <words>
                                  speak as a named peer (an LLM watching the
                                  channel); heard at that source's trust, not
                                  the operator's
  golem trust <source> [level]    show or set how far GOLEM trusts a source
                                  (setting needs the master key)
  golem watch                     stream the talking channel: what GOLEM says
                                  and what each source says to it
  golem recall [query]            recall memory plus its linked context;
                                  without a query, returns the most
                                  stressful moment
  golem compute <program>         run a small Go program or expression and
                                  ground its result; the runtime is closed
                                  until runtime_enabled is set in config.json
  golem concepts                  list what GOLEM has abstracted: each
                                  concept, its words, and its grounding
  golem health                    audit the naming graph for consolidation
                                  pollution: concepts split across nodes and
                                  words that name more than one concept;
                                  exits non-zero when any is found
  golem teach <cue> <command>     ground a cue to a command by hand; repeat
                                  to strengthen toward graduation
  golem teach --mind <word> <relation> <signal>
                                  name a kind of thinking: the word comes to
                                  mean the walk(relation, signal) thought
                                  operation, and saying it leans idle thought
                                  that way (relations: temporal, successor,
                                  grounds, names, feature; signals: weight,
                                  stress, saliency, recency)
  golem teach --mind <word> act <name>
                                  name a faculty instead: the word comes to
                                  mean one of GOLEM's own acts (recall,
                                  ground, consolidate, speak, propose,
                                  introspect), the same acts the quiet loop
                                  fires on its own when its state calls
  golem teach --say <intent> <template>
                                  add a way to say a thing: a construction
                                  with {slot} markers the engine fills, e.g.
                                  introspect "my mind was on {thought}"; it
                                  outweighs the seed until forgotten
  golem teach --inflect <lang> <feature> <base:inflected>...
                                  induce an inflection rule from examples and
                                  keep it as a fluid weight, e.g. tr past
                                  kovala:kovaladı gel:geldi oku:okudu
  golem propose <cue>             weigh the actions grounded to a cue and
                                  propose one, or ask when it is ambiguous
  golem approve                   approve the pending proposal
  golem deny                      reject the pending proposal
  golem habits                    list grounded cue-and-command pairs
  golem audit                     show the recent decision trail
  golem voice [on|off|status]     control whether replies are spoken
  golem voice use [NAME]          pick the speaking voice; omit NAME for
                                  the system default (say -v ? lists them)
  golem dump                      list every fragment held in memory
  golem dump --dot                export the memory graph as graphviz dot
  golem forget <what>             truly delete a remembered shape and its
                                  edges, the concept and episodes alike
  golem sleep                     consolidate once now and keep running
  golem stop                      ask the daemon to sleep once and shut down
  golem shellhook                 print the zsh hook that feeds the shell channel
  golem launchd                   print a launchd plist for start-at-login

The daemon keeps its memory, config, and socket in ~/.golem (override with
GOLEM_HOME). Edit ~/.golem/config.json to tune thresholds and sleep timing.

Senses are selected in config.json under "channels": vitals and syslog
ship on; "watch_dirs" adds file events, "tail_files" reads appended lines
as the reading channel, "watch_urls" fetches pages onto the untrusted web
channel, "screen" reports the frontmost app. Transduction stays outside
the binary by design: pipe a captioner into  golem observe vision
"<caption>"  and a speech recognizer into  golem tell  for eyes and ears,
and "voice" emits each reply as a say event for an external speaker to
render rather than speaking it itself. Set "listen_tcp" to serve the wire
protocol over TCP as well, so those transducers reach the brain across a
container boundary; adapters/ holds a reference set.`

func main() {
	args := os.Args[1:]
	if len(args) == 0 {
		fmt.Println(usageText)
		return
	}

	home, err := config.Home()
	if err != nil {
		fail(err)
	}

	switch args[0] {
	case "demo":
		switch {
		case len(args) >= 2 && args[1] == "decide":
			daemon.DemoDecide()
		case len(args) >= 2 && args[1] == "execute":
			daemon.DemoExecute()
		case len(args) >= 2 && args[1] == "consolidate":
			daemon.DemoConsolidate()
		case len(args) >= 2 && args[1] == "language":
			daemon.DemoLanguage()
		case len(args) >= 2 && args[1] == "pipeline":
			daemon.DemoPipeline()
		case len(args) >= 2 && args[1] == "think":
			daemon.DemoThink()
		default:
			runDemo()
		}

	case "run":
		cfg, err := config.Load(home)
		if err != nil {
			fail(err)
		}
		d, err := daemon.New(home, cfg, os.Stdout)
		if err != nil {
			fail(err)
		}
		if err := d.Run(); err != nil {
			fail(err)
		}

	case "status":
		resp := call(home, wire.Request{Cmd: "status"})
		printStatus(resp.Status)

	case "chem":
		watch := len(args) >= 2 && args[1] == "--watch"
		for {
			resp := call(home, wire.Request{Cmd: "chem"})
			c := resp.Chem
			if !watch {
				fmt.Printf("dopamine %.2f   noradrenaline %.2f   serotonin %.2f   oxytocin %.2f   adenosine %.2f\n",
					c.Dopamine, c.Noradrenaline, c.Serotonin, c.Oxytocin, c.Adenosine)
				break
			}
			fmt.Printf("%s  dopamine %.2f   noradrenaline %.2f   serotonin %.2f   oxytocin %.2f   adenosine %.2f\n",
				time.Now().Format("15:04:05"), c.Dopamine, c.Noradrenaline, c.Serotonin, c.Oxytocin, c.Adenosine)
			time.Sleep(time.Second)
		}

	case "observe":
		rest := args[1:]
		mediaPath := ""
		force := false
		for len(rest) >= 1 {
			switch {
			case rest[0] == "--media" && len(rest) >= 2:
				mediaPath = rest[1]
				rest = rest[2:]
			case rest[0] == "--force":
				force = true
				rest = rest[1:]
			default:
				goto optsDone
			}
		}
	optsDone:
		if len(rest) < 2 {
			fail(fmt.Errorf("usage: golem observe [--media <file>] [--force] <channel> <text>"))
		}
		req := wire.Request{Cmd: "observe", Channel: rest[0], Content: strings.Join(rest[1:], " "), Force: force}
		if mediaPath != "" {
			data, err := os.ReadFile(mediaPath)
			if err != nil {
				fail(err)
			}
			req.Media = data
			req.Mime = mime.TypeByExtension(filepath.Ext(mediaPath))
		}
		resp := call(home, req)
		o := resp.Observe
		if o.Conscious {
			fmt.Printf("%.2f bits, conscious -> fragment #%d\n", o.Bits, o.FragmentID)
		} else {
			fmt.Printf("%.2f bits, predicted away\n", o.Bits)
		}

	case "see":
		rest := args[1:]
		if len(rest) < 1 {
			fail(fmt.Errorf("usage: golem see <imagefile>"))
		}
		data, err := os.ReadFile(rest[0])
		if err != nil {
			fail(err)
		}
		resp := call(home, wire.Request{Cmd: "see", Media: data, Mime: mime.TypeByExtension(filepath.Ext(rest[0]))})
		if resp.Say != "" {
			fmt.Println(resp.Say)
		}

	case "tell":
		rest := args[1:]
		lang, source, key := "", "", ""
		for len(rest) >= 2 {
			consumed := true
			switch rest[0] {
			case "--lang":
				lang = rest[1]
			case "--source":
				source = rest[1]
			case "--key":
				key = rest[1]
			default:
				consumed = false
			}
			if !consumed {
				break
			}
			rest = rest[2:]
		}
		switch {
		case len(rest) >= 3 && rest[0] == "--about":
			tellAboutOnce(home, lang, rest[1], strings.Join(rest[2:], " "))
		case len(rest) >= 1 && rest[0] == "--about":
			fail(fmt.Errorf("usage: golem tell [--lang <tag>] --about <query> <words>"))
		case len(rest) >= 1:
			tellOnce(home, lang, source, key, strings.Join(rest, " "))
		default:
			tellSession(home)
		}

	case "trust":
		rest := args[1:]
		if len(rest) < 1 {
			fail(fmt.Errorf("usage: golem trust <source> [level]"))
		}
		cfg, err := config.Load(home)
		if err != nil {
			fail(err)
		}
		req := wire.Request{Cmd: "trust", Query: rest[0], Key: cfg.MasterKey}
		if len(rest) >= 2 {
			lvl, perr := strconv.ParseFloat(rest[1], 64)
			if perr != nil {
				fail(fmt.Errorf("level must be a number between 0 and 1"))
			}
			req.Command = "set"
			req.Weight = lvl
		}
		resp := call(home, req)
		if resp.Error != "" {
			fail(fmt.Errorf("%s", resp.Error))
		}
		fmt.Println(resp.Say)

	case "watch":
		watchStream(home)

	case "name":
		rest := args[1:]
		lang := ""
		if len(rest) >= 2 && rest[0] == "--lang" {
			lang = rest[1]
			rest = rest[2:]
		}
		if len(rest) < 2 {
			fail(fmt.Errorf("usage: golem name [--lang <tag>] <word> <concept phrase>"))
		}
		resp := call(home, wire.Request{Cmd: "name", Content: rest[0], Query: strings.Join(rest[1:], " "), Lang: lang})
		if resp.Error != "" {
			fail(fmt.Errorf("%s", resp.Error))
		}
		fmt.Println(resp.Say)

	case "recall":
		resp := call(home, wire.Request{Cmd: "recall", Query: strings.Join(args[1:], " ")})
		if len(resp.Fragments) == 0 {
			fmt.Println("nothing comes to mind.")
			return
		}
		printFragments(resp.Fragments)

	case "compute":
		if len(args) < 2 {
			fail(fmt.Errorf("usage: golem compute <program>"))
		}
		resp := call(home, wire.Request{Cmd: "runcode", Content: strings.Join(args[1:], " ")})
		fmt.Println(resp.Say)

	case "dump":
		resp := call(home, wire.Request{Cmd: "dump"})
		if len(args) >= 2 && args[1] == "--dot" {
			fmt.Print(renderDOT(resp.Fragments, resp.Links))
			return
		}
		if len(resp.Fragments) == 0 {
			fmt.Println("memory is empty.")
			return
		}
		printFragments(resp.Fragments)

	case "teach":
		if len(args) >= 2 && args[1] == "--mind" {
			if len(args) != 5 {
				fail(fmt.Errorf("usage: golem teach --mind <word> <relation> <signal>"))
			}
			call(home, wire.Request{Cmd: "teach", Mind: true, Content: args[2], Command: args[3] + " " + args[4]})
			if args[3] == "act" {
				fmt.Printf("named the %s act %q\n", args[4], args[2])
			} else {
				fmt.Printf("named the thought operation walk(%s,%s) %q\n", args[3], args[4], args[2])
			}
			return
		}
		if len(args) >= 2 && args[1] == "--say" {
			if len(args) < 4 {
				fail(fmt.Errorf("usage: golem teach --say <intent> <template>   (e.g. introspect \"my mind was on {thought}\")"))
			}
			call(home, wire.Request{Cmd: "teach", Say: true, Content: args[2], Command: strings.Join(args[3:], " ")})
			fmt.Printf("taught a way to say %q: %q\n", args[2], strings.Join(args[3:], " "))
			return
		}
		if len(args) >= 2 && args[1] == "--inflect" {
			if len(args) < 6 {
				fail(fmt.Errorf("usage: golem teach --inflect <lang> <feature> <base:inflected>...   (e.g. tr past kovala:kovaladı gel:geldi oku:okudu)"))
			}
			call(home, wire.Request{Cmd: "teach", Inflect: true, Lang: args[2], Content: args[3], Command: strings.Join(args[4:], " ")})
			fmt.Printf("taught the %s %s inflection from %d examples\n", args[2], args[3], len(args)-4)
			return
		}
		if len(args) < 3 {
			fail(fmt.Errorf("usage: golem teach <cue> <command>   (repeat to strengthen)"))
		}
		call(home, wire.Request{Cmd: "teach", Content: args[1], Command: strings.Join(args[2:], " ")})
		fmt.Printf("grounded %q -> %q\n", args[1], strings.Join(args[2:], " "))

	case "propose":
		if len(args) < 2 {
			fail(fmt.Errorf("usage: golem propose <cue>"))
		}
		resp := call(home, wire.Request{Cmd: "propose", Content: strings.Join(args[1:], " ")})
		printDecision(resp.Decision)

	case "approve":
		resp := call(home, wire.Request{Cmd: "approve"})
		printAction(resp.Action)

	case "deny":
		resp := call(home, wire.Request{Cmd: "deny"})
		printAction(resp.Action)

	case "concepts":
		resp := call(home, wire.Request{Cmd: "concepts"})
		if len(resp.Concepts) == 0 {
			fmt.Println("no concepts born yet; they distil from recurring shapes during sleep.")
			return
		}
		for _, c := range resp.Concepts {
			warmth := ""
			if c.Heat > 0 {
				warmth = fmt.Sprintf("  warm %.2f", c.Heat)
			}
			fmt.Printf("#%-3d [%-4s] %q  grounded in %d occurrence(s)%s\n",
				c.ID, c.Category, c.Gloss, c.Grounded, warmth)
			if len(c.Names) > 0 {
				fmt.Printf("     named by: %s\n", strings.Join(c.Names, ", "))
			}
			if len(c.Features) > 0 {
				fmt.Printf("     signature: %s\n", strings.Join(c.Features, ", "))
			}
		}

	case "health":
		resp := call(home, wire.Request{Cmd: "health"})
		h := resp.Health
		if h == nil {
			fmt.Println("no health report.")
			return
		}
		if len(h.Duplicates) == 0 && len(h.Promiscuous) == 0 {
			fmt.Printf("clean: %d concepts, no duplicate nodes, no word naming more than one concept.\n", h.Concepts)
			return
		}
		fmt.Printf("examined %d concepts\n", h.Concepts)
		if len(h.Duplicates) > 0 {
			fmt.Println("\nduplicate nodes (one meaning split across concepts):")
			for _, dp := range h.Duplicates {
				marks := make([]string, len(dp.IDs))
				for i, id := range dp.IDs {
					marks[i] = fmt.Sprintf("#%d", id)
				}
				fmt.Printf("  [%s] %q -> %s\n", dp.Category, dp.Gloss, strings.Join(marks, ", "))
			}
		}
		if len(h.Promiscuous) > 0 {
			fmt.Println("\nwords naming more than one concept (a weak target is a strayed name):")
			for _, lw := range h.Promiscuous {
				parts := make([]string, len(lw.Targets))
				for i, t := range lw.Targets {
					parts[i] = fmt.Sprintf("%s %.2f", t.Gloss, t.Weight)
				}
				fmt.Printf("  %s(%s) -> %s\n", lw.Word, lw.Lang, strings.Join(parts, ", "))
			}
		}
		os.Exit(1)

	case "habits":
		resp := call(home, wire.Request{Cmd: "habits"})
		if len(resp.Habits) == 0 {
			fmt.Println("no grounded habits yet.")
			return
		}
		for _, h := range resp.Habits {
			fmt.Printf("  [%-9s] %q -> %q  (approvals %d, denials %d)\n",
				h.Status, h.Cue, h.Action, h.Approvals, h.Denials)
		}

	case "audit":
		resp := call(home, wire.Request{Cmd: "audit"})
		if len(resp.Audit) == 0 {
			fmt.Println("no decisions recorded yet.")
			return
		}
		for _, e := range resp.Audit {
			fmt.Printf("  %s  %q -> %q  %s\n",
				time.Unix(e.AtUnix, 0).Format("Jan 02 15:04:05"), e.Cue, e.Action, e.Outcome)
		}

	case "voice":
		req := wire.Request{Cmd: "voice", Arg: "status"}
		if len(args) > 1 {
			switch args[1] {
			case "on", "off", "status":
				req.Arg = args[1]
			case "use":
				req.Arg = "use"
				req.Content = strings.Join(args[2:], " ")
			default:
				fail(fmt.Errorf("usage: golem voice [on|off|status|use NAME]"))
			}
		}
		resp := call(home, req)
		name := "system default"
		if resp.Voice != nil && resp.Voice.Name != "" {
			name = resp.Voice.Name
		}
		state := "off"
		if resp.Voice != nil && resp.Voice.Enabled {
			state = "on"
		}
		fmt.Printf("voice: %s (%s)\n", state, name)

	case "sleep":
		call(home, wire.Request{Cmd: "sleep"})
		fmt.Println("golem consolidated once and is still awake.")

	case "forget":
		if len(args) < 2 {
			fail(fmt.Errorf("usage: golem forget <what>"))
		}
		resp := call(home, wire.Request{Cmd: "forget", Query: strings.Join(args[1:], " ")})
		if resp.Say != "" {
			fmt.Println(resp.Say)
		} else {
			fmt.Println("forgotten.")
		}

	case "stop":
		call(home, wire.Request{Cmd: "stop"})
		fmt.Println("golem is sleeping once and shutting down.")

	case "shellhook":
		fmt.Print(shellHook)

	case "launchd":
		printLaunchd(home)

	default:
		fmt.Println(usageText)
		os.Exit(2)
	}
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "golem:", err)
	os.Exit(1)
}

// shellHook is the zsh snippet that turns the user's shell into a sensory
// channel. Commands and their failures arrive like any other observation;
// nothing is recorded when the daemon is not running.
const shellHook = `# golem shell sense: report each command and its failures to the daemon.
# Install: golem shellhook >> ~/.zshrc   (then open a new shell)
if command -v golem >/dev/null 2>&1; then
  _golem_preexec() { (golem observe shell "run: $1" >/dev/null 2>&1 &) }
  _golem_precmd()  {
    local _s=$?
    (( _s != 0 )) && (golem observe shell "command failed with exit status $_s" >/dev/null 2>&1 &)
  }
  autoload -Uz add-zsh-hook
  add-zsh-hook preexec _golem_preexec
  add-zsh-hook precmd  _golem_precmd
fi
`

// watchStream subscribes to the daemon's event stream and renders the talking
// channel as a two-sided conversation: what GOLEM says and what each source says to
// it, labelled by who spoke. It is how the architect opens and observes a GOLEM-LLM
// exchange live. Any other event is left to the structured trail.
func watchStream(home string) {
	conn, err := net.Dial("unix", wire.SocketPath(home))
	if err != nil {
		if _, statErr := os.Stat(wire.SocketPath(home)); os.IsNotExist(statErr) {
			fail(fmt.Errorf("the daemon is not running; start it with: golem run"))
		}
		fail(err)
	}
	defer conn.Close()
	fmt.Fprintln(conn, `{"cmd":"subscribe"}`)
	sc := bufio.NewScanner(conn)
	sc.Buffer(make([]byte, 0, 64*1024), 1<<20)
	for sc.Scan() {
		var ev events.Event
		if json.Unmarshal(sc.Bytes(), &ev) != nil {
			continue
		}
		switch ev.Ev {
		case "heard":
			src := ev.Cue
			if src == "" {
				src = "someone"
			}
			fmt.Printf("%s: %s\n", src, ev.Tpl)
		case "say":
			fmt.Printf("golem: %s\n", ev.Tpl)
		}
	}
}

// call performs one IPC round trip and exits with a friendly message when
// the daemon is not running.
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

func printStatus(st *wire.StatusInfo) {
	fmt.Printf("golem daemon, pid %d, up %s\n", st.PID, time.Duration(st.UptimeSec*float64(time.Second)).Round(time.Second))
	fmt.Printf("  fragments held: %d (%d in the hot working set)\n", st.Fragments, st.HotFragments)
	if st.Concepts > 0 || st.Words > 0 {
		fmt.Printf("  semantic memory: %d concept(s), %d word(s)\n", st.Concepts, st.Words)
	}
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
	if len(st.Channels) == 0 {
		fmt.Println("  channels: none observed yet")
	}
	for name, ch := range st.Channels {
		fmt.Printf("  channel %-10s observed %d, conscious %d, suppressed %d, dropped %d\n",
			name, ch.Observed, ch.Conscious, ch.Suppressed, ch.Dropped)
	}
	if st.Sleeps > 0 {
		fmt.Printf("  nights slept: %d (last %s)\n", st.Sleeps, time.Unix(st.LastSleepUnix, 0).Format("15:04:05"))
	} else {
		fmt.Println("  nights slept: 0")
	}
	fmt.Printf("  chem: dopamine %.2f, noradrenaline %.2f, serotonin %.2f, oxytocin %.2f, adenosine %.2f\n",
		st.Chem.Dopamine, st.Chem.Noradrenaline, st.Chem.Serotonin, st.Chem.Oxytocin, st.Chem.Adenosine)
	if st.PromotionBar > 0 {
		fmt.Printf("  graduation bar: %d consistent approval(s)\n", st.PromotionBar)
	}
	if st.ExecuteEnabled {
		fmt.Printf("  execution: ON, %d command(s) allowlisted\n", st.AllowlistSize)
	} else {
		fmt.Println("  execution: off (dry-run); enable in config.json")
	}
	if st.VoiceEnabled {
		name := st.VoiceName
		if name == "" {
			name = "system default"
		}
		fmt.Printf("  voice: on (%s)\n", name)
	} else {
		fmt.Println("  voice: off")
	}
	for _, cmd := range st.Running {
		fmt.Printf("  running now: %q\n", cmd)
	}
	if st.Queued > 0 {
		fmt.Printf("  queued: %d action(s) waiting on a slot or a claimed resource\n", st.Queued)
	}
}

func tellOnce(home, lang, source, key, words string) {
	resp := call(home, wire.Request{Cmd: "tell", Content: words, Lang: lang, Source: source, Key: key})
	if resp.Error != "" {
		fail(fmt.Errorf("%s", resp.Error))
	}
	// A definition is learned, not stored as a percept, so it answers with a
	// spoken line and no remembered fragment.
	if resp.Observe != nil {
		fmt.Printf("remembered as #%d (%.2f bits)\n", resp.Observe.FragmentID, resp.Observe.Bits)
	}
	if resp.Say != "" {
		fmt.Printf("golem> %s\n", resp.Say)
	}
	printTellProposalHint(resp.Decision)
	if len(resp.Fragments) > 0 {
		fmt.Println("it brings to mind:")
		printFragments(resp.Fragments)
	}
}

// printTellProposalHint follows a spoken proposal with the lever names,
// since a tell-staged proposal waits on the same approve and deny.
func printTellProposalHint(di *wire.DecisionInfo) {
	if di == nil || !di.Grounded {
		return
	}
	if di.Conflict {
		fmt.Println("  resolve by proposing the specific cue, then approve.")
		return
	}
	fmt.Println("  golem approve  to run it, or  golem deny  to reject.")
}

func tellAboutOnce(home, lang, query, words string) {
	resp := call(home, wire.Request{Cmd: "tell", Query: query, Content: words, Lang: lang})
	fmt.Printf("remembered as #%d, bound to moments matching %q\n", resp.Observe.FragmentID, query)
	if len(resp.Fragments) > 0 {
		fmt.Println("it brings to mind:")
		printFragments(resp.Fragments)
	}
}

func tellSession(home string) {
	fmt.Println("speak to golem; an empty line ends the session.")
	sc := bufio.NewScanner(os.Stdin)
	for {
		fmt.Print("you> ")
		if !sc.Scan() {
			if err := sc.Err(); err != nil {
				fmt.Fprintln(os.Stderr, "golem: input error:", err)
			}
			fmt.Println()
			return
		}
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			return
		}
		resp := call(home, wire.Request{Cmd: "tell", Content: line})
		if resp.Say != "" {
			fmt.Printf("golem> %s\n", resp.Say)
		}
		printTellProposalHint(resp.Decision)
		if len(resp.Fragments) == 0 {
			if resp.Say == "" {
				fmt.Printf("golem> remembered #%d; nothing comes to mind yet.\n", resp.Observe.FragmentID)
			}
			continue
		}
		fmt.Printf("golem> remembered #%d; it brings to mind:\n", resp.Observe.FragmentID)
		for _, f := range resp.Fragments {
			fmt.Printf("golem>   ~%.2f #%d %s %q\n", f.Activation, f.ID, f.Channel, f.Content)
		}
	}
}

func printDecision(d *wire.DecisionInfo) {
	if d == nil || !d.Grounded {
		fmt.Printf("nothing grounded for %q yet.\n", d.Cue)
		return
	}
	if d.Conflict {
		fmt.Printf("conflict: %s\n", d.Question)
		fmt.Println("  resolve by proposing the specific cue, then approve.")
		return
	}
	if d.Autonomous {
		fmt.Printf("ran autonomously: %s\n  %s\n", d.Proposed, d.Outcome)
		if d.Demoted {
			fmt.Println("  the run failed, so this pair is demoted until you reconfirm it.")
		}
		return
	}
	mark := ""
	if d.Denylisted {
		mark = "  (denylisted; always confirm)"
	}
	fmt.Printf("proposing: %s  [saliency %.2f, %s]%s\n", d.Proposed, d.Saliency, d.Status, mark)
	if d.Status == "graduated" {
		fmt.Println("  graduated, but execution is off or this command is not allowlisted,")
		fmt.Println("  so it stays pending. golem approve  to run it now.")
	} else {
		fmt.Println("  golem approve  to run and confirm, or  golem deny  to reject.")
	}
	if len(d.Alternatives) > 0 {
		fmt.Printf("  also considered: %s\n", strings.Join(d.Alternatives, ", "))
	}
}

func printAction(a *wire.ActionResult) {
	if a == nil {
		return
	}
	if a.Outcome == "denied" {
		fmt.Printf("denied %q (demoted; approvals %d, denials %d)\n", a.Command, a.Approvals, a.Denials)
		return
	}
	fmt.Printf("%s\n  %q is now %q (approvals %d)\n", a.Outcome, a.Command, a.Status, a.Approvals)
}

func printFragments(frags []*wire.FragmentInfo) {
	for _, f := range frags {
		extra := ""
		if f.Activation > 0 && f.Activation < 1 {
			extra = fmt.Sprintf("  assoc %.2f", f.Activation)
		}
		if f.Count > 1 {
			extra += fmt.Sprintf("  x%d", f.Count)
		}
		if f.LinkCount > 0 {
			extra += fmt.Sprintf("  links %d", f.LinkCount)
		}
		head := f.Channel
		switch f.Kind {
		case memory.KindWord:
			head = "word"
			if f.Lang != "" {
				head = "word(" + f.Lang + ")"
			}
		case memory.KindConcept:
			head = "concept"
			if f.Category != "" {
				head = "concept(" + f.Category + ")"
			}
		case memory.KindOperation:
			head = "operation"
		case memory.KindGrammar:
			head = "grammar"
		case memory.KindFeature:
			head = "feature"
		case memory.KindConstruction:
			head = "construction"
		}
		fmt.Printf("#%-3d %s  %-12s %.2f bits  NA=%.2f  weight %.2f%s\n     %q\n",
			f.ID, time.Unix(f.AtUnix, 0).Format("Jan 02 15:04:05"), head,
			f.Surprise, f.Limbic.Noradrenaline, f.Weight, extra, f.Content)
		if f.Media != "" {
			fmt.Printf("     saw: %s (%s)\n", f.Media, f.Mime)
		}
	}
}

func printLaunchd(home string) {
	bin, err := os.Executable()
	if err != nil {
		fail(err)
	}
	fmt.Printf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>Label</key><string>com.golem.daemon</string>
	<key>ProgramArguments</key>
	<array>
		<string>%s</string>
		<string>run</string>
	</array>
	<key>EnvironmentVariables</key>
	<dict>
		<key>GOLEM_HOME</key><string>%s</string>
	</dict>
	<key>RunAtLoad</key><true/>
	<key>KeepAlive</key><true/>
	<key>StandardOutPath</key><string>%s/golem.log</string>
	<key>StandardErrorPath</key><string>%s/golem.err.log</string>
</dict>
</plist>
`, bin, home, home, home)
	fmt.Fprintf(os.Stderr, "\nSave as ~/Library/LaunchAgents/com.golem.daemon.plist, then:\n  launchctl load ~/Library/LaunchAgents/com.golem.daemon.plist\n")
}
