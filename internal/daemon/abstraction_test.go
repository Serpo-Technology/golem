package daemon

import (
	"testing"
	"time"

	"golem/internal/attention"
	"golem/internal/memory"
	"golem/wire"
)

// constellationDay builds two concrete concepts from lived shapes and
// returns one binding anchor per shape per situation.
func constellationDay(t *testing.T, d *Daemon, s *memory.Store) (disk, deploy []recentMoment) {
	t.Helper()
	base := time.Now().Add(-3 * time.Hour)
	disk = feedShape(t, s, "syslog", "Disk full on /dev/disk1s9", 4, base)
	deploy = feedShape(t, s, "shell", "run: make deploy", 4, base.Add(time.Minute))
	if _, err := s.MergeDuplicates(); err != nil {
		t.Fatal(err)
	}
	calm(d)
	d.growConcepts()
	return disk, deploy
}

// A word used across many situations with a consistent conceptual
// neighbourhood, never tied to a single sensory shape, distils into an
// abstract concept grounded in its constellation.
func TestAbstractionByConstellation(t *testing.T) {
	d, s := testDaemon(t)
	disk, deploy := constellationDay(t, d, s)

	// "trouble" keeps company with both shapes, three situations over.
	for i := range 3 {
		d.mintWords("trouble", "en", 0, []recentMoment{disk[i], deploy[i]}, false)
	}
	calm(d)
	d.growConcepts() // the naming pass weighs the word's company
	born := d.growAbstractions()
	if born != 1 {
		t.Fatalf("the stable constellation should mint one abstraction, got %d", born)
	}

	abs, err := s.ConceptByTemplate("abs:en:trouble")
	if err != nil || abs == nil {
		t.Fatalf("the abstraction should exist: %v", err)
	}
	if abs.Content != "trouble" || abs.Category != "noun" {
		t.Fatalf("the abstraction speaks as its word: %+v", abs)
	}

	// Grounded in concepts, and through them in concrete experience: the
	// whole chain stays auditable.
	grounds, err := s.LinksFromRel([]uint64{abs.ID}, memory.RelGrounds)
	if err != nil || len(grounds) != 2 {
		t.Fatalf("the abstraction should ground its two core concepts: %v %v", grounds, err)
	}
	for _, g := range grounds {
		core, err := s.FragmentsByIDs([]uint64{g.To})
		if err != nil || len(core) != 1 || core[0].Kind != memory.KindConcept {
			t.Fatalf("a constellation member should be a concept: %+v", core)
		}
		under, err := s.LinksFromRel([]uint64{g.To}, memory.RelGrounds)
		if err != nil || len(under) == 0 {
			t.Fatalf("the chain should unfold to episodes underneath: %v", err)
		}
		episodes, _ := s.FragmentsByIDs([]uint64{under[0].To})
		if len(episodes) != 1 || episodes[0].Kind != memory.KindEpisodic {
			t.Fatalf("the chain should bottom out in lived moments: %+v", episodes)
		}
		if g.Weight < absCoreFloor || g.Weight >= absConcentration {
			t.Fatalf("the grounding weight should be the reliability share: %.2f", g.Weight)
		}
	}

	// The word names its abstraction firmly.
	if w := edgeWeightRel(t, s, wordID(t, s, "trouble"), abs.ID, memory.RelNames); w != absNameWeight {
		t.Fatalf("the word should name its abstraction at %.2f, got %.2f", absNameWeight, w)
	}

	// Genesis is idempotent: the next night refreshes, never re-mints.
	if again := d.growAbstractions(); again != 0 {
		t.Fatalf("a held constellation must not mint twice, got %d", again)
	}
}

// An inflected form of a relation verb keeps company with the verb's
// arguments in every telling, which would otherwise distil a noun shadowing
// the relation in the naming vote. The relation is keyed on the lemma, so the
// guard must reach it through the inflection's lemma: "chased" is skipped
// because "chase" names a relation, while an ordinary word over the same
// company still abstracts.
func TestAbstractionSkipsInflectedRelationVerb(t *testing.T) {
	d, s := testDaemon(t)
	disk, deploy := constellationDay(t, d, s)
	now := time.Now()

	rel, err := s.Encode(&memory.Fragment{Kind: memory.KindConcept, Content: "chase",
		Template: "rel:en:chase", Category: "relation", Time: now})
	if err != nil {
		t.Fatal(err)
	}
	w, _, err := s.WordNode("chase", "en", d.chem.Snapshot(), now)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.SetLinkRel(w.ID, rel, memory.RelNames, opNameWeight); err != nil {
		t.Fatal(err)
	}
	// The inflected surface reaches the relation only through its lemma; the
	// guard relies on that, so assert it before leaning on it.
	if got := d.resolveRelation(d.gram.Lemma("en", "chased")); got == nil || got.ID != rel {
		t.Fatalf("the inflection should resolve to the relation through its lemma: %+v", got)
	}

	// Both words keep the same company across three situations, the shape that
	// distils a constellation. "chased" is the verb's inflection and must be
	// skipped; "trouble" is an ordinary word and must still abstract, proving the
	// pass ran and the company was abstraction-worthy.
	for i := range 3 {
		d.mintWords("chased", "en", 0, []recentMoment{disk[i], deploy[i]}, false)
		d.mintWords("trouble", "en", 0, []recentMoment{disk[i], deploy[i]}, false)
	}
	calm(d)
	d.growConcepts()

	if born := d.growAbstractions(); born != 1 {
		t.Fatalf("only the ordinary word should abstract, got %d", born)
	}
	if abs, _ := s.ConceptByTemplate("abs:en:chased"); abs != nil {
		t.Fatalf("an inflection of a relation verb must not mint a shadow noun: %+v", abs)
	}
	if abs, _ := s.ConceptByTemplate("abs:en:trouble"); abs == nil {
		t.Fatal("the ordinary word over the same company should still abstract")
	}
}

