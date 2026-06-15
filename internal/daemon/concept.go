package daemon

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"golem/internal/events"
	"golem/internal/grammar"
	"golem/internal/memory"
	"golem/internal/senses"
	"golem/wire"
)

// The concept layer is GOLEM's semantic hub: an amodal node between the
// word and the experience, so meaning accrues per concept across episodes
// and across languages. Words point at concepts through names edges;
// concepts point at the episodes that instantiate them through grounds
// edges. Many words may name one concept, which is all the machinery
// cross-lingual synonymy needs.
//
// Concepts are born during consolidation, the same pass that merges
// recurring shapes into prototypes: a shape that has recurred enough to
// earn a prototype has earned an abstraction. Serotonin paces how many
// may be born in one night, and dopamine gates how readily a word bonds
// to a concept the next time naming runs.
const (
	conceptBudgetScale = 16 // concepts mintable in one night at full serotonin
	groundsWeight      = 0.8
	// Naming is cross-situational: a bond's strength is the reliability
	// of the pairing, not the count of meetings. The smoothing mass is
	// about one co-occurrence, so a single meeting reads as half
	// evidence; the gain scales reliability into link-weight range with
	// dopamine deciding how readily words bond at all; bonds that would
	// start below the write floor are not worth an edge.
	namesSmoothing    = 0.3
	namesGainBase     = 0.6
	namesDopamineGain = 0.75
	namesMaxWeight    = 0.95
	namesWriteFloor   = 0.1
	// deliberateName is the weight of a name asserted outright through the name
	// command, firm but below the lived ceiling so usage can still move it. It
	// is the pointer made directly, where cross-situational naming would have
	// to infer it from company.
	deliberateName = 0.9
	// A name well below the concept's firmest is a weak pointer, spoken
	// tentatively. The gap reads the reliability spread, so it tracks
	// how much less faithfully a word kept the concept's company, not
	// the shared dopamine gain that lifts every name alike.
	namesContrastGap = 0.12
	// The anti-Hebbian step: how much a bond weakens each time its word
	// recurs in a window whose active concepts exclude it. A firm bond
	// takes several misses to fall, the way one coincidence took only a
	// share of a night to form.
	namesDepression = 0.08
)

// Abstraction by constellation: a word that never settles on one
// sensory shape but keeps stable company with the same small cloud of
// concepts across situations is naming something those concepts only
// jointly carry. Genesis extends to that case: an abstract concept is
// born grounded in its constellation (grounds edges to concepts rather
// than to episodes), so recall and the inner loop reach it the same way
// and the chain still bottoms out in concrete experience underneath.
const (
	absMinSituations = 3    // a constellation takes several showings to trust
	absCoreFloor     = 0.05 // below this slice of a word's company a concept is noise, not a member
	absCoreFraction  = 0.5  // a member must hold this fraction of the strongest member's share, so a broad category whose company spreads thin still gathers a core
	absConcentration = 0.5  // a word this faithful to one concept is a name, an abstraction is not
	absPerNight      = 4    // abstractions mintable in one night
	absNameWeight    = 0.8  // the word's bond to its own abstraction
	absRefreshFloor  = 2    // a core smaller than this has dissolved; stop refreshing and let decay take it
)

