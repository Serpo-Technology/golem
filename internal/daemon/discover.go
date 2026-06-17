package daemon

import (
	"sort"
	"strconv"
	"strings"

	"golem/internal/memory"
)

const (
	discoverKindFloor = 0.6 // how near in the learned space concepts must sit to read as one kind
	discoverKindCap   = 5   // nearest neighbours weighed for a kind
	discoverKindMin   = 2   // members beyond the seed before a cluster reads as a kind
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
	return upperFirst(d.coordinatedNP(lang, names)) + " " + akin + ".", true
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
