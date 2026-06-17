// Package grammar is the innate language faculty: one concept-frame
// read and written by parsers and realizers whose lexicon alone is
// learned. English and Turkish are parameter settings over the same
// frame.
package grammar

import (
	"strings"

	"golem/internal/memory"
)

// Force is the illocutionary force of an utterance: what the speaker is
// doing with it.
type Force string

const (
	ForceStatement Force = "statement"
	ForceQuestion  Force = "question"
	ForceCommand   Force = "command"
)

// Term is one filled thematic role: a concept when the phrase grounded
// to one, otherwise just the surface phrase, which is what lets the
// planner admit it does not know. Number lives on the term so the verb
// can agree with its subject instead of guessing.
type Term struct {
	Concept *memory.Fragment // nil when nothing grounds the phrase
	Phrase  string
	Plural  bool
	// The generation flags a meaning carries so Speak renders the surface from
	// the tongue's data rather than the caller pre-rendering it. Definite asks
	// for the tongue's definite marking, Given for the pronoun a referent in
	// play takes, and With holds further co-arguments coordinated with this one.
	Definite bool
	Given    bool
	With     []string
}

func (t Term) Empty() bool { return t.Concept == nil && t.Phrase == "" && !t.Given }

// Speak is the term's surface form for realization: the concept's gloss
// when grounded, the raw phrase when not.
func (t Term) Speak() string {
	if t.Concept != nil {
		return t.Concept.Content
	}
	return t.Phrase
}

// Frame is the concept-frame: the small typed structure the parser fills
// and the realizer reads, the interface both halves of the grammar
// share. Thematic roles hold concepts where grounding found them; the
// temporal qualifier carries "what follows X" shapes; Unknown keeps the
// words that resolved to nothing.
type Frame struct {
	Force    Force
	Lang     string      // the language the utterance arrived in; answers go back the same way
	Wh       string      // the interrogative that opened a question: what, which, when, where, why, who, how
	Speech   string      // a speech-act verb's move when the clause is about the talk itself: clarify, recall
	Agent    Term        // who or what the clause is about
	Action   Term        // the predicate; for innate verbs only Phrase is set
	Patient  Term        // what is acted on or asked about
	Quals    []Qualifier // temporal adjuncts in spoken order; "after X and before Y" is two
	Sort     string      // sortal restriction from "which command": the kind of answer asked for
	SortPl   bool        // the sortal was plural, inviting more than one answer
	Counting bool        // "how many" or "how often": the answer is a count
	Past     bool        // tense: the clause is about a settled moment, "followed" against "follows"
	Negate   bool        // the predicate is denied
	Hedge    bool        // the claim is tentative ("may name")
	Habit    bool        // habitual aspect: the clause states a regularity ("usually")
	Times    int         // occurrence count, for "I have seen it N times"
	At       string      // a time or place complement, preformatted by the planner
	Last     bool        // the most recent occurrence: "last happened"
	Pro      bool        // the patient was a pro-form; resolve it from the workspace
	ProType  string      // the pro-form's animacy preference: animate, inanimate, or empty
	Alt      bool        // "the other one": resolve past the warmest referent
	Again    bool        // repetition was requested
	Unknown  []string
}

// Qualifier is one temporal adjunct: a relator and the anchor it
// attaches to.
type Qualifier struct {
	Rel    string
	Anchor Term
}

// FirstQual returns the leading temporal qualifier, the common case.
func (f Frame) FirstQual() (string, Term) {
	if len(f.Quals) == 0 {
		return "", Term{}
	}
	return f.Quals[0].Rel, f.Quals[0].Anchor
}

// Tokenize lowercases an utterance and splits it into word tokens,
// trimming punctuation off both ends of each token so "Deploy," and
// "deploy" are one form. Word-internal marks survive, which keeps
// shapes like "go.mod" whole. A question mark anywhere is reported
// separately because it carries force, not content.
func Tokenize(s string) (tokens []string, question bool) {
	question = strings.Contains(s, "?")
	for field := range strings.FieldsSeq(strings.ToLower(s)) {
		tok := strings.TrimFunc(field, func(r rune) bool {
			return !(r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r > 127)
		})
		if tok != "" {
			tokens = append(tokens, tok)
		}
	}
	return tokens, question
}

// ContentTokens returns the distinct open-class words of an utterance
// under the built-in closed class, for callers without a memory-backed
// grammar.
func ContentTokens(s string) []string {
	return Builtin().ContentTokens(s)
}

// AllTokens returns the distinct word tokens of an utterance, closed
// class included. Deliberate labeling (tell --about) uses it, so a
// label can teach even a form the grammar claims, and sustained usage
// can out-weigh the seeded parameter.
func AllTokens(s string) []string {
	tokens, _ := Tokenize(s)
	seen := map[string]bool{}
	var out []string
	for _, tok := range tokens {
		if len(tok) < 2 || seen[tok] {
			continue
		}
		seen[tok] = true
		out = append(out, tok)
	}
	return out
}
