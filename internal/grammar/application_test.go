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

func stubExt(word string) (string, bool) {
	switch word {
	case "greatest", "maximum":
		return ">", true
	case "smallest", "minimum":
		return "<", true
	}
	return "", false
}

// An extremal goal names a new function by the extreme of a list, the comparison
// the reduction turns on read from the extremal word.
func TestParseExtremal(t *testing.T) {
	name, sym, ok := Builtin().ParseExtremal("en", "the peak is the greatest", stubHead, stubExt)
	if !ok || name != "peak" || sym != ">" {
		t.Fatalf("ParseExtremal = (%q, %q, %v), want (peak, >, true)", name, sym, ok)
	}
	name, sym, ok = Builtin().ParseExtremal("en", "the trough is the smallest", stubHead, stubExt)
	if !ok || name != "trough" || sym != "<" {
		t.Fatalf("the smallest should be the minimum: (%q, %q, %v)", name, sym, ok)
	}
}

// The form is grounded, so a copula statement with no extremal, a new name that
// already names an operation or an extremal, and a longer predicate all decline.
func TestParseExtremalRejections(t *testing.T) {
	b := Builtin()
	for _, s := range []string{
		"the dog is an animal",         // no extremal word on the right
		"the peak is the double",       // the right side names an operation, not an extremal
		"the double is the greatest",   // the new name already names an operation
		"the greatest is the peak",     // the new name is itself an extremal
		"the peak is the greatest one", // a longer predicate, not a bare extremal
	} {
		if name, sym, ok := b.ParseExtremal("en", s, stubHead, stubExt); ok {
			t.Errorf("%q should not parse as an extremal goal, got (%q, %q)", s, name, sym)
		}
	}
}

// An inverse goal names a new function as the inverse of a known unary operation.
func TestParseInverse(t *testing.T) {
	name, op, ok := Builtin().ParseInverse("en", "the halving is the inverse of the double", stubHead)
	if !ok || name != "halving" || op != "double" {
		t.Fatalf("ParseInverse = (%q, %q, %v), want (halving, double, true)", name, op, ok)
	}
}

// The form is grounded, so an ordinary copula statement, an inverse of a
// non-operation, an inverse of a binary operation, and a copula clause with no
// inverse marker all decline.
func TestParseInverseRejections(t *testing.T) {
	b := Builtin()
	for _, s := range []string{
		"the dog is an animal",                       // no inverse marker, no operation
		"the halving is the inverse of the cat",      // the inverted word names no operation
		"the unsum is the inverse of the sum",        // the inverted operation is binary
		"the halving is the double of the number",    // no inverse marker
	} {
		if name, op, ok := b.ParseInverse("en", s, stubHead); ok {
			t.Errorf("%q should not parse as an inverse goal, got (%q, %q)", s, name, op)
		}
	}
}

