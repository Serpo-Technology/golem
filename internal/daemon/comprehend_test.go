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

// linksTo reports whether any edge points at the given target.
func linksTo(edges []memory.LinkEdge, to uint64) bool {
	for _, e := range edges {
		if e.To == to {
			return true
		}
	}
	return false
}
