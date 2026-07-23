package daemon

import (
	"fmt"
	"reflect"
	"sort"
	"testing"
	"time"

	"golem/internal/attention"
	"golem/internal/memory"
	"golem/wire"
)

// The prefetched abstraction index must reach exactly what resolving each word against the
// store directly would: the same mints, naming and grounding edges, and weights. Two
// identical brains are grown, one abstraction pass reading the index and one the store, and
// every naming and grounding edge is compared. Minting and refreshing are both exercised, so
// a word that reads a naming edge an earlier word wrote in the same pass is covered.
func TestAbstractionIndexEqualsStorePath(t *testing.T) {
	dump := func(idxOff bool) []string {
		d, s := testDaemon(t)
		d.absIdxOff = idxOff
		disk, deploy := constellationDay(t, d, s)
		for i := range 3 {
			d.mintWords("trouble", "en", 0, []recentMoment{disk[i], deploy[i]}, false)
		}
		calm(d)
		d.growConcepts()
		d.growAbstractions() // mints the abstraction
		d.growAbstractions() // refreshes the held constellation
		return namingAndGroundingEdges(t, s)
	}
	idx, store := dump(false), dump(true)
	if !reflect.DeepEqual(idx, store) {
		t.Fatalf("the index path diverged from the store path:\n index=%v\n store=%v", idx, store)
	}
	if len(idx) == 0 {
		t.Fatal("the fixture wrote no naming or grounding edges, so it proves nothing")
	}
}

// The index folds a word form to look it up; that fold must match the store's lower() exactly,
// or a word stored with a non-ASCII capital resolves one way through the index and another
// through the store. This compares the two paths of wordNamesConcept on a Turkish word across
// several query forms, so it both guards the fold and pins which fold the store actually uses.
func TestAbstractionIndexFoldMatchesStore(t *testing.T) {
	d, s := testDaemon(t)
	concept := &memory.Fragment{Kind: memory.KindConcept, Channel: "concept", Content: "noon",
		Template: "noon", Category: "noun", Time: time.Now()}
	cid, err := s.Encode(concept)
	if err != nil {
		t.Fatal(err)
	}
	word := &memory.Fragment{Kind: memory.KindWord, Channel: "word", Content: "Öğle", Lang: "tr", Time: time.Now()}
	wid, err := s.Encode(word)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.SetLinkRel(wid, cid, memory.RelNames, 1.0); err != nil {
		t.Fatal(err)
	}
	idx, err := d.buildAbstractionIndex()
	if err != nil {
		t.Fatal(err)
	}
	for _, form := range []string{"Öğle", "öğle", "OGLE", "ogle"} {
		d.absIdx = nil
		store := d.wordNamesConcept(form, cid)
		d.absIdx = idx
		index := d.wordNamesConcept(form, cid)
		d.absIdx = nil
		if store != index {
			t.Fatalf("fold mismatch for %q: store=%v index=%v", form, store, index)
		}
	}
}

