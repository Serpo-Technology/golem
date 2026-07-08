package daemon

import (
	"strconv"

	"golem/internal/code"
)

// A comparative morphology rule is one function-as-graph over the string primitives,
// composed here in Go, stored as a code:<name> construction, tagged in the meta layer,
// and RUN OFFLINE during the grow so its output freezes into the durable
// RelComparativeForm edge the serve path reads. Nothing about rule / regular / irregular
// is Go; only the composition of the primitives is, the way the arithmetic floor is Go.
//
// The English "-er" rule is ONE ordered nested conditional cascade with explicit
// precedence, not four sibling cases: y -> i, then silent e, then consonant doubling on
// a CVC monosyllable, then a plain suffix as the fall-through. Written as siblings the
// pieces emit wrong forms ("biger", "happyer"); grounded as a whole on the held-out set
// they emit "bigger", "larger", "happier", "faster". The class predicates (vowel,
// consonant) are decision trees over == disjunctions, never a boolean operator or a
// strings.* call, so the whole rule stays import-free and a walkable graph.

// mLit is a quoted string literal leaf, its leading quote the durable string marker.
func mLit(s string) code.Computation { return code.Computation{Lit: strconv.Quote(s)} }

// mVar is a reference to the rule's parameter (the stem).
func mVar(name string) code.Computation { return code.Computation{Var: name} }

// mCall builds a string-primitive application over its operands.
func mCall(op string, args ...code.Computation) code.Computation {
	return code.Computation{Op: op, Args: args}
}

// mEq builds "if left == right then t else e", the one control the cascade turns on:
// every test is string equality over a peeled character.
func mEq(left, right, t, e code.Computation) code.Computation {
	return code.Computation{Cond: &code.Conditional{Cmp: "==", Left: left, Right: right, Then: t, Else: e}}
}

// vowels is the fixed set the class predicates test against, ordered so the decision
// tree is deterministic across runs.
var vowels = []string{"a", "e", "i", "o", "u"}

// mIsVowel yields t when the character c is a vowel, else e: a right-nested cascade of
// equality tests over the vowel set, the import-free stand-in for a membership test.
func mIsVowel(c, t, e code.Computation) code.Computation {
	out := e
	for i := len(vowels) - 1; i >= 0; i-- {
		out = mEq(c, mLit(vowels[i]), t, out)
	}
	return out
}

// mIsConsonant yields t when c is a consonant, else e, the complement of mIsVowel.
func mIsConsonant(c, t, e code.Computation) code.Computation {
	return mIsVowel(c, e, t)
}

// buildCmpEN composes the English "-er" comparative rule over the stem parameter. The
// character peels (last, second-last, third-last) are read with lastChar / dropLast, and
// the doubling case reaches inside the stem to copy its final consonant, which is why
// lastChar is mandatory: concat alone cannot double a consonant.
func buildCmpEN(param string) code.Op {
	stem := mVar(param)
	pre := mCall("dropLast", stem)                  // stem without its last character
	c1 := mCall("lastChar", stem)                   // the final character
	c2 := mCall("lastChar", pre)                    // the second-from-last
	c3 := mCall("lastChar", mCall("dropLast", pre)) // the third-from-last

	plain := mCall("concat", stem, mLit("er"))                       // fast -> faster
	double := mCall("concat", mCall("concat", stem, c1), mLit("er")) // big -> bigger
	yToI := mCall("concat", pre, mLit("ier"))                        // happy -> happier
	silentE := mCall("concat", pre, mLit("er"))                      // large -> larger

	// The CVC-doubling branch, reached only after y and silent-e are ruled out: a final
	// consonant (not w / x / y), a vowel before it, and a consonant before that, else the
	// plain suffix. It folds "cvc else plain" into one decision so the cascade's else is
	// the plain fall-through.
	cvc := mIsConsonant(c1,
		mEq(c1, mLit("w"), plain,
			mEq(c1, mLit("x"), plain,
				mEq(c1, mLit("y"), plain,
					mIsVowel(c2, mIsConsonant(c3, double, plain), plain)))),
		plain)

	// The ordered cascade: y after a consonant turns to i (a vowel before y falls through,
	// so "stay" -> "stayer" not "staier"); a silent e is dropped; a CVC monosyllable
	// doubles; anything else takes the plain suffix.
	body := mEq(c1, mLit("y"),
		mIsConsonant(c2, yToI, plain),
		mEq(c1, mLit("e"),
			silentE,
			cvc))

	return code.Op{Name: "cmpEN", Word: "cmpEN", Params: []string{param}, ParamTypes: []string{"string"}, Out: "string", Body: &body}
}

// buildCmpTR composes the Turkish comparative rule: the preposed particle "daha " ahead
// of the adjective, deposited whole so the surface reads as a comparative and not the
// bare positive. prepend touches no character, so the multibyte adjective is safe.
func buildCmpTR(param string) code.Op {
	body := mCall("prepend", mLit("daha "), mVar(param))
	return code.Op{Name: "cmpTR", Word: "cmpTR", Params: []string{param}, ParamTypes: []string{"string"}, Out: "string", Body: &body}
}

// comparativeRule builds the morphology rule a tongue's comparative is realized by, the
// composition kept in Go while the rule / regular / irregular notions stay graph data. It
// returns false for a tongue with no productive rule (Latin, whose comparatives are
// suppletive overrides only).
func comparativeRule(lang string) (code.Op, bool) {
	switch lang {
	case "en":
		return buildCmpEN("stem"), true
	case "tr":
		return buildCmpTR("adj"), true
	}
	return code.Op{}, false
}

// pureMorphOps is the allowlist a morphology rule's body may compose over, the pure,
// deterministic subset that grounds the same output every run: the string primitives,
// string equality, and the conditional shape. A rule referencing any operation outside it
// (a time- or rand-backed op reachable through the runtime allowlist) is refused at
// induction, so determinism is enforced structurally on the Computation, never by trusting
// the runner.
var pureMorphOps = map[string]bool{"concat": true, "prepend": true, "lastChar": true, "dropLast": true}

// isPureMorph reports whether a computation composes only over the pure morphology
// allowlist: string primitives, string-equality conditionals, literals, and the parameter.
// It walks the whole body so a non-pure op buried in a branch is caught.
func isPureMorph(c code.Computation) bool {
	if c.Map != nil || c.Each {
		return false
	}
	if c.Cond != nil {
		if c.Cond.Cmp != "==" && c.Cond.Cmp != ">" && c.Cond.Cmp != "<" {
			return false
		}
		return isPureMorph(c.Cond.Left) && isPureMorph(c.Cond.Right) &&
			isPureMorph(c.Cond.Then) && isPureMorph(c.Cond.Else)
	}
	if c.Op != "" {
		if !pureMorphOps[c.Op] {
			return false
		}
	}
	for _, a := range c.Args {
		if !isPureMorph(a) {
			return false
		}
	}
	return true
}
