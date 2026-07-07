package daemon

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"golem/internal/grammar"
	"golem/internal/memory"
	"golem/wire"
)

// groundNoun seeds a concept and a word that names it, so the planner's
// resolver reaches the concept from its surface form the way it would after
// the concept distilled from lived experience.
func groundNoun(t *testing.T, d *Daemon, gloss string) *memory.Fragment {
	return groundNounLang(t, d, gloss, "en")
}

// groundNounLang is groundNoun in a chosen language, for grounding a concept
// a non-English word names.
func groundNounLang(t *testing.T, d *Daemon, gloss, lang string) *memory.Fragment {
	t.Helper()
	now := time.Now()
	c := &memory.Fragment{Kind: memory.KindConcept, Channel: "concept", Content: gloss,
		Template: gloss, Category: "noun", Limbic: d.chem.Snapshot(), Time: now}
	if _, err := d.store.Encode(c); err != nil {
		t.Fatal(err)
	}
	w, _, err := d.store.WordNode(gloss, lang, d.chem.Snapshot(), now)
	if err != nil {
		t.Fatal(err)
	}
	if err := d.store.SetLinkRel(w.ID, c.ID, memory.RelNames, 0.9); err != nil {
		t.Fatal(err)
	}
	return c
}

// nameInLang adds a second tongue's name to an existing concept, so one concept
// is named in both languages (the leaf is "leaf" and "yaprak").
func nameInLang(t *testing.T, d *Daemon, c *memory.Fragment, word, lang string) {
	t.Helper()
	now := time.Now()
	w, _, err := d.store.WordNode(word, lang, d.chem.Snapshot(), now)
	if err != nil {
		t.Fatal(err)
	}
	if err := d.store.SetLinkRel(w.ID, c.ID, memory.RelNames, 0.9); err != nil {
		t.Fatal(err)
	}
}

// sayLang tells GOLEM an utterance tagged with its language, the path the
// CLI takes for "tell --lang tr".
func sayLang(t *testing.T, d *Daemon, lang, words string) wire.Response {
	t.Helper()
	resp, _, _ := d.handle(wire.Request{Cmd: "tell", Content: words, Lang: lang}, nil)
	if !resp.OK {
		t.Fatalf("tell %q failed: %s", words, resp.Error)
	}
	return resp
}

// The done-when of the comprehension producer: "a dog chased a cat," told to
// a brain that has grounded dog and cat, parses into chaser and chased and is
// written as a proposition. The relation is minted and grounds in the moment
// of its telling, and the moment points at the dog as agent and the cat as
// patient.
func TestComprehendWritesProposition(t *testing.T) {
	d, s := testDaemon(t)
	dog := groundNoun(t, d, "dog")
	cat := groundNoun(t, d, "cat")

	resp := say(t, d, "a dog chased a cat")
	moment := resp.Observe.FragmentID
	if moment == 0 {
		t.Fatal("the telling should leave a moment")
	}

	chase, err := s.ConceptByTemplate("rel:en:chase")
	if err != nil {
		t.Fatal(err)
	}
	if chase == nil {
		t.Fatal("the verb should mint a relation concept")
	}
	if chase.Category != "relation" {
		t.Fatalf("chase should be a relation, got category %q", chase.Category)
	}

	grounds, _ := s.LinksFromRel([]uint64{chase.ID}, memory.RelGrounds)
	if !linksTo(grounds, moment) {
		t.Fatalf("chase should ground in the telling moment: %+v", grounds)
	}
	ag, _ := s.LinksFromRel([]uint64{moment}, memory.RelAgent)
	if !linksTo(ag, dog.ID) {
		t.Fatalf("the dog should be the agent: %+v", ag)
	}
	pat, _ := s.LinksFromRel([]uint64{moment}, memory.RelPatient)
	if !linksTo(pat, cat.ID) {
		t.Fatalf("the cat should be the patient: %+v", pat)
	}
}

// Once the proposition exists, the walk that crosses a relation reaches the
// co-argument: from the dog to the cat it chased, the dormant crossing lit.
func TestComprehendLightsWalkCrossing(t *testing.T) {
	d, _ := testDaemon(t)
	dog := groundNoun(t, d, "dog")
	cat := groundNoun(t, d, "cat")
	say(t, d, "a dog chased a cat")

	dests, err := d.walkableDests(dog.ID)
	if err != nil {
		t.Fatal(err)
	}
	crossed := false
	for _, dst := range dests[memory.RelPatient] {
		if dst.id == cat.ID {
			crossed = true
			if dst.via != "chase" {
				t.Fatalf("the crossing should name the relation, got %q", dst.via)
			}
		}
	}
	if !crossed {
		t.Fatal("the walk from the dog should cross to the cat it chased")
	}
}

// A second relation taught the same way stands apart from the first, so the
// brain accrues distinct relations rather than collapsing them into one.
func TestComprehendDistinctRelations(t *testing.T) {
	d, s := testDaemon(t)
	groundNoun(t, d, "dog")
	groundNoun(t, d, "cat")
	groundNoun(t, d, "ball")
	say(t, d, "a dog chased a cat")
	say(t, d, "a dog fetched a ball")

	chase, _ := s.ConceptByTemplate("rel:en:chase")
	fetch, _ := s.ConceptByTemplate("rel:en:fetch")
	if chase == nil || fetch == nil {
		t.Fatal("both verbs should mint relations")
	}
	if chase.ID == fetch.ID {
		t.Fatal("the two relations should be distinct concepts")
	}
}

// A relation warmed by talk in one tongue lends its identity to its first verb
// in another. Telling a proposition in Turkish while the chase relation is warm
// from English binds the Turkish verb to the one chase relation rather than
// minting a parallel one, so a question in either tongue reaches the same
// proposition. The conceptual thread survives the change of tongue, language
// kept separate from reasoning.
func TestRelationUnifiesAcrossTonguesByWarmth(t *testing.T) {
	d, s := testDaemon(t)
	dog := groundNoun(t, d, "dog")
	cat := groundNoun(t, d, "cat")
	// The same concepts are named in Turkish too, the way a noun is shared.
	addName := func(form, lang string, conceptID uint64) {
		w, _, err := s.WordNode(form, lang, d.chem.Snapshot(), time.Now())
		if err != nil {
			t.Fatal(err)
		}
		if err := s.SetLinkRel(w.ID, conceptID, memory.RelNames, 0.9); err != nil {
			t.Fatal(err)
		}
	}
	addName("köpek", "tr", dog.ID)
	addName("kedi", "tr", cat.ID)

	// Establish and warm the chase relation in English.
	say(t, d, "a dog chased a cat")
	chase, _ := s.ConceptByTemplate("rel:en:chase")
	if chase == nil {
		t.Fatal("English should mint the chase relation")
	}

	// Tell the same in Turkish while chase is warm: the Turkish verb binds to it.
	sayLang(t, d, "tr", "köpek kediyi kovaladı")

	if parallel, _ := s.ConceptByTemplate("rel:tr:kovala"); parallel != nil {
		t.Fatalf("no parallel Turkish relation should be minted: %+v", parallel)
	}
	if got := d.resolveRelation("kovala"); got == nil || got.ID != chase.ID {
		t.Fatalf("the Turkish verb should name the one chase relation: %+v", got)
	}
	if !d.relationNamedIn(chase.ID, "tr") || !d.relationNamedIn(chase.ID, "en") {
		t.Fatal("the one chase relation should be named in both tongues")
	}
}

// Warmth does not merge two verbs of one tongue: a relation already spoken in a
// tongue keeps its name there, so a second English verb is its own relation
// even with the first still warm.
func TestRelationWarmthDoesNotMergeSameTongue(t *testing.T) {
	d, s := testDaemon(t)
	groundNoun(t, d, "dog")
	groundNoun(t, d, "cat")
	groundNoun(t, d, "ball")
	say(t, d, "a dog chased a cat")
	say(t, d, "a dog fetched a ball")

	chase, _ := s.ConceptByTemplate("rel:en:chase")
	fetch, _ := s.ConceptByTemplate("rel:en:fetch")
	if chase == nil || fetch == nil {
		t.Fatal("both verbs should mint relations")
	}
	if chase.ID == fetch.ID {
		t.Fatal("two verbs of one tongue stay two relations even when one is warm")
	}
}

// A novel verb degrades gracefully rather than failing to parse: an unknown
// verb in transitive position becomes a candidate relation with its slots
// bound, and the brain acknowledges the telling rather than blurting a
// confused question about it.
func TestComprehendUnknownVerbDegradesGracefully(t *testing.T) {
	d, s := testDaemon(t)
	dog := groundNoun(t, d, "dog")
	cat := groundNoun(t, d, "cat")

	resp := say(t, d, "a dog blorped a cat")
	if resp.Say != "I see." {
		t.Fatalf("an understood novel statement is acknowledged, nothing more: %q", resp.Say)
	}
	blorp, _ := s.ConceptByTemplate("rel:en:blorp")
	if blorp == nil || blorp.Category != "relation" {
		t.Fatal("an unknown verb should become a candidate relation")
	}
	moment := resp.Observe.FragmentID
	ag, _ := s.LinksFromRel([]uint64{moment}, memory.RelAgent)
	pat, _ := s.LinksFromRel([]uint64{moment}, memory.RelPatient)
	if !linksTo(ag, dog.ID) || !linksTo(pat, cat.ID) {
		t.Fatalf("the unknown verb should still leave its slots bound: agent %+v patient %+v", ag, pat)
	}
}

// An unknown noun in a told proposition is acquired, not held as a gap: "a dog
// chased a wug" mints "wug" and writes the whole proposition, the way it already
// mints the unknown verb. The acquisition is by usage, so the minted concept has no
// features and is placed by the company it keeps.
func TestComprehendAcquiresUnknownArgument(t *testing.T) {
	d, s := testDaemon(t)
	dog := groundNoun(t, d, "dog")

	resp := say(t, d, "a dog chased a wug")
	moment := resp.Observe.FragmentID

	chase, _ := s.ConceptByTemplate("rel:en:chase")
	if chase == nil {
		t.Fatal("the relation should be written")
	}
	wug := d.resolveConcept("wug")
	if wug == nil {
		t.Fatal("the unknown patient should be minted as a concept")
	}
	ag, _ := s.LinksFromRel([]uint64{moment}, memory.RelAgent)
	if !linksTo(ag, dog.ID) {
		t.Fatal("the dog should be the agent")
	}
	pat, _ := s.LinksFromRel([]uint64{moment}, memory.RelPatient)
	if !linksTo(pat, wug.ID) {
		t.Fatal("the minted wug should be the patient of the whole proposition")
	}
}

// The done-when's third clause: an answer the old planner hardcoded now comes
// from a meaning structure run through the engine. After "a dog chased a cat"
// is comprehended and written, "what did the dog chase" is answered by
// crossing the relation to the co-argument and speaking the proposition back
// through the same construction, and "who chased the cat" answers the agent.
func TestRoleQuestionAnsweredThroughEngine(t *testing.T) {
	d, _ := testDaemon(t)
	groundNoun(t, d, "dog")
	groundNoun(t, d, "cat")
	say(t, d, "a dog chased a cat")

	if got := say(t, d, "what did the dog chase").Say; got != "The dog chased the cat." {
		t.Fatalf("the patient question should be answered from the proposition: %q", got)
	}
	if got := say(t, d, "who chased the cat").Say; got != "The dog chased the cat." {
		t.Fatalf("the agent question should be answered from the proposition: %q", got)
	}
}

// A role question about a relation that holds no such proposition admits the
// gap rather than inventing an answer.
func TestRoleQuestionAdmitsGap(t *testing.T) {
	d, _ := testDaemon(t)
	groundNoun(t, d, "dog")
	groundNoun(t, d, "cat")
	groundNoun(t, d, "ball")
	say(t, d, "a dog chased a cat")

	// The dog chased the cat, not the ball; asking what chased the ball finds
	// no agent for that proposition.
	got := say(t, d, "what chased the ball").Say
	if got == "The dog chased the ball." {
		t.Fatalf("a proposition that was never told should not be invented: %q", got)
	}
}

// A relation is first-class in answers too: "what is chase" is answered that
// chase is an action, not a thing, the kind a verb names.
func TestRelationDefinedAsAction(t *testing.T) {
	d, _ := testDaemon(t)
	groundNoun(t, d, "dog")
	groundNoun(t, d, "cat")
	say(t, d, "a dog chased a cat") // mints chase as a relation

	got := say(t, d, "what is chase").Say
	if !strings.HasPrefix(got, "Chase is an action.") {
		t.Fatalf("a relation should be defined as an action: %q", got)
	}
}

// "What did the dog do" asks the relation itself: the walk reads the
// strongest proposition the dog took part in and the engine speaks the whole
// clause back, an answer built from a meaning structure.
func TestActivityQuestionThroughEngine(t *testing.T) {
	d, _ := testDaemon(t)
	groundNoun(t, d, "dog")
	groundNoun(t, d, "cat")
	say(t, d, "a dog chased a cat")

	if got := say(t, d, "what did the dog do").Say; got != "The dog chased the cat." {
		t.Fatalf("the activity question should speak the whole proposition: %q", got)
	}
}

// "What did the dog do" reports activities across relations: with the dog both
// chasing and fetching, the answer names both, the later clause carrying the
// agent as "it", rather than collapsing to a single relation.
func TestActivityQuestionCoordinatesRelations(t *testing.T) {
	d, _ := testDaemon(t)
	for _, n := range []string{"dog", "cat", "ball"} {
		groundNoun(t, d, n)
	}
	say(t, d, "a dog chased a cat")
	say(t, d, "a dog fetched a ball")

	got := say(t, d, "what did the dog do").Say
	if !strings.Contains(got, "chased") || !strings.Contains(got, "fetched") {
		t.Fatalf("the activity answer should report both relations: %q", got)
	}
	if !strings.Contains(got, "cat") || !strings.Contains(got, "ball") {
		t.Fatalf("the activity answer should name both co-arguments: %q", got)
	}
}

// "Tell me about X" is the open case of the one generator: with no filter it
// says what the focus is and the propositions it took part in, the same walk
// the role and activity questions narrow.
func TestDescribeFocusReportsIdentityAndFacts(t *testing.T) {
	d, _ := testDaemon(t)
	for _, n := range []string{"dog", "cat", "ball"} {
		groundNoun(t, d, n)
	}
	say(t, d, "a dog chased a cat")
	say(t, d, "a dog fetched a ball")

	got := say(t, d, "tell me about the dog").Say
	if !strings.Contains(got, "chased") || !strings.Contains(got, "fetched") {
		t.Fatalf("describe should report the dog's activities across relations: %q", got)
	}
	if !strings.Contains(got, "cat") || !strings.Contains(got, "ball") {
		t.Fatalf("describe should name the co-arguments: %q", got)
	}
}

// The conversational probe end to end: a chasing is told and grounded, then
// asked about from both roles, the relation defined, and the activity
// recalled, every answer generated from a meaning structure run through the
// one construction engine.
func TestConversationalProbe(t *testing.T) {
	d, _ := testDaemon(t)
	groundNoun(t, d, "dog")
	groundNoun(t, d, "cat")

	if got := say(t, d, "a dog chased a cat").Say; got != "I see." {
		t.Fatalf("a comprehended statement is acknowledged: %q", got)
	}
	for _, c := range []struct{ ask, want string }{
		{"what did the dog chase", "The dog chased the cat."},
		{"who chased the cat", "The dog chased the cat."},
		{"what did the dog do", "The dog chased the cat."},
	} {
		if got := say(t, d, c.ask).Say; got != c.want {
			t.Fatalf("%q -> %q, want %q", c.ask, got, c.want)
		}
	}
	if got := say(t, d, "what is chase").Say; !strings.HasPrefix(got, "Chase is an action.") {
		t.Fatalf("the relation should define as an action: %q", got)
	}
}

// The same construction engine comprehends Turkish: a verb-final transitive
// clause, the object accusative, is read into a proposition over the seeded
// Turkish constructions, with no separate parser. "köpek kediyi kovaladı"
// binds the dog as agent and the cat as patient and mints the relation from
// the verb's stem, the per-language parse fork folded into the one engine for
// the meaning-carrying path.
func TestComprehendTurkishProposition(t *testing.T) {
	d, s := testDaemon(t)
	kopek := groundNounLang(t, d, "köpek", "tr")
	kedi := groundNounLang(t, d, "kedi", "tr")

	resp := sayLang(t, d, "tr", "köpek kediyi kovaladı")
	moment := resp.Observe.FragmentID
	if moment == 0 {
		t.Fatal("the telling should leave a moment")
	}

	rel, _ := s.ConceptByTemplate("rel:tr:kovala")
	if rel == nil || rel.Category != "relation" {
		t.Fatalf("the Turkish verb should mint a relation from its stem: %+v", rel)
	}
	ag, _ := s.LinksFromRel([]uint64{moment}, memory.RelAgent)
	if !linksTo(ag, kopek.ID) {
		t.Fatalf("köpek should be the agent: %+v", ag)
	}
	pat, _ := s.LinksFromRel([]uint64{moment}, memory.RelPatient)
	if !linksTo(pat, kedi.ID) {
		t.Fatalf("the accusative kedi should be the patient: %+v", pat)
	}
}

// A yes/no question is settled against the graph, not asserted. With the dog's
// chase of the cat grounded, "did the dog chase the cat" affirms it and "did
// the dog chase the mouse" denies it, the denial running do-support's negative.
// And asking is not telling: the mouse question writes no proposition, so the
// dog still chased only the cat afterward.
func TestPolarQuestionAffirmsAndDenies(t *testing.T) {
	d, _ := testDaemon(t)
	for _, n := range []string{"dog", "cat", "mouse"} {
		groundNoun(t, d, n)
	}
	say(t, d, "a dog chased a cat")

	if got := say(t, d, "did the dog chase the cat").Say; got != "Yes, the dog chased the cat." {
		t.Fatalf("a held proposition should affirm: %q", got)
	}
	if got := say(t, d, "did the dog chase the mouse").Say; got != "No, the dog did not chase the mouse." {
		t.Fatalf("an unheld proposition should deny: %q", got)
	}
	if got := say(t, d, "what did the dog chase").Say; got != "The dog chased the cat." {
		t.Fatalf("a polar question must not assert; the dog still chased only the cat: %q", got)
	}
}

