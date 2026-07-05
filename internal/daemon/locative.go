package daemon

import (
	"fmt"
	"strings"
	"time"

	"golem/internal/grammar"
	"golem/internal/limbic"
	"golem/internal/memory"
)

// Locativity is a property a relation can carry: that it places its figure at its
// ground ("the leaf is on the tree"). Like transitivity it lives in the graph as
// a fact about the relation, not in a construction. GOLEM does not need to be told
// it separately, though: the locative construction itself teaches it. A relation
// spoken as a copula with a seeded preposition ("is on", "is in", "is under") is
// marked locative when it is comprehended, so the property grows from the same
// sentences that lay the spatial facts, and thereafter a "where" query reads it
// and the clause renders as a copula with a prepositional complement rather than
// a verb.

// locativeWeight is the weight of the self-edge that marks a relation locative,
// matched to the transitive marker so a learned relation-property persists across
// the nightly decay the same way.
const locativeWeight = 1.0

// markLocative records that a relation is locative: a self-edge on the relation
// concept carrying the learned property.
func (d *Daemon) markLocative(relID uint64) {
	d.store.SetLinkRel(relID, relID, memory.RelLocative, locativeWeight)
}

// relationIsLocative reports whether a relation has been marked locative, so a
// query over it reads its figure's place and the clause renders with a
// preposition rather than as a verb.
func (d *Daemon) relationIsLocative(relID uint64) bool {
	edges, err := d.store.LinksFromRel([]uint64{relID}, memory.RelLocative)
	return err == nil && len(edges) > 0
}

// factIsLocative reports whether a gathered fact's relation is locative, the test
// the answer paths use to render a clause with its preposition rather than a verb.
func (d *Daemon) factIsLocative(relContent string) bool {
	rel := d.resolveRelation(relContent)
	return rel != nil && d.relationIsLocative(rel.ID)
}

// markPartWhole records that a relation holds one thing as part of another, a self-edge on the
// relation concept carrying the learned property, so the containment walk steps through it.
func (d *Daemon) markPartWhole(relID uint64) {
	d.store.SetLinkRel(relID, relID, memory.RelPartWhole, locativeWeight)
}

// factIsPartWhole reports whether a gathered fact's relation holds a part of a whole, the test the
// containment walk uses to step from a part to the whole it is located with.
func (d *Daemon) factIsPartWhole(relContent string) bool {
	rel := d.resolveRelation(relContent)
	if rel == nil {
		return false
	}
	edges, err := d.store.LinksFromRel([]uint64{rel.ID}, memory.RelPartWhole)
	return err == nil && len(edges) > 0
}

// partWholes lists the wholes a figure is a part of: the patients of its part-whole propositions
// ("the leaf is part of the tree" yields the tree).
func (d *Daemon) partWholes(figureID uint64) []uint64 {
	frags, err := d.store.FragmentsByIDs([]uint64{figureID})
	if err != nil || len(frags) != 1 {
		return nil
	}
	var wholes []uint64
	for _, f := range d.gatherFacts(frags[0], "agent", "") {
		if d.factIsPartWhole(f.rel) {
			wholes = append(wholes, f.coID)
		}
	}
	return wholes
}

// partWholeRelation is the canonical content of the part-whole relation, distinct from the noun
// "part" so the relation a part-of clause stores is one concept however the clause was phrased.
const partWholeRelation = "part-of"

