package grammar

import (
	"testing"

	"golem/internal/limbic"
)

// RealizeDecl reproduces the coded Realize for the copula statement, so the declarative engine
// can shadow the frame realizer with no divergence. The agent and complement ride verbatim
// (they arrive pre-rendered); the copula form (is / are / was / were, the Turkish -DIr) and the
// clause order come from grammar data.
func TestRealizeDeclMatchesRealize(t *testing.T) {
	p := Builtin()
	var chem limbic.State
	be := func(f Frame) Frame { f.Action = Term{Phrase: "be"}; f.Force = ForceStatement; return f }
	cases := []struct {
		name string
		f    Frame
		want string
	}{
		{"present singular", be(Frame{Agent: Term{Phrase: "the dog"}, Patient: Term{Phrase: Indefinite("animal")}}), "The dog is an animal."},
		{"past singular", be(Frame{Agent: Term{Phrase: "the dog"}, Patient: Term{Phrase: Indefinite("animal")}, Past: true}), "The dog was an animal."},
		{"present plural", be(Frame{Agent: Term{Phrase: "the dogs", Plural: true}, Patient: Term{Phrase: Indefinite("animal")}}), "The dogs are an animal."},
		{"past plural", be(Frame{Agent: Term{Phrase: "the dogs", Plural: true}, Patient: Term{Phrase: Indefinite("animal")}, Past: true}), "The dogs were an animal."},
		{"a or an rides the phrase", be(Frame{Agent: Term{Phrase: "make test"}, Patient: Term{Phrase: Indefinite("action")}}), "Make test is an action."},
		{"negated", be(Frame{Agent: Term{Phrase: "the lion"}, Patient: Term{Phrase: "bigger than the mouse"}, Negate: true}), "The lion is not bigger than the mouse."},
		{"turkish", be(Frame{Lang: "tr", Agent: Term{Phrase: "make test"}, Patient: Term{Phrase: "bir eylem"}}), "Make test bir eylemdir."},
		{"turkish suffixes the last word", be(Frame{Lang: "tr", Agent: Term{Phrase: "köpek"}, Patient: Term{Phrase: "bir şey"}}), "Köpek bir şeydir."},
		{"english question", Frame{Action: Term{Phrase: "be"}, Force: ForceQuestion, Patient: Term{Phrase: "flibber"}}, "What is flibber?"},
		{"turkish question", Frame{Action: Term{Phrase: "be"}, Force: ForceQuestion, Lang: "tr", Patient: Term{Phrase: "make test"}}, "Make test nedir?"},
	}
	for _, c := range cases {
		got, ok := p.RealizeDecl(c.f, chem)
		want := p.Realize(c.f, chem)
		if !ok {
			t.Fatalf("%s: RealizeDecl declined a clause it should speak", c.name)
		}
		if got != want || got != c.want {
			t.Fatalf("%s: RealizeDecl %q should match Realize %q (want %q)", c.name, got, want, c.want)
		}
	}
}

