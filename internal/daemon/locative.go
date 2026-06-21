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
		return grammar.Sentence(clause), true
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
	belief, _, supposed := d.assessProposition(figure, rel.Content, ground)
	return d.answerPolar(frame.Lang, figure, rel, ground, false, belief, supposed), true
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
				return d.storeLocativeTR(momentID, figure, ground, d.prepositionRelation(prep))
			}
		}
	}
	// General locative form: figure ground-LOC ("yaprak ağaçta"). The locative-cased
	// noun is the ground and the bare noun the figure; the relation is the warm
	// locative one or a general Turkish locative.
	if len(tokens) != 2 {
		return false
	}
	groundIdx, ground := -1, (*memory.Fragment)(nil)
	for i, tok := range tokens {
		if stem, ok := d.gram.LocativeGround("tr", tok); ok {
			if c := d.resolveConcept(stem); c != nil {
				groundIdx, ground = i, c
				break
			}
		}
	}
	if ground == nil {
		return false
	}
	figure := d.resolveConcept(tokens[1-groundIdx])
	if figure == nil || figure.ID == ground.ID {
		return false
	}
	return d.storeLocativeTR(momentID, figure, ground, d.locativeRelationTR())
}

// storeLocativeTR writes a Turkish locative proposition (the figure at the ground
// under the given relation), marks the relation locative, and warms the parts. It
// returns false when the relation could not be resolved.
func (d *Daemon) storeLocativeTR(momentID uint64, figure, ground, rel *memory.Fragment) bool {
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
