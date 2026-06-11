package senses

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// A page reduces to its visible text lines: scripts and styles vanish,
// tags fall away, entities decode, and block boundaries become line
// breaks.
func TestStripHTML(t *testing.T) {
	page := `<html><head><title>Status Page</title>
		<style>body { color: red }</style>
		<script>var hidden = "never seen";</script></head>
		<body><h1>All systems &amp; services</h1>
		<p>Database: <b>healthy</b></p>
		<div>Queue depth &lt;10</div>
		<ul><li>x</li><li>api ok</li></ul>
		</body></html>`
	lines := StripHTML(page)
	want := []string{"Status Page", "All systems & services", "Database: healthy", "Queue depth <10", "api ok"}
	if len(lines) != len(want) {
		t.Fatalf("expected %d lines, got %v", len(want), lines)
	}
	for i, w := range want {
		if lines[i] != w {
			t.Fatalf("line %d: got %q, want %q", i, lines[i], w)
		}
	}
	for _, l := range lines {
		if strings.Contains(l, "hidden") || strings.Contains(l, "color") {
			t.Fatalf("invisible content leaked: %q", l)
		}
	}
}

// The tail sense reads from the end: existing content is old news, and
// each appended line arrives prefixed with the file's name.
func TestTailSensorEmitsAppendedLines(t *testing.T) {
	path := filepath.Join(t.TempDir(), "app.log")
	if err := os.WriteFile(path, []byte("old line\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	out := make(chan Observation, 16)
	stop := make(chan struct{})
	defer close(stop)
	var dropped atomic.Int64
	go tailOne(path, out, stop, &dropped)

	// The first poll records the end position; the append lands after it.
	time.Sleep(tailInterval + 500*time.Millisecond)
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	fmt.Fprintln(f, "fresh line")
	f.Close()

	select {
	case o := <-out:
		if o.Channel != "reading" || o.Content != "app.log: fresh line" {
			t.Fatalf("unexpected observation: %+v", o)
		}
	case <-time.After(3 * tailInterval):
		t.Fatal("the appended line never arrived")
	}
	select {
	case o := <-out:
		t.Fatalf("the existing content must not replay: %+v", o)
	default:
	}
}

// The web sense fetches a page and emits its stripped lines on the web
// channel.
func TestWebSensorEmitsStrippedText(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "<html><body><p>deploy finished</p><p>all green</p></body></html>")
	}))
	defer srv.Close()

	out := make(chan Observation, 16)
	stop := make(chan struct{})
	var dropped atomic.Int64
	go WebSensor([]string{srv.URL}, time.Hour, out, stop, &dropped)
	defer close(stop)

	deadline := time.After(5 * time.Second)
	var got []string
	for len(got) < 2 {
		select {
		case o := <-out:
			if o.Channel != "web" {
				t.Fatalf("web text must arrive on the web channel: %+v", o)
			}
			got = append(got, o.Content)
		case <-deadline:
			t.Fatalf("expected two lines, got %v", got)
		}
	}
	if got[0] != "deploy finished" || got[1] != "all green" {
		t.Fatalf("unexpected lines: %v", got)
	}
}