// A verb names its relation, not the arguments it keeps company with. The
// naming pass clears a relation-word's bond to an argument noun while leaving
// its bond to the relation, so however often the verb is taught through
// propositions the naming vote stays on the relation and does not drift to the
// nouns the verb took.
func TestNamingClearsVerbToArgumentBond(t *testing.T) {
	d, s := testDaemon(t)
	disk, _ := constellationDay(t, d, s)
	now := time.Now()

	rel, err := s.Encode(&memory.Fragment{Kind: memory.KindConcept, Content: "chase",
		Template: "rel:en:chase", Category: "relation", Time: now})
	if err != nil {
		t.Fatal(err)
	}
	w, _, err := s.WordNode("chase", "en", d.chem.Snapshot(), now)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.SetLinkRel(w.ID, rel, memory.RelNames, opNameWeight); err != nil {
		t.Fatal(err)
	}

	argConcept, err := s.ConceptByTemplate(attention.Normalize("Disk full on /dev/disk1s9"))
	if err != nil || argConcept == nil {
		t.Fatalf("the argument concept should exist: %v", err)
	}
	argID := argConcept.ID

	// The verb keeps company with the argument across situations, the shape that
	// would otherwise bond it to the noun, and a stale bond already stands.
	for i := range 3 {
		d.mintWords("chase", "en", 0, []recentMoment{disk[i]}, false)
	}
	if err := s.SetLinkRel(w.ID, argID, memory.RelNames, 0.5); err != nil {
		t.Fatal(err)
	}

	calm(d)
	d.growConcepts()

	if got := edgeWeightRel(t, s, w.ID, rel, memory.RelNames); got <= 0 {
		t.Fatalf("the verb should keep naming its relation, got %.2f", got)
	}
	if got := edgeWeightRel(t, s, w.ID, argID, memory.RelNames); got != 0 {
		t.Fatalf("the verb's bond to an argument noun should be cleared, got %.2f", got)
	}
}

// wordID resolves a word node's id by form.
func wordID(t *testing.T, s *memory.Store, form string) uint64 {
	t.Helper()
	words, err := s.WordNodesByForms([]string{form})
	if err != nil || len(words) != 1 {
		t.Fatalf("expected one word node for %q: %v %v", form, words, err)
	}
	return words[0].ID
}

// No abstraction without spread and recurrence: a word faithful to one
// shape is a name, and a word said once knows nothing yet.
func TestAbstractionNeedsSpreadAndRecurrence(t *testing.T) {
	d, s := testDaemon(t)
	disk, deploy := constellationDay(t, d, s)

	// Concentrated: five situations, but the company is mostly one shape.
	for i := range 4 {
		d.mintWords("disky", "en", 0, []recentMoment{disk[i%len(disk)]}, false)
	}
	d.mintWords("disky", "en", 0, []recentMoment{deploy[0]}, false)

	// Sparse: a stable pair of concepts, but only one showing.
	d.mintWords("rare", "en", 0, []recentMoment{disk[0], deploy[0]}, false)

	calm(d)
	d.growConcepts()
	if born := d.growAbstractions(); born != 0 {
		t.Fatalf("neither word should distil, got %d", born)
	}
	if c, _ := s.ConceptByTemplate("abs:en:disky"); c != nil {
		t.Fatal("a word faithful to one shape must stay a name")
	}
	if c, _ := s.ConceptByTemplate("abs:en:rare"); c != nil {
		t.Fatal("one showing must not mint an abstraction")
	}
}

