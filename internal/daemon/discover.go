package daemon

import (
	"sort"
	"strconv"
	"strings"
	"time"

	"golem/internal/grammar"
	"golem/internal/memory"
)

const (
	discoverKindFloor = 0.6 // how near in the learned space concepts must sit to read as one kind
	discoverKindCap   = 5   // nearest neighbours weighed for a kind
	discoverKindMin   = 2   // members beyond the seed before a cluster reads as a kind
	namedKindWeight   = 1.0 // grounding weight a told naming gives the new category, a firm tie
)

// discoverKind finds the un-named kind a concept sits in: the concepts nearest it
// in the learned space that share no category with it yet. It is the vector
// proposing a category the graph never grew, concepts that cluster by their own
// grounded company but were given no common word. It returns the co-members (the
// seed excluded), or nothing when the seed has no embedding or no near company
// outside the categories it already belongs to (an already-named kind is not
// rediscovered). The graph disposes: this only proposes, nothing is grounded here.
func (d *Daemon) discoverKind(seedID uint64) []*memory.Fragment {
	vec, ok := d.store.Vector(seedID)
	if !ok {
		return nil
	}
	matches, err := d.store.NearestVectors(vec, memory.KindConcept, discoverKindCap, seedID)
	if err != nil {
		return nil
	}
	seedCats := map[uint64]bool{}
	for _, c := range d.categoriesOf(seedID) {
		seedCats[c] = true
	}
	var ids []uint64
	for _, m := range matches {
		if m.Score < discoverKindFloor {
			continue
		}
		shares := false
		for _, c := range d.categoriesOf(m.ID) {
			if seedCats[c] {
				shares = true
				break
			}
		}
		if !shares {
			ids = append(ids, m.ID)
		}
	}
	if len(ids) < discoverKindMin {
		return nil
	}
	frags, err := d.fragmentsByIDsTiered(ids)
	if err != nil {
		return nil
	}
	var members []*memory.Fragment
	for _, f := range frags {
		// A kind gathers members, so an abstraction caught nearby is not one of them.
		if f.Kind == memory.KindConcept && !d.isCategory(f) {
			members = append(members, f)
		}
	}
	if len(members) < discoverKindMin {
		return nil
	}
	return members
}

// proposeKind voices an un-named kind GOLEM has noticed around the topic: the
// topic and its near company, named together and offered as a kind, hedged, for
// the speaker to name or wave off. Said once per cluster. It is the proposing half
// of propose-and-dispose; nothing is grounded until the kind is given a word.
func (d *Daemon) proposeKind(lang string) (string, bool) {
	members := d.discoverKind(d.topic)
	if len(members) == 0 {
		return "", false
	}
	topic, err := d.fragmentsByIDsTiered([]uint64{d.topic})
	if err != nil || len(topic) != 1 {
		return "", false
	}
	akin := d.gram.Word(lang, "akin")
	if akin == "" {
		return "", false
	}
	names := []string{d.conceptName(d.topic, topic[0].Content, lang)}
	ids := []uint64{d.topic}
	for _, m := range members {
		names = append(names, d.conceptName(m.ID, m.Content, lang))
		ids = append(ids, m.ID)
	}
	sig := kindSignature(ids)
	if d.proposedKinds[sig] {
		return "", false
	}
	if d.proposedKinds == nil {
		d.proposedKinds = map[string]bool{}
	}
	d.proposedKinds[sig] = true
	// The offer stands open: the speaker's next naming grounds these members as a
	// category. A fresh proposal replaces it; the sleep clear lets it lapse.
	d.pendingKind = ids
	return upperFirst(d.coordinatedNP(lang, names)) + " " + akin + ".", true
}

// planNameKind grounds the kind GOLEM proposed once the speaker names it: "they
// are fruit" after "the cat, the dog and the mouse seem to be a kind" turns the
// noticed cluster into a real category, the dispose half of propose-and-dispose.
// It fires only while an offer stands, and only on a naming (an ungrounded,
// group-anaphor subject with a predicate noun, or a bare noun), so an ordinary
// is-a statement about a known concept passes through untouched.
func (d *Daemon) planNameKind(frame grammar.Frame) (string, bool) {
	if len(d.pendingKind) == 0 {
		return "", false
	}
	name := kindNameFromFrame(frame)
	if name == "" {
		return "", false // this turn did not name the kind; the offer stands
	}
	members := d.pendingKind
	d.pendingKind = nil
	return d.groundKind(frame.Lang, name, members)
}

