package daemon

import (
	"testing"
	"time"
)

// The done-when of the grammar step: a Turkish question is answered in
// Turkish, about the same concept the English question reaches, and
// the two answers name the same successor.
func TestTurkishQuestionAnsweredInTurkish(t *testing.T) {
	d, _ := testDaemon(t)
	feedSequence(t, d, "run: make test", "run: make deploy", 3, time.Now().Add(-3*time.Hour))

	tr := say(t, d, "make test'ten sonra ne gelir?")
	if tr.Say != "Make test'ten sonra genellikle make deploy gelir." {
		t.Fatalf("the Turkish question should get a Turkish answer: %q", tr.Say)
	}
	en := say(t, d, "what usually happens after make test")
	if en.Say != "After make test, make deploy usually follows." {
		t.Fatalf("the English question should reach the same successor: %q", en.Say)
	}
}

// A Turkish question about a gap admits the gap in Turkish.
func TestTurkishIgnorance(t *testing.T) {
	d, _ := testDaemon(t)
	resp := say(t, d, "make zzz'den sonra ne gelir?")
	if resp.Say != "Make zzz'den sonra ne gelir, henüz bilmiyorum." {
		t.Fatalf("Turkish ignorance should speak Turkish: %q", resp.Say)
	}
}

// A Turkish definition reads the concept layer and speaks the copula
// with its harmony, tally included.
func TestTurkishDefinition(t *testing.T) {
	d, s := testDaemon(t)
	feedSequence(t, d, "run: make test", "run: make deploy", 4, time.Now().Add(-4*time.Hour))
	if _, err := s.MergeDuplicates(); err != nil {
		t.Fatal(err)
	}
	calm(d)
	if born, _ := d.growConcepts(); born == 0 {
		t.Fatal("the merged shapes should distil into concepts")
	}

	resp := say(t, d, "make test nedir")
	want := "Make test bir eylemdir. Onu dört kez gördüm."
	if resp.Say != want {
		t.Fatalf("Turkish definition wrong: got %q want %q", resp.Say, want)
	}
}
