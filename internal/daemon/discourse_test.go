package daemon

import (
	"strings"
	"testing"
	"time"
)

// A pronoun binds by best fit over the recent referents, not only the hottest.
// After "a dog chased a cat" the dog stood as agent and the cat as patient, so
// "it" finds the inanimate cat and "she" the animate dog, the lean read from
// the roles they played rather than a hand-listed animacy.
func TestPronounBindsByAnimacy(t *testing.T) {
	d, _ := testDaemon(t)
	groundNoun(t, d, "dog")
	groundNoun(t, d, "cat")
	say(t, d, "a dog chased a cat")

	if got := say(t, d, "what is it").Say; !strings.HasPrefix(got, "Cat is a thing") {
		t.Fatalf("\"it\" should bind to the inanimate patient: %q", got)
	}
	if got := say(t, d, "what is she").Say; !strings.HasPrefix(got, "Dog is a thing") {
		t.Fatalf("\"she\" should bind to the animate agent: %q", got)
	}
}

// "Why" is a move on the warm proposition: it walks to the antecedent in the
// learned sequence, GOLEM's grounded answer where it models no cause. After
// learning that make deploy follows make test, "why" about the warm make
// deploy walks back to make test.
func TestWhyWalksToAntecedent(t *testing.T) {
	d, s := testDaemon(t)
	feedSequence(t, d, "run: make test", "run: make deploy", 4, time.Now().Add(-4*time.Hour))
	if _, err := s.MergeDuplicates(); err != nil {
		t.Fatal(err)
	}
	calm(d)
	d.growConcepts()
	d.recent = nil

	say(t, d, "what usually happens after make test?")
	resp := say(t, d, "why?")
	low := strings.ToLower(resp.Say)
	if !strings.Contains(low, "make test") || !strings.Contains(low, "preced") {
		t.Fatalf("why should walk to the antecedent: %q", resp.Say)
	}
}

// "And then" is a move on the warm proposition the other way: it walks to the
// successor. After the warm subject is make test, "and then" reaches make
// deploy, the consequent.
func TestAndThenWalksToSuccessor(t *testing.T) {
	d, s := testDaemon(t)
	feedSequence(t, d, "run: make test", "run: make deploy", 4, time.Now().Add(-4*time.Hour))
	if _, err := s.MergeDuplicates(); err != nil {
		t.Fatal(err)
	}
	calm(d)
	d.growConcepts()
	d.recent = nil

	say(t, d, "what usually happens before make deploy?")
	resp := say(t, d, "and then?")
	if !strings.Contains(resp.Say, "make deploy") {
		t.Fatalf("and then should walk to the successor: %q", resp.Say)
	}
}

// "What about X" shifts the conversation onto X and answers about it, the
// referent move that lets a later turn follow the new topic.
func TestWhatAboutAnswersTopic(t *testing.T) {
	d, _ := testDaemon(t)
	groundNoun(t, d, "dog")
	groundNoun(t, d, "cat")
	say(t, d, "a dog chased a cat")

	if got := say(t, d, "what about the dog").Say; !strings.HasPrefix(got, "Dog is a thing") {
		t.Fatalf("what about X should answer about X: %q", got)
	}
}

// A topic is followed across turns as the sustained warm subject, and when a
// new subject takes the field the change is noticed: the dog held while the
// talk stays on it, the cat displacing it when the talk turns.
func TestTopicFollowedAndChangeNoticed(t *testing.T) {
	d, _ := testDaemon(t)
	dog := groundNoun(t, d, "dog")
	cat := groundNoun(t, d, "cat")

	say(t, d, "what is a dog")
	if d.topic != dog.ID {
		t.Fatalf("the dog should be the topic: got %d want %d", d.topic, dog.ID)
	}
	say(t, d, "what is it")
	if d.topic != dog.ID {
		t.Fatalf("the topic should persist across a turn that stays on it: %d", d.topic)
	}
	say(t, d, "what is a cat")
	if d.topic != cat.ID {
		t.Fatalf("the topic should change to the cat when the talk turns: %d", d.topic)
	}
}
