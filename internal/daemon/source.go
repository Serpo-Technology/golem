package daemon

import (
	"fmt"
	"strings"
	"time"

	"golem/internal/events"
	"golem/internal/grammar"
	"golem/internal/memory"
	"golem/wire"
)

// A source is who spoke on the talking channel. GOLEM does not accept every voice
// equally: it weighs a telling by how far it trusts the source. The architect holds
// a master key and is trusted outright; a peer that watches the channel (an LLM)
// declares its name and is heard at that source's trust, low until the architect
// raises it. Trust lives in the graph as a self-edge weight on a source node
// (RelTrust), auditable and adjustable like any other weight, and a told moment is
// tagged with its source (RelSource) so a proposition is auditable back to who said it.

const (
	// architectSource is the local operator and the master-key holder, trusted outright.
	architectSource = "architect"
	// architectTrust is full trust: a master-key telling writes as firmly as a
	// grounded observation would.
	architectTrust = 1.0
	// defaultSourceTrust is where a newly met, declared source starts: low, so its
	// tellings enter defeasibly and earn weight only by corroboration or by the
	// architect raising it. Above the store link floor so a single telling still lands.
	defaultSourceTrust = 0.3
	// trustStep is how far one spoken trust move shifts a source's trust, so "trust the
	// llm" nudges it up and "doubt the llm" down, repeatable rather than absolute.
	trustStep = 0.2
	// trustAuthorityFloor is how trusted a speaker must be to change whom GOLEM trusts:
	// full trust, so only the architect (the master key, or the untagged operator) may
	// vouch, and a peer cannot talk its own trust up.
	trustAuthorityFloor = architectTrust
	// The nightly trust-learning rates: a source's trust follows its record. A night of
	// unchallenged claims restores a little (slow to build), a night of claims GOLEM has
	// grounded otherwise costs more (fast to lose), each capped so trust is sticky.
	trustRiseRate       = 0.04
	trustDropRate       = 0.30
	maxNightlyTrustRise = 0.05
	maxNightlyTrustDrop = 0.25
	// trustFloorLearned keeps a repeatedly-contradicted source recoverable rather than
	// gone; trustCeilingLearned caps what the record alone earns, so full trust stays an
	// explicit grant the architect makes, never one a source talks itself into.
	trustFloorLearned   = 0.05
	trustCeilingLearned = 0.9
	// sourceTemplate namespaces source nodes so a source named "llm" never collides
	// with a content concept of the same name, and consolidation can pass them over.
	sourceCategory = "source"
)

func sourceTemplate(name string) string { return "source:" + name }

// ensureSource returns the node for a named peer source, minting it on first sight
// at a low starting trust so a newly met voice is heard defeasibly until the architect
// raises it or corroboration earns it. The node is namespaced and categorised so it is
// never read as a content concept and is spared the nightly consolidation, its trust
// standing as a structural fact. The architect is not a node: its full trust is
// conferred by the master key, so no keyless client can claim it by declaring the name.
func (d *Daemon) ensureSource(name string) (*memory.Fragment, error) {
	tpl := sourceTemplate(name)
	c, err := d.store.ConceptByTemplate(tpl)
	if err != nil {
		return nil, err
	}
	if c != nil {
		return c, nil
	}
	c = &memory.Fragment{
		Kind:     memory.KindConcept,
		Channel:  "concept",
		Content:  name,
		Template: tpl,
		Category: sourceCategory,
		Limbic:   d.chem.Snapshot(),
		Time:     time.Now(),
	}
	if _, err := d.store.Encode(c); err != nil {
		return nil, err
	}
	if err := d.store.SetLinkRel(c.ID, c.ID, memory.RelTrust, defaultSourceTrust); err != nil {
		return nil, err
	}
	return c, nil
}

// sourceTrust reads a source node's trust, the weight of its RelTrust self-edge.
func (d *Daemon) sourceTrust(sourceID uint64) float64 {
	edges, err := d.store.LinksFromRel([]uint64{sourceID}, memory.RelTrust)
	if err != nil {
		return defaultSourceTrust
	}
	for _, e := range edges {
		if e.To == sourceID {
			return e.Weight
		}
	}
	return defaultSourceTrust
}

// setSourceTrust sets a source's trust outright, the architect vouching for whom
// GOLEM should believe. It mints the source if new.
func (d *Daemon) setSourceTrust(name string, level float64) error {
	c, err := d.ensureSource(name)
	if err != nil {
		return err
	}
	return d.store.SetLinkRel(c.ID, c.ID, memory.RelTrust, level)
}

