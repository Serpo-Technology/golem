package daemon

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"golem/internal/attention"
	"golem/internal/memory"
	"golem/wire"
)

// feedShape encodes n occurrences of one shape, minutes apart, and
// returns the moments as binding anchors.
func feedShape(t *testing.T, s *memory.Store, channel, content string, n int, base time.Time) []recentMoment {
	t.Helper()
	var anchors []recentMoment
	for i := range n {
		f := encodeAt(t, s, channel, content, 3.0, base.Add(time.Duration(i)*10*time.Minute))
		anchors = append(anchors, recentMoment{id: f.ID, template: f.Template, at: f.Time})
	}
	return anchors
}

// A shape that has recurred into a prototype is distilled into a concept
// at sleep: grounded to its episodes, categorized by its grounding role.
func TestConceptGenesisAtSleep(t *testing.T) {
	d, s := testDaemon(t)
	base := time.Now().Add(-3 * time.Hour)

	feedShape(t, s, "shell", "run: go test ./...", 4, base)
	feedShape(t, s, "syslog", "Disk full on /dev/disk1s9", 4, base.Add(time.Minute))

	if _, err := s.MergeDuplicates(); err != nil {
		t.Fatal(err)
	}
	born, _ := d.growConcepts()
	if born != 2 {
		t.Fatalf("two recurring shapes should distil into two concepts, got %d", born)
	}

	verb, err := s.ConceptByTemplate(attention.Normalize("run: go test ./..."))
	if err != nil || verb == nil {
		t.Fatalf("the command shape should have a concept: %v", err)
	}
	if verb.Category != "verb" {
		t.Fatalf("a shape in command position is verb-like, got %q", verb.Category)
	}
	if verb.Content != "go test ./..." {
		t.Fatalf("the gloss should drop the command marker, got %q", verb.Content)
	}
	noun, err := s.ConceptByTemplate(attention.Normalize("Disk full on /dev/disk1s9"))
	if err != nil || noun == nil {
		t.Fatalf("the sensory shape should have a concept: %v", err)
	}
	if noun.Category != "noun" {
		t.Fatalf("a recurring sensory object is noun-like, got %q", noun.Category)
	}

	// The concept grounds out to every surviving episode of its shape.
	grounds, err := s.LinksFromRel([]uint64{verb.ID}, memory.RelGrounds)
	if err != nil || len(grounds) == 0 {
		t.Fatalf("a concept must ground its episodes: %v %v", grounds, err)
	}

	// Genesis is idempotent: the same shapes do not mint twice.
	again, _ := d.growConcepts()
	if again != 0 {
		t.Fatalf("a distilled shape must not mint a second concept, got %d", again)
	}
}

// Genesis is paced by what is ready, not by mood: a consolidation forms the
// recurring shape whether the system is calm or depleted. The restraint on an
// alarmed system lives at the sleep gate, which withholds the consolidation;
// once it runs, it forms what was taken in.
func TestConceptGenesisIsNotPacedByMood(t *testing.T) {
	d, s := testDaemon(t)
	feedShape(t, s, "syslog", "Disk full on /dev/disk1s9", 4, time.Now().Add(-time.Hour))
	if _, err := s.MergeDuplicates(); err != nil {
		t.Fatal(err)
	}
	d.chem.SuppressSleep(1.0) // calm at its floor, as a stretch of intake leaves it
	if born, _ := d.growConcepts(); born != 1 {
		t.Fatalf("a consolidation should form the ready shape regardless of mood, got %d", born)
	}
}

