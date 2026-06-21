package daemon

import (
	"strings"
	"testing"
)

// Turkish asks a yes/no question in the aorist by fusing the particle with the
// person ("ister misin" = want + mi + you), where past tense carries the person
// on the verb and a bare particle. The fused form drops the subject onto the
// particle, so the reflexive "do you want X" is answered of GOLEM, spoken back in
// the first person.
func TestTurkishFusedInterrogativeReflexive(t *testing.T) {
	d, _ := testDaemon(t)
	groundNounLang(t, d, "kedi", "tr")
	groundNounLang(t, d, "köpek", "tr")
	sayLang(t, d, "tr", "köpek kediyi ister") // mint the want relation
	sayLang(t, d, "tr", "sen kediyi ister")   // (you) want the cat -> GOLEM as experiencer

	// "kediyi ister misin" = "do you want the cat", the fused particle 2nd person.
	if got := sayLang(t, d, "tr", "kediyi ister misin").Say; !strings.HasPrefix(got, "Evet") {
		t.Errorf("a fused-particle reflexive should affirm what GOLEM holds: %q", got)
	}
	// "köpeği ister misin" = "do you want the dog"; never told, denied.
	if got := sayLang(t, d, "tr", "köpeği ister misin").Say; !strings.HasPrefix(got, "Hayır") {
		t.Errorf("a fused-particle reflexive GOLEM cannot affirm should deny: %q", got)
	}
}

// Latin drops the subject onto the verb's person, so a polar question can be two
// words ("amasne rosam" = love-you + rose). The enclitic -ne marks the question
// and the second person recovers GOLEM as the one asked about.
func TestLatinProDropPolarReflexive(t *testing.T) {
	d, _ := testDaemon(t)
	groundLatinPair(t, d, "girl", "girl", "puella")
	groundLatinPair(t, d, "rose", "rose", "rosa")
	say(t, d, "the girl loves the rose")
	sayLang(t, d, "la", "puella rosam amat") // bind amat to the love relation
	sayLang(t, d, "la", "rosam amas")        // (you) love the rose -> GOLEM as agent

	if got := sayLang(t, d, "la", "amasne rosam?").Say; !strings.HasPrefix(got, "Ita") {
		t.Errorf("a pro-dropped second-person Latin polar should affirm: %q", got)
	}
}
