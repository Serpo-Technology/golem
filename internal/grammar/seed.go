package grammar

import (
	_ "embed"
	"strings"
	"sync"
)

// seedText is the embedded grammar and construction bootstrap, the first
// curriculum in data. The engine carries no grammar content; this file is
// where the seed lives, and the daemon writes it into starter memory on first
// run, after which the memory copy competes with lived usage.
//
//go:embed seed.txt
var seedText string

// parseSeed reads the bootstrap into parameters: one per line, tab-separated
// key, value, language, and category, with blank lines and # comments
// skipped. Every entry enters at full weight, the ceiling the weighted
// competition in memory bends from.
func parseSeed() []Param {
	lines := strings.Split(seedText, "\n")
	out := make([]Param, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimRight(line, "\r")
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		f := strings.SplitN(line, "\t", 4)
		if len(f) < 3 {
			continue
		}
		cat := ""
		if len(f) == 4 {
			cat = f[3]
		}
		out = append(out, Param{Key: f[0], Value: f[1], Lang: f[2], Category: cat, Weight: 1.0})
	}
	return out
}

var seedOnce = sync.OnceValue(parseSeed)

// isConstruction reports whether a seed key carries a construction rather
// than a grammar parameter: an answer template (cxn), an argument-structure
// construction (acxn), or a code construction (code). All of the construction
// inventory rides the construction nodes, leaving the curriculum the engine
// proper.
func isConstruction(key string) bool {
	return strings.HasPrefix(key, "cxn:") ||
		strings.HasPrefix(key, "acxn:") ||
		strings.HasPrefix(key, "code:")
}

// CodeConstructions returns the programming-language half of the seed, the code
// entries written into starter memory beside the rest. Each key is
// "code:<name>" and the value is "<symbol> <word>", the operator and the word
// that teach the operation, two surfaces of one computation.
func CodeConstructions() []Param {
	var out []Param
	for _, p := range seedOnce() {
		if strings.HasPrefix(p.Key, "code:") {
			out = append(out, p)
		}
	}
	return out
}

// Curriculum returns the grammar half of the seed, every parameter but the
// constructions, the first curriculum the daemon writes into starter memory.
// The closed class, sortals, event verbs, principal parts, word order, suffix
// recipes, and the declarative grammar all ride here; once seeded, the engine
// reads the memory copy.
func Curriculum() []Param {
	var out []Param
	for _, p := range seedOnce() {
		if !isConstruction(p.Key) {
			out = append(out, p)
		}
	}
	return out
}

// Constructions returns the construction half of the seed, both the answer
// templates and the argument-structure constructions, written into starter
// memory beside the grammar parameters. Once seeded, the engine reads the
// memory copy, and a taught or induced construction competes with these by
// weight.
func Constructions() []Param {
	var out []Param
	for _, p := range seedOnce() {
		if isConstruction(p.Key) {
			out = append(out, p)
		}
	}
	return out
}
