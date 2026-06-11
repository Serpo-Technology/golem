package main

import (
	"strings"
	"testing"

	"golem/internal/wire"
)

// Content with quotes and backslashes cannot break out of a DOT label.
func TestDotEscapes(t *testing.T) {
	frags := []*wire.FragmentInfo{{ID: 1, Channel: "shell",
		Content: `run: echo "hi\there"`, Weight: 1.0}}
	dot := renderDOT(frags, nil)
	if strings.Contains(dot, `"hi\there"`) {
		t.Fatalf("label should be escaped:\n%s", dot)
	}
	if !strings.Contains(dot, `\"hi\\there\"`) {
		t.Fatalf("escaped quotes and backslashes should survive:\n%s", dot)
	}
}
