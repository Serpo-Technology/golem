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
