package daemon

import (
	"bytes"
	"strconv"
	"strings"
	"testing"

	"golem/internal/code"
	"golem/internal/memory"
	"golem/wire"
)

// The string primitives lower to import-free Go and run end to end: an infix join
// (concat, prepend) and an ASCII byte slice (lastChar, dropLast). The offline runner
// evaluates a bare expression with no import statement, so a primitive that emitted a
// strings.* selector would fail to resolve; these emit only "+", slicing, and len.
func TestStringPrimitivesEvalImportFree(t *testing.T) {
	d, _ := testDaemon(t)
	d.cfg.RuntimeEnabled = true

	lit := func(s string) code.Computation { return code.Computation{Lit: strconv.Quote(s)} }
	run := func(c code.Computation) (string, bool) {
		src, ok := d.code.ToCode(c)
		if !ok {
			t.Fatalf("the typed render should produce source for %+v", c)
		}
		return d.evalCode(src)
	}

	cases := []struct {
		name string
		comp code.Computation
		want string
	}{
		{"prepend", code.Computation{Op: "prepend", Args: []code.Computation{lit("daha "), lit("buyuk")}}, "daha buyuk"},
		{"concat", code.Computation{Op: "concat", Args: []code.Computation{lit("big"), lit("er")}}, "biger"},
		{"lastChar", code.Computation{Op: "lastChar", Args: []code.Computation{lit("big")}}, "g"},
		{"dropLast", code.Computation{Op: "dropLast", Args: []code.Computation{lit("big")}}, "bi"},
		// A nested slice inside a concat, the shape the -er cascade doubles a consonant with:
		// concat(concat("big", lastChar("big")), "er") -> "bigger".
		{"double", code.Computation{Op: "concat", Args: []code.Computation{
			{Op: "concat", Args: []code.Computation{lit("big"), {Op: "lastChar", Args: []code.Computation{lit("big")}}}},
			lit("er"),
		}}, "bigger"},
	}
	for _, tc := range cases {
		got, ok := run(tc.comp)
		if !ok || got != tc.want {
			t.Fatalf("%s should run to %q, got %q ok=%v", tc.name, tc.want, got, ok)
		}
	}
}

// The graph twin and the typed render emit byte-identical Go for the string shapes,
// including a nested two-operand rule (the consonant-doubling shape), so the
// codematerialize mismatch tripwire stays silent when generation walks the graph. This
// is the check that both executors agree before the tripwire is trusted to enforce it.
func TestStringShapeShadowByteIdentical(t *testing.T) {
	d, _ := testDaemon(t)
	d.cfg.RuntimeEnabled = true
	var buf bytes.Buffer
	d.out = &buf

	lit := func(s string) code.Computation { return code.Computation{Lit: strconv.Quote(s)} }
	call := func(op string, args ...code.Computation) code.Computation {
		return code.Computation{Op: op, Args: args}
	}
	cases := []struct {
		name, wantSrc, wantVal string
		comp                   code.Computation
	}{
		{"concat", `"big" + "er"`, "biger", call("concat", lit("big"), lit("er"))},
		{"prepend", `"daha " + "buyuk"`, "daha buyuk", call("prepend", lit("daha "), lit("buyuk"))},
		{"lastChar", `"big"[len("big")-1:]`, "g", call("lastChar", lit("big"))},
		{"dropLast", `"big"[:len("big")-1]`, "bi", call("dropLast", lit("big"))},
		{
			"nested-double", `("big" + "big"[len("big")-1:]) + "er"`, "bigger",
			call("concat", call("concat", lit("big"), call("lastChar", lit("big"))), lit("er")),
		},
	}
	for _, tc := range cases {
		walked, wok := d.traverseCode(tc.comp)
		typed, tok := d.code.ToCode(tc.comp)
		if !wok || !tok {
			t.Fatalf("%s should render on both twins (walk %v, typed %v)", tc.name, wok, tok)
		}
		if walked != tc.wantSrc || typed != tc.wantSrc {
			t.Fatalf("%s: want %q, walked %q, typed %q", tc.name, tc.wantSrc, walked, typed)
		}
		gen, gok := d.generateCode(tc.comp, nil, 0)
		if !gok || gen != tc.wantSrc {
			t.Fatalf("%s: generateCode should emit %q, got %q ok=%v", tc.name, tc.wantSrc, gen, gok)
		}
		if v, ok := d.evalCode(gen); !ok || v != tc.wantVal {
			t.Fatalf("%s: %q should run to %q, got %q ok=%v", tc.name, gen, tc.wantVal, v, ok)
		}
		if !d.typeCheckGraph(newScratchGraphFrom(t, d, tc.comp)) {
			t.Fatalf("%s: the string shape should type-check on the graph", tc.name)
		}
	}
	if strings.Contains(buf.String(), "mismatch") {
		t.Fatalf("the two executors should agree on the string shapes, but logged: %q", buf.String())
	}
}

