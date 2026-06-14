package daemon

import (
	"fmt"
	"time"

	"golem/internal/attention"
)

// Raw edits are the editor's scalpel: a direct splice, reweight, deletion, or
// provenance mark on one node by id. Each is logged to the trail so no
// hand-touch is silent, and each drops the node from the working set so the
// change is seen at once rather than masked by a cached copy. Authoring goes the
// grounded way through teach and observe; this is the surgery beside it, and it
// runs on the brain loop like every other state change.

// editContent splices a fragment's content and the template prediction runs on,
// marks it edited, and refreshes the cache.
func (d *Daemon) editContent(id uint64, content string) error {
	if content == "" {
		return fmt.Errorf("an edit needs new content")
	}
	if err := d.store.UpdateContent(id, content, attention.Normalize(content)); err != nil {
		return err
	}
	d.hot.Remove(id)
	d.auditEdit(id, "edit", content)
	return nil
}

// reweight sets a fragment's synaptic weight and marks it edited.
func (d *Daemon) reweight(id uint64, w float64) error {
	if w <= 0 || w > 1 {
		return fmt.Errorf("a weight is between 0 and 1, not %v", w)
	}
	if err := d.store.SetWeight(id, w); err != nil {
		return err
	}
	if err := d.store.MarkEdited(id); err != nil {
		return err
	}
	d.hot.Remove(id)
	d.auditEdit(id, "reweight", fmt.Sprintf("%.2f", w))
	return nil
}

// deleteNode removes one fragment by id and the edges touching it, reporting
// whether a node was there to go.
func (d *Daemon) deleteNode(id uint64) (int, error) {
	n, err := d.store.DeleteByID(id)
	if err != nil {
		return 0, err
	}
	d.hot.Remove(id)
	if n > 0 {
		d.auditEdit(id, "delete", "removed")
	}
	return n, nil
}

// markNode records that a fragment was hand-touched, without otherwise changing
// it, so the provenance is available even when the value is left alone.
func (d *Daemon) markNode(id uint64) error {
	if err := d.store.MarkEdited(id); err != nil {
		return err
	}
	d.hot.Remove(id)
	d.auditEdit(id, "mark", "edited")
	return nil
}

// auditEdit records one raw edit on the trail, the same trail the daemon's own
// decisions write, so the editor's hand is as readable as GOLEM's thought.
func (d *Daemon) auditEdit(id uint64, op, detail string) {
	if err := d.store.Audit(time.Now().Unix(), "editor", fmt.Sprintf("%s #%d", op, id), detail); err != nil {
		fmt.Fprintf(d.out, "audit write failed: %v\n", err)
	}
}
