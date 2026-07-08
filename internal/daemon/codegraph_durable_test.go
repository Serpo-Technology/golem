package daemon

import (
	"reflect"
	"strings"
	"testing"
	"time"

	"golem/internal/memory"
	"golem/wire"
)

// codeConstruction returns the code:<name> construction node and the root of its durable
// code subgraph, failing if either is absent (a learned function must both persist as a
// construction and root a code subgraph once it materializes).
func codeConstruction(t *testing.T, d *Daemon, name string) (*memory.Fragment, uint64) {
	t.Helper()
	nodes, err := d.store.NodesByKind(memory.KindConstruction)
	if err != nil {
		t.Fatal(err)
	}
	for _, n := range nodes {
		if n.Template == "code:"+name {
			return n, firstDest(d.storeView(), n.ID, codeRootRel)
		}
	}
	t.Fatalf("no code:%s construction", name)
	return nil, 0
}

// A learned function's meaning is durable in the graph and reconstructs and runs by walking
// the store, across the shapes the materializer covers: a plain expression, a two-parameter
// expression, a conditional, a self-recursion, a map, a value-fold, and a composition that
// calls other functions. After a recompile (a reload in miniature) every one still computes,
// its body sourced from the graph.
func TestLearnedFunctionReconstructsFromGraph(t *testing.T) {
	d, _ := testDaemon(t)
	d.cfg.RuntimeEnabled = true
	teachReductions(t, d, "total", "length")
	for _, u := range []string{
		"the double of a number is the product of the number and 2",
		"the gap of a high and a low is the difference of the high and the low",
		"the abs of a number is the number if the number is above 0 else the difference of 0 and the number",
		"the factorial of a number is 1 if the number is below 1 else the product of the number and the factorial of the difference of the number and 1",
		"the bumped of a list of numbers is the sum of each number and 10",
		"the average of a list of numbers is the quotient of the total of the numbers and the length of the numbers",
	} {
		say(t, d, u)
	}

	// Every learned function roots a durable code subgraph the walk reconstructs.
	for _, name := range []string{"double", "gap", "abs", "factorial", "bumped", "average", "total"} {
		if _, root := codeConstruction(t, d, name); root == 0 {
			t.Fatalf("%s should root a durable code subgraph", name)
		}
	}

	// Recompile from the store, the reload path, then run each: the body reconstructs from
	// the graph and computes.
	if err := d.recompileGrammar(); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct{ in, want string }{
		{"the double of 5", "10"},
		{"the gap of 9 and 4", "5"},
		{"the abs of the difference of 3 and 8", "5"},
		{"the factorial of 5", "120"},
		{"the total of the bumped of the list of 1 and 2 and 3", "36"}, // (1+10)+(2+10)+(3+10)
		{"the average of the list of 2 and 4 and 6", "4"},
	} {
		resp, _, _ := d.handle(wire.Request{Cmd: "runcode", Content: tc.in}, nil)
		if !resp.OK || !strings.Contains(resp.Say, tc.want) {
			t.Fatalf("runcode %q should yield %q, got %q (ok %v)", tc.in, tc.want, resp.Say, resp.OK)
		}
	}
}

// The graph is authoritative for execution: with the stored text body overwritten to a
// different expression, the function still runs from the durable subgraph, proving the body
// is walked out of the graph rather than read from the text. double is taught as number * 2,
// its stored text rewritten to number * 3, and it still computes number * 2 after a reload.
func TestGraphBodyOverridesStoredText(t *testing.T) {
	d, _ := testDaemon(t)
	d.cfg.RuntimeEnabled = true
	say(t, d, "the double of a number is the product of the number and 2")

	n, root := codeConstruction(t, d, "double")
	if root == 0 {
		t.Fatal("double should root a code subgraph")
	}
	// Overwrite the stored body to a different, well-formed expression. If reconstruction
	// were off, codeOps would read this and double would compute number * 3.
	if err := d.store.UpdateContent(n.ID, "func:number number * 3", "code:double"); err != nil {
		t.Fatal(err)
	}
	if err := d.recompileGrammar(); err != nil {
		t.Fatal(err)
	}
	resp, _, _ := d.handle(wire.Request{Cmd: "runcode", Content: "the double of 5"}, nil)
	if !resp.OK || !strings.Contains(resp.Say, "10") || strings.Contains(resp.Say, "15") {
		t.Fatalf("double of 5 should be 10 from the graph, not 15 from the corrupted text: %q", resp.Say)
	}
}

// dropCodeSubgraph clears a construction's coderoot and deletes the code nodes its old
// subgraph owns: the mechanism a re-teach relies on so a stale meaning cannot linger as
// authoritative and dead nodes cannot accumulate. It runs before a re-teach materializes
// the new shape, so even a shape that gets no new root leaves the construction rooted at
// nothing (its updated text body then serves) rather than at the old subgraph.
func TestDropCodeSubgraphClearsRootAndNodes(t *testing.T) {
	d, s := testDaemon(t)
	d.cfg.RuntimeEnabled = true
	say(t, d, "the double of a number is the product of the number and 2")
	n, root := codeConstruction(t, d, "double")
	if root == 0 {
		t.Fatal("double should root a code subgraph")
	}
	before, err := s.NodesByKind(memory.KindCodeNode)
	if err != nil || len(before) == 0 {
		t.Fatalf("double's subgraph should hold code nodes (err %v, count %d)", err, len(before))
	}
	d.dropCodeSubgraph(n.ID)
	if _, r := codeConstruction(t, d, "double"); r != 0 {
		t.Fatalf("dropCodeSubgraph should clear the coderoot, got %d", r)
	}
	after, err := s.NodesByKind(memory.KindCodeNode)
	if err != nil {
		t.Fatal(err)
	}
	if len(after) != 0 {
		t.Fatalf("dropCodeSubgraph should delete the old subgraph's code nodes, %d remain", len(after))
	}
}