// A polar question is about the proposition the asker states, in the direction
// they state it. With the mouse the one that chased the cat, "did the cat chase
// the mouse" asks the reverse, which never happened: the answer is no, the
// learned role history not silently swapping the question into the chase that
// did occur. Word order fixes the agent in a yes/no question.
func TestPolarQuestionRespectsAskedDirection(t *testing.T) {
	d, _ := testDaemon(t)
	groundNoun(t, d, "cat")
	groundNoun(t, d, "mouse")
	say(t, d, "a mouse chased a cat")

	if got := say(t, d, "did the mouse chase the cat").Say; got != "Yes, the mouse chased the cat." {
		t.Fatalf("the proposition as it happened should affirm: %q", got)
	}
	if got := say(t, d, "did the cat chase the mouse").Say; got != "No, the cat did not chase the mouse." {
		t.Fatalf("the reverse proposition should deny, not swap into the chase that happened: %q", got)
	}
}

// The same check answers in Turkish: the trailing particle "mı" marks the polar
// clause, the answer affirms with "evet" or denies with "hayır", and the denial
// conjugates the negative "kovalamadı" the morphology forms by rule, the object
// taking the accusative either way.
func TestPolarQuestionTurkish(t *testing.T) {
	d, _ := testDaemon(t)
	groundNounLang(t, d, "köpek", "tr")
	groundNounLang(t, d, "kedi", "tr")
	groundNounLang(t, d, "fare", "tr")
	sayLang(t, d, "tr", "köpek kediyi kovaladı")

	if got := sayLang(t, d, "tr", "köpek kediyi kovaladı mı").Say; got != "Evet, köpek kediyi kovaladı." {
		t.Fatalf("a held proposition should affirm in Turkish: %q", got)
	}
	if got := sayLang(t, d, "tr", "köpek fareyi kovaladı mı").Say; got != "Hayır, köpek fareyi kovalamadı." {
		t.Fatalf("an unheld proposition should deny in Turkish: %q", got)
	}
}

// A yes/no question is never written as a fact, even when it cannot be
// answered. Asking about an ungrounded referent admits the gap rather than
// falling through to the statement path, which would mint the unknown noun and
// the stripped Turkish "mı" particle as concepts. A non-empty admission proves
// planPolar consumed the utterance as a question.
func TestPolarQuestionNeverAsserts(t *testing.T) {
	d, _ := testDaemon(t)
	groundNounLang(t, d, "köpek", "tr")
	groundNounLang(t, d, "kedi", "tr")
	sayLang(t, d, "tr", "köpek kediyi kovaladı")
	if got := sayLang(t, d, "tr", "köpek tavşanı kovaladı mı").Say; got == "" {
		t.Fatal("an unanswerable polar question should admit the gap, not fall through and assert")
	}
}

// An answer coordinates each distinct co-argument name once. Two concepts a
// split left under one name, or one thing two tongues name alike, must read
// "the dog chased the rabbit", not "the rabbit and the rabbit".
func TestRenderFactsDedupsRepeatedName(t *testing.T) {
	d, _ := testDaemon(t)
	dog := groundNoun(t, d, "dog")
	facts := []focusFact{
		{rel: "chase", focusRole: "agent", coGloss: "rabbit", coID: 9001},
		{rel: "chase", focusRole: "agent", coGloss: "rabbit", coID: 9002},
	}
	line, ok := d.renderFacts("en", d.conceptName(dog.ID, dog.Content, "en"), facts, true, false)
	if !ok {
		t.Fatal("the facts should render")
	}
	if strings.Count(line, "rabbit") != 1 {
		t.Fatalf("a repeated co-argument name should be coordinated once: %q", line)
	}
}

// Minting keys a word on its lemma: an inflected object reinforces the base
// concept's word rather than spawning a shadow node per inflection, so the
// lexicon does not accrete "kediyi" beside "kedi" (which over repetition would
// distil a shadow concept).
func TestMintingReducesAccusativeToLemma(t *testing.T) {
	d, s := testDaemon(t)
	groundNounLang(t, d, "köpek", "tr")
	groundNounLang(t, d, "kedi", "tr")
	sayLang(t, d, "tr", "köpek kediyi kovaladı")
	if w, _ := s.WordNodesByForms([]string{"kediyi"}); len(w) > 0 {
		t.Fatalf("the accusative kediyi should reduce to kedi, not mint its own word node")
	}
	if w, _ := s.WordNodesByForms([]string{"kedi"}); len(w) == 0 {
		t.Fatal("the base kedi should be in the lexicon")
	}
}

// On a relation's first telling the verb's inflection no longer leaves a stray
// word node. Minting runs after comprehension establishes the relation, so
// "chased" keys on the lemma "chase" the proposition just minted, rather than
// storing a surface node before its lemma is a concept.
func TestFirstTellingVerbKeysOnLemma(t *testing.T) {
	d, s := testDaemon(t)
	groundNoun(t, d, "dog")
	groundNoun(t, d, "cat")
	say(t, d, "a dog chased a cat")
	if w, _ := s.WordNodesByForms([]string{"chased"}); len(w) > 0 {
		t.Fatal("the inflected verb should reduce to its lemma on first telling, not mint 'chased'")
	}
	if w, _ := s.WordNodesByForms([]string{"chase"}); len(w) == 0 {
		t.Fatal("the lemma 'chase' should be in the lexicon, naming the relation")
	}
}

// A yes/no question GOLEM cannot answer from observation it answers from
// inference, honestly hedged. The wolf was never seen chasing the cat, but it
// resembles the dog, which was: the resemblance layer proposes the dog, the
// graph confirms the dog chased the cat, and the answer is "probably", a guess
// offered as a guess. An observed proposition still affirms plainly, and an
// unsupported one still denies, so the hedge marks only what is actually a guess.
func TestPolarInfersFromResemblance(t *testing.T) {
	d, s := testDaemon(t)
	dog := groundNoun(t, d, "dog")
	groundNoun(t, d, "cat")
	wolf := groundNoun(t, d, "wolf")
	say(t, d, "a dog chased a cat")
	feat := &memory.Fragment{Kind: memory.KindFeature, Channel: "feature", Content: "predator",
		Template: "predator", Limbic: d.chem.Snapshot(), Time: time.Now()}
	if _, err := s.Encode(feat); err != nil {
		t.Fatal(err)
	}
	s.SetLinkRel(dog.ID, feat.ID, memory.RelFeature, 1.0)
	s.SetLinkRel(wolf.ID, feat.ID, memory.RelFeature, 1.0)

	if got := say(t, d, "did the dog chase the cat").Say; got != "Yes, the dog chased the cat." {
		t.Fatalf("an observed proposition should affirm plainly: %q", got)
	}
	if got := say(t, d, "did the wolf chase the cat").Say; got != "Probably, the wolf chased the cat." {
		t.Fatalf("an inferable proposition should be a hedged guess: %q", got)
	}
	if got := say(t, d, "did the cat chase the wolf").Say; got != "No, the cat did not chase the wolf." {
		t.Fatalf("an unsupported proposition should deny: %q", got)
	}
}

// A guess GOLEM has drawn and is holding lets it draw a further guess that
// stands on it: the chain builds through working memory, a hop further from
// observation and a notch less sure. The dog chased the cat; the wolf resembles
// the dog and the coyote resembles the wolf, but the coyote resembles the dog
// not at all, so the coyote can reach the cat only through the held belief about
// the wolf. Cold, before that belief is in mind, the chain does not form.
func TestPolarChainsThroughHeldBelief(t *testing.T) {
	d, s := testDaemon(t)
	dog := groundNoun(t, d, "dog")
	groundNoun(t, d, "cat")
	wolf := groundNoun(t, d, "wolf")
	coyote := groundNoun(t, d, "coyote")
	say(t, d, "a dog chased a cat")

	feat := func(name string) *memory.Fragment {
		f := &memory.Fragment{Kind: memory.KindFeature, Channel: "feature", Content: name,
			Template: name, Limbic: d.chem.Snapshot(), Time: time.Now()}
		if _, err := s.Encode(f); err != nil {
			t.Fatal(err)
		}
		return f
	}
	a, b := feat("pads"), feat("howls")
	// The dog and the wolf share one feature, the wolf and the coyote another,
	// the dog and the coyote none, so the only path from coyote to dog is the wolf.
	s.SetLinkRel(dog.ID, a.ID, memory.RelFeature, 1.0)
	s.SetLinkRel(wolf.ID, a.ID, memory.RelFeature, 1.0)
	s.SetLinkRel(wolf.ID, b.ID, memory.RelFeature, 1.0)
	s.SetLinkRel(coyote.ID, b.ID, memory.RelFeature, 1.0)

	// Cold: the wolf is not yet in mind, so the coyote cannot reach the dog.
	if got := say(t, d, "did the coyote chase the cat").Say; got != "No, the coyote did not chase the cat." {
		t.Fatalf("with no held intermediate the chain should not form: %q", got)
	}
	// One hop from the observed fact, and the conclusion is held.
	if got := say(t, d, "did the wolf chase the cat").Say; got != "Probably, the wolf chased the cat." {
		t.Fatalf("a one-hop guess should be hedged: %q", got)
	}
	// Now the coyote reaches the cat through the held belief about the wolf.
	if got := say(t, d, "did the coyote chase the cat").Say; got != "Probably, the coyote chased the cat." {
		t.Fatalf("a two-hop chain through the held belief should form: %q", got)
	}
}

// GOLEM can entertain a proposition it does not believe, reason under it, and
// keep it out of memory. The dog chased the cat; the coyote resembles the wolf
// but reaches the cat through nothing observed. Supposing the wolf chased the
// cat opens a route: under the supposition the coyote probably chased it too. The
// supposition never becomes a belief, and releasing it closes the route again.
func TestHypothesisEntertainsWithoutAsserting(t *testing.T) {
	d, s := testDaemon(t)
	dog := groundNoun(t, d, "dog")
	groundNoun(t, d, "cat")
	wolf := groundNoun(t, d, "wolf")
	coyote := groundNoun(t, d, "coyote")
	say(t, d, "a dog chased a cat")
	_ = dog
	feat := func(name string) *memory.Fragment {
		f := &memory.Fragment{Kind: memory.KindFeature, Channel: "feature", Content: name,
			Template: name, Limbic: d.chem.Snapshot(), Time: time.Now()}
		if _, err := s.Encode(f); err != nil {
			t.Fatal(err)
		}
		return f
	}
	// The coyote resembles the wolf alone; the wolf is observed chasing nothing,
	// so without a supposition the coyote has no route to the cat.
	b := feat("howls")
	s.SetLinkRel(wolf.ID, b.ID, memory.RelFeature, 1.0)
	s.SetLinkRel(coyote.ID, b.ID, memory.RelFeature, 1.0)

	if got := say(t, d, "did the coyote chase the cat").Say; got != "No, the coyote did not chase the cat." {
		t.Fatalf("cold, the coyote cannot reach the cat: %q", got)
	}
	if got := say(t, d, "suppose the wolf chased the cat").Say; got != "Supposing the wolf chased the cat." {
		t.Fatalf("a supposition should be echoed back, not asserted: %q", got)
	}
	if got := say(t, d, "did the coyote chase the cat").Say; got != "If so, probably the coyote chased the cat." {
		t.Fatalf("under the supposition the answer should be marked contingent on it: %q", got)
	}
	// The supposition and anything contingent on it are entertained, not believed.
	for _, p := range d.held.props {
		if p.status == heldInferred {
			t.Fatalf("a supposition must not become a belief: %+v", p)
		}
	}
	if got := say(t, d, "never mind").Say; got != "All right." {
		t.Fatalf("a release should be acknowledged: %q", got)
	}
	if got := say(t, d, "did the coyote chase the cat").Say; got != "As I said, no, the coyote did not chase the cat." {
		t.Fatalf("after release the supposition should no longer bear, and GOLEM stands by the denial it gave: %q", got)
	}
}

// GOLEM volunteers a conclusion it has drawn about the topic in mind, once, and
// then has nothing more to add. The wolf resembles the dog, which chased the cat,
// so invited to go on about the wolf, GOLEM offers that the wolf probably chased
// the cat too, on its own account, and does not repeat it.
func TestVolunteersHeldConclusion(t *testing.T) {
	d, s := testDaemon(t)
	dog := groundNoun(t, d, "dog")
	groundNoun(t, d, "cat")
	wolf := groundNoun(t, d, "wolf")
	say(t, d, "a dog chased a cat")
	feat := &memory.Fragment{Kind: memory.KindFeature, Channel: "feature", Content: "predator",
		Template: "predator", Limbic: d.chem.Snapshot(), Time: time.Now()}
	if _, err := s.Encode(feat); err != nil {
		t.Fatal(err)
	}
	s.SetLinkRel(dog.ID, feat.ID, memory.RelFeature, 1.0)
	s.SetLinkRel(wolf.ID, feat.ID, memory.RelFeature, 1.0)

	d.topic = wolf.ID
	got, ok := d.volunteerRemark("en")
	if !ok || got != "Probably, the wolf chased the cat." {
		t.Fatalf("should volunteer a drawn conclusion about the topic: %q (%v)", got, ok)
	}
	if line, ok := d.volunteerRemark("en"); ok {
		t.Fatalf("a conclusion already voiced should not be volunteered again: %q", line)
	}

	// The open invitation routes here too: a fresh mind draws the conclusion and
	// offers it.
	d.held.clear()
	d.topic = wolf.ID
	if got := say(t, d, "what else").Say; got != "Probably, the wolf chased the cat." {
		t.Fatalf("an invitation to go on should volunteer the conclusion: %q", got)
	}
}

// The idle draw leaves orderings and placements where every fact appendix
// leaves them: a neighbour's bigger-than does not transfer by likeness, and a
// volunteered ordering would render its relation as a verb ("the wolf biggered
// the mouse"). The plain action still transfers and is offered.
func TestIdleDrawSkipsComparative(t *testing.T) {
	d, s := testDaemon(t)
	dog := groundNoun(t, d, "dog")
	groundNoun(t, d, "cat")
	mouse := groundNoun(t, d, "mouse")
	wolf := groundNoun(t, d, "wolf")
	say(t, d, "a dog chased a cat")
	say(t, d, "a dog is bigger than a mouse")
	say(t, d, "a mouse is smaller than a dog") // both poles, so bigger reads as a gradable pair
	feat := &memory.Fragment{Kind: memory.KindFeature, Channel: "feature", Content: "predator",
		Template: "predator", Limbic: d.chem.Snapshot(), Time: time.Now()}
	if _, err := s.Encode(feat); err != nil {
		t.Fatal(err)
	}
	s.SetLinkRel(dog.ID, feat.ID, memory.RelFeature, 1.0)
	s.SetLinkRel(wolf.ID, feat.ID, memory.RelFeature, 1.0)

	d.drawInferenceAbout(wolf)
	if _, ok := d.held.holds("bigger", wolf.ID, mouse.ID, heldInferred); ok {
		t.Fatal("a neighbour's ordering must not transfer as an idle guess")
	}
	d.topic = wolf.ID
	if got, ok := d.volunteerRemark("en"); !ok || got != "Probably, the wolf chased the cat." {
		t.Fatalf("the plain action should still transfer and be offered: %q (%v)", got, ok)
	}
}

// Reasoning to itself, GOLEM chains a conclusion onto a conclusion it has
// already drawn. The dog chased the cat; the wolf resembles the dog and the
// coyote the wolf, but the coyote resembles the dog not at all. Drawing about the
// wolf concludes it chased the cat; drawing about the coyote then reaches the cat
// only through that held conclusion, a hop further from observation, and the
// chained guess can be volunteered.
func TestThinkLoopChainsInference(t *testing.T) {
	d, s := testDaemon(t)
	dog := groundNoun(t, d, "dog")
	cat := groundNoun(t, d, "cat")
	wolf := groundNoun(t, d, "wolf")
	coyote := groundNoun(t, d, "coyote")
	say(t, d, "a dog chased a cat")
	feat := func(name string) *memory.Fragment {
		f := &memory.Fragment{Kind: memory.KindFeature, Channel: "feature", Content: name,
			Template: name, Limbic: d.chem.Snapshot(), Time: time.Now()}
		if _, err := s.Encode(f); err != nil {
			t.Fatal(err)
		}
		return f
	}
	a, b := feat("pads"), feat("howls")
	s.SetLinkRel(dog.ID, a.ID, memory.RelFeature, 1.0)
	s.SetLinkRel(wolf.ID, a.ID, memory.RelFeature, 1.0)
	s.SetLinkRel(wolf.ID, b.ID, memory.RelFeature, 1.0)
	s.SetLinkRel(coyote.ID, b.ID, memory.RelFeature, 1.0)

	d.drawInferenceAbout(wolf)
	if _, ok := d.held.holds("chase", wolf.ID, cat.ID, heldInferred); !ok {
		t.Fatal("drawing about the wolf should conclude it chased the cat")
	}
	d.drawInferenceAbout(coyote)
	p := d.held.findID("chase", coyote.ID, cat.ID, heldInferred)
	if p == nil {
		t.Fatal("drawing about the coyote should reach the cat through the held wolf belief")
	}
	if p.depth != 1 {
		t.Fatalf("the coyote's conclusion should stand one hop past the wolf's: depth %d", p.depth)
	}
	d.topic = coyote.ID
	if got, ok := d.volunteerRemark("en"); !ok || got != "Probably, the coyote chased the cat." {
		t.Fatalf("the chained conclusion should be offerable: %q (%v)", got, ok)
	}
}

