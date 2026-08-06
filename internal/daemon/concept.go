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
// earn a prototype has earned an abstraction.
const (
	// genesisCap is the most concepts one consolidation forms: the recurring
	// shapes that crossed the merge threshold, taken in count order up to this
	// many. Formation is paced by what is ready, a capacity limit, not by mood,
	// so a deliberate nap during teaching forms the percepts it has even with
	// sleep pressure at its floor. An alarmed system still does not abstract,
	// because it does not reach a consolidation: the sleep gate withholds the
	// permission. Once consolidating, it forms what it took in.
	genesisCap    = 16
	groundsWeight = 0.8
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
	// A word whose whole lived company has decayed below this mass has no
	// standing to re-litigate its namings from the ratio of what little company
	// remains. A basic name that reaches neither the ostensive nor the
	// cross-tongue refresh (an English head-word once its own percept thins out
	// of its core) would otherwise be thinned toward nothing by the recount
	// while a colour it shares company with, or its cross-tongue twin kept firm
	// by the borrow, stands over it (lion surrendered to silver, dog to cat).
	// Below the floor the earned bond is left as it stands and no new bond is
	// born; a word with real company (whole units of mass) recounts as before,
	// and a rehearsed word resumes recounting as its company rebuilds.
	namesEvidenceFloor = 0.5
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
	// namesUnpointedKeep is the share a naming bond keeps when the pointing contradicts
	// it. A scene-mate bond falls under the write floor within a couple of nights, while a
	// bond whose pointing merely aged out of the graph has that long to be shown again.
	namesUnpointedKeep = 0.5
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
	// absPwcFloor is the size-invariant membership bar. The word-side share (a member's
	// slice of the category word's whole company) dilutes as ~1/N as a category gains
	// members, so the absolute absCoreFloor silently dissolves a broad category once it
	// passes ~20 members. The concept-side share -- how much of THIS percept's naming
	// company is the category word -- does not dilute with category size (whether "animal"
	// is a strong namer of the tiger is independent of how many other animals it names), so
	// a concept whose company is at least this fraction the category word's is a reliable
	// member however broad the category, while a one-shot co-namer sits an order below.
	absPwcFloor      = 0.10
	absConcentration = 0.5 // a word this faithful to one concept is a name, an abstraction is not
	// ostensiveDominance is the share of a word's whole ostension one percept must hold
	// for the word to count as that percept's name. A name points at one thing every
	// telling, so its ostension piles up there; a clear majority distinguishes it from a
	// category, whose ostension spreads across the members it was said over, and from a
	// near-even split, which withholds rather than guessing a referent.
	ostensiveDominance = 0.6
	absPerNight        = 4   // abstractions mintable in one night
	absNameWeight      = 0.8 // the word's bond to its own abstraction
	absRefreshFloor    = 2   // a core smaller than this has dissolved; stop refreshing and let decay take it
	// crossTongueTwinOverlap is how much of an other-tongue word's percept company
	// must be shared with a category word before it counts as that word's translation
	// twin, whose agreement corroborates a candidate member. A real translation names
	// nearly the same percepts (overlap approaches the whole of the smaller company);
	// another category that merely shares a stray scene-mate falls well below this, so
	// it cannot vouch for the scene-mate it dragged in.
	crossTongueTwinOverlap = 0.7
)

// growAbstractions runs after naming in the same consolidation pass:
// for each word whose lived company shows a stable conceptual core with
// no single dominant shape, mint (or refresh) an abstract concept whose
// grounds edges carry the reliability shares of its constellation. A
// constellation that dissolves stops being refreshed, its grounds edges
// decay with the nightly link fade, and the concept dies of
// disconnection like anything else no longer lived.
// grownConcepts returns the concepts the nightly consolidation works over, leaving out the grammar
// scaffolding (argument constructions, declarative phrase rules and lexical forms, grammatical classes)
// that now persists in the graph as a fluid weight. Scaffolding is grammar living in the graph rather
// than a perceptual concept, so it is not embedded or featured; this keeps consolidation's view of the
// grown layer the same as before the scaffolding was spared the nightly prune.
func (d *Daemon) grownConcepts() ([]*memory.Fragment, error) {
	concepts, err := d.store.Concepts()
	if err != nil {
		return nil, err
	}
	var out []*memory.Fragment
	for _, c := range concepts {
		switch c.Category {
		case "construction", declGrammarCategory, "class", answerCxnCategory, sourceCategory, "number":
			continue
		}
		out = append(out, c)
	}
	return out, nil
}

// abstractionIndex is the naming graph the abstraction pass walks, read once from the
// store and held in memory for the pass. wordsByForm mirrors WordNodesByForms (keyed by
// lower-cased content), namesByWord mirrors the outgoing "names" edges of a word node,
// conceptByTemplate mirrors the concept-by-template lookup, and grounded is the set of
// concepts carrying a grounding or feature edge (the complement of perceptless). The pass
// mutates the graph as it runs, so its own naming and mint writes are folded back in
// through the note methods, keeping the index a faithful mirror the later words read.
type abstractionIndex struct {
	wordsByForm       map[string][]*memory.Fragment
	namesByWord       map[uint64]map[uint64]float64
	conceptByTemplate map[string]*memory.Fragment
	grounded          map[uint64]bool
	relationByID      map[uint64]*memory.Fragment
}

// asciiLower folds only A-Z to a-z and leaves every other byte untouched, mirroring the
// pure-Go SQLite driver's built-in lower(). The abstraction index keys each word node with
// it, matching how WordNodesByForms folds the stored content (`lower(content)` in SQL); the
// lookup side folds the query form with Go's Unicode ToLower, matching how that method folds
// its argument, so the two-sided fold reproduces the store's exactly. A UTF-8 continuation
// byte never lands in the A-Z range, so folding by byte cannot corrupt a rune.
func asciiLower(s string) string {
	var b []byte
	for i := 0; i < len(s); i++ {
		if c := s[i]; c >= 'A' && c <= 'Z' {
			if b == nil {
				b = []byte(s)
			}
			b[i] = c + ('a' - 'A')
		}
	}
	if b == nil {
		return s
	}
	return string(b)
}

func (d *Daemon) buildAbstractionIndex() (*abstractionIndex, error) {
	words, err := d.store.NodesByKind(memory.KindWord)
	if err != nil {
		return nil, err
	}
	concepts, err := d.store.Concepts()
	if err != nil {
		return nil, err
	}
	idx := &abstractionIndex{
		wordsByForm:       make(map[string][]*memory.Fragment),
		namesByWord:       make(map[uint64]map[uint64]float64),
		conceptByTemplate: make(map[string]*memory.Fragment, len(concepts)),
		grounded:          make(map[uint64]bool),
		relationByID:      make(map[uint64]*memory.Fragment),
	}
	wordIDs := make([]uint64, 0, len(words))
	for _, w := range words {
		key := asciiLower(w.Content)
		idx.wordsByForm[key] = append(idx.wordsByForm[key], w)
		wordIDs = append(wordIDs, w.ID)
	}
	conceptIDs := make([]uint64, 0, len(concepts))
	for _, c := range concepts {
		// ConceptByTemplate keeps the first row a template matches; the first-written
		// concept has the lower id, so seeding in store order (ascending id) leaves the
		// same winner when a template is somehow shared.
		if _, seen := idx.conceptByTemplate[c.Template]; !seen {
			idx.conceptByTemplate[c.Template] = c
		}
		if c.Category == "relation" {
			idx.relationByID[c.ID] = c
		}
		conceptIDs = append(conceptIDs, c.ID)
	}
	names, err := d.store.LinksFromRel(wordIDs, memory.RelNames)
	if err != nil {
		return nil, err
	}
	for _, e := range names {
		idx.noteName(e.From, e.To, e.Weight)
	}
	for _, rel := range []string{memory.RelGrounds, memory.RelFeature} {
		edges, err := d.store.LinksFromRel(conceptIDs, rel)
		if err != nil {
			return nil, err
		}
		for _, e := range edges {
			idx.grounded[e.From] = true
		}
	}
	return idx, nil
}

