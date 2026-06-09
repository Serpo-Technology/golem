package main

import (
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
  golem status                    show daemon status
  golem chem                      show current neurochemistry
  golem observe <channel> <text>  feed one observation to the daemon
  golem recall [query]            recall memory; without a query, returns the
                                  most stressful moment
  golem dump                      list every fragment held in memory
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
		runDemo()

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
		resp := call(home, Request{Cmd: "chem"})
		c := resp.Chem
		fmt.Printf("dopamine %.2f   noradrenaline %.2f   serotonin %.2f\n",
			c.Dopamine, c.Noradrenaline, c.Serotonin)

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

	case "recall":
		resp := call(home, Request{Cmd: "recall", Query: strings.Join(args[1:], " ")})
		if len(resp.Fragments) == 0 {
			fmt.Println("nothing comes to mind.")
			return
		}
		printFragments(resp.Fragments)

	case "dump":
		resp := call(home, Request{Cmd: "dump"})
		if len(resp.Fragments) == 0 {
			fmt.Println("memory is empty.")
			return
		}
		printFragments(resp.Fragments)

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
	fmt.Printf("  fragments held: %d\n", st.Fragments)
	if len(st.Channels) == 0 {
		fmt.Println("  channels: none observed yet")
	}
	for name, ch := range st.Channels {
		fmt.Printf("  channel %-10s observed %d, conscious %d, dropped %d\n",
			name, ch.Observed, ch.Conscious, ch.Dropped)
	}
	if st.Sleeps > 0 {
		fmt.Printf("  nights slept: %d (last %s)\n", st.Sleeps, time.Unix(st.LastSleepUnix, 0).Format("15:04:05"))
	} else {
		fmt.Println("  nights slept: 0")
	}
	fmt.Printf("  chem: dopamine %.2f, noradrenaline %.2f, serotonin %.2f\n",
		st.Chem.Dopamine, st.Chem.Noradrenaline, st.Chem.Serotonin)
}

func printFragments(frags []*FragmentInfo) {
	for _, f := range frags {
		fmt.Printf("#%-3d %s  %-8s %.2f bits  NA=%.2f  weight %.2f\n     %q\n",
			f.ID, time.Unix(f.AtUnix, 0).Format("Jan 02 15:04:05"), f.Channel,
			f.Surprise, f.Limbic.Noradrenaline, f.Weight, f.Content)
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
