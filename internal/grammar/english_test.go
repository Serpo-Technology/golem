package grammar

import (
	"strings"
	"testing"

	"golem/internal/limbic"
)

// A clause rises by its first rune, so one a tongue opens with an accented
// letter capitalizes too ("üzüm bir şeydir" -> "Üzüm bir şeydir"), not only an
// ASCII one; a clause already ending in terminal punctuation keeps it.
func TestSentenceCapitalizesByRune(t *testing.T) {
	if got := sentence("üzüm bir şeydir.", ""); got != "Üzüm bir şeydir." {
		t.Fatalf("an accented leading letter should rise: %q", got)
	}
	if got := sentence("the dog ran", "."); got != "The dog ran." {
		t.Fatalf("an ASCII clause should rise and take the mark: %q", got)
	}
}

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

// Language detection reads the seed: a third verb-final language, defined
// only as data, is detected from its closed class and parsed through the SOV
// branch with no new code, the parser inferring the tongue from a marker in
// the grammar rather than a hardcoded fork.
func TestParseDetectsSeededLanguage(t *testing.T) {
	extra := append(Curriculum(),
		Param{Key: "order:xx", Value: "sov", Lang: "xx", Weight: 1},
		Param{Key: "fw:xx:zog", Value: "zog", Lang: "xx", Category: catWh, Weight: 1},
	)
	p := Compile(extra, nil)
	f := p.Parse("zog beta gamma", nil)
	if f.Lang != "xx" {
		t.Fatalf("a seeded third language should be detected and parsed as itself: %+v", f)
	}
	if f.Force != ForceQuestion {
		t.Fatalf("its seeded wh marker should open a question: %+v", f)
	}
	// English still reads as English, the default tongue.
	if g := p.Parse("what follows make test", nil); g.Lang == "xx" {
		t.Fatalf("an English utterance must not be taken for the third language: %+v", g)
	}
}

// A temporal question fills the qualifier and its anchor, with the
// grammatical filler stripped from the anchor span.
func TestParseFillsTemporalAnchor(t *testing.T) {
	f := Parse("what usually happens after the make test", nil)
	rel, anchor := f.FirstQual()
	if rel != "after" {
		t.Fatalf("the relator should be read, got %q", rel)
	}
	if anchor.Phrase != "make test" {
		t.Fatalf("the anchor span should drop the determiner, got %q", anchor.Phrase)
	}
	if f.Action.Phrase != "follow" {
		t.Fatalf("a happen-verb should fill the action, got %q", f.Action.Phrase)
	}
	if !f.Habit {
		t.Fatal("the habit adverb should be read")
	}

	grel, ganchor := Parse("what comes before make deploy", nil).FirstQual()
	if grel != "before" || ganchor.Phrase != "make deploy" {
		t.Fatalf("before-questions parse the same way, got %q %q", grel, ganchor.Phrase)
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
		Quals:  []Qualifier{{Rel: "after", Anchor: Term{Phrase: "make test"}}},
	}
	calm := limbic.State{Oxytocin: limbic.OxytocinBaseline}
	if got := Realize(f, calm); got != "After make test, make deploy usually follows." {
		t.Fatalf("assertive form wrong: %q", got)
	}
	corrected := limbic.State{Oxytocin: 0.2}
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
	calm := limbic.State{Oxytocin: limbic.OxytocinBaseline}
	if got := Realize(f, calm); got != "Make test usually precedes make deploy." {
		t.Fatalf("precede form wrong: %q", got)
	}
}

