package daemon

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"golem/internal/attention"
	"golem/internal/memory"
	"golem/internal/wire"
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

// Serotonin paces genesis: a system at the alarm floor abstracts nothing
// that night.
func TestSerotoninPacesConceptGenesis(t *testing.T) {
	d, s := testDaemon(t)
	feedShape(t, s, "syslog", "Disk full on /dev/disk1s9", 4, time.Now().Add(-time.Hour))
	if _, err := s.MergeDuplicates(); err != nil {
		t.Fatal(err)
	}

	d.chem.SuppressSleep(1.0) // floored at 0.05: an alarmed night
	born, _ := d.growConcepts()
	if born != 0 {
		t.Fatalf("an alarmed system should not abstract, got %d concept(s)", born)
	}
	d.chem.WakeRested()
	born, _ = d.growConcepts()
	if born != 1 {
		t.Fatalf("a rested system should distil the shape, got %d", born)
	}
}

// Words bond to the concepts their episodes ground: tell words near the
// moments, sleep, and the names edge appears.
func TestNamingBondsWordsToConcepts(t *testing.T) {
	d, s := testDaemon(t)
	base := time.Now().Add(-2 * time.Hour)

	anchors := feedShape(t, s, "shell", "run: go test ./...", 4, base)
	words := d.mintWords("the tests are running", "en", 0, anchors)
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
	en := d.mintWords("tests", "en", 0, anchors)
	tr := d.mintWords("testler", "tr", 0, anchors)
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
	d.mintWords("depolama", "tr", 0, anchors) // no substring overlap with the moments
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
	d.mintWords("disk", "en", 0, anchors)
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
	if n, _ := s.CountKind(memory.KindWord); n != 0 {
		t.Fatalf("an edgeless word should prune, %d held", n)
	}
	if n, _ := s.CountKind(memory.KindConcept); n != 0 {
		t.Fatalf("a groundless concept should prune, %d held", n)
	}
}

// A grounded concept and a naming word survive the same sleep that kills
// disconnected ones.
func TestConnectedSemanticNodesSurviveSleep(t *testing.T) {
	d, s := testDaemon(t)
	anchors := feedShape(t, s, "syslog", "Disk full on /dev/disk1s9", 4, time.Now().Add(-time.Hour))
	d.mintWords("disk", "en", 0, anchors)
	if _, err := s.MergeDuplicates(); err != nil {
		t.Fatal(err)
	}
	d.growConcepts()

	if _, err := s.Sleep(0.10, 0.10); err != nil {
		t.Fatal(err)
	}
	if n, _ := s.CountKind(memory.KindConcept); n != 1 {
		t.Fatalf("a grounded concept must survive sleep, %d held", n)
	}
	if n, _ := s.CountKind(memory.KindWord); n != 1 {
		t.Fatalf("a bonded word must survive sleep, %d held", n)
	}
}

// The concepts listing shows the layer whole: gloss, category, names per
// language, and the grounding tally.
func TestConceptInfosListing(t *testing.T) {
	d, s := testDaemon(t)
	anchors := feedShape(t, s, "shell", "run: go test ./...", 4, time.Now().Add(-time.Hour))
	d.mintWords("tests", "en", 0, anchors)
	d.mintWords("testler", "tr", 0, anchors)
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
	for _, c := range concepts {
		if strings.Contains(c.Content, "usually happens") {
			t.Fatalf("an utterance shape must not distil into a concept: %q", c.Content)
		}
	}
	if len(concepts) == 0 || concepts[0].Content != "disk space low" {
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
	d.mintWords("disk", "en", 0, disk)
	scattered := []recentMoment{disk[0], cpu[0], net[0]}
	d.mintWords("low", "en", 0, scattered)

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
	d.mintWords("blinker", "en", 0, disk[:1])
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
		d.mintWords("blinker", "en", 0, cpu[:1])
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
