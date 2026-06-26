package daemon

import (
	"bytes"
	"strings"
	"testing"

	"golem/internal/grammar"
	"golem/internal/memory"
)

// The argument-structure constructions live as concepts in the graph, each carrying its form pole
// as a slot subgraph, and the graph representation round-trips the compiled construction (the shadow
// stays silent). The construction concepts are system scaffolding, kept out of the grown layer.
func TestArgConstructionsLiveInGraph(t *testing.T) {
	d, s := testDaemon(t) // boot ran ensureArgConstructions + the shadow
	var buf bytes.Buffer
	d.out = &buf
	if err := d.recompileGrammar(); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(buf.String(), "argcxn mismatch") {
		t.Fatalf("the graph-derived constructions should match the seed:\n%s", buf.String())
	}

	c, err := s.ConceptByTemplate("acxn:en:transitive")
	if err != nil || c == nil {
		t.Fatalf("the transitive construction should be a concept in the graph: %v", err)
	}
	if !d.isSystemConcept(c.ID) {
		t.Fatal("a construction concept should be excluded as system scaffolding")
	}

	// It carries its three ordered argument slots and marks the predicate role.
	g := d.storeView()
	if slots := g.dests(c.ID, cxnSlotRel); len(slots) != 3 {
		t.Fatalf("the transitive construction should carry 3 slots, got %d", len(slots))
	}
	if preds := g.dests(c.ID, cxnPredRel); len(preds) != 1 || g.content(preds[0]) != "rel" {
		t.Fatalf("the predicate role should be rel: %v", preds)
	}
	// A slot resolves to its role through the shared vocabulary.
	roles := map[string]bool{}
	for _, sid := range g.dests(c.ID, cxnSlotRel) {
		if r := g.dests(sid, cxnRoleRel); len(r) == 1 {
			roles[g.content(r[0])] = true
		}
	}
	for _, want := range []string{"agent", "rel", "patient"} {
		if !roles[want] {
			t.Fatalf("the transitive construction should hold the %q role, got %v", want, roles)
		}
	}
}

// The construction's form pole is derived from the graph: where a stored parameter disagrees with the
// concept the graph holds, the graph drives and the tripwire fires. So editing the construction in the
// graph reaches the engine, the cutover that makes the graph the source.
func TestArgConstructionsDeriveFromGraph(t *testing.T) {
	d, _ := testDaemon(t)
	cnodes, err := d.store.NodesByKind(memory.KindConstruction)
	if err != nil {
		t.Fatal(err)
	}
	cparams := paramsOf(cnodes)
	idx := -1
	for i, n := range cnodes {
		if n.Template == "acxn:en:transitive" {
			cparams[i].Value = "agent/np" // a seed disagreeing with the graph's full transitive
			idx = i
		}
	}
	if idx < 0 {
		t.Fatal("the transitive construction node should exist")
	}
	var buf bytes.Buffer
	d.out = &buf
	d.deriveArgConstructions(cnodes, cparams)
	if cparams[idx].Value != "agent/np/animate rel/v patient/np" {
		t.Fatalf("the graph should drive the construction's form pole, got %q", cparams[idx].Value)
	}
	if cparams[idx].Category != "rel" {
		t.Fatalf("the graph should drive the predicate role, got %q", cparams[idx].Category)
	}
	if !strings.Contains(buf.String(), "argcxn mismatch") {
		t.Fatalf("a seed disagreeing with the graph should log the tripwire:\n%s", buf.String())
	}
}