// newScratchGraphFrom materializes a computation into a fresh scratch graph and returns
// it with its root, so a test can type-check the graph the twin walks.
func newScratchGraphFrom(t *testing.T, d *Daemon, c code.Computation) (*scratchGraph, uint64) {
	t.Helper()
	g := newScratchGraph()
	root, ok := d.materializeCode(g, c, "", map[string]uint64{})
	if !ok {
		t.Fatalf("the computation should materialize: %+v", c)
	}
	return g, root
}

// The English "-er" rule, composed as one ordered nested conditional cascade and run
// offline, emits exactly the held-out forms in both directions: forward it yields
// bigger / larger / happier / faster (never biger / happyer), and each output decomposes
// back through stemComparativeEN to the stem it was built from. The rule persists through
// learnFunction and reloads structurally, so what runs is the reloaded graph, not the
// in-memory builder. The graph twin and the typed render agree byte for byte on the whole
// cascade, so the mismatch tripwire stays silent.
func TestCmpENCascadeGrounds(t *testing.T) {
	d, _ := testDaemon(t)
	d.cfg.RuntimeEnabled = true
	var buf bytes.Buffer
	d.out = &buf

	if _, err := d.learnFunction(buildCmpEN("stem")); err != nil {
		t.Fatalf("learning cmpEN: %v", err)
	}
	// The rule reloaded from its structural (morph) storage, not the in-memory builder.
	if op, ok := d.opByName("cmpEN"); !ok || op.Body == nil {
		t.Fatal("cmpEN should reload as a defined function")
	}

	pairs := []struct{ stem, want string }{
		{"big", "bigger"},
		{"large", "larger"},
		{"happy", "happier"},
		{"fast", "faster"},
	}
	for _, p := range pairs {
		comp := code.Computation{Op: "cmpEN", Args: []code.Computation{{Lit: strconv.Quote(p.stem)}}}
		src, ok := d.generateCode(comp, nil, 0)
		if !ok {
			t.Fatalf("cmpEN(%q) should generate", p.stem)
		}
		got, ok := d.evalCode(src)
		if !ok || got != p.want {
			t.Fatalf("cmpEN(%q) should run to %q, got %q ok=%v", p.stem, p.want, got, ok)
		}
		if got == p.stem+"er" && p.want != p.stem+"er" {
			t.Fatalf("cmpEN(%q) regularized to %q instead of %q", p.stem, got, p.want)
		}
		// The reverse stemmer undoes the forward orthography: the stem is among the
		// candidates the comparative surface decomposes to.
		cands := stemComparativeEN(p.want)
		found := false
		for _, c := range cands {
			if c == p.stem {
				found = true
			}
		}
		if !found {
			t.Fatalf("stemComparativeEN(%q) should offer %q, got %v", p.want, p.stem, cands)
		}
	}
	// The rule alone would regularize an irregular ("good" -> "gooder"): the override, not
	// the rule, is what keeps "gooder" from ever being deposited (Stage 5). This documents
	// why the override check must run before the rule.
	comp := code.Computation{Op: "cmpEN", Args: []code.Computation{{Lit: strconv.Quote("good")}}}
	if src, ok := d.generateCode(comp, nil, 0); ok {
		if got, _ := d.evalCode(src); got != "gooder" {
			t.Fatalf("cmpEN(good) should regularize to gooder (the reason an override is needed), got %q", got)
		}
	}
	if strings.Contains(buf.String(), "mismatch") {
		t.Fatalf("the two executors should agree on the -er cascade, but logged: %q", buf.String())
	}
}

