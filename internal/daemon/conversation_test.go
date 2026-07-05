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

// A deliberate label binds only to a FRESHLY ATTENDED referent. A concept that is
// merely warm from an earlier scene (mulled internally, never attended) must not
// receive a firm name, or the name would outlive the workspace on the wrong
// concept; the label declines instead.
func TestRememberAsDeclinesOnStaleWarmth(t *testing.T) {
	d, s := testDaemon(t)
	c := &memory.Fragment{Kind: memory.KindConcept, Channel: "concept", Content: "brass shape",
		Template: "brass shape", Category: "noun", Limbic: d.chem.Snapshot(), Time: time.Now()}
	if _, err := s.Encode(c); err != nil {
		t.Fatal(err)
	}
	d.ws.mull(c.ID, 1.0) // warm, but not freshly attended (no seenAt)

	say(t, d, "remember this as a bell")
	if named := d.resolveConcept("bell"); named != nil {
		t.Fatalf("a label must not bind to a merely-warm, unattended concept: %+v", named)
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

// A greeting is met with the tongue's greeting and a farewell with its
// farewell, in every seeded tongue, so the exchange opens and closes as a
// social move rather than in silence. The words route as grounded speech
// acts; the replies are the seeded greeting surfaces.
func TestGreetingMet(t *testing.T) {
	d, _ := testDaemon(t)
	for _, c := range []struct{ say, want string }{
		{"hello", "Hello."},
		{"hi", "Hello."},
		{"merhaba", "Merhaba."},
		{"salve", "Salve."},
		{"goodbye", "Goodbye."},
		{"hoşçakal", "Hoşçakal."},
		{"vale", "Vale."},
	} {
		if got := say(t, d, c.say).Say; got != c.want {
			t.Fatalf("%q should be met with %q, got %q", c.say, c.want, got)
		}
	}
}

// A clause that carries more than the greeting is not swallowed by it: the
// question rides through to its own answer, and a greeting leaves the topic
// in play untouched.
func TestGreetingDoesNotSwallowContent(t *testing.T) {
	d, _ := testDaemon(t)
	groundNoun(t, d, "dog")
	if got := strings.ToLower(say(t, d, "hello what is a dog").Say); !strings.Contains(got, "dog") {
		t.Fatalf("a greeting-opened question should still answer: %q", got)
	}
	say(t, d, "what is a dog") // the dog is the topic in mind
	before := d.topic
	say(t, d, "hello")
	if d.topic != before {
		t.Fatalf("a greeting should not displace the topic: %d then %d", before, d.topic)
	}
}

// A pro-form in a role question resolves against working memory: after "a fox
// chased a rabbit", "it" is the fox when the question seats it as the chaser
// and the rabbit when it seats it as the chased, read off the roles each
// concept lately played rather than off animacy.
func TestRoleQuestionResolvesProForm(t *testing.T) {
	d, _ := testDaemon(t)
	groundNoun(t, d, "fox")
	groundNoun(t, d, "rabbit")
	say(t, d, "a fox chased a rabbit")
	if got := say(t, d, "what did it chase").Say; got != "The fox chased the rabbit." {
		t.Fatalf("\"it\" should resolve to the fox that chased: %q", got)
	}
	if got := say(t, d, "who chased it").Say; got != "The fox chased the rabbit." {
		t.Fatalf("\"it\" should resolve to the rabbit that was chased: %q", got)
	}
}

// A role gap invites the telling that would fill it: the admission trails the
// tongue's invitation, and the filled gap is still followed up through the open
// question the admission recorded.
func TestRoleGapInvitesTeaching(t *testing.T) {
	d, _ := testDaemon(t)
	groundNoun(t, d, "dog")
	groundNoun(t, d, "cat")
	groundNoun(t, d, "spoon")
	say(t, d, "a dog chased a cat")
	got := say(t, d, "what did the spoon chase").Say
	if !strings.HasPrefix(got, "I do not know what yet.") || !strings.Contains(got, "Tell me, if you know.") {
		t.Fatalf("a role gap should admit and invite: %q", got)
	}
}

// The invitation rides the admission whatever the chemistry: the curiosity
// follow-up never re-asks a word the admission already speaks, so the invite
// is what keeps the gap open to the speaker at any dopamine.
func TestInviteAtHighDopamine(t *testing.T) {
	d, _ := testDaemon(t)
	d.chem.SpikeDopamine(0.6)
	got := say(t, d, "what is flibber").Say
	if got != "I do not know flibber yet. Tell me, if you know." {
		t.Fatalf("the admission should invite at high dopamine too: %q", got)
	}
}

// A participant's identity is taught by talk and read back by the who
// question: "you are golem" names the self, "I am Ada" names the speaker,
// and each answers from its own side of the conversation afterward.
func TestIdentityToldAndAsked(t *testing.T) {
	d, _ := testDaemon(t)
	if got := say(t, d, "who are you").Say; !strings.HasPrefix(got, "I do not know who yet.") {
		t.Fatalf("unnamed self should admit the gap: %q", got)
	}
	if got := say(t, d, "you are golem").Say; got != "I am golem." {
		t.Fatalf("the naming should confirm the identity: %q", got)
	}
	if got := say(t, d, "who are you").Say; got != "I am golem." {
		t.Fatalf("the who question should answer the given name: %q", got)
	}
	if got := say(t, d, "I am Ada").Say; got != "You are ada." {
		t.Fatalf("the speaker's name should land on the speaker: %q", got)
	}
	if got := say(t, d, "who am i").Say; got != "You are ada." {
		t.Fatalf("the speaker's who question should answer their name: %q", got)
	}
}

// An articled predicate is a kind and stays on the told-membership path; a
// judgement word does not rename a participant already named.
func TestIdentityKindAndJudgementStayMembership(t *testing.T) {
	d, _ := testDaemon(t)
	if got := say(t, d, "you are a daemon").Say; !strings.Contains(got, "I am a daemon") {
		t.Fatalf("an articled predicate is a membership: %q", got)
	}
	say(t, d, "you are golem")
	say(t, d, "you are right") // a judgement, not a rename
	if got := say(t, d, "who are you").Say; got != "I am golem." {
		t.Fatalf("a bare judgement word must not rename the self: %q", got)
	}
}

// Identity is the architect's to give: a peer's bare claim lands as the
// defeasible told-identity, never as the name.
func TestPeerCannotNameParticipant(t *testing.T) {
	d, _ := testDaemon(t)
	sayAs(t, d, "llm", "you are hal")
	if got := say(t, d, "who are you").Say; !strings.HasPrefix(got, "I do not know who yet.") {
		t.Fatalf("a peer's claim must not become the name: %q", got)
	}
}

// A because-compound teaches both clauses and the cause between them, and a
// later why answers from the told cause rather than the succession walk.
func TestBecauseTeachesAndWhyReads(t *testing.T) {
	d, _ := testDaemon(t)
	groundNoun(t, d, "dog")
	groundNoun(t, d, "cat")
	groundNoun(t, d, "bone")
	if got := say(t, d, "a dog chased a cat because the cat grabbed a bone").Say; got != "I see." {
		t.Fatalf("the compound telling should be acknowledged: %q", got)
	}
	if got := say(t, d, "what did the dog chase").Say; got != "The dog chased the cat." {
		t.Fatalf("the main clause should have taught: %q", got)
	}
	if got := say(t, d, "why did the dog chase the cat").Say; got != "Because the cat grabbed the bone." {
		t.Fatalf("why should answer the told cause: %q", got)
	}
	// A second compound and a night: each cause stays on its own moment (an
	// untemplated cause moment once merged into another compound's and answered
	// with its cause), and the told causes survive the consolidation.
	say(t, d, "a cat chased a mouse because the mouse grabbed a cheese")
	d.sleepCycle()
	if got := say(t, d, "why did the dog chase the cat").Say; got != "Because the cat grabbed the bone." {
		t.Fatalf("the first cause should hold its own moment across the night: %q", got)
	}
	if got := say(t, d, "why did the cat chase the mouse").Say; got != "Because the mouse grabbed the cheese." {
		t.Fatalf("the second cause should hold its own moment across the night: %q", got)
	}
	// A fact with no told cause keeps the old succession answer.
	say(t, d, "a dog fetched a ball")
	if got := say(t, d, "why did the dog fetch the ball").Say; strings.HasPrefix(got, "Because") {
		t.Fatalf("a fact without a told cause must not invent one: %q", got)
	}
}

// An untagged Turkish clause is placed by the lexicon GOLEM has grown: once
// Turkish words exist, a bare Turkish telling answers in its own tongue.
func TestUntaggedTurkishSniffed(t *testing.T) {
	d, _ := testDaemon(t)
	sayLang(t, d, "tr", "köpek kediyi kovaladı")
	if got := say(t, d, "kedi köpeği kovaladı").Say; got != "Anladım." {
		t.Fatalf("an untagged Turkish clause should answer in Turkish: %q", got)
	}
	// An English clause with no closed-class token keeps the English default.
	groundNoun(t, d, "dog")
	if got := say(t, d, "dog chased cat").Say; got == "Anladım." {
		t.Fatalf("an English clause must not read as Turkish: %q", got)
	}
}

// A given identity survives the night: the participant is spared the grounding
// prune and its name the statistical rewrite, so "who are you" answers the
// same after consolidations as before them.
func TestIdentitySurvivesSleep(t *testing.T) {
	d, _ := testDaemon(t)
	say(t, d, "you are golem")
	d.sleepCycle()
	d.sleepCycle()
	if got := say(t, d, "who are you").Say; got != "I am golem." {
		t.Fatalf("the given name should survive consolidation: %q", got)
	}
}

// A participant asked about with any interrogative answers its identity:
// "what are you" is the who-question in other clothes, never the pointer
// shape "You names GOLEM".
func TestWhatAreYouAnswersIdentity(t *testing.T) {
	d, _ := testDaemon(t)
	if got := say(t, d, "what are you").Say; !strings.HasPrefix(got, "I do not know who yet.") {
		t.Fatalf("an unnamed self should admit the identity gap: %q", got)
	}
	say(t, d, "you are golem")
	if got := say(t, d, "what are you").Say; got != "I am golem." {
		t.Fatalf("the what-question should answer identity: %q", got)
	}
}

// A definition leads with the grounded kind and closes on what the thing
// does, so "what is X" reads the knowledge the graph holds and not only the
// caption pointer or the vacuous kind.
func TestDefinitionLeadsWithKindAndFacts(t *testing.T) {
	d, s := testDaemon(t)
	dog := groundNoun(t, d, "dog")
	groundNoun(t, d, "cat")
	animal := &memory.Fragment{Kind: memory.KindConcept, Channel: "concept", Content: "animal",
		Template: "abs:en:animal", Category: "noun", Limbic: d.chem.Snapshot(), Time: time.Now()}
	if _, err := s.Encode(animal); err != nil {
		t.Fatal(err)
	}
	if err := s.LinkRel(animal.ID, dog.ID, memory.RelGrounds, 0.8); err != nil {
		t.Fatal(err)
	}
	say(t, d, "a dog chased a cat")
	d.sleepCycle() // the fresh fact entrenches; the appendix reads the graph
	got := say(t, d, "what is a dog").Say
	if !strings.Contains(got, "Dog is an animal") {
		t.Fatalf("the definition should lead with the grounded kind: %q", got)
	}
	if !strings.Contains(got, "chase") {
		t.Fatalf("the definition should close on what the dog does: %q", got)
	}
}

// A clause the grammar cannot frame is admitted and the missing word asked,
// never met with silence: incomprehension is audible.
func TestUnframedClauseAdmitsAndAsks(t *testing.T) {
	d, _ := testDaemon(t)
	if got := say(t, d, "take it down").Say; got != "I did not understand that. What is take?" {
		t.Fatalf("an unframed clause should admit and ask: %q", got)
	}
}

// A pro-form two referents fit equally is asked about, the conflict veto
// applied to reference, and the next turn naming one settles the parked
// question.
func TestAmbiguousReferentAskedAndSettled(t *testing.T) {
	d, _ := testDaemon(t)
	for _, n := range []string{"dog", "cat", "fox", "rabbit"} {
		groundNoun(t, d, n)
	}
	say(t, d, "a dog chased a cat")
	say(t, d, "a fox chased a rabbit")
	got := say(t, d, "what did it chase").Say
	if !strings.Contains(got, "Do you mean the ") || !strings.Contains(got, "fox") || !strings.Contains(got, "dog") {
		t.Fatalf("two equally warm chasers should be asked about: %q", got)
	}
	if got := say(t, d, "the fox").Say; got != "The fox chased the rabbit." {
		t.Fatalf("naming a candidate should answer the parked question: %q", got)
	}
	if got := say(t, d, "what did the dog chase").Say; got != "The dog chased the cat." {
		t.Fatalf("ordinary questions should be untouched after the reply: %q", got)
	}
}