// The inverse marker is a seeded surface word, so a tongue without it declines.
func TestParseInverseUnseededTongue(t *testing.T) {
	if _, _, ok := Builtin().ParseInverse("tr", "the halving is the inverse of the double", stubHead); ok {
		t.Fatal("a tongue with no seeded inverse marker should decline")
	}
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

// bodyHead resolves the operations a body names, the function being defined among
// them ("factorial") so a self-call reads as a call. bodyCmp resolves the
// comparison relators, the empty relator the bare copula's equality.
func bodyHead(word string) (string, int, bool) {
	switch word {
	case "product":
		return "multiply", 2, true
	case "difference":
		return "subtract", 2, true
	case "sum":
		return "add", 2, true
	case "factorial":
		return "factorial", 1, true
	case "double":
		return "double", 1, true
	}
	return "", 0, false
}

func bodyCmp(relator string) (string, bool) {
	switch relator {
	case "above":
		return ">", true
	case "below":
		return "<", true
	case "":
		return "==", true
	}
	return "", false
}

func TestParseBodyConditional(t *testing.T) {
	app, ok := Builtin().ParseBody("en",
		"the number if the number is above the count else the count",
		[]string{"number", "count"}, bodyHead, bodyCmp)
	if !ok {
		t.Fatal("a conditional body should parse")
	}
	want := Application{Cond: &Conditional{
		Cmp:  ">",
		Left: Application{Var: "number"}, Right: Application{Var: "count"},
		Then: Application{Var: "number"}, Else: Application{Var: "count"},
	}}
	if !reflect.DeepEqual(app, want) {
		t.Fatalf("ParseBody = %+v, want %+v", app, want)
	}
}

// The bare copula with no ordering relator is equality.
func TestParseBodyEquality(t *testing.T) {
	app, ok := Builtin().ParseBody("en", "1 if the number is 0 else the number",
		[]string{"number"}, bodyHead, bodyCmp)
	if !ok {
		t.Fatal("an equality conditional should parse")
	}
	want := Application{Cond: &Conditional{
		Cmp:  "==",
		Left: Application{Var: "number"}, Right: Application{Lit: "0"},
		Then: Application{Lit: "1"}, Else: Application{Var: "number"},
	}}
	if !reflect.DeepEqual(app, want) {
		t.Fatalf("ParseBody = %+v, want %+v", app, want)
	}
}

// A comma before "else" is shed by tokenization, so the conditional reads the same.
func TestParseBodyToleratesComma(t *testing.T) {
	with, ok := Builtin().ParseBody("en",
		"the number if the number is above the count, else the count",
		[]string{"number", "count"}, bodyHead, bodyCmp)
	if !ok {
		t.Fatal("a conditional with a comma before else should parse")
	}
	without, _ := Builtin().ParseBody("en",
		"the number if the number is above the count else the count",
		[]string{"number", "count"}, bodyHead, bodyCmp)
	if !reflect.DeepEqual(with, without) {
		t.Fatalf("the comma should not change the parse: %+v vs %+v", with, without)
	}
}

// A self-call inside the body reads as an ordinary call, the recursion that lets a
// body name the function being defined before it is in the inventory.
func TestParseBodyRecursion(t *testing.T) {
	app, ok := Builtin().ParseBody("en",
		"the product of the number and the factorial of the difference of the number and 1",
		[]string{"number"}, bodyHead, bodyCmp)
	if !ok {
		t.Fatal("a body with a self-call should parse")
	}
	want := Application{Head: "multiply", Args: []Application{
		{Var: "number"},
		{Head: "factorial", Args: []Application{
			{Head: "subtract", Args: []Application{{Var: "number"}, {Lit: "1"}}},
		}},
	}}
	if !reflect.DeepEqual(app, want) {
		t.Fatalf("ParseBody = %+v, want %+v", app, want)
	}
}

// A plain body with no conditional is a single operand: a parameter reference, or a
// call over the parameters.
func TestParseBodyPlain(t *testing.T) {
	app, ok := Builtin().ParseBody("en", "the product of the number and 2",
		[]string{"number"}, bodyHead, bodyCmp)
	if !ok {
		t.Fatal("a plain arithmetic body should parse")
	}
	want := Application{Head: "multiply", Args: []Application{{Var: "number"}, {Lit: "2"}}}
	if !reflect.DeepEqual(app, want) {
		t.Fatalf("ParseBody = %+v, want %+v", app, want)
	}
}

// A body naming "each <element>" reads the element as a map reference, the form
// the caller lifts into a map over the list parameter. The element name is taken
// as it stands, so it need not be one of the parameters in scope.
func TestParseBodyMapEach(t *testing.T) {
	app, ok := Builtin().ParseBody("en", "the double of each number",
		[]string{"numbers"}, bodyHead, bodyCmp)
	if !ok {
		t.Fatal("a map body naming an element should parse")
	}
	want := Application{Head: "double", Args: []Application{{Var: "number", Each: true}}}
	if !reflect.DeepEqual(app, want) {
		t.Fatalf("ParseBody = %+v, want %+v", app, want)
	}
}

// All-or-nothing: a leading "if", an "else" with no consequent, and an unconsumed
// tail each fail rather than half-parsing.
func TestParseBodyRejections(t *testing.T) {
	cases := []string{
		"if the number is above the count else the count", // a leading if, no then
		"the number if the number is above the count else", // an else with no branch
		"the number extra",                                 // an unconsumed tail
	}
	for _, s := range cases {
		if _, ok := Builtin().ParseBody("en", s, []string{"number", "count"}, bodyHead, bodyCmp); ok {
			t.Errorf("%q should not parse as a body", s)
		}
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
