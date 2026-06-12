package daemon

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	"golem/internal/attention"
	"golem/internal/grammar"
	"golem/internal/memory"
	"golem/internal/senses"
)

// A plain request drives the matching internal act: "recall the disk"
// runs the recall faculty, answers with what came to mind, and the
// firing is on the trail as an asked act.
func TestRecallIntentDrivesAct(t *testing.T) {
	d, _ := testDaemon(t)
	d.process(senses.Observation{Channel: "syslog",
		Content: "Disk full on /dev/disk1s9", At: time.Now().Add(-time.Hour)}, true, true)
	d.recent = nil

	resp := say(t, d, "recall the disk")
	if !strings.Contains(resp.Say, "I remember:") || !strings.Contains(resp.Say, "Disk full") {
		t.Fatalf("the spoken request should drive recall and speak the moment: %q", resp.Say)
	}
	outcome, ok := auditHas(t, d.store, "act:recall")
	if !ok || !strings.HasPrefix(outcome, "asked:") {
		t.Fatalf("the asked recall should land on the trail: %q %v", outcome, ok)
	}
}

// A recall request about nothing remembered admits the gap instead of
// bluffing.
func TestRecallIntentAdmitsGap(t *testing.T) {
	d, _ := testDaemon(t)
	resp := say(t, d, "recall the flurble")
	if resp.Say != "I do not know flurble yet." {
		t.Fatalf("an empty recall should admit it: %q", resp.Say)
	}
}

// "Remember this as a bell" grounds: the pro-form picks the warm
// referent, the word comes to name it, and the word recalls the same
// concept afterwards.
func TestRememberAsNamesWarmConcept(t *testing.T) {
	d, s := testDaemon(t)
	c := &memory.Fragment{Kind: memory.KindConcept, Channel: "concept", Content: "brass shape",
		Template: "brass shape", Category: "noun", Limbic: d.chem.Snapshot(), Time: time.Now()}
	if _, err := s.Encode(c); err != nil {
		t.Fatal(err)
	}
	d.ws.excite(c.ID, 1.0)

	resp := say(t, d, "remember this as a bell")
	if resp.Say != "Bell names brass shape." {
		t.Fatalf("the naming should be confirmed: %q", resp.Say)
	}
	named := d.resolveConcept("bell")
	if named == nil || named.ID != c.ID {
		t.Fatalf("the word should now name the warm concept: %+v", named)
	}
	if _, ok := auditHas(t, s, "act:ground"); !ok {
		t.Fatal("deliberate naming is the ground act and belongs on the trail")
	}
}

// "What were you thinking" reads the inner loop, the same report the
// introspect act and verb give.
func TestIntrospectIntent(t *testing.T) {
	d, _ := testDaemon(t)
	d.lastThought = "thought: after make test, make deploy"
	d.lastThoughtAt = time.Now()

	resp := say(t, d, "what were you thinking")
	if resp.Say != "I was thinking: after make test, make deploy" {
		t.Fatalf("the question should read the inner loop: %q", resp.Say)
	}
	resp = say(t, d, "introspect")
	if !strings.Contains(resp.Say, "after make test, make deploy") {
		t.Fatalf("the imperative should drive the same act: %q", resp.Say)
	}

	d2, _ := testDaemon(t)
	resp = say(t, d2, "what were you thinking")
	if resp.Say != "I have not been thinking about anything." {
		t.Fatalf("an empty inner loop admits it: %q", resp.Say)
	}
}

// "Tidy" consolidates on request, the same night the sleep verb runs,
// and says what it did.
func TestConsolidateIntent(t *testing.T) {
	d, _ := testDaemon(t)
	resp := say(t, d, "tidy")
	if !strings.Contains(resp.Say, "I consolidated") {
		t.Fatalf("the request should consolidate and report: %q", resp.Say)
	}
	if d.sleeps != 1 {
		t.Fatalf("the intent should run the same cycle the sleep verb runs, sleeps=%d", d.sleeps)
	}
	outcome, ok := auditHas(t, d.store, "act:consolidate")
	if !ok || !strings.HasPrefix(outcome, "asked:") {
		t.Fatalf("the asked consolidation should land on the trail: %q %v", outcome, ok)
	}
}

