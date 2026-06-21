package daemon

import (
	"fmt"
	"time"

	"golem/internal/grammar"
	"golem/internal/memory"
)

// Possession is a relation that holds one thing as another's ("the tree has a
// leaf"). English marks it with a transitive verb (have, own), so its English side
// needs no special path; Turkish marks it by case and the existential "var"
// ("ağacın yaprağı var", the tree's leaf exists) with no verb, so it has its own
// comprehension and generation, unified with the English relation through the
// possessive property the two share.

const possessiveWeight = 1.0

// markPossessive records that a relation holds a possession, the property a Turkish
// "var" clause renders by and an English "have" unifies onto.
func (d *Daemon) markPossessive(relID uint64) {
	d.store.SetLinkRel(relID, relID, memory.RelPossessive, possessiveWeight)
}

// relationIsPossessive reports whether a relation has been marked possessive.
func (d *Daemon) relationIsPossessive(relID uint64) bool {
	edges, err := d.store.LinksFromRel([]uint64{relID}, memory.RelPossessive)
	return err == nil && len(edges) > 0
}

// factIsPossessive reports whether a gathered fact's relation is possessive.
func (d *Daemon) factIsPossessive(relContent string) bool {
	rel := d.resolveRelation(relContent)
	return rel != nil && d.relationIsPossessive(rel.ID)
}

// possessionVerb reports whether an English verb names a possession, the small
// closed class whose use marks its relation possessive so a Turkish "var" clause
// unifies onto it.
func possessionVerb(lemma string) bool {
	switch lemma {
	case "have", "own", "possess":
		return true
	}
	return false
}

// resolveGround resolves the first of a set of candidate stems that names a
// concept, the case-stripped forms a Turkish cased noun might reduce to.
func (d *Daemon) resolveGround(cands []string, ok bool) *memory.Fragment {
	if !ok {
		return nil
	}
	for _, c := range cands {
		if f := d.resolveConcept(c); f != nil {
			return f
		}
	}
	return nil
}

// comprehendPossessionTR reads a Turkish possession clause "X-GEN Y-POSS var"
// ("ağacın yaprağı var"): the genitive noun is the possessor (the agent of having),
// the possessive noun the possessed (the patient), and "var" closes it. The relation
// is the possessive one warm from a recent telling (so it unifies with an English
// "the tree has a leaf") or a general Turkish possession when none is warm. It
// returns false when the clause is not that shape, so the verb-clause path stands.
func (d *Daemon) comprehendPossessionTR(raw string, momentID uint64) bool {
	tokens, _ := grammar.Tokenize(raw)
	if len(tokens) != 3 || !d.gram.Existential("tr", tokens[2]) {
		return false
	}
	possessor := d.resolveGround(d.gram.GenitiveGround("tr", tokens[0]))
	possessed := d.resolveGround(d.gram.PossessiveGround("tr", tokens[1]))
	if possessor == nil || possessed == nil || possessor.ID == possessed.ID {
		return false
	}
	rel := d.possessionRelationTR()
	if rel == nil {
		return false
	}
	if err := d.store.Propose(rel.ID, momentID, possessor.ID, possessed.ID, comprehendWeight); err != nil {
		fmt.Fprintf(d.out, "possession propose error: %v\n", err)
		return false
	}
	d.markPossessive(rel.ID)
	d.noteFocus(possessor)
	d.recordThreadProp(rel.Content, possessor.ID, possessed.ID, possessor.Content, possessed.Content)
	d.ws.excite(rel.ID, exciteSpeech)
	d.ws.exciteRole(possessor.ID, exciteSpeech, "agent")
	d.ws.exciteRole(possessed.ID, exciteSpeech, "patient")
	return true
}

// possessionRelationTR resolves the relation a Turkish possession clause stands in:
// the possessive relation warm from a recent telling (so a Turkish "var" clause
// spoken right after its English form joins that one relation), or a general Turkish
// possession concept when none is warm. The general one is named so a later query
// resolves it by content; resolveRelation keeps only a relation, so sharing the form
// with the English "have" verb is harmless.
func (d *Daemon) possessionRelationTR() *memory.Fragment {
	if warm := d.warmestRelation(); warm != nil && d.relationIsPossessive(warm.ID) {
		return warm
	}
	const tpl = "rel:tr:have"
	if c, err := d.store.ConceptByTemplate(tpl); err == nil && c != nil {
		return c
	}
	c := &memory.Fragment{Kind: memory.KindConcept, Channel: "concept", Content: "have",
		Template: tpl, Category: "relation", Limbic: d.chem.Snapshot(), Time: time.Now()}
	if _, err := d.store.Encode(c); err != nil {
		fmt.Fprintf(d.out, "possession relation mint error: %v\n", err)
		return nil
	}
	d.nameRelationInTongue(c, "have", "tr")
	d.markPossessive(c.ID)
	return c
}

// possessionClauseTR renders "X-GEN Y-POSS var" ("Ağacın yaprağı var") through the
// genitive and possessive cases, the Turkish form a possession relation takes where
// English uses "has".
func (d *Daemon) possessionClauseTR(possessor, possessed string) string {
	clause := d.gram.Genitive("tr", possessor) + " " + d.gram.Possessive("tr", possessed) + " var"
	return grammar.Sentence(clause)
}

// planPossessionPolarTR answers a Turkish possession question "X-GEN Y-POSS var mı"
// ("ağacın yaprağı var mı", does the tree have a leaf) from the stored proposition,
// the affirmed clause the same "var" form the figure's possession reads. It returns
// false when the clause is not a possession question, so ordinary handling stands.
func (d *Daemon) planPossessionPolarTR(frame grammar.Frame, raw string) (string, bool) {
	if frame.Lang != "tr" {
		return "", false
	}
	// The existential clause carries no verb, so the frame's force is unreliable; the
	// question particle is read off the surface the same way a polar question is.
	clause, _, ok := polarClause(frame.Lang, raw)
	if !ok {
		return "", false
	}
	tokens, _ := grammar.Tokenize(clause)
	if len(tokens) != 3 || !d.gram.Existential("tr", tokens[2]) {
		return "", false
	}
	possessor := d.resolveGround(d.gram.GenitiveGround("tr", tokens[0]))
	possessed := d.resolveGround(d.gram.PossessiveGround("tr", tokens[1]))
	if possessor == nil || possessed == nil {
		return "", false
	}
	d.noteFocus(possessor)
	held := false
	for _, f := range d.gatherFacts(possessor, "agent", "") {
		if f.coID == possessed.ID && d.factIsPossessive(f.rel) {
			held = true
			break
		}
	}
	if !held {
		return upperFirst(d.gram.Word(frame.Lang, "no")) + ".", true
	}
	affirmed := d.possessionClauseTR(
		d.conceptName(possessor.ID, possessor.Content, frame.Lang),
		d.conceptName(possessed.ID, possessed.Content, frame.Lang))
	return upperFirst(d.gram.Word(frame.Lang, "yes")) + ", " + lowerFirst(affirmed), true
}