// The offline derivation runs the tagged rule during the grow and freezes the surface
// into the durable edge, so the serve path speaks the derived comparative with the runtime
// CLOSED. The English poles derive bigger / smaller through the -er cascade and the Turkish
// poles daha büyük / daha küçük through the particle rule, all from one keyed rule per
// tongue. The deposited form carries its base -> rule -> form provenance.
func TestOfflineDerivationServesCodeFree(t *testing.T) {
	d, _ := testDaemon(t)
	d.cfg.RuntimeEnabled = true
	cmd := func(req wire.Request) {
		t.Helper()
		if resp, _, _ := d.handle(req, nil); !resp.OK {
			t.Fatalf("%s failed: %s", req.Cmd, resp.Error)
		}
	}
	cmd(wire.Request{Cmd: "scale", Content: "size", Query: "big", Command: "small"})
	big, small := d.resolveConcept("big"), d.resolveConcept("small")
	if big == nil || small == nil {
		t.Fatal("scale should mint the poles")
	}
	d.nameConceptWord(big.ID, "büyük", "tr")
	d.nameConceptWord(small.ID, "küçük", "tr")
	cmd(wire.Request{Cmd: "cmprule", Lang: "en", Content: "cmpEN", Query: "comparative"})
	cmd(wire.Request{Cmd: "cmprule", Lang: "tr", Content: "cmpTR", Query: "comparative"})
	if name, ok := d.comparativeRuleName("en"); !ok || name != "cmpEN" {
		t.Fatalf("the en rule should select cmpEN, got %q ok=%v", name, ok)
	}
	cmd(wire.Request{Cmd: "cmpderive"})

	// Serve with the runtime CLOSED: the surface is read from the durable edge, never run.
	d.cfg.RuntimeEnabled = false
	for _, c := range []struct {
		pole       *memory.Fragment
		lang, want string
	}{
		{big, "en", "bigger"}, {small, "en", "smaller"},
		{big, "tr", "daha büyük"}, {small, "tr", "daha küçük"},
	} {
		if got := d.realizeComparative(c.pole, c.lang, ""); got != c.want {
			t.Fatalf("realizeComparative(%s,%s)=%q want %q", c.pole.Content, c.lang, got, c.want)
		}
	}

	// Provenance: the deposited English form is marked meta:regular and links to the rule
	// construction, so base -> rule -> form is walkable.
	formID := d.comparativeFormID(t, big, "en")
	if !d.marks(formID, d.scaffoldID("meta:regular")) {
		t.Fatal("a derived form should be marked meta:regular")
	}
	ruleNode, err := d.store.FragmentByTemplate("code:cmpEN")
	if err != nil || ruleNode == nil || !d.marks(formID, ruleNode.ID) {
		t.Fatal("a derived form should link to its rule construction")
	}

	// Idempotent: a second sweep deposits nothing new and never overwrites.
	d.cfg.RuntimeEnabled = true
	if resp, _, _ := d.handle(wire.Request{Cmd: "cmpderive"}, nil); !resp.OK || !strings.Contains(resp.Say, "derived 0") {
		t.Fatalf("a re-run should derive 0, got %q ok=%v", resp.Say, resp.OK)
	}
}

// comparativeFormID reads the id of a pole's deposited comparative surface in a tongue.
func (d *Daemon) comparativeFormID(t *testing.T, pole *memory.Fragment, tongue string) uint64 {
	t.Helper()
	edges, err := d.store.LinksFromRel([]uint64{pole.ID}, memory.RelComparativeForm)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range edges {
		frags, err := d.store.FragmentsByIDs([]uint64{e.To})
		if err == nil && len(frags) == 1 && frags[0].Lang == tongue {
			return e.To
		}
	}
	t.Fatalf("no comparative form for %s in %s", pole.Content, tongue)
	return 0
}

