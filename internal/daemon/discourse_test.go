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

// An elliptical follow-up to a role question inherits its relation: after "what
// did the cat chase", "what about the dog" asks chase of the dog, walking the
// same relation from a new referent rather than dropping back to a definition.
func TestRoleEllipsisInheritsRelation(t *testing.T) {
	d, _ := testDaemon(t)
	for _, n := range []string{"cat", "mouse", "dog", "bird"} {
		groundNoun(t, d, n)
	}
	say(t, d, "a cat chased a mouse")
	say(t, d, "a dog chased a bird")

	first := say(t, d, "what did the cat chase").Say
	if !strings.Contains(first, "cat") || !strings.Contains(first, "mouse") {
		t.Fatalf("the role question should answer from the cat's proposition: %q", first)
	}
	follow := say(t, d, "what about the dog").Say
	if !strings.Contains(follow, "dog") || !strings.Contains(follow, "bird") {
		t.Fatalf("the follow-up should inherit chase and walk it from the dog: %q", follow)
	}
}

// Recall leans toward what the exchange just established: the dog chasing the
// cat is drilled in, but with the bird set down this turn, "what did the dog
// chase" answers with the bird, the fresh claim winning the conversation
// without the long-term weight being touched.
func TestDiscourseSaliencePrefersFreshProposition(t *testing.T) {
	d, _ := testDaemon(t)
	for _, n := range []string{"dog", "cat", "bird"} {
		groundNoun(t, d, n)
	}
	for range 4 {
		say(t, d, "a dog chased a cat")
	}
	say(t, d, "a dog chased a bird")

	if got := say(t, d, "what did the dog chase").Say; !strings.Contains(got, "bird") {
		t.Fatalf("the freshly established proposition should win the thread: %q", got)
	}
}

// The answer composes beyond one clause: with two chases established this
// exchange, "what did the dog chase" coordinates both rather than naming a
// single co-argument.
func TestRoleAnswerCoordinatesThreadPropositions(t *testing.T) {
	d, _ := testDaemon(t)
	for _, n := range []string{"dog", "cat", "mouse"} {
		groundNoun(t, d, n)
	}
	say(t, d, "a dog chased a cat")
	say(t, d, "a dog chased a mouse")

	got := say(t, d, "what did the dog chase").Say
	if !strings.Contains(got, "cat") || !strings.Contains(got, "mouse") || !strings.Contains(got, " and ") {
		t.Fatalf("the answer should coordinate both established chases: %q", got)
	}
}
