package main

import (
	"testing"

	"github.com/fsnotify/fsnotify"
)

// File events must collapse into a vocabulary coarse enough to repeat:
// operation, extension, parent directory.
func TestFileEventContent(t *testing.T) {
	cases := []struct {
		op   fsnotify.Op
		name string
		want string
	}{
		{fsnotify.Write, "/Users/x/projects/golem/main.go", "write .go in golem"},
		{fsnotify.Create, "/Users/x/projects/golem/notes.md", "create .md in golem"},
		{fsnotify.Remove, "/tmp/cache/blob", "remove (no extension) in cache"},
		{fsnotify.Rename, "/Users/x/projects/golem/old.go", "rename .go in golem"},
		{fsnotify.Chmod, "/Users/x/projects/golem/main.go", ""},
	}
	for _, c := range cases {
		if got := fileEventContent(c.op, c.name); got != c.want {
			t.Errorf("fileEventContent(%v, %q) = %q, want %q", c.op, c.name, got, c.want)
		}
	}
}

// Vitals must leave the sensor as words, never as raw numbers, because
// numbers never repeat and would defeat prediction.
func TestVitalsBands(t *testing.T) {
	if got := loadBand(1.0, 8); got != "cpu load normal" {
		t.Errorf("loadBand low = %q", got)
	}
	if got := loadBand(8.0, 8); got != "cpu load elevated" {
		t.Errorf("loadBand mid = %q", got)
	}
	if got := loadBand(16.0, 8); got != "cpu load high" {
		t.Errorf("loadBand high = %q", got)
	}
	if got := pressureBand(1); got != "memory pressure normal" {
		t.Errorf("pressureBand(1) = %q", got)
	}
	if got := pressureBand(4); got != "memory pressure critical" {
		t.Errorf("pressureBand(4) = %q", got)
	}
	if got := pressureBand(0); got != "" {
		t.Errorf("unknown pressure level should be skipped, got %q", got)
	}
	if got := diskBand(35); got != "disk space normal" {
		t.Errorf("diskBand(35) = %q", got)
	}
	if got := diskBand(5); got != "disk space critical" {
		t.Errorf("diskBand(5) = %q", got)
	}
}
