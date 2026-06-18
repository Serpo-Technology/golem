package daemon

import (
	"fmt"
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
	if body, ok := d.code.InduceBody(inducedParam, ins, outs, d.evalCode); ok {
		return code.Op{Name: name, Word: name, Params: []string{inducedParam}, Body: &body}, true
	}
	// Beyond a fixed number of steps, search for a simple recursive rule: factorial
	// and the running total fit no fixed-step rule, but a base case and a self-call
	// over the predecessor carry them. The recursion is grounded by running.
	if op, ok := d.code.InduceRecursive(name, inducedParam, ins, outs, d.evalCode); ok {
		return op, true
	}
	return code.Op{}, false
}

// inducedParam is the parameter name an induced function abstracts over, the
// same noun a told definition introduces with "a number".
const inducedParam = "number"

// encodeParams writes an operation's parameters for storage: a list parameter
// keeps its type ("numbers:list"), a number parameter just its name, so the
// types reload with the function.
func encodeParams(op code.Op) string {
	parts := make([]string, len(op.Params))
	for i, p := range op.Params {
		if i < len(op.ParamTypes) && op.ParamTypes[i] == "list" {
			parts[i] = p + ":list"
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
	for _, n := range nodes {
		if n.Template == key {
			if err := d.store.UpdateContent(n.ID, value, key); err != nil {
				return "", err
			}
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
		d.event(events.Event{Ev: "grammar", ID: id, Tpl: key, Weight: taughtConstructionWeight, Outcome: "defined"})
	}

	if err := d.recompileGrammar(); err != nil {
		return "", err
	}
	if src, ok := d.code.ToFunc(op.Name); ok {
		return "I learned " + op.Name + ". I write it as " + src, nil
	}
	return "I learned " + op.Name + ".", nil
}
