package daemon

import (
	"strings"
	"testing"
	"time"

	"golem/internal/memory"
)

// namedConceptTR seeds a Turkish concept named by its proper (diacritic) form, the
// lexicon the fold resolves against.
func namedConceptTR(t *testing.T, d *Daemon, s *memory.Store, gloss, word string) uint64 {
	t.Helper()
	id, err := s.Encode(&memory.Fragment{Kind: memory.KindConcept, Content: gloss,
		Template: "noun:tr:" + gloss, Category: "noun", Time: time.Now()})
	if err != nil {
		t.Fatal(err)
	}
	d.nameConceptWord(id, word, "tr")
	return id
}

// An ASCII Turkish token that misses exactly folds to its proper form and resolves to
// the concept GOLEM already holds, minting nothing: no second concept and no ASCII
// word row, so "buyuk" exists only as a lossy view of "büyük".
func TestFoldResolveSingleFormHitsExistingConcept(t *testing.T) {
	d, s := testDaemon(t)
	d.foldEnabled = true
	buyuk := namedConceptTR(t, d, s, "büyük", "büyük")

	got := d.resolveConceptLang("buyuk", "tr")
	if got == nil || got.ID != buyuk {
		t.Fatalf("resolveConceptLang(buyuk, tr) should resolve to the existing büyük %d, got %+v", buyuk, got)
	}
	if words, err := s.WordNodesByForms([]string{"buyuk"}); err != nil || len(words) != 0 {
		t.Fatalf("resolving buyuk must not mint a word row, got %d rows (err %v)", len(words), err)
	}
	note, ok := d.restored["buyuk"]
	if !ok || note.conceptID != buyuk {
		t.Fatalf("the fold should record buyuk -> büyük for the mint/naming sites, got %+v (present %v)", note, ok)
	}
	if note.conf >= weakHedgeConf {
		t.Fatalf("a restored note's confidence must sit below weakHedgeConf, got %v", note.conf)
	}
}

// The decline belt holds an ambiguous minimal pair as a gap rather than guessing: when
// an ASCII miss folds to two DISTINCT known forms ("sık" and "şık"), the resolver
// returns nil and records no restoration, mirroring the reference-conflict discipline.
func TestFoldResolveDeclinesMinimalPair(t *testing.T) {
	d, s := testDaemon(t)
	d.foldEnabled = true
	namedConceptTR(t, d, s, "sık", "sık") // tight / frequent
	namedConceptTR(t, d, s, "şık", "şık") // chic

	if got := d.resolveConceptLang("sik", "tr"); got != nil {
		t.Fatalf("an ambiguous minimal-pair fold should decline (nil), got %+v", got)
	}
	if _, ok := d.restored["sik"]; ok {
		t.Fatalf("a declined fold should record no restoration note")
	}
}

// A single surviving form whose several concepts are ordinary polysemy is NOT declined:
// the belt keys on distinct FORMS, so one form's polysemy proceeds to the warmth vote,
// which elects the warmer concept the same way the exact path would.
func TestFoldResolveSingleFormPolysemyProceeds(t *testing.T) {
	d, s := testDaemon(t)
	d.foldEnabled = true
	// One proper form "yüz" naming two concepts (face, hundred); "yuz" folds to only
	// this one form, so the belt lets the vote pick, not decline.
	face, err := s.Encode(&memory.Fragment{Kind: memory.KindConcept, Content: "yüz-face",
		Template: "noun:tr:yüz-face", Category: "noun", Time: time.Now()})
	if err != nil {
		t.Fatal(err)
	}
	hundred, err := s.Encode(&memory.Fragment{Kind: memory.KindConcept, Content: "yüz-hundred",
		Template: "noun:tr:yüz-hundred", Category: "noun", Time: time.Now()})
	if err != nil {
		t.Fatal(err)
	}
	w, _, err := s.WordNode("yüz", "tr", d.chem.Snapshot(), time.Now())
	if err != nil {
		t.Fatal(err)
	}
	// The face reading is the warmer name; the vote should elect it.
	if err := s.SetLinkRel(w.ID, face, memory.RelNames, 0.9); err != nil {
		t.Fatal(err)
	}
	if err := s.SetLinkRel(w.ID, hundred, memory.RelNames, 0.5); err != nil {
		t.Fatal(err)
	}
	got := d.resolveConceptLang("yuz", "tr")
	if got == nil || got.ID != face {
		t.Fatalf("a single-form polysemy should proceed to the warmth vote (face %d), got %+v", face, got)
	}
	_ = hundred
}

