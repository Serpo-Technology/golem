package grammar

import (
	"testing"

	"golem/internal/limbic"
)

// Vowel harmony and voicing assimilation, on real anchor shapes: the
// last written vowel picks the suffix vowel, a voiceless final
// consonant hardens the D.
func TestTurkishSuffixHarmony(t *testing.T) {
	ablatives := map[string]string{
		"make test":   "make test'ten",   // front vowel, voiceless t
		"make deploy": "make deploy'dan", // back vowel, voiced y
		"make push":   "make push'tan",   // back round u, voiceless h
		"make lint":   "make lint'ten",
		"log":         "log'dan",
	}
	for w, want := range ablatives {
		if got := ablativeTR(w); got != want {
			t.Fatalf("ablative of %q = %q, want %q", w, got, want)
		}
	}

	accusatives := map[string]string{
		"make test":   "make test'i",
		"make deploy": "make deploy'u",
		"elma":        "elma'yı", // vowel-final takes the buffer y
		"ütü":         "ütü'yü",
	}
	for w, want := range accusatives {
		if got := accusativeTR(w); got != want {
			t.Fatalf("accusative of %q = %q, want %q", w, got, want)
		}
	}

	copulas := map[string]string{
		"eylem": "eylemdir",
		"şey":   "şeydir",
	}
	for w, want := range copulas {
		if got := copulaTR(w); got != want {
			t.Fatalf("copula on %q = %q, want %q", w, got, want)
		}
	}

	// A clock time harmonizes by the spoken form of its last digit.
	times := map[string]string{
		"09:14": "09:14'te", // dört: voiceless, front
		"09:10": "09:10'da", // on: voiced, back
		"09:12": "09:12'de", // iki: voiced, front
		"09:13": "09:13'te", // üç: voiceless, front
	}
	for w, want := range times {
		if got := timeLocativeTR(w); got != want {
			t.Fatalf("time locative of %q = %q, want %q", w, got, want)
		}
	}
}

// The SOV parser reads the anchor from before its relator, strips the
// case suffix, and lands in the same frame the English parser fills.
func TestTurkishParse(t *testing.T) {
	f := Parse("make test'ten sonra ne gelir", nil)
	if f.Lang != "tr" || f.Force != ForceQuestion || f.Wh != "what" {
		t.Fatalf("a Turkish question should be detected and typed: %+v", f)
	}
	rel, anchor := f.FirstQual()
	if rel != "after" || anchor.Phrase != "make test" {
		t.Fatalf("the anchor precedes its relator in SOV: %q %q", rel, anchor.Phrase)
	}

	g := Parse("make deploy'dan önce ne olur", nil)
	grel, ganchor := g.FirstQual()
	if grel != "before" || ganchor.Phrase != "make deploy" {
		t.Fatalf("before-questions parse the same way: %q %q", grel, ganchor.Phrase)
	}

	h := Parse("make test nedir", nil)
	if h.Lang != "tr" || h.Force != ForceQuestion || h.Action.Phrase != "be" {
		t.Fatalf("nedir asks for a definition: %+v", h)
	}
	if h.Patient.Phrase != "make test" {
		t.Fatalf("the definition topic precedes nedir: %q", h.Patient.Phrase)
	}

	c := Parse("make test kaç kez oldu", nil)
	if !c.Counting || !c.Past || c.Patient.Phrase != "make test" {
		t.Fatalf("kaç kez counts in the past: %+v", c)
	}

	w := Parse("ne zaman make test oldu", nil)
	if w.Wh != "when" || !w.Past {
		t.Fatalf("ne zaman asks when: %+v", w)
	}
}

// The Turkish realizer speaks SOV with the morphology carrying what
// English carries with word order: assertion, hedging, and denial all
// end on the verb.
func TestTurkishRealize(t *testing.T) {
	calmChem := limbic.State{Oxytocin: limbic.OxytocinBaseline}
	f := Frame{
		Lang:   "tr",
		Force:  ForceStatement,
		Agent:  Term{Phrase: "make deploy"},
		Action: Term{Phrase: "follow"},
		Habit:  true,
		Quals:  []Qualifier{{Rel: "after", Anchor: Term{Phrase: "make test"}}},
	}
	if got := Realize(f, calmChem); got != "Make test'ten sonra genellikle make deploy gelir." {
		t.Fatalf("assertive Turkish form wrong: %q", got)
	}
	if got := Realize(f, limbic.State{Oxytocin: 0.2}); got != "Make test'ten sonra make deploy gelebilir." {
		t.Fatalf("hedged Turkish form wrong: %q", got)
	}
	f.Negate = true
	f.Habit = false
	if got := Realize(f, calmChem); got != "Make test'ten sonra make deploy gelmez." {
		t.Fatalf("denied Turkish form wrong: %q", got)
	}

	ign := Frame{Lang: "tr", Force: ForceStatement, Action: Term{Phrase: "know"}, Negate: true,
		Quals: []Qualifier{{Rel: "after", Anchor: Term{Phrase: "make zzz"}}}}
	if got := Realize(ign, calmChem); got != "Make zzz'den sonra ne gelir, henüz bilmiyorum." {
		t.Fatalf("Turkish ignorance form wrong: %q", got)
	}

	def := Frame{Lang: "tr", Force: ForceStatement, Agent: Term{Phrase: "make test"},
		Action: Term{Phrase: "be"}, Patient: Term{Phrase: "bir eylem"}}
	if got := Realize(def, calmChem); got != "Make test bir eylemdir." {
		t.Fatalf("Turkish definition form wrong: %q", got)
	}

	count := Frame{Lang: "tr", Force: ForceStatement, Action: Term{Phrase: "see"},
		Patient: Term{Phrase: "make test"}, Times: 4}
	if got := Realize(count, calmChem); got != "Make test'i dört kez gördüm." {
		t.Fatalf("Turkish count form wrong: %q", got)
	}
}