// growAbstractions runs after naming in the same consolidation pass:
// for each word whose lived company shows a stable conceptual core with
// no single dominant shape, mint (or refresh) an abstract concept whose
// grounds edges carry the reliability shares of its constellation. A
// constellation that dissolves stops being refreshed, its grounds edges
// decay with the nightly link fade, and the concept dies of
// disconnection like anything else no longer lived.
func (d *Daemon) growAbstractions() (born int) {
	pairs, err := d.store.NamingPairs()
	if err != nil {
		fmt.Fprintf(d.out, "abstraction error: %v\n", err)
		return 0
	}
	byWord := map[uint64][]memory.NamingPair{}
	wordIDs := make([]uint64, 0, len(pairs))
	for _, p := range pairs {
		if len(byWord[p.Word]) == 0 {
			wordIDs = append(wordIDs, p.Word)
		}
		byWord[p.Word] = append(byWord[p.Word], p)
	}
	words, err := d.store.FragmentsByIDs(wordIDs)
	if err != nil {
		fmt.Fprintf(d.out, "abstraction error: %v\n", err)
		return 0
	}
	sort.Slice(words, func(i, j int) bool { return words[i].ID < words[j].ID })

	for _, w := range words {
		type coreShare struct {
			concept uint64
			share   float64
		}
		shares := make([]coreShare, 0, len(byWord[w.ID]))
		maxShare := 0.0
		for _, p := range byWord[w.ID] {
			share := p.Cooc / (p.WordTotal + namesSmoothing)
			shares = append(shares, coreShare{concept: p.Concept, share: share})
			if share > maxShare {
				maxShare = share
			}
		}
		// Core members are those in the same league as the strongest, above a
		// noise floor. Judging core relative to the peak rather than against a
		// fixed share lets a broad category, whose company spreads thin across
		// many members, still gather a core; the old absolute bar only a narrow
		// naming could clear left a category dissolving the more faithfully it
		// was taught.
		cut := absCoreFloor
		if rel := absCoreFraction * maxShare; rel > cut {
			cut = rel
		}
		var core []coreShare
		for _, s := range shares {
			if s.share >= cut {
				core = append(core, s)
			}
		}
		sort.Slice(core, func(i, j int) bool { return core[i].concept < core[j].concept })

		tpl := "abs:" + w.Lang + ":" + w.Content
		c, err := d.store.ConceptByTemplate(tpl)
		if err != nil {
			fmt.Fprintf(d.out, "abstraction error: %v\n", err)
			return born
		}
		// The full conditions gate a mint; an existing abstraction keeps
		// refreshing while its core stands, however the drift around it
		// moves.
		healthy := w.Count >= absMinSituations && len(core) >= absRefreshFloor && maxShare < absConcentration
		// A word that names a relation is a verb, every inflection of it too: its
		// lived company is the relation's arguments, not a constellation, so
		// abstracting it into a noun would only mint a shadow that shrouds the
		// relation in the naming vote. Skip it, and any earlier shadow stops
		// refreshing and decays.
		if d.namesRelation(w.Content, w.Lang) {
			continue
		}
		switch {
		case c == nil:
			if !healthy || born >= absPerNight {
				continue
			}
			c = &memory.Fragment{
				Kind:     memory.KindConcept,
				Channel:  "concept",
				Content:  w.Content,
				Template: tpl,
				Category: "noun",
				Limbic:   d.chem.Snapshot(),
				Time:     time.Now(),
			}
			if _, err := d.store.Encode(c); err != nil {
				fmt.Fprintf(d.out, "abstraction mint error: %v\n", err)
				continue
			}
			born++
			d.event(events.Event{Ev: "concept", ID: c.ID, Tpl: wire.Clip(tpl, 120), Outcome: "abstract"})
		case len(core) < absRefreshFloor:
			continue // dissolved: decay carries the grounding away
		}
		// The constellation is the grounding: each edge's weight is the
		// concept's reliability share in the word's company, written
		// fresh each night, so the chain to concrete experience stays
		// auditable and current.
		for _, cs := range core {
			if err := d.store.SetLinkRel(c.ID, cs.concept, memory.RelGrounds, cs.share); err != nil {
				fmt.Fprintf(d.out, "abstraction grounding error: %v\n", err)
			}
		}
		if err := d.store.SetLinkRel(w.ID, c.ID, memory.RelNames, absNameWeight); err != nil {
			fmt.Fprintf(d.out, "abstraction naming error: %v\n", err)
		}
	}
	return born
}

// lemmaCategory derives a concept's syntactic category from its grounding
// role: a shape in command position is verb-like, a recurring sensory
// object is noun-like.
func lemmaCategory(proto *memory.Fragment) string {
	if proto.Channel == "shell" && strings.HasPrefix(proto.Content, senses.ActionPrefix) {
		return "verb"
	}
	return "noun"
}

// glossFor is the speakable face of a concept: its source template, with
// the command marker stripped from shapes that live in command position.
func glossFor(proto *memory.Fragment) string {
	if proto.Channel == "shell" && strings.HasPrefix(proto.Template, senses.ActionPrefix) {
		return strings.TrimSpace(strings.TrimPrefix(proto.Template, senses.ActionPrefix))
	}
	return proto.Template
}