// The fold is confined to the main pass and to Turkish: disabled, it restores nothing
// even for tr; enabled, it never fires for English or an untagged clause, so language
// detection is never hijacked and an English clause never restores a Turkish shadow.
func TestFoldGatedByFlagAndTongue(t *testing.T) {
	d, s := testDaemon(t)
	namedConceptTR(t, d, s, "büyük", "büyük")

	d.foldEnabled = false
	if got := d.resolveConceptLang("buyuk", "tr"); got != nil {
		t.Fatalf("with the fold disabled, buyuk should not restore, got %+v", got)
	}

	d.foldEnabled = true
	if got := d.resolveConceptLang("buyuk", "en"); got != nil {
		t.Fatalf("the fold is Turkish-gated; buyuk under en should not restore, got %+v", got)
	}
	if got := d.resolveConceptLang("buyuk", ""); got != nil {
		t.Fatalf("the fold is Turkish-gated; buyuk under empty lang should not restore, got %+v", got)
	}
}

// The word-form mint suppresses a surface the fold restored this turn, so no KindWord
// row is forked for the lossy ASCII form, while an ordinary token in the same utterance
// still mints.
func TestMintSuppressesRestoredForm(t *testing.T) {
	d, s := testDaemon(t)
	d.restored = map[string]restoredNote{"buyuk": {conceptID: 1, conf: restoredConf}}

	words := d.mintWordsScaled("buyuk kedi", "tr", 0, nil, false, 1.0)
	for _, w := range words {
		if strings.EqualFold(w.Content, "buyuk") {
			t.Fatalf("mintWordsScaled must not mint a row for a restored surface, got %q", w.Content)
		}
	}
	if rows, err := s.WordNodesByForms([]string{"buyuk"}); err != nil || len(rows) != 0 {
		t.Fatalf("no KindWord row should exist for the restored surface, got %d (err %v)", len(rows), err)
	}
	if rows, err := s.WordNodesByForms([]string{"kedi"}); err != nil || len(rows) == 0 {
		t.Fatalf("a non-restored token should still mint, got %d rows (err %v)", len(rows), err)
	}
}

// dropRestoredWords keeps a restored surface out of the naming company, so no RelNames
// alias from a lossy ASCII word to the proper concept is ever born.
func TestDropRestoredWordsExcludesAlias(t *testing.T) {
	d := testDaemonOn(t, mustStore(t))
	d.restored = map[string]restoredNote{"buyuk": {conceptID: 7, conf: restoredConf}}
	words := []*memory.Fragment{
		{Kind: memory.KindWord, Content: "buyuk", Lang: "tr"},
		{Kind: memory.KindWord, Content: "kedi", Lang: "tr"},
	}
	kept := d.dropRestoredWords(words)
	if len(kept) != 1 || kept[0].Content != "kedi" {
		t.Fatalf("dropRestoredWords should keep only the non-restored word, got %+v", kept)
	}
}

// The bare doubt speaks a lone restorable Turkish token through the weak hedge rail and
// the youmean carrier, verb-final in Turkish; the same render in English places the
// carrier before the form.
func TestBareRestoreDoubtRenders(t *testing.T) {
	d, s := testDaemon(t)
	d.foldEnabled = true
	namedConceptTR(t, d, s, "büyük", "büyük")

	line, ok := d.bareRestoreDoubt("tr", "buyuk")
	if !ok {
		t.Fatalf("a lone restorable Turkish token should produce a doubt")
	}
	if !strings.Contains(line, "büyük") || !strings.Contains(line, "demek istiyorsun") {
		t.Fatalf("the tr doubt should name the restored form and use the youmean carrier: %q", line)
	}
	if en := d.restoreDoubt("en", "büyük"); en != "Perhaps you mean büyük?" {
		t.Fatalf("en doubt render mismatch: %q", en)
	}
}

func mustStore(t *testing.T) *memory.Store {
	t.Helper()
	s, err := memory.OpenStore(t.TempDir() + "/m.db")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}