// Words bond to the concepts their episodes ground: tell words near the
// moments, sleep, and the names edge appears.
func TestNamingBondsWordsToConcepts(t *testing.T) {
	d, s := testDaemon(t)
	base := time.Now().Add(-2 * time.Hour)

	anchors := feedShape(t, s, "shell", "run: go test ./...", 4, base)
	words := d.mintWords("the tests are running", "en", 0, anchors, false)
	if len(words) != 2 { // "tests", "running"; the rest is closed class
		t.Fatalf("content words should mint word nodes, got %v", words)
	}

	if _, err := s.MergeDuplicates(); err != nil {
		t.Fatal(err)
	}
	born, named := d.growConcepts()
	if born != 1 || named == 0 {
		t.Fatalf("the shape should distil and its words should bond: born %d, named %d", born, named)
	}

	concept, _ := s.ConceptByTemplate(attention.Normalize("run: go test ./..."))
	var tests *memory.Fragment
	for _, w := range words {
		if w.Content == "tests" {
			tests = w
		}
	}
	if w := edgeWeightRel(t, s, tests.ID, concept.ID, memory.RelNames); w == 0 {
		t.Fatal("a word that kept company with grounded episodes should name their concept")
	}
}

// Two words in two languages that ground to the same experience converge
// on one concept, and recall through either reaches the same moments.
func TestCrossLanguageSynonymsShareOneConcept(t *testing.T) {
	d, s := testDaemon(t)
	base := time.Now().Add(-2 * time.Hour)

	anchors := feedShape(t, s, "shell", "run: go test ./...", 4, base)
	en := d.mintWords("tests", "en", 0, anchors, false)
	tr := d.mintWords("testler", "tr", 0, anchors, false)
	if len(en) != 1 || len(tr) != 1 {
		t.Fatalf("both forms should mint: %v %v", en, tr)
	}

	if _, err := s.MergeDuplicates(); err != nil {
		t.Fatal(err)
	}
	d.growConcepts()

	enNames, _ := s.LinksFromRel([]uint64{en[0].ID}, memory.RelNames)
	trNames, _ := s.LinksFromRel([]uint64{tr[0].ID}, memory.RelNames)
	if len(enNames) == 0 || len(trNames) == 0 {
		t.Fatal("both words should name a concept")
	}
	if enNames[0].To != trNames[0].To {
		t.Fatalf("synonyms across languages must share one concept: %d vs %d",
			enNames[0].To, trNames[0].To)
	}

	episodes, _ := d.semanticSeeds("testler")
	if len(episodes) == 0 {
		t.Fatal("the second language should recall the grounded moments")
	}
	for _, f := range episodes {
		if !strings.Contains(f.Content, "go test") {
			t.Fatalf("recall through the concept should reach the experience, got %q", f.Content)
		}
	}
}

// The recall command reads through the lexicon: a query word that shares
// no substring with any moment still surfaces the grounded experience.
func TestRecallReadsThroughConcepts(t *testing.T) {
	d, s := testDaemon(t)
	base := time.Now().Add(-2 * time.Hour)

	anchors := feedShape(t, s, "syslog", "Disk full on /dev/disk1s9", 4, base)
	d.mintWords("depolama", "tr", 0, anchors, false) // no substring overlap with the moments
	if _, err := s.MergeDuplicates(); err != nil {
		t.Fatal(err)
	}
	d.growConcepts()

	resp, _, _ := d.handle(wire.Request{Cmd: "recall", Query: "depolama"}, nil)
	var sawDisk, sawConcept bool
	for _, f := range resp.Fragments {
		if strings.Contains(f.Content, "Disk full") && f.Kind == "" {
			sawDisk = true
		}
		if f.Kind == memory.KindConcept {
			sawConcept = true
		}
	}
	if !sawDisk {
		t.Fatalf("recall should reach the moments through the concept, got %+v", resp.Fragments)
	}
	if !sawConcept {
		t.Fatal("the reply should show the concept the meaning walked through")
	}
}

// Plain substring recall returns moments only; the lexicon's nodes are
// not moments.
func TestWordAndConceptNodesStayOutOfPlainRecall(t *testing.T) {
	d, s := testDaemon(t)
	anchors := feedShape(t, s, "syslog", "Disk full on /dev/disk1s9", 4, time.Now().Add(-time.Hour))
	d.mintWords("disk", "en", 0, anchors, false)
	if _, err := s.MergeDuplicates(); err != nil {
		t.Fatal(err)
	}
	d.growConcepts()

	frags, err := s.Recall("disk", 10)
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range frags {
		if f.Kind != memory.KindEpisodic {
			t.Fatalf("plain recall must return moments only, got %s %q", f.Kind, f.Content)
		}
	}
}