// comprehendPartWhole reads a part-of clause ("the leaf is part of the tree") into a part-whole
// proposition: the construction's marker is "part of" between the part and the whole, so the figure
// is the noun before it and the whole the noun after. The standard argument parse mistakes "of" for
// the verb, so the construction is read here, before that path, the way the locative clause is. It
// returns false when the clause is not a part-of one, so ordinary comprehension stands.
func (d *Daemon) comprehendPartWhole(raw string, momentID uint64) bool {
	tokens, _ := grammar.Tokenize(raw)
	mark := -1
	for i := 0; i+1 < len(tokens); i++ {
		if tokens[i] == "part" && tokens[i+1] == "of" {
			mark = i
			break
		}
	}
	if mark < 1 || mark+2 >= len(tokens) {
		return false
	}
	resolveNoun := func(toks []string) *memory.Fragment {
		for _, t := range toks {
			if !d.gram.IsContentWord("en", t) {
				continue // the determiner and the copula name no part or whole
			}
			if c := d.resolveConcept(t); c != nil && c.Category != "relation" {
				return c
			}
		}
		return nil
	}
	part := resolveNoun(tokens[:mark])
	whole := resolveNoun(tokens[mark+2:])
	if part == nil || whole == nil || part.ID == whole.ID {
		return false
	}
	rel, err := d.relationConcept(grammar.Term{Phrase: partWholeRelation}, "en")
	if err != nil || rel == nil {
		return false
	}
	if err := d.store.Propose(rel.ID, momentID, part.ID, whole.ID, comprehendWeight); err != nil {
		fmt.Fprintf(d.out, "part-whole propose error: %v\n", err)
		return false
	}
	d.markPartWhole(rel.ID)
	d.noteFocus(part)
	d.recordThreadProp(rel.Content, part.ID, whole.ID, part.Content, whole.Content)
	d.ws.excite(rel.ID, exciteSpeech)
	d.ws.exciteRole(part.ID, exciteSpeech, "agent")
	d.ws.exciteRole(whole.ID, exciteSpeech, "patient")
	return true
}

// planPartWholePolar answers "is the wing part of the bird" from the stored
// part-whole proposition, read before the is-a path would misread the clause
// as a membership question ("is a wing a bird") and deny it. The marker is the
// same "part of" surface the told construction reads. It declines when either
// noun is unknown, so the ordinary paths stand.
func (d *Daemon) planPartWholePolar(frame grammar.Frame, raw string) (string, bool) {
	if frame.Force != grammar.ForceQuestion || frame.Wh != "" {
		return "", false // a told part-of clause teaches; only the inverted question reads
	}
	tokens, _ := grammar.Tokenize(raw)
	mark := -1
	for i := 0; i+1 < len(tokens); i++ {
		if tokens[i] == "part" && tokens[i+1] == "of" {
			mark = i
			break
		}
	}
	if mark < 1 || mark+2 >= len(tokens) {
		return "", false
	}
	resolveNoun := func(toks []string) *memory.Fragment {
		for _, t := range toks {
			if !d.gram.IsContentWord("en", t) {
				continue
			}
			if c := d.resolveConcept(t); c != nil && c.Category != "relation" {
				return c
			}
		}
		return nil
	}
	part := resolveNoun(tokens[:mark])
	whole := resolveNoun(tokens[mark+2:])
	if part == nil || whole == nil || part.ID == whole.ID {
		return "", false
	}
	d.noteFocus(part)
	partName := d.conceptName(part.ID, part.Content, frame.Lang)
	wholeName := d.conceptName(whole.ID, whole.Content, frame.Lang)
	if d.propositionHolds(part, partWholeRelation, whole.ID) {
		answer := upperFirst(d.gram.Word(frame.Lang, "yes")) + ", the " + partName +
			" is part of the " + wholeName + "."
		d.traceClaim(partWholeRelation, part.ID, whole.ID, answer,
			d.polarEvidence(partWholeRelation, part.ID, whole.ID, 0))
		return answer, true
	}
	return upperFirst(d.gram.Word(frame.Lang, "no")) + ", the " + partName +
		" is not part of the " + wholeName + ".", true
}

