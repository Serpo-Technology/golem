package daemon

import (
	"strings"
	"testing"
)

// A clause with a second-person subject takes the base verb form, not the
// third-person -s: "You want the rose", not "You wants the rose".
func TestSecondPersonAgreement(t *testing.T) {
	d, _ := testDaemon(t)
	groundNoun(t, d, "rose")
	groundNoun(t, d, "dog")
	say(t, d, "the dog wants the rose") // grounds the want relation
	say(t, d, "I want the rose")        // a participant (the user) as agent
	got := say(t, d, "what do I want").Say
	if strings.Contains(got, "wants") {
		t.Errorf("second person should take the base form: %q", got)
	}
	if !strings.Contains(strings.ToLower(got), "want the rose") {
		t.Errorf("expected 'You want the rose': %q", got)
	}
}
