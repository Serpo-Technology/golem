package daemon

import (
	"testing"
)

// The gradable-comparison machinery is seeded as graph concepts at boot: the comparative
// operator, the greater and lesser pole roles, and the rule, regular, irregular, and
// exception meta concepts. They are class concepts, so they sit outside the grown layer
// and are kept out of cognition, and they are seeded idempotently, so a re-boot or a
// recompile finds them rather than forking a duplicate.
func TestComparativeScaffoldSeeded(t *testing.T) {
	d, s := testDaemon(t)

	want := map[string]string{
		"cmp:comparative": "comparative",
		"pole:greater":    "greater",
		"pole:lesser":     "lesser",
		"meta:rule":       "rule",
		"meta:regular":    "regular",
		"meta:irregular":  "irregular",
		"meta:exception":  "exception",
	}

	if len(d.gradableScaffold) != len(want) {
		t.Fatalf("the comparative scaffold should seed %d concepts, got %d", len(want), len(d.gradableScaffold))
	}

	for tmpl, content := range want {
		c, err := s.ConceptByTemplate(tmpl)
		if err != nil || c == nil {
			t.Fatalf("scaffold concept %q should be seeded: %v", tmpl, err)
		}
		if c.Content != content {
			t.Errorf("%q should have content %q, got %q", tmpl, content, c.Content)
		}
		if c.Category != "class" {
			t.Errorf("%q should be a class concept, got category %q", tmpl, c.Category)
		}
		if !d.isSystemConcept(c.ID) {
			t.Errorf("%q should be a system concept, kept out of cognition", tmpl)
		}
		if d.gradableScaffold[c.ID] != content {
			t.Errorf("%q should be recorded in the scaffold map as %q", tmpl, content)
		}
	}

	// Seeding is idempotent: a second pass finds the concepts rather than minting
	// duplicates, so a re-boot or a recompile does not fork the scaffold.
	before := len(d.gradableScaffold)
	if err := d.ensureComparativeScaffold(); err != nil {
		t.Fatal(err)
	}
	if len(d.gradableScaffold) != before {
		t.Fatalf("re-seeding should be idempotent: had %d, now %d", before, len(d.gradableScaffold))
	}
}