// Initiative offers a conclusion GOLEM has not reached, not a hedged restatement
// of what it has seen. A fact the topic already observes is skipped when drawing,
// so "what else" volunteers something new rather than "probably X" for an X it
// could affirm outright.
func TestDrawInferenceSkipsObservedFact(t *testing.T) {
	d, s := testDaemon(t)
	cat := groundNoun(t, d, "cat")
	mouse := groundNoun(t, d, "mouse")
	rabbit := groundNoun(t, d, "rabbit")
	dog := groundNoun(t, d, "dog")
	say(t, d, "a cat chased a mouse")  // the cat's own observed fact
	say(t, d, "a dog chased a mouse")  // a neighbour that did the same
	say(t, d, "a dog chased a rabbit") // and one the cat has not
	feat := &memory.Fragment{Kind: memory.KindFeature, Channel: "feature", Content: "predator",
		Template: "predator", Limbic: d.chem.Snapshot(), Time: time.Now()}
	if _, err := s.Encode(feat); err != nil {
		t.Fatal(err)
	}
	s.SetLinkRel(cat.ID, feat.ID, memory.RelFeature, 1.0)
	s.SetLinkRel(dog.ID, feat.ID, memory.RelFeature, 1.0)

	d.drawInferenceAbout(cat)
	if _, ok := d.held.holds("chase", cat.ID, mouse.ID, heldInferred); ok {
		t.Fatal("an observed fact must not be re-deposited as a guess to volunteer")
	}
	if _, ok := d.held.holds("chase", cat.ID, rabbit.ID, heldInferred); !ok {
		t.Fatal("a genuinely new conclusion should still be drawn")
	}
}

// A supposition reasons across every question shape, not only yes/no: a held
// hypothetical in which the asked concept stands in the asked role answers a
// wh-question behind the conditional lead, so it is never taken for the observed.
// Released, it no longer answers.
func TestRoleQuestionReasonsUnderSupposition(t *testing.T) {
	d, _ := testDaemon(t)
	cat := groundNoun(t, d, "cat")
	groundNoun(t, d, "mouse")
	rabbit := groundNoun(t, d, "rabbit")
	say(t, d, "a cat chased a mouse") // establishes the chase relation
	d.held.add(heldProp{rel: "chase", agent: cat.ID, patient: rabbit.ID,
		agentGloss: cat.Content, patientGloss: rabbit.Content, status: heldHypothetical, conf: 1})

	got, ok := d.heldRoleGuess(grammar.Frame{Lang: "en"}, cat, "agent", "chase", true, false)
	if !ok || !strings.HasPrefix(got, "If so") || !strings.Contains(got, "rabbit") {
		t.Fatalf("a wh-question should reason under a supposition with the conditional lead: %q (%v)", got, ok)
	}

	d.held.clearStatus(heldHypothetical)
	if _, ok := d.heldRoleGuess(grammar.Frame{Lang: "en"}, cat, "agent", "chase", true, false); ok {
		t.Fatal("a released supposition should no longer answer")
	}
}

// A conclusion GOLEM keeps drawing earns a place in long-term memory: it
// survives the night that clears working memory, returns on waking, and is still
// read as a belief, never as something seen. A conclusion drawn once does not
// earn it.
func TestEarnedBeliefSurvivesSleep(t *testing.T) {
	d, s := testDaemon(t)
	dog := groundNoun(t, d, "dog")
	groundNoun(t, d, "cat")
	wolf := groundNoun(t, d, "wolf")
	say(t, d, "a dog chased a cat")
	feat := &memory.Fragment{Kind: memory.KindFeature, Channel: "feature", Content: "predator",
		Template: "predator", Limbic: d.chem.Snapshot(), Time: time.Now()}
	if _, err := s.Encode(feat); err != nil {
		t.Fatal(err)
	}
	s.SetLinkRel(dog.ID, feat.ID, memory.RelFeature, 1.0)
	s.SetLinkRel(wolf.ID, feat.ID, memory.RelFeature, 1.0)

	// Concluded once, it has not earned a place.
	say(t, d, "did the wolf chase the cat")
	if got := d.held.earned(promoteReinforce); len(got) != 0 {
		t.Fatalf("a once-drawn conclusion should not be earned: %d", len(got))
	}

	// Re-drawn, it earns one. The night keeps it.
	say(t, d, "did the wolf chase the cat")
	say(t, d, "did the wolf chase the cat")
	d.promoteBeliefs()
	bs, err := s.Beliefs()
	if err != nil || len(bs) != 1 || bs[0].Agent != wolf.ID || bs[0].Rel != "chase" {
		t.Fatalf("a re-drawn conclusion should be kept in long-term memory: %+v (%v)", bs, err)
	}

	// Waking from the saved brain, the belief is in mind again and still a guess,
	// never mistaken for something observed. A new morning is a new exchange, so
	// the night's claim log is cleared the way sleep clears it.
	d.held.clear()
	d.claims = nil
	d.loadBeliefs()
	if got := say(t, d, "did the wolf chase the cat").Say; got != "Probably, the wolf chased the cat." {
		t.Fatalf("a promoted belief should survive and still read as a guess: %q", got)
	}
}

// When a guess is finally seen, it retires: the inferred belief leaves working
// memory and long-term memory, and the observed fact answers plainly in its
// place, so observation supersedes inference rather than standing beside it.
func TestObservationRetiresBelief(t *testing.T) {
	d, s := testDaemon(t)
	dog := groundNoun(t, d, "dog")
	groundNoun(t, d, "cat")
	wolf := groundNoun(t, d, "wolf")
	say(t, d, "a dog chased a cat")
	feat := &memory.Fragment{Kind: memory.KindFeature, Channel: "feature", Content: "predator",
		Template: "predator", Limbic: d.chem.Snapshot(), Time: time.Now()}
	if _, err := s.Encode(feat); err != nil {
		t.Fatal(err)
	}
	s.SetLinkRel(dog.ID, feat.ID, memory.RelFeature, 1.0)
	s.SetLinkRel(wolf.ID, feat.ID, memory.RelFeature, 1.0)

	say(t, d, "did the wolf chase the cat")
	say(t, d, "did the wolf chase the cat")
	say(t, d, "did the wolf chase the cat")
	d.promoteBeliefs()
	if bs, _ := s.Beliefs(); len(bs) != 1 {
		t.Fatalf("the belief should be kept before it is seen: %d", len(bs))
	}

	// Now it is actually told: the guess retires, the fact stands.
	say(t, d, "a wolf chased a cat")
	if bs, _ := s.Beliefs(); len(bs) != 0 {
		t.Fatalf("an observed proposition should retire the guess: %d kept", len(bs))
	}
	if got := say(t, d, "did the wolf chase the cat").Say; got != "Yes, the wolf chased the cat." {
		t.Fatalf("the observed fact should now answer plainly: %q", got)
	}
}

// GOLEM keeps a record of its own claims: a re-asked question is noted as already
// answered, and a claim it affirmed is stood by rather than quietly contradicted
// within the exchange, the honesty rail carried into the conversation.
func TestPolarRecordsAndStandsByClaim(t *testing.T) {
	d, s := testDaemon(t)
	dog := groundNoun(t, d, "dog")
	cat := groundNoun(t, d, "cat")
	wolf := groundNoun(t, d, "wolf")
	say(t, d, "a dog chased a cat")
	feat := &memory.Fragment{Kind: memory.KindFeature, Channel: "feature", Content: "predator",
		Template: "predator", Limbic: d.chem.Snapshot(), Time: time.Now()}
	if _, err := s.Encode(feat); err != nil {
		t.Fatal(err)
	}
	s.SetLinkRel(dog.ID, feat.ID, memory.RelFeature, 1.0)
	s.SetLinkRel(wolf.ID, feat.ID, memory.RelFeature, 1.0)

	if got := say(t, d, "did the wolf chase the cat").Say; got != "Probably, the wolf chased the cat." {
		t.Fatalf("the first answer is a fresh hedged guess: %q", got)
	}
	if got := say(t, d, "did the wolf chase the cat").Say; got != "As I said, probably, the wolf chased the cat." {
		t.Fatalf("a re-asked claim should be noted as already said: %q", got)
	}
	// Were the brain to lose the inference and a denial be computed, GOLEM stands
	// by the claim it gave rather than flip on it within the exchange.
	rel := d.resolveRelation("chase")
	if rel == nil {
		t.Fatal("the chase relation should resolve")
	}
	if got := d.answerPolar("en", wolf, rel, cat, true, beliefUnknown, false, 0, 0); got != "As I said, probably, the wolf chased the cat." {
		t.Fatalf("GOLEM should hold its prior affirmation, not contradict it: %q", got)
	}
}

// A FIRM affirmation is stood by only as a hedge once the live grounding no longer
// supports it: a claim held on memory of having said it is not a present fact, so the
// firm "yes" softens to "probably" rather than being restored firm on memory alone.
func TestFirmClaimStoodByAsHedge(t *testing.T) {
	d, _ := testDaemon(t)
	dog := groundNoun(t, d, "dog")
	cat := groundNoun(t, d, "cat")
	say(t, d, "a dog chased a cat")
	if got := say(t, d, "did the dog chase the cat").Say; got != "Yes, the dog chased the cat." {
		t.Fatalf("an observed fact affirms firmly first: %q", got)
	}
	rel := d.resolveRelation("chase")
	if rel == nil {
		t.Fatal("the chase relation should resolve")
	}
	got := strings.ToLower(d.answerPolar("en", dog, rel, cat, true, beliefUnknown, false, 0, 0))
	if !strings.Contains(got, "probably") {
		t.Fatalf("a firm claim with no live grounding should soften to a hedge: %q", got)
	}
	if strings.Contains(got, "yes,") {
		t.Fatalf("a firm claim must not be restored to a firm yes on memory alone: %q", got)
	}
}

// A wh-object question GOLEM holds no observed fact for is not answered by borrowing
// an unrelated neighbour's co-argument: that would fabricate the very object the
// question asks for and speak it under the asked concept's name. The wolf chased
// nothing and belongs to no chasing category, so "what did the wolf chase" admits
// the gap rather than lending it the dog's cat through feature resemblance; likewise
// "who chased the mouse". An observed role still answers plainly. A grounded tie does
// answer a wh-object gap, but only through category inheritance, tested in
// TestDefeasibleCategoryInheritance.
func TestRoleQuestionDoesNotFabricateFromResemblance(t *testing.T) {
	d, s := testDaemon(t)
	dog := groundNoun(t, d, "dog")
	cat := groundNoun(t, d, "cat")
	wolf := groundNoun(t, d, "wolf")
	mouse := groundNoun(t, d, "mouse")
	say(t, d, "a dog chased a cat")
	share := func(a, b *memory.Fragment, name string) {
		feat := &memory.Fragment{Kind: memory.KindFeature, Channel: "feature", Content: name,
			Template: name, Limbic: d.chem.Snapshot(), Time: time.Now()}
		if _, err := s.Encode(feat); err != nil {
			t.Fatal(err)
		}
		s.SetLinkRel(a.ID, feat.ID, memory.RelFeature, 1.0)
		s.SetLinkRel(b.ID, feat.ID, memory.RelFeature, 1.0)
	}
	share(dog, wolf, "predator") // the wolf resembles the dog, the chaser
	share(cat, mouse, "prey")    // the mouse resembles the cat, the chased

	if got := say(t, d, "what did the dog chase").Say; got != "The dog chased the cat." {
		t.Fatalf("an observed role should answer plainly: %q", got)
	}
	// The wolf has no chase fact and no category that chased: resemblance to the dog
	// must not lend the dog's cat under the wolf's name.
	if got := say(t, d, "what did the wolf chase").Say; strings.Contains(got, "cat") || strings.HasPrefix(got, "Probably") {
		t.Fatalf("an ungrounded subject should admit the gap, not fabricate from resemblance: %q", got)
	}
	if got := say(t, d, "who chased the mouse").Say; strings.Contains(got, "dog") || strings.HasPrefix(got, "Probably") {
		t.Fatalf("an ungrounded patient should admit the gap, not fabricate from resemblance: %q", got)
	}
}

// Two animals alike enough to resemble each other strongly keep the roles word order
// gave them: "a wolf chased a rabbit" stays the wolf chasing, not the reverse, even
// though the wolf resembles the rabbit (a known patient) and the rabbit resembles a
// known agent. Role-fit reads feature kinship, which two look-alikes give no reliable
// signal, so the order that named them is authoritative.
func TestRoleFitDoesNotSwapLookAlikes(t *testing.T) {
	d, s := testDaemon(t)
	groundNoun(t, d, "dog")
	groundNoun(t, d, "cat")
	mouse := groundNoun(t, d, "mouse")
	wolf := groundNoun(t, d, "wolf")
	rabbit := groundNoun(t, d, "rabbit")
	share := func(a, b *memory.Fragment, name string) {
		feat := &memory.Fragment{Kind: memory.KindFeature, Channel: "feature", Content: name,
			Template: name, Limbic: d.chem.Snapshot(), Time: time.Now()}
		if _, err := s.Encode(feat); err != nil {
			t.Fatal(err)
		}
		s.SetLinkRel(a.ID, feat.ID, memory.RelFeature, 1.0)
		s.SetLinkRel(b.ID, feat.ID, memory.RelFeature, 1.0)
	}
	// Role history that would bias a swap: the mouse chases, the rabbit is chased.
	say(t, d, "a dog chased a cat")
	say(t, d, "a mouse chased a cat")
	say(t, d, "a dog chased a rabbit")
	// The wolf and the rabbit look alike; the rabbit also shares a trait with the
	// mouse, a known chaser, so role-fit would read the rabbit as the better agent.
	share(wolf, rabbit, "gray")
	share(wolf, rabbit, "furry")
	share(wolf, rabbit, "small")
	share(rabbit, mouse, "quick")

	m := grammar.Meaning{Roles: map[string]grammar.Term{
		"agent": {Concept: wolf}, "patient": {Concept: rabbit}}}
	if !d.argsIndistinctForFit(m) {
		t.Fatal("the wolf and the rabbit should be too alike for role-fit to separate them")
	}

	say(t, d, "a wolf chased a rabbit")
	chasedByWolf := false
	for _, f := range d.gatherFacts(wolf, "agent", "chase") {
		if f.coID == rabbit.ID {
			chasedByWolf = true
		}
	}
	if !chasedByWolf {
		t.Fatal("the wolf should be stored as the chaser of the rabbit, word order authoritative")
	}
	for _, f := range d.gatherFacts(rabbit, "agent", "chase") {
		if f.coID == wolf.ID {
			t.Fatal("the rabbit must not be stored as the chaser of the wolf (no role swap)")
		}
	}
}

// A "tell me about X" GOLEM has observed nothing about it answers with a hedged
// guess at what it does, from the thing it most resembles. The wolf, resembling
// the dog that chased the cat, is described "A wolf is a thing. Probably, it
// chased the cat." A focus with observed facts of its own is described from
// those, never a neighbour's.
func TestDescribeInfersProperty(t *testing.T) {
	d, s := testDaemon(t)
	dog := groundNoun(t, d, "dog")
	groundNoun(t, d, "cat")
	wolf := groundNoun(t, d, "wolf")
	say(t, d, "a dog chased a cat")
	feat := &memory.Fragment{Kind: memory.KindFeature, Channel: "feature", Content: "predator",
		Template: "predator", Limbic: d.chem.Snapshot(), Time: time.Now()}
	if _, err := s.Encode(feat); err != nil {
		t.Fatal(err)
	}
	s.SetLinkRel(dog.ID, feat.ID, memory.RelFeature, 1.0)
	s.SetLinkRel(wolf.ID, feat.ID, memory.RelFeature, 1.0)

	if got := say(t, d, "tell me about a wolf").Say; !strings.Contains(got, "Probably, it chased the cat.") {
		t.Fatalf("an unobserved focus should guess a property from a neighbour: %q", got)
	}
	if got := say(t, d, "tell me about a dog").Say; strings.Contains(got, "Probably") {
		t.Fatalf("an observed focus should be described from its own facts: %q", got)
	}
}

// A description elaborates the focus across all its grounded relations, where a focused
// question stops at the terse cap. The dog taught four activities, "tell me about the dog"
// names all four; "what did the dog do" keeps the shorter cap of a focused answer.
func TestDescribeElaboratesBeyondFocusedCap(t *testing.T) {
	d, _ := testDaemon(t)
	for _, n := range []string{"dog", "cat", "ball", "bird", "bone"} {
		groundNoun(t, d, n)
	}
	say(t, d, "a dog chased a cat")
	say(t, d, "a dog fetched a ball")
	say(t, d, "a dog watched a bird")
	say(t, d, "a dog ate a bone")

	verbs := []string{"chased", "fetched", "watched", "ate"}
	count := func(s string) int {
		n := 0
		for _, v := range verbs {
			if strings.Contains(s, v) {
				n++
			}
		}
		return n
	}
	if got := say(t, d, "tell me about the dog").Say; count(got) != len(verbs) {
		t.Fatalf("a description should elaborate across all four relations: %q", got)
	}
	if got := say(t, d, "what did the dog do").Say; count(got) > factClauseCap {
		t.Fatalf("a focused question should keep the terse cap of %d: %q", factClauseCap, got)
	}
}

// salientFacts orders by weight and stops where the tail thins: an exchange-established fact
// is kept whatever its weight, the rest come strongest first, and one fainter than a fraction
// of the strongest is dropped. A ceiling backstops a hub of equally strong facts.
func TestSalientFactsOrdersAndStops(t *testing.T) {
	facts := []focusFact{
		{rel: "chase", focusRole: "agent", coID: 1, weight: 0.9},
		{rel: "watch", focusRole: "agent", coID: 2, weight: 0.5},
		{rel: "want", focusRole: "agent", coID: 3, weight: 0.05}, // below 0.9*0.15, the thinned tail
		{rel: "love", focusRole: "agent", coID: 4, weight: 0.0, fromThread: true},
	}
	got := salientFacts(facts, 8)
	if len(got) != 3 {
		t.Fatalf("the faint want should drop and the rest stay, got %d: %+v", len(got), got)
	}
	if got[0].rel != "love" {
		t.Errorf("an exchange-established fact should lead, kept whatever its weight: %+v", got)
	}
	if got[1].rel != "chase" || got[2].rel != "watch" {
		t.Errorf("the graph facts should come strongest first: %+v", got)
	}
	for _, f := range got {
		if f.rel == "want" {
			t.Errorf("the thinned tail should be dropped: %+v", got)
		}
	}
	// The ceiling backstops a run of equally strong facts.
	many := []focusFact{{rel: "a", coID: 1, weight: 1}, {rel: "b", coID: 2, weight: 1}, {rel: "c", coID: 3, weight: 1}}
	if got := salientFacts(many, 2); len(got) != 2 {
		t.Errorf("the ceiling should cap a hub of equally strong facts: %d", len(got))
	}
}

