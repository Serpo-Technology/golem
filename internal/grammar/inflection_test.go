package grammar

import "testing"

// GOLEM learns a regular suffix from a few examples and generalizes it to a held-out base,
// the seed of morphology as learned constructions rather than coded rules.
func TestLearnsRegularSuffix(t *testing.T) {
	p := Compile(nil, nil)
	if !p.LearnInflection("en", "past", [][2]string{{"walk", "walked"}, {"jump", "jumped"}, {"call", "called"}}) {
		t.Fatal("the regular -ed suffix should be learned")
	}
	if got, ok := p.Inflect("en", "past", "kick"); !ok || got != "kicked" {
		t.Fatalf("the learned rule should inflect kick -> kicked, got %q (ok %v)", got, ok)
	}
}

// Verbs that already end in the vowel the suffix shares are learned as the shorter affix the
// longest-common-prefix finds (chase -> chased is "+d"), and it generalizes to a held-out
// base of the same shape.
func TestLearnsStemShare(t *testing.T) {
	p := Compile(nil, nil)
	if !p.LearnInflection("en", "past", [][2]string{{"chase", "chased"}, {"like", "liked"}, {"dance", "danced"}}) {
		t.Fatal("the shared-vowel past should be learned")
	}
	if got, ok := p.Inflect("en", "past", "bake"); !ok || got != "baked" {
		t.Fatalf("the learned rule should inflect bake -> baked, got %q (ok %v)", got, ok)
	}
}

// A pattern the engine has no code for is learned the same way, proving genuine learning
// rather than shadowing the built-in English rules.
func TestLearnsSyntheticLanguage(t *testing.T) {
	p := Compile(nil, nil)
	if !p.LearnInflection("xx", "past", [][2]string{{"walk", "walkmo"}, {"run", "runmo"}}) {
		t.Fatal("a synthetic suffix the engine has no code for should be learned")
	}
	if got, ok := p.Inflect("xx", "past", "jump"); !ok || got != "jumpmo" {
		t.Fatalf("the learned rule should inflect jump -> jumpmo, got %q (ok %v)", got, ok)
	}
}

// GOLEM learns a HARMONIC suffix from examples -- the Turkish -DI past, whose vowel bends to
// the stem's last vowel -- and generalizes it to held-out stems, the suffix vowel chosen by
// backness and rounding. The vowel features are seeded phonetics; the harmony rule is learned.
func TestLearnsVowelHarmony(t *testing.T) {
	p := Compile(nil, nil)
	if !p.LearnInflection("tr", "past", [][2]string{
		{"kovala", "kovaladı"}, // back unround a -> ı
		{"gel", "geldi"},       // front unround e -> i
		{"oku", "okudu"},       // back round u -> u
		{"gör", "gördü"},       // front round ö -> ü
	}) {
		t.Fatal("the harmonic -DI past should be learned")
	}
	// Held-out stems, vowel- or voiced-final so the vowel harmony alone is correct Turkish.
	for base, want := range map[string]string{
		"ara":  "aradı",  // back unround -> ı
		"bil":  "bildi",  // front unround -> i
		"uyu":  "uyudu",  // back round -> u
		"yürü": "yürüdü", // front round -> ü
	} {
		if got, ok := p.Inflect("tr", "past", base); !ok || got != want {
			t.Fatalf("harmony should inflect %s -> %s, got %q (ok %v)", base, want, got, ok)
		}
	}
}

// Pairs no single regular rule explains are declined, the Occam guard, and nothing is left
// to inflect with.
func TestDeclinesInconsistent(t *testing.T) {
	p := Compile(nil, nil)
	if p.LearnInflection("en", "past", [][2]string{{"go", "went"}, {"walk", "walked"}}) {
		t.Fatal("pairs with no single regular rule should be declined")
	}
	if _, ok := p.Inflect("en", "past", "talk"); ok {
		t.Fatal("nothing should be learned to inflect with")
	}
}
