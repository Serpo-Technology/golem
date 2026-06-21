package daemon

import (
	"strings"
	"testing"
)

// English possession is a plain transitive relation once "have" is a known verb:
// "the tree has a leaf" answers "what does the tree have" and the polar question.
func TestEnglishPossession(t *testing.T) {
	d, _ := testDaemon(t)
	groundNoun(t, d, "tree")
	groundNoun(t, d, "leaf")
	say(t, d, "the tree has a leaf")
	if got := say(t, d, "what does the tree have").Say; !strings.Contains(got, "tree has the leaf") {
		t.Errorf("what-does-X-have should name the possessed: %q", got)
	}
	if got := say(t, d, "does the tree have a leaf").Say; !strings.Contains(got, "yes") {
		t.Errorf("the possession should affirm: %q", got)
	}
}

// Turkish marks possession by the genitive and possessive cases and the existential
// "var", no verb. Its English echo unifies onto the same relation, so the possession
// answers in either tongue.
func TestTurkishPossession(t *testing.T) {
	d, _ := testDaemon(t)
	groundBilingual(t, d, "tree", "tree", "ağaç")
	groundBilingual(t, d, "leaf", "leaf", "yaprak")
	groundBilingual(t, d, "ball", "ball", "top")
	say(t, d, "the tree has a leaf")
	sayLang(t, d, "tr", "ağacın yaprağı var")

	tr := sayLang(t, d, "tr", "ağacın yaprağı var mı").Say
	t.Logf("ağacın yaprağı var mı -> %q", tr)
	en := say(t, d, "what does the tree have").Say
	t.Logf("what does the tree have -> %q", en)
	no := sayLang(t, d, "tr", "ağacın topu var mı").Say
	t.Logf("ağacın topu var mı -> %q", no)

	if tr != "Evet, ağacın yaprağı var." {
		t.Errorf("Turkish possession should affirm in the var form: %q", tr)
	}
	if !strings.Contains(en, "tree has the leaf") {
		t.Errorf("the unified relation should answer in English: %q", en)
	}
	if !strings.HasPrefix(no, "Hayır") {
		t.Errorf("an unheld possession should deny in Turkish: %q", no)
	}
}

// Turkish possession comprehends without an English priming: a bare "var" clause
// mints a general Turkish possession and the polar reads it back.
func TestTurkishPossessionStandalone(t *testing.T) {
	d, _ := testDaemon(t)
	groundBilingual(t, d, "cat", "cat", "kedi")
	groundBilingual(t, d, "ball", "ball", "top")
	sayLang(t, d, "tr", "kedinin topu var")
	if got := sayLang(t, d, "tr", "kedinin topu var mı").Say; got != "Evet, kedinin topu var." {
		t.Errorf("a standalone Turkish possession should read back: %q", got)
	}
}
