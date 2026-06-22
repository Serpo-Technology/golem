package grammar

import (
	"reflect"
	"testing"
)

// stubHead stands in for the grounded operation lookup the daemon supplies: a
// unary "double" and a binary "sum", nothing else, so the parser's arity-driven
// reading and its rejection of an unknown head are exercised without the concept
// graph.
func stubHead(word string) (string, int, bool) {
	switch word {
	case "double":
		return "double", 1, true
	case "sum":
		return "add", 2, true
	case "total":
		return "total", 1, true // a unary list reducer, for list-argument cases
	}
	return "", 0, false
}

func TestParseApplicationFlat(t *testing.T) {
	app, ok := Builtin().ParseApplication("en", "the double of 4", stubHead)
	if !ok {
		t.Fatal("a unary call should parse")
	}
	want := Application{Head: "double", Args: []Application{{Lit: "4"}}}
	if !reflect.DeepEqual(app, want) {
		t.Fatalf("ParseApplication = %+v, want %+v", app, want)
	}
}

func TestParseApplicationNAry(t *testing.T) {
	app, ok := Builtin().ParseApplication("en", "the sum of 6 and 7", stubHead)
	if !ok {
		t.Fatal("a binary call should parse")
	}
	want := Application{Head: "add", Args: []Application{{Lit: "6"}, {Lit: "7"}}}
	if !reflect.DeepEqual(app, want) {
		t.Fatalf("ParseApplication = %+v, want %+v", app, want)
	}
}

// The arity-driven nesting: the unary double reads one operand, so the coordinator
// after it joins the binary sum's second operand, not a third of double's.
func TestParseApplicationNested(t *testing.T) {
	app, ok := Builtin().ParseApplication("en", "the sum of the double of 4 and 5", stubHead)
	if !ok {
		t.Fatal("a nested call should parse")
	}
	want := Application{Head: "add", Args: []Application{
		{Head: "double", Args: []Application{{Lit: "4"}}},
		{Lit: "5"},
	}}
	if !reflect.DeepEqual(app, want) {
		t.Fatalf("ParseApplication = %+v, want %+v", app, want)
	}
}

// All-or-nothing: a binary head handed a third operand leaves a tail the parse
// cannot place, so the surface is not this call and the caller reads it another way.
func TestParseApplicationRejectsTrailingOperand(t *testing.T) {
	if _, ok := Builtin().ParseApplication("en", "the sum of 2 and 3 and 4", stubHead); ok {
		t.Fatal("a binary call with a third operand should not parse")
	}
}

// An unresolvable head (an unlearned operation) fails the parse, so the unknown
// surface falls through to the code engine intact.
func TestParseApplicationRejectsUnknownHead(t *testing.T) {
	if _, ok := Builtin().ParseApplication("en", "the wibble of 1 and 2", stubHead); ok {
		t.Fatal("a non-operation head should not parse as a call")
	}
}

// A call broken off after its coordinator does not half-parse: the missing operand
// fails the whole surface.
func TestParseApplicationRejectsPartial(t *testing.T) {
	if _, ok := Builtin().ParseApplication("en", "the double of 4 and", stubHead); ok {
		t.Fatal("a call missing its operand should not parse")
	}
}

// A bare value opens no call, so it is not the operation-application this
// construction reads.
func TestParseApplicationRejectsBareLiteral(t *testing.T) {
	if _, ok := Builtin().ParseApplication("en", "6", stubHead); ok {
		t.Fatal("a bare literal is not a call")
	}
}

// A tongue that seeds no call surface (no determiner, no genitive linker) parses no
// call, so the construction is declared in data and a tongue without it falls back.
func TestParseApplicationUnseededTongue(t *testing.T) {
	if _, ok := Builtin().ParseApplication("tr", "the double of 4", stubHead); ok {
		t.Fatal("a tongue with no seeded call surface should not parse a call")
	}
}

