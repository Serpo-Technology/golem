package daemon

import (
	"strings"
	"testing"
	"time"

	"golem/internal/memory"
)

// groundOccurrences grounds a concept in experience the way perception does: it
// points the concept at an episode standing for n sightings, so the concept's
// grounding tally is positive. groundNoun alone only names a concept; knowing a
// thing is having grounded it.
func groundOccurrences(t *testing.T, d *Daemon, c *memory.Fragment, n int) {
	t.Helper()
	ep := &memory.Fragment{Kind: memory.KindEpisodic, Channel: "vision", Content: c.Content,
		Count: n, Limbic: d.chem.Snapshot(), Time: time.Now()}
	if _, err := d.store.Encode(ep); err != nil {
		t.Fatal(err)
	}
	if err := d.store.SetLinkRel(c.ID, ep.ID, memory.RelGrounds, 0.9); err != nil {
		t.Fatal(err)
	}
}

// A grounded thing is known: "do you know the lion" affirms once the lion has
// been seen, and names what the question asked about.
func TestEpistemicKnowsGroundedThing(t *testing.T) {
	d, _ := testDaemon(t)
	lion := groundNoun(t, d, "lion")
	groundOccurrences(t, d, lion, 3)
	got := strings.ToLower(say(t, d, "do you know the lion").Say)
	if !strings.Contains(got, "yes") || !strings.Contains(got, "i know the lion") {
		t.Errorf("a grounded thing should be known: %q", got)
	}
}

// A thing whose word resolves but which was never grounded is not known: knowing
// is grounding, not merely being able to resolve a name.
func TestEpistemicUngroundedNotKnown(t *testing.T) {
	d, _ := testDaemon(t)
	groundNoun(t, d, "lion") // named, never seen
	got := strings.ToLower(say(t, d, "do you know the lion").Say)
	if !strings.Contains(got, "do not know") {
		t.Errorf("an ungrounded thing should not be known: %q", got)
	}
}

// A thing known only by description is known. An abstract word names no percept, so
// it never gains a grounding tally, yet once a fact has been told of it GOLEM knows
// it, by description rather than acquaintance, the same facts it can report back.
func TestEpistemicKnowsByDescription(t *testing.T) {
	d, _ := testDaemon(t)
	say(t, d, "a model predicts a word") // mints "model"; no percept ever grounds it
	got := strings.ToLower(say(t, d, "do you know a model").Say)
	if !strings.HasPrefix(got, "yes") || !strings.Contains(got, "model") {
		t.Errorf("a thing told of is known by description, even unperceived: %q", got)
	}
}

// A thing with no concept at all is not known, and the question is still caught,
// so the imperative misreading of "do" never surfaces.
func TestEpistemicUnknownThingCaught(t *testing.T) {
	d, _ := testDaemon(t)
	got := strings.ToLower(say(t, d, "do you know a dragon").Say)
	if !strings.Contains(got, "do not know") {
		t.Errorf("an unknown thing should not be known: %q", got)
	}
	if strings.Contains(got, "how to") {
		t.Errorf("the clause should not leak as a command: %q", got)
	}
}

// Every auxiliary that fronts the question is read the same way: "did you know"
// is the reflexive epistemic too, not a polar that misses to "No".
func TestEpistemicDidOpener(t *testing.T) {
	d, _ := testDaemon(t)
	cat := groundNoun(t, d, "cat")
	groundOccurrences(t, d, cat, 2)
	got := strings.ToLower(say(t, d, "did you know the cat").Say)
	if !strings.Contains(got, "yes") {
		t.Errorf("did-you-know should read as the epistemic question: %q", got)
	}
}

