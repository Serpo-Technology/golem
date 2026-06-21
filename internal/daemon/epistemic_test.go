package daemon

import (
	"strings"
	"testing"
	"time"

	"golem/internal/memory"
)

// groundOccurrences grounds a concept in experience the way perception does: it
// points the concept at an episode standing for n sightings, so the concept's
// grounding tally is positive. groundNoun alone only names a concept; knowing a
// thing is having grounded it.
func groundOccurrences(t *testing.T, d *Daemon, c *memory.Fragment, n int) {
	t.Helper()
	ep := &memory.Fragment{Kind: memory.KindEpisodic, Channel: "vision", Content: c.Content,
		Count: n, Limbic: d.chem.Snapshot(), Time: time.Now()}
	if _, err := d.store.Encode(ep); err != nil {
		t.Fatal(err)
	}
	if err := d.store.SetLinkRel(c.ID, ep.ID, memory.RelGrounds, 0.9); err != nil {
		t.Fatal(err)
	}
}

// A grounded thing is known: "do you know the lion" affirms once the lion has
// been seen, and names what the question asked about.
func TestEpistemicKnowsGroundedThing(t *testing.T) {
	d, _ := testDaemon(t)
	lion := groundNoun(t, d, "lion")
	groundOccurrences(t, d, lion, 3)
	got := strings.ToLower(say(t, d, "do you know the lion").Say)
	if !strings.Contains(got, "yes") || !strings.Contains(got, "i know the lion") {
		t.Errorf("a grounded thing should be known: %q", got)
	}
}

// A thing whose word resolves but which was never grounded is not known: knowing
// is grounding, not merely being able to resolve a name.
func TestEpistemicUngroundedNotKnown(t *testing.T) {
	d, _ := testDaemon(t)
	groundNoun(t, d, "lion") // named, never seen
	got := strings.ToLower(say(t, d, "do you know the lion").Say)
	if !strings.Contains(got, "do not know") {
		t.Errorf("an ungrounded thing should not be known: %q", got)
	}
}

// A thing with no concept at all is not known, and the question is still caught,
// so the imperative misreading of "do" never surfaces.
func TestEpistemicUnknownThingCaught(t *testing.T) {
	d, _ := testDaemon(t)
	got := strings.ToLower(say(t, d, "do you know a dragon").Say)
	if !strings.Contains(got, "do not know") {
		t.Errorf("an unknown thing should not be known: %q", got)
	}
	if strings.Contains(got, "how to") {
		t.Errorf("the clause should not leak as a command: %q", got)
	}
}

// Every auxiliary that fronts the question is read the same way: "did you know"
// is the reflexive epistemic too, not a polar that misses to "No".
func TestEpistemicDidOpener(t *testing.T) {
	d, _ := testDaemon(t)
	cat := groundNoun(t, d, "cat")
	groundOccurrences(t, d, cat, 2)
	got := strings.ToLower(say(t, d, "did you know the cat").Say)
	if !strings.Contains(got, "yes") {
		t.Errorf("did-you-know should read as the epistemic question: %q", got)
	}
}

// "Do I know X" asks of the speaker, whose mind GOLEM cannot read, so it admits
// the limit rather than answering from its own grounding.
func TestEpistemicSpeakerDeferred(t *testing.T) {
	d, _ := testDaemon(t)
	lion := groundNoun(t, d, "lion")
	groundOccurrences(t, d, lion, 3)
	got := strings.ToLower(say(t, d, "do i know the lion").Say)
	if !strings.Contains(got, "cannot tell what you know") {
		t.Errorf("a question about the speaker's knowledge should be deferred: %q", got)
	}
}