// growConcepts is concept genesis, run during sleep: shapes that have
// recurred into prototypes are distilled into concept nodes, every
// concept is grounded to the episodes matching its shape, and words bond
// to the concepts their episodes ground. Serotonin decides how many
// concepts one night may mint; dopamine decides how eagerly words bond.
func (d *Daemon) growConcepts() (born, named int) {
	budget := int(d.chem.Serotonin() * conceptBudgetScale)
	candidates, err := d.store.ConceptCandidates(memory.MergeThreshold, budget)
	if err != nil {
		fmt.Fprintf(d.out, "concept genesis error: %v\n", err)
		return 0, 0
	}
	for _, proto := range candidates {
		c := &memory.Fragment{
			Kind:     memory.KindConcept,
			Channel:  "concept",
			Content:  glossFor(proto),
			Template: proto.Template,
			Category: lemmaCategory(proto),
			Limbic:   d.chem.Snapshot(),
			Time:     time.Now(),
		}
		if _, err := d.store.Encode(c); err != nil {
			fmt.Fprintf(d.out, "concept mint error: %v\n", err)
			continue
		}
		born++
		d.event(events.Event{Ev: "concept", ID: c.ID, Tpl: wire.Clip(c.Template, 120), Outcome: c.Category})
	}

	if _, err := d.store.Reground(groundsWeight); err != nil {
		fmt.Fprintf(d.out, "reground error: %v\n", err)
	}

	pairs, err := d.store.NamingPairs()
	if err != nil {
		fmt.Fprintf(d.out, "naming error: %v\n", err)
		return born, 0
	}
	conceptTotal := map[uint64]float64{}
	wordIDs := make([]uint64, 0, len(pairs))
	conceptIDs := make([]uint64, 0, len(pairs))
	seenWord := map[uint64]bool{}
	seenConcept := map[uint64]bool{}
	for _, p := range pairs {
		conceptTotal[p.Concept] += p.Cooc
		if !seenWord[p.Word] {
			seenWord[p.Word] = true
			wordIDs = append(wordIDs, p.Word)
		}
		if !seenConcept[p.Concept] {
			seenConcept[p.Concept] = true
			conceptIDs = append(conceptIDs, p.Concept)
		}
	}
	// A verb names its relation, not the arguments it keeps company with. A word
	// that names a relation, then, does not also name a noun: its bond to an
	// argument concept is cleared so the naming vote stays on the relation rather
	// than drifting to the nouns the verb took. relConcept marks which targets
	// are the relation itself, the one bond such a word keeps.
	verbWord := map[uint64]bool{}
	if wfrags, err := d.store.FragmentsByIDs(wordIDs); err == nil {
		for _, wf := range wfrags {
			if d.namesRelation(wf.Content, wf.Lang) {
				verbWord[wf.ID] = true
			}
		}
	}
	relConcept := map[uint64]bool{}
	if cfrags, err := d.store.FragmentsByIDs(conceptIDs); err == nil {
		for _, cf := range cfrags {
			if cf.Category == "relation" {
				relConcept[cf.ID] = true
			}
		}
	}
	gain := namesGainBase + namesDopamineGain*d.chem.Snapshot().Dopamine
	for _, p := range pairs {
		// Cross-situational reliability: the share of the word's company
		// that lands on this concept, blended with the share of the
		// concept's company that is this word. A promiscuous word splits
		// its mass across its targets and bonds weakly to each; a word
		// paired with one concept nearly every time keeps almost all of
		// its mass there. The bond is set from the fresh ratio, down as
		// well as up, so a word that drifts to new company loosens its
		// old bonds at the next consolidation.
		pcw := p.Cooc / (p.WordTotal + namesSmoothing)
		pwc := p.Cooc / (conceptTotal[p.Concept] + namesSmoothing)
		w := math.Sqrt(pcw*pwc) * gain
		if w > namesMaxWeight {
			w = namesMaxWeight
		}
		// A verb's bond to an argument noun is no name: clear it so the vote
		// stays on the relation. A standing bond is set down to nothing, a new
		// one never written, while the verb keeps its bond to the relation
		// itself.
		if verbWord[p.Word] && !relConcept[p.Concept] {
			w = 0
		}
		if p.Existing == 0 && w < namesWriteFloor {
			continue
		}
		if err := d.store.SetLinkRel(p.Word, p.Concept, memory.RelNames, w); err != nil {
			fmt.Fprintf(d.out, "naming link error: %v\n", err)
			continue
		}
		if p.Existing == 0 {
			named++
			d.event(events.Event{Ev: "name", From: p.Word, To: p.Concept, Rel: memory.RelNames, Weight: w})
		}
	}
	return born, named
}

