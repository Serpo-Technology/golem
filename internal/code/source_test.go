package code

import (
	"strings"
	"testing"
)

const sampleSource = `package sample

import "fmt"

// threshold is the cutoff the check compares against.
const threshold = 3

// Point is a spot on the grid.
type Point struct{ X, Y int }

// over reports whether n clears the threshold.
func over(n int) bool { return n > threshold }

func main() { fmt.Println(over(5)) }
`

func declMap(t *testing.T, src string) map[string]Decl {
	t.Helper()
	decls, err := Declarations(src)
	if err != nil {
		t.Fatalf("Declarations: %v", err)
	}
	m := map[string]Decl{}
	for _, d := range decls {
		m[d.Name] = d
	}
	return m
}

// Reading a file yields its declarations: the kind, the name, the doc, and the
// shape of each, so GOLEM can reason about source from what is there.
func TestDeclarationsReadsSource(t *testing.T) {
	m := declMap(t, sampleSource)

	c, ok := m["threshold"]
	if !ok || c.Kind != "const" || c.Detail != "3" {
		t.Fatalf("the constant should read as const threshold = 3: %+v", c)
	}
	if !strings.Contains(c.Doc, "cutoff") {
		t.Fatalf("the constant's doc should be read: %q", c.Doc)
	}
	if p, ok := m["Point"]; !ok || p.Kind != "type" {
		t.Fatalf("the type should read as a type: %+v", p)
	}
	if f, ok := m["over"]; !ok || f.Kind != "func" {
		t.Fatalf("the function should read as a func: %+v", f)
	}
	if i, ok := m["fmt"]; !ok || i.Kind != "import" {
		t.Fatalf("the import should be read: %+v", i)
	}
}

// The current value of a constant reads back out of the source, so a change is
// reasoned from what the constant is now.
func TestConstValueReads(t *testing.T) {
	v, err := ConstValue(sampleSource, "threshold")
	if err != nil || v != "3" {
		t.Fatalf("threshold should read as 3: %q err %v", v, err)
	}
}

// Rewriting a constant's value splices only that value: the new source carries
// the new value, still parses, and every other declaration and comment stands.
func TestSetConstRewritesValue(t *testing.T) {
	out, err := SetConst(sampleSource, "threshold", "5")
	if err != nil {
		t.Fatalf("SetConst: %v", err)
	}
	if v, _ := ConstValue(out, "threshold"); v != "5" {
		t.Fatalf("the rewritten constant should be 5: %q", v)
	}
	if !strings.Contains(out, "// threshold is the cutoff") {
		t.Fatal("the doc comment must be preserved")
	}
	if !strings.Contains(out, "type Point") || !strings.Contains(out, "func over") {
		t.Fatal("the other declarations must be preserved")
	}
}

// A change to a constant that is not there is declined rather than guessed.
func TestSetConstUnknownFails(t *testing.T) {
	if _, err := SetConst(sampleSource, "missing", "1"); err == nil {
		t.Fatal("rewriting an absent constant should fail")
	}
}

// A new value that would not parse is refused at the rewrite, before the
// pipeline ever builds it.
func TestSetConstRejectsUnparsable(t *testing.T) {
	if _, err := SetConst(sampleSource, "threshold", "3 +"); err == nil {
		t.Fatal("an unparsable value should be refused")
	}
}

// A declaration says what it is in literal language, the reading said back.
func TestDescribe(t *testing.T) {
	d := Decl{Kind: "const", Name: "threshold", Detail: "3"}
	if got := d.Describe(); got != "the constant threshold is 3" {
		t.Fatalf("the constant should describe itself: %q", got)
	}
}
