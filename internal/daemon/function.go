package daemon

import (
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"time"

	"golem/internal/code"
	"golem/internal/events"
	"golem/internal/memory"
)

// induceFunction works out a function from the input/output pairs it was shown,
// rather than being told its body: the code faculty searches the primitive
// operations for the rule that maps every input to its output, grounding each
// candidate in the runtime. It names the parameter the same way a told
// definition does so the two surfaces converge, and refuses a name that would
// shadow a seeded operation. It returns the inferred operation, false when no
// rule fits or the runtime is closed.
func (d *Daemon) induceFunction(name string, ins, outs []string) (code.Op, bool) {
	if !d.cfg.RuntimeEnabled || d.code == nil {
		return code.Op{}, false
	}
	if name == "" || name == inducedParam || d.code.Primitive(name) {
		return code.Op{}, false
	}
	// The function being defined is left out of its own search, so an already-learned
	// operation is re-synthesized to its rule, not the trivial call to itself it would
	// otherwise find sitting in the inventory.
	d.code.SetSynthSelf(name)
	defer d.code.SetSynthSelf("")
	// The rule is searched for, not fitted to a template: the engine assembles
	// expressions over the parameter and runs them, keeping the smallest whose
	// effect is the examples. The procedure emerges from composing the Go floor and
	// is grounded by running.
	if body, ok := d.code.SynthesizeExpr(inducedParam, ins, outs, d.evalCode); ok {
		return code.Op{Name: name, Word: name, Params: []string{inducedParam}, Body: &body}, true
	}
	// A rule that is no single expression may still be a two-way conditional, a
	// predicate over the input choosing between two expressions (abs, relu); that
	// branch shape is searched and grounded by running.
	if body, ok := d.code.SynthesizeConditional(inducedParam, ins, outs, d.evalCode); ok {
		return code.Op{Name: name, Word: name, Params: []string{inducedParam}, Body: &body}, true
	}
	// A rule that is no single expression, like factorial, is a base case and a
	// self-call over the predecessor; that recursive shape is searched separately
	// and grounded by running.
	if op, ok := d.code.InduceRecursive(name, inducedParam, ins, outs, d.evalCode); ok {
		return op, true
	}
	return code.Op{}, false
}

// inducedParam is the parameter name an induced function abstracts over, the
// same noun a told definition introduces with "a number".
const inducedParam = "number"

// listParam is the parameter name a synthesized list function ranges over, the
// noun a told list definition introduces with "a list of numbers".
const listParam = "numbers"

// synthesizeFold works out a list function from the example lists it was shown,
// the loop assembled and grounded rather than fitted to a template: the code
// faculty searches for a program that folds each example list to its output and
// runs it to decide. It refuses a name that shadows a primitive or built-in, and
// returns the synthesized operation, false when no program fits or the runtime is
// closed.
func (d *Daemon) synthesizeFold(name string, lists [][]string, outs []string) (code.Op, bool) {
	if !d.cfg.RuntimeEnabled || d.code == nil {
		return code.Op{}, false
	}
	if name == "" || name == listParam || d.code.Primitive(name) {
		return code.Op{}, false
	}
	d.code.SetSynthSelf(name)
	defer d.code.SetSynthSelf("")
	ilists, ok := toIntLists(lists)
	iouts, iok := toInts(outs)
	if !ok || !iok {
		return code.Op{}, false
	}
	body, ok := d.code.SynthesizeFold(listParam, ilists, iouts, d.evalCode)
	if !ok {
		return code.Op{}, false
	}
	return code.Op{Name: name, Word: name, Params: []string{listParam}, ParamTypes: []string{"list"}, Ret: "int", Raw: body}, true
}

// synthesizeListReturn works out a list-to-list function from the example lists it
// was shown, the loop assembled and grounded: the faculty searches for a program
// that maps or filters each example input to its output list and runs it to decide.
// It refuses a name that shadows a primitive or built-in.
func (d *Daemon) synthesizeListReturn(name string, ins, outs [][]string) (code.Op, bool) {
	if !d.cfg.RuntimeEnabled || d.code == nil {
		return code.Op{}, false
	}
	if name == "" || name == listParam || d.code.Primitive(name) {
		return code.Op{}, false
	}
	d.code.SetSynthSelf(name)
	defer d.code.SetSynthSelf("")
	ilists, iok := toIntLists(ins)
	olists, ook := toIntLists(outs)
	if !iok || !ook {
		return code.Op{}, false
	}
	body, ok := d.code.SynthesizeListReturn(listParam, ilists, olists, d.evalCode)
	if !ok {
		return code.Op{}, false
	}
	return code.Op{Name: name, Word: name, Params: []string{listParam}, ParamTypes: []string{"list"}, Ret: "list", Raw: body}, true
}

// toIntLists parses a set of number-string lists into integers.
func toIntLists(rows [][]string) ([][]int, bool) {
	out := make([][]int, len(rows))
	for i, row := range rows {
		nums, ok := toInts(row)
		if !ok {
			return nil, false
		}
		out[i] = nums
	}
	return out, true
}

// toInts parses number strings into integers.
func toInts(row []string) ([]int, bool) {
	nums := make([]int, len(row))
	for j, s := range row {
		n, err := strconv.Atoi(s)
		if err != nil {
			return nil, false
		}
		nums[j] = n
	}
	return nums, true
}

// encodeParams writes an operation's parameters for storage: a non-int parameter
// keeps its type ("numbers:list", "stem:string"), a number parameter just its name,
// so the types reload with the function.
func encodeParams(op code.Op) string {
	parts := make([]string, len(op.Params))
	for i, p := range op.Params {
		if i < len(op.ParamTypes) && op.ParamTypes[i] != "" && op.ParamTypes[i] != "int" {
			parts[i] = p + ":" + op.ParamTypes[i]
		} else {
			parts[i] = p
		}
	}
	return strings.Join(parts, ",")
}

