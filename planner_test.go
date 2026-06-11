package main

import (
	"strings"
	"testing"
	"time"
)

// feedSequence plays the ordered pair (first, then second) n times, each
// round its own episode, so successor edges accumulate.
func feedSequence(t *testing.T, d *Daemon, first, second string, n int, base time.Time) {
	t.Helper()
	for i := range n {
		at := base.Add(time.Duration(i) * 10 * time.Minute)
		d.recent = nil
		d.process(Observation{Channel: "shell", Content: first, At: at}, true, true)
		d.process(Observation{Channel: "shell", Content: second, At: at.Add(5 * time.Second)}, true, true)
	}
	d.recent = nil
}

func say(t *testing.T, d *Daemon, words string) Response {
	t.Helper()
	resp, _, _ := d.handle(Request{Cmd: "tell", Content: words}, nil)
	if !resp.OK {
		t.Fatalf("tell %q failed: %s", words, resp.Error)
	}
	return resp
}

// calm builds the sleep pressure a real night would have: genesis is
// serotonin-paced, and a day of surprises leaves serotonin floored.
func calm(d *Daemon) {
	for range 20 {
		d.chem.BuildSleepPressure()
	}
}

// The done-when of the language step: "what usually happens after make
// test" is answered with a constructed sentence naming the successor.
func TestAnswersWhatFollowsQuestion(t *testing.T) {
	d, _ := testDaemon(t)
	feedSequence(t, d, "run: make test", "run: make deploy", 3, time.Now().Add(-3*time.Hour))

	resp := say(t, d, "what usually happens after make test")
	if resp.Say != "After make test, make deploy usually follows." {
		t.Fatalf("the answer should be built, not retrieved: %q", resp.Say)
	}
}

// The same question answers through the concept layer once the shapes
// have distilled, speaking concepts rather than raw lines.
func TestAnswersThroughConcepts(t *testing.T) {
	d, s := testDaemon(t)
	feedSequence(t, d, "run: make test", "run: make deploy", 4, time.Now().Add(-4*time.Hour))
	if _, err := s.MergeDuplicates(); err != nil {
		t.Fatal(err)
	}
	calm(d)
	if born, _ := d.growConcepts(); born == 0 {
		t.Fatal("the merged shapes should distil into concepts")
	}

	resp := say(t, d, "what usually happens after make test")
	if resp.Say != "After make test, make deploy usually follows." {
		t.Fatalf("the merged graph should answer the same: %q", resp.Say)
	}
}

// A corrected system hedges the same inference it would otherwise
// assert.
func TestLowTrustHedgesTheAnswer(t *testing.T) {
	d, _ := testDaemon(t)
	feedSequence(t, d, "run: make test", "run: make deploy", 3, time.Now().Add(-3*time.Hour))
	d.chem.LowerTrust(0.4)

	resp := say(t, d, "what usually happens after make test")
	if resp.Say != "After make test, make deploy may follow." {
		t.Fatalf("low oxytocin should hedge: %q", resp.Say)
	}
}

// "Before" questions read the successor edges backward.
func TestAnswersWhatPrecedesQuestion(t *testing.T) {
	d, _ := testDaemon(t)
	feedSequence(t, d, "run: make test", "run: make deploy", 3, time.Now().Add(-3*time.Hour))

	resp := say(t, d, "what comes before make deploy")
	if resp.Say != "Make test usually precedes make deploy." {
		t.Fatalf("precedence should answer from the same edges: %q", resp.Say)
	}
}

// A question asked seconds after an event binds as that event's
// successor in the graph, and must still never appear in the answer.
// With nothing else following the fresh moment, the honest answer is
// ignorance, never the question itself.
func TestQuestionNeverAnswersItself(t *testing.T) {
	d, _ := testDaemon(t)
	d.process(Observation{Channel: "vitals", Content: "disk space low", At: time.Now()}, true, true)

	resp := say(t, d, "what usually happens after disk space low")
	if resp.Say != "I do not know what follows disk space low yet." {
		t.Fatalf("the asking must not become the answer: %q", resp.Say)
	}
	// Asked again, the first question is now also in the graph; the
	// answer still must not surface it.
	resp = say(t, d, "what usually happens after disk space low")
	if strings.Contains(resp.Say, "usually happens") {
		t.Fatalf("an earlier question leaked into the answer: %q", resp.Say)
	}
}

