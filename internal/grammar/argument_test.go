package grammar

import (
	"strings"
	"testing"

	"golem/internal/memory"
)

// Speak renders an answer's surface from the tongue's data, not from literals:
// a given referent takes the tongue's pronoun, a definite one its article,
// coordinated co-arguments its coordinator. English reads "It chased the cat
// and the mouse"; Turkish carries its own pronoun and coordinator and no
// English article, the surface words all read from the seed.
func TestSpeakRendersSurfaceFromData(t *testing.T) {
	p := Builtin()
	en, ok := p.Speak("en", Meaning{
		Relation: Term{Phrase: "chase"},
		Roles: map[string]Term{
			"agent":   {Given: true},
			"patient": {Phrase: "cat", With: []string{"mouse"}, Definite: true},
		}, Past: true})
	if !ok || en != "It chased the cat and the mouse." {
		t.Fatalf("english surface should come from data: %q (%v)", en, ok)
	}
	tr, ok := p.Speak("tr", Meaning{
		Relation: Term{Phrase: "kovala"},
		Roles: map[string]Term{
			"agent":   {Phrase: "köpek", Definite: true},
			"patient": {Phrase: "kedi", With: []string{"fare"}, Definite: true},
		}, Past: true})
	if !ok {
		t.Fatal("turkish should speak")
	}
	if strings.Contains(tr, " the ") || strings.Contains(tr, " and ") {
		t.Fatalf("turkish should carry no english article or coordinator: %q", tr)
	}
	if !strings.Contains(tr, "ve") {
		t.Fatalf("turkish should coordinate with its own word: %q", tr)
	}
}

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

// A filler that has merely been an agent more often does not unseat a perfectly
// good agent the order named first: "a dog chased a rabbit" stays the dog
// chasing the rabbit even when the rabbit's agent history is the larger.
func TestComprehendKeepsOrderAgentOverHistory(t *testing.T) {
	p := Builtin()
	dog := &memory.Fragment{ID: 1, Content: "dog"}
	rabbit := &memory.Fragment{ID: 2, Content: "rabbit"}
	resolve := func(s string) *memory.Fragment {
		switch s {
		case "dog":
			return dog
		case "rabbit":
			return rabbit
		}
		return nil
	}
	// Both are good agents; the rabbit's history is larger, as on a trained brain.
	fit := func(c *memory.Fragment, role string) float64 {
		if role != "agent" {
			return 0.3
		}
		if c.ID == rabbit.ID {
			return 1.5
		}
		return 1.0
	}
	m, ok := p.Comprehend("en", "a dog chased a rabbit", resolve, fit)
	if !ok {
		t.Fatal("should comprehend")
	}
	if m.Roles["agent"].Concept != dog || m.Roles["patient"].Concept != rabbit {
		t.Fatalf("the order's agent should stand against a larger history: %+v", m.Roles)
	}
}

// On a trained graph a heavily grounded concept fits both roles highly through
// its feature kin, so the order's agent can read as fitting the patient role by
// a hair while remaining the strongest available agent. The order must still
// stand: "a dog chased a rabbit" stays the dog chasing even when the dog's
// patient fit just edges its agent fit, because the dog out-fits the rabbit for
// the agent role. These fit values mirror what a trained brain produced live.
func TestComprehendKeepsOrderAgentInNearTie(t *testing.T) {
	p := Builtin()
	dog := &memory.Fragment{ID: 1, Content: "dog"}
	rabbit := &memory.Fragment{ID: 2, Content: "rabbit"}
	resolve := func(s string) *memory.Fragment {
		switch s {
		case "dog":
			return dog
		case "rabbit":
			return rabbit
		}
		return nil
	}
	// The dog fits both roles highly (patient a hair above agent through kin);
	// the rabbit is a weaker agent and a poor patient. The dog is the better
	// agent, so the order holds.
	fit := func(c *memory.Fragment, role string) float64 {
		switch {
		case c.ID == dog.ID && role == "agent":
			return 2.513
		case c.ID == dog.ID:
			return 2.541
		case c.ID == rabbit.ID && role == "agent":
			return 2.227
		default:
			return 0.831
		}
	}
	m, ok := p.Comprehend("en", "a dog chased a rabbit", resolve, fit)
	if !ok {
		t.Fatal("should comprehend")
	}
	if m.Roles["agent"].Concept != dog || m.Roles["patient"].Concept != rabbit {
		t.Fatalf("the order's agent should stand in a near tie when it is the better agent: %+v", m.Roles)
	}
}

// Even when the co-argument genuinely out-fits the order's agent for the agent
// role, the order stands unless the co-argument is the better agent by a clear
// multiple: on a trained graph feature kin lifts every animate filler, so a
// central concept edges ahead as an agent without being the right subject. These
// fit values are what a trained brain produced for "a dog chased a cat", where
// the cat, the most-grounded animal, out-fits the dog as an agent yet the dog is
// the chaser.
func TestComprehendKeepsOrderAgentWhenCoArgEdgesAhead(t *testing.T) {
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
	fit := func(c *memory.Fragment, role string) float64 {
		switch {
		case c.ID == dog.ID && role == "agent":
			return 2.513
		case c.ID == dog.ID:
			return 2.541
		case c.ID == cat.ID && role == "agent":
			return 3.215
		default:
			return 1.990
		}
	}
	m, ok := p.Comprehend("en", "a dog chased a cat", resolve, fit)
	if !ok {
		t.Fatal("should comprehend")
	}
	if m.Roles["agent"].Concept != dog || m.Roles["patient"].Concept != cat {
		t.Fatalf("the order's agent should stand when the co-argument only edges ahead: %+v", m.Roles)
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
