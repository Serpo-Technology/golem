package grammar

import "testing"

// The transitive construction runs meaning to form: a proposition of a
// relation over an agent and a patient lays out in the language's word
// order and the verb is conjugated by the morphology engine, so the brain
// can say a chasing it has in mind.
func TestSpeakTransitive(t *testing.T) {
	p := Builtin()
	m := Meaning{
		Relation: Term{Phrase: "chase"},
		Roles: map[string]Term{
			"agent":   {Phrase: "the dog"},
			"patient": {Phrase: "the cat"},
		},
		Past: true,
	}
	got, ok := p.Speak("en", m)
	if !ok || got != "The dog chased the cat." {
		t.Fatalf("the transitive should speak past SVO: got %q ok %v", got, ok)
	}
}

// The intransitive construction binds an agent and no patient, so a
// one-place relation speaks through the same engine: agreement still flows
// from the subject's number.
func TestSpeakIntransitive(t *testing.T) {
	p := Builtin()
	m := Meaning{
		Relation: Term{Phrase: "sleep"},
		Roles:    map[string]Term{"agent": {Phrase: "the dog"}},
	}
	got, ok := p.Speak("en", m)
	if !ok || got != "The dog sleeps." {
		t.Fatalf("the intransitive should agree in the present: got %q ok %v", got, ok)
	}
}

// Word order is data, not code: editing the order parameter to verb-final
// reorders the same construction to SOV with no change to the engine, the
// claim that one construction speaks several tongues.
func TestSpeakWordOrderIsData(t *testing.T) {
	seed := Curriculum()
	for i := range seed {
		if seed[i].Key == "order:en" {
			seed[i].Value = "sov"
		}
	}
	p := Compile(seed, nil).SetArgConstructions(Constructions())
	m := Meaning{
		Relation: Term{Phrase: "chase"},
		Roles: map[string]Term{
			"agent":   {Phrase: "the dog"},
			"patient": {Phrase: "the cat"},
		},
		Past: true,
	}
	got, ok := p.Speak("en", m)
	if !ok || got != "The dog the cat chased." {
		t.Fatalf("verb-final order should close the clause with the verb: got %q ok %v", got, ok)
	}
}

// A meaning no construction binds fails cleanly, so the caller can fall
// back to the procedural realizer rather than speak a broken clause.
func TestSpeakFailsClean(t *testing.T) {
	p := Builtin()
	if _, ok := p.Speak("en", Meaning{Relation: Term{Phrase: "chase"}}); ok {
		t.Fatal("a meaning with no bound roles should not match a construction")
	}
	if _, ok := p.Speak("xx", Meaning{Relation: Term{Phrase: "chase"},
		Roles: map[string]Term{"agent": {Phrase: "the dog"}}}); ok {
		t.Fatal("a language with no argument constructions should fail to the fallback")
	}
}