// Outside what is grounded, GOLEM says it does not know.
func TestAdmitsIgnorance(t *testing.T) {
	d, _ := testDaemon(t)
	feedSequence(t, d, "run: make test", "run: make deploy", 3, time.Now().Add(-3*time.Hour))

	resp := say(t, d, "what usually happens after make deploy")
	if resp.Say != "I do not know what follows make deploy yet." {
		t.Fatalf("nothing follows the last step, so admit it: %q", resp.Say)
	}
	resp = say(t, d, "what is flibber")
	if resp.Say != "I do not know flibber yet." {
		t.Fatalf("an ungrounded word should be admitted: %q", resp.Say)
	}
}

// Greedy naming can bond a question's words to a bystander concept; the
// answer must still anchor on the event the question named when that
// shape exists in memory, never on what the words happen to point at.
func TestAnchorPrefersExactShapeOverWordVotes(t *testing.T) {
	d, s := testDaemon(t)
	base := time.Now().Add(-4 * time.Hour)

	// "disk space low" is always followed by a cleanup; "memory pressure
	// warning" keeps it company so both shapes earn concepts.
	for i := range 4 {
		at := base.Add(time.Duration(i) * 30 * time.Minute)
		d.recent = nil
		d.process(Observation{Channel: "vitals", Content: "memory pressure warning", At: at}, true, true)
		d.process(Observation{Channel: "vitals", Content: "disk space low", At: at.Add(2 * time.Second)}, true, true)
		d.process(Observation{Channel: "shell", Content: "run: df cleanup", At: at.Add(5 * time.Second)}, true, true)
	}
	d.recent = nil
	// The words of the phrase bond to the bystander concept only.
	memEpisodes, _ := s.EpisodesByExactShape("memory pressure warning", 8)
	var anchors []recentMoment
	for _, f := range memEpisodes {
		anchors = append(anchors, recentMoment{id: f.ID, template: f.Template, at: f.Time})
	}
	d.mintWords("disk space low", "en", 0, anchors)
	if _, err := s.MergeDuplicates(); err != nil {
		t.Fatal(err)
	}
	calm(d)
	d.growConcepts()

	resp := say(t, d, "what usually happens after disk space low")
	if !strings.HasPrefix(resp.Say, "After disk space low,") {
		t.Fatalf("the answer must anchor on the asked shape: %q", resp.Say)
	}
	if !strings.Contains(resp.Say, "df cleanup") {
		t.Fatalf("the successor should be the asked shape's, got %q", resp.Say)
	}
}

// One co-occurrence is an anecdote: a pair seen a single time is not
// spoken as a habit, and repetition is what lifts it past the floor.
func TestSingleOccurrenceIsNotAHabit(t *testing.T) {
	d, _ := testDaemon(t)
	feedSequence(t, d, "run: make test", "run: make deploy", 1, time.Now().Add(-time.Hour))

	resp := say(t, d, "what usually happens after make test")
	if resp.Say != "I do not know what follows make test yet." {
		t.Fatalf("one episode must not be asserted as a habit: %q", resp.Say)
	}

	feedSequence(t, d, "run: make test", "run: make deploy", 1, time.Now().Add(-30*time.Minute))
	resp = say(t, d, "what usually happens after make test")
	if resp.Say != "After make test, make deploy usually follows." {
		t.Fatalf("a repeated pair is a habit: %q", resp.Say)
	}
}

// A definition question reads the lexicon: the word names its concept,
// and the grounding tally counts the experience behind it.
func TestAnswersDefinitionQuestion(t *testing.T) {
	d, s := testDaemon(t)
	anchors := feedShape(t, s, "shell", "run: go test ./...", 4, time.Now().Add(-2*time.Hour))
	d.mintWords("testler", "tr", 0, anchors)
	if _, err := s.MergeDuplicates(); err != nil {
		t.Fatal(err)
	}
	d.growConcepts()

	resp := say(t, d, "what is testler")
	if !strings.HasPrefix(resp.Say, "Testler names go test ./...") {
		t.Fatalf("the word should resolve to its concept: %q", resp.Say)
	}
	if !strings.Contains(resp.Say, "I have seen it 4 times.") {
		t.Fatalf("the grounding tally should be spoken: %q", resp.Say)
	}
}

