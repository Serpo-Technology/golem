package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"time"
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
  golem status                    show daemon status
  golem chem                      show current neurochemistry
  golem chem --watch              print it once a second until interrupted
  golem observe <channel> <text>  feed one observation to the daemon
  golem tell <words>              speak to GOLEM; the words become a memory
                                  bound to the moments around them, a
                                  question is answered from what is
                                  grounded, and a command becomes a proposal
  golem tell                      the same, as an interactive session
  golem tell --lang <tag> <words> tag the words' language ("tr", "de");
                                  words in any language converge on the
                                  same concepts once grounded alike
  golem tell --about <query> <words>
                                  bind the words to remembered moments that
                                  match the query instead of to the present
  golem recall [query]            recall memory plus its linked context;
                                  without a query, returns the most
                                  stressful moment
  golem concepts                  list what GOLEM has abstracted: each
                                  concept, its words, and its grounding
  golem teach <cue> <command>     ground a cue to a command by hand; repeat
                                  to strengthen toward graduation
  golem propose <cue>             weigh the actions grounded to a cue and
                                  propose one, or ask when it is ambiguous
  golem approve                   approve the pending proposal
  golem deny                      reject the pending proposal
  golem habits                    list grounded cue-and-command pairs
  golem audit                     show the recent decision trail
  golem dump                      list every fragment held in memory
  golem dump --dot                export the memory graph as graphviz dot
  golem stop                      ask the daemon to sleep once and shut down
  golem shellhook                 print the zsh hook that feeds the shell channel
  golem launchd                   print a launchd plist for start-at-login