// An irregular override pre-empts the rule in both directions: the hand-stored surface is
// served rather than a regularized guess, the sweep never overwrites it, and the reverse
// RelComparativeForm lookup resolves the suppletive surface to its pole without the stemmer
// producing junk. This holds for the English good -> better and the Latin magnus -> maior,
// the same reverse lookup dissolving the Latin latinComparativeBase map into graph overrides.
func TestIrregularOverridePreemptsAndReverses(t *testing.T) {
	d, _ := testDaemon(t)
	d.cfg.RuntimeEnabled = true
	cmd := func(req wire.Request) {
		t.Helper()
		if resp, _, _ := d.handle(req, nil); !resp.OK {
			t.Fatalf("%s failed: %s", req.Cmd, resp.Error)
		}
	}
	cmd(wire.Request{Cmd: "scale", Content: "quality", Query: "good", Command: "bad"})
	good := d.resolveConcept("good")
	cmd(wire.Request{Cmd: "cmprule", Lang: "en"})
	// The irregulars are hand-stored before the sweep, so the reverse side resolves them and
	// the sweep cannot overwrite them.
	cmd(wire.Request{Cmd: "cmpform", Content: "good", Query: "better"})
	cmd(wire.Request{Cmd: "cmpform", Content: "bad", Query: "worse"})

	// Latin: a suppletive override, no productive rule, dissolving the old Go map.
	cmd(wire.Request{Cmd: "scale", Content: "magnitudo", Query: "magnus", Command: "parvus", Lang: "la"})
	magnus := d.resolveConcept("magnus")
	cmd(wire.Request{Cmd: "cmpform", Content: "magnus", Query: "maior", Lang: "la"})
	cmd(wire.Request{Cmd: "cmpform", Content: "parvus", Query: "minor", Lang: "la"})

	cmd(wire.Request{Cmd: "cmpderive"})

	d.cfg.RuntimeEnabled = false
	// The sweep never overwrote the irregulars: "better", not "gooder".
	if got := d.realizeComparative(good, "en", ""); got != "better" {
		t.Fatalf("the sweep must not overwrite the irregular: got %q", got)
	}
	// The reverse lookup resolves the suppletive surface to its pole, fail-closed, without
	// stemComparativeEN ever producing junk.
	if pole, _, sign, ok := d.comparativeToPole("better", "en"); !ok || pole == nil || pole.ID != good.ID || sign != "gt" {
		t.Fatalf("\"better\" should resolve to the good pole (gt), got ok=%v sign=%q", ok, sign)
	}
	if pole, _, sign, ok := d.comparativeToPole("maior", "la"); !ok || pole == nil || pole.ID != magnus.ID || sign != "gt" {
		t.Fatalf("\"maior\" should resolve to the magnus pole (gt), got ok=%v sign=%q", ok, sign)
	}
	if !d.isLatinComparativeWord("maior") {
		t.Fatal("maior should be recognized as a Latin comparative from the graph override")
	}
	if d.isLatinComparativeWord("vacca") {
		t.Fatal("a plain noun should not be read as a comparative")
	}
	// A regular English comparative still decomposes by stemming, not the reverse override,
	// so the two paths do not shadow each other.
	if pole, _, _, ok := d.comparativeToPole("better", "en"); !ok || pole.ID != good.ID {
		t.Fatal("the reverse override should own the irregular token")
	}
}

// An analytic-marked pole is realized "more <base>" without running the synthetic rule,
// the driver-level branch that keeps the -er rule from ever depositing "cleverer". The
// rule body cannot make this choice (its only input is the stem), so the derivation reads
// the pole's cmp:analytic mark before selecting a rule.
func TestAnalyticPoleDerivesMore(t *testing.T) {
	d, _ := testDaemon(t)
	d.cfg.RuntimeEnabled = true
	cmd := func(req wire.Request) {
		t.Helper()
		if resp, _, _ := d.handle(req, nil); !resp.OK {
			t.Fatalf("%s failed: %s", req.Cmd, resp.Error)
		}
	}
	cmd(wire.Request{Cmd: "scale", Content: "wit", Query: "clever", Command: "dull"})
	cmd(wire.Request{Cmd: "cmprule", Lang: "en"})
	clever := d.resolveConcept("clever")
	d.store.SetLinkRel(clever.ID, d.scaffoldID("cmp:analytic"), memory.RelMarks, deliberateName)
	cmd(wire.Request{Cmd: "cmpderive"})
	if got := d.realizeComparative(clever, "en", ""); got != "more clever" {
		t.Fatalf("an analytic pole should realize \"more clever\", got %q", got)
	}
}

// The determinism guard is enforced structurally on the rule Computation: a body composing
// only the pure string primitives passes, one reaching a time- or rand-backed operation is
// refused at induction, so the offline runner's exposure to math/rand and time cannot make
// a rule nondeterministic.
func TestMorphologyDeterminismGuard(t *testing.T) {
	if !isPureMorph(*buildCmpEN("stem").Body) {
		t.Fatal("the -er cascade should pass the pure-op guard")
	}
	if !isPureMorph(*buildCmpTR("adj").Body) {
		t.Fatal("the particle rule should pass the pure-op guard")
	}
	// A rule that reaches a non-morphology operation (here an arithmetic add standing for any
	// time- or rand-backed op the runtime allowlist exposes) is refused.
	impure := code.Computation{Op: "concat", Args: []code.Computation{
		{Var: "stem"},
		{Op: "add", Args: []code.Computation{{Lit: "1"}, {Lit: "2"}}},
	}}
	if isPureMorph(impure) {
		t.Fatal("a rule reaching a non-pure op must be refused")
	}
}

// The "+" the infix string primitives emit stays owned by code:add on the raw-source
// parse path: a quoted argument types as a string and a bare numeral as an int, so the
// string primitives never register a symbol and "6 + 7" still comprehends as add.
func TestPlusStaysAddOnRawSource(t *testing.T) {
	d, _ := testDaemon(t)
	comp, ok := d.code.Comprehend("6 + 7")
	if !ok {
		t.Fatal("6 + 7 should comprehend")
	}
	if comp.Op != "add" {
		t.Fatalf("the raw + should still resolve to add, got %q", comp.Op)
	}
}
