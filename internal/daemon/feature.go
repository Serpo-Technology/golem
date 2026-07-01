package daemon

import (
	"fmt"
	"math"
	"regexp"
	"sort"
	"time"

	"golem/internal/events"
	"golem/internal/memory"
	"golem/wire"
)

// The sparse feature layer hangs under the concept nodes: each concept
// carries a small signature of interpretable attributes mined from its
// grounded experience (the recurring words of its shape, the channel it
// lives on, how it felt), each a shared feature node reached by a
// weighted feature edge. The localist node stays the auditable handle
// and the provenance of every thought; the feature layer is the one
// place reading is interpretive, and similarity becomes feature
// overlap: a phrase never seen lands near the concept it resembles, and
// the shared features can be read off to explain why.
const (
	featureTopK          = 12   // a signature stays sparse
	featureShareFloor    = 0.25 // an attribute this rare in the experience is drift, not signature
	featureMinMoments    = 2    // one moment is not clustered experience
	featureFeelGate      = 0.5  // mean chemistry above this becomes a feel attribute
	featureEpisodeCap    = 64   // newest grounded moments mined per concept
	featureOverlapMin    = 2    // shared features a match needs; one shared word is coincidence
	featureSeedConcept   = 2    // concepts an overlap recall may land on
	senseSynonymFloor    = 0.3  // how firmly a word must name a concept for a co-namer to count as its synonym
	senseSynonymCap      = 6    // synonyms a token expands to, so a loosely-used word cannot pull in a crowd
	universalFeatureFrac = 0.8  // a feature carried by this fraction of concepts discriminates nothing
	universalMinConcepts = 10   // below this a shared feature is the signal, not universal noise
	// A discovered feature is a concept's conceptual company reified: a
	// concept that recurs as the lived neighbor of several others is a
	// shared trait in its own right, so two things resemble each other by
	// the company they keep, even with no describing word in common.
	companyShareFloor = 0.2 // company this thin in a concept's neighborhood is incidental
	companyMinBearers = 2   // company shared by at least this many concepts is a learned feature
)

// rePlaceholder strips the normalizer's variable slots before token
// mining, so a placeholder never becomes an attribute.
var rePlaceholder = regexp.MustCompile(`<[a-z]+>`)

// growFeatures runs in consolidation after abstraction: every concept's
// signature is rewritten whole from its current grounding, so the layer
// stays sparse, current, and auditable down to the moments underneath.
// Features earn their place from clustered experience: an attribute
// must recur across the concept's lived moments to enter the signature.
func (d *Daemon) growFeatures() (built int) {
	concepts, err := d.grownConcepts()
	if err != nil {
		fmt.Fprintf(d.out, "feature error: %v\n", err)
		return 0
	}
	// Discovered features come from the whole concept graph at once: which
	// company recurs across enough concepts to count as a shared trait.
	company, err := d.conceptCompany(concepts)
	if err != nil {
		fmt.Fprintf(d.out, "feature error: %v\n", err)
		return 0
	}
	// First pass: gather every concept's candidate features, uncapped, and
	// count how many concepts carry each, so the second pass can weigh a
	// feature by how much it discriminates rather than how often it recurs.
	candidates := make(map[uint64][]featureShare, len(concepts))
	df := map[string]int{}
	for _, c := range concepts {
		episodes, err := d.conceptEpisodes(c.ID)
		if err != nil {
			fmt.Fprintf(d.out, "feature error: %v\n", err)
			return built
		}
		feats := mergeShares(mineFeatures(d, episodes), company[c.ID])
		candidates[c.ID] = feats
		for _, f := range feats {
			df[f.name]++
		}
	}
	// Second pass: scale each candidate by its information, keep the sparse
	// top, and write the signature. A frequent feature is discounted so it
	// cannot crowd out a rarer but defining one, and the weight it leaves on
	// the edge is what resemblance later ranks by.
	n := len(concepts)
	for _, c := range concepts {
		feats := candidates[c.ID]
		for i := range feats {
			feats[i].share *= featureIDF(df[feats[i].name], n)
		}
		sort.Slice(feats, func(i, j int) bool {
			if feats[i].share != feats[j].share {
				return feats[i].share > feats[j].share
			}
			return feats[i].name < feats[j].name
		})
		if len(feats) > featureTopK {
			feats = feats[:featureTopK]
		}
		if err := d.store.DropRelFrom([]uint64{c.ID}, memory.RelFeature); err != nil {
			fmt.Fprintf(d.out, "feature error: %v\n", err)
			return built
		}
		for _, fs := range feats {
			node, err := d.store.FeatureNode(fs.name, time.Now())
			if err != nil {
				fmt.Fprintf(d.out, "feature mint error: %v\n", err)
				continue
			}
			if err := d.store.SetLinkRel(c.ID, node.ID, memory.RelFeature, fs.share); err != nil {
				fmt.Fprintf(d.out, "feature link error: %v\n", err)
				continue
			}
			built++
		}
	}
	if built > 0 {
		d.event(events.Event{Ev: "feature", Count: built})
	}
	d.refreshUniversalFeats(concepts)
	d.weighConceptInformation(concepts)
	return built
}