// resolveRelation off the index must reach the same relation the store would: the same
// weight aggregation across a word's naming edges, the same relation-category filter, and the
// same lowest-id tie-break. This builds two tied relations and a heavier noun co-namer and
// checks the two paths agree across single, mixed-case, multi-token, and missing phrases.
func TestResolveRelationIndexMatchesStore(t *testing.T) {
	d, s := testDaemon(t)
	mkConcept := func(content, cat string) uint64 {
		c := &memory.Fragment{Kind: memory.KindConcept, Channel: "concept", Content: content,
			Template: "rel:" + content, Category: cat, Time: time.Now()}
		id, err := s.Encode(c)
		if err != nil {
			t.Fatal(err)
		}
		return id
	}
	mkWord := func(content string) uint64 {
		w := &memory.Fragment{Kind: memory.KindWord, Channel: "word", Content: content, Lang: "en", Time: time.Now()}
		id, err := s.Encode(w)
		if err != nil {
			t.Fatal(err)
		}
		return id
	}
	link := func(from, to uint64, wgt float64) {
		if err := s.SetLinkRel(from, to, memory.RelNames, wgt); err != nil {
			t.Fatal(err)
		}
	}
	relA := mkConcept("chase", "relation")
	relB := mkConcept("pursue", "relation")
	noun := mkConcept("dog", "noun")
	runs := mkWord("runs")
	sole := mkWord("sole")
	link(runs, relA, 0.7) // tie in weight with relB, so the lower id (relA) must win
	link(runs, relB, 0.7)
	link(runs, noun, 0.9) // heavier, but not a relation, so it never wins
	link(sole, relB, 0.5) // "runs sole" sums relB to 1.2, overtaking relA

	idx, err := d.buildAbstractionIndex()
	if err != nil {
		t.Fatal(err)
	}
	fragID := func(f *memory.Fragment) uint64 {
		if f == nil {
			return 0
		}
		return f.ID
	}
	for _, phrase := range []string{"runs", "Runs", "sole", "dog", "missing", "runs sole"} {
		d.absIdx = nil
		store := fragID(d.resolveRelation(phrase))
		d.absIdx = idx
		index := fragID(d.resolveRelation(phrase))
		d.absIdx = nil
		if store != index {
			t.Fatalf("resolveRelation mismatch for %q: store=%d index=%d", phrase, store, index)
		}
	}
}

// namingAndGroundingEdges returns every "names" and "grounds" edge out of a word or concept,
// as sorted "from|to|rel|weight" rows, the writes the abstraction pass makes.
func namingAndGroundingEdges(t *testing.T, s *memory.Store) []string {
	t.Helper()
	words, err := s.NodesByKind(memory.KindWord)
	if err != nil {
		t.Fatal(err)
	}
	concepts, err := s.Concepts()
	if err != nil {
		t.Fatal(err)
	}
	ids := make([]uint64, 0, len(words)+len(concepts))
	for _, w := range words {
		ids = append(ids, w.ID)
	}
	for _, c := range concepts {
		ids = append(ids, c.ID)
	}
	var rows []string
	for _, rel := range []string{memory.RelNames, memory.RelGrounds} {
		edges, err := s.LinksFromRel(ids, rel)
		if err != nil {
			t.Fatal(err)
		}
		for _, e := range edges {
			rows = append(rows, fmt.Sprintf("%d|%d|%s|%.6f", e.From, e.To, e.Rel, e.Weight))
		}
	}
	sort.Strings(rows)
	return rows
}

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