// RealizeDecl reproduces the coded Realize for the plain event clause, the verb's form coming
// from the same morphology the proposition clause uses while the agent and patient ride
// verbatim. Agreement follows finiteVerb: the first person and the plural take the base form.
func TestRealizeDeclEventVerbs(t *testing.T) {
	p := Builtin()
	chem := limbic.State{Oxytocin: limbic.OxytocinBaseline} // at baseline, not hedged
	ev := func(f Frame) Frame { f.Force = ForceStatement; return f }
	cases := []struct {
		name string
		f    Frame
		want string
	}{
		{"transitive present", ev(Frame{Action: Term{Phrase: "follow"}, Agent: Term{Phrase: "the dog"}, Patient: Term{Phrase: "the cat"}}), "The dog follows the cat."},
		{"transitive past", ev(Frame{Action: Term{Phrase: "follow"}, Agent: Term{Phrase: "the dog"}, Patient: Term{Phrase: "the cat"}, Past: true}), "The dog followed the cat."},
		{"plural subject", ev(Frame{Action: Term{Phrase: "follow"}, Agent: Term{Phrase: "the dogs", Plural: true}, Patient: Term{Phrase: "the cat"}}), "The dogs follow the cat."},
		{"first person", ev(Frame{Action: Term{Phrase: "follow"}, Agent: Term{Phrase: "I"}, Patient: Term{Phrase: "the cat"}}), "I follow the cat."},
		{"intransitive", ev(Frame{Action: Term{Phrase: "happen"}, Agent: Term{Phrase: "make test"}}), "Make test happens."},
		{"precede past", ev(Frame{Action: Term{Phrase: "precede"}, Agent: Term{Phrase: "make test"}, Patient: Term{Phrase: "make deploy"}, Past: true}), "Make test preceded make deploy."},
		{"habitual", ev(Frame{Action: Term{Phrase: "follow"}, Agent: Term{Phrase: "the dog"}, Patient: Term{Phrase: "the cat"}, Habit: true}), "The dog usually follows the cat."},
		{"habitual past", ev(Frame{Action: Term{Phrase: "follow"}, Agent: Term{Phrase: "the dog"}, Patient: Term{Phrase: "the cat"}, Habit: true, Past: true}), "The dog usually followed the cat."},
		{"habitual intransitive", ev(Frame{Action: Term{Phrase: "happen"}, Agent: Term{Phrase: "make test"}, Habit: true}), "Make test usually happens."},
		{"last occurrence", ev(Frame{Action: Term{Phrase: "follow"}, Agent: Term{Phrase: "the dog"}, Patient: Term{Phrase: "the cat"}, Last: true, Past: true}), "The dog last followed the cat."},
		{"negated present", ev(Frame{Action: Term{Phrase: "follow"}, Agent: Term{Phrase: "the dog"}, Patient: Term{Phrase: "the cat"}, Negate: true}), "The dog does not follow the cat."},
		{"negated plural", ev(Frame{Action: Term{Phrase: "follow"}, Agent: Term{Phrase: "the dogs", Plural: true}, Patient: Term{Phrase: "the cat"}, Negate: true}), "The dogs do not follow the cat."},
		{"negated past", ev(Frame{Action: Term{Phrase: "follow"}, Agent: Term{Phrase: "the dog"}, Patient: Term{Phrase: "the cat"}, Negate: true, Past: true}), "The dog did not follow the cat."},
		{"negated intransitive", ev(Frame{Action: Term{Phrase: "happen"}, Agent: Term{Phrase: "make test"}, Negate: true}), "Make test does not happen."},
		{"fronted adjunct", ev(Frame{Action: Term{Phrase: "follow"}, Agent: Term{Phrase: "make deploy"}, Quals: []Qualifier{{Rel: "after", Anchor: Term{Phrase: "make test"}}}}), "After make test, make deploy follows."},
		{"adjunct with habit", ev(Frame{Action: Term{Phrase: "happen"}, Agent: Term{Phrase: "make deploy"}, Habit: true, Quals: []Qualifier{{Rel: "after", Anchor: Term{Phrase: "make test"}}}}), "After make test, make deploy usually happens."},
		{"two adjuncts", ev(Frame{Action: Term{Phrase: "happen"}, Agent: Term{Phrase: "make deploy"}, Quals: []Qualifier{{Rel: "after", Anchor: Term{Phrase: "make test"}}, {Rel: "before", Anchor: Term{Phrase: "make ship"}}}}), "After make test and before make ship, make deploy happens."},
		{"hedged present", ev(Frame{Action: Term{Phrase: "follow"}, Agent: Term{Phrase: "the dog"}, Patient: Term{Phrase: "the cat"}, Hedge: true}), "The dog may follow the cat."},
		{"hedged past", ev(Frame{Action: Term{Phrase: "follow"}, Agent: Term{Phrase: "the dog"}, Patient: Term{Phrase: "the cat"}, Hedge: true, Past: true}), "The dog may have followed the cat."},
		{"hedged intransitive", ev(Frame{Action: Term{Phrase: "happen"}, Agent: Term{Phrase: "make test"}, Hedge: true}), "Make test may happen."},
	}
	for _, c := range cases {
		got, ok := p.RealizeDecl(c.f, chem)
		want := p.Realize(c.f, chem)
		if !ok {
			t.Fatalf("%s: RealizeDecl declined a clause it should speak", c.name)
		}
		if got != want || got != c.want {
			t.Fatalf("%s: RealizeDecl %q should match Realize %q (want %q)", c.name, got, want, c.want)
		}
	}
}

// The Turkish event clause speaks the suppletive verb from its seeded paradigm, verb-final, with
// the past person suffix, matching realizeTurkish across present, past, person, negation, hedge.
func TestRealizeDeclTurkishEvent(t *testing.T) {
	p := Builtin()
	chem := limbic.State{Oxytocin: limbic.OxytocinBaseline}
	ev := func(f Frame) Frame {
		f.Action = Term{Phrase: "follow"}
		f.Force = ForceStatement
		f.Lang = "tr"
		return f
	}
	cases := []struct {
		name string
		f    Frame
		want string
	}{
		{"present", ev(Frame{Agent: Term{Phrase: "make deploy"}}), "Make deploy gelir."},
		{"past", ev(Frame{Agent: Term{Phrase: "make deploy"}, Past: true}), "Make deploy geldi."},
		{"past first person", ev(Frame{Agent: Term{Phrase: "ben"}, Past: true}), "Ben geldim."},
		{"negative present", ev(Frame{Agent: Term{Phrase: "make deploy"}, Negate: true}), "Make deploy gelmez."},
		{"hedged", ev(Frame{Agent: Term{Phrase: "make deploy"}, Hedge: true}), "Make deploy gelebilir."},
	}
	for _, c := range cases {
		got, ok := p.RealizeDecl(c.f, chem)
		want := p.Realize(c.f, chem)
		if !ok || got != want || got != c.want {
			t.Fatalf("%s: RealizeDecl %q should match Realize %q (want %q)", c.name, got, want, c.want)
		}
	}
}

