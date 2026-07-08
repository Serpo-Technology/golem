package daemon

import (
	"strings"
	"testing"
)

// A symmetric relation is taught in language and then reasoned with: told "match is symmetric",
// GOLEM answers that the dog matched the cat because the cat matched the dog, an inference it had no
// built-in handler for. The reversed fact is read by running the compositional symmetric rule
// against the graph, so the new reasoning rule is a taught data artifact, not new Go. The
// property reads back, and an ordinary relation does not symmetrize.
func TestSymmetricRelationTaughtAndReasoned(t *testing.T) {
	d, _ := testDaemon(t)
	groundNoun(t, d, "cat")
	groundNoun(t, d, "dog")
	groundNoun(t, d, "mouse")

	say(t, d, "the cat matched the dog")
	if got := say(t, d, "match is symmetric").Say; !strings.Contains(strings.ToLower(got), "symmetric") {
		t.Fatalf("teaching the property should confirm it: %q", got)
	}
	if got := say(t, d, "is match symmetric").Say; !strings.HasPrefix(got, "Yes") {
		t.Fatalf("the taught property should read back yes: %q", got)
	}
	// The new capability: the reversed fact holds by symmetry, though never told directly.
	if got := say(t, d, "did the dog match the cat").Say; !strings.HasPrefix(got, "Yes") {
		t.Fatalf("a symmetric relation should hold both ways: %q", got)
	}
	if d.symmetricRuleID == 0 {
		t.Fatal("the symmetric query should have ensured and cached the compositional rule")
	}

	// An ordinary relation does not symmetrize: the mouse did not chase the cat.
	say(t, d, "the cat chased the mouse")
	if got := say(t, d, "did the mouse chase the cat").Say; strings.HasPrefix(got, "Yes") {
		t.Fatalf("an unmarked relation should not hold both ways: %q", got)
	}
}
