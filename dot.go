package main

import (
	"fmt"
	"strings"
)

// renderDOT lays the memory graph out for graphviz: one node per fragment
// labeled with its id, channel, and content, sized by weight; one edge
// per link, weighted by strength. Node shape marks the kind (moments are
// boxes, words ellipses, concepts double octagons) and edge style marks
// the relation (successor bold, names dashed, grounds dotted). Render
// with, for example:
//
//	golem dump --dot | dot -Tsvg -o memory.svg
func renderDOT(frags []*FragmentInfo, links []*LinkInfo) string {
	var b strings.Builder
	b.WriteString("digraph golem {\n")
	b.WriteString("  rankdir=LR;\n")
	b.WriteString("  node [shape=box, style=rounded, fontname=\"Helvetica\"];\n")
	for _, f := range frags {
		head := dotEscape(f.Channel)
		shape := ""
		switch f.Kind {
		case KindWord:
			shape = ", shape=ellipse"
			if f.Lang != "" {
				head = "word " + dotEscape(f.Lang)
			}
		case KindConcept:
			shape = ", shape=doubleoctagon"
			if f.Category != "" {
				head = "concept " + dotEscape(f.Category)
			}
		}
		label := fmt.Sprintf("#%d %s\\n%s", f.ID, head, dotEscape(clip(f.Content, 60)))
		if f.Count > 1 {
			label += fmt.Sprintf("\\nx%d", f.Count)
		}
		b.WriteString(fmt.Sprintf("  f%d [label=\"%s\", penwidth=%.2f%s];\n",
			f.ID, label, 0.5+2*f.Weight, shape))
	}
	for _, e := range links {
		style := ""
		switch e.Rel {
		case RelSuccessor:
			style = ", style=bold"
		case RelNames:
			style = ", style=dashed"
		case RelGrounds:
			style = ", style=dotted"
		}
		b.WriteString(fmt.Sprintf("  f%d -> f%d [penwidth=%.2f%s];\n",
			e.From, e.To, 0.3+2*e.Weight, style))
	}
	b.WriteString("}\n")
	return b.String()
}

// dotEscape keeps fragment content from breaking out of a DOT label.
func dotEscape(s string) string {
	return strings.NewReplacer(`\`, `\\`, `"`, `\"`, "\n", " ").Replace(s)
}