// locativeClause renders "FIGURE is PREP GROUND" ("the leaf is on the tree")
// through the copula construction, the preposition and ground riding as the
// complement the way an is-a answer's category does. The figure and ground are
// already glossed; the figure is "it" when the subject is carried from a clause
// just spoken. It returns false when the engine cannot speak the clause.
func (d *Daemon) locativeClause(lang, figure, prep, ground string, given, negate bool, chem limbic.State) (string, bool) {
	if lang == "tr" {
		// Turkish marks place by case, no preposition or copula. A specific place
		// takes the postposition over the genitive ground ("yaprak ağacın üstünde");
		// a general one takes the locative case ("yaprak ağaçta"). The figure is named
		// (no inflected pronoun for it here).
		var clause string
		if post, ok := d.gram.PostpositionFor(lang, prep); ok {
			clause = figure + " " + d.gram.Genitive(lang, ground) + " " + post
		} else {
			clause = figure + " " + d.gram.Locative(lang, ground)
		}
		if negate {
			// Turkish denies a copular place with the standing negator "değil" after the
			// placed phrase ("yaprak ormanda değil"), the same word a comparison is denied with.
			if not := d.gram.Word(lang, "notcompare"); not != "" {
				clause += " " + not
			}
		}
		return grammar.Sentence(clause), true
	}
	if lang == "la" {
		// Latin places the figure with a preposition over the ground and the copula
		// est: "rosa in mensa est". The preposition is the Latin form of the relation.
		prepWord := latinPrepFor[prep]
		if prepWord == "" {
			prepWord = prep
		}
		return grammar.Sentence(figure + " " + prepWord + " " + ground + " est"), true
	}
	subject := definiteForm(lang, figure)
	if given {
		if pron := d.gram.Pronoun(lang, "agent"); pron != "" {
			subject = pron
		}
	}
	comp := prep + " " + definiteForm(lang, ground)
	line := d.realizeAs(lang, grammar.Frame{
		Force:   grammar.ForceStatement,
		Action:  grammar.Term{Phrase: "be"},
		Agent:   grammar.Term{Phrase: subject},
		Patient: grammar.Term{Phrase: comp},
		Negate:  negate,
	}, chem)
	if line == "" {
		return "", false
	}
	return line, true
}

// definiteForm marks a gloss definite for a tongue that uses an article ("the
// tree"); a caseless tongue (Turkish) leaves the bare noun, its definiteness read
// from context until the locative case lands.
func definiteForm(lang, gloss string) string {
	if lang == "tr" || gloss == "" {
		return gloss
	}
	return "the " + gloss
}

// locativeChainDepth bounds the placement walk so a cycle in the grounds cannot loop.
const locativeChainDepth = 8

// containmentPrep is the canonical preposition of the containment relation, the one a chain of
// placements composes into: a thing placed at something within a place is within that place.
// Containment composes where contact does not, so it is the relation the chain reads.
const containmentPrep = "in"

// locativeGrounds lists the grounds a figure is placed at: the patients of its locative
// propositions ("the leaf is on the tree" yields the tree).
func (d *Daemon) locativeGrounds(figureID uint64) []uint64 {
	frags, err := d.store.FragmentsByIDs([]uint64{figureID})
	if err != nil || len(frags) != 1 {
		return nil
	}
	var grounds []uint64
	for _, f := range d.gatherFacts(frags[0], "agent", "") {
		if d.factIsLocative(f.rel) {
			grounds = append(grounds, f.coID)
		}
	}
	return grounds
}

// locativelyInside reports whether the figure is spatially within the target through a chain
// of placements: a thing on or in something that is itself within the target is within the
// target ("the leaf is on the tree, the tree is in the forest, so the leaf is in the forest").
// Containment composes across the specific placements; the deduction is sound. The walk is
// bounded so a cycle in the placements cannot loop. The chain hops follow any placement, but
// the step that reaches the target is containment, so a leaf on a tree is in the tree's forest
// without being in the tree.
func (d *Daemon) locativelyInside(figureID, targetID uint64, depth int) bool {
	if depth <= 0 || figureID == targetID {
		return false
	}
	if frags, err := d.store.FragmentsByIDs([]uint64{figureID}); err == nil && len(frags) == 1 &&
		d.propositionHolds(frags[0], containmentPrep, targetID) {
		return true
	}
	for _, ground := range d.locativeGrounds(figureID) {
		if ground != figureID && d.locativelyInside(ground, targetID, depth-1) {
			return true
		}
	}
	// A part is located where its whole is, so the walk steps from a part to its whole: a leaf
	// that is part of a tree that is in the forest is in the forest. Part-whole composes with
	// placement the way placements compose with each other.
	for _, whole := range d.partWholes(figureID) {
		if whole != figureID && d.locativelyInside(whole, targetID, depth-1) {
			return true
		}
	}
	return false
}

