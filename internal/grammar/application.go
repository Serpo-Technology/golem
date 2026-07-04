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
// node is a reference to a parameter inside a function body ("the number"); the
// same node with Each set is the element of a map ("each number"), the per-element
// reference the caller lifts into a map over the list parameter. A List node is
// the list literal "the list of A and B and C", its elements in Args, variadic
// where an operation is fixed-arity. A Cond node is a conditional, a choice
// between two branches on a comparison. It is the neutral structure the call and
// body parsers yield, which the caller walks into the computation it means, so the
// grammar produces the form while the code faculty owns the meaning.
type Application struct {
	Head string        // the operation's canonical name; empty for a literal, a var, or a list
	Lit  string        // the literal value, when Head, Var, List, and Cond are all empty
	Var  string        // a parameter reference inside a body, "the number", or a map element with Each
	Each bool          // the Var is the element of a map, "each number", before it is lifted into a map
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

// ExtremalFunc resolves an extremal word to the comparison symbol the reduction it
// names turns on ("greatest" -> ">", "smallest" -> "<"). It is a namespace apart
// from HeadFunc and CmpFunc so an extremal word never reads as a call head or a
// conditional relator, and so the symbol stays grounded in the extremal concept
// rather than a word-to-symbol table in the grammar. ok is false for a word that
// names no extremal.
type ExtremalFunc func(word string) (symbol string, ok bool)

// PropertyFunc resolves a property word to the property concept it names ("magnitude"),
// returning the concept's name and whether the word names a property. It is a namespace
// apart from the others so a property word never reads as a call head, relator, or extremal;
// the property's grounded meaning stays in the concept, the grammar only needing to know the
// word names one. ok is false for a word that names no property.
type PropertyFunc func(word string) (kind string, ok bool)

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
	det, linker, coord, listMarker, eachMarker string
	params                                     []string
	head                                       HeadFunc
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
		listMarker: p.surfaceWord(lang, "list"), eachMarker: p.surfaceWord(lang, "each"),
		params: params, head: head}
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

