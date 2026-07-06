package daemon

import (
	"strings"
	"testing"
	"time"

	"golem/internal/grammar"
	"golem/internal/memory"
)

// groundLatinPair grounds one concept named in both English and Latin, so a clause
// in either tongue resolves to the same referent.
func groundLatinPair(t *testing.T, d *Daemon, gloss, en, la string) *memory.Fragment {
	t.Helper()
	now := time.Now()
	c := &memory.Fragment{Kind: memory.KindConcept, Channel: "concept", Content: gloss,
		Template: gloss, Category: "noun", Limbic: d.chem.Snapshot(), Time: now}
	if _, err := d.store.Encode(c); err != nil {
		t.Fatal(err)
	}
	for _, p := range []struct{ w, lang string }{{en, "en"}, {la, "la"}} {
		w, _, err := d.store.WordNode(p.w, p.lang, d.chem.Snapshot(), now)
		if err != nil {
			t.Fatal(err)
		}
		if err := d.store.SetLinkRel(w.ID, c.ID, memory.RelNames, 0.9); err != nil {
			t.Fatal(err)
		}
	}
	return c
}

func hasProp(d *Daemon, agent *memory.Fragment, rel string, patient *memory.Fragment) bool {
	for _, f := range d.gatherFacts(agent, "agent", "") {
		if f.rel == rel && f.coID == patient.ID {
			return true
		}
	}
	return false
}

// Latin marks roles by case, so word order is free: every permutation of "puella
// rosam amat" (the girl loves the rose) yields the one proposition, the nominative
// puella the agent and the accusative rosam the patient wherever they stand.
func TestLatinFreeWordOrder(t *testing.T) {
	orders := []string{
		"puella rosam amat", "rosam puella amat", "amat puella rosam",
		"rosam amat puella", "puella amat rosam", "amat rosam puella",
	}
	for _, clause := range orders {
		d, _ := testDaemon(t)
		puella := groundNounLang(t, d, "puella", "la")
		rosa := groundNounLang(t, d, "rosa", "la")
		sayLang(t, d, "la", clause)
		if !hasProp(d, puella, "amat", rosa) {
			t.Errorf("%q should comprehend as girl-loves-rose", clause)
		}
		if hasProp(d, rosa, "amat", puella) {
			t.Errorf("%q wrongly made the rose the agent", clause)
		}
	}
}

// The construction-traversal comprehender reads Latin's case-bound roles the way the coded path
// does: across any word order the nominative puella is the agent and the accusative rosam the
// patient. The traversal walks the construction by case here, not position, the same in either
// direction.
func TestComprehendByTraversalReadsLatinCase(t *testing.T) {
	d, _ := testDaemon(t)
	puella := groundNounLang(t, d, "puella", "la")
	rosa := groundNounLang(t, d, "rosa", "la")
	sayLang(t, d, "la", "puella rosam amat") // establishes the amat relation

	for _, clause := range []string{"puella rosam amat", "rosam puella amat", "amat rosam puella"} {
		coded, ok1 := d.gram.Comprehend("la", clause, d.resolveConcept, d.roleFit)
		walked, ok2 := d.gram.ComprehendByTraversal("la", clause, d.resolveConcept, d.roleFit)
		if !ok1 || !ok2 {
			t.Fatalf("%q: both paths should read it: coded=%v walked=%v", clause, ok1, ok2)
		}
		if !meaningsAgree(walked, coded) {
			t.Fatalf("%q: traversal should match the coded reading: walked %+v vs coded %+v", clause, walked, coded)
		}
		if a := walked.Roles["agent"].Concept; a == nil || a.ID != puella.ID {
			t.Fatalf("%q: the nominative puella should be the agent: %+v", clause, walked.Roles["agent"])
		}
		if p := walked.Roles["patient"].Concept; p == nil || p.ID != rosa.ID {
			t.Fatalf("%q: the accusative rosam should be the patient: %+v", clause, walked.Roles["patient"])
		}
	}
}

