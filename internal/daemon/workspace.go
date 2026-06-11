package daemon

import "sort"

// The workspace is GOLEM's working memory: a decaying activation pattern
// over concept nodes, held across ticks and turns. Encoding a moment
// warms the concept its shape grounds, speaking about a concept warms it
// directly, and the pattern fades on its own, so "that one" and "do it
// again" resolve to whatever is still warm from the last few minutes of
// experience. Sleep clears it: working memory does not survive the
// night, and a stale referent is worse than none.
//
// Sustained, re-broadcast activation is also the substrate the Global
// Workspace sits on, which is why the buffer lives on the daemon rather
// than inside any one request.
const (
	workspaceDecayKeep = 0.998 // per homeostasis tick; minutes to fade fully
	workspaceFloor     = 0.05
	workspaceBudget    = 64

	exciteSpeech = 1.0 // a concept spoken about is maximally present
	exciteRecall = 0.8 // a concept recalled through is nearly so
	exciteEncode = 0.6 // a concept whose shape just recurred warms
)

type workspace struct {
	act   map[uint64]float64
	touch map[uint64]uint64 // sequence of last excitation, for recency tie-breaks
	seq   uint64
}

func newWorkspace() *workspace {
	return &workspace{act: make(map[uint64]float64), touch: make(map[uint64]uint64)}
}

// excite warms a concept, keeping whichever is higher: the new activation
// or what remains of the old. Past the budget the coldest entry is
// evicted, because a working memory that holds everything holds nothing.
func (w *workspace) excite(id uint64, amount float64) {
	if id == 0 {
		return
	}
	if amount > 1 {
		amount = 1
	}
	if amount > w.act[id] {
		w.act[id] = amount
	}
	// Touching a concept refers to it now, even when it was already this
	// warm. Recency is what separates two concepts pinned at the ceiling,
	// so "do it again" follows the latest mention rather than the lowest id.
	w.seq++
	w.touch[id] = w.seq
	if len(w.act) <= workspaceBudget {
		return
	}
	var coldest uint64
	cold := 2.0
	for cid, a := range w.act {
		if a < cold {
			coldest, cold = cid, a
		}
	}
	delete(w.act, coldest)
	delete(w.touch, coldest)
}

// decay cools every activation a little and forgets what has faded below
// notice. The main loop calls it on the homeostasis tick.
func (w *workspace) decay() {
	for id, a := range w.act {
		a *= workspaceDecayKeep
		if a < workspaceFloor {
			delete(w.act, id)
			delete(w.touch, id)
			continue
		}
		w.act[id] = a
	}
}

// cool applies fatigue to one concept: a thought that has just been
// thought from hands its warmth to its associations and steps back, so
// the chain moves instead of circling.
func (w *workspace) cool(id uint64, keep float64) {
	if a, ok := w.act[id]; ok {
		a *= keep
		if a < workspaceFloor {
			delete(w.act, id)
			delete(w.touch, id)
			return
		}
		w.act[id] = a
	}
}

func (w *workspace) clear() {
	w.act = make(map[uint64]float64)
	w.touch = make(map[uint64]uint64)
}

// heat reports a concept's current activation.
func (w *workspace) heat(id uint64) float64 { return w.act[id] }

// warmEntry is one workspace concept with its activation.
type warmEntry struct {
	id   uint64
	heat float64
}

// warmest returns the n most active concepts, hottest first.
func (w *workspace) warmest(n int) []warmEntry {
	out := make([]warmEntry, 0, len(w.act))
	for id, a := range w.act {
		out = append(out, warmEntry{id: id, heat: a})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].heat != out[j].heat {
			return out[i].heat > out[j].heat
		}
		return w.touch[out[i].id] > w.touch[out[j].id]
	})
	if len(out) > n {
		out = out[:n]
	}
	return out
}