// kindNameFromFrame reads the name a turn gives the pending kind: the predicate of
// a copular statement whose subject is the proposed group ("they are fruit"), or a
// bare noun. The subject being ungrounded is what tells a naming from an is-a
// claim about a specific concept ("a cat is an animal" names nothing).
func kindNameFromFrame(frame grammar.Frame) string {
	if frame.Force != grammar.ForceStatement || frame.Agent.Concept != nil {
		return ""
	}
	if frame.Action.Phrase == "be" {
		if frame.Patient.Phrase != "" {
			return singularNoun(frame.Patient.Phrase)
		}
		if frame.Patient.Concept != nil {
			return singularNoun(frame.Patient.Concept.Content)
		}
		return ""
	}
	// A bare noun, the whole turn ("fruit"): the subject with nothing predicated.
	if frame.Action.Phrase == "" && frame.Agent.Phrase != "" && frame.Patient.Empty() {
		return singularNoun(frame.Agent.Phrase)
	}
	return ""
}

// groundKind turns a named cluster into a category: an abstraction grounded in the
// members, the name bound to it, so the kind now answers is-a like any other. The
// graph disposes here, the noticed similarity becoming a fact the speaker gave.
func (d *Daemon) groundKind(lang, name string, members []uint64) (string, bool) {
	if lang == "" {
		lang = "en" // an English frame carries no lang tag; the template keys on the word's tongue
	}
	name = singularNoun(name)
	if name == "" || len(members) == 0 {
		return "", false
	}
	tpl := "abs:" + lang + ":" + name
	abs, err := d.store.ConceptByTemplate(tpl)
	if err != nil {
		return "", false
	}
	if abs == nil {
		abs = &memory.Fragment{Kind: memory.KindConcept, Channel: "concept", Content: name,
			Template: tpl, Category: "noun", Limbic: d.chem.Snapshot(), Time: time.Now()}
		if _, err := d.store.Encode(abs); err != nil {
			return "", false
		}
	}
	for _, m := range members {
		d.store.SetLinkRel(abs.ID, m, memory.RelGrounds, namedKindWeight)
	}
	if w, _, err := d.store.WordNode(name, lang, d.chem.Snapshot(), time.Now()); err == nil && w != nil {
		d.store.SetLinkRel(w.ID, abs.ID, memory.RelNames, absNameWeight)
	}
	d.noteFocus(abs)
	// Confirm, firm now that the kind is grounded: one member named as the kind,
	// the same shape an is-a answer takes, prefixed with an acknowledgement.
	var memberName string
	if f, err := d.fragmentsByIDsTiered([]uint64{members[0]}); err == nil && len(f) == 1 {
		memberName = d.conceptName(members[0], f[0].Content, lang)
	}
	ack := upperFirst(d.gram.Word(lang, "isee"))
	clause := d.realizeAs(lang, grammar.Frame{Force: grammar.ForceStatement,
		Agent:  grammar.Term{Phrase: grammar.Indefinite(memberName)},
		Action: grammar.Term{Phrase: "be"}, Patient: grammar.Term{Phrase: grammar.Indefinite(name)}},
		d.chem.Snapshot())
	switch {
	case ack != "" && clause != "":
		return ack + ". " + clause, true
	case clause != "":
		return clause, true
	case ack != "":
		return ack + ".", true
	}
	return "", false
}

// singularNoun strips a regular English plural so a kind named "animals" grounds
// as "animal". A best-effort lemma over the regular paradigm, left whole when no
// rule applies.
func singularNoun(s string) string {
	switch {
	case len(s) > 4 && strings.HasSuffix(s, "es") && !strings.HasSuffix(s, "ses"):
		return s[:len(s)-2]
	case len(s) > 3 && strings.HasSuffix(s, "s") && !strings.HasSuffix(s, "ss"):
		return s[:len(s)-1]
	}
	return s
}

// kindSignature keys a proposed cluster by its sorted members, so the same kind is
// offered once however its neighbours are ordered.
func kindSignature(ids []uint64) string {
	sorted := append([]uint64(nil), ids...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })
	parts := make([]string, len(sorted))
	for i, id := range sorted {
		parts[i] = strconv.FormatUint(id, 10)
	}
	return strings.Join(parts, ",")
}

// coordinatedNP names several concepts as one definite, coordinated phrase in the
// tongue's order ("the cat, the dog and the mouse"), the article and coordinator
// read from the tongue's data so a tongue that marks definiteness by case drops
// the article.
func (d *Daemon) coordinatedNP(lang string, names []string) string {
	art := d.gram.Article(lang)
	parts := make([]string, len(names))
	for i, n := range names {
		if art != "" {
			parts[i] = art + " " + n
		} else {
			parts[i] = n
		}
	}
	switch len(parts) {
	case 0:
		return ""
	case 1:
		return parts[0]
	}
	coord := d.gram.Coordinator(lang)
	return strings.Join(parts[:len(parts)-1], ", ") + " " + coord + " " + parts[len(parts)-1]
}
