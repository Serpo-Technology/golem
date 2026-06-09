package main

import (
	"fmt"
	"path/filepath"
	"sync/atomic"
	"time"

	"github.com/fsnotify/fsnotify"
)

// FileSensor watches directories (not their subdirectories) and reports
// file events in a coarse vocabulary: the operation, the file's extension,
// and the parent directory's name. Exact paths vary endlessly; "write to a
// .go file in golem" is the level at which prediction is meaningful. A
// short cooldown collapses the write bursts editors produce.
func FileSensor(dirs []string, out chan Observation, stop <-chan struct{}, dropped *atomic.Int64) {
	w, err := fsnotify.NewWatcher()
	if err != nil {
		return
	}
	defer w.Close()
	for _, dir := range dirs {
		w.Add(dir)
	}

	recent := map[string]time.Time{}
	for {
		select {
		case <-stop:
			return
		case ev, ok := <-w.Events:
			if !ok {
				return
			}
			content := fileEventContent(ev.Op, ev.Name)
			if content == "" {
				continue
			}
			if t, seen := recent[content]; seen && time.Since(t) < 2*time.Second {
				continue
			}
			if len(recent) > 256 {
				recent = map[string]time.Time{}
			}
			recent[content] = time.Now()
			emit(out, dropped, Observation{Channel: "files", Content: content, At: time.Now()})
		case <-w.Errors:
		}
	}
}

// fileEventContent buckets one filesystem event into the sensor's
// vocabulary, or returns "" for noise (chmod and friends).
func fileEventContent(op fsnotify.Op, name string) string {
	var verb string
	switch {
	case op.Has(fsnotify.Create):
		verb = "create"
	case op.Has(fsnotify.Write):
		verb = "write"
	case op.Has(fsnotify.Remove):
		verb = "remove"
	case op.Has(fsnotify.Rename):
		verb = "rename"
	default:
		return ""
	}
	ext := filepath.Ext(name)
	if ext == "" {
		ext = "(no extension)"
	}
	return fmt.Sprintf("%s %s in %s", verb, ext, filepath.Base(filepath.Dir(name)))
}