// refreshUniversalFeats names the features that discriminate nothing: it counts,
// over the signatures just written, how many concepts carry each feature, and
// records the fraction. A feature carried by nearly all of them is universal,
// kept in the graph but passed over by resemblance, so a shared channel or a
// baseline stress cannot make every concept look alike. Recomputed each night,
// so a feature stops being universal the moment it turns rare. The mark is both
// stored, to survive a restart, and cached, for the resemblance walk to read.
func (d *Daemon) refreshUniversalFeats(concepts []*memory.Fragment) {
	n := len(concepts)
	// Universality is statistical: with only a handful of concepts, a feature on
	// most of them is the trait that relates them, not noise. Below the floor the
	// brain names nothing universal and resemblance reads every feature.
	if n < universalMinConcepts {
		d.universalFeats = nil
		if err := d.store.SetFeatureUniversal(nil); err != nil {
			fmt.Fprintf(d.out, "universal feature clear error: %v\n", err)
		}
		return
	}
	ids := make([]uint64, 0, n)
	for _, c := range concepts {
		ids = append(ids, c.ID)
	}
	edges, err := d.store.LinksFromRel(ids, memory.RelFeature)
	if err != nil {
		fmt.Fprintf(d.out, "universal feature read error: %v\n", err)
		return
	}
	df := map[uint64]int{}
	for _, e := range edges {
		df[e.To]++ // each concept carries a feature once, so an edge is one bearer
	}
	fracs := make(map[uint64]float64, len(df))
	skip := map[uint64]float64{}
	for feat, c := range df {
		frac := float64(c) / float64(n)
		fracs[feat] = frac
		if frac >= universalFeatureFrac {
			skip[feat] = frac
		}
	}
	d.universalFeats = skip
	if err := d.store.SetFeatureUniversal(fracs); err != nil {
		fmt.Fprintf(d.out, "universal feature save error: %v\n", err)
	}
}

// loadUniversalFeats reads the stored universality marks into the resemblance
// walk's skip set at boot, so a brain woken from a checkpoint passes over its
// universal features at once rather than waiting for the first night to name them.
func (d *Daemon) loadUniversalFeats() {
	fracs, err := d.store.FeatureUniversal()
	if err != nil {
		fmt.Fprintf(d.out, "universal feature load error: %v\n", err)
		return
	}
	skip := map[uint64]float64{}
	for id, frac := range fracs {
		if frac >= universalFeatureFrac {
			skip[id] = frac
		}
	}
	d.universalFeats = skip
}

// Information, not frequency, is what should make a concept salient and
// durable. A thing lived in one fixed context, however often, carries
// little information; a thing lived across varied contexts, or one
// deliberately taught, carries a lot. weighConceptInformation scores each
// concept by the diversity of its grounding and by whether a firm name was
// taught for it, caches the score on the node, and refreshes the grounded
// experience of the informative ones so they resist the night's decay while
// a high-frequency low-information node is left to fade.
const (
	durableInfoGate = 0.5 // a concept this informative has its experience entrenched
	taughtNameFloor = 0.8 // a name at least this firm was deliberately taught
	taughtInfoBoost = 0.5 // the durability a deliberate teaching is worth
)

