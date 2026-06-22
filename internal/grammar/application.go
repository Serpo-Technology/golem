package grammar

// An operation-application call is a construction like any other: the form is the
// genitive surface "the <operation> of <arg> and <arg>...", the meaning is the
// operation applied to its operands. The grammar holds the form, parsing the
// surface into the application it names; which operation a word names, and how
// many operands it takes, is the caller's grounded concern, supplied as a
// resolver so the call reads cross-tongue through the concept graph rather than a
// word list kept here. So the same of-construction reads "the double of 4" and
// "the sum of 6 and 7", and a tongue declares its call surface in seeded words:
// the determiner, the genitive linker, the coordinator, and the word that marks a
// list literal "the list of A and B and C".

// Application is one node of an operation-application call: Head is the operation's
// canonical name as the resolver names it, Args its operands in order. A leaf
// carries no Head and a bare Lit, a value sitting where an operand does. A Var
// node is a reference to a parameter inside a function body ("the number"). A List
// node is the list literal "the list of A and B and C", its elements in Args,
// variadic where an operation is fixed-arity. A Cond node is a conditional, a
// choice between two branches on a comparison. It is the neutral structure the
// call and body parsers yield, which the caller walks into the computation it
// means, so the grammar produces the form while the code faculty owns the meaning.
type Application struct {
	Head string        // the operation's canonical name; empty for a literal, a var, or a list
	Lit  string        // the literal value, when Head, Var, List, and Cond are all empty
	Var  string        // a parameter reference inside a body, "the number"
	List bool          // the operands are a list literal's elements, not an operation's arguments
	Cond *Conditional  // a choice between two branches on a comparison, when set
	Args []Application // the operands, as many as the head's arity, or the list's elements
}

// Conditional is a body that chooses between two branches on a comparison: "the
// number if the number is above the count else the count" is the number when it is
// above the count, else the count. Cmp is the comparison operator's symbol (">",
// "<", "=="), the canonical floor form the caller resolved each relator word to;
// Left and Right are the comparison's operands; Then is taken when it holds, Else
// otherwise.
type Conditional struct {
	Cmp         string
	Left, Right Application
	Then, Else  Application
}

// HeadFunc resolves an operation word to the operation it names: the canonical
// name the call carries and the arity that drives how many operands it reads. It
// is the grounded, cross-tongue lookup the caller supplies, so the grammar parses
// the call's form while which operation a word names stays a concern of the
// concept graph. ok is false for a word that names no operation, which fails the
// parse so the surface is read another way.
type HeadFunc func(word string) (name string, arity int, ok bool)

// CmpFunc resolves a comparison relator word to the operator symbol it names
// ("above" -> ">"), the empty word naming the bare copula's default comparison
// (equality, "=="). It is a namespace apart from HeadFunc so a comparison word
// never reads as a call head, and so the operator symbol stays grounded in the
// comparison concept rather than a word-to-symbol table in the grammar. ok is
// false for a word that names no comparison.
type CmpFunc func(relator string) (symbol string, ok bool)

// ParseApplication reads the operation-application call "the <operation> of <arg>
// and <arg>..." into the application it means, the of-construction run form to
// meaning. The determiner, the genitive linker, and the coordinator are the
// tongue's seeded surface words, so a new tongue declares its call surface in
// data; the operation word and its arity come from the head resolver, so arity
// drives the nesting (a unary operand reads one argument, a binary two) and a
// nested call sits where a number would. It is all-or-nothing: the parse must
// consume every token, the surface must open a call, and every head must resolve,
// so an ill-formed or not-a-call surface returns false and the caller reads it
// another way, never a half-built tree.
func (p *Params) ParseApplication(lang, utterance string, head HeadFunc) (Application, bool) {
	p = p.get()
	if lang == "" {
		lang = "en"
	}
	c := p.callCtx(lang, nil, head)
	if c == nil {
		return Application{}, false
	}
	toks, _ := Tokenize(utterance)
	app, n, ok := c.operand(toks)
	// The whole utterance must be one call and nothing left over: a leftover token
	// (a binary head handed a third operand) or a bare literal that opens no call
	// is not the call this construction reads. A list literal stands as a call too.
	if !ok || n != len(toks) || (app.Head == "" && !app.List) {
		return Application{}, false
	}
	return app, true
}

