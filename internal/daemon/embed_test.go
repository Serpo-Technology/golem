package daemon

import (
	"strings"
	"testing"
)

// Usage resemblance carries an inference between abstract words: the model and the
// network are used alike (both learn, both read), so when only the model is said to
// predict an output, the network's predicting follows as a hedged guess. Neither
// has a feature, so the feature space could not place them; usage is the only
// signal, and the reasoning rides it the way it rides feature resemblance.
func TestAbstractInferenceRidesUsageResemblance(t *testing.T) {
	d, _ := testDaemon(t)
	for _, w := range []string{"model", "network", "pattern", "data", "output"} {
		groundNoun(t, d, w)
	}
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