// Attention-grounding: a told word binds to what is attended, not only to
// what is in view. The warmest concepts in the workspace lend their
// grounded experience as binding targets, so naming a recalled or
// thought-about concept points the word there even with nothing perceptual
// present. This is the brain's own arrangement, a word for what is in mind
// and not only what is sensed; cross-situational naming still discards a
// warming that does not recur, so it does not become a new junk source.
const (
	attentionGroundingGate = 0.4 // a concept must be at least this warm to lend its experience
	attentionAnchorCap     = 4   // grounded moments of an attended concept a told word binds to
	attentionConceptCap    = 2   // how many warm concepts can be attended at once
)

// attendedWarmest returns the n hottest concepts after lateral
// inhibition: a concept warm enough to attend damps the concepts it
// grounds in, so a category held in mind quiets its members instead of
// being swamped by them. The damping scales with the inhibitor's own
// warmth and the cfg.LateralInhibition knob; at zero this is plain
// warmest. A concept with no attended parent above it keeps its warmth,
// so a specific merely perceived is left alone.
func (d *Daemon) attendedWarmest(n int) []warmEntry {
	all := d.ws.warmest(workspaceBudget)
	strength := d.cfg.LateralInhibition
	if strength <= 0 || len(all) == 0 {
		if len(all) > n {
			return all[:n]
		}
		return all
	}
	heat := make(map[uint64]float64, len(all))
	inhibitors := make([]uint64, 0, len(all))
	for _, e := range all {
		heat[e.id] = e.heat
		if e.heat >= attentionGroundingGate {
			inhibitors = append(inhibitors, e.id)
		}
	}
	damp := make(map[uint64]float64, len(all))
	if grounds, err := d.store.LinksFromRel(inhibitors, memory.RelGrounds); err == nil {
		for _, g := range grounds {
			if _, held := heat[g.To]; !held {
				continue
			}
			damp[g.To] += strength * heat[g.From]
		}
	}
	out := make([]warmEntry, 0, len(all))
	for _, e := range all {
		h := e.heat - damp[e.id]
		if h < 0 {
			h = 0
		}
		out = append(out, warmEntry{id: e.id, heat: h})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].heat != out[j].heat {
			return out[i].heat > out[j].heat
		}
		return d.ws.touchOf(out[i].id) > d.ws.touchOf(out[j].id)
	})
	if len(out) > n {
		out = out[:n]
	}
	return out
}

// attentionAnchors returns the grounded moments of the warmest workspace
// concepts above the attention gate, newest first and capped, so a told
// word can keep company with what is held in mind. The moments are the
// concept's own direct groundings, so the word grounds in that concept
// rather than in its specifics. Lateral inhibition picks the warmest, so
// an attended category wins the slot over the members it subsumes.
func (d *Daemon) attentionAnchors() []recentMoment {
	warm := d.attendedWarmest(attentionConceptCap)
	var out []recentMoment
	for _, e := range warm {
		if e.heat < attentionGroundingGate {
			continue
		}
		frags, err := d.store.FragmentsByIDs([]uint64{e.id})
		if err != nil || len(frags) != 1 || frags[0].Kind != memory.KindConcept {
			continue
		}
		grounds, err := d.store.LinksFromRel([]uint64{e.id}, memory.RelGrounds)
		if err != nil {
			continue
		}
		epIDs := make([]uint64, 0, len(grounds))
		for _, g := range grounds {
			epIDs = append(epIDs, g.To)
		}
		eps, err := d.store.FragmentsByIDs(epIDs)
		if err != nil {
			continue
		}
		sort.Slice(eps, func(i, j int) bool { return eps[i].ID > eps[j].ID })
		n := 0
		for _, f := range eps {
			if f.Kind != memory.KindEpisodic {
				continue
			}
			out = append(out, recentMoment{id: f.ID, template: f.Template, at: f.Time})
			if n++; n >= attentionAnchorCap {
				break
			}
		}
		// A higher abstraction grounds in concepts, not episodes, so it
		// has no episodic moment to lend even though it is what is held in
		// mind. Bind the word to the concept itself, so a name taught
		// across several attended categories grounds in those categories
		// rather than falling through to one of their members.
		if n == 0 {
			out = append(out, recentMoment{id: frags[0].ID, template: frags[0].Template, at: frags[0].Time})
		}
	}
	return out
}