// A function whose body calls another function that is named "fold" or "listloop" is not
// misread as a synthesized loop: the reconstruction tells a loop from a call by the loop
// edges, not by the grounded name. A function literally named fold is taught, then a caller
// of it; the caller must reconstruct as a call (rooting a code subgraph) and run.
func TestReconstructDistinguishesFoldNameFromLoop(t *testing.T) {
	d, _ := testDaemon(t)
	d.cfg.RuntimeEnabled = true
	teachReductions(t, d, "total", "largest")
	say(t, d, "the fold of a list of numbers is the largest of the numbers")
	say(t, d, "the ff of a list of numbers is the fold of the numbers")
	if _, root := codeConstruction(t, d, "ff"); root == 0 {
		t.Fatal("a caller of a function named fold must reconstruct as a call and root a subgraph")
	}
	if err := d.recompileGrammar(); err != nil {
		t.Fatal(err)
	}
	resp, _, _ := d.handle(wire.Request{Cmd: "runcode", Content: "the ff of the list of 1 and 2 and 3"}, nil)
	if !resp.OK || !strings.Contains(resp.Say, "3") {
		t.Fatalf("ff should compute the largest (3) through the fold-named function: %q", resp.Say)
	}
}

// A faithful re-teach replaces the durable subgraph rather than accreting a second one: the
// old code nodes are deleted, so re-teaching does not leak dead subgraphs into the store.
func TestReteachReplacesSubgraphWithoutLeak(t *testing.T) {
	d, s := testDaemon(t)
	d.cfg.RuntimeEnabled = true
	say(t, d, "the double of a number is the product of the number and 2")
	before, err := s.NodesByKind(memory.KindCodeNode)
	if err != nil {
		t.Fatal(err)
	}
	say(t, d, "the double of a number is the product of the number and 3")
	after, err := s.NodesByKind(memory.KindCodeNode)
	if err != nil {
		t.Fatal(err)
	}
	if len(after) > len(before) {
		t.Fatalf("re-teaching double should not leak dead code nodes: %d before, %d after", len(before), len(after))
	}
	if err := d.recompileGrammar(); err != nil {
		t.Fatal(err)
	}
	resp, _, _ := d.handle(wire.Request{Cmd: "runcode", Content: "the double of 5"}, nil)
	if !resp.OK || !strings.Contains(resp.Say, "15") {
		t.Fatalf("the re-taught double should compute number*3 (15), got %q", resp.Say)
	}
}

// The durable code subgraph is spared the nightly consolidation: its edges keep full weight
// across naps rather than decaying toward the link floor where the orphan drop would erase
// the function, so the reconstruction still yields the same body after many Sleep passes. A
// control edge at the same starting weight decays, proving Sleep is doing its work.
func TestCodeSubgraphSparedNightlyPrune(t *testing.T) {
	d, s := testDaemon(t)
	d.cfg.RuntimeEnabled = true
	say(t, d, "the double of a number is the product of the number and 2")
	n, root := codeConstruction(t, d, "double")
	if root == 0 {
		t.Fatal("double should root a code subgraph")
	}

	want, wok := d.graphToComputation(d.storeView(), firstDest(d.storeView(), root, bodyRel))
	if !wok {
		t.Fatal("double should reconstruct before the prune")
	}

	// A control edge between two ordinary concepts at the same weight the code edges carry:
	// it must decay away, so the survival of the code edges is sparing, not inertness.
	a := encodeAt(t, s, "concept", "alpha", 0, time.Now())
	b := encodeAt(t, s, "concept", "beta", 0, time.Now())
	if err := s.SetLinkRel(a.ID, b.ID, memory.RelTemporal, deliberateName); err != nil {
		t.Fatal(err)
	}

	// Many nights: enough that an unspared edge at deliberateName would fall below the link
	// floor and be dropped (0.9 * 0.98^n <= 0.05 around n = 143).
	for i := 0; i < 200; i++ {
		if _, err := s.Sleep(0.1, 0.05); err != nil {
			t.Fatal(err)
		}
	}

	if w := edgeWeightRel(t, s, n.ID, root, codeRootRel); w != deliberateName {
		t.Fatalf("the coderoot edge should keep full weight across naps, got %v", w)
	}
	if w := edgeWeightRel(t, s, a.ID, b.ID, memory.RelTemporal); w != 0 {
		t.Fatalf("the control edge should have decayed away, weight %v", w)
	}
	got, gok := d.graphToComputation(d.storeView(), firstDest(d.storeView(), root, bodyRel))
	if !gok {
		t.Fatal("the code subgraph must still reconstruct after the nightly prune")
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("the reconstructed body must be unchanged across naps")
	}
	// On reload after the naps, the body is walked out of the spared subgraph and the
	// function runs again: the durable code subgraph is what carries it across the nights.
	if err := d.recompileGrammar(); err != nil {
		t.Fatal(err)
	}
	d.ensureOperations()
	resp, _, _ := d.handle(wire.Request{Cmd: "runcode", Content: "the double of 6"}, nil)
	if !resp.OK || !strings.Contains(resp.Say, "12") {
		t.Fatalf("double of 6 should still be 12 after the prune: %q", resp.Say)
	}
}