// A description speaks every grounded relation by its kind, a gradable one as "X is bigger
// than Y", so a comparison is part of a description; the activity question keeps to actions,
// so the same comparison is left out of "what did X do".
func TestDescribeSpeaksComparisonActivityKeepsToActions(t *testing.T) {
	d, _ := testDaemon(t)
	for _, n := range []string{"lion", "mouse", "deer"} {
		groundNoun(t, d, n)
	}
	say(t, d, "a lion chased a deer")
	say(t, d, "a lion is bigger than a mouse")
	say(t, d, "a mouse is smaller than a lion") // the converse pole, so bigger is known gradable

	desc := say(t, d, "tell me about the lion").Say
	if !strings.Contains(desc, "chased") || !strings.Contains(desc, "bigger than") {
		t.Fatalf("a description should speak both the action and the comparison: %q", desc)
	}
	act := say(t, d, "what did the lion do").Say
	if !strings.Contains(act, "chased") {
		t.Fatalf("the activity should report the action: %q", act)
	}
	if strings.Contains(act, "bigger") {
		t.Fatalf("the activity should keep to actions, not the comparison state: %q", act)
	}
}

// planDescription groups a description's facts by aspect so like is spoken with like, the
// strongest-grounded aspect first, instead of letting kinds interleave by raw salience. The
// ordering is over the relation's grounded kind, not any word, so it holds in any tongue.
func TestPlanDescriptionGroupsByAspect(t *testing.T) {
	d, _ := testDaemon(t)
	for _, n := range []string{"dog", "cat", "lion", "mouse"} {
		groundNoun(t, d, n)
	}
	say(t, d, "a dog chased a cat")            // chase: a plain action
	say(t, d, "a lion is bigger than a mouse") // bigger and smaller, a known gradable pair
	say(t, d, "a mouse is smaller than a lion")

	// Facts that interleave by weight, so only grouping can cluster them.
	facts := []focusFact{
		{rel: "chase", focusRole: "agent", coID: 1, weight: 0.9},
		{rel: "bigger", focusRole: "agent", coID: 2, weight: 0.8},
		{rel: "chase", focusRole: "agent", coID: 3, weight: 0.7},
		{rel: "bigger", focusRole: "agent", coID: 4, weight: 0.6},
	}
	got := d.planDescription(facts)
	seen := map[string]bool{}
	prev := ""
	for _, f := range got {
		a := d.aspectKey(f)
		if a != prev && seen[a] {
			t.Fatalf("aspect %q reappeared, so the facts were not grouped: %v", a, got)
		}
		seen[a] = true
		prev = a
	}
	// The stronger-grounded aspect (chase, peak 0.9) leads the weaker (bigger, peak 0.8).
	if d.aspectKey(got[0]) != "action:agent" {
		t.Errorf("the strongest aspect should lead: %v", got)
	}
}

// Opposite poles of one gradable scale read as a contrast: a thing bigger than one thing and
// smaller than another is described "bigger than X, but smaller than Y". The contrast is the
// meaning (the opposite pole); the connective is the tongue's seeded word.
func TestDescriptionContrastsOppositePoles(t *testing.T) {
	d, _ := testDaemon(t)
	for _, n := range []string{"hare", "mouse", "horse"} {
		groundNoun(t, d, n)
	}
	say(t, d, "a hare is bigger than a mouse")
	say(t, d, "a mouse is smaller than a hare") // pairs bigger and smaller as one scale
	say(t, d, "a hare is smaller than a horse") // the hare is the lesser here, same scale

	got := say(t, d, "tell me about the hare").Say
	if !strings.Contains(got, "bigger than") || !strings.Contains(got, "smaller than") {
		t.Fatalf("the description should speak both poles: %q", got)
	}
	if !strings.Contains(strings.ToLower(got), "but") {
		t.Fatalf("the opposite poles of one scale should be joined by a contrast: %q", got)
	}
}

// A description of a thing that did several things marks the further actions as additions:
// "It chased the mouse. It also watched the horse." The focus is the constant subject, so a
// second action is something it also did. The additive relation is read from meaning (the same
// subject across actions); the word is the tongue's seeded data, spoken once rather than on
// every action.
func TestDescriptionAddsAcrossActions(t *testing.T) {
	d, _ := testDaemon(t)
	for _, n := range []string{"hare", "mouse", "horse"} {
		groundNoun(t, d, n)
	}
	say(t, d, "a hare chased a mouse")
	say(t, d, "a hare watched a horse")

	got := say(t, d, "tell me about the hare").Say
	if c := strings.Count(strings.ToLower(got), "also"); c != 1 {
		t.Fatalf("a second action by the same subject should be marked as an addition once, not %d: %q", c, got)
	}
	// English places the additive after the subject ("it also watched"), not at the front.
	if !strings.Contains(strings.ToLower(got), "it also ") {
		t.Fatalf("the additive should sit after the subject in English: %q", got)
	}

	// A lone action stands on its own, so nothing is added to it.
	d2, _ := testDaemon(t)
	for _, n := range []string{"hare", "mouse"} {
		groundNoun(t, d2, n)
	}
	say(t, d2, "a hare chased a mouse")
	if got := say(t, d2, "tell me about the hare").Say; strings.Contains(strings.ToLower(got), "also") {
		t.Fatalf("a single action should not be marked as an addition: %q", got)
	}
}

// Comparisons that share a standard speak as one predicate, the standard named once ("the mouse
// is lighter and slower than the cat"), while opposite poles of another scale still contrast. The
// grouping is by the standard (meaning); the coordinator and contrast words are the tongue's data.
func TestDescriptionCoordinatesSameStandardComparisons(t *testing.T) {
	d, _ := testDaemon(t)
	for _, n := range []string{"mouse", "cat", "snail", "lion"} {
		groundNoun(t, d, n)
	}
	// Two scales sharing the cat as standard: the mouse is lighter and slower than the cat.
	say(t, d, "a mouse is lighter than a cat")
	say(t, d, "a cat is heavier than a mouse") // pairs lighter and heavier as one scale
	say(t, d, "a mouse is slower than a cat")
	say(t, d, "a cat is faster than a mouse") // pairs slower and faster as one scale
	// A size scale with opposite poles against two standards, so the contrast still coexists.
	say(t, d, "a mouse is bigger than a snail")
	say(t, d, "a snail is smaller than a mouse") // pairs bigger and smaller
	say(t, d, "a mouse is smaller than a lion")

	got := say(t, d, "tell me about the mouse").Say
	coord := strings.Contains(got, "lighter and slower than the cat") ||
		strings.Contains(got, "slower and lighter than the cat")
	if !coord {
		t.Fatalf("same-standard comparisons should coordinate into one predicate: %q", got)
	}
	if c := strings.Count(got, "than the cat"); c != 1 {
		t.Fatalf("the shared standard should be named once, not %d times: %q", c, got)
	}
	if !strings.Contains(strings.ToLower(got), "but") {
		t.Fatalf("opposite poles of one scale should still read as a contrast: %q", got)
	}
}

// Several actions over the same subject and object coordinate their verbs into one clause ("it
// chased and watched the fox") instead of a clause each, so the shared pair is named once. The
// grouping is by the role and the co-argument (meaning); the coordinator is the tongue's data.
func TestDescriptionCoordinatesSameObjectActions(t *testing.T) {
	d, _ := testDaemon(t)
	for _, n := range []string{"hare", "fox"} {
		groundNoun(t, d, n)
	}
	say(t, d, "a hare chased a fox")
	say(t, d, "a hare watched a fox")

	got := say(t, d, "tell me about the hare").Say
	if c := strings.Count(got, "fox"); c != 1 {
		t.Fatalf("the shared object should be named once when the verbs coordinate, not %d: %q", c, got)
	}
	if !strings.Contains(got, "chased") || !strings.Contains(got, "watched") {
		t.Fatalf("both verbs should be spoken in the coordinated clause: %q", got)
	}
	if !strings.Contains(got, " and ") {
		t.Fatalf("the verbs over one object should join with the coordinator: %q", got)
	}
}

// The same agent acting on the focus several times coordinates likewise from the focus's side
// ("the fox chased and watched it"), the agent named once.
func TestDescriptionCoordinatesSameAgentActionsOnFocus(t *testing.T) {
	d, _ := testDaemon(t)
	for _, n := range []string{"hare", "fox"} {
		groundNoun(t, d, n)
	}
	say(t, d, "a fox chased a hare")
	say(t, d, "a fox watched a hare")

	got := say(t, d, "tell me about the hare").Say
	if c := strings.Count(got, "fox"); c != 1 {
		t.Fatalf("the shared agent should be named once when the verbs coordinate, not %d: %q", c, got)
	}
	if !strings.Contains(got, "chased") || !strings.Contains(got, "watched") {
		t.Fatalf("both verbs should be spoken in the coordinated clause: %q", got)
	}
}

// The focus standing in one direction against several standards coordinates them ("lighter than
// the cat and the dog"), the direction named once. This is the standard's axis of comparison
// coordination, mirroring how the verbs of one action already coordinate their co-arguments.
func TestDescriptionCoordinatesSameDirectionStandards(t *testing.T) {
	d, _ := testDaemon(t)
	for _, n := range []string{"mouse", "cat", "dog"} {
		groundNoun(t, d, n)
	}
	say(t, d, "a mouse is lighter than a cat")
	say(t, d, "a cat is heavier than a mouse") // pairs lighter and heavier so the relation is gradable
	say(t, d, "a mouse is lighter than a dog")

	got := say(t, d, "tell me about the mouse").Say
	if !strings.Contains(got, "lighter than the cat and the dog") &&
		!strings.Contains(got, "lighter than the dog and the cat") {
		t.Fatalf("one direction against several standards should coordinate them: %q", got)
	}
	if c := strings.Count(got, "lighter"); c != 1 {
		t.Fatalf("the direction should be named once, not %d: %q", c, got)
	}
}

// The construction-traversal comprehender reads a clause to the same meaning the coded path does,
// and the construction round-trips: generation walks it back to the clause. One bidirectional
// form-meaning pairing, walked both ways. This is the first slice of comprehension-by-traversal,
// shadowed against the coded parser.
func TestComprehendByTraversalShadowsAndRoundTrips(t *testing.T) {
	d, _ := testDaemon(t)
	groundNoun(t, d, "dog")
	groundNoun(t, d, "cat")
	say(t, d, "a dog chased a cat") // establishes the chase relation

	coded, ok1 := d.gram.Comprehend("en", "a dog chased a cat", d.resolveConcept, d.roleFit)
	walked, ok2 := d.gram.ComprehendByTraversal("en", "a dog chased a cat", d.resolveConcept, d.roleFit)
	if !ok1 || !ok2 {
		t.Fatalf("both paths should comprehend the clause: coded=%v walked=%v", ok1, ok2)
	}
	if !meaningsAgree(walked, coded) {
		t.Fatalf("the traversal should read the meaning the coded path does: walked %+v vs coded %+v", walked, coded)
	}
	if walked.Relation.Concept == nil || walked.Roles["agent"].Concept == nil || walked.Roles["patient"].Concept == nil {
		t.Fatalf("the traversal should ground the relation and both roles: %+v", walked)
	}

	// Round-trip: generation walks the same construction back to the clause (definiteness is the
	// generator's to choose, so it is set for the surface).
	back := grammar.Meaning{Relation: walked.Relation, Past: walked.Past, Lang: "en", Roles: map[string]grammar.Term{
		"agent":   {Concept: walked.Roles["agent"].Concept, Definite: true},
		"patient": {Concept: walked.Roles["patient"].Concept, Definite: true},
	}}
	if got, ok := d.gram.Speak("en", back); !ok || got != "The dog chased the cat." {
		t.Fatalf("the construction should round-trip to the clause: %q (ok=%v)", got, ok)
	}
}

// The traversal comprehender is load-bearing on the live teaching path: told clauses are read
// through the walk (the coded path the cross-check), so they comprehend correctly and the cutover
// stays silent. A proposition taught this way reads back the way it was told, and across a range of
// clauses no mismatch fires.
func TestStatementCutoverDrivesComprehension(t *testing.T) {
	d, _ := testDaemon(t)
	var buf bytes.Buffer
	d.out = &buf
	for _, n := range []string{"dog", "cat", "mouse", "wolf"} {
		groundNoun(t, d, n)
	}
	for _, clause := range []string{
		"a dog chased a cat",
		"a wolf chased a mouse",
		"a cat chased a mouse",
		"the dog chased the cat",
	} {
		say(t, d, clause)
	}
	// The walk drove the read, so the proposition is in the graph and answers back.
	if got := say(t, d, "what did the wolf chase").Say; got != "The wolf chased the mouse." {
		t.Fatalf("the clause comprehended through the traversal should read back: %q", got)
	}
	if strings.Contains(buf.String(), "comprehension traversal mismatch") {
		t.Fatalf("the traversal should agree with the coded path across clauses, but a mismatch fired:\n%s", buf.String())
	}
}

// The traversal recovers a Turkish pro-dropped subject the way the coded path does: "kediyi
// kovaladım" is "(I) chased the cat", the verb's -m carrying the speaker as the agent.
func TestComprehendByTraversalReadsTurkishProDrop(t *testing.T) {
	d, _ := testDaemon(t)
	dog := groundNoun(t, d, "dog")
	cat := groundNoun(t, d, "cat")
	nameInLang(t, d, dog, "köpek", "tr")
	nameInLang(t, d, cat, "kedi", "tr")
	sayLang(t, d, "tr", "köpek kediyi kovaladı") // establishes the kovala relation

	// Word order is authoritative for a deictic subject (nil fit), the way the daemon reads it.
	coded, ok1 := d.gram.Comprehend("tr", "kediyi kovaladım", d.resolveConcept, nil)
	walked, ok2 := d.gram.ComprehendByTraversal("tr", "kediyi kovaladım", d.resolveConcept, nil)
	if !ok1 || !ok2 {
		t.Fatalf("both paths should read the pro-drop clause: coded=%v walked=%v", ok1, ok2)
	}
	if !meaningsAgree(walked, coded) {
		t.Fatalf("the traversal should match the coded reading of pro-drop: walked %+v vs coded %+v", walked, coded)
	}
	if a := walked.Roles["agent"].Concept; a == nil || a.ID != d.userID {
		t.Fatalf("the dropped subject should resolve to the speaker: %+v", walked.Roles["agent"])
	}
	if p := walked.Roles["patient"].Concept; p == nil || p.ID != cat.ID {
		t.Fatalf("the patient should be the cat: %+v", walked.Roles["patient"])
	}
}

// A "what is X" GOLEM holds only the generic kind for it answers with a hedged
// guess at what it is, from the better-known thing it resembles. The wolf, seen
// once, resembles the dog GOLEM has seen many times, so "what is a wolf" reaches
// past "a thing" to "probably a dog". A firmly-known thing is never reclassified
// as the thing it merely resembles.
func TestDefinitionInfersKind(t *testing.T) {
	d, s := testDaemon(t)
	dog := groundNoun(t, d, "dog")
	wolf := groundNoun(t, d, "wolf")
	// The dog is well grounded, the wolf not at all: a prototype percept the dog
	// rolled many sightings into, so the dog is the better-known thing.
	moment := &memory.Fragment{Channel: "vision", Content: "a brown dog", Template: "a brown dog",
		Count: 8, Limbic: d.chem.Snapshot(), Time: time.Now()}
	if _, err := s.Encode(moment); err != nil {
		t.Fatal(err)
	}
	if err := s.SetLinkRel(dog.ID, moment.ID, memory.RelGrounds, 1.0); err != nil {
		t.Fatal(err)
	}
	feat := &memory.Fragment{Kind: memory.KindFeature, Channel: "feature", Content: "predator",
		Template: "predator", Limbic: d.chem.Snapshot(), Time: time.Now()}
	if _, err := s.Encode(feat); err != nil {
		t.Fatal(err)
	}
	s.SetLinkRel(dog.ID, feat.ID, memory.RelFeature, 1.0)
	s.SetLinkRel(wolf.ID, feat.ID, memory.RelFeature, 1.0)

	if got := say(t, d, "what is a wolf").Say; !strings.Contains(got, "Probably, it is a dog.") {
		t.Fatalf("a thinly-known thing should guess its kind from a near neighbour: %q", got)
	}
	if got := say(t, d, "what is a dog").Say; strings.Contains(got, "Probably") {
		t.Fatalf("a firmly-known thing should not guess its kind: %q", got)
	}
}

// linksTo reports whether any edge points at the given target.
func linksTo(edges []memory.LinkEdge, to uint64) bool {
	for _, e := range edges {
		if e.To == to {
			return true
		}
	}
	return false
}

// A member word resolves to its own specific concept, not the superordinate it
// names only by inheritance. The abstraction's name-inheritance can bond "cat"
// to the animal abstraction more firmly than to the cat percept; left unchecked
// that bound a chase argument to "animal". A category word still resolves to its
// own abstraction, which it matches.
func TestResolveConceptPrefersSpecificOverInheritedAbstraction(t *testing.T) {
	d, s := testDaemon(t)
	mkConcept := func(template, content string) *memory.Fragment {
		c := &memory.Fragment{Kind: memory.KindConcept, Channel: "vision", Content: content,
			Template: template, Limbic: d.chem.Snapshot(), Time: time.Now()}
		if _, err := s.Encode(c); err != nil {
			t.Fatal(err)
		}
		return c
	}
	percept := mkConcept("a small black cat", "a small black cat")
	animal := mkConcept("abs:en:animal", "animal")
	catW, _, err := s.WordNode("cat", "en", d.chem.Snapshot(), time.Now())
	if err != nil {
		t.Fatal(err)
	}
	animalW, _, err := s.WordNode("animal", "en", d.chem.Snapshot(), time.Now())
	if err != nil {
		t.Fatal(err)
	}
	// "cat" names the cat percept directly (0.4) and the animal abstraction by
	// inheritance, more firmly (0.8); "animal" names its own abstraction.
	s.SetLinkRel(catW.ID, percept.ID, memory.RelNames, 0.4)
	s.SetLinkRel(catW.ID, animal.ID, memory.RelNames, 0.8)
	s.SetLinkRel(animalW.ID, animal.ID, memory.RelNames, 0.8)

	if got := d.resolveConcept("cat"); got == nil || got.ID != percept.ID {
		t.Fatalf("a member word should resolve to its specific concept, not the inherited superordinate: %+v", got)
	}
	if got := d.resolveConcept("animal"); got == nil || got.ID != animal.ID {
		t.Fatalf("a category word should resolve to its own abstraction: %+v", got)
	}
}

