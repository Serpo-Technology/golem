package daemon

import (
	"bytes"
	"strings"
	"testing"

	"golem/internal/grammar"
	"golem/internal/memory"
)

// The declarative grammar's phrase rules live as concepts in the graph, each carrying its form pole as
// a slot subgraph, and the graph representation round-trips the compiled rule (the derive stays silent).
// The rule concepts are system scaffolding, kept out of the grown layer.
func TestDeclRulesLiveInGraph(t *testing.T) {
	d, s := testDaemon(t) // boot ran ensureDeclGrammar
	var buf bytes.Buffer
	d.out = &buf
	if err := d.recompileGrammar(); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(buf.String(), "declgrammar mismatch") {
		t.Fatalf("the graph-derived phrase rules should match the seed:\n%s", buf.String())
	}

	c, err := s.ConceptByTemplate("syn:en:s")
	if err != nil || c == nil {
		t.Fatalf("the main clause rule should be a concept in the graph: %v", err)
	}
	if !d.isSystemConcept(c.ID) {
		t.Fatal("a phrase rule concept should be excluded as system scaffolding")
	}

	// It reads back as its daughter order, head, and the subject-verb agreement flow.
	got := d.declSynFromGraph(d.storeView(), c.ID)
	if strings.Join(got.order, " ") != "subj vp" {
		t.Fatalf("the main clause rule should order subj before vp, got %v", got.order)
	}
	if got.head != "vp" {
		t.Fatalf("the main clause rule's head should be vp, got %q", got.head)
	}
	if len(got.flows) != 1 || got.flows[0] != (declFlow{feat: "num", from: "subj", to: "vp"}) {
		t.Fatalf("the main clause rule should flow num from subj to vp, got %+v", got.flows)
	}
}

// A phrase rule's form pole is derived from the graph: where a stored parameter disagrees with the
// concept the graph holds, the graph drives and the tripwire fires. So editing a rule in the graph
// reaches the engine, the cutover that makes the graph the source.
func TestDeclRulesDeriveFromGraph(t *testing.T) {
	d, _ := testDaemon(t)
	gnodes, err := d.store.NodesByKind(memory.KindGrammar)
	if err != nil {
		t.Fatal(err)
	}
	gparams := paramsOf(gnodes)
	idx := -1
	for i, n := range gnodes {
		if n.Template == "syn:en:s" {
			gparams[i].Value = "vp | | vp" // a seed disagreeing with the graph's subject-first clause
			idx = i
		}
	}
	if idx < 0 {
		t.Fatal("the main clause rule node should exist")
	}
	var buf bytes.Buffer
	d.out = &buf
	d.deriveDeclGrammar(gnodes, gparams)
	if gparams[idx].Value != "subj vp | num:subj>vp | vp" {
		t.Fatalf("the graph should drive the rule's form pole, got %q", gparams[idx].Value)
	}
	if !strings.Contains(buf.String(), "declgrammar mismatch") {
		t.Fatalf("a seed disagreeing with the graph should log the tripwire:\n%s", buf.String())
	}
}

// A function word's inflected forms live on its own fw node: the realizer category and each form keyed
// by its agreement value, so a word's morphology lives with the word. The derive of the lex pole stays
// silent against the seed.
func TestDeclLexLivesOnWordNode(t *testing.T) {
	d, s := testDaemon(t) // boot ran ensureDeclGrammar, attaching lex forms to fw nodes
	var buf bytes.Buffer
	d.out = &buf
	if err := d.recompileGrammar(); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(buf.String(), "declgrammar lex mismatch") {
		t.Fatalf("the word-node-derived lex forms should match the seed:\n%s", buf.String())
	}

	gnodes, err := s.NodesByKind(memory.KindGrammar)
	if err != nil {
		t.Fatal(err)
	}
	var doNode *memory.Fragment
	for _, n := range gnodes {
		if n.Template == "fw:en:do" {
			doNode = n
		}
	}
	if doNode == nil {
		t.Fatal("the do function word should be an fw node")
	}
	// The do node carries its realizer category (aux, distinct from its parser category imp) and its
	// suppletive forms read back off the node.
	lx := d.declLexFromGraph(d.storeView(), doNode.ID)
	if lx.cat != "aux" {
		t.Fatalf("do's realizer category should be aux, got %q", lx.cat)
	}
	for k, want := range map[string]string{"sg": "does", "pl": "do", "past": "did"} {
		if lx.forms[k] != want {
			t.Fatalf("do's %s form should be %q, got %q", k, want, lx.forms[k])
		}
	}
}

