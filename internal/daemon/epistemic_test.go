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