func (d *Daemon) weighConceptInformation(concepts []*memory.Fragment) {
	if len(concepts) == 0 {
		return
	}
	ids := make([]uint64, 0, len(concepts))
	for _, c := range concepts {
		ids = append(ids, c.ID)
	}
	taught := map[uint64]bool{}
	if names, err := d.store.LinksToRel(ids, memory.RelNames); err == nil {
		for _, e := range names {
			if e.Weight >= taughtNameFloor {
				taught[e.To] = true
			}
		}
	}
	for _, c := range concepts {
		episodes, err := d.conceptEpisodes(c.ID)
		if err != nil {
			fmt.Fprintf(d.out, "information error: %v\n", err)
			continue
		}
		channels := map[string]bool{}
		for _, f := range episodes {
			channels[f.Channel] = true
		}
		// One context is no information; each further context adds a half,
		// and a deliberate teaching adds its own, capped at full.
		info := 0.5 * float64(len(channels)-1)
		if taught[c.ID] {
			info += taughtInfoBoost
		}
		info = clampUnit(info)
		if err := d.store.SetInfo(c.ID, info); err != nil {
			fmt.Fprintf(d.out, "information error: %v\n", err)
			continue
		}
		// Persistence by information: the experience under an informative
		// concept is refreshed so it outlives the decay, while a node
		// repeated in one fixed context is left to fade despite its count.
		if info >= durableInfoGate {
			ids := make([]uint64, 0, len(episodes))
			for _, f := range episodes {
				ids = append(ids, f.ID)
			}
			if err := d.store.Refresh(ids); err != nil {
				fmt.Fprintf(d.out, "information error: %v\n", err)
			}
		}
	}
}

// clampUnit holds a value in [0, 1].
func clampUnit(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}

// infoScale maps a concept's cached information to the share of full
// salience its shape earns when it recurs: a node lived in one fixed
// context warms faintly, a varied or taught one fully, so attention
// follows meaning rather than mere repetition.
func infoScale(info float64) float64 {
	return 0.3 + 0.7*clampUnit(info)
}

// conceptEpisodes gathers the lived moments behind a concept: its
// grounded episodes directly, and for an abstraction, the episodes of
// the concepts in its constellation, so an abstract signature still
// bottoms out in concrete experience.
func (d *Daemon) conceptEpisodes(id uint64) ([]*memory.Fragment, error) {
	grounds, err := d.store.LinksFromRel([]uint64{id}, memory.RelGrounds)
	if err != nil {
		return nil, err
	}
	ids := make([]uint64, 0, len(grounds))
	for _, e := range grounds {
		ids = append(ids, e.To)
	}
	targets, err := d.store.FragmentsByIDs(ids)
	if err != nil {
		return nil, err
	}
	var episodes []*memory.Fragment
	var constellation []uint64
	for _, f := range targets {
		switch f.Kind {
		case memory.KindEpisodic:
			episodes = append(episodes, f)
		case memory.KindConcept:
			constellation = append(constellation, f.ID)
		}
	}
	if len(constellation) > 0 {
		deeper, err := d.store.LinksFromRel(constellation, memory.RelGrounds)
		if err != nil {
			return nil, err
		}
		deepIDs := make([]uint64, 0, len(deeper))
		for _, e := range deeper {
			deepIDs = append(deepIDs, e.To)
		}
		deepFrags, err := d.store.FragmentsByIDs(deepIDs)
		if err != nil {
			return nil, err
		}
		for _, f := range deepFrags {
			if f.Kind == memory.KindEpisodic {
				episodes = append(episodes, f)
			}
		}
	}
	// Newest first under the cap: the signature follows what the concept
	// has lately been, not everything it has ever touched.
	sort.Slice(episodes, func(i, j int) bool { return episodes[i].ID > episodes[j].ID })
	if len(episodes) > featureEpisodeCap {
		episodes = episodes[:featureEpisodeCap]
	}
	return episodes, nil
}

// featureShare is one mined attribute with the share of experience that
// exhibits it.
type featureShare struct {
	name  string
	share float64
}