// A word's lex pole is derived from its node: where a stored parameter disagrees with the forms the node
// carries, the node drives and the tripwire fires. So editing a word's forms on its node reaches the
// realizer.
func TestDeclLexDeriveFromNode(t *testing.T) {
	d, _ := testDaemon(t)
	gnodes, err := d.store.NodesByKind(memory.KindGrammar)
	if err != nil {
		t.Fatal(err)
	}
	gparams := paramsOf(gnodes)
	idx := -1
	for i, n := range gnodes {
		if n.Template == "lex:en:do" {
			gparams[i].Value = "cat=aux;sg=does" // a seed missing the forms the node carries
			idx = i
		}
	}
	if idx < 0 {
		t.Fatal("the do lex node should exist")
	}
	var buf bytes.Buffer
	d.out = &buf
	d.deriveDeclLex(gnodes, gparams)
	if gparams[idx].Value != "cat=aux;past=did;pl=do;sg=does" {
		t.Fatalf("the node should drive do's lex pole, got %q", gparams[idx].Value)
	}
	if !strings.Contains(buf.String(), "declgrammar lex mismatch") {
		t.Fatalf("a seed disagreeing with the node should log the tripwire:\n%s", buf.String())
	}
}

// A closed-class word with no fw node (the copula) gets a created lexeme word node that hosts its
// realizer category and irregular forms, so even a word with no other graph presence carries its
// morphology on its own node; the derive sources the lex pole from there.
func TestDeclLexOrphanLivesOnLexemeNode(t *testing.T) {
	d, s := testDaemon(t)
	be, err := s.WordByTemplate("lexeme:en:be")
	if err != nil || be == nil {
		t.Fatalf("the copula should have a lexeme node: %v", err)
	}
	if be.Kind != memory.KindWord {
		t.Fatalf("the lexeme node should be a word node, got kind %q", be.Kind)
	}
	lx := d.declLexFromGraph(d.storeView(), be.ID)
	if lx.cat != "cop" {
		t.Fatalf("be's realizer category should be cop, got %q", lx.cat)
	}
	for k, want := range map[string]string{"sg": "is", "pl": "are", "past:sg": "was", "past:pl": "were"} {
		if lx.forms[k] != want {
			t.Fatalf("be's %s form should be %q, got %q", k, want, lx.forms[k])
		}
	}

	// The derive sources the lex:en:be parameter from the lexeme node.
	gnodes, err := s.NodesByKind(memory.KindGrammar)
	if err != nil {
		t.Fatal(err)
	}
	gparams := paramsOf(gnodes)
	idx := -1
	for i, n := range gnodes {
		if n.Template == "lex:en:be" {
			gparams[i].Value = "cat=cop;sg=is" // missing the rest, which the lexeme node carries
			idx = i
		}
	}
	if idx < 0 {
		t.Fatal("the be lex node should exist")
	}
	d.deriveDeclLex(gnodes, gparams)
	if !strings.Contains(gparams[idx].Value, "past:pl=were") {
		t.Fatalf("the lexeme node should drive be's lex pole, got %q", gparams[idx].Value)
	}
}

