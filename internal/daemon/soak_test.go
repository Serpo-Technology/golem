package daemon

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"golem/internal/memory"
)

// TestDeclShadowSoak drives the live renderFacts shadow over every concept's facts in a real
// trained brain and fails on any divergence between the declarative engine and the coded Speak.
// It proves the shadow's parity in the wild before the cutover, not just on the unit corpus. It
// runs only when GOLEM_SOAK_DB points at a snapshot, and works on a copy so the snapshot is not
// touched. The Turkish morphology rules are taught first (a brain from before them has none), so
// both tongues exercise the engine.
func TestDeclShadowSoak(t *testing.T) {
	dbPath := os.Getenv("GOLEM_SOAK_DB")
	if dbPath == "" {
		t.Skip("set GOLEM_SOAK_DB to a snapshot path to soak the declarative shadow")
	}
	tmp := filepath.Join(t.TempDir(), "soak.db")
	data, err := os.ReadFile(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		t.Fatal(err)
	}
	s, err := memory.OpenStore(tmp)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })

	d := testDaemonOn(t, s)
	d.shadowVerbose = true // report every clause covered or declined, to measure coverage
	teachTurkishMorphology(t, d)

	concepts, err := s.Concepts()
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	d.out = &buf
	for _, c := range concepts {
		facts := d.gatherFacts(c, "", "")
		if len(facts) == 0 {
			continue
		}
		for _, lang := range []string{"en", "tr"} {
			name := d.conceptName(c.ID, c.Content, lang)
			for _, past := range []bool{true, false} {
				// continuation true makes the focus speak as its pronoun, exercising that path too
				for _, cont := range []bool{false, true} {
					d.renderFacts(lang, name, facts, past, cont)
				}
			}
		}
	}
	log := buf.String()
	covered := strings.Count(log, "declgen ok")
	declined := strings.Count(log, "declgen decline")
	var mismatches []string
	for _, line := range strings.Split(log, "\n") {
		if strings.Contains(line, "declgen mismatch") {
			mismatches = append(mismatches, line)
		}
	}
	total := covered + declined
	pct := 0.0
	if total > 0 {
		pct = 100 * float64(covered) / float64(total)
	}
	t.Logf("soak: %d concepts, %d clauses (%d covered %.0f%%, %d declined), %d mismatches",
		len(concepts), total, covered, pct, declined, len(mismatches))
	shown := 0
	for _, line := range strings.Split(log, "\n") {
		if strings.Contains(line, "declgen decline") && shown < 12 {
			t.Logf("  %s", line)
			shown++
		}
	}
	for i, m := range mismatches {
		if i >= 20 {
			break
		}
		t.Logf("  %s", m)
	}
	if len(mismatches) > 0 {
		t.Fatalf("the declarative shadow diverged on %d live clauses", len(mismatches))
	}
}

func teachTurkishMorphology(t *testing.T, d *Daemon) {
	t.Helper()
	rules := map[string][][2]string{
		"past":    {{"kovala", "kovaladı"}, {"gel", "geldi"}, {"oku", "okudu"}, {"gör", "gördü"}, {"bak", "baktı"}, {"geç", "geçti"}},
		"neg":     {{"kovala", "kovalama"}, {"gel", "gelme"}, {"oku", "okuma"}, {"gör", "görme"}},
		"pers1sg": {{"kovaladı", "kovaladım"}, {"geldi", "geldim"}, {"okudu", "okudum"}, {"gördü", "gördüm"}},
		"pers2sg": {{"kovaladı", "kovaladın"}, {"geldi", "geldin"}, {"okudu", "okudun"}, {"gördü", "gördün"}},
		"pers1pl": {{"kovaladı", "kovaladık"}, {"geldi", "geldik"}, {"okudu", "okuduk"}, {"gördü", "gördük"}},
	}
	for feature, pairs := range rules {
		if err := d.teachInflection("tr", feature, pairs); err != nil {
			t.Fatalf("teaching tr %s: %v", feature, err)
		}
	}
}
