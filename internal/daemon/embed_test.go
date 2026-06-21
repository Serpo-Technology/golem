package daemon

import "testing"

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