// A list literal "the list of A and B and C" is a variadic constructor: it reads
// elements until the coordinator runs out, where an operation is fixed-arity.
func TestParseApplicationListLiteral(t *testing.T) {
	app, ok := Builtin().ParseApplication("en", "the list of 1 and 2 and 3", stubHead)
	if !ok {
		t.Fatal("a list literal should parse")
	}
	want := Application{List: true, Args: []Application{{Lit: "1"}, {Lit: "2"}, {Lit: "3"}}}
	if !reflect.DeepEqual(app, want) {
		t.Fatalf("ParseApplication = %+v, want %+v", app, want)
	}
}

// A list literal sits where an operand does, so a reducer over a list parses as the
// operation applied to the list constructor.
func TestParseApplicationListArgument(t *testing.T) {
	app, ok := Builtin().ParseApplication("en", "the total of the list of 1 and 2 and 3", stubHead)
	if !ok {
		t.Fatal("a reducer over a list literal should parse")
	}
	want := Application{Head: "total", Args: []Application{
		{List: true, Args: []Application{{Lit: "1"}, {Lit: "2"}, {Lit: "3"}}},
	}}
	if !reflect.DeepEqual(app, want) {
		t.Fatalf("ParseApplication = %+v, want %+v", app, want)
	}
}

// A list element is itself an operand, so a call sits inside a list.
func TestParseApplicationListOfCalls(t *testing.T) {
	app, ok := Builtin().ParseApplication("en", "the list of the double of 4 and 5", stubHead)
	if !ok {
		t.Fatal("a list of a call and a literal should parse")
	}
	want := Application{List: true, Args: []Application{
		{Head: "double", Args: []Application{{Lit: "4"}}},
		{Lit: "5"},
	}}
	if !reflect.DeepEqual(app, want) {
		t.Fatalf("ParseApplication = %+v, want %+v", app, want)
	}
}

func TestParseBareCallUnary(t *testing.T) {
	app, ok := Builtin().ParseBareCall("en", "double 4", stubHead)
	if !ok {
		t.Fatal("a bare unary call should parse")
	}
	want := Application{Head: "double", Args: []Application{{Lit: "4"}}}
	if !reflect.DeepEqual(app, want) {
		t.Fatalf("ParseBareCall = %+v, want %+v", app, want)
	}
}

func TestParseBareCallNAry(t *testing.T) {
	app, ok := Builtin().ParseBareCall("en", "sum 6 and 7", stubHead)
	if !ok {
		t.Fatal("a bare binary call should parse")
	}
	want := Application{Head: "add", Args: []Application{{Lit: "6"}, {Lit: "7"}}}
	if !reflect.DeepEqual(app, want) {
		t.Fatalf("ParseBareCall = %+v, want %+v", app, want)
	}
}

// A bare call takes a genitive operand where a value would sit, so the two call
// surfaces compose.
func TestParseBareCallNestedGenitiveArg(t *testing.T) {
	app, ok := Builtin().ParseBareCall("en", "sum the double of 4 and 5", stubHead)
	if !ok {
		t.Fatal("a bare call with a genitive operand should parse")
	}
	want := Application{Head: "add", Args: []Application{
		{Head: "double", Args: []Application{{Lit: "4"}}},
		{Lit: "5"},
	}}
	if !reflect.DeepEqual(app, want) {
		t.Fatalf("ParseBareCall = %+v, want %+v", app, want)
	}
}

// All-or-nothing: a lone operation word, a missing coordinator, a missing operand,
// a trailing word, and an unknown leading word each fail, so an ordinary phrase that
// merely opens with an operation word is not read as a call.
func TestParseBareCallRejections(t *testing.T) {
	for _, s := range []string{
		"double",         // no operand
		"sum 6 7",        // binary call missing the coordinator
		"double 4 and",   // missing operand after the coordinator
		"double 4 more",  // a trailing word the call cannot place
		"lion 4",         // the leading word names no operation
	} {
		if _, ok := Builtin().ParseBareCall("en", s, stubHead); ok {
			t.Fatalf("%q should not parse as a bare call", s)
		}
	}
}
