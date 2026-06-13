package grammar

import (
	"testing"

	"golem/internal/memory"
)

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

// The transitive construction runs form to meaning, the engine reversed: a
// told "a dog chased a cat" binds the chaser and the chased into the roles
// and recovers the relation's lemma and tense, the meaning the brain can
// then write down.
func TestComprehendTransitive(t *testing.T) {
	p := Builtin()
	dog := &memory.Fragment{ID: 1, Content: "dog"}
	cat := &memory.Fragment{ID: 2, Content: "cat"}
	resolve := func(s string) *memory.Fragment {
		switch s {
		case "dog":
			return dog
		case "cat":
			return cat
		}
		return nil
	}
	m, ok := p.Comprehend("en", "a dog chased a cat", resolve, nil)
	if !ok {
		t.Fatal("the transitive clause should comprehend")
	}
	if m.Roles["agent"].Concept != dog {
		t.Fatalf("the dog should be the agent: %+v", m.Roles["agent"])
	}
	if m.Roles["patient"].Concept != cat {
		t.Fatalf("the cat should be the patient: %+v", m.Roles["patient"])
	}
	if m.Relation.Phrase != "chase" {
		t.Fatalf("the relation should lemmatize to chase: %q", m.Relation.Phrase)
	}
	if !m.Past {
		t.Fatal("the past inflection should read as a settled clause")
	}
}

// A multi-word noun phrase still binds, the determiners marking where the
// agent ends and the patient begins, so an adjective rides along into the
// role's phrase.
func TestComprehendMultiWordPhrases(t *testing.T) {
	p := Builtin()
	resolve := func(string) *memory.Fragment { return nil }
	m, ok := p.Comprehend("en", "the big dog chased the small cat", resolve, nil)
	if !ok {
		t.Fatal("the clause should comprehend")
	}
	if m.Roles["agent"].Phrase != "big dog" || m.Roles["patient"].Phrase != "small cat" {
		t.Fatalf("the phrases should split at the determiner: %+v", m.Roles)
	}
}

// The intransitive construction binds an agent and no patient, the present
// tense reading as unsettled, so a one-place clause comprehends through the
// same engine.
func TestComprehendIntransitive(t *testing.T) {
	p := Builtin()
	dog := &memory.Fragment{ID: 1, Content: "dog"}
	resolve := func(s string) *memory.Fragment {
		if s == "dog" {
			return dog
		}
		return nil
	}
	m, ok := p.Comprehend("en", "a dog barks", resolve, nil)
	if !ok {
		t.Fatal("the intransitive clause should comprehend")
	}
	if m.Roles["agent"].Concept != dog {
		t.Fatal("the dog should be the agent")
	}
	if _, ok := m.Roles["patient"]; ok {
		t.Fatal("an intransitive clause has no patient")
	}
	if m.Relation.Phrase != "bark" || m.Past {
		t.Fatalf("the relation should be a present bark: %q past %v", m.Relation.Phrase, m.Past)
	}
}

// Best fit refines what the word order proposed: an animate concept the
// order stranded in the patient slot is read as the agent instead, the
// feature signatures constraining the binding. When position already agrees
// the order stands.
func TestComprehendBestFitPrefersAnimateAgent(t *testing.T) {
	p := Builtin()
	dog := &memory.Fragment{ID: 1, Content: "dog"}
	rock := &memory.Fragment{ID: 2, Content: "rock"}
	resolve := func(s string) *memory.Fragment {
		switch s {
		case "dog":
			return dog
		case "rock":
			return rock
		}
		return nil
	}
	animate := map[uint64]bool{dog.ID: true}
	fit := func(c *memory.Fragment, role string) float64 {
		if role == "agent" && animate[c.ID] {
			return 1
		}
		return 0.3
	}
	m, ok := p.Comprehend("en", "a rock chased a dog", resolve, fit)
	if !ok {
		t.Fatal("should comprehend")
	}
	if m.Roles["agent"].Concept != dog || m.Roles["patient"].Concept != rock {
		t.Fatalf("best fit should hand the agent role to the dog: %+v", m.Roles)
	}
	// When the order already seats the animate concept as the agent, the
	// margin is not met and position stands.
	m2, _ := p.Comprehend("en", "a dog chased a rock", resolve, fit)
	if m2.Roles["agent"].Concept != dog {
		t.Fatalf("position should stand when it already agrees: %+v", m2.Roles)
	}
}

// The same construction both says and understands: a meaning generated to a
// surface string comprehends back into the same roles over the same relation,
// the round trip through one inventory read each way that is the whole point.
func TestArgConstructionRoundTrip(t *testing.T) {
	p := Builtin()
	dog := &memory.Fragment{ID: 1, Content: "dog"}
	cat := &memory.Fragment{ID: 2, Content: "cat"}
	resolve := func(s string) *memory.Fragment {
		switch s {
		case "dog":
			return dog
		case "cat":
			return cat
		}
		return nil
	}
	said, ok := p.Speak("en", Meaning{
		Relation: Term{Phrase: "chase"},
		Roles:    map[string]Term{"agent": {Phrase: "the dog"}, "patient": {Phrase: "the cat"}},
		Past:     true,
	})
	if !ok || said != "The dog chased the cat." {
		t.Fatalf("generation should speak the proposition: %q", said)
	}
	back, ok := p.Comprehend("en", said, resolve, nil)
	if !ok {
		t.Fatal("the said sentence should comprehend back")
	}
	if back.Roles["agent"].Concept != dog || back.Roles["patient"].Concept != cat {
		t.Fatalf("the round trip should recover the roles: %+v", back.Roles)
	}
	if back.Relation.Phrase != "chase" || !back.Past {
		t.Fatalf("the round trip should recover the relation and tense: %q past %v", back.Relation.Phrase, back.Past)
	}
}
