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
	Force       Force
	Lang        string      // the language the utterance arrived in; answers go back the same way
	Wh          string      // the interrogative that opened a question: what, which, when, where, why, who, how
	LocPrep     string      // a locative preposition seen in a copula clause ("is the leaf ON the tree"): the clause asks a place, not a category
	Speech      string      // a speech-act verb's move when the clause is about the talk itself: clarify, recall
	Property    string      // a property predicated of a relation ("before is transitive"): the learned rule the predicate names
	Agent       Term        // who or what the clause is about
	Action      Term        // the predicate; for innate verbs only Phrase is set
	Patient     Term        // what is acted on or asked about
	Quals       []Qualifier // temporal adjuncts in spoken order; "after X and before Y" is two
	Sort        string      // sortal restriction from "which command": the kind of answer asked for
	SortPl      bool        // the sortal was plural, inviting more than one answer
	Counting    bool        // "how many" or "how often": the answer is a count
	Past        bool        // tense: the clause is about a settled moment, "followed" against "follows"
	Negate      bool        // the predicate is denied
	Hedge       bool        // the claim is tentative ("may name")
	Habit       bool        // habitual aspect: the clause states a regularity ("usually")
	Times       int         // occurrence count, for "I have seen it N times"
	At          string      // a time or place complement, preformatted by the planner
	Last        bool        // the most recent occurrence: "last happened"
	Comparative bool        // the clause compares two concepts along a gradable relation ("X is bigger than Y")
	Magnitude   bool        // the question asks the DEGREE of a comparison ("how much bigger is X than Y")
	Analogy     bool        // a proportion "A is to B as C is to what": Agent:Patient :: Third:?
	Third       Term        // the third term of a proportion, the one whose fourth is asked
	Inner       string      // an embedded clause filling this question's asked role ("who chased WHAT THE DOG CHASED")
	Pro         bool        // the patient was a pro-form; resolve it from the workspace
	ProType     string      // the pro-form's animacy preference: animate, inanimate, or empty
	Alt         bool        // "the other one": resolve past the warmest referent
	Again       bool        // repetition was requested
	Unknown     []string
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
// shapes like "go.mod" whole, and a leading minus on a number survives
// so a negative literal ("-8") reads as itself rather than its
// magnitude. A question mark anywhere is reported separately because it
// carries force, not content.
func Tokenize(s string) (tokens []string, question bool) {
	question = strings.Contains(s, "?")
	for field := range strings.FieldsSeq(strings.ToLower(s)) {
		if tok := cleanToken(field); tok != "" {
			tokens = append(tokens, tok)
		}
	}
	return tokens, question
}

// symbolRunes are the operator marks that stand as tokens of their own: the
// arithmetic and comparison symbols the graph can name the way it names any
// word, so "+" is teachable as a name of an operation. Sentence punctuation
// stays outside the set, consumed as before, so "?" never becomes a token and
// keeps carrying force alone.
const symbolRunes = "+-*/<>="

// cleanToken strips the surrounding punctuation off a field down to its content,
// keeping a leading minus on a negative number so "-8" is not read as 8. The
// general trim drops both ends, so the sign is restored when the field is a minus
// then digits after any surrounding punctuation. A field that is nothing but
// operator symbols survives whole as a symbol token.
func cleanToken(field string) string {
	content := func(r rune) bool {
		return r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r > 127
	}
	tok := strings.TrimFunc(field, func(r rune) bool { return !content(r) })
	if tok == "" {
		// A field of hyphens alone is a prose dash, punctuation as before; the
		// subtraction symbol's citizenship waits on the punctuation work that
		// can tell the two apart. Every other operator field is a token.
		if field != "" && strings.Trim(field, symbolRunes) == "" && strings.Trim(field, "-") != "" {
			return field
		}
		return tok
	}
	if isDigits(tok) {
		if inner := strings.TrimFunc(field, func(r rune) bool { return r != '-' && !content(r) }); inner == "-"+tok {
			return inner
		}
	}
	return tok
}

// isDigits reports whether s is a non-empty run of decimal digits.
func isDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
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