// mineFeatures distils a signature from moments: recurring open-class
// words of the shape (tok:), the channels the experience arrived on
// (ch:), and the felt chemistry at encoding (feel:). Every attribute is
// weighted by the share of occurrences exhibiting it, prototype counts
// included, then floored and capped so the signature stays sparse.
func mineFeatures(d *Daemon, episodes []*memory.Fragment) []featureShare {
	if len(episodes) < featureMinMoments {
		return nil
	}
	counts := map[string]float64{}
	total := 0.0
	var stress, reward float64
	for _, f := range episodes {
		n := float64(f.Count)
		total += n
		shape := rePlaceholder.ReplaceAllString(f.Template, " ")
		seen := map[string]bool{}
		for _, tok := range d.gram.ContentTokens(shape) {
			if !seen[tok] {
				seen[tok] = true
				counts["tok:"+tok] += n
			}
		}
		counts["ch:"+f.Channel] += n
		stress += n * f.Limbic.Noradrenaline
		reward += n * f.Limbic.Dopamine
	}
	if total == 0 {
		return nil
	}
	var out []featureShare
	for name, c := range counts {
		if share := c / total; share >= featureShareFloor {
			out = append(out, featureShare{name: name, share: share})
		}
	}
	if stress/total >= featureFeelGate {
		out = append(out, featureShare{name: "feel:stress", share: stress / total})
	}
	if reward/total >= featureFeelGate {
		out = append(out, featureShare{name: "feel:reward", share: reward / total})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].share != out[j].share {
			return out[i].share > out[j].share
		}
		return out[i].name < out[j].name
	})
	// The cap is not applied here: the corpus pass weighs each candidate by
	// how much it discriminates before the signature is trimmed, so a rare
	// defining token is not dropped for a frequent common one.
	return out
}

// featureIDF weighs a feature by how much it discriminates: a feature
// carried by few concepts is informative and keeps its full share, one
// carried by many is common and is discounted, normalized so the rarest
// feature is unscaled and a single-concept corpus (nothing to discriminate
// against) leaves every share untouched. This is the cross-situational
// currency Steps 11 and 26 already speak, brought to the feature layer.
func featureIDF(df, n int) float64 {
	if n <= 1 || df <= 0 {
		return 1
	}
	// Smoothed so a feature carried by every concept is discounted, never
	// annihilated: it still says something, just less than a rare one.
	idf := math.Log(float64(n+1)/float64(df+1)) + 1
	maxIDF := math.Log(float64(n+1)/2.0) + 1 // a feature in a single concept (df = 1)
	if maxIDF <= 0 {
		return 1
	}
	r := idf / maxIDF
	if r < 0 {
		return 0
	}
	if r > 1 {
		return 1
	}
	return r
}

// mergeShares folds the discovered company features into the surface
// signature; both kinds then compete for the same sparse slots after the
// corpus pass has weighed each by how much it discriminates.
func mergeShares(surface, discovered []featureShare) []featureShare {
	if len(discovered) == 0 {
		return surface
	}
	return append(surface, discovered...)
}

