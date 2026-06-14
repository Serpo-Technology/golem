package daemon

import (
	"strings"
	"testing"
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