// readVocabulary mints the words of a line read on an untrusted channel into the
// lexicon at the untrusted scale. It is the bridge that lets reading teach
// vocabulary while the trust model keeps what is read from cueing a habit or
// anchoring a command: a word grows familiar by recurring across the watched
// sources, never by appearing once. The words bond to the line they were read
// in, so the night's naming pass can ground them from the company they keep.
func (d *Daemon) readVocabulary(content string, fragID uint64) {
	d.mintWordsScaled(content, "en", fragID, nil, false, untrustedNameScale)
}

// mintWords stores or refreshes word nodes for the content words of an
// utterance and binds each to the tell moment and to the moments the
// utterance bound to, so the night's naming pass can see which
// experiences the words kept company with. A deliberate labeling (tell
// --about) mints every word, closed class included: an explicit label
// may claim a form the grammar holds, and sustained usage of that kind
// is what can out-weigh a seeded parameter. The bond is at full trust;
// mintWordsScaled lets a reading channel bond more lightly.
func (d *Daemon) mintWords(utterance, lang string, tellID uint64, anchors []recentMoment, deliberate bool) []*memory.Fragment {
	return d.mintWordsScaled(utterance, lang, tellID, anchors, deliberate, 1.0)
}

// mintWordsScaled is mintWords with the bond weight scaled by the source's
// trust, so a word read on an untrusted channel bonds lightly and entrenches
// only by recurring across reads.
func (d *Daemon) mintWordsScaled(utterance, lang string, tellID uint64, anchors []recentMoment, deliberate bool, scale float64) []*memory.Fragment {
	if lang == "" {
		lang = "en"
	}
	forms := d.gram.ContentTokens(utterance)
	if deliberate {
		forms = grammar.AllTokens(utterance)
	}
	if len(forms) == 0 {
		return nil
	}
	da := d.chem.Snapshot().Dopamine
	// The bond a word earns is scaled by the source's trust: a word read on an
	// untrusted channel bonds lightly, so it entrenches only by recurring across
	// many reads and a word from a single page fades.
	w := (linkBaseWeight + linkDopamineGain*da) * scale
	var words []*memory.Fragment
	for _, form := range forms {
		word, minted, err := d.store.WordNode(form, lang, d.chem.Snapshot(), time.Now())
		if err != nil {
			fmt.Fprintf(d.out, "word mint error: %v\n", err)
			continue
		}
		if minted {
			d.event(events.Event{Ev: "word", ID: word.ID, Channel: lang, Tpl: form})
		} else {
			d.depressStaleNames(word, anchors)
		}
		targets := make([]uint64, 0, len(anchors)+1)
		if tellID != 0 {
			targets = append(targets, tellID)
		}
		for _, a := range anchors {
			if a.id != word.ID {
				targets = append(targets, a.id)
			}
		}
		for _, id := range targets {
			if err := d.store.Link(word.ID, id, w); err != nil {
				fmt.Fprintf(d.out, "word link error: %v\n", err)
				continue
			}
			d.store.Link(id, word.ID, w)
		}
		words = append(words, word)
	}
	return words
}

// depressStaleNames is heterosynaptic depression, the negative half of
// cross-situational learning: a word that recurs in a window whose
// active concepts do not include one it is bonded to was paired with
// that concept by coincidence, and the bond weakens at once instead of
// waiting for the night's recount. A window with no active concepts is
// no evidence either way: silence does not disconfirm. Bonds to
// abstractions and to mental operations are exempt: their referents are
// never directly in a sensory window (only their constellations are),
// so window-based disconfirmation does not apply; what dissolves them
// is the nightly share recount and link decay.
func (d *Daemon) depressStaleNames(word *memory.Fragment, anchors []recentMoment) {
	if len(anchors) == 0 {
		return
	}
	active := map[uint64]bool{}
	for _, a := range anchors {
		if c, err := d.store.ConceptByTemplate(a.template); err == nil && c != nil {
			active[c.ID] = true
		}
	}
	if len(active) == 0 {
		return
	}
	names, err := d.store.LinksFromRel([]uint64{word.ID}, memory.RelNames)
	if err != nil {
		return
	}
	var stale []memory.LinkEdge
	staleIDs := make([]uint64, 0, len(names))
	for _, e := range names {
		if !active[e.To] {
			stale = append(stale, e)
			staleIDs = append(staleIDs, e.To)
		}
	}
	if len(stale) == 0 {
		return
	}
	exempt, err := d.conceptsGroundedBeyondEpisodes(staleIDs)
	if err != nil {
		fmt.Fprintf(d.out, "naming depression error: %v\n", err)
		return
	}
	kept := stale[:0]
	for _, e := range stale {
		if !exempt[e.To] {
			kept = append(kept, e)
		}
	}
	if len(kept) == 0 {
		return
	}
	if err := d.store.NudgeLinks(kept, -namesDepression); err != nil {
		fmt.Fprintf(d.out, "naming depression error: %v\n", err)
	}
}