// The epistemic question is recognized off the frame rather than a token pattern: the know
// verb is a closed-class speech move, so "do you know X" carries frame.Speech == "know" with
// no wh word, which sets it apart from the "what do you know" forms that carry a wh word and
// route to the summary and the describe instead. A new auxiliary or phrasing rides the same
// frame, not a new token list.
func TestEpistemicRoutesOffKnowVerb(t *testing.T) {
	d, _ := testDaemon(t)
	lion := groundNoun(t, d, "lion")
	groundOccurrences(t, d, lion, 3)

	// The know verb marks the clause epistemic; the polar concept question carries no wh word.
	if f := d.gram.ParseHint("do you know the lion", "en", d.resolveConcept); f.Speech != "know" || f.Wh != "" {
		t.Fatalf("do you know X should be an epistemic clause with no wh word: speech %q wh %q", f.Speech, f.Wh)
	}
	// The about-X form carries the same know verb but a wh word, so it is not the polar
	// epistemic and routes to the describe instead.
	if f := d.gram.ParseHint("what do you know about the lion", "en", d.resolveConcept); f.Speech != "know" || f.Wh == "" {
		t.Fatalf("what do you know about X should carry a wh word: speech %q wh %q", f.Speech, f.Wh)
	}
	// End to end, the polar question answers yes from grounding, off the frame.
	if got := strings.ToLower(say(t, d, "do you know the lion").Say); !strings.Contains(got, "yes") {
		t.Fatalf("a grounded thing should be known: %q", got)
	}
	// The about-X form still describes, not the yes/no the polar epistemic gives.
	if got := strings.ToLower(say(t, d, "what do you know about the lion").Say); strings.HasPrefix(got, "yes") || strings.HasPrefix(got, "no") {
		t.Fatalf("what do you know about X should describe, not answer the polar epistemic: %q", got)
	}
}

// "Do I know X" asks of the speaker, whose mind GOLEM cannot read, so it admits
// the limit rather than answering from its own grounding.
func TestEpistemicSpeakerDeferred(t *testing.T) {
	d, _ := testDaemon(t)
	lion := groundNoun(t, d, "lion")
	groundOccurrences(t, d, lion, 3)
	got := strings.ToLower(say(t, d, "do i know the lion").Say)
	if !strings.Contains(got, "cannot tell what you know") {
		t.Errorf("a question about the speaker's knowledge should be deferred: %q", got)
	}
}

// The reflexive know-question answers from the same grounding in Turkish: "X
// biliyor musun" affirms a grounded thing and denies an ungrounded one.
func TestEpistemicConceptTurkish(t *testing.T) {
	d, _ := testDaemon(t)
	kedi := groundNounLang(t, d, "kedi", "tr")
	groundOccurrences(t, d, kedi, 3)
	groundNounLang(t, d, "köpek", "tr") // named, never grounded

	if got := sayLang(t, d, "tr", "kediyi biliyor musun").Say; !strings.HasPrefix(got, "Evet") || !strings.Contains(got, "biliyorum") {
		t.Errorf("a grounded thing should be known in Turkish: %q", got)
	}
	if got := sayLang(t, d, "tr", "köpeği biliyor musun").Say; !strings.HasPrefix(got, "Hayır") {
		t.Errorf("an ungrounded thing should not be known in Turkish: %q", got)
	}
}

// And in Latin: "scisne X" reads the accusative object and answers from the same
// grounding, ita for a thing known and minime for one not.
func TestEpistemicConceptLatin(t *testing.T) {
	d, _ := testDaemon(t)
	puella := groundLatinPair(t, d, "girl", "girl", "puella")
	groundOccurrences(t, d, puella, 3)
	groundLatinPair(t, d, "rose", "rose", "rosa") // named, never grounded

	if got := sayLang(t, d, "la", "scisne puellam").Say; !strings.HasPrefix(got, "Ita") || !strings.Contains(got, "scio") {
		t.Errorf("a grounded thing should be known in Latin: %q", got)
	}
	if got := sayLang(t, d, "la", "scisne rosam").Say; !strings.HasPrefix(got, "Minime") {
		t.Errorf("an ungrounded thing should not be known in Latin: %q", got)
	}
}

// A proposition can be the object of know: "do you know that the dog chased the
// cat" asks whether GOLEM holds that fact, settled against the graph the way a
// polar question is, not by resolving an incidental noun in it.
func TestEpistemicPropositionHeld(t *testing.T) {
	d, _ := testDaemon(t)
	dog := groundNoun(t, d, "dog")
	groundNoun(t, d, "cat")
	say(t, d, "the dog chased the cat") // GOLEM is told the fact

	// The fact it was told, it knows, naming the proposition back.
	if got := say(t, d, "do you know that the dog chased the cat").Say; !strings.HasPrefix(got, "Yes") || !strings.Contains(got, "dog chased the cat") {
		t.Errorf("a told proposition should be known: %q", got)
	}
	// The reverse, never told, it does not know: word order fixes the roles.
	if got := say(t, d, "do you know that the cat chased the dog").Say; !strings.HasPrefix(got, "No") {
		t.Errorf("an unheld proposition should not be known: %q", got)
	}
	// "Do you know that dog" is the demonstrative, not a proposition, so it falls
	// through to the concept check.
	groundOccurrences(t, d, dog, 2)
	if got := say(t, d, "do you know that dog").Say; !strings.HasPrefix(got, "Yes") {
		t.Errorf("the demonstrative should fall through to the concept check: %q", got)
	}
}

