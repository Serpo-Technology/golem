package grammar

import (
	"testing"

	"golem/internal/limbic"
)

// ParseTemplate splits literal runs from {slot} markers, and Render fills
// the slots, closes up spaces, and raises the first letter while leaving
// the template's own terminal punctuation in place.
func TestConstructionParseAndRender(t *testing.T) {
	c := Construction{Tokens: ParseTemplate("{adjunct}, {subject} {vp}.")}
	got := c.Render(map[string]string{
		"adjunct": "after make test",
		"subject": "make deploy",
		"vp":      "usually follows",
	})
	if got != "After make test, make deploy usually follows." {
		t.Fatalf("the slots should fill and the clause finish: %q", got)
	}

	// A fragment-style template keeps no terminal mark, and a missing slot
	// collapses to nothing rather than a stray gap.
	frag := Construction{Tokens: ParseTemplate("I was thinking: {thought}")}
	if got := frag.Render(map[string]string{"thought": "the disk"}); got != "I was thinking: the disk" {
		t.Fatalf("a fragment template stays unpunctuated: %q", got)
	}
	if got := frag.Render(nil); got != "I was thinking:" {
		t.Fatalf("a missing slot should not leave a gap: %q", got)
	}
}

// The strongest eligible variant speaks, and a chemistry condition gates
// eligibility: trust splits a confident assertion from a tentative hedge.
func TestConstructionSelectionByChemistryAndWeight(t *testing.T) {
	p := Compile(nil, nil).SetConstructions([]Param{
		{Key: "cxn:en:report:plain", Value: "I think {x}.", Category: "", Weight: 1.0},
		{Key: "cxn:en:report:loud", Value: "I really think {x}.", Category: "", Weight: 1.5},
	})
	calm := limbic.State{Oxytocin: 0.5}
	if got, ok := p.Construct("en", "report", map[string]string{"x": "so"}, calm); !ok || got != "I really think so." {
		t.Fatalf("the heavier variant should win: %q %v", got, ok)
	}

	gated := Compile(nil, nil).SetConstructions([]Param{
		{Key: "cxn:en:say:assert", Value: "{x} follows.", Category: "confident", Weight: 1.0},
		{Key: "cxn:en:say:hedge", Value: "{x} may follow.", Category: "tentative", Weight: 1.0},
	})
	confident := limbic.State{Oxytocin: limbic.OxytocinBaseline}
	tentative := limbic.State{Oxytocin: 0.1}
	if got, _ := gated.Construct("en", "say", map[string]string{"x": "it"}, confident); got != "It follows." {
		t.Fatalf("trust at baseline should assert: %q", got)
	}
	if got, _ := gated.Construct("en", "say", map[string]string{"x": "it"}, tentative); got != "It may follow." {
		t.Fatalf("low trust should hedge: %q", got)
	}

	// An intent with nothing seeded returns false, so the caller falls back
	// to the innate realizer.
	if _, ok := gated.Construct("en", "absent", nil, calm); ok {
		t.Fatal("an unseeded intent should not construct")
	}
}

// Every seeded construction parses and renders through the builtin, so the
// curriculum cannot ship a broken template, and the definition pattern
// reads the same answer the realizer used to assemble.
func TestCurriculumConstructionsRender(t *testing.T) {
	p := Builtin()
	got, ok := p.Construct("en", "definition",
		map[string]string{"subject": "make test", "kind": "an action"}, limbic.State{Oxytocin: 0.5})
	if !ok || got != "Make test is an action." {
		t.Fatalf("the definition construction should build the answer: %q %v", got, ok)
	}
	for _, seed := range Constructions() {
		toks := ParseTemplate(seed.Value)
		if len(toks) == 0 {
			t.Fatalf("a seeded construction has an empty template: %q", seed.Key)
		}
	}
}

// Step 22: a construction slot filled by another construction, rendered by
// recursive slot-filling, with selection at every layer driven by
// chemistry, and the composition reading back as the constructions that
// made it.
func TestComposeNestsAndReadsBack(t *testing.T) {
	p := Compile(nil, nil).SetConstructions([]Param{
		{Key: "cxn:en:inner:plain", Value: "the disk is full", Weight: 1.0},
		{Key: "cxn:en:frame:calm", Value: "I noticed {clause}.", Category: "calm", Weight: 1.0},
		{Key: "cxn:en:frame:alarm", Value: "Watch out: {clause}!", Category: "aroused", Weight: 1.0},
	})

	calm := limbic.State{Noradrenaline: 0.1}
	clause := func() map[string]Filler {
		return map[string]Filler{"clause": {Sub: &Message{Intent: "inner"}}}
	}
	line, trace, ok := p.Compose("en", Message{Intent: "frame", Slots: clause()}, calm)
	if !ok || line != "I noticed the disk is full." {
		t.Fatalf("a calm frame should wrap the nested clause: %q %v", line, ok)
	}
	if len(trace) != 2 || trace[0] != "inner:plain" || trace[1] != "frame:calm" {
		t.Fatalf("the composition should read back as its constructions, leaves first: %v", trace)
	}

	// The same nesting, different chemistry: the outer layer's selection
	// follows arousal, so the novel sentence comes out differently and is
	// still grammatical.
	aroused := limbic.State{Noradrenaline: 0.8}
	if loud, _, _ := p.Compose("en", Message{Intent: "frame", Slots: clause()}, aroused); loud != "Watch out: the disk is full!" {
		t.Fatalf("an aroused frame should wrap it loudly: %q", loud)
	}

	// A missing construction at any layer fails the whole compose, so the
	// caller falls back to the innate realizer rather than a half clause.
	if _, _, ok := p.Compose("en", Message{Intent: "frame",
		Slots: map[string]Filler{"clause": {Sub: &Message{Intent: "absent"}}}}, calm); ok {
		t.Fatal("a missing nested construction should not compose")
	}
}