// resolveSpeaker reads who is speaking from a request and returns the source name
// and its trust. A declared source is that peer, heard at its stored trust,
// whatever transport carried it: a transducer relaying an LLM's words signs the
// connection, but the words are still the peer's. A source-less request is the
// architect, whether the local operator on the socket or a signed request from
// afar; an unsigned source-less request over the network never reaches here,
// having been refused at the transport. A peer cannot borrow the architect's
// trust by declaring the name "architect": that resolves to a low-seeded peer
// node below, not to the architect.
func (d *Daemon) resolveSpeaker(req wire.Request) (name string, trust float64) {
	if req.Source == "" {
		return architectSource, architectTrust
	}
	src, err := d.ensureSource(req.Source)
	if err != nil {
		return req.Source, defaultSourceTrust
	}
	return req.Source, d.sourceTrust(src.ID)
}

// tagMomentSource records who spoke a told moment, so a proposition drawn from it is
// auditable back to its source and comprehension can weigh it by that source's trust.
func (d *Daemon) tagMomentSource(momentID uint64, name string) {
	src, err := d.ensureSource(name)
	if err != nil || src == nil {
		return
	}
	if err := d.store.SetLinkRel(momentID, src.ID, memory.RelSource, 1.0); err != nil {
		return
	}
}

// planTrustMove handles talking about whom to believe: "trust the llm", "I doubt the
// llm", and their Turkish and Latin forms, all routed off the grounded speech-act verb
// the graph marks. It reaches the trust the graph holds through language rather than an
// out-of-band command. Only a trusted speaker may move trust: the change is authorized
// by WHO is speaking (the master key, or the untagged operator), so a peer cannot vouch
// for itself. A doubt aimed at a proposition rather than a source ("I doubt that a dog
// chased a zebra") names no source here and is left to the belief path.
func (d *Daemon) planTrustMove(frame grammar.Frame, raw string, momentID uint64) (string, bool) {
	raise := frame.Speech == "trust"
	lower := frame.Speech == "doubt"
	if !raise && !lower {
		return "", false
	}
	name := sourceNameFrom(frame.Patient.Phrase)
	if name == "" {
		name = sourceNameFrom(speechComplement(raw))
	}
	if name == "" {
		return "", false // no source named: a claim doubt, handled on the belief path
	}
	if d.momentTrust(momentID) < trustAuthorityFloor {
		return "I decide whom to trust with the architect, not on my own.", true
	}
	if frame.Negate { // "do not trust the llm" inverts the move
		raise, lower = lower, raise
	}
	src, err := d.ensureSource(name)
	if err != nil {
		return "", false
	}
	next := d.sourceTrust(src.ID) + trustStep
	if lower {
		next = d.sourceTrust(src.ID) - trustStep
	}
	if next < 0 {
		next = 0
	}
	if next > architectTrust {
		next = architectTrust
	}
	if err := d.setSourceTrust(name, next); err != nil {
		return "", false
	}
	if lower {
		return fmt.Sprintf("I doubt %s more now, trusting it %.2f.", name, next), true
	}
	return fmt.Sprintf("I trust %s more now, at %.2f.", name, next), true
}

// sourceNameFrom reads a source's name from the object of a trust move, stripping a
// leading article. A phrase that opens as a clause or a participant ("that ...", "it",
// "you", "me", "I") names no source, so the move declines and the belief path takes it.
func sourceNameFrom(phrase string) string {
	f := strings.Fields(strings.ToLower(strings.Trim(phrase, " ?.!")))
	for len(f) > 0 && (f[0] == "the" || f[0] == "a" || f[0] == "an") {
		f = f[1:]
	}
	if len(f) != 1 {
		return "" // a bare source name is one word; a clause or empty is not a source
	}
	switch f[0] {
	case "that", "it", "you", "me", "i":
		return ""
	}
	return f[0]
}

// speechComplement reads the object after the speech verb of a bare move ("trust the
// llm" -> "the llm"), the fallback when the frame did not carry the patient. A leading
// participant ("I trust the llm") is dropped so the verb-led remainder stands.
func speechComplement(raw string) string {
	f := strings.Fields(strings.ToLower(strings.Trim(raw, " ?.!")))
	if len(f) > 0 && (f[0] == "i" || f[0] == "you" || f[0] == "we") {
		f = f[1:]
	}
	if len(f) < 2 {
		return ""
	}
	return strings.Join(f[1:], " ") // drop the verb, keep the object
}

// momentTrust reads how far to trust a told moment, from the source it was tagged
// with. An untagged moment is the local operator's, trusted outright, so a telling
// with no declared source (every existing curriculum telling) writes at full weight.
func (d *Daemon) momentTrust(momentID uint64) float64 {
	edges, err := d.store.LinksFromRel([]uint64{momentID}, memory.RelSource)
	if err != nil || len(edges) == 0 {
		return architectTrust
	}
	return d.sourceTrust(edges[0].To)
}

