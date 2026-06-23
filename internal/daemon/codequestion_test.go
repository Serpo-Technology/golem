package daemon

import (
	"bytes"
	"strings"
	"testing"

	"golem/internal/grammar"
)

// A computation posed as a question is computed, not recalled: with the runtime
// open "what is the sum of 6 and 7" comprehends to its meaning, runs, and speaks
// the value. A pair never seen is computed the same way, so the answer is the
// run and not a stored fact.
func TestComputationQuestionRunsAndAnswers(t *testing.T) {
	d, _ := testDaemon(t)
	d.cfg.RuntimeEnabled = true

	resp := say(t, d, "what is the sum of 6 and 7")
	if !strings.Contains(resp.Say, "13") {
		t.Fatalf("the computation question should run to 13: %q", resp.Say)
	}
	resp = say(t, d, "what is the product of 12 and 11")
	if !strings.Contains(resp.Say, "132") {
		t.Fatalf("a novel computation should run to 132: %q", resp.Say)
	}
}

// The in-conversation compute path generates by traversal as the explicit runcode path
// does: a computation posed as a question materializes onto the graph and is walked for
// the source it runs, with the typed render only a cross-check. The answer is the right
// value and nothing is logged, so the conversational generator agrees with the render on
// real input.
func TestComputationQuestionGeneratesViaTraversal(t *testing.T) {
	d, _ := testDaemon(t)
	d.cfg.RuntimeEnabled = true

	var buf bytes.Buffer
	d.out = &buf
	resp := say(t, d, "what is the sum of 6 and 7")
	if !strings.Contains(resp.Say, "13") {
		t.Fatalf("the computation question should run to 13: %q", resp.Say)
	}
	if strings.Contains(buf.String(), "mismatch") {
		t.Fatalf("the conversational compute path should agree with the render, but logged: %q", buf.String())
	}
}

// With the runtime closed the same question is never run: the daemon admits the
// gap, so a production brain with the sandbox shut keeps the execute path dark.
func TestComputationQuestionGatedByRuntime(t *testing.T) {
	d, _ := testDaemon(t)

	resp := say(t, d, "what is the sum of 6 and 7")
	if strings.Contains(resp.Say, "13") {
		t.Fatalf("with the runtime closed the computation must not run: %q", resp.Say)
	}
}

// A question carrying no number is a definition, not a computation, so the code
// faculty steps aside before the runtime is ever spun.
func TestPlanComputationSkipsNonNumber(t *testing.T) {
	d, _ := testDaemon(t)
	d.cfg.RuntimeEnabled = true

	if line, ok := d.planComputation(grammar.Frame{Lang: "en", Force: grammar.ForceQuestion}, "what is a dog"); ok {
		t.Fatalf("a question with no number is not a computation: %q", line)
	}
}

// With curiosity open an unknown word still trails a question, but a bare
// numeral operand does not: "6 + 7 is 13" is the whole answer, not a prompt to
// ask after its digits.
func TestFollowUpAsksWordsNotNumerals(t *testing.T) {
	d, _ := testDaemon(t)
	d.chem.SpikeDopamine(0.6) // above the curiosity gate

	numeric := d.withFollowUp(grammar.Frame{Lang: "en", Unknown: []string{"6 7"}}, "6 + 7 is 13.")
	if numeric != "6 + 7 is 13." {
		t.Fatalf("a numeral operand should not trail a question: %q", numeric)
	}
	worded := d.withFollowUp(grammar.Frame{Lang: "en", Unknown: []string{"flurble"}}, "It is unclear.")
	if !strings.Contains(strings.ToLower(worded), "flurble") {
		t.Fatalf("an unknown word should still trail a curiosity question: %q", worded)
	}
}

// computationSpan finds where a computation begins in a question by comprehension:
// the conversational lead-in falls away because it is no computation the faculty
// reads, not by a strip list. The bare computation and the operator-word form are
// found unchanged, and a question that is no computation yields no span.
func TestComputationSpan(t *testing.T) {
	d, _ := testDaemon(t)
	cases := map[string]string{
		"what is the sum of 6 and 7":         "the sum of 6 and 7",
		"what is 6 + 7?":                     "6 + 7",
		"how much is the product of 3 and 4": "the product of 3 and 4",
		"6 + 7":                              "6 + 7",
	}
	for in, want := range cases {
		if got, ok := d.computationSpan("en", in); !ok || got != want {
			t.Fatalf("computationSpan(%q) = %q (ok=%v), want %q", in, got, ok, want)
		}
	}
	if got, ok := d.computationSpan("en", "what is a lion"); ok {
		t.Errorf("a non-computation question should yield no span, got %q", got)
	}
}