// Ignorance names exactly what is missing, and curiosity is a
// grammatical question.
func TestRealizeIgnoranceAndCuriosity(t *testing.T) {
	calm := limbic.State{Oxytocin: limbic.OxytocinBaseline}
	ign := Frame{Force: ForceStatement, Action: Term{Phrase: "know"}, Negate: true,
		Quals: []Qualifier{{Rel: "after", Anchor: Term{Phrase: "make test"}}}}
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
	calm := limbic.State{Oxytocin: limbic.OxytocinBaseline}
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
	calm := limbic.State{Oxytocin: limbic.OxytocinBaseline}
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

// A negator denies the predicate rather than asserting its opposite: the
// inversion where "never follows" used to read as "usually follows".
func TestNegationDoesNotInvert(t *testing.T) {
	calm := limbic.State{Oxytocin: limbic.OxytocinBaseline}
	f := Parse("make deploy never follows make test", nil)
	if !f.Negate {
		t.Fatalf("a negator should deny the predicate: %+v", f)
	}
	if f.Habit {
		t.Fatalf("'never' must not read as a habit adverb: %+v", f)
	}
	if got := Realize(f, calm); got != "Make deploy does not follow make test." {
		t.Fatalf("a denial must not come out as its opposite: %q", got)
	}
	// Do-support negation parses the base verb too, so "does not follow"
	// fills the action rather than scattering into the noun phrase.
	g := Parse("make test does not follow make deploy", nil)
	if !g.Negate || g.Action.Phrase != "follow" {
		t.Fatalf("do-support negation should parse to a denied follow: %+v", g)
	}
}

// Each relator opens its own qualifier, so a second temporal clause
// stops folding into the first anchor.
func TestParseTwoQualifiers(t *testing.T) {
	f := Parse("what happens after make test and before make deploy", nil)
	if len(f.Quals) != 2 {
		t.Fatalf("two relators should make two qualifiers: %+v", f.Quals)
	}
	if f.Quals[0].Rel != "after" || f.Quals[0].Anchor.Phrase != "make test" {
		t.Fatalf("first qualifier wrong: %+v", f.Quals[0])
	}
	if f.Quals[1].Rel != "before" || f.Quals[1].Anchor.Phrase != "make deploy" {
		t.Fatalf("second qualifier wrong: %+v", f.Quals[1])
	}
}

// The interrogative is typed, the sortal narrows the kind of answer,
// and a post-verb span is the object even with no relator anywhere.
func TestParseWhTypingAndSortal(t *testing.T) {
	f := Parse("which command follows make test", nil)
	if f.Wh != "which" || f.Sort != "command" || f.SortPl {
		t.Fatalf("wh and sortal should be read: %+v", f)
	}
	if f.Action.Phrase != "follow" || f.Patient.Phrase != "make test" {
		t.Fatalf("the verb-carried relation should leave the anchor in the patient: %+v", f)
	}
	g := Parse("which commands follow make test", nil)
	if !g.SortPl {
		t.Fatalf("a plural sortal should be marked: %+v", g)
	}
	h := Parse("when did make test happen", nil)
	if h.Wh != "when" || !h.Past || h.Agent.Phrase != "make test" {
		t.Fatalf("when-questions read tense and subject: %+v", h)
	}
	c := Parse("how many times did make test happen", nil)
	if c.Wh != "how" || !c.Counting || !c.Past {
		t.Fatalf("how-many should count: %+v", c)
	}
	o := Parse("how often does make test happen", nil)
	if !o.Counting {
		t.Fatalf("how-often should count: %+v", o)
	}
}

// Two qualifiers front the clause coordinated, with a neutral verb.
func TestRealizeTwoQualifiers(t *testing.T) {
	calm := limbic.State{Oxytocin: limbic.OxytocinBaseline}
	f := Frame{
		Force:  ForceStatement,
		Agent:  Term{Phrase: "make deploy"},
		Action: Term{Phrase: "happen"},
		Habit:  true,
		Quals: []Qualifier{
			{Rel: "after", Anchor: Term{Phrase: "make test"}},
			{Rel: "before", Anchor: Term{Phrase: "make push"}},
		},
	}
	if got := Realize(f, calm); got != "After make test and before make push, make deploy usually happens." {
		t.Fatalf("coordinated qualifiers wrong: %q", got)
	}
	ign := Frame{Force: ForceStatement, Action: Term{Phrase: "know"}, Negate: true,
		Quals: []Qualifier{
			{Rel: "after", Anchor: Term{Phrase: "make test"}},
			{Rel: "before", Anchor: Term{Phrase: "make push"}},
		}}
	if got := Realize(ign, calm); got != "I do not know what follows make test and what precedes make push yet." {
		t.Fatalf("coordinated ignorance wrong: %q", got)
	}
}

// A time or place complement and the "last" adverb realize cleanly.
func TestRealizeWhenceAnswers(t *testing.T) {
	calm := limbic.State{Oxytocin: limbic.OxytocinBaseline}
	when := Frame{Force: ForceStatement, Agent: Term{Phrase: "make test"},
		Action: Term{Phrase: "happen"}, Past: true, Last: true, At: "Jun 11 at 09:14"}
	if got := Realize(when, calm); got != "Make test last happened on Jun 11 at 09:14." {
		t.Fatalf("when-answer wrong: %q", got)
	}
	where := Frame{Force: ForceStatement, Agent: Term{Phrase: "make test"},
		Action: Term{Phrase: "happen"}, Past: true, At: "the shell channel"}
	if got := Realize(where, calm); got != "Make test happened on the shell channel." {
		t.Fatalf("where-answer wrong: %q", got)
	}
}

// A property predicated of a relation parses to the property and its subject,
// not a fact: "outrank is transitive" reads f.Property and leaves the relation as
// the subject, and the inverted "is outrank transitive" asks it. This is the
// hook a learned relation-property routes off, the way a speech-act verb routes
// a discourse move.
// refinedBuiltin compiles the seed with the colon-refined categories the daemon
// derives from the graph in a real run; the refinement lives in the graph, not the
// seed, so a pure-package parser test supplies it here to exercise the parser's
// handling of a refined category.
func refinedBuiltin() *Params {
	refined := append([]Param{}, Curriculum()...)
	refined = append(refined,
		Param{Key: "fw:en:transitive", Value: "transitive", Lang: "en", Category: "property:transitive", Weight: 1},
		Param{Key: "fw:tr:sonra", Value: "sonra", Lang: "tr", Category: "rel:after", Weight: 1},
		Param{Key: "fw:tr:önce", Value: "önce", Lang: "tr", Category: "rel:before", Weight: 1},
	)
	return Compile(refined, nil).SetConstructions(Constructions()).SetDecl(Curriculum()).SetArgConstructions(Constructions())
}

func TestPropertyAssertionParses(t *testing.T) {
	p := refinedBuiltin()
	f := p.Parse("outrank is transitive", nil)
	if f.Property != "transitive" {
		t.Fatalf("the predicate should be read as the property: %q", f.Property)
	}
	if f.Agent.Phrase != "outrank" {
		t.Fatalf("the subject should be the relation: %q", f.Agent.Phrase)
	}
	if f.Force != ForceStatement {
		t.Fatalf("a plain property assertion states: %v", f.Force)
	}
	q := p.Parse("is outrank transitive", nil)
	if q.Property != "transitive" || q.Force != ForceQuestion {
		t.Fatalf("an inverted property clause asks: property=%q force=%v agent=%q", q.Property, q.Force, q.Agent.Phrase)
	}
}