// The roles come from the case, not the order: swapping which noun wears the
// accusative reverses who loves whom even at the same word order.
func TestLatinCaseAssignsRoles(t *testing.T) {
	d, _ := testDaemon(t)
	puella := groundNounLang(t, d, "puella", "la")
	rosa := groundNounLang(t, d, "rosa", "la")
	sayLang(t, d, "la", "rosa puellam amat") // the rose loves the girl
	if !hasProp(d, rosa, "amat", puella) {
		t.Errorf("the nominative rosa should be the agent")
	}
	if hasProp(d, puella, "amat", rosa) {
		t.Errorf("the accusative puellam should not be the agent")
	}
}

// GOLEM speaks Latin back, casing the nouns and answering a -ne question: the
// agent in the nominative bare stem, the patient in the accusative, the verb last.
// A free-order question reaches the same proposition and answers in the unmarked
// order, and the case decides a denial.
func TestLatinSpeaksByCase(t *testing.T) {
	d, _ := testDaemon(t)
	groundNounLang(t, d, "puella", "la")
	groundNounLang(t, d, "rosa", "la")
	sayLang(t, d, "la", "puella rosam amat")

	if got := sayLang(t, d, "la", "amatne puella rosam?").Say; got != "Ita, puella rosam amat." {
		t.Errorf("Latin should affirm in the unmarked order: %q", got)
	}
	if got := sayLang(t, d, "la", "rosamne puella amat?").Say; got != "Ita, puella rosam amat." {
		t.Errorf("a free-order question should answer the same: %q", got)
	}
	if got := sayLang(t, d, "la", "amatne rosa puellam?").Say; got != "Minime, rosa puellam non amat." {
		t.Errorf("the case should deny rose-loves-girl with a clause: %q", got)
	}
}

// Latin compares with a comparative adjective and quam ("rosa maior quam viola", the
// rose is bigger than the violet), ordered on the shared scale so the told orderings
// chain transitively the way the English comparison does.
func TestLatinComparison(t *testing.T) {
	d, _ := testDaemon(t)
	groundNounLang(t, d, "rosa", "la")
	groundNounLang(t, d, "viola", "la")
	groundNounLang(t, d, "lilia", "la")
	sayLang(t, d, "la", "rosa maior quam viola")
	sayLang(t, d, "la", "viola maior quam lilia")

	if got := sayLang(t, d, "la", "estne rosa maior quam viola?").Say; got != "Ita, rosa maior quam viola." {
		t.Errorf("a told comparison should affirm: %q", got)
	}
	if got := sayLang(t, d, "la", "estne viola maior quam rosa?").Say; got != "Minime." {
		t.Errorf("the reverse ordering should be denied: %q", got)
	}
	if got := sayLang(t, d, "la", "estne rosa maior quam lilia?").Say; got != "Ita, rosa maior quam lilia." {
		t.Errorf("the ordering should chain transitively: %q", got)
	}
}

// Latin possesses with habere, a transitive verb of the second conjugation ("puella
// rosam habet", the girl has the rose), which joins the English have relation, so a
// possession taught only in Latin answers the English question.
func TestLatinPossession(t *testing.T) {
	d, _ := testDaemon(t)
	groundLatinPair(t, d, "girl", "girl", "puella")
	groundLatinPair(t, d, "rose", "rose", "rosa")
	groundLatinPair(t, d, "water", "water", "aqua")
	say(t, d, "the girl has the rose")
	sayLang(t, d, "la", "puella rosam habet") // unify habet with have
	sayLang(t, d, "la", "puella aquam habet") // a possession said only in Latin
	if got := say(t, d, "does the girl have the water").Say; !strings.HasPrefix(got, "Yes") {
		t.Errorf("a Latin-taught possession should answer in English: %q", got)
	}
}

// Latin places the figure with a preposition over the ground ("rosa in mensa est"),
// and the preposition joins the same locative relation English uses, so "ubi est"
// reads the place back and a Latin place answers the English query.
func TestLatinSpatial(t *testing.T) {
	d, _ := testDaemon(t)
	groundNounLang(t, d, "rosa", "la")
	groundNounLang(t, d, "mensa", "la")
	sayLang(t, d, "la", "rosa in mensa est")
	if got := sayLang(t, d, "la", "rosa ubi est?").Say; got != "Rosa in mensa est." {
		t.Errorf("ubi should read the figure's place: %q", got)
	}
	groundLatinPair(t, d, "book", "book", "liber")
	groundLatinPair(t, d, "box", "box", "arca")
	sayLang(t, d, "la", "liber in arca est")
	if got := say(t, d, "is the book in the box").Say; !strings.HasPrefix(got, "Yes") {
		t.Errorf("a Latin-taught place should answer the English query: %q", got)
	}
}