// An abstraction's dominant grounding percept is its bridge across tongues: a
// word that names the percept firmly comes to name the abstraction too, scaled
// by how firmly it names the percept, so a tongue taught the percept but not
// enough to distil its own abstraction still reaches the concept the facts
// attach to. The anchor word is left untouched and a faint co-namer stays out,
// so noise in the percept's company does not spread upward.
func TestAbstractionInheritsGroundingNamesAcrossTongues(t *testing.T) {
	d, s := testDaemon(t)
	now := time.Now()
	concept := func(content, tpl string) *memory.Fragment {
		c := &memory.Fragment{Kind: memory.KindConcept, Channel: "concept", Content: content,
			Template: tpl, Category: "noun", Limbic: d.chem.Snapshot(), Time: now}
		if _, err := s.Encode(c); err != nil {
			t.Fatal(err)
		}
		return c
	}
	percept := concept("a white rabbit", "a white rabbit")
	abs := concept("rabbit", "abs:en:rabbit")
	if err := s.SetLinkRel(abs.ID, percept.ID, memory.RelGrounds, 0.5); err != nil {
		t.Fatal(err)
	}
	name := func(form, lang string, weight float64) uint64 {
		w, _, err := s.WordNode(form, lang, d.chem.Snapshot(), now)
		if err != nil {
			t.Fatal(err)
		}
		if err := s.SetLinkRel(w.ID, percept.ID, memory.RelNames, weight); err != nil {
			t.Fatal(err)
		}
		return w.ID
	}
	en := name("rabbit", "en", 0.8) // the anchor, the firmest namer of the percept
	tr := name("tavşan", "tr", 0.6) // a second tongue that named the percept ostensively
	noise := name("cat", "en", 0.1) // a faint co-namer that must not inherit
	// The anchor already names the abstraction directly, as growAbstractions sets it.
	if err := s.SetLinkRel(en, abs.ID, memory.RelNames, absNameWeight); err != nil {
		t.Fatal(err)
	}

	d.inheritGroundingNames(abs, percept.ID, en)

	// The second tongue's word now names the abstraction, scaled to its share of
	// the firmest namer (0.8 * 0.6/0.8 = 0.6), so it beats its bare-percept edge.
	if got := edgeWeightRel(t, s, tr, abs.ID, memory.RelNames); got < 0.59 || got > 0.61 {
		t.Fatalf("the second tongue should inherit the abstraction at ~0.6, got %.3f", got)
	}
	// The faint namer stays out (0.1 < 0.5*0.8), so percept noise does not climb.
	if got := edgeWeightRel(t, s, noise, abs.ID, memory.RelNames); got != 0 {
		t.Fatalf("a faint co-namer must not inherit, got %.3f", got)
	}
	// The anchor's own naming is untouched.
	if got := edgeWeightRel(t, s, en, abs.ID, memory.RelNames); got != absNameWeight {
		t.Fatalf("the anchor's naming should be unchanged at %.2f, got %.3f", absNameWeight, got)
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

// A category taught in two tongues has its real members in BOTH tongues' company; a
// percept the category word merely shared a scene with in one tongue (the eater
// category warm beside an eaten food) is an incidental scene-mate the night must not
// promote to membership. The cross-tongue twin (hayvan for animal) corroborates the
// real animals but not the food, so the food is left out of animal; the same food is
// KEPT in its own category, whose twin (yiyecek for food) does corroborate it.
func TestAbstractionDropsCrossTongueUncorroboratedScenemate(t *testing.T) {
	d, s := testDaemon(t)
	base := time.Now().Add(-3 * time.Hour)
	feed := func(shape string, n int) []recentMoment {
		return feedShape(t, s, "vision", shape, 4, base.Add(time.Duration(n)*time.Minute))
	}
	dog := feed("a small brown dog", 0)
	cat := feed("a small black cat", 1)
	mouse := feed("a small gray mouse", 2)
	carrot := feed("a long orange carrot", 3)
	cheese := feed("a yellow cheese wedge", 4)
	if _, err := s.MergeDuplicates(); err != nil {
		t.Fatal(err)
	}
	calm(d)
	d.growConcepts()

	// Build every category word's company first, so both twins are present before the
	// single promotion pass (four abstractions, the night's mint cap).
	mint := func(word, lang string, momentsets ...[]recentMoment) {
		for i := range 3 {
			anchors := make([]recentMoment, 0, len(momentsets))
			for _, ms := range momentsets {
				anchors = append(anchors, ms[i])
			}
			d.mintWords(word, lang, 0, anchors, false)
		}
	}
	mint("animal", "en", dog, cat, mouse, carrot) // carrot bleeds into animal (one tongue)
	mint("hayvan", "tr", dog, cat, mouse)         // the twin never kept carrot's company
	mint("food", "en", carrot, cheese)
	mint("yiyecek", "tr", carrot, cheese)
	calm(d)
	d.growConcepts()
	d.growAbstractions()

	groundedContents := func(tpl string) map[string]bool {
		abs, err := s.ConceptByTemplate(tpl)
		if err != nil || abs == nil {
			t.Fatalf("the abstraction %s should exist: %v", tpl, err)
		}
		gs, err := s.LinksFromRel([]uint64{abs.ID}, memory.RelGrounds)
		if err != nil {
			t.Fatal(err)
		}
		ids := make([]uint64, 0, len(gs))
		for _, g := range gs {
			ids = append(ids, g.To)
		}
		frags, _ := s.FragmentsByIDs(ids)
		out := map[string]bool{}
		for _, f := range frags {
			out[f.Content] = true
		}
		return out
	}

	animal := groundedContents("abs:en:animal")
	if animal["a long orange carrot"] {
		t.Fatal("carrot kept animal's company in one tongue only; the uncorroborated scene-mate must not be a member")
	}
	if !animal["a small brown dog"] || !animal["a small black cat"] || !animal["a small gray mouse"] {
		t.Fatalf("real members corroborated by the cross-tongue twin must stay: %v", animal)
	}
	food := groundedContents("abs:en:food")
	if !food["a long orange carrot"] || !food["a yellow cheese wedge"] {
		t.Fatalf("the food's own twin corroborates it, so it stays a food member: %v", food)
	}
}

// A broad category survives the night. Past ~20 members the category word's company
// spreads so thin that each member's word-side share falls below the absolute floor,
// which used to empty the core, mark the category dissolved, and let its grounds decay
// away. The size-invariant concept-side reliability keeps a thin-spread member, and the
// grounds edge is written above the prune floor, so a 24-member category keeps its
// members at mint AND across a sleep. A one-shot co-namer of a percept that already has
// its own firm name stays out.
func TestBroadCategorySurvivesSleep(t *testing.T) {
	const linkFloor = 0.05 // mirrors memory.linkFloor (unexported): the nightly prune drops edges at/below this
	d, s := testDaemon(t)
	base := time.Now().Add(-3 * time.Hour)
	shapes := []string{
		"a small black cat", "a small brown dog", "a gray wolf", "a gray rabbit",
		"a small gray mouse", "a large brown horse", "a green frog", "a silver fish",
		"a large black bear", "a golden lion", "a brown owl", "a spotted deer",
		"a small blue bird", "a tiny gray snail", "a striped orange tiger", "a huge gray elephant",
		"a tall spotted giraffe", "a black soaring raven", "a slim green vine snake", "a hopping brown kangaroo",
		"a fierce golden eagle", "a sleek gray dolphin", "a fluffy white arctic fox", "a stocky brown jaguar",
	}
	members := make([][]recentMoment, len(shapes))
	for i, sh := range shapes {
		members[i] = feedShape(t, s, "vision", sh, 4, base.Add(time.Duration(i)*time.Minute))
	}
	if _, err := s.MergeDuplicates(); err != nil {
		t.Fatal(err)
	}
	calm(d)
	d.growConcepts()
	for r := 0; r < 3; r++ {
		anchors := make([]recentMoment, 0, len(members))
		for _, m := range members {
			anchors = append(anchors, m[r])
		}
		d.mintWords("animal", "en", 0, anchors, false)
	}
	calm(d)
	d.growConcepts()
	d.growAbstractions()

	grounds := func() []memory.LinkEdge {
		abs, err := s.ConceptByTemplate("abs:en:animal")
		if err != nil || abs == nil {
			t.Fatalf("a 24-member category should distil an abstraction, got nil (%v)", err)
		}
		g, err := s.LinksFromRel([]uint64{abs.ID}, memory.RelGrounds)
		if err != nil {
			t.Fatal(err)
		}
		return g
	}
	g0 := grounds()
	if len(g0) < 20 {
		t.Fatalf("the broad category kept only %d of 24 members at mint", len(g0))
	}
	for _, e := range g0 {
		if e.Weight <= linkFloor {
			t.Fatalf("a member grounds at %.3f, at/below the prune floor: it will be deleted on the first sleep", e.Weight)
		}
	}
	// Durability: a night prunes then regrows; the members must persist.
	if _, err := s.Sleep(d.cfg.NightlyDecay, d.cfg.PruneFloor); err != nil {
		t.Fatal(err)
	}
	d.growConcepts()
	d.growAbstractions()
	if g1 := grounds(); len(g1) < 20 {
		t.Fatalf("the broad category lost members across the night: %d remain", len(g1))
	}

	// A one-shot co-namer stays out: "wagon" names the cart percept firmly, so the
	// cart's naming company is dominated by "wagon"; a single stray "animal" mention
	// of it gives "animal" a negligible concept-side share, below the membership floor.
	cart := feedShape(t, s, "vision", "a red wooden cart", 4, base.Add(40*time.Minute))
	if _, err := s.MergeDuplicates(); err != nil {
		t.Fatal(err)
	}
	calm(d)
	d.growConcepts()
	for r := 0; r < 4; r++ {
		d.mintWords("wagon", "en", 0, []recentMoment{cart[r]}, true)
	}
	d.mintWords("animal", "en", 0, []recentMoment{cart[0]}, false) // one coincidental mention
	calm(d)
	d.growConcepts()
	d.growAbstractions()
	for _, e := range grounds() {
		fr, _ := s.FragmentsByIDs([]uint64{e.To})
		if len(fr) == 1 && fr[0].Content == "a red wooden cart" {
			t.Fatal("a one-shot co-namer became a category member; the concept-side floor should exclude it")
		}
	}
}

// A basic word does not abstract even when its company spreads like a category's.
// "cat" kept company with the cat, the dog, and the rabbit (as a chase
// co-participant would), enough to clear the gate, but it heads the caption of one
// of them, so it names a thing rather than a class of things and mints no shadow.
// The same company under "animal", which heads no member's caption, still
// abstracts: the gate reads the word, not the company, which is identical.
func TestAbstractionSkipsBasicLevelWord(t *testing.T) {
	d, s := testDaemon(t)
	base := time.Now().Add(-3 * time.Hour)
	shapes := []string{"a small black cat", "a small brown dog", "a gray rabbit"}
	members := make([][]recentMoment, len(shapes))
	for i, shape := range shapes {
		members[i] = feedShape(t, s, "vision", shape, 4, base.Add(time.Duration(i)*time.Minute))
	}
	if _, err := s.MergeDuplicates(); err != nil {
		t.Fatal(err)
	}
	calm(d)
	d.growConcepts()
	mintOverMembers := func(word string) {
		for i := range 3 {
			anchors := make([]recentMoment, 0, len(members))
			for _, m := range members {
				anchors = append(anchors, m[i])
			}
			d.mintWords(word, "en", 0, anchors, false)
		}
		calm(d)
		d.growConcepts()
		d.growAbstractions()
	}

	mintOverMembers("cat")
	if abs, _ := s.ConceptByTemplate("abs:en:cat"); abs != nil {
		t.Fatal("a basic word that heads one of its own core percepts must not abstract")
	}
	// Each member's head names its own percept, so the category below spans three
	// named basic-level things, the realistic case (a superordinate generalizes
	// over things that have names, not one named thing among unnamed company).
	mintOverMembers("dog")
	mintOverMembers("rabbit")

	mintOverMembers("animal")
	if abs, _ := s.ConceptByTemplate("abs:en:animal"); abs == nil {
		t.Fatal("a superordinate over the same members, heading none, should still abstract")
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

// A word that names one basic-level percept already named in another tongue binds
// to that concept rather than splitting a parallel abstraction: "kurt" names the
// wolf "wolf" already names, even when junk company spreads it to the abstraction
// gate, so both tongues stay on one node and the propositions reach it from
// either. A superordinate naming several named percepts still abstracts.
func TestAbstractionBindsCrossTongueNameToPercept(t *testing.T) {
	d, s := testDaemon(t)
	base := time.Now().Add(-3 * time.Hour)
	wolf := feedShape(t, s, "vision", "a gray wolf", 4, base)
	junk := feedShape(t, s, "vision", "a passing shape", 4, base.Add(time.Minute))
	cat := feedShape(t, s, "vision", "a small black cat", 4, base.Add(2*time.Minute))
	if _, err := s.MergeDuplicates(); err != nil {
		t.Fatal(err)
	}
	calm(d)
	d.growConcepts()
	mintOver := func(word, lang string, momentsets ...[]recentMoment) {
		for i := range 3 {
			anchors := make([]recentMoment, 0, len(momentsets))
			for _, ms := range momentsets {
				anchors = append(anchors, ms[i])
			}
			d.mintWords(word, lang, 0, anchors, false)
		}
		calm(d)
		d.growConcepts()
		d.growAbstractions()
	}
	// The English heads name their percepts.
	mintOver("wolf", "en", wolf)
	mintOver("cat", "en", cat)
	// "kurt" (tr), spread across the wolf percept and the junk one, reaches the
	// abstraction gate but heads neither caption.
	mintOver("kurt", "tr", wolf, junk)
	if abs, _ := s.ConceptByTemplate("abs:tr:kurt"); abs != nil {
		t.Fatal("a cross-tongue name for one named percept must not split into a parallel abstraction")
	}
	kurtC, wolfC := d.resolveConcept("kurt"), d.resolveConcept("wolf")
	if kurtC == nil || wolfC == nil || kurtC.ID != wolfC.ID {
		t.Fatalf("kurt should name the same concept as wolf: kurt=%v wolf=%v", kurtC, wolfC)
	}
	// A superordinate over two named percepts still abstracts.
	mintOver("hayvan", "tr", wolf, cat)
	if abs, _ := s.ConceptByTemplate("abs:tr:hayvan"); abs == nil {
		t.Fatal("a superordinate naming several named percepts should still abstract")
	}
}

// A healthy category that comes to own exactly one of its members must keep abstracting,
// not collapse its name onto that member. When a cast is re-grounded and one member is left
// un-shown, its lone naming bond stays above the floor while the refreshed members' bonds
// decay below it, so the class owns exactly one member. The company-based reading must still
// read the word as the class it spans; the target logic re-homing "animal" onto the one bird
// it still owns is the is-a break this guards against.
func TestHealthyCategoryOwningOneMemberKeepsAbstracting(t *testing.T) {
	d, s := testDaemon(t)
	base := time.Now().Add(-3 * time.Hour)
	dog := feedShape(t, s, "vision", "a small brown dog", 4, base)
	cat := feedShape(t, s, "vision", "a small black cat", 4, base.Add(time.Minute))
	bird := feedShape(t, s, "vision", "a small blue bird", 4, base.Add(2*time.Minute))
	if _, err := s.MergeDuplicates(); err != nil {
		t.Fatal(err)
	}
	calm(d)
	d.growConcepts()
	mintOver := func(word, lang string, momentsets ...[]recentMoment) {
		for i := range 3 {
			anchors := make([]recentMoment, 0, len(momentsets))
			for _, ms := range momentsets {
				anchors = append(anchors, ms[i])
			}
			d.mintWords(word, lang, 0, anchors, false)
		}
		calm(d)
		d.growConcepts()
		d.growAbstractions()
	}
	// The English heads name their percepts, and "animal" said over all three abstracts.
	mintOver("dog", "en", dog)
	mintOver("cat", "en", cat)
	mintOver("bird", "en", bird)
	mintOver("animal", "en", dog, cat, bird)
	absC, _ := s.ConceptByTemplate("abs:en:animal")
	if absC == nil {
		t.Fatal("animal over three named percepts should abstract into a category")
	}
	animalID := wordID(t, s, "animal")
	dogC, catC, birdC := d.resolveConcept("dog"), d.resolveConcept("cat"), d.resolveConcept("bird")
	if dogC == nil || catC == nil || birdC == nil {
		t.Fatalf("dog, cat, bird should resolve: dog=%v cat=%v bird=%v", dogC, catC, birdC)
	}
	// Re-ground the cast: the bird is left un-shown, so its naming bond stays above the
	// floor while the refreshed members' bonds decay below it. The class now owns one member.
	if err := s.SetLinkRel(animalID, birdC.ID, memory.RelNames, namesWriteFloor+0.05); err != nil {
		t.Fatal(err)
	}
	for _, id := range []uint64{dogC.ID, catC.ID} {
		if err := s.SetLinkRel(animalID, id, memory.RelNames, namesWriteFloor-0.05); err != nil {
			t.Fatal(err)
		}
	}
	// The category, now owning only the bird, must keep naming its class rather than collapse
	// onto that one member. Re-run the abstraction pass without re-spreading the company.
	d.growAbstractions()
	if animalC := d.resolveConcept("animal"); animalC == nil || animalC.ID != absC.ID {
		t.Fatalf("animal should still name its category, not re-home onto a member: got %v want abs %d", animalC, absC.ID)
	}
	edges, err := s.LinksFromRel([]uint64{animalID}, memory.RelNames)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range edges {
		if e.To == birdC.ID && e.Weight >= absNameWeight {
			t.Fatalf("animal re-homed onto the bird at %.2f: a healthy category must not take its lone owned member as a name", e.Weight)
		}
	}
}

// groundedEpisode returns one surviving episode a concept grounds, the percept a
// telling could have been pointed at.
func groundedEpisode(t *testing.T, s *memory.Store, concept uint64) uint64 {
	t.Helper()
	g, err := s.LinksFromRel([]uint64{concept}, memory.RelGrounds)
	if err != nil || len(g) == 0 {
		t.Fatalf("concept %d should ground at least one episode: %v %v", concept, g, err)
	}
	return g[0].To
}

// dilutedNameSetup builds two named percepts (cat, dog) and a cross-tongue word whose
// company spreads across both, the dilution a lingering scene-mate causes when nouns are
// taught back to back. The word names both head-named percepts above the floor, the
// ambiguous case that splits a phantom. Setup stops just before the abstraction pass so a
// caller can record ostension first, and returns the word, the two concepts, and one
// grounded episode of each.
func dilutedNameSetup(t *testing.T, word string) (d *Daemon, s *memory.Store, wid, catC, dogC, catEp, dogEp uint64) {
	t.Helper()
	d, s = testDaemon(t)
	base := time.Now().Add(-3 * time.Hour)
	cat := feedShape(t, s, "vision", "a small black cat", 4, base)
	dog := feedShape(t, s, "vision", "a small brown dog", 4, base.Add(time.Minute))
	if _, err := s.MergeDuplicates(); err != nil {
		t.Fatal(err)
	}
	calm(d)
	d.growConcepts()
	mintNamed := func(w, lang string, ms []recentMoment) {
		for i := range 3 {
			d.mintWords(w, lang, 0, []recentMoment{ms[i]}, false)
		}
	}
	mintNamed("cat", "en", cat)
	mintNamed("dog", "en", dog)
	calm(d)
	d.growConcepts()
	d.growAbstractions() // cat names the cat, dog names the dog
	// The cross-tongue word keeps company with both percepts, so it names both above the
	// floor: the diluted cross-tongue case the uniform company cannot resolve.
	for i := range 3 {
		d.mintWords(word, "tr", 0, []recentMoment{cat[i], dog[i]}, false)
	}
	calm(d)
	d.growConcepts() // names written; the abstraction pass is left to the caller
	catFrag, dogFrag := d.resolveConcept("cat"), d.resolveConcept("dog")
	if catFrag == nil || dogFrag == nil {
		t.Fatalf("cat and dog should resolve to concepts: cat=%v dog=%v", catFrag, dogFrag)
	}
	catC, dogC = catFrag.ID, dogFrag.ID
	wid = wordID(t, s, word)
	catEp = groundedEpisode(t, s, catC)
	dogEp = groundedEpisode(t, s, dogC)
	return
}

// Without ostension the diluted cross-tongue name splits a phantom: its company is spread
// evenly across two head-named percepts, so the company-based reading cannot tell which it
// names and abstracts a parallel node beside them. This is the bug the ostension signal
// removes; the test guards that the fix is load-bearing, not a no-op.
func TestDilutedCrossTongueNameSplitsPhantomWithoutOstension(t *testing.T) {
	d, s, _, _, _, _, _ := dilutedNameSetup(t, "kedi")
	d.growAbstractions()
	if abs, _ := s.ConceptByTemplate("abs:tr:kedi"); abs == nil {
		t.Fatal("without ostension the diluted cross-tongue name should split a phantom abstraction")
	}
}

// With ostension concentrated on the percept it was pointed at, the diluted cross-tongue
// name binds to its referent rather than splitting a phantom: a name points at one thing
// every telling, so its ostension piles up there even when a lingering scene-mate diluted
// its company.
func TestOstensionBindsDilutedCrossTongueNameToPercept(t *testing.T) {
	d, s, wid, catC, _, catEp, _ := dilutedNameSetup(t, "kedi")
	// kedi was shown over the cat every time it named, the salience the uniform company
	// discards.
	if err := s.SetLinkRel(wid, catEp, memory.RelOstends, linkWeight); err != nil {
		t.Fatal(err)
	}
	d.growAbstractions()
	if abs, _ := s.ConceptByTemplate("abs:tr:kedi"); abs != nil {
		t.Fatal("a cross-tongue name whose ostension concentrates on one percept must not split a phantom")
	}
	if got := d.resolveConcept("kedi"); got == nil || got.ID != catC {
		t.Fatalf("kedi should resolve to the cat it was pointed at, got %+v", got)
	}
	if w := edgeWeightRel(t, s, wid, catC, memory.RelNames); w != absNameWeight {
		t.Fatalf("the ostended percept should be named at %.2f, got %.2f", absNameWeight, w)
	}
}

// A category whose ostension spreads across the members it was said over still abstracts:
// pointed at a different member each telling, it names no single referent, so the spread
// ostension yields none and the company-based reading abstracts as before. This is what
// keeps the signal from collapsing a real category into one member's name.
func TestOstensionSpreadStillAbstractsCategory(t *testing.T) {
	d, s, wid, _, _, catEp, dogEp := dilutedNameSetup(t, "hayvan")
	if err := s.SetLinkRel(wid, catEp, memory.RelOstends, linkWeight); err != nil {
		t.Fatal(err)
	}
	if err := s.SetLinkRel(wid, dogEp, memory.RelOstends, linkWeight); err != nil {
		t.Fatal(err)
	}
	d.growAbstractions()
	if abs, _ := s.ConceptByTemplate("abs:tr:hayvan"); abs == nil {
		t.Fatal("a category whose ostension spreads across members should still abstract")
	}
}

// A diluted BASIC name binds to the percept it was pointed at, not the scene-mate it
// shared company with. "cat" keeps company with both the cat and a dog that lingered
// from the block before, enough to name the dog as firmly, but it was shown over the cat
// every time it named, so its ostension binds it there. Without the signal the company
// alone can tip a near-even basic name onto the wrong percept.
func TestOstensionBindsDilutedBasicNameToPercept(t *testing.T) {
	d, s := testDaemon(t)
	base := time.Now().Add(-3 * time.Hour)
	cat := feedShape(t, s, "vision", "a small black cat", 4, base)
	dog := feedShape(t, s, "vision", "a small brown dog", 4, base.Add(time.Minute))
	if _, err := s.MergeDuplicates(); err != nil {
		t.Fatal(err)
	}
	calm(d)
	d.growConcepts()
	catC, _ := s.ConceptByTemplate(attention.Normalize("a small black cat"))
	dogC, _ := s.ConceptByTemplate(attention.Normalize("a small brown dog"))
	if catC == nil || dogC == nil {
		t.Fatalf("cat and dog should distil: cat=%v dog=%v", catC, dogC)
	}
	for i := range 3 {
		d.mintWords("dog", "en", 0, []recentMoment{dog[i]}, false)
	}
	for i := range 3 {
		d.mintWords("cat", "en", 0, []recentMoment{cat[i], dog[i]}, false)
	}
	calm(d)
	d.growConcepts()
	catEp := groundedEpisode(t, s, catC.ID)
	if err := s.SetLinkRel(wordID(t, s, "cat"), catEp, memory.RelOstends, linkWeight); err != nil {
		t.Fatal(err)
	}
	d.growAbstractions()
	if got := d.resolveConcept("cat"); got == nil || got.ID != catC.ID {
		t.Fatalf("cat should resolve to the cat it was pointed at, not the dog it shared a scene with, got %+v", got)
	}
	if w := edgeWeightRel(t, s, wordID(t, s, "cat"), catC.ID, memory.RelNames); w != absNameWeight {
		t.Fatalf("the ostended percept should be named at %.2f, got %.2f", absNameWeight, w)
	}
}