// planLocativePolar answers "is X on Y" from the placed proposition. A copula
// question carrying a locative preposition is a place question, not a category
// one: subject-auxiliary inversion seats the figure in the patient slot and the
// ground in the agent slot, and the preposition names the relation. It reuses
// answerPolar, so the affirmed clause is the same copula the figure's "where"
// reads ("Yes, the leaf is on the tree"). It returns false when the clause is not
// a locative copula question, so ordinary copula handling stands.
func (d *Daemon) planLocativePolar(frame grammar.Frame) (string, bool) {
	// A wh-question carrying a preposition ("what is on the tree") asks for a role,
	// not a yes/no, so it is left to the role-question path; only a bare yes/no
	// copula question is a locative polar.
	if frame.Force != grammar.ForceQuestion || frame.LocPrep == "" || frame.Wh != "" {
		return "", false
	}
	figure, ground := frame.Patient.Concept, frame.Agent.Concept
	if figure == nil {
		figure, ground = frame.Agent.Concept, frame.Patient.Concept
	}
	if figure == nil || ground == nil {
		return d.realizeAs(frame.Lang, grammar.Frame{Force: grammar.ForceStatement,
			Action: grammar.Term{Phrase: "know"}, Negate: true}, d.chem.Snapshot()), true
	}
	d.noteFocus(figure)
	rel := d.resolveRelation(frame.LocPrep)
	if rel == nil {
		// The preposition has never placed anything, so the proposition cannot hold.
		return upperFirst(d.gram.Word(frame.Lang, "no")) + ".", true
	}
	belief, _, supposed, lender := d.assessProposition(figure, rel.Content, ground)
	return d.answerPolar(frame.Lang, figure, rel, ground, false, belief, supposed, lender), true
}

// comprehendLocativeTR reads a Turkish locative clause, which marks place by the
// locative case and carries no relation word ("yaprak ağaçta", the leaf is at the
// tree). The locative-cased noun is the ground and the bare noun the figure. The
// relation is the locative one warm from a recent telling, so an English "the leaf
// is on the tree" and its Turkish echo are the one relation; a general Turkish
// locative is minted when none is warm. It returns false when the clause is not a
// bare locative (no locative case, a referent unresolved, or more than the two
// nouns), so the ordinary verb-clause path stands.
func (d *Daemon) comprehendLocativeTR(raw string, momentID uint64) bool {
	tokens, _ := grammar.Tokenize(raw)
	// Postposition form: figure ground-GEN postposition ("yaprak ağacın üstünde"),
	// the postposition naming the precise place its English preposition does, so it
	// joins that relation across tongues.
	if len(tokens) == 3 {
		if prep, ok := d.gram.Postposition("tr", tokens[2]); ok {
			figure := d.resolveConcept(tokens[0])
			ground := d.resolveGround(d.gram.GenitiveGround("tr", tokens[1]))
			if figure != nil && ground != nil && figure.ID != ground.ID {
				return d.storeLocative(momentID, figure, ground, d.prepositionRelation(prep))
			}
		}
	}
	// General locative form: figure ground-LOC ("yaprak ağaçta"). The relation is the warm
	// locative one or a general Turkish locative.
	figure, ground, ok := d.parseLocativeContainmentTR(raw)
	if !ok {
		return false
	}
	return d.storeLocative(momentID, figure, ground, d.locativeRelationTR())
}

// parseLocativeContainmentTR reads a Turkish general-locative clause ("yaprak ormanda", the leaf
// in the forest) into its figure and ground: the locative-cased noun (-da/-de) is the ground and
// the bare noun the figure. The locative case asks containment, so it is the form a "is X in Y"
// question takes, distinct from the postposition forms (on, under) that state contact. It returns
// false when the clause is not a two-noun general locative.
func (d *Daemon) parseLocativeContainmentTR(raw string) (figure, ground *memory.Fragment, ok bool) {
	tokens, _ := grammar.Tokenize(raw)
	if len(tokens) != 2 {
		return nil, nil, false
	}
	groundIdx := -1
	for i, tok := range tokens {
		if stem, isLoc := d.gram.LocativeGround("tr", tok); isLoc {
			if c := d.resolveConcept(stem); c != nil {
				groundIdx, ground = i, c
				break
			}
		}
	}
	if ground == nil {
		return nil, nil, false
	}
	figure = d.resolveConcept(tokens[1-groundIdx])
	if figure == nil || figure.ID == ground.ID {
		return nil, nil, false
	}
	return figure, ground, true
}