// conceptCompany discovers the semantic features that live in the concept
// graph rather than in any caption: the company each concept keeps. For
// every concept it sums the lived neighbors of its grounded episodes into
// the concepts they instantiate, normalizes to shares of its whole
// company, and keeps a company concept only where it recurs across enough
// concepts to be a shared trait. The result is, per concept, the "co:"
// features its company earned, so two concepts with no word in common but
// the same neighborhood come to share a feature node.
func (d *Daemon) conceptCompany(concepts []*memory.Fragment) (map[uint64][]featureShare, error) {
	glossByID := make(map[uint64]string, len(concepts))
	for _, c := range concepts {
		glossByID[c.ID] = c.Content
	}
	byTemplate := map[string]*memory.Fragment{} // resolved concept per template, cached
	resolve := func(tpl string) (*memory.Fragment, error) {
		if c, ok := byTemplate[tpl]; ok {
			return c, nil
		}
		c, err := d.store.ConceptByTemplate(tpl)
		byTemplate[tpl] = c
		return c, err
	}

	shareOf := make(map[uint64]map[uint64]float64, len(concepts))
	for _, c := range concepts {
		grounds, err := d.store.LinksFromRel([]uint64{c.ID}, memory.RelGrounds)
		if err != nil {
			return nil, err
		}
		epIDs := make([]uint64, 0, len(grounds))
		for _, g := range grounds {
			epIDs = append(epIDs, g.To)
		}
		if len(epIDs) == 0 {
			continue
		}
		edges, err := d.store.LinksFrom(epIDs)
		if err != nil {
			return nil, err
		}
		neighborIDs := make([]uint64, 0, len(edges))
		for _, e := range edges {
			if e.Rel == memory.RelTemporal || e.Rel == memory.RelSuccessor {
				neighborIDs = append(neighborIDs, e.To)
			}
		}
		neighbors, err := d.fragmentsByIDsTiered(neighborIDs)
		if err != nil {
			return nil, err
		}
		byID := make(map[uint64]*memory.Fragment, len(neighbors))
		for _, f := range neighbors {
			byID[f.ID] = f
		}
		mass := map[uint64]float64{}
		for _, e := range edges {
			if e.Rel != memory.RelTemporal && e.Rel != memory.RelSuccessor {
				continue
			}
			nf := byID[e.To]
			if nf == nil || nf.Kind != memory.KindEpisodic ||
				nf.Channel == utteranceChannel || nf.Channel == thoughtChannel {
				continue
			}
			company, err := resolve(nf.Template)
			if err != nil {
				return nil, err
			}
			if company == nil || company.ID == c.ID {
				continue
			}
			mass[company.ID] += e.Weight * nf.Weight
		}
		total := 0.0
		for _, m := range mass {
			total += m
		}
		if total == 0 {
			continue
		}
		shares := map[uint64]float64{}
		for id, m := range mass {
			if sh := m / total; sh >= companyShareFloor {
				shares[id] = sh
			}
		}
		if len(shares) > 0 {
			shareOf[c.ID] = shares
		}
	}

	// A company concept is a discovered feature only where it recurs as the
	// company of several concepts; company kept by one alone is incidental.
	bearers := map[uint64]int{}
	for _, shares := range shareOf {
		for id := range shares {
			bearers[id]++
		}
	}
	out := make(map[uint64][]featureShare, len(shareOf))
	for k, shares := range shareOf {
		for id, sh := range shares {
			if bearers[id] >= companyMinBearers {
				out[k] = append(out[k], featureShare{name: "co:" + glossByID[id], share: sh})
			}
		}
	}
	return out, nil
}

// featureSeeds reads a query through the feature layer: its words match
// tok: attributes, concepts sharing enough of them are the resemblance,
// and their grounded moments join the recall even when no word, gloss,
// or substring connects them. The matched feature nodes come back as
// mediators, so the reply shows exactly which shared attributes carried
// the match.
// senseSynonyms returns the other words that firmly share a sense with a token: the words
// that co-name a concept the token itself firmly names. Two words naming one concept are
// synonyms by co-reference (a colour term and the born-in word for its cell, a noun and its
// other-tongue name), so recognition can match through the shared sense rather than the
// surface form. Gated by naming firmness and capped, so a word used loosely across many
// things does not pull in unrelated company.
func (d *Daemon) senseSynonyms(token string) []string {
	words, err := d.store.WordNodesByForms([]string{token})
	if err != nil || len(words) == 0 {
		return nil
	}
	self := make(map[uint64]bool, len(words))
	wordIDs := make([]uint64, 0, len(words))
	for _, w := range words {
		self[w.ID] = true
		wordIDs = append(wordIDs, w.ID)
	}
	names, err := d.store.LinksFromRel(wordIDs, memory.RelNames)
	if err != nil {
		return nil
	}
	senses := make([]uint64, 0, len(names))
	for _, e := range names {
		if e.Weight >= senseSynonymFloor {
			senses = append(senses, e.To)
		}
	}
	if len(senses) == 0 {
		return nil
	}
	conamers, err := d.store.LinksToRel(senses, memory.RelNames)
	if err != nil {
		return nil
	}
	seen := make(map[uint64]bool, len(conamers))
	ids := make([]uint64, 0, len(conamers))
	for _, e := range conamers {
		if e.Weight < senseSynonymFloor || self[e.From] || seen[e.From] {
			continue
		}
		seen[e.From] = true
		ids = append(ids, e.From)
	}
	frags, err := d.store.FragmentsByIDs(ids)
	if err != nil {
		return nil
	}
	out := make([]string, 0, senseSynonymCap)
	for _, f := range frags {
		out = append(out, f.Content)
		if len(out) >= senseSynonymCap {
			break
		}
	}
	return out
}