// An unknown command admits incomprehension instead of bluffing.
func TestUnknownCommandAdmits(t *testing.T) {
	d, _ := testDaemon(t)
	resp := say(t, d, "do the flurble dance")
	if !strings.Contains(resp.Say, "do not know how to") {
		t.Fatalf("an ungrounded command should admit it: %q", resp.Say)
	}
}

// The workspace carries reference across turns: "it" resolves to the
// warmest concept, "the other one" to the concept past it.
func TestProAndOtherResolveAcrossTurns(t *testing.T) {
	d, s := testDaemon(t)
	first := &memory.Fragment{Kind: memory.KindConcept, Channel: "concept", Content: "brass bell",
		Template: "brass bell", Category: "noun", Limbic: d.chem.Snapshot(), Time: time.Now()}
	second := &memory.Fragment{Kind: memory.KindConcept, Channel: "concept", Content: "small dog",
		Template: "small dog", Category: "noun", Limbic: d.chem.Snapshot(), Time: time.Now()}
	for _, c := range []*memory.Fragment{first, second} {
		if _, err := s.Encode(c); err != nil {
			t.Fatal(err)
		}
	}
	d.ws.excite(second.ID, 0.9)
	d.ws.excite(first.ID, 1.0)

	resp := say(t, d, "what is it?")
	if !strings.Contains(resp.Say, "Brass bell is a thing.") {
		t.Fatalf("a bare pro-form should resolve to the warmest concept: %q", resp.Say)
	}
	resp = say(t, d, "what is the other one?")
	if !strings.Contains(resp.Say, "Small dog is a thing.") {
		t.Fatalf("the other one should resolve past the warmest: %q", resp.Say)
	}
}

// A genuine gap earns a follow-up question when curiosity is high:
// GOLEM decides what to say next rather than only answering.
func TestFollowUpQuestionOnGap(t *testing.T) {
	d, _ := testDaemon(t)
	d.process(senses.Observation{Channel: "syslog",
		Content: "Disk full on /dev/disk1s9", At: time.Now().Add(-time.Hour)}, true, true)
	d.recent = nil
	d.chem.SpikeDopamine(0.6)

	resp := say(t, d, "when did the flurble arrive near the disk")
	if !strings.Contains(resp.Say, "What is flurble") {
		t.Fatalf("a high-dopamine gap should earn a follow-up question: %q", resp.Say)
	}
}

// The done-when: a multi-turn exchange about a grounded topic holds the
// thread. The answer's subject becomes the warm referent, "it" keeps
// pointing at it across turns, and "do it again" reaches that same
// topic's grounded command with no cue text at all.
func TestConversationHoldsThread(t *testing.T) {
	d, s := testDaemon(t)
	feedSequence(t, d, "run: make test", "run: make deploy", 4, time.Now().Add(-4*time.Hour))
	if _, err := s.MergeDuplicates(); err != nil {
		t.Fatal(err)
	}
	calm(d)
	d.growConcepts()
	d.recent = nil

	resp := say(t, d, "what usually happens after make test?")
	if !strings.Contains(resp.Say, "make deploy") {
		t.Fatalf("the exchange should open with the grounded answer: %q", resp.Say)
	}
	resp = say(t, d, "when did it happen?")
	if !strings.Contains(resp.Say, "Make deploy last happened on") {
		t.Fatalf("the pro-form should follow the answer's subject: %q", resp.Say)
	}
	resp = say(t, d, "do it again")
	if resp.Decision == nil || !resp.Decision.Grounded || resp.Decision.Proposed != "make deploy" {
		t.Fatalf("the thread should reach the topic's command: %+v", resp.Decision)
	}
	if !strings.Contains(resp.Say, "make deploy") {
		t.Fatalf("the proposal should be spoken: %q", resp.Say)
	}
}