// A concept answers to its own name. A superordinate is named by its own word and,
// through inheritance, by its dominant member's word, which heavy use can grow past
// the category's own ("cat" naming the animal abstraction above "animal"); it should
// still speak as the category, not as one of its members. A percept's caption is no
// single word, so it still glosses by its firmest namer.
func TestConceptNameSpeaksOwnNameOverStrongerMember(t *testing.T) {
	d, s := testDaemon(t)
	mk := func(tpl, content string) *memory.Fragment {
		c := &memory.Fragment{Kind: memory.KindConcept, Channel: "concept", Content: content,
			Template: tpl, Category: "noun", Limbic: d.chem.Snapshot(), Time: time.Now()}
		if _, err := s.Encode(c); err != nil {
			t.Fatal(err)
		}
		return c
	}
	animal := mk("abs:en:animal", "animal")
	animalW, _, err := s.WordNode("animal", "en", d.chem.Snapshot(), time.Now())
	if err != nil {
		t.Fatal(err)
	}
	catW, _, err := s.WordNode("cat", "en", d.chem.Snapshot(), time.Now())
	if err != nil {
		t.Fatal(err)
	}
	// The category's own word names it; its dominant member's word names it more
	// strongly through inheritance.
	s.SetLinkRel(animalW.ID, animal.ID, memory.RelNames, 0.80)
	s.SetLinkRel(catW.ID, animal.ID, memory.RelNames, 0.85)
	if got := d.conceptName(animal.ID, "animal", "en"); got != "animal" {
		t.Fatalf("a category should speak by its own name, not its stronger member's: %q", got)
	}

	percept := mk("a small black cat", "a small black cat")
	s.SetLinkRel(catW.ID, percept.ID, memory.RelNames, 0.50)
	if got := d.conceptName(percept.ID, "a small black cat", "en"); got != "cat" {
		t.Fatalf("a percept should gloss by its firmest namer: %q", got)
	}
}

// A member inherits its category's doing, defeasibly: told the animal chased the
// mouse, a cat (an animal) probably did too, hedged because a category's doing
// binds no one member. The inheritance answers the polar and the wh alike, and a
// non-member inherits nothing.
func TestDefeasibleCategoryInheritance(t *testing.T) {
	d, s := testDaemon(t)
	cat := groundNoun(t, d, "cat")
	rabbit := groundNoun(t, d, "rabbit")
	groundNoun(t, d, "mouse")
	groundNoun(t, d, "bottle")
	animal := &memory.Fragment{Kind: memory.KindConcept, Channel: "concept", Content: "animal",
		Template: "abs:en:animal", Category: "noun", Limbic: d.chem.Snapshot(), Time: time.Now()}
	if _, err := s.Encode(animal); err != nil {
		t.Fatal(err)
	}
	animalW, _, err := s.WordNode("animal", "en", d.chem.Snapshot(), time.Now())
	if err != nil {
		t.Fatal(err)
	}
	s.SetLinkRel(animalW.ID, animal.ID, memory.RelNames, 0.8)
	s.SetLinkRel(animal.ID, cat.ID, memory.RelGrounds, 0.5)
	s.SetLinkRel(animal.ID, rabbit.ID, memory.RelGrounds, 0.5)
	say(t, d, "an animal chased the mouse") // a category-level fact

	if got := say(t, d, "did the cat chase the mouse").Say; got != "Probably, the cat chased the mouse." {
		t.Fatalf("a member should inherit the category's doing, hedged: %q", got)
	}
	if got := say(t, d, "what did the rabbit chase").Say; got != "Probably, the rabbit chased the mouse." {
		t.Fatalf("the wh answer should inherit too, for consistency with the polar: %q", got)
	}
	if got := say(t, d, "did the bottle chase the mouse").Say; !strings.HasPrefix(got, "No") {
		t.Fatalf("a non-member inherits nothing: %q", got)
	}
}

// A told negative overrides an inherited default, the defeasible exception: the cat would inherit
// the animal's chasing of the mouse, but told it did not, GOLEM denies it. The negative is a
// standing denial in the graph, honoured before the inheritance is weighed, so the specific beats
// the general even after the guess was already voiced.
func TestDefeasibleToldNegativeOverridesInheritance(t *testing.T) {
	d, s := testDaemon(t)
	cat := groundNoun(t, d, "cat")
	groundNoun(t, d, "mouse")
	animal := &memory.Fragment{Kind: memory.KindConcept, Channel: "concept", Content: "animal",
		Template: "abs:en:animal", Category: "noun", Limbic: d.chem.Snapshot(), Time: time.Now()}
	if _, err := s.Encode(animal); err != nil {
		t.Fatal(err)
	}
	animalW, _, err := s.WordNode("animal", "en", d.chem.Snapshot(), time.Now())
	if err != nil {
		t.Fatal(err)
	}
	s.SetLinkRel(animalW.ID, animal.ID, memory.RelNames, 0.8)
	s.SetLinkRel(animal.ID, cat.ID, memory.RelGrounds, 0.5)
	say(t, d, "an animal chased the mouse")

	// The cat inherits the category's doing as a hedged default.
	if got := say(t, d, "did the cat chase the mouse").Say; !strings.HasPrefix(got, "Probably") {
		t.Fatalf("the cat should inherit the animal's doing, hedged: %q", got)
	}
	// Told it did not, GOLEM denies it, the specific negative overriding the general default.
	say(t, d, "the cat did not chase the mouse")
	if got := say(t, d, "did the cat chase the mouse").Say; !strings.HasPrefix(got, "No") {
		t.Fatalf("a told negative should override the inherited default: %q", got)
	}
}

// A denial told at a category is inherited by its members, the defeasible exception carried the way
// the default is: the animal chases the mouse so the cat would too, but told mammals do not, the cat
// (a mammal) denies it. A category-level negative is deny-once for every member.
func TestDefeasibleCategoryDenialInheritedByMembers(t *testing.T) {
	d, s := testDaemon(t)
	cat := groundNoun(t, d, "cat")
	groundNoun(t, d, "mouse")
	mkCategory := func(name string, member uint64) {
		c := &memory.Fragment{Kind: memory.KindConcept, Channel: "concept", Content: name,
			Template: "abs:en:" + name, Category: "noun", Limbic: d.chem.Snapshot(), Time: time.Now()}
		if _, err := s.Encode(c); err != nil {
			t.Fatal(err)
		}
		w, _, err := s.WordNode(name, "en", d.chem.Snapshot(), time.Now())
		if err != nil {
			t.Fatal(err)
		}
		s.SetLinkRel(w.ID, c.ID, memory.RelNames, 0.8)
		s.SetLinkRel(c.ID, member, memory.RelGrounds, 0.5)
	}
	mkCategory("animal", cat.ID)
	mkCategory("mammal", cat.ID)
	say(t, d, "an animal chased the mouse")

	// The cat inherits the animal's doing as a hedged default.
	if got := say(t, d, "did the cat chase the mouse").Say; !strings.HasPrefix(got, "Probably") {
		t.Fatalf("the cat should inherit the animal's doing, hedged: %q", got)
	}
	// Told mammals do not, the cat (a mammal) denies it, the category-level denial overriding the
	// inherited default.
	say(t, d, "the mammal did not chase the mouse")
	if got := say(t, d, "did the cat chase the mouse").Say; !strings.HasPrefix(got, "No") {
		t.Fatalf("a category-level denial should be inherited and override the inherited default: %q", got)
	}
}

// Part-whole composes with placement: a part is located where its whole is, so a leaf that is part
// of a tree that is in the forest is in the forest. A part is not thereby inside its whole, the way
// contact does not compose, so the leaf is not in the tree itself.
func TestPartWholeComposesWithContainment(t *testing.T) {
	d, _ := testDaemon(t)
	groundNoun(t, d, "leaf")
	groundNoun(t, d, "tree")
	groundNoun(t, d, "forest")
	say(t, d, "the leaf is part of the tree")
	say(t, d, "the tree is in the forest")

	if got := say(t, d, "is the leaf in the forest").Say; !strings.HasPrefix(got, "Yes") {
		t.Fatalf("a part of a thing in a place is in that place: %q", got)
	}
	if got := say(t, d, "is the leaf in the tree").Say; strings.HasPrefix(got, "Yes") {
		t.Fatalf("a part is located with its whole, not inside it: %q", got)
	}
}

// GOLEM reasons over the is-a hierarchy: it answers whether a thing belongs to a
// category, and composes membership with a relation, soundly and firmly, so "did
// the dog chase an animal" holds because the dog chased a cat and a cat is an
// animal, while a thing that chased nothing chased no animal.
func TestCategoricalDeduction(t *testing.T) {
	d, s := testDaemon(t)
	groundNoun(t, d, "dog")
	cat := groundNoun(t, d, "cat")
	animal := &memory.Fragment{Kind: memory.KindConcept, Channel: "concept", Content: "animal",
		Template: "abs:en:animal", Category: "noun", Limbic: d.chem.Snapshot(), Time: time.Now()}
	if _, err := s.Encode(animal); err != nil {
		t.Fatal(err)
	}
	animalW, _, err := s.WordNode("animal", "en", d.chem.Snapshot(), time.Now())
	if err != nil {
		t.Fatal(err)
	}
	s.SetLinkRel(animalW.ID, animal.ID, memory.RelNames, 0.8)
	s.SetLinkRel(animal.ID, cat.ID, memory.RelGrounds, 0.5) // the animal category grounds in the cat
	say(t, d, "a dog chased a cat")

	if got := say(t, d, "is a cat an animal").Say; got != "Yes, a cat is an animal." {
		t.Fatalf("the cat should be known an animal: %q", got)
	}
	if got := say(t, d, "is a cat a dog").Say; !strings.HasPrefix(got, "No") {
		t.Fatalf("a cat is not a dog: %q", got)
	}
	if got := say(t, d, "did the dog chase an animal").Say; !strings.HasPrefix(got, "Yes") {
		t.Fatalf("the dog chased a cat, and a cat is an animal: %q", got)
	}
	if got := say(t, d, "did the cat chase an animal").Say; !strings.HasPrefix(got, "No") {
		t.Fatalf("the cat chased nothing, so it chased no animal: %q", got)
	}
}

// Categorical deduction composes through BOTH roles at once: an animal ate a food
// because the rabbit (an animal) ate the carrot (a food), two grounded is-a hops
// chained. The same shape holds when both roles are the one category: an animal
// chased an animal because the wolf chased the rabbit, both animals. A category pair
// with no doubly-grounded fact stays a No, so the chain deduces, never fabricates.
func TestCategoricalDeductionBothRoles(t *testing.T) {
	d, s := testDaemon(t)
	rabbit := groundNoun(t, d, "rabbit")
	carrot := groundNoun(t, d, "carrot")
	wolf := groundNoun(t, d, "wolf")
	tree := groundNoun(t, d, "tree")
	category := func(tpl, word string, members ...*memory.Fragment) {
		c := &memory.Fragment{Kind: memory.KindConcept, Channel: "concept", Content: word,
			Template: tpl, Category: "noun", Limbic: d.chem.Snapshot(), Time: time.Now()}
		if _, err := s.Encode(c); err != nil {
			t.Fatal(err)
		}
		w, _, err := s.WordNode(word, "en", d.chem.Snapshot(), time.Now())
		if err != nil {
			t.Fatal(err)
		}
		s.SetLinkRel(w.ID, c.ID, memory.RelNames, 0.8)
		for _, m := range members {
			s.SetLinkRel(c.ID, m.ID, memory.RelGrounds, 0.5)
		}
	}
	category("abs:en:animal", "animal", rabbit, wolf)
	category("abs:en:food", "food", carrot)
	category("abs:en:plant", "plant", tree)
	say(t, d, "a rabbit ate a carrot")  // a grounded fact, both ends concrete
	say(t, d, "a wolf chased a rabbit") // a same-category fact, both animals

	// Both roles are categories: the chain composes two is-a hops.
	if got := say(t, d, "did an animal eat a food").Say; !strings.HasPrefix(got, "Yes") {
		t.Fatalf("a rabbit (an animal) ate a carrot (a food): %q", got)
	}
	// Both categories the same: an animal chased an animal because a wolf chased a rabbit.
	if got := say(t, d, "did an animal chase an animal").Say; !strings.HasPrefix(got, "Yes") {
		t.Fatalf("a wolf chased a rabbit, both animals: %q", got)
	}
	// Single-role categorical still holds (no regression from the new branch).
	if got := say(t, d, "did the rabbit eat a food").Say; !strings.HasPrefix(got, "Yes") {
		t.Fatalf("concrete agent, category patient: %q", got)
	}
	if got := say(t, d, "did an animal eat a carrot").Say; !strings.HasPrefix(got, "Yes") {
		t.Fatalf("category agent, concrete patient: %q", got)
	}
	// No doubly-grounded fact: an animal ate a plant is not invented.
	if got := say(t, d, "did an animal eat a plant").Say; !strings.HasPrefix(got, "No") {
		t.Fatalf("no animal-member ate a plant-member, so the chain must not fabricate: %q", got)
	}
}

// A category refinement is grammar scaffolding, so the nightly prune spares it: the
// colon-class taxonomy must not decay on the first sleep. Before the fix the refinement
// concepts were bare nouns the prune deleted, which (for "wh:what") reverted Turkish "ne"
// to a bare interrogative and stopped a wh-object question from routing after a night.
func TestCategoryRefinementSurvivesSleep(t *testing.T) {
	d, s := testDaemon(t) // loadGrammar grows the refinements at boot
	refinements := []string{"wh:what", "speech:know", "fn:locative"}
	for _, tpl := range refinements {
		if c, _ := s.ConceptByTemplate(tpl); c == nil {
			t.Fatalf("%s should exist after boot", tpl)
		}
	}
	if _, err := s.Sleep(d.cfg.NightlyDecay, d.cfg.PruneFloor); err != nil {
		t.Fatal(err)
	}
	if err := d.recompileGrammar(); err != nil {
		t.Fatal(err)
	}
	// Deliberately do not re-run ensureCategoryRefinements: the point is that the prune
	// spared the concept, not that a reboot re-grew it.
	for _, tpl := range refinements {
		if c, _ := s.ConceptByTemplate(tpl); c == nil {
			t.Fatalf("%s should survive the night as scaffolding, got pruned", tpl)
		}
	}
}

// GOLEM treats the conversation's participants as concepts the deictic pronouns
// resolve to, from the speaker's side: "you" is GOLEM, "I" the speaker. It reasons
// over a clause about them like any other and answers from its own side, so a fact
// told with "you" comes back as "I", and one told with "I" comes back as "you".
func TestParticipantDeixisRoundTrips(t *testing.T) {
	d, _ := testDaemon(t)
	groundNoun(t, d, "cat")
	groundNoun(t, d, "dog")

	// A fact GOLEM is told of itself in the second person it holds of itself, and
	// speaks back in the first: "you chased the cat" comes back as "I chased the
	// cat".
	say(t, d, "you chased the cat")
	if got := say(t, d, "did you chase the cat").Say; got != "Yes, I chased the cat." {
		t.Fatalf("the fact told with you should hold, spoken back as I: %q", got)
	}
	// What it was not told of itself, it denies of itself, again as "I".
	if got := say(t, d, "did you chase the dog").Say; got != "No, I did not chase the dog." {
		t.Fatalf("you is GOLEM, which denies of itself as I: %q", got)
	}
	// Told of the speaker in the first person, GOLEM speaks back in the second.
	say(t, d, "I chased the dog")
	if got := say(t, d, "did I chase the dog").Say; got != "Yes, you chased the dog." {
		t.Fatalf("the fact told with I should hold, spoken back as you: %q", got)
	}
}

// The participants resolve in Turkish too, the asymmetry the deixis pronouns left
// open: "sen" (you) is GOLEM, "ben" (I) the speaker, so a Turkish clause about the
// speakers grounds in them and a fact told with "sen" comes back affirmed.
func TestParticipantDeixisTurkish(t *testing.T) {
	d, _ := testDaemon(t)
	groundNounLang(t, d, "kedi", "tr")
	groundNounLang(t, d, "köpek", "tr")
	// "sen" (you) is GOLEM; told of itself, it affirms of itself as "ben", the verb
	// agreeing in the first person.
	sayLang(t, d, "tr", "sen kediyi kovaladın")
	if got := sayLang(t, d, "tr", "sen kediyi kovaladın mı").Say; got != "Evet, ben kediyi kovaladım." {
		t.Fatalf("the fact told with sen should hold, spoken back as ben: %q", got)
	}
	if got := sayLang(t, d, "tr", "sen köpeği kovaladın mı").Say; got != "Hayır, ben köpeği kovalamadım." {
		t.Fatalf("an unheld fact about GOLEM should deny as ben: %q", got)
	}
	// "ben" (I) is the speaker; told of the speaker, GOLEM speaks back as "sen".
	sayLang(t, d, "tr", "ben köpeği kovaladım")
	if got := sayLang(t, d, "tr", "ben köpeği kovaladım mı").Say; got != "Evet, sen köpeği kovaladın." {
		t.Fatalf("the fact told with ben should hold, spoken back as sen: %q", got)
	}
}

