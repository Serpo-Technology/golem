package daemon

import (
	"strings"
	"testing"

	"golem/internal/memory"
	"golem/wire"
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

// The self carries a told identity and speaks it in its own person: "you are
// a daemon" is held defeasibly on the participant, read back hedged as
// "Probably, I am a daemon", and a denial of an untold identity comes back in
// the first person too, never as the thing-noun surface "an I".
func TestSelfIdentityToldAndSpokenFirstPerson(t *testing.T) {
	d, _ := testDaemon(t)
	ack := say(t, d, "you are a daemon").Say
	if !strings.Contains(ack, "I am a daemon") {
		t.Fatalf("the told identity should be acknowledged in the first person: %q", ack)
	}
	got := say(t, d, "are you a daemon").Say
	if !strings.Contains(got, "Probably") || !strings.Contains(got, "I am a daemon") {
		t.Fatalf("a told identity reads back hedged and first-person: %q", got)
	}
	say(t, d, "you run a program") // mints the noun the denial is asked over
	no := say(t, d, "are you a program").Say
	if strings.Contains(no, "an I") || !strings.Contains(no, "I am not") {
		t.Fatalf("an untold identity denies in the first person: %q", no)
	}
}

// A deictic role question whose verb surface is closed-class ("run" is a
// seeded imperative) still finds its learned relation, so the answer is the
// first-person fact, never a definition lead about the pronoun.
func TestDeicticRoleQuestionSpeaksFirstPerson(t *testing.T) {
	d, _ := testDaemon(t)
	say(t, d, "you run a program")
	got := say(t, d, "what do you run").Say
	if strings.Contains(got, "names") {
		t.Fatalf("a role question must not open with a definition of the pronoun: %q", got)
	}
	if !strings.Contains(got, "I r") || !strings.Contains(got, "program") {
		t.Fatalf("the answer should be the first-person fact: %q", got)
	}
}

// The retune entry keeps its gates: with the runtime shut or no source path,
// the pipeline refuses and says why, and a malformed ask is refused outright.
func TestRetuneEntryGated(t *testing.T) {
	d, _ := testDaemon(t)
	resp := d.handleRetune(wire.Request{Query: "internal/daemon/act.go", Arg: "opWeightCeil", Content: "2.5"})
	if !strings.Contains(resp.Say, "closed") {
		t.Fatalf("a shut gate should refuse with the reason: %q", resp.Say)
	}
	resp = d.handleRetune(wire.Request{Query: "", Arg: "x", Content: "1"})
	if resp.Error == "" {
		t.Fatal("a malformed retune should be refused")
	}
}