// Command force stages a proposal under the usual gates; speech never
// executes by itself.
func TestCommandForceStagesProposal(t *testing.T) {
	d, _ := testDaemon(t)
	ground(t, d, "run the tests", "go test ./...", 4)

	resp := say(t, d, "run the tests")
	if resp.Say != "I propose go test ./..." {
		t.Fatalf("a grounded command should be proposed: %q", resp.Say)
	}
	if resp.Decision == nil || !resp.Decision.Grounded || resp.Decision.Proposed != "go test ./..." {
		t.Fatalf("the proposal should ride the reply: %+v", resp.Decision)
	}
	if d.pending == nil || d.pending.Proposed.Command != "go test ./..." {
		t.Fatal("the proposal should wait on approve or deny")
	}
}

// An ungrounded command is admitted, with nothing staged.
func TestUngroundedCommandAdmitted(t *testing.T) {
	d, _ := testDaemon(t)
	resp := say(t, d, "run the florble")
	if resp.Say != "I do not know how to run the florble yet." {
		t.Fatalf("an ungrounded command should be admitted: %q", resp.Say)
	}
	if d.pending != nil {
		t.Fatal("nothing should be staged for an unknown command")
	}
}

// "Do it again" resolves through the workspace: the warmest verb-like
// concept supplies the command.
func TestDoItAgainResolvesFromWorkspace(t *testing.T) {
	d, s := testDaemon(t)
	anchors := feedShape(t, s, "shell", "run: go test ./...", 4, time.Now().Add(-2*time.Hour))
	d.mintWords("tests", "en", 0, anchors)
	if _, err := s.MergeDuplicates(); err != nil {
		t.Fatal(err)
	}
	d.growConcepts()

	// Mentioning the tests warms their concept; the pro-form then has a
	// referent.
	say(t, d, "the tests look slow today")
	resp := say(t, d, "do it again")
	if resp.Say != "I propose go test ./..." {
		t.Fatalf("the pro-form should resolve to the warm concept: %q", resp.Say)
	}
	if d.pending == nil {
		t.Fatal("the resolved command should be staged for approval")
	}
}

// With nothing warm, the pro-form has no referent and GOLEM admits it.
func TestDoItAgainWithColdWorkspace(t *testing.T) {
	d, _ := testDaemon(t)
	resp := say(t, d, "do it again")
	if resp.Say != "I do not know how to do it again yet." {
		t.Fatalf("a cold workspace leaves the pro-form unresolved: %q", resp.Say)
	}
}

// Statements stay quiet at baseline, and curiosity asks about the first
// unknown when dopamine is high.
func TestStatementCuriosity(t *testing.T) {
	d, _ := testDaemon(t)

	resp := say(t, d, "that gizmo is broken")
	if resp.Say != "" {
		t.Fatalf("a statement at baseline drive should stay quiet: %q", resp.Say)
	}
	d.chem.SpikeDopamine(0.5)
	resp = say(t, d, "that gizmo is broken")
	if resp.Say != "What is gizmo?" {
		t.Fatalf("high dopamine should ask about the unknown: %q", resp.Say)
	}
}

// Greedy naming bonds a once-said word too; a name that kept the concept's
// company far less than its firmest is spoken as a tentative pointer, and
// does not borrow the concept's tally as if it were the word's own.
func TestWeakNamingIsTentative(t *testing.T) {
	d, s := testDaemon(t)
	anchors := feedShape(t, s, "shell", "run: go test ./...", 4, time.Now().Add(-2*time.Hour))
	// "tests" keeps company every run; "green" only once.
	d.mintWords("tests", "en", 0, anchors)
	d.mintWords("green", "en", 0, anchors[:1])
	if _, err := s.MergeDuplicates(); err != nil {
		t.Fatal(err)
	}
	d.growConcepts()

	firm := say(t, d, "what is tests")
	if !strings.HasPrefix(firm.Say, "Tests names go test ./...") {
		t.Fatalf("a firm name should be asserted: %q", firm.Say)
	}
	if !strings.Contains(firm.Say, "seen it") {
		t.Fatalf("a firm answer counts the experience: %q", firm.Say)
	}
	weak := say(t, d, "what is green")
	if !strings.HasPrefix(weak.Say, "Green may name go test ./...") {
		t.Fatalf("a weak name should be hedged: %q", weak.Say)
	}
	if strings.Contains(weak.Say, "seen it") {
		t.Fatalf("a tentative pointer must not borrow the concept's tally: %q", weak.Say)
	}
}