// ParseInfixCall reads the operator-register call "<A> <op> <B>": a binary
// operation word or symbol standing between its two operands, the register
// arithmetic is written in ("3 + 4", "3 plus 4"). The head resolves through the
// same grounded callback as every call form, so a symbol reads as a call head
// only when the graph names an operation with it, and each side reads as a full
// operand, so a nested of-construction still sits where a number would. It is
// all-or-nothing like the other call forms: one head, a whole operand on each
// side, every token consumed.
func (p *Params) ParseInfixCall(lang, utterance string, head HeadFunc) (Application, bool) {
	p = p.get()
	if lang == "" {
		lang = "en"
	}
	c := p.callCtx(lang, nil, head)
	if c == nil {
		return Application{}, false
	}
	toks, _ := Tokenize(utterance)
	if len(toks) < 3 {
		return Application{}, false
	}
	for i := 1; i < len(toks)-1; i++ {
		name, arity, ok := head(toks[i])
		if !ok || arity != 2 {
			continue
		}
		left, ln, lok := c.operand(toks[:i])
		if !lok || ln != i {
			continue
		}
		right, rn, rok := c.operand(toks[i+1:])
		if !rok || rn != len(toks)-i-1 {
			continue
		}
		return Application{Head: name, Args: []Application{left, right}}, true
	}
	return Application{}, false
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
	// "each <element>" is the element of a map, "the double of each number"; it is a
	// reference the caller lifts into a map over the list parameter once the body is
	// read, so the element name is taken as it stands and need not be a parameter.
	if c.eachMarker != "" && toks[0] == c.eachMarker && len(toks) >= 2 {
		return Application{Var: toks[1], Each: true}, 2, true
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

// ParseInverse reads a goal that names a new function as the inverse of a known
// one, "the halving is the inverse of the double": the copula joins the two sides
// (read from the closed class, not a literal "is"), the inverse relator and the
// genitive linker are seeded surface words, and the operation inverted is resolved
// through head. It returns the new function's name and the canonical name of the
// unary operation it inverts. The form is grounded, not matched: the operation must
// resolve and be unary, and the new name must NOT resolve (it is the function the
// goal asks to be discovered), so an ordinary copula statement and a goal naming no
// known operation both decline.
func (p *Params) ParseInverse(lang, utterance string, head HeadFunc) (newName, op string, ok bool) {
	p = p.get()
	if lang == "" {
		lang = "en"
	}
	inv := p.surfaceWord(lang, "inverse")
	linker := p.surfaceWord(lang, "genitive")
	if inv == "" || linker == "" || head == nil {
		return "", "", false
	}
	toks, _ := Tokenize(utterance)
	cop := -1
	for i, t := range toks {
		if p.category(lang, t) == catCop {
			cop = i
			break
		}
	}
	if cop < 1 || cop+1 >= len(toks) {
		return "", "", false
	}
	// The new function is the content word just before the copula; a function word
	// there is no name.
	newName = toks[cop-1]
	if p.isFunctionWordLang(lang, newName) {
		return "", "", false
	}
	// The right side names the inverse of an operation: the inverse relator, then the
	// genitive linker, then the operation as the final word.
	rhs := toks[cop+1:]
	invAt, linkAt := -1, -1
	for i, t := range rhs {
		if t == inv && invAt < 0 {
			invAt = i
		} else if t == linker && invAt >= 0 {
			linkAt = i
			break
		}
	}
	if linkAt < 0 || linkAt+1 >= len(rhs) {
		return "", "", false
	}
	opWord := rhs[len(rhs)-1]
	if newName == opWord {
		return "", "", false
	}
	// The operation inverted must resolve and be unary; the new name must not resolve,
	// since it is the function the goal asks to be discovered.
	canonical, arity, resolved := head(opWord)
	if !resolved || arity != 1 {
		return "", "", false
	}
	if _, _, taken := head(newName); taken {
		return "", "", false
	}
	return newName, canonical, true
}

// ParseExtremal reads a goal that names a new function by an extremal property of a
// list, "the peak is the greatest": the copula joins the two sides (read from the
// closed class, not a literal "is"), the new function's name is the content word
// before the copula, and the right side names an extremal, the determiner then a
// word the ext callback resolves to the comparison the reduction turns on. It
// returns the new function's name and that comparison symbol (">", the maximum;
// "<", the minimum). The list parameter is the caller's to supply, unspoken here as
// the inverse's parameter is. The form is grounded, not matched: the new name must
// not already name an operation or an extremal (it is the function to be
// discovered), and the right side's word must resolve through ext, so an ordinary
// copula statement declines.
func (p *Params) ParseExtremal(lang, utterance string, head HeadFunc, ext ExtremalFunc) (newName, symbol string, ok bool) {
	p = p.get()
	if lang == "" {
		lang = "en"
	}
	det := p.Article(lang)
	if det == "" || head == nil || ext == nil {
		return "", "", false
	}
	toks, _ := Tokenize(utterance)
	cop := -1
	for i, t := range toks {
		if p.category(lang, t) == catCop {
			cop = i
			break
		}
	}
	if cop < 1 || cop+1 >= len(toks) {
		return "", "", false
	}
	// The new function is the content word just before the copula; a function word
	// there is no name, and a name that already resolves as an operation or an
	// extremal is not one to discover.
	newName = toks[cop-1]
	if p.isFunctionWordLang(lang, newName) {
		return "", "", false
	}
	if _, _, taken := head(newName); taken {
		return "", "", false
	}
	if _, taken := ext(newName); taken {
		return "", "", false
	}
	// The right side is the determiner then an extremal word the ext callback names,
	// and nothing more, so a longer predicate is not read as a bare extremal.
	rhs := toks[cop+1:]
	if len(rhs) != 2 || rhs[0] != det {
		return "", "", false
	}
	sym, resolved := ext(rhs[1])
	if !resolved {
		return "", "", false
	}
	return newName, sym, true
}

// ParseProperty reads a property goal, "the abs is the magnitude": the new function before
// the copula, a property word the prop callback names after it. It mirrors ParseExtremal,
// the right side a determiner then one property word and nothing more, the new name a content
// word that does not already resolve as an operation or a property. It returns the new name
// and the property word, the daemon reading that word's grounded meaning to drive the search.
func (p *Params) ParseProperty(lang, utterance string, head HeadFunc, prop PropertyFunc) (newName, propertyWord string, ok bool) {
	p = p.get()
	if lang == "" {
		lang = "en"
	}
	det := p.Article(lang)
	if det == "" || head == nil || prop == nil {
		return "", "", false
	}
	toks, _ := Tokenize(utterance)
	cop := -1
	for i, t := range toks {
		if p.category(lang, t) == catCop {
			cop = i
			break
		}
	}
	if cop < 1 || cop+1 >= len(toks) {
		return "", "", false
	}
	newName = toks[cop-1]
	if p.isFunctionWordLang(lang, newName) {
		return "", "", false
	}
	if _, _, taken := head(newName); taken {
		return "", "", false
	}
	if _, taken := prop(newName); taken {
		return "", "", false
	}
	rhs := toks[cop+1:]
	if len(rhs) != 2 || rhs[0] != det {
		return "", "", false
	}
	if _, resolved := prop(rhs[1]); !resolved {
		return "", "", false
	}
	return newName, rhs[1], true
}