// Semantic nodes die by disconnection: a word with no edges and a
// concept with no groundings vanish during sleep.
func TestDisconnectedSemanticNodesPrune(t *testing.T) {
	_, s := testDaemon(t)

	if _, err := s.Encode(&memory.Fragment{Kind: memory.KindWord, Channel: "word", Content: "orphan",
		Template: "orphan", Lang: "en", Time: time.Now()}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Encode(&memory.Fragment{Kind: memory.KindConcept, Channel: "concept", Content: "hollow",
		Template: "hollow shape", Category: "noun", Time: time.Now()}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Sleep(0.10, 0.10); err != nil {
		t.Fatal(err)
	}
	// The innate faculty vocabulary re-seeds each boot and is not counted here, so
	// the test asserts the orphan and the hollow concept specifically pruned.
	if held(t, s, memory.KindWord, "orphan") {
		t.Fatalf("an edgeless word should prune")
	}
	if held(t, s, memory.KindConcept, "hollow") {
		t.Fatalf("a groundless concept should prune")
	}
}

// held reports whether a node of a kind with the given content survives in the
// store, so a prune test can check a specific node without depending on the count
// of innate seeded vocabulary.
func held(t *testing.T, s *memory.Store, kind, content string) bool {
	t.Helper()
	frags, err := s.Dump()
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range frags {
		if f.Kind == kind && f.Content == content {
			return true
		}
	}
	return false
}

// A grounded concept and a naming word survive the same sleep that kills
// disconnected ones.
func TestConnectedSemanticNodesSurviveSleep(t *testing.T) {
	d, s := testDaemon(t)
	anchors := feedShape(t, s, "syslog", "Disk full on /dev/disk1s9", 4, time.Now().Add(-time.Hour))
	d.mintWords("disk", "en", 0, anchors, false)
	if _, err := s.MergeDuplicates(); err != nil {
		t.Fatal(err)
	}
	d.growConcepts()

	if _, err := s.Sleep(0.10, 0.10); err != nil {
		t.Fatal(err)
	}
	// Counting non-system concepts skips the innate faculty vocabulary, so the one
	// grown concept is what must remain.
	if infos, err := d.conceptInfos(); err != nil || len(infos) != 1 {
		t.Fatalf("a grounded concept must survive sleep, %d listed", len(infos))
	}
	if !held(t, s, memory.KindWord, "disk") {
		t.Fatalf("a bonded word must survive sleep")
	}
}

// The concepts listing shows the layer whole: gloss, category, names per
// language, and the grounding tally.
func TestConceptInfosListing(t *testing.T) {
	d, s := testDaemon(t)
	anchors := feedShape(t, s, "shell", "run: go test ./...", 4, time.Now().Add(-time.Hour))
	d.mintWords("tests", "en", 0, anchors, false)
	d.mintWords("testler", "tr", 0, anchors, false)
	if _, err := s.MergeDuplicates(); err != nil {
		t.Fatal(err)
	}
	d.growConcepts()

	infos, err := d.conceptInfos()
	if err != nil || len(infos) != 1 {
		t.Fatalf("one concept should list: %+v %v", infos, err)
	}
	c := infos[0]
	if c.Gloss != "go test ./..." || c.Category != "verb" {
		t.Fatalf("gloss and category should carry: %+v", c)
	}
	if c.Grounded < 4 {
		t.Fatalf("the tally should count all occurrences, got %d", c.Grounded)
	}
	want := fmt.Sprintf("%s, %s", "testler(tr)", "tests(en)")
	if got := strings.Join(c.Names, ", "); got != want {
		t.Fatalf("names should list per language: got %q, want %q", got, want)
	}
}

// A question asked often enough used to recur into a concept of its
// own; genesis now refuses utterance shapes, so the lexicon keeps
// pointing at the world.
func TestQuestionsDoNotBecomeConcepts(t *testing.T) {
	d, s := testDaemon(t)
	feedShape(t, s, "syslog", "disk space low", 4, time.Now().Add(-2*time.Hour))
	for range 4 {
		say(t, d, "what usually happens after disk space low")
	}
	if _, err := s.MergeDuplicates(); err != nil {
		t.Fatal(err)
	}
	calm(d)
	d.growConcepts()

	concepts, err := s.Concepts()
	if err != nil {
		t.Fatal(err)
	}
	lived := false
	for _, c := range concepts {
		if strings.Contains(c.Content, "usually happens") {
			t.Fatalf("an utterance shape must not distil into a concept: %q", c.Content)
		}
		if c.Content == "disk space low" {
			lived = true
		}
	}
	if !lived {
		t.Fatalf("the lived shape should still distil: %+v", concepts)
	}
}

// A word scattered across many concepts splits its mass and bonds
// weakly to each; a word faithful to one concept bonds firmly. The
// textbook cross-situational contrast.
func TestPromiscuousWordBondsWeakly(t *testing.T) {
	d, s := testDaemon(t)
	base := time.Now().Add(-3 * time.Hour)

	disk := feedShape(t, s, "syslog", "Disk full on /dev/disk1s9", 4, base)
	cpu := feedShape(t, s, "vitals", "cpu load went high", 4, base.Add(time.Minute))
	net := feedShape(t, s, "syslog", "Connection refused peer", 4, base.Add(2*time.Minute))

	// "disk" keeps company only with the disk shape; "low" is said once
	// into each of the three.
	d.mintWords("disk", "en", 0, disk, false)
	scattered := []recentMoment{disk[0], cpu[0], net[0]}
	d.mintWords("low", "en", 0, scattered, false)

	if _, err := s.MergeDuplicates(); err != nil {
		t.Fatal(err)
	}
	d.growConcepts()

	diskConcept, _ := s.ConceptByTemplate(attention.Normalize("Disk full on /dev/disk1s9"))
	var faithful, promiscuous float64
	words, _ := s.WordNodesByForms([]string{"disk", "low"})
	for _, w := range words {
		bond := edgeWeightRel(t, s, w.ID, diskConcept.ID, memory.RelNames)
		if w.Content == "disk" {
			faithful = bond
		} else {
			promiscuous = bond
		}
	}
	if faithful == 0 {
		t.Fatal("the faithful word should bond to its concept")
	}
	if promiscuous >= faithful/2 {
		t.Fatalf("a scattered word should bond far more weakly: faithful %.2f, scattered %.2f",
			faithful, promiscuous)
	}
}

// Reliability is recomputed and set at each consolidation, down as well
// as up: a bond formed when a word kept one concept's company loosens
// once the word drifts to other company.
func TestBondLoosensWhenMassShifts(t *testing.T) {
	d, s := testDaemon(t)
	base := time.Now().Add(-4 * time.Hour)

	disk := feedShape(t, s, "syslog", "Disk full on /dev/disk1s9", 4, base)
	d.mintWords("blinker", "en", 0, disk[:1], false)
	if _, err := s.MergeDuplicates(); err != nil {
		t.Fatal(err)
	}
	d.growConcepts()
	diskConcept, _ := s.ConceptByTemplate(attention.Normalize("Disk full on /dev/disk1s9"))
	words, _ := s.WordNodesByForms([]string{"blinker"})
	before := edgeWeightRel(t, s, words[0].ID, diskConcept.ID, memory.RelNames)
	if before == 0 {
		t.Fatal("the first pairing should bond, even if weakly")
	}

	// The word then keeps company with a different recurring shape.
	cpu := feedShape(t, s, "vitals", "cpu load went high", 4, base.Add(time.Hour))
	for range 3 {
		d.mintWords("blinker", "en", 0, cpu[:1], false)
	}
	if _, err := s.MergeDuplicates(); err != nil {
		t.Fatal(err)
	}
	d.growConcepts()

	after := edgeWeightRel(t, s, words[0].ID, diskConcept.ID, memory.RelNames)
	if after >= before {
		t.Fatalf("drifting company should loosen the old bond: before %.2f, after %.2f", before, after)
	}
}

// A bond formed by a one-off coincidence weakens each time its word
// recurs while other concepts are the active ones, without waiting for
// the night's recount.
func TestCoincidenceFadesOnNegativeEvidence(t *testing.T) {
	d, s := testDaemon(t)
	base := time.Now().Add(-4 * time.Hour)

	disk := feedShape(t, s, "syslog", "Disk full on /dev/disk1s9", 4, base)
	cpu := feedShape(t, s, "vitals", "cpu load went high", 4, base.Add(time.Minute))
	d.mintWords("blinker", "en", 0, disk[:1], false) // one coincidental pairing
	if _, err := s.MergeDuplicates(); err != nil {
		t.Fatal(err)
	}
	d.growConcepts()

	diskConcept, _ := s.ConceptByTemplate(attention.Normalize("Disk full on /dev/disk1s9"))
	words, _ := s.WordNodesByForms([]string{"blinker"})
	before := edgeWeightRel(t, s, words[0].ID, diskConcept.ID, memory.RelNames)
	if before == 0 {
		t.Fatal("the coincidence should have bonded, weakly")
	}

	// The word recurs while the cpu shape is what is happening.
	d.mintWords("blinker", "en", 0, cpu[:1], false)
	after := edgeWeightRel(t, s, words[0].ID, diskConcept.ID, memory.RelNames)
	if after >= before {
		t.Fatalf("negative evidence should weaken the bond at once: before %.2f, after %.2f",
			before, after)
	}
	cpuConcept, _ := s.ConceptByTemplate(attention.Normalize("cpu load went high"))
	if cpuConcept != nil {
		if w := edgeWeightRel(t, s, words[0].ID, cpuConcept.ID, memory.RelNames); w != 0 {
			t.Fatalf("depression must not mint a bond to the active concept early: %.2f", w)
		}
	}
}

// Silence is not disconfirmation: a recurrence with no anchors, or with
// anchors whose shapes have no concepts, leaves every bond untouched.
func TestSilenceDoesNotDisconfirm(t *testing.T) {
	d, s := testDaemon(t)
	base := time.Now().Add(-3 * time.Hour)

	disk := feedShape(t, s, "syslog", "Disk full on /dev/disk1s9", 4, base)
	d.mintWords("blinker", "en", 0, disk[:1], false)
	if _, err := s.MergeDuplicates(); err != nil {
		t.Fatal(err)
	}
	d.growConcepts()

	diskConcept, _ := s.ConceptByTemplate(attention.Normalize("Disk full on /dev/disk1s9"))
	words, _ := s.WordNodesByForms([]string{"blinker"})
	before := edgeWeightRel(t, s, words[0].ID, diskConcept.ID, memory.RelNames)

	// Spoken into an empty room: no anchors at all.
	d.mintWords("blinker", "en", 0, nil, false)
	// Spoken near a one-off moment whose shape has no concept.
	stray := encodeAt(t, s, "syslog", "a shape seen once only", 3.0, base.Add(2*time.Hour))
	d.mintWords("blinker", "en", 0, []recentMoment{{id: stray.ID, template: stray.Template, at: stray.Time}}, false)

	after := edgeWeightRel(t, s, words[0].ID, diskConcept.ID, memory.RelNames)
	if after != before {
		t.Fatalf("a window with no active concepts is no evidence: before %.2f, after %.2f",
			before, after)
	}
}

// A noun ending in what looks like a verb-past suffix yields a short mangled stem
// ("kedi" -> "ke"); minting keeps the noun whole rather than taking the stem for
// it on a loose gloss match against some unrelated concept that merely contains
// those letters ("a wooden basket").
func TestMintKeepsNounWithVerbLikeEnding(t *testing.T) {
	d, s := testDaemon(t)
	c := &memory.Fragment{Kind: memory.KindConcept, Channel: "concept", Content: "a wooden basket",
		Template: "a wooden basket", Category: "noun", Limbic: d.chem.Snapshot(), Time: time.Now()}
	if _, err := s.Encode(c); err != nil {
		t.Fatal(err)
	}
	if got := d.baseForMint("tr", "kedi"); got != "kedi" {
		t.Fatalf("a noun ending in a verb-like suffix should mint whole, got %q", got)
	}
}

// A percept-less concept gathers neither stray names nor duplicate nodes from the
// company it keeps, even in a brain busy with featured concepts. Abstract words used
// alike -- a model and an llm that both predict and learn -- co-occur heavily, and
// against a backdrop of featured concepts the naming recount once let each drift onto
// the other while growAbstractions minted a duplicate beside each. With no percept to
// tell naming from company, a percept-less concept is named only by the words that
// introduce or translate it, so the abstract vocabulary stays one clean concept per
// word across the nights, while the featured concepts keep their own naming.
func TestPerceptlessConceptStaysCleanlyNamed(t *testing.T) {
	d, s := testDaemon(t)
	feat := func(name string) *memory.Fragment {
		f := &memory.Fragment{Kind: memory.KindFeature, Channel: "feature", Content: name,
			Template: name, Limbic: d.chem.Snapshot(), Time: time.Now()}
		if _, err := s.Encode(f); err != nil {
			t.Fatal(err)
		}
		return f
	}
	// A backdrop of featured, named concepts: the scale that surfaced the drift.
	furry, legs, wild := feat("furry"), feat("four-legs"), feat("wild")
	for _, a := range []struct {
		name string
		fs   []*memory.Fragment
	}{{"dog", []*memory.Fragment{furry, legs}}, {"cat", []*memory.Fragment{furry, legs}}, {"wolf", []*memory.Fragment{furry, legs, wild}}} {
		c := groundNoun(t, d, a.name)
		for _, f := range a.fs {
			s.SetLinkRel(c.ID, f.ID, memory.RelFeature, 1)
		}
		groundOccurrences(t, d, c, 3) // grounded in percepts, so they persist as the real animals do
	}
	namesOf := func(word string) []string {
		c := d.resolveConcept(word)
		if c == nil {
			return nil
		}
		links, _ := s.LinksToRel([]uint64{c.ID}, memory.RelNames)
		var out []string
		for _, e := range links {
			if fr, _ := s.FragmentsByIDs([]uint64{e.From}); len(fr) == 1 {
				out = append(out, fr[0].Content)
			}
		}
		return out
	}
	countNamed := func(name string) int {
		cs, _ := s.Concepts()
		n := 0
		for _, c := range cs {
			if c.Content == name && c.Category == "noun" {
				n++
			}
		}
		return n
	}
	for i := 0; i < 6; i++ {
		say(t, d, "a model predicts a word")
		say(t, d, "an llm predicts a word")
		say(t, d, "a model learns a rule")
		say(t, d, "an llm learns a pattern")
		say(t, d, "a model reads a text")
		say(t, d, "an llm reads a sentence")
		d.sleepCycle()
	}
	if n := namesOf("model"); len(n) != 1 || n[0] != "model" {
		t.Errorf("a percept-less concept should be named only by its own word, got %v", n)
	}
	for _, w := range []string{"model", "llm", "rule", "pattern", "word", "text"} {
		if c := countNamed(w); c != 1 {
			t.Errorf("%q should be one concept after consolidation, got %d", w, c)
		}
	}
	// The featured backdrop keeps its own names intact (the guards never touched it).
	if n := namesOf("dog"); len(n) != 1 || n[0] != "dog" {
		t.Errorf("a featured concept should keep its own name, got %v", n)
	}
}

// perceptlessSet picks out the concepts that ground in nothing -- the abstract words
// the naming and abstraction guards scope themselves to -- and leaves alone every
// concept with a percept beneath it or a feature of its own. This is the line the
// guards draw, so the abstract vocabulary is held to its direct names while the
// perceived world keeps cross-situational naming untouched.
func TestPerceptlessSet(t *testing.T) {
	d, s := testDaemon(t)
	say(t, d, "a model predicts a word") // model: an abstract word, grounds in nothing
	model := d.resolveConcept("model")
	dog := groundNoun(t, d, "dog") // featured concept
	furry := &memory.Fragment{Kind: memory.KindFeature, Channel: "feature", Content: "furry",
		Template: "furry", Limbic: d.chem.Snapshot(), Time: time.Now()}
	if _, err := s.Encode(furry); err != nil {
		t.Fatal(err)
	}
	s.SetLinkRel(dog.ID, furry.ID, memory.RelFeature, 1)
	cat := groundNoun(t, d, "cat") // perceptually grounded concept
	groundOccurrences(t, d, cat, 3)

	set := d.perceptlessSet([]uint64{model.ID, dog.ID, cat.ID})
	if !set[model.ID] {
		t.Error("an abstract word grounded only in the propositions that use it is percept-less")
	}
	if set[dog.ID] {
		t.Error("a featured concept is not percept-less")
	}
	if set[cat.ID] {
		t.Error("a perceptually grounded concept is not percept-less")
	}
	if d.isPerceptless(dog.ID) || !d.isPerceptless(model.ID) {
		t.Error("isPerceptless should agree with the set on a single concept")
	}
}

// The hygiene audit surfaces the two shapes consolidation pollution takes and passes
// the graph's legitimate structure by. A clean brain reports neither shape. A second
// node under one gloss is a duplicate; a word that strays onto a percept-less concept
// past the one it names is a loose word; a category word sitting weakly beside its
// grounded members is neither, so it is not flagged.
func TestHygieneReport(t *testing.T) {
	d, s := testDaemon(t)
	say(t, d, "a model predicts a word") // model, word: percept-less abstract concepts
	model := d.resolveConcept("model")
	cat := groundNoun(t, d, "cat") // a grounded concept the audit must leave alone
	groundOccurrences(t, d, cat, 3)

	if h, _ := d.hygieneReport(); len(h.Duplicates) != 0 || len(h.Promiscuous) != 0 {
		t.Fatalf("a clean brain reports neither shape: dups=%v loose=%v", h.Duplicates, h.Promiscuous)
	}

	// A stray: the "cat" word, which names the grounded cat firmly, also bonds to the
	// percept-less model. That is the cross-naming, and only that should flag.
	catWord, _, err := s.WordNode("cat", "en", d.chem.Snapshot(), time.Now())
	if err != nil {
		t.Fatal(err)
	}
	s.SetLinkRel(catWord.ID, model.ID, memory.RelNames, 0.2)
	// A duplicate: a second concept node under the gloss "cat".
	dup := &memory.Fragment{Kind: memory.KindConcept, Channel: "concept", Content: "cat",
		Template: "noun:en:cat-shadow", Category: "noun", Limbic: d.chem.Snapshot(), Time: time.Now()}
	if _, err := s.Encode(dup); err != nil {
		t.Fatal(err)
	}

	h, _ := d.hygieneReport()
	if len(h.Duplicates) != 1 || h.Duplicates[0].Gloss != "cat" {
		t.Errorf("the second cat node should be flagged a duplicate: %+v", h.Duplicates)
	}
	flagged := false
	for _, lw := range h.Promiscuous {
		if lw.Word == "cat" {
			flagged = true
		}
	}
	if !flagged {
		t.Errorf("the cat word straying onto a percept-less concept should be flagged: %+v", h.Promiscuous)
	}
}
