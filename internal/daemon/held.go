package daemon

import "sort"

// The held store is GOLEM's propositional working memory: the propositions it
// holds in mind that are not facts in the graph. An inference it draws and a
// supposition it entertains both live here, each tagged with how it is held and
// what it was drawn from, so a chain of reasoning can build on a held step and
// the answer can be spoken at its true confidence. It decays alongside the
// workspace and clears at sleep; nothing here reaches long-term memory unless a
// separate act promotes it. This is the substrate a hypothesis sits on while it
// is elaborated, and the place a multi-step inference keeps its intermediates.
//
// Two strengths are kept apart on purpose. conf is the epistemic confidence, set
// where the proposition is derived and attenuated only by a further derivation
// step, so a chain grows less sure the further it stands from observation. heat
// is the activation, high when the thought is fresh and decaying with time, so a
// held proposition is forgotten when it cools rather than when GOLEM stops being
// sure of it. A guess can be firmly held yet fade from mind; the two come apart.

// propStatus is how a held proposition is entertained.
type propStatus int

const (
	heldInferred     propStatus = iota // drawn from resemblance or a relation; a guess
	heldHypothetical                   // entertained under a supposition; not believed
)

// heldProp is one proposition held in working memory: a directed relation
// between an agent and a patient, the glosses that speak it, how it is held and
// how firmly, how far it stands from observation, and what it was drawn from.
type heldProp struct {
	id                       uint64
	rel                      string
	agent, patient           uint64
	agentGloss, patientGloss string
	status                   propStatus
	conf                     float64  // epistemic confidence; attenuated only by a derivation step
	heat                     float64  // activation; decays with time, forgotten below the floor
	depth                    int      // derivation hops from observation; 0 is one inference from a graph fact
	from                     []uint64 // provenance: the held props this was drawn from, empty when straight from the graph
	voiced                   bool     // GOLEM has spoken this, so initiative does not offer it twice
	reinforced               int      // times this conclusion has been re-drawn; enough earns it a place in long-term memory
	persistent               bool     // promoted to long-term memory, so it neither decays nor clears at sleep
	seq                      uint64
}

const (
	heldDecayKeep = 0.99 // per homeostasis tick: a held thought fades, slower than a turn, gone by sleep
	heldFloor     = 0.1  // below this activation a held proposition leaves mind
	heldBudget    = 32   // propositions held at once; the coldest is evicted past it
	heldMaxDepth  = 3    // derivation hops a chain may stand from observation before it is too thin to draw
	heldStepKeep  = 0.85 // confidence kept per derivation step, so a chain grows less sure with length

	promoteReinforce = 2 // times a conclusion must be re-drawn before a night keeps it in long-term memory
)

type heldStore struct {
	props []heldProp
	seq   uint64
}

func newHeldStore() *heldStore { return &heldStore{} }

// add records a held proposition, freshly activated. A proposition already held
// in the same mode is refreshed rather than duplicated, keeping the higher
// confidence and the shallower derivation, so re-deriving a held belief does not
// multiply it or make it look further from observation than it is. Past the
// budget the coldest entry is evicted. It returns the proposition's id.
func (h *heldStore) add(p heldProp) uint64 {
	h.seq++
	if existing := h.findID(p.rel, p.agent, p.patient, p.status); existing != nil {
		existing.heat = 1
		existing.seq = h.seq
		existing.reinforced++ // re-drawing the same conclusion is what earns it a place
		if p.conf > existing.conf {
			existing.conf = p.conf
		}
		if p.depth < existing.depth {
			existing.depth = p.depth
			existing.from = p.from
		}
		if p.voiced {
			existing.voiced = true // voicing is monotonic: once said, never re-offered
		}
		if p.persistent {
			existing.persistent = true
		}
		return existing.id
	}
	p.id = h.seq
	p.heat = 1
	p.seq = h.seq
	h.props = append(h.props, p)
	if len(h.props) > heldBudget {
		h.evictColdest()
	}
	return p.id
}

func (h *heldStore) evictColdest() {
	cold := 0
	for i := range h.props {
		if h.props[i].heat < h.props[cold].heat {
			cold = i
		}
	}
	h.props = append(h.props[:cold], h.props[cold+1:]...)
}

