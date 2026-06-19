package daemon

import (
	"strconv"
	"testing"
	"time"

	"golem/internal/grammar"
	"golem/internal/memory"
)

// The shared traverser renders Go code: walking a code subgraph through goSurface emits
// the same source genCodeFromGraph does, and it runs to the same value. One driver, the
// Go surface, reproducing the code generator on the binding-free shapes.
func TestTraverseRendersCode(t *testing.T) {
	d, s := testDaemon(t)
	d.cfg.RuntimeEnabled = true

	mk := func(content string) uint64 {
		f := &memory.Fragment{Kind: memory.KindConcept, Channel: "test", Content: content, Time: time.Now()}
		if _, err := s.Encode(f); err != nil {
			t.Fatal(err)
		}
		return f.ID
	}
	link := func(from, to uint64, rel string) {
		if err := s.LinkRel(from, to, rel, 1); err != nil {
			t.Fatal(err)
		}
	}
	op := func(name string, l, r uint64) uint64 {
		root := mk("")
		link(root, mk(name), memory.RelGrounds)
		link(root, l, operand0Rel)
		link(root, r, operand1Rel)
		return root
	}
	slotInto := func(parent uint64, rel string, rank int, val uint64) {
		sl := mk("")
		link(parent, sl, rel)
		link(sl, mk(strconv.Itoa(rank)), rankRel)
		link(sl, val, holdsRel)
	}
	list := func(vals ...uint64) uint64 {
		root := mk("")
		link(root, mk("list"), memory.RelGrounds)
		for i, v := range vals {
			slotInto(root, elemRel, i, v)
		}
		return root
	}
	cond := func(cmp string, l, r, then, els uint64) uint64 {
		root := mk(cmp)
		link(root, l, condLeftRel)
		link(root, r, condRightRel)
		link(root, then, condThenRel)
		link(root, els, condElseRel)
		return root
	}

	g := d.storeView()
	roots := []uint64{
		op("add", mk("6"), mk("7")),                          // a flat operation
		op("multiply", op("add", mk("2"), mk("3")), mk("4")), // a nested one, parenthesized
		list(mk("1"), mk("2"), mk("3")),                      // a list literal
		cond(">", mk("6"), mk("4"), mk("6"), mk("4")),        // a conditional
	}
	for _, root := range roots {
		got, ok := d.traverse(goSurface{}, g, root, nil)
		want, wok := d.genCodeFromGraph(root)
		if !ok || !wok || got.src != want {
			t.Fatalf("traversed %q (ok %v) should equal genCode %q (ok %v)", got.src, ok, want, wok)
		}
		gv, gok := d.evalCode(got.src)
		wv, wvok := d.evalCode(want)
		if !gok || !wvok || gv != wv {
			t.Fatalf("traversed %q (=%q) should run to genCode %q (=%q)", got.src, gv, want, wv)
		}
	}
}

// The same traverser renders English: walking a proposition subgraph through nlSurface
// emits the clause grammar.Speak produces from the equivalent meaning. One driver, the
// English surface, reproducing the grammar engine on a transitive clause.
func TestTraverseRendersClause(t *testing.T) {
	d, s := testDaemon(t)

	mk := func(content string) uint64 {
		f := &memory.Fragment{Kind: memory.KindConcept, Channel: "test", Content: content, Time: time.Now()}
		if _, err := s.Encode(f); err != nil {
			t.Fatal(err)
		}
		return f.ID
	}
	link := func(from, to uint64, rel string) {
		if err := s.LinkRel(from, to, rel, 1); err != nil {
			t.Fatal(err)
		}
	}
	// A proposition: the dog chased the cat (past). The moment grounds in its relation
	// and binds the agent and patient; its content marks the past tense.
	moment := mk("past")
	link(moment, mk("chase"), memory.RelGrounds)
	link(moment, mk("dog"), memory.RelAgent)
	link(moment, mk("cat"), memory.RelPatient)

	got, ok := d.traverse(nlSurface{lang: "en"}, d.storeView(), moment, nil)
	if !ok {
		t.Fatal("the clause should traverse")
	}
	want, wok := d.gram.Speak("en", grammar.Meaning{
		Relation: grammar.Term{Phrase: "chase"},
		Roles: map[string]grammar.Term{
			"agent":   {Phrase: "dog"},
			"patient": {Phrase: "cat", Definite: true},
		},
		Past: true,
		Lang: "en",
	})
	if !wok {
		t.Fatal("grammar should speak the transitive clause")
	}
	if got.src != want {
		t.Fatalf("the traversed clause %q should match Speak %q", got.src, want)
	}
}

