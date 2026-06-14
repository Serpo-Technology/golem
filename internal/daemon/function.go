package daemon

import (
	"fmt"
	"time"

	"golem/internal/code"
	"golem/internal/events"
	"golem/internal/memory"
)

// learnFunction keeps a function GOLEM has just comprehended from a definition.
// It stores the operation as a "code:<name>" construction the way the seeded
// operations are stored, updating one already there or minting a new one, then
// recompiles so the function can be called and written at once. Because the
// construction lives in memory alongside the seeds, the function returns after a
// restart with the rest of the grammar. The line it returns shows the Go it can
// now write.
func (d *Daemon) learnFunction(op code.Op) (string, error) {
	param, body, ok := d.code.DefSource(op)
	if !ok {
		return "", fmt.Errorf("the function's body could not be written")
	}
	key := "code:" + op.Name
	value := "func:" + param + " " + body

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