// A word taught faithfully across many members abstracts even though no single
// member dominates its company. The shares spread thin, each well under the old
// absolute core bar, yet judged against the strongest member they still gather
// a core, the case where a better-taught category used to fail to distil.
func TestAbstractionFromBroadNaming(t *testing.T) {
	d, s := testDaemon(t)
	base := time.Now().Add(-3 * time.Hour)
	shapes := []string{
		"a brass key", "a brass lantern", "a round copper coin",
		"a small iron nail", "a steel bolt", "a tin cup",
	}
	members := make([][]recentMoment, len(shapes))
	for i, shape := range shapes {
		members[i] = feedShape(t, s, "vision", shape, 4, base.Add(time.Duration(i)*time.Minute))
	}
	if _, err := s.MergeDuplicates(); err != nil {
		t.Fatal(err)
	}
	calm(d)
	d.growConcepts()

	// "metal" keeps company with all six across three situations, each member
	// holding about a sixth of the word's company, under the old 0.2 floor.
	for i := range 3 {
		anchors := make([]recentMoment, 0, len(members))
		for _, m := range members {
			anchors = append(anchors, m[i])
		}
		d.mintWords("metal", "en", 0, anchors, false)
	}
	calm(d)
	d.growConcepts()

	if born := d.growAbstractions(); born != 1 {
		t.Fatalf("a broad category should still distil one abstraction, got %d", born)
	}
	abs, err := s.ConceptByTemplate("abs:en:metal")
	if err != nil || abs == nil {
		t.Fatalf("the broad category should mint an abstraction: %v", err)
	}
	grounds, err := s.LinksFromRel([]uint64{abs.ID}, memory.RelGrounds)
	if err != nil || len(grounds) < 4 {
		t.Fatalf("it should ground most of its members, got %d (%v)", len(grounds), err)
	}
}

// Recall reaches concrete experience through the constellation: the
// query word names the abstraction, the abstraction grounds concepts,
// and the concepts ground the moments the reply returns.
func TestRecallReachesThroughConstellation(t *testing.T) {
	d, s := testDaemon(t)
	disk, deploy := constellationDay(t, d, s)
	for i := range 3 {
		d.mintWords("trouble", "en", 0, []recentMoment{disk[i], deploy[i]}, false)
	}
	calm(d)
	d.growConcepts()
	if born := d.growAbstractions(); born != 1 {
		t.Fatalf("expected the abstraction, got %d", born)
	}

	resp, _, _ := d.handle(wire.Request{Cmd: "recall", Query: "trouble"}, nil)
	var lived, throughAbs bool
	for _, f := range resp.Fragments {
		if f.Kind == "" && (f.Channel == "syslog" || f.Channel == "shell") {
			lived = true
		}
		if f.Kind == memory.KindConcept && f.Content == "trouble" {
			throughAbs = true
		}
	}
	if !lived {
		t.Fatalf("recall should reach lived moments through the constellation: %+v", resp.Fragments)
	}
	if !throughAbs {
		t.Fatalf("the reply should show the abstraction meaning walked through: %+v", resp.Fragments)
	}
}

// Window-based disconfirmation does not apply to an abstraction: its
// referent is never directly in a sensory window, so a stretch of
// one-sided company leaves the bond alone while an ordinary stale bond
// still weakens.
func TestAbstractionBondSurvivesDepression(t *testing.T) {
	d, s := testDaemon(t)
	disk, deploy := constellationDay(t, d, s)
	for i := range 3 {
		d.mintWords("trouble", "en", 0, []recentMoment{disk[i], deploy[i]}, false)
	}
	calm(d)
	d.growConcepts()
	if born := d.growAbstractions(); born != 1 {
		t.Fatalf("expected the abstraction, got %d", born)
	}
	abs, _ := s.ConceptByTemplate("abs:en:trouble")
	deployConcept, _ := s.ConceptByTemplate("run: make deploy")
	if abs == nil || deployConcept == nil {
		t.Fatal("both concepts should exist")
	}
	w := wordID(t, s, "trouble")
	absBefore := edgeWeightRel(t, s, w, abs.ID, memory.RelNames)
	deployBefore := edgeWeightRel(t, s, w, deployConcept.ID, memory.RelNames)
	if deployBefore == 0 {
		t.Fatal("the word should hold an ordinary bond to the deploy concept")
	}

	// The word recurs amid disk company alone: the deploy bond is stale
	// and weakens; the abstraction's bond stands.
	d.mintWords("trouble", "en", 0, []recentMoment{disk[3]}, false)
	if got := edgeWeightRel(t, s, w, abs.ID, memory.RelNames); got != absBefore {
		t.Fatalf("the abstraction bond must not depress: %.2f -> %.2f", absBefore, got)
	}
	if got := edgeWeightRel(t, s, w, deployConcept.ID, memory.RelNames); got >= deployBefore {
		t.Fatalf("the stale concrete bond should weaken: %.2f -> %.2f", deployBefore, got)
	}
}