// The Turkish event clause declines a verb with no seeded paradigm and the fronting adverbs and
// adjuncts, each a later stage, so they fall back to the coded path.
func TestRealizeDeclTurkishEventDeclines(t *testing.T) {
	p := Builtin()
	base := limbic.State{Oxytocin: limbic.OxytocinBaseline}
	cases := []Frame{
		{Action: Term{Phrase: "run"}, Force: ForceStatement, Lang: "tr", Agent: Term{Phrase: "make test"}},
		{Action: Term{Phrase: "follow"}, Force: ForceStatement, Lang: "tr", Agent: Term{Phrase: "make deploy"}, Habit: true},
		{Action: Term{Phrase: "follow"}, Force: ForceStatement, Lang: "tr", Agent: Term{Phrase: "make deploy"}, Quals: []Qualifier{{Rel: "after", Anchor: Term{Phrase: "make test"}}}},
	}
	for i, f := range cases {
		if got, ok := p.RealizeDecl(f, base); ok {
			t.Fatalf("case %d: RealizeDecl should decline, got %q", i, got)
		}
	}
}

// A corrected system (low oxytocin) hedges an affirmative event clause even without the hedge
// flag, the way Realize does, and the declarative engine matches it.
func TestRealizeDeclEventLowOxytocinHedges(t *testing.T) {
	p := Builtin()
	low := limbic.State{Oxytocin: 0}
	f := Frame{Action: Term{Phrase: "follow"}, Force: ForceStatement, Agent: Term{Phrase: "the dog"}, Patient: Term{Phrase: "the cat"}}
	got, ok := p.RealizeDecl(f, low)
	want := p.Realize(f, low)
	if !ok || got != want || got != "The dog may follow the cat." {
		t.Fatalf("low oxytocin should hedge: RealizeDecl %q vs Realize %q", got, want)
	}
}

// The event clause declines its place-marked and Turkish shapes, and a non-fronting verb's
// adjuncts, each a later stage, so the shadow stays silent and the coded realizer stands.
func TestRealizeDeclEventDeclines(t *testing.T) {
	p := Builtin()
	base := limbic.State{Oxytocin: limbic.OxytocinBaseline}
	ev := func(f Frame) Frame {
		f.Action = Term{Phrase: "follow"}
		f.Force = ForceStatement
		f.Agent = Term{Phrase: "the dog"}
		f.Patient = Term{Phrase: "the cat"}
		return f
	}
	cases := []struct {
		name string
		f    Frame
		chem limbic.State
	}{
		{"precede with adjunct", Frame{Action: Term{Phrase: "precede"}, Force: ForceStatement, Agent: Term{Phrase: "make test"}, Quals: []Qualifier{{Rel: "after", Anchor: Term{Phrase: "make deploy"}}}}, base},
		{"with place", ev(Frame{At: "09:14"}), base},
	}
	for _, c := range cases {
		if got, ok := p.RealizeDecl(c.f, c.chem); ok {
			t.Fatalf("%s: RealizeDecl should decline, got %q", c.name, got)
		}
	}
}

// RealizeDecl declines the shapes the copula stage does not cover, so the shadow stays silent
// and the coded realizer stands.
func TestRealizeDeclDeclines(t *testing.T) {
	p := Builtin()
	var chem limbic.State
	cases := []struct {
		name string
		f    Frame
	}{
		{"question with no patient", Frame{Action: Term{Phrase: "be"}, Force: ForceQuestion}},
		{"unhandled action", Frame{Action: Term{Phrase: "know"}, Force: ForceStatement, Agent: Term{Phrase: "I"}, Patient: Term{Phrase: "the cat"}}},
		{"given subject", Frame{Action: Term{Phrase: "be"}, Force: ForceStatement, Agent: Term{Given: true}, Patient: Term{Phrase: "an animal"}}},
		{"first person", Frame{Action: Term{Phrase: "be"}, Force: ForceStatement, Agent: Term{Phrase: "I"}, Patient: Term{Phrase: "an animal"}}},
		{"hedged be", Frame{Action: Term{Phrase: "be"}, Force: ForceStatement, Agent: Term{Phrase: "the dog"}, Patient: Term{Phrase: "an animal"}, Hedge: true}},
		{"turkish negation", Frame{Action: Term{Phrase: "be"}, Force: ForceStatement, Lang: "tr", Agent: Term{Phrase: "köpek"}, Patient: Term{Phrase: "bir şey"}, Negate: true}},
	}
	for _, c := range cases {
		if got, ok := p.RealizeDecl(c.f, chem); ok {
			t.Fatalf("%s: RealizeDecl should decline, got %q", c.name, got)
		}
	}
}
