package daemon

import (
	"strings"
	"testing"

	"golem/internal/memory"
)

// A statement comprehension read as a proposition and then abandoned (a
// misparse) is inert: its words lay no naming company and bind no attention,
// so a block of abstract self-knowledge tellings cannot cross-name its words
// onto whatever perceptless concept happened to be warm. This replays the
// traced corruption verbatim: before the guards, "golem" ended up firmly
// naming the unrelated concept "value" and answered "Golem names value."
func TestMisparsedStatementIsInert(t *testing.T) {
	d, s := testDaemon(t)
	block := []string{
		"golem is a daemon",
		"the memory is a graph",
		"a concept is a node",
		"a word labels a concept",
		"an edge carries a weight",
		"a sleep cleans the memory",
		"a run grounds a value",
		"the architect teaches golem",
		"a test checks a program",
		"a run proves a program",
		"a name shows the purpose",
	}
	for _, line := range block {
		say(t, d, line)
	}
	d.sleepCycle()

	// The misparsed words must hold no stray name onto a perceptless stranger.
	for _, w := range []string{"golem", "architect", "teaches"} {
		words, err := s.WordNodesByForms([]string{w})
		if err != nil {
			t.Fatal(err)
		}
		for _, wd := range words {
			names, _ := s.LinksFromRel([]uint64{wd.ID}, memory.RelNames)
			for _, e := range names {
				frags, _ := s.FragmentsByIDs([]uint64{e.To})
				if len(frags) == 1 && frags[0].Content == "value" {
					t.Fatalf("the misparse laundered %q into naming the stray concept value", w)
				}
			}
		}
	}
	if got := say(t, d, "is golem a daemon").Say; strings.Contains(got, "names value") {
		t.Fatalf("the corruption surface returned: %q", got)
	}
}

// Grammar scaffolding shares the graph but never the conversation: an answer
// template whose form pole contains "vp_hedge" must not resolve as the
// referent of "edge", the taught fact must mint the real noun, and the answer
// must speak the taught word rather than a raw template.
func TestScaffoldingSealedFromContent(t *testing.T) {
	d, s := testDaemon(t)
	if c := d.resolveConcept("edge"); c != nil {
		t.Fatalf("an untaught noun must not resolve to scaffolding: %q %q", c.Template, c.Content)
	}
	say(t, d, "an edge carries a weight")
	edge, err := s.ConceptByTemplate("noun:en:edge")
	if err != nil || edge == nil {
		t.Fatal("the teaching should mint the real edge concept")
	}
	got := say(t, d, "what does an edge carry").Say
	if strings.Contains(got, "{") {
		t.Fatalf("a raw template leaked into the answer: %q", got)
	}
	if !strings.Contains(got, "weight") || !strings.Contains(strings.ToLower(got), "edge") {
		t.Fatalf("the answer should speak the taught fact: %q", got)
	}
}