// A proposition can be the object of know: "do you know that the dog chased the
// cat" asks whether GOLEM holds that fact, settled against the graph the way a
// polar question is, not by resolving an incidental noun in it.
func TestEpistemicPropositionHeld(t *testing.T) {
	d, _ := testDaemon(t)
	dog := groundNoun(t, d, "dog")
	groundNoun(t, d, "cat")
	say(t, d, "the dog chased the cat") // GOLEM is told the fact

	// The fact it was told, it knows, naming the proposition back.
	if got := say(t, d, "do you know that the dog chased the cat").Say; !strings.HasPrefix(got, "Yes") || !strings.Contains(got, "dog chased the cat") {
		t.Errorf("a told proposition should be known: %q", got)
	}
	// The reverse, never told, it does not know: word order fixes the roles.
	if got := say(t, d, "do you know that the cat chased the dog").Say; !strings.HasPrefix(got, "No") {
		t.Errorf("an unheld proposition should not be known: %q", got)
	}
	// "Do you know that dog" is the demonstrative, not a proposition, so it falls
	// through to the concept check.
	groundOccurrences(t, d, dog, 2)
	if got := say(t, d, "do you know that dog").Say; !strings.HasPrefix(got, "Yes") {
		t.Errorf("the demonstrative should fall through to the concept check: %q", got)
	}
}

// "What do you know" is the open self-report: it reads the whole graph and names
// the kinds GOLEM has formed and the actions it has learned, where "what do you
// know about X" reads a single concept.
func TestKnowledgeSummaryReportsKindsAndActions(t *testing.T) {
	d, _ := testDaemon(t)
	dog := groundNoun(t, d, "dog")
	cat := groundNoun(t, d, "cat")
	say(t, d, "a dog chased a cat") // mints the chase relation

	// A true superordinate GOLEM has formed: a kind generalizing over two members,
	// where a basic noun's own abstraction grounds in just its one percept.
	animal := &memory.Fragment{Kind: memory.KindConcept, Channel: "concept", Content: "animal",
		Template: "abs:en:animal", Category: "noun", Limbic: d.chem.Snapshot(), Time: time.Now()}
	if _, err := d.store.Encode(animal); err != nil {
		t.Fatal(err)
	}
	w, _, err := d.store.WordNode("animal", "en", d.chem.Snapshot(), time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if err := d.store.SetLinkRel(w.ID, animal.ID, memory.RelNames, 0.9); err != nil {
		t.Fatal(err)
	}
	for _, m := range []*memory.Fragment{dog, cat} {
		if err := d.store.SetLinkRel(animal.ID, m.ID, memory.RelGrounds, 0.8); err != nil {
			t.Fatal(err)
		}
	}

	got := say(t, d, "what do you know").Say
	if !strings.Contains(got, "animal") {
		t.Errorf("the summary should name a kind it formed: %q", got)
	}
	if !strings.Contains(got, "chase") {
		t.Errorf("the summary should name an action it learned: %q", got)
	}
	// The open form must not be read as a description of GOLEM itself.
	if strings.Contains(strings.ToLower(got), "you names") || strings.Contains(got, "GOLEM") {
		t.Errorf("the open question should summarize knowledge, not describe the self: %q", got)
	}
}

// "What do you know about X" still reads the single concept, not the whole-graph
// summary: the open form must not swallow the focused one.
func TestKnowledgeSummaryLeavesAboutForm(t *testing.T) {
	d, _ := testDaemon(t)
	groundNoun(t, d, "dog")
	groundNoun(t, d, "cat")
	say(t, d, "a dog chased a cat")
	got := say(t, d, "what do you know about the dog").Say
	if !strings.Contains(strings.ToLower(got), "dog") {
		t.Errorf("the about-form should describe the named concept: %q", got)
	}
	if strings.Contains(got, "kinds like") {
		t.Errorf("the about-form should not give the whole-graph summary: %q", got)
	}
}

// The recognizer is specific: an ordinary "does" question, not a "do you know",
// is left alone, so a real polar still answers from the graph.
func TestEpistemicLeavesOrdinaryQuestion(t *testing.T) {
	d, _ := testDaemon(t)
	groundNoun(t, d, "dog")
	groundNoun(t, d, "cat")
	say(t, d, "a dog chased a cat")
	got := strings.ToLower(say(t, d, "does the dog chase the cat").Say)
	if strings.Contains(got, "i know") || strings.Contains(got, "cannot tell") {
		t.Errorf("an ordinary polar should not be read as epistemic: %q", got)
	}
}
