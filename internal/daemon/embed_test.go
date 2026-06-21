package daemon

import (
	"strings"
	"testing"
)

// An abstract word enters the graph through a telling: "a model predicts a word"
// mints both nouns it could not perceive, the way it already mints the unknown verb,
// so the proposition holds and the words become concepts the usage space can place.
func TestStatementMintsUnknownNouns(t *testing.T) {
	d, _ := testDaemon(t)
	say(t, d, "a model predicts a word") // model, predict, word all unknown

	if d.resolveConcept("model") == nil {
		t.Error("an unknown noun subject should be minted as a concept")
	}
	if d.resolveConcept("word") == nil {
		t.Error("an unknown noun object should be minted as a concept")
	}
	if got := say(t, d, "what does the model predict").Say; !strings.Contains(strings.ToLower(got), "word") {
		t.Errorf("the proposition over the minted nouns should hold: %q", got)
	}
}

// A minted abstract concept survives consolidation. It grounds in no percept, so
// the groundless-concept prune would take it, but it fills a role in a proposition,
// which is its grounding, so it and the proposition over it stand through a night.
func TestMintedConceptSurvivesConsolidation(t *testing.T) {
	d, s := testDaemon(t)
	say(t, d, "a model predicts a word") // mints model, predict, word

	if _, err := s.Sleep(d.cfg.NightlyDecay, d.cfg.PruneFloor); err != nil {
		t.Fatal(err)
	}
	if d.resolveConcept("model") == nil {
		t.Error("a minted concept that fills a role should survive consolidation")
	}
	if got := say(t, d, "did a model predict a word").Say; !strings.Contains(strings.ToLower(got), "yes") {
		t.Errorf("the proposition over the minted nouns should survive the night: %q", got)
	}
}

// Usage resemblance carries an inference between abstract words: the model and the
// network are used alike (both learn, both read), so when only the model is said to
// predict an output, the network's predicting follows as a hedged guess. Neither
// has a feature, so the feature space could not place them; usage is the only
// signal, and the reasoning rides it the way it rides feature resemblance.
func TestAbstractInferenceRidesUsageResemblance(t *testing.T) {
	d, _ := testDaemon(t)
	// No grounding: the abstract nouns enter through the tellings themselves.
	say(t, d, "a model learns a pattern")
	say(t, d, "a network learns a pattern")
	say(t, d, "a model reads data")
	say(t, d, "a network reads data")
	say(t, d, "a model predicts an output") // only the model

	d.growEmbeddings()

	got := strings.ToLower(say(t, d, "does the network predict the output").Say)
	if !strings.Contains(got, "probab") {
		t.Errorf("the model's predicting should carry to the like-used network: %q", got)
	}
}

// Two concepts used the same way, filling the same role of the same relation,
// resemble in the usage space even with no features in common, the placement
// abstract words need: "model" and "network" both predict an output, so they read
// as near though neither names a percept the feature embedding could place.
func TestConceptUsageResemblanceClustersByUse(t *testing.T) {
	d, _ := testDaemon(t)
	model := groundNoun(t, d, "model")
	network := groundNoun(t, d, "network")
	output := groundNoun(t, d, "output")
	groundNoun(t, d, "input")
	say(t, d, "a model predicts an output")
	say(t, d, "a network predicts an output")
	say(t, d, "a model reads an input")

	d.growEmbeddings()

	// Both predict an output, the same role of the same relation, so usage places
	// them near although neither carries a feature.
	near := d.conceptUsageResemblance(model.ID, network.ID)
	if near <= 0 {
		t.Fatalf("concepts used alike should resemble in usage: got %.3f", near)
	}
	// And nearer than the model sits to the output it predicts, which fills the
	// other role of that relation.
	far := d.conceptUsageResemblance(model.ID, output.ID)
	if near <= far {
		t.Errorf("a fellow predictor should sit nearer than the predicted thing: model~network=%.3f model~output=%.3f", near, far)
	}
}
