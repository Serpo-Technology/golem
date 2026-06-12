package daemon

import (
	"fmt"
	"regexp"
	"sort"
	"time"

	"golem/internal/events"
	"golem/internal/memory"
	"golem/internal/wire"
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
	featureTopK        = 12   // a signature stays sparse
	featureShareFloor  = 0.25 // an attribute this rare in the experience is drift, not signature
	featureMinMoments  = 2    // one moment is not clustered experience
	featureFeelGate    = 0.5  // mean chemistry above this becomes a feel attribute
	featureEpisodeCap  = 64   // newest grounded moments mined per concept
	featureOverlapMin  = 2    // shared features a match needs; one shared word is coincidence
	featureSeedConcept = 2    // concepts an overlap recall may land on
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
	concepts, err := d.store.Concepts()
	if err != nil {
		fmt.Fprintf(d.out, "feature error: %v\n", err)
		return 0
	}
	for _, c := range concepts {
		episodes, err := d.conceptEpisodes(c.ID)
		if err != nil {
			fmt.Fprintf(d.out, "feature error: %v\n", err)
			return built
		}
		shares := mineFeatures(d, episodes)
		if err := d.store.DropRelFrom([]uint64{c.ID}, memory.RelFeature); err != nil {
			fmt.Fprintf(d.out, "feature error: %v\n", err)
			return built
		}
		for _, fs := range shares {
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
	return built
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
	if len(out) > featureTopK {
		out = out[:featureTopK]
	}
	return out
}

// featureSeeds reads a query through the feature layer: its words match
// tok: attributes, concepts sharing enough of them are the resemblance,
// and their grounded moments join the recall even when no word, gloss,
// or substring connects them. The matched feature nodes come back as
// mediators, so the reply shows exactly which shared attributes carried
// the match.
func (d *Daemon) featureSeeds(query string) ([]*memory.Fragment, []*wire.FragmentInfo) {
	tokens := d.gram.ContentTokens(query)
	if len(tokens) < featureOverlapMin {
		return nil, nil
	}
	names := make([]string, 0, len(tokens))
	for _, tok := range tokens {
		names = append(names, "tok:"+tok)
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
		d.ws.excite(m.concept, exciteRecall)
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
		info := wire.NewFragmentInfo(f)
		info.Activation = 0.8
		mediators = append(mediators, info)
	}
	if concepts, err := d.store.FragmentsByIDs(conceptIDs); err == nil {
		for _, c := range concepts {
			info := wire.NewFragmentInfo(c)
			info.Activation = 0.8
			mediators = append(mediators, info)
		}
	}
	return episodes, mediators
}