// The same clause render serves Turkish through the language's parameters: the verb closes
// the clause (verb-final) and the object takes the accusative, the layout and morphology
// coming from grammar. The traversed clause matches Speak in Turkish too, so one driver
// renders the clause across languages.
func TestTraverseRendersClauseTR(t *testing.T) {
	d, s := testDaemon(t)

	mk := func(content string) uint64 {
		f := &memory.Fragment{Kind: memory.KindConcept, Channel: "test", Content: content, Time: time.Now()}
		if _, err := s.Encode(f); err != nil {
			t.Fatal(err)
		}
		return f.ID
	}
	link := func(from, to uint64, rel string) {
		if err := s.LinkRel(from, to, rel, 1); err != nil {
			t.Fatal(err)
		}
	}
	moment := mk("past")
	link(moment, mk("kovala"), memory.RelGrounds)
	link(moment, mk("köpek"), memory.RelAgent)
	link(moment, mk("kedi"), memory.RelPatient)

	got, ok := d.traverse(nlSurface{lang: "tr"}, d.storeView(), moment, nil)
	if !ok {
		t.Fatal("the Turkish clause should traverse")
	}
	want, wok := d.gram.Speak("tr", grammar.Meaning{
		Relation: grammar.Term{Phrase: "kovala"},
		Roles: map[string]grammar.Term{
			"agent":   {Phrase: "köpek"},
			"patient": {Phrase: "kedi", Definite: true},
		},
		Past: true,
		Lang: "tr",
	})
	if !wok {
		t.Fatal("grammar should speak the Turkish clause")
	}
	if got.src != want {
		t.Fatalf("the traversed Turkish clause %q should match Speak %q", got.src, want)
	}
}

// A role with more than one filler coordinates: a clause whose patient binds two concepts
// speaks them joined by the coordinator, matching Speak's co-argument render.
func TestTraverseRendersCoordination(t *testing.T) {
	d, s := testDaemon(t)

	mk := func(content string) uint64 {
		f := &memory.Fragment{Kind: memory.KindConcept, Channel: "test", Content: content, Time: time.Now()}
		if _, err := s.Encode(f); err != nil {
			t.Fatal(err)
		}
		return f.ID
	}
	link := func(from, to uint64, rel string) {
		if err := s.LinkRel(from, to, rel, 1); err != nil {
			t.Fatal(err)
		}
	}
	moment := mk("past")
	link(moment, mk("chase"), memory.RelGrounds)
	link(moment, mk("dog"), memory.RelAgent)
	link(moment, mk("cat"), memory.RelPatient)
	link(moment, mk("mouse"), memory.RelPatient)

	got, ok := d.traverse(nlSurface{lang: "en"}, d.storeView(), moment, nil)
	if !ok {
		t.Fatal("the coordinated clause should traverse")
	}
	want, wok := d.gram.Speak("en", grammar.Meaning{
		Relation: grammar.Term{Phrase: "chase"},
		Roles: map[string]grammar.Term{
			"agent":   {Phrase: "dog"},
			"patient": {Phrase: "cat", With: []string{"mouse"}, Definite: true},
		},
		Past: true,
		Lang: "en",
	})
	if !wok {
		t.Fatal("grammar should speak the coordinated clause")
	}
	if got.src != want {
		t.Fatalf("the traversed coordinated clause %q should match Speak %q", got.src, want)
	}
}

// The Go surface widens to calls and recursion: walking a call subgraph through the
// driver inlines a non-recursive function (binding its parameters to the rendered
// arguments and walking the body) and writes a recursive one as its local-function form,
// matching genCode and running the same. The shapes come from the daemon's own
// comprehension, materialized as a subgraph.
func TestTraverseRendersCalls(t *testing.T) {
	d, _ := testDaemon(t)
	d.cfg.RuntimeEnabled = true
	say(t, d, "the triple of a number is the product of the number and 3")
	say(t, d, "the factorial of a number is 1 if the number is below 1 else the product of the number and the factorial of the difference of the number and 1")
	if err := d.recompileGrammar(); err != nil {
		t.Fatal(err)
	}

	for _, utterance := range []string{"the triple of 4", "the factorial of 5"} {
		comp, ok := d.code.Comprehend(utterance)
		if !ok {
			t.Fatalf("the engine should comprehend %q", utterance)
		}
		g := newScratchGraph()
		root, mok := d.materializeCode(g, comp, "", map[string]uint64{})
		if !mok {
			t.Fatalf("the computation should materialize: %q", utterance)
		}
		got, gok := d.traverse(goSurface{}, g, root, nil)
		want, _, wok := d.genCode(g, root, nil, 0)
		if !gok || !wok || got.src != want {
			t.Fatalf("%q: traversed %q (ok %v) should equal genCode %q (ok %v)", utterance, got.src, gok, want, wok)
		}
		gv, gvok := d.evalCode(got.src)
		wv, wvok := d.evalCode(want)
		if !gvok || !wvok || gv != wv {
			t.Fatalf("%q: traversed %q (=%q) should run to genCode %q (=%q)", utterance, got.src, gv, want, wv)
		}
	}
}
