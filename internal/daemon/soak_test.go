package daemon

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"golem/internal/grammar"
	"golem/internal/limbic"
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
		// The copula and event frames over a real concept name must all be covered by the
		// declarative engine now that the coded be/event paths are gone; a decline would mean a
		// stranded shape with no realizer.
		base := limbic.State{Oxytocin: limbic.OxytocinBaseline}
		check := func(name string, fr grammar.Frame) {
			if name == "" {
				return
			}
			fr.Agent = grammar.Term{Phrase: name}
			for _, past := range []bool{false, true} {
				fr.Past = past
				if _, ok := d.gram.RealizeDecl(fr, base); !ok {
					t.Errorf("RealizeDecl should cover the %s %s frame for %q", fr.Lang, fr.Action.Phrase, name)
				}
			}
		}
		enName := d.conceptName(c.ID, c.Content, "en")
		check(enName, grammar.Frame{Action: grammar.Term{Phrase: "be"}, Force: grammar.ForceStatement, Lang: "en", Patient: grammar.Term{Phrase: grammar.Indefinite("thing")}})
		check(enName, grammar.Frame{Action: grammar.Term{Phrase: "follow"}, Force: grammar.ForceStatement, Lang: "en"})
		check(d.conceptName(c.ID, c.Content, "tr"), grammar.Frame{Action: grammar.Term{Phrase: "follow"}, Force: grammar.ForceStatement, Lang: "tr"})
	}
	log := buf.String()
	covered := strings.Count(log, "declgen ok")
	declined := strings.Count(log, "declgen decline")
	var mismatches []string
	for _, line := range strings.Split(log, "\n") {
		if strings.Contains(line, "mismatch") {
			mismatches = append(mismatches, line)
		}
	}
	total := covered + declined
	pct := 0.0
	if total > 0 {
		pct = 100 * float64(covered) / float64(total)
	}
	t.Logf("soak: %d concepts, declgen %d clauses (%d covered %.0f%%, %d declined), %d mismatches",
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

// TestComprehensionTraversalSoak generates a clause from every stored proposition and reads it
// back through both comprehension paths, failing on any divergence between the construction
// traversal and the coded comprehender. It measures the traversal's agreement at scale (every
// relation and concept the brain holds, both tongues) before the cutover. Env-gated on
// GOLEM_SOAK_DB, on a copy of the snapshot so it is not touched.
func TestComprehensionTraversalSoak(t *testing.T) {
	dbPath := os.Getenv("GOLEM_SOAK_DB")
	if dbPath == "" {
		t.Skip("set GOLEM_SOAK_DB to a snapshot path to soak the comprehension traversal")
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
	teachTurkishMorphology(t, d) // so a Turkish clause generates with the learned verb morphology

	concepts, err := s.Concepts()
	if err != nil {
		t.Fatal(err)
	}
	covered, declined := 0, 0
	var mismatches []string
	qCovered := 0
	var qMismatch []string
	for _, c := range concepts {
		for _, f := range d.gatherFacts(c, "agent", "") {
			// An asked comparison goes through its own traversal recognizer, so a comparison told
			// is rebuilt as the question "is a X bigger than a Y" and read both ways. A placement
			// renders as a copula, not the verb clause the round-trip below reads, so it is left to
			// that path's own soak.
			if d.factIsComparative(f.rel) {
				subj := d.conceptName(c.ID, c.Content, "en")
				cmpRel := d.resolveRelation(f.rel)
				cf, err := s.FragmentsByIDs([]uint64{f.coID})
				if err != nil || len(cf) != 1 || cmpRel == nil || subj == "" || strings.Contains(subj, " ") {
					continue
				}
				std := d.conceptName(f.coID, cf[0].Content, "en")
				relName := d.relationName(cmpRel.Content, "en")
				if std == "" || strings.Contains(std, " ") || relName == "" || strings.Contains(relName, " ") {
					continue
				}
				q := "is " + grammar.Indefinite(subj) + " " + relName + " than " + grammar.Indefinite(std)
				coded := d.gram.ParseHint(q, "en", d.resolveConcept)
				if walked, wok := d.gram.ParseByTraversal("en", q, d.resolveConcept); wok {
					if framesAgree(walked, coded) {
						qCovered++
					} else {
						qMismatch = append(qMismatch, q)
					}
				}
				continue
			}
			if d.factIsLocative(f.rel) {
				continue
			}
			rel := d.resolveRelation(f.rel)
			if rel == nil {
				continue
			}
			frags, err := s.FragmentsByIDs([]uint64{f.coID})
			if err != nil || len(frags) != 1 {
				continue
			}
			patient := frags[0]
			for _, lang := range []string{"en", "tr"} {
				m := grammar.Meaning{
					Relation: grammar.Term{Phrase: d.relationName(rel.Content, lang)},
					Past:     true,
					Lang:     lang,
					Roles: map[string]grammar.Term{
						"agent":   {Phrase: d.conceptName(c.ID, c.Content, lang), Definite: true},
						"patient": {Phrase: d.conceptName(patient.ID, patient.Content, lang), Definite: true},
					},
				}
				clause, ok := d.gram.Speak(lang, m)
				if !ok {
					continue
				}
				coded, cok := d.gram.Comprehend(lang, clause, d.resolveConcept, d.roleFit)
				if !cok {
					continue // the coded path cannot read its own generated clause; out of this slice's scope
				}
				walked, wok := d.gram.ComprehendByTraversal(lang, clause, d.resolveConcept, d.roleFit)
				if !wok {
					declined++
					continue
				}
				if meaningsAgree(walked, coded) {
					covered++
				} else {
					mismatches = append(mismatches, clause+": walked "+meaningGloss(walked)+" vs coded "+meaningGloss(coded))
				}
			}

			// wh role-question (subject-wh): "who <verb> a <patient>", read both ways.
			if pn := d.conceptName(patient.ID, patient.Content, "en"); pn != "" && !strings.Contains(pn, " ") {
				q := "who " + d.relationName(rel.Content, "en") + " " + grammar.Indefinite(pn)
				coded := d.gram.ParseHint(q, "en", d.resolveConcept)
				if walked, wok := d.gram.ParseByTraversal("en", q, d.resolveConcept); wok {
					if framesAgree(walked, coded) {
						qCovered++
					} else {
						qMismatch = append(qMismatch, q)
					}
				}
			}
		}

		// is-a questions: for each category the concept belongs to, ask it both ways and confirm
		// the question traversal reads the frame the coded parser does.
		cName := d.conceptName(c.ID, c.Content, "en")
		if cName != "" && !strings.Contains(cName, " ") {
			for _, catID := range d.categoriesOf(c.ID) {
				cf, err := s.FragmentsByIDs([]uint64{catID})
				if err != nil || len(cf) != 1 {
					continue
				}
				catName := d.conceptName(catID, cf[0].Content, "en")
				if catName == "" || strings.Contains(catName, " ") {
					continue
				}
				q := "is " + grammar.Indefinite(cName) + " " + grammar.Indefinite(catName)
				coded := d.gram.ParseHint(q, "en", d.resolveConcept)
				if walked, wok := d.gram.ParseByTraversal("en", q, d.resolveConcept); wok {
					if framesAgree(walked, coded) {
						qCovered++
					} else {
						qMismatch = append(qMismatch, q)
					}
				}
			}
		}
	}
	total := covered + declined + len(mismatches)
	pct := 0.0
	if total > 0 {
		pct = 100 * float64(covered) / float64(total)
	}
	t.Logf("comprehension soak: %d concepts, %d clauses (%d covered %.0f%%, %d declined, %d mismatch)",
		len(concepts), total, covered, pct, declined, len(mismatches))
	for i, m := range mismatches {
		if i >= 20 {
			break
		}
		t.Logf("  %s", m)
	}
	t.Logf("  questions (is-a + wh): %d covered, %d mismatch", qCovered, len(qMismatch))
	for i, q := range qMismatch {
		if i >= 20 {
			break
		}
		t.Logf("    %s", q)
	}
	if len(mismatches) > 0 || len(qMismatch) > 0 {
		t.Fatalf("the traversal diverged: %d statement clauses, %d is-a questions", len(mismatches), len(qMismatch))
	}
}

func teachTurkishMorphology(t *testing.T, d *Daemon) {
	t.Helper()
	rules := map[string][][2]string{
		"past":    {{"kovala", "kovaladı"}, {"gel", "geldi"}, {"oku", "okudu"}, {"gör", "gördü"}, {"bak", "baktı"}, {"geç", "geçti"}},
		"aorist":  {{"kovala", "kovalar"}, {"oku", "okur"}, {"yürü", "yürür"}, {"ara", "arar"}},
		"aoristC": {{"gel", "gelir"}, {"bak", "bakır"}, {"gör", "görür"}, {"bul", "bulur"}},
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
