package daemon

import (
	"reflect"
	"strings"
	"testing"
)

func contains(cands []string, want string) bool {
	for _, c := range cands {
		if c == want {
			return true
		}
	}
	return false
}

// The fold generator recovers the proper Turkish spelling among its candidates: an
// ASCII "buyuk" folds to a set that contains "büyük", the word GOLEM knows.
func TestRestoreDiacriticIncludesProperForm(t *testing.T) {
	got := restoreDiacriticTR("buyuk")
	if !contains(got, "büyük") {
		t.Fatalf("restoreDiacriticTR(%q) should include %q, got %v", "buyuk", "büyük", got)
	}
}

// The i/ı fold enumerates both members of the class, so "kedi" produces both the
// dotted "kedi" and the dotless "kedı"; the lexicon, not the fold, decides which is a
// word, so a genuine ı/i pair is never merged by generation alone.
func TestRestoreDiacriticEnumeratesBothIDotted(t *testing.T) {
	got := restoreDiacriticTR("kedi")
	if !contains(got, "kedi") || !contains(got, "kedı") {
		t.Fatalf("restoreDiacriticTR(%q) should include both %q and %q, got %v", "kedi", "kedi", "kedı", got)
	}
}

// The Turkish-locale pre-pass maps the capital dotless I to ı before folding, so a
// plainly cased "Isik" still reaches the diacritic "ışık" the default casing could not.
func TestRestoreDiacriticTurkishLocaleCasing(t *testing.T) {
	got := restoreDiacriticTR("Isik")
	if !contains(got, "ışık") {
		t.Fatalf("restoreDiacriticTR(%q) should reach %q through the locale pre-pass, got %v", "Isik", "ışık", got)
	}
}

// A token with more foldable positions than the cap returns empty rather than
// enumerating an unbounded product: nine foldable letters is past diacriticKMAX = 8.
func TestRestoreDiacriticCapReturnsEmpty(t *testing.T) {
	over := strings.Repeat("u", diacriticKMAX+1)
	if got := restoreDiacriticTR(over); len(got) != 0 {
		t.Fatalf("restoreDiacriticTR(%q) with %d foldable chars should be empty, got %d candidates",
			over, diacriticKMAX+1, len(got))
	}
	// A token exactly at the cap still generates (2^8 = 256 candidates).
	at := strings.Repeat("u", diacriticKMAX)
	if got := restoreDiacriticTR(at); len(got) != 256 {
		t.Fatalf("restoreDiacriticTR(%q) at the cap should generate 256 candidates, got %d", at, len(got))
	}
}

// Generation is deterministic: two runs over the same surface produce byte-identical
// candidate lists in the same order, as the capstone gate requires.
func TestRestoreDiacriticDeterministicOrder(t *testing.T) {
	a := restoreDiacriticTR("kucuk")
	b := restoreDiacriticTR("kucuk")
	if !reflect.DeepEqual(a, b) {
		t.Fatalf("restoreDiacriticTR should be order-stable across runs: %v vs %v", a, b)
	}
	if !contains(a, "küçük") {
		t.Fatalf("restoreDiacriticTR(%q) should include %q, got %v", "kucuk", "küçük", a)
	}
}
