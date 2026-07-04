package daemon

import (
	"fmt"
	"strconv"
	"time"

	"golem/internal/events"
	"golem/internal/grammar"
	"golem/internal/limbic"
	"golem/internal/memory"
)

// A counting question over the graph is a QUERY GOLEM composes from the
// asking's own meaning and runs over its own memory: a relation, a focus, the
// focus's role, and an optional category filter, one executor for every shape
// rather than a route per question. "How many animals did the dog chase"
// composes count(chase, focus: dog as agent, filter: animal); "how many
// things did the dog chase" drops the filter; "how many animals do you know"
// drops the relation and tallies the category's members. The look is fresh at
// each asking and audited; a count is an observation of a growing graph,
// never a cached fact.

// graphQuery is one composed introspective count: the relation walked (empty
// for a bare member tally), the focus and the role it holds, and the category
// the counted co-arguments must belong to (nil for all of them).
type graphQuery struct {
	rel       *memory.Fragment
	focus     *memory.Fragment
	focusRole string
	category  *memory.Fragment
}

// composeCountQuery reads a counting frame into the query it means. Each
// term's tokens resolve through their lemmas and sort themselves: a relation,
// a category, a focus concept. The span that carries the relation tells the
// asked side: a category sharing the relation's span sits as its subject
// ("how many animals chased the dog"), so the focus is the other term and the
// counted co-arguments are agents; otherwise the focus shares the relation's
// clause ("did the dog chase") and the counted co-arguments are patients.
func (d *Daemon) composeCountQuery(frame grammar.Frame) (graphQuery, bool) {
	if !frame.Counting {
		return graphQuery{}, false
	}
	var q graphQuery
	catSpan, focusSpan := -1, -1
	relSpan := -1
	terms := []grammar.Term{frame.Agent, frame.Patient}
	for i, term := range terms {
		if term.Phrase == "" && term.Concept == nil {
			continue
		}
		phrase := term.Phrase
		if phrase == "" {
			phrase = term.Concept.Content
		}
		toks, _ := grammar.Tokenize(phrase)
		for _, tok := range toks {
			lemma := d.gram.Lemma(frame.Lang, tok)
			if q.rel == nil {
				if r := d.resolveRelation(lemma); r != nil {
					q.rel, relSpan = r, i
					continue
				}
			}
			c := d.resolveConceptLang(lemma, frame.Lang)
			if c == nil {
				continue
			}
			switch {
			case d.isCategory(c) && q.category == nil:
				q.category, catSpan = c, i
			case !d.isCategory(c) && q.focus == nil && !d.isParticipant(c.ID):
				q.focus, focusSpan = c, i
			}
		}
	}
	if q.rel == nil {
		// No relation asked: a bare category tally, the member count.
		return q, q.category != nil
	}
	if q.focus == nil {
		return graphQuery{}, false
	}
	q.focusRole = "agent"
	if q.category != nil && catSpan == relSpan && focusSpan != relSpan {
		// The counted things share the relation's span as its subject, so the
		// focus stands on the other side and the agents are counted.
		q.focusRole = "patient"
	}
	return q, true
}

// runCountQuery executes a composed count over the graph: the relation's
// co-arguments of the focus, filtered by category membership when the query
// carries one, distinct concepts counted. With no relation it tallies the
// category's grounded member concepts. The look is audited with the composed
// query spelled out, so the derivation reads off the trail.
func (d *Daemon) runCountQuery(q graphQuery) (int, bool) {
	if q.rel == nil {
		if q.category == nil {
			return 0, false
		}
		edges, err := d.store.LinksFromRel([]uint64{q.category.ID}, memory.RelGrounds)
		if err != nil {
			return 0, false
		}
		ids := make([]uint64, 0, len(edges))
		for _, e := range edges {
			ids = append(ids, e.To)
		}
		frags, err := d.store.FragmentsByIDs(ids)
		if err != nil {
			return 0, false
		}
		n := 0
		for _, f := range frags {
			if f.Kind == memory.KindConcept {
				n++
			}
		}
		d.auditCount("members of "+q.category.Content, n)
		return n, true
	}
	// A tally reads the whole graph, not the conversational thread: gatherFacts
	// is thread-first so an answer stays with what the exchange spoke of, but a
	// count is about everything the focus stands in, so the entrenched walk is
	// read directly.
	dests, err := d.walkableDests(q.focus.ID)
	if err != nil {
		return 0, false
	}
	side := memory.RelPatient // the focus as agent reaches its patients
	if q.focusRole == "patient" {
		side = memory.RelAgent
	}
	seen := map[uint64]bool{}
	for _, dst := range dests[side] {
		if dst.via != q.rel.Content || seen[dst.id] {
			continue
		}
		if q.category != nil && !d.isA(dst.id, q.category.ID) {
			continue
		}
		seen[dst.id] = true
	}
	n := len(seen)
	what := q.rel.Content + "(" + q.focus.Content + " as " + q.focusRole + ")"
	if q.category != nil {
		what += " in " + q.category.Content
	}
	d.auditCount(what, n)
	return n, true
}

// auditCount leaves the introspective look on the trail: what was counted and
// what the graph held at that moment.
func (d *Daemon) auditCount(what string, n int) {
	d.event(events.Event{Ev: "count", Cue: what, Outcome: strconv.Itoa(n)})
	if err := d.store.Audit(time.Now().Unix(), "count", what, strconv.Itoa(n)); err != nil {
		fmt.Fprintf(d.out, "audit write failed: %v\n", err)
	}
}

// sayCount speaks a composed count in the asking's own shape: a relation
// query as the focus's clause ("The dog chased 2 animals."), a bare member
// tally as a knowledge claim ("I know 79 animals."). The counted noun is the
// category's name, or the asking's sortal when no category filtered.
func (d *Daemon) sayCount(frame grammar.Frame, q graphQuery, n int, chem limbic.State) string {
	noun := frame.Sort
	if q.category != nil {
		noun = d.conceptName(q.category.ID, q.category.Content, frame.Lang)
	}
	if noun == "" {
		noun = "thing"
	}
	if n != 1 {
		noun = d.gram.PluralNoun(frame.Lang, noun)
	}
	counted := strconv.Itoa(n) + " " + noun
	if q.rel == nil {
		return d.realizeAs(frame.Lang, grammar.Frame{Force: grammar.ForceStatement,
			Action:  grammar.Term{Phrase: "know"},
			Patient: grammar.Term{Phrase: counted}}, chem)
	}
	// A relation count speaks as the focus's own clause through the argument
	// constructions, the engine every proposition answer runs: "The dog
	// chased 2 animals."
	focus := grammar.Term{Phrase: d.conceptName(q.focus.ID, q.focus.Content, frame.Lang), Definite: true}
	countTerm := grammar.Term{Phrase: counted}
	m := grammar.Meaning{
		Relation: grammar.Term{Phrase: d.relationName(q.rel.Content, frame.Lang)},
		Roles:    map[string]grammar.Term{"agent": focus, "patient": countTerm},
		Past:     frame.Past, Lang: frame.Lang,
	}
	if q.focusRole == "patient" {
		m.Roles = map[string]grammar.Term{"agent": countTerm, "patient": focus}
	}
	if line, ok := d.gram.Speak(frame.Lang, m); ok {
		return line
	}
	return d.realizeAs(frame.Lang, grammar.Frame{Force: grammar.ForceStatement,
		Action:  grammar.Term{Phrase: "know"},
		Patient: grammar.Term{Phrase: counted}}, chem)
}
