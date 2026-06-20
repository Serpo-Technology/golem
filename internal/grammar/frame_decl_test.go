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
		{"non-be action", Frame{Action: Term{Phrase: "follow"}, Force: ForceStatement, Agent: Term{Phrase: "the dog"}, Patient: Term{Phrase: "the cat"}}},
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