// planLocativePolarTR answers a Turkish "is X in Y" asked with the locative case ("yaprak ormanda
// mı"). The locative case asks containment, so the placement chain reads the way the English "is X
// in Y" reads it: a leaf on a tree that is in the forest is in the forest. It reuses
// assessProposition (the containment composition) and answerPolar (the locative copula), so the
// reasoning and the surface are the English path's; only the parse is Turkish. It returns false
// when the clause is not a Turkish locative question, so the ordinary polar path stands.
func (d *Daemon) planLocativePolarTR(frame grammar.Frame, raw string) (string, bool) {
	if frame.Lang != "tr" {
		return "", false
	}
	clause, _, ok := polarClause(frame.Lang, raw)
	if !ok {
		return "", false
	}
	figure, ground, ok := d.parseLocativeContainmentTR(clause)
	if !ok {
		return "", false
	}
	rel := d.resolveRelation(containmentPrep)
	if rel == nil {
		// Nothing has ever been placed inside anything, so the chain cannot hold.
		return upperFirst(d.gram.Word(frame.Lang, "no")) + ".", true
	}
	d.noteFocus(figure)
	belief, _, supposed, lender := d.assessProposition(figure, containmentPrep, ground)
	return d.answerPolar(frame.Lang, figure, rel, ground, false, belief, supposed, lender), true
}

// storeLocative writes a locative proposition (the figure at the ground
// under the given relation), marks the relation locative, and warms the parts. It
// returns false when the relation could not be resolved.
func (d *Daemon) storeLocative(momentID uint64, figure, ground, rel *memory.Fragment) bool {
	if rel == nil {
		return false
	}
	if err := d.store.Propose(rel.ID, momentID, figure.ID, ground.ID, comprehendWeight); err != nil {
		fmt.Fprintf(d.out, "locative propose error: %v\n", err)
		return false
	}
	d.markLocative(rel.ID)
	d.noteFocus(figure)
	d.recordThreadProp(rel.Content, figure.ID, ground.ID, figure.Content, ground.Content)
	d.ws.excite(rel.ID, exciteSpeech)
	d.ws.exciteRole(figure.ID, exciteSpeech, "agent")
	d.ws.exciteRole(ground.ID, exciteSpeech, "patient")
	return true
}

// prepositionRelation resolves the canonical locative relation a preposition names
// (on, in, under), the one both tongues' spatial forms join: the relation already
// named by the preposition if it exists, else one keyed and named by it. The English
// copula path and the Turkish postposition reach the same concept, so the place is
// one relation however it was spoken. It is marked locative so a "where" reads it.
func (d *Daemon) prepositionRelation(prep string) *memory.Fragment {
	if r := d.resolveRelation(prep); r != nil {
		d.markLocative(r.ID)
		return r
	}
	tpl := "rel:en:" + prep
	if c, err := d.store.ConceptByTemplate(tpl); err == nil && c != nil {
		d.markLocative(c.ID)
		return c
	}
	c := &memory.Fragment{Kind: memory.KindConcept, Channel: "concept", Content: prep,
		Template: tpl, Category: "relation", Limbic: d.chem.Snapshot(), Time: time.Now()}
	if _, err := d.store.Encode(c); err != nil {
		fmt.Fprintf(d.out, "locative relation mint error: %v\n", err)
		return nil
	}
	d.nameRelationInTongue(c, prep, "en")
	d.markLocative(c.ID)
	return c
}