The daemon keeps its memory, config, and socket in ~/.golem (override with
GOLEM_HOME). Edit ~/.golem/config.json to tune thresholds and sleep timing.`

func main() {
	args := os.Args[1:]
	if len(args) == 0 {
		fmt.Println(usageText)
		return
	}

	home, err := GolemHome()
	if err != nil {
		fail(err)
	}

	switch args[0] {
	case "demo":
		switch {
		case len(args) >= 2 && args[1] == "decide":
			runDecisionDemo()
		case len(args) >= 2 && args[1] == "execute":
			runExecutionDemo()
		case len(args) >= 2 && args[1] == "consolidate":
			runConsolidationDemo()
		case len(args) >= 2 && args[1] == "language":
			runLanguageDemo()
		case len(args) >= 2 && args[1] == "pipeline":
			runPipelineDemo()
		default:
			runDemo()
		}

	case "run":
		cfg, err := LoadConfig(home)
		if err != nil {
			fail(err)
		}
		d, err := NewDaemon(home, cfg, os.Stdout)
		if err != nil {
			fail(err)
		}
		if err := d.Run(); err != nil {
			fail(err)
		}

	case "status":
		resp := call(home, Request{Cmd: "status"})
		printStatus(resp.Status)

	case "chem":
		watch := len(args) >= 2 && args[1] == "--watch"
		for {
			resp := call(home, Request{Cmd: "chem"})
			c := resp.Chem
			if !watch {
				fmt.Printf("dopamine %.2f   noradrenaline %.2f   serotonin %.2f   oxytocin %.2f\n",
					c.Dopamine, c.Noradrenaline, c.Serotonin, c.Oxytocin)
				break
			}
			fmt.Printf("%s  dopamine %.2f   noradrenaline %.2f   serotonin %.2f   oxytocin %.2f\n",
				time.Now().Format("15:04:05"), c.Dopamine, c.Noradrenaline, c.Serotonin, c.Oxytocin)
			time.Sleep(time.Second)
		}

	case "observe":
		if len(args) < 3 {
			fail(fmt.Errorf("usage: golem observe <channel> <text>"))
		}
		resp := call(home, Request{Cmd: "observe", Channel: args[1], Content: strings.Join(args[2:], " ")})
		o := resp.Observe
		if o.Conscious {
			fmt.Printf("%.2f bits, conscious -> fragment #%d\n", o.Bits, o.FragmentID)
		} else {
			fmt.Printf("%.2f bits, predicted away\n", o.Bits)
		}

	case "tell":
		rest := args[1:]
		lang := ""
		if len(rest) >= 2 && rest[0] == "--lang" {
			lang = rest[1]
			rest = rest[2:]
		}
		switch {
		case len(rest) >= 3 && rest[0] == "--about":
			tellAboutOnce(home, lang, rest[1], strings.Join(rest[2:], " "))
		case len(rest) >= 1 && rest[0] == "--about":
			fail(fmt.Errorf("usage: golem tell [--lang <tag>] --about <query> <words>"))
		case len(rest) >= 1:
			tellOnce(home, lang, strings.Join(rest, " "))
		default:
			tellSession(home)
		}

	case "recall":
		resp := call(home, Request{Cmd: "recall", Query: strings.Join(args[1:], " ")})
		if len(resp.Fragments) == 0 {
			fmt.Println("nothing comes to mind.")
			return
		}
		printFragments(resp.Fragments)

	case "dump":
		resp := call(home, Request{Cmd: "dump"})
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
		if len(args) < 3 {
			fail(fmt.Errorf("usage: golem teach <cue> <command>   (repeat to strengthen)"))
		}
		call(home, Request{Cmd: "teach", Content: args[1], Command: strings.Join(args[2:], " ")})
		fmt.Printf("grounded %q -> %q\n", args[1], strings.Join(args[2:], " "))

	case "propose":
		if len(args) < 2 {
			fail(fmt.Errorf("usage: golem propose <cue>"))
		}
		resp := call(home, Request{Cmd: "propose", Content: strings.Join(args[1:], " ")})
		printDecision(resp.Decision)

	case "approve":
		resp := call(home, Request{Cmd: "approve"})
		printAction(resp.Action)

	case "deny":
		resp := call(home, Request{Cmd: "deny"})
		printAction(resp.Action)

	case "concepts":
		resp := call(home, Request{Cmd: "concepts"})
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
		}

	case "habits":
		resp := call(home, Request{Cmd: "habits"})
		if len(resp.Habits) == 0 {
			fmt.Println("no grounded habits yet.")
			return
		}
		for _, h := range resp.Habits {
			fmt.Printf("  [%-9s] %q -> %q  (approvals %d, denials %d)\n",
				h.Status, h.Cue, h.Action, h.Approvals, h.Denials)
		}

	case "audit":
		resp := call(home, Request{Cmd: "audit"})
		if len(resp.Audit) == 0 {
			fmt.Println("no decisions recorded yet.")
			return
		}
		for _, e := range resp.Audit {
			fmt.Printf("  %s  %q -> %q  %s\n",
				time.Unix(e.AtUnix, 0).Format("Jan 02 15:04:05"), e.Cue, e.Action, e.Outcome)
		}

	case "stop":
		call(home, Request{Cmd: "stop"})
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

// call performs one IPC round trip and exits with a friendly message when
// the daemon is not running.
func call(home string, req Request) Response {
	resp, err := ipcCall(home, req)
	if err != nil {
		if _, statErr := os.Stat(socketPath(home)); os.IsNotExist(statErr) {
			fail(fmt.Errorf("the daemon is not running; start it with: golem run"))
		}
		fail(err)
	}
	return resp
}

func printStatus(st *StatusInfo) {
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
	fmt.Printf("  chem: dopamine %.2f, noradrenaline %.2f, serotonin %.2f, oxytocin %.2f\n",
		st.Chem.Dopamine, st.Chem.Noradrenaline, st.Chem.Serotonin, st.Chem.Oxytocin)
	if st.PromotionBar > 0 {
		fmt.Printf("  graduation bar: %d consistent approval(s)\n", st.PromotionBar)
	}
	if st.ExecuteEnabled {
		fmt.Printf("  execution: ON, %d command(s) allowlisted\n", st.AllowlistSize)
	} else {
		fmt.Println("  execution: off (dry-run); enable in config.json")
	}
	for _, cmd := range st.Running {
		fmt.Printf("  running now: %q\n", cmd)
	}
	if st.Queued > 0 {
		fmt.Printf("  queued: %d action(s) waiting on a slot or a claimed resource\n", st.Queued)
	}
}

func tellOnce(home, lang, words string) {
	resp := call(home, Request{Cmd: "tell", Content: words, Lang: lang})
	fmt.Printf("remembered as #%d (%.2f bits)\n", resp.Observe.FragmentID, resp.Observe.Bits)
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
func printTellProposalHint(di *DecisionInfo) {
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
	resp := call(home, Request{Cmd: "tell", Query: query, Content: words, Lang: lang})
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
		resp := call(home, Request{Cmd: "tell", Content: line})
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

func printDecision(d *DecisionInfo) {
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

func printAction(a *ActionResult) {
	if a == nil {
		return
	}
	if a.Outcome == "denied" {
		fmt.Printf("denied %q (demoted; approvals %d, denials %d)\n", a.Command, a.Approvals, a.Denials)
		return
	}
	fmt.Printf("%s\n  %q is now %q (approvals %d)\n", a.Outcome, a.Command, a.Status, a.Approvals)
}

func printFragments(frags []*FragmentInfo) {
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
		case KindWord:
			head = "word"
			if f.Lang != "" {
				head = "word(" + f.Lang + ")"
			}
		case KindConcept:
			head = "concept"
			if f.Category != "" {
				head = "concept(" + f.Category + ")"
			}
		}
		fmt.Printf("#%-3d %s  %-12s %.2f bits  NA=%.2f  weight %.2f%s\n     %q\n",
			f.ID, time.Unix(f.AtUnix, 0).Format("Jan 02 15:04:05"), head,
			f.Surprise, f.Limbic.Noradrenaline, f.Weight, extra, f.Content)
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