func (d *Daemon) featureSeeds(query string) ([]*memory.Fragment, []*wire.FragmentInfo) {
	tokens := d.gram.ContentTokens(query)
	if len(tokens) < featureOverlapMin {
		return nil, nil
	}
	names := make([]string, 0, len(tokens))
	seenName := map[string]bool{}
	addName := func(tok string) {
		key := "tok:" + tok
		if !seenName[key] {
			seenName[key] = true
			names = append(names, key)
		}
	}
	for _, tok := range tokens {
		addName(tok)
		// A word and its synonyms share a sense, so a percept described with one word
		// ("a round crimson form") resembles a concept described with a synonym ("a round
		// red ball") through the shared sense, not a shared surface token. The synonyms
		// are read from the graph (co-namers of a sense the word names), so this covers a
		// taught colour term, a cross-tongue name, and any future shared-sense concept.
		for _, syn := range d.senseSynonyms(tok) {
			addName(syn)
		}
	}
	feats, err := d.store.FeaturesByNames(names)
	if err != nil {
		fmt.Fprintf(d.out, "feature recall error: %v\n", err)
		return nil, nil
	}
	if len(feats) < featureOverlapMin {
		return nil, nil
	}
	featIDs := make([]uint64, 0, len(feats))
	for _, f := range feats {
		featIDs = append(featIDs, f.ID)
	}
	edges, err := d.store.LinksToRel(featIDs, memory.RelFeature)
	if err != nil {
		fmt.Fprintf(d.out, "feature recall error: %v\n", err)
		return nil, nil
	}
	shared := map[uint64]int{}
	score := map[uint64]float64{}
	carrier := map[uint64][]uint64{} // concept -> the features that matched
	for _, e := range edges {
		shared[e.From]++
		score[e.From] += e.Weight
		carrier[e.From] = append(carrier[e.From], e.To)
	}
	type match struct {
		concept uint64
		score   float64
	}
	var matches []match
	for id, n := range shared {
		if n >= featureOverlapMin {
			matches = append(matches, match{concept: id, score: score[id]})
		}
	}
	if len(matches) == 0 {
		return nil, nil
	}
	sort.Slice(matches, func(i, j int) bool {
		if matches[i].score != matches[j].score {
			return matches[i].score > matches[j].score
		}
		return matches[i].concept < matches[j].concept
	})
	if len(matches) > featureSeedConcept {
		matches = matches[:featureSeedConcept]
	}

	conceptIDs := make([]uint64, 0, len(matches))
	matchedFeatures := map[uint64]bool{}
	for _, m := range matches {
		conceptIDs = append(conceptIDs, m.concept)
		for _, fid := range carrier[m.concept] {
			matchedFeatures[fid] = true
		}
		d.ws.mull(m.concept, exciteRecall)
	}
	grounds, err := d.store.LinksFromRel(conceptIDs, memory.RelGrounds)
	if err != nil {
		fmt.Fprintf(d.out, "feature recall error: %v\n", err)
		return nil, nil
	}
	episodeIDs := make([]uint64, 0, len(grounds))
	for _, e := range grounds {
		episodeIDs = append(episodeIDs, e.To)
	}
	episodes, err := d.fragmentsByIDsTiered(episodeIDs)
	if err != nil {
		fmt.Fprintf(d.out, "feature recall error: %v\n", err)
		return nil, nil
	}
	kept := episodes[:0]
	for _, f := range episodes {
		if f.Kind == memory.KindEpisodic {
			kept = append(kept, f)
		}
	}
	episodes = kept
	if len(episodes) == 0 {
		return nil, nil
	}
	if err := d.store.Refresh(episodeIDs); err == nil {
		for _, f := range episodes {
			f.Weight = 1.0
		}
	}

	var mediators []*wire.FragmentInfo
	for _, f := range feats {
		if !matchedFeatures[f.ID] {
			continue
		}
		info := fragmentInfo(f)
		info.Activation = 0.8
		mediators = append(mediators, info)
	}
	if concepts, err := d.store.FragmentsByIDs(conceptIDs); err == nil {
		for _, c := range concepts {
			info := fragmentInfo(c)
			info.Activation = 0.8
			mediators = append(mediators, info)
		}
	}
	return episodes, mediators
}