// A Latin role question carries its interrogative in the case: quis asks the
// nominative agent, quem and quid the accusative patient. Each reads the asked role
// from the proposition graph and speaks the whole clause back.
func TestLatinWhQuestion(t *testing.T) {
	d, _ := testDaemon(t)
	groundNounLang(t, d, "puella", "la")
	groundNounLang(t, d, "rosa", "la")
	sayLang(t, d, "la", "puella rosam amat")

	if got := sayLang(t, d, "la", "quis rosam amat?").Say; got != "Puella rosam amat." {
		t.Errorf("quis (who) should name the agent: %q", got)
	}
	if got := sayLang(t, d, "la", "quem puella amat?").Say; got != "Puella rosam amat." {
		t.Errorf("quem (whom) should name the patient: %q", got)
	}
}

// Latin conjugates across persons and drops the subject: "rosam amo" (I love the
// rose) is recognized through the first-person ending and the speaker is recovered as
// the agent, where before only the third person -t was read.
func TestLatinConjugationProDrop(t *testing.T) {
	d, _ := testDaemon(t)
	rose := groundNounLang(t, d, "rosa", "la")
	sayLang(t, d, "la", "rosam amo") // (I) love the rose
	user := d.resolveConcept("ego")
	if user == nil {
		t.Fatal("the first-person pronoun should resolve to the speaker")
	}
	if !hasProp(d, user, "amat", rose) {
		t.Errorf("a pro-dropped first-person clause should make the speaker the agent")
	}
}

// Latin joins the cross-tongue substrate with no new machinery: the bilingual nouns
// resolve to one concept each, and the Latin verb binds to the warm English relation,
// so a fact taught only in Latin answers in English and the reverse holds too.
func TestLatinCrossTongue(t *testing.T) {
	d, _ := testDaemon(t)
	groundLatinPair(t, d, "girl", "girl", "puella")
	groundLatinPair(t, d, "rose", "rose", "rosa")
	// English establishes the relation; its Latin echo, back to back, binds amat to
	// the warm love relation so the two tongues share one relation.
	say(t, d, "the girl loves the rose")
	sayLang(t, d, "la", "puella rosam amat")

	// A fact taught ONLY in Latin answers in English, through the unified relation
	// and the bilingual nouns: the strong cross-tongue litmus.
	sayLang(t, d, "la", "rosa puellam amat") // rose loves girl, never said in English
	if got := say(t, d, "does the rose love the girl").Say; !strings.HasPrefix(got, "Yes") {
		t.Errorf("a Latin-only fact should answer in English: %q", got)
	}
	// And the English-taught fact answers back in Latin.
	if got := sayLang(t, d, "la", "amatne puella rosam?").Say; got != "Ita, puella rosam amat." {
		t.Errorf("the English-taught fact should answer in Latin: %q", got)
	}
}

// latinMember grounds a Latin-named concept under a Latin-named category, so the
// category is an abstraction the member grounds (categoriesOf/isA/groundedKindOf
// all read it), the shape a Latin definition and is-a answer over.
func latinMember(t *testing.T, d *Daemon, memberGloss, memberLA, catGloss, catLA string) (member, cat *memory.Fragment) {
	t.Helper()
	member = groundLatinPair(t, d, memberGloss, memberGloss, memberLA)
	// The category is a genuine abstraction (an abs: template) so groundedKindOf
	// and isA read it as a kind rather than a bare percept.
	now := time.Now()
	cat = &memory.Fragment{Kind: memory.KindConcept, Channel: "concept", Content: catGloss,
		Template: "abs:en:" + catGloss, Category: "noun", Limbic: d.chem.Snapshot(), Time: now}
	if _, err := d.store.Encode(cat); err != nil {
		t.Fatal(err)
	}
	for _, p := range []struct{ w, lang string }{{catGloss, "en"}, {catLA, "la"}} {
		w, _, err := d.store.WordNode(p.w, p.lang, d.chem.Snapshot(), now)
		if err != nil {
			t.Fatal(err)
		}
		if err := d.store.SetLinkRel(w.ID, cat.ID, memory.RelNames, 0.9); err != nil {
			t.Fatal(err)
		}
	}
	if err := d.store.SetLinkRel(cat.ID, member.ID, memory.RelGrounds, 0.8); err != nil {
		t.Fatal(err)
	}
	return member, cat
}