// Turkish drops a subject its verb's agreement carries: "kediyi kovaladım" is
// "(I) chased the cat", the -m marking first person, so the fact holds of the
// speaker and GOLEM speaks it back as "sen". No overt pronoun is spoken.
func TestParticipantProDropTurkish(t *testing.T) {
	d, _ := testDaemon(t)
	groundNounLang(t, d, "kedi", "tr")
	groundNounLang(t, d, "köpek", "tr")
	sayLang(t, d, "tr", "kediyi kovaladım") // (I) chased the cat, subject dropped
	if got := sayLang(t, d, "tr", "kediyi kovaladım mı").Say; got != "Evet, sen kediyi kovaladın." {
		t.Fatalf("a dropped first-person subject should be recovered as the speaker: %q", got)
	}
	if got := sayLang(t, d, "tr", "köpeği kovaladım mı").Say; !strings.HasPrefix(got, "Hayır") {
		t.Fatalf("an unheld fact about the dropped speaker should deny: %q", got)
	}
}

// A wh-question can ask after a participant: told "you chased the cat", "what did
// you chase" walks the chase from GOLEM and answers in the first person. The
// deictic pronoun is closed-class, so the question parse must keep it as the known
// argument; otherwise "you chase" reads as a thing to define.
func TestParticipantAgentQuestion(t *testing.T) {
	d, _ := testDaemon(t)
	groundNoun(t, d, "cat")
	say(t, d, "you chased the cat")
	if got := say(t, d, "what did you chase").Say; got != "I chased the cat." {
		t.Fatalf("a wh-question with a participant agent should answer in the first person: %q", got)
	}
}

// A participant in the patient role takes English object case: told "the dog
// chased you", GOLEM answers "the dog chased me" across the polar and wh paths,
// while a participant in the agent role keeps the subject form "I". Turkish,
// which cases the object by the accusative, is covered by the deixis tests above.
func TestParticipantObjectCase(t *testing.T) {
	d, _ := testDaemon(t)
	groundNoun(t, d, "dog")
	groundNoun(t, d, "cat")
	say(t, d, "the dog chased you")
	if got := say(t, d, "did the dog chase you").Say; got != "Yes, the dog chased me." {
		t.Fatalf("a participant patient should take object case in a polar answer: %q", got)
	}
	if got := say(t, d, "who chased you").Say; got != "The dog chased me." {
		t.Fatalf("a participant patient should take object case in a wh-answer: %q", got)
	}
	say(t, d, "you chased the cat")
	if got := say(t, d, "did you chase the cat").Say; got != "Yes, I chased the cat." {
		t.Fatalf("a participant agent should keep the subject form: %q", got)
	}
}

// GOLEM orders concepts along a gradable relation it was told about, and reasons
// over the ordering transitively: told the cat is bigger than the mouse and the
// mouse than the snail, it answers the cat is bigger than the snail, a chain it was
// never told directly. The ordering lives in the told facts, not a scale in code.
func TestComparativeOrdersTransitively(t *testing.T) {
	d, _ := testDaemon(t)
	groundNoun(t, d, "cat")
	groundNoun(t, d, "mouse")
	groundNoun(t, d, "snail")
	say(t, d, "the cat is bigger than the mouse")
	say(t, d, "the mouse is bigger than the snail")
	if got := say(t, d, "is the cat bigger than the snail").Say; got != "Yes, the cat is bigger than the snail." {
		t.Fatalf("a chained comparison should hold transitively: %q", got)
	}
	if got := say(t, d, "is the snail bigger than the cat").Say; got != "No, the snail is not bigger than the cat." {
		t.Fatalf("the reverse of a told ordering should not hold: %q", got)
	}
	if got := say(t, d, "is the cat bigger than the mouse").Say; got != "Yes, the cat is bigger than the mouse." {
		t.Fatalf("a directly told comparison should hold: %q", got)
	}
	// A dimension never compared on ("smaller", only "bigger" was told) is admitted,
	// not denied: GOLEM does not assert an ordering it never heard.
	if got := say(t, d, "is the mouse smaller than the cat").Say; strings.HasPrefix(got, "No") {
		t.Fatalf("an untold dimension should be admitted, not denied: %q", got)
	}
}

// An analytic comparative ("more clever than") is read as one relation "more
// clever", chaining transitively like a synthetic "-er" one. "Less" is the
// grammatical inverse, so "the mouse is less clever than the dog" is answered as
// the positive comparative, "the dog is more clever than the mouse".
func TestAnalyticComparative(t *testing.T) {
	d, _ := testDaemon(t)
	groundNoun(t, d, "cat")
	groundNoun(t, d, "dog")
	groundNoun(t, d, "mouse")
	say(t, d, "the dog is more clever than the cat")
	say(t, d, "the cat is more clever than the mouse")
	if got := say(t, d, "is the dog more clever than the mouse").Say; got != "Yes, the dog is more clever than the mouse." {
		t.Fatalf("an analytic comparative should chain transitively: %q", got)
	}
	if got := say(t, d, "is the mouse less clever than the dog").Say; got != "Yes, the dog is more clever than the mouse." {
		t.Fatalf("less should invert to the positive comparative: %q", got)
	}
}

// Turkish marks a comparison's standard by ablative case and drops the copula
// ("kedi fareden büyük"), so it parses its own way, but the ordering reasoning is
// the same: a chained comparison holds transitively, the reverse does not, and the
// answer is spoken with the standard back in the ablative.
func TestComparativeTurkish(t *testing.T) {
	d, _ := testDaemon(t)
	groundNounLang(t, d, "kedi", "tr")
	groundNounLang(t, d, "fare", "tr")
	groundNounLang(t, d, "köpek", "tr")
	sayLang(t, d, "tr", "köpek kediden büyük")     // the dog is bigger than the cat (ablative, no daha)
	sayLang(t, d, "tr", "kedi fareden daha büyük") // the cat is bigger than the mouse (with daha)
	// Both forms normalize to the same comparative "daha büyük", so the chain holds.
	if got := sayLang(t, d, "tr", "köpek fareden büyük mü").Say; got != "Evet, köpek fareden daha büyük." {
		t.Fatalf("a chained Turkish comparison should hold transitively, spoken as the comparative: %q", got)
	}
	if got := sayLang(t, d, "tr", "fare köpekten daha büyük mü").Say; got != "Hayır, fare köpekten daha büyük değil." {
		t.Fatalf("the reverse Turkish ordering should be denied: %q", got)
	}
}

// The comparative is one relation across tongues: told in English then Turkish
// back to back, "daha büyük" unifies with "bigger" the way kovala unifies with
// chase, so a comparison chain split across the tongues holds transitively and is
// answered in either. The nouns are bilingual, one concept named in both.
func TestComparativeUnifiesCrossTongue(t *testing.T) {
	d, _ := testDaemon(t)
	bilingual := func(en, tr string) *memory.Fragment {
		now := time.Now()
		c := &memory.Fragment{Kind: memory.KindConcept, Channel: "concept", Content: en,
			Template: en, Category: "noun", Limbic: d.chem.Snapshot(), Time: now}
		if _, err := d.store.Encode(c); err != nil {
			t.Fatal(err)
		}
		for _, nm := range []struct{ form, lang string }{{en, "en"}, {tr, "tr"}} {
			w, _, err := d.store.WordNode(nm.form, nm.lang, d.chem.Snapshot(), now)
			if err != nil {
				t.Fatal(err)
			}
			if err := d.store.SetLinkRel(w.ID, c.ID, memory.RelNames, 0.9); err != nil {
				t.Fatal(err)
			}
		}
		return c
	}
	bilingual("cat", "kedi")
	bilingual("mouse", "fare")
	bilingual("dog", "köpek")
	// The chain is split across tongues, taught back to back so the relation unifies.
	say(t, d, "the dog is bigger than the cat")    // English: the bigger relation, warm
	sayLang(t, d, "tr", "kedi fareden daha büyük") // Turkish: daha büyük joins the same relation
	// Asked in Turkish over the whole chain (dog > cat told in English, cat > mouse in Turkish).
	if got := sayLang(t, d, "tr", "köpek fareden daha büyük mü").Say; !strings.HasPrefix(got, "Evet") {
		t.Fatalf("a chain split across tongues should hold, asked in Turkish: %q", got)
	}
	// And the same chain asked in English.
	if got := say(t, d, "is the dog bigger than the mouse").Say; !strings.HasPrefix(got, "Yes") {
		t.Fatalf("the same chain should hold asked in English: %q", got)
	}
}

// GOLEM discovers that two gradable relations are converse poles of one scale from
// the ordering itself: told one pair the other way, it learns "smaller" reverses
// "bigger" and answers smaller-questions across the whole bigger-chain, though it
// was told only one smaller fact. The link is discovered, not seeded or told as an
// antonym.
func TestComparativeConverseDiscovered(t *testing.T) {
	d, _ := testDaemon(t)
	groundNoun(t, d, "cat")
	groundNoun(t, d, "mouse")
	groundNoun(t, d, "snail")
	say(t, d, "the cat is bigger than the mouse")
	say(t, d, "the mouse is bigger than the snail")
	// One reversed telling reveals the converse: the cat stands above the mouse, so
	// "smaller" is the other reading of the same scale.
	say(t, d, "the mouse is smaller than the cat")
	if got := say(t, d, "is the snail smaller than the cat").Say; got != "Yes, the snail is smaller than the cat." {
		t.Fatalf("the converse should generalize across the chain: %q", got)
	}
	if got := say(t, d, "is the cat smaller than the snail").Say; got != "No, the cat is not smaller than the snail." {
		t.Fatalf("the larger end is not smaller than the smaller: %q", got)
	}
}

// Two scales that correlate (size and speed both order the horse above the dog)
// must not have their converse poles conflated. Telling "the dog is slower than the
// horse" reverses the size pair as well, but slower is the converse of faster, the
// scale it shares, not of bigger. A scale keeps the one converse its pole-pair
// claimed, so a speed question is never answered off the size chain.
func TestConverseDoesNotConflateScales(t *testing.T) {
	d, s := testDaemon(t)
	for _, n := range []string{"lion", "horse", "dog", "snail"} {
		groundNoun(t, d, n)
	}
	// Size, with the lion at the top: taught for size only, never for speed.
	say(t, d, "the lion is bigger than the horse")
	say(t, d, "the horse is bigger than the dog")
	say(t, d, "the dog is bigger than the snail")
	say(t, d, "the snail is smaller than the dog") // size converse: smaller <-> bigger
	// Speed over the lower animals: the horse above the dog above the snail.
	say(t, d, "the horse is faster than the dog")
	say(t, d, "the dog is faster than the snail")
	say(t, d, "the dog is slower than the horse") // reverses bigger AND faster

	bigger, faster, slower := d.resolveRelation("bigger"), d.resolveRelation("faster"), d.resolveRelation("slower")
	if bigger == nil || faster == nil || slower == nil {
		t.Fatal("the three relations should be known after the tellings")
	}
	// Slower is the converse of faster (its own scale), not of bigger (a correlate).
	if edgeWeightRel(t, s, slower.ID, faster.ID, memory.RelOpposite) == 0 {
		t.Fatal("slower should be discovered as the converse of faster")
	}
	if w := edgeWeightRel(t, s, slower.ID, bigger.ID, memory.RelOpposite); w != 0 {
		t.Fatalf("slower must not become the converse of bigger (scale conflation), got %.2f", w)
	}
	// The genuine converses still reason across their whole chains.
	if got := say(t, d, "is the snail slower than the horse").Say; got != "Yes, the snail is slower than the horse." {
		t.Fatalf("the speed converse should chain: %q", got)
	}
	if got := say(t, d, "is the snail smaller than the lion").Say; got != "Yes, the snail is smaller than the lion." {
		t.Fatalf("the size converse should chain: %q", got)
	}
	// The lion has a size but no speed; a speed question about it must not be
	// answered off the size chain through a conflated converse.
	if got := say(t, d, "is the horse slower than the lion").Say; strings.HasPrefix(got, "Yes") {
		t.Fatalf("the lion's size must not answer a speed question: %q", got)
	}
}

// Magnitude is read from the told order: a concept's rank is how many it stands
// above on the scale, and the gap between two ranks is the ordinal distance, sorted
// into a qualitative degree. The comparisons are ordinal, so a wide gap reads as
// great and a one-step gap as slight, no metric amount claimed.
func TestComparativeMagnitudeFromRank(t *testing.T) {
	d, _ := testDaemon(t)
	lion := groundNoun(t, d, "lion")
	dog := groundNoun(t, d, "dog")
	cat := groundNoun(t, d, "cat")
	mouse := groundNoun(t, d, "mouse")
	snail := groundNoun(t, d, "snail")
	say(t, d, "the lion is bigger than the dog")
	say(t, d, "the dog is bigger than the cat")
	say(t, d, "the cat is bigger than the mouse")
	say(t, d, "the mouse is bigger than the snail")
	rel := d.resolveRelation("bigger")
	if rel == nil {
		t.Fatal("bigger should be a known relation after the tellings")
	}
	rank := func(c *memory.Fragment) int { return len(d.comparativeReach(rel, c.ID)) }
	if rank(snail) != 0 || rank(lion) != 4 {
		t.Fatalf("ranks from the chain wrong: snail=%d (want 0) lion=%d (want 4)", rank(snail), rank(lion))
	}
	cases := []struct {
		a, b *memory.Fragment
		gap  int
		deg  string
	}{
		{lion, snail, 4, "great"},
		{lion, mouse, 3, "great"},
		{cat, snail, 2, ""},
		{dog, cat, 1, "slight"},
	}
	for _, c := range cases {
		gap, ok := d.comparativeMagnitude(rel, c.a.ID, c.b.ID)
		if !ok || gap != c.gap {
			t.Fatalf("%s over %s: gap=%d ok=%v, want gap %d", c.a.Content, c.b.Content, gap, ok, c.gap)
		}
		if got := degreeBucket(gap); got != c.deg {
			t.Fatalf("gap %d bucketed %q, want %q", gap, got, c.deg)
		}
	}
	// The smaller end does not stand above the larger, so there is nothing to measure.
	if _, ok := d.comparativeMagnitude(rel, snail.ID, lion.ID); ok {
		t.Fatal("the snail is not bigger than the lion; no magnitude to give")
	}
}

// "How much bigger is X than Y" is answered with the degree read from the told
// order: a wide rank gap reads as much bigger, a one-step gap as a little, a
// middling gap as the plain comparative. The smaller end has no degree, so the
// plain comparison is denied instead.
func TestMagnitudeQuestionAnswered(t *testing.T) {
	d, _ := testDaemon(t)
	groundNoun(t, d, "lion")
	groundNoun(t, d, "dog")
	groundNoun(t, d, "cat")
	groundNoun(t, d, "mouse")
	groundNoun(t, d, "snail")
	say(t, d, "the lion is bigger than the dog")
	say(t, d, "the dog is bigger than the cat")
	say(t, d, "the cat is bigger than the mouse")
	say(t, d, "the mouse is bigger than the snail")
	if got := say(t, d, "how much bigger is the lion than the snail").Say; got != "The lion is much bigger than the snail." {
		t.Fatalf("a wide rank gap should read as much bigger: %q", got)
	}
	if got := say(t, d, "how much bigger is the dog than the cat").Say; got != "The dog is a little bigger than the cat." {
		t.Fatalf("a one-step gap should read as a little bigger: %q", got)
	}
	if got := say(t, d, "how much bigger is the cat than the snail").Say; got != "The cat is bigger than the snail." {
		t.Fatalf("a middling gap should read as the plain comparative: %q", got)
	}
	if got := say(t, d, "how much bigger is the snail than the lion").Say; !strings.HasPrefix(got, "No") {
		t.Fatalf("the smaller end is not bigger, so there is no degree to give: %q", got)
	}
}

// The question-side construction traversal is load-bearing on the live handler path for the shapes
// it reads completely: the copular is-a, the comparison, and its degree question are driven by the
// walk's frame, the coded parse the cross-check. The answers are unchanged (the walk agrees with the
// coded reading) and the parse-site shadow stays silent, so the cutover routes through the traversal
// without a mismatch. A wh role-question keeps the coded frame and is silent there too.
func TestQuestionCutoverDrivesCoveredShapes(t *testing.T) {
	d, s := testDaemon(t)
	var buf bytes.Buffer
	d.out = &buf
	cat := groundNoun(t, d, "cat")
	groundNoun(t, d, "mouse")
	animal := &memory.Fragment{Kind: memory.KindConcept, Channel: "concept", Content: "animal",
		Template: "abs:en:animal", Category: "noun", Limbic: d.chem.Snapshot(), Time: time.Now()}
	if _, err := s.Encode(animal); err != nil {
		t.Fatal(err)
	}
	animalW, _, err := s.WordNode("animal", "en", d.chem.Snapshot(), time.Now())
	if err != nil {
		t.Fatal(err)
	}
	s.SetLinkRel(animalW.ID, animal.ID, memory.RelNames, 0.8)
	s.SetLinkRel(animal.ID, cat.ID, memory.RelGrounds, 0.5)
	say(t, d, "the cat is bigger than the mouse")

	for _, c := range []struct{ q, want string }{
		{"is a cat an animal", "Yes, a cat is an animal."},
		{"is the cat bigger than the mouse", "Yes, the cat is bigger than the mouse."},
		{"how much bigger is the cat than the mouse", "The cat is a little bigger than the mouse."},
	} {
		if got := say(t, d, c.q).Say; got != c.want {
			t.Fatalf("%q: got %q, want %q", c.q, got, c.want)
		}
	}
	// A wh role-question is shadowed but keeps the coded frame; it must be silent too.
	say(t, d, "the cat chased the mouse")
	say(t, d, "what did the cat chase")
	if strings.Contains(buf.String(), "parse traversal mismatch") {
		t.Fatalf("the cutover should drive the covered shapes without a mismatch: %s", buf.String())
	}
}

// A proportion a:b::c:? is completed from the told relation: the relation that
// holds cat -> mouse (chase) is found and applied to the dog, so the fourth term is
// what the dog chased. Grounded in the relation, not a vector offset.
func TestAnalogicalProportion(t *testing.T) {
	d, _ := testDaemon(t)
	cat := groundNoun(t, d, "cat")
	mouse := groundNoun(t, d, "mouse")
	dog := groundNoun(t, d, "dog")
	bird := groundNoun(t, d, "bird")
	say(t, d, "the cat chased the mouse")
	say(t, d, "the dog chased the bird")
	rel, ans, ok := d.analogize(cat, mouse, dog)
	if !ok || rel != "chase" {
		t.Fatalf("the proportion's relation should be the chase that holds cat->mouse: rel=%q ok=%v", rel, ok)
	}
	found := false
	for _, f := range ans {
		if f.coID == bird.ID {
			found = true
		}
	}
	if !found {
		t.Fatalf("the dog is to the bird as the cat is to the mouse: %+v", ans)
	}
	// No relation holds from the bird to the dog, so the proportion has no fourth term.
	if _, _, ok := d.analogize(bird, dog, cat); ok {
		t.Fatal("a proportion with no relation between its first pair has no answer")
	}
}