// claimAt reconstructs the proposition a told moment laid down: its agent, its patient,
// and the relation that grounds it. It is how the trust pass rereads what a source
// claimed, to judge each claim against what GOLEM has since grounded.
func (d *Daemon) claimAt(momentID uint64) (agentID, patientID uint64, relContent string, ok bool) {
	ag, err := d.store.LinksFromRel([]uint64{momentID}, memory.RelAgent)
	if err != nil || len(ag) == 0 {
		return 0, 0, "", false
	}
	pa, err := d.store.LinksFromRel([]uint64{momentID}, memory.RelPatient)
	if err != nil || len(pa) == 0 {
		return 0, 0, "", false
	}
	grounds, err := d.store.LinksToRel([]uint64{momentID}, memory.RelGrounds)
	if err != nil {
		return 0, 0, "", false
	}
	for _, g := range grounds {
		rf, err := d.store.FragmentsByIDs([]uint64{g.From})
		if err != nil || len(rf) != 1 {
			continue
		}
		if rf[0].Category == "relation" {
			return ag[0].To, pa[0].To, rf[0].Content, true
		}
	}
	return 0, 0, "", false
}

// propositionTrust reads the firmest trust behind a held proposition: over the moments
// whose role edges carry this agent and patient under this relation, the highest source
// trust. A fact the architect taught or GOLEM observed reads full; one heard only from a
// low-trust peer reads that source's trust, however often the peer repeated it, so the
// answer built on it stays hedged until a fully trusted telling arrives. A proposition
// whose carrying moments cannot be found reads full: its support is entrenched or
// derived, provenance the nightly consolidation already weighed.
func (d *Daemon) propositionTrust(agentID uint64, relContent string, patientID uint64) float64 {
	agEdges, err := d.store.LinksToRel([]uint64{agentID}, memory.RelAgent)
	if err != nil {
		return architectTrust
	}
	best, found := 0.0, false
	for _, e := range agEdges {
		a, p, rel, ok := d.claimAt(e.From)
		if !ok || a != agentID || p != patientID || rel != relContent {
			continue
		}
		found = true
		if t := d.momentTrust(e.From); t > best {
			best = t
		}
	}
	if !found {
		return architectTrust
	}
	return best
}

// growSourceTrust moves each source's trust by how its claims have held up, the earned
// half of the trust model. A claim GOLEM has since grounded a denial of counts against
// the source; a night of claims it has had no reason to deny restores a little. Trust
// is sticky (each night's shift is capped and slow to build, quicker to lose), floored
// so a contradicted source stays recoverable, and ceilinged so full trust remains an
// explicit grant rather than one a source earns on its own. The architect is no node
// here, so its trust is never touched. (A grounded confirmation is the strong positive
// signal; with perception unbuilt it is rare, so an unchallenged record is the gentle
// positive for now.)
func (d *Daemon) growSourceTrust() {
	concepts, err := d.store.Concepts()
	if err != nil {
		return
	}
	for _, s := range concepts {
		if s.Category != sourceCategory {
			continue
		}
		tags, err := d.store.LinksToRel([]uint64{s.ID}, memory.RelSource)
		if err != nil || len(tags) == 0 {
			continue
		}
		momIDs := make([]uint64, 0, len(tags))
		for _, t := range tags {
			momIDs = append(momIDs, t.From)
		}
		moms, err := d.store.FragmentsByIDs(momIDs)
		if err != nil {
			continue
		}
		total, contradicted := 0, 0
		for _, m := range moms {
			// Judge the source on the night's own claims, weighed against what GOLEM
			// grounds now, rather than re-litigating its whole record every night, which
			// would keep a once-contradicted source falling forever and never let a clean
			// stretch restore it.
			if !m.Time.After(d.lastSleep) {
				continue
			}
			a, p, rel, ok := d.claimAt(m.ID)
			if !ok {
				continue
			}
			total++
			ag := &memory.Fragment{ID: a}
			if d.propositionDenied(ag, rel, p) || d.propositionDeniedCategorically(ag, rel, p) {
				contradicted++
			}
		}
		if total == 0 {
			continue
		}
		cur := d.sourceTrust(s.ID)
		badFrac := float64(contradicted) / float64(total)
		delta := trustRiseRate*(1-badFrac) - trustDropRate*badFrac
		if delta > maxNightlyTrustRise {
			delta = maxNightlyTrustRise
		}
		if delta < -maxNightlyTrustDrop {
			delta = -maxNightlyTrustDrop
		}
		next := cur + delta
		if delta >= 0 {
			if next > trustCeilingLearned {
				next = trustCeilingLearned
			}
			if next < cur { // a clean night never pulls back a source the architect vouched higher
				next = cur
			}
		} else if next < trustFloorLearned {
			next = trustFloorLearned
		}
		if next == cur {
			continue
		}
		if err := d.store.SetLinkRel(s.ID, s.ID, memory.RelTrust, next); err != nil {
			continue
		}
		d.event(events.Event{Ev: "trust", Cue: s.Content, Weight: next})
	}
}
