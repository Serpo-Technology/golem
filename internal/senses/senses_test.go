package senses

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

// The vitals readings are parsed per platform; the parsers read either form.
func TestVitalsCrossPlatformParsers(t *testing.T) {
	if f, ok := parseLoadavgFirst("0.52 0.58 0.59 1/890 12345\n"); !ok || f != 0.52 {
		t.Errorf("linux loadavg = %v %v", f, ok)
	}
	if f, ok := parseLoadavgFirst("{ 1.84 1.74 1.71 }\n"); !ok || f != 1.84 {
		t.Errorf("darwin loadavg = %v %v", f, ok)
	}
	meminfo := "MemTotal:       16384000 kB\nMemFree:         1000000 kB\nMemAvailable:    8192000 kB\n"
	if pct, ok := memAvailablePctFrom(meminfo); !ok || pct != 50 {
		t.Errorf("memAvailablePctFrom = %v %v, want 50", pct, ok)
	}
	if _, ok := memAvailablePctFrom("MemFree: 100 kB\n"); ok {
		t.Error("missing MemTotal/MemAvailable should not parse")
	}
	if got := memBandLinux(50); got != "memory pressure normal" {
		t.Errorf("memBandLinux(50) = %q", got)
	}
	if got := memBandLinux(5); got != "memory pressure critical" {
		t.Errorf("memBandLinux(5) = %q", got)
	}
}
