package daemon

import (
	"testing"

	"golem/internal/memory"
)

func hasProp(d *Daemon, agent *memory.Fragment, rel string, patient *memory.Fragment) bool {
	for _, f := range d.gatherFacts(agent, "agent", "") {
		if f.rel == rel && f.coID == patient.ID {
			return true
		}
	}
	return false
}

// Latin marks roles by case, so word order is free: every permutation of "puella
// rosam amat" (the girl loves the rose) yields the one proposition, the nominative
// puella the agent and the accusative rosam the patient wherever they stand.
func TestLatinFreeWordOrder(t *testing.T) {
	orders := []string{
		"puella rosam amat", "rosam puella amat", "amat puella rosam",
		"rosam amat puella", "puella amat rosam", "amat rosam puella",
	}
	for _, clause := range orders {
		d, _ := testDaemon(t)
		puella := groundNounLang(t, d, "puella", "la")
		rosa := groundNounLang(t, d, "rosa", "la")
		sayLang(t, d, "la", clause)
		if !hasProp(d, puella, "amat", rosa) {
			t.Errorf("%q should comprehend as girl-loves-rose", clause)
		}
		if hasProp(d, rosa, "amat", puella) {
			t.Errorf("%q wrongly made the rose the agent", clause)
		}
	}
}

// The roles come from the case, not the order: swapping which noun wears the
// accusative reverses who loves whom even at the same word order.
func TestLatinCaseAssignsRoles(t *testing.T) {
	d, _ := testDaemon(t)
	puella := groundNounLang(t, d, "puella", "la")
	rosa := groundNounLang(t, d, "rosa", "la")
	sayLang(t, d, "la", "rosa puellam amat") // the rose loves the girl
	if !hasProp(d, rosa, "amat", puella) {
		t.Errorf("the nominative rosa should be the agent")
	}
	if hasProp(d, puella, "amat", rosa) {
		t.Errorf("the accusative puellam should not be the agent")
	}
}
