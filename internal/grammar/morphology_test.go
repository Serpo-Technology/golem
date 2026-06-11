package grammar

import (
	"testing"

	"golem/internal/limbic"
)

// Agreement is computed from person, number, and tense: third singular
// present takes -s, everything else the base, and the past levels them.
func TestVerbAgreement(t *testing.T) {
	cases := []struct {
		first, plural, past bool
		want                string
	}{
		{false, false, false, "follows"},
		{true, false, false, "follow"},
		{false, true, false, "follow"},
		{false, false, true, "followed"},
		{true, false, true, "followed"},
		{false, true, true, "followed"},
	}
	for _, c := range cases {
		if got := finiteVerb("follow", c.first, c.plural, c.past); got != c.want {
			t.Fatalf("follow(first=%v plural=%v past=%v) = %q, want %q",
				c.first, c.plural, c.past, got, c.want)
		}
	}
	// Irregular principal parts come from the table.
	if finiteVerb("see", false, false, true) != "saw" || pastPart("see") != "seen" {
		t.Fatal("see should inflect irregularly")
	}
	if finiteVerb("know", true, false, true) != "knew" {
		t.Fatal("know should inflect irregularly")
	}
}

// Do-support carries agreement and tense on the auxiliary.
func TestNegationDoSupport(t *testing.T) {
	if got := negatedVerb("follow", false, false, false); got != "does not follow" {
		t.Fatalf("3sg present: %q", got)
	}
	if got := negatedVerb("follow", false, true, false); got != "do not follow" {
		t.Fatalf("plural present: %q", got)
	}
	if got := negatedVerb("follow", false, false, true); got != "did not follow" {
		t.Fatalf("past: %q", got)
	}
	if got := negatedVerb("know", true, false, false); got != "do not know" {
		t.Fatalf("1sg present: %q", got)
	}
}

// The copula keeps agreement in the past, alone among the verbs here.
func TestBeForms(t *testing.T) {
	cases := []struct {
		first, plural, past bool
		want                string
	}{
		{false, false, false, "is"},
		{false, true, false, "are"},
		{true, false, false, "am"},
		{false, false, true, "was"},
		{false, true, true, "were"},
	}
	for _, c := range cases {
		if got := beForm(c.first, c.plural, c.past); got != c.want {
			t.Fatalf("be(first=%v plural=%v past=%v) = %q, want %q",
				c.first, c.plural, c.past, got, c.want)
		}
	}
}

// Noun number runs on the rule table with the irregulars beside it.
func TestNounPlurals(t *testing.T) {
	cases := map[string]string{
		"command": "commands",
		"action":  "actions",
		"glass":   "glasses",
		"match":   "matches",
		"box":     "boxes",
		"query":   "queries",
		"day":     "days",
		"leaf":    "leaves",
		"knife":   "knives",
		"child":   "children",
		"person":  "people",
		"mouse":   "mice",
		"sheep":   "sheep",
	}
	for noun, want := range cases {
		if got := pluralEN(noun); got != want {
			t.Fatalf("plural of %q = %q, want %q", noun, got, want)
		}
	}
}

// Articles go by the following sound, never the letter: silent h takes
// an, glided u takes a.
func TestIndefiniteArticles(t *testing.T) {
	cases := map[string]string{
		"action":         "an action",
		"event":          "an event",
		"thing":          "a thing",
		"command":        "a command",
		"hour":           "an hour",
		"honest mistake": "an honest mistake",
		"user":           "a user",
		"unique shape":   "a unique shape",
		"one-off":        "a one-off",
	}
	for noun, want := range cases {
		if got := Indefinite(noun); got != want {
			t.Fatalf("Indefinite(%q) = %q, want %q", noun, got, want)
		}
	}
}

func TestNumberWordsAndCoordination(t *testing.T) {
	if numberWord(2) != "two" || numberWord(12) != "twelve" || numberWord(13) != "13" {
		t.Fatal("small counts speak as words, large ones as digits")
	}
	if JoinAnd([]string{"a"}) != "a" {
		t.Fatal("single item stands alone")
	}
	if JoinAnd([]string{"a", "b"}) != "a and b" {
		t.Fatal("pair coordinates without a comma")
	}
	if JoinAnd([]string{"a", "b", "c"}) != "a, b, and c" {
		t.Fatal("triple coordinates with commas")
	}
}

// The parser reads tense off the verb form and the auxiliaries, and the
// realizer speaks it back, so "followed" and "follows" stay distinct
// through the round trip.
func TestTenseParsesAndRealizes(t *testing.T) {
	f := Parse("what happened after make test", nil)
	if !f.Past {
		t.Fatalf("a past verb form should set tense: %+v", f)
	}
	g := Parse("what usually happens after make test", nil)
	if g.Past {
		t.Fatalf("a present verb form must not set tense: %+v", g)
	}
	h := Parse("did make deploy follow make test?", nil)
	if !h.Past || h.Force != ForceQuestion {
		t.Fatalf("do-support should carry tense into the question: %+v", h)
	}
	w := Parse("make test was green", nil)
	if !w.Past || w.Action.Phrase != "be" {
		t.Fatalf("a past copula should read as past be: %+v", w)
	}

	calm := limbic.State{Oxytocin: limbic.OxytocinBaseline}
	past := Frame{
		Force:  ForceStatement,
		Agent:  Term{Phrase: "make deploy"},
		Action: Term{Phrase: "follow"},
		Habit:  true,
		Past:   true,
		Quals:  []Qualifier{{Rel: "after", Anchor: Term{Phrase: "make test"}}},
	}
	if got := Realize(past, calm); got != "After make test, make deploy usually followed." {
		t.Fatalf("past habitual form wrong: %q", got)
	}
	denied := Frame{
		Force:   ForceStatement,
		Agent:   Term{Phrase: "make deploy"},
		Action:  Term{Phrase: "follow"},
		Negate:  true,
		Past:    true,
		Patient: Term{Phrase: "make test"},
	}
	if got := Realize(denied, calm); got != "Make deploy did not follow make test." {
		t.Fatalf("past denial form wrong: %q", got)
	}
	hedgedPast := Frame{
		Force:  ForceStatement,
		Agent:  Term{Phrase: "make deploy"},
		Action: Term{Phrase: "follow"},
		Past:   true,
		Quals:  []Qualifier{{Rel: "after", Anchor: Term{Phrase: "make test"}}},
	}
	if got := Realize(hedgedPast, limbic.State{Oxytocin: 0.2}); got != "After make test, make deploy may have followed." {
		t.Fatalf("hedged past form wrong: %q", got)
	}
}

// A plural subject agrees: coordination hands the realizer a plural
// term and the -s goes away.
func TestPluralSubjectAgrees(t *testing.T) {
	calm := limbic.State{Oxytocin: limbic.OxytocinBaseline}
	f := Frame{
		Force:  ForceStatement,
		Agent:  Term{Phrase: "make deploy and make lint", Plural: true},
		Action: Term{Phrase: "follow"},
		Habit:  true,
		Quals:  []Qualifier{{Rel: "after", Anchor: Term{Phrase: "make test"}}},
	}
	if got := Realize(f, calm); got != "After make test, make deploy and make lint usually follow." {
		t.Fatalf("plural agreement wrong: %q", got)
	}
}
