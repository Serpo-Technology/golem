package main

import (
	"fmt"
	"io"
	"os"
	"strings"
	"time"
)

// runLanguageDemo plays one synthetic day into a throwaway memory and
// holds a conversation about it: sequences bind in order, sleep distils
// recurring shapes into concepts and bonds the day's words to them, and
// then GOLEM answers questions with sentences it builds, admits what it
// does not know, and turns spoken commands into staged proposals.
func runLanguageDemo() {
	home, err := os.MkdirTemp("", "golem-language-")
	if err != nil {
		fail(err)
	}
	defer os.RemoveAll(home)

	cfg := DefaultConfig()
	cfg.Channels = ChannelsConfig{}
	d, err := NewDaemon(home, cfg, io.Discard)
	if err != nil {
		fail(err)
	}
	defer d.store.Close()

	line := "--------------------------------------------------------------------"
	day := time.Now().Add(-6 * time.Hour)

	feed := func(channel, content string, at time.Time) {
		d.process(Observation{Channel: channel, Content: content, At: at}, true, true)
	}
	speakAt := func(words, lang string, at time.Time) {
		res := d.process(Observation{Channel: "tell", Content: words, At: at}, true, true)
		anchors := d.recentWithin(at, res.FragmentID)
		minted := d.mintWords(words, lang, res.FragmentID, anchors)
		d.warmConcepts(minted, exciteSpeech)
	}
	ask := func(words string) {
		fmt.Printf("   you>   %s\n", words)
		resp, _ := d.handleNow(Request{Cmd: "tell", Content: words})
		if resp.Say != "" {
			fmt.Printf("   golem> %s\n", resp.Say)
		}
	}

	fmt.Println("GOLEM language demo. One synthetic day, one night of sleep, then a")
	fmt.Println("conversation. Memory is throwaway; commands are proposed, never run.")
	fmt.Println(line)

	fmt.Println("\n1. A ROUTINE. The shell sees the same sequence four times across")
	fmt.Println("   the day:  make test  ->  make deploy --staging")
	fmt.Println("   Binding is asymmetric: each earlier moment points at what")
	fmt.Println("   followed it, and nothing points backward.")
	for i := range 4 {
		at := day.Add(time.Duration(i) * 80 * time.Minute)
		d.recent = nil
		feed("shell", "run: make test", at)
		feed("shell", "run: make deploy --staging", at.Add(20*time.Second))
	}
	d.recent = nil

	fmt.Println("\n2. WORDS KEEP COMPANY. While the tests run you speak, in two")
	fmt.Println("   languages. A word you use every run bonds firmly to the")
	fmt.Println("   moment; a word said once in passing bonds weakly:")
	fmt.Println("      \"the tests pass\" / \"testler temiz\"   every run")
	fmt.Println("      \"looks green\"                       said once")
	for i := range 3 {
		at := day.Add(time.Duration(i)*90*time.Minute + 5*time.Second)
		feed("shell", "run: make test", at)
		speakAt("the tests pass", "en", at.Add(3*time.Second))
		speakAt("testler temiz", "tr", at.Add(6*time.Second))
		d.recent = nil
	}
	// A single passing remark: the bystander adjective bonds only once.
	oneOff := day.Add(3*90*time.Minute + 5*time.Second)
	feed("shell", "run: make test", oneOff)
	speakAt("looks green", "en", oneOff.Add(3*time.Second))
	d.recent = nil

	fmt.Println("\n3. SLEEP. The night merges repeats into prototypes, distils")
	fmt.Println("   prototypes into concepts, and bonds the day's words to the")
	fmt.Println("   concepts their moments ground.")
	for range 20 {
		d.chem.BuildSleepPressure()
	}
	d.sleepCycle()
	concepts, err := d.conceptInfos()
	if err != nil {
		fail(err)
	}
	for _, c := range concepts {
		fmt.Printf("   concept [%-4s] %q  grounded in %d occurrence(s)\n", c.Category, c.Gloss, c.Grounded)
		if len(c.Names) > 0 {
			fmt.Printf("           named by: %s\n", strings.Join(c.Names, ", "))
		}
	}
	fmt.Println("   Naming is greedy, so \"green\" bonds too, but only once: it")
	fmt.Println("   keeps the concept's company far less than \"testler\", and that")
	fmt.Println("   gap is what GOLEM hears as a weak, tentative pointer.")

	fmt.Println("\n4. QUESTIONS. The answers are built by the grammar, never replayed")
	fmt.Println("   from stored text; outside its grounding GOLEM says so. The")
	fmt.Println("   interrogative routes the answer, tense is read and spoken back,")
	fmt.Println("   and inflection is computed, never bolted on.")
	ask("what usually happens after make test")
	ask("what happened after make test")
	ask("which command follows make test")
	ask("what comes before make deploy")
	ask("how many times did make test happen")
	ask("when did make test happen")
	ask("why did make test happen")
	ask("what is testler")
	ask("what is green")
	ask("what usually happens after make deploy")

	fmt.Println("\n5. THE OTHER LANGUAGE. A Turkish question is answered in Turkish,")
	fmt.Println("   about the same concept the English question reaches: SOV order,")
	fmt.Println("   the anchor wearing its case suffix under vowel harmony.")
	ask("make test'ten sonra ne gelir?")
	ask("make test nedir")
	ask("make zzz'den sonra ne gelir?")
	fmt.Println("   And \"testler\" shares no text with any shell moment; recall")
	fmt.Println("   reaches the experience through the concept:")
	resp, _ := d.handleNow(Request{Cmd: "recall", Query: "testler"})
	shown := 0
	for _, f := range resp.Fragments {
		if f.Kind == KindConcept {
			fmt.Printf("   [concept] %q\n", f.Content)
			continue
		}
		if f.Kind == "" && f.Channel == "shell" && shown < 2 {
			count := ""
			if f.Count > 1 {
				count = fmt.Sprintf("  x%d", f.Count)
			}
			fmt.Printf("   [moment]  %q%s\n", f.Content, count)
			shown++
		}
	}

	fmt.Println("\n6. SPEECH MOVES THE HANDS, BEHIND THE SAME GATES. A spoken command")
	fmt.Println("   becomes a proposal; nothing runs without approval.")
	ask("run the tests")
	ask("do it again")

	fmt.Println("\n" + line)
	fmt.Println("The pointer chain under every answer: word -> concept -> moments,")
	fmt.Println("with order carried on directed edges laid down by lived sequence.")
}