// The proportion parses from "A is to B as C is to what" and is answered with the
// fourth term: the cat chased the mouse and the dog chased the bird, so "the cat is
// to the mouse as the dog is to what" yields the bird. A proportion whose first
// pair shares no relation is admitted, not forced.
func TestAnalogyQuestionAnswered(t *testing.T) {
	d, _ := testDaemon(t)
	groundNoun(t, d, "cat")
	groundNoun(t, d, "mouse")
	groundNoun(t, d, "dog")
	groundNoun(t, d, "bird")
	say(t, d, "the cat chased the mouse")
	say(t, d, "the dog chased the bird")
	if got := say(t, d, "the cat is to the mouse as the dog is to what").Say; got != "The bird." {
		t.Fatalf("the proportion should yield what the dog chased: %q", got)
	}
	if got := say(t, d, "the bird is to the dog as the cat is to what").Say; strings.HasPrefix(got, "The ") {
		t.Fatalf("no relation holds bird->dog, so the proportion should be admitted, not answered: %q", got)
	}
}

// A nested question composes two relational hops: "who chased what the dog chased"
// resolves the inner clause (the dog chased the cat) and asks the outer of its
// answer (who chased the cat), naming the dog and the mouse. A nested query whose
// inner clause finds nothing is admitted, not forced.
func TestMultiHopNestedQuery(t *testing.T) {
	d, _ := testDaemon(t)
	groundNoun(t, d, "dog")
	groundNoun(t, d, "cat")
	groundNoun(t, d, "mouse")
	say(t, d, "the dog chased the cat")
	say(t, d, "the mouse chased the cat")
	got := say(t, d, "who chased what the dog chased").Say
	if !strings.Contains(got, "dog") || !strings.Contains(got, "mouse") {
		t.Fatalf("who chased what the dog chased should name the dog and the mouse: %q", got)
	}
	// The cat chased nothing here, so the inner clause is empty and the gap admitted.
	if got := say(t, d, "who chased what the cat chased").Say; strings.Contains(got, "dog") || strings.Contains(got, "mouse") {
		t.Fatalf("the cat chased nothing, so the nested query has no answer: %q", got)
	}
}

// Asked what it means by a guess, GOLEM explains the reasoning: the concept the
// subject is like, and the fact that neighbour stands in, so a hedged answer is
// justified rather than only repeated.
func TestClarifyExplainsGuess(t *testing.T) {
	d, s := testDaemon(t)
	dog := groundNoun(t, d, "dog")
	groundNoun(t, d, "cat")
	wolf := groundNoun(t, d, "wolf")
	say(t, d, "a dog chased a cat")
	feat := &memory.Fragment{Kind: memory.KindFeature, Channel: "feature", Content: "predator",
		Template: "predator", Limbic: d.chem.Snapshot(), Time: time.Now()}
	if _, err := s.Encode(feat); err != nil {
		t.Fatal(err)
	}
	s.SetLinkRel(dog.ID, feat.ID, memory.RelFeature, 1.0)
	s.SetLinkRel(wolf.ID, feat.ID, memory.RelFeature, 1.0)
	if got := say(t, d, "did the wolf chase the cat").Say; got != "Probably, the wolf chased the cat." {
		t.Fatalf("the guess should fire: %q", got)
	}
	got := strings.ToLower(say(t, d, "what do you mean").Say)
	if !strings.Contains(got, "like") || !strings.Contains(got, "dog") || !strings.Contains(got, "chased the cat") {
		t.Fatalf("clarify should explain the reasoning behind the guess: %q", got)
	}
}

// A category-inheritance guess is clarified by its MEMBERSHIP basis, not a
// resemblance: "Cat is an animal. The animal chased the mouse." The is-a is the
// firm part, the inherited category fact the defeasible part the "probably" rode.
func TestClarifyExplainsCategoricalGuess(t *testing.T) {
	d, s := testDaemon(t)
	cat := groundNoun(t, d, "cat")
	groundNoun(t, d, "mouse")
	animal := &memory.Fragment{Kind: memory.KindConcept, Channel: "concept", Content: "animal",
		Template: "abs:en:animal", Category: "noun", Limbic: d.chem.Snapshot(), Time: time.Now()}
	if _, err := s.Encode(animal); err != nil {
		t.Fatal(err)
	}
	animalW, _, err := s.WordNode("animal", "en", d.chem.Snapshot(), time.Now())
	if err != nil {
		t.Fatal(err)
	}
	s.SetLinkRel(animalW.ID, animal.ID, memory.RelNames, 0.8)
	s.SetLinkRel(animal.ID, cat.ID, memory.RelGrounds, 0.5)
	say(t, d, "an animal chased the mouse") // a category-level fact

	if got := say(t, d, "did the cat chase the mouse").Say; got != "Probably, the cat chased the mouse." {
		t.Fatalf("the category-inheritance guess should fire: %q", got)
	}
	got := strings.ToLower(say(t, d, "what do you mean").Say)
	if !strings.Contains(got, "animal") || !strings.Contains(got, "chased the mouse") {
		t.Fatalf("clarify should explain the membership basis: %q", got)
	}
}

// The clarify move is recognized from the speech-act verb that carries it, so
// phrasings never listed anywhere ("explain", "what does that mean") reach the
// same explanation as "what do you mean": new ways of asking are vocabulary, not
// new code.
func TestClarifyRoutesOffSpeechActVerb(t *testing.T) {
	d, s := testDaemon(t)
	dog := groundNoun(t, d, "dog")
	groundNoun(t, d, "cat")
	wolf := groundNoun(t, d, "wolf")
	say(t, d, "a dog chased a cat")
	feat := &memory.Fragment{Kind: memory.KindFeature, Channel: "feature", Content: "predator",
		Template: "predator", Limbic: d.chem.Snapshot(), Time: time.Now()}
	if _, err := s.Encode(feat); err != nil {
		t.Fatal(err)
	}
	s.SetLinkRel(dog.ID, feat.ID, memory.RelFeature, 1.0)
	s.SetLinkRel(wolf.ID, feat.ID, memory.RelFeature, 1.0)
	if got := say(t, d, "did the wolf chase the cat").Say; got != "Probably, the wolf chased the cat." {
		t.Fatalf("the guess should fire: %q", got)
	}
	for _, ask := range []string{"explain", "what does that mean", "elaborate"} {
		got := strings.ToLower(say(t, d, ask).Say)
		if !strings.Contains(got, "like") || !strings.Contains(got, "dog") {
			t.Fatalf("%q should reach clarify through its verb: %q", ask, got)
		}
	}
}

// A correction of a claim retracts the conflicting fact and accepts the new one:
// "no, the dog chased the rabbit" after GOLEM affirmed the dog chased the cat
// leaves only the corrected fact standing.
func TestRepairCorrectsFact(t *testing.T) {
	d, _ := testDaemon(t)
	groundNoun(t, d, "dog")
	groundNoun(t, d, "cat")
	groundNoun(t, d, "rabbit")
	say(t, d, "a dog chased a cat")
	say(t, d, "did the dog chase the cat") // GOLEM commits to the dog chasing the cat
	if got := say(t, d, "no, the dog chased the rabbit").Say; got != "I see. The dog chased the rabbit." {
		t.Fatalf("a correction should be acknowledged and accepted: %q", got)
	}
	if got := say(t, d, "did the dog chase the cat").Say; !strings.HasPrefix(got, "No") {
		t.Fatalf("the corrected-away fact should no longer hold: %q", got)
	}
	if got := say(t, d, "what did the dog chase").Say; !strings.Contains(got, "rabbit") || strings.Contains(got, "cat") {
		t.Fatalf("only the corrected fact should stand: %q", got)
	}
}

// A correction re-asks the last question of the right referent: "no, I meant the
// cat" after "what did the dog chase" answers about the cat, the relation inherited
// from the thread and only the subject repaired.
func TestRepairCorrectsReferent(t *testing.T) {
	d, _ := testDaemon(t)
	groundNoun(t, d, "dog")
	groundNoun(t, d, "cat")
	groundNoun(t, d, "mouse")
	say(t, d, "a dog chased a cat")
	say(t, d, "a cat chased a mouse")
	if got := say(t, d, "what did the dog chase").Say; !strings.Contains(got, "cat") {
		t.Fatalf("the role question should answer about the dog: %q", got)
	}
	if got := say(t, d, "no, I meant the cat").Say; got != "The cat chased the mouse." {
		t.Fatalf("a correction should re-ask the question of the right referent: %q", got)
	}
}

// The describe move is recognized off the speech-act verb rather than from a phrase list.
// "describe" derives the move and reports its patient's activity; the subject is read off the
// frame the way a clarify reads its own. The Turkish describe verb derives the same move with
// a case-marked object, so the move fires cross-tongue. It is the first dialogue move that does.
func TestDescribeRoutesOffSpeechActVerb(t *testing.T) {
	d, _ := testDaemon(t)
	for _, n := range []string{"dog", "cat"} {
		groundNoun(t, d, n)
	}
	say(t, d, "a dog chased a cat")

	// The describe verb produces the describe move end to end. The subject is the parsed
	// patient; no phrase matcher is consulted. "tell me about X" is exercised end to end by
	// TestDescribeFocusReportsIdentityAndFacts.
	if got := say(t, d, "describe the dog").Say; !strings.Contains(got, "chased") || !strings.Contains(got, "cat") {
		t.Fatalf("describe the dog should report the chase: %q", got)
	}
	// The move is recognized off the verb, in any tongue it is seeded in.
	if mv := d.gram.ParseHint("describe", "en", d.resolveConcept).Speech; mv != "describe" {
		t.Fatalf("the describe verb should derive the describe move: %q", mv)
	}
	if mv := d.gram.ParseHint("anlat", "tr", d.resolveConcept).Speech; mv != "describe" {
		t.Fatalf("the Turkish describe verb should derive the describe move: %q", mv)
	}

	// Cross-tongue: the Turkish describe verb with a case-marked object ("köpeği anlat")
	// describes the dog. The accusative is resolved to the bare concept off the frame. The
	// assertion is that the move fires (a non-empty identity line); the fact wording rides the
	// Turkish generation path tested elsewhere.
	for _, n := range []string{"köpek", "kedi"} {
		groundNoun(t, d, n)
	}
	say(t, d, "köpek kediyi kovaladı")
	if got := say(t, d, "köpeği anlat").Say; !strings.Contains(got, "Köpek") {
		t.Fatalf("köpeği anlat should describe the dog in Turkish: %q", got)
	}
}

func TestSpeechMoveDerivedFromGraph(t *testing.T) {
	d, s := testDaemon(t)
	if f := d.gram.ParseHint("explain", "en", d.resolveConcept); f.Speech != "clarify" {
		t.Fatalf("a seeded speech verb should derive clarify from the graph: %q", f.Speech)
	}
	var recallID uint64
	for id, name := range d.refinements {
		if name == "recall" {
			recallID = id
		}
	}
	if recallID == 0 {
		t.Fatal("the recall move concept should exist after boot")
	}
	// A fresh closed-class speech word carries no seeded move; its move is whatever
	// it is linked to in the graph.
	fw := &memory.Fragment{Kind: memory.KindGrammar, Channel: "grammar", Content: "ravel",
		Template: "fw:en:ravel", Lang: "en", Category: "speech", Time: time.Now()}
	if _, err := s.Encode(fw); err != nil {
		t.Fatal(err)
	}
	if err := s.LinkRel(fw.ID, recallID, memory.RelMarks, 1); err != nil {
		t.Fatal(err)
	}
	if err := d.recompileGrammar(); err != nil {
		t.Fatal(err)
	}
	if f := d.gram.ParseHint("ravel", "en", d.resolveConcept); f.Speech != "recall" {
		t.Fatalf("a word linked to the recall move should derive recall, not %q", f.Speech)
	}
}

// The clarify and recall moves derive from the speech-act verb in every tongue they are
// seeded in, so a Turkish or Latin conversation reaches them the same way English does. The
// move is one concept; each tongue contributes a verb that links to it.
func TestClarifyRecallCrossTongue(t *testing.T) {
	d, _ := testDaemon(t)
	for _, c := range []struct{ word, lang, move string }{
		{"açıkla", "tr", "clarify"}, {"explica", "la", "clarify"}, {"quid significas", "la", "clarify"},
		{"söyle", "tr", "recall"}, {"ne söyledim", "tr", "recall"}, {"dixi", "la", "recall"},
	} {
		if mv := d.gram.ParseHint(c.word, c.lang, d.resolveConcept).Speech; mv != c.move {
			t.Errorf("%q (%s) should derive the %s move, got %q", c.word, c.lang, c.move, mv)
		}
	}
}

// A deictic pronoun resolves to the participant it is linked to in the graph, not
// a table in code: a seeded pronoun resolves through its link, and a fresh deixis
// word linked to a participant resolves to it once the index reloads.
func TestDeicticResolvesFromGraph(t *testing.T) {
	d, s := testDaemon(t)
	if id := d.deicticParticipant("you"); id != d.selfID {
		t.Fatalf("you should resolve to GOLEM through its graph link, got %d", id)
	}
	if id := d.deicticParticipant("i"); id != d.userID {
		t.Fatalf("i should resolve to the speaker through its graph link, got %d", id)
	}
	// A fresh closed-class deixis word linked to GOLEM resolves to it, learned from
	// the graph rather than added to a table.
	fw := &memory.Fragment{Kind: memory.KindGrammar, Channel: "grammar", Content: "yourself",
		Template: "fw:en:yourself", Lang: "en", Category: "deixis", Time: time.Now()}
	if _, err := s.Encode(fw); err != nil {
		t.Fatal(err)
	}
	if err := s.LinkRel(fw.ID, d.selfID, memory.RelMarks, 1); err != nil {
		t.Fatal(err)
	}
	d.loadDeicticForms()
	if id := d.deicticParticipant("yourself"); id != d.selfID {
		t.Fatalf("a word linked to GOLEM should resolve to it from the graph, got %d", id)
	}
}

// "What do you mean" elaborates the subject in mind rather than reading as a fresh
// question, so a turn can ask GOLEM to say more about what it just said.
func TestClarifyElaboratesTopic(t *testing.T) {
	d, _ := testDaemon(t)
	groundNoun(t, d, "dog")
	groundNoun(t, d, "cat")
	say(t, d, "a dog chased a cat")
	say(t, d, "what did the dog chase") // the dog is now the subject in mind
	got := strings.ToLower(say(t, d, "what do you mean").Say)
	if !strings.Contains(got, "dog") || !strings.Contains(got, "thing") {
		t.Fatalf("clarify should elaborate the subject in mind: %q", got)
	}
}

// A question GOLEM cannot answer is held open, and a later telling that fills the
// gap is followed up with the answer rather than received in silence, so the talk
// connects what was asked to what it learns.
func TestOpenQuestionFollowedUp(t *testing.T) {
	d, _ := testDaemon(t)
	groundNoun(t, d, "dog")
	groundNoun(t, d, "cat")
	groundNoun(t, d, "spoon")
	say(t, d, "a dog chased a cat") // establishes the chase relation
	// The spoon resembles nothing that chased, so the question has no answer yet.
	say(t, d, "what did the spoon chase")
	if len(d.openQuestions) == 0 {
		t.Fatal("a question GOLEM could not answer should be held open")
	}
	if got := say(t, d, "a spoon chased a cat").Say; got != "Now I know the spoon chased the cat." {
		t.Fatalf("a telling that fills the gap should be followed up: %q", got)
	}
	if len(d.openQuestions) != 0 {
		t.Fatal("the answered question should no longer be open")
	}
	// A telling that answers nothing open earns the plain acknowledgment.
	if got := say(t, d, "a cat chased a mouse").Say; got != "I see." {
		t.Fatalf("an ordinary telling should be acknowledged: %q", got)
	}
}

// The topic follows the subject a turn raised and keeps it across a turn that
// raises none, and a displaced subject is kept on the trail to return to: the
// memory a conversation holds so it stays on a thread and can step back to one.
func TestTopicTracksFocusAndTrails(t *testing.T) {
	d, _ := testDaemon(t)
	d.turnFocus = 10
	d.trackTopic()
	if d.topic != 10 {
		t.Fatalf("the topic should follow the subject raised: %d", d.topic)
	}
	d.turnFocus = 20
	d.trackTopic()
	if d.topic != 20 || len(d.topicTrail) != 1 || d.topicTrail[0] != 10 {
		t.Fatalf("a new subject takes the field and trails the old: topic=%d trail=%v", d.topic, d.topicTrail)
	}
	d.turnFocus = 0
	d.trackTopic()
	if d.topic != 20 {
		t.Fatalf("a turn that raised no subject keeps the one in play: %d", d.topic)
	}
	d.turnFocus = 10
	d.trackTopic()
	if d.topic != 10 || len(d.topicTrail) != 1 || d.topicTrail[0] != 20 {
		t.Fatalf("returning should re-take 10 and trail 20: topic=%d trail=%v", d.topic, d.topicTrail)
	}
}

// "What have I told you" reads back the propositions the exchange established, the
// discourse memory rather than the entrenched graph, so a conversation can be
// asked what it has covered.
func TestRecallSaidReportsEstablished(t *testing.T) {
	d, _ := testDaemon(t)
	groundNoun(t, d, "dog")
	groundNoun(t, d, "cat")
	groundNoun(t, d, "mouse")
	say(t, d, "a dog chased a cat")
	say(t, d, "a cat chased a mouse")
	// The real utterance routes through the speech-act verb: "told" carries the
	// recall move, no phrase list in between.
	got := say(t, d, "what have I told you").Say
	if !strings.Contains(got, "dog") || !strings.Contains(got, "cat") || !strings.Contains(got, "mouse") {
		t.Fatalf("recall should report what the exchange established: %q", got)
	}
	// With nothing established, the move declines so the older path can answer.
	frame := grammar.Frame{Lang: "en", Speech: "recall", Force: grammar.ForceQuestion}
	d.threadProps = nil
	if _, ok := d.planRecallSaid(frame); ok {
		t.Fatal("with nothing established, recall should decline")
	}
}

