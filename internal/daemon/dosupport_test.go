package daemon

import (
	"strings"
	"testing"
)

// Do-support fronts a yes/no question, not a command: "do you want the rose" is
// settled against the graph, never read as an imperative to act on. The reflexive
// "you" asks of GOLEM, "I" of the speaker, each weighed against what was grounded.
func TestDoSupportPolarReflexive(t *testing.T) {
	d, _ := testDaemon(t)
	groundNoun(t, d, "rose")
	groundNoun(t, d, "dog")
	say(t, d, "the dog wants the rose") // grounds the want relation
	say(t, d, "I want the rose")        // the speaker as the wanting participant

	// "Do I want the rose" asks of the speaker, who said as much: yes.
	if got := strings.ToLower(say(t, d, "do i want the rose").Say); !strings.Contains(got, "yes") {
		t.Errorf("the speaker's own want should affirm: %q", got)
	}
	// "Do you want the rose" asks of GOLEM, who was never said to want it, and is
	// answered, not taken for a command to carry out.
	got := strings.ToLower(say(t, d, "do you want the rose").Say)
	if strings.Contains(got, "yes") {
		t.Errorf("a want GOLEM does not hold should not affirm: %q", got)
	}
	if strings.Contains(got, "how to") {
		t.Errorf("do-support must not leak as a command: %q", got)
	}
}

// The bare imperative "do" still commands: "do the dishes" is not do-support, so
// it is left to the command path rather than read as a yes/no question.
func TestDoSupportLeavesImperative(t *testing.T) {
	d, _ := testDaemon(t)
	got := strings.ToLower(say(t, d, "do the dishes").Say)
	if strings.Contains(got, "yes,") || strings.Contains(got, "no,") {
		t.Errorf("a bare imperative should not be answered as a yes/no question: %q", got)
	}
}