// findID returns a mutable handle to a held proposition matching a relation,
// its arguments, and how it is held, or nil when none is held.
func (h *heldStore) findID(rel string, agent, patient uint64, status propStatus) *heldProp {
	for i := range h.props {
		p := &h.props[i]
		if p.rel == rel && p.agent == agent && p.patient == patient && p.status == status {
			return p
		}
	}
	return nil
}

// holds reports whether a proposition is held in a given mode, with the
// confidence it is held at, the read a chain makes to lean on a held step the
// way it leans on a graph fact.
func (h *heldStore) holds(rel string, agent, patient uint64, status propStatus) (float64, bool) {
	if p := h.findID(rel, agent, patient, status); p != nil {
		return p.conf, true
	}
	return 0, false
}

// byAgent returns the held propositions a concept is the agent of, in the given
// mode, freshest first, the read a "what does X do" makes of what is merely
// supposed or guessed about X.
func (h *heldStore) byAgent(agent uint64, status propStatus) []heldProp {
	var out []heldProp
	for _, p := range h.props {
		if p.agent == agent && p.status == status {
			out = append(out, p)
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].seq > out[j].seq })
	return out
}

// markVoiced records that GOLEM has spoken a held proposition, so initiative
// offers it once and then moves on to what it has not yet said.
func (h *heldStore) markVoiced(id uint64) {
	for i := range h.props {
		if h.props[i].id == id {
			h.props[i].voiced = true
			return
		}
	}
}

// unvoicedAbout returns the strongest belief held about a concept as agent that
// GOLEM has not yet voiced, the conclusion it could volunteer, or nil when there
// is nothing left to share. Only what GOLEM believes is offered, never a
// supposition, so initiative speaks for itself and not for an entertained
// premise.
func (h *heldStore) unvoicedAbout(agent uint64) *heldProp {
	var best *heldProp
	for i := range h.props {
		p := &h.props[i]
		if p.agent != agent || p.status != heldInferred || p.voiced {
			continue
		}
		if best == nil || p.conf > best.conf {
			best = p
		}
	}
	return best
}

// decay cools every held proposition and forgets what has faded below notice.
// Confidence is untouched: a thought leaves mind by cooling, not by GOLEM
// growing unsure of it. A belief promoted to long-term memory neither cools nor
// is forgotten; it is no longer a passing thought. The main loop calls this on
// the homeostasis tick.
func (h *heldStore) decay() {
	kept := h.props[:0]
	for _, p := range h.props {
		if p.persistent {
			kept = append(kept, p)
			continue
		}
		p.heat *= heldDecayKeep
		if p.heat >= heldFloor {
			kept = append(kept, p)
		}
	}
	h.props = kept
}

// earned returns the inferred beliefs GOLEM has re-drawn often enough to keep:
// the conclusions a night should carry into long-term memory rather than clear.
// A supposition, however firmly entertained, is never earned.
func (h *heldStore) earned(minReinforced int) []heldProp {
	var out []heldProp
	for _, p := range h.props {
		if p.status == heldInferred && !p.persistent && p.reinforced >= minReinforced {
			out = append(out, p)
		}
	}
	return out
}

// keepPersistent drops every passing thought, leaving the promoted beliefs in
// place: the night clears working memory but spares what has been remembered.
func (h *heldStore) keepPersistent() {
	kept := h.props[:0]
	for _, p := range h.props {
		if p.persistent {
			kept = append(kept, p)
		}
	}
	h.props = kept
}

// forget drops a held proposition outright, whatever its mode: the path a belief
// leaves by when the observation it stood in for is finally seen, so a guess does
// not linger once the fact is known.
func (h *heldStore) forget(rel string, agent, patient uint64) {
	kept := h.props[:0]
	for _, p := range h.props {
		if p.rel == rel && p.agent == agent && p.patient == patient {
			continue
		}
		kept = append(kept, p)
	}
	h.props = kept
}

// clearStatus drops every held proposition entertained in a given mode, the act
// that ends a supposition: the hypotheticals built under it leave together while
// any inferred beliefs stand.
func (h *heldStore) clearStatus(status propStatus) {
	kept := h.props[:0]
	for _, p := range h.props {
		if p.status != status {
			kept = append(kept, p)
		}
	}
	h.props = kept
}

func (h *heldStore) clear() { h.props = nil }

func (h *heldStore) len() int { return len(h.props) }