// A basic word that has kept enough varied company can grow an "abs:en:cat"
// shadow of its own percept; that shadow then outweighs the percept in the
// naming vote and every clause about the cat mis-resolves. The shadow is a
// basic-level abstraction (its lemma heads a percept's caption), unlike a true
// superordinate, so it is set aside and the noun resolves to the thing it names.
func TestResolveConceptPrefersPerceptOverBasicLevelShadow(t *testing.T) {
	d, s := testDaemon(t)
	mkConcept := func(template, content string) *memory.Fragment {
		c := &memory.Fragment{Kind: memory.KindConcept, Channel: "vision", Content: content,
			Template: template, Limbic: d.chem.Snapshot(), Time: time.Now()}
		if _, err := s.Encode(c); err != nil {
			t.Fatal(err)
		}
		return c
	}
	percept := mkConcept("a small black cat", "a small black cat")
	catShadow := mkConcept("abs:en:cat", "cat") // the spurious same-lemma abstraction
	animal := mkConcept("abs:en:animal", "animal")
	catW, _, err := s.WordNode("cat", "en", d.chem.Snapshot(), time.Now())
	if err != nil {
		t.Fatal(err)
	}
	animalW, _, err := s.WordNode("animal", "en", d.chem.Snapshot(), time.Now())
	if err != nil {
		t.Fatal(err)
	}
	// "cat" names the percept faintly (0.24) but its shadow firmly (0.8): the
	// bug is the shadow winning the vote. "animal" names its own abstraction,
	// which heads no percept's caption.
	s.SetLinkRel(catW.ID, percept.ID, memory.RelNames, 0.24)
	s.SetLinkRel(catW.ID, catShadow.ID, memory.RelNames, 0.8)
	s.SetLinkRel(animalW.ID, animal.ID, memory.RelNames, 0.8)
	s.SetLinkRel(animalW.ID, percept.ID, memory.RelNames, 0.21) // animal names the cat percept by inheritance

	if got := d.resolveConcept("cat"); got == nil || got.ID != percept.ID {
		t.Fatalf("a basic noun must resolve to its percept, not a same-lemma shadow: %+v", got)
	}
	if got := d.resolveConcept("animal"); got == nil || got.ID != animal.ID {
		t.Fatalf("a true superordinate (heads no caption) must still resolve to its abstraction: %+v", got)
	}
}

// A colour word that also names a real thing (the fruit "orange") must resolve to
// the thing, not to the perceptual colour cell the word innately names. The cell
// edge is immortal (links into a class are decay-spared and re-seeded at boot),
// while the fruit's name decays after its episodes fade, so late in a long grow the
// cell edge out-votes the fruit unless it is set aside. A bare colour word that names
// only its cell must still resolve to the cell, through the fallback.
func TestResolveConceptPrefersThingOverColourCell(t *testing.T) {
	d, s := testDaemon(t)
	mkConcept := func(template, content string) *memory.Fragment {
		c := &memory.Fragment{Kind: memory.KindConcept, Channel: "vision", Content: content,
			Template: template, Limbic: d.chem.Snapshot(), Time: time.Now()}
		if _, err := s.Encode(c); err != nil {
			t.Fatal(err)
		}
		return c
	}
	fruit := mkConcept("a bright dimpled orange", "a bright dimpled orange")
	cell := mkConcept("percept:col:orange", "col:orange") // the innate colour sense-cell
	if d.colourCells == nil {
		d.colourCells = map[uint64]bool{}
	}
	d.colourCells[cell.ID] = true
	orangeW, _, err := s.WordNode("orange", "en", d.chem.Snapshot(), time.Now())
	if err != nil {
		t.Fatal(err)
	}
	// The immortal cell edge has ratcheted above the fruit's decaying name: the exact
	// late-grow hijack. Without the set-aside the perceptless cell wins the vote.
	s.SetLinkRel(orangeW.ID, fruit.ID, memory.RelNames, 0.72)
	s.SetLinkRel(orangeW.ID, cell.ID, memory.RelNames, 0.85)
	if got := d.resolveConcept("orange"); got == nil || got.ID != fruit.ID {
		t.Fatalf("the fruit noun must win over its perceptless colour cell: %+v", got)
	}

	// A colour word naming only its cell must still resolve to the cell (fallback).
	redCell := mkConcept("percept:col:red", "col:red")
	d.colourCells[redCell.ID] = true
	redW, _, err := s.WordNode("red", "en", d.chem.Snapshot(), time.Now())
	if err != nil {
		t.Fatal(err)
	}
	s.SetLinkRel(redW.ID, redCell.ID, memory.RelNames, 0.5)
	if got := d.resolveConcept("red"); got == nil || got.ID != redCell.ID {
		t.Fatalf("a bare colour word must still resolve to its cell via fallback: %+v", got)
	}
}

// A phrase read in a tongue resolves through that tongue's word node: a spurious
// cross-language shadow of the same form (an English "kedi" naming a wren, minted when an
// untagged Turkish clause misdetects) must not pool its votes with the real Turkish "kedi"
// and win the resolution. With no tongue given, both nodes still pool (the prior behavior).
func TestResolveConceptPrefersTongue(t *testing.T) {
	d, s := testDaemon(t)
	mk := func(content string) *memory.Fragment {
		c := &memory.Fragment{Kind: memory.KindConcept, Channel: "vision", Content: content,
			Template: content, Limbic: d.chem.Snapshot(), Time: time.Now()}
		if _, err := s.Encode(c); err != nil {
			t.Fatal(err)
		}
		return c
	}
	cat := mk("a small black cat")
	wren := mk("a small gray wren")
	name := func(form, lang string, cid uint64, w float64) {
		wn, _, err := s.WordNode(form, lang, d.chem.Snapshot(), time.Now())
		if err != nil {
			t.Fatal(err)
		}
		if err := s.SetLinkRel(wn.ID, cid, memory.RelNames, w); err != nil {
			t.Fatal(err)
		}
	}
	// The real Turkish "kedi" names the cat; a spurious English shadow names the wren, at
	// the same weight, with the wren's higher id so it would win the tongue-blind tie.
	name("kedi", "tr", cat.ID, 0.784)
	name("kedi", "en", wren.ID, 0.784)

	if got := d.resolveConceptLang("kedi", "tr"); got == nil || got.ID != cat.ID {
		t.Fatalf("read in Turkish, kedi must resolve to the cat, not the English shadow's wren: %+v", got)
	}
	if got := d.resolveConceptLang("kedi", "en"); got == nil || got.ID != wren.ID {
		t.Fatalf("read in English, kedi must resolve through the English node: %+v", got)
	}
	// Tongue-blind, the higher-id shadow still wins the tie: the prior behavior is unchanged.
	if got := d.resolveConcept("kedi"); got == nil || got.ID != wren.ID {
		t.Fatalf("with no tongue, resolution pools both nodes as before: %+v", got)
	}
}

// A question lays no naming company. Asked something that mentions a novel word while a
// percept lingers warm from just being seen, the word must not bond to that percept, a bond
// the night would read as a name. Only a declarative telling names, so an untagged question
// misread in another tongue cannot mint a spurious name onto whatever was in view.
func TestQuestionLaysNoNamingCompany(t *testing.T) {
	d, s := testDaemon(t)
	wren := &memory.Fragment{Kind: memory.KindEpisodic, Channel: "vision",
		Content: "a small gray wren", Template: "a small gray wren", Time: time.Now()}
	if _, err := s.Encode(wren); err != nil {
		t.Fatal(err)
	}
	// The percept was just seen, so it sits in the naming-recency window a naming reads.
	d.recent = append(d.recent, recentMoment{id: wren.ID, channel: "vision", at: time.Now()})

	say(t, d, "does a zosk chase the wren")

	words, err := s.WordNodesByForms([]string{"zosk"})
	if err != nil {
		t.Fatal(err)
	}
	for _, w := range words {
		links, _ := s.LinksFromRel([]uint64{w.ID}, memory.RelTemporal)
		for _, e := range links {
			if e.To == wren.ID {
				t.Fatal("a question's word must not keep company with a warm percept it merely mentioned near")
			}
		}
	}
}

// Role-fit reads animacy from resemblance, never a concept's own role-count: a
// concept that has only ever been a patient must not be locked into that role,
// because counting its history fed back, a thing once chased reading as never a
// chaser and unseating a clear subject ("a cat chased a mouse" written reversed).
func TestRoleFitReadsAnimacyNotRoleCount(t *testing.T) {
	d, s := testDaemon(t)
	c := groundNoun(t, d, "cat")
	// Three moments in which the cat stood as patient, but it resembles nothing.
	for i := 0; i < 3; i++ {
		mom := &memory.Fragment{Kind: memory.KindEpisodic, Channel: "tell",
			Content: "a dog chased a cat", Template: "chased " + string(rune('a'+i)),
			Limbic: d.chem.Snapshot(), Time: time.Now()}
		if _, err := s.Encode(mom); err != nil {
			t.Fatal(err)
		}
		if err := s.SetLinkRel(mom.ID, c.ID, memory.RelPatient, 0.8); err != nil {
			t.Fatal(err)
		}
	}
	if got := d.roleFit(c, "patient"); got != 0 {
		t.Fatalf("role-fit must not count a concept's own role history (that fed the swap loop): %v", got)
	}
}

// Transitivity is a property a relation is TOLD it has, not one a construction
// gives. Told an ordering chain on an ordinary verb and that the verb is
// transitive, GOLEM chains it: the cat outranked the dog and the dog the mouse
// answer the cat outranked the mouse, a step it was never told. Before the rule
// the chain is not asserted firmly; the property is read back like any fact.
func TestRelationTransitivityTold(t *testing.T) {
	d, _ := testDaemon(t)
	groundNoun(t, d, "cat")
	groundNoun(t, d, "dog")
	groundNoun(t, d, "mouse")
	say(t, d, "the cat outranked the dog")
	say(t, d, "the dog outranked the mouse")
	// Without the rule, the indirect step is not asserted as a fact.
	if got := say(t, d, "did the cat outrank the mouse").Say; strings.HasPrefix(got, "Yes") {
		t.Fatalf("without the transitive rule the chain should not be asserted firmly: %q", got)
	}
	// Tell the rule; it is learned and confirmed.
	if got := say(t, d, "outrank is transitive").Say; got != "I see. Outrank is transitive." {
		t.Fatalf("the property should be learned and confirmed: %q", got)
	}
	// Now the chain holds, firm (a sound deduction), not hedged.
	if got := say(t, d, "did the cat outrank the mouse").Say; got != "Yes, the cat outranked the mouse." {
		t.Fatalf("a transitive relation should chain its told orderings firmly: %q", got)
	}
	// The order is asymmetric: the reverse of a told ordering does not hold.
	if got := say(t, d, "did the mouse outrank the cat").Say; strings.HasPrefix(got, "Yes") {
		t.Fatalf("the reverse of a told ordering should not hold: %q", got)
	}
	// The learned property reads back.
	if got := say(t, d, "is outrank transitive").Say; got != "Yes." {
		t.Fatalf("the learned property should read back: %q", got)
	}
}

// Transitivity is a property of the particular relation, not a blanket
// assumption: told outrank is transitive but never told it of chase, GOLEM
// chains outrank and does not chain chase, even with an identical ordering told
// on each. So the chaining is learned per relation, not hardcoded.
func TestTransitivityIsPerRelation(t *testing.T) {
	d, _ := testDaemon(t)
	groundNoun(t, d, "cat")
	groundNoun(t, d, "dog")
	groundNoun(t, d, "mouse")
	say(t, d, "the cat outranked the dog")
	say(t, d, "the dog outranked the mouse")
	say(t, d, "the cat chased the dog")
	say(t, d, "the dog chased the mouse")
	say(t, d, "outrank is transitive")
	if got := say(t, d, "did the cat outrank the mouse").Say; got != "Yes, the cat outranked the mouse." {
		t.Fatalf("the relation told transitive should chain: %q", got)
	}
	// Chase was never told transitive, so its identical chain does not chain firmly.
	if got := say(t, d, "did the cat chase the mouse").Say; strings.HasPrefix(got, "Yes") {
		t.Fatalf("a relation not told transitive should not chain: %q", got)
	}
}

// A property predicated of a thing GOLEM knows no relation for is an honest gap,
// not a silent no-op: there is no relation to mark, so it admits it.
func TestPropertyOfUnknownRelationAdmitsGap(t *testing.T) {
	d, _ := testDaemon(t)
	groundNoun(t, d, "cat")
	if got := say(t, d, "zorp is transitive").Say; !strings.Contains(got, "know") {
		t.Fatalf("a property of an unknown relation should admit the gap: %q", got)
	}
}

// An abstract noun binds across tongues through the relation, not through a shared
// percept it can never have. "a model learns a pattern" then "model deseni öğrendi"
// back to back binds öğren to learn and, on that warm relation, desen to the pattern
// concept, so the object told in one tongue is the same thing in the other rather
// than a parallel concept, and a fact reads back in either tongue.
func TestCrossTongueAbstractNounBinds(t *testing.T) {
	d, _ := testDaemon(t)
	say(t, d, "a model learns a pattern")
	sayLang(t, d, "tr", "model deseni öğrendi") // back to back: the bi_rel adjacency
	id := func(n string) uint64 {
		if c := d.resolveConcept(n); c != nil {
			return c.ID
		}
		return 0
	}
	if id("pattern") == 0 || id("pattern") != id("desen") {
		t.Fatalf("desen should bind to the pattern concept, not mint its own: pattern=%d desen=%d", id("pattern"), id("desen"))
	}
	if id("learn") == 0 || id("learn") != id("öğren") {
		t.Fatalf("öğren should bind to the learn relation: learn=%d öğren=%d", id("learn"), id("öğren"))
	}
	if got := sayLang(t, d, "tr", "model ne öğrendi").Say; !strings.Contains(got, "desen") {
		t.Errorf("the TR query should answer with the bound TR noun: %q", got)
	}
}

// The cross-tongue object twin is found by the whole proposition (agent + relation),
// not the relation alone. With the cat's chase of the mouse the most recent, the dog's
// twin must still be what the DOG chased, and an agent that chased nothing in the
// thread has no twin (so its object mints fresh rather than fusing onto another's).
func TestCrossTonguePatientMatchesAgent(t *testing.T) {
	d, s := testDaemon(t)
	dog := groundNoun(t, d, "dog")
	cat := groundNoun(t, d, "cat")
	mouse := groundNoun(t, d, "mouse")
	deer := groundNoun(t, d, "deer")
	chase := &memory.Fragment{Kind: memory.KindConcept, Channel: "concept", Content: "chase",
		Template: "rel:en:chase", Category: "relation", Limbic: d.chem.Snapshot(), Time: time.Now()}
	if _, err := s.Encode(chase); err != nil {
		t.Fatal(err)
	}
	// Newest first: the cat's chase of the mouse is more recent than the dog's of the deer.
	d.threadProps = []threadProp{
		{rel: "chase", agent: cat.ID, patient: mouse.ID},
		{rel: "chase", agent: dog.ID, patient: deer.ID},
	}
	if got := d.crossTonguePatient(chase, dog, "tr"); got == nil || got.ID != deer.ID {
		t.Fatalf("the dog's twin should be the deer it chased, not the more recent mouse: %v", got)
	}
	if got := d.crossTonguePatient(chase, cat, "tr"); got == nil || got.ID != mouse.ID {
		t.Fatalf("the cat's twin should be the mouse it chased: %v", got)
	}
	if got := d.crossTonguePatient(chase, mouse, "tr"); got != nil {
		t.Fatalf("an agent that chased nothing in the thread should have no twin: %v", got)
	}
}

// A verb new to a tongue binds to the warm relation only when that relation was
// FRESHLY attended (genuinely just spoken of), not when it merely lingers warm from
// an earlier scene; otherwise a stale warm relation would fuse a brand-new verb onto
// the wrong one. A relation only mulled (warmed internally, never attended) is not
// offered, so the new verb mints its own relation and reunifies later.
func TestWarmestRelationNeedsFreshAttention(t *testing.T) {
	d, s := testDaemon(t)
	chase := &memory.Fragment{Kind: memory.KindConcept, Channel: "concept", Content: "chase",
		Template: "rel:en:chase", Category: "relation", Limbic: d.chem.Snapshot(), Time: time.Now()}
	if _, err := s.Encode(chase); err != nil {
		t.Fatal(err)
	}
	d.ws.mull(chase.ID, 1.0) // warm, but not freshly attended
	if got := d.warmestRelation(); got != nil {
		t.Fatalf("a relation only internally warm must not be offered to a new tongue's verb: %v", got)
	}
	d.ws.excite(chase.ID, 1.0) // freshly attended now
	if got := d.warmestRelation(); got == nil || got.ID != chase.ID {
		t.Fatalf("a freshly attended relation should be offered: %v", got)
	}
}

// "What is part of the table" names the parts stored against the whole, rendered
// with the part-of phrasing rather than the relation conjugated as a verb, and
// firm because the part-whole fact is grounded.
func TestPartWholeWhNamesParts(t *testing.T) {
	d, _ := testDaemon(t)
	groundNoun(t, d, "wood")
	groundNoun(t, d, "table")
	say(t, d, "wood is part of a table")
	if got := say(t, d, "what is part of the table").Say; got != "The wood is part of the table." {
		t.Fatalf("what-is-part-of should name the parts firmly: %q", got)
	}
	// An unknown whole is admitted, not answered as a verb clause.
	groundNoun(t, d, "cloud")
	if got := say(t, d, "what is part of the cloud").Say; !strings.Contains(strings.ToLower(got), "know") {
		t.Fatalf("a whole with no parts should admit the gap: %q", got)
	}
}
