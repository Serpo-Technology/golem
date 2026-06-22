package daemon

import (
	"reflect"
	"testing"
)

// The grammar reads a function body to the same computation the code engine's own
// parser does, so routing the definition path through the grammar changes which
// parser reads the body, not what it means. parseBodyGrammar is the grammar's
// reading; the same engine with its body parser off is the floor it must match. A
// divergence here is what the production cross-check would catch and serve the
// floor for, so pinning equality keeps the dissolution honest.
func TestComprehendDefinitionMatchesEngine(t *testing.T) {
	d, _ := testDaemon(t)
	cases := []struct {
		def, body string
		params    []string
		self      string
		arity     int
	}{
		{"the larger of a number and a count is the number if the number is above the count else the count",
			"the number if the number is above the count else the count", []string{"number", "count"}, "larger", 2},
		{"the factorial of a number is 1 if the number is below 1 else the product of the number and the factorial of the difference of the number and 1",
			"1 if the number is below 1 else the product of the number and the factorial of the difference of the number and 1", []string{"number"}, "factorial", 1},
		{"the double of a number is the product of the number and 2",
			"the product of the number and 2", []string{"number"}, "double", 1},
		{"the blend of a number and a factor is the product of the number and the factor",
			"the product of the number and the factor", []string{"number", "factor"}, "blend", 2},
	}
	for _, c := range cases {
		grammarBody, gok := d.parseBodyGrammar(c.body, c.params, c.self, c.arity)
		if !gok {
			t.Errorf("the grammar should parse the body of %q", c.def)
			continue
		}
		d.code.SetBodyParser(nil)
		op, cok := d.code.ComprehendDefinition(c.def)
		d.code.SetBodyParser(d.parseBodyGrammar)
		if !cok || op.Body == nil {
			t.Errorf("the engine should parse %q", c.def)
			continue
		}
		if !reflect.DeepEqual(grammarBody, *op.Body) {
			t.Errorf("body parse diverged for %q:\n grammar %+v\n engine  %+v", c.def, grammarBody, *op.Body)
		}
	}
}

// A comparison is not a value computation, so seeding the comparisons as concepts
// must not let the code engine read "6 > 7" as one: their operator symbols stay out
// of the engine's symbol index, reachable only through the comparison resolver.
func TestComprehendRejectsComparison(t *testing.T) {
	d, _ := testDaemon(t)
	if comp, ok := d.code.Comprehend("6 > 7"); ok {
		t.Errorf("a comparison should not comprehend as a value computation: %+v", comp)
	}
}

// A comparison relator resolves only in a conditional's test, never as a call head,
// so "the above of 6 and 7" is no call; the comparison resolver, by contrast, names
// its operator symbol.
func TestComparisonRelatorIsNotACallHead(t *testing.T) {
	d, _ := testDaemon(t)
	if _, _, ok := d.opHead("above"); ok {
		t.Error("opHead should not resolve a comparison relator as a call")
	}
	if _, ok := d.gram.ParseApplication("en", "the above of 6 and 7", d.opHead); ok {
		t.Error("a comparison relator should not open a call")
	}
	if sym, ok := d.cmpRelator("above"); !ok || sym != ">" {
		t.Errorf("cmpRelator(above) = %q ok=%v, want >", sym, ok)
	}
	if sym, ok := d.cmpRelator(""); !ok || sym != "==" {
		t.Errorf("cmpRelator default = %q ok=%v, want ==", sym, ok)
	}
}