// "Quid est equus" answers with the kind the horse grounds under, spoken in Latin.
func TestLatinDefinitionAnswersKind(t *testing.T) {
	d, _ := testDaemon(t)
	latinMember(t, d, "a golden horse", "equus", "animal", "animal")
	if got := sayLang(t, d, "la", "quid est equus").Say; got != "Equus est animal." {
		t.Fatalf("a Latin what-is should define by the grounded kind: %q", got)
	}
	// An unknown noun is admitted, not guessed.
	if got := sayLang(t, d, "la", "quid est draco").Say; !strings.Contains(got, "Nescio") {
		t.Fatalf("an unknown Latin noun should be admitted: %q", got)
	}
}

// "Estne equus animal" checks membership and answers in Latin; a non-membership
// is denied, and a comparison is left to the comparison path.
func TestLatinIsAChecksMembership(t *testing.T) {
	d, _ := testDaemon(t)
	equus, _ := latinMember(t, d, "a golden horse", "equus", "animal", "animal")
	groundLatinPair(t, d, "a grass tuft", "grass", "herba")
	if got := sayLang(t, d, "la", "estne equus animal").Say; got != "Ita, equus est animal." {
		t.Fatalf("a grounded membership should affirm in Latin: %q", got)
	}
	if got := sayLang(t, d, "la", "estne equus herba").Say; got != "Minime, equus non est herba." {
		t.Fatalf("a non-membership should deny in Latin: %q", got)
	}
	_ = equus
}

// The is-a path declines a comparison so "estne X maior quam Y" still routes to
// the Latin comparison handler.
func TestLatinIsADeclinesComparison(t *testing.T) {
	d, _ := testDaemon(t)
	latinMember(t, d, "a golden horse", "equus", "animal", "animal")
	groundLatinPair(t, d, "a spotted cow", "cow", "vacca")
	if _, ok := d.planLatinIsA(grammar.Frame{Lang: "la", Force: grammar.ForceQuestion}, "estne equus maior quam vacca"); ok {
		t.Fatal("the is-a path must decline a comparison, leaving it to planLatinComparison")
	}
}

// An irregular Latin verb (velle, to want) is read where the ending rules cannot
// reach: "equus aquam vult" parses by case into the one proposition, and the role
// questions answer it, the same as a regular verb.
func TestLatinIrregularVerbFact(t *testing.T) {
	d, _ := testDaemon(t)
	groundNounLang(t, d, "equus", "la")
	groundNounLang(t, d, "aqua", "la")
	sayLang(t, d, "la", "equus aquam vult") // the horse wants water
	if got := sayLang(t, d, "la", "quid equus vult?").Say; got != "Equus aquam vult." {
		t.Fatalf("an irregular verb fact should answer its role question: %q", got)
	}
	if got := sayLang(t, d, "la", "quis aquam vult?").Say; got != "Equus aquam vult." {
		t.Fatalf("quis should name the agent of an irregular verb fact: %q", got)
	}
}

// A third-conjugation verb (edere, to eat, "edit") is read through its -it ending,
// the conjugation the ending rules gained beyond the first two.
func TestLatinThirdConjugationFact(t *testing.T) {
	d, _ := testDaemon(t)
	groundNounLang(t, d, "simia", "la")
	groundNounLang(t, d, "herba", "la")
	sayLang(t, d, "la", "simia herbam edit") // the monkey eats grass
	if got := sayLang(t, d, "la", "quid simia edit?").Say; got != "Simia herbam edit." {
		t.Fatalf("a third-conjugation verb fact should answer its role question: %q", got)
	}
}