// The propositional know-question reads in Turkish too: the -DIK nominalization
// "köpeğin kediyi kovaladığını" names the proposition, settled against the graph.
func TestEpistemicPropositionTurkish(t *testing.T) {
	d, _ := testDaemon(t)
	groundNounLang(t, d, "köpek", "tr")
	groundNounLang(t, d, "kedi", "tr")
	sayLang(t, d, "tr", "köpek kediyi kovaladı") // GOLEM is told the fact

	if got := sayLang(t, d, "tr", "köpeğin kediyi kovaladığını biliyor musun").Say; !strings.HasPrefix(got, "Evet") {
		t.Errorf("a told proposition should be known in Turkish: %q", got)
	}
	if got := sayLang(t, d, "tr", "kedinin köpeği kovaladığını biliyor musun").Say; !strings.HasPrefix(got, "Hayır") {
		t.Errorf("an unheld proposition should not be known in Turkish: %q", got)
	}

	// An n-final subject ("aslan" -> "aslanın") must not over-strip the genitive to
	// "asla"; the proposition still resolves and answers.
	groundNounLang(t, d, "aslan", "tr")
	groundNounLang(t, d, "fare", "tr")
	sayLang(t, d, "tr", "aslan fareyi kovaladı")
	if got := sayLang(t, d, "tr", "aslanın fareyi kovaladığını biliyor musun").Say; !strings.HasPrefix(got, "Evet") {
		t.Errorf("an n-final subject's genitive should resolve: %q", got)
	}
}

// And in Latin: "scisne quod <clause>" comprehends the clause by case and answers
// from the same observed-vs-guessed bar.
func TestEpistemicPropositionLatin(t *testing.T) {
	d, _ := testDaemon(t)
	groundLatinPair(t, d, "girl", "girl", "puella")
	groundLatinPair(t, d, "rose", "rose", "rosa")
	say(t, d, "the girl loves the rose")
	sayLang(t, d, "la", "puella rosam amat") // bind amat and tell the fact in Latin

	if got := sayLang(t, d, "la", "scisne quod puella rosam amat").Say; !strings.HasPrefix(got, "Ita") {
		t.Errorf("a told proposition should be known in Latin: %q", got)
	}
	if got := sayLang(t, d, "la", "scisne quod rosa puellam amat").Say; !strings.HasPrefix(got, "Minime") {
		t.Errorf("an unheld proposition should not be known in Latin: %q", got)
	}
}