// A new argument-structure construction taught into the graph (no seed edit, no recompile from code)
// reaches the engine and both parses and speaks a clause of its shape: a ditransitive binds four
// roles. The construction is learned the way the seeded ones live, in the graph the snapshot keeps.
func TestTaughtConstructionParsesAndSpeaks(t *testing.T) {
	d, _ := testDaemon(t)
	dog := groundNoun(t, d, "dog")
	cat := groundNoun(t, d, "cat")
	bone := groundNoun(t, d, "bone")

	ditransitive := grammar.ArgConstruction{Name: "ditransitive", Lang: "en", Pred: "rel", Slots: []grammar.ArgSlot{
		{Role: "agent", Syn: "np", Prefer: "animate"},
		{Role: "rel", Syn: "v"},
		{Role: "recipient", Syn: "np"},
		{Role: "patient", Syn: "np"},
	}}
	if _, err := d.buildArgConstructionConcept(ditransitive, taughtConstructionWeight); err != nil {
		t.Fatal(err)
	}
	if err := d.registerGraphConstructions(); err != nil {
		t.Fatal(err)
	}
	if err := d.recompileGrammar(); err != nil {
		t.Fatal(err)
	}

	// The engine now holds the ditransitive, sourced from the graph with no seed entry.
	var got grammar.ArgConstruction
	for _, c := range d.gram.ArgConstructions("en") {
		if c.Name == "ditransitive" {
			got = c
		}
	}
	if len(got.Slots) != 4 || got.Slots[2].Role != "recipient" || got.Slots[3].Role != "patient" {
		t.Fatalf("the taught ditransitive should be sourced from the graph: %+v", got)
	}

	// It comprehends a four-role clause, binding each constituent to its role.
	m, ok := d.gram.Comprehend("en", "the dog gave the cat the bone", d.resolveConcept, nil)
	if !ok {
		t.Fatal("the taught ditransitive should comprehend a four-role clause")
	}
	boundTo := func(role string, want *memory.Fragment) bool {
		c := m.Roles[role].Concept
		return c != nil && c.ID == want.ID
	}
	if !boundTo("agent", dog) || !boundTo("recipient", cat) || !boundTo("patient", bone) {
		t.Fatalf("the roles should bind through the taught construction: agent=%v recipient=%v patient=%v",
			m.Roles["agent"].Concept, m.Roles["recipient"].Concept, m.Roles["patient"].Concept)
	}

	// And it speaks the meaning back through the same construction, rendering all four constituents.
	clause, ok := d.gram.Speak("en", m)
	if !ok {
		t.Fatal("the taught ditransitive should speak")
	}
	lc := strings.ToLower(clause)
	for _, w := range []string{"dog", "cat", "bone"} {
		if !strings.Contains(lc, w) {
			t.Fatalf("the spoken clause should render every role, got %q", clause)
		}
	}
}

// A clause a learned construction reads into more than two roles is stored with every grounded role
// on the moment, each edge labelled by its name. So a ditransitive told through the taught
// construction keeps its recipient alongside the agent and patient, the grounded proposition the
// reasoning reads from carrying all three.
func TestComprehendStoresGroundedRolesBeyondAgentPatient(t *testing.T) {
	d, s := testDaemon(t)
	dog := groundNoun(t, d, "dog")
	cat := groundNoun(t, d, "cat")
	bone := groundNoun(t, d, "bone")
	ditransitive := grammar.ArgConstruction{Name: "ditransitive", Lang: "en", Pred: "rel", Slots: []grammar.ArgSlot{
		{Role: "agent", Syn: "np", Prefer: "animate"},
		{Role: "rel", Syn: "v"},
		{Role: "recipient", Syn: "np"},
		{Role: "patient", Syn: "np"},
	}}
	if _, err := d.buildArgConstructionConcept(ditransitive, taughtConstructionWeight); err != nil {
		t.Fatal(err)
	}
	if err := d.registerGraphConstructions(); err != nil {
		t.Fatal(err)
	}
	if err := d.recompileGrammar(); err != nil {
		t.Fatal(err)
	}

	say(t, d, "the dog traded the cat the bone")

	// The moment the dog is the agent of carries the recipient and the patient too.
	agentEdges, err := s.LinksToRel([]uint64{dog.ID}, memory.RelAgent)
	if err != nil || len(agentEdges) == 0 {
		t.Fatalf("the dog should be the agent of the told proposition: %v", err)
	}
	moment := agentEdges[0].From
	roleTarget := func(role string) uint64 {
		edges, _ := s.LinksFromRel([]uint64{moment}, role)
		if len(edges) == 0 {
			return 0
		}
		return edges[0].To
	}
	if roleTarget("agent") != dog.ID || roleTarget("recipient") != cat.ID || roleTarget("patient") != bone.ID {
		t.Fatalf("the moment should carry all three grounded roles: agent=%d recipient=%d patient=%d (want dog=%d cat=%d bone=%d)",
			roleTarget("agent"), roleTarget("recipient"), roleTarget("patient"), dog.ID, cat.ID, bone.ID)
	}

	// A question giving two of the three roles and asking the third is answered from the
	// proposition they all stand in, spoken back whole through the construction.
	answer := say(t, d, "what did the dog trade the cat").Say
	if !strings.Contains(strings.ToLower(answer), "bone") {
		t.Fatalf("the two-known question should answer with the asked patient, got %q", answer)
	}
}
