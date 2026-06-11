package main

import (
	"fmt"
	"strings"
)

// renderDOT lays the memory graph out for graphviz: one box per fragment
// labeled with its id, channel, and content, sized by weight; one edge
// per link, weighted by strength. Render with, for example:
//
//	golem dump --dot | dot -Tsvg -o memory.svg
func renderDOT(frags []*FragmentInfo, links []*LinkInfo) string {
	var b strings.Builder
	b.WriteString("digraph golem {\n")
	b.WriteString("  rankdir=LR;\n")
	b.WriteString("  node [shape=box, style=rounded, fontname=\"Helvetica\"];\n")
	for _, f := range frags {
		label := fmt.Sprintf("#%d %s\\n%s", f.ID, dotEscape(f.Channel), dotEscape(clip(f.Content, 60)))
		if f.Count > 1 {
			label += fmt.Sprintf("\\nx%d", f.Count)
		}
		b.WriteString(fmt.Sprintf("  f%d [label=\"%s\", penwidth=%.2f];\n",
			f.ID, label, 0.5+2*f.Weight))
	}
	for _, e := range links {
		b.WriteString(fmt.Sprintf("  f%d -> f%d [penwidth=%.2f];\n",
			e.From, e.To, 0.3+2*e.Weight))
	}
	b.WriteString("}\n")
	return b.String()
}

// dotEscape keeps fragment content from breaking out of a DOT label.
func dotEscape(s string) string {
	return strings.NewReplacer(`\`, `\\`, `"`, `\"`, "\n", " ").Replace(s)
}