// locativeRelationTR resolves the relation a Turkish locative clause stands in:
// the locative relation warm from a recent telling (so a Turkish place clause spoken
// right after its English form joins that one relation), or a general Turkish
// locative concept when none is warm. The general one renders back as the case, not
// a word, so its English-side lemma is only ever read on a cross-tongue query.
func (d *Daemon) locativeRelationTR() *memory.Fragment {
	if warm := d.warmestRelation(); warm != nil && d.relationIsLocative(warm.ID) {
		return warm
	}
	const tpl = "rel:tr:at"
	if c, err := d.store.ConceptByTemplate(tpl); err == nil && c != nil {
		return c
	}
	c := &memory.Fragment{Kind: memory.KindConcept, Channel: "concept", Content: "at",
		Template: tpl, Category: "relation", Limbic: d.chem.Snapshot(), Time: time.Now()}
	if _, err := d.store.Encode(c); err != nil {
		fmt.Fprintf(d.out, "locative relation mint error: %v\n", err)
		return nil
	}
	// Name it so a later query resolves it by content; resolveRelation keeps only a
	// relation, so sharing the form with the Turkish horse "at" (a noun) is harmless.
	d.nameRelationInTongue(c, "at", "tr")
	return c
}

// planLocativeWh answers "what is on/in/under X" by naming the figures that hold
// that place at the ground. The preposition the clause carried filters the ground's
// locative relations, so the in-things and the under-things stay apart; without it
// the ground's name alone would read as a definition and any place would answer. It
// returns false when the clause carries no locative preposition, so an ordinary
// "what" falls through.
func (d *Daemon) planLocativeWh(frame grammar.Frame, chem limbic.State) (string, bool) {
	if frame.LocPrep == "" || (frame.Wh != "what" && frame.Wh != "which") {
		return "", false
	}
	ground := frame.Agent.Concept
	if ground == nil {
		ground = frame.Patient.Concept
	}
	if ground == nil {
		name := frame.Agent.Phrase
		if name == "" {
			name = frame.Patient.Phrase
		}
		ground = d.resolveConcept(name)
	}
	if ground == nil {
		return "", false
	}
	d.noteFocus(ground)
	groundName := d.conceptName(ground.ID, ground.Content, frame.Lang)
	var clauses []string
	for _, f := range d.gatherFacts(ground, "patient", frame.LocPrep) {
		figure := d.conceptName(f.coID, f.coGloss, frame.Lang)
		if cl, ok := d.locativeClause(frame.Lang, figure, frame.LocPrep, groundName, false, false, chem); ok {
			clauses = append(clauses, cl)
		}
	}
	if len(clauses) == 0 {
		return "", false
	}
	return strings.Join(clauses, " "), true
}

// planWhere answers "where is X" from the figure's locative relations: it walks
// the relations marked locative that X stands in as figure and speaks the place,
// "The leaf is on the tree." It returns false when X holds no place, so an event
// "where did X happen" falls through to the occurrence answer rather than a
// spatial one it has nothing for.
func (d *Daemon) planWhere(frame grammar.Frame, chem limbic.State) (string, bool) {
	figure := frame.Agent
	if figure.Concept == nil && figure.Phrase == "" {
		figure = frame.Patient
	}
	fc := figure.Concept
	if fc == nil && figure.Phrase != "" {
		fc = d.resolveConcept(figure.Phrase)
	}
	if fc == nil {
		return "", false
	}
	d.noteFocus(fc)
	figName := d.conceptName(fc.ID, fc.Content, frame.Lang)
	var clauses []string
	for _, f := range d.gatherFacts(fc, "agent", "") {
		if !d.factIsLocative(f.rel) {
			continue
		}
		ground := d.conceptName(f.coID, f.coGloss, frame.Lang)
		// The relation's content is the canonical preposition (on/in/under/at): English
		// speaks it, Turkish maps it to a postposition or the locative case.
		prep := f.rel
		if clause, ok := d.locativeClause(frame.Lang, figName, prep, ground, len(clauses) > 0, false, chem); ok {
			clauses = append(clauses, clause)
		}
	}
	if len(clauses) == 0 {
		return "", false
	}
	return strings.Join(clauses, " "), true
}