// parseCtx carries the surface markers and the grounded resolver an operand parse
// reads, so the recursion threads one value rather than a handful of arguments.
// The markers are the tongue's seeded words; params are the names in scope inside
// a function body, empty for a bare call; head resolves an operation word to its
// name and arity.
type parseCtx struct {
	det, linker, coord, listMarker string
	params                         []string
	head                           HeadFunc
}

// callCtx builds the operand-parse context from a tongue's seeded call-surface
// words, nil when the tongue declares none. params are the body parameters in
// scope, nil for a bare call.
func (p *Params) callCtx(lang string, params []string, head HeadFunc) *parseCtx {
	det := p.Article(lang)
	linker := p.surfaceWord(lang, "genitive")
	coord := p.Coordinator(lang)
	if det == "" || linker == "" || coord == "" || head == nil {
		return nil
	}
	return &parseCtx{det: det, linker: linker, coord: coord,
		listMarker: p.surfaceWord(lang, "list"), params: params, head: head}
}

// ParseBareCall reads the bare operation call "double 4" or "add 6 and 7", the
// operation word applied straight to its arguments with none of the genitive
// scaffolding the of-construction carries: the leading word names the operation,
// resolved cross-tongue by the head callback, and its arity many operands follow,
// joined by the coordinator and each read as the of-construction reads one, so a
// nested "the double of 4" still sits where a number would. It is all-or-nothing
// like ParseApplication: the leading word must name an operation and every token
// must be consumed, so an ordinary phrase that merely opens with an operation word
// is not mistaken for a call.
func (p *Params) ParseBareCall(lang, utterance string, head HeadFunc) (Application, bool) {
	p = p.get()
	if lang == "" {
		lang = "en"
	}
	c := p.callCtx(lang, nil, head)
	if c == nil {
		return Application{}, false
	}
	toks, _ := Tokenize(utterance)
	// A bare call is the operation word and at least one operand; a lone word is no
	// call.
	if len(toks) < 2 {
		return Application{}, false
	}
	name, arity, ok := head(toks[0])
	if !ok || arity < 1 {
		return Application{}, false
	}
	args, n, ok := c.arguments(toks[1:], arity)
	if !ok || n != len(toks)-1 {
		return Application{}, false
	}
	return Application{Head: name, Args: args}, true
}

// operand reads one operand from the front of the tokens and reports how many it
// spanned: a bare token is a literal; "the <param>" inside a body is a reference
// to a parameter in scope; "the list of A and B and ..." is a variadic list
// literal; and "the <head> of <A> and <B> and ..." is the operation its word
// names over as many operands as its arity, each read the same way, the recursion
// that lets a call nest where a value would. It fails on a determiner that opens
// no resolvable head, a missing coordinator between operands, or running out of
// tokens mid-arity, so a half-formed call does not half-parse.
func (c *parseCtx) operand(toks []string) (Application, int, bool) {
	if len(toks) == 0 {
		return Application{}, 0, false
	}
	// A token that does not open a call is a literal operand, a value sitting where
	// an operand does.
	if toks[0] != c.det {
		return Application{Lit: toks[0]}, 1, true
	}
	// "the <param>" is a reference to a parameter in scope, read before "the" is
	// taken as the opening of a call.
	if len(toks) >= 2 && isParam(c.params, toks[1]) {
		return Application{Var: toks[1]}, 2, true
	}
	// The determiner opens a call only when a head and the genitive linker follow.
	if len(toks) < 3 || toks[2] != c.linker {
		return Application{}, 0, false
	}
	// "the list of A and B and ...": a list literal, a variadic constructor where an
	// operation is fixed-arity, so it reads operands until the coordinator runs out.
	if c.listMarker != "" && toks[1] == c.listMarker {
		elems, n, ok := c.arguments(toks[3:], -1)
		if !ok {
			return Application{}, 0, false
		}
		return Application{List: true, Args: elems}, 3 + n, true
	}
	// Otherwise the head names an operation of at least one operand.
	name, arity, ok := c.head(toks[1])
	if !ok || arity < 1 {
		return Application{}, 0, false
	}
	args, n, ok := c.arguments(toks[3:], arity)
	if !ok {
		return Application{}, 0, false
	}
	return Application{Head: name, Args: args}, 3 + n, true
}