// "What do you know" is the open self-report: it reads the whole graph and names
// the kinds GOLEM has formed and the actions it has learned, where "what do you
// know about X" reads a single concept.
func TestKnowledgeSummaryReportsKindsAndActions(t *testing.T) {
	d, _ := testDaemon(t)
	dog := groundNoun(t, d, "dog")
	cat := groundNoun(t, d, "cat")
	mouse := groundNoun(t, d, "mouse")
	say(t, d, "a dog chased a cat") // mints the chase relation

	// A true superordinate GOLEM has formed: a kind generalizing over several
	// members, where a basic noun's own abstraction grounds in just its one percept.
	animal := &memory.Fragment{Kind: memory.KindConcept, Channel: "concept", Content: "animal",
		Template: "abs:en:animal", Category: "noun", Limbic: d.chem.Snapshot(), Time: time.Now()}
	if _, err := d.store.Encode(animal); err != nil {
		t.Fatal(err)
	}
	w, _, err := d.store.WordNode("animal", "en", d.chem.Snapshot(), time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if err := d.store.SetLinkRel(w.ID, animal.ID, memory.RelNames, 0.9); err != nil {
		t.Fatal(err)
	}
	for _, m := range []*memory.Fragment{dog, cat, mouse} {
		if err := d.store.SetLinkRel(animal.ID, m.ID, memory.RelGrounds, 0.8); err != nil {
			t.Fatal(err)
		}
	}

	got := say(t, d, "what do you know").Say
	if !strings.Contains(got, "animal") {
		t.Errorf("the summary should name a kind it formed: %q", got)
	}
	if !strings.Contains(got, "chase") {
		t.Errorf("the summary should name an action it learned: %q", got)
	}
	// The open form must not be read as a description of GOLEM itself.
	if strings.Contains(strings.ToLower(got), "you names") || strings.Contains(got, "GOLEM") {
		t.Errorf("the open question should summarize knowledge, not describe the self: %q", got)
	}
}

// The about-form and the summary reach parity too. Turkish "X hakkında ne
// biliyorsun" describes the concept, and "ne biliyorsun" reports the kinds GOLEM
// has formed, both phrased in Turkish.
func TestKnowledgeAboutAndSummaryTurkish(t *testing.T) {
	d, _ := testDaemon(t)
	kedi := groundNounLang(t, d, "kedi", "tr")
	kopek := groundNounLang(t, d, "köpek", "tr")
	fare := groundNounLang(t, d, "fare", "tr")
	sayLang(t, d, "tr", "köpek kediyi kovaladı")

	if got := sayLang(t, d, "tr", "köpek hakkında ne biliyorsun").Say; !strings.Contains(strings.ToLower(got), "köpek") {
		t.Errorf("the Turkish about-form should describe the concept: %q", got)
	}

	// A Turkish superordinate GOLEM has formed: a kind the summary names.
	hayvan := &memory.Fragment{Kind: memory.KindConcept, Channel: "concept", Content: "hayvan",
		Template: "abs:tr:hayvan", Category: "noun", Limbic: d.chem.Snapshot(), Time: time.Now()}
	if _, err := d.store.Encode(hayvan); err != nil {
		t.Fatal(err)
	}
	w, _, err := d.store.WordNode("hayvan", "tr", d.chem.Snapshot(), time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if err := d.store.SetLinkRel(w.ID, hayvan.ID, memory.RelNames, 0.9); err != nil {
		t.Fatal(err)
	}
	for _, m := range []*memory.Fragment{kedi, kopek, fare} {
		if err := d.store.SetLinkRel(hayvan.ID, m.ID, memory.RelGrounds, 0.8); err != nil {
			t.Fatal(err)
		}
	}
	got := sayLang(t, d, "tr", "ne biliyorsun").Say
	if !strings.Contains(strings.ToLower(got), "hayvan") || !strings.Contains(got, "biliyorum") {
		t.Errorf("the Turkish summary should name the kind in Turkish: %q", got)
	}
}

// Latin reaches the same surfaces: "quid de X scis" describes X, and "quid scis"
// summarizes, the empty summary spoken as the Latin gap.
func TestKnowledgeAboutAndSummaryLatin(t *testing.T) {
	d, _ := testDaemon(t)
	groundLatinPair(t, d, "girl", "girl", "puella")
	groundLatinPair(t, d, "rose", "rose", "rosa")
	say(t, d, "the girl loves the rose")
	sayLang(t, d, "la", "puella rosam amat")

	if got := sayLang(t, d, "la", "quid de puella scis").Say; got == "" {
		t.Error("the Latin about-form should describe the concept")
	}
	if got := sayLang(t, d, "la", "quid scis").Say; got != "Parum novi." {
		t.Errorf("an empty Latin summary should be the Latin gap: %q", got)
	}
}

// "What do you know about X" still reads the single concept, not the whole-graph
// summary: the open form must not swallow the focused one.
func TestKnowledgeSummaryLeavesAboutForm(t *testing.T) {
	d, _ := testDaemon(t)
	groundNoun(t, d, "dog")
	groundNoun(t, d, "cat")
	say(t, d, "a dog chased a cat")
	got := say(t, d, "what do you know about the dog").Say
	if !strings.Contains(strings.ToLower(got), "dog") {
		t.Errorf("the about-form should describe the named concept: %q", got)
	}
	if strings.Contains(got, "kinds like") {
		t.Errorf("the about-form should not give the whole-graph summary: %q", got)
	}
}

// The recognizer is specific: an ordinary "does" question, not a "do you know",
// is left alone, so a real polar still answers from the graph.
func TestEpistemicLeavesOrdinaryQuestion(t *testing.T) {
	d, _ := testDaemon(t)
	groundNoun(t, d, "dog")
	groundNoun(t, d, "cat")
	say(t, d, "a dog chased a cat")
	got := strings.ToLower(say(t, d, "does the dog chase the cat").Say)
	if strings.Contains(got, "i know") || strings.Contains(got, "cannot tell") {
		t.Errorf("an ordinary polar should not be read as epistemic: %q", got)
	}
}