// conceptsGroundedBeyondEpisodes reports which of the given concepts
// ground in something other than episodes: an abstraction's
// constellation or an operation node.
func (d *Daemon) conceptsGroundedBeyondEpisodes(ids []uint64) (map[uint64]bool, error) {
	grounds, err := d.store.LinksFromRel(ids, memory.RelGrounds)
	if err != nil {
		return nil, err
	}
	targetIDs := make([]uint64, 0, len(grounds))
	for _, g := range grounds {
		targetIDs = append(targetIDs, g.To)
	}
	targets, err := d.store.FragmentsByIDs(targetIDs)
	if err != nil {
		return nil, err
	}
	kind := make(map[uint64]string, len(targets))
	for _, f := range targets {
		kind[f.ID] = f.Kind
	}
	out := map[uint64]bool{}
	for _, g := range grounds {
		if k, ok := kind[g.To]; ok && k != memory.KindEpisodic {
			out[g.From] = true
		}
	}
	return out, nil
}

// conceptInfos assembles the wire view of the concept layer: every
// concept with the words that name it, the experience that grounds it,
// and its workspace heat.
func (d *Daemon) conceptInfos() ([]*wire.ConceptInfo, error) {
	concepts, err := d.store.Concepts()
	if err != nil {
		return nil, err
	}
	tally, err := d.store.GroundingTally()
	if err != nil {
		return nil, err
	}
	ids := make([]uint64, 0, len(concepts))
	for _, c := range concepts {
		ids = append(ids, c.ID)
	}
	names, err := d.store.LinksToRel(ids, memory.RelNames)
	if err != nil {
		return nil, err
	}
	wordIDs := make([]uint64, 0, len(names))
	for _, e := range names {
		wordIDs = append(wordIDs, e.From)
	}
	words, err := d.store.FragmentsByIDs(wordIDs)
	if err != nil {
		return nil, err
	}
	wordByID := make(map[uint64]*memory.Fragment, len(words))
	for _, w := range words {
		wordByID[w.ID] = w
	}
	namesOf := make(map[uint64][]string)
	for _, e := range names {
		if w := wordByID[e.From]; w != nil {
			namesOf[e.To] = append(namesOf[e.To], fmt.Sprintf("%s(%s)", w.Content, w.Lang))
		}
	}
	featEdges, err := d.store.LinksFromRel(ids, memory.RelFeature)
	if err != nil {
		return nil, err
	}
	featIDs := make([]uint64, 0, len(featEdges))
	for _, e := range featEdges {
		featIDs = append(featIDs, e.To)
	}
	feats, err := d.store.FragmentsByIDs(featIDs)
	if err != nil {
		return nil, err
	}
	featByID := make(map[uint64]string, len(feats))
	for _, f := range feats {
		featByID[f.ID] = f.Content
	}
	featuresOf := make(map[uint64][]string)
	for _, e := range featEdges {
		if name, ok := featByID[e.To]; ok {
			featuresOf[e.From] = append(featuresOf[e.From], name)
		}
	}
	infos := make([]*wire.ConceptInfo, 0, len(concepts))
	for _, c := range concepts {
		sort.Strings(namesOf[c.ID])
		sort.Strings(featuresOf[c.ID])
		infos = append(infos, &wire.ConceptInfo{
			ID:       c.ID,
			Gloss:    c.Content,
			Category: c.Category,
			Names:    namesOf[c.ID],
			Grounded: tally[c.ID],
			Heat:     d.ws.heat(c.ID),
			Features: featuresOf[c.ID],
		})
	}
	return infos, nil
}

