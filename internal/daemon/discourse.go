package daemon

import (
	"strings"

	"golem/internal/grammar"
	"golem/internal/memory"
)

// The discourse layer holds the thread of an exchange as conceptual structure,
// not as text. A role question leaves its relation in mind, so an elliptical
// follow-up can ask the same thing of a new referent: "what did the cat chase"
// then "what about the dog" is one thread, the relation carried over and only
// the referent swapped. The relation and role are reasoned over the graph; no
// words from the first turn are reused, so the thread holds across a switch of
// tongue.

// roleContext is the thread a role question leaves behind: the relation asked
// about, which role the named argument played, and the verb that named it.
type roleContext struct {
	rel       *memory.Fragment
	knownRole string
	verb      string
	past      bool
}

// threadProp is a proposition the current exchange has set down: a relation
// between an agent and a patient. Recall leans toward it, so a fresh claim wins
// the conversation without rewriting the long-term weight it would take many
// tellings to earn. It is how the talk prefers what was just said over what is
// worn deepest into the graph.
type threadProp struct {
	rel          string
	agent        uint64
	patient      uint64
	agentGloss   string
	patientGloss string
}

const threadPropCap = 16

// recordThreadProp remembers a proposition the exchange just established, newest
// first, keeping only the last few so an old turn does not haunt a new one. The
// glosses are kept too, so the answer can be spoken from the thread even when
// the walk has dropped the proposition.
func (d *Daemon) recordThreadProp(rel string, agent, patient uint64, agentGloss, patientGloss string) {
	if rel == "" || agent == 0 || patient == 0 {
		return
	}
	d.threadProps = append([]threadProp{{rel: rel, agent: agent, patient: patient, agentGloss: agentGloss, patientGloss: patientGloss}}, d.threadProps...)
	if len(d.threadProps) > threadPropCap {
		d.threadProps = d.threadProps[:threadPropCap]
	}
}

// threadFillers returns every distinct co-argument the current thread has
// established for a relation and a known argument, newest first, each with its
// gloss. A role answer reads from it directly, so what the exchange set down is
// named in full and a fresh claim is never lost behind the strongest memory
// even when the walk, in a trained graph, has pruned it from its candidates.
func (d *Daemon) threadFillers(rel string, knownID uint64, questioned string) []walkDest {
	var out []walkDest
	seen := map[uint64]bool{}
	for _, p := range d.threadProps {
		var id uint64
		var gloss string
		switch {
		case questioned == "patient" && p.rel == rel && p.agent == knownID:
			id, gloss = p.patient, p.patientGloss
		case questioned == "agent" && p.rel == rel && p.patient == knownID:
			id, gloss = p.agent, p.agentGloss
		default:
			continue
		}
		if id == 0 || seen[id] {
			continue
		}
		seen[id] = true
		out = append(out, walkDest{id: id, gloss: gloss})
	}
	return out
}

// completeRoleEllipsis answers an elliptical follow-up to a role question by
// inheriting its relation and asking it of a new referent: after "what did the
// cat chase", "what about the dog" walks chase from the dog. The relation and
// role come from the thread, the referent from the follow-up. It returns false
// when there is no role thread, the turn is not such a follow-up, or the
// referent names nothing GOLEM knows.
func (d *Daemon) completeRoleEllipsis(frame grammar.Frame, raw string, thread *roleContext) (string, bool) {
	if thread == nil {
		return "", false
	}
	ref, ok := aboutReferent(raw)
	if !ok {
		return "", false
	}
	known := d.resolveConcept(ref)
	if known == nil {
		return "", false
	}
	// The tense is part of the inherited question: "what did the cat chase" was
	// past, so the follow-up about the dog is past too.
	frame.Past = frame.Past || thread.past
	return d.answerRole(frame, thread.rel, known, thread.knownRole, thread.verb)
}

// aboutReferent reads the referent of an elliptical follow-up, the noun in "what
// about a dog" or "how about the cat", with any article dropped. It returns
// false when the turn is not one of those continuations.
func aboutReferent(raw string) (string, bool) {
	s := strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(strings.ToLower(raw)), "?"))
	for _, lead := range []string{"what about ", "how about ", "and what about ", "and how about "} {
		rest, ok := strings.CutPrefix(s, lead)
		if !ok {
			continue
		}
		rest = strings.TrimSpace(rest)
		for _, art := range []string{"a ", "an ", "the "} {
			rest = strings.TrimPrefix(rest, art)
		}
		if rest = strings.TrimSpace(rest); rest != "" {
			return rest, true
		}
	}
	return "", false
}