// learnFunction keeps a function GOLEM has just comprehended from a definition.
// It stores the operation as a "code:<name>" construction the way the seeded
// operations are stored, updating one already there or minting a new one, then
// recompiles so the function can be called and written at once. Because the
// construction lives in memory alongside the seeds, the function returns after a
// restart with the rest of the grammar. The line it returns shows the Go it can
// now write.
func (d *Daemon) learnFunction(op code.Op) (string, error) {
	_, body, ok := d.code.DefSource(op)
	if !ok {
		return "", fmt.Errorf("the function's body could not be written")
	}
	key := "code:" + op.Name
	// The parameters are comma-joined into a single space-free field so the body,
	// which carries spaces, is cleanly separable on reload; a list parameter
	// carries its type ("numbers:list"), a number parameter just its name.
	value := "func:" + encodeParams(op) + " " + body

	nodes, err := d.store.NodesByKind(memory.KindConstruction)
	if err != nil {
		return "", err
	}
	updated := false
	var constructionID uint64
	for _, n := range nodes {
		if n.Template == key {
			if err := d.store.UpdateContent(n.ID, value, key); err != nil {
				return "", err
			}
			constructionID = n.ID
			updated = true
			break
		}
	}
	if !updated {
		node := &memory.Fragment{
			Kind:     memory.KindConstruction,
			Channel:  "grammar",
			Content:  value,
			Template: key,
			Lang:     "en",
			Category: "code",
			Time:     time.Now(),
		}
		id, err := d.store.Encode(node)
		if err != nil {
			return "", err
		}
		if err := d.store.SetWeight(id, taughtConstructionWeight); err != nil {
			return "", err
		}
		constructionID = id
		d.event(events.Event{Ev: "grammar", ID: id, Tpl: key, Weight: taughtConstructionWeight, Outcome: "defined"})
	}

	// Persist the definition as a durable code subgraph and root it on the construction, so
	// on reload the body is walked out of the graph rather than re-parsed from the stored
	// text. This is behaviour-neutral where it materializes (the graph reconstructs the same
	// meaning) and a no-op where it does not, the text body serving as the fallback either way.
	d.materializeDurable(op, constructionID)

	if err := d.recompileGrammar(); err != nil {
		return "", err
	}
	// The learned operation becomes a grounded concept now, not at the next
	// consolidation, so it resolves and computes on the very turn it is taught.
	d.ensureOperations()
	if src, ok := d.code.ToFunc(op.Name); ok {
		return "I learned " + op.Name + ". I write it as " + src, nil
	}
	return "I learned " + op.Name + ".", nil
}

// materializeDurable writes a learned function's meaning into the graph as a durable code
// subgraph and, once it has verified the graph reconstructs the same meaning, roots it on
// the construction, so on reload the body is walked out of the store rather than re-parsed
// from the stored text. A re-teach redefines the function, so any prior subgraph and its
// root are dropped first: the old meaning must never linger as authoritative once the text
// body has been updated to a new one. The new shape is materialized into a throwaway graph
// and cross-checked there, so nothing durable is written unless the whole subgraph
// reconstructs faithfully; a shape the materializer refuses (mutual recursion, an
// unrecognized raw loop) or one that reconstructs to a different meaning leaves no durable
// node and no root, and the updated text body serves as the fallback (a divergence is
// logged as a tripwire).
func (d *Daemon) materializeDurable(op code.Op, constructionID uint64) {
	if constructionID == 0 {
		return
	}
	d.dropCodeSubgraph(constructionID)
	scratch := newScratchGraph()
	sfn, ok := d.materializeFnNode(scratch, op, map[string]uint64{})
	if !ok {
		return
	}
	comp, raw, rok := d.reconstructedBody(scratch, sfn)
	if !rok {
		return
	}
	faithful := (comp != nil && op.Body != nil && reflect.DeepEqual(*comp, *op.Body)) ||
		(raw != "" && op.Raw != "" && raw == code.RawDefPrefix+op.Ret+" "+op.Raw)
	if !faithful {
		fmt.Fprintf(d.out, "codegraph def mismatch for %s\n", op.Name)
		return
	}
	fn, ok := d.materializeFnNode(storeCodeWriter{d}, op, map[string]uint64{})
	if !ok {
		return
	}
	if err := d.store.SetLinkRel(constructionID, fn, codeRootRel, deliberateName); err != nil {
		fmt.Fprintf(d.out, "codegraph root link failed for %s: %v\n", op.Name, err)
	}
}

// dropCodeSubgraph removes a construction's durable code subgraph and its root edge, so a
// re-teach leaves neither a stale meaning that could override the new text nor dead nodes
// the nightly prune spares forever. It clears the root, then deletes every code node the
// old subgraph owns (gathered from the root by a walk that admits only code nodes, so it
// stays within the subgraph and never reaches a shared concept). A construction with no
// root is untouched.
func (d *Daemon) dropCodeSubgraph(constructionID uint64) {
	root := firstDest(d.storeView(), constructionID, codeRootRel)
	if err := d.store.DeleteLinksFrom(constructionID, codeRootRel); err != nil {
		fmt.Fprintf(d.out, "codegraph root clear failed: %v\n", err)
	}
	if root == 0 {
		return
	}
	victims := map[uint64]bool{}
	d.collectCodeNodes(root, victims)
	for id := range victims {
		if _, err := d.store.DeleteByID(id); err != nil {
			fmt.Fprintf(d.out, "codegraph node delete failed: %v\n", err)
		}
	}
}
