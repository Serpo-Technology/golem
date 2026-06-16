package daemon

import (
	"strings"
	"testing"
	"time"

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
// bound, and the brain does not blurt a confused question about it.
func TestComprehendUnknownVerbDegradesGracefully(t *testing.T) {
	d, s := testDaemon(t)
	dog := groundNoun(t, d, "dog")
	cat := groundNoun(t, d, "cat")

	resp := say(t, d, "a dog blorped a cat")
	if resp.Say != "" {
		t.Fatalf("an understood novel statement should not blurt: %q", resp.Say)
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

// A partial utterance yields a partial meaning and a learning signal: the
// grounded part is written, and the role whose filler grounds in nothing is
// held as a goal rather than dropped.
func TestComprehendPartialHoldsTheGap(t *testing.T) {
	d, s := testDaemon(t)
	dog := groundNoun(t, d, "dog")
	d.chem.SpikeDopamine(0.6) // curious enough to hold the gap

	resp := say(t, d, "a dog chased a wug")
	moment := resp.Observe.FragmentID

	chase, _ := s.ConceptByTemplate("rel:en:chase")
	if chase == nil {
		t.Fatal("the grounded part of the proposition should still be written")
	}
	ag, _ := s.LinksFromRel([]uint64{moment}, memory.RelAgent)
	if !linksTo(ag, dog.ID) {
		t.Fatal("the dog should be the agent of the partial proposition")
	}
	if d.goal == nil || d.goal.topic != "wug" {
		t.Fatalf("the unknown patient should be held as a learning goal: %+v", d.goal)
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

	if got := say(t, d, "a dog chased a cat").Say; got != "" {
		t.Fatalf("a comprehended statement teaches silently: %q", got)
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
	if got := say(t, d, "did the coyote chase the cat").Say; got != "No, the coyote did not chase the cat." {
		t.Fatalf("after release the supposition should no longer bear: %q", got)
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
	// never mistaken for something observed.
	d.held.clear()
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

// A wh-question GOLEM holds no observed answer for it answers from inference,
// hedged, the counterpart of the polar guess and symmetric across the role
// asked. The wolf was never seen chasing anything, but it resembles the dog,
// which chased the cat, so "what did the wolf chase" guesses the cat; and the
// mouse resembles the cat, which the dog chased, so "who chased the mouse"
// guesses the dog. The resemblance proposes the neighbour, its co-argument is
// spoken back under the asked concept's name, and the hedge marks the guess. An
// observed role still answers plainly.
func TestRoleQuestionInfersFromResemblance(t *testing.T) {
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
	if got := say(t, d, "what did the wolf chase").Say; got != "Probably, the wolf chased the cat." {
		t.Fatalf("agent-side inference should be a hedged guess: %q", got)
	}
	if got := say(t, d, "who chased the mouse").Say; got != "Probably, the dog chased the mouse." {
		t.Fatalf("patient-side inference should be a hedged guess: %q", got)
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