// The new innate forms parse: faculty intents, the naming shape, the
// think question, and the alt pro-form.
func TestIntentParses(t *testing.T) {
	g := grammar.Builtin()
	f := g.Parse("recall the dog", nil)
	if f.Force != grammar.ForceCommand || f.Action.Phrase != "recall" || f.Patient.Phrase != "dog" {
		t.Fatalf("recall intent misparsed: %+v", f)
	}
	f = g.Parse("remember this as a bell", nil)
	if f.Action.Phrase != "recall" || !f.Pro || f.Patient.Phrase != "bell" {
		t.Fatalf("naming shape misparsed: %+v", f)
	}
	f = g.Parse("what were you thinking", nil)
	if f.Force != grammar.ForceQuestion || f.Action.Phrase != "think" || !f.Past {
		t.Fatalf("think question misparsed: %+v", f)
	}
	f = g.Parse("what is the other one?", nil)
	if f.Force != grammar.ForceQuestion || !f.Alt || !f.Pro {
		t.Fatalf("alt pro-form misparsed: %+v", f)
	}
	f = g.Parse("tidy", nil)
	if f.Force != grammar.ForceCommand || f.Action.Phrase != "consolidate" {
		t.Fatalf("tidy intent misparsed: %+v", f)
	}
}

// A store trained by an earlier build tops up: missing curriculum forms
// seed in at boot while held parameters keep their learned weights.
func TestGrammarTopUp(t *testing.T) {
	s, err := memory.OpenStore(filepath.Join(t.TempDir(), "m.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	// An older build's memory: the curriculum without the recall form,
	// and a form whose weight the nightly competition has eroded.
	for _, p := range grammar.Curriculum() {
		if p.Key == "fw:en:recall" {
			continue
		}
		f := &memory.Fragment{Kind: memory.KindGrammar, Channel: "grammar",
			Content: p.Value, Template: p.Key, Lang: p.Lang, Category: p.Category, Time: time.Now()}
		if _, err := s.Encode(f); err != nil {
			t.Fatal(err)
		}
		if p.Key == "fw:en:what" {
			if err := s.SetWeight(f.ID, 0.4); err != nil {
				t.Fatal(err)
			}
		}
	}

	d := testDaemonOn(t, s)
	reloaded, err := s.NodesByKind(memory.KindGrammar)
	if err != nil {
		t.Fatal(err)
	}
	var recallBack, whatKept bool
	for _, n := range reloaded {
		if n.Template == "fw:en:recall" {
			recallBack = true
		}
		if n.Template == "fw:en:what" && n.Weight == 0.4 {
			whatKept = true
		}
	}
	if !recallBack {
		t.Fatal("a missing curriculum form should seed in at boot")
	}
	if !whatKept {
		t.Fatal("a held parameter must keep its learned weight through the top-up")
	}
	frame := d.gram.Parse("recall the dog", d.resolveConcept)
	if frame.Action.Phrase != "recall" {
		t.Fatalf("the topped-up grammar should carry the intent: %+v", frame)
	}
}

// The thought channel never hijacks an introspection answer into an
// anchor: asking about thinking is not asking about the word thought.
func TestThinkQuestionDoesNotAnchor(t *testing.T) {
	d, _ := testDaemon(t)
	d.process(senses.Observation{Channel: thoughtChannel,
		Content: "thought: a stray association", At: time.Now()}, true, true)
	d.recent = nil
	d.lastThought = "thought: a stray association"
	d.lastThoughtAt = time.Now()

	resp := say(t, d, "what were you thinking")
	if !strings.Contains(resp.Say, "a stray association") {
		t.Fatalf("the introspection should report the thought: %q", resp.Say)
	}
	if tpl := attention.Normalize("what were you thinking"); tpl == "" {
		t.Fatal("normalize sanity")
	}
}