// arguments reads coordinator-joined operands and reports how many tokens they
// spanned: exactly want of them for a fixed-arity call, or as many as are joined
// when want is negative (a variadic list). A fixed call short of its arity with no
// coordinator is ill-formed; a list simply ends where the coordinator runs out.
func (c *parseCtx) arguments(toks []string, want int) ([]Application, int, bool) {
	var args []Application
	rest, consumed := toks, 0
	for {
		arg, n, ok := c.operand(rest)
		if !ok {
			return nil, 0, false
		}
		args = append(args, arg)
		rest, consumed = rest[n:], consumed+n
		if want >= 0 && len(args) == want {
			return args, consumed, true
		}
		if len(rest) == 0 || rest[0] != c.coord {
			if want < 0 {
				return args, consumed, true
			}
			return nil, 0, false
		}
		rest, consumed = rest[1:], consumed+1
	}
}

// isParam reports whether a word names one of the parameters in scope, so "the
// number" inside a body reads as a reference to it rather than the opening of a
// call.
func isParam(params []string, word string) bool {
	for _, p := range params {
		if p == word {
			return true
		}
	}
	return false
}

// ParseBody reads a function body taught in language into the application it
// means, the body of "the double of a number is BODY" read with the parameters in
// scope. A conditional "<then> if <comparison> else <else>" chooses between two
// branches; any other body is a plain operand, a parameter reference, a call, a
// list, or a nested computation. callHead resolves an operation word, the function
// being defined among them so a self-call (recursion) reads as an ordinary call;
// cmp resolves a comparison relator word to its operator symbol. It is
// all-or-nothing, consuming every body token, with the seeded if/else markers
// splitting the conditional and the copula joining the comparison.
func (p *Params) ParseBody(lang, body string, params []string, callHead HeadFunc, cmp CmpFunc) (Application, bool) {
	p = p.get()
	if lang == "" {
		lang = "en"
	}
	c := p.callCtx(lang, params, callHead)
	if c == nil {
		return Application{}, false
	}
	toks, _ := Tokenize(body)
	if len(toks) == 0 {
		return Application{}, false
	}
	// The if/else markers split a conditional; without them seeded, or with no "if"
	// in the body, the body is a single operand.
	ifWord := p.surfaceWord(lang, "if")
	elseWord := p.surfaceWord(lang, "else")
	ifAt, elseAt := -1, -1
	if ifWord != "" && elseWord != "" {
		for i, t := range toks {
			switch {
			case t == ifWord && ifAt < 0:
				ifAt = i
			case t == elseWord && ifAt >= 0:
				elseAt = i
			}
			if elseAt >= 0 {
				break
			}
		}
	}
	if ifAt < 0 {
		app, n, ok := c.operand(toks)
		if !ok || n != len(toks) {
			return Application{}, false
		}
		return app, true
	}
	if ifAt == 0 || elseAt <= ifAt+1 || elseAt == len(toks)-1 {
		return Application{}, false
	}
	then, n, ok := c.operand(toks[:ifAt])
	if !ok || n != ifAt {
		return Application{}, false
	}
	cond, ok := p.bodyComparison(lang, toks[ifAt+1:elseAt], c, cmp)
	if !ok {
		return Application{}, false
	}
	els, m, ok := c.operand(toks[elseAt+1:])
	if !ok || m != len(toks)-elseAt-1 {
		return Application{}, false
	}
	cond.Then, cond.Else = then, els
	return Application{Cond: cond}, true
}

// bodyComparison reads a conditional's test "<A> is <relator> <B>" into the
// operator symbol and its operands. The copula opens the comparison (read from the
// closed class, not a literal "is"); an ordered relator word resolves through cmp
// to ">" or "<", and the bare copula with no relator is equality, its "==" the
// comparison concept's symbol. The branches are left empty for the caller to fill.
func (p *Params) bodyComparison(lang string, toks []string, c *parseCtx, cmp CmpFunc) (*Conditional, bool) {
	left, n, ok := c.operand(toks)
	if !ok || n >= len(toks) || p.category(lang, toks[n]) != catCop {
		return nil, false
	}
	rest := toks[n+1:]
	if len(rest) == 0 {
		return nil, false
	}
	sym, ok := cmp(rest[0])
	if ok {
		rest = rest[1:] // an ordered relator, consumed; the bare copula keeps its operand
	} else if sym, ok = cmp(""); !ok {
		return nil, false
	}
	right, m, ok := c.operand(rest)
	if !ok || m != len(rest) {
		return nil, false
	}
	return &Conditional{Cmp: sym, Left: left, Right: right}, true
}