// warmInfos resolves the workspace's hottest concepts into glosses, for
// status display.
func (d *Daemon) warmInfos(n int) []*wire.WarmInfo {
	entries := d.ws.warmest(n)
	if len(entries) == 0 {
		return nil
	}
	ids := make([]uint64, 0, len(entries))
	for _, e := range entries {
		ids = append(ids, e.id)
	}
	frags, err := d.store.FragmentsByIDs(ids)
	if err != nil {
		return nil
	}
	gloss := make(map[uint64]string, len(frags))
	for _, f := range frags {
		gloss[f.ID] = f.Content
	}
	out := make([]*wire.WarmInfo, 0, len(entries))
	for _, e := range entries {
		if g, ok := gloss[e.id]; ok {
			out = append(out, &wire.WarmInfo{Gloss: g, Heat: e.heat})
		}
	}
	return out
}

// warmConcepts excites the concepts the given words name: speaking about
// something is what keeps it in working memory.
func (d *Daemon) warmConcepts(words []*memory.Fragment, amount float64) {
	if len(words) == 0 {
		return
	}
	ids := make([]uint64, 0, len(words))
	for _, w := range words {
		ids = append(ids, w.ID)
	}
	names, err := d.store.LinksFromRel(ids, memory.RelNames)
	if err != nil {
		return
	}
	for _, e := range names {
		d.ws.excite(e.To, amount)
	}
}

// semanticSeeds reads a query through the lexicon: its words name
// concepts, the concepts ground episodes, and those episodes join the
// recall as seeds even when they share no substring with the query. The
// walked names and grounds edges gain practice like any recalled path.
// The mediating word and concept nodes come back too, so a reply can
// show the path meaning took.
func (d *Daemon) semanticSeeds(query string) ([]*memory.Fragment, []*wire.FragmentInfo) {
	tokens, _ := grammar.Tokenize(query)
	words, err := d.store.WordNodesByForms(tokens)
	if err != nil {
		fmt.Fprintf(d.out, "semantic recall error: %v\n", err)
		return nil, nil
	}

	var walked []memory.LinkEdge
	conceptIDs := map[uint64]bool{}
	wordIDs := make([]uint64, 0, len(words))
	for _, w := range words {
		wordIDs = append(wordIDs, w.ID)
	}
	names, err := d.store.LinksFromRel(wordIDs, memory.RelNames)
	if err != nil {
		fmt.Fprintf(d.out, "semantic recall error: %v\n", err)
		return nil, nil
	}
	for _, e := range names {
		conceptIDs[e.To] = true
		walked = append(walked, e)
	}
	// A phrase no word names yet can still reach a concept by content,
	// the same fallback episodic recall has always used.
	if glossed, err := d.store.ConceptsByGloss(query, 3); err == nil {
		for _, c := range glossed {
			conceptIDs[c.ID] = true
		}
	}
	if len(conceptIDs) == 0 {
		return nil, nil
	}

	cids := make([]uint64, 0, len(conceptIDs))
	for id := range conceptIDs {
		cids = append(cids, id)
	}
	grounds, err := d.store.LinksFromRel(cids, memory.RelGrounds)
	if err != nil {
		fmt.Fprintf(d.out, "semantic recall error: %v\n", err)
		return nil, nil
	}
	episodeIDs := make([]uint64, 0, len(grounds))
	for _, e := range grounds {
		episodeIDs = append(episodeIDs, e.To)
		walked = append(walked, e)
	}
	episodes, err := d.fragmentsByIDsTiered(episodeIDs)
	if err != nil {
		fmt.Fprintf(d.out, "semantic recall error: %v\n", err)
		return nil, nil
	}
	// Reaching an episode through meaning is retrieval practice for the
	// episode and for every edge the meaning walked, and it warms the
	// concepts it walked through.
	if err := d.store.Refresh(episodeIDs); err == nil {
		for _, f := range episodes {
			f.Weight = 1.0
		}
	}
	if len(walked) > 0 {
		d.store.StrengthenLinks(walked, linkPracticeDelta)
	}
	for id := range conceptIDs {
		d.ws.excite(id, exciteRecall)
	}

	var mediators []*wire.FragmentInfo
	for _, w := range words {
		info := fragmentInfo(w)
		info.Activation = 0.9
		mediators = append(mediators, info)
	}
	if concepts, err := d.store.FragmentsByIDs(cids); err == nil {
		for _, c := range concepts {
			info := fragmentInfo(c)
			info.Activation = 0.9
			mediators = append(mediators, info)
		}
	}
	return episodes, mediators
}