// noteName folds a "names" edge write into the index, so a word processed later reads the
// same naming graph the store now holds. wordNamesConcept only asks whether the edge
// exists, but the real weight is kept so a weight-reading reader would see the store's.
func (idx *abstractionIndex) noteName(from, to uint64, weight float64) {
	m := idx.namesByWord[from]
	if m == nil {
		m = make(map[uint64]float64)
		idx.namesByWord[from] = m
	}
	m[to] = weight
}

// noteConcept folds a freshly minted abstraction into the template lookup and marks it
// grounded, since it is minted with its constellation, so a later word resolves it as the
// store would rather than minting a duplicate.
func (idx *abstractionIndex) noteConcept(c *memory.Fragment) {
	if _, seen := idx.conceptByTemplate[c.Template]; !seen {
		idx.conceptByTemplate[c.Template] = c
	}
	idx.grounded[c.ID] = true
}

func (d *Daemon) growAbstractions() (born int) {
	pairs, err := d.store.NamingPairs()
	if err != nil {
		fmt.Fprintf(d.out, "abstraction error: %v\n", err)
		return 0
	}
	// The abstraction pass resolves every candidate word against the naming graph
	// dozens of times over; prefetch that graph once so those resolutions read memory
	// instead of the single-writer store per word. The index is consulted only through
	// the resolution helpers, which fall back to the store when it is absent, so the
	// answers are identical whether or not the pass ran with it.
	if !d.absIdxOff {
		if idx, err := d.buildAbstractionIndex(); err != nil {
			// A prefetch failure is not fatal: the resolution helpers fall back to the store,
			// reaching the same result the slower way rather than skipping the night's pass.
			fmt.Fprintf(d.out, "abstraction error: %v\n", err)
		} else {
			d.absIdx = idx
			defer func() { d.absIdx = nil }()
		}
	}
	byWord := map[uint64][]memory.NamingPair{}
	wordIDs := make([]uint64, 0, len(pairs))
	// A concept's total naming company, summed over every word that names it, is the
	// denominator of the size-invariant concept-side share (mirrors growConcepts).
	conceptTotal := map[uint64]float64{}
	for _, p := range pairs {
		if len(byWord[p.Word]) == 0 {
			wordIDs = append(wordIDs, p.Word)
		}
		byWord[p.Word] = append(byWord[p.Word], p)
		conceptTotal[p.Concept] += p.Cooc
	}
	words, err := d.store.FragmentsByIDs(wordIDs)
	if err != nil {
		fmt.Fprintf(d.out, "abstraction error: %v\n", err)
		return 0
	}
	sort.Slice(words, func(i, j int) bool { return words[i].ID < words[j].ID })

	// A word's ostensive salience, apart from its naming company: what it was pointed
	// at when it was uttered to name something. A name concentrates it on one percept,
	// a category spreads it across the members it was said over, so it tells the two
	// apart where the uniform company, diluted by a lingering scene-mate, cannot.
	ostPairs, err := d.store.OstensionPairs()
	if err != nil {
		fmt.Fprintf(d.out, "abstraction error: %v\n", err)
		return 0
	}
	ostByWord := map[uint64]map[uint64]float64{}
	ostTotal := map[uint64]float64{}
	for _, p := range ostPairs {
		if ostByWord[p.Word] == nil {
			ostByWord[p.Word] = map[uint64]float64{}
		}
		ostByWord[p.Word][p.Concept] += p.Mass
		ostTotal[p.Word] += p.Mass
	}

	// Cross-tongue corroboration of category membership. A category taught in two
	// tongues over the same percepts has its real members in BOTH tongues' company,
	// while a percept that kept the category word's company in one tongue only is an
	// incidental scene-mate the night would otherwise promote to membership (the eater
	// category warm beside the eaten food). The company a word keeps is the percepts
	// its lived episodes ground; a cross-tongue twin is an other-tongue word whose
	// company is mostly the same percepts -- a translation, not another category that
	// shares a stray member. A candidate no twin corroborates is left out; a word with
	// no twin keeps every candidate, so a monolingual category is unchanged.
	conceptIDs := make([]uint64, 0)
	seenConcept := map[uint64]bool{}
	for _, ps := range byWord {
		for _, p := range ps {
			if !seenConcept[p.Concept] {
				seenConcept[p.Concept] = true
				conceptIDs = append(conceptIDs, p.Concept)
			}
		}
	}
	perceptless := d.perceptlessSet(conceptIDs)
	// A member that is itself an abstraction was never pointed at and never could be:
	// there is no percept to show for a class, so it earns no ostension however
	// faithfully it was taught. It keeps the company bar, so the backbone a category
	// builds over its narrower categories stands whatever the ostensive gate does to
	// the percept members beside it.
	abstractMember := map[uint64]bool{}
	if d.cfg.OstensiveCoreEnabled {
		if frs, err := d.store.FragmentsByIDs(conceptIDs); err == nil {
			for _, f := range frs {
				if _, ok := abstractionLemma(f.Template); ok {
					abstractMember[f.ID] = true
				}
			}
		}
	}
	company := make(map[uint64]map[uint64]bool, len(words))
	for _, w := range words {
		set := make(map[uint64]bool)
		for _, p := range byWord[w.ID] {
			if !perceptless[p.Concept] {
				set[p.Concept] = true
			}
		}
		company[w.ID] = set
	}
	corroboratedBy := func(w *memory.Fragment) (map[uint64]bool, bool) {
		mine := company[w.ID]
		if len(mine) == 0 {
			return nil, false
		}
		union := map[uint64]bool{}
		found := false
		for _, other := range words {
			if other.ID == w.ID || other.Lang == w.Lang {
				continue
			}
			oc := company[other.ID]
			// A translation twin of a category is itself a category, naming several
			// percepts; a basic name names one. Skip a one-percept word, so a member's
			// own other-tongue name (whose only company is that member) cannot vouch for
			// it as if it were a category twin.
			if len(oc) < absRefreshFloor {
				continue
			}
			shared := 0
			for c := range mine {
				if oc[c] {
					shared++
				}
			}
			smaller := len(mine)
			if len(oc) < smaller {
				smaller = len(oc)
			}
			if float64(shared) >= crossTongueTwinOverlap*float64(smaller) {
				found = true
				for c := range oc {
					union[c] = true
				}
			}
		}
		return union, found
	}

	for _, w := range words {
		type coreShare struct {
			concept uint64
			share   float64 // pcw: this member's slice of the word's whole company
			pwc     float64 // this word's slice of the member's whole naming company
		}
		shares := make([]coreShare, 0, len(byWord[w.ID]))
		maxShare := 0.0
		for _, p := range byWord[w.ID] {
			share := p.Cooc / (p.WordTotal + namesSmoothing)
			pwc := p.Cooc / (conceptTotal[p.Concept] + namesSmoothing)
			shares = append(shares, coreShare{concept: p.Concept, share: share, pwc: pwc})
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
		corrob, hasTwin := corroboratedBy(w)
		// Two readings of the same company, kept apart because they answer different
		// questions. The company core answers whether the word is a name or a class, which
		// the guards below settle by how its company spreads over the percepts it might
		// refer to, so they go on reading the whole company. The core answers what the word
		// is a class OF, and there the company misleads: it reaches every percept still in
		// the naming window, so a dense stretch of teaching spreads a word across the whole
		// room, while the ostension stays on the one thing meant, a naming earning a single
		// edge to the freshest percept in view. So membership follows the pointing wherever
		// there was any, and the two readings coincide for a word never shown a thing.
		ostended := ostByWord[w.ID]
		ostensive := d.cfg.OstensiveCoreEnabled && len(ostended) > 0
		// A word that was read off its pointing before, and has none left tonight, is about
		// to have its class chosen by company again. The reading changes under an answer
		// that will not look any different, so it is said out loud: a basis that changes in
		// silence is the one thing the glass box cannot afford.
		if d.cfg.OstensiveCoreEnabled && len(ostended) == 0 && d.namedAbstraction(w) {
			d.event(events.Event{Ev: "concept", Tpl: wire.Clip("abs:"+w.Lang+":"+w.Content, 120),
				Outcome: "unpointed"})
		}
		var core, companyCore []coreShare
		for _, s := range shares {
			// A member is in the strongest's league on EITHER axis: the word-side
			// share, which a small category's members hold, OR the concept-side
			// reliability, which a broad category's members keep even as the
			// word-side share thins to ~1/N. A stray co-namer sits below both. The
			// concept-side floor is what stops a broad category dissolving the more
			// faithfully it was taught.
			kept := s.share >= cut || s.pwc >= absPwcFloor
			// A percept no cross-tongue twin corroborates kept this word's company in
			// one tongue only: an incidental scene-mate, not a member. Perceptless
			// co-members (an abstraction reached relationally, not through a tongue's
			// percepts) are exempt, since they have no percept company to corroborate.
			if hasTwin && !perceptless[s.concept] && !corrob[s.concept] {
				kept = false
			}
			if kept {
				companyCore = append(companyCore, s)
			}
			if ostensive && !abstractMember[s.concept] {
				if ostended[s.concept] > 0 {
					core = append(core, s)
				}
				continue
			}
			if kept {
				core = append(core, s)
			}
		}
		sort.Slice(core, func(i, j int) bool { return core[i].concept < core[j].concept })
		sort.Slice(companyCore, func(i, j int) bool { return companyCore[i].concept < companyCore[j].concept })
		// A class earns its place inside a broader class by what it covers, never by the
		// company its word kept: most of what it grounds has to fall under the category
		// it is being placed in. Company cannot tell a real rung from two words that
		// shared a room, and a wrong rung here costs far more than a wrong percept, since
		// the categories are climbed transitively and one bad edge puts everything the
		// narrower class holds under the broader one. That is what let an oak reach animal
		// through a category of parts.
		//
		// This asks nothing of the pointing, so it runs for every category word, not only
		// the ones that happen to have been shown something. A word carried wholly in talk
		// is where the coverage test is needed MOST: it never earns a pointing, so its
		// members are chosen by company alone, and it is exactly those words (a circle, a
		// rectangle, an idea) whose rungs hand a firm membership to everything beneath them.
		if d.cfg.OstensiveCoreEnabled {
			held := make(map[uint64]bool, len(core))
			for _, cs := range core {
				held[cs.concept] = true
			}
			kept := core[:0]
			for _, cs := range core {
				if !abstractMember[cs.concept] {
					kept = append(kept, cs)
					continue
				}
				sub, err := d.store.LinksFromRel([]uint64{cs.concept}, memory.RelGrounds)
				if err != nil || len(sub) == 0 {
					continue // grounds nothing, so there is no extension to place
				}
				covered := 0
				for _, e := range sub {
					if held[e.To] {
						covered++
					}
				}
				if float64(covered) >= kindLadderOverlap*float64(len(sub)) {
					kept = append(kept, cs)
				}
			}
			core = kept
		}

		tpl := "abs:" + w.Lang + ":" + w.Content
		var c *memory.Fragment
		if idx := d.absIdx; idx != nil {
			c = idx.conceptByTemplate[tpl]
		} else {
			var err error
			if c, err = d.store.ConceptByTemplate(tpl); err != nil {
				fmt.Fprintf(d.out, "abstraction error: %v\n", err)
				return born
			}
		}
		// The full conditions gate a mint; an existing abstraction keeps
		// refreshing while its core stands, however the drift around it
		// moves.
		// Read from the company alone, as the name-or-class guards below need it: whether a
		// word looks like a class is a fact about how its company spreads, and a word whose
		// pointing happens to be thin is still a class if it was taught like one.
		healthy := w.Count >= absMinSituations && len(companyCore) >= absRefreshFloor && maxShare < absConcentration
		// A word that names a relation is a verb, every inflection of it too: its
		// lived company is the relation's arguments, not a constellation, so
		// abstracting it into a noun would only mint a shadow that shrouds the
		// relation in the naming vote. Skip it, and any earlier shadow stops
		// refreshing and decays.
		if d.namesRelation(w.Content, w.Lang) {
			continue
		}
		coreIDs := make([]uint64, 0, len(core))
		for _, cs := range core {
			coreIDs = append(coreIDs, cs.concept)
		}
		// The referent hunt below reads the company core, never the pointed-at one. Whether
		// a word is a name is settled by how its company spreads over the percepts it might
		// refer to, and narrowing that field to the pointing would hand a word whose whole
		// ostension is one passing glance a firm name for the thing it glanced at.
		namingIDs := make([]uint64, 0, len(companyCore))
		for _, cs := range companyCore {
			namingIDs = append(namingIDs, cs.concept)
		}
		named := d.headNamedPercepts(namingIDs)
		// What the word was pointed at decides first, a basic name as much as a
		// cross-tongue one: a word shown over one percept every telling is that percept's
		// name, however its company spread to a scene-mate that lingered warm from the
		// block before, so it binds there firmly rather than diluting across both or
		// splitting a phantom beside them. A category, pointed at a different member each
		// telling, spreads its ostension and clears no single referent, so it falls
		// through to the company-based reading and abstracts.
		if ref := d.ostensiveReferent(w.ID, named, ostByWord[w.ID], ostTotal[w.ID]); ref != 0 {
			if err := d.store.SetLinkRel(w.ID, ref, memory.RelNames, absNameWeight); err != nil {
				fmt.Fprintf(d.out, "ostensive name error: %v\n", err)
			} else if d.absIdx != nil {
				d.absIdx.noteName(w.ID, ref, absNameWeight)
			}
			continue
		}
		// A basic word is a name, not a category: it abstracts nothing. When the
		// word heads the caption of a percept in its own core ("cat" heads "a small
		// black cat"), the constellation it gathered is its own kind standing among
		// its neighbours, not a class above them, so no abstraction is minted and a
		// shadow already grown stops refreshing and decays. A true superordinate
		// ("animal") heads no member's caption and abstracts as before.
		if d.headsCorePercept(w.Content, namingIDs) {
			continue
		}
		// With no ostension to point the way, a word that names a single basic-level
		// percept is taken as that percept's name in this tongue, not a category over it:
		// "kurt" names the wolf "wolf" already names, beside a nameless shape, so both
		// tongues share one node rather than splitting a parallel abs:tr:kurt the
		// propositions never reach. When the company spreads across SEVERAL named percepts
		// and exactly one of them is already owned, that owned percept is the referent of a
		// name-like word; a word already healthy as a category keeps its spread and abstracts
		// instead (the guard below), and an even split with no ostension to break it is left
		// to abstract too.
		existing := make(map[uint64]float64, len(byWord[w.ID]))
		for _, p := range byWord[w.ID] {
			existing[p.Concept] = p.Existing
		}
		target := uint64(0)
		switch {
		case len(named) == 1:
			target = named[0]
		case len(named) > 1:
			owned, ownedN := uint64(0), 0
			for _, id := range named {
				if existing[id] >= namesWriteFloor {
					owned, ownedN = id, ownedN+1
				}
			}
			// A word that spreads across several named percepts and owns exactly one
			// borrows that one ONLY when it is name-like. A word that would abstract into
			// a category is not seeking a single referent: a class spans its members, so
			// it must not re-home onto the one member it still owns when the others dip
			// below the naming floor, which is just what a re-grounded cast leaves (the
			// taught animals refreshed, one member un-shown, its bond alone above the
			// floor). Such a word falls through and abstracts instead.
			if ownedN == 1 && !healthy {
				target = owned
			}
		}
		// A word that already firmly names a percept of its own is not an unbound
		// second-tongue word seeking a referent: it has one. The cross-tongue
		// reuse must not re-home it onto a percept it merely keeps company with
		// through a relation (a cat named beside the dog it was told to be chased
		// by), which happens once the word's own percept decays out of its core
		// while the co-argument's stays. A genuine second-tongue synonym owns no
		// other percept, so it still borrows; a word with its own name keeps it.
		if target != 0 && d.namesOwnPercept(w, target) {
			continue
		}
		if target != 0 {
			// The bind must land on something real: a percept, or a concept this
			// word already names outright (an abstract noun refreshing its own
			// name). A perceptless stranger the word never named is a stray
			// attention edge, not a referent; promoting it would launder one
			// mis-binding into a firm name, the guard growConcepts already keeps.
			if existing[target] > 0 || !d.isPerceptless(target) {
				if err := d.store.SetLinkRel(w.ID, target, memory.RelNames, absNameWeight); err != nil {
					fmt.Fprintf(d.out, "cross-tongue name error: %v\n", err)
				} else if d.absIdx != nil {
					d.absIdx.noteName(w.ID, target, absNameWeight)
				}
			}
			continue
		}
		// A word that already names a percept-less concept abstracts nothing: that
		// concept, grounded only in the propositions that use it, IS the abstraction,
		// so minting an abs: node beside it would only duplicate it. Skip, and a
		// duplicate already grown stops refreshing and decays.
		//
		// Unless the word was shown things. A clause that mentions a thing before
		// anything has shown one mints a bare placeholder for it, and that stub reads
		// exactly like a genuine abstract noun here, so the guard would take a class
		// word out of the running for good on the strength of an early telling. What
		// tells them apart is the pointing: a word shown percepts is a word for a kind
		// of thing, whatever a passing clause minted for it, while a word carried
		// wholly in talk was never shown anything at all.
		if !ostensive {
			if c := d.resolveConcept(w.Content); c != nil && d.isPerceptless(c.ID) {
				continue
			}
		}
		switch {
		case c == nil:
			// A class of nothing is not worth minting: the pointing has to reach as many
			// members as the refresh floor asks of one already grown, or the night would
			// mint a node whose grounding the same pass then declines to write.
			if !healthy || len(core) < absRefreshFloor || born >= absPerNight {
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
			if d.absIdx != nil {
				d.absIdx.noteConcept(c)
			}
			born++
			d.event(events.Event{Ev: "concept", ID: c.ID, Tpl: wire.Clip(tpl, 120), Outcome: "abstract"})
		case len(core) < absRefreshFloor:
			continue // dissolved: decay carries the grounding away
		}
		// The constellation is the grounding: each edge's weight is the member's
		// reliability share in the word's company, written fresh each night so the
		// chain to concrete experience stays auditable and current. It is floored
		// above the store's link floor so a broad category's thin-spread member,
		// whose word-side share is a fraction of the floor, is not written at the
		// noise level and pruned before the next refresh can reground it; a normal
		// member already sits above the floor and keeps its share.
		for _, cs := range core {
			gw := cs.share
			if gw < absPwcFloor {
				gw = absPwcFloor
			}
			if err := d.store.SetLinkRel(c.ID, cs.concept, memory.RelGrounds, gw); err != nil {
				fmt.Fprintf(d.out, "abstraction grounding error: %v\n", err)
			}
		}
		if err := d.store.SetLinkRel(w.ID, c.ID, memory.RelNames, absNameWeight); err != nil {
			fmt.Fprintf(d.out, "abstraction naming error: %v\n", err)
		} else if d.absIdx != nil {
			d.absIdx.noteName(w.ID, c.ID, absNameWeight)
		}
		// The percept the abstraction leans on most is its bridge across tongues:
		// the other words that name that percept name the generalization too, so a
		// tongue taught the percept but not enough to abstract on its own thinner
		// exposure still reaches the concept the propositions attach to. This is
		// reuse across tongues for nouns, the echo of the one relation a verb of
		// either tongue binds to: a word that names what an abstraction grounds in
		// names the abstraction.
		var dom coreShare
		for _, cs := range core {
			if cs.share > dom.share {
				dom = cs
			}
		}
		// Only an abstraction resting on a single core percept inherits that
		// percept's names: it is essentially one thing under a word too unfaithful to
		// read as a plain name, so a second tongue that pointed at the same percept
		// reaches the concept the facts attach to. A category rests on a spread core,
		// many members in the same league, and there the dominant grounding is just
		// the densest member, so inheriting its name would bleed a member's name onto
		// the class (animal acquiring köpek). A per-tongue category keeps only its
		// own name.
		if dom.concept != 0 && len(core) == 1 {
			d.inheritGroundingNames(c, dom.concept, w.ID)
		}
	}
	return born
}

// headsCorePercept reports whether a word is the head of a percept in its own
// core: the basic kind a percept's caption ends in ("a small black cat" is a
// cat). A word that heads one of the very percepts it would generalize is that
// percept's name, so it labels a thing rather than a class of things, and minting
// an abstraction from it would only shadow the percept. A category word
// ("animal") heads no member's caption as long as each caption names the member's own
// kind, and an adjective ("green") heads none either, so neither is caught. A caption
// ending in its own class word instead ("a small blue bird" for one bird among many)
// makes that class word head a core percept, and the class then stops abstracting
// altogether, so a caption names the species and leaves the class to the kind above it. Abstractions and relations in the core are passed
// over: only a sensory percept's caption carries a basic-level head.
func (d *Daemon) headsCorePercept(word string, coreIDs []uint64) bool {
	frags, err := d.store.FragmentsByIDs(coreIDs)
	if err != nil {
		return false
	}
	for _, f := range frags {
		if _, ok := abstractionLemma(f.Template); ok || f.Category == "relation" {
			continue
		}
		// The word is this percept's basic name when it is a content word of the caption
		// and names it, wherever it falls ("cheese" in "a yellow cheese wedge"). The
		// caption membership keeps a category word, absent from any member's caption,
		// from reading as a basic name; the naming check keeps a shared modifier (a
		// color the caption carries) from doing so.
		for _, fld := range strings.Fields(f.Content) {
			if strings.EqualFold(fld, word) && d.wordNamesConcept(word, f.ID) {
				return true
			}
		}
	}
	return false
}

// headNamedPercepts returns the core members that are basic-level named percepts:
// a sensory percept whose caption head is a word that names it ("wolf" names "a
// gray wolf"). Abstractions, relations, and junk that accreted a caption but no
// deliberate name are excluded. The count tells a cross-tongue basic name (one
// such percept, the thing it names in another tongue) from a true superordinate
// (several, the class it spans), so the abstraction pass can bind the one and
// abstract the other.
func (d *Daemon) headNamedPercepts(coreIDs []uint64) []uint64 {
	frags, err := d.store.FragmentsByIDs(coreIDs)
	if err != nil {
		return nil
	}
	var out []uint64
	for _, f := range frags {
		if _, ok := abstractionLemma(f.Template); ok || f.Category == "relation" {
			continue
		}
		// A percept is basic-named when a content word of its caption names it. The
		// naming word is not always the caption's last token: "a yellow cheese wedge"
		// is named "cheese", a modifier ahead of its head "wedge". Reading only the
		// last token would miss such a percept's own name and, with a sibling whose
		// name does fall last, leave that sibling as the lone head-named core member,
		// cross-tongue binding a word that has its own percept onto the sibling.
		for _, fld := range strings.Fields(f.Content) {
			if d.wordNamesConcept(fld, f.ID) {
				out = append(out, f.ID)
				break
			}
		}
	}
	return out
}

// ostensiveReferent returns the head-named percept a word was pointed at when it was
// uttered to name something, when one clearly dominates the word's ostension. A name
// concentrates its ostension on its referent, so even a name diluted across several
// percepts by a scene it shared binds to the one it was actually shown over; a category,
// pointed at a different member each telling, spreads its ostension and yields none, as
// does a word never said ostensively. The candidate is sought among the head-named
// percepts only, so the referent is a thing seen, and it must hold a clear majority of
// the word's whole ostension, so a near-even split withholds rather than guessing.
func (d *Daemon) ostensiveReferent(word uint64, named []uint64, ost map[uint64]float64, total float64) uint64 {
	if total <= 0 || len(ost) == 0 {
		return 0
	}
	best, bestMass := uint64(0), 0.0
	for _, id := range named {
		if m := ost[id]; m > bestMass {
			best, bestMass = id, m
		}
	}
	if best != 0 && bestMass >= ostensiveDominance*total {
		return best
	}
	return 0
}

// namesOwnPercept reports whether the word already firmly names a percept of its
// own that it HEADS, other than the candidate: a referent it earned by being the
// basic name of a thing seen, held apart both from a concept it merely keeps
// relation-company with (the cross-tongue borrow's misfire) and from a percept
// it was only shown beside (incidental company the recount wrote). The head test
// is the discriminator: cat heads "a small black cat", so it keeps that name and
// is not re-homed onto the dog it was told to be chased by; a genuine
// second-tongue word (kurt) heads no caption of its own, so it still borrows the
// percept it was shown. It reads the standing names edges, not the night's
// company, so a word whose own-percept company has thinned still counts.
func (d *Daemon) namesOwnPercept(word *memory.Fragment, target uint64) bool {
	edges, err := d.store.LinksFromRel([]uint64{word.ID}, memory.RelNames)
	if err != nil {
		return false
	}
	for _, e := range edges {
		if e.To == target || e.Weight < namesWriteFloor || d.isPerceptless(e.To) {
			continue
		}
		if d.headsCorePercept(word.Content, []uint64{e.To}) {
			return true
		}
	}
	return false
}

// wordNamesConcept reports whether a word form names a concept directly, the test
// for whether a percept's caption head is its established name.
func (d *Daemon) wordNamesConcept(form string, conceptID uint64) bool {
	if idx := d.absIdx; idx != nil {
		// WordNodesByForms matches SQLite lower(content) against a Go-lowered form, so the
		// index mirrors that asymmetry: the key was folded ASCII-only like the column, and
		// the lookup form is folded with Go's Unicode ToLower like the query argument.
		for _, w := range idx.wordsByForm[strings.ToLower(form)] {
			if _, ok := idx.namesByWord[w.ID][conceptID]; ok {
				return true
			}
		}
		return false
	}
	words, err := d.store.WordNodesByForms([]string{form})
	if err != nil || len(words) == 0 {
		return false
	}
	ids := make([]uint64, 0, len(words))
	for _, w := range words {
		ids = append(ids, w.ID)
	}
	edges, err := d.store.LinksFromRel(ids, memory.RelNames)
	if err != nil {
		return false
	}
	for _, e := range edges {
		if e.To == conceptID {
			return true
		}
	}
	return false
}

// inheritGroundingNames carries the names of an abstraction's dominant grounding
// percept onto the abstraction across tongues. Each word that names the percept
// firmly (half the firmest namer's weight, the core-membership bar reused) comes
// to name the abstraction, scaled by how firmly it names the percept, so a second
// tongue's word reaches the generalization through the percept both tongues
// pointed at rather than having to distil one on its own. The anchor word already
// names the abstraction directly and is left untouched; a faint namer of the
// percept stays out, so noise in the percept's company does not spread upward.
func (d *Daemon) inheritGroundingNames(abstraction *memory.Fragment, percept, anchorWord uint64) {
	namers, err := d.store.LinksToRel([]uint64{percept}, memory.RelNames)
	if err != nil || len(namers) == 0 {
		return
	}
	peak := 0.0
	for _, e := range namers {
		if e.Weight > peak {
			peak = e.Weight
		}
	}
	if peak <= 0 {
		return
	}
	for _, e := range namers {
		if e.From == anchorWord || e.Weight < peak*absCoreFraction {
			continue
		}
		weight := absNameWeight * e.Weight / peak
		if weight > absNameWeight {
			weight = absNameWeight
		}
		if err := d.store.SetLinkRel(e.From, abstraction.ID, memory.RelNames, weight); err != nil {
			fmt.Fprintf(d.out, "abstraction cross-name error: %v\n", err)
		} else if d.absIdx != nil {
			d.absIdx.noteName(e.From, abstraction.ID, weight)
		}
	}
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

const (
	// kindLadderWeight is the fixed weight of a derived kind-ladder edge. It sits well above
	// the link floor and is spared the nightly decay, so an edge laid on one healthy pass
	// persists. It is not a confidence -- the edge is read only by the hedged is-a branch --
	// only a durable presence above the prune floor.
	kindLadderWeight = 0.5
	// kindLadderMinCore is the smallest grounded core an abstraction needs before it is read
	// as a category on the ladder. Two or three noisy percepts coincidentally nest inside all
	// sorts of unrelated sets on a lived graph, so a real kind must have distilled a proper
	// core first; this is what keeps the ladder from wiring a thin abstraction to everything
	// that happens to contain its handful of members.
	kindLadderMinCore = 4
	// kindLadderOverlap is the fraction of a kind's members that must also fall under the
	// broader category. Full containment is too brittle on a lived graph -- one stray
	// grounding hides a real kind (mammal ends a night grounding one thing animal has not
	// re-grounded) -- and a low bar admits unrelated categories that share a few members;
	// four-fifths is the defeasible middle the hedged answer stands on. It also orients the
	// arrow soundly: a narrow kind overlaps its superordinate heavily, but the broad
	// superordinate overlaps the narrow kind only thinly, so the test passes one way, not both.
	kindLadderOverlap = 0.8
)

// growKindLadder derives the kind ladder from the percept groundings alone: a defeasible
// RelKindOf edge from a narrower category to a strictly broader one that grounds most of
// its members (kindLadderOverlap), both sides holding a settled core of their own
// (kindLadderMinCore). It runs right after growAbstractions on a consolidation night, when
// every category's core is freshest, so the containment is read while a superordinate is
// still richly grounded;
// the edge is spared the nightly decay, so it outlives that superordinate's later
// thinning, which is exactly when a member can no longer reach it directly. It reads only
// the RelGrounds cores the corpus already lays -- no telling, no teaching change -- and
// writes RelKindOf, which only the hedged is-a branch reads, never the grounded
// machinery. Off (and a no-op that writes nothing) unless KindLadderEnabled.
func (d *Daemon) growKindLadder() (laid int) {
	if !d.cfg.KindLadderEnabled {
		return 0
	}
	concepts, err := d.store.Concepts()
	if err != nil {
		return 0
	}
	// members[A] is the percept set A grounds. Only a real abstraction with a core of at least
	// kindLadderMinCore is a candidate on either side, so a degenerate or thin abstraction is
	// never read as a kind and never wired to whatever coincidentally contains its few members.
	members := make(map[uint64]map[uint64]bool, len(concepts))
	ids := make([]uint64, 0, len(concepts))
	for _, c := range concepts {
		if _, ok := abstractionLemma(c.Template); !ok {
			continue
		}
		set := map[uint64]bool{}
		for _, m := range d.categoryMembers(c.ID) {
			set[m] = true
		}
		if len(set) < kindLadderMinCore {
			continue
		}
		members[c.ID] = set
		ids = append(ids, c.ID)
	}
	for _, a := range ids {
		sa := members[a]
		// A candidate broader is any abstraction that grounds one of A's members; gathering
		// from A's members keeps this near-linear rather than a full pass over every pair.
		cand := map[uint64]bool{}
		for m := range sa {
			groundedBy, err := d.store.LinksToRel([]uint64{m}, memory.RelGrounds)
			if err != nil {
				continue
			}
			for _, e := range groundedBy {
				if e.From != a && members[e.From] != nil {
					cand[e.From] = true
				}
			}
		}
		for b := range cand {
			sb := members[b]
			if len(sb) <= len(sa) {
				continue // B must be STRICTLY broader: it excludes an equal-core synonym pair.
			}
			// Most of A's members fall under B: a defeasible containment that tolerates the
			// few stray groundings a lived graph carries, while a category sharing only a few
			// members with A (cross-cutting or unrelated) falls below the bar and lays no edge.
			overlap := 0
			for m := range sa {
				if sb[m] {
					overlap++
				}
			}
			if float64(overlap) < kindLadderOverlap*float64(len(sa)) {
				continue
			}
			// Keep the first orientation: if B was already made a kind of A on an earlier,
			// richer pass (when A was the broader core), do not now invert it. Extensional
			// containment is judged on the current grounding, so once the true superordinate
			// thins below a kind, a naive pass would read the containment backward and lay the
			// reversed edge; skipping when the reverse already stands keeps the sound
			// orientation and forbids a two-cycle. The residual case, a superordinate never
			// once grounded as the broader core (genuinely uneven teaching), can still orient
			// the arrow by extension alone, but its only effect is a mildly-wrong hedge.
			if rev, err := d.store.LinksFromRel([]uint64{b}, memory.RelKindOf); err == nil {
				reversed := false
				for _, e := range rev {
					if e.To == a {
						reversed = true
						break
					}
				}
				if reversed {
					continue
				}
			}
			if err := d.store.SetLinkRel(a, b, memory.RelKindOf, kindLadderWeight); err == nil {
				laid++
			}
		}
	}
	return laid
}

// growConcepts is concept genesis, run during sleep: shapes that have
// recurred into prototypes are distilled into concept nodes, every
// concept is grounded to the episodes matching its shape, and words bond
// to the concepts their episodes ground. Serotonin decides how many
// concepts one night may mint; dopamine decides how eagerly words bond.
func (d *Daemon) growConcepts() (born, named int) {
	budget := genesisCap
	// A concept is earned by salience mass, not raw count: the merge threshold in full-mass units,
	// so a few attended showings suffice while ambient repetition (fractional mass) does not.
	candidates, err := d.store.ConceptCandidates(float64(memory.MergeThreshold), budget)
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

	// The naming recount resolves each word's relation and grounding against the graph as it
	// stands now, after the mints and the regrounding, so prefetch that graph once and read it
	// from memory rather than a round-trip per word. The recount's own naming writes are batched
	// at the very end, after every read, so this snapshot needs no in-pass refresh.
	if !d.absIdxOff {
		if idx, err := d.buildAbstractionIndex(); err != nil {
			fmt.Fprintf(d.out, "concept genesis error: %v\n", err)
		} else {
			d.absIdx = idx
			defer func() { d.absIdx = nil }()
		}
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
	abstractConcept := map[uint64]bool{}
	conceptHead := map[uint64]string{}
	if cfrags, err := d.store.FragmentsByIDs(conceptIDs); err == nil {
		for _, cf := range cfrags {
			if cf.Category == "relation" {
				relConcept[cf.ID] = true
			}
			if _, ok := abstractionLemma(cf.Template); ok {
				abstractConcept[cf.ID] = true
			}
			if fields := strings.Fields(cf.Content); len(fields) > 0 {
				conceptHead[cf.ID] = strings.ToLower(fields[len(fields)-1])
			}
		}
	}
	// What a word was pointed at, for the same reason the membership pass reads it: the
	// company reaches whatever shared the window, so a word said in a busy stretch keeps
	// company with the whole room, while a naming points at one thing. A word that was
	// shown things is named against those things alone, and its shares are read over that
	// company rather than the room, so what it does name stands at its true strength. A
	// word shown nothing is recounted as before.
	pointedAt := map[uint64]map[uint64]float64{}
	if d.cfg.OstensiveNamingEnabled {
		if ostPairs, err := d.store.OstensionPairs(); err == nil {
			for _, p := range ostPairs {
				if pointedAt[p.Word] == nil {
					pointedAt[p.Word] = map[uint64]float64{}
				}
				pointedAt[p.Word][p.Concept] += p.Mass
			}
		}
	}
	// A concept with no percept to show for it cannot be pointed at, so it is judged on
	// company as before: an abstraction, a relation, and anything perceptless.
	perceptless := d.perceptlessSet(conceptIDs)
	unpointable := func(c uint64) bool {
		return abstractConcept[c] || relConcept[c] || perceptless[c]
	}
	pointedTotal := map[uint64]float64{}
	// The KINDS a word was shown, by the head of each pointed-at caption. A pointing is one
	// perishable edge to one episode, so a word shown two gloves a month apart may still hold
	// the pointing for the later one alone, and asking whether it pointed at THIS glove would
	// read that gap as evidence it never names gloves. Asking whether it points at a glove at
	// all does not: a second exemplar of a kind cannot evict the first, while a word shown
	// apples and standing beside a leaf still loses the leaf.
	pointedKinds := map[uint64]map[string]bool{}
	for _, p := range pairs {
		if o := pointedAt[p.Word]; len(o) > 0 && (o[p.Concept] > 0 || unpointable(p.Concept)) {
			pointedTotal[p.Word] += p.Cooc
		}
	}
	for w, o := range pointedAt {
		kinds := map[string]bool{}
		for c := range o {
			if h := conceptHead[c]; h != "" {
				kinds[h] = true
			}
		}
		pointedKinds[w] = kinds
	}
	gain := namesGainBase + namesDopamineGain*d.chem.Snapshot().Dopamine
	nameEdges := make([]memory.LinkEdge, 0, len(pairs))
	for _, p := range pairs {
		// A participant's name is given by a telling ("you are golem"), not by
		// cross-situational company: the self keeps almost no episodic company,
		// so the statistical rewrite would only erase what was given. The given
		// name stands, decay-spared with its participant, until a telling
		// renames it.
		if d.isParticipant(p.Concept) {
			continue
		}
		// Cross-situational reliability: the share of the word's company
		// that lands on this concept, blended with the share of the
		// concept's company that is this word. A promiscuous word splits
		// its mass across its targets and bonds weakly to each; a word
		// paired with one concept nearly every time keeps almost all of
		// its mass there. The bond is set from the fresh ratio, down as
		// well as up, so a word that drifts to new company loosens its
		// old bonds at the next consolidation.
		// A word that was shown things is read against the things it was shown, so what
		// it does name holds its true share instead of being divided by a room it never
		// pointed at.
		pointed := pointedAt[p.Word]
		shown := len(pointed) > 0
		wordTotal := p.WordTotal
		if shown {
			wordTotal = pointedTotal[p.Word]
		}
		pcw := p.Cooc / (wordTotal + namesSmoothing)
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
		} else if shown && !unpointable(p.Concept) && pointed[p.Concept] <= 0 &&
			!pointedKinds[p.Word][conceptHead[p.Concept]] {
			// The word was shown things, and nothing of this percept's kind was among
			// them: it shared a window, which is not a naming. The bond is written DOWN
			// rather than skipped, because a naming bond is spared the nightly fade, so
			// a pair left unwritten would keep what it stands at forever; and down
			// rather than to nothing, because a bond written to zero is deleted outright
			// on the next sleep, and a word whose pointing merely aged out deserves to
			// fade back over a few nights instead of losing its name irrecoverably.
			w = p.Existing * namesUnpointedKeep
		} else if p.WordTotal < namesEvidenceFloor {
			// Starved company stops voting: a word whose lived company has thinned
			// to crumbs keeps its earned bonds as they stand and births none, so a
			// basic name is not thinned toward nothing by a ratio computed from
			// noise while a rival kept firm elsewhere stands over it. A word with
			// real company recounts as before.
			if p.Existing == 0 {
				continue
			}
			w = p.Existing
		}
		// A percept-less concept gains no NEW name from cross-situational company.
		// It is named when a telling introduces it and when a word binds to it
		// across tongues, both direct namings; with no percept to tell naming from
		// company, a word merely warm beside it must not become its name too. Its
		// standing names still refresh (down as well as up), so two abstract words
		// used alike stay two concepts rather than drifting onto each other.
		if perceptless[p.Concept] && p.Existing == 0 {
			continue
		}
		if p.Existing == 0 && w < namesWriteFloor {
			continue
		}
		nameEdges = append(nameEdges, memory.LinkEdge{From: p.Word, To: p.Concept, Rel: memory.RelNames, Weight: w})
		if p.Existing == 0 {
			named++
			d.event(events.Event{Ev: "name", From: p.Word, To: p.Concept, Rel: memory.RelNames, Weight: w})
		}
	}
	// One transaction for the night's naming, not one commit per pair: the store is
	// single-connection autocommit, so thousands of per-pair writes paid a transaction
	// boundary each.
	if err := d.store.SetLinkRelBatch(nameEdges); err != nil {
		fmt.Fprintf(d.out, "naming link batch error: %v\n", err)
	}
	return born, named
}

// perceptlessSet returns those of the given concepts that ground in nothing: no
// percept and no member beneath them (no grounds edge) and no feature. These are
// the abstract words that name no thing GOLEM can perceive, grounded only in the
// propositions that use them, so the percept that tells naming from company and a
// category from its members is absent for them and the consolidation guards that
// lean on it must scope themselves here.
func (d *Daemon) perceptlessSet(ids []uint64) map[uint64]bool {
	out := map[uint64]bool{}
	if len(ids) == 0 {
		return out
	}
	if idx := d.absIdx; idx != nil {
		for _, id := range ids {
			if !idx.grounded[id] {
				out[id] = true
			}
		}
		return out
	}
	grounded := map[uint64]bool{}
	if g, err := d.store.LinksFromRel(ids, memory.RelGrounds); err == nil {
		for _, e := range g {
			grounded[e.From] = true
		}
	}
	if f, err := d.store.LinksFromRel(ids, memory.RelFeature); err == nil {
		for _, e := range f {
			grounded[e.From] = true
		}
	}
	for _, id := range ids {
		if !grounded[id] {
			out[id] = true
		}
	}
	return out
}

// isPerceptless reports whether a single concept grounds in nothing, the abstract
// word's mark that several consolidation guards scope themselves to.
func (d *Daemon) isPerceptless(id uint64) bool {
	return d.perceptlessSet([]uint64{id})[id]
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
	// A concept anchors a name only if it was attended this recently. Warmth
	// lingers for minutes to serve discourse, but a name grounds in what is
	// freshly in attention, so a word taught in one scene does not bind to what
	// was merely still warm from the scene before. Deliberate co-attention, like
	// naming a member and its category in one scene, falls well inside this.
	attentionRecency = 5 * time.Second
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

// freshlyAttendedConcept returns the warmest workspace concept that is freshly
// attended: above the attention gate and seen within the recency window. A
// deliberate label ("remember this as a bell") binds to what is genuinely present
// now, not to a concept that merely lingers warm from an earlier scene; lateral
// inhibition lets an attended category win the slot over the members it subsumes.
// It returns nil when nothing qualifies, so the label declines rather than bonding
// firm to a stale referent that outlives the workspace.
func (d *Daemon) freshlyAttendedConcept() *memory.Fragment {
	now := time.Now()
	for _, e := range d.attendedWarmest(attentionConceptCap) {
		if e.heat < attentionGroundingGate {
			continue
		}
		if t, ok := d.ws.seenAt(e.id); !ok || now.Sub(t) > attentionRecency {
			continue
		}
		frags, err := d.store.FragmentsByIDs([]uint64{e.id})
		if err == nil && len(frags) == 1 && frags[0].Kind == memory.KindConcept {
			return frags[0]
		}
	}
	return nil
}

// attentionAnchors returns the grounded moments of the warmest workspace
// concepts above the attention gate, newest first and capped, so a told
// word can keep company with what is held in mind. The moments are the
// concept's own direct groundings, so the word grounds in that concept
// rather than in its specifics. Lateral inhibition picks the warmest, so
// an attended category wins the slot over the members it subsumes.
func (d *Daemon) attentionAnchors() []recentMoment {
	warm := d.attendedWarmest(attentionConceptCap)
	now := time.Now()
	var out []recentMoment
	for _, e := range warm {
		if e.heat < attentionGroundingGate {
			continue
		}
		// Warm but not freshly attended: it lingers from a scene before, or
		// GOLEM mulled it over, and may still serve discourse, but it does not
		// ground a new name.
		if t, ok := d.ws.seenAt(e.id); !ok || now.Sub(t) > attentionRecency {
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
			out = append(out, recentMoment{id: f.ID, template: f.Template, at: f.Time, heat: e.heat})
			if n++; n >= attentionAnchorCap {
				break
			}
		}
		// A higher abstraction grounds in concepts, not episodes, so it
		// has no episodic moment to lend even though it is what is held in
		// mind. Bind the word to the concept itself, so a name taught
		// across several attended categories grounds in those categories
		// rather than falling through to one of their members. Only a
		// concept that grounds SOMETHING earns this: a bare perceptless
		// mint, merely warm from the previous telling, has no standing to
		// become the referent of an unrelated word.
		if n == 0 && len(epIDs) > 0 {
			out = append(out, recentMoment{id: frags[0].ID, template: frags[0].Template, at: frags[0].Time, heat: e.heat})
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

// lemmatizeForms reduces each content form to a known morphological base before
// minting, so an inflection ("kediyi", "chased") reinforces the word that names
// its base ("kedi", "chase") rather than minting a separate node that shadows
// it. The lexicon then keys on the lemma, the way comprehension already resolves
// inflections to it. Forms that collapse to one base are merged; a base, or a
// word new to the brain, keeps its surface.
func (d *Daemon) lemmatizeForms(lang string, forms []string) []string {
	seen := map[string]bool{}
	out := forms[:0:0]
	for _, f := range forms {
		base := d.baseForMint(lang, f)
		if !seen[base] {
			seen[base] = true
			out = append(out, base)
		}
	}
	return out
}

// baseForMint returns the form to mint for a content token: the first
// morphological reduction that already names a concept (so an inflection keys
// on its lemma), else the surface. Resolving each candidate is what tells a real
// stem from a mangled one, a mangled stem names nothing, so the surface stands.
func (d *Daemon) baseForMint(lang, tok string) string {
	for _, cand := range d.gram.StemCandidates(lang, tok) {
		if d.formNamesConcept(cand) {
			return cand
		}
	}
	return tok
}

// formNamesConcept reports whether a word form has a node that already names
// some concept. baseForMint reduces an inflection to such a lemma; it asks this
// rather than full resolution so a morphological stem is accepted only when it
// is a real named lemma, never when it merely matches a concept's gloss as a
// substring. A noun ending in what looks like a verb suffix ("kedi") yields a
// short mangled stem ("ke") that the loose gloss match would take for the word;
// requiring a naming edge rejects it, so the surface stands and the word is
// kept whole.
func (d *Daemon) formNamesConcept(form string) bool {
	words, err := d.store.WordNodesByForms([]string{form})
	if err != nil || len(words) == 0 {
		return false
	}
	ids := make([]uint64, 0, len(words))
	for _, w := range words {
		ids = append(ids, w.ID)
	}
	names, err := d.store.LinksFromRel(ids, memory.RelNames)
	return err == nil && len(names) > 0
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

// bindAttention grounds already-minted words in what was held in mind, the
// attention anchors captured before the utterance. It is the half of word
// minting held back from a proposition: a proposition's words bind to their
// roles, so an argument word is not also named after the co-arguments that
// happened to be warm. A bare naming or a definition has no roles and grounds
// here, the way attention grounding intends.
func (d *Daemon) bindAttention(words []*memory.Fragment, anchors []recentMoment) {
	if len(anchors) == 0 {
		return
	}
	w := linkWeight
	for _, word := range words {
		for _, a := range anchors {
			if a.id == word.ID {
				continue
			}
			// The bond is scaled by how strongly the anchor is attended now: a
			// concept held at full presence grounds the word fully, one merely
			// warm from a moment ago grounds it faintly, so a freshly attended
			// percept wins the naming over what lingers from the scene before.
			wa := w * attentionWeight(a.heat)
			if wa <= 0 {
				continue
			}
			if err := d.store.Link(word.ID, a.id, wa); err != nil {
				continue
			}
			d.store.Link(a.id, word.ID, wa)
		}
	}
}

// recordOstension records what a word was pointed at when it was uttered to name
// something: the freshest percept of the moment, weighted by how fresh it was. It is
// the salience the uniform naming company discards. The uniform bonds spread a word
// across whatever lingered warm in the scene, so they cannot tell a name said over
// one constant percept from a category said over varying members; what was freshly
// attended can, and only it earns this edge. The bond grounds in the percept episode,
// not a concept, so it works at genesis when the percept has no concept yet; the
// abstraction pass joins it through to the concept the episode grounds. Only the
// single freshest perceptual moment is taken: a scene-mate lingering from the block
// before is never what a naming points at, so it never becomes a word's referent.
func (d *Daemon) recordOstension(words []*memory.Fragment, now time.Time, exclude uint64) {
	var fresh *recentMoment
	for i := len(d.recent) - 1; i >= 0; i-- {
		r := d.recent[i]
		if now.Sub(r.at) > attentionRecency {
			break
		}
		if r.id == exclude || r.channel == utteranceChannel || r.channel == thoughtChannel {
			continue
		}
		m := r
		fresh = &m
		break
	}
	if fresh == nil {
		return
	}
	w := linkWeight * ostensiveRecency(now.Sub(fresh.at))
	if w <= 0 {
		return
	}
	for _, word := range words {
		if word.ID == fresh.id {
			continue
		}
		if err := d.store.LinkRel(word.ID, fresh.id, memory.RelOstends, w); err != nil {
			continue
		}
	}
}

// ostensiveRecency maps a percept's age at the moment of a naming to the share of an
// ostension bond it earns: the whole bond when it is being looked at, nothing once it
// has left the attention window. The freshest percept is what a naming points at, so a
// word said over a percept just shown earns a near-full edge and one fading from the
// scene before earns a faint one.
func ostensiveRecency(age time.Duration) float64 {
	w := 1 - age.Seconds()/attentionRecency.Seconds()
	if w < 0 {
		return 0
	}
	if w > 1 {
		return 1
	}
	return w
}

// attentionWeight maps a concept's heat to how much of an attention bond it
// earns: nothing at the grounding gate, the whole bond at full presence. Lingering
// warmth, just above the gate, binds faintly, so a word grounds in what it is
// pointed at and only weakly in what is fading from the scene before it.
func attentionWeight(heat float64) float64 {
	w := (heat - attentionGroundingGate) / (1 - attentionGroundingGate)
	if w < 0 {
		return 0
	}
	if w > 1 {
		return 1
	}
	return w
}

// mintWordsScaled is mintWords with the bond weight scaled by the source's
// trust, so a word read on an untrusted channel bonds lightly and entrenches
// only by recurring across reads.
func (d *Daemon) mintWordsScaled(utterance, lang string, tellID uint64, anchors []recentMoment, deliberate bool, scale float64) []*memory.Fragment {
	if lang == "" {
		lang = "en"
	}
	forms := d.gram.ContentTokensLang(lang, utterance)
	if deliberate {
		forms = grammar.AllTokens(utterance)
	} else {
		forms = d.lemmatizeForms(lang, forms)
	}
	if len(forms) == 0 {
		return nil
	}
	// The bond a word earns is scaled by the source's trust: a word read on an
	// untrusted channel bonds lightly, so it entrenches only by recurring across
	// many reads and a word from a single page fades. Trust scales it, not mood.
	w := linkWeight * scale
	var words []*memory.Fragment
	for _, form := range forms {
		// A surface the diacritic fold recovered this turn is a lossy view of a proper
		// word ("buyuk" of "büyük"), not a word of its own: minting a KindWord row for
		// it would fork one word into two nodes. Skip it, so it exists only through the
		// concept the fold resolved it to.
		if d.isRestoredForm(form) {
			continue
		}
		word, minted, err := d.mintWordNode(form, lang, d.chem.Snapshot(), time.Now())
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
		if d.isSystemConcept(c.ID) {
			continue // the innate participants and the speech moves are not grown concepts
		}
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

// hygieneReport audits the naming graph for the two shapes consolidation pollution
// takes, so a teaching pass can be checked before it is kept rather than read as
// clean from rendered output that picks the best name. A DUPLICATE is one gloss and
// category split across nodes, the shape a percept-less concept gathered when an
// abstraction was minted beside it. A PROMISCUOUS word names more than one concept,
// the cross-naming a word gathers when it bonds to what it merely kept company with;
// the targets carry their weight so a strong true name reads apart from a weak
// strayed one. Only grown concepts are audited; the innate scaffolding is no part of
// what teaching writes. It reads and changes nothing.
func (d *Daemon) hygieneReport() (*wire.Health, error) {
	concepts, err := d.store.Concepts()
	if err != nil {
		return nil, err
	}
	glossByID := make(map[uint64]string, len(concepts))
	grownSet := make(map[uint64]bool, len(concepts))
	ids := make([]uint64, 0, len(concepts))
	type dupKey struct{ gloss, cat string }
	byKey := map[dupKey][]uint64{}
	grown := 0
	for _, c := range concepts {
		if d.isSystemConcept(c.ID) {
			continue
		}
		grown++
		glossByID[c.ID] = c.Content
		grownSet[c.ID] = true
		ids = append(ids, c.ID)
		k := dupKey{strings.ToLower(c.Content), c.Category}
		byKey[k] = append(byKey[k], c.ID)
	}
	var dups []*wire.DupConcept
	for k, dupIDs := range byKey {
		if len(dupIDs) <= 1 {
			continue
		}
		sort.Slice(dupIDs, func(i, j int) bool { return dupIDs[i] < dupIDs[j] })
		dups = append(dups, &wire.DupConcept{Gloss: k.gloss, Category: k.cat, IDs: dupIDs})
	}
	sort.Slice(dups, func(i, j int) bool { return dups[i].Gloss < dups[j].Gloss })

	names, err := d.store.LinksToRel(ids, memory.RelNames)
	if err != nil {
		return nil, err
	}
	// A category word names its grounded members in passing, which is the graph's
	// own structure rather than a stray: "animal" sits weakly beside the dog and cat
	// it spans. The pollution is sharper than that. It is a word naming a PERCEPT-LESS
	// concept that is not the one word it primarily names, an abstract concept drifting
	// onto a word whose real referent lies elsewhere. Scoping the flag to a stray onto
	// a percept-less concept passes the legitimate category company by.
	perceptless := d.perceptlessSet(ids)
	type target struct {
		concept uint64
		weight  float64
	}
	byWord := map[uint64][]target{}
	for _, e := range names {
		if grownSet[e.To] {
			byWord[e.From] = append(byWord[e.From], target{e.To, e.Weight})
		}
	}
	loomIDs := make([]uint64, 0)
	for wid, ts := range byWord {
		if len(ts) > 1 {
			loomIDs = append(loomIDs, wid)
		}
	}
	wordFrags, err := d.store.FragmentsByIDs(loomIDs)
	if err != nil {
		return nil, err
	}
	wordByID := make(map[uint64]*memory.Fragment, len(wordFrags))
	for _, w := range wordFrags {
		wordByID[w.ID] = w
	}
	var loose []*wire.LooseWord
	for _, wid := range loomIDs {
		w := wordByID[wid]
		if w == nil {
			continue
		}
		ts := byWord[wid]
		sort.Slice(ts, func(i, j int) bool { return ts[i].weight > ts[j].weight })
		// A percept-less target past the strongest bond is a strayed name: the word
		// already names something more firmly, so an abstract concept hanging weaker
		// off it is the drift, where a single percept-less target IS the word's own
		// abstract referent and stands.
		stray := false
		for i, t := range ts {
			if i > 0 && perceptless[t.concept] {
				stray = true
				break
			}
		}
		if !stray {
			continue
		}
		targets := make([]*wire.NameTarget, 0, len(ts))
		for _, t := range ts {
			targets = append(targets, &wire.NameTarget{Gloss: glossByID[t.concept], Weight: t.weight})
		}
		loose = append(loose, &wire.LooseWord{Word: w.Content, Lang: w.Lang, Targets: targets})
	}
	sort.Slice(loose, func(i, j int) bool { return loose[i].Word < loose[j].Word })
	return &wire.Health{Concepts: grown, Duplicates: dups, Promiscuous: loose}, nil
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
		d.ws.mull(id, exciteRecall)
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

// namedAbstraction reports whether the word already names a class of its own, the mark
// that an earlier pass read this word off its pointing. It is asked only to tell a word
// whose reading is about to change from one that never had that reading at all.
func (d *Daemon) namedAbstraction(w *memory.Fragment) bool {
	c, err := d.store.ConceptByTemplate("abs:" + w.Lang + ":" + w.Content)
	return err == nil && c != nil
}