// A phrase rule taught ONLY into the graph (no seed entry, no recompile from code) reaches the
// realization engine and drives generation, and it survives the night, so a grammar rule learned into
// the graph is a durable fluid weight. The taught rule orders its daughters b then a; the engine renders
// that order though no seed line describes it.
func TestTaughtPhraseRuleGeneratesAndSurvivesSleep(t *testing.T) {
	d, s := testDaemon(t)
	if _, err := d.buildDeclSynConcept("en", "demo", declSyn{order: []string{"b", "a"}, head: "a"}); err != nil {
		t.Fatal(err)
	}
	if err := d.registerGraphDeclGrammar(); err != nil {
		t.Fatal(err)
	}
	if err := d.recompileGrammar(); err != nil {
		t.Fatal(err)
	}

	tree := func() *grammar.GenNode {
		return grammar.Phrase("demo", nil, map[string]*grammar.GenNode{
			"a": grammar.Leaf("dog", map[string]string{"cat": "n", "num": "sg"}),
			"b": grammar.Leaf("cat", map[string]string{"cat": "n", "num": "sg"}),
		})
	}
	surface, _, ok := d.gram.Generate("en", tree())
	if !ok || surface != "cat dog" {
		t.Fatalf("the taught rule should order b before a: %q (ok %v)", surface, ok)
	}

	// It survives the night and still drives generation: a rule taught into the graph is durable.
	if _, err := s.Sleep(d.cfg.NightlyDecay, d.cfg.PruneFloor); err != nil {
		t.Fatal(err)
	}
	if err := d.recompileGrammar(); err != nil {
		t.Fatal(err)
	}
	surface, _, ok = d.gram.Generate("en", tree())
	if !ok || surface != "cat dog" {
		t.Fatalf("the taught rule should survive the night and still generate: %q (ok %v)", surface, ok)
	}
}

// Grammar scaffolding lives in the graph as a fluid weight: a night's prune spares the dissolution
// concepts and their structural edges, so a phrase rule, an argument construction, a lexical form, and
// a construction taught with no seed entry all survive a sleep intact rather than fading and re-growing
// only from the seed.
func TestDissolutionScaffoldingSurvivesSleep(t *testing.T) {
	d, s := testDaemon(t)
	// Teach a construction with no seed entry, so its survival can only come from the graph.
	taught := grammar.ArgConstruction{Name: "benefactive", Lang: "en", Pred: "rel", Slots: []grammar.ArgSlot{
		{Role: "agent", Syn: "np"}, {Role: "rel", Syn: "v"}, {Role: "patient", Syn: "np"}, {Role: "beneficiary", Syn: "np"},
	}}
	if _, err := d.buildArgConstructionConcept(taught, taughtConstructionWeight); err != nil {
		t.Fatal(err)
	}

	present := func(tpl string) *memory.Fragment {
		c, _ := s.ConceptByTemplate(tpl)
		return c
	}
	scaffolding := []string{"syn:en:s", "acxn:en:transitive", "acxn:en:benefactive", "lexform:en:do/past", "lexform:en:be/sg"}
	for _, tpl := range scaffolding {
		if present(tpl) == nil {
			t.Fatalf("%s should exist before the sleep", tpl)
		}
	}
	if be, _ := s.WordByTemplate("lexeme:en:be"); be == nil {
		t.Fatal("the copula's lexeme node should exist before the sleep")
	}

	if _, err := s.Sleep(d.cfg.NightlyDecay, d.cfg.PruneFloor); err != nil {
		t.Fatal(err)
	}

	for _, tpl := range scaffolding {
		if present(tpl) == nil {
			t.Fatalf("%s should survive the night as a fluid weight, got pruned", tpl)
		}
	}
	if be, _ := s.WordByTemplate("lexeme:en:be"); be == nil {
		t.Fatal("the copula's lexeme node should survive the night")
	}
	// The taught construction keeps all four slots: its structural edges did not decay away.
	bc := present("acxn:en:benefactive")
	if got := d.argConstructionFromGraph(d.storeView(), bc.ID, "en", "benefactive"); len(got.Slots) != 4 {
		t.Fatalf("the taught construction should keep its 4 slots through the night, got %d", len(got.Slots))
	}
	// A scaffolding edge is structural, not associative, so its weight does not decay overnight.
	edges, err := s.LinksFromRel([]uint64{present("acxn:en:transitive").ID}, cxnSlotRel)
	if err != nil || len(edges) == 0 {
		t.Fatalf("the transitive construction should keep its slot edges: %v", err)
	}
	if edges[0].Weight != 1.0 {
		t.Fatalf("a scaffolding edge should not decay overnight, got weight %v", edges[0].Weight)
	}
}
