package main

import (
	"strings"
	"testing"
)

// The parser reads illocutionary force from the innate grammar: a
// wh-opener asks, an imperative opener commands, a bare clause states,
// and a question mark asks even without a wh-word.
func TestParseReadsForce(t *testing.T) {
	cases := []struct {
		in   string
		want Force
	}{
		{"what usually happens after make test", ForceQuestion},
		{"run the tests", ForceCommand},
		{"do it again", ForceCommand},
		{"the disk filled up overnight", ForceStatement},
		{"make deploy follows make test?", ForceQuestion},
	}
	for _, c := range cases {
		if got := Parse(c.in, nil).Force; got != c.want {
			t.Fatalf("%q should read as %s, got %s", c.in, c.want, got)
		}
	}
}

// A temporal question fills the qualifier and its anchor, with the
// grammatical filler stripped from the anchor span.
func TestParseFillsTemporalAnchor(t *testing.T) {
	f := Parse("what usually happens after the make test", nil)
	if f.Rel != "after" {
		t.Fatalf("the relator should be read, got %q", f.Rel)
	}
	if f.Anchor.Phrase != "make test" {
		t.Fatalf("the anchor span should drop the determiner, got %q", f.Anchor.Phrase)
	}
	if f.Action.Phrase != "follow" {
		t.Fatalf("a happen-verb should fill the action, got %q", f.Action.Phrase)
	}
	if !f.Habit {
		t.Fatal("the habit adverb should be read")
	}

	g := Parse("what comes before make deploy", nil)
	if g.Rel != "before" || g.Anchor.Phrase != "make deploy" {
		t.Fatalf("before-questions parse the same way, got %q %q", g.Rel, g.Anchor.Phrase)
	}
}

// Pro-forms and the repetition word mark slots for the workspace to
// resolve; content spans land in the patient.
func TestParseProFormsAndPatient(t *testing.T) {
	f := Parse("do it again", nil)
	if !f.Pro || !f.Again {
		t.Fatalf("pro-form and repetition should be marked: %+v", f)
	}
	g := Parse("run the tests", nil)
	if g.Patient.Phrase != "tests" {
		t.Fatalf("the imperative's object lands in the patient, got %q", g.Patient.Phrase)
	}
}

// Words that resolve to nothing are kept, so the planner can admit
// ignorance instead of bluffing.
func TestParseCollectsUnknowns(t *testing.T) {
	f := Parse("what is flibber", nil)
	if f.Action.Phrase != "be" {
		t.Fatalf("a copula question should read as definition, got %q", f.Action.Phrase)
	}
	if len(f.Unknown) != 1 || f.Unknown[0] != "flibber" {
		t.Fatalf("the unresolved word should be kept: %+v", f.Unknown)
	}
}

// The realizer builds the successor answer as a clause, asserts at
// baseline trust, and hedges when trust has been corrected away.
func TestRealizeSuccessorAnswer(t *testing.T) {
	f := Frame{
		Force:  ForceStatement,
		Agent:  Term{Phrase: "make deploy"},
		Action: Term{Phrase: "follow"},
		Habit:  true,
		Rel:    "after",
		Anchor: Term{Phrase: "make test"},
	}
	calm := LimbicState{Oxytocin: oxytocinBaseline}
	if got := Realize(f, calm); got != "After make test, make deploy usually follows." {
		t.Fatalf("assertive form wrong: %q", got)
	}
	corrected := LimbicState{Oxytocin: 0.2}
	if got := Realize(f, corrected); got != "After make test, make deploy may follow." {
		t.Fatalf("hedged form wrong: %q", got)
	}
}

// Precedence answers read subject-first, since the anchor is the later
// event.
func TestRealizePrecedeAnswer(t *testing.T) {
	f := Frame{
		Force:   ForceStatement,
		Agent:   Term{Phrase: "make test"},
		Action:  Term{Phrase: "precede"},
		Habit:   true,
		Patient: Term{Phrase: "make deploy"},
	}
	calm := LimbicState{Oxytocin: oxytocinBaseline}
	if got := Realize(f, calm); got != "Make test usually precedes make deploy." {
		t.Fatalf("precede form wrong: %q", got)
	}
}

// Ignorance names exactly what is missing, and curiosity is a
// grammatical question.
func TestRealizeIgnoranceAndCuriosity(t *testing.T) {
	calm := LimbicState{Oxytocin: oxytocinBaseline}
	ign := Frame{Force: ForceStatement, Action: Term{Phrase: "know"}, Negate: true,
		Rel: "after", Anchor: Term{Phrase: "make test"}}
	if got := Realize(ign, calm); got != "I do not know what follows make test yet." {
		t.Fatalf("ignorance form wrong: %q", got)
	}
	word := Frame{Force: ForceStatement, Action: Term{Phrase: "know"}, Negate: true,
		Patient: Term{Phrase: "flibber"}}
	if got := Realize(word, calm); got != "I do not know flibber yet." {
		t.Fatalf("word ignorance wrong: %q", got)
	}
	ask := Frame{Force: ForceQuestion, Action: Term{Phrase: "be"}, Patient: Term{Phrase: "flibber"}}
	if got := Realize(ask, calm); got != "What is flibber?" {
		t.Fatalf("curiosity form wrong: %q", got)
	}
}

// Occurrence counts come out as words where numerals would be wrong.
func TestRealizeTimes(t *testing.T) {
	calm := LimbicState{Oxytocin: oxytocinBaseline}
	for n, want := range map[int]string{
		1: "I have seen it once.",
		2: "I have seen it twice.",
		7: "I have seen it 7 times.",
	} {
		f := Frame{Force: ForceStatement, Action: Term{Phrase: "see"},
			Patient: Term{Phrase: "it"}, Times: n}
		if got := Realize(f, calm); got != want {
			t.Fatalf("times %d: got %q, want %q", n, got, want)
		}
	}
}

// A proposal sentence carries the command verbatim, with no second
// terminal mark when the command already ends in one.
func TestRealizeProposal(t *testing.T) {
	calm := LimbicState{Oxytocin: oxytocinBaseline}
	f := Frame{Force: ForceStatement, Action: Term{Phrase: "propose"},
		Patient: Term{Phrase: "go test ./..."}}
	got := Realize(f, calm)
	if got != "I propose go test ./..." {
		t.Fatalf("proposal form wrong: %q", got)
	}
	if strings.HasSuffix(got, "....") {
		t.Fatalf("no doubled terminal mark: %q", got)
	}
}
